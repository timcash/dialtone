package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[ticket] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[ticket] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// RunStart handles 'ticket start <ticket-name>'
func RunStart(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket start <ticket-name> [--plugin <plugin-name>]")
	}

	arg := args[0]
	var pluginName string

	// implemented simple arg parsing
	for i := 1; i < len(args); i++ {
		if args[i] == "--plugin" && i+1 < len(args) {
			pluginName = args[i+1]
			i++
		}
	}

	ticketName := GetTicketName(arg)

	// 1. Create/Switch Git Branch
	// Check if branch exists
	cmd := exec.Command("git", "branch", "--list", ticketName)
	output, err := cmd.Output()
	if err != nil {
		logFatal("Failed to check git branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		logInfo("Switching to existing branch: %s", ticketName)
		if err := exec.Command("git", "checkout", ticketName).Run(); err != nil {
			logFatal("Failed to checkout branch: %v", err)
		}
	} else {
		logInfo("Creating new branch: %s", ticketName)
		if err := exec.Command("git", "checkout", "-b", ticketName).Run(); err != nil {
			logFatal("Failed to create branch: %v", err)
		}
	}

	// 2. Ticket Scaffolding
	ticketDir := filepath.Join("tickets", ticketName)
	ensureDir(ticketDir)
	ensureDir(filepath.Join(ticketDir, "code"))
	ensureDir(filepath.Join(ticketDir, "test"))

	// Create test templates
	createTestTemplates(filepath.Join(ticketDir, "test"), ticketName)

	ticketTaskMd := filepath.Join(ticketDir, "task.md")
	if _, err := os.Stat(ticketTaskMd); os.IsNotExist(err) {
		content := fmt.Sprintf("# Task: %s\n\n- [ ] Initial setup\n", ticketName)
		if err := os.WriteFile(ticketTaskMd, []byte(content), 0644); err != nil {
			logFatal("Failed to create task.md: %v", err)
		}
		logInfo("Created %s", ticketTaskMd)
	}

	// 3. Plugin Scaffolding
	if pluginName != "" {
		pluginDir := filepath.Join("src", "plugins", pluginName)
		ensureDir(pluginDir)
		ensureDir(filepath.Join(pluginDir, "app"))
		ensureDir(filepath.Join(pluginDir, "cli"))
		ensureDir(filepath.Join(pluginDir, "test"))

		readmePath := filepath.Join(pluginDir, "README.md")
		if _, err := os.Stat(readmePath); os.IsNotExist(err) {
			content := fmt.Sprintf("# Plugin: %s\n\nDescription of %s.\n", pluginName, pluginName)
			if err := os.WriteFile(readmePath, []byte(content), 0644); err != nil {
				logFatal("Failed to create README.md: %v", err)
			}
			logInfo("Created %s", readmePath)
		}

		// Create plugin test templates
		createTestTemplates(filepath.Join(pluginDir, "test"), pluginName)
	}

	logInfo("Ticket %s started successfully", ticketName)
}

// RunTest handles 'ticket test <ticket-name>'
func RunTest(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket test <ticket-name>")
	}
	ticketName := args[0]
	testDir := filepath.Join("tickets", ticketName, "test")

	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		logFatal("Test directory not found: %s", testDir)
	}

	logInfo("Running tests in %s...", testDir)
	cmd := exec.Command("go", "test", "-v", "./"+testDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logFatal("Tests failed: %v", err)
	}
	logInfo("All tests passed.")
}

// RunDone handles 'ticket done <ticket-name>'
func RunDone(args []string) {
	if len(args) < 1 {
		logFatal("Usage: ticket done <ticket-name>")
	}
	ticketName := args[0]

	// 1. Verify ticket.md subtasks
	// Assuming ticket.md is in tickets/<ticketName>/ticket.md
	ticketMd := filepath.Join("tickets", ticketName, "ticket.md")
	if _, err := os.Stat(ticketMd); err == nil {
		content, err := os.ReadFile(ticketMd)
		if err != nil {
			logFatal("Failed to read ticket.md: %v", err)
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- status:") {
				// Parse status value
				parts := strings.SplitN(trimmed, ":", 2)
				if len(parts) == 2 {
					statusVal := strings.TrimSpace(parts[1])
					if statusVal != "done" {
						logFatal("Ticket has incomplete subtask at line %d: '%s' (expected 'status: done')", i+1, trimmed)
					}
				}
			}
		}
		logInfo("All subtasks in ticket.md are verified as 'status: done'.")
	} else {
		logInfo("Warning: ticket.md not found at %s, skipping subtask verification.", ticketMd)
	}

	// 2. Run tests
	RunTest(args)

	logInfo("Ticket %s verified as DONE.", ticketName)
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logFatal("Failed to create directory %s: %v", path, err)
	}
}

// GetTicketName parses the ticket name from an argument (name or file path)
func GetTicketName(arg string) string {
	if strings.HasSuffix(arg, ".md") {
		if _, err := os.Stat(arg); err == nil {
			// It's a file, try to parse Branch
			content, err := os.ReadFile(arg)
			if err != nil {
				// Should not happen if Stat succeeded, but log just in case
				logInfo("Failed to read ticket file: %v", err)
				return ""
			}
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# Branch:") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "# Branch:"))
					logInfo("Parsed ticket name from file: %s", name)
					return name
				}
			}
			
			logInfo("No '# Branch:' found in %s, using filename as ticket name", arg)
			base := filepath.Base(arg)
			return strings.TrimSuffix(base, filepath.Ext(base))
		} else {
			// Ends in .md but doesn't exist
			logInfo("Ticket name '%s' ends in .md but file not found; treating as ticket name.", arg)
			return arg
		}
	}
	return arg
}

func createTestTemplates(testDir, ticketName string) {
	// Clean up ticket name for package (replace - with _)
	pkgName := strings.ReplaceAll(ticketName, "-", "_")

	templates := map[string]string{
		"unit_test.go": fmt.Sprintf(`package test

import "testing"

func TestUnit_Example(t *testing.T) {
	t.Log("Unit test for %s")
	t.Fatal("Not implemented")
}
`, pkgName),
		"integration_test.go": fmt.Sprintf(`package test

import "testing"

func TestIntegration_Example(t *testing.T) {
	t.Log("Integration test for %s")
	t.Fatal("Not implemented")
}
`, pkgName),
		"e2e_test.go": fmt.Sprintf(`package test

import "testing"

func TestE2E_Example(t *testing.T) {
	t.Log("E2E test for %s")
	t.Fatal("Not implemented")
}
`, pkgName),
	}

	for filename, content := range templates {
		fullPath := filepath.Join(testDir, filename)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				logFatal("Failed to create test file %s: %v", filename, err)
			}
			logInfo("Created test template: %s", fullPath)
		}
	}
}
