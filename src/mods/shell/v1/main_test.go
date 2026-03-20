package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitForPaneTimesOutWithContext(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write dialtone_mod stub: %v", err)
	}
	err := waitForPane(tmp, "codex-view:0:0", 10*time.Millisecond)
	if err == nil {
		t.Fatalf("expected waitForPane to time out")
	}
	if !strings.Contains(err.Error(), "codex-view:0:0") {
		t.Fatalf("missing pane in error: %v", err)
	}
}

func TestRunDialtoneModQuietCapturesOutput(t *testing.T) {
	tmp := t.TempDir()
	script := "#!/bin/sh\nprintf 'ok from stub\\n'\n"
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte(script), 0o755); err != nil {
		t.Fatalf("write dialtone_mod stub: %v", err)
	}
	out, err := runDialtoneModQuiet(tmp, "ghostty", "v1", "list")
	if err != nil {
		t.Fatalf("runDialtoneModQuiet returned error: %v", err)
	}
	if strings.TrimSpace(out) != "ok from stub" {
		t.Fatalf("unexpected output: %q", out)
	}
}
