package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func Test(extraArgs []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	testPkg := "./" + filepath.Join("src", "plugins", "robot", "src_v1", "test")

	args := []string{"go", "exec", "run", testPkg}
	args = append(args, extraArgs...)

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass ROBOT_TEST_ATTACH=1 if --attach is present
	for _, arg := range extraArgs {
		if arg == "--attach" {
			cmd.Env = append(os.Environ(), "ROBOT_TEST_ATTACH=1")
			break
		}
	}

	return cmd.Run()
}
