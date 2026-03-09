package bootinject

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

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "bootstrap-leader-and-inject-command",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
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
			listenURL := listenURLFromClientURL(natsURL)

			leader := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "leader",
				"--embedded-nats",
				"--nats-url", listenURL,
				"--room", "index",
				"--hostname", "DIALTONE-SERVER",
			)
			leader.Dir = srcRoot
			if err := leader.Start(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForEndpoint(natsURL, 10*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer stopCmd(leader)
			nc, err := nats.Connect(natsURL, nats.Timeout(1200*time.Millisecond))
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()
			msgCh := make(chan string, 512)
			sub, err := nc.Subscribe("repl.>", func(m *nats.Msg) {
				msgCh <- string(m.Data)
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer sub.Unsubscribe()
			if err := nc.Flush(); err != nil {
				return testv1.StepRunResult{}, err
			}
			probe := map[string]string{
				"type":    "probe",
				"from":    "repl-src-v3-test",
				"room":    "index",
				"message": "probe",
			}
			rawProbe, err := json.Marshal(probe)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := nc.Publish("repl.cmd", rawProbe); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := nc.Flush(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForPatterns(msgCh, 15*time.Second, []string{"DIALTONE leader active"}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("leader did not announce ready: %w", err)
			}

			join := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "join",
				"--nats-url", natsURL,
				"--name", "local-human",
				"--room", "index",
			)
			join.Dir = srcRoot
			joinIn, err := join.StdinPipe()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := join.Start(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForPatterns(msgCh, 12*time.Second, []string{`"type":"join","from":"local-human"`}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("join client did not connect: %w", err)
			}
			defer func() {
				_, _ = io.WriteString(joinIn, "quit\n")
				_ = joinIn.Close()
				stopCmd(join)
			}()

			bootstrapPatterns := []string{
				`"type":"command","from":"llm-codex"`,
				`"type":"input","from":"llm-codex"`,
				`/repl src_v3 bootstrap --apply`,
				`Request received. Spawning subtone for repl src_v3`,
				`mesh host wsl`,
				`Subtone for repl src_v3 exited with code 0.`,
			}
			inject := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "inject",
				"--nats-url", natsURL,
				"--room", "index",
				"--user", "llm-codex",
				"repl", "src_v3", "bootstrap", "--apply",
				"--wsl-host", "wsl.shad-artichoke.ts.net",
				"--wsl-user", "user",
			)
			inject.Dir = srcRoot
			if err := inject.Run(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForPatterns(msgCh, 35*time.Second, bootstrapPatterns); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("bootstrap injection did not complete: %w", err)
			}

			waitPatterns := []string{
				`"type":"command","from":"llm-codex"`,
				`"type":"input","from":"llm-codex"`,
				`/go src_v1 version`,
				`Request received. Spawning subtone for go src_v1`,
				`Subtone for go src_v1 exited with code 0.`,
			}
			inject = exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "inject",
				"--nats-url", natsURL,
				"--room", "index",
				"--user", "llm-codex",
				"go", "src_v1", "version",
			)
			inject.Dir = srcRoot
			if err := inject.Run(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForPatterns(msgCh, 35*time.Second, waitPatterns); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("injected command did not complete: %w", err)
			}

			ctx.TestPassf("leader boot, NATS command injection, and completion wait all succeeded")
			return testv1.StepRunResult{
				Report: "Started leader, joined from a second REPL client, injected bootstrap command to add wsl host, then injected go version command and observed completion over NATS.",
			}, nil
		},
	})
}

func stopCmd(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func waitForPatterns(msgCh <-chan string, timeout time.Duration, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	deadline := time.Now().Add(timeout)
	for len(seen) < len(patterns) && time.Now().Before(deadline) {
		select {
		case msg := <-msgCh:
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
		conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 600*time.Millisecond)
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
