package pixi

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

const (
	installScriptURL = "https://pixi.sh/install.sh"
	installPS1URL    = "https://pixi.sh/install.ps1"
)

type InstallReceipt struct {
	InstalledAtUTC string `json:"installed_at_utc"`
	PixiVersion    string `json:"pixi_version"`
	PixiBin        string `json:"pixi_bin"`
	PixiHome       string `json:"pixi_home"`
	PixiCacheDir   string `json:"pixi_cache_dir"`
}

func ManagedHome(rt configv1.Runtime) string {
	return configv1.ManagedPixiHomePath(rt.DialtoneEnv)
}

func ManagedBin(rt configv1.Runtime) string {
	return configv1.ManagedPixiBinPath(rt.DialtoneEnv)
}

func ResolveBinary(rt configv1.Runtime) (string, error) {
	if v := strings.TrimSpace(rt.PixiBin); v != "" {
		return v, nil
	}
	managed := ManagedBin(rt)
	if _, err := os.Stat(managed); err == nil {
		return managed, nil
	}
	return "", fmt.Errorf("pixi runtime not found at %s. Run './dialtone.sh pixi src_v1 install' first.", managed)
}

func EnsureManaged(rt configv1.Runtime) (string, error) {
	managed := ManagedBin(rt)
	if _, err := os.Stat(managed); err == nil {
		return managed, nil
	}

	home := ManagedHome(rt)
	cacheDir := strings.TrimSpace(rt.PixiCacheDir)
	if cacheDir == "" {
		cacheDir = configv1.DefaultPixiCacheDir()
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return "", fmt.Errorf("create pixi home: %w", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", fmt.Errorf("create pixi cache dir: %w", err)
	}

	cmd, err := installerCommand()
	if err != nil {
		return "", err
	}
	cmd.Env = append(os.Environ(), installerEnv(rt, cacheDir)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("managed pixi install failed: %w", err)
	}
	if _, err := os.Stat(managed); err != nil {
		return "", fmt.Errorf("managed pixi install did not create %s", managed)
	}
	return managed, nil
}

func InstallReceiptPath(rt configv1.Runtime) string {
	return configv1.PluginInstallStatePath(rt, "pixi", "src_v1")
}

func WriteInstallReceipt(rt configv1.Runtime, bin string) (string, error) {
	bin = strings.TrimSpace(bin)
	if bin == "" {
		resolved, err := ResolveBinary(rt)
		if err != nil {
			return "", err
		}
		bin = resolved
	}

	version, err := installedVersion(bin)
	if err != nil {
		return "", err
	}

	path := InstallReceiptPath(rt)
	receipt := InstallReceipt{
		InstalledAtUTC: time.Now().UTC().Format(time.RFC3339),
		PixiVersion:    version,
		PixiBin:        bin,
		PixiHome:       ManagedHome(rt),
		PixiCacheDir:   strings.TrimSpace(rt.PixiCacheDir),
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create pixi receipt dir: %w", err)
	}
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal pixi receipt: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write pixi receipt: %w", err)
	}
	return path, nil
}

func NewCommand(rt configv1.Runtime, cwd string, args ...string) (*exec.Cmd, error) {
	bin, err := ResolveBinary(rt)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin, args...)
	if strings.TrimSpace(cwd) != "" {
		cmd.Dir = cwd
	}
	pathPrefix := filepath.Dir(bin)
	pathValue := pathPrefix
	if current := strings.TrimSpace(os.Getenv("PATH")); current != "" {
		pathValue += string(os.PathListSeparator) + current
	}
	cmd.Env = append(os.Environ(),
		"PIXI_HOME="+ManagedHome(rt),
		"PATH="+pathValue,
	)
	return cmd, nil
}

func installerCommand() (*exec.Cmd, error) {
	if runtime.GOOS == "windows" {
		return exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm -useb "+installPS1URL+" | iex"), nil
	}
	if _, err := exec.LookPath("curl"); err == nil {
		return exec.Command("bash", "-lc", "curl -fsSL "+installScriptURL+" | sh"), nil
	}
	if _, err := exec.LookPath("wget"); err == nil {
		return exec.Command("bash", "-lc", "wget -qO- "+installScriptURL+" | sh"), nil
	}
	return nil, fmt.Errorf("pixi install requires curl or wget in PATH")
}

func installerEnv(rt configv1.Runtime, cacheDir string) []string {
	env := []string{
		"PIXI_HOME=" + ManagedHome(rt),
		"PIXI_NO_PATH_UPDATE=1",
		"TMP_DIR=" + cacheDir,
	}
	if v := strings.TrimSpace(os.Getenv("DIALTONE_PIXI_VERSION")); v != "" {
		env = append(env, "PIXI_VERSION="+v)
	}
	return env
}

func installedVersion(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pixi version check failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
