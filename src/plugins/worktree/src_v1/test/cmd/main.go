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
	fmt.Println("Running Worktree Plugin Agent Test...")

	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	name := "test-worktree-agent"
	taskRel := filepath.Join("src", "plugins", "worktree", "src_v1", "agent_test", "task.md")
	worktreePath := filepath.Join(filepath.Dir(repoRoot), name)
	taskInWorktree := filepath.Join(worktreePath, "TASK.md")
	agentTestDir := filepath.Join(worktreePath, "src", "plugins", "worktree", "src_v1", "agent_test")
	testLogPath := filepath.Join(worktreePath, "test.log")

	_ = worktree.Remove(name)
	defer func() {
		fmt.Printf("Cleanup: removing worktree '%s'...\n", name)
		_ = worktree.Remove(name)
	}()

	fmt.Printf("Step 1: add '%s' with task fixture...\n", name)
	if err := worktree.Add(name, taskRel, ""); err != nil {
		fmt.Printf("FAIL: add failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Step 2: start '%s' (launch gemini-cli in tmux)...\n", name)
	if err := worktree.Start(name, ""); err != nil {
		fmt.Printf("FAIL: start failed: %v\n", err)
		os.Exit(1)
	}
	if err := startTmuxLogPipe(name, testLogPath); err != nil {
		fmt.Printf("FAIL: failed to start tmux log piping: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Tmux output is being logged to %s\n", testLogPath)

	deadline := time.Now().Add(3 * time.Minute)

	fmt.Println("Step 3: waiting for agent to set TASK.md status to work...")
	if err := waitForStatus(taskInWorktree, "work", deadline); err != nil {
		fmt.Printf("FAIL: did not observe status: work: %v\n", err)
		printTmuxPane(name)
		fmt.Printf("See log: %s\n", testLogPath)
		os.Exit(1)
	}

	fmt.Println("Step 4: polling agent_test verification (go run test.go calc.go) ...")
	if err := waitForVerificationPass(agentTestDir, deadline); err != nil {
		fmt.Printf("FAIL: verification did not pass: %v\n", err)
		printTmuxPane(name)
		fmt.Printf("See log: %s\n", testLogPath)
		os.Exit(1)
	}

	fmt.Println("Step 5: waiting for TASK.md signature after pass...")
	if err := waitForDoneSignature(taskInWorktree, deadline); err != nil {
		fmt.Printf("FAIL: done signature not found before deadline: %v\n", err)
		printTmuxPane(name)
		fmt.Printf("See log: %s\n", testLogPath)
		os.Exit(1)
	}
	fmt.Println("Done signature detected in TASK.md")

	fmt.Println("PASS: worktree test completed with gemini-cli, verification pass, and task signature.")
	fmt.Printf("Log file: %s\n", testLogPath)
}

func waitForVerificationPass(agentTestDir string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		cmd := exec.Command("go", "run", "test.go", "calc.go")
		cmd.Dir = agentTestDir
		cmd.Env = append(os.Environ(), "GOCACHE=/tmp/gocache")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("timed out at %s", deadline.Format(time.RFC3339))
}

func waitForDoneSignature(taskPath string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		status, err := readTaskStatus(taskPath)
		if err == nil && status == "done" {
			return nil
		}
		if err == nil && status == "fail" {
			return fmt.Errorf("agent reported status: fail")
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for status: done in %s (deadline %s)", taskPath, deadline.Format(time.RFC3339))
}

func waitForStatus(taskPath, want string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		status, err := readTaskStatus(taskPath)
		if err == nil {
			if status == want {
				return nil
			}
			if status == "fail" {
				return fmt.Errorf("agent reported status: fail")
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timed out waiting for status: %s in %s (deadline %s)", want, taskPath, deadline.Format(time.RFC3339))
}

func printTmuxPane(session string) {
	fmt.Printf("Tmux pane snapshot for '%s':\n", session)
	cmd := exec.Command("tmux", "capture-pane", "-pt", session, "-S", "-200")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func startTmuxLogPipe(session, logPath string) error {
	header := fmt.Sprintf("=== worktree test log (%s) ===\n", time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(logPath, []byte(header), 0644); err != nil {
		return err
	}
	cmd := exec.Command("tmux", "pipe-pane", "-o", "-t", session, fmt.Sprintf("cat >> %s", logPath))
	return cmd.Run()
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
