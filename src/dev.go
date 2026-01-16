package dialtone

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ExecuteDev is the entry point for the dialtone-dev CLI
func ExecuteDev() {
	if len(os.Args) < 2 {
		printDevUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "plan":
		runPlan(args)
	case "branch":
		runBranch(args)
	case "test":
		runTest(args)
	case "pull-request", "pr":
		runPullRequest(args)
	case "help", "-h", "--help":
		printDevUsage()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printDevUsage()
		os.Exit(1)
	}
}

func printDevUsage() {
	fmt.Println("Usage: dialtone-dev <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  plan [name]        List plans or create/view a plan file")
	fmt.Println("  branch <name>      Create or checkout a feature branch")
	fmt.Println("  test [name]        Run tests (all or for specific feature, creates templates if missing)")
	fmt.Println("  pull-request       Create or update a pull request")
	fmt.Println("  help               Show this help message")
	fmt.Println("\nPull Request Options:")
	fmt.Println("  --title, -t <title>   Set PR title (default: branch name)")
	fmt.Println("  --body, -b <body>     Set PR body (default: plan file or auto-generated)")
	fmt.Println("  --draft, -d           Create as draft PR")
	fmt.Println("\nExamples:")
	fmt.Println("  dialtone-dev plan                    # List all plan files")
	fmt.Println("  dialtone-dev plan my-feature         # Create/view plan for my-feature")
	fmt.Println("  dialtone-dev branch my-feature       # Create or checkout branch")
	fmt.Println("  dialtone-dev test                    # Run all tests")
	fmt.Println("  dialtone-dev test my-feature         # Run tests for my-feature (creates templates)")
	fmt.Println("  dialtone-dev pull-request            # Create/update PR using branch name and plan")
	fmt.Println("  dialtone-dev pull-request --draft    # Create draft PR")
	fmt.Println("  dialtone-dev pull-request --title \"My Feature\" --body \"Description\"")
}

// runPlan handles the plan command
func runPlan(args []string) {
	planDir := "plan"

	// Ensure plan directory exists
	if err := os.MkdirAll(planDir, 0755); err != nil {
		LogFatal("Failed to create plan directory: %v", err)
	}

	// No args: list all plans
	if len(args) == 0 {
		listPlans(planDir)
		return
	}

	// With name: create or show plan
	name := args[0]
	planFile := filepath.Join(planDir, fmt.Sprintf("plan-%s.md", name))

	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		// Create new plan from template
		createPlan(planFile, name)
	} else {
		// Show existing plan
		showPlan(planFile)
	}
}

// listPlans lists all plan files with their completion status
func listPlans(planDir string) {
	entries, err := os.ReadDir(planDir)
	if err != nil {
		LogFatal("Failed to read plan directory: %v", err)
	}

	fmt.Println("Plan Files:")
	fmt.Println("===========")

	planFound := false
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "plan-") && strings.HasSuffix(entry.Name(), ".md") {
			planFound = true
			planPath := filepath.Join(planDir, entry.Name())
			completed, total := countProgress(planPath)

			// Extract feature name from filename
			name := strings.TrimPrefix(entry.Name(), "plan-")
			name = strings.TrimSuffix(name, ".md")

			status := "[ ]"
			if total > 0 {
				if completed == total {
					status = "[x]"
				} else if completed > 0 {
					status = "[~]"
				}
			}

			fmt.Printf("  %s %s [%d/%d] %s\n", status, name, completed, total, entry.Name())
		}
	}

	if !planFound {
		fmt.Println("  No plan files found.")
		fmt.Println("\nCreate a new plan with: dialtone-dev plan <feature-name>")
	}
}

// countProgress counts completed items (- [x]) vs total items (- [ ] or - [x])
func countProgress(planPath string) (completed, total int) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(content), "\n")
	checkboxPattern := regexp.MustCompile(`^- \[([ xX])\]`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		matches := checkboxPattern.FindStringSubmatch(trimmed)
		if len(matches) > 1 {
			total++
			if matches[1] == "x" || matches[1] == "X" {
				completed++
			}
		}
	}

	return completed, total
}

// createPlan creates a new plan file from template
func createPlan(planPath, name string) {
	template := fmt.Sprintf(`# Plan: %s

## Goal
[Describe the goal of this feature]

## Tests
- [ ] test_example_1: [Description of first test]
- [ ] test_example_2: [Description of second test]

## Notes
- [Add any relevant notes]

## Blocking Issues
- None

## Progress Log
- %s: Created plan file
`, name, time.Now().Format("2006-01-02"))

	if err := os.WriteFile(planPath, []byte(template), 0644); err != nil {
		LogFatal("Failed to create plan file: %v", err)
	}

	LogInfo("Created plan file: %s", planPath)
	fmt.Println("\nPlan Template Created:")
	fmt.Println("======================")
	fmt.Println(template)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit the plan file to define your goal and tests")
	fmt.Println("  2. Create a branch: dialtone-dev branch", name)
	fmt.Println("  3. Start implementing tests from the plan")
}

// showPlan displays the contents of a plan file
func showPlan(planPath string) {
	content, err := os.ReadFile(planPath)
	if err != nil {
		LogFatal("Failed to read plan file: %v", err)
	}

	completed, total := countProgress(planPath)
	
	fmt.Println("Plan File:", planPath)
	fmt.Printf("Progress: %d/%d tests completed\n", completed, total)
	fmt.Println("======================")
	fmt.Println(string(content))
}

// runBranch handles the branch command
func runBranch(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone-dev branch <name>")
		fmt.Println("\nThis command creates or checks out a feature branch.")
		os.Exit(1)
	}

	branchName := args[0]

	// Check if branch exists
	cmd := exec.Command("git", "branch", "--list", branchName)
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to check branches: %v", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		// Branch exists, checkout
		LogInfo("Branch '%s' exists, checking out...", branchName)
		cmd = exec.Command("git", "checkout", branchName)
	} else {
		// Branch doesn't exist, create
		LogInfo("Creating new branch '%s'...", branchName)
		cmd = exec.Command("git", "checkout", "-b", branchName)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Git operation failed: %v", err)
	}

	LogInfo("Now on branch: %s", branchName)
}

// runTest handles the test command
func runTest(args []string) {
	var testPath string
	if len(args) == 0 {
		testPath = "./test/..."
		LogInfo("Running all tests...")
	} else {
		name := args[0]
		testDir := filepath.Join("test", name)
		testPath = fmt.Sprintf("./test/%s/...", name)

		// Ensure test directory and template files exist
		ensureTestFiles(testDir, name)

		LogInfo("Running tests for: %s", name)
	}

	cmd := exec.Command("go", "test", "-v", testPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		// Test failures are not fatal errors, just exit with the same code
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		LogFatal("Failed to run tests: %v", err)
	}
}

// ensureTestFiles creates the test directory and template test files if they don't exist
func ensureTestFiles(testDir, featureName string) {
	// Create test directory if it doesn't exist
	if err := os.MkdirAll(testDir, 0755); err != nil {
		LogFatal("Failed to create test directory: %v", err)
	}

	// Convert feature-name to package name (replace - with _)
	packageName := strings.ReplaceAll(featureName, "-", "_")

	// Define test file templates
	testFiles := map[string]string{
		"unit_test.go":        generateUnitTestTemplate(packageName, featureName),
		"integration_test.go": generateIntegrationTestTemplate(packageName, featureName),
		"end_to_end_test.go":  generateEndToEndTestTemplate(packageName, featureName),
	}

	for filename, template := range testFiles {
		filePath := filepath.Join(testDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if err := os.WriteFile(filePath, []byte(template), 0644); err != nil {
				LogFatal("Failed to create test file %s: %v", filename, err)
			}
			LogInfo("Created test file: %s", filePath)
		}
	}
}

// generateUnitTestTemplate creates a unit test template
func generateUnitTestTemplate(packageName, featureName string) string {
	return fmt.Sprintf(`package %s

import (
	"testing"

	dialtone "dialtone/cli/src"
)

// Unit tests: Simple tests that run locally without IO operations
// These tests should be fast and test individual functions/components

func TestUnit_Example(t *testing.T) {
	dialtone.LogInfo("Running unit test for %s")
	
	// TODO: Add your unit tests here
	// Example:
	// result := SomeFunction(input)
	// if result != expected {
	//     t.Errorf("Expected %%v, got %%v", expected, result)
	// }
	
	t.Log("Unit test placeholder - implement your tests")
}

func TestUnit_Validation(t *testing.T) {
	// Test input validation, data parsing, etc.
	dialtone.LogInfo("Testing validation for %s")
	
	// TODO: Add validation tests
	t.Log("Validation test placeholder")
}
`, packageName, featureName, featureName)
}

// generateIntegrationTestTemplate creates an integration test template
func generateIntegrationTestTemplate(packageName, featureName string) string {
	return fmt.Sprintf(`package %s

import (
	"os"
	"path/filepath"
	"testing"

	dialtone "dialtone/cli/src"
)

// Integration tests: Test 2+ components together using test_data/
// These tests may use files, but should not require network or external services

func TestIntegration_Example(t *testing.T) {
	dialtone.LogInfo("Running integration test for %s")
	
	// Get project root for test data access
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %%v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	_ = projectRoot // Use for accessing test_data/
	
	// TODO: Add your integration tests here
	// Example using test data:
	// testDataPath := filepath.Join(projectRoot, "test_data", "sample.json")
	// data, err := os.ReadFile(testDataPath)
	// if err != nil {
	//     t.Skip("Test data not available")
	// }
	
	t.Log("Integration test placeholder - implement your tests")
}

func TestIntegration_Components(t *testing.T) {
	// Test how multiple components work together
	dialtone.LogInfo("Testing component integration for %s")
	
	// TODO: Add component integration tests
	t.Log("Component integration test placeholder")
}
`, packageName, featureName, featureName)
}

// generateEndToEndTestTemplate creates an end-to-end test template
func generateEndToEndTestTemplate(packageName, featureName string) string {
	return fmt.Sprintf(`package %s

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

// End-to-end tests: Browser and CLI tests on a live system or simulator
// These tests may require network, external services, or user interaction setup

func TestE2E_CLICommand(t *testing.T) {
	// Skip if running in CI without required setup
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running E2E CLI test for %s")
	
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %%v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	
	// TODO: Add your end-to-end CLI tests here
	// Example: Test a CLI command
	// cmd := exec.Command("go", "run", ".", "your-command", "--flag")
	// cmd.Dir = projectRoot
	// output, err := cmd.CombinedOutput()
	// if err != nil {
	//     t.Fatalf("Command failed: %%v\n%%s", err, output)
	// }
	
	_ = projectRoot
	t.Log("E2E CLI test placeholder - implement your tests")
}

func TestE2E_FullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test (SKIP_E2E is set)")
	}
	
	dialtone.LogInfo("Running full workflow E2E test for %s")
	
	// TODO: Test complete user workflows
	// This might include:
	// - Starting services
	// - Making API calls
	// - Verifying responses
	// - Cleaning up
	
	t.Log("Full workflow E2E test placeholder")
}

func TestE2E_BinaryExists(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %%v", err)
	}
	projectRoot := filepath.Join(cwd, "..", "..")
	binPath := filepath.Join(projectRoot, "bin", "dialtone")
	
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("Binary not built - run 'dialtone build' first")
	}
	
	// Verify binary runs
	cmd := exec.Command(binPath, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// --help might exit non-zero, check output instead
		if !strings.Contains(string(output), "dialtone") {
			t.Errorf("Binary output doesn't contain 'dialtone': %%s", output)
		}
	}
	
	dialtone.LogInfo("Binary exists and runs for %s tests")
}
`, packageName, featureName, featureName, featureName)
}

// runPullRequest handles the pull-request command
func runPullRequest(args []string) {
	// Check if gh CLI is available
	if _, err := exec.LookPath("gh"); err != nil {
		LogFatal("GitHub CLI (gh) not found. Install it from: https://cli.github.com/")
	}

	// Parse flags
	var title, body string
	var draft bool
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--title", "-t":
			if i+1 < len(args) {
				title = args[i+1]
				i++
			}
		case "--body", "-b":
			if i+1 < len(args) {
				body = args[i+1]
				i++
			}
		case "--draft", "-d":
			draft = true
		}
	}

	// Get current branch name
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		LogFatal("Failed to get current branch: %v", err)
	}
	branch := strings.TrimSpace(string(output))

	if branch == "main" || branch == "master" {
		LogFatal("Cannot create PR from main/master branch. Create a feature branch first.")
	}

	LogInfo("Creating/updating PR for branch: %s", branch)

	// Check if PR already exists
	checkCmd := exec.Command("gh", "pr", "view", "--json", "number")
	if err := checkCmd.Run(); err != nil {
		// PR doesn't exist, create it
		LogInfo("Creating new pull request...")
		
		var createArgs []string
		createArgs = append(createArgs, "pr", "create")
		
		// Use provided title or default to branch name
		if title != "" {
			createArgs = append(createArgs, "--title", title)
		} else {
			createArgs = append(createArgs, "--title", branch)
		}
		
		// Use provided body, or plan file, or default message
		if body != "" {
			createArgs = append(createArgs, "--body", body)
		} else {
			planFile := filepath.Join("plan", fmt.Sprintf("plan-%s.md", branch))
			if _, statErr := os.Stat(planFile); statErr == nil {
				createArgs = append(createArgs, "--body-file", planFile)
			} else {
				createArgs = append(createArgs, "--body", fmt.Sprintf("Feature: %s\n\nSee plan file for details.", branch))
			}
		}
		
		// Add draft flag if specified
		if draft {
			createArgs = append(createArgs, "--draft")
		}

		cmd = exec.Command("gh", createArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			LogFatal("Failed to create PR: %v", err)
		}
	} else {
		// PR exists, show info
		LogInfo("Pull request already exists. Opening in browser...")
		cmd = exec.Command("gh", "pr", "view", "--web")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
}
