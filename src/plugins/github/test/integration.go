package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	fmt.Println("=== Starting GitHub Plugin Integration Test ===")

	allPassed := true
	runTest := func(name string, fn func() error) {
		fmt.Printf("\n[TEST] %s\n", name)
		if err := fn(); err != nil {
			fmt.Printf("FAIL: %s - %v\n", name, err)
			allPassed = false
		} else {
			fmt.Printf("PASS: %s\n", name)
		}
	}

	defer func() {
		fmt.Println("\n=== Integration Tests Completed ===")
		if !allPassed {
			fmt.Println("\n!!! SOME TESTS FAILED !!!")
			os.Exit(1)
		}
	}()

	runTest("Full PR Lifecycle Workflow", TestGHFullWorkflow)
}

func TestGHFullWorkflow() error {
	ts := time.Now().Unix()
	testBranch := fmt.Sprintf("test-github-plugin-%d", ts)

	// --- SETUP: Save current branch ---
	fmt.Println("--- SETUP: Save current branch ---")
	cmd := exec.Command("git", "branch", "--show-current")
	origBranchOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %v", err)
	}
	origBranch := strings.TrimSpace(string(origBranchOutput))
	fmt.Printf("Original branch: %s\n", origBranch)

	defer func() {
		fmt.Printf("\n--- CLEANUP: Returning to %s ---\n", origBranch)
		runCmd("git", "checkout", origBranch)
		runCmd("git", "branch", "-D", testBranch)
		// Remote branch cleanup is done in the test if it reaches there,
		// but we should attempt it here too if not successful.
		exec.Command("git", "push", "origin", "--delete", testBranch).Run()
	}()

	// --- STEP 1: Create Branch ---
	fmt.Println("\n--- STEP 1: Create Branch ---")
	runCmd("./dialtone.sh", "branch", testBranch)

	// --- STEP 2: Make Change & Commit ---
	fmt.Println("\n--- STEP 2: Make Change & Commit ---")
	testFile := "github_test_marker.txt"
	content := fmt.Sprintf("Test change at %v", time.Now())
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	runCmd("git", "add", testFile)
	runCmd("git", "commit", "-m", "test: github plugin integration test")

	// --- STEP 3: Push Branch ---
	fmt.Println("\n--- STEP 3: Push Branch ---")
	runCmd("git", "push", "-u", "origin", testBranch)

	// --- STEP 4: Create Draft PR ---
	fmt.Println("\n--- STEP 4: Create Draft PR ---")
	output := runCmd("./dialtone.sh", "github", "pr", "create", "--draft", "--title", "Test PR (Draft)", "--body", "This is an automated test PR.")
	if !strings.Contains(output, "https://github.com/") {
		return fmt.Errorf("failed to create PR or get PR URL")
	}

	// --- STEP 5: Mark Ready ---
	fmt.Println("\n--- STEP 5: Mark Ready ---")
	runCmd("./dialtone.sh", "github", "pr", "create", "--ready")

	// --- STEP 6: Close PR ---
	fmt.Println("\n--- STEP 6: Close PR ---")
	runCmd("./dialtone.sh", "github", "pr", "close")

	// --- STEP 7: Delete Remote Branch ---
	fmt.Println("\n--- STEP 7: Delete Remote Branch ---")
	runCmd("git", "push", "origin", "--delete", testBranch)

	return nil
}

func runCmd(name string, args ...string) string {
	fmt.Printf("> %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	output, _ := cmd.CombinedOutput()
	fmt.Println(string(output))
	return string(output)
}
