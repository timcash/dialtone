package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/dev/core/config"
	"dialtone/dev/core/logger"
)

const (
	ToolGo  = "go"
	ToolBun = "bun"
)

type Requirement struct {
	Tool    string
	Version string
}

func EnsureRequirements(reqs []Requirement) error {
	for _, req := range reqs {
		if err := EnsureRequirement(req); err != nil {
			return err
		}
	}
	return nil
}

func EnsureRequirement(req Requirement) error {
	switch req.Tool {
	case ToolGo:
		return ensureGo(req.Version)
	case ToolBun:
		return ensureBun(req.Version)
	default:
		return fmt.Errorf("unsupported install requirement tool: %s", req.Tool)
	}
}

func ensureGo(version string) error {
	depsDir := config.GetDialtoneEnv()
	goBin := filepath.Join(depsDir, "go", "bin", "go")
	if _, err := os.Stat(goBin); os.IsNotExist(err) {
		logger.LogInfo("[install] Go missing; running ./dialtone.sh go install")
		runSimpleShell("./dialtone.sh go install")
	}

	if version == "" {
		return nil
	}

	out, err := exec.Command(goBin, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed checking go version: %w", err)
	}
	want := "go" + version
	if !strings.Contains(string(out), want) {
		return fmt.Errorf("go version mismatch: want %s, got %s", want, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureBun(version string) error {
	depsDir := config.GetDialtoneEnv()
	bunBin := filepath.Join(depsDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); os.IsNotExist(err) {
		logger.LogInfo("[install] Bun missing; installing into %s", depsDir)
		installBun(depsDir, "Install Requirement")
	}

	if version == "" || version == "latest" {
		return nil
	}

	out, err := exec.Command(bunBin, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed checking bun version: %w", err)
	}
	got := strings.TrimSpace(string(out))
	if got != version {
		return fmt.Errorf("bun version mismatch: want %s, got %s", version, got)
	}
	return nil
}
