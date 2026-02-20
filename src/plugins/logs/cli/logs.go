package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/nats-io/nats.go"
)

// RunLogs handles the logs command
func RunLogs(versionDir string, args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	remote := fs.Bool("remote", false, "Stream logs from remote robot")
	streamTopic := fs.String("stream", "logs.>", "NATS subject to subscribe to (supports wildcards)")
	topic := fs.String("topic", "", "Alias for --stream; use '*' for all logs")
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	embedded := fs.Bool("embedded", false, "Start embedded NATS server for local stream")
	stdout := fs.Bool("stdout", true, "Print streamed messages to stdout")
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	lines := fs.Int("lines", 0, "Number of lines to show (default: stream logs)")
	showHelp := fs.Bool("help", false, "Show help for logs command")

	fs.Usage = func() {
		fmt.Println("Usage: ./dialtone.sh logs stream [src_vN] [options]")
		fmt.Println()
		fmt.Println("Stream logs from Dialtone.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --topic       Topic alias for --stream (use '*' for all logs)")
		fmt.Println("  --stream      NATS subject to subscribe to (default: logs.>)")
		fmt.Println("  --nats-url    NATS server URL (default: nats://127.0.0.1:4222)")
		fmt.Println("  --embedded    Start embedded NATS server for local stream")
		fmt.Println("  --stdout      Print streamed messages (default: true)")
		fmt.Println("  --remote      Stream logs from remote robot")
		fmt.Println("  --lines       Number of lines to show (if set, does not stream)")
		fmt.Println("  --host        SSH host (user@host) [env: ROBOT_HOST]")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username [env: ROBOT_USER]")
		fmt.Println("  --pass        SSH password [env: ROBOT_PASSWORD]")
		fmt.Println("  --help        Show this help message")
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
		if !*stdout {
			logs.Fatal("Error: local stream requires --stdout=true")
		}
		if err := runLocalNATSStream(versionDir, *natsURL, resolveTopic(*topic, *streamTopic), *embedded); err != nil {
			logs.Fatal("%v", err)
		}
	}
}

func resolveTopic(topic, streamTopic string) string {
	t := strings.TrimSpace(topic)
	if t == "" {
		t = strings.TrimSpace(streamTopic)
	}
	if t == "" {
		return "logs.>"
	}
	if t == "*" || strings.EqualFold(t, "all") {
		return "logs.>"
	}
	return t
}

func runLocalNATSStream(versionDir, natsURL, subject string, embedded bool) error {
	var (
		nc     *nats.Conn
		broker *logs.EmbeddedNATS
		err    error
	)
	if embedded {
		broker, err = logs.StartEmbeddedNATS()
		if err != nil {
			return fmt.Errorf("embedded NATS start failed: %w", err)
		}
		defer broker.Close()
		natsURL = broker.URL()
		logs.Info("Started embedded NATS for local stream (%s): %s", versionDir, natsURL)
	}

	logs.Info("Streaming local NATS logs (%s): subject=%s via %s", versionDir, subject, natsURL)

	nc, err = nats.Connect(natsURL)
	if err != nil {
		return fmt.Errorf("NATS connection failed: %w", err)
	}
	defer nc.Close()

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		fmt.Println(string(msg.Data))
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
