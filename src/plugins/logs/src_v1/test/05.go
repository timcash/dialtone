package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func Run05ExamplePluginImport(ctx *testCtx) (string, error) {
	topic := "logs.example.plugin"
	outPath := filepath.Join(ctx.testDir, "example_plugin.log")
	binPath := filepath.Join(ctx.testDir, "example_plugin_bin")

	_ = os.Remove(outPath)
	_ = os.Remove(binPath)

	build := exec.Command("go", "build", "-o", binPath, "./plugins/logs/src_v1/test/05_example_plugin")
	build.Dir = filepath.Join(ctx.repoRoot, "src")
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build example plugin failed: %v\n%s", err, string(out))
	}

	if err := ctx.ensureBroker(); err != nil {
		return "", err
	}
	usedNatsURL := ctx.broker.URL()
	nc := ctx.broker.Conn()
	
	sub, _ := nc.SubscribeSync(topic)
	defer sub.Unsubscribe()

	run := exec.Command(binPath,
		"--nats-url", usedNatsURL,
		"--topic", topic,
		"--count", "4",
		"--out", outPath,
	)
	run.Dir = ctx.repoRoot
	if err := run.Start(); err != nil {
		return "", fmt.Errorf("run example plugin failed: %v", err)
	}

	// Verify via NATS messages
	for i := 1; i <= 4; i++ {
		msg, err := sub.NextMsg(10 * time.Second)
		if err != nil {
			return "", fmt.Errorf("missing message %d in NATS: %v", i, err)
		}
		needle := fmt.Sprintf("example plugin message %d", i)
		if !strings.Contains(string(msg.Data), needle) {
			return "", fmt.Errorf("unexpected message in NATS: %s (wanted %s)", string(msg.Data), needle)
		}
	}

	// Verify the listener still worked (writing from topic to file)
	if err := waitForContains(outPath, "example plugin message 4", 4*time.Second); err != nil {
		return "", fmt.Errorf("listener failed to write to file: %v", err)
	}

	return "Verified example plugin binary imports logs library, and verified via both NATS messages and file listener.", nil
}

func pickFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to resolve tcp addr")
	}
	return addr.Port, nil
}
