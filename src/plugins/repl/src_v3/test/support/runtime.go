package support

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

type Runtime struct {
	RepoRoot string
	SrcRoot  string
	GoBin    string
	NATSURL  string
	Room     string

	leader *exec.Cmd
	join   *exec.Cmd
	joinIn io.WriteCloser
	nc     *nats.Conn
	msgCh  chan string
}

func NewRuntime(ctx *testv1.StepContext) (*Runtime, error) {
	repoRoot := strings.TrimSpace(ctx.RepoRoot())
	if strings.HasSuffix(filepath.ToSlash(repoRoot), "/src") {
		repoRoot = filepath.Dir(repoRoot)
	}
	srcRoot := filepath.Join(repoRoot, "src")
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	natsURL := strings.TrimSpace(ctx.NATSURL())
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:46222"
	}
	return &Runtime{
		RepoRoot: repoRoot,
		SrcRoot:  srcRoot,
		GoBin:    goBin,
		NATSURL:  natsURL,
		Room:     "index",
		msgCh:    make(chan string, 4096),
	}, nil
}

func (rt *Runtime) StartLeader() error {
	listenURL := listenURLFromClientURL(rt.NATSURL)
	rt.leader = exec.Command(rt.GoBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--room", rt.Room,
		"--hostname", "DIALTONE-SERVER",
	)
	rt.leader.Dir = rt.SrcRoot
	if err := rt.leader.Start(); err != nil {
		return err
	}
	if err := waitForEndpoint(rt.NATSURL, 10*time.Second); err != nil {
		return err
	}
	nc, err := nats.Connect(rt.NATSURL, nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return err
	}
	rt.nc = nc
	sub, err := rt.nc.Subscribe("repl.>", func(m *nats.Msg) {
		select {
		case rt.msgCh <- string(m.Data):
		default:
		}
	})
	if err != nil {
		return err
	}
	_ = sub
	if err := rt.nc.Flush(); err != nil {
		return err
	}
	probe := map[string]string{
		"type":    "probe",
		"from":    "repl-src-v3-test",
		"room":    rt.Room,
		"message": "probe",
	}
	rawProbe, err := json.Marshal(probe)
	if err != nil {
		return err
	}
	if err := rt.nc.Publish("repl.cmd", rawProbe); err != nil {
		return err
	}
	if err := rt.nc.Flush(); err != nil {
		return err
	}
	return rt.WaitForPatterns(12*time.Second, []string{"DIALTONE leader active"})
}

func (rt *Runtime) StartJoin(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "observer"
	}
	rt.join = exec.Command(rt.GoBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "join",
		"--nats-url", rt.NATSURL,
		"--name", name,
		"--room", rt.Room,
	)
	rt.join.Dir = rt.SrcRoot
	in, err := rt.join.StdinPipe()
	if err != nil {
		return err
	}
	rt.joinIn = in
	if err := rt.join.Start(); err != nil {
		return err
	}
	p := fmt.Sprintf(`"type":"join","from":"%s"`, name)
	return rt.WaitForPatterns(12*time.Second, []string{p})
}

func (rt *Runtime) Inject(user string, args ...string) error {
	user = strings.TrimSpace(user)
	if user == "" {
		user = "llm-codex"
	}
	if len(args) == 0 {
		return fmt.Errorf("inject command args are required")
	}
	injectArgs := []string{
		"run", "./plugins/repl/scaffold/main.go", "src_v3", "inject",
		"--nats-url", rt.NATSURL,
		"--room", rt.Room,
		"--user", user,
	}
	injectArgs = append(injectArgs, args...)
	cmd := exec.Command(rt.GoBin, injectArgs...)
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (rt *Runtime) WaitForPatterns(timeout time.Duration, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	deadline := time.Now().Add(timeout)
	for len(seen) < len(patterns) && time.Now().Before(deadline) {
		select {
		case msg := <-rt.msgCh:
			for _, p := range patterns {
				if !seen[p] && strings.Contains(msg, p) {
					seen[p] = true
				}
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	if len(seen) == len(patterns) {
		return nil
	}
	missing := make([]string, 0, len(patterns)-len(seen))
	for _, p := range patterns {
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	return fmt.Errorf("timeout waiting for patterns: %s", strings.Join(missing, ", "))
}

func (rt *Runtime) WaitForAnyPattern(timeout time.Duration, patterns []string) (string, error) {
	if len(patterns) == 0 {
		return "", fmt.Errorf("patterns are required")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case msg := <-rt.msgCh:
			for _, p := range patterns {
				if strings.Contains(msg, p) {
					return p, nil
				}
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("timeout waiting for any pattern: %s", strings.Join(patterns, ", "))
}

func (rt *Runtime) RunDialtone(args ...string) (string, error) {
	cmd := exec.Command("./dialtone.sh", args...)
	cmd.Dir = rt.RepoRoot
	cmd.Env = append(os.Environ(), "DIALTONE_USE_NIX=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (rt *Runtime) Stop() {
	if rt.joinIn != nil {
		_, _ = io.WriteString(rt.joinIn, "quit\n")
		_ = rt.joinIn.Close()
		rt.joinIn = nil
	}
	if rt.join != nil && rt.join.Process != nil {
		_ = rt.join.Process.Kill()
		_, _ = rt.join.Process.Wait()
		rt.join = nil
	}
	if rt.nc != nil {
		rt.nc.Close()
		rt.nc = nil
	}
	if rt.leader != nil && rt.leader.Process != nil {
		_ = rt.leader.Process.Kill()
		_, _ = rt.leader.Process.Wait()
		rt.leader = nil
	}
}

func waitForEndpoint(natsURL string, timeout time.Duration) error {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return err
	}
	host := strings.TrimSpace(u.Hostname())
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 700*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("nats endpoint did not become reachable at %s", natsURL)
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
