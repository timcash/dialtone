package src_v3

import "testing"

func TestDaemonCommandSubjectsUsesHostScopedManagerSubjectOnly(t *testing.T) {
	subjects := daemonCommandSubjects("legion", "dev", false)
	if len(subjects) != 1 {
		t.Fatalf("expected exactly one manager subject, got %d", len(subjects))
	}
	if got := subjects[0]; got != "chrome.src_v3.legion.dev.cmd" {
		t.Fatalf("unexpected manager subject %q", got)
	}
}

func TestDaemonCommandSubjectsKeepsLegacyLocalSubjectForEmbeddedNATS(t *testing.T) {
	subjects := daemonCommandSubjects("legion", "dev", true)
	if len(subjects) != 1 {
		t.Fatalf("expected exactly one embedded subject, got %d", len(subjects))
	}
	if got := subjects[0]; got != "chrome.src_v3.dev.cmd" {
		t.Fatalf("unexpected embedded subject %q", got)
	}
}

func TestShouldRetryManagedCommandSkipsLifecycleCommands(t *testing.T) {
	for _, command := range []string{"status", "open", "close", "reset", "shutdown"} {
		if shouldRetryManagedCommand(command) {
			t.Fatalf("expected %q to skip managed retry", command)
		}
	}
	if !shouldRetryManagedCommand("click-aria") {
		t.Fatalf("expected click-aria to allow managed retry")
	}
}

func TestIsRecoverableServiceCommandErrorRecognizesStaleTargetError(t *testing.T) {
	if !isRecoverableServiceCommandError(assertErr("No target with given id found")) {
		t.Fatalf("expected stale target error to be recoverable")
	}
	if isRecoverableServiceCommandError(assertErr("permission denied")) {
		t.Fatalf("expected unrelated error to stay non-recoverable")
	}
}

func assertErr(text string) error {
	return testError(text)
}

type testError string

func (e testError) Error() string {
	return string(e)
}
