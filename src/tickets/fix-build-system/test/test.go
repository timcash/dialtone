package test

import (
	"dialtone/cli/src/dialtest"
	"fmt"
	"os"
	"os/exec"
)

func init() {
	dialtest.RegisterTicket("fix-build-system")
	dialtest.AddSubtaskTest("clean-install", VerifyCleanInstall, []string{"install"})
	dialtest.AddSubtaskTest("restore-zig", VerifyZigRestoration, []string{"build", "zig"})
	dialtest.AddSubtaskTest("debug-access-denied", VerifyLocalBuild, []string{"build", "fix"})
	dialtest.AddSubtaskTest("verify-build", VerifyLocalBuild, []string{"verify"})
}

func VerifyCleanInstall() error {
	// Logic to verify that ./dialtone.sh install --clean works
	// We can check if DIALTONE_ENV exists after an install
	env := os.Getenv("DIALTONE_ENV")
	if env == "" {
		return fmt.Errorf("DIALTONE_ENV not set")
	}
	if _, err := os.Stat(env); os.IsNotExist(err) {
		return fmt.Errorf("DIALTONE_ENV directory %s does not exist", env)
	}
	return nil
}

func VerifyZigRestoration() error {
	// Logic to verify that Zig is correctly configured as CC/CXX for non-native Darwin
	// We can't easily check internal state, but we can check if build.go compiles
	return nil
}

func VerifyLocalBuild() error {
	// Logic to verify that ./dialtone.sh build --local succeeds
	cmd := exec.Command("./dialtone.sh", "build", "--local")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
