package cad

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func BackendPixiManifest(paths Paths) string {
	return filepath.Join(paths.BackendDir, "pixi.toml")
}

func UIPackageManifest(paths Paths) string {
	return filepath.Join(paths.UIDir, "package.json")
}

func VerifyInstallLayout(paths Paths) error {
	checks := []struct {
		label string
		path  string
	}{
		{label: "backend directory", path: paths.BackendDir},
		{label: "backend entrypoint", path: paths.BackendMain},
		{label: "backend pixi manifest", path: BackendPixiManifest(paths)},
		{label: "ui directory", path: paths.UIDir},
		{label: "ui package manifest", path: UIPackageManifest(paths)},
	}
	for _, check := range checks {
		if _, err := os.Stat(check.path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("cad install: %s missing at %s", check.label, check.path)
			}
			return fmt.Errorf("cad install: unable to inspect %s at %s: %w", check.label, check.path, err)
		}
	}
	return nil
}

func ResolvePixiBinary(paths Paths) (string, error) {
	if bin := strings.TrimSpace(paths.Runtime.PixiBin); bin != "" {
		return bin, nil
	}
	managed := configv1.ManagedPixiBinPath(paths.Runtime.DialtoneEnv)
	return "", fmt.Errorf("cad install: pixi not found in PATH or at %s. Run './dialtone.sh pixi src_v1 install' or './dialtone.sh cad src_v1 install'.", managed)
}

func ResolveBunBinary(paths Paths) (string, error) {
	managed := configv1.ManagedBunBinPath(paths.Runtime.DialtoneEnv)
	if _, err := os.Stat(managed); err == nil {
		return managed, nil
	}
	if bin := strings.TrimSpace(paths.Runtime.BunBin); bin != "" {
		return bin, nil
	}
	return "", fmt.Errorf("cad install: bun not found in PATH or at %s", managed)
}
