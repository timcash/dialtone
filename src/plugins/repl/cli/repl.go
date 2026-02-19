package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	repl_test "dialtone/dev/plugins/repl/test"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "test":
		if len(args) > 1 && args[1] == "src_v1" {
			return runVersionedTest("src_v1")
		}
		testFlags := flag.NewFlagSet("repl test", flag.ContinueOnError)
		timeout := testFlags.Int("timeout", 180, "Timeout in seconds for REPL workflow test")
		if err := testFlags.Parse(args[1:]); err != nil {
			return err
		}
		return repl_test.RunInstallWorkflow(*timeout)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown repl command: %s", args[0])
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
