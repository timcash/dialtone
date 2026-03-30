package main

import (
	"os"
	"os/exec"
	"strings"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	start := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT"))
	if start == "" {
		start = ".."
	}

	paths, err := cloudflarev1.ResolvePaths(start, "src_v1")
	if err != nil {
		logs.Error("cloudflare src_v1 test init failed: %v", err)
		os.Exit(1)
	}

	goBin := strings.TrimSpace(paths.Runtime.GoBin)
	if goBin == "" {
		goBin = "go"
	}

	args := append([]string{"run", "./plugins/cloudflare/src_v1/test"}, os.Args[1:]...)
	cmd := exec.Command(goBin, args...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		logs.Error("cloudflare src_v1 tests failed: %v", err)
		os.Exit(1)
	}
}
