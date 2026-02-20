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

	taskBytes, err := os.ReadFile(taskFile)
	if err != nil {
		return fmt.Errorf("failed to read task file %s: %w", taskFile, err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	absTask := taskFile
	if !filepath.IsAbs(taskFile) {
		absTask = filepath.Join(cwd, taskFile)
	}

	prompt := promptOverride
	if strings.TrimSpace(prompt) == "" {
		prompt = buildPrompt(cwd, absTask, string(taskBytes))
	}
	cmd := exec.Command("gemini", "-m", model, "-p", prompt)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
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
