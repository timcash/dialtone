package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
)

func TestParseStartArgsRejectsPositionalArgs(t *testing.T) {
	_, err := parseStartArgs([]string{"extra"})
	if err == nil {
		t.Fatalf("expected positional args to be rejected")
	}
	if !strings.Contains(err.Error(), "does not accept positional arguments") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseStartArgsRejectsBlankSession(t *testing.T) {
	_, err := parseStartArgs([]string{"--session", "   "})
	if err == nil {
		t.Fatalf("expected blank session to be rejected")
	}
	if !strings.Contains(err.Error(), "--session is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseStartArgsRejectsNonPositiveWaitSeconds(t *testing.T) {
	_, err := parseStartArgs([]string{"--wait-seconds", "0"})
	if err == nil {
		t.Fatalf("expected non-positive wait-seconds to be rejected")
	}
	if !strings.Contains(err.Error(), "--wait-seconds must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFormatArgsKeepsBlankDirEmpty(t *testing.T) {
	got, err := parseFormatArgs(nil)
	if err != nil {
		t.Fatalf("parseFormatArgs returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseFormatArgs(nil) = %q, want empty string so format defaults to src/mods/test/v1", got)
	}
}

func TestBuildPromptTextUsesTokenByDefault(t *testing.T) {
	scenario := buildCodexAgentScenario("TOKEN_123")
	got := buildPromptText("TOKEN_123", "", scenario)
	if !strings.Contains(got, "TOKEN_123") {
		t.Fatalf("expected token in default prompt, got %q", got)
	}
	if !strings.Contains(got, dispatch.BuildDialtoneCommand(scenario.Args)) {
		t.Fatalf("expected exact Codex command in default prompt, got %q", got)
	}
	if !strings.Contains(got, "./dialtone_mod mods v1 probe --mode fail --label TEST_FAILURE") {
		t.Fatalf("expected routed example commands in default prompt, got %q", got)
	}
}

func TestBuildPromptTextPreservesExplicitPrompt(t *testing.T) {
	scenario := buildCodexAgentScenario("TOKEN_123")
	got := buildPromptText("TOKEN_123", "custom prompt", scenario)
	if !strings.Contains(got, "Dialtone test v1 start TOKEN_123") {
		t.Fatalf("expected prompt headline in custom prompt, got %q", got)
	}
	if !strings.Contains(got, "custom prompt") {
		t.Fatalf("unexpected prompt text: %q", got)
	}
	if !strings.Contains(got, dispatch.BuildDialtoneCommand(scenario.Args)) {
		t.Fatalf("expected Codex verification command in custom prompt, got %q", got)
	}
}

func TestBuildCodexStartCommandTargetsPromptPane(t *testing.T) {
	got := buildCodexStartCommand("codex-view", "codex-view:0:0", "gpt-5.4", "medium")
	if !strings.Contains(got, "./dialtone_mod codex v1 start") {
		t.Fatalf("expected codex start command, got %q", got)
	}
	if !strings.Contains(got, "--session codex-view") {
		t.Fatalf("expected session flag, got %q", got)
	}
	if !strings.Contains(got, "--pane codex-view:0:0") {
		t.Fatalf("expected prompt pane target, got %q", got)
	}
	if !strings.Contains(got, "--reasoning medium") || !strings.Contains(got, "--model gpt-5.4") {
		t.Fatalf("expected reasoning/model flags, got %q", got)
	}
}

func TestParseCommandIDReadsRouteReport(t *testing.T) {
	out := "route\tqueued\ncommand_id\t948\ninspect\t./dialtone_mod shell v1 status --row-id 948 --full --sync=false\n"
	got, err := parseCommandID(out)
	if err != nil {
		t.Fatalf("parseCommandID returned error: %v", err)
	}
	if got != 948 {
		t.Fatalf("expected command id 948, got %d", got)
	}
}

func TestParseBracketRowIDReadsShellCLIRow(t *testing.T) {
	out := "submitted prompt via shell bus [row_id=951]"
	got, err := parseBracketRowID(out)
	if err != nil {
		t.Fatalf("parseBracketRowID returned error: %v", err)
	}
	if got != 951 {
		t.Fatalf("expected row id 951, got %d", got)
	}
}

func TestValidateE2EOutputsRequiresHealthyState(t *testing.T) {
	results := []routedScenarioResult{
		{
			Scenario: buildCodexAgentScenario("prompt token"),
			RowID:    901,
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command: commandContext{
				target:    "codex-view:0:1",
				output:    "probe_mode\tsuccess\nprobe_label\tCODEX_AGENT_prompt_token\nprobe_result\tsuccess\n",
				exitCode:  0,
				runtimeMS: 125,
			},
		},
		{
			Scenario: routedScenario{Name: "graph"},
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command: commandContext{
				target:    "codex-view:0:1",
				output:    "- shell:v1\n- tsnet:v1\n",
				exitCode:  0,
				runtimeMS: 321,
			},
		},
		{
			Scenario:        routedScenario{Name: "long_running"},
			Record:          modstate.ShellBusRecord{Status: "done"},
			RunningObserved: true,
			Command: commandContext{
				target:    "codex-view:0:1",
				output:    "probe_mode\tsleep\nprobe_result\tsuccess\n",
				exitCode:  0,
				runtimeMS: 1600,
			},
		},
		{
			Scenario: routedScenario{Name: "failing"},
			Record:   modstate.ShellBusRecord{Status: "failed"},
			Command: commandContext{
				target:    "codex-view:0:1",
				output:    "probe_mode\tfail\nprobe_result\tfailure\n",
				exitCode:  1,
				errorText: "requested failure",
			},
		},
		{
			Scenario: routedScenario{Name: "invalid_mode"},
			Record:   modstate.ShellBusRecord{Status: "failed"},
			Command: commandContext{
				target:    "codex-view:0:1",
				exitCode:  1,
				errorText: `unsupported --mode "invalid"`,
			},
		},
		{
			Scenario: routedScenario{Name: "recovery"},
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command: commandContext{
				target:   "codex-view:0:1",
				output:   "probe_mode\tsuccess\nprobe_result\tsuccess\n",
				exitCode: 0,
			},
		},
		{
			Scenario:       routedScenario{Name: "background"},
			Record:         modstate.ShellBusRecord{Status: "done"},
			BackgroundText: "probe_background_done\tTEST_BACKGROUND\nprobe_label\tTEST_BACKGROUND\n",
			Command: commandContext{
				target:   "codex-view:0:1",
				output:   "probe_mode\tbackground\nprobe_label\tTEST_BACKGROUND\nprobe_result\tbackground-started\n",
				exitCode: 0,
			},
		},
	}
	err := validateE2EOutputs(
		"prompt token",
		"codex-view:0:0",
		"codex-view:0:1",
		strings.Join([]string{
			"role\tprompt",
			"text",
			"Dialtone test v1 start",
			"prompt token",
			"./dialtone_mod mods v1 probe --mode success --label CODEX_AGENT_prompt_token",
			"1. ./dialtone_mod mods v1 db graph --format outline",
			"2. ./dialtone_mod mods v1 probe --mode sleep --sleep-ms 1500 --label TEST_LONG_RUNNING",
			"3. ./dialtone_mod mods v1 probe",
			"--mode fail",
			"--label TEST_FAILURE",
			"4. ./dialtone_mod mods v1 probe --mode invalid --label TEST_INVALID_MODE",
			"5. ./dialtone_mod mods v1 probe --mode background --sleep-ms 4000 --label TEST_BACKGROUND",
			"--background-file <marker-",
			"file>",
		}, "\n"),
		"role\tcommand\ntext\nprobe_label\tTEST_BACKGROUND\nprobe_result\tbackground-started\n",
		"prompt_target\tcodex-view:0:0\ncommand_target\tcodex-view:0:1\ndialtone_status\trunning\nworker_status\trunning\nworker_pane\tcodex-view:0:1\n",
		results,
	)
	if err != nil {
		t.Fatalf("validateE2EOutputs returned error: %v", err)
	}
}

func TestValidateE2EOutputsRejectsMissingPrompt(t *testing.T) {
	results := []routedScenarioResult{
		{
			Scenario: buildCodexAgentScenario("prompt token"),
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command:  commandContext{target: "codex-view:0:1", output: "probe_label\tCODEX_AGENT_prompt_token\n", exitCode: 0},
		},
		{
			Scenario: routedScenario{Name: "graph"},
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command:  commandContext{target: "codex-view:0:1", output: "- shell:v1\n", exitCode: 0},
		},
		{
			Scenario:        routedScenario{Name: "long_running"},
			Record:          modstate.ShellBusRecord{Status: "done"},
			RunningObserved: true,
			Command:         commandContext{target: "codex-view:0:1", output: "probe_mode\tsleep\n", exitCode: 0},
		},
		{
			Scenario: routedScenario{Name: "failing"},
			Record:   modstate.ShellBusRecord{Status: "failed"},
			Command:  commandContext{target: "codex-view:0:1", output: "probe_mode\tfail\n", exitCode: 1, errorText: "requested failure"},
		},
		{
			Scenario: routedScenario{Name: "invalid_mode"},
			Record:   modstate.ShellBusRecord{Status: "failed"},
			Command:  commandContext{target: "codex-view:0:1", exitCode: 1, errorText: `unsupported --mode "invalid"`},
		},
		{
			Scenario: routedScenario{Name: "recovery"},
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command:  commandContext{target: "codex-view:0:1", output: "probe_mode\tsuccess\n", exitCode: 0},
		},
		{
			Scenario:       routedScenario{Name: "background"},
			Record:         modstate.ShellBusRecord{Status: "done"},
			BackgroundText: "probe_background_done\tTEST_BACKGROUND\n",
			Command:        commandContext{target: "codex-view:0:1", output: "probe_label\tTEST_BACKGROUND\n", exitCode: 0},
		},
	}
	err := validateE2EOutputs(
		"prompt token",
		"codex-view:0:0",
		"codex-view:0:1",
		"role\tprompt\ntext\nother prompt\n",
		"role\tcommand\ntext\nprobe_label\tTEST_BACKGROUND\n",
		"prompt_target\tcodex-view:0:0\ncommand_target\tcodex-view:0:1\ndialtone_status\trunning\nworker_status\trunning\nworker_pane\tcodex-view:0:1\n",
		results,
	)
	if err == nil || !strings.Contains(err.Error(), "prompt pane read") {
		t.Fatalf("expected missing prompt failure, got %v", err)
	}
}

func TestValidateE2EOutputsRejectsPromptLeakIntoCommandPane(t *testing.T) {
	results := []routedScenarioResult{
		{
			Scenario: buildCodexAgentScenario("prompt token"),
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command:  commandContext{target: "codex-view:0:1", output: "probe_label\tCODEX_AGENT_prompt_token\n", exitCode: 0},
		},
		{
			Scenario: routedScenario{Name: "graph"},
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command:  commandContext{target: "codex-view:0:1", output: "- shell:v1\n", exitCode: 0},
		},
		{
			Scenario:        routedScenario{Name: "long_running"},
			Record:          modstate.ShellBusRecord{Status: "done"},
			RunningObserved: true,
			Command:         commandContext{target: "codex-view:0:1", output: "probe_mode\tsleep\n", exitCode: 0},
		},
		{
			Scenario: routedScenario{Name: "failing"},
			Record:   modstate.ShellBusRecord{Status: "failed"},
			Command:  commandContext{target: "codex-view:0:1", output: "probe_mode\tfail\n", exitCode: 1, errorText: "requested failure"},
		},
		{
			Scenario: routedScenario{Name: "invalid_mode"},
			Record:   modstate.ShellBusRecord{Status: "failed"},
			Command:  commandContext{target: "codex-view:0:1", exitCode: 1, errorText: `unsupported --mode "invalid"`},
		},
		{
			Scenario: routedScenario{Name: "recovery"},
			Record:   modstate.ShellBusRecord{Status: "done"},
			Command:  commandContext{target: "codex-view:0:1", output: "probe_mode\tsuccess\n", exitCode: 0},
		},
		{
			Scenario:       routedScenario{Name: "background"},
			Record:         modstate.ShellBusRecord{Status: "done"},
			BackgroundText: "probe_background_done\tTEST_BACKGROUND\n",
			Command:        commandContext{target: "codex-view:0:1", output: "probe_label\tTEST_BACKGROUND\n", exitCode: 0},
		},
	}
	err := validateE2EOutputs(
		"prompt token",
		"codex-view:0:0",
		"codex-view:0:1",
		strings.Join([]string{
			"role\tprompt",
			"text",
			"Dialtone test v1 start",
			"prompt token",
			"./dialtone_mod mods v1 probe --mode success --label CODEX_AGENT_prompt_token",
			"1. ./dialtone_mod mods v1 db graph --format outline",
			"2. ./dialtone_mod mods v1 probe --mode sleep --sleep-ms 1500 --label TEST_LONG_RUNNING",
			"3. ./dialtone_mod mods v1 probe --mode fail --label TEST_FAILURE",
			"4. ./dialtone_mod mods v1 probe --mode invalid --label TEST_INVALID_MODE",
			"5. ./dialtone_mod mods v1 probe --mode background --sleep-ms 4000 --label TEST_BACKGROUND --background-file <marker-file>",
		}, "\n"),
		"role\tcommand\ntext\nDialtone test v1 start prompt token\nprobe_label\tTEST_BACKGROUND\n",
		"prompt_target\tcodex-view:0:0\ncommand_target\tcodex-view:0:1\ndialtone_status\trunning\nworker_status\trunning\nworker_pane\tcodex-view:0:1\n",
		results,
	)
	if err == nil || !strings.Contains(err.Error(), "unexpectedly contained the submitted prompt token") {
		t.Fatalf("expected prompt leak failure, got %v", err)
	}
}

func TestWaitForShellBusCommandByTextFindsMatchingCodexRow(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("open state db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	olderBody, err := dispatch.EncodeIntentBody(dispatch.ShellCommandIntent{
		Command:        "./dialtone_mod mods v1 probe --mode success --label OLDER",
		InnerCommand:   "./dialtone_mod mods v1 probe --mode success --label OLDER",
		DisplayCommand: "./dialtone_mod mods v1 probe --mode success --label OLDER",
		Target:         "codex-view:0:1",
	})
	if err != nil {
		t.Fatalf("encode older body: %v", err)
	}
	olderID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", olderBody)
	if err != nil {
		t.Fatalf("enqueue older row: %v", err)
	}

	wantCommand := "./dialtone_mod mods v1 probe --mode success --label CODEX_AGENT_TOKEN"
	wantBody, err := dispatch.EncodeIntentBody(dispatch.ShellCommandIntent{
		Command:        wantCommand,
		InnerCommand:   wantCommand,
		DisplayCommand: wantCommand,
		Target:         "codex-view:0:1",
	})
	if err != nil {
		t.Fatalf("encode wanted body: %v", err)
	}
	wantID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", "codex-view", "codex-view:0:1", wantBody)
	if err != nil {
		t.Fatalf("enqueue wanted row: %v", err)
	}

	record, err := waitForShellBusCommandByText(db, wantCommand, olderID, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForShellBusCommandByText returned error: %v", err)
	}
	if record.ID != wantID {
		t.Fatalf("expected row id %d, got %+v", wantID, record)
	}
}
