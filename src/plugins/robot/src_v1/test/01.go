package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Run01Preflight() error {
	steps := []struct {
		name string
		run  func() error
	}{
		{name: "UI Install", run: Run00Install},
		{name: "Go Format", run: Run01GoFormat},
		{name: "Go Vet", run: Run02GoVet},
		{name: "Go Build", run: Run03GoBuild},
		{name: "UI Lint", run: Run04UILint},
		{name: "UI Format", run: Run05UIFormat},
		{name: "UI Build", run: Run06UIBuild},
	}

	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}
	return nil
}

func Run00Install() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "install", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Run01GoFormat() error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "robot", "fmt", "src_v1")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
