package test

import (
	"fmt"
	"os"
	"path/filepath"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	test.Register("workflow-linking", "ide", []string{"plugin", "ide"}, RunWorkflowLinking)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running ide plugin suite...")
	return test.RunPlugin("ide")
}

func RunWorkflowLinking() error {
	// 1. Check if .agent/workflows exists and has links
	destDir := filepath.Join(".agent", "workflows")
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Errorf("FAIL: .agent/workflows does not exist")
	}

	// 2. Check for a specific file, e.g., ticket.md
	ticketLink := filepath.Join(destDir, "ticket.md")
	info, err := os.Lstat(ticketLink)
	if err != nil {
		return fmt.Errorf("FAIL: %s does not exist: %v", ticketLink, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("FAIL: %s is not a symlink", ticketLink)
	}

	// 3. Verify target
	target, err := os.Readlink(ticketLink)
	if err != nil {
		return fmt.Errorf("FAIL: could not read symlink %s: %v", ticketLink, err)
	}

	expectedTarget, _ := filepath.Abs(filepath.Join("docs", "workflows", "ticket.md"))
	if target != expectedTarget {
		return fmt.Errorf("FAIL: symlink target mismatch. Got %s, want %s", target, expectedTarget)
	}

	fmt.Println("PASS: [ide] Workflow linking verified")
	return nil
}
