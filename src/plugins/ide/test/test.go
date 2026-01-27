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
	test.Register("ide-setup-verify", "ide", []string{"plugin", "ide"}, RunIDESetupVerify)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running ide plugin suite...")
	return test.RunPlugin("ide")
}

func RunIDESetupVerify() error {
	// Verify Workflows
	if err := verifyCopy("docs/workflows", ".agent/workflows", "ticket.md"); err != nil {
		return err
	}

	// Verify Rules
	if err := verifyCopy("docs/rules", ".agent/rules", "rule-cli.md"); err != nil {
		return err
	}

	fmt.Println("PASS: [ide] IDE setup verified (workflows & rules)")
	return nil
}

func verifyCopy(srcDir, destDir, sampleFile string) error {
	// 1. Check if destDir exists
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Errorf("FAIL: %s does not exist", destDir)
	}

	// 2. Check for a sample file
	destFile := filepath.Join(destDir, sampleFile)
	info, err := os.Lstat(destFile)
	if err != nil {
		return fmt.Errorf("FAIL: %s does not exist: %v", destFile, err)
	}

	// 3. Verify it is NOT a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("FAIL: %s is a symlink, should be a regular file", destFile)
	}

	// 4. Verify contents match
	srcFile := filepath.Join(srcDir, sampleFile)
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

	return nil
}
