package test

import (
	"fmt"
	"os"
)

func Run01Preflight(ctx *testCtx) (string, error) {
	steps := []struct {
		name string
		run  func(string) error
	}{
		{name: "UI Install", run: Run00Install},
		{name: "Go Format", run: Run01GoFormat},
		{name: "Go Vet", run: Run02GoVet},
		{name: "Go Build", run: Run03GoBuild},
		{name: "UI Build", run: Run06UIBuild},
	}

	for _, step := range steps {
		if err := step.run(ctx.repoRoot); err != nil {
			return "", fmt.Errorf("%s failed: %w", step.name, err)
		}
	}
	return "Preflight checks passed.", nil
}

func Run00Install(repoRoot string) error {
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, "wsl", "src_v3", "install")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run01GoFormat(repoRoot string) error {
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, "wsl", "src_v3", "fmt")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run02GoVet(repoRoot string) error {
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, "wsl", "src_v3", "vet")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run03GoBuild(repoRoot string) error {
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, "wsl", "src_v3", "go-build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run06UIBuild(repoRoot string) error {
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, "wsl", "src_v3", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
