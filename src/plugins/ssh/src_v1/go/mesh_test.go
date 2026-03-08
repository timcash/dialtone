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

func TestResolvePreferredHostPrefersLANOverTailnetWhenReachable(t *testing.T) {
	prev := canReachHostFn
	canReachHostFn = func(host, port string, _ time.Duration) bool {
		return host == "192.168.4.36" && port == "22"
	}
	defer func() { canReachHostFn = prev }()

	node := MeshNode{
		Name:           "rover",
		Host:           "rover-1.shad-artichoke.ts.net",
		HostCandidates: []string{"169.254.217.151", "192.168.4.36", "rover-1.shad-artichoke.ts.net"},
	}
	got := resolvePreferredHost(node, "22")
	if got != "192.168.4.36" {
		t.Fatalf("expected LAN host, got %s", got)
	}
}

func TestResolvePreferredHostPrefersTailscaleWhenConfigured(t *testing.T) {
	prev := canReachHostFn
	canReachHostFn = func(host, port string, _ time.Duration) bool {
		return host == "rover-1.shad-artichoke.ts.net" && port == "22"
	}
	defer func() { canReachHostFn = prev }()

	node := MeshNode{
		Name:            "rover",
		Host:            "rover-1.shad-artichoke.ts.net",
		RoutePreference: []string{"tailscale", "private", "link-local", "other"},
		HostCandidates:  []string{"169.254.217.151", "192.168.4.36", "rover-1.shad-artichoke.ts.net"},
	}
	got := resolvePreferredHost(node, "22")
	if got != "rover-1.shad-artichoke.ts.net" {
		t.Fatalf("expected tailscale host, got %s", got)
	}
}

func TestRouteHostReturnsConfiguredTailnetCandidate(t *testing.T) {
	prev := canReachHostFn
	canReachHostFn = func(host, port string, _ time.Duration) bool {
		return host == "grey.shad-artichoke.ts.net" && port == "22"
	}
	defer func() { canReachHostFn = prev }()

	node := MeshNode{
		Name:            "grey",
		Host:            "192.168.4.31",
		RoutePreference: []string{"tailscale", "private", "other"},
		HostCandidates:  []string{"grey.shad-artichoke.ts.net", "192.168.4.31"},
	}
	got := RouteHost(node, "tailscale", "22")
	if got != "grey.shad-artichoke.ts.net" {
		t.Fatalf("expected tailscale route host, got %s", got)
	}
}

func TestPrioritizedRouteHostsForNodeFiltersToRequestedRoute(t *testing.T) {
	node := MeshNode{
		Name:           "rover",
		HostCandidates: []string{"rover-1.shad-artichoke.ts.net", "192.168.4.36", "169.254.217.151"},
	}
	got := prioritizedRouteHostsForNode(node, "tailscale", resolveMeshCandidates(node))
	want := []string{"rover-1.shad-artichoke.ts.net"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidate count: got %d want %d", len(got), len(want))
	}
	for i, gotHost := range want {
		if got[i] != gotHost {
			t.Fatalf("priority[%d]: expected %q got %q", i, gotHost, got[i])
		}
	}
}

func TestPrioritizedMeshHostsFallsBackToDefaultWhenRoutePreferenceIsUnknown(t *testing.T) {
	node := MeshNode{
		Name:            "rover",
		RoutePreference: []string{"bogus"},
		HostCandidates:  []string{"rover-1.shad-artichoke.ts.net", "169.254.217.151", "192.168.4.36"},
	}
	got := prioritizedMeshHostsForNode(node, resolveMeshCandidates(node))
	want := []string{"rover-1.shad-artichoke.ts.net", "192.168.4.36", "169.254.217.151"}
	if len(got) != len(want) {
		t.Fatalf("unexpected candidate count: got %d want %d", len(got), len(want))
	}
	for i, gotHost := range want {
		if got[i] != gotHost {
			t.Fatalf("priority[%d]: expected %q got %q", i, gotHost, got[i])
		}
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
	if got != "169.254.217.151" {
		t.Fatalf("expected first remaining candidate host, got %s", got)
	}
}
