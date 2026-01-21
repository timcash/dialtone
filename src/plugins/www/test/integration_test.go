package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIntegration_AppStructure(t *testing.T) {
	// Verify that the app directory exists and contains expected files
	// Relative path from src/plugins/www/test to src/plugins/www/app is ../app
	
	wd, _ := os.Getwd()
	t.Logf("Working dir: %s", wd)

	appDir := "../app"
	
	info, err := os.Stat(appDir)
	if err != nil {
		t.Fatalf("App directory %s does not exist: %v", appDir, err)
	}
	
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", appDir)
	}
	
	expectedFiles := []string{
		"package.json",
		"next.config.mjs",
		"app/page.tsx",
	}
	
	for _, f := range expectedFiles {
		p := filepath.Join(appDir, f)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("Expected file %s not found in app directory", f)
		}
	}
}
