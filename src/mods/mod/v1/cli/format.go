package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func runFormat(args []string) error {
	dir, err := parseFormatArgs(args)
	if err != nil {
		return err
	}

	targetDir := filepath.Clean(dir)
	if targetDir == "." || targetDir == "" {
		targetDir, err = locateModRoot("")
		if err != nil {
			return err
		}
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

	argsOut := append([]string{"-w"}, goFiles...)
	cmd := exec.Command("gofmt", argsOut...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gofmt failed: %w", err)
	}
	return nil
}
