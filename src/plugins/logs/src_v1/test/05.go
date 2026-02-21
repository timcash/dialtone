package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run05ExamplePluginImport(ctx *testCtx) (string, error) {
	port, err := pickFreePort()
	if err != nil {
		return "", err
	}
	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", port)
	topic := "logs.example.plugin"
	outPath := filepath.Join(ctx.testDir, "example_plugin.log")
	binPath := filepath.Join(ctx.testDir, "example_plugin_bin")

	_ = os.Remove(outPath)
	_ = os.Remove(binPath)

	build := exec.Command("go", "build", "-o", binPath, "./plugins/logs/src_v1/test/example_plugin")
	build.Dir = filepath.Join(ctx.repoRoot, "src")
	var buildOut bytes.Buffer
	build.Stdout = &buildOut
	build.Stderr = &buildOut
	if err := build.Run(); err != nil {
		return "", fmt.Errorf("build example plugin failed: %v\n%s", err, buildOut.String())
	}

	run := exec.Command(binPath,
		"--nats-url", natsURL,
		"--topic", topic,
		"--count", "4",
		"--out", outPath,
	)
	run.Dir = ctx.repoRoot
	var runOut bytes.Buffer
	run.Stdout = &runOut
	run.Stderr = &runOut
	if err := run.Run(); err != nil {
		return "", fmt.Errorf("run example plugin failed: %v\n%s", err, runOut.String())
	}

	text := runOut.String()
	if !strings.Contains(text, "EXAMPLE_PLUGIN PASS") {
		return "", fmt.Errorf("missing PASS marker in output:\n%s", text)
	}
	if !strings.Contains(text, "started_embedded=true") {
		return "", fmt.Errorf("expected auto embedded start in output:\n%s", text)
	}

	for i := 1; i <= 4; i++ {
		needle := fmt.Sprintf("example plugin message %d", i)
		if !fileContains(outPath, needle) {
			return "", fmt.Errorf("missing %q in %s", needle, outPath)
		}
	}

	return "Verified example plugin binary imports logs library, auto-starts embedded NATS when missing, and publishes/listens on topic.", nil
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
