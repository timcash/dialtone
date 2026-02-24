package multiplayerrobot

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	repl "dialtone/dev/plugins/repl/src_v1/go/repl"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

type observedFrame struct {
	Subject string
	Frame   repl.BusFrame
}

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-multiplayer-live-with-robot-over-ssh",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			robotHost := strings.TrimSpace(os.Getenv("ROBOT_HOST"))
			robotUser := strings.TrimSpace(os.Getenv("ROBOT_USER"))
			robotPass := os.Getenv("ROBOT_PASSWORD")
			if robotHost == "" || robotUser == "" || strings.TrimSpace(robotPass) == "" {
				return testv1.StepRunResult{
					Report: "skipped live robot multiplayer test (ROBOT_HOST/ROBOT_USER/ROBOT_PASSWORD not set)",
				}, nil
			}
			if _, err := exec.LookPath("sshpass"); err != nil {
				return testv1.StepRunResult{
					Report: "skipped live robot multiplayer test (sshpass unavailable)",
				}, nil
			}

			paths, err := repl.ResolvePaths("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			dialtoneBin := filepath.Join(paths.Runtime.RepoRoot, "dialtone.sh")
			natsURL := strings.TrimSpace(ctx.NATSURL())
			if natsURL == "" {
				natsURL = "nats://127.0.0.1:4222"
			}
			robotNATSURL, err := rewriteNATSURLForRobot(natsURL, robotHost)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			nc := ctx.NATSConn()
			if nc == nil {
				return testv1.StepRunResult{}, fmt.Errorf("NATS connection unavailable in test context")
			}

			var (
				obsMu sync.Mutex
				obs   []observedFrame
			)
			sub, err := nc.Subscribe("repl.>", func(msg *nats.Msg) {
				var frame repl.BusFrame
				if json.Unmarshal(msg.Data, &frame) != nil {
					return
				}
				obsMu.Lock()
				obs = append(obs, observedFrame{Subject: msg.Subject, Frame: frame})
				obsMu.Unlock()
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer sub.Unsubscribe()
			_ = nc.Flush()

			leaderCtx, cancelLeader := context.WithCancel(context.Background())
			defer cancelLeader()
			leaderCmd := exec.CommandContext(
				leaderCtx,
				dialtoneBin,
				"repl", "src_v1", "leader",
				"--embedded-nats=false",
				"--nats-url", natsURL,
				"--room", "index",
				"--hostname", "DIALTONE-SERVER",
			)
			leaderCmd.Dir = paths.Runtime.RepoRoot
			if err := leaderCmd.Start(); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer func() {
				cancelLeader()
				if leaderCmd.Process != nil {
					_ = leaderCmd.Process.Kill()
				}
			}()

			if err := waitFor(12*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "server"
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("leader did not publish ready frame: %w", err)
			}

			remoteRepo := strings.TrimSpace(os.Getenv("ROBOT_REPL_REPO"))
			if remoteRepo == "" {
				remoteRepo = "/home/" + robotUser + "/dialtone"
			}
			remoteScript := fmt.Sprintf(
				"set -e; cd %s; printf 'hello-from-robot\\n/quit\\n' | ./dialtone.sh repl src_v1 join --nats-url %s --name robot-client --room index",
				shellQuote(remoteRepo),
				shellQuote(robotNATSURL),
			)
			sshTarget := fmt.Sprintf("%s@%s", robotUser, robotHost)
			sshCmd := exec.Command(
				"sshpass", "-p", robotPass,
				"ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=12",
				sshTarget,
				remoteScript,
			)
			sshOut, sshErr := sshCmd.CombinedOutput()
			if sshErr != nil {
				return testv1.StepRunResult{}, fmt.Errorf("robot join command failed: %w\n%s", sshErr, strings.TrimSpace(string(sshOut)))
			}

			if err := waitFor(15*time.Second, func() bool {
				joined := hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "join" && of.Frame.From == "robot-client"
				})
				chatted := hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "chat" && of.Frame.From == "robot-client" && strings.Contains(of.Frame.Message, "hello-from-robot")
				})
				left := hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "left" && of.Frame.From == "robot-client"
				})
				return joined && chatted && left
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("robot join/chat/left frames not fully observed: %w", err)
			}

			return testv1.StepRunResult{
				Report: fmt.Sprintf("live multiplayer verified with robot client over SSH (nats=%s)", robotNATSURL),
			}, nil
		},
	})
}

func waitFor(timeout time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("condition not satisfied within %s", timeout)
}

func hasFrame(mu *sync.Mutex, frames *[]observedFrame, predicate func(observedFrame) bool) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, f := range *frames {
		if predicate(f) {
			return true
		}
	}
	return false
}

func rewriteNATSURLForRobot(natsURL, robotHost string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return "", fmt.Errorf("invalid NATS URL %q: %w", natsURL, err)
	}
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" {
		return u.String(), nil
	}
	port := u.Port()
	if port == "" {
		port = "4222"
	}
	localIP, err := outboundIPFor(robotHost)
	if err != nil {
		return "", fmt.Errorf("failed to resolve local IP for robot route: %w", err)
	}
	u.Host = net.JoinHostPort(localIP, port)
	return u.String(), nil
}

func outboundIPFor(remoteHost string) (string, error) {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(remoteHost, "9"), 3*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return "", fmt.Errorf("unexpected local address type")
	}
	return addr.IP.String(), nil
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'"
}
