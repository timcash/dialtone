package main

import (
	"fmt"
	"strings"
)

func TestLLMWorkflowGuidance() error {
	logLine("step", "LLM-guided tone workflow")

	if err := resetToneDB(); err != nil {
		return err
	}

	// Microtone 1: set-git-clean
	output := runCmd("./dialtone.sh", "nexttone")
	if !strings.Contains(output, "DIALTONE [set-git-clean]") {
		return fmt.Errorf("expected set-git-clean prompt")
	}
	if !strings.Contains(output, "./dialtone.sh nexttone --sign yes") {
		return fmt.Errorf("expected sign command guidance")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if err := assertState("align-goal-subtone-names", "alpha"); err != nil {
		return err
	}

	// Microtone 2: align goal + subtone names (simulate LLM edit, sign no then yes)
	output = runCmd("./dialtone.sh", "nexttone")
	if !strings.Contains(output, "DIALTONE [align-goal-subtone-names]") {
		return fmt.Errorf("expected align-goal-subtone-names prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "no")
	if !strings.Contains(output, "DIALTONE [align-goal-subtone-names]") {
		return fmt.Errorf("expected same prompt after sign no")
	}
	// LLM performs a small update via CLI
	runCmd("./dialtone.sh", "nexttone", "subtone", "set", "alpha", "--desc", "aligned to goal")
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if err := assertState("review-all-subtones", "alpha"); err != nil {
		return err
	}

	// Microtone 3: review-all-subtones
	if !strings.Contains(output, "DIALTONE [review-all-subtones]") {
		return fmt.Errorf("expected review-all-subtones prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if err := assertState("subtone-review", "alpha"); err != nil {
		return err
	}

	// Microtone 4: subtone-review -> run-test loop
	if !strings.Contains(output, "DIALTONE [subtone-review]") {
		return fmt.Errorf("expected subtone-review prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [subtone-run-test]") {
		return fmt.Errorf("expected subtone-run-test prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "DIALTONE [subtone-review]") {
		return fmt.Errorf("expected subtone-review loop prompt")
	}

	// Complete flow
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // beta -> run-test
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // run-test -> review-complete
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // start-complete-phase
	if !strings.Contains(output, "start-complete-phase") {
		return fmt.Errorf("expected start-complete-phase prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // confirm-pr-merged
	if !strings.Contains(output, "confirm-pr-merged") {
		return fmt.Errorf("expected confirm-pr-merged prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes") // complete
	if !strings.Contains(output, "DIALTONE [complete]") {
		return fmt.Errorf("expected complete prompt")
	}
	output = runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(output, "COMPLETE!") {
		return fmt.Errorf("expected COMPLETE confirmation")
	}
	return nil
}
