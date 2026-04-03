package gacad

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func UIPackageManifest(paths Paths) string {
	return filepath.Join(paths.UIDir, "package.json")
}

func VerifyInstallLayout(paths Paths) error {
	checks := []struct {
		label string
		path  string
	}{
		{label: "ui directory", path: paths.UIDir},
		{label: "ui package manifest", path: UIPackageManifest(paths)},
	}
	for _, check := range checks {
		if _, err := os.Stat(check.path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("ga_cad install: %s missing at %s", check.label, check.path)
			}
			return fmt.Errorf("ga_cad install: unable to inspect %s at %s: %w", check.label, check.path, err)
		}
	}
	return nil
}

func ResolveBunBinary(paths Paths) (string, error) {
	managed := configv1.ManagedBunBinPath(paths.Runtime.DialtoneEnv)
	if _, err := os.Stat(managed); err == nil {
		return managed, nil
	}
	if bin := strings.TrimSpace(paths.Runtime.BunBin); bin != "" {
		return bin, nil
	}
	return "", fmt.Errorf("ga_cad install: bun not found in PATH or at %s", managed)
}
