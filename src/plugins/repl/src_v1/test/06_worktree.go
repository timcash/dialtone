package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func Run06Worktree(ctx *testCtx) (string, error) {
	// Clean up potential leftovers from previous runs
	worktreePath := filepath.Join(filepath.Dir(ctx.repoRoot), "test-agent")
	_ = os.RemoveAll(worktreePath)
	// Also prune git worktree metadata if needed?
	// If the folder is gone, 'git worktree add' might still complain if git thinks it exists.
	// We should run 'git worktree prune' too.
	exec.Command("git", "-C", ctx.repoRoot, "worktree", "prune").Run()

	// Interactive REPL test for Worktree plugin
	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Close()

	// Wait for prompt
	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// 1. Add Worktree
	// We use README.md as a dummy task file
	if err := ctx.SendInput("worktree add test-agent --task README.md"); err != nil {
		return "", err
	}

	// Wait for confirmation
	// Standard output is suppressed in REPL, so we check logs.
	if err := ctx.WaitForLogEntry("subtone-", "[Worktree] Creating worktree", 10*time.Second); err != nil {
		return "", fmt.Errorf("worktree creation log not found: %w", err)
	}

	// 2. List Worktrees
	if err := ctx.SendInput("worktree list"); err != nil {
		return "", err
	}

	// List output is also suppressed? 
	// Yes, `worktree list` runs in subtone.
	if err := ctx.WaitForLogEntry("subtone-", "Git Worktrees:", 10*time.Second); err != nil {
		return "", fmt.Errorf("worktree list log not found: %w", err)
	}

	// 3. Remove Worktree
	if err := ctx.SendInput("worktree remove test-agent"); err != nil {
		return "", err
	}

	if err := ctx.WaitForLogEntry("subtone-", "[Worktree] Removing worktree", 10*time.Second); err != nil {
		return "", fmt.Errorf("worktree remove log not found: %w", err)
	}

	return "Verified worktree add, list, and remove commands via logs.", nil
}
