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
	testFeatureName := "test-cmd-structure-" + randomSuffix()
	testDir := filepath.Join(projectRoot, "test", testFeatureName)

	// Ensure cleanup of created test directory
	defer os.RemoveAll(testDir)

	// Run test command on a non-existent feature to verify structure
	// This will fail but we just want to verify it tries to run go test
	cmd := exec.Command("go", "run", "dialtone-dev.go", "test", testFeatureName)
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

	// Check for status indicators ([x] for complete, [~] for in-progress)
	if !strings.Contains(listOutput, "[x]") {
		t.Errorf("Should show [x] for completed plan")
	}
	if !strings.Contains(listOutput, "[~]") {
		t.Errorf("Should show [~] for in-progress plan")
	}

	dialtone.LogInfo("Status icons verified: [x] for complete, [~] for in-progress")
}

// randomSuffix generates a simple suffix using process ID and time
func randomSuffix() string {
	return string(rune('a'+os.Getpid()%26)) + string(rune('0'+os.Getpid()%10))
}

// TestTestCommandCreatesTemplates verifies test command creates template files
func TestTestCommandCreatesTemplates(t *testing.T) {
	projectRoot := getProjectRoot(t)
	testFeatureName := "test-template-gen-" + randomSuffix()
	testDir := filepath.Join(projectRoot, "test", testFeatureName)

	// Ensure cleanup
	defer func() {
		os.RemoveAll(testDir)
		dialtone.LogInfo("Cleaned up test directory: %s", testDir)
	}()

	// Verify test dir doesn't exist yet
	if _, err := os.Stat(testDir); err == nil {
		t.Fatalf("Test directory already exists: %s", testDir)
	}

	// Run dialtone-dev test <name> - this should create the test files
	cmd := exec.Command("go", "run", "dialtone-dev.go", "test", testFeatureName)
	cmd.Dir = projectRoot
	cmd.CombinedOutput() // Ignore exit code, tests will fail but files should be created

	// Verify test directory was created
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Fatalf("Test directory was not created: %s", testDir)
	}

	// Verify all three test files were created
	expectedFiles := []string{
		"unit_test.go",
		"integration_test.go",
		"end_to_end_test.go",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(testDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected test file not created: %s", filePath)
		} else {
			dialtone.LogInfo("Test file created: %s", filename)
		}
	}
}

// TestTestTemplateContents verifies the generated templates have correct content
func TestTestTemplateContents(t *testing.T) {
	projectRoot := getProjectRoot(t)
	testFeatureName := "test-content-check-" + randomSuffix()
	testDir := filepath.Join(projectRoot, "test", testFeatureName)

	// Ensure cleanup
	defer func() {
		os.RemoveAll(testDir)
	}()

	// Run dialtone-dev test to create files
	cmd := exec.Command("go", "run", "dialtone-dev.go", "test", testFeatureName)
	cmd.Dir = projectRoot
	cmd.CombinedOutput()

	// Check unit_test.go content
	unitTestPath := filepath.Join(testDir, "unit_test.go")
	content, err := os.ReadFile(unitTestPath)
	if err != nil {
		t.Fatalf("Failed to read unit_test.go: %v", err)
	}

	unitContent := string(content)

	// Verify package name uses underscores (Go convention)
	expectedPackage := strings.ReplaceAll(testFeatureName, "-", "_")
	if !strings.Contains(unitContent, "package "+expectedPackage) {
		t.Errorf("Unit test should have package %s", expectedPackage)
	}

	// Verify it imports dialtone
	if !strings.Contains(unitContent, `dialtone "dialtone/cli/src"`) {
		t.Errorf("Unit test should import dialtone package")
	}

	// Verify it has test functions
	if !strings.Contains(unitContent, "func Test") {
		t.Errorf("Unit test should have Test functions")
	}

	dialtone.LogInfo("Unit test template contents verified")

	// Check integration_test.go has integration-specific content
	integrationPath := filepath.Join(testDir, "integration_test.go")
	integrationContent, _ := os.ReadFile(integrationPath)
	if !strings.Contains(string(integrationContent), "Integration") {
		t.Errorf("Integration test should mention 'Integration'")
	}

	// Check end_to_end_test.go has E2E-specific content
	e2ePath := filepath.Join(testDir, "end_to_end_test.go")
	e2eContent, _ := os.ReadFile(e2ePath)
	if !strings.Contains(string(e2eContent), "E2E") {
		t.Errorf("E2E test should mention 'E2E'")
	}
	if !strings.Contains(string(e2eContent), "SKIP_E2E") {
		t.Errorf("E2E test should check SKIP_E2E env var")
	}

	dialtone.LogInfo("All test template contents verified")
}

// TestTestCommandSkipsExistingFiles verifies test command doesn't overwrite existing files
func TestTestCommandSkipsExistingFiles(t *testing.T) {
	projectRoot := getProjectRoot(t)
	testFeatureName := "test-no-overwrite-" + randomSuffix()
	testDir := filepath.Join(projectRoot, "test", testFeatureName)

	// Create test directory and a custom unit_test.go
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	customContent := `package test_no_overwrite

// This is a custom test file that should NOT be overwritten
func TestCustom(t *testing.T) {
	t.Log("Custom test")
}
`
	unitTestPath := filepath.Join(testDir, "unit_test.go")
	if err := os.WriteFile(unitTestPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("Failed to write custom test file: %v", err)
	}

	// Run dialtone-dev test
	cmd := exec.Command("go", "run", "dialtone-dev.go", "test", testFeatureName)
	cmd.Dir = projectRoot
	cmd.CombinedOutput()

	// Verify unit_test.go was NOT overwritten
	content, err := os.ReadFile(unitTestPath)
	if err != nil {
		t.Fatalf("Failed to read unit_test.go: %v", err)
	}

	if !strings.Contains(string(content), "This is a custom test file") {
		t.Errorf("unit_test.go was overwritten! Custom content should be preserved")
	}

	// But integration_test.go and end_to_end_test.go should be created
	integrationPath := filepath.Join(testDir, "integration_test.go")
	if _, err := os.Stat(integrationPath); os.IsNotExist(err) {
		t.Errorf("integration_test.go should have been created")
	}

	e2ePath := filepath.Join(testDir, "end_to_end_test.go")
	if _, err := os.Stat(e2ePath); os.IsNotExist(err) {
		t.Errorf("end_to_end_test.go should have been created")
	}

	dialtone.LogInfo("Verified existing files are not overwritten")
}
