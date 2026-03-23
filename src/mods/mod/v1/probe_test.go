package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunProbeWithIORejectsPositionalArgs(t *testing.T) {
	err := runProbeWithIO([]string{"extra"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "does not accept positional arguments") {
		t.Fatalf("expected positional arg rejection, got %v", err)
	}
}

func TestRunProbeWithIORejectsNegativeSleep(t *testing.T) {
	err := runProbeWithIO([]string{"--sleep-ms", "-1"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--sleep-ms must be non-negative") {
		t.Fatalf("expected negative sleep rejection, got %v", err)
	}
}

func TestRunProbeWithIOSuccessWritesMarkers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runProbeWithIO([]string{"--mode", "success", "--label", "SUCCESS_CASE"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProbeWithIO returned error: %v", err)
	}
	text := stdout.String()
	for _, want := range []string{
		"probe_mode\tsuccess",
		"probe_label\tSUCCESS_CASE",
		"probe_result\tsuccess",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in probe output, got:\n%s", want, text)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestRunProbeWithIOFailReturnsErrorAndMarkers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runProbeWithIO([]string{"--mode", "fail", "--label", "FAIL_CASE"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "requested failure") {
		t.Fatalf("expected requested failure, got %v", err)
	}
	text := stdout.String()
	for _, want := range []string{
		"probe_mode\tfail",
		"probe_label\tFAIL_CASE",
		"probe_result\tfailure",
		"probe_error\trequested failure",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in probe output, got:\n%s", want, text)
		}
	}
	if !strings.Contains(stderr.String(), "requested failure") {
		t.Fatalf("expected stderr to mention failure, got %q", stderr.String())
	}
}

func TestRunProbeWithIOBackgroundWritesMarkerFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	target := filepath.Join(t.TempDir(), "background.txt")
	err := runProbeWithIO([]string{
		"--mode", "background",
		"--sleep-ms", "50",
		"--label", "BACKGROUND_CASE",
		"--background-file", target,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runProbeWithIO returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "probe_result\tbackground-started") {
		t.Fatalf("expected background start marker, got:\n%s", stdout.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := os.ReadFile(target)
		if readErr == nil {
			text := string(data)
			if strings.Contains(text, "probe_background_done\tBACKGROUND_CASE") &&
				strings.Contains(text, "probe_label\tBACKGROUND_CASE") {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	data, _ := os.ReadFile(target)
	t.Fatalf("background marker file was not written in time: %s", string(data))
}
