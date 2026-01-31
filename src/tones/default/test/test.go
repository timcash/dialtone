package test

import (
	"os"
	"testing"
)

func TestSubtone(t *testing.T) {
	if os.Getenv("NEXTTONE_SUBTONE") == "" {
		t.Skip("no subtone provided")
	}
}
