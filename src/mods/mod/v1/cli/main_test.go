package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestModV1CLISmoke(t *testing.T) {
	for _, name := range []string{
		"main.go",
		"install.go",
		"build.go",
		"format.go",
		"test.go",
		"paths.go",
	} {
		path := filepath.Join(testDataDir(), name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("mod v1 cli is missing %s: %v", name, err)
		}
	}

	usage := captureStdout(t, printUsage)
	for _, cmd := range []string{
		"install",
		"build",
		"format",
		"test",
	} {
		if !strings.Contains(usage, cmd) {
			t.Fatalf("mod v1 usage missing command %q", cmd)
		}
	}

	if _, err := parseFormatArgs([]string{"--help"}); err == nil {
		t.Fatalf("expected parseFormatArgs to reject --help")
	}
	if got, err := parseFormatArgs(nil); err != nil {
		t.Fatalf("parseFormatArgs should accept blank args: %v", err)
	} else if got != "" {
		t.Fatalf("parseFormatArgs(nil) = %q, want empty string so format defaults to the mod root", got)
	}

	modRoot := filepath.Join(testDataDir(), "..")
	if err := os.Chdir(modRoot); err != nil {
		t.Fatalf("chdir to mod root failed: %v", err)
	}

	if _, err := parseFormatArgs([]string{"--dir", modRoot}); err != nil {
		t.Fatalf("parseFormatArgs should accept explicit dir: %v", err)
	}

	if err := runInstall([]string{"unexpected-arg"}); err == nil {
		t.Fatalf("mod install should reject positional args")
	}
	if err := runBuild([]string{"unexpected-arg"}); err == nil {
		t.Fatalf("mod build should reject positional args")
	}
	if err := runTest([]string{"unexpected-arg"}); err == nil {
		t.Fatalf("mod test should reject positional args")
	}
}

func testDataDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to locate test source file")
	}
	return filepath.Dir(file)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("reading stdout pipe failed: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader failed: %v", err)
	}
	return buf.String()
}
