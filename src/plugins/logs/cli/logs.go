package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/nats-io/nats.go"
)

// RunLogs handles the logs command
func RunLogs(versionDir string, args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	remote := fs.Bool("remote", false, "Stream logs from remote robot")
	topic := fs.String("topic", "logs.>", "NATS subject to subscribe to (supports wildcards '*' and '>')")
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	embedded := fs.Bool("embedded", false, "Start embedded NATS server for local stream")
	stdout := fs.Bool("stdout", true, "Print streamed messages to stdout")
	outFile := fs.String("file", "", "Append streamed messages to file path")
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	lines := fs.Int("lines", 0, "Number of lines to show (default: stream logs)")
	showHelp := fs.Bool("help", false, "Show help for logs command")

	fs.Usage = func() {
		fmt.Println("Usage: ./dialtone.sh logs stream [src_vN] [options]")
		fmt.Println()
		fmt.Println("Stream logs from Dialtone via NATS subjects.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --topic <subject>  NATS subject filter (default: 'logs.>')")
		fmt.Println("                     - use '>' to match all tokens (e.g., 'logs.>')")
		fmt.Println("                     - use '*' to match a single token (e.g., 'logs.*.smoke')")
		fmt.Println("  --nats-url <url>   NATS server URL (default: nats://127.0.0.1:4222)")
		fmt.Println("  --embedded         Start embedded NATS server for local stream")
		fmt.Println("  --stdout           Print streamed messages (default: true)")
		fmt.Println("  --file <path>      Append streamed messages to file")
		fmt.Println("  --remote           Stream logs from remote robot")
		fmt.Println("  --lines            Number of lines to show (if set, does not stream)")
		fmt.Println("  --host             SSH host (user@host) [env: ROBOT_HOST]")
		fmt.Println("  --port             SSH port (default: 22)")
		fmt.Println("  --user             SSH username [env: ROBOT_USER]")
		fmt.Println("  --pass             SSH password [env: ROBOT_PASSWORD]")
		fmt.Println("  --help             Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  ./dialtone.sh logs stream --topic 'logs.>'          # All logs")
		fmt.Println("  ./dialtone.sh logs stream --topic 'logs.task.>'     # All task plugin logs")
		fmt.Println("  ./dialtone.sh logs stream --topic 'logs.*.smoke'    # Smoke test logs for any plugin")
		fmt.Println("  ./dialtone.sh logs stream --topic 'logs.dag.v1'     # Specific dag v1 log stream")
		fmt.Println("  ./dialtone.sh logs stream --topic 'logs.>' --file ./logs.txt")
		fmt.Println()
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	if *remote {
		if *host == "" || *pass == "" {
			logs.Fatal("Error: --host and --pass are required for remote logs")
		}

		runRemoteLogs(*host, *port, *user, *pass, *lines)
	} else {
		if !*stdout && strings.TrimSpace(*outFile) == "" {
			logs.Fatal("Error: local stream requires at least one output sink (--stdout or --file)")
		}
		if err := runLocalNATSStream(versionDir, *natsURL, resolveTopic(*topic), *embedded, *stdout, *outFile); err != nil {
			logs.Fatal("%v", err)
		}
	}
}

func resolveTopic(topic string) string {
	t := strings.TrimSpace(topic)
	if t == "" || t == "*" || strings.EqualFold(t, "all") {
		return "logs.>"
	}
	return t
}

func runLocalNATSStream(versionDir, natsURL, subject string, embedded, toStdout bool, filePath string) error {
	nc, broker, usedURL, err := connectLocalNATS(versionDir, natsURL, embedded)
	if err != nil {
		return err
	}
	defer nc.Close()
	if broker != nil {
		defer broker.Close()
	}

	var sinks []io.Writer
	if toStdout {
		sinks = append(sinks, os.Stdout)
	}
	if strings.TrimSpace(filePath) != "" {
		clean := strings.TrimSpace(filePath)
		if err := os.MkdirAll(filepath.Dir(clean), 0755); err != nil {
			return fmt.Errorf("failed creating stream file directory: %w", err)
		}
		f, err := os.OpenFile(clean, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed opening stream file: %w", err)
		}
		defer f.Close()
		sinks = append(sinks, f)
	}
	if len(sinks) == 0 {
		return fmt.Errorf("no output sink configured")
	}
	out := io.MultiWriter(sinks...)

	logs.Info("Streaming local NATS logs (%s): subject=%s via %s", versionDir, subject, usedURL)

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		fmt.Fprintln(out, logs.FormatMessage(msg.Subject, msg.Data))
	})
	if err != nil {
		return fmt.Errorf("NATS subscribe failed for subject %q: %w", subject, err)
	}

	if err := nc.Flush(); err != nil {
		return fmt.Errorf("NATS flush failed: %w", err)
	}
	if err := nc.LastError(); err != nil {
		return fmt.Errorf("NATS subscription error: %w", err)
	}

	select {}
	return nil
}

func connectLocalNATS(versionDir, natsURL string, forceEmbedded bool) (*nats.Conn, *logs.EmbeddedNATS, string, error) {
	tryConnect := func(url string) (*nats.Conn, error) {
		return nats.Connect(url, nats.Timeout(1200*time.Millisecond))
	}

	if forceEmbedded {
		broker, err := logs.StartEmbeddedNATSOnURL(natsURL)
		if err != nil {
			return nil, nil, "", fmt.Errorf("embedded NATS start failed: %w", err)
		}
		nc, err := tryConnect(broker.URL())
		if err != nil {
			broker.Close()
			return nil, nil, "", fmt.Errorf("embedded NATS connect failed: %w", err)
		}
		logs.Info("Started embedded NATS for local stream (%s): %s", versionDir, broker.URL())
		return nc, broker, broker.URL(), nil
	}

	nc, err := tryConnect(natsURL)
	if err == nil {
		return nc, nil, natsURL, nil
	}
	connectErr := err

	if daemonErr := startNATSDaemon(versionDir, natsURL); daemonErr != nil {
		return nil, nil, "", fmt.Errorf("NATS connection failed (%v) and daemon start failed: %w", connectErr, daemonErr)
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		nc, retryErr := tryConnect(natsURL)
		if retryErr == nil {
			logs.Info("Connected after auto-starting embedded NATS daemon: %s", natsURL)
			return nc, nil, natsURL, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, nil, "", fmt.Errorf("NATS connection failed: %w", connectErr)
}

func startNATSDaemon(versionDir, natsURL string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	if _, err := nats.Connect(natsURL, nats.Timeout(600*time.Millisecond)); err == nil {
		return nil
	}

	logPath := filepath.Join(repoRoot, ".dialtone", "logs", "logs-nats-daemon.log")
	pidPath := natsDaemonPIDPath(repoRoot)
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)
	cmdLine := fmt.Sprintf(
		"nohup %s logs nats-daemon %s --nats-url %s >> %s 2>&1 < /dev/null & echo $! > %s",
		shellQuote(filepath.Join(repoRoot, "dialtone.sh")),
		shellQuote(versionDir),
		shellQuote(natsURL),
		shellQuote(logPath),
		shellQuote(pidPath),
	)
	cmd := exec.Command("bash", "-c", cmdLine)
	cmd.Dir = repoRoot
	return cmd.Run()
}

func natsDaemonPIDPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".dialtone", "logs", "logs-nats-daemon.pid")
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

type pingPongMessage struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
	Seq  int    `json:"seq"`
}

func RunPingPong(versionDir string, args []string) error {
	fs := flag.NewFlagSet("logs pingpong", flag.ContinueOnError)
	id := fs.String("id", "", "participant id")
	peer := fs.String("peer", "", "peer id")
	topic := fs.String("topic", "logs.pingpong", "pingpong subject")
	rounds := fs.Int("rounds", 3, "ping/pong rounds")
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *peer == "" {
		return fmt.Errorf("--id and --peer are required")
	}
	if *rounds < 1 {
		return fmt.Errorf("--rounds must be >= 1")
	}

	nc, broker, usedURL, err := connectLocalNATS(versionDir, *natsURL, false)
	if err != nil {
		return err
	}
	defer nc.Close()
	if broker != nil {
		defer broker.Close()
	}
	logs.Info("[%s] pingpong connect topic=%s url=%s", *id, *topic, usedURL)

	pongCh := make(chan int, 32)
	pingCh := make(chan int, 32)
	_, err = nc.Subscribe(*topic, func(msg *nats.Msg) {
		var m pingPongMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			return
		}
		if m.To != *id || m.From != *peer {
			return
		}
		switch m.Kind {
		case "ping":
			pingCh <- m.Seq
		case "pong":
			pongCh <- m.Seq
		}
	})
	if err != nil {
		return err
	}
	if err := nc.Flush(); err != nil {
		return err
	}

	logger, _ := logs.NewNATSLogger(nc, "logs.pingpong.results")

	isInitiator := *id < *peer
	publish := func(kind string, seq int) error {
		m := pingPongMessage{Kind: kind, From: *id, To: *peer, Seq: seq}
		data, _ := json.Marshal(m)
		logs.Info("[%s] send %s seq=%d to=%s", *id, kind, seq, *peer)
		return nc.Publish(*topic, data)
	}

	if isInitiator {
		for seq := 1; seq <= *rounds; seq++ {
			got := false
			for retry := 0; retry < 8 && !got; retry++ {
				_ = publish("ping", seq)
				select {
				case gotSeq := <-pongCh:
					if gotSeq == seq {
						got = true
					}
				case <-time.After(1 * time.Second):
				}
			}
			if !got {
				return fmt.Errorf("[%s] timeout waiting pong seq=%d", *id, seq)
			}
		}
		logger.Infof("[%s] PINGPONG PASS rounds=%d", *id, *rounds)
		return nil
	}

	for seq := 1; seq <= *rounds; seq++ {
		deadline := time.Now().Add(12 * time.Second)
		seen := false
		for time.Now().Before(deadline) && !seen {
			select {
			case gotSeq := <-pingCh:
				if gotSeq == seq {
					seen = true
				}
			case <-time.After(300 * time.Millisecond):
			}
		}
		if !seen {
			return fmt.Errorf("[%s] timeout waiting ping seq=%d", *id, seq)
		}
		if err := publish("pong", seq); err != nil {
			return err
		}
	}
	logger.Infof("[%s] PINGPONG PASS rounds=%d", *id, *rounds)
	return nil
}

func RunNATSDaemon(versionDir string, args []string) error {
	fs := flag.NewFlagSet("logs nats-daemon", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	u, err := url.Parse(*natsURL)
	if err != nil {
		return fmt.Errorf("invalid nats url: %w", err)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("invalid nats url host")
	}
	broker, err := logs.StartEmbeddedNATSOnURL(*natsURL)
	if err != nil {
		return err
	}
	defer broker.Close()
	logs.Info("[nats-daemon] started (%s): %s", versionDir, broker.URL())
	select {}
}

func RunNATSStart(versionDir string, args []string) error {
	fs := flag.NewFlagSet("logs nats-start", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := nats.Connect(*natsURL, nats.Timeout(600*time.Millisecond)); err == nil {
		fmt.Printf("[INFO] NATS already reachable at %s\n", *natsURL)
		return nil
	}
	if err := startNATSDaemon(versionDir, *natsURL); err != nil {
		return err
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if nc, err := nats.Connect(*natsURL, nats.Timeout(600*time.Millisecond)); err == nil {
			nc.Close()
			fmt.Printf("[INFO] NATS started at %s\n", *natsURL)
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for nats at %s", *natsURL)
}

func RunNATSStatus(versionDir string, args []string) error {
	_ = versionDir
	fs := flag.NewFlagSet("logs nats-status", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	pidPath := natsDaemonPIDPath(repoRoot)
	pidText, _ := os.ReadFile(pidPath)
	pid := strings.TrimSpace(string(pidText))

	if nc, err := nats.Connect(*natsURL, nats.Timeout(600*time.Millisecond)); err == nil {
		nc.Close()
		if pid != "" {
			fmt.Printf("[INFO] NATS is UP at %s (pid=%s)\n", *natsURL, pid)
		} else {
			fmt.Printf("[INFO] NATS is UP at %s\n", *natsURL)
		}
		return nil
	}

	if pid != "" {
		fmt.Printf("[WARN] NATS appears DOWN at %s (stale pid file: %s)\n", *natsURL, pid)
	} else {
		fmt.Printf("[INFO] NATS is DOWN at %s\n", *natsURL)
	}
	return nil
}

func RunNATSStop(versionDir string, args []string) error {
	_ = versionDir
	fs := flag.NewFlagSet("logs nats-stop", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	pidPath := natsDaemonPIDPath(repoRoot)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		fmt.Println("[INFO] No tracked daemon pid file found.")
		return nil
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		_ = os.Remove(pidPath)
		return fmt.Errorf("invalid pid in %s: %s", pidPath, pidStr)
	}

	proc, err := os.FindProcess(pid)
	if err == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}
	_ = os.Remove(pidPath)
	time.Sleep(300 * time.Millisecond)

	if _, err := nats.Connect(*natsURL, nats.Timeout(600*time.Millisecond)); err == nil {
		fmt.Printf("[WARN] NATS still reachable at %s (may be another server instance)\n", *natsURL)
		return nil
	}
	fmt.Printf("[INFO] NATS daemon stop requested (pid=%d)\n", pid)
	return nil
}

func runRemoteLogs(host, port, user, pass string, lines int) {
	logs.Info("Connecting to %s to stream logs...", host)

	client, err := ssh.DialSSH(host, port, user, pass)
	if err != nil {
		logs.Fatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	// We want to tail the log file.
	// Based on the ticket description, the start command redirects output to ~/nats.log
	var cmd string
	if lines > 0 {
		cmd = fmt.Sprintf("tail -n %d ~/nats.log", lines)
		logs.Info("Getting last %d lines from ~/nats.log...", lines)
	} else {
		cmd = "tail -f ~/nats.log"
		logs.Info("Streaming logs from ~/nats.log...")
	}

	// Use RunSSHCommand but we actually want to stream it.
	// RunSSHCommand waits for completion, but tail -f runs forever.
	// So we need a way to stream stdout.

	session, err := client.NewSession()
	if err != nil {
		logs.Fatal("Failed to create SSH session: %v", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Run(cmd); err != nil {
		// Ignore error if it's just a signal kill (which happens when user Ctrl+C)
		// But for -n, it should exit cleanly.
		if lines > 0 {
			logs.Fatal("Command failed: %v", err)
		}
	}
}
