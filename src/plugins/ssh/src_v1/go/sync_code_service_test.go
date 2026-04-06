package ssh

import (
	"strings"
	"testing"
	"time"
)

func TestBuildSyncCodeLoopExecUsesPublicDialtoneEntrypoint(t *testing.T) {
	got := buildSyncCodeLoopExec("/home/user/dialtone", SyncCodeOptions{
		Node:     "wsl",
		Source:   "/mnt/c/Users/timca/dialtone",
		Dest:     "/home/user/dialtone",
		Delete:   true,
		Excludes: []string{"tmp-artifacts/"},
	}, 5*time.Second)

	for _, want := range []string{
		"/bin/bash -lc",
		"./dialtone.sh",
		"ssh",
		"src_v1",
		"sync-code",
		"--host",
		"wsl",
		"--src",
		"/mnt/c/Users/timca/dialtone",
		"--dest",
		"/home/user/dialtone",
		"--delete",
		"--exclude",
		"tmp-artifacts/",
		"sleep",
		"5s",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected ExecStart to contain %q, got %q", want, got)
		}
	}
	for _, unwanted := range []string{
		"repl src_v3 inject",
		"go run ./plugins/",
		"rsync -az",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("expected ExecStart to avoid %q, got %q", unwanted, got)
		}
	}
}

func TestSummarizeSyncCodeServiceState(t *testing.T) {
	t.Run("active and enabled", func(t *testing.T) {
		got := summarizeSyncCodeServiceState("active\n", "enabled\n", nil, nil)
		if got != "active=active enabled=enabled" {
			t.Fatalf("unexpected summary: %q", got)
		}
	})

	t.Run("not installed", func(t *testing.T) {
		got := summarizeSyncCodeServiceState("", "", assertErr{}, assertErr{})
		if got != "not installed" {
			t.Fatalf("unexpected summary: %q", got)
		}
	})
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
