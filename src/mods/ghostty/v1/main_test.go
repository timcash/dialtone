package main

import (
	"strings"
	"testing"
)

func TestBuildListScriptUsesFocusedTerminalAndTabTerminals(t *testing.T) {
	script := buildListScript()
	if !strings.Contains(script, `selected tab of win`) {
		t.Fatalf("list script does not target selected tab: %q", script)
	}
	if !strings.Contains(script, `focused terminal of tabRef`) {
		t.Fatalf("list script does not use focused terminal: %q", script)
	}
	if !strings.Contains(script, `terminals of tabRef`) {
		t.Fatalf("list script does not iterate terminals: %q", script)
	}
}

func TestBuildWriteScriptTargetsRequestedTerminal(t *testing.T) {
	script := buildWriteScript(2, "echo hi", true, false)
	if !strings.Contains(script, `terminal 2 of tabRef`) {
		t.Fatalf("write script does not target terminal 2: %q", script)
	}
	if !strings.Contains(script, `input text "echo hi" to targetTerm`) {
		t.Fatalf("write script missing input text command: %q", script)
	}
	if !strings.Contains(script, `send key "enter" to targetTerm`) {
		t.Fatalf("write script missing enter key: %q", script)
	}
}

func TestBuildWriteScriptCanSkipEnterAndFocusTarget(t *testing.T) {
	script := buildWriteScript(1, "pwd", false, true)
	if !strings.Contains(script, `focus targetTerm`) {
		t.Fatalf("write script missing focus command: %q", script)
	}
	if strings.Contains(script, `send key "enter" to targetTerm`) {
		t.Fatalf("write script should not send enter: %q", script)
	}
}
