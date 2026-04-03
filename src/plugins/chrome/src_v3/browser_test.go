package src_v3

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCloseBrowserClearsStateAfterGracefulExit(t *testing.T) {
	originalGraceful := gracefulCloseBrowserFunc
	originalWait := waitForBrowserPIDExitFunc
	originalKill := killBrowserPIDFunc
	originalPersist := persistDaemonStateFunc
	originalCleanup := cleanupProfileLocksFunc
	t.Cleanup(func() {
		gracefulCloseBrowserFunc = originalGraceful
		waitForBrowserPIDExitFunc = originalWait
		killBrowserPIDFunc = originalKill
		persistDaemonStateFunc = originalPersist
		cleanupProfileLocksFunc = originalCleanup
	})

	canceledTab := 0
	canceledAlloc := 0
	killedPID := 0
	persisted := false

	gracefulCloseBrowserFunc = func(tabCtx, allocCtx context.Context) error { return nil }
	waitForBrowserPIDExitFunc = func(pid int, _ time.Duration) error { return nil }
	killBrowserPIDFunc = func(pid int) error {
		killedPID = pid
		return nil
	}
	persistDaemonStateFunc = func(_ *daemonState) { persisted = true }
	cleanupProfileLocksFunc = func(string) error { return nil }

	d := &daemonState{
		role:          "dev",
		profileDir:    "C:\\Users\\timca\\.dialtone\\chrome-v3\\dev",
		browserPID:    321,
		currentURL:    "about:blank",
		browserWS:     "ws://127.0.0.1/devtools/browser/test",
		managedTarget: "managed",
		consoleLines:  []string{"line"},
		tabCtx:        context.Background(),
		allocCtx:      context.Background(),
		cancelTab:     func() { canceledTab++ },
		cancelAlloc:   func() { canceledAlloc++ },
	}

	if err := d.closeBrowser(); err != nil {
		t.Fatalf("closeBrowser returned error: %v", err)
	}
	if killedPID != 0 {
		t.Fatalf("expected graceful close without kill, got kill pid=%d", killedPID)
	}
	if canceledTab != 1 || canceledAlloc != 1 {
		t.Fatalf("expected both contexts canceled once, got tab=%d alloc=%d", canceledTab, canceledAlloc)
	}
	if !persisted {
		t.Fatalf("expected daemon state persistence after close")
	}
	if d.browserPID != 0 || d.currentURL != "" || d.browserWS != "" || d.managedTarget != "" {
		t.Fatalf("expected browser state cleared after close, got pid=%d url=%q ws=%q target=%q", d.browserPID, d.currentURL, d.browserWS, d.managedTarget)
	}
	if len(d.consoleLines) != 0 {
		t.Fatalf("expected console lines cleared after close, got %d", len(d.consoleLines))
	}
	if !d.intentionalStop {
		t.Fatalf("expected intentionalStop to remain true after close")
	}
}

func TestCloseBrowserFallsBackToForceKillWhenGracefulExitFails(t *testing.T) {
	originalGraceful := gracefulCloseBrowserFunc
	originalWait := waitForBrowserPIDExitFunc
	originalKill := killBrowserPIDFunc
	originalPersist := persistDaemonStateFunc
	originalCleanup := cleanupProfileLocksFunc
	t.Cleanup(func() {
		gracefulCloseBrowserFunc = originalGraceful
		waitForBrowserPIDExitFunc = originalWait
		killBrowserPIDFunc = originalKill
		persistDaemonStateFunc = originalPersist
		cleanupProfileLocksFunc = originalCleanup
	})

	killedPID := 0
	gracefulCloseBrowserFunc = func(tabCtx, allocCtx context.Context) error { return errors.New("browser close failed") }
	waitForBrowserPIDExitFunc = func(pid int, _ time.Duration) error { return errors.New("still running") }
	killBrowserPIDFunc = func(pid int) error {
		killedPID = pid
		return nil
	}
	persistDaemonStateFunc = func(_ *daemonState) {}
	cleanupProfileLocksFunc = func(string) error { return nil }

	d := &daemonState{
		role:       "dev",
		profileDir: "C:\\Users\\timca\\.dialtone\\chrome-v3\\dev",
		browserPID: 654,
		tabCtx:     context.Background(),
		allocCtx:   context.Background(),
	}

	if err := d.closeBrowser(); err != nil {
		t.Fatalf("closeBrowser returned error: %v", err)
	}
	if killedPID != 654 {
		t.Fatalf("expected forced kill for pid 654, got %d", killedPID)
	}
	if d.browserPID != 0 {
		t.Fatalf("expected browser pid cleared after forced kill, got %d", d.browserPID)
	}
}
