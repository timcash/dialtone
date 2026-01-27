package test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	test.Register("workflow-copy", "ide", []string{"plugin", "ide"}, RunWorkflowCopy)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running ide plugin suite...")
	return test.RunPlugin("ide")
}

func RunWorkflowCopy() error {
	// 1. Check if .agent/workflows exists
	destDir := filepath.Join(".agent", "workflows")
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Errorf("FAIL: .agent/workflows does not exist")
	}

	// 2. Check for a specific file, e.g., ticket.md
	destFile := filepath.Join(destDir, "ticket.md")
	info, err := os.Lstat(destFile)
	if err != nil {
		return fmt.Errorf("FAIL: %s does not exist: %v", destFile, err)
	}

	// 3. Verify it is NOT a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("FAIL: %s is a symlink, should be a regular file", destFile)
	}

	// 4. Verify contents match
	srcFile := filepath.Join("docs", "workflows", "ticket.md")
	srcContent, err := os.ReadFile(srcFile)
	if err != nil {
		return fmt.Errorf("FAIL: could not read source file %s: %v", srcFile, err)
	}

	destContent, err := os.ReadFile(destFile)
	if err != nil {
		return fmt.Errorf("FAIL: could not read destination file %s: %v", destFile, err)
	}

	if !bytes.Equal(srcContent, destContent) {
		return fmt.Errorf("FAIL: file contents mismatch for %s", destFile)
	}

	fmt.Println("PASS: [ide] Workflow copy verified")
	return nil
}
