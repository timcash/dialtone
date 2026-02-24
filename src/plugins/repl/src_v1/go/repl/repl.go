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
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/proc/src_v1/go/proc"
	tsnetlib "dialtone/dev/plugins/tsnet/src_v1/go"
	"github.com/nats-io/nats.go"
)

const (
	defaultNATSURL = "nats://127.0.0.1:4222"
	defaultRoom    = "index"
	commandSubject = "repl.cmd"
)

const (
	frameTypeInput     = "input"
	frameTypeLine      = "line"
	frameTypeProbe     = "probe"
	frameTypeServer    = "server"
	frameTypeHeartbeat = "heartbeat"
	frameTypeJoin      = "join"
	frameTypeLeft      = "left"
	frameTypeChat      = "chat"
	frameTypeCommand   = "command"
	frameTypeControl   = "control"
	frameTypeError     = "error"
)

const controlJoinRoom = "join_room"

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
	Type      string   `json:"type"`
	From      string   `json:"from,omitempty"`
	Target    string   `json:"target,omitempty"`
	Room      string   `json:"room,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Prefix    string   `json:"prefix,omitempty"`
	Message   string   `json:"message,omitempty"`
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
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Primary REPL room")
	embedded := fs.Bool("embedded-nats", true, "Start embedded NATS on --nats-url")
	enableTSNet := fs.Bool("tsnet", false, "Start embedded tsnet identity on host")
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
	if *enableTSNet {
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

	// Publish initial presence line to NATS so every connected client sees it.
	publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("DIALTONE leader online on %s (subject=%s nats=%s)", h, replRoomSubject(roomName), usedURL)})
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
				logs.Info("REPL tsnet NATS endpoint active: %s -> %s", tsURL, targetAddr)
				publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("DIALTONE tsnet NATS endpoint: %s", tsURL)})
			}
		}
	}
	defer func() {
		if tsnetListener != nil {
			_ = tsnetListener.Close()
		}
	}()

	var roomMu sync.RWMutex
	userRoom := map[string]string{}
	var runMu sync.Mutex
	cmdSub, err := nc.Subscribe(commandSubject, func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		switch frame.Type {
		case frameTypeProbe:
			targetRoom := sanitizeRoom(frame.Room)
			publishRoom(targetRoom, BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("DIALTONE leader active on %s", h)})
		case frameTypeCommand:
			sender := normalizePromptName(frame.From)
			currentRoom := sanitizeRoom(frame.Room)
			if sender == "" {
				return
			}
			roomMu.RLock()
			if currentRoom == "" {
				currentRoom = sanitizeRoom(userRoom[sender])
			}
			roomMu.RUnlock()
			if currentRoom == "" {
				currentRoom = defaultRoom
			}

			raw := strings.TrimSpace(frame.Message)
			if strings.HasPrefix(raw, "/") {
				raw = strings.TrimSpace(strings.TrimPrefix(raw, "/"))
			}
			args := strings.Fields(raw)
			if len(args) == 0 {
				return
			}

			roomMu.Lock()
			userRoom[sender] = currentRoom
			roomMu.Unlock()
			publishRoom(currentRoom, BusFrame{Type: frameTypeInput, From: sender, Message: "/" + raw})

			if len(args) >= 4 && args[0] == "repl" && args[1] == "src_v1" && args[2] == "join" {
				targetRoom := sanitizeRoom(args[3])
				if targetRoom == "" {
					publishRoom(currentRoom, BusFrame{Type: frameTypeError, Message: "Usage: /repl src_v1 join <room-name>"})
					return
				}
				if targetRoom == currentRoom {
					publishRoom(currentRoom, BusFrame{Type: frameTypeLine, Prefix: "DIALTONE", Message: fmt.Sprintf("%s is already in room %s", sender, targetRoom)})
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
				roomMu.Lock()
				userRoom[sender] = targetRoom
				roomMu.Unlock()
				return
			}

			go func(in BusFrame) {
				runMu.Lock()
				defer runMu.Unlock()
				executeCommand(strings.TrimSpace(in.Message), func(prefix, msg string) {
					publishRoom(currentRoom, BusFrame{Type: frameTypeLine, Prefix: prefix, Message: msg})
				})
			}(BusFrame{Message: raw})
		}
	})
	if err != nil {
		return err
	}
	defer cmdSub.Unsubscribe()

	roomSub, err := nc.Subscribe("repl.room.*", func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
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
			seen := map[string]struct{}{roomName: {}}
			roomMu.RLock()
			for _, r := range userRoom {
				r = sanitizeRoom(r)
				if r == "" {
					continue
				}
				seen[r] = struct{}{}
			}
			roomMu.RUnlock()
			for r := range seen {
				publishRoom(r, BusFrame{Type: frameTypeHeartbeat, Message: "alive"})
			}
		case <-sig:
			publishRoom(roomName, BusFrame{Type: frameTypeServer, Message: "DIALTONE leader shutting down."})
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
	var switchRoom func(string, bool) error

	onRoomFrame := func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		printFrame(os.Stdout, frame)
		if frame.Type == frameTypeControl && frame.Target == prompt && frame.Command == controlJoinRoom {
			nextRoom := sanitizeRoom(frame.Room)
			_ = switchRoom(nextRoom, true)
		}
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
		_ = publishFrame(nc, targetSubj, BusFrame{Type: frameTypeJoin, From: prompt, Room: targetRoom})
		_ = nc.Flush()
		if announce {
			fmt.Fprintf(os.Stdout, "DIALTONE> Connected to %s via %s\n", targetSubj, natsAddr)
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
		if sub != nil {
			_ = sub.Unsubscribe()
		}
	}()
	if err := nc.Flush(); err != nil {
		return err
	}

	_ = publishFrame(nc, commandSubject, BusFrame{Type: frameTypeProbe, From: prompt, Room: currentRoom, Message: "probe"})
	_ = publishFrame(nc, currentSubj, BusFrame{Type: frameTypeJoin, From: prompt, Room: currentRoom})
	_ = nc.Flush()
	fmt.Fprintf(os.Stdout, "DIALTONE> Connected to %s via %s\n", currentSubj, natsAddr)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprintf(os.Stdout, "%s> ", prompt)
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

		subMu.Lock()
		roomNow := currentRoom
		subjectNow := currentSubj
		subMu.Unlock()
		if strings.HasPrefix(line, "/") {
			if err := publishFrame(nc, commandSubject, BusFrame{
				Type:    frameTypeCommand,
				From:    prompt,
				Room:    roomNow,
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
	st.ChromePath = strings.TrimSpace(chrome.FindChromePath())
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

func runLocalSession(in io.Reader, out io.Writer, promptName string, logFn func(category, msg string)) error {
	if logFn == nil {
		logFn = func(string, string) {}
	}

	say := func(msg string) {
		line := "DIALTONE> " + msg
		fmt.Fprintln(out, line)
		logs.Info("[REPL] %s", line)
		logFn("REPL", line)
	}
	sayPrefixed := func(prefix, msg string) {
		line := fmt.Sprintf("%s> %s", prefix, msg)
		fmt.Fprintln(out, line)
		logs.Info("[REPL] %s", line)
		logFn("REPL", line)
	}

	say("Virtual Librarian online.")
	say("Type 'help' for commands, or 'exit' to quit.")

	scanner := bufio.NewScanner(in)
	tty := isInputTTY(in)
	for {
		fmt.Fprintf(out, "%s> ", promptName)
		if !scanner.Scan() {
			say("Session closed.")
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
			say("Goodbye.")
			break
		}
		executeCommand(line, func(prefix, msg string) {
			sayPrefixed(prefix, msg)
		})
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

func executeCommand(line string, emit func(prefix, msg string)) {
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
		printManagedProcesses(emit)
		return
	}
	if strings.HasPrefix(line, "kill ") {
		pidText := strings.TrimSpace(strings.TrimPrefix(line, "kill"))
		pid := 0
		if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid <= 0 {
			emit("DIALTONE", "Usage: kill <pid>")
			return
		}
		if err := killManagedProcessFn(pid); err != nil {
			emit("DIALTONE", fmt.Sprintf("Failed to kill process %d: %v", pid, err))
		} else {
			emit("DIALTONE", fmt.Sprintf("Killed managed process %d.", pid))
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

	emit("DIALTONE", fmt.Sprintf("Request received. Spawning subtone for %s...", cmdName))
	onEvent := func(ev proc.SubtoneEvent) {
		switch ev.Type {
		case proc.SubtoneEventStarted:
			if ev.PID <= 0 {
				return
			}
			emit(fmt.Sprintf("DIALTONE:%d", ev.PID), fmt.Sprintf("Started at %s", ev.StartedAt.Format(time.RFC3339)))
			emit(fmt.Sprintf("DIALTONE:%d", ev.PID), fmt.Sprintf("Command: %v", ev.Args))
			if strings.TrimSpace(ev.LogPath) != "" {
				emit(fmt.Sprintf("DIALTONE:%d", ev.PID), fmt.Sprintf("Log: %s", ev.LogPath))
			}
		case proc.SubtoneEventStdout:
			if ev.PID <= 0 {
				return
			}
			if hasStructuredLevel(ev.Line) {
				emit(fmt.Sprintf("DIALTONE:%d", ev.PID), ev.Line)
			}
		case proc.SubtoneEventStderr:
			if ev.PID <= 0 {
				return
			}
			line := strings.TrimSpace(ev.Line)
			if line == "" {
				return
			}
			emit(fmt.Sprintf("DIALTONE:%d", ev.PID), "[ERROR] "+line)
		case proc.SubtoneEventExited:
			if ev.PID > 0 {
				emit("DIALTONE", fmt.Sprintf("Subtone for %s exited with code %d.", cmdName, ev.ExitCode))
			}
		}
	}

	if isBackground {
		go runSubtoneWithEventsFn(args, onEvent)
		emit("DIALTONE", fmt.Sprintf("Subtone for %s started in background.", cmdName))
		return
	}
	runSubtoneWithEventsFn(args, onEvent)
}

func printHelp(emit func(prefix, msg string)) {
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
		"`kill <pid>`",
		"Kill a managed subtone process by PID",
		"",
		"`<any command>`",
		"Run any dialtone command on a managed subtone",
	}
	for _, line := range content {
		emit("DIALTONE", line)
	}
}

func printManagedProcesses(emit func(prefix, msg string)) {
	procs := listManagedFn()
	if len(procs) == 0 {
		emit("DIALTONE", "No active subtones.")
		return
	}
	emit("DIALTONE", "Active Subtones:")
	emit("DIALTONE", fmt.Sprintf("%-8s %-8s %-10s %-8s %s", "PID", "UPTIME", "CPU%", "PORTS", "COMMAND"))
	for _, p := range procs {
		emit("DIALTONE", fmt.Sprintf("%-8d %-8s %-10.1f %-8d %s", p.PID, p.StartedAgo, p.CPUPercent, p.PortCount, p.Command))
	}
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
		fmt.Fprintf(w, "DIALTONE> [CHAT] %s: %s\n", name, strings.TrimSpace(frame.Message))
	case frameTypeLine:
		prefix := strings.TrimSpace(frame.Prefix)
		if prefix == "" {
			prefix = "DIALTONE"
		}
		fmt.Fprintf(w, "%s> %s\n", prefix, strings.TrimSpace(frame.Message))
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
			fmt.Fprintf(w, "DIALTONE> [JOIN] %s (room=%s)\n", name, sanitizeRoom(frame.Room))
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
