package ssh

import (
	"strings"
	"testing"
)

func TestResolveBootstrapTargets(t *testing.T) {
	nodes, err := resolveBootstrapTargets("all")
	if err != nil {
		t.Fatalf("resolve all: %v", err)
	}
	if len(nodes) < 5 {
		t.Fatalf("expected at least 5 nodes, got %d", len(nodes))
	}

	one, err := resolveBootstrapTargets("rover")
	if err != nil {
		t.Fatalf("resolve rover: %v", err)
	}
	if len(one) != 1 || one[0].Name != "rover" {
		t.Fatalf("unexpected single target: %+v", one)
	}
}

func TestBuildBootstrapCommand(t *testing.T) {
	cmd := buildBootstrapCommand("/tmp/repo", []string{
		"printf 'y\\n' | ./dialtone.sh go src_v1 install",
	}, "./dialtone.sh go src_v1 exec version")

	want := []string{
		"set -euo pipefail",
		"repo='/tmp/repo'",
		"mkdir -p \"$repo\"",
		"cd \"$repo\"",
		"./dialtone.sh go src_v1 install",
		"./dialtone.sh go src_v1 exec version",
	}
	for _, token := range want {
		if !strings.Contains(cmd, token) {
			t.Fatalf("command missing token %q: %s", token, cmd)
		}
	}
}

