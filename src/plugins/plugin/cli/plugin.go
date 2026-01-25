package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
	case "create", "add":
		runCreate(subArgs)
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
	fmt.Println("  add <plugin-name>      Create a new plugin structure")
	fmt.Println("  create <plugin-name>   Alias for 'add'")
	fmt.Println("  help                   Show this help message")
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
	fullPath := filepath.Join(testDir, "test.go")
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		return
	}

	content := fmt.Sprintf(`package test

import (
	"fmt"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	// Register plugin tests here: test.Register("<test-name>", "<plugin-name>", []string{"plugin", "<tag>"}, Run<TestName>)
	test.Register("example-test", "%s", []string{"plugin", "%s"}, RunExample)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this plugin.
func RunAll() error {
	logger.LogInfo("Running %s plugin suite...")
	return test.RunPlugin("%s")
}

func RunExample() error {
	fmt.Println("PASS: [%s] Plugin logic verified")
	return nil
}
`, pluginName, pluginName, pluginName, pluginName, pluginName)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		logFatal("Failed to create test file %s: %v", fullPath, err)
	}
	logInfo("Created test template: %s", fullPath)
}
