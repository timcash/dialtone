package test

import (
	dialtone "dialtone/cli/src"
	"path/filepath"
	"testing"
)

func TestUnit_GetDialtoneEnv(t *testing.T) {
	env := dialtone.GetDialtoneEnv()
	if env == "" {
		t.Fatal("GetDialtoneEnv returned empty string")
	}
	if !filepath.IsAbs(env) {
		t.Errorf("GetDialtoneEnv returned non-absolute path: %s", env)
	}
}
