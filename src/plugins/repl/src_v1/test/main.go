package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type Step struct {
	Name       string
	Conditions string
	Run        func() (string, error)
}

func main() {
	ctx := newTestCtx()
	steps := []Step{
		{
			Name: "Test 1: REPL Startup",
			Conditions: "1. `DIALTONE>` should introduce itself and print the help command",
			Run: func() (string, error) {
				return Run01Startup(ctx)
			},
		},
		{
			Name: "Test 2: dev install",
			Conditions: "1. `USER-1>` should request the install of the latest stable Go runtime at the `env/.env` DIALTONE_ENV path... \n2. `DIALTONE>` should run that command on a subtone",
			Run: func() (string, error) {
				return Run02DevInstall(ctx)
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
		
		fmt.Fprintf(f, "# %s\n", s.Name)
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
			// We continue to run all steps to see results in TEST.md
		} else {
			fmt.Printf("%s PASSED\n", s.Name)
		}
	}
	
	fmt.Printf("Report written to %s\n", reportPath)
}
