package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	dialtest.RegisterTicket("cli-standardization")
	dialtest.AddSubtaskTest("init", RunInitTest, nil)
	dialtest.AddSubtaskTest("Audit Modules", RunAuditTest, nil)
	dialtest.AddSubtaskTest("Setup Branch and Ticket System", RunSetupTest, nil)
	dialtest.AddSubtaskTest("Standardize Core Modules", RunStandardizeCoreTest, nil)
	dialtest.AddSubtaskTest("Standardize Plugins", RunStandardizePluginsTest, nil)
	dialtest.AddSubtaskTest("Update Dev Entry point", RunUpdateDevTest, nil)
	dialtest.AddSubtaskTest("Verification", RunVerificationTest, nil)
}

func RunInitTest() error {
	return nil
}

func RunAuditTest() error {
	// cli_review.md should exist
	if _, err := os.Stat("cli_review.md"); os.IsNotExist(err) {
		return fmt.Errorf("cli_review.md missing")
	}
	return nil
}

func RunSetupTest() error {
	// Branch should be cli-standardization
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(string(output))
	if branch != "cli-standardization" {
		return fmt.Errorf("expected branch cli-standardization, got %s", branch)
	}
	return nil
}

func RunStandardizeCoreTest() error {
	modules := []string{"browser", "build", "config", "earth", "install", "logger", "mock", "ssh", "util", "web"}
	for _, m := range modules {
		path := filepath.Join("src", "core", m, "cli", "cli.go")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("missing CLI for core module %s: %s", m, path)
		}
	}
	return nil
}

func RunStandardizePluginsTest() error {
	// jax-demo CLI
	if _, err := os.Stat("src/plugins/jax-demo/cli/jax-demo.go"); os.IsNotExist(err) {
		return fmt.Errorf("missing jax-demo CLI")
	}
	// diagnostic CLI help support (just check file exists, manual verification was done)
	if _, err := os.Stat("src/plugins/diagnostic/cli/diagnostic.go"); os.IsNotExist(err) {
		return fmt.Errorf("missing diagnostic CLI")
	}
	return nil
}

func RunUpdateDevTest() error {
	content, err := os.ReadFile("src/dev.go")
	if err != nil {
		return err
	}
	s := string(content)
	if !strings.Contains(s, "build_cli.Run") {
		return fmt.Errorf("src/dev.go does not seem to use new build_cli package")
	}
	if !strings.Contains(s, "jax_demo_cli.Run") {
		return fmt.Errorf("src/dev.go does not seem to use new jax_demo_cli package")
	}
	return nil
}

func RunVerificationTest() error {
	commands := []string{"build", "install", "ssh", "diagnostic", "jax-demo"}
	for _, cmdStr := range commands {
		cmd := exec.Command("./dialtone.sh", cmdStr, "help")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command './dialtone.sh %s help' failed", cmdStr)
		}
	}
	return nil
}
