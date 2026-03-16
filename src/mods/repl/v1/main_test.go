package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLogStoreAppendAndTail(t *testing.T) {
	t.Parallel()

	store, err := NewLogStore(filepath.Join(t.TempDir(), "repl.log"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}

	for _, text := range []string{"one", "two", "three"} {
		if err := store.Append(LogEntry{Kind: "input", Text: text}); err != nil {
			t.Fatalf("Append(%q) error = %v", text, err)
		}
	}

	entries, err := store.Tail(2)
	if err != nil {
		t.Fatalf("Tail() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Tail() len = %d, want 2", len(entries))
	}
	if got := entries[0].Text; got != "two" {
		t.Fatalf("Tail()[0] = %q, want %q", got, "two")
	}
	if got := entries[1].Text; got != "three" {
		t.Fatalf("Tail()[1] = %q, want %q", got, "three")
	}
}

func TestSessionCommands(t *testing.T) {
	t.Parallel()

	store, err := NewLogStore(filepath.Join(t.TempDir(), "repl.log"))
	if err != nil {
		t.Fatalf("NewLogStore() error = %v", err)
	}
	session := NewSession(Config{Name: "tester", Room: "lab", Prompt: "repl>"}, store)
	if err := session.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	resp, err := session.HandleLine(":help")
	if err != nil {
		t.Fatalf("HandleLine(:help) error = %v", err)
	}
	if !strings.Contains(resp.Text, ":history") {
		t.Fatalf("HandleLine(:help) = %q, want help text", resp.Text)
	}

	resp, err = session.HandleLine("hello")
	if err != nil {
		t.Fatalf("HandleLine(hello) error = %v", err)
	}
	if got := resp.Text; got != "ok: hello" {
		t.Fatalf("HandleLine(hello) = %q, want %q", got, "ok: hello")
	}

	resp, err = session.HandleLine(":quit")
	if err != nil {
		t.Fatalf("HandleLine(:quit) error = %v", err)
	}
	if !resp.Exit {
		t.Fatalf("HandleLine(:quit) Exit = false, want true")
	}

	entries, err := store.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(entries) < 6 {
		t.Fatalf("ReadAll() len = %d, want at least 6", len(entries))
	}
}
