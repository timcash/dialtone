package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	fmt.Printf(">> [Robot] Building UI: src_v1\n")
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
	fmt.Printf(">> [Robot] Building server: src_v1\n")
	// Use dialtone.sh go src_v1 exec build
	buildArgs := []string{"go", "src_v1", "exec", "build", "-o", filepath.Join(robotBinDir, "robot-src_v1"), "./plugins/robot/src_v1/cmd/server/main.go"}
	buildArgs = append(buildArgs, localFlags...)
	buildCmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), buildArgs...)
	buildCmd.Dir = repoRoot
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	return buildCmd.Run()
}
