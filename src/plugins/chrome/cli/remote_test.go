package cli

import (
	"strings"
	"testing"
)

func TestParsePortsCSV(t *testing.T) {
	got := parsePortsCSV("9222, 9223,foo,9222,-1,0,9333")
	want := []int{9222, 9223, 9333}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ports mismatch: got=%v want=%v", got, want)
		}
	}
}

func TestDetectRoleAndOriginAndPort(t *testing.T) {
	cmd := "/Applications/Google Chrome --remote-debugging-port=9222 --dialtone-origin=true --dialtone-role=dev"
	if p := debugPortFromCmd(cmd); p != 9222 {
		t.Fatalf("debugPortFromCmd got=%d want=9222", p)
	}
	if role := detectRoleFromCmd(cmd); role != "dev" {
		t.Fatalf("detectRoleFromCmd got=%q want=dev", role)
	}
	if origin := detectOriginFromCmd(cmd); origin != "Dialtone" {
		t.Fatalf("detectOriginFromCmd got=%q want=Dialtone", origin)
	}
}

func TestBuildRemoteRelayShell_StartAndStop(t *testing.T) {
	start := buildRemoteRelayShell(9223, 9222, false)
	for _, needle := range []string{
		"listen=9223",
		"target=9222",
		"socat TCP-LISTEN:${listen}",
		"chrome-relay.py",
		"relay started pid=",
	} {
		if !strings.Contains(start, needle) {
			t.Fatalf("start script missing %q", needle)
		}
	}

	stop := buildRemoteRelayShell(9223, 9222, true)
	for _, needle := range []string{
		"stopped relay on :${listen}",
		"pkill -f \"chrome-relay.py",
		"pkill -f \"socat .*TCP-LISTEN:${listen}",
	} {
		if !strings.Contains(stop, needle) {
			t.Fatalf("stop script missing %q", needle)
		}
	}
}
