package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/plugins/worktree/src_v1/go/worktree"
)

func main() {
	fmt.Println("Running Worktree Plugin E2E Test (src_v1)...")

	repoRoot, err := findRepoRoot()
	if err != nil {
		fail("repo root", err)
	}

	if err := preflight(repoRoot); err != nil {
		fail("preflight", err)
	}

	name := "test-worktree-e2e"
	taskRel := filepath.Join("src", "plugins", "worktree", "src_v1", "agent_test", "task.md")
	worktreePath := filepath.Join(filepath.Dir(repoRoot), "dialtone_worktree", name)
	taskPath := filepath.Join(worktreePath, "TASK.md")
	logPath := filepath.Join(worktreePath, "tmux.log")

	// Idempotent setup: remove any leftovers from previous runs.
	_ = worktree.Remove(name)
	defer func() {
		fmt.Printf("Cleanup: remove '%s' (first pass)\n", name)
		_ = worktree.Remove(name)
		fmt.Printf("Cleanup: remove '%s' (second pass, idempotency check)\n", name)
		_ = worktree.Remove(name)
	}()

	fmt.Printf("Step 1: add %s\n", name)
	if err := worktree.Add(name, taskRel, ""); err != nil {
		fail("add", err)
	}
	mustPathExists("worktree path", worktreePath)
	mustPathExists("task path", taskPath)

	fmt.Printf("Step 2: start %s\n", name)
	if err := worktree.Start(name, ""); err != nil {
		fail("start", err)
	}
	mustPathExists("tmux log", logPath)

	deadline := time.Now().Add(4 * time.Minute)

	fmt.Println("Step 3: wait for TASK.md status=work")
	if err := waitForStatus(taskPath, "work", deadline); err != nil {
		printDebug(name, taskPath, logPath)
		fail("wait work status", err)
	}

	fmt.Println("Step 4: tmux-logs command")
	if err := worktree.TmuxLogs(name, 10); err != nil {
		fail("tmux-logs", err)
	}

	fmt.Println("Step 5: wait for TASK.md status=done")
	if err := waitForStatus(taskPath, "done", deadline); err != nil {
		printDebug(name, taskPath, logPath)
		fail("wait done status", err)
	}

	fmt.Println("Step 6: verify-done")
	if err := worktree.VerifyDone(name); err != nil {
		printDebug(name, taskPath, logPath)
		fail("verify-done", err)
	}

	fmt.Println("Step 7: remove")
	if err := worktree.Remove(name); err != nil {
		fail("remove", err)
	}

	if _, err := os.Stat(worktreePath); err == nil {
		fail("remove", fmt.Errorf("worktree path still exists: %s", worktreePath))
	}

	fmt.Println("PASS: add -> start -> tmux-logs -> verify-done -> remove")
}

func preflight(repoRoot string) error {
	for _, bin := range []string{"git", "tmux", "gemini", "go"} {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("missing dependency in PATH: %s", bin)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "dialtone.sh")); err != nil {
		return fmt.Errorf("dialtone.sh not found at repo root: %w", err)
	}
	return nil
}

func waitForStatus(taskPath, want string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		status, err := readTaskStatus(taskPath)
		if err == nil {
			if status == want {
				return nil
			}
			if status == "fail" {
				return fmt.Errorf("agent reported status=fail")
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for status=%s", want)
}

func readTaskStatus(taskPath string) (string, error) {
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	inBlock := false
	for _, raw := range lines {
		line := strings.TrimSpace(strings.ToLower(raw))
		if line == "```signature" {
			inBlock = true
			continue
		}
		if inBlock && line == "```" {
			break
		}
		if !inBlock {
			continue
		}
		if strings.HasPrefix(line, "status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "status:")), nil
		}
	}
	return "", fmt.Errorf("signature status not found")
}

func printDebug(name, taskPath, logPath string) {
	fmt.Printf("[debug] TASK.md status block (%s):\n", taskPath)
	if data, err := os.ReadFile(taskPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "status:") || strings.HasPrefix(t, "note:") || strings.HasPrefix(t, "updated_at:") {
				fmt.Println(line)
			}
		}
	}
	fmt.Printf("[debug] tmux log tail (%s):\n", logPath)
	if data, err := os.ReadFile(logPath); err == nil {
		lines := strings.Split(string(data), "\n")
		start := len(lines) - 40
		if start < 0 {
			start = 0
		}
		for _, line := range lines[start:] {
			fmt.Println(line)
		}
	}

	fmt.Printf("[debug] tmux pane tail (%s):\n", name)
	cmd := exec.Command("tmux", "capture-pane", "-pt", name, "-S", "-120")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func mustPathExists(label, path string) {
	if _, err := os.Stat(path); err != nil {
		fail(label, fmt.Errorf("expected path missing: %s (%w)", path, err))
	}
}

func fail(step string, err error) {
	fmt.Printf("FAIL [%s]: %v\n", step, err)
	os.Exit(1)
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}
