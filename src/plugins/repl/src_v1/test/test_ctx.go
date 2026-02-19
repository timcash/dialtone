package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type testCtx struct {
	repoRoot string
}

func newTestCtx() *testCtx {
	cwd, _ := os.Getwd()
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "dialtone.sh")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			root = cwd
			break
		}
		root = parent
	}
	return &testCtx{repoRoot: root}
}

func (ctx *testCtx) runREPL(input string) (string, error) {
	cmd := exec.Command(filepath.Join(ctx.repoRoot, "dialtone.sh"))
	cmd.Dir = ctx.repoRoot
	
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	
	if err := cmd.Start(); err != nil {
		return "", err
	}
	
	_, _ = io.WriteString(stdin, input)
	_ = stdin.Close()
	
	err = cmd.Wait()
	return stdout.String(), err
}

func (ctx *testCtx) runDirect(args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(ctx.repoRoot, "dialtone.sh"), args...)
	cmd.Dir = ctx.repoRoot
	
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	
	err := cmd.Run()
	return stdout.String(), err
}
