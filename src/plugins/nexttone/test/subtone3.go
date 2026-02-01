package main

import (
	"fmt"
	"strings"
)

func TestLoopOverSubtones() error {
	logLine("step", "Loop over subtones")
	if err := resetToneDB(); err != nil {
		return err
	}
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	runCmd("./dialtone.sh", "nexttone", "--sign", "yes")

	first := runCmd("./dialtone.sh", "nexttone")
	if !strings.Contains(first, "DIALTONE [") {
		return fmt.Errorf("expected DIALTONE prompt")
	}
	if !strings.Contains(first, "SUBTONE:") {
		return fmt.Errorf("expected subtone prompt")
	}
	if err := assertState("subtone-review", "alpha"); err != nil {
		return err
	}

	next := runCmd("./dialtone.sh", "nexttone", "--sign", "yes")
	if !strings.Contains(next, "DIALTONE [subtone-run-test]") {
		return fmt.Errorf("expected subtone-run-test prompt")
	}
	if !strings.Contains(next, "TEST RESULT: PASS") {
		return fmt.Errorf("expected subtone test result output")
	}
	if strings.Contains(first, "SUBTONE: alpha") {
		if err := assertState("subtone-run-test", "alpha"); err != nil {
			return err
		}
		if err := assertDBExists(); err != nil {
			return err
		}
		return nil
	}
	if strings.Contains(first, "SUBTONE: beta") {
		if err := assertState("subtone-run-test", "beta"); err != nil {
			return err
		}
		if err := assertDBExists(); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("unexpected subtone prompt content")
}
