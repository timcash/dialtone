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
	indexStatusTag = "DIALTONE_INDEX:"
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
const controlRunHostTask = "run_host_task"

type Hooks struct {
	RunTaskWorkerWithEvents func(args []string, onEvent proc.TaskWorkerEventHandler) int
	ListManaged             func() []proc.ManagedProcessSnapshot
	KillManagedProcess      func(pid int) error
}

var (
	runTaskWorkerWithEventsFn = proc.RunTaskWorkerWithEvents
	listManagedFn             = proc.ListManagedProcesses
	killManagedProcessFn      = proc.KillManagedProcess
	taskIDMu                  sync.Mutex
	taskIDLastStamp           string
	taskIDSeq                 int
)

// SetHooksForTest overrides REPL side-effect functions and returns a restore function.
func SetHooksForTest(h Hooks) func() {
	prevRunTaskWorkerWithEvents := runTaskWorkerWithEventsFn
	prevListManaged := listManagedFn
	prevKillManaged := killManagedProcessFn

	if h.RunTaskWorkerWithEvents != nil {
		runTaskWorkerWithEventsFn = h.RunTaskWorkerWithEvents
	}
	if h.ListManaged != nil {
		listManagedFn = h.ListManaged
	}
	if h.KillManagedProcess != nil {
		killManagedProcessFn = h.KillManagedProcess
	}
	return func() {
		runTaskWorkerWithEventsFn = prevRunTaskWorkerWithEvents
		listManagedFn = prevListManaged
		killManagedProcessFn = prevKillManaged
	}
}

type BusFrame struct {
	Type      string   `json:"type"`
	Scope     string   `json:"scope,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	From      string   `json:"from,omitempty"`
	Target    string   `json:"target,omitempty"`
	Room      string   `json:"room,omitempty"`
	Version   string   `json:"version,omitempty"`
	OS        string   `json:"os,omitempty"`
	Arch      string   `json:"arch,omitempty"`
	ReplVer   string   `json:"repl_version,omitempty"`
	DaemonVer string   `json:"daemon_version,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Prefix    string   `json:"prefix,omitempty"`
	Message   string   `json:"message,omitempty"`
	TaskID    string   `json:"task_id,omitempty"`
	PID       int      `json:"pid,omitempty"`
	LogPath   string   `json:"log_path,omitempty"`
	ExitCode  int      `json:"exit_code,omitempty"`
	Ready     bool     `json:"ready,omitempty"`
	ServerID  string   `json:"server_id,omitempty"`
	Timestamp string   `json:"timestamp"`
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
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL")
	topic := topicFlag(fs, "Primary REPL topic")
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
	clientNATSURL := leaderClientNATSURL(usedURL)
	_ = os.Setenv("DIALTONE_REPL_NATS_URL", clientNATSURL)

	stopTSNet := func() {}
	var tsRuntime *tsnetRuntime
	tsnetStatusMessage := ""
	tsnetPublicURL := ""

	h := normalizePromptName(*hostname)
	roomName := sanitizeRoom(*topic)
	serverID := h + "@" + roomName
	startedAt := time.Now()
	if err := writeLeaderStateHeartbeat(clientNATSURL, tsnetPublicURL, roomName, h, serverID, *embedded, startedAt); err != nil {
		logs.Warn("REPL leader state write failed: %v", err)
	}
	defer markLeaderStopped()

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
		case "task":
			if strings.TrimSpace(f.Room) == "" {
				f.Room = taskRoomName(f.TaskID)
			}
			if strings.TrimSpace(f.Room) == "" {
				f.Room = indexRoom
			}
			_ = publishFrame(nc, replRoomSubject(f.Room), f)
		case "task-worker":
			if f.PID <= 0 {
				if strings.TrimSpace(f.Room) == "" {
					f.Room = indexRoom
				}
				_ = publishFrame(nc, replRoomSubject(indexRoom), f)
				return
			}
			if strings.TrimSpace(f.Room) == "" {
				f.Room = taskWorkerRoomName(f.PID)
			}
			_ = publishFrame(nc, replTaskWorkerSubject(f.PID), f)
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
		}
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
	defer stopTSNet()

	// Publish initial presence line to NATS so every connected client sees it.
	publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("Leader online on %s (topic=%s nats=%s)", h, replTopicSubjectLabel(roomName), usedURL)})
	logs.Info("REPL host serving: hostname=%s topic=%s cmd_subject=%s nats=%s", h, roomName, commandSubject, usedURL)
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
				tsnetPublicURL = tsURL
				tsnetStatusMessage = fmt.Sprintf("tsnet NATS endpoint: %s", tsURL)
				logs.Info("REPL tsnet NATS endpoint active: %s -> %s", tsURL, targetAddr)
				publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: tsnetStatusMessage})
				if err := writeLeaderStateHeartbeat(clientNATSURL, tsnetPublicURL, roomName, h, serverID, *embedded, startedAt); err != nil {
					logs.Warn("REPL leader state write failed after tsnet activation: %v", err)
				}
			}
		}
	}
	defer func() {
		if tsnetListener != nil {
			_ = tsnetListener.Close()
		}
	}()

	presence := newPresenceTracker()
	tasks := newTaskRegistry(256)
	taskStore, err := newTaskKVStore(nc)
	if err != nil {
		return err
	}
	services := newServiceRegistry(128)
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
					Command: controlRunHostTask,
					Room:    currentRoom,
					Message: targetCommand,
				})
				publishRoom(currentRoom, BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "status",
					Room:    currentRoom,
					Message: fmt.Sprintf("Dispatching host task on %s.", targetHost),
				})
				return
			}
			args := shellSplit(raw)
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
					publishRoom(currentRoom, BusFrame{Type: frameTypeError, Message: "Usage: /repl src_v3 join <topic-name> | /repl src_v3 who | /repl src_v3 versions"})
					return
				}
				if targetRoom == currentRoom {
					publishDialtoneIndexLine(publishRoom, currentRoom, "status", fmt.Sprintf("%s is already on topic %s", sender, targetRoom))
					return
				}
				publishRoom(currentRoom, BusFrame{Type: frameTypeLeft, From: sender})
				publishRoom(currentRoom, BusFrame{
					Type:    frameTypeControl,
					Target:  sender,
					Command: controlJoinRoom,
					Room:    targetRoom,
					Message: fmt.Sprintf("switching %s to topic %s", sender, targetRoom),
				})
				presence.UpsertClient(sender, targetRoom, frame.Version, frame.OS, frame.Arch)
				return
			}

			go func(in BusFrame) {
				runMu.Lock()
				defer runMu.Unlock()
				executeCommand(strings.TrimSpace(in.Message), currentRoom, h, tasks, taskStore, services, func(subject string, payload []byte) error {
					return nc.Publish(subject, payload)
				}, func(frame BusFrame) {
					publishScopedFrame(currentRoom, frame)
				})
			}(BusFrame{Message: raw})
		}
	})
	if err != nil {
		return err
	}
	defer cmdSub.Unsubscribe()
	healthSub, err := nc.Subscribe(leaderHealthSubject, func(msg *nats.Msg) {
		st := buildLeaderState(clientNATSURL, tsnetPublicURL, roomName, h, serverID, *embedded, startedAt)
		raw, _ := json.Marshal(st)
		_ = msg.Respond(raw)
	})
	if err != nil {
		return err
	}
	defer healthSub.Unsubscribe()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := writeLeaderStateHeartbeat(clientNATSURL, tsnetPublicURL, roomName, h, serverID, *embedded, startedAt); err != nil {
				logs.Warn("REPL leader heartbeat write failed: %v", err)
			}
		}
	}()

	registrySub, err := nc.Subscribe(taskRegistrySubject, func(msg *nats.Msg) {
		if strings.TrimSpace(msg.Reply) == "" {
			return
		}
		req := taskRegistryRequest{}
		if len(msg.Data) > 0 {
			_ = json.Unmarshal(msg.Data, &req)
		}
		items := tasks.Snapshot(req.Count, listManagedFn())
		payload, err := encodeTaskRegistrySnapshot(items)
		if err != nil {
			return
		}
		_ = nc.Publish(msg.Reply, payload)
	})
	if err != nil {
		return err
	}
	defer registrySub.Unsubscribe()
	serviceRegistrySub, err := nc.Subscribe(serviceRegistrySubject, func(msg *nats.Msg) {
		if strings.TrimSpace(msg.Reply) == "" {
			return
		}
		req := serviceRegistryRequest{}
		if len(msg.Data) > 0 {
			_ = json.Unmarshal(msg.Data, &req)
		}
		items := services.Snapshot(req.Count, listManagedFn())
		payload, err := encodeServiceRegistrySnapshot(items)
		if err != nil {
			return
		}
		_ = nc.Publish(msg.Reply, payload)
	})
	if err != nil {
		return err
	}
	defer serviceRegistrySub.Unsubscribe()
	serviceHeartbeatSub, err := nc.Subscribe("repl.host.*.heartbeat.service.*", func(msg *nats.Msg) {
		var hb managedHeartbeat
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			return
		}
		services.ObserveHeartbeat(hb)
	})
	if err != nil {
		return err
	}
	defer serviceHeartbeatSub.Unsubscribe()

	roomSub, err := nc.Subscribe("repl.topic.*", func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		switch frame.Type {
		case frameTypeJoin:
			presence.UpsertClient(frame.From, frame.Room, frame.Version, frame.OS, frame.Arch)
			targetRoom := sanitizeRoom(frame.Room)
			if targetRoom == "" {
				targetRoom = roomName
			}
			publishRoom(targetRoom, BusFrame{
				Type:    frameTypeServer,
				Message: fmt.Sprintf("Leader online on %s (topic=%s nats=%s)", h, replTopicSubjectLabel(roomName), usedURL),
			})
			if strings.TrimSpace(tsnetStatusMessage) != "" {
				publishRoom(targetRoom, BusFrame{Type: frameTypeServer, Message: tsnetStatusMessage})
			}
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
	if taskStore != nil {
		if err := taskStore.ReconcileLocalRuntime(h, listManagedFn()); err != nil {
			logs.Warn("REPL task KV initial reconcile failed: %v", err)
		}
	}

	heartbeat := time.NewTicker(5 * time.Second)
	defer heartbeat.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-heartbeat.C:
			if taskStore != nil {
				if err := taskStore.ReconcileLocalRuntime(h, listManagedFn()); err != nil {
					logs.Warn("REPL task KV reconcile failed: %v", err)
				}
			}
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
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL")
	topic := topicFlag(fs, "Shared REPL topic")
	name := fs.String("name", DefaultPromptName(), "Prompt name for this client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: join [topic-name] [--nats-url URL] [--name HOST]")
	}
	if fs.NArg() == 1 {
		*topic = fs.Arg(0)
	}

	nc, err := nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	prompt := normalizePromptName(*name)
	roomName := sanitizeRoom(*topic)
	currentRoom := roomName
	currentSubj := replRoomSubject(currentRoom)
	natsAddr := strings.TrimSpace(*natsURL)

	var subMu sync.Mutex
	var sub *nats.Subscription
	var attachedSub *nats.Subscription
	attachedTaskID := ""
	var hostRunMu sync.Mutex
	var switchRoom func(string, bool) error
	var switchAttached func(string) error
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
		if frame.Type == frameTypeControl && frame.Target == prompt && frame.Command == controlRunHostTask {
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
				exitCode := proc.RunHostCommandWithEvents(cmdText, func(ev proc.TaskWorkerEvent) {
					switch ev.Type {
					case proc.TaskWorkerEventStarted:
						publishHostFrame := func(frame BusFrame) {
							switch strings.TrimSpace(frame.Scope) {
							case "task-worker":
								if frame.PID <= 0 {
									frame.PID = ev.PID
								}
								if strings.TrimSpace(frame.Room) == "" {
									frame.Room = taskWorkerRoomName(frame.PID)
								}
								_ = publishFrame(nc, replTaskWorkerSubject(frame.PID), frame)
							default:
								if strings.TrimSpace(frame.Room) == "" {
									frame.Room = room
								}
								_ = publishFrame(nc, replRoomSubject(room), frame)
							}
						}
						publishHostFrame(BusFrame{
							Type:    frameTypeLine,
							Scope:   "index",
							Kind:    "lifecycle",
							PID:     ev.PID,
							Message: fmt.Sprintf("Task worker started as pid %d.", ev.PID),
						})
						publishHostFrame(BusFrame{
							Type:    frameTypeLine,
							Scope:   "index",
							Kind:    "lifecycle",
							PID:     ev.PID,
							Message: fmt.Sprintf("Task topic: %s", taskWorkerRoomName(ev.PID)),
						})
						if strings.TrimSpace(ev.LogPath) != "" {
							publishHostFrame(BusFrame{
								Type:    frameTypeLine,
								Scope:   "index",
								Kind:    "lifecycle",
								PID:     ev.PID,
								LogPath: strings.TrimSpace(ev.LogPath),
								Message: fmt.Sprintf("Task log: %s", strings.TrimSpace(ev.LogPath)),
							})
						}
						publishHostFrame(BusFrame{
							Type:    frameTypeLine,
							Scope:   "task-worker",
							Kind:    "lifecycle",
							PID:     ev.PID,
							Room:    taskWorkerRoomName(ev.PID),
							Message: fmt.Sprintf("Started at %s", ev.StartedAt.Format(time.RFC3339)),
						})
						publishHostFrame(BusFrame{
							Type:    frameTypeLine,
							Scope:   "task-worker",
							Kind:    "lifecycle",
							PID:     ev.PID,
							Room:    taskWorkerRoomName(ev.PID),
							Message: fmt.Sprintf("Command: %s", cmdText),
						})
					case proc.TaskWorkerEventStdout, proc.TaskWorkerEventStderr:
						kind, line, ok := normalizeTaskWorkerLine(ev.Line, ev.Type == proc.TaskWorkerEventStderr)
						if ev.PID <= 0 || !ok {
							return
						}
						_ = publishFrame(nc, replTaskWorkerSubject(ev.PID), BusFrame{
							Type:    frameTypeLine,
							Scope:   "task-worker",
							Kind:    kind,
							Room:    taskWorkerRoomName(ev.PID),
							PID:     ev.PID,
							Message: line,
						})
					}
				})
				_ = publishFrame(nc, replRoomSubject(room), BusFrame{
					Type:     frameTypeLine,
					Scope:    "index",
					Kind:     "lifecycle",
					Room:     room,
					ExitCode: exitCode,
					Message:  fmt.Sprintf("Host task on %s exited with code %d.", host, exitCode),
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
		if strings.TrimSpace(frame.Prefix) == "" && strings.HasPrefix(strings.TrimSpace(frame.TaskID), "task-") {
			frame.Prefix = "DIALTONE:" + strings.TrimSpace(frame.TaskID)
		}
		console.PrintFrame(frame)
	}

	switchAttached = func(taskID string) error {
		subMu.Lock()
		defer subMu.Unlock()
		if attachedSub != nil {
			_ = attachedSub.Unsubscribe()
			attachedSub = nil
		}
		attachedTaskID = ""
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			return nil
		}
		subj := replRoomSubject(taskRoomName(taskID))
		if strings.TrimSpace(subj) == "" {
			return fmt.Errorf("invalid task id %s", taskID)
		}
		nextSub, err := nc.Subscribe(subj, onAttachedFrame)
		if err != nil {
			return err
		}
		attachedSub = nextSub
		attachedTaskID = taskID
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
				Message: fmt.Sprintf("Connected to %s via %s", replTopicSubjectLabel(targetRoom), natsAddr),
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
		Message: fmt.Sprintf("Connected to %s via %s", replTopicSubjectLabel(currentRoom), natsAddr),
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
		if taskID, ok, parseErr := parseTaskAttachCommand(line); ok {
			if parseErr != nil {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "error",
					Message: parseErr.Error(),
				})
				continue
			}
			if err := switchAttached(taskID); err != nil {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "error",
					Message: fmt.Sprintf("Failed to attach to task %s: %v", taskID, err),
				})
				continue
			}
			console.PrintFrame(BusFrame{
				Type:    frameTypeLine,
				Scope:   "index",
				Kind:    "status",
				Message: fmt.Sprintf("Attached to task %s.", taskID),
			})
			continue
		}
		if isDetachCommand(line) {
			subMu.Lock()
			currentAttachedTaskID := attachedTaskID
			subMu.Unlock()
			if err := switchAttached(""); err != nil {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "error",
					Message: fmt.Sprintf("Failed to detach from task %s: %v", currentAttachedTaskID, err),
				})
				continue
			}
			if strings.TrimSpace(currentAttachedTaskID) != "" {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "status",
					Message: fmt.Sprintf("Detached from task %s.", currentAttachedTaskID),
				})
			} else {
				console.PrintFrame(BusFrame{
					Type:    frameTypeLine,
					Scope:   "index",
					Kind:    "status",
					Message: "No task attachment is active.",
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
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL")
	topic := topicFlag(fs, "Shared REPL topic")
	if err := fs.Parse(args); err != nil {
		return err
	}

	roomName := sanitizeRoom(*topic)
	subject := replRoomSubject(roomName)
	st := HostStatus{
		HostName: normalizePromptName(DefaultPromptName()),
		NATSURL:  strings.TrimSpace(*natsURL),
		Room:     roomName,
		Subject:  subject,
	}

	leaderSt, healthErr := leaderHealth(st.NATSURL, 1500*time.Millisecond)
	if healthErr == nil {
		st.NATSReachable = true
		st.ServerSeen = true
	} else if endpointReachable(st.NATSURL, 700*time.Millisecond) {
		st.NATSReachable = true
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
	logs.Raw("  Topic: %s", st.Room)
	logs.Raw("  Subject: %s", st.Subject)
	logs.Raw("  NATS Reachable: %t", st.NATSReachable)
	logs.Raw("  DIALTONE Server Seen: %t", st.ServerSeen)
	if healthErr == nil {
		logs.Raw("  Leader PID: %d", leaderSt.PID)
		logs.Raw("  Leader Started: %s", leaderSt.StartedAt)
		logs.Raw("  Leader Healthy At: %s", leaderSt.LastHealthyAt)
		logs.Raw("  Leader Version: %s", leaderSt.Version)
	} else if saved, err := readLeaderState(); err == nil {
		logs.Raw("  Leader State File: %s", savedStatePathOrUnknown())
		logs.Raw("  Saved Leader PID: %d", saved.PID)
		logs.Raw("  Saved Leader Running: %t", saved.Running)
		logs.Raw("  Saved Leader Healthy At: %s", saved.LastHealthyAt)
		logs.Raw("  Leader Health Error: %v", healthErr)
	} else if healthErr != nil {
		logs.Raw("  Leader Health Error: %v", healthErr)
	}
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

func savedStatePathOrUnknown() string {
	path, err := leaderStatePath()
	if err != nil {
		return "<unknown>"
	}
	return path
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

	emitDialtoneIndexLine(say, "status", "Virtual Librarian online.")
	emitDialtoneIndexLine(say, "status", "Type 'help' for commands, or 'exit' to quit.")

	scanner := bufio.NewScanner(in)
	tty := isInputTTY(in)
	for {
		fmt.Fprintf(out, "%s> ", promptName)
		if !scanner.Scan() {
			emitDialtoneIndexLine(say, "status", "Session closed.")
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
			emitDialtoneIndexLine(say, "status", "Goodbye.")
			break
		}
		executeCommand(line, defaultRoom, "", nil, nil, nil, nil, say)
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

func executeCommand(
	line string,
	room string,
	hostName string,
	registry *taskRegistry,
	taskStore *taskKVStore,
	services *serviceRegistry,
	publish func(subject string, payload []byte) error,
	emit func(BusFrame),
) {
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
	args := shellSplit(line)
	if len(args) == 0 {
		return
	}
	if args[0] == "service-list" {
		printManagedServices(room, services, emit)
		return
	}
	if args[0] == "task-stop" || args[0] == "task-kill" {
		pid, err := parseManagedPIDCommand(args)
		if err != nil {
			emitDialtoneIndexLine(emit, "status", err.Error())
			return
		}
		emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Stopping task-worker-%d.", pid))
		if err := killManagedProcessFn(pid); err != nil {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Failed to stop task-worker-%d: %v", pid, err))
			return
		}
		if registry != nil {
			if item, ok := registry.Find(pid); ok && taskStore != nil && strings.TrimSpace(item.TaskID) != "" {
				_ = taskStore.MarkExited(strings.TrimSpace(item.TaskID), pid, -1)
			}
			registry.Exited(pid, -1)
		}
		emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Stopped task-worker-%d.", pid))
		return
	}
	if args[0] == "service-stop" {
		name, err := parseServiceNameCommand(args, "service-stop")
		if err != nil {
			emitDialtoneIndexLine(emit, "status", err.Error())
			return
		}
		item, ok := services.ActiveByName(name)
		if !ok || item.PID <= 0 {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("No active service named %s.", name))
			return
		}
		emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Stopping service %s (pid %d).", name, item.PID))
		if err := killManagedProcessFn(item.PID); err != nil {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Failed to stop service %s: %v", name, err))
			return
		}
		if registry != nil {
			if taskItem, ok := registry.Find(item.PID); ok && taskStore != nil && strings.TrimSpace(taskItem.TaskID) != "" {
				_ = taskStore.MarkExited(strings.TrimSpace(taskItem.TaskID), item.PID, -1)
			}
			registry.Exited(item.PID, -1)
		}
		if services != nil {
			services.Exited(name, item.PID, -1)
		}
		if publish != nil {
			ev := proc.TaskWorkerEvent{
				PID:       item.PID,
				Args:      shellSplit(item.Command),
				LogPath:   item.LogPath,
				StartedAt: parseRFC3339(item.StartedAt),
			}
			hb := buildManagedHeartbeat(hostName, item.Room, "service", name, ev, heartbeatStateToken(false, -1), -1)
			if payload, err := encodeManagedHeartbeat(hb); err == nil {
				_ = publish(heartbeatSubject(hostName, "service", name, item.PID), payload)
			}
		}
		emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Stopped service %s.", name))
		return
	}
	if strings.HasPrefix(line, "kill ") {
		pidText := strings.TrimSpace(strings.TrimPrefix(line, "kill"))
		pid := 0
		if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid <= 0 {
			emitDialtoneIndexLine(emit, "status", "Usage: kill <pid>")
			return
		}
		if err := killManagedProcessFn(pid); err != nil {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Failed to kill process %d: %v", pid, err))
		} else {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Killed managed process %d.", pid))
		}
		return
	}
	serviceName := ""
	if args[0] == "service-start" {
		name, cmdArgs, err := parseServiceStartCommand(args)
		if err != nil {
			emitDialtoneIndexLine(emit, "status", err.Error())
			return
		}
		if item, ok := services.ActiveByName(name); ok && item.PID > 0 {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("Service %s is already running as pid %d.", name, item.PID))
			return
		}
		serviceName = name
		args = cmdArgs
	}
	if err := validateSingleCommandTokens(args); err != nil {
		emitDialtoneIndexLine(emit, "error", err.Error())
		return
	}
	isBackground := false
	if args[len(args)-1] == "&" {
		isBackground = true
		args = args[:len(args)-1]
	}
	mode := "foreground"
	if serviceName != "" {
		mode = "service"
	} else if isBackground {
		mode = "background"
	}
	taskID := nextTaskID(time.Now().UTC())
	taskRoom := taskRoomName(taskID)
	taskLog, err := newTaskLogWriter(taskID, args)
	if err != nil {
		emitDialtoneIndexFrame(emit, BusFrame{Kind: "error", TaskID: taskID, Message: fmt.Sprintf("Task %s could not create its log: %v", taskID, err)})
		return
	}
	closeTaskLog := sync.OnceFunc(func() {
		taskLog.Close()
	})
	if taskStore != nil {
		if err := taskStore.PutQueued(taskID, args, taskRoom, taskLog.LogPath, hostName, mode, serviceName); err != nil {
			taskLog.LogError(fmt.Sprintf("task kv queued write failed: %v", err))
			emitDialtoneIndexFrame(emit, BusFrame{Kind: "error", TaskID: taskID, Message: fmt.Sprintf("Task %s could not persist queued state: %v", taskID, err)})
			closeTaskLog()
			return
		}
	}
	emitTaskFrame := func(frame BusFrame) {
		if publish == nil {
			return
		}
		frame.Type = frameTypeLine
		frame.Scope = "task"
		frame.TaskID = taskID
		if strings.TrimSpace(frame.Room) == "" {
			frame.Room = taskRoom
		}
		emit(frame)
	}
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "lifecycle", TaskID: taskID, Message: "Request received."})
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "lifecycle", TaskID: taskID, Message: fmt.Sprintf("Task queued as %s.", taskID)})
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "lifecycle", TaskID: taskID, Message: fmt.Sprintf("Task topic: %s", taskRoom)})
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "lifecycle", TaskID: taskID, LogPath: taskLog.LogPath, Message: fmt.Sprintf("Task log: %s", taskLog.LogPath)})
	taskLog.LogLifecycle("task topic=%s mode=%s service=%s", taskRoom, mode, strings.TrimSpace(serviceName))
	emitTaskFrame(BusFrame{Kind: "lifecycle", Message: fmt.Sprintf("Task queued as %s.", taskID)})
	emitTaskFrame(BusFrame{Kind: "lifecycle", LogPath: taskLog.LogPath, Message: fmt.Sprintf("Task log: %s", taskLog.LogPath)})
	heartbeatInterval := 5 * time.Second
	if raw := strings.TrimSpace(os.Getenv("DIALTONE_TASK_HEARTBEAT_SEC")); raw != "" {
		if sec, err := strconv.Atoi(raw); err == nil && sec > 0 {
			heartbeatInterval = time.Duration(sec) * time.Second
		}
	}
	stopHeartbeat := make(chan struct{})
	lastLineByPID := map[int]string{}
	serviceRoom := serviceRoomName(serviceName)
	emitTaskWorkerLine := func(pid int, stderr bool, line string) {
		kind, text, ok := normalizeTaskWorkerLine(line, stderr)
		if !ok || pid <= 0 {
			return
		}
		if promoted, promotedOK := promotedIndexMessage(text); promotedOK {
			emitDialtoneIndexFrame(emit, BusFrame{Kind: "status", TaskID: taskID, PID: pid, Message: promoted})
			return
		}
		if stderr {
			taskLog.LogError(text)
		} else {
			taskLog.LogLine(text)
		}
		if lastLineByPID[pid] == kind+"\n"+text {
			return
		}
		lastLineByPID[pid] = kind + "\n" + text
		emit(BusFrame{Type: frameTypeLine, Scope: "task", Kind: kind, Room: taskRoom, TaskID: taskID, PID: pid, Message: text})
	}
	publishHeartbeat := func(ev proc.TaskWorkerEvent, state string, exitCode int) {
		if publish == nil || ev.PID <= 0 {
			return
		}
		hb := buildManagedHeartbeat(hostName, room, mode, serviceName, ev, state, exitCode)
		if serviceName != "" {
			hb.Room = serviceRoom
		}
		payload, err := encodeManagedHeartbeat(hb)
		if err != nil {
			return
		}
		_ = publish(heartbeatSubject(hostName, mode, serviceName, ev.PID), payload)
	}
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
					if registry != nil {
						registry.Heartbeat(pid)
					}
					if taskStore != nil {
						_ = taskStore.MarkHeartbeat(taskID)
					}
					if services != nil && serviceName != "" {
						services.Heartbeat(serviceName)
					}
					emit(BusFrame{
						Type:    frameTypeLine,
						Scope:   "task",
						Kind:    "lifecycle",
						Room:    taskRoom,
						TaskID:  taskID,
						PID:     pid,
						Message: fmt.Sprintf("Heartbeat: running for %s", uptime),
					})
					publishHeartbeat(proc.TaskWorkerEvent{
						PID:       pid,
						Args:      append([]string(nil), args...),
						StartedAt: startedAt,
					}, heartbeatStateToken(true, 0), 0)
				case <-stopHeartbeat:
					return
				}
			}
		}()
	}
	stopHeartbeatOnce := sync.OnceFunc(func() {
		close(stopHeartbeat)
	})
	onEvent := func(ev proc.TaskWorkerEvent) {
		switch ev.Type {
		case proc.TaskWorkerEventStarted:
			if ev.PID <= 0 {
				return
			}
			taskLog.LogLifecycle("started pid=%d worker_log=%s", ev.PID, strings.TrimSpace(ev.LogPath))
			if registry != nil {
				registryRoom := room
				if serviceName != "" {
					registryRoom = serviceRoom
				}
				registry.Started(taskID, registryRoom, mode, taskLog.LogPath, ev)
			}
			if taskStore != nil {
				if err := taskStore.MarkRunning(taskID, ev); err != nil {
					taskLog.LogError(fmt.Sprintf("task kv running update failed: %v", err))
				}
			}
			if services != nil && serviceName != "" {
				services.Started(serviceName, serviceRoom, ev)
			}
			emitDialtoneIndexFrame(emit, BusFrame{
				Kind:    "lifecycle",
				TaskID:  taskID,
				PID:     ev.PID,
				Message: fmt.Sprintf("Task %s assigned pid %d.", taskID, ev.PID),
			})
			if strings.TrimSpace(ev.LogPath) != "" {
				taskLog.LogStatus("worker log=%s", strings.TrimSpace(ev.LogPath))
			}
			if serviceName != "" {
				emitDialtoneIndexFrame(emit, BusFrame{
					Kind:    "lifecycle",
					TaskID:  taskID,
					PID:     ev.PID,
					Message: fmt.Sprintf("Service %s is running.", serviceName),
				})
			} else if isBackground {
				emitDialtoneIndexFrame(emit, BusFrame{
					Kind:    "lifecycle",
					TaskID:  taskID,
					PID:     ev.PID,
					Message: fmt.Sprintf("Task %s is running in background.", taskID),
				})
			}
			emitTaskFrame(BusFrame{Kind: "lifecycle", PID: ev.PID, Message: fmt.Sprintf("Task %s assigned pid %d.", taskID, ev.PID)})
			emitTaskFrame(BusFrame{Kind: "lifecycle", PID: ev.PID, Message: fmt.Sprintf("Started at %s", ev.StartedAt.Format(time.RFC3339))})
			emitTaskFrame(BusFrame{Kind: "lifecycle", PID: ev.PID, Message: fmt.Sprintf("Command: %v", ev.Args)})
			publishHeartbeat(ev, heartbeatStateToken(true, 0), 0)
			startHeartbeat(ev.PID, ev.StartedAt)
		case proc.TaskWorkerEventStdout:
			emitTaskWorkerLine(ev.PID, false, ev.Line)
		case proc.TaskWorkerEventStderr:
			emitTaskWorkerLine(ev.PID, true, ev.Line)
		case proc.TaskWorkerEventExited:
			stopHeartbeatOnce()
			if ev.PID > 0 {
				if registry != nil {
					registry.Exited(ev.PID, ev.ExitCode)
				}
				if taskStore != nil {
					if err := taskStore.MarkExited(taskID, ev.PID, ev.ExitCode); err != nil {
						taskLog.LogError(fmt.Sprintf("task kv exit update failed: %v", err))
					}
				}
				if services != nil && serviceName != "" {
					services.Exited(serviceName, ev.PID, ev.ExitCode)
				}
				publishHeartbeat(ev, heartbeatStateToken(false, ev.ExitCode), ev.ExitCode)
				taskLog.LogLifecycle("exited pid=%d code=%d", ev.PID, ev.ExitCode)
				emitDialtoneIndexFrame(emit, BusFrame{
					Kind:     "lifecycle",
					TaskID:   taskID,
					PID:      ev.PID,
					ExitCode: ev.ExitCode,
					Message:  taskExitMessage(taskID, ev.ExitCode),
				})
				emitTaskFrame(BusFrame{Kind: "lifecycle", PID: ev.PID, ExitCode: ev.ExitCode, Message: taskExitMessage(taskID, ev.ExitCode)})
				closeTaskLog()
				return
			}
			if line := strings.TrimSpace(ev.Line); line != "" {
				if taskStore != nil {
					_ = taskStore.MarkExited(taskID, 0, 1)
				}
				taskLog.LogError(fmt.Sprintf("failed to start: %s", line))
				emitDialtoneIndexFrame(emit, BusFrame{Kind: "error", TaskID: taskID, Message: fmt.Sprintf("Task %s failed to start: %s", taskID, line)})
			} else {
				if taskStore != nil {
					_ = taskStore.MarkExited(taskID, 0, 1)
				}
				taskLog.LogError("failed to start")
				emitDialtoneIndexFrame(emit, BusFrame{Kind: "error", TaskID: taskID, Message: fmt.Sprintf("Task %s failed to start.", taskID)})
			}
			emitTaskFrame(BusFrame{Kind: "error", Message: fmt.Sprintf("Task %s failed to start.", taskID)})
			closeTaskLog()
		}
	}

	if err := waitForTaskStartHold(); err != nil {
		if taskStore != nil {
			_ = taskStore.MarkExited(taskID, 0, 1)
		}
		taskLog.LogError(fmt.Sprintf("failed to start: %v", err))
		emitDialtoneIndexFrame(emit, BusFrame{Kind: "error", TaskID: taskID, Message: fmt.Sprintf("Task %s failed to start: %v", taskID, err)})
		emitTaskFrame(BusFrame{Kind: "error", Message: fmt.Sprintf("Task %s failed to start.", taskID)})
		closeTaskLog()
		return
	}
	if isBackground || serviceName != "" {
		go runTaskWorkerWithEventsFn(args, onEvent)
		return
	}
	runTaskWorkerWithEventsFn(args, onEvent)
}

func waitForTaskStartHold() error {
	holdPath := strings.TrimSpace(os.Getenv("DIALTONE_REPL_TEST_TASK_START_HOLD_FILE"))
	if holdPath == "" {
		return nil
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(holdPath); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for test task start hold file %s to clear", holdPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func shellSplit(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	args := make([]string, 0, 8)
	var cur strings.Builder
	var quote rune
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		args = append(args, cur.String())
		cur.Reset()
	}
	for i := 0; i < len(line); i++ {
		ch := rune(line[i])
		if quote != 0 {
			if ch == quote {
				quote = 0
				continue
			}
			if ch == '\\' && quote == '"' && i+1 < len(line) {
				i++
				cur.WriteByte(line[i])
				continue
			}
			cur.WriteRune(ch)
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case ' ', '\t', '\n', '\r':
			flush()
		case '\\':
			if i+1 < len(line) {
				i++
				cur.WriteByte(line[i])
			}
		default:
			cur.WriteRune(ch)
		}
	}
	flush()
	return args
}

func validateSingleCommandTokens(args []string) error {
	if len(args) == 0 {
		return nil
	}
	for i, arg := range args {
		token := strings.TrimSpace(arg)
		switch token {
		case "&&", "||", ";":
			return fmt.Errorf("DIALTONE ERROR: run exactly one command at a time; command chaining with %q is not allowed. Use one foreground command per turn, or a single command with a trailing & for background mode.", token)
		case "&":
			if i != len(args)-1 {
				return fmt.Errorf("DIALTONE ERROR: run exactly one command at a time; only a trailing & is allowed for background mode")
			}
		}
	}
	return nil
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
		"Run logs plugin tests as a task",
		"",
		"System",
		"`ps`",
		"List running tasks",
		"",
		"`repl src_v3 task list`",
		"List task snapshots from the leader task registry",
		"",
		"`repl src_v3 task show --task-id <task-id>`",
		"Show the current task snapshot for one task",
		"",
		"`repl src_v3 task log --task-id <task-id> --lines 200`",
		"Read the durable task log for one task",
		"",
		"`repl src_v3 task kill --task-id <task-id>`",
		"Stop a running task by task id",
		"",
		"`/task-attach --task-id <task-id>`",
		"Stream one task topic directly in the REPL console",
		"",
		"`/task-detach`",
		"Return to the shared topic after streaming one task",
		"",
		"`/service-start --name <name> -- <command...>`",
		"Start a managed long-lived service",
		"",
		"`/service-stop --name <name>`",
		"Stop a managed service by name",
		"",
		"`/service-list`",
		"List managed services",
		"",
		"`<any command>`",
		"Queue any dialtone command as a task",
	}
	for _, line := range content {
		emitDialtoneIndexLine(emit, "status", line)
	}
}

func printManagedProcesses(room string, registry *taskRegistry, emit func(BusFrame)) {
	items := []taskRegistryItem(nil)
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
			emitDialtoneIndexLine(emit, "status", "No running tasks.")
			return
		}
		emitDialtoneIndexLine(emit, "status", "Running Tasks:")
		emitDialtoneIndexLine(emit, "status", fmt.Sprintf("%-28s %-8s %-8s %-12s %-8s %-8s %s", "TASK ID", "PID", "UPTIME", "MODE", "CPU%", "PORTS", "COMMAND"))
		for _, p := range procs {
			emitDialtoneIndexLine(emit, "status", fmt.Sprintf("%-28s %-8d %-8s %-12s %-8.1f %-8d %s", "-", p.PID, p.StartedAgo, "unknown", p.CPUPercent, p.PortCount, p.Command))
		}
		return
	}
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "status", Room: room, Message: "Running Tasks:"})
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "status", Room: room, Message: fmt.Sprintf("%-28s %-8s %-8s %-12s %-8s %-8s %s", "TASK ID", "PID", "UPTIME", "MODE", "CPU%", "PORTS", "COMMAND")})
	for _, item := range items {
		uptime := strings.TrimSpace(item.StartedAgo)
		if uptime == "" {
			uptime = "-"
		}
		taskID := strings.TrimSpace(item.TaskID)
		if taskID == "" {
			taskID = "-"
		}
		command := strings.TrimSpace(item.Command)
		if command == "" {
			command = "-"
		}
		emitDialtoneIndexFrame(emit, BusFrame{
			Kind:    "status",
			Room:    room,
			LogPath: strings.TrimSpace(item.LogPath),
			Message: fmt.Sprintf("%-28s %-8d %-8s %-12s %-8.1f %-8d %s", taskID, item.PID, uptime, defaultManagedMode(item.Mode), item.CPUPercent, item.PortCount, command),
		})
	}
}

func printManagedServices(room string, registry *serviceRegistry, emit func(BusFrame)) {
	items := []serviceRegistryItem(nil)
	if registry != nil {
		items = registry.Snapshot(0, listManagedFn())
	}
	if len(items) == 0 {
		emitDialtoneIndexLine(emit, "status", "No managed services.")
		return
	}
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "status", Room: room, Message: "Managed Services:"})
	emitDialtoneIndexFrame(emit, BusFrame{Kind: "status", Room: room, Message: fmt.Sprintf("%-16s %-10s %-8s %-24s %-8s %-12s %s", "NAME", "HOST", "PID", "UPDATED", "STATE", "MODE", "COMMAND")})
	for _, item := range items {
		command := strings.TrimSpace(item.Command)
		if command == "" {
			command = "-"
		}
		updated := strings.TrimSpace(item.LastHeartbeat)
		if updated == "" {
			updated = strings.TrimSpace(item.LastUpdate)
		}
		if updated == "" {
			updated = "-"
		}
		state := "done"
		if item.Active {
			state = "active"
		}
		host := strings.TrimSpace(item.Host)
		if host == "" {
			host = "local"
		}
		emitDialtoneIndexFrame(emit, BusFrame{
			Kind:    "status",
			Room:    room,
			LogPath: strings.TrimSpace(item.LogPath),
			Message: fmt.Sprintf("%-16s %-10s %-8d %-24s %-8s %-12s %s", item.Name, host, item.PID, updated, state, defaultManagedMode(item.Mode), command),
		})
	}
}

func parseManagedPIDCommand(args []string) (int, error) {
	if len(args) < 3 {
		return 0, fmt.Errorf("Usage: task-stop --pid <pid>")
	}
	for i := 1; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		switch {
		case token == "--pid" && i+1 < len(args):
			pid, err := strconv.Atoi(strings.TrimSpace(args[i+1]))
			if err != nil || pid <= 0 {
				return 0, fmt.Errorf("Usage: task-stop --pid <pid>")
			}
			return pid, nil
		case strings.HasPrefix(token, "--pid="):
			pid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(token, "--pid=")))
			if err != nil || pid <= 0 {
				return 0, fmt.Errorf("Usage: task-stop --pid <pid>")
			}
			return pid, nil
		}
	}
	return 0, fmt.Errorf("Usage: task-stop --pid <pid>")
}

func parseServiceStartCommand(args []string) (string, []string, error) {
	if len(args) < 5 {
		return "", nil, fmt.Errorf("Usage: service-start --name <name> -- <command...>")
	}
	name, err := parseServiceNameCommand(args[:3], "service-start")
	if err != nil {
		return "", nil, err
	}
	for i := 3; i < len(args); i++ {
		if strings.TrimSpace(args[i]) == "--" {
			cmdArgs := append([]string(nil), args[i+1:]...)
			if len(cmdArgs) == 0 {
				return "", nil, fmt.Errorf("Usage: service-start --name <name> -- <command...>")
			}
			return name, cmdArgs, nil
		}
	}
	return "", nil, fmt.Errorf("Usage: service-start --name <name> -- <command...>")
}

func parseServiceNameCommand(args []string, command string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("Usage: %s --name <name>", command)
	}
	for i := 1; i < len(args); i++ {
		token := strings.TrimSpace(args[i])
		switch {
		case token == "--name" && i+1 < len(args):
			name := strings.TrimSpace(args[i+1])
			if name == "" {
				return "", fmt.Errorf("Usage: %s --name <name>", command)
			}
			return name, nil
		case strings.HasPrefix(token, "--name="):
			name := strings.TrimSpace(strings.TrimPrefix(token, "--name="))
			if name == "" {
				return "", fmt.Errorf("Usage: %s --name <name>", command)
			}
			return name, nil
		}
	}
	return "", fmt.Errorf("Usage: %s --name <name>", command)
}

func defaultManagedMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "foreground"
	}
	return mode
}

func serviceRoomName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return "service:" + sanitizeRoom(name)
}

func parseRFC3339(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func taskWorkerRoomName(pid int) string {
	if pid <= 0 {
		return ""
	}
	return fmt.Sprintf("task-worker-%d", pid)
}

func taskRoomName(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ""
	}
	return "task." + taskID
}

func nextTaskID(now time.Time) string {
	now = now.UTC()
	stamp := now.Format("20060102-150405-000")
	taskIDMu.Lock()
	defer taskIDMu.Unlock()
	if stamp == taskIDLastStamp {
		taskIDSeq++
	} else {
		taskIDLastStamp = stamp
		taskIDSeq = 1
	}
	if taskIDSeq <= 1 {
		return fmt.Sprintf("task-%s", stamp)
	}
	return fmt.Sprintf("task-%s-%03d", stamp, taskIDSeq)
}

func taskExitMessage(taskID string, exitCode int) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		if exitCode < 0 {
			return "Task stopped."
		}
		return fmt.Sprintf("Task exited with code %d.", exitCode)
	}
	if exitCode < 0 {
		return fmt.Sprintf("Task %s stopped.", taskID)
	}
	return fmt.Sprintf("Task %s exited with code %d.", taskID, exitCode)
}

func publishPresenceReport(
	room string,
	mode string,
	rows []presenceRow,
	publishRoom func(targetRoom string, f BusFrame),
) {
	if len(rows) == 0 {
		publishDialtoneIndexLine(publishRoom, room, "status", "No connected users.")
		return
	}
	switch mode {
	case "versions":
		publishDialtoneIndexLine(publishRoom, room, "status", "Connected versions:")
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
				publishDialtoneIndexFrame(publishRoom, room, BusFrame{
					Kind: "status",
					Message: fmt.Sprintf(
						"- [daemon] %s daemon=%s repl=%s topic=%s os=%s arch=%s",
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
			publishDialtoneIndexFrame(publishRoom, room, BusFrame{
				Kind: "status",
				Message: fmt.Sprintf(
					"- [client] %s repl=%s topic=%s os=%s arch=%s",
					row.Name,
					version,
					sanitizeRoom(row.Room),
					fallbackUnknown(row.OS),
					fallbackUnknown(row.Arch),
				),
			})
		}
	default:
		publishDialtoneIndexLine(publishRoom, room, "status", "Connected sessions:")
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
				publishDialtoneIndexFrame(publishRoom, room, BusFrame{
					Kind: "status",
					Message: fmt.Sprintf(
						"- [daemon] %s topic=%s daemon=%s repl=%s os=%s arch=%s",
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
			publishDialtoneIndexFrame(publishRoom, room, BusFrame{
				Kind: "status",
				Message: fmt.Sprintf(
					"- [client] %s topic=%s repl=%s os=%s arch=%s",
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
	return "repl.topic." + sanitizeRoom(room)
}

func replTopicSubjectLabel(room string) string {
	return "repl.topic." + sanitizeRoom(room)
}

func replTaskWorkerSubject(pid int) string {
	if pid <= 0 {
		return ""
	}
	return fmt.Sprintf("repl.task-worker.%d", pid)
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

func parseTaskAttachCommand(line string) (taskID string, ok bool, err error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/task-attach") {
		return "", false, nil
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", true, fmt.Errorf("usage: /task-attach --task-id <task-id>")
	}
	for i := 1; i < len(fields); i++ {
		switch strings.TrimSpace(fields[i]) {
		case "--task-id":
			if i+1 >= len(fields) {
				return "", true, fmt.Errorf("usage: /task-attach --task-id <task-id>")
			}
			taskID = strings.TrimSpace(fields[i+1])
			if !strings.HasPrefix(taskID, "task-") {
				return "", true, fmt.Errorf("invalid task id %q", strings.TrimSpace(fields[i+1]))
			}
			return taskID, true, nil
		}
	}
	if len(fields) == 2 {
		taskID = strings.TrimSpace(fields[1])
		if strings.HasPrefix(taskID, "task-") {
			return taskID, true, nil
		}
	}
	return "", true, fmt.Errorf("usage: /task-attach --task-id <task-id>")
}

func isDetachCommand(line string) bool {
	line = strings.TrimSpace(line)
	return line == "/task-detach" || line == "task-detach"
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
		natsURL = resolveREPLNATSURL()
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

func normalizeTaskWorkerLine(line string, stderr bool) (kind string, text string, ok bool) {
	text = strings.TrimSpace(line)
	if text == "" {
		return "", "", false
	}
	if looksLikeProgressNoise(text) {
		return "", "", false
	}
	kind = "log"
	if stderr && !looksLikeBenignStderr(text) {
		kind = "error"
	}
	return kind, text, true
}

func looksLikeProgressNoise(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if strings.Contains(line, "\r") && strings.Contains(line, "%") {
		return true
	}
	if strings.HasPrefix(line, "#=#=#") || strings.HasPrefix(line, "##O") {
		return true
	}
	return false
}

func looksLikeBenignStderr(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	switch line {
	case "Saved lockfile":
		return true
	}
	return false
}

func promotedIndexMessage(line string) (string, bool) {
	text := strings.TrimSpace(line)
	if !strings.HasPrefix(text, indexStatusTag) {
		return "", false
	}
	text = strings.TrimSpace(strings.TrimPrefix(text, indexStatusTag))
	if text == "" {
		return "", false
	}
	return text, true
}

// Keep the local names for readability in this file, but route all behavior
// through the canonical helpers in dialtone_output.go.
func dialtoneIndexFrame(frame BusFrame) BusFrame {
	return DialtoneIndexFrame(frame)
}

func emitDialtoneIndexFrame(emit func(BusFrame), frame BusFrame) {
	EmitDialtoneIndexFrame(emit, frame)
}

func emitDialtoneIndexLine(emit func(BusFrame), kind, message string) {
	EmitDialtoneIndexLine(emit, kind, message)
}

func publishDialtoneIndexFrame(publishRoom func(string, BusFrame), room string, frame BusFrame) {
	PublishDialtoneIndexFrame(publishRoom, room, frame)
}

func publishDialtoneIndexLine(publishRoom func(string, BusFrame), room, kind, message string) {
	PublishDialtoneIndexLine(publishRoom, room, kind, message)
}

func writeDialtoneLine(w io.Writer, prefix, message string) {
	WriteDialtoneLine(w, prefix, message)
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
		writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("%s: %s", name, strings.TrimSpace(frame.Message)))
	case frameTypeLine:
		prefix := strings.TrimSpace(frame.Prefix)
		if prefix == "" && frame.Scope == "task-worker" && frame.PID > 0 {
			prefix = fmt.Sprintf("DIALTONE:%d", frame.PID)
		}
		if prefix == "" {
			prefix = "DIALTONE"
		}
		text := strings.TrimSpace(frame.Message)
		if frame.Kind == "error" && text != "" && !strings.HasPrefix(text, "[ERROR]") {
			text = "[ERROR] " + text
		}
		writeDialtoneLine(w, prefix, text)
	case frameTypeServer:
		writeDialtoneLine(w, "DIALTONE", strings.TrimSpace(frame.Message))
	case frameTypeJoin:
		name := normalizePromptName(frame.From)
		if name == "" {
			name = normalizePromptName(frame.Message)
		}
		if name == "" {
			name = "unknown"
		}
		if strings.TrimSpace(frame.Room) == "" {
			writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("%s joined.", name))
		} else if strings.TrimSpace(frame.Version) == "" {
			writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("%s joined topic %s.", name, sanitizeRoom(frame.Room)))
		} else {
			writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("%s joined topic %s (version=%s).", name, sanitizeRoom(frame.Room), strings.TrimSpace(frame.Version)))
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
			writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("%s left.", name))
		} else {
			writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("%s left topic %s.", name, sanitizeRoom(frame.Room)))
		}
	case frameTypeControl:
		text := strings.TrimSpace(frame.Message)
		if text == "" {
			text = fmt.Sprintf("%s %s", strings.TrimSpace(frame.Command), strings.TrimSpace(frame.Room))
		}
		writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("Control: %s", strings.TrimSpace(text)))
	case frameTypeError:
		writeDialtoneLine(w, "DIALTONE", fmt.Sprintf("Error: %s", strings.TrimSpace(frame.Message)))
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
	srv.Ephemeral = true
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
