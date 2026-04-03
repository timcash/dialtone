package src_v3

import (
	"errors"
	"testing"
)

func TestReportedManagedProcessCountWindowsUsesManagedBrowserAsPrimaryTruth(t *testing.T) {
	count, err := reportedManagedProcessCount("windows", 27876, 0, nil, false)
	if err != nil {
		t.Fatalf("reportedManagedProcessCount returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected managed Windows browser to report count 1, got %d", count)
	}
}

func TestReportedManagedProcessCountWindowsStillSurfacesDuplicates(t *testing.T) {
	count, err := reportedManagedProcessCount("windows", 27876, 3, nil, false)
	if err != nil {
		t.Fatalf("reportedManagedProcessCount returned error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected duplicate Windows browser count 3, got %d", count)
	}
}

func TestReportedManagedProcessCountFallsBackToLiveBrowserOnEnumerationError(t *testing.T) {
	count, err := reportedManagedProcessCount("linux", 321, 0, errors.New("count timed out"), true)
	if err != nil {
		t.Fatalf("expected live browser fallback without error, got %v", err)
	}
	if count != 1 {
		t.Fatalf("expected live browser fallback count 1, got %d", count)
	}
}
