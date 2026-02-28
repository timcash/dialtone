package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func main() {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		panic(err)
	}
	// Default to src_v1 if no version specified, or use the provided one.
	version := "src_v1"
	args := os.Args[1:]
	if len(args) > 0 && strings.HasPrefix(args[0], "src_v") {
		version = args[0]
		args = args[1:]
	}

	pluginRoot := filepath.Join(rt.SrcRoot, "plugins", "lyra3")
	versionPath := filepath.Join(pluginRoot, version)
	mainGo := filepath.Join(versionPath, "cmd", "main.go")

	cmd := exec.Command("go", append([]string{"run", mainGo}, args...)...)
	cmd.Dir = rt.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
