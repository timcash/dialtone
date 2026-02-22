package repl

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
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
	defaultRoom    = "main"
)

const (
	frameTypeInput     = "input"
	frameTypeLine      = "line"
	frameTypeProbe     = "probe"
	frameTypeServer    = "server"
	frameTypeHeartbeat = "heartbeat"
	frameTypeJoin      = "join"
	frameTypeLeft      = "left"
)

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
	Type      string `json:"type"`
	From      string `json:"from,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Message   string `json:"message,omitempty"`
	ServerID  string `json:"server_id,omitempty"`
	Timestamp string `json:"timestamp"`
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

func RunServe(args []string) error {
	fs := flag.NewFlagSet("repl-serve", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared REPL room")
	embedded := fs.Bool("embedded-nats", true, "Start embedded NATS on --nats-url")
	enableTSNet := fs.Bool("tsnet", false, "Start embedded tsnet identity on host")
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
	if *enableTSNet {
		cleanup, upErr := startTSNetInstance(normalizePromptName(*hostname))
		if upErr != nil {
			logs.Warn("REPL tsnet startup failed: %v", upErr)
		} else {
			stopTSNet = cleanup
		}
	}
	defer stopTSNet()

	h := normalizePromptName(*hostname)
	roomName := sanitizeRoom(*room)
	subject := replSubject(roomName)
	serverID := h + "@" + roomName

	publish := func(f BusFrame) {
		f.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
		f.ServerID = serverID
		_ = publishFrame(nc, subject, f)
	}

	// Publish initial presence line to NATS so every connected client sees it.
	publish(BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("DIALTONE server online on %s (subject=%s nats=%s)", h, subject, usedURL)})
	logs.Info("REPL host serving: hostname=%s room=%s subject=%s nats=%s", h, roomName, subject, usedURL)

	var runMu sync.Mutex
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		printFrame(os.Stdout, frame)
		switch frame.Type {
		case frameTypeProbe:
			publish(BusFrame{Type: frameTypeServer, Message: fmt.Sprintf("DIALTONE server active on %s", h)})
		case frameTypeInput:
			go func(in BusFrame) {
				runMu.Lock()
				defer runMu.Unlock()
				executeCommand(strings.TrimSpace(in.Message), func(prefix, msg string) {
					publish(BusFrame{Type: frameTypeLine, Prefix: prefix, Message: msg})
				})
			}(frame)
		}
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
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
			publish(BusFrame{Type: frameTypeHeartbeat, Message: "alive"})
		case <-sig:
			publish(BusFrame{Type: frameTypeServer, Message: "DIALTONE server shutting down."})
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

	nc, err := nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	prompt := normalizePromptName(*name)
	roomName := sanitizeRoom(*room)
	subject := replSubject(roomName)

	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		frame, ok := decodeFrame(msg.Data)
		if !ok {
			return
		}
		printFrame(os.Stdout, frame)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}

	_ = publishFrame(nc, subject, BusFrame{Type: frameTypeProbe, From: prompt, Message: "probe"})
	_ = publishFrame(nc, subject, BusFrame{Type: frameTypeJoin, From: prompt, Message: prompt})
	_ = nc.Flush()
	fmt.Fprintf(os.Stdout, "DIALTONE> Connected to %s via %s\n", subject, strings.TrimSpace(*natsURL))

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
		if err := publishFrame(nc, subject, BusFrame{Type: frameTypeInput, From: prompt, Message: line}); err != nil {
			return err
		}
		if err := nc.Flush(); err != nil {
			return err
		}
	}
	_ = publishFrame(nc, subject, BusFrame{Type: frameTypeLeft, From: prompt, Message: prompt})
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
	subject := replSubject(roomName)
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
			_ = publishFrame(nc, subject, BusFrame{Type: frameTypeProbe, From: st.HostName, Message: "status-probe"})
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

func replSubject(room string) string {
	return "repl." + sanitizeRoom(room)
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
		fmt.Fprintf(w, "DIALTONE> [JOIN] %s\n", name)
	case frameTypeLeft:
		name := normalizePromptName(frame.From)
		if name == "" {
			name = normalizePromptName(frame.Message)
		}
		if name == "" {
			name = "unknown"
		}
		fmt.Fprintf(w, "DIALTONE> [LEFT] %s\n", name)
	}
}

func startTSNetInstance(hostname string) (func(), error) {
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
	logs.Info("REPL tsnet identity online: hostname=%s tailnet=%s ips=%v", status.Self.HostName, cfg.Tailnet, status.TailscaleIPs)
	return func() {
		_ = srv.Close()
	}, nil
}
