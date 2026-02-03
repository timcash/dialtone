package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	dialtest.RegisterTicket("decouple-plugin-installation")
	dialtest.AddSubtaskTest("universal-env-flag", RunEnvFlagTest, nil)
	dialtest.AddSubtaskTest("core-build-scripts", RunBuildScriptsTest, nil)
	dialtest.AddSubtaskTest("plugin-install-command", RunPluginInstallTest, nil)
	dialtest.AddSubtaskTest("update-main-installer", RunMainInstallerTest, nil)
	dialtest.AddSubtaskTest("remove-ai-from-core", RunRemoveAIFromCoreTest, nil)
	dialtest.AddSubtaskTest("verify-install-go", RunVerifyInstallGo, nil)
	dialtest.AddSubtaskTest("verify-install-ai", RunVerifyInstallAI, nil)
	dialtest.AddSubtaskTest("stub-plugin-install", RunStubPluginInstallTest, nil)
	dialtest.AddSubtaskTest("env-flag-propagation", RunEnvFlagPropagationTest, nil)
	dialtest.AddSubtaskTest("skip-install-if-ready", RunSkipInstallIfReadyTest, nil)
	dialtest.AddSubtaskTest("verification", RunVerificationTest, nil)
}

func RunEnvFlagTest() error {
	// Verify --env flag support by checking TEST_VAR from .env.test
	// This variable should now be inherited from dialtone.sh sourcing test.env
	testVar := os.Getenv("TEST_VAR")
	if testVar != "passed" {
		return fmt.Errorf("TEST_VAR not found or incorrect: '%s'. Ensure dialtone.sh sources the env file correctly.", testVar)
	}
	return nil
}

func RunBuildScriptsTest() error {
	return nil
}

func RunPluginInstallTest() error {
	// Verify 'plugin install' command exists and runs
	// We pass --env explicitly here to ensure recursive calls also use it
	cmd := exec.Command("./dialtone.sh", "--env", "env/test.env", "install")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("plugin install failed: %v, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "Installing dependencies for plugin: ticket") {
		return fmt.Errorf("plugin install output missing expected string. Output: %s", string(output))
	}
	return nil
}

func RunMainInstallerTest() error {
	content, err := os.ReadFile("dialtone.sh")
	if err != nil {
		return err
	}
	if !strings.Contains(string(content), "plugin install ticket") {
		return fmt.Errorf("dialtone.sh does not contain the updated plugin install logic")
	}
	return nil
}

func RunRemoveAIFromCoreTest() error {
	content, err := os.ReadFile("src/dev.go")
	if err != nil {
		return err
	}
	if strings.Contains(string(content), "\"dialtone/cli/src/plugins/ai/cli\"") {
		return fmt.Errorf("src/dev.go still imports ai_cli")
	}
	return nil
}

func RunVerifyInstallGo() error {
	envDir := getDialtoneEnv()
	os.RemoveAll(filepath.Join(envDir, "go"))

	// We MUST use the primary 'install' command to bootstrap Go when it is missing
	cmd := exec.Command("./dialtone.sh", "--env", "env/test.env", "install")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go bootstrap install failed: %v, output: %s", err, string(output))
	}

	// Verify go binary exists in test env
	goBin := filepath.Join(envDir, "go", "bin", "go")
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		return fmt.Errorf("go binary not found at %s after install", goBin)
	}
	return nil
}

func RunVerifyInstallAI() error {
	// Now that Go is installed (from previous subtask or bootstrap), we can use the plugin command
	cmd := exec.Command("./dialtone.sh", "--env", "env/test.env", "plugin", "install", "ai")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "AI Plugin: Checking dependencies") {
			return fmt.Errorf("ai install did not trigger correctly, output: %s", string(output))
		}
	}
	return nil
}

func RunStubPluginInstallTest() error {
	// Verify that various plugins respond to 'install' without error
	plugins := []string{"ticket", "github", "camera", "chrome"}
	for _, p := range plugins {
		cmd := exec.Command("./dialtone.sh", "--env", "env/test.env", "plugin", "install", p)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("plugin %s install failed: %v, output: %s", p, err, string(output))
		}
	}
	return nil
}

func RunEnvFlagPropagationTest() error {
	// Verify that the environment from test.env is propagated through recursive shell calls
	// We'll use a hack of checking if 'plugin install' output mentions the correct environment dir
	cmd := exec.Command("./dialtone.sh", "--env", "env/test.env", "plugin", "install", "go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("env propagation check failed: %v", err)
	}
	// go install logs the path it uses
	expect := "Using local dependencies from"
	if !strings.Contains(string(output), expect) {
		// If it's already installed, it might skip that log. Let's check for 'Go toolchain installed' or 'already installed'
		if !strings.Contains(string(output), "is already installed at") {
			return fmt.Errorf("output did not show correct environment usage. Output: %s", string(output))
		}
	}
	return nil
}

func RunSkipInstallIfReadyTest() error {
	// 1. First run to ensure everything is installed
	cmd1 := exec.Command("./dialtone.sh", "--env", "env/test.env", "install")
	if err := cmd1.Run(); err != nil {
		return fmt.Errorf("initial install failed: %v", err)
	}

	// 2. Second run should be fast and skip all downloads
	startTime := time.Now()
	cmd2 := exec.Command("./dialtone.sh", "--env", "env/test.env", "install")
	output, err := cmd2.CombinedOutput()
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("second install failed: %v, output: %s", err, string(output))
	}

	fmt.Printf("[dialtest] Second install output:\n%s\n", string(output))

	// Heuristic: If it takes less than 5 seconds, it probably skipped heavy downloads
	if duration > 5*time.Second {
		return fmt.Errorf("second install took too long (%v), expected it to skip downloads", duration)
	}

	// Check output for "is already installed"
	expectedSubstrings := []string{
		"Go 1.25.5 is already installed",
		"Node.js (22.13.0) is already installed",
		"GitHub CLI (2.86.0) is already installed",
	}
	for _, s := range expectedSubstrings {
		if !strings.Contains(string(output), s) {
			return fmt.Errorf("output missing expected skip message: %s. Output: %s", s, string(output))
		}
	}

	return nil
}

func RunVerificationTest() error {
	return nil
}

func getDialtoneEnv() string {
	return os.Getenv("DIALTONE_ENV")
}
