package infra

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Run04TwoProcessPingPong(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	natsURL := sc.NATSURL()
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	topic := "logs.pingpong.test"
	paths, err := logs.ResolvePaths("", "src_v1")
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}

	var alphaOut bytes.Buffer
	var betaOut bytes.Buffer

	cmdA := exec.Command(goBin, "run", "./plugins/logs/scaffold/main.go",
		"src_v1", "pingpong",
		"--id", "alpha",
		"--peer", "beta",
		"--topic", topic,
		"--rounds", "3",
		"--nats-url", natsURL,
	)
	cmdA.Dir = paths.Runtime.SrcRoot
	cmdA.Env = os.Environ()
	cmdA.Stdout = &alphaOut
	cmdA.Stderr = &alphaOut

	cmdB := exec.Command(goBin, "run", "./plugins/logs/scaffold/main.go",
		"src_v1", "pingpong",
		"--id", "beta",
		"--peer", "alpha",
		"--topic", topic,
		"--rounds", "3",
		"--nats-url", natsURL,
	)
	cmdB.Dir = paths.Runtime.SrcRoot
	cmdB.Env = os.Environ()
	cmdB.Stdout = &betaOut
	cmdB.Stderr = &betaOut

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
		return testv1.StepRunResult{}, fmt.Errorf("verification failed: %w%s", err, formatPingPongOutputs(alphaOut.String(), betaOut.String()))
	}

	// Wait for processes to exit
	if waitErr := cmdA.Wait(); waitErr != nil {
		return testv1.StepRunResult{}, fmt.Errorf("alpha pingpong failed: %w%s", waitErr, formatPingPongOutputs(alphaOut.String(), betaOut.String()))
	}
	if waitErr := cmdB.Wait(); waitErr != nil {
		return testv1.StepRunResult{}, fmt.Errorf("beta pingpong failed: %w%s", waitErr, formatPingPongOutputs(alphaOut.String(), betaOut.String()))
	}

	return testv1.StepRunResult{Report: "Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic."}, nil
}

func formatPingPongOutputs(alpha, beta string) string {
	alpha = strings.TrimSpace(alpha)
	beta = strings.TrimSpace(beta)
	if alpha == "" && beta == "" {
		return ""
	}
	var b strings.Builder
	if alpha != "" {
		b.WriteString("\nalpha output:\n")
		b.WriteString(alpha)
	}
	if beta != "" {
		b.WriteString("\nbeta output:\n")
		b.WriteString(beta)
	}
	return b.String()
}
