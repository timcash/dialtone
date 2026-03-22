package main

import (
	"errors"
	"strings"
	"testing"
	"time"
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

func TestBuildQuitScriptForceKillsGhostty(t *testing.T) {
	script := buildQuitScript()
	if !strings.Contains(script, `pkill -9 -x ghostty`) {
		t.Fatalf("quit script missing force-kill command: %q", script)
	}
	if !strings.Contains(script, `/Applications/Ghostty.app/Contents/MacOS/ghostty`) {
		t.Fatalf("quit script missing process-path fallback: %q", script)
	}
	if strings.Contains(script, `tell application "Ghostty"`) {
		t.Fatalf("quit script should bypass app-level quit prompts: %q", script)
	}
}

func TestFreshWindowResetsBeforeOpeningNewWindow(t *testing.T) {
	t.Cleanup(func() {
		ghosttyAppleScriptRunner = runAppleScript
		ghosttySleep = time.Sleep
	})

	var scripts []string
	var slept time.Duration
	ghosttyAppleScriptRunner = func(script string) (string, error) {
		scripts = append(scripts, script)
		switch len(scripts) {
		case 1:
			if script != buildQuitScript() {
				t.Fatalf("first script should be quit script, got %q", script)
			}
			return "", nil
		case 2:
			if script != buildNewWindowScript(ghosttySurfaceConfig{WorkingDirectory: "/Users/user/dialtone"}) {
				t.Fatalf("second script should be new window script, got %q", script)
			}
			return "win-1\ttab-1\tterm-1", nil
		default:
			t.Fatalf("unexpected extra AppleScript call %d: %q", len(scripts), script)
			return "", nil
		}
	}
	ghosttySleep = func(d time.Duration) {
		slept = d
	}

	windowID, tabID, terminalID, err := freshWindow(ghosttySurfaceConfig{WorkingDirectory: "/Users/user/dialtone"})
	if err != nil {
		t.Fatalf("freshWindow returned error: %v", err)
	}
	if windowID != "win-1" || tabID != "tab-1" || terminalID != "term-1" {
		t.Fatalf("unexpected freshWindow result: %q %q %q", windowID, tabID, terminalID)
	}
	if len(scripts) != 2 {
		t.Fatalf("expected 2 AppleScript calls, got %d", len(scripts))
	}
	if slept != 500*time.Millisecond {
		t.Fatalf("expected 500ms reset delay, got %v", slept)
	}
}

func TestFreshWindowContinuesAfterBestEffortResetFailure(t *testing.T) {
	t.Cleanup(func() {
		ghosttyAppleScriptRunner = runAppleScript
		ghosttySleep = time.Sleep
	})

	var scripts []string
	ghosttyAppleScriptRunner = func(script string) (string, error) {
		scripts = append(scripts, script)
		if len(scripts) == 1 {
			return "", errors.New("pkill failed")
		}
		return "win-2\ttab-2\tterm-2", nil
	}
	ghosttySleep = func(time.Duration) {}

	windowID, tabID, terminalID, err := freshWindow(ghosttySurfaceConfig{})
	if err != nil {
		t.Fatalf("freshWindow should continue after reset failure: %v", err)
	}
	if windowID != "win-2" || tabID != "tab-2" || terminalID != "term-2" {
		t.Fatalf("unexpected freshWindow result after reset failure: %q %q %q", windowID, tabID, terminalID)
	}
	if len(scripts) != 2 {
		t.Fatalf("expected quit and new-window scripts, got %d calls", len(scripts))
	}
	if scripts[0] != buildQuitScript() {
		t.Fatalf("first script should be quit script, got %q", scripts[0])
	}
	if scripts[1] != buildNewWindowScript(ghosttySurfaceConfig{}) {
		t.Fatalf("second script should be new-window script, got %q", scripts[1])
	}
}

func TestFreshWindowReturnsNewWindowLaunchError(t *testing.T) {
	t.Cleanup(func() {
		ghosttyAppleScriptRunner = runAppleScript
		ghosttySleep = time.Sleep
	})

	wantErr := errors.New("launch failed")
	ghosttyAppleScriptRunner = func(script string) (string, error) {
		if script == buildQuitScript() {
			return "", nil
		}
		return "", wantErr
	}
	ghosttySleep = func(time.Duration) {}

	_, _, _, err := freshWindow(ghosttySurfaceConfig{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected new-window error, got %v", err)
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
