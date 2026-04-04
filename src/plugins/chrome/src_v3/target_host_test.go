package src_v3

import (
	"testing"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func TestDefaultChromeTargetHostUsesWindowsMeshNodeFromWSL(t *testing.T) {
	originalList := listChromeMeshNodesFunc
	listChromeMeshNodesFunc = func() []sshv1.MeshNode {
		return []sshv1.MeshNode{
			{Name: "wsl", OS: "linux"},
			{Name: "legion", OS: "windows", PreferWSLPowerShell: true},
		}
	}
	t.Cleanup(func() {
		listChromeMeshNodesFunc = originalList
	})
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu-24.04")
	t.Setenv("DIALTONE_CHROME_DEFAULT_HOST", "")
	t.Setenv("DIALTONE_CHROME_TEST_HOST", "")

	if got := defaultChromeTargetHost(); got != "legion" {
		t.Fatalf("expected WSL default host to resolve to legion, got %q", got)
	}
	if got := effectiveChromeTargetHost(""); got != "legion" {
		t.Fatalf("expected empty host to resolve to legion, got %q", got)
	}
	if isLocalHost("") {
		t.Fatalf("expected empty host to stop being local when WSL default host resolves to windows")
	}
}

func TestDefaultChromeTargetHostPrefersExplicitEnvOverride(t *testing.T) {
	originalList := listChromeMeshNodesFunc
	listChromeMeshNodesFunc = func() []sshv1.MeshNode {
		return []sshv1.MeshNode{{Name: "legion", OS: "windows", PreferWSLPowerShell: true}}
	}
	t.Cleanup(func() {
		listChromeMeshNodesFunc = originalList
	})
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu-24.04")
	t.Setenv("DIALTONE_CHROME_DEFAULT_HOST", "lab-windows")
	t.Setenv("DIALTONE_CHROME_TEST_HOST", "")

	if got := defaultChromeTargetHost(); got != "lab-windows" {
		t.Fatalf("expected env override to win, got %q", got)
	}
}

func TestEffectiveChromeTargetHostPreservesExplicitLocalTarget(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu-24.04")
	t.Setenv("DIALTONE_CHROME_DEFAULT_HOST", "legion")

	if got := effectiveChromeTargetHost("local"); got != "local" {
		t.Fatalf("expected explicit local host to remain local, got %q", got)
	}
	if !isLocalHost("local") {
		t.Fatalf("expected explicit local host to remain local")
	}
}
