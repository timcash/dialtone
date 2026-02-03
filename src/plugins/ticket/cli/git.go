package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func currentGitBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func ensureOnGitBranch(branchName string) error {
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return fmt.Errorf("branch name is empty")
	}

	cur, err := currentGitBranch()
	if err == nil && cur == branchName {
		return nil
	}

	// Check if the branch exists.
	cmd := exec.Command("git", "branch", "--list", branchName)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to check git branches: %w", err)
	}

	var checkout *exec.Cmd
	if strings.TrimSpace(out.String()) != "" {
		checkout = exec.Command("git", "checkout", branchName)
	} else {
		checkout = exec.Command("git", "checkout", "-b", branchName)
	}
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("git checkout failed (commit/stash local changes?): %w", err)
	}
	return nil
}
