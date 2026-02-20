package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/worktree/src_v1/go/worktree"
)

const (
	modelName = "gemini-2.5-flash"
	// Estimated Gemini 2.5 Flash pricing (USD per 1M tokens).
	// Update if pricing changes.
	priceInputPerMTok  = 0.30
	priceOutputPerMTok = 2.50
)

type usageStats struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type runResult struct {
	Usage usageStats
}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Printf("FAIL [repo root]: %v\n", err)
		os.Exit(1)
	}

	resultLabel := "FAIL"
	reason := "unknown"
	res := runResult{}
	startAt := time.Now().UTC()

	res, err = runE2E(repoRoot)
	if err == nil {
		resultLabel = "PASS"
		reason = "ok"
	} else {
		reason = err.Error()
	}

	cost := estimateCostUSD(res.Usage)
	fmt.Printf("[Worktree] %s token summary model=%s input=%d output=%d total=%d est_cost_usd=%.6f\n",
		resultLabel, modelName, res.Usage.InputTokens, res.Usage.OutputTokens, res.Usage.TotalTokens, cost)
	fmt.Printf("[COST] model=%s input_tokens=%d output_tokens=%d total_tokens=%d est_cost_usd=%.6f\n",
		modelName, res.Usage.InputTokens, res.Usage.OutputTokens, res.Usage.TotalTokens, cost)

	if appendErr := appendTestRecord(repoRoot, startAt, resultLabel, reason, res.Usage, cost); appendErr != nil {
		fmt.Printf("WARN: failed to append test record: %v\n", appendErr)
	}

	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("PASS: add -> start -> tmux-logs -> verify-done -> remove")
}

func runE2E(repoRoot string) (runResult, error) {
	fmt.Println("Running Worktree Plugin E2E Test (src_v1)...")

	if err := preflight(repoRoot); err != nil {
		return runResult{}, fmt.Errorf("preflight: %w", err)
	}

	name := fmt.Sprintf("test-worktree-e2e-%d", time.Now().Unix())
	taskRel := filepath.Join("src", "plugins", "worktree", "src_v1", "agent_test", "task.md")
	worktreePath := filepath.Join(filepath.Dir(repoRoot), "dialtone_worktree", name)
	taskPath := filepath.Join(worktreePath, "TASK.md")
	logPath := filepath.Join(worktreePath, "tmux.log")

	_ = worktree.Remove(name)
	defer func() {
		fmt.Printf("Cleanup: remove '%s' (first pass)\n", name)
		_ = worktree.Remove(name)
		fmt.Printf("Cleanup: remove '%s' (second pass, idempotency check)\n", name)
		_ = worktree.Remove(name)
		_ = exec.Command("git", "-C", repoRoot, "branch", "-D", name).Run()
	}()

	fmt.Printf("Step 1: add %s\n", name)
	if err := worktree.Add(name, taskRel, ""); err != nil {
		return runResult{}, fmt.Errorf("add: %w", err)
	}
	if err := mustPathExists(worktreePath); err != nil {
		return runResult{}, fmt.Errorf("worktree path: %w", err)
	}
	if err := mustPathExists(taskPath); err != nil {
		return runResult{}, fmt.Errorf("task path: %w", err)
	}

	fmt.Printf("Step 2: start %s\n", name)
	if err := worktree.Start(name, ""); err != nil {
		return runResult{}, fmt.Errorf("start: %w", err)
	}
	if err := mustPathExists(logPath); err != nil {
		return runResult{}, fmt.Errorf("tmux log: %w", err)
	}

	deadline := time.Now().Add(4 * time.Minute)

	fmt.Println("Step 3: wait for TASK.md status=work")
	if err := waitForStatus(taskPath, "work", deadline); err != nil {
		printDebug(name, taskPath, logPath)
		return runResult{}, fmt.Errorf("wait work status: %w", err)
	}

	fmt.Println("Step 4: tmux-logs command")
	if err := worktree.TmuxLogs(name, 10); err != nil {
		return runResult{}, fmt.Errorf("tmux-logs: %w", err)
	}

	fmt.Println("Step 5: wait for TASK.md status=done")
	if err := waitForStatus(taskPath, "done", deadline); err != nil {
		printDebug(name, taskPath, logPath)
		return runResult{}, fmt.Errorf("wait done status: %w", err)
	}

	fmt.Println("Step 6: verify-done")
	if err := worktree.VerifyDone(name); err != nil {
		printDebug(name, taskPath, logPath)
		return runResult{}, fmt.Errorf("verify-done: %w", err)
	}

	_ = waitForUsageStats(logPath, 20*time.Second)
	usage := parseUsageStats(logPath)

	fmt.Println("Step 7: remove")
	if err := worktree.Remove(name); err != nil {
		return runResult{}, fmt.Errorf("remove: %w", err)
	}
	if _, err := os.Stat(worktreePath); err == nil {
		return runResult{}, fmt.Errorf("remove: worktree path still exists: %s", worktreePath)
	}

	return runResult{Usage: usage}, nil
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

func parseUsageStats(logPath string) usageStats {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return usageStats{}
	}
	text := string(data)
	input := lastIntMatch(text, regexp.MustCompile(`"input_tokens"\s*:\s*([0-9]+)`))
	output := lastIntMatch(text, regexp.MustCompile(`"output_tokens"\s*:\s*([0-9]+)`))
	total := lastIntMatch(text, regexp.MustCompile(`"total_tokens"\s*:\s*([0-9]+)`))
	if total == 0 {
		total = input + output
	}
	return usageStats{InputTokens: input, OutputTokens: output, TotalTokens: total}
}

func waitForUsageStats(logPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(logPath)
		if err == nil {
			text := string(data)
			if strings.Contains(text, `"input_tokens"`) || strings.Contains(text, `"output_tokens"`) || strings.Contains(text, `"total_tokens"`) {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for usage stats in %s", logPath)
}

func lastIntMatch(text string, re *regexp.Regexp) int {
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return 0
	}
	v, err := strconv.Atoi(matches[len(matches)-1][1])
	if err != nil {
		return 0
	}
	return v
}

func estimateCostUSD(u usageStats) float64 {
	inCost := (float64(u.InputTokens) / 1_000_000.0) * priceInputPerMTok
	outCost := (float64(u.OutputTokens) / 1_000_000.0) * priceOutputPerMTok
	return inCost + outCost
}

func appendTestRecord(repoRoot string, ts time.Time, result, reason string, u usageStats, cost float64) error {
	readmePath := filepath.Join(repoRoot, "src", "plugins", "worktree", "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("- %s | result=%s | model=%s | input=%d output=%d total=%d | estimated_cost_usd=%.6f | note=%s",
		ts.Format(time.RFC3339), result, modelName, u.InputTokens, u.OutputTokens, u.TotalTokens, cost, sanitizeNote(reason))
	content := string(data)
	if strings.Contains(content, "\n## Test\n") {
		content = strings.TrimRight(content, "\n") + "\n" + line + "\n"
	} else {
		content = strings.TrimRight(content, "\n") + "\n\n## Test\n" + line + "\n"
	}
	return os.WriteFile(readmePath, []byte(content), 0644)
}

func sanitizeNote(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	if len(s) > 120 {
		return s[:120]
	}
	return s
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

func mustPathExists(path string) error {
	_, err := os.Stat(path)
	return err
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
