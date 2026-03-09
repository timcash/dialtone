package repl

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	replv1 "dialtone/dev/plugins/repl/src_v1/go/repl"
	"github.com/nats-io/nats.go"
)

const (
	defaultNATSURL = "nats://127.0.0.1:4222"
	defaultRoom    = "index"
	commandSubject = "repl.cmd"
)

type busFrame struct {
	Type      string `json:"type"`
	From      string `json:"from,omitempty"`
	Room      string `json:"room,omitempty"`
	Version   string `json:"version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

func Run(args []string) error {
	fs := flag.NewFlagSet("repl-v3-run", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared room name")
	name := fs.String("name", replv1.DefaultPromptName(), "Prompt name for this client")
	isTest := fs.Bool("test", false, "Run REPL v3 end-to-end tests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *isTest {
		return RunTest(fs.Args())
	}
	if err := EnsureLeaderRunning(strings.TrimSpace(*natsURL), strings.TrimSpace(*room)); err != nil {
		return err
	}
	joinArgs := []string{
		"--nats-url", strings.TrimSpace(*natsURL),
		"--room", strings.TrimSpace(*room),
		"--name", strings.TrimSpace(*name),
	}
	return replv1.RunJoin(joinArgs)
}

func RunLeader(args []string) error {
	return replv1.RunLeader(args)
}

func RunJoin(args []string) error {
	return replv1.RunJoin(args)
}

func RunStatus(args []string) error {
	return replv1.RunStatus(args)
}

func RunService(args []string) error {
	return replv1.RunService(args)
}

func Inject(args []string) error {
	fs := flag.NewFlagSet("repl-v3-inject", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared room name")
	user := fs.String("user", "llm-codex", "Logical user name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if command == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 inject --user <name> [--nats-url URL] [--room ROOM] <command>")
	}
	return InjectCommand(strings.TrimSpace(*natsURL), strings.TrimSpace(*room), strings.TrimSpace(*user), command)
}

func InjectCommand(natsURL, room, user, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command is required")
	}
	if strings.TrimSpace(natsURL) == "" {
		natsURL = defaultNATSURL
	}
	if strings.TrimSpace(room) == "" {
		room = defaultRoom
	}
	if strings.TrimSpace(user) == "" {
		user = "llm-codex"
	}
	command = strings.TrimPrefix(strings.TrimSpace(command), "/")
	if err := EnsureLeaderRunning(natsURL, room); err != nil {
		return err
	}
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	frame := busFrame{
		Type:      "command",
		From:      strings.TrimSpace(user),
		Room:      strings.TrimSpace(room),
		Version:   "src_v3",
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Message:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := nc.Publish(commandSubject, raw); err != nil {
		return err
	}
	return nc.FlushTimeout(1500 * time.Millisecond)
}

func RunTest(args []string) error {
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmdArgs := []string{"run", "./plugins/repl/src_v3/test/cmd/main.go"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "DIALTONE_REPO_ROOT="+repoRoot, "DIALTONE_SRC_ROOT="+srcRoot)
	return cmd.Run()
}

func resolveRoots() (repoRoot, srcRoot string, err error) {
	cwd, e := os.Getwd()
	if e != nil {
		return "", "", e
	}
	abs, _ := filepath.Abs(cwd)
	if filepath.Base(abs) == "src" {
		return filepath.Dir(abs), abs, nil
	}
	repoGuess := abs
	if _, statErr := os.Stat(filepath.Join(repoGuess, "src")); statErr != nil {
		return "", "", fmt.Errorf("unable to resolve repo root from %s", abs)
	}
	return repoGuess, filepath.Join(repoGuess, "src"), nil
}

func EnsureLeaderRunning(clientNATSURL, room string) error {
	clientNATSURL = strings.TrimSpace(clientNATSURL)
	if clientNATSURL == "" {
		clientNATSURL = defaultNATSURL
	}
	if strings.TrimSpace(room) == "" {
		room = defaultRoom
	}
	if endpointReachable(clientNATSURL, 700*time.Millisecond) {
		return nil
	}
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	listenURL := listenURLFromClientURL(clientNATSURL)
	cmd := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--room", room,
		"--hostname", "DIALTONE-SERVER",
	)
	cmd.Dir = srcRoot
	cmd.Env = append(os.Environ(),
		"DIALTONE_REPO_ROOT="+repoRoot,
		"DIALTONE_SRC_ROOT="+srcRoot,
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if endpointReachable(clientNATSURL, 600*time.Millisecond) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("repl v3 leader did not start nats endpoint at %s", clientNATSURL)
}

func endpointReachable(natsURL string, timeout time.Duration) bool {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return false
	}
	host := strings.TrimSpace(u.Hostname())
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func listenURLFromClientURL(clientURL string) string {
	u, err := url.Parse(strings.TrimSpace(clientURL))
	if err != nil {
		return "nats://0.0.0.0:4222"
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	return "nats://0.0.0.0:" + port
}
