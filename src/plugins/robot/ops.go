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

	"dialtone/cli/src/core/logger"
	core_ssh "dialtone/cli/src/core/ssh"
	test_v2 "dialtone/cli/src/libs/test_v2"
	robot_ops "dialtone/cli/src/plugins/robot/src_v1/cmd/ops"
)

// Versioned Source Commands (routing to ops files)

func RunInstall(versionDir string, flags ...string) error {
	if versionDir == "src_v1" {
		// Check for --remote flag
		for _, flag := range flags {
			if flag == "--remote" {
				return RunRemoteInstall(versionDir, flags)
			}
		}
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

func RunRemoteInstall(versionDir string, flags []string) error {
	logger.LogInfo("[ROBOT] Remote install for %s requested.", versionDir)

	// 1. Sync code to the remote robot (includes env/.env now)
	RunSyncCode(versionDir, []string{})

	// 2. SSH into the robot and run the install command
	host := os.Getenv("ROBOT_HOST")
	user := os.Getenv("ROBOT_USER")
	pass := os.Getenv("ROBOT_PASSWORD")

	if host == "" || user == "" || pass == "" {
		logger.LogFatal("Error: ROBOT_HOST, ROBOT_USER, ROBOT_PASSWORD must be set for remote install.")
	}

	client, err := core_ssh.DialSSH(host, "22", user, pass)
	if err != nil {
		logger.LogFatal("Failed to connect to robot via SSH: %v", err)
	}
	defer client.Close()

	cwd, _ := os.Getwd()
	baseDir := filepath.Base(cwd)
	remoteHome := os.Getenv("REMOTE_DIR_SRC")
	if remoteHome == "" {
		remoteHome = fmt.Sprintf("/home/%s/%s", user, baseDir)
	}
	
	// Set DIALTONE_ENV explicitly for the remote command and ensure we use the same relative path
	// MIRROR LOGIC: We assume the robot has the same directory structure.
	remoteCmd := fmt.Sprintf("cd %s && export DIALTONE_ENV=env/.env && ./dialtone.sh install && ./dialtone.sh robot install %s", remoteHome, versionDir)
	logger.LogInfo("[ROBOT] Executing remote install: %s", remoteCmd)

	output, err := core_ssh.RunSSHCommand(client, remoteCmd)
	if err != nil {
		logger.LogFatal("Remote install failed: %v\nOutput: %s", err, output)
	}

	logger.LogInfo("[ROBOT] Remote install successful.")
	fmt.Print(output)
	return nil
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
		// Check for --remote flag
		for _, flag := range flags {
			if flag == "--remote" {
				return RunRemoteBuild(versionDir, flags)
			}
		}
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

func RunDev(versionDir string, args []string) error {
	if versionDir == "src_v1" {
		return robot_ops.Dev(args)
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

func RunRemoteBuild(versionDir string, flags []string) error {
	logger.LogInfo("[ROBOT] Remote build for %s requested.", versionDir)

	// 1. Sync code to the remote robot (includes env/.env now)
	RunSyncCode(versionDir, []string{})

	// 2. SSH into the robot and run the build command
	host := os.Getenv("ROBOT_HOST")
	user := os.Getenv("ROBOT_USER")
	pass := os.Getenv("ROBOT_PASSWORD")

	if host == "" || user == "" || pass == "" {
		logger.LogFatal("Error: ROBOT_HOST, ROBOT_USER, ROBOT_PASSWORD must be set for remote build.")
	}

	client, err := core_ssh.DialSSH(host, "22", user, pass)
	if err != nil {
		logger.LogFatal("Failed to connect to robot via SSH: %v", err)
	}
	defer client.Close()

	cwd, _ := os.Getwd()
	baseDir := filepath.Base(cwd)
	remoteHome := os.Getenv("REMOTE_DIR_SRC")
	if remoteHome == "" {
		remoteHome = fmt.Sprintf("/home/%s/%s", user, baseDir)
	}
	
	// Set DIALTONE_ENV explicitly for the remote command and ensure we use the same relative path
	remoteCmd := fmt.Sprintf("cd %s && export DIALTONE_ENV=env/.env && ./dialtone.sh robot build %s", remoteHome, versionDir)
	logger.LogInfo("[ROBOT] Executing remote build: %s", remoteCmd)

	output, err := core_ssh.RunSSHCommand(client, remoteCmd)
	if err != nil {
		logger.LogFatal("Remote build failed: %v\nOutput: %s", err, output)
	}

	logger.LogInfo("[ROBOT] Remote build successful.")
	fmt.Print(output) // Print remote build output
	return nil
}
