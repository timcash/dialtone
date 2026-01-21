package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestE2E_CreateAndCleanupPR verifies that we can create a PR and then clean it up.
// This test performs real Git and GitHub operations.
func TestE2E_CreateAndCleanupPR(t *testing.T) {
	// 1. Check if we should run this test
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test because SKIP_E2E is set")
	}

	// Check if gh is installed and authenticated
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not found, skipping E2E test")
	}
	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		t.Skip("gh CLI not authenticated, skipping E2E test")
	}

	t.Log("Starting E2E PR creation and cleanup test")

	// 2. Setup: Create a unique branch and commit
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("e2e-test-pr-%s", timestamp)
	
	// Get current branch to return to later
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get current branch: %v", err)
	}
	originalBranch := strings.TrimSpace(string(output))
	
	// Checkout new branch
	if err := exec.Command("git", "checkout", "-b", branchName).Run(); err != nil {
		t.Fatalf("Failed to create branch %s: %v", branchName, err)
	}
	defer func() {
		// Cleanup: Checkout original branch and delete test branch
		exec.Command("git", "checkout", originalBranch).Run()
		exec.Command("git", "branch", "-D", branchName).Run()
	}()

	// Create an empty commit so we have something to push
	if err := exec.Command("git", "commit", "--allow-empty", "-m", "E2E Test Commit").Run(); err != nil {
		t.Fatalf("Failed to create commit: %v", err)
	}

	// 3. Run functionality: dialtone-dev github pull-request
	// We locate dialtone-dev.go relative to this test file.
	// This file is in src/plugins/github/test/
	// dialtone-dev.go is in src/
	// So we need to go up 3 levels: ../../../
	
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "..", "..", "..") // src/plugins/github/test -> src/plugins/github -> src/plugins -> src
	
	// Verify dialtone-dev.go exists
	devGo := filepath.Join(projectRoot, "dialtone-dev.go")
	if _, err := os.Stat(devGo); os.IsNotExist(err) {
		// Try one more level up if cwd logic is tricky or if run from root
		// If running from root, cwd is root.
		if _, err := os.Stat("dialtone-dev.go"); err == nil {
			projectRoot = "."
		} else {
             // Try standard relative path from repo root
             projectRoot = "../../../.."
        }
	}
    
    // Harder to reliably find project root from test execution context without more info,
    // assuming standard execution from repo root:
    // dialtone-dev.go is at root of repo (based on previous view_file of dev.go location? No, dev.go is in src/dev.go, dialtone-dev.go is likely at root based on usage in docs)
    // Wait, dev.go is in `src/dev.go`. `dialtone-dev.go` is usually the wrapper at the top.
    // Let's assume we can run `go run src/dev.go` if we are at repo root.
    // If the test runner runs from package dir, we need to find repo root.
    
    // Let's look for "dialtone-dev.go" by walking up.
    root := findRepoRoot(t)
    
	t.Logf("Repo root determined as: %s", root)

	// Command: go run dialtone-dev.go github pull-request --title "E2E Test PR" --body "This is an auto-generated test PR."
    // Note: gh pr create might prompt for push. We should probably push first to be safe, OR rely on gh behavior but it might be interactive.
    // `gh pr create` has `--head` but typically it pushes for you if you answer yes.
    // To make it non-interactive, `gh` usually handles it if we push first.
    
    // Push the branch first
    t.Log("Pushing branch to remote...")
    if err := exec.Command("git", "push", "origin", branchName).Run(); err != nil {
        t.Fatalf("Failed to push branch: %v. Ensure you have write access and origin is set.", err)
    }
    // Defer remote branch deletion
    defer func() {
        exec.Command("git", "push", "origin", "--delete", branchName).Run()
    }()

	cmd = exec.Command("go", "run", "dialtone-dev.go", "github", "pull-request", 
		"--title", fmt.Sprintf("E2E Test PR %s", timestamp),
		"--body", "This is an auto-generated test PR from dialtone-dev E2E test.")
	cmd.Dir = root
	
	t.Log("Running dialtone-dev github pull-request...")
	out, err := cmd.CombinedOutput()
	outputStr := string(out)
	t.Logf("Output: %s", outputStr)
	
	if err != nil {
		t.Fatalf("dialtone-dev command failed: %v", err)
	}

	// 4. Verify: Check if PR was created
	// Output should contain URL like https://github.com/org/repo/pull/123
	re := regexp.MustCompile(`https://github\.com/.+/pull/(\d+)`)
	matches := re.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		t.Fatalf("Could not find PR URL in output. Output was:\n%s", outputStr)
	}
	prNumber := matches[1]
	t.Logf("Created PR #%s", prNumber)

	// 5. Cleanup: Close PR
	t.Logf("Closing PR #%s...", prNumber)
	if err := exec.Command("gh", "pr", "close", prNumber, "--delete-branch").Run(); err != nil {
		t.Logf("Failed to close PR (might already be closed or logic error): %v", err)
		// We already handle local branch deletion in defer, and remote branch deletion in defer
	} else {
		t.Log("PR closed successfully")
	}
}

func findRepoRoot(t *testing.T) string {
    dir, _ := os.Getwd()
    for {
        if _, err := os.Stat(filepath.Join(dir, "dialtone-dev.go")); err == nil {
            return dir
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            break
        }
        dir = parent
    }
    // Fallback/Fail
    t.Fatalf("Could not find repo root (containing dialtone-dev.go)")
    return ""
}
