package wslv3

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type BuildImageConfig struct {
	ImageName      string
	CacheDir       string
	DockerfilePath string
	ContextDir     string
	StatePath      string
}

type buildImageState struct {
	ImageName     string `json:"image_name"`
	DockerfileSHA string `json:"dockerfile_sha"`
	GoModSHA      string `json:"go_mod_sha"`
	GoSumSHA      string `json:"go_sum_sha"`
}

func ResolveBuildImageConfig(start string) (BuildImageConfig, error) {
	rt, err := configv1.ResolveRuntime(start)
	if err != nil {
		return BuildImageConfig{}, err
	}
	cacheDir := strings.TrimSpace(rt.ContainerCacheDir)
	if cacheDir == "" {
		cacheDir = configv1.DefaultContainerCacheDir()
	}
	cacheDir, _ = filepath.Abs(cacheDir)
	return BuildImageConfig{
		ImageName:      rt.WslBuildImage,
		CacheDir:       cacheDir,
		DockerfilePath: filepath.Join(rt.RepoRoot, "containers", "Dockerfile.builder-alpine"),
		ContextDir:     rt.RepoRoot,
		StatePath:      filepath.Join(cacheDir, "wsl", "build-image-state.json"),
	}, nil
}

func EnsureBuildImage(start string) (BuildImageConfig, error) {
	cfg, err := ResolveBuildImageConfig(start)
	if err != nil {
		return BuildImageConfig{}, err
	}
	if _, err := exec.LookPath("podman"); err != nil {
		return BuildImageConfig{}, fmt.Errorf("podman is required for managed alpine build image but not found in PATH")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.StatePath), 0o755); err != nil {
		return BuildImageConfig{}, err
	}

	current, err := currentBuildImageState(cfg)
	if err != nil {
		return BuildImageConfig{}, err
	}
	if imageExists(cfg.ImageName) {
		if cached, err := readBuildImageState(cfg.StatePath); err == nil && cached == current {
			return cfg, nil
		}
	}
	if err := buildImage(cfg); err != nil {
		return BuildImageConfig{}, err
	}
	if err := writeBuildImageState(cfg.StatePath, current); err != nil {
		return BuildImageConfig{}, err
	}
	return cfg, nil
}

func imageExists(name string) bool {
	cmd := exec.Command("podman", "image", "exists", name)
	return cmd.Run() == nil
}

func buildImage(cfg BuildImageConfig) error {
	cmd := exec.Command("podman", "build", "-t", cfg.ImageName, "-f", cfg.DockerfilePath, cfg.ContextDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func currentBuildImageState(cfg BuildImageConfig) (buildImageState, error) {
	dockerfileSHA, err := sha256File(cfg.DockerfilePath)
	if err != nil {
		return buildImageState{}, err
	}
	goModSHA, err := sha256File(filepath.Join(cfg.ContextDir, "src", "go.mod"))
	if err != nil {
		return buildImageState{}, err
	}
	goSumSHA, err := sha256File(filepath.Join(cfg.ContextDir, "src", "go.sum"))
	if err != nil {
		return buildImageState{}, err
	}
	return buildImageState{
		ImageName:     cfg.ImageName,
		DockerfileSHA: dockerfileSHA,
		GoModSHA:      goModSHA,
		GoSumSHA:      goSumSHA,
	}, nil
}

func readBuildImageState(path string) (buildImageState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return buildImageState{}, err
	}
	var state buildImageState
	if err := json.Unmarshal(raw, &state); err != nil {
		return buildImageState{}, err
	}
	return state, nil
}

func writeBuildImageState(path string, state buildImageState) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func sha256File(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
