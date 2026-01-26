package util

import (
	"strings"
	"testing"
)

func TestGenerateCodename(t *testing.T) {
	// Test basic generation
	name1 := GenerateCodename("")
	if name1 == "" {
		t.Error("Generated codename is empty")
	}
	if !strings.Contains(name1, "-") {
		t.Error("Codename should contain a hyphen")
	}

	// Test with prefix
	prefix := "drone"
	name2 := GenerateCodename(prefix)
	if !strings.HasPrefix(name2, prefix+"-") {
		t.Errorf("Codename %s should start with prefix %s", name2, prefix)
	}

	// Test randomness (probabilistic)
	name3 := GenerateCodename("test")
	name4 := GenerateCodename("test")
	if name3 == name4 {
		// It's possible but very unlikely with the given dictionary size
		t.Logf("Warning: Generated identical codenames: %s", name3)
	}
}
