package infra

import (
	"fmt"
	"os/exec"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Run04TwoProcessPingPong(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	natsURL := sc.NATSURL()
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	topic := "logs.pingpong.test"
	repoRoot, err := findRepoRoot()
	if err != nil {
		return testv1.StepRunResult{}, err
	}

	cmdA := exec.Command("./dialtone.sh", "logs", "pingpong", "src_v1",
		"--id", "alpha",
		"--peer", "beta",
		"--topic", topic,
		"--rounds", "3",
		"--nats-url", natsURL,
	)
	cmdA.Dir = repoRoot

	cmdB := exec.Command("./dialtone.sh", "logs", "pingpong", "src_v1",
		"--id", "beta",
		"--peer", "alpha",
		"--topic", topic,
		"--rounds", "3",
		"--nats-url", natsURL,
	)
	cmdB.Dir = repoRoot

	// Verify via "act then wait" with one subscription and both expected messages.
	err = sc.WaitForAllMessagesAfterAction(
		"logs.pingpong.results",
		[]string{"[alpha] PINGPONG PASS", "[beta] PINGPONG PASS"},
		30*time.Second,
		func() error {
			if err := cmdA.Start(); err != nil {
				return fmt.Errorf("start alpha pingpong: %w", err)
			}
			time.Sleep(500 * time.Millisecond)
			if err := cmdB.Start(); err != nil {
				_ = cmdA.Process.Kill()
				return fmt.Errorf("start beta pingpong: %w", err)
			}
			return nil
		},
	)
	if err != nil {
		_ = cmdA.Process.Kill()
		_ = cmdB.Process.Kill()
		return testv1.StepRunResult{}, fmt.Errorf("verification failed: %w", err)
	}

	// Wait for processes to exit
	_ = cmdA.Wait()
	_ = cmdB.Wait()

	return testv1.StepRunResult{Report: "Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic."}, nil
}
