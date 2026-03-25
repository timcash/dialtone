package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Runtime struct {
	RepoRoot    string `json:"repo_root"`
	SrcRoot     string `json:"src_root"`
	EnvFile     string `json:"env_file"`
	DialtoneEnv string `json:"dialtone_env"`
	GoBin       string `json:"go_bin"`
	BunBin      string `json:"bun_bin"`
}

type PluginPreset struct {
	Runtime           Runtime `json:"runtime"`
	Plugin            string  `json:"plugin"`
	Version           string  `json:"version"`
	PluginVersionRoot string  `json:"plugin_version_root"`
	PluginBase        string  `json:"plugin_base"`
	UI                string  `json:"ui"`
	UIDist            string  `json:"ui_dist"`
	Test              string  `json:"test"`
	TestCmd           string  `json:"test_cmd"`
	Cmd               string  `json:"cmd"`
	Go                string  `json:"go"`
	Bin               string  `json:"bin"`
	Readme            string  `json:"readme"`
}

func RepoPath(rt Runtime, elems ...string) string {
	parts := append([]string{rt.RepoRoot}, elems...)
	return filepath.Join(parts...)
}

func SrcPath(rt Runtime, elems ...string) string {
	parts := append([]string{rt.SrcRoot}, elems...)
	return filepath.Join(parts...)
}

func PluginPath(rt Runtime, plugin, version string, elems ...string) string {
	parts := []string{rt.SrcRoot, "plugins", plugin}
	if strings.TrimSpace(version) != "" {
		parts = append(parts, version)
	}
	parts = append(parts, elems...)
	return filepath.Join(parts...)
}

func NewPluginPreset(rt Runtime, plugin, version string) PluginPreset {
	base := PluginPath(rt, plugin, "")
	versionRoot := PluginPath(rt, plugin, version)
	return PluginPreset{
		Runtime:           rt,
		Plugin:            plugin,
		Version:           version,
		PluginVersionRoot: versionRoot,
		PluginBase:        base,
		UI:                filepath.Join(versionRoot, "ui"),
		UIDist:            filepath.Join(versionRoot, "ui", "dist"),
		Test:              filepath.Join(versionRoot, "test"),
		TestCmd:           filepath.Join(versionRoot, "test", "cmd"),
		Cmd:               filepath.Join(versionRoot, "cmd"),
		Go:                filepath.Join(versionRoot, "go"),
		Bin:               filepath.Join(base, "bin"),
		Readme:            filepath.Join(base, "README.md"),
	}
}

func (p PluginPreset) Join(elems ...string) string {
	parts := append([]string{p.PluginVersionRoot}, elems...)
	return filepath.Join(parts...)
}

func EnvPath(rt Runtime) string {
	if strings.TrimSpace(rt.EnvFile) != "" {
		return rt.EnvFile
	}
	return RepoPath(rt, "env", "dialtone.json")
}

func FindRepoRoot(start string) (string, error) {
	cwd := strings.TrimSpace(start)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	cwd, _ = filepath.Abs(cwd)
	for {
		if HasDialtoneScript(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", errors.New("repo root not found")
		}
		cwd = parent
	}
}

func DialtoneScriptName() string {
	if runtime.GOOS == "windows" {
		return "dialtone.ps1"
	}
	return "dialtone.sh"
}

func DialtoneScriptPath(repoRoot string) string {
	return filepath.Join(repoRoot, DialtoneScriptName())
}

func HasDialtoneScript(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "dialtone.sh")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "dialtone.ps1")); err == nil {
		return true
	}
	return false
}

func DialtoneCommand(repoRoot string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		all := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", DialtoneScriptPath(repoRoot)}
		all = append(all, args...)
		return exec.Command("powershell.exe", all...)
	}
	return exec.Command(DialtoneScriptPath(repoRoot), args...)
}

func DefaultDialtoneEnv() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_ENV")); v != "" {
		return expandHome(v)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone")
}

func ResolveRuntime(start string) (Runtime, error) {
	repoRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT"))
	if repoRoot == "" {
		resolved, err := FindRepoRoot(start)
		if err != nil {
			return Runtime{}, err
		}
		repoRoot = resolved
	}
	repoRoot, _ = filepath.Abs(repoRoot)

	srcRoot := strings.TrimSpace(os.Getenv("DIALTONE_SRC_ROOT"))
	if srcRoot == "" {
		srcRoot = filepath.Join(repoRoot, "src")
	}
	srcRoot, _ = filepath.Abs(srcRoot)

	envFile := strings.TrimSpace(os.Getenv("DIALTONE_ENV_FILE"))
	if envFile == "" {
		envFile = filepath.Join(repoRoot, "env", "dialtone.json")
	}
	envFile, _ = filepath.Abs(envFile)

	dialtoneEnv := DefaultDialtoneEnv()
	dialtoneEnv, _ = filepath.Abs(dialtoneEnv)

	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		candidate := filepath.Join(dialtoneEnv, "go", "bin", "go")
		if _, err := os.Stat(candidate); err == nil {
			goBin = candidate
		} else if p, lookErr := exec.LookPath("go"); lookErr == nil {
			goBin = p
		}
	}

	bunBin := strings.TrimSpace(os.Getenv("DIALTONE_BUN_BIN"))
	if bunBin == "" {
		candidate := filepath.Join(dialtoneEnv, "bun", "bin", "bun")
		if _, err := os.Stat(candidate); err == nil {
			bunBin = candidate
		} else if p, lookErr := exec.LookPath("bun"); lookErr == nil {
			bunBin = p
		}
	}

	return Runtime{
		RepoRoot:    repoRoot,
		SrcRoot:     srcRoot,
		EnvFile:     envFile,
		DialtoneEnv: dialtoneEnv,
		GoBin:       goBin,
		BunBin:      bunBin,
	}, nil
}

func ApplyRuntimeEnv(rt Runtime) error {
	if strings.TrimSpace(rt.RepoRoot) == "" || strings.TrimSpace(rt.SrcRoot) == "" {
		return fmt.Errorf("invalid runtime: missing repo or src root")
	}
	_ = os.Setenv("DIALTONE_REPO_ROOT", rt.RepoRoot)
	_ = os.Setenv("DIALTONE_SRC_ROOT", rt.SrcRoot)
	if strings.TrimSpace(rt.EnvFile) != "" {
		_ = os.Setenv("DIALTONE_ENV_FILE", rt.EnvFile)
	}
	if strings.TrimSpace(rt.DialtoneEnv) != "" {
		_ = os.Setenv("DIALTONE_ENV", rt.DialtoneEnv)
	}
	if strings.TrimSpace(rt.GoBin) != "" {
		_ = os.Setenv("DIALTONE_GO_BIN", rt.GoBin)
	}
	if strings.TrimSpace(rt.BunBin) != "" {
		_ = os.Setenv("DIALTONE_BUN_BIN", rt.BunBin)
	}

	prepend := []string{}
	if strings.TrimSpace(rt.GoBin) != "" {
		prepend = append(prepend, filepath.Dir(rt.GoBin))
	}
	if strings.TrimSpace(rt.BunBin) != "" {
		prepend = append(prepend, filepath.Dir(rt.BunBin))
	}
	prependPath(prepend...)
	return nil
}

func LoadEnvFile(rt Runtime) error {
	if strings.TrimSpace(rt.EnvFile) == "" {
		return nil
	}
	if _, err := os.Stat(rt.EnvFile); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	raw, err := os.ReadFile(rt.EnvFile)
	if err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	for key, value := range config {
		switch v := value.(type) {
		case string:
			_ = os.Setenv(key, v)
		case float64:
			_ = os.Setenv(key, fmt.Sprintf("%v", v))
		case bool:
			if v {
				_ = os.Setenv(key, "true")
			} else {
				_ = os.Setenv(key, "false")
			}
		default:
			_ = os.Setenv(key, fmt.Sprintf("%v", v))
		}
	}
	return nil
}

func expandHome(v string) string {
	if strings.HasPrefix(v, "~") {
		home, _ := os.UserHomeDir()
		if len(v) == 1 {
			return home
		}
		return filepath.Join(home, strings.TrimPrefix(v, "~/"))
	}
	return v
}

func prependPath(entries ...string) {
	current := strings.TrimSpace(os.Getenv("PATH"))
	parts := []string{}
	seen := map[string]struct{}{}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if _, err := os.Stat(e); err != nil {
			continue
		}
		if _, ok := seen[e]; !ok {
			parts = append(parts, e)
			seen[e] = struct{}{}
		}
	}
	for _, p := range strings.Split(current, string(os.PathListSeparator)) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; !ok {
			parts = append(parts, p)
			seen[p] = struct{}{}
		}
	}
	_ = os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}
