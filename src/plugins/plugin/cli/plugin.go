package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[plugin] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[plugin] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

// RunPlugin handles the 'plugin' command
func RunPlugin(args []string) {
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	subArgs := args[1:]

	switch command {
	case "create":
		runCreate(subArgs)
	case "test":
		runTest(subArgs)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown plugin command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev plugin <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  create <plugin-name>   Create a new plugin structure")
	fmt.Println("  test <plugin-name>     Run tests for a plugin")
	fmt.Println("  help                   Show this help message")
}

func runTest(args []string) {
	if len(args) < 1 {
		logFatal("Usage: plugin test <plugin-name>")
	}
	pluginName := args[0]
	testDir := filepath.Join("src", "plugins", pluginName, "test")

	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		logFatal("Test directory not found: %s", testDir)
	}

	logInfo("Running tests in %s...", testDir)
	cmd := exec.Command("./dialtone.sh", "test", pluginName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logFatal("Tests failed: %v", err)
	}
	logInfo("All tests passed.")
}

func runCreate(args []string) {
	if len(args) < 1 {
		logFatal("Usage: plugin create <plugin-name>")
	}

	pluginName := args[0]
	logInfo("Creating plugin: %s", pluginName)

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

	logInfo("Plugin %s created successfully", pluginName)
}

func ensureDir(path string) {
	if err := os.MkdirAll(path, 0755); err != nil {
		logFatal("Failed to create directory %s: %v", path, err)
	}
}

func createTestTemplates(testDir, pluginName string) {
	// Clean up plugin name for package (replace - with _)
	pkgName := strings.ReplaceAll(pluginName, "-", "_")

	templates := map[string]string{
		"unit_test.go": fmt.Sprintf(`package test

import "fmt"

func RunUnit() error {
	fmt.Println("Running unit test for %s")
	return nil
}
`, pkgName),
		"integration_test.go": fmt.Sprintf(`package test

import "fmt"

func RunIntegration() error {
	fmt.Println("Running integration test for %s")
	return nil
}
`, pkgName),
		"e2e_test.go": fmt.Sprintf(`package test

import "fmt"

func RunE2E() error {
	fmt.Println("Running E2E test for %s")
	return nil
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
