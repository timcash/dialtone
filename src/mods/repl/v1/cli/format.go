package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func runFormat(args []string) error {
	dir, err := parseFormatArgs(args)
	if err != nil {
		return err
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	modRoot, err := locateModRoot(repoRoot)
	if err != nil {
		return err
	}

	targetDir := filepath.Clean(dir)
	if targetDir == "." || targetDir == "" {
		targetDir = modRoot
	}
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(repoRoot, targetDir)
	}

	goFiles := []string{}
	if err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			goFiles = append(goFiles, path)
		}
		return nil
	}); err != nil {
		return err
	}
	if len(goFiles) == 0 {
		return nil
	}

	argsOut := append([]string{"gofmt", "-w"}, goFiles...)
	cmd := nixDevelopCommand(repoRoot, argsOut...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("repl format failed: %w", err)
	}
	return nil
}
