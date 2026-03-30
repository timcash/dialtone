package config

import (
	"bytes"
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
	RepoRoot          string `json:"repo_root"`
	SrcRoot           string `json:"src_root"`
	EnvFile           string `json:"env_file"`
	DialtoneHome      string `json:"dialtone_home"`
	DialtoneEnv       string `json:"dialtone_env"`
	GoCacheDir        string `json:"go_cache_dir"`
	BunCacheDir       string `json:"bun_cache_dir"`
	PixiCacheDir      string `json:"pixi_cache_dir"`
	ToolCacheDir      string `json:"tool_cache_dir"`
	ContainerCacheDir string `json:"container_cache_dir"`
	WslBuildImage     string `json:"wsl_build_image"`
	GoBin             string `json:"go_bin"`
	BunBin            string `json:"bun_bin"`
	PixiBin           string `json:"pixi_bin"`
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

func PluginBinaryDir(rt Runtime, plugin, version string, elems ...string) string {
	parts := []string{rt.RepoRoot, "bin", "plugins", plugin}
	if strings.TrimSpace(version) != "" {
		parts = append(parts, version)
	}
	parts = append(parts, elems...)
	return filepath.Join(parts...)
}

func PluginBinaryPath(rt Runtime, plugin, version, binary string) string {
	return PluginBinaryDir(rt, plugin, version, strings.TrimSpace(binary))
}

func PluginCachePath(rt Runtime, plugin, version string, elems ...string) string {
	env := strings.TrimSpace(rt.DialtoneEnv)
	if env == "" {
		env = DefaultDialtoneEnv()
	}
	parts := []string{env, "cache", "plugins", plugin}
	if strings.TrimSpace(version) != "" {
		parts = append(parts, version)
	}
	parts = append(parts, elems...)
	return filepath.Join(parts...)
}

func PluginInstallStatePath(rt Runtime, plugin, version string) string {
	return PluginCachePath(rt, plugin, version, "install-state.json")
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
		Bin:               PluginBinaryDir(rt, plugin, version),
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

func explicitEnvFilePath() string {
	envFile := strings.TrimSpace(os.Getenv("DIALTONE_ENV_FILE"))
	if envFile == "" {
		return ""
	}
	envFile = expandHome(envFile)
	if abs, err := filepath.Abs(envFile); err == nil {
		return abs
	}
	return envFile
}

func configRootFromEnvFile(envFile string) string {
	envFile = strings.TrimSpace(envFile)
	if envFile == "" {
		return ""
	}
	dir := filepath.Dir(envFile)
	if strings.EqualFold(filepath.Base(dir), "env") {
		return filepath.Dir(dir)
	}
	return dir
}

func normalizePathValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = expandHome(v)
	if abs, err := filepath.Abs(v); err == nil {
		return abs
	}
	return v
}

func repoRootFromEnvFile(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if repoRoot := strings.TrimSpace(EnvFileString(path, "DIALTONE_REPO_ROOT")); repoRoot != "" {
		return normalizePathValue(repoRoot)
	}
	if strings.EqualFold(filepath.Base(filepath.Dir(path)), "env") {
		return normalizePathValue(configRootFromEnvFile(path))
	}
	return ""
}

func resolveRepoRootCandidate(start string) string {
	if repoRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); repoRoot != "" {
		return normalizePathValue(repoRoot)
	}
	if envFile := explicitEnvFilePath(); envFile != "" {
		if repoRoot := repoRootFromEnvFile(envFile); repoRoot != "" {
			return repoRoot
		}
	}
	resolved, err := FindRepoRoot(start)
	if err != nil {
		return ""
	}
	return normalizePathValue(resolved)
}

func resolveEnvFileCandidate(start string) string {
	if envFile := explicitEnvFilePath(); envFile != "" {
		return envFile
	}
	if repoRoot := resolveRepoRootCandidate(start); repoRoot != "" {
		return normalizePathValue(DefaultDialtoneJSONPath(repoRoot))
	}
	return ""
}

func lookupEnvFileString(start, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	path := resolveEnvFileCandidate(start)
	if path == "" {
		return ""
	}
	return strings.TrimSpace(EnvFileString(path, key))
}

func ResolveEnvFilePath(start string) string {
	return resolveEnvFileCandidate(start)
}

func ReadEnvFileMap(path string) (map[string]any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return map[string]any{}, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func WriteEnvFileMap(path string, doc map[string]any) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if doc == nil {
		doc = map[string]any{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

func UpdateEnvFileValues(path string, updates map[string]any) error {
	path = strings.TrimSpace(path)
	if path == "" || len(updates) == 0 {
		return nil
	}
	doc, err := ReadEnvFileMap(path)
	if err != nil {
		return err
	}
	for key, value := range updates {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if value == nil {
			delete(doc, key)
			continue
		}
		doc[key] = value
	}
	return WriteEnvFileMap(path, doc)
}

func UpdateRuntimeEnvFile(start string, updates map[string]any) error {
	path := ResolveEnvFilePath(start)
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("env file path not resolved")
	}
	return UpdateEnvFileValues(path, updates)
}

func EnvFileString(path, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	doc, err := ReadEnvFileMap(path)
	if err != nil {
		return ""
	}
	return stringifyEnvValue(doc[key])
}

func LookupEnvString(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		return raw
	}
	return lookupEnvFileString("", key)
}

func ResolveREPLNATSURL() string {
	if raw := LookupEnvString("DIALTONE_REPL_NATS_URL"); raw != "" {
		return raw
	}
	return "nats://127.0.0.1:4222"
}

func ResolveREPLManagerNATSURL() string {
	for _, key := range []string{
		"DIALTONE_REPL_MANAGER_NATS_URL",
		"DIALTONE_REPL_TSNET_NATS_URL",
		"DIALTONE_REPL_NATS_URL",
	} {
		if raw := LookupEnvString(key); raw != "" {
			return raw
		}
	}
	return ""
}

func DefaultDialtoneJSONPath(repoRoot string) string {
	return filepath.Join(repoRoot, "env", "dialtone.json")
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
	if v := lookupEnvFileString("", "DIALTONE_ENV"); v != "" {
		return expandHome(v)
	}
	if home := strings.TrimSpace(os.Getenv("DIALTONE_HOME")); home != "" {
		expanded := expandHome(home)
		parent := filepath.Dir(expanded)
		base := filepath.Base(expanded)
		return filepath.Join(parent, base+"_env")
	}
	if home := lookupEnvFileString("", "DIALTONE_HOME"); home != "" {
		expanded := expandHome(home)
		parent := filepath.Dir(expanded)
		base := filepath.Base(expanded)
		return filepath.Join(parent, base+"_env")
	}
	if configRoot := configRootFromEnvFile(explicitEnvFilePath()); configRoot != "" {
		return filepath.Join(configRoot, ".dialtone_env")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone_env")
}

func DefaultDialtoneHome() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_HOME")); v != "" {
		return expandHome(v)
	}
	if v := lookupEnvFileString("", "DIALTONE_HOME"); v != "" {
		return expandHome(v)
	}
	if configRoot := configRootFromEnvFile(explicitEnvFilePath()); configRoot != "" {
		return filepath.Join(configRoot, ".dialtone")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone")
}

func DefaultGoCacheDir() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_GO_CACHE_DIR")); v != "" {
		return expandHome(v)
	}
	if v := lookupEnvFileString("", "DIALTONE_GO_CACHE_DIR"); v != "" {
		return expandHome(v)
	}
	return filepath.Join(DefaultDialtoneEnv(), "cache", "go")
}

func DefaultBunCacheDir() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_BUN_CACHE_DIR")); v != "" {
		return expandHome(v)
	}
	if v := lookupEnvFileString("", "DIALTONE_BUN_CACHE_DIR"); v != "" {
		return expandHome(v)
	}
	return filepath.Join(DefaultDialtoneEnv(), "cache", "bun")
}

func DefaultPixiCacheDir() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_PIXI_CACHE_DIR")); v != "" {
		return expandHome(v)
	}
	if v := lookupEnvFileString("", "DIALTONE_PIXI_CACHE_DIR"); v != "" {
		return expandHome(v)
	}
	return filepath.Join(DefaultDialtoneEnv(), "cache", "pixi")
}

func DefaultContainerCacheDir() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_CONTAINER_CACHE_DIR")); v != "" {
		return expandHome(v)
	}
	if v := lookupEnvFileString("", "DIALTONE_CONTAINER_CACHE_DIR"); v != "" {
		return expandHome(v)
	}
	return filepath.Join(DefaultDialtoneEnv(), "cache", "containers")
}

func DefaultToolCacheDir() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_TOOL_CACHE_DIR")); v != "" {
		return expandHome(v)
	}
	if v := lookupEnvFileString("", "DIALTONE_TOOL_CACHE_DIR"); v != "" {
		return expandHome(v)
	}
	return filepath.Join(DefaultDialtoneEnv(), "cache", "tools")
}

func DefaultWSLBuildImage() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_WSL_BUILD_IMAGE")); v != "" {
		return v
	}
	if v := lookupEnvFileString("", "DIALTONE_WSL_BUILD_IMAGE"); v != "" {
		return v
	}
	return "dialtone-builder-alpine:go1.25.5"
}

func ManagedGoBinPath(dialtoneEnv string) string {
	env := strings.TrimSpace(dialtoneEnv)
	if env == "" {
		env = DefaultDialtoneEnv()
	}
	name := "go"
	if runtime.GOOS == "windows" {
		name = "go.exe"
	}
	return filepath.Join(env, "go", "bin", name)
}

func ManagedBunBinPath(dialtoneEnv string) string {
	env := strings.TrimSpace(dialtoneEnv)
	if env == "" {
		env = DefaultDialtoneEnv()
	}
	name := "bun"
	if runtime.GOOS == "windows" {
		name = "bun.exe"
	}
	return filepath.Join(env, "bun", "bin", name)
}

func ManagedPixiHomePath(dialtoneEnv string) string {
	env := strings.TrimSpace(dialtoneEnv)
	if env == "" {
		env = DefaultDialtoneEnv()
	}
	return filepath.Join(env, "pixi")
}

func ManagedPixiBinPath(dialtoneEnv string) string {
	home := ManagedPixiHomePath(dialtoneEnv)
	name := "pixi"
	if runtime.GOOS == "windows" {
		name = "pixi.exe"
	}
	return filepath.Join(home, "bin", name)
}

func ResolveRuntime(start string) (Runtime, error) {
	repoRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT"))
	if repoRoot == "" {
		repoRoot = lookupEnvFileString(start, "DIALTONE_REPO_ROOT")
	}
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
		srcRoot = lookupEnvFileString(start, "DIALTONE_SRC_ROOT")
	}
	if srcRoot == "" {
		srcRoot = filepath.Join(repoRoot, "src")
	}
	srcRoot, _ = filepath.Abs(srcRoot)

	envFile := explicitEnvFilePath()
	defaultEnvFile := DefaultDialtoneJSONPath(repoRoot)
	if envFile == "" {
		envFile = defaultEnvFile
	}
	envFile, _ = filepath.Abs(envFile)
	defaultEnvFile, _ = filepath.Abs(defaultEnvFile)

	dialtoneEnv := DefaultDialtoneEnv()
	dialtoneEnv, _ = filepath.Abs(dialtoneEnv)
	dialtoneHome := DefaultDialtoneHome()
	dialtoneHome, _ = filepath.Abs(dialtoneHome)
	goCacheDir := DefaultGoCacheDir()
	goCacheDir, _ = filepath.Abs(goCacheDir)
	bunCacheDir := DefaultBunCacheDir()
	bunCacheDir, _ = filepath.Abs(bunCacheDir)
	pixiCacheDir := DefaultPixiCacheDir()
	pixiCacheDir, _ = filepath.Abs(pixiCacheDir)
	toolCacheDir := DefaultToolCacheDir()
	toolCacheDir, _ = filepath.Abs(toolCacheDir)
	containerCacheDir := DefaultContainerCacheDir()
	containerCacheDir, _ = filepath.Abs(containerCacheDir)
	wslBuildImage := DefaultWSLBuildImage()

	goBin := strings.TrimSpace(LookupEnvString("DIALTONE_GO_BIN"))
	if goBin == "" {
		candidate := ManagedGoBinPath(dialtoneEnv)
		if _, err := os.Stat(candidate); err == nil {
			goBin = candidate
		} else if p, lookErr := exec.LookPath("go"); lookErr == nil {
			goBin = p
		}
	}

	bunBin := strings.TrimSpace(LookupEnvString("DIALTONE_BUN_BIN"))
	if bunBin == "" {
		candidate := ManagedBunBinPath(dialtoneEnv)
		if _, err := os.Stat(candidate); err == nil {
			bunBin = candidate
		} else if p, lookErr := exec.LookPath("bun"); lookErr == nil {
			bunBin = p
		}
	}

	pixiBin := strings.TrimSpace(LookupEnvString("DIALTONE_PIXI_BIN"))
	if pixiBin == "" {
		candidate := ManagedPixiBinPath(dialtoneEnv)
		if _, err := os.Stat(candidate); err == nil {
			pixiBin = candidate
		} else if p, lookErr := exec.LookPath("pixi"); lookErr == nil {
			pixiBin = p
		}
	}

	return Runtime{
		RepoRoot:          repoRoot,
		SrcRoot:           srcRoot,
		EnvFile:           envFile,
		DialtoneHome:      dialtoneHome,
		DialtoneEnv:       dialtoneEnv,
		GoCacheDir:        goCacheDir,
		BunCacheDir:       bunCacheDir,
		PixiCacheDir:      pixiCacheDir,
		ToolCacheDir:      toolCacheDir,
		ContainerCacheDir: containerCacheDir,
		WslBuildImage:     wslBuildImage,
		GoBin:             goBin,
		BunBin:            bunBin,
		PixiBin:           pixiBin,
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
	if strings.TrimSpace(rt.DialtoneHome) != "" {
		_ = os.Setenv("DIALTONE_HOME", rt.DialtoneHome)
	}
	if strings.TrimSpace(rt.DialtoneEnv) != "" {
		_ = os.Setenv("DIALTONE_ENV", rt.DialtoneEnv)
	}
	if strings.TrimSpace(rt.GoCacheDir) != "" {
		_ = os.Setenv("DIALTONE_GO_CACHE_DIR", rt.GoCacheDir)
	}
	if strings.TrimSpace(rt.BunCacheDir) != "" {
		_ = os.Setenv("DIALTONE_BUN_CACHE_DIR", rt.BunCacheDir)
	}
	if strings.TrimSpace(rt.PixiCacheDir) != "" {
		_ = os.Setenv("DIALTONE_PIXI_CACHE_DIR", rt.PixiCacheDir)
	}
	if strings.TrimSpace(rt.ToolCacheDir) != "" {
		_ = os.Setenv("DIALTONE_TOOL_CACHE_DIR", rt.ToolCacheDir)
	}
	if strings.TrimSpace(rt.ContainerCacheDir) != "" {
		_ = os.Setenv("DIALTONE_CONTAINER_CACHE_DIR", rt.ContainerCacheDir)
	}
	if strings.TrimSpace(rt.WslBuildImage) != "" {
		_ = os.Setenv("DIALTONE_WSL_BUILD_IMAGE", rt.WslBuildImage)
	}
	if strings.TrimSpace(rt.GoBin) != "" {
		_ = os.Setenv("DIALTONE_GO_BIN", rt.GoBin)
	}
	if strings.TrimSpace(rt.BunBin) != "" {
		_ = os.Setenv("DIALTONE_BUN_BIN", rt.BunBin)
	}
	if strings.TrimSpace(rt.PixiBin) != "" {
		_ = os.Setenv("DIALTONE_PIXI_BIN", rt.PixiBin)
	}

	prepend := []string{}
	if strings.TrimSpace(rt.GoBin) != "" {
		prepend = append(prepend, filepath.Dir(rt.GoBin))
	}
	if strings.TrimSpace(rt.BunBin) != "" {
		prepend = append(prepend, filepath.Dir(rt.BunBin))
	}
	if strings.TrimSpace(rt.PixiBin) != "" {
		prepend = append(prepend, filepath.Dir(rt.PixiBin))
	}
	prependPath(prepend...)
	return nil
}

func LoadEnvFile(rt Runtime) error {
	if strings.TrimSpace(rt.EnvFile) == "" {
		return nil
	}
	config, err := ReadEnvFileMap(rt.EnvFile)
	if err != nil {
		return err
	}
	for key, value := range config {
		if text := stringifyEnvValue(value); text != "" {
			_ = os.Setenv(key, text)
		}
	}
	return nil
}

func stringifyEnvValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
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
