package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "test":
		version := "src_v1"
		var extraArgs []string
		if len(args) > 1 {
			// Check if arg 1 is version or subtest
			if strings.HasPrefix(args[1], "src_v") {
				version = args[1]
				if len(args) > 2 {
					extraArgs = args[2:]
				}
			} else {
				// Arg 1 is subtest, assume default version
				extraArgs = args[1:]
			}
		}
		if err := runVersionedTest(version, extraArgs); err != nil {
			fmt.Printf("REPL test error: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Printf("Unknown repl command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func runVersionedTest(versionDir string, args []string) error {
	cwd, _ := os.Getwd()
	root := cwd
	for {
		if _, err := os.Stat(filepath.Join(root, "dialtone.sh")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			root = cwd
			break
		}
		root = parent
	}
	
	testPkg := "./plugins/repl/" + versionDir + "/test/01_bootstrap"
	// Pass remaining args to the test runner
	goArgs := append([]string{"exec", "run", testPkg}, args...)
	fullArgs := append([]string{"go"}, goArgs...)
	cmd := exec.Command(filepath.Join(root, "dialtone.sh"), fullArgs...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh repl <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  test [src_v1]            Run REPL workflow tests")
	fmt.Println("  help                     Show this help")
}
