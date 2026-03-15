package repl

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/proc/src_v1/go/proc"
	tsnetlib "dialtone/dev/plugins/tsnet/src_v1/go"
	"github.com/nats-io/nats.go"
)

const (
	defaultNATSURL = "nats://127.0.0.1:4222"
	defaultRoom    = "index"
	commandSubject = "repl.cmd"
	commandQueue   = "repl.leader"
)

const (
	frameTypeInput     = "input"
	frameTypeLine      = "line"
	frameTypeProbe     = "probe"
	frameTypeServer    = "server"
	frameTypeHeartbeat = "heartbeat"
	frameTypeDaemon    = "daemon"
	frameTypeJoin      = "join"
	frameTypeLeft      = "left"
	frameTypeChat      = "chat"
	frameTypeCommand   = "command"
	frameTypeControl   = "control"
	frameTypeError     = "error"
)

const controlJoinRoom = "join_room"
const controlRunHostSubtone = "run_host_subtone"

type Hooks struct {
	RunSubtoneWithEvents func(args []string, onEvent proc.SubtoneEventHandler) int
	ListManaged          func() []proc.ManagedProcessSnapshot
	KillManagedProcess   func(pid int) error
}

var (
	runSubtoneWithEventsFn = proc.RunSubtoneWithEvents
	listManagedFn          = proc.ListManagedProcesses
	killManagedProcessFn   = proc.KillManagedProcess
)

// SetHooksForTest overrides REPL side-effect functions and returns a restore function.
func SetHooksForTest(h Hooks) func() {
	prevRunSubtoneWithEvents := runSubtoneWithEventsFn
	prevListManaged := listManagedFn
	prevKillManaged := killManagedProcessFn

	if h.RunSubtoneWithEvents != nil {
		runSubtoneWithEventsFn = h.RunSubtoneWithEvents
	}
	if h.ListManaged != nil {
		listManagedFn = h.ListManaged
	}
	if h.KillManagedProcess != nil {
		killManagedProcessFn = h.KillManagedProcess
	}
	return func() {
		runSubtoneWithEventsFn = prevRunSubtoneWithEvents
		listManagedFn = prevListManaged
		killManagedProcessFn = prevKillManaged
	}
}

type BusFrame struct {
	Type       string   `json:"type"`
	Scope      string   `json:"scope,omitempty"`
	Kind       string   `json:"kind,omitempty"`
	From       string   `json:"from,omitempty"`
	Target     string   `json:"target,omitempty"`
	Room       string   `json:"room,omitempty"`
	Version    string   `json:"version,omitempty"`
	OS         string   `json:"os,omitempty"`
	Arch       string   `json:"arch,omitempty"`
	ReplVer    string   `json:"repl_version,omitempty"`
	DaemonVer  string   `json:"daemon_version,omitempty"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	Prefix     string   `json:"prefix,omitempty"`
	Message    string   `json:"message,omitempty"`
	SubtonePID int      `json:"subtone_pid,omitempty"`
	LogPath    string   `json:"log_path,omitempty"`
	ExitCode   int      `json:"exit_code,omitempty"`
	Ready      bool     `json:"ready,omitempty"`
	ServerID   string   `json:"server_id,omitempty"`
	Timestamp  string   `json:"timestamp"`
}

type HostStatus struct {
	HostName      string `json:"hostname"`
	NATSURL       string `json:"nats_url"`
	Room          string `json:"room"`
	Subject       string `json:"subject"`
	NATSReachable bool   `json:"nats_reachable"`
	ServerSeen    bool   `json:"server_seen"`
	TSNetTailnet  string `json:"tsnet_tailnet"`
	TSAuthKey     bool   `json:"ts_authkey_present"`
	TSApiKey      bool   `json:"ts_api_key_present"`
	ChromeFound   bool   `json:"chrome_found"`
	ChromePath    string `json:"chrome_path"`
}

func Start(logFn func(category, msg string)) error {
	return RunLocal(logFn, nil)
}

func RunLocal(logFn func(category, msg string), args []string) error {
	fs := flag.NewFlagSet("repl-run", flag.ContinueOnError)
	promptName := fs.String("name", DefaultPromptName(), "Prompt name for this session")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if logFn == nil {
		logFn = func(string, string) {}
	}
	return runLocalSession(os.Stdin, os.Stdout, normalizePromptName(*promptName), logFn)
}

func RunLeader(args []string) error {
	fs := flag.NewFlagSet("repl-leader", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Primary REPL room")
	embedded := fs.Bool("embedded-nats", true, "Start embedded NATS on --nats-url")
	enableTSNet := fs.Bool("tsnet", true, "Start embedded tsnet identity on host when native tailscale is not already connected")
	tsnetNATSPort := fs.Int("tsnet-nats-port", 0, "Expose NATS over tsnet on this port (default: port from --nats-url)")
	hostname := fs.String("hostname", DefaultPromptName(), "Host name used in prompts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	nc, broker, usedURL, err := connectNATS(strings.TrimSpace(*natsURL), *embedded)
	if err != nil {
		return err
	}
	defer nc.Close()
	if broker != nil {
		defer broker.Close()
	}

	stopTSNet := func() {}
	var tsRuntime *tsnetRuntime
	tsnetStatusMessage := ""

	h := normalizePromptName(*hostname)
	roomName := sanitizeRoom(*room)
	serverID := h + "@" + roomName

	publishRoom := func(targetRoom string, f BusFrame) {
		targetRoom = sanitizeRoom(targetRoom)
		f.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
		f.ServerID = serverID
		if strings.TrimSpace(f.Room) == "" {
			f.Room = targetRoom
		}
		_ = publishFrame(nc, replRoomSubject(targetRoom), f)
	}
	publishScopedFrame := func(indexRoom string, f BusFrame) {
		indexRoom = sanitizeRoom(indexRoom)
		if indexRoom == "" {
			indexRoom = defaultRoom
		}
		f.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
		f.ServerID = serverID
		switch strings.TrimSpace(f.Scope) {
		case "subtone":
			if f.SubtonePID <= 0 {
				if strings.TrimSpace(f.Room) == "" {
					f.Room = indexRoom
				}
				_ = publishFrame(nc, replRoomSubject(indexRoom), f)
				return
			}
			if strings.TrimSpace(f.Room) == "" {
				f.Room = subtoneRoomName(f.SubtonePID)
			}
			_ = publishFrame(nc, replSubtoneSubject(f.SubtonePID), f)
		default:
			if strings.TrimSpace(f.Room) == "" {
				f.Room = indexRoom
			}
			_ = publishFrame(nc, replRoomSubject(indexRoom), f)
		}
	}

	if *enableTSNet {
		if active, provider, tailnet := tsnetlib.NativeTailnetConnected(); active {
			tsnetStatusMessage = fmt.Sprintf("Native tailscale already connected via %s; skipping embedded tsnet startup (tailnet=%s)", provider, strings.TrimSpace(tailnet))
			logs.Info("REPL native tailscale already connected via %s; skipping embedded tsnet startup (tailnet=%s)", provider, strings.TrimSpace(tailnet))
			publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: tsnetStatusMessage})
		} else {
			cleanup, upErr := startTSNetInstance(normalizeTSNetHostname(normalizePromptName(*hostname)))
			if upErr != nil {
				logs.Warn("REPL tsnet startup failed: %v", upErr)
			} else {
				tsRuntime = cleanup
				stopTSNet = func() {
					_ = tsRuntime.Close()
				}
			}
		}
	}
	defer stopTSNet()

	// Publish initial presence line to NATS so every connected client sees it.
	publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("Leader online on %s (subject=%s nats=%s)", h, replRoomSubject(roomName), usedURL)})
	logs.Info("REPL host serving: hostname=%s room=%s cmd_subject=%s nats=%s", h, roomName, commandSubject, usedURL)
	var tsnetListener net.Listener
	if tsRuntime != nil {
		targetAddr, parsedPort, parseErr := natsProxyTarget(usedURL)
		if parseErr != nil {
			logs.Warn("REPL tsnet NATS proxy parse failed: %v", parseErr)
		} else {
			exposePort := parsedPort
			if *tsnetNATSPort > 0 {
				exposePort = *tsnetNATSPort
			}
			ln, lnErr := tsRuntime.Listen("tcp", fmt.Sprintf(":%d", exposePort))
			if lnErr != nil {
				logs.Warn("REPL tsnet NATS proxy listen failed: %v", lnErr)
			} else {
				tsnetListener = ln
				go serveTCPProxy(tsnetListener, targetAddr)
				tsURL := fmt.Sprintf("nats://%s:%d", tsRuntime.DNSName, exposePort)
				tsnetStatusMessage = fmt.Sprintf("tsnet NATS endpoint: %s", tsURL)
				logs.Info("REPL tsnet NATS endpoint active: %s -> %s", tsURL, targetAddr)
				publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: tsnetStatusMessage})
			}
		}
	}
	defer func() {
		if tsnetListener != nil {
			_ = tsnetListener.Close()
		}
	}()

	presence := newPresenceTracker()
	subtones := newSubtoneRegistry(256)
	daemonTTL := 20 * time.Second
	var runMu sync.Mutex
	cmdSub, err := nc.QueueSubscribe(commandSubject, commandQueue, func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		switch frame.Type {
		case frameTypeProbe:
			targetRoom := sanitizeRoom(frame.Room)
			publishRoom(targetRoom, BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("Leader active on %s", h)})
			if strings.TrimSpace(tsnetStatusMessage) != "" {
				publishRoom(targetRoom, BusFrame{Type: frameTypeServer, Message: tsnetStatusMessage})
			}
		case frameTypeCommand:
			sender := normalizePromptName(frame.From)
			currentRoom := sanitizeRoom(frame.Room)
			if sender == "" {
				return
			}
			if currentRoom == "" {
				currentRoom = presence.ClientRoom(sender)
			}
			if currentRoom == "" {
				currentRoom = defaultRoom
			}

			raw := strings.TrimSpace(frame.Message)
			if strings.HasPrefix(raw, "/") {
				raw = strings.TrimSpace(strings.TrimPrefix(raw, "/"))
			}
			if targetHost, targetCommand, ok := parseTargetCommand(raw); ok {
				publishRoom(currentRoom, BusFrame{
					Type:    frameTypeControl,
					Target:  targetHost,
					Command: controlRunHostSubtone,
					Room:    currentRoom,
					Message: targetCommand,
				})
				publishRoom(currentRoom, BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "status",
					Room:    currentRoom,
					Message: fmt.Sprintf("Dispatching host subtone on %s.", targetHost),
				})
				return
			}
			args := strings.Fields(raw)
			if len(args) == 0 {
				return
			}

			presence.UpsertClient(sender, currentRoom, frame.Version, frame.OS, frame.Arch)
			publishRoom(currentRoom, BusFrame{Type: frameTypeInput, From: sender, Message: "/" + raw})

			if len(args) >= 3 && args[0] == "repl" && args[1] == "src_v3" && args[2] == "who" {
				publishPresenceReport(currentRoom, "who", presence.Snapshot(time.Now(), daemonTTL), publishRoom)
				return
			}
			if len(args) >= 3 && args[0] == "repl" && args[1] == "src_v3" && args[2] == "versions" {
				publishPresenceReport(currentRoom, "versions", presence.Snapshot(time.Now(), daemonTTL), publishRoom)
				return
			}

			if len(args) >= 4 && args[0] == "repl" && args[1] == "src_v3" && args[2] == "join" {
				targetRoom := sanitizeRoom(args[3])
				if targetRoom == "" {
					publishRoom(currentRoom, BusFrame{Type: frameTypeError, Message: "Usage: /repl src_v3 join <room-name> | /repl src_v3 who | /repl src_v3 versions"})
					return
				}
				if targetRoom == currentRoom {
					publishRoom(currentRoom, BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: fmt.Sprintf("%s is already in room %s", sender, targetRoom)})
					return
				}
				publishRoom(currentRoom, BusFrame{Type: frameTypeLeft, From: sender})
				publishRoom(currentRoom, BusFrame{
					Type:    frameTypeControl,
					Target:  sender,
					Command: controlJoinRoom,
					Room:    targetRoom,
					Message: fmt.Sprintf("switching %s to room %s", sender, targetRoom),
				})
				presence.UpsertClient(sender, targetRoom, frame.Version, frame.OS, frame.Arch)
				return
			}

			go func(in BusFrame) {
				runMu.Lock()
				defer runMu.Unlock()
				executeCommand(strings.TrimSpace(in.Message), currentRoom, subtones, func(frame BusFrame) {
					publishScopedFrame(currentRoom, frame)
				})
			}(BusFrame{Message: raw})
		}
	})
	if err != nil {
		return err
	}
	defer cmdSub.Unsubscribe()

	registrySub, err := nc.Subscribe(subtoneRegistrySubject, func(msg *nats.Msg) {
		if strings.TrimSpace(msg.Reply) == "" {
			return
		}
		req := subtoneRegistryRequest{}
		if len(msg.Data) > 0 {
			_ = json.Unmarshal(msg.Data, &req)
		}
		items := subtones.Snapshot(req.Count, listManagedFn())
		payload, err := encodeSubtoneRegistrySnapshot(items)
		if err != nil {
			return
		}
		_ = nc.Publish(msg.Reply, payload)
	})
	if err != nil {
		return err
	}
	defer registrySub.Unsubscribe()

	roomSub, err := nc.Subscribe("repl.room.*", func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		switch frame.Type {
		case frameTypeJoin:
			presence.UpsertClient(frame.From, frame.Room, frame.Version, frame.OS, frame.Arch)
		case frameTypeLeft:
			presence.RemoveClient(frame.From)
		case frameTypeDaemon:
			presence.UpsertDaemon(frame.From, frame.Room, frame.DaemonVer, frame.ReplVer, frame.OS, frame.Arch, time.Now())
		}
		if frame.Type == frameTypeDaemon {
			return
		}
		printFrame(os.Stdout, frame)
	})
	if err != nil {
		return err
	}
	defer roomSub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}

	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-heartbeat.C:
			for _, r := range presence.Rooms(roomName, time.Now(), daemonTTL) {
				publishRoom(r, BusFrame{Type: frameTypeHeartbeat, Message: "alive"})
			}
		case <-sig:
			publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: "Leader shutting down."})
			return nil
		}
	}
}

func RunJoin(args []string) error {
	fs := flag.NewFlagSet("repl-join", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared REPL room")
	name := fs.String("name", DefaultPromptName(), "Prompt name for this client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: join [room-name] [--nats-url URL] [--name HOST]")
	}
	if fs.NArg() == 1 {
		*room = fs.Arg(0)
	}

	nc, err := nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	prompt := normalizePromptName(*name)
	roomName := sanitizeRoom(*room)
	currentRoom := roomName
	currentSubj := replRoomSubject(currentRoom)
	natsAddr := strings.TrimSpace(*natsURL)

	var subMu sync.Mutex
	var sub *nats.Subscription
	var attachedSub *nats.Subscription
	attachedPID := 0
	var hostRunMu sync.Mutex
	var switchRoom func(string, bool) error
	var switchAttached func(int) error
	interactive := isInputTTY(os.Stdin)
	console := newJoinConsole(os.Stdout, prompt, interactive)

	onRoomFrame := func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		console.PrintFrame(frame)
		if frame.Type == frameTypeControl && frame.Target == prompt && frame.Command == controlJoinRoom {
			nextRoom := sanitizeRoom(frame.Room)
			_ = switchRoom(nextRoom, true)
			return
		}
		if frame.Type == frameTypeControl && frame.Target == prompt && frame.Command == controlRunHostSubtone {
			command := strings.TrimSpace(frame.Message)
			targetRoom := sanitizeRoom(frame.Room)
			if targetRoom == "" {
				targetRoom = currentRoom
			}
			if command == "" {
				return
			}
			go func(room, host, cmdText string) {
				hostRunMu.Lock()
				defer hostRunMu.Unlock()
				exitCode := proc.RunHostCommandWithEvents(cmdText, func(ev proc.SubtoneEvent) {
					switch ev.Type {
					case proc.SubtoneEventStarted:
						publishHostFrame := func(frame BusFrame) {
							switch strings.TrimSpace(frame.Scope) {
							case "subtone":
								if frame.SubtonePID <= 0 {
									frame.SubtonePID = ev.PID
								}
								if strings.TrimSpace(frame.Room) == "" {
									frame.Room = subtoneRoomName(frame.SubtonePID)
								}
								_ = publishFrame(nc, replSubtoneSubject(frame.SubtonePID), frame)
							default:
								if strings.TrimSpace(frame.Room) == "" {
									frame.Room = room
								}
								_ = publishFrame(nc, replRoomSubject(room), frame)
							}
						}
						publishHostFrame(BusFrame{
							Type:       frameTypeLine,
							Scope:      "index",
							Kind:       "lifecycle",
							SubtonePID: ev.PID,
							Message:    fmt.Sprintf("Subtone started as pid %d.", ev.PID),
						})
						publishHostFrame(BusFrame{
							Type:       frameTypeLine,
							Scope:      "index",
							Kind:       "lifecycle",
							SubtonePID: ev.PID,
							Message:    fmt.Sprintf("Subtone room: %s", subtoneRoomName(ev.PID)),
						})
						if strings.TrimSpace(ev.LogPath) != "" {
							publishHostFrame(BusFrame{
								Type:       frameTypeLine,
								Scope:      "index",
								Kind:       "lifecycle",
								SubtonePID: ev.PID,
								LogPath:    strings.TrimSpace(ev.LogPath),
								Message:    fmt.Sprintf("Subtone log file: %s", strings.TrimSpace(ev.LogPath)),
							})
						}
						publishHostFrame(BusFrame{
							Type:       frameTypeLine,
							Scope:      "subtone",
							Kind:       "lifecycle",
							SubtonePID: ev.PID,
							Room:       subtoneRoomName(ev.PID),
							Message:    fmt.Sprintf("Started at %s", ev.StartedAt.Format(time.RFC3339)),
						})
						publishHostFrame(BusFrame{
							Type:       frameTypeLine,
							Scope:      "subtone",
							Kind:       "lifecycle",
							SubtonePID: ev.PID,
							Room:       subtoneRoomName(ev.PID),
							Message:    fmt.Sprintf("Command: %s", cmdText),
						})
					case proc.SubtoneEventStdout, proc.SubtoneEventStderr:
						line := strings.TrimSpace(ev.Line)
						if ev.PID <= 0 || line == "" {
							return
						}
						kind := "log"
						if ev.Type == proc.SubtoneEventStderr {
							kind = "error"
						}
						_ = publishFrame(nc, replSubtoneSubject(ev.PID), BusFrame{
							Type:       frameTypeLine,
							Scope:      "subtone",
							Kind:       kind,
							Room:       subtoneRoomName(ev.PID),
							SubtonePID: ev.PID,
							Message:    line,
						})
					}
				})
				_ = publishFrame(nc, replRoomSubject(room), BusFrame{
					Type:     frameTypeLine,
					Scope:    "index",
					Kind:     "lifecycle",
					Room:     room,
					ExitCode: exitCode,
					Message:  fmt.Sprintf("Subtone on %s exited with code %d.", host, exitCode),
				})
				_ = nc.FlushTimeout(1200 * time.Millisecond)
			}(targetRoom, prompt, command)
		}
	}

	onAttachedFrame := func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		console.PrintFrame(frame)
	}

	switchAttached = func(pid int) error {
		subMu.Lock()
		defer subMu.Unlock()
		if attachedSub != nil {
			_ = attachedSub.Unsubscribe()
			attachedSub = nil
		}
		attachedPID = 0
		if pid <= 0 {
			return nil
		}
		subj := replSubtoneSubject(pid)
		if subj == "" {
			return fmt.Errorf("invalid subtone pid %d", pid)
		}
		nextSub, err := nc.Subscribe(subj, onAttachedFrame)
		if err != nil {
			return err
		}
		attachedSub = nextSub
		attachedPID = pid
		return nc.Flush()
	}

	switchRoom = func(targetRoom string, announce bool) error {
		targetRoom = sanitizeRoom(targetRoom)
		if targetRoom == "" {
			targetRoom = defaultRoom
		}
		subMu.Lock()
		if targetRoom == currentRoom {
			subMu.Unlock()
			return nil
		}
		targetSubj := replRoomSubject(targetRoom)
		nextSub, err := nc.Subscribe(targetSubj, onRoomFrame)
		if err != nil {
			subMu.Unlock()
			return err
		}
		prevSub := sub
		sub = nextSub
		currentRoom = targetRoom
		currentSubj = targetSubj
		subMu.Unlock()

		if prevSub != nil {
			_ = prevSub.Unsubscribe()
		}
		_ = publishFrame(nc, targetSubj, BusFrame{Type: frameTypeJoin, From: prompt, Room: targetRoom, Version: BuildVersion, OS: runtime.GOOS, Arch: runtime.GOARCH})
		_ = nc.Flush()
		if announce {
			console.PrintFrame(BusFrame{
				Type:    frameTypeLine,
				Scope:   "index",
				Kind:    "status",
				Message: fmt.Sprintf("Connected to %s via %s", targetSubj, natsAddr),
			})
		}
		return nil
	}

	initialSub, err := nc.Subscribe(currentSubj, onRoomFrame)
	if err != nil {
		return err
	}
	sub = initialSub
	defer func() {
		subMu.Lock()
		defer subMu.Unlock()
		if attachedSub != nil {
			_ = attachedSub.Unsubscribe()
		}
		if sub != nil {
			_ = sub.Unsubscribe()
		}
	}()
	if err := nc.Flush(); err != nil {
		return err
	}

	_ = publishFrame(nc, commandSubject, BusFrame{Type: frameTypeProbe, From: prompt, Room: currentRoom, Message: "probe"})
	_ = publishFrame(nc, currentSubj, BusFrame{Type: frameTypeJoin, From: prompt, Room: currentRoom, Version: BuildVersion, OS: runtime.GOOS, Arch: runtime.GOARCH})
	_ = nc.Flush()
	console.PrintFrame(BusFrame{
		Type:    frameTypeLine,
		Scope:   "index",
		Kind:    "status",
		Message: fmt.Sprintf("Connected to %s via %s", currentSubj, natsAddr),
	})

	scanner := bufio.NewScanner(os.Stdin)
	for {
		console.Prompt()
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		if pid, ok, parseErr := parseAttachCommand(line); ok {
			if parseErr != nil {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "error",
					Message: parseErr.Error(),
				})
				continue
			}
			if err := switchAttached(pid); err != nil {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "error",
					Message: fmt.Sprintf("Failed to attach to subtone-%d: %v", pid, err),
				})
				continue
			}
			console.PrintFrame(BusFrame{
				Type:    frameTypeLine,
				Scope:   "index",
				Kind:    "status",
				Message: fmt.Sprintf("Attached to subtone-%d.", pid),
			})
			continue
		}
		if isDetachCommand(line) {
			subMu.Lock()
			currentAttachedPID := attachedPID
			subMu.Unlock()
			if err := switchAttached(0); err != nil {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "error",
					Message: fmt.Sprintf("Failed to detach from subtone-%d: %v", currentAttachedPID, err),
				})
				continue
			}
			if currentAttachedPID > 0 {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "status",
					Message: fmt.Sprintf("Detached from subtone-%d.", currentAttachedPID),
				})
			} else {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "status",
					Message: "No subtone attachment is active.",
				})
			}
			continue
		}

		subMu.Lock()
		roomNow := currentRoom
		subjectNow := currentSubj
		subMu.Unlock()
		if targetHost, targetCommand, ok := parseTargetCommand(line); ok {
			if err := publishFrame(nc, commandSubject, BusFrame{
				Type:    frameTypeCommand,
				From:    prompt,
				Room:    roomNow,
				Version: BuildVersion,
				OS:      runtime.GOOS,
				Arch:    runtime.GOARCH,
				Message: fmt.Sprintf("@%s %s", targetHost, targetCommand),
			}); err != nil {
				return err
			}
			if err := nc.Flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "/") {
			if err := publishFrame(nc, commandSubject, BusFrame{
				Type:    frameTypeCommand,
				From:    prompt,
				Room:    roomNow,
				Version: BuildVersion,
				OS:      runtime.GOOS,
				Arch:    runtime.GOARCH,
				Message: line,
			}); err != nil {
				return err
			}
		} else {
			if err := publishFrame(nc, subjectNow, BusFrame{
				Type:    frameTypeChat,
				From:    prompt,
				Room:    roomNow,
				Message: line,
			}); err != nil {
				return err
			}
		}
		if err := nc.Flush(); err != nil {
			return err
		}
	}
	subMu.Lock()
	roomNow := currentRoom
	subjectNow := currentSubj
	subMu.Unlock()
	_ = publishFrame(nc, subjectNow, BusFrame{Type: frameTypeLeft, From: prompt, Room: roomNow})
	_ = nc.Flush()
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func RunStatus(args []string) error {
	fs := flag.NewFlagSet("repl-status", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared REPL room")
	if err := fs.Parse(args); err != nil {
		return err
	}

	roomName := sanitizeRoom(*room)
	subject := replRoomSubject(roomName)
	st := HostStatus{
		HostName: normalizePromptName(DefaultPromptName()),
		NATSURL:  strings.TrimSpace(*natsURL),
		Room:     roomName,
		Subject:  subject,
	}

	nc, err := nats.Connect(st.NATSURL, nats.Timeout(1200*time.Millisecond))
	if err == nil {
		st.NATSReachable = true
		serverSeen := make(chan bool, 1)
		sub, subErr := nc.Subscribe(subject, func(msg *nats.Msg) {
			frame, ok := decodeFrame(msg.Data)
			if !ok {
				return
			}
			if frame.Type == frameTypeServer || frame.Type == frameTypeHeartbeat {
				select {
				case serverSeen <- true:
				default:
				}
			}
		})
		if subErr == nil {
			_ = nc.Flush()
			_ = publishFrame(nc, commandSubject, BusFrame{Type: frameTypeProbe, From: st.HostName, Room: roomName, Message: "status-probe"})
			select {
			case <-serverSeen:
				st.ServerSeen = true
			case <-time.After(1400 * time.Millisecond):
			}
			_ = sub.Unsubscribe()
		}
		nc.Close()
	}

	if cfg, err := tsnetlib.ResolveConfig(st.HostName, ""); err == nil {
		st.TSNetTailnet = cfg.Tailnet
		st.TSAuthKey = cfg.AuthKeyPresent
		st.TSApiKey = cfg.APIKeyPresent
	}
	st.ChromePath = findChromePath()
	st.ChromeFound = st.ChromePath != ""

	logs.Raw("REPL status")
	logs.Raw("  Hostname: %s", st.HostName)
	logs.Raw("  NATS URL: %s", st.NATSURL)
	logs.Raw("  Room: %s", st.Room)
	logs.Raw("  Subject: %s", st.Subject)
	logs.Raw("  NATS Reachable: %t", st.NATSReachable)
	logs.Raw("  DIALTONE Server Seen: %t", st.ServerSeen)
	logs.Raw("  TS Tailnet: %s", st.TSNetTailnet)
	logs.Raw("  TS AuthKey Present: %t", st.TSAuthKey)
	logs.Raw("  TS API Key Present: %t", st.TSApiKey)
	if st.ChromeFound {
		logs.Raw("  Chrome: %s", st.ChromePath)
	} else {
		logs.Raw("  Chrome: not found")
	}
	return nil
}

func DefaultPromptName() string {
	host, err := os.Hostname()
	if err != nil {
		return "USER-1"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "USER-1"
	}
	return host
}

func findChromePath() string {
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	case "windows":
		programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
		programFilesX86 := strings.TrimSpace(os.Getenv("ProgramFiles(x86)"))
		localAppData := strings.TrimSpace(os.Getenv("LocalAppData"))
		candidates = append(candidates,
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"),
		)
	default:
		candidates = append(candidates,
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome"} {
		if path, err := exec.LookPath(name); err == nil {
			return strings.TrimSpace(path)
		}
	}
	return ""
}

func runLocalSession(in io.Reader, out io.Writer, promptName string, logFn func(category, msg string)) error {
	if logFn == nil {
		logFn = func(string, string) {}
	}

	say := func(frame BusFrame) {
		var buf strings.Builder
		printFrame(&buf, frame)
		line := strings.TrimRight(buf.String(), "\n")
		if line == "" {
			return
		}
		fmt.Fprintln(out, line)
		logs.Info("[REPL] %s", line)
		logFn("REPL", line)
	}

	say(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Virtual Librarian online."})
	say(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Type 'help' for commands, or 'exit' to quit."})

	scanner := bufio.NewScanner(in)
	tty := isInputTTY(in)
	for {
		fmt.Fprintf(out, "%s> ", promptName)
		if !scanner.Scan() {
			say(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Session closed."})
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !tty {
			fmt.Fprintln(out, line)
		}
		logFn("REPL", fmt.Sprintf("%s> %s", promptName, line))
		if line == "exit" || line == "quit" {
			say(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Goodbye."})
			break
		}
		executeCommand(line, defaultRoom, nil, say)
	}
	return scanner.Err()
}

func isInputTTY(in io.Reader) bool {
	stdin, ok := in.(*os.File)
	if !ok {
		return false
	}
	fi, err := stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func executeCommand(line string, room string, registry *subtoneRegistry, emit func(BusFrame)) {
	if emit == nil {
		return
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if line == "help" {
		printHelp(emit)
		return
	}
	if line == "ps" {
		printManagedProcesses(room, registry, emit)
		return
	}
	if strings.HasPrefix(line, "kill ") {
		pidText := strings.TrimSpace(strings.TrimPrefix(line, "kill"))
		pid := 0
		if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid <= 0 {
			emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Usage: kill <pid>"})
			return
		}
		if err := killManagedProcessFn(pid); err != nil {
			emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: fmt.Sprintf("Failed to kill process %d: %v", pid, err)})
		} else {
			emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: fmt.Sprintf("Killed managed process %d.", pid)})
		}
		return
	}

	args := strings.Fields(line)
	if len(args) == 0 {
		return
	}
	cmdName := args[0]
	if len(args) > 1 {
		cmdName += " " + args[1]
	}

	isBackground := false
	if args[len(args)-1] == "&" {
		isBackground = true
		args = args[:len(args)-1]
		cmdName = strings.TrimSuffix(cmdName, " &")
	}

	emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "lifecycle", Message: fmt.Sprintf("Request received. Spawning subtone for %s...", cmdName)})
	heartbeatInterval := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("DIALTONE_SUBTONE_HEARTBEAT_SEC")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			heartbeatInterval = time.Duration(sec) * time.Second
		}
	}
	stopHeartbeat := make(chan struct{})
	startHeartbeat := func(pid int, startedAt time.Time) {
		if pid <= 0 || heartbeatInterval <= 0 {
			return
		}
		go func() {
			t := time.NewTicker(heartbeatInterval)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					uptime := time.Since(startedAt).Round(time.Second)
					emit(BusFrame{
						Type:       frameTypeLine,
						Scope:      "subtone",
						Kind:       "lifecycle",
						Room:       subtoneRoomName(pid),
						SubtonePID: pid,
						Message:    fmt.Sprintf("[HEARTBEAT] running for %s", uptime),
					})
				case <-stopHeartbeat:
					return
				}
			}
		}()
	}
	stopHeartbeatOnce := sync.OnceFunc(func() {
		close(stopHeartbeat)
	})
	onEvent := func(ev proc.SubtoneEvent) {
		switch ev.Type {
		case proc.SubtoneEventStarted:
			if ev.PID <= 0 {
				return
			}
			if registry != nil {
				registry.Started(room, ev)
			}
			subtoneRoom := subtoneRoomName(ev.PID)
			emit(BusFrame{
				Type:       frameTypeLine,
				Scope:      "index",
				Kind:       "lifecycle",
				SubtonePID: ev.PID,
				Message:    fmt.Sprintf("Subtone started as pid %d.", ev.PID),
			})
			emit(BusFrame{
				Type:       frameTypeLine,
				Scope:      "index",
				Kind:       "lifecycle",
				SubtonePID: ev.PID,
				Message:    fmt.Sprintf("Subtone room: %s", subtoneRoom),
			})
			if strings.TrimSpace(ev.LogPath) != "" {
				emit(BusFrame{
					Type:       frameTypeLine,
					Scope:      "index",
					Kind:       "lifecycle",
					SubtonePID: ev.PID,
					LogPath:    strings.TrimSpace(ev.LogPath),
					Message:    fmt.Sprintf("Subtone log file: %s", ev.LogPath),
				})
			}
			if isBackground {
				emit(BusFrame{
					Type:       frameTypeLine,
					Scope:      "index",
					Kind:       "lifecycle",
					SubtonePID: ev.PID,
					Message:    fmt.Sprintf("Subtone for %s is running in background.", cmdName),
				})
			}
			emit(BusFrame{Type: frameTypeLine, Scope: "subtone", Kind: "lifecycle", Room: subtoneRoom, SubtonePID: ev.PID, Message: fmt.Sprintf("Started at %s", ev.StartedAt.Format(time.RFC3339))})
			emit(BusFrame{Type: frameTypeLine, Scope: "subtone", Kind: "lifecycle", Room: subtoneRoom, SubtonePID: ev.PID, Message: fmt.Sprintf("Command: %v", ev.Args)})
			startHeartbeat(ev.PID, ev.StartedAt)
		case proc.SubtoneEventStdout:
			if ev.PID <= 0 {
				return
			}
			line := strings.TrimSpace(ev.Line)
			if line == "" {
				return
			}
			emit(BusFrame{Type: frameTypeLine, Scope: "subtone", Kind: "log", Room: subtoneRoomName(ev.PID), SubtonePID: ev.PID, Message: line})
		case proc.SubtoneEventStderr:
			if ev.PID <= 0 {
				return
			}
			line := strings.TrimSpace(ev.Line)
			if line == "" {
				return
			}
			emit(BusFrame{Type: frameTypeLine, Scope: "subtone", Kind: "error", Room: subtoneRoomName(ev.PID), SubtonePID: ev.PID, Message: line})
		case proc.SubtoneEventExited:
			stopHeartbeatOnce()
			if ev.PID > 0 {
				if registry != nil {
					registry.Exited(ev.PID, ev.ExitCode)
				}
				emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "lifecycle", SubtonePID: ev.PID, ExitCode: ev.ExitCode, Message: fmt.Sprintf("Subtone for %s exited with code %d.", cmdName, ev.ExitCode)})
				return
			}
			if line := strings.TrimSpace(ev.Line); line != "" {
				emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "error", Message: fmt.Sprintf("Subtone for %s failed to start: %s", cmdName, line)})
			} else {
				emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "error", Message: fmt.Sprintf("Subtone for %s failed to start.", cmdName)})
			}
		}
	}

	if isBackground {
		go runSubtoneWithEventsFn(args, onEvent)
		return
	}
	runSubtoneWithEventsFn(args, onEvent)
}

func printHelp(emit func(BusFrame)) {
	content := []string{
		"Help",
		"",
		"Bootstrap",
		"`dev install`",
		"Install latest Go and bootstrap dev.go command scaffold",
		"",
		"Plugins",
		"`robot src_v1 install`",
		"Install robot src_v1 dependencies",
		"",
		"`dag src_v3 install`",
		"Install dag src_v3 dependencies",
		"",
		"`logs src_v1 test`",
		"Run logs plugin tests on a subtone",
		"",
		"System",
		"`ps`",
		"List active subtones",
		"",
		"`/subtone-attach --pid <pid>`",
		"Attach this console to a subtone room",
		"",
		"`/subtone-detach`",
		"Stop streaming attached subtone output",
		"",
		"`kill <pid>`",
		"Kill a managed subtone process by PID",
		"",
		"`<any command>`",
		"Run any dialtone command on a managed subtone",
	}
	for _, line := range content {
		emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: line})
	}
}

func printManagedProcesses(room string, registry *subtoneRegistry, emit func(BusFrame)) {
	items := []subtoneRegistryItem(nil)
	if registry != nil {
		for _, item := range registry.Snapshot(0, listManagedFn()) {
			if item.Active {
				items = append(items, item)
			}
		}
	}
	if len(items) == 0 {
		procs := listManagedFn()
		if len(procs) == 0 {
			emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "No active subtones."})
			return
		}
		emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Active Subtones:"})
		emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: fmt.Sprintf("%-8s %-8s %-10s %-8s %s", "PID", "UPTIME", "CPU%", "PORTS", "COMMAND")})
		for _, p := range procs {
			emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: fmt.Sprintf("%-8d %-8s %-10.1f %-8d %s", p.PID, p.StartedAgo, p.CPUPercent, p.PortCount, p.Command)})
		}
		return
	}
	emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Room: sanitizeRoom(room), Message: "Active Subtones:"})
	emit(BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Room: sanitizeRoom(room), Message: fmt.Sprintf("%-8s %-8s %-10s %-8s %s", "PID", "UPTIME", "CPU%", "PORTS", "COMMAND")})
	for _, item := range items {
		uptime := strings.TrimSpace(item.StartedAgo)
		if uptime == "" {
			uptime = "-"
		}
		command := strings.TrimSpace(item.Command)
		if command == "" {
			command = "-"
		}
		emit(BusFrame{
			Type:    frameTypeLine,
			Scope:   "index",
			Kind:    "status",
			Room:    sanitizeRoom(room),
			LogPath: strings.TrimSpace(item.LogPath),
			Message: fmt.Sprintf("%-8d %-8s %-10.1f %-8d %s", item.PID, uptime, item.CPUPercent, item.PortCount, command),
		})
	}
}

func subtoneRoomName(pid int) string {
	if pid <= 0 {
		return ""
	}
	return fmt.Sprintf("subtone-%d", pid)
}

func publishPresenceReport(
	room string,
	mode string,
	rows []presenceRow,
	publishRoom func(targetRoom string, f BusFrame),
) {
	if len(rows) == 0 {
		publishRoom(room, BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "No connected users."})
		return
	}
	switch mode {
	case "versions":
		publishRoom(room, BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Connected versions:"})
		for _, row := range rows {
			if row.Kind == "daemon" {
				daemonVer := strings.TrimSpace(row.DaemonVersion)
				replVer := strings.TrimSpace(row.ReplVersion)
				if daemonVer == "" {
					daemonVer = "unknown"
				}
				if replVer == "" {
					replVer = "unknown"
				}
				publishRoom(room, BusFrame{
					Type:  frameTypeLine,
					Scope: "index",
					Kind:  "status",
					Message: fmt.Sprintf(
						"- [daemon] %s daemon=%s repl=%s room=%s os=%s arch=%s",
						row.Name,
						daemonVer,
						replVer,
						sanitizeRoom(row.Room),
						fallbackUnknown(row.OS),
						fallbackUnknown(row.Arch),
					),
				})
				continue
			}
			version := strings.TrimSpace(row.Version)
			if version == "" {
				version = "unknown"
			}
			publishRoom(room, BusFrame{
				Type:  frameTypeLine,
				Scope: "index",
				Kind:  "status",
				Message: fmt.Sprintf(
					"- [client] %s repl=%s room=%s os=%s arch=%s",
					row.Name,
					version,
					sanitizeRoom(row.Room),
					fallbackUnknown(row.OS),
					fallbackUnknown(row.Arch),
				),
			})
		}
	default:
		publishRoom(room, BusFrame{Type: frameTypeLine, Scope: "index", Kind: "status", Message: "Connected sessions:"})
		for _, row := range rows {
			if row.Kind == "daemon" {
				daemonVer := strings.TrimSpace(row.DaemonVersion)
				replVer := strings.TrimSpace(row.ReplVersion)
				if daemonVer == "" {
					daemonVer = "unknown"
				}
				if replVer == "" {
					replVer = "unknown"
				}
				publishRoom(room, BusFrame{
					Type:  frameTypeLine,
					Scope: "index",
					Kind:  "status",
					Message: fmt.Sprintf(
						"- [daemon] %s room=%s daemon=%s repl=%s os=%s arch=%s",
						row.Name,
						sanitizeRoom(row.Room),
						daemonVer,
						replVer,
						fallbackUnknown(row.OS),
						fallbackUnknown(row.Arch),
					),
				})
				continue
			}
			version := strings.TrimSpace(row.Version)
			if version == "" {
				version = "unknown"
			}
			publishRoom(room, BusFrame{
				Type:  frameTypeLine,
				Scope: "index",
				Kind:  "status",
				Message: fmt.Sprintf(
					"- [client] %s room=%s repl=%s os=%s arch=%s",
					row.Name,
					sanitizeRoom(row.Room),
					version,
					fallbackUnknown(row.OS),
					fallbackUnknown(row.Arch),
				),
			})
		}
	}
}

func fallbackUnknown(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "unknown"
	}
	return v
}

func hasStructuredLevel(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"[INFO]", "[WARN]", "[ERROR]", "[COST]", "[T+"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func normalizePromptName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "-")
	if s == "" {
		return "USER"
	}
	return s
}

func sanitizeRoom(room string) string {
	room = strings.TrimSpace(room)
	if room == "" {
		return defaultRoom
	}
	room = strings.ReplaceAll(room, " ", "-")
	return room
}

func replRoomSubject(room string) string {
	return "repl.room." + sanitizeRoom(room)
}

func replSubtoneSubject(pid int) string {
	if pid <= 0 {
		return ""
	}
	return fmt.Sprintf("repl.subtone.%d", pid)
}

func parseTargetCommand(line string) (targetHost, command string, ok bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "@") {
		return "", "", false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	target := strings.TrimSpace(strings.TrimPrefix(fields[0], "@"))
	target = normalizePromptName(target)
	if target == "" {
		return "", "", false
	}
	command = strings.TrimSpace(strings.TrimPrefix(strings.Join(fields[1:], " "), "/"))
	if command == "" {
		return "", "", false
	}
	return target, command, true
}

func parseAttachCommand(line string) (pid int, ok bool, err error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/subtone-attach") {
		return 0, false, nil
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, true, fmt.Errorf("usage: /subtone-attach --pid <pid>")
	}
	for i := 1; i < len(fields); i++ {
		switch strings.TrimSpace(fields[i]) {
		case "--pid":
			if i+1 >= len(fields) {
				return 0, true, fmt.Errorf("usage: /subtone-attach --pid <pid>")
			}
			parsed, convErr := strconv.Atoi(strings.TrimSpace(fields[i+1]))
			if convErr != nil || parsed <= 0 {
				return 0, true, fmt.Errorf("invalid subtone pid %q", strings.TrimSpace(fields[i+1]))
			}
			return parsed, true, nil
		}
	}
	if len(fields) == 2 {
		parsed, convErr := strconv.Atoi(strings.TrimSpace(fields[1]))
		if convErr == nil && parsed > 0 {
			return parsed, true, nil
		}
	}
	return 0, true, fmt.Errorf("usage: /subtone-attach --pid <pid>")
}

func isDetachCommand(line string) bool {
	line = strings.TrimSpace(line)
	return line == "/subtone-detach" || line == "subtone-detach"
}

func normalizeTargetHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return normalizePromptName(raw)
}

func connectNATS(natsURL string, embedded bool) (*nats.Conn, *logs.EmbeddedNATS, string, error) {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		natsURL = defaultNATSURL
	}
	if embedded {
		broker, err := logs.StartEmbeddedNATSOnURL(natsURL)
		if err != nil {
			return nil, nil, "", err
		}
		nc, err := nats.Connect(broker.URL(), nats.Timeout(1500*time.Millisecond))
		if err != nil {
			broker.Close()
			return nil, nil, "", err
		}
		return nc, broker, broker.URL(), nil
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return nil, nil, "", err
	}
	return nc, nil, natsURL, nil
}

func publishFrame(nc *nats.Conn, subject string, frame BusFrame) error {
	if nc == nil {
		return fmt.Errorf("nil nats connection")
	}
	frame.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return nc.Publish(subject, payload)
}

func decodeFrame(data []byte) (BusFrame, bool) {
	var f BusFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return BusFrame{}, false
	}
	f.Type = strings.TrimSpace(f.Type)
	if f.Type == "" {
		return BusFrame{}, false
	}
	return f, true
}

func printFrame(w io.Writer, frame BusFrame) {
	switch frame.Type {
	case frameTypeInput:
		name := normalizePromptName(frame.From)
		if name == "" {
			name = "USER"
		}
		fmt.Fprintf(w, "%s> %s\n", name, strings.TrimSpace(frame.Message))
	case frameTypeChat:
		name := normalizePromptName(frame.From)
		if name == "" {
			name = "USER"
		}
		fmt.Fprintf(w, "DIALTONE> %s: %s\n", name, strings.TrimSpace(frame.Message))
	case frameTypeLine:
		prefix := strings.TrimSpace(frame.Prefix)
		if prefix == "" && frame.Scope == "subtone" && frame.SubtonePID > 0 {
			prefix = fmt.Sprintf("DIALTONE:%d", frame.SubtonePID)
		}
		if prefix == "" {
			prefix = "DIALTONE"
		}
		text := strings.TrimSpace(frame.Message)
		if frame.Kind == "error" && text != "" && !strings.HasPrefix(text, "[ERROR]") {
			text = "[ERROR] " + text
		}
		fmt.Fprintf(w, "%s> %s\n", prefix, text)
	case frameTypeServer:
		fmt.Fprintf(w, "DIALTONE> %s\n", strings.TrimSpace(frame.Message))
	case frameTypeJoin:
		name := normalizePromptName(frame.From)
		if name == "" {
			name = normalizePromptName(frame.Message)
		}
		if name == "" {
			name = "unknown"
		}
		if strings.TrimSpace(frame.Room) == "" {
			fmt.Fprintf(w, "DIALTONE> [JOIN] %s\n", name)
		} else {
			if strings.TrimSpace(frame.Version) == "" {
				fmt.Fprintf(w, "DIALTONE> [JOIN] %s (room=%s)\n", name, sanitizeRoom(frame.Room))
			} else {
				fmt.Fprintf(w, "DIALTONE> [JOIN] %s (room=%s version=%s)\n", name, sanitizeRoom(frame.Room), strings.TrimSpace(frame.Version))
			}
		}
	case frameTypeLeft:
		name := normalizePromptName(frame.From)
		if name == "" {
			name = normalizePromptName(frame.Message)
		}
		if name == "" {
			name = "unknown"
		}
		if strings.TrimSpace(frame.Room) == "" {
			fmt.Fprintf(w, "DIALTONE> [LEFT] %s\n", name)
		} else {
			fmt.Fprintf(w, "DIALTONE> [LEFT] %s (room=%s)\n", name, sanitizeRoom(frame.Room))
		}
	case frameTypeControl:
		text := strings.TrimSpace(frame.Message)
		if text == "" {
			text = fmt.Sprintf("%s %s", strings.TrimSpace(frame.Command), strings.TrimSpace(frame.Room))
		}
		fmt.Fprintf(w, "DIALTONE> [CONTROL] %s\n", strings.TrimSpace(text))
	case frameTypeError:
		fmt.Fprintf(w, "DIALTONE> [ERROR] %s\n", strings.TrimSpace(frame.Message))
	}
}

type tsnetRuntime struct {
	DNSName string
	Listen  func(network, addr string) (net.Listener, error)
	Close   func() error
}

func startTSNetInstance(hostname string) (*tsnetRuntime, error) {
	cfg, err := tsnetlib.ResolveConfig(hostname, "")
	if err != nil {
		return nil, err
	}
	if err := tsnetlib.EnsureAuthKeyForEmbedded(&cfg); err != nil {
		return nil, err
	}
	srv := tsnetlib.BuildServer(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	status, err := srv.Up(ctx)
	if err != nil {
		_ = srv.Close()
		return nil, err
	}
	dnsName := strings.TrimSpace(status.Self.DNSName)
	dnsName = strings.TrimSuffix(dnsName, ".")
	if dnsName == "" {
		dnsName = strings.TrimSpace(status.Self.HostName)
	}
	logs.Info("REPL tsnet identity online: hostname=%s tailnet=%s dns=%s ips=%v", status.Self.HostName, cfg.Tailnet, dnsName, status.TailscaleIPs)
	return &tsnetRuntime{
		DNSName: dnsName,
		Listen:  srv.Listen,
		Close:   srv.Close,
	}, nil
}

func natsProxyTarget(natsURL string) (string, int, error) {
	raw := strings.TrimSpace(natsURL)
	if raw == "" {
		raw = defaultNATSURL
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, err
	}
	host := strings.TrimSpace(u.Hostname())
	portText := strings.TrimSpace(u.Port())
	port := 4222
	if portText != "" {
		p, pErr := strconv.Atoi(portText)
		if pErr != nil {
			return "", 0, pErr
		}
		port = p
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), port, nil
}

func serveTCPProxy(ln net.Listener, targetAddr string) {
	for {
		srcConn, err := ln.Accept()
		if err != nil {
			return
		}
		go proxyConn(srcConn, targetAddr)
	}
}

func proxyConn(src net.Conn, targetAddr string) {
	defer src.Close()
	dst, err := net.DialTimeout("tcp", targetAddr, 4*time.Second)
	if err != nil {
		return
	}
	defer dst.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(src, dst)
		done <- struct{}{}
	}()
	<-done
}

func normalizeTSNetHostname(host string) string {
	host = normalizePromptName(host)
	if host == "" {
		host = "dialtone-node"
	}
	if runningInWSL() && !strings.Contains(host, "wsl") {
		host += "-wsl"
	}
	return host
}

func runningInWSL() bool {
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}
