package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runTest(args []string) error {
	fs := flag.NewFlagSet("chrome v1 test", flag.ContinueOnError)
	filter := fs.String("filter", "", "Run only tests matching this go test -run pattern")
	integration := fs.Bool("integration", false, "Enable integration tests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 0 {
		return fmt.Errorf("chrome test does not accept positional arguments")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	cliRoot, err := locateCliRoot(repoRoot)
	if err != nil {
		return err
	}

	testArgs := []string{"test", "./..."}
	filterPattern := strings.TrimSpace(*filter)
	if filterPattern != "" {
		if !strings.Contains(filterPattern, "/") {
			filterPattern = "Test.*/" + filterPattern
		}
		testArgs = append(testArgs, "-run", filterPattern)
	}
	if *integration {
		testArgs = append(testArgs, "-args", "-chrome-v1-integration")
	}

	cmd := exec.Command("go", testArgs...)
	cmd.Dir = cliRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
