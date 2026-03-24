package main

import (
	"os"
	"strings"
	"testing"
)

func TestDBCLIUsageIncludesContractCommands(t *testing.T) {
	output := captureStdout(t, printUsage)
	for _, want := range []string{"install", "build", "format", "test", "run"} {
		if !strings.Contains(output, want) {
			t.Fatalf("usage missing %q: %s", want, output)
		}
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
