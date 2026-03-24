package main

import (
	"os"
	"strings"
	"testing"
)

func TestDialtoneCLIUsageIncludesContractCommands(t *testing.T) {
	output := captureStdout(t, printUsage)
	for _, want := range []string{"install", "build", "format", "test", "queue", "paths", "processes", "commands", "command", "log", "protocol-runs", "test-runs"} {
		if !strings.Contains(output, want) {
			t.Fatalf("usage missing %q: %s", want, output)
		}
	}
}

func TestDialtoneTestPackagesCoverRuntimeAndCLI(t *testing.T) {
	got := dialtoneTestPackages()
	want := []string{
		"./internal/modstate",
		"./mods/dialtone/v1/...",
		"./mods/shared/dispatch",
		"./mods/shared/router",
		"./mods/shared/sqlitestate",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected dialtone test packages\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestDialtoneParseFormatArgsKeepsBlankDirEmpty(t *testing.T) {
	got, err := parseFormatArgs(nil)
	if err != nil {
		t.Fatalf("parseFormatArgs returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseFormatArgs(nil) = %q, want empty string so format defaults to src/mods/dialtone/v1", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	done := make(chan string, 1)
	go func() {
		var buf [4096]byte
		var out strings.Builder
		for {
			n, readErr := r.Read(buf[:])
			if n > 0 {
				out.Write(buf[:n])
			}
			if readErr != nil {
				done <- out.String()
				return
			}
		}
	}()
	fn()
	_ = w.Close()
	return <-done
}
