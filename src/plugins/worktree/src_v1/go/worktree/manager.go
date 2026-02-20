package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// 4. Create Tmux session
	sessionName := name
	fmt.Printf("[Worktree] Starting tmux session '%s'...\n", sessionName)
	tmuxCmd := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", worktreePath)
	if err := tmuxCmd.Run(); err != nil {
		// If failed, maybe session exists?
		return fmt.Errorf("tmux new-session failed: %w", err)
	}

	return nil
}

func Start(name, taskFile string) error {
	// 1. Create worktree (and tmux session)
	if err := Add(name, taskFile, ""); err != nil {
		// If add failed, maybe it already exists?
		// We can try to continue if it's just "already exists" but Add returns generic error.
		// For now, abort on error unless we want to handle resume.
		return err
	}

	// 2. Launch Agent in Tmux
	fmt.Printf("[Worktree] Launching Gemini Agent in session '%s'...\n", name)
	// We assume TASK.md is at the root of worktree (Add copies it there)
	// Command: ./dialtone.sh gemini run --task TASK.md
	// Note: inside the worktree, we are at the repo root (it's a checkout).
	// So ./dialtone.sh exists.
	cmd := fmt.Sprintf("./dialtone.sh gemini run --task TASK.md")
	
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
	exec.Command("tmux", "kill-session", "-t", name).Run()

	// Remove worktree
	fmt.Printf("[Worktree] Removing worktree %s...\n", worktreePath)
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func List() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	fmt.Println("Git Worktrees:")
	cmd := exec.Command("git", "worktree", "list")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Run()

	fmt.Println("\nTmux Sessions:")
	tmuxCmd := exec.Command("tmux", "list-sessions")
	tmuxCmd.Stdout = os.Stdout
	tmuxCmd.Run() // Ignore error if no server running

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
