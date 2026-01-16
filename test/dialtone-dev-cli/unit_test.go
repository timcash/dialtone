package dialtone_dev_cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dialtone "dialtone/cli/src"
)

// getProjectRoot returns the project root directory
func getProjectRoot(t *testing.T) string {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	return filepath.Join(cwd, "..", "..")
}

// TestDevCliHelp verifies dialtone-dev --help shows usage
func TestDevCliHelp(t *testing.T) {
	projectRoot := getProjectRoot(t)

	cmd := exec.Command("go", "run", "dialtone-dev.go", "help")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run dialtone-dev help: %v\n%s", err, output)
	}

	helpText := string(output)
	dialtone.LogInfo("Help output received (%d bytes)", len(helpText))

	expectedPhrases := []string{
		"dialtone-dev",
		"plan",
		"branch",
		"test",
		"pull-request",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(helpText, phrase) {
			t.Errorf("Help missing expected phrase: %s", phrase)
		}
	}
}

// TestPlanCreate verifies dialtone-dev plan <name> creates a new plan file
func TestPlanCreate(t *testing.T) {
	projectRoot := getProjectRoot(t)
	testPlanName := "test-temp-plan-" + randomSuffix()
	planFile := filepath.Join(projectRoot, "plan", "plan-"+testPlanName+".md")

	// Ensure cleanup
	defer func() {
		os.Remove(planFile)
		dialtone.LogInfo("Cleaned up test plan file: %s", planFile)
	}()

	// Verify plan doesn't exist yet
	if _, err := os.Stat(planFile); err == nil {
		t.Fatalf("Test plan file already exists: %s", planFile)
	}

	// Create plan
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plan", testPlanName)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create plan: %v\n%s", err, output)
	}

	dialtone.LogInfo("Plan creation output: %s", strings.TrimSpace(string(output)))

	// Verify plan file was created
	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		t.Fatalf("Plan file was not created: %s", planFile)
	}

	dialtone.LogInfo("Plan file created successfully: %s", planFile)
}

// TestPlanTemplate verifies the plan template has required sections
func TestPlanTemplate(t *testing.T) {
	projectRoot := getProjectRoot(t)
	testPlanName := "test-template-" + randomSuffix()
	planFile := filepath.Join(projectRoot, "plan", "plan-"+testPlanName+".md")

	// Ensure cleanup
	defer func() {
		os.Remove(planFile)
	}()

	// Create plan
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plan", testPlanName)
	cmd.Dir = projectRoot

	if _, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	// Read the plan file
	content, err := os.ReadFile(planFile)
	if err != nil {
		t.Fatalf("Failed to read plan file: %v", err)
	}

	planContent := string(content)
	dialtone.LogInfo("Plan content length: %d bytes", len(planContent))

	// Verify required sections
	requiredSections := []string{
		"# Plan:",
		"## Goal",
		"## Tests",
		"## Notes",
		"## Blocking Issues",
		"## Progress Log",
	}

	for _, section := range requiredSections {
		if !strings.Contains(planContent, section) {
			t.Errorf("Plan template missing required section: %s", section)
		} else {
			dialtone.LogInfo("Found section: %s", section)
		}
	}

	// Verify template has checkbox items
	if !strings.Contains(planContent, "- [ ]") {
		t.Errorf("Plan template missing checkbox items")
	}
}

// TestPlanList verifies dialtone-dev plan lists existing plans
func TestPlanList(t *testing.T) {
	projectRoot := getProjectRoot(t)

	cmd := exec.Command("go", "run", "dialtone-dev.go", "plan")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list plans: %v\n%s", err, output)
	}

	listOutput := string(output)
	dialtone.LogInfo("Plan list output: %s", strings.TrimSpace(listOutput))

	// Should contain header
	if !strings.Contains(listOutput, "Plan Files:") {
		t.Errorf("Plan list missing 'Plan Files:' header")
	}

	// Should list at least the dialtone-dev-cli plan we created
	if !strings.Contains(listOutput, "dialtone-dev-cli") {
		t.Errorf("Plan list should contain 'dialtone-dev-cli' plan")
	}
}

// TestPlanView verifies viewing an existing plan shows its content
func TestPlanView(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// View the existing dialtone-dev-cli plan
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plan", "dialtone-dev-cli")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to view plan: %v\n%s", err, output)
	}

	viewOutput := string(output)
	dialtone.LogInfo("Plan view output length: %d bytes", len(viewOutput))

	// Should contain the plan content
	expectedContent := []string{
		"Plan File:",
		"Progress:",
		"dialtone-dev-cli",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(viewOutput, expected) {
			t.Errorf("Plan view missing expected content: %s", expected)
		}
	}
}

// TestPlanProgress verifies progress counting works correctly
func TestPlanProgress(t *testing.T) {
	projectRoot := getProjectRoot(t)
	testPlanName := "test-progress-" + randomSuffix()
	planFile := filepath.Join(projectRoot, "plan", "plan-"+testPlanName+".md")

	// Ensure cleanup
	defer func() {
		os.Remove(planFile)
	}()

	// Create a custom plan with known progress
	planContent := `# Plan: test-progress

## Goal
Test progress counting

## Tests
- [x] test_1: First test (completed)
- [x] test_2: Second test (completed)
- [ ] test_3: Third test (pending)
- [ ] test_4: Fourth test (pending)

## Notes
- Test plan for progress counting
`

	if err := os.WriteFile(planFile, []byte(planContent), 0644); err != nil {
		t.Fatalf("Failed to write test plan: %v", err)
	}

	// List plans and check progress
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plan")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list plans: %v\n%s", err, output)
	}

	listOutput := string(output)

	// Should show 2/4 progress for our test plan
	if !strings.Contains(listOutput, "[2/4]") {
		t.Errorf("Plan list should show [2/4] progress for test plan, got: %s", listOutput)
	}

	dialtone.LogInfo("Progress counting verified: 2/4 shown correctly")
}

// TestBranchCommand verifies branch command shows proper usage without args
func TestBranchCommand(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// Run branch without args to see usage
	cmd := exec.Command("go", "run", "dialtone-dev.go", "branch")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	// Expected to exit with error code 1 (no branch name provided)
	if err == nil {
		t.Logf("Branch command output: %s", output)
	}

	branchOutput := string(output)

	// Should show usage info
	if !strings.Contains(branchOutput, "Usage:") || !strings.Contains(branchOutput, "branch") {
		t.Errorf("Branch command should show usage when no args provided")
	}

	dialtone.LogInfo("Branch command shows usage correctly")
}

// TestTestCommand verifies test command structure
func TestTestCommand(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// Run test command on a non-existent feature to verify structure
	// This will fail but we just want to verify it tries to run go test
	cmd := exec.Command("go", "run", "dialtone-dev.go", "test", "nonexistent-feature-xyz")
	cmd.Dir = projectRoot

	output, _ := cmd.CombinedOutput()
	testOutput := string(output)

	// Should indicate it's running tests
	if !strings.Contains(testOutput, "Running tests") {
		t.Errorf("Test command should log 'Running tests', got: %s", testOutput)
	}

	dialtone.LogInfo("Test command structure verified")
}

// TestPullRequestRequiresGh verifies PR command checks for gh CLI
func TestPullRequestRequiresGh(t *testing.T) {
	// Skip if gh is installed (we can't test the error case)
	if _, err := exec.LookPath("gh"); err == nil {
		t.Skip("gh CLI is installed, skipping 'gh not found' test")
	}

	projectRoot := getProjectRoot(t)

	cmd := exec.Command("go", "run", "dialtone-dev.go", "pull-request")
	cmd.Dir = projectRoot

	output, _ := cmd.CombinedOutput()
	prOutput := string(output)

	// Should mention gh CLI not found
	if !strings.Contains(prOutput, "gh") {
		t.Errorf("PR command should mention gh CLI, got: %s", prOutput)
	}

	dialtone.LogInfo("Pull-request command checks for gh CLI")
}

// TestUnknownCommand verifies unknown commands show error and usage
func TestUnknownCommand(t *testing.T) {
	projectRoot := getProjectRoot(t)

	cmd := exec.Command("go", "run", "dialtone-dev.go", "invalid-command-xyz")
	cmd.Dir = projectRoot

	output, _ := cmd.CombinedOutput()
	errorOutput := string(output)

	// Should show unknown command error
	if !strings.Contains(errorOutput, "Unknown command") {
		t.Errorf("Should show 'Unknown command' error, got: %s", errorOutput)
	}

	// Should still show usage
	if !strings.Contains(errorOutput, "Usage:") {
		t.Errorf("Should show usage after unknown command")
	}

	dialtone.LogInfo("Unknown command handling verified")
}

// TestPlanStatusIcons verifies list shows correct status icons
func TestPlanStatusIcons(t *testing.T) {
	projectRoot := getProjectRoot(t)

	// Create a completed plan
	completedPlanName := "test-complete-" + randomSuffix()
	completedPlanFile := filepath.Join(projectRoot, "plan", "plan-"+completedPlanName+".md")
	completedContent := `# Plan: test-complete

## Goal
Test completed status

## Tests
- [x] test_1: Done
- [x] test_2: Done
`
	if err := os.WriteFile(completedPlanFile, []byte(completedContent), 0644); err != nil {
		t.Fatalf("Failed to write completed plan: %v", err)
	}
	defer os.Remove(completedPlanFile)

	// Create an in-progress plan
	inProgressPlanName := "test-inprogress-" + randomSuffix()
	inProgressPlanFile := filepath.Join(projectRoot, "plan", "plan-"+inProgressPlanName+".md")
	inProgressContent := `# Plan: test-inprogress

## Goal
Test in-progress status

## Tests
- [x] test_1: Done
- [ ] test_2: Pending
`
	if err := os.WriteFile(inProgressPlanFile, []byte(inProgressContent), 0644); err != nil {
		t.Fatalf("Failed to write in-progress plan: %v", err)
	}
	defer os.Remove(inProgressPlanFile)

	// List plans
	cmd := exec.Command("go", "run", "dialtone-dev.go", "plan")
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list plans: %v\n%s", err, output)
	}

	listOutput := string(output)

	// Check for status indicators (✅ for complete, 🔄 for in-progress)
	if !strings.Contains(listOutput, "✅") {
		t.Errorf("Should show ✅ for completed plan")
	}
	if !strings.Contains(listOutput, "🔄") {
		t.Errorf("Should show 🔄 for in-progress plan")
	}

	dialtone.LogInfo("Status icons verified: ✅ for complete, 🔄 for in-progress")
}

// randomSuffix generates a simple suffix using process ID and time
func randomSuffix() string {
	return string(rune('a'+os.Getpid()%26)) + string(rune('0'+os.Getpid()%10))
}
