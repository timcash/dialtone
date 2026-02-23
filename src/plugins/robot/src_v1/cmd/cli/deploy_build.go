package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func buildRobotUI(repoRoot, versionDir string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	preset := configv1.NewPluginPreset(rt, "robot", versionDir)
	uiDir := preset.UI
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "src_v1", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logs.Info("[DEPLOY] Building Robot UI...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build UI failed: %w", err)
	}
	return nil
}

func buildRobotBinary(repoRoot, versionDir, goos, goarch string) (string, error) {
	_ = versionDir
	outDir := filepath.Join(repoRoot, ".dialtone", "bin")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	binaryName := fmt.Sprintf("robot-src_v1-%s-%s", goos, goarch)
	out := filepath.Join(outDir, binaryName)

	// If current arch matches target, build locally.
	if goos == runtime.GOOS && (goarch == runtime.GOARCH || (runtime.GOARCH == "amd64" && goarch == "x86_64")) {
		dialtoneEnv := logs.GetDialtoneEnv()
		goBin := filepath.Join(dialtoneEnv, "go", "bin", "go")
		if _, err := os.Stat(goBin); err != nil {
			fallback, lookErr := exec.LookPath("go")
			if lookErr != nil {
				return "", fmt.Errorf("go binary not found (managed and PATH)")
			}
			goBin = fallback
		}

		cmd := exec.Command(goBin, "build", "-o", out, "./plugins/robot/src_v1/cmd/server/main.go")
		rt, rtErr := configv1.ResolveRuntime(repoRoot)
		if rtErr != nil {
			return "", rtErr
		}
		cmd.Dir = rt.SrcRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch, "GOROOT="+filepath.Join(dialtoneEnv, "go"))
		logs.Info("[DEPLOY] Cross-compiling robot server for %s/%s (Local)...", goos, goarch)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("local build failed: %w", err)
		}
		return out, nil
	}

	// Cross-compilation path: Podman.
	logs.Info("[DEPLOY] Cross-compiling robot server for %s/%s (Podman)...", goos, goarch)
	if _, err := exec.LookPath("podman"); err != nil {
		return "", fmt.Errorf("podman is required for cross-compilation but not found in PATH")
	}

	dockerfilePath := filepath.Join(repoRoot, "containers", "Dockerfile.arm")
	imageName := "dialtone-builder-arm"

	buildImg := exec.Command("podman", "build", "-t", imageName, "-f", dockerfilePath, ".")
	buildImg.Dir = repoRoot
	buildImg.Stdout = os.Stdout
	buildImg.Stderr = os.Stderr
	if err := buildImg.Run(); err != nil {
		return "", fmt.Errorf("podman build failed: %w", err)
	}

	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return "", err
	}
	srcDir := rt.SrcRoot
	remoteBinPath := "/src/plugins/robot/bin/" + binaryName
	var gcc string
	if goarch == "arm64" || goarch == "aarch64" {
		gcc = "aarch64-linux-gnu-gcc"
	} else {
		gcc = "arm-linux-gnueabihf-gcc"
	}

	dialtoneEnv := logs.GetDialtoneEnv()
	goModCache := filepath.Join(os.Getenv("HOME"), "go", "pkg", "mod")
	if _, err := os.Stat(goModCache); os.IsNotExist(err) {
		altCache := filepath.Join(dialtoneEnv, "go", "pkg", "mod")
		if _, err := os.Stat(altCache); err == nil {
			goModCache = altCache
		}
	}

	podmanArgs := []string{"run", "--rm",
		"-v", srcDir + ":/src:z",
		"-w", "/src",
		"-e", "CGO_ENABLED=1",
		"-e", "GOOS=" + goos,
		"-e", "GOARCH=" + goarch,
		"-e", "CC=" + gcc,
		"-e", "GOPATH=/go",
	}
	if _, err := os.Stat(goModCache); err == nil {
		podmanArgs = append(podmanArgs, "-v", goModCache+":/go/pkg/mod:z")
	}
	podmanArgs = append(podmanArgs, imageName,
		"go", "build", "-o", remoteBinPath, "./plugins/robot/src_v1/cmd/server/main.go")

	runCmd := exec.Command("podman", podmanArgs...)
	runCmd.Dir = repoRoot
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return "", fmt.Errorf("podman run build failed: %w", err)
	}

	builtBin := filepath.Join(srcDir, "plugins", "robot", "bin", binaryName)
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(builtBin, out); err != nil {
		input, readErr := os.ReadFile(builtBin)
		if readErr != nil {
			return "", readErr
		}
		if writeErr := os.WriteFile(out, input, 0o755); writeErr != nil {
			return "", writeErr
		}
	}

	return out, nil
}
