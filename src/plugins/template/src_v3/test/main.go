package main

import (
	"fmt"
	"os"
)

type step struct {
	name string
	fn   func() error
}

func main() {
	steps := []step{
		{name: "01 Go Format", fn: Run01GoFormat},
		{name: "02 Go Vet", fn: Run02GoVet},
		{name: "03 Go Build", fn: Run03GoBuild},
		{name: "04 UI Lint", fn: Run04UILint},
		{name: "05 UI Format", fn: Run05UIFormat},
		{name: "06 UI Build", fn: Run06UIBuild},
		{name: "07 Go Run", fn: Run07GoRun},
		{name: "08 UI Run", fn: Run08UIRun},
		{name: "09 Expected Errors (Proof of Life)", fn: Run09ExpectedErrorsProofOfLife},
		{name: "10 Dev Server Running (latest UI)", fn: Run10DevServerRunningLatestUI},
		{name: "11 Hero Section Validation", fn: Run11HeroSectionValidation},
		{name: "12 Docs Section Validation", fn: Run12DocsSectionValidation},
		{name: "13 Table Section Validation", fn: Run13TableSectionValidation},
		{name: "14 Three Section Validation", fn: Run14ThreeSectionValidation},
		{name: "15 Xterm Section Validation", fn: Run15XtermSectionValidation},
		{name: "16 Video Section Validation", fn: Run16VideoSectionValidation},
		{name: "17 Lifecycle / Invariants", fn: Run17LifecycleInvariants},
		{name: "18 Cleanup Verification", fn: Run18CleanupVerification},
	}

	for _, s := range steps {
		fmt.Printf("[TEST] START %s\n", s.name)
		if err := s.fn(); err != nil {
			fmt.Printf("[TEST] FAIL  %s: %v\n", s.name, err)
			os.Exit(1)
		}
		fmt.Printf("[TEST] PASS  %s\n", s.name)
	}

	fmt.Println("[TEST] COMPLETE")
}
