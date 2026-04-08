package repl

import (
	"testing"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

func TestReconcileTaskKVRecordMarksMissingRunningTaskDone(t *testing.T) {
	now := time.Date(2026, time.April, 6, 22, 0, 0, 0, time.UTC)
	record := taskKVRecord{
		TaskID:    "task-1",
		Host:      "legion",
		State:     "running",
		PID:       4321,
		StartedAt: now.Add(-time.Minute).Format(time.RFC3339),
	}

	next, changed := reconcileTaskKVRecord(record, "legion", nil, now, func(int) (time.Time, bool) {
		return time.Time{}, false
	})
	if !changed {
		t.Fatalf("expected missing running task to change")
	}
	if next.State != "done" {
		t.Fatalf("expected reconciled state done, got %q", next.State)
	}
	if next.ExitCode == nil || *next.ExitCode != -1 {
		t.Fatalf("expected unknown exit code -1, got %+v", next.ExitCode)
	}
	if next.UpdatedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("expected updated_at refresh, got %q", next.UpdatedAt)
	}
}

func TestReconcileTaskKVRecordRefreshesLiveRunningTask(t *testing.T) {
	started := time.Date(2026, time.April, 6, 22, 0, 0, 0, time.UTC)
	now := started.Add(10 * time.Second)
	record := taskKVRecord{
		TaskID:    "task-2",
		Host:      "legion",
		State:     "running",
		PID:       9876,
		StartedAt: started.Format(time.RFC3339),
		UpdatedAt: started.Format(time.RFC3339),
	}

	next, changed := reconcileTaskKVRecord(record, "legion", map[int]proc.ManagedProcessSnapshot{
		9876: {PID: 9876},
	}, now, func(int) (time.Time, bool) {
		return time.Time{}, false
	})
	if !changed {
		t.Fatalf("expected live running task to refresh timestamps")
	}
	if next.State != "running" {
		t.Fatalf("expected state running, got %q", next.State)
	}
	if next.UpdatedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("expected updated_at refresh, got %q", next.UpdatedAt)
	}
	if next.LastOKAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("expected last_ok_at refresh, got %q", next.LastOKAt)
	}
}

func TestReconcileTaskKVRecordSkipsRemoteHost(t *testing.T) {
	now := time.Date(2026, time.April, 6, 22, 0, 0, 0, time.UTC)
	record := taskKVRecord{
		TaskID:    "task-3",
		Host:      "remote-host",
		State:     "running",
		PID:       1001,
		StartedAt: now.Add(-time.Minute).Format(time.RFC3339),
	}

	next, changed := reconcileTaskKVRecord(record, "legion", nil, now, func(int) (time.Time, bool) {
		return time.Time{}, false
	})
	if changed {
		t.Fatalf("expected remote host record to remain unchanged")
	}
	if next.State != "running" {
		t.Fatalf("expected remote state to stay running, got %q", next.State)
	}
}
