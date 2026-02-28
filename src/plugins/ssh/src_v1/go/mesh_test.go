package ssh

import (
	"testing"
	"time"
)

func TestResolveMeshNodeAlias(t *testing.T) {
	n, err := ResolveMeshNode("rover-1")
	if err != nil {
		t.Fatalf("ResolveMeshNode(rover-1) failed: %v", err)
	}
	if n.Name != "rover" {
		t.Fatalf("expected rover node, got %s", n.Name)
	}
	if n.User != "tim" {
		t.Fatalf("expected user tim, got %s", n.User)
	}
}

func TestResolveCommandTransportLegionUsesPowerShellInWSL(t *testing.T) {
	prev := isWSLFunc
	isWSLFunc = func() bool { return true }
	defer func() { isWSLFunc = prev }()

	transport, err := ResolveCommandTransport("legion")
	if err != nil {
		t.Fatalf("ResolveCommandTransport failed: %v", err)
	}
	if transport != "powershell" {
		t.Fatalf("expected powershell transport, got %s", transport)
	}
}

func TestResolveCommandTransportDefaultSSH(t *testing.T) {
	prev := isWSLFunc
	isWSLFunc = func() bool { return false }
	defer func() { isWSLFunc = prev }()

	transport, err := ResolveCommandTransport("legion")
	if err != nil {
		t.Fatalf("ResolveCommandTransport failed: %v", err)
	}
	if transport != "ssh" {
		t.Fatalf("expected ssh transport, got %s", transport)
	}
}

func TestResolvePreferredHostUsesReachableCandidate(t *testing.T) {
	prev := canReachHostFn
	canReachHostFn = func(host, port string, _ time.Duration) bool {
		return host == "169.254.217.151" && port == "22"
	}
	defer func() { canReachHostFn = prev }()

	node := MeshNode{
		Name:           "rover",
		Host:           "rover-1.shad-artichoke.ts.net",
		HostCandidates: []string{"169.254.217.151"},
	}
	got := resolvePreferredHost(node, "22")
	if got != "169.254.217.151" {
		t.Fatalf("expected ethernet host, got %s", got)
	}
}

func TestResolvePreferredHostFallsBackToPrimaryHost(t *testing.T) {
	prev := canReachHostFn
	canReachHostFn = func(host, port string, _ time.Duration) bool {
		return false
	}
	defer func() { canReachHostFn = prev }()

	node := MeshNode{
		Name:           "rover",
		Host:           "rover-1.shad-artichoke.ts.net",
		HostCandidates: []string{"169.254.217.151"},
	}
	got := resolvePreferredHost(node, "22")
	if got != "rover-1.shad-artichoke.ts.net" {
		t.Fatalf("expected tailscale fallback host, got %s", got)
	}
}
