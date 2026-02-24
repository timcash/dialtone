package ops

import (
	"os"
	"path/filepath"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Build(args ...string) error {
	repoRoot, uiDir, err := resolveWSLPaths()
	if err != nil {
		return err
	}

	logs.Info(">> [WSL] Building UI: src_v3")
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, "bun", "src_v1", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	logs.Info(">> [WSL] Building server: src_v3")
	buildArgs := []string{"go", "src_v1", "exec", "build", "-o", filepath.Join(repoRoot, ".dialtone", "bin", "wsl-src_v3"), "./plugins/wsl/src_v3/cmd/server/main.go"}
	buildArgs = append(buildArgs, args...)
	buildCmd := getDialtoneCmd(repoRoot)
	buildCmd.Args = append(buildCmd.Args, buildArgs...)
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	return buildCmd.Run()
}
