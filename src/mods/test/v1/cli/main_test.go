package main

import (
	"strings"
	"testing"
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

func TestBuildPromptTextUsesTokenByDefault(t *testing.T) {
	got := buildPromptText("TOKEN_123", "")
	if !strings.Contains(got, "TOKEN_123") {
		t.Fatalf("expected token in default prompt, got %q", got)
	}
}

func TestBuildPromptTextPreservesExplicitPrompt(t *testing.T) {
	got := buildPromptText("TOKEN_123", "custom prompt")
	if got != "custom prompt" {
		t.Fatalf("unexpected prompt text: %q", got)
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
	command := commandContext{
		output:    "- shell:v1\n- tsnet:v1\n",
		exitCode:  0,
		runtimeMS: 321,
	}
	err := validateE2EOutputs(
		"prompt token",
		"route\tqueued\ncommand_id\t1\n",
		"dialtone_status\trunning\nworker_status\trunning\n",
		"role\tprompt\ntext\nprompt token\n",
		"role\tcommand\ntext\n- test:v1\n  - shell:v1\n",
		command,
	)
	if err != nil {
		t.Fatalf("validateE2EOutputs returned error: %v", err)
	}
}

func TestValidateE2EOutputsRejectsMissingPrompt(t *testing.T) {
	command := commandContext{
		output:   "- shell:v1\n",
		exitCode: 0,
	}
	err := validateE2EOutputs(
		"prompt token",
		"route\tqueued\ncommand_id\t1\n",
		"dialtone_status\trunning\nworker_status\trunning\n",
		"role\tprompt\ntext\nother prompt\n",
		"role\tcommand\ntext\n- test:v1\n  - shell:v1\n",
		command,
	)
	if err == nil || !strings.Contains(err.Error(), "prompt pane read") {
		t.Fatalf("expected missing prompt failure, got %v", err)
	}
}
