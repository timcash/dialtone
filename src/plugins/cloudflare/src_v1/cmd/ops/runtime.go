package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cloudflarev1 "dialtone/dev/plugins/cloudflare/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func resolveCloudflarePaths() (cloudflarev1.Paths, error) {
	return cloudflarev1.ResolvePaths("", "src_v1")
}

func runDialtone(repoRoot string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{}, args...)
	if rt, err := configv1.ResolveRuntime(repoRoot); err == nil {
		if envFile := strings.TrimSpace(rt.EnvFile); envFile != "" {
			cmdArgs = append([]string{"--env", envFile}, cmdArgs...)
		}
	}
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = repoRoot
	cmd.Env = cloudflareRuntimeEnv(repoRoot)
	return cmd
}

func cloudflareRuntimeEnv(repoRoot string) []string {
	env := append([]string{}, os.Environ()...)
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return env
	}
	for _, pair := range [][2]string{
		{"DIALTONE_REPO_ROOT", rt.RepoRoot},
		{"DIALTONE_SRC_ROOT", rt.SrcRoot},
		{"DIALTONE_ENV_FILE", rt.EnvFile},
		{"DIALTONE_MESH_CONFIG", rt.EnvFile},
		{"DIALTONE_HOME", rt.DialtoneHome},
		{"DIALTONE_ENV", rt.DialtoneEnv},
		{"DIALTONE_GO_CACHE_DIR", rt.GoCacheDir},
		{"DIALTONE_BUN_CACHE_DIR", rt.BunCacheDir},
		{"DIALTONE_PIXI_CACHE_DIR", rt.PixiCacheDir},
		{"DIALTONE_TOOL_CACHE_DIR", rt.ToolCacheDir},
		{"DIALTONE_CONTAINER_CACHE_DIR", rt.ContainerCacheDir},
		{"DIALTONE_WSL_BUILD_IMAGE", rt.WslBuildImage},
		{"DIALTONE_GO_BIN", rt.GoBin},
		{"DIALTONE_BUN_BIN", rt.BunBin},
		{"DIALTONE_PIXI_BIN", rt.PixiBin},
	} {
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		if key == "" || value == "" {
			continue
		}
		env = append(env, key+"="+value)
	}
	return env
}
