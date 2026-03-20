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

func TestBuildNewTabScriptIncludesSurfaceConfigurationAndSelect(t *testing.T) {
	script := buildNewTabScript(ghosttySurfaceConfig{
		WorkingDirectory: "/Users/user/dialtone",
		Command:          "tmux new-session -A -s codex-view",
		InitialInput:     "echo ready",
	})
	if !strings.Contains(script, `set cfg to new surface configuration`) {
		t.Fatalf("new-tab script missing surface configuration: %q", script)
	}
	if !strings.Contains(script, `set initial working directory of cfg to "/Users/user/dialtone"`) {
		t.Fatalf("new-tab script missing cwd: %q", script)
	}
	if !strings.Contains(script, `set command of cfg to "tmux new-session -A -s codex-view"`) {
		t.Fatalf("new-tab script missing command: %q", script)
	}
	if !strings.Contains(script, `set initial input of cfg to "echo ready"`) {
		t.Fatalf("new-tab script missing initial input: %q", script)
	}
	if !strings.Contains(script, `set newTab to new tab in win with configuration cfg`) {
		t.Fatalf("new-tab script missing new tab command: %q", script)
	}
	if !strings.Contains(script, `select tab newTab`) {
		t.Fatalf("new-tab script missing select tab command: %q", script)
	}
}

func TestBuildNewWindowScriptCanOmitConfiguration(t *testing.T) {
	script := buildNewWindowScript(ghosttySurfaceConfig{})
	if !strings.Contains(script, `set newWin to new window`) {
		t.Fatalf("new-window script missing new window command: %q", script)
	}
	if strings.Contains(script, `with configuration cfg`) {
		t.Fatalf("new-window script should not include configuration: %q", script)
	}
	if !strings.Contains(script, `activate window newWin`) {
		t.Fatalf("new-window script missing activate window command: %q", script)
	}
}

func TestBuildQuitScriptQuitsGhostty(t *testing.T) {
	script := buildQuitScript()
	if !strings.Contains(script, `quit`) {
		t.Fatalf("quit script missing quit command: %q", script)
	}
}

func TestBuildSplitScriptTargetsDirectionAndCanFocusNewTerminal(t *testing.T) {
	script := buildSplitScript(2, "down", true, ghosttySurfaceConfig{
		WorkingDirectory: "/Users/user/dialtone",
	})
	if !strings.Contains(script, `set targetTerm to terminal 2 of tabRef`) {
		t.Fatalf("split script missing target terminal: %q", script)
	}
	if !strings.Contains(script, `set newTerm to split targetTerm direction down with configuration cfg`) {
		t.Fatalf("split script missing split command: %q", script)
	}
	if !strings.Contains(script, `focus newTerm`) {
		t.Fatalf("split script missing focus command: %q", script)
	}
}

func TestBuildFocusScriptTargetsRequestedTerminal(t *testing.T) {
	script := buildFocusScript(3)
	if !strings.Contains(script, `set targetTerm to terminal 3 of tabRef`) {
		t.Fatalf("focus script missing target terminal: %q", script)
	}
	if !strings.Contains(script, `focus targetTerm`) {
		t.Fatalf("focus script missing focus command: %q", script)
	}
}

func TestBuildFullscreenScriptTargetsFrontWindow(t *testing.T) {
	script := buildFullscreenScript(true)
	if !strings.Contains(script, `set win to front window`) {
		t.Fatalf("fullscreen script missing front window target: %q", script)
	}
	if !strings.Contains(script, `set value of attribute "AXFullScreen" of window 1 to true`) {
		t.Fatalf("fullscreen script missing fullscreen enable: %q", script)
	}
}

func TestParseCreatedTabResult(t *testing.T) {
	tabID, tabIndex, terminalID, err := parseCreatedTabResult("tab-1\t2\tterm-9")
	if err != nil {
		t.Fatalf("parseCreatedTabResult returned error: %v", err)
	}
	if tabID != "tab-1" || tabIndex != 2 || terminalID != "term-9" {
		t.Fatalf("unexpected parseCreatedTabResult values: %q %d %q", tabID, tabIndex, terminalID)
	}
}

func TestParseCreatedWindowResult(t *testing.T) {
	windowID, tabID, terminalID, err := parseCreatedWindowResult("win-1\ttab-1\tterm-1")
	if err != nil {
		t.Fatalf("parseCreatedWindowResult returned error: %v", err)
	}
	if windowID != "win-1" || tabID != "tab-1" || terminalID != "term-1" {
		t.Fatalf("unexpected parseCreatedWindowResult values: %q %q %q", windowID, tabID, terminalID)
	}
}

func TestIsValidSplitDirection(t *testing.T) {
	valid := []string{"right", "left", "down", "up"}
	for _, value := range valid {
		if !isValidSplitDirection(value) {
			t.Fatalf("expected valid direction %q", value)
		}
	}
	if isValidSplitDirection("sideways") {
		t.Fatalf("unexpected valid direction")
	}
}
