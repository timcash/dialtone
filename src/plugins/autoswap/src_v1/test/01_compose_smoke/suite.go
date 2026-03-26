package composesmoke

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "01-build-compose-artifacts",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("compose artifacts built", 120*time.Second, func() error {
				repo := repoRoot()
				autoswapBin := configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repo}, "autoswap", "src_v1", "dialtone_autoswap_v1")
				robotBin := configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repo}, "robot", "src_v2", "dialtone_robot_v2")
				cameraBin := configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repo}, "camera", "src_v1", "dialtone_camera_v1")
				mavlinkBin := configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repo}, "mavlink", "src_v1", "dialtone_mavlink_v1")
				replBin := configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repo}, "repl", "src_v1", "dialtone_repl_v1")
				builds := [][]string{
					{"./dialtone.sh", "go", "src_v1", "exec", "build", "-o", autoswapBin, "./plugins/autoswap/src_v1/cmd/main.go"},
					{"./dialtone.sh", "go", "src_v1", "exec", "build", "-o", robotBin, "./plugins/robot/src_v2/cmd/server/main.go"},
					{"./dialtone.sh", "camera", "src_v1", "build", "--goos", "linux", "--goarch", "amd64", "--out", cameraBin, "--podman=false"},
					{"./dialtone.sh", "go", "src_v1", "exec", "build", "-o", mavlinkBin, "./plugins/mavlink/src_v1/cmd/main.go"},
					{"./dialtone.sh", "go", "src_v1", "exec", "build", "-o", replBin, "./plugins/repl/src_v1/cmd/repld/main.go"},
					{"./dialtone.sh", "robot", "src_v2", "build"},
				}
				for _, args := range builds {
					if err := runCmd(repo, args[0], args[1:]...); err != nil {
						ctx.Errorf("build command failed: %v", err)
						return err
					}
				}
				// Stage built artifacts into autoswap-managed paths so compose manifest
				// can run without relying on src repo paths.
				autoswapRoot := filepath.Join(os.Getenv("HOME"), ".dialtone", "autoswap")
				artifactDir := filepath.Join(autoswapRoot, "artifacts")
				if err := os.MkdirAll(artifactDir, 0o755); err != nil {
					ctx.Errorf("artifact dir create failed: %v", err)
					return err
				}
				copies := [][2]string{
					{autoswapBin, filepath.Join(artifactDir, "dialtone_autoswap_v1")},
					{robotBin, filepath.Join(artifactDir, "dialtone_robot_v2")},
					{cameraBin, filepath.Join(artifactDir, "dialtone_camera_v1")},
					{mavlinkBin, filepath.Join(artifactDir, "dialtone_mavlink_v1")},
					{replBin, filepath.Join(artifactDir, "dialtone_repl_v1")},
				}
				for _, cp := range copies {
					if err := runCmd(repo, "cp", "-f", cp[0], cp[1]); err != nil {
						ctx.Errorf("artifact copy failed: %s -> %s (%v)", cp[0], cp[1], err)
						return err
					}
					if err := runCmd(repo, "chmod", "+x", cp[1]); err != nil {
						ctx.Errorf("artifact chmod failed: %s (%v)", cp[1], err)
						return err
					}
				}
				uiSrc := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "ui", "dist")
				uiDst := filepath.Join(artifactDir, "robot_src_v2_ui_dist")
				if err := runCmd(repo, "rm", "-rf", uiDst); err != nil {
					ctx.Errorf("ui artifact reset failed: %v", err)
					return err
				}
				if err := runCmd(repo, "cp", "-a", uiSrc, uiDst); err != nil {
					ctx.Errorf("ui artifact copy failed: %v", err)
					return err
				}
				ctx.Infof("compose artifacts built")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "compose artifacts built"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "02-stage-manifest-artifacts",
		Timeout: 20 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("manifest stage succeeded", 20*time.Second, func() error {
				repo := repoRoot()
				manifest := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
				if err := runCmd(repo, "./dialtone.sh", "autoswap", "src_v1", "stage", "--manifest", manifest, "--repo-root", repo); err != nil {
					ctx.Errorf("autoswap stage failed: %v", err)
					return err
				}
				ctx.Infof("manifest stage succeeded")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "manifest stage smoke verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "03-run-compose-stack",
		Timeout: 45 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if err := ctx.WaitForStepMessageAfterAction("autoswap compose run succeeded", 45*time.Second, func() error {
				repo := repoRoot()
				manifest := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
				err := runCmd(
					repo,
					"./dialtone.sh", "autoswap", "src_v1", "run",
					"--manifest", manifest,
					"--repo-root", repo,
					"--listen", ":18086",
					"--nats-port", "18236",
					"--nats-ws-port", "18237",
					"--timeout", "35s",
				)
				if err != nil {
					ctx.Errorf("autoswap compose run failed: %v", err)
					return err
				}
				ctx.Infof("autoswap compose run succeeded")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "autoswap compose run verified"}, nil
		},
	})
}

func runCmd(repo, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func repoRoot() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); v != "" {
		return v
	}
	cwd, _ := os.Getwd()
	return cwd
}
