package worktree

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Add(name, taskFile, branch string) error {
	// 1. Resolve paths
	// Assume we are in repo root or subfolder. Find root.
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	worktreePath := filepath.Join(filepath.Dir(repoRoot), name)
	
	// 2. Create git worktree
	fmt.Printf("[Worktree] Creating worktree at %s...\n", worktreePath)
	gitArgs := []string{"worktree", "add", worktreePath}
	if branch != "" {
		gitArgs = append(gitArgs, "-b", branch)
	} else {
		// Default to creating a branch with the name if it doesn't exist
		// Check if branch exists
		if !branchExists(name) {
			gitArgs = append(gitArgs, "-b", name)
		} else {
			gitArgs = append(gitArgs, name)
		}
	}
	
	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git worktree failed: %w", err)
	}

	// 3. Setup Task file
	if taskFile != "" {
		src := filepath.Join(repoRoot, taskFile)
		dst := filepath.Join(worktreePath, "TASK.md")
		if err := copyFile(src, dst); err != nil {
			fmt.Printf("[Worktree] Warning: failed to copy task file: %v\n", err)
		}
	}

	// 4. Ensure worktree has full env config and shared dependency environment.
	dialtoneEnv, err := resolveDialtoneEnv(repoRoot)
	if err != nil {
		return err
	}
	if err := syncWorktreeEnv(repoRoot, worktreePath, dialtoneEnv); err != nil {
		return fmt.Errorf("failed to prepare worktree env: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dialtoneEnv, "go", "bin", "go")); err != nil {
		return fmt.Errorf("Go runtime not found at %s; install dependencies first", filepath.Join(dialtoneEnv, "go"))
	}

	// 5. Create Tmux session
	sessionName := name
	fmt.Printf("[Worktree] Starting tmux session '%s'...\n", sessionName)
	tmuxCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath)
	if err := tmuxCmd.Run(); err != nil {
		// If failed, maybe session exists?
		return fmt.Errorf("tmux new-session failed: %w", err)
	}

	return nil
}

func Start(name, prompt string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	worktreePath := filepath.Join(filepath.Dir(repoRoot), name)

	// Start requires a pre-existing worktree created by `worktree add`.
	info, err := os.Stat(worktreePath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("worktree '%s' does not exist at %s; run `worktree add %s --task <file>` first", name, worktreePath, name)
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux is required but not found in PATH")
	}
	if !tmuxSessionExists(name) {
		return fmt.Errorf("tmux session '%s' does not exist; run `worktree add %s --task <file>` first", name, name)
	}

	// Validate local runtime dependency to avoid interactive prompts in tmux.
	dialtoneEnv, err := resolveDialtoneEnv(worktreePath)
	if err != nil {
		return err
	}
	goRuntime := filepath.Join(dialtoneEnv, "go")
	if _, err := os.Stat(goRuntime); err != nil {
		return fmt.Errorf("required runtime missing at %s", goRuntime)
	}

	// Start expects TASK.md to already be present from `worktree add --task ...`.
	taskPath := filepath.Join(worktreePath, "TASK.md")
	if _, err := os.Stat(taskPath); err != nil {
		return fmt.Errorf("TASK.md missing in %s; run `worktree add %s --task <file>` first", worktreePath, name)
	}
	if _, err := os.Stat(filepath.Join(worktreePath, "dialtone.sh")); err != nil {
		return fmt.Errorf("dialtone.sh not found in worktree path: %s", worktreePath)
	}

	// Launch Agent in existing tmux session.
	if strings.TrimSpace(prompt) == "" {
		prompt = fmt.Sprintf("use software development skills and finish %s ... make sure to sign before you start work", taskPath)
	}

	fmt.Printf("[Worktree] Launching Gemini Agent in session '%s'...\n", name)
	cmd := fmt.Sprintf("./dialtone.sh gemini run --task TASK.md --prompt %s", strconv.Quote(prompt))
	
	tmuxCmd := exec.Command("tmux", "send-keys", "-t", name, cmd, "C-m")
	if err := tmuxCmd.Run(); err != nil {
		return fmt.Errorf("failed to send keys to tmux: %w", err)
	}
	
	return nil
}

func Remove(name string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	worktreePath := filepath.Join(filepath.Dir(repoRoot), name)

	// Kill tmux session
	if err := killTmuxSession(name); err != nil {
		return err
	}

	// Remove worktree
	fmt.Printf("[Worktree] Removing worktree %s...\n", worktreePath)
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	// Ensure stale worktree metadata is cleaned up.
	prune := exec.Command("git", "worktree", "prune")
	prune.Dir = repoRoot
	_ = prune.Run()

	// Guarantee the folder is gone even if git metadata was already missing.
	if _, err := os.Stat(worktreePath); err == nil {
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove leftover worktree directory %s: %w", worktreePath, err)
		}
	}
	return nil
}

func List() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	entries, err := loadWorktreeEntries(repoRoot)
	if err != nil {
		return err
	}

	fmt.Println("Worktrees:")
	fmt.Println("IDX  NAME                 TASK      TMUX     BRANCH            PATH")
	for i, e := range entries {
		tmux := "no"
		if e.Session {
			tmux = "yes"
		}
		fmt.Printf("%-4d %-20s %-9s %-8s %-17s %s\n", i+1, e.Name, e.TaskStatus, tmux, e.Branch, e.Path)
	}

	return nil
}

func Attach(selector string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	entries, err := loadWorktreeEntries(repoRoot)
	if err != nil {
		return err
	}
	entry, err := resolveWorktreeSelector(entries, selector)
	if err != nil {
		return err
	}
	if !entry.Session {
		return fmt.Errorf("tmux session not running for '%s'", entry.Name)
	}
	var cmd *exec.Cmd
	if os.Getenv("TMUX") != "" {
		cmd = exec.Command("tmux", "switch-client", "-t", entry.Name)
	} else {
		cmd = exec.Command("tmux", "attach", "-t", entry.Name)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func TmuxLogs(selector string, n int) error {
	if n <= 0 {
		n = 10
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	entries, err := loadWorktreeEntries(repoRoot)
	if err != nil {
		return err
	}
	entry, err := resolveWorktreeSelector(entries, selector)
	if err != nil {
		return err
	}
	if !entry.Session {
		return fmt.Errorf("tmux session not running for '%s'", entry.Name)
	}

	scrollback := 2000
	if n > scrollback {
		scrollback = n + 100
	}
	cmd := exec.Command("tmux", "capture-pane", "-pt", entry.Name, "-S", fmt.Sprintf("-%d", scrollback))
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to capture tmux pane: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	start := end - n
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:end] {
		fmt.Println(line)
	}
	return nil
}

// Helpers

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

func branchExists(branch string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	return cmd.Run() == nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func tmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func resolveDialtoneEnv(baseDir string) (string, error) {
	if env := os.Getenv("DIALTONE_ENV"); env != "" {
		return expandHome(env)
	}

	envFile := filepath.Join(baseDir, "env", ".env")
	file, err := os.Open(envFile)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "DIALTONE_ENV=") {
				value := strings.TrimPrefix(line, "DIALTONE_ENV=")
				value = strings.TrimSpace(strings.Trim(value, `"'`))
				if value != "" {
					return expandHome(value)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("failed reading %s: %w", envFile, err)
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(home, ".dialtone_env"), nil
}

func syncWorktreeEnv(repoRoot, worktreePath, dialtoneEnv string) error {
	envDir := filepath.Join(worktreePath, "env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return err
	}

	srcEnv := filepath.Join(repoRoot, "env", ".env")
	dstEnv := filepath.Join(envDir, ".env")
	if _, err := os.Stat(srcEnv); err == nil {
		if err := copyFile(srcEnv, dstEnv); err != nil {
			return err
		}
		// Keep DIALTONE_ENV explicit/normalized in copied env for this worktree.
		current, _ := os.ReadFile(dstEnv)
		lines := []string{}
		for _, line := range strings.Split(string(current), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "DIALTONE_ENV=") {
				continue
			}
			lines = append(lines, line)
		}
		lines = append(lines, fmt.Sprintf("DIALTONE_ENV=%s", dialtoneEnv))
		content := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
		return os.WriteFile(dstEnv, []byte(content), 0644)
	}

	// Fallback: create minimal env file if source env/.env does not exist.
	content := fmt.Sprintf("DIALTONE_ENV=%s\n", dialtoneEnv)
	return os.WriteFile(dstEnv, []byte(content), 0644)
}

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		if strings.HasPrefix(path, "~/") {
			return filepath.Join(home, path[2:]), nil
		}
	}
	return path, nil
}

type worktreeEntry struct {
	Path       string
	Branch     string
	Name       string
	TaskStatus string
	Session    bool
}

func loadWorktreeEntries(repoRoot string) ([]worktreeEntry, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	entries := []worktreeEntry{}
	current := worktreeEntry{}
	inEntry := false

	flush := func() {
		if !inEntry || current.Path == "" {
			return
		}
		current.Name = filepath.Base(current.Path)
		current.Session = tmuxSessionExists(current.Name)
		current.TaskStatus = detectTaskStatus(current.Path, current.Session)
		entries = append(entries, current)
		current = worktreeEntry{}
		inEntry = false
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			flush()
			inEntry = true
			current.Path = strings.TrimPrefix(line, "worktree ")
			continue
		}
		if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		}
	}
	flush()

	return entries, nil
}

func detectTaskStatus(worktreePath string, session bool) string {
	taskFile := filepath.Join(worktreePath, "TASK.md")
	data, err := os.ReadFile(taskFile)
	if err != nil {
		return "-"
	}
	if status := parseTaskSignatureStatus(string(data)); status != "" {
		return status
	}
	if session {
		return "working"
	}
	return "pending"
}

func resolveWorktreeSelector(entries []worktreeEntry, selector string) (worktreeEntry, error) {
	if idx, err := strconv.Atoi(selector); err == nil {
		if idx < 1 || idx > len(entries) {
			return worktreeEntry{}, fmt.Errorf("invalid worktree index %d", idx)
		}
		return entries[idx-1], nil
	}
	for _, e := range entries {
		if e.Name == selector {
			return e, nil
		}
	}
	return worktreeEntry{}, fmt.Errorf("worktree not found: %s", selector)
}

func parseTaskSignatureStatus(content string) string {
	lines := strings.Split(content, "\n")
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
			status := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
			switch status {
			case "wait", "work", "done", "fail":
				return status
			}
			return status
		}
	}
	return ""
}

func killTmuxSession(name string) error {
	if !tmuxSessionExists(name) {
		return nil
	}
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to kill tmux session '%s': %v (%s)", name, err, strings.TrimSpace(string(out)))
	}
	if tmuxSessionExists(name) {
		return fmt.Errorf("tmux session '%s' still exists after kill-session", name)
	}
	return nil
}
