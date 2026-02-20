package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type Step struct {
	Name       string
	Files      []string
	Conditions string
	Run        func() (string, error)
}

func main() {
	ctx := newTestCtx()
	steps := []Step{
		{
			Name: "Test 1: REPL Startup",
			Files: []string{
				"src/plugins/repl/src_v1/test/01.go",
				"dialtone.sh",
			},
			Conditions: "1. `DIALTONE>` should introduce itself and print the help command",
			Run: func() (string, error) {
				return Run01Startup(ctx)
			},
		},
		{
			Name: "Test 2: dev install",
			Files: []string{
				"src/plugins/repl/src_v1/test/02.go",
				"src/plugins/go/install.sh",
				"dialtone.sh",
			},
			Conditions: "1. `USER-1>` should request the install of the latest stable Go runtime at the `env/.env` DIALTONE_ENV path... \n2. `DIALTONE>` should run that command on a subtone",
			Run: func() (string, error) {
				return Run02DevInstall(ctx)
			},
		},
		{
			Name: "Test 3: robot install src_v1",
			Files: []string{
				"src/plugins/repl/src_v1/test/03.go",
				"src/plugins/robot/ops.go",
				"dialtone.sh",
			},
			Conditions: "1. `USER-1>` should request robot install... \n2. `DIALTONE>` should run that command on a subtone",
			Run: func() (string, error) {
				return Run03RobotInstall(ctx)
			},
		},
		{
			Name: "Test 4: dag install src_v3",
			Files: []string{
				"src/plugins/repl/src_v1/test/04.go",
				"src/plugins/dag/cli/install.go",
				"dialtone.sh",
			},
			Conditions: "1. `USER-1>` should request dag install... \n2. `DIALTONE>` should run that command on a subtone",
			Run: func() (string, error) {
				return Run04DagInstall(ctx)
			},
		},
	}

	reportPath := filepath.Join(ctx.repoRoot, "src/plugins/repl/src_v1/test/TEST.md")
	f, err := os.Create(reportPath)
	if err != nil {
		fmt.Printf("Failed to create report: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	for _, s := range steps {
		fmt.Printf("Running %s...\n", s.Name)
		output, err := s.Run()
		
		fmt.Fprintf(f, "# %s\n\n", s.Name)
		
		fmt.Fprintf(f, "### Files\n")
		for _, file := range s.Files {
			fmt.Fprintf(f, "- `%s`\n", file)
		}
		fmt.Fprintf(f, "\n")

		fmt.Fprintf(f, "### Conditions\n")
		fmt.Fprintf(f, "%s\n\n", s.Conditions)

		fmt.Fprintf(f, "### Results\n")
		fmt.Fprintf(f, "```text\n")
		if output != "" {
			fmt.Fprintf(f, "%s\n", output)
		}
		if err != nil {
			fmt.Fprintf(f, "ERROR: %v\n", err)
		}
		fmt.Fprintf(f, "```\n\n")
		
		if err != nil {
			fmt.Printf("%s FAILED: %v\n", s.Name, err)
		} else {
			fmt.Printf("%s PASSED\n", s.Name)
		}
	}
	
	fmt.Printf("Report written to %s\n", reportPath)
}
