package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Run(taskFile, model, promptOverride string) error {
	if _, err := exec.LookPath("gemini"); err != nil {
		return fmt.Errorf("gemini CLI not found in PATH")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	absTask, err := resolveTaskPath(cwd, taskFile)
	if err != nil {
		return err
	}
	taskBytes, err := os.ReadFile(absTask)
	if err != nil {
		return fmt.Errorf("failed to read task file %s: %w", absTask, err)
	}

	prompt := promptOverride
	if strings.TrimSpace(prompt) == "" {
		prompt = buildPrompt(cwd, absTask, string(taskBytes))
	}
	workspaceRoot := filepath.Dir(cwd)
	cmd := exec.Command(
		"gemini",
		"-m", model,
		"-p", prompt,
		"--approval-mode", "yolo",
		"--include-directories", workspaceRoot,
	)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func Doctor() error {
	path, err := exec.LookPath("gemini")
	if err != nil {
		return fmt.Errorf("gemini CLI not found in PATH")
	}

	fmt.Printf("[gemini] CLI: %s\n", path)

	authVars := []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"}
	found := false
	for _, key := range authVars {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			fmt.Printf("[gemini] Auth env present: %s\n", key)
			found = true
			break
		}
	}
	if !found {
		fmt.Println("[gemini] No API key env set. CLI may still work via interactive login.")
	}
	return nil
}

func buildPrompt(cwd, taskPath, taskBody string) string {
	return strings.TrimSpace(fmt.Sprintf(`
You are running inside a git worktree at: %s
Primary task file: %s

Task content:
%s

Instructions:
1) Do the task by editing files in this repo.
2) Run the relevant tests/commands to verify completion.
3) Keep changes minimal and focused.
4) Print a short completion summary and test results.
`, cwd, taskPath, strings.TrimSpace(taskBody)))
}

func resolveTaskPath(cwd, taskFile string) (string, error) {
	if filepath.IsAbs(taskFile) {
		return taskFile, nil
	}
	candidate := filepath.Join(cwd, taskFile)
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	// dialtone.sh runs plugins from repoRoot/src. If task lives at repo root,
	// resolve one level up.
	parentCandidate := filepath.Join(filepath.Dir(cwd), taskFile)
	if _, err := os.Stat(parentCandidate); err == nil {
		return parentCandidate, nil
	}
	return "", fmt.Errorf("failed to read task file %s: not found in %s or %s", taskFile, candidate, parentCandidate)
}
