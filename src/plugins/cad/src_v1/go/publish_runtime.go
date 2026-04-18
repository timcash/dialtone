package cad

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func ensurePublishServer(paths Paths, plan PublishPlan) error {
	ok, _ := checkHealth(plan.BackendPort)
	if ok {
		return nil
	}
	if err := os.MkdirAll(plan.StateDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(plan.StateDir, "cad-publish-server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := prepareNestedDialtoneCommand(paths.Runtime, "cad", "src_v1", "serve", "--port", strconv.Itoa(plan.BackendPort))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	configureDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return waitForHTTPOK(plan.LocalBackendURL+"/health", 25*time.Second)
}

func ensurePublishTunnel(paths Paths, plan PublishPlan) error {
	if err := waitForHTTPOK(plan.BackendOrigin+"/health", 5*time.Second); err == nil {
		return nil
	}
	if err := runNestedDialtone(paths.Runtime, "cloudflare", "src_v1", "provision", plan.TunnelName, "--domain", plan.Domain); err != nil {
		return err
	}
	if err := runNestedDialtone(paths.Runtime, "cloudflare", "src_v1", "tunnel", "start", plan.TunnelName, "--url", plan.LocalBackendURL); err != nil {
		return err
	}
	return waitForHTTPOK(plan.BackendOrigin+"/health", 75*time.Second)
}

func buildPublishPages(paths Paths, plan PublishPlan) error {
	if err := os.RemoveAll(plan.OutputRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(plan.OutputRoot, 0o755); err != nil {
		return err
	}
	if err := ensurePublishUIDeps(paths); err != nil {
		return err
	}

	bunBin, err := ResolveBunBinary(paths)
	if err != nil {
		return err
	}
	cmd := exec.Command(bunBin, "run", "build")
	cmd.Dir = paths.UIDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = withEnvMap(os.Environ(), map[string]string{
		"VITE_SITE_BASE_PATH":   plan.PagesBasePath,
		"VITE_CAD_API_BASE_URL": plan.BackendOrigin,
		"VITE_BUILD_OUT_DIR":    plan.AppDir,
	})
	if err := cmd.Run(); err != nil {
		return err
	}
	if err := copyFile(plan.AppIndex, plan.App404); err != nil {
		return err
	}
	if err := os.WriteFile(plan.NoJekyllPath, []byte(""), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(plan.RootIndex, []byte(BuildPublishLandingHTML(plan)+"\n"), 0o644); err != nil {
		return err
	}
	meta, err := MarshalPublishMetadata(plan)
	if err != nil {
		return err
	}
	if err := os.WriteFile(plan.MetadataPath, append(meta, '\n'), 0o644); err != nil {
		return err
	}
	return nil
}

func writePublishState(plan PublishPlan) error {
	if err := os.MkdirAll(plan.StateDir, 0o755); err != nil {
		return err
	}
	meta, err := MarshalPublishMetadata(plan)
	if err != nil {
		return err
	}
	return os.WriteFile(plan.StatePath, append(meta, '\n'), 0o644)
}

func ensurePublishUIDeps(paths Paths) error {
	if _, err := os.Stat(filepath.Join(paths.UIDir, "node_modules")); err == nil {
		return nil
	}
	bunBin, err := ResolveBunBinary(paths)
	if err != nil {
		return err
	}
	cmd := exec.Command(bunBin, "install")
	cmd.Dir = paths.UIDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func prepareNestedDialtoneCommand(rt configv1.Runtime, args ...string) *exec.Cmd {
	cmd := configv1.DialtoneCommand(rt.RepoRoot, args...)
	cmd.Dir = rt.RepoRoot
	cmd.Env = withEnvMap(os.Environ(), map[string]string{
		"DIALTONE_CONTEXT":   "repl",
		"DIALTONE_REPO_ROOT": rt.RepoRoot,
		"DIALTONE_SRC_ROOT":  rt.SrcRoot,
		"DIALTONE_ENV_FILE":  rt.EnvFile,
		"DIALTONE_HOME":      rt.DialtoneHome,
		"DIALTONE_ENV":       rt.DialtoneEnv,
		"DIALTONE_GO_BIN":    rt.GoBin,
		"DIALTONE_BUN_BIN":   rt.BunBin,
		"DIALTONE_PIXI_BIN":  rt.PixiBin,
	})
	return cmd
}

func runNestedDialtone(rt configv1.Runtime, args ...string) error {
	cmd := prepareNestedDialtoneCommand(rt, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func withEnvMap(env []string, values map[string]string) []string {
	result := append([]string{}, env...)
	for key, value := range values {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		prefix := key + "="
		replaced := false
		for i, item := range result {
			if strings.HasPrefix(item, prefix) {
				result[i] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, prefix+value)
		}
	}
	return result
}

func waitForHTTPOK(rawURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(strings.TrimSpace(rawURL))
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", rawURL)
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}
