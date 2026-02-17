package robot

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	robot_ops "dialtone/cli/src/plugins/robot/src_v1/cmd/ops"
)

// Versioned Source Commands (routing to ops files)

func RunInstall(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Install()
	}
	fmt.Printf(">> [Robot] Install: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "install", "--force")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFmt(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Fmt()
	}
	fmt.Printf(">> [Robot] Fmt: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/robot/"+versionDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Format()
	}
	fmt.Printf(">> [Robot] Format: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "format")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunVet(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Vet()
	}
	fmt.Printf(">> [Robot] Vet: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/robot/"+versionDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.GoBuild()
	}
	fmt.Printf(">> [Robot] Go Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/robot/"+versionDir+"/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Lint()
	}
	fmt.Printf(">> [Robot] Lint: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "lint")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunBuild(versionDir string, flags ...string) error {
	if versionDir == "src_v1" {
		return robot_ops.Build(flags...)
	}
	fmt.Printf(">> [Robot] Build: %s\n", versionDir)
	// Fallback/Generic build could be added here
	return nil
}

func RunServe(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Serve()
	}
	fmt.Printf(">> [Robot] Serve: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.Join("src", "plugins", "robot", versionDir, "cmd", "main.go"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	if versionDir == "src_v1" {
		return robot_ops.UIRun(port)
	}
	fmt.Printf(">> [Robot] UI Run: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLocalWebRemoteRobot(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.LocalWebRemoteRobot()
	}
	return fmt.Errorf("local-web-remote-robot not implemented for %s", versionDir)
}

func RunDev(versionDir string) error {
	if versionDir == "src_v1" {
		return robot_ops.Dev()
	}
	fmt.Printf(">> [Robot] Dev: %s\n", versionDir)
	cwd, _ := os.Getwd()
	
	// 1. Ensure UI dev server is running in the background
	uiDir := filepath.Join(cwd, "src", "plugins", "robot", versionDir, "ui")
	devCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "bun", "exec", "--cwd", uiDir, "run", "dev", "--host", "127.0.0.1", "--port", "3000")
	devCmd.Stdout = os.Stdout
	devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dev server: %w", err)
	}
	defer devCmd.Process.Kill()

	// 2. Wait for dev server
	if err := test_v2.WaitForPort(3000, 15*time.Second); err != nil {
		return fmt.Errorf("dev server failed to start: %w", err)
	}

	// 3. Launch or attach to Chrome dev session
	chromeCmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "chrome", "new", "http://127.0.0.1:3000", "--role", "dev", "--reuse-existing", "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		return fmt.Errorf("failed to launch chrome: %w", err)
	}

	// 4. Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	return nil
}

func RunVersionedTest(versionDir string, extraArgs []string) error {
	if versionDir == "src_v1" {
		return robot_ops.Test(extraArgs)
	}
	cwd, _ := os.Getwd()
	testPkg := "./" + filepath.Join("src", "plugins", "robot", versionDir, "test")
	
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
