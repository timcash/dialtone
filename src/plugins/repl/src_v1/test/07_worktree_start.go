package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func Run07WorktreeStart(ctx *testCtx) (string, error) {
	ctx.SetTimeout(60 * time.Second)

	// Clean up potential leftovers
	worktreeName := "agent-worktree-test"
	worktreePath := filepath.Join(filepath.Dir(ctx.repoRoot), worktreeName)
	_ = os.RemoveAll(worktreePath)
	exec.Command("git", "-C", ctx.repoRoot, "worktree", "prune").Run()
	// Also ensure tmux session is killed if exists (from previous fail)
	// We can't easily kill tmux from here without assuming 'tmux' in path, but worktree remove does it.
	// We'll rely on worktree start failing if session exists, or we can try to remove first via REPL.

	if err := ctx.StartREPL(); err != nil {
		return "", fmt.Errorf("failed to start REPL: %w", err)
	}
	defer ctx.Cleanup()

	if err := ctx.WaitForOutput("USER-1>", 5*time.Second); err != nil {
		return "", err
	}

	// 0. Ensure clean state via REPL
	ctx.SendInput("worktree remove " + worktreeName)
	// Ignore output/error, just best effort.

	// 1. Run worktree start
	// We use the agent_test task file we created.
	taskPath := "src/plugins/worktree/src_v1/agent_test/task.md"
	cmd := fmt.Sprintf("worktree start %s --task %s", worktreeName, taskPath)
	if err := ctx.SendInput(cmd); err != nil {
		return "", err
	}

	// 2. Verify logs
	// We expect:
	// - Worktree creation (Add) -> suppressed in REPL, check log?
	// - Launching Agent -> printed by Start -> check log?
	// - "gemini run" output -> this runs IN the tmux session.
	// Does the tmux session output stream to dev.go?
	// NO. `tmux new-session -d` runs detached. `tmux send-keys` sends input.
	// The output of `gemini run` goes to the tmux pane.
	// dev.go does NOT see it.
	// So we cannot verify "gemini run" output via REPL logs.
	
	// However, `worktree start` itself prints "[Worktree] Launching Gemini Agent..." to stdout.
	// This stdout is captured by `RunSubtone` and logged to file.
	// So we can check the logs for "[Worktree] Launching Gemini Agent".

	if err := ctx.WaitForLogEntry("subtone-", "[Worktree] Launching Gemini Agent", 20*time.Second); err != nil {
		return "", fmt.Errorf("failed to find agent launch log: %w", err)
	}

	// 3. Verify side effects
	// Worktree directory should exist.
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return "", fmt.Errorf("worktree directory not created: %s", worktreePath)
	}
	
	// TASK.md should exist in worktree
	if _, err := os.Stat(filepath.Join(worktreePath, "TASK.md")); os.IsNotExist(err) {
		return "", fmt.Errorf("TASK.md not copied to worktree")
	}

	// 4. Cleanup
	if err := ctx.SendInput("worktree remove " + worktreeName); err != nil {
		return "", err
	}
	if err := ctx.WaitForLogEntry("subtone-", "[Worktree] Removing worktree", 10*time.Second); err != nil {
		return "", fmt.Errorf("cleanup failed: %w", err)
	}

	return "Verified worktree start command (creation + agent launch trigger).", nil
}
