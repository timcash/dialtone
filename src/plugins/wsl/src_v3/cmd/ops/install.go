package ops

import (
	"crypto/sha256"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type installState struct {
	InstalledAtUTC string `json:"installed_at_utc"`
	GoRequired     string `json:"go_required"`
	GoVersion      string `json:"go_version"`
	BunVersion     string `json:"bun_version"`
	PackageSHA256  string `json:"package_sha256"`
	LockSHA256     string `json:"lock_sha256"`
}

func Install(args ...string) error {
	repoRoot, uiDir, err := resolveWSLPaths()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("missing src_v3 ui package.json: %w", err)
	}

	if err := run(repoRoot, "go", "src_v1", "install"); err != nil {
		return fmt.Errorf("go toolchain install failed: %w", err)
	}

	if err := ensureBunToolchain(repoRoot); err != nil {
		return err
	}

	statePath, state, err := currentInstallState(repoRoot, uiDir)
	if err != nil {
		return err
	}

	if isInstallUpToDate(uiDir, state) {
		logs.Info("[WSL INSTALL] cache hit: src_v3 ui dependencies already up to date")
		return nil
	}

	logs.Info("[WSL INSTALL] installing ui deps in %s", uiDir)
	if err := run(repoRoot, "bun", "src_v1", "exec", "--cwd", uiDir, "install", "--frozen-lockfile"); err != nil {
		return fmt.Errorf("bun install failed: %w", err)
	}

	if err := writeInstallState(statePath, state); err != nil {
		return err
	}
	logs.Info("[WSL INSTALL] cache updated: %s", statePath)
	return nil
}

func resolveWSLPaths() (repoRoot, uiDir string, err error) {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return "", "", err
	}
	preset := configv1.NewPluginPreset(rt, "wsl", "src_v3")
	return rt.RepoRoot, preset.UI, nil
}

func run(repoRoot string, args ...string) error {
	cmd := getDialtoneCmd(repoRoot)
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureBunToolchain(repoRoot string) error {
	envDir := logs.GetDialtoneEnv()
	bunBin := filepath.Join(envDir, "bun", "bin", "bun")
	if _, err := os.Stat(bunBin); err == nil {
		return nil
	}

	logs.Warn("[WSL INSTALL] managed bun runtime missing at %s; installing into DIALTONE_ENV", bunBin)
	if err := os.MkdirAll(filepath.Join(envDir, "bun"), 0o755); err != nil {
		return err
	}
	installCmd := "curl -fsSL https://bun.sh/install | BUN_INSTALL='" + filepath.Join(envDir, "bun") + "' bash"
	cmd := exec.Command("bash", "-lc", installCmd)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install bun runtime into %s: %w", bunBin, err)
	}
	return nil
}

func currentInstallState(repoRoot, uiDir string) (string, installState, error) {
	pkgHash, err := sha256File(filepath.Join(uiDir, "package.json"))
	if err != nil {
		return "", installState{}, err
	}
	lockHash, err := sha256File(filepath.Join(uiDir, "bun.lock"))
	if err != nil {
		return "", installState{}, err
	}

	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return "", installState{}, err
	}
	goRequired, err := requiredGoVersion(configv1.SrcPath(rt, "go.mod"))
	if err != nil {
		return "", installState{}, err
	}
	goVersion, _ := managedToolVersion("go", "version")
	bunVersion, _ := managedToolVersion("bun", "--version")

	state := installState{
		InstalledAtUTC: time.Now().UTC().Format(time.RFC3339),
		GoRequired:     goRequired,
		GoVersion:      strings.TrimSpace(goVersion),
		BunVersion:     strings.TrimSpace(bunVersion),
		PackageSHA256:  pkgHash,
		LockSHA256:     lockHash,
	}
	statePath := filepath.Join(logs.GetDialtoneEnv(), "cache", "wsl", "src_v3", "install-state.json")
	return statePath, state, nil
}

func isInstallUpToDate(uiDir string, current installState) bool {
	nodeModules := filepath.Join(uiDir, "node_modules")
	if _, err := os.Stat(nodeModules); err != nil {
		return false
	}

	statePath := filepath.Join(logs.GetDialtoneEnv(), "cache", "wsl", "src_v3", "install-state.json")
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return false
	}
	var prev installState
	if err := json.Unmarshal(raw, &prev); err != nil {
		return false
	}
	return prev.GoRequired == current.GoRequired &&
		prev.GoVersion == current.GoVersion &&
		prev.BunVersion == current.BunVersion &&
		prev.PackageSHA256 == current.PackageSHA256 &&
		prev.LockSHA256 == current.LockSHA256
}

func writeInstallState(path string, state installState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func requiredGoVersion(goModPath string) (string, error) {
	raw, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go ")), nil
		}
	}
	return "", fmt.Errorf("go version not found in %s", goModPath)
}

func managedToolVersion(tool string, args ...string) (string, error) {
	envDir := logs.GetDialtoneEnv()
	var bin string
	switch tool {
	case "go":
		bin = filepath.Join(envDir, "go", "bin", "go")
	case "bun":
		bin = filepath.Join(envDir, "bun", "bin", "bun")
	default:
		return "", fmt.Errorf("unsupported tool %s", tool)
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s version check failed: %w", tool, err)
	}
	return strings.TrimSpace(string(out)), nil
}
