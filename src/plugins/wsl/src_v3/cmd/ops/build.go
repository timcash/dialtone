package ops

import (
	"flag"
	"os"
	"path/filepath"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Build(args ...string) error {
	fs := flag.NewFlagSet("wsl-build", flag.ContinueOnError)
	withUI := fs.Bool("with-ui", false, "Build optional UI assets")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repoRoot, uiDir, err := resolveWSLPaths()
	if err != nil {
		return err
	}

	if *withUI {
		logs.Info(">> [WSL] Building UI: src_v3")
		cmd := getDialtoneCmd(repoRoot)
		cmd.Args = append(cmd.Args, "bun", "src_v1", "exec", "--cwd", uiDir, "run", "build")
		cmd.Dir = repoRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	} else {
		logs.Info(">> [WSL] Skipping UI build; pass --with-ui to compile frontend assets")
	}

	logs.Info(">> [WSL] Building server: src_v3")
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	outPath := filepath.Join(home, ".dialtone", "bin", "wsl-src_v3")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	buildArgs := []string{"go", "src_v1", "exec", "build", "-o", outPath, "./plugins/wsl/src_v3/cmd/server/main.go"}
	buildArgs = append(buildArgs, fs.Args()...)
	buildCmd := getDialtoneCmd(repoRoot)
	buildCmd.Args = append(buildCmd.Args, buildArgs...)
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	return buildCmd.Run()
}
