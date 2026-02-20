package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	repl_test "dialtone/dev/plugins/repl/test"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "test":
		if len(args) > 1 && args[1] == "src_v1" {
			if err := runVersionedTest("src_v1"); err != nil {
				fmt.Printf("REPL test error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		testFlags := flag.NewFlagSet("repl test", flag.ContinueOnError)
		timeout := testFlags.Int("timeout", 180, "Timeout in seconds for REPL workflow test")
		if err := testFlags.Parse(args[1:]); err != nil {
			fmt.Printf("REPL test error: %v\n", err)
			os.Exit(1)
		}
		if err := repl_test.RunInstallWorkflow(*timeout); err != nil {
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

func runVersionedTest(versionDir string) error {
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
	
	testPkg := "./plugins/repl/" + versionDir + "/test"
	cmd := exec.Command(filepath.Join(root, "dialtone.sh"), "go", "exec", "run", testPkg)
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
