package ops

import (
	"os"
	"os/exec"
	"path/filepath"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Build(flags ...string) error {
	remoteOpts, localFlags, err := parseRemoteOptions("robot-build", flags)
	if err != nil {
		return err
	}
	if remoteOpts.Remote {
		return runRemoteBuild(remoteOpts)
	}

	paths, err := resolveRobotPathsPreset()
	if err != nil {
		return err
	}
	repoRoot := paths.Runtime.RepoRoot
	uiDir := paths.Preset.UI

	logs.Info(">> [Robot] Building UI: src_v1")
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "bun", "src_v1", "exec", "--cwd", uiDir, "run", "build")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	robotBinDir := paths.Preset.Bin
	if err := os.MkdirAll(robotBinDir, 0755); err != nil {
		return err
	}
	logs.Info(">> [Robot] Building server: src_v1")
	// Use dialtone.sh go src_v1 exec build
	buildArgs := []string{"go", "src_v1", "exec", "build", "-o", filepath.Join(robotBinDir, "robot-src_v1"), "./plugins/robot/src_v1/cmd/server/main.go"}
	buildArgs = append(buildArgs, localFlags...)
	buildCmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), buildArgs...)
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	return buildCmd.Run()
}
