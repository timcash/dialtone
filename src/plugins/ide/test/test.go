package test

import (
	"dialtone/dev/logger"
	"dialtone/dev/test_core"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	test.Register("ide-setup-mode-verify", "ide", []string{"plugin", "ide"}, RunIDESetupModeVerify)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running ide plugin suite...")
	return test.RunPlugin("ide")
}

func RunIDESetupModeVerify() error {
	// 1. Verify Symlink Mode
	logger.LogInfo("Testing symlink mode...")
	cmd := exec.Command("./dialtone.sh", "ide", "setup-workflows", "--symlink")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FAIL: setup-workflows --symlink failed: %v", err)
	}
	if err := verifyMode(".agent/workflows", "ticket.md", true); err != nil {
		return err
	}

	// 2. Verify Copy Mode
	logger.LogInfo("Testing copy mode...")
	cmd = exec.Command("./dialtone.sh", "ide", "setup-workflows", "--copy")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FAIL: setup-workflows --copy failed: %v", err)
	}
	if err := verifyMode(".agent/workflows", "ticket.md", false); err != nil {
		return err
	}

	fmt.Println("PASS: [ide] IDE setup verified (symlink & copy modes)")
	return nil
}

func verifyMode(destDir, sampleFile string, expectSymlink bool) error {
	destFile := filepath.Join(destDir, sampleFile)
	info, err := os.Lstat(destFile)
	if err != nil {
		return fmt.Errorf("FAIL: %s does not exist: %v", destFile, err)
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	if isSymlink != expectSymlink {
		return fmt.Errorf("FAIL: %s symlink status mismatch. Got isSymlink=%v, want %v", destFile, isSymlink, expectSymlink)
	}

	return nil
}
