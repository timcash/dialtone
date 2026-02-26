package ssh

import "testing"

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

func TestResolveCommandTransportWSLFallbackForLegion(t *testing.T) {
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
