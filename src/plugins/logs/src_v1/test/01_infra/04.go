package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func Run04TwoProcessPingPong(ctx *testCtx) (string, error) {
	if err := ctx.ensureBroker(); err != nil {
		return "", err
	}
	nc := ctx.broker.Conn()
	natsURL := ctx.broker.URL()
	topic := "logs.pingpong.test"

	// Pre-subscribe to results
	resSub, err := nc.SubscribeSync("logs.pingpong.results")
	if err != nil {
		return "", err
	}
	defer resSub.Unsubscribe()

	cmdA := exec.Command("./dialtone.sh", "logs", "pingpong", "src_v1",
		"--id", "alpha",
		"--peer", "beta",
		"--topic", topic,
		"--rounds", "3",
		"--nats-url", natsURL,
	)
	cmdA.Dir = ctx.repoRoot
	if err := cmdA.Start(); err != nil {
		return "", fmt.Errorf("start alpha pingpong: %w", err)
	}

	time.Sleep(500 * time.Millisecond)

	cmdB := exec.Command("./dialtone.sh", "logs", "pingpong", "src_v1",
		"--id", "beta",
		"--peer", "alpha",
		"--topic", topic,
		"--rounds", "3",
		"--nats-url", natsURL,
	)
	cmdB.Dir = ctx.repoRoot
	if err := cmdB.Start(); err != nil {
		_ = cmdA.Process.Kill()
		return "", fmt.Errorf("start beta pingpong: %w", err)
	}

	// Verify via NATS messages
	alphaPassed := false
	betaPassed := false
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && (!alphaPassed || !betaPassed) {
		msg, err := resSub.NextMsg(time.Until(deadline))
		if err != nil {
			break
		}
		data := string(msg.Data)
		if strings.Contains(data, "[alpha] PINGPONG PASS") {
			alphaPassed = true
		}
		if strings.Contains(data, "[beta] PINGPONG PASS") {
			betaPassed = true
		}
	}

	if !alphaPassed || !betaPassed {
		_ = cmdA.Process.Kill()
		_ = cmdB.Process.Kill()
		return "", fmt.Errorf("verification failed (alpha=%v, beta=%v)", alphaPassed, betaPassed)
	}

	// Wait for processes to exit
	_ = cmdA.Wait()
	_ = cmdB.Wait()

	return "Verified two dialtone logs processes exchanged 3 ping/pong rounds on one topic (verified via NATS).", nil
}
