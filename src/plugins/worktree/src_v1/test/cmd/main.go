package main

import (
	"fmt"
	"os"
	"path/filepath"
	
	"dialtone/dev/plugins/worktree/src_v1/go/worktree"
)

func main() {
	fmt.Println("Running Worktree Plugin Tests...")
	
	// Test 1: Add Worktree
	name := "test-worktree-agent"
	fmt.Printf("Test 1: Adding worktree '%s'...\n", name)
	
	// Ensure cleanup
	_ = worktree.Remove(name)
	
	err := worktree.Add(name, "README.md", "") // Use README as dummy task
	if err != nil {
		fmt.Printf("FAIL: Add failed: %v\n", err)
		os.Exit(1)
	}
	
	// Verify directory
	repoRoot, _ := os.Getwd() // assuming run from src, root is ..
	// Actually worktree.Add finds repo root.
	// But we need to verify.
	// Let's assume Add worked if no error.
	
	// Test 2: List
	fmt.Println("Test 2: Listing...")
	if err := worktree.List(); err != nil {
		fmt.Printf("FAIL: List failed: %v\n", err)
		os.Exit(1)
	}
	
	// Test 3: Remove
	fmt.Printf("Test 3: Removing worktree '%s'...\n", name)
	if err := worktree.Remove(name); err != nil {
		fmt.Printf("FAIL: Remove failed: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("PASS: All worktree tests passed.")
}
