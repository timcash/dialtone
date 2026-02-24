package multiplayer

import (
	"context"
	"fmt"
	"os/exec"
	"sync/atomic"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-multiplayer-real-hosts-over-ssh",
		Timeout: 180 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := exec.LookPath("sshpass"); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("sshpass required for real host multiplayer test: %w", err)
			}

			hosts, err := resolveHostSpecs()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if len(hosts) < 2 {
				return testv1.StepRunResult{}, fmt.Errorf("need at least 2 hosts for real multiplayer test")
			}

			var stage atomic.Value
			stage.Store("init")
			stopHeartbeat := make(chan struct{})
			go func() {
				t := time.NewTicker(5 * time.Second)
				defer t.Stop()
				for {
					select {
					case <-t.C:
						if v, ok := stage.Load().(string); ok {
							ctx.Infof("multiplayer stage=%s", v)
						}
					case <-stopHeartbeat:
						return
					}
				}
			}()
			defer close(stopHeartbeat)

			rt, err := configv1.ResolveRuntime("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			dialtoneBin := configv1.DialtoneScriptPath(rt.RepoRoot)

			natsURL := ctx.NATSURL()
			advertiseHost := resolveNATSAdvertiseHost("")
			remoteNATSURL, err := ctx.NATSURLForHost(advertiseHost)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			stage.Store("verify-host-connectivity")
			for _, h := range hosts {
				if err := verifyHostCanDialNATS(h, remoteNATSURL); err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("host %s cannot reach %s: %w", h.Name, remoteNATSURL, err)
				}
			}

			stage.Store("start-leader")
			leaderCtx, cancelLeader := context.WithCancel(context.Background())
			defer cancelLeader()
			var leaderCmd *exec.Cmd
			if err := ctx.WaitForMessageAfterAction("repl.room.index", "DIALTONE leader online", 12*time.Second, func() error {
				leaderCmd = exec.CommandContext(
					leaderCtx,
					dialtoneBin,
					"repl", "src_v1", "leader",
					"--embedded-nats=false",
					"--nats-url", natsURL,
					"--room", "index",
					"--hostname", "DIALTONE-SERVER",
				)
				leaderCmd.Dir = rt.RepoRoot
				return leaderCmd.Start()
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("leader did not publish ready frame: %w", err)
			}
			defer func() {
				cancelLeader()
				if leaderCmd != nil && leaderCmd.Process != nil {
					_ = leaderCmd.Process.Kill()
				}
			}()

			stage.Store("run-remote-clients")
			for i, h := range hosts {
				target := hosts[(i+1)%len(hosts)].Name
				token := fmt.Sprintf("from-%s-%d", h.Name, i+1)
				patterns := []string{
					fmt.Sprintf("\"type\":\"join\",\"from\":\"%s\"", h.Name),
					fmt.Sprintf("\"type\":\"input\",\"from\":\"%s\"", h.Name),
					"/go src_v1 version",
					"Subtone for go src_v1 exited with code 0.",
					fmt.Sprintf("/@%s echo %s", target, token),
					"\"command\":\"run_host_subtone\"",
					fmt.Sprintf("\"target\":\"%s\"", target),
					fmt.Sprintf("echo %s", token),
					fmt.Sprintf("Subtone on %s exited with code 0.", target),
				}
				err := ctx.WaitForAllMessagesAfterAction("repl.>", patterns, 35*time.Second, func() error {
					return runRemoteJoinScript(h, remoteNATSURL, target, token)
				})
				if err != nil {
					return testv1.StepRunResult{}, fmt.Errorf("remote host %s validation failed: %w", h.Name, err)
				}
			}

			return testv1.StepRunResult{
				Report: fmt.Sprintf("real multiplayer verified across %d hosts; each host executed one / command and one @target command via leader dispatch", len(hosts)),
			}, nil
		},
	})
}
