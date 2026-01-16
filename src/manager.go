package dialtone

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"os/exec"

	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// LoadConfig loads environment variables from .env
func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		LogInfo("Warning: godotenv.Load() failed: %v", err)
	}
}

// RunBuild handles building for different platforms
func RunBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	full := fs.Bool("full", false, "Build Web UI, local CLI, and ARM64 binary")
	local := fs.Bool("local", false, "Build natively on the local system")
	showHelp := fs.Bool("help", false, "Show help for build command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone build [options]")
		fmt.Println()
		fmt.Println("Build the Dialtone binary and web UI for deployment.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --local    Build natively on the local system (uses ~/.dialtone_env if available)")
		fmt.Println("  --full     Full rebuild: Web UI + local CLI + ARM64 binary")
		fmt.Println("  --help     Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone build              # Build web UI + binary (Podman or local)")
		fmt.Println("  dialtone build --local      # Build web UI + native binary")
		fmt.Println("  dialtone build --full       # Force full rebuild of everything")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Automatically builds web UI if not already built")
		fmt.Println("  - Uses Podman by default for ARM64 cross-compilation")
		fmt.Println("  - Falls back to local build if Podman is not installed")
		fmt.Println("  - Run 'dialtone install' first to set up build dependencies")
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	if *full {
		buildEverything(*local)
	} else {
		if *local || !hasPodman() {
			buildLocally()
		} else {
			buildWithPodman()
		}
	}
}

func hasPodman() bool {
	_, err := exec.LookPath("podman")
	return err == nil
}

// buildWebIfNeeded builds the web UI if web_build is missing or empty
func buildWebIfNeeded() {
	webBuildDir := filepath.Join("src", "web_build")
	indexPath := filepath.Join(webBuildDir, "index.html")

	// Check if index.html exists and has real content
	if info, err := os.Stat(indexPath); err == nil && info.Size() > 100 {
		LogInfo("Web UI already built (found %s)", indexPath)
		return
	}

	LogInfo("Building Web UI...")

	// Check if src/web exists
	webDir := filepath.Join("src", "web")
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		LogInfo("Warning: src/web directory not found, skipping web build")
		return
	}

	// Check for npm
	if _, err := exec.LookPath("npm"); err != nil {
		// Try to use npm from .dialtone_env
		homeDir, _ := os.UserHomeDir()
		npmPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "npm")
		if _, err := os.Stat(npmPath); os.IsNotExist(err) {
			LogInfo("Warning: npm not found, skipping web build. Run 'dialtone install' first.")
			return
		}
		// Add node to PATH
		nodeBin := filepath.Join(homeDir, ".dialtone_env", "node", "bin")
		os.Setenv("PATH", fmt.Sprintf("%s:%s", nodeBin, os.Getenv("PATH")))
	}

	// Install and build
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// Sync to web_build
	LogInfo("Syncing web assets to src/web_build...")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		LogFatal("Failed to create web_build dir: %v", err)
	}

	distDir := filepath.Join(webDir, "dist")
	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		LogInfo("Warning: npm build did not create dist directory")
		return
	}

	copyDir(distDir, webBuildDir)
	LogInfo("Web UI build complete")
}

func buildLocally() {
	LogInfo("Building Dialtone locally (Native Build)...")

	// Build web UI if needed
	buildWebIfNeeded()

	if err := os.MkdirAll("bin", 0755); err != nil {
		LogFatal("Failed to create bin directory: %v", err)
	}

	// For local builds, we enable CGO to support V4L2 drivers
	os.Setenv("CGO_ENABLED", "1")

	// If local environment exists, use it
	homeDir, _ := os.UserHomeDir()
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	if _, err := os.Stat(depsDir); err == nil {
		LogInfo("Using local dependencies from %s", depsDir)
		
		// Add Go and Node to PATH
		goBin := filepath.Join(depsDir, "go", "bin")
		nodeBin := filepath.Join(depsDir, "node", "bin")
		os.Setenv("PATH", fmt.Sprintf("%s:%s:%s", goBin, nodeBin, os.Getenv("PATH")))

		// If Zig exists, use it as C compiler
		zigPath := filepath.Join(depsDir, "zig", "zig")
		if _, err := os.Stat(zigPath); err == nil {
			os.Setenv("CC", fmt.Sprintf("%s cc -target x86_64-linux-gnu", zigPath))
		}

		// Add include paths for CGO (V4L2 headers)
		includePath := filepath.Join(depsDir, "usr", "include")
		cgoCflags := fmt.Sprintf("-I%s", includePath)

		// Also check for multiarch include path (e.g. x86_64-linux-gnu)
		matches, _ := filepath.Glob(filepath.Join(includePath, "*-linux-gnu"))
		for _, match := range matches {
			cgoCflags += fmt.Sprintf(" -I%s", match)
		}
		os.Setenv("CGO_CFLAGS", cgoCflags)
	}

	// Choose binary name based on OS
	binaryName := "dialtone"
	if runtime.GOOS == "windows" {
		binaryName = "dialtone.exe"
	}

	outputPath := filepath.Join("bin", binaryName)
	runShell(".", "go", "build", "-o", outputPath, ".")
	LogInfo("Build successful: %s", outputPath)
}

// RunDeploy handles deployment to remote robot
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", true, "Register as ephemeral node on Tailscale")
	showHelp := fs.Bool("help", false, "Show help for deploy command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone deploy [options]")
		fmt.Println()
		fmt.Println("Deploy the Dialtone binary to a remote robot via SSH.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --host        SSH host (user@host) [env: ROBOT_HOST]")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username [env: ROBOT_USER]")
		fmt.Println("  --pass        SSH password [env: ROBOT_PASSWORD]")
		fmt.Println("  --ephemeral   Register as ephemeral node on Tailscale (default: true)")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone deploy --host pi@192.168.1.100 --pass mypassword")
		fmt.Println("  dialtone deploy   # Uses ROBOT_HOST, ROBOT_PASSWORD from .env")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Builds ARM64 binary if not already present (bin/dialtone-arm64)")
		fmt.Println("  - Auto-provisions TS_AUTHKEY if TS_API_KEY is set")
		fmt.Println("  - Requires REMOTE_DIR_SRC, REMOTE_DIR_DEPLOY, DIALTONE_HOSTNAME, TS_AUTHKEY")
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	// If TS_API_KEY is available, provision a fresh key before deployment
	if os.Getenv("TS_API_KEY") != "" {
		LogInfo("TS_API_KEY found, auto-provisioning fresh TS_AUTHKEY for deployment...")
		provisionKey(os.Getenv("TS_API_KEY"))
	}

	if *host == "" || *pass == "" {
		LogFatal("Error: --host (user@host) and --pass are required for deployment")
	}

	validateRequiredVars([]string{"REMOTE_DIR_SRC", "REMOTE_DIR_DEPLOY", "DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	deployDialtone(*host, *port, *user, *pass, *ephemeral)
}

// RunSSH handles general SSH tools (upload, download, cmd)
func RunSSH(args []string) {
	fs := flag.NewFlagSet("ssh", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH username (overrides user@host)")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	cmd := fs.String("cmd", "", "Command to execute remotely")
	upload := fs.String("upload", "", "Local file to upload")
	dest := fs.String("dest", "", "Remote destination path")
	download := fs.String("download", "", "Remote file to download")
	localDest := fs.String("local-dest", "", "Local destination for download")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		LogFatal("Error: -host and -pass are required for SSH tools")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	if *cmd != "" {
		output, err := runCommand(client, *cmd)
		if err != nil {
			LogFatal("Command failed: %v", err)
		}
		fmt.Print(output)
	}

	if *upload != "" {
		if *dest == "" {
			*dest = filepath.Base(*upload)
		}
		if err := uploadFile(client, *upload, *dest); err != nil {
			LogFatal("Upload failed: %v", err)
		}
		LogInfo("Uploaded %s to %s", *upload, *dest)
	}

	if *download != "" {
		if *localDest == "" {
			*localDest = filepath.Base(*download)
		}
		if err := downloadFile(client, *download, *localDest); err != nil {
			LogFatal("Download failed: %v", err)
		}
		LogInfo("Downloaded %s to %s", *download, *localDest)
	}
}

func validateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			LogFatal("ERROR: Environment variable %s is not set. Please check your .env file.", v)
		}
	}
}

func buildWithPodman() {
	LogInfo("Building Dialtone for Linux ARM64 using Podman...")

	cwd, err := os.Getwd()
	if err != nil {
		LogFatal("Failed to get current directory: %v", err)
	}

	if err := os.MkdirAll("bin", 0755); err != nil {
		LogFatal("Failed to create bin directory: %v", err)
	}

	buildCmd := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/src:Z", cwd),
		"-w", "/src",
		"-e", "GOOS=linux",
		"-e", "GOARCH=arm64",
		"-e", "CGO_ENABLED=1",
		"-e", "CC=aarch64-linux-gnu-gcc",
		"golang:1.25.5",
		"bash", "-c", "apt-get update && apt-get install -y gcc-aarch64-linux-gnu && go build -buildvcs=false -o bin/dialtone-arm64 .",
	}

	LogInfo("Running: podman %v", buildCmd)
	cmd := exec.Command("podman", buildCmd...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		LogFatal("Podman build failed: %v", err)
	}

	LogInfo("Build successful: bin/dialtone-arm64")
}

func buildEverything(local bool) {
	LogInfo("Starting Full Build Process...")

	// 1. Build Web UI
	LogInfo("Building Web UI...")
	webDir := filepath.Join("src", "web")
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// 2. Sync web assets
	LogInfo("Syncing web assets to src/web_build...")
	webBuildDir := filepath.Join("src", "web_build")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		LogFatal("Failed to create web_build dir: %v", err)
	}
	copyDir(filepath.Join("src", "web", "dist"), webBuildDir)

	// 3. Build Dialtone locally (the tool itself)
	BuildSelf()

	// 4. Build for ARM64
	if local || !hasPodman() {
		buildLocally()
	} else {
		buildWithPodman()
	}

	LogInfo("Full build successful!")
}

// BuildSelf rebuilds the current binary and replaces it
func BuildSelf() {
	LogInfo("Building Dialtone CLI (Self)...")

	// Always aim for bin/dialtone.exe when building from source
	exePath := filepath.Join("bin", "dialtone.exe")
	if _, err := os.Stat("bin"); os.IsNotExist(err) {
		os.MkdirAll("bin", 0755)
	}

	oldExePath := exePath + ".old"

	// Rename old exe if it exists (allows overwriting while running on Windows)
	os.Remove(oldExePath) // Clean up any previous old file
	if _, err := os.Stat(exePath); err == nil {
		if err := os.Rename(exePath, oldExePath); err != nil {
			LogInfo("Warning: Failed to rename current exe, build might fail: %v", err)
		} else {
			LogInfo("Renamed current binary to %s", filepath.Base(oldExePath))
		}
	}

	runShell(".", "go", "build", "-o", exePath, ".")
	LogInfo("Successfully built %s", exePath)
}

func runShell(dir string, name string, args ...string) {
	LogInfo("Running: %s %v in %s", name, args, dir)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}

func copyDir(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		LogFatal("Failed to read src dir %s: %v", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				LogFatal("Failed to create dir %s: %v", dstPath, err)
			}
			copyDir(srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				LogFatal("Failed to read file %s: %v", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				LogFatal("Failed to write file %s: %v", dstPath, err)
			}
		}
	}
}

func dialSSH(host, port, user, pass string) (*ssh.Client, error) {
	username := user
	hostname := host
	if username == "" {
		if i := strings.Index(host, "@"); i != -1 {
			username = host[:i]
			hostname = host[i+1:]
		}
	}
	if username == "" {
		username = os.Getenv("USER")
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", hostname, port)
	return ssh.Dial("tcp", addr, config)
}

func runCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command error: %w\nOutput: %s", err, output)
	}
	return string(output), nil
}

func runCommandNoWait(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	err = session.Start(cmd)
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to start command: %w", err)
	}

	time.Sleep(500 * time.Millisecond)
	session.Close()
	return nil
}

func uploadFile(client *ssh.Client, localPath, remotePath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	localInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	_ = sftpClient.Chmod(remotePath, localInfo.Mode())
	return nil
}

func uploadDir(client *ssh.Client, localDir, remoteDir string) {
	LogInfo("Uploading directory %s to %s...", localDir, remoteDir)

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create SFTP client: %v\n", err)
		return
	}
	defer sftpClient.Close()

	filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		remotePath := filepath.Join(remoteDir, relPath)
		remotePath = filepath.ToSlash(remotePath)

		if info.IsDir() {
			return sftpClient.MkdirAll(remotePath)
		}

		localFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer localFile.Close()

		remoteFile, err := sftpClient.Create(remotePath)
		if err != nil {
			return err
		}
		defer remoteFile.Close()

		_, err = io.Copy(remoteFile, localFile)
		if err != nil {
			return err
		}

		_ = sftpClient.Chmod(remotePath, info.Mode())
		return nil
	})
}

func downloadFile(client *ssh.Client, remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func getRemoteHome(client *ssh.Client) (string, error) {
	output, err := runCommand(client, "echo $HOME")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func deployDialtone(host, port, user, pass string, ephemeral bool) {
	LogInfo("Starting deployment of Dialtone (Remote Build)...")

	localBinary := "bin/dialtone-arm64"
	usePrebuilt := false
	if _, err := os.Stat(localBinary); err == nil {
		usePrebuilt = true
		LogInfo("Found pre-built binary, using it for deployment.")
	}

	client, err := dialSSH(host, port, user, pass)
	if err != nil {
		LogFatal("Failed to connect: %v", err)
	}
	defer client.Close()

	if usePrebuilt {
		remoteDir := os.Getenv("REMOTE_DIR_DEPLOY")
		if remoteDir == "" {
			home, err := getRemoteHome(client)
			if err != nil {
				LogFatal("Failed to get remote home: %v", err)
			}
			remoteDir = path.Join(home, "dialtone_deploy")
		}
		LogInfo("Cleaning and creating remote directory %s...", remoteDir)
		_, _ = runCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDir, remoteDir))

		LogInfo("Uploading pre-built binary %s...", localBinary)
		remotePath := path.Join(remoteDir, "dialtone")
		if err := uploadFile(client, localBinary, remotePath); err != nil {
			LogFatal("Failed to upload binary: %v", err)
		}
	} else {
		remoteDir := os.Getenv("REMOTE_DIR_SRC")
		if remoteDir == "" {
			home, err := getRemoteHome(client)
			if err != nil {
				LogFatal("Failed to get remote home: %v", err)
			}
			remoteDir = path.Join(home, "dialtone_src")
		}
		LogInfo("Cleaning and creating remote directory %s...", remoteDir)
		_, _ = runCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s/src", remoteDir, remoteDir))

		filesToUpload := []string{"go.mod", "go.sum", "dialtone.go"}
		srcFiles, _ := filepath.Glob("src/*.go")
		for _, f := range srcFiles {
			filesToUpload = append(filesToUpload, f)
		}

		for _, file := range filesToUpload {
			LogInfo("Uploading %s...", file)
			remotePath := path.Join(remoteDir, file)
			// Ensure parent directory exists on remote
			parentDir := filepath.ToSlash(path.Dir(remotePath))
			_, _ = runCommand(client, fmt.Sprintf("mkdir -p %s", parentDir))
			if err := uploadFile(client, file, remotePath); err != nil {
				LogFatal("Failed to upload %s: %v", file, err)
			}
		}

		// Also upload web_build if it exists
		webBuildDir := filepath.Join("src", "web_build")
		if _, err := os.Stat(webBuildDir); err == nil {
			LogInfo("Uploading web assets...")
			uploadDir(client, webBuildDir, path.Join(remoteDir, "src", "web_build"))
		}

		LogInfo("Building on Raspberry Pi...")
		buildCmd := fmt.Sprintf("cd %s && /usr/local/go/bin/go build -v -o dialtone .", remoteDir)
		output, err := runCommand(client, buildCmd)
		if err != nil {
			LogFatal("Remote build failed: %v\nOutput: %s", err, output)
		}
	}

	LogInfo("Stopping remote dialtone...")
	_, _ = runCommand(client, "pkill dialtone || true")

	LogInfo("Starting service...")
	remoteBaseDir := os.Getenv("REMOTE_DIR_SRC")
	if usePrebuilt {
		remoteBaseDir = os.Getenv("REMOTE_DIR_DEPLOY")
	}
	remoteBinaryPath := path.Join(remoteBaseDir, "dialtone")

	hostnameParam := os.Getenv("DIALTONE_HOSTNAME")
	if hostnameParam == "" {
		hostnameParam = "dialtone-1"
	}

	tsAuthKey := os.Getenv("TS_AUTHKEY")
	ephemeralFlag := ""
	if ephemeral {
		ephemeralFlag = "-ephemeral"
	}

	mavlinkEndpoint := os.Getenv("MAVLINK_ENDPOINT")
	mavlinkFlag := ""
	if mavlinkEndpoint != "" {
		mavlinkFlag = fmt.Sprintf("-mavlink %s", mavlinkEndpoint)
	}

	startCmd := fmt.Sprintf("cp %s ~/dialtone && chmod +x ~/dialtone && nohup sh -c 'TS_AUTHKEY=%s ~/dialtone start -hostname %s %s %s' > ~/nats.log 2>&1 < /dev/null &", remoteBinaryPath, tsAuthKey, hostnameParam, ephemeralFlag, mavlinkFlag)

	if err := runCommandNoWait(client, startCmd); err != nil {
		LogFatal("Failed to start: %v", err)
	}

	LogInfo("Deployment complete!")
}

func runLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH username")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		LogFatal("Error: -host and -pass are required for logs")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("Failed to connect: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		LogFatal("Failed to create session: %v", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	LogInfo("Tailing logs on %s...", *host)
	_ = session.Run("tail -f ~/nats.log")
}

func RunProvision(args []string) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Tailscale API Access Token")
	optional := fs.Bool("optional", false, "Skip instead of failing if TS_API_KEY is missing")
	fs.Parse(args)

	token := *apiKey
	if token == "" {
		token = os.Getenv("TS_API_KEY")
	}

	if token == "" {
		if *optional {
			LogInfo("TS_API_KEY not found, skipping provisioning.")
			return
		}
		LogFatal("Error: --api-key flag or TS_API_KEY environment variable is required.")
	}

	provisionKey(token)
}

func RunInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	linuxWSL := fs.Bool("linux-wsl", false, "Install dependencies natively on Linux/WSL (x86_64)")
	macosARM := fs.Bool("macos-arm", false, "Install dependencies natively on macOS ARM (Apple Silicon)")
	showHelp := fs.Bool("help", false, "Show help for install command")

	fs.Usage = func() {
		fmt.Println("Usage: dialtone install [options]")
		fmt.Println()
		fmt.Println("Install development dependencies (Go, Node.js, Zig) for building Dialtone.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  --linux-wsl   Install for Linux/WSL x86_64")
		fmt.Println("  --macos-arm   Install for macOS ARM (Apple Silicon)")
		fmt.Println("  --host        SSH host for remote installation (user@host)")
		fmt.Println("  --port        SSH port (default: 22)")
		fmt.Println("  --user        SSH username")
		fmt.Println("  --pass        SSH password")
		fmt.Println("  --help        Show this help message")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  dialtone install                    # Auto-detect OS/arch and install locally")
		fmt.Println("  dialtone install --macos-arm        # Install for macOS Apple Silicon")
		fmt.Println("  dialtone install --linux-wsl        # Install for Linux/WSL x86_64")
		fmt.Println("  dialtone install --host pi@robot    # Install on remote robot via SSH")
		fmt.Println()
		fmt.Println("Notes:")
		fmt.Println("  - Dependencies are installed to ~/.dialtone_env (no sudo required)")
		fmt.Println("  - Auto-detects platform if no flags provided")
		fmt.Println("  - Skips already-installed dependencies")
		fmt.Println("  - Supported platforms: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64")
	}

	fs.Parse(args)

	if *showHelp {
		fs.Usage()
		return
	}

	// Explicit flags take priority
	if *linuxWSL {
		installLocalDepsWSL()
		return
	}

	if *macosARM {
		installLocalDepsMacOSARM()
		return
	}

	// If no host specified, auto-detect local OS/arch
	if *host == "" && *pass == "" {
		installLocalAuto()
		return
	}

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for remote install")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	LogInfo("Installing dependencies on %s...", *host)

	// Install Go
	goVersion := "1.25.5"
	goTarball := fmt.Sprintf("go%s.linux-arm64.tar.gz", goVersion)
	installGoCmd := fmt.Sprintf(`
		if ! command -v go &> /dev/null; then
			echo "Installing Go %s..."
			wget https://go.dev/dl/%s
			echo "%s" | sudo -S rm -rf /usr/local/go
			echo "%s" | sudo -S tar -C /usr/local -xzf %s
			rm %s
			echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
			echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
		else
			echo "Go is already installed."
		fi
	`, goVersion, goTarball, *pass, *pass, goTarball, goTarball)

	output, err := runCommand(client, installGoCmd)
	if err != nil {
		LogFatal("Failed to install Go: %v\nOutput: %s", err, output)
	}
	LogInfo(output)

	// Install Node.js
	installNodeCmd := fmt.Sprintf(`
		if ! command -v node &> /dev/null; then
			echo "Installing Node.js..."
			curl -fsSL https://deb.nodesource.com/setup_20.x | echo "%s" | sudo -S -E bash -
			echo "%s" | sudo -S apt-get install -y nodejs
		else
			echo "Node.js is already installed."
		fi
	`, *pass, *pass)
	output, err = runCommand(client, installNodeCmd)
	if err != nil {
		LogFatal("Failed to install Node.js: %v\nOutput: %s", err, output)
	}
	LogInfo(output)
}

func runSimpleShell(command string) {
	LogInfo("Running: %s", command)
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}

func installLocalDepsWSL() {
	LogInfo("Installing local dependencies for Linux/WSL (User-Local, No Sudo)...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go 1.25.5
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s...", goVersion)
		goTarball := fmt.Sprintf("go%s.linux-amd64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js...")
		nodeVersion := "22.13.0" // Current LTS
		nodeTarball := fmt.Sprintf("node-v%s-linux-x64.tar.xz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 2.5 Install Zig (as portable C compiler)
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 2.5: Installing Zig (portable C compiler)...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-linux-x86_64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("wget -q -O %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	// 3. Install V4L2 headers (extract from deb)
	includeDir := filepath.Join(depsDir, "usr", "include")
	if _, err := os.Stat(filepath.Join(includeDir, "linux", "videodev2.h")); err != nil {
		LogInfo("Step 3: Extracting V4L2 headers...")
		// Try apt-get download first, then fall back to direct mirrors
		err := os.Chdir(depsDir)
		if err == nil {
			LogInfo("Attempting apt-get download...")
			cmd := exec.Command("apt-get", "download", "libv4l-dev", "linux-libc-dev")
			if cmd.Run() != nil {
				LogInfo("apt-get download failed, falling back to Ubuntu mirrors...")
				// Noble Noble (24.04) mirrors
				runSimpleShell("wget -q http://archive.ubuntu.com/ubuntu/pool/main/v/v4l-utils/libv4l-dev_1.26.1-4build3_amd64.deb")
				runSimpleShell("wget -q http://archive.ubuntu.com/ubuntu/pool/main/l/linux/linux-libc-dev_6.8.0-31.31_amd64.deb")
			}
			runSimpleShell("dpkg -x libv4l-dev*.deb .")
			runSimpleShell("dpkg -x linux-libc-dev*.deb .")
			runSimpleShell("rm *.deb")
			os.Chdir(homeDir)
		}
	} else {
		LogInfo("V4L2 headers already present in %s", includeDir)
	}

	LogInfo("Local dependencies installation complete in %s", depsDir)
	LogInfo("To use these in your shell, add them to your PATH:")
	LogInfo("export PATH=$PATH:%s/go/bin:%s/node/bin", depsDir, depsDir)
}

func installLocalAuto() {
	LogInfo("Auto-detecting system: %s/%s", runtime.GOOS, runtime.GOARCH)

	switch {
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		installLocalDepsMacOSARM()
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		LogInfo("macOS x86_64 detected. Installing with Rosetta-compatible deps...")
		installLocalDepsMacOSAMD64()
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		installLocalDepsWSL()
	case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
		LogInfo("Linux ARM64 detected (likely Raspberry Pi).")
		installLocalDepsLinuxARM64()
	default:
		LogFatal("Unsupported platform: %s/%s. Use --linux-wsl or --macos-arm explicitly.", runtime.GOOS, runtime.GOARCH)
	}
}

func installLocalDepsMacOSAMD64() {
	LogInfo("Installing local dependencies for macOS x86_64 (Intel)...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go for darwin-amd64
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s for macOS x86_64...", goVersion)
		goTarball := fmt.Sprintf("go%s.darwin-amd64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js for darwin-x64
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js for macOS x86_64...")
		nodeVersion := "22.13.0"
		nodeTarball := fmt.Sprintf("node-v%s-darwin-x64.tar.gz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 3. Install Zig for darwin-x86_64
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 3: Installing Zig for macOS x86_64...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-macos-x86_64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	printInstallComplete(depsDir)
}

func installLocalDepsLinuxARM64() {
	LogInfo("Installing local dependencies for Linux ARM64...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go for linux-arm64
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s for Linux ARM64...", goVersion)
		goTarball := fmt.Sprintf("go%s.linux-arm64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js for linux-arm64
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js for Linux ARM64...")
		nodeVersion := "22.13.0"
		nodeTarball := fmt.Sprintf("node-v%s-linux-arm64.tar.xz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 3. Install Zig for linux-aarch64
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 3: Installing Zig for Linux ARM64...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-linux-aarch64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("wget -O %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	printInstallComplete(depsDir)
}

func printInstallComplete(depsDir string) {
	LogInfo("")
	LogInfo("========================================")
	LogInfo("Installation complete in %s", depsDir)
	LogInfo("========================================")
	LogInfo("")
	LogInfo("Add to your shell profile (~/.zshrc or ~/.bashrc):")
	LogInfo("  export PATH=\"%s/go/bin:%s/node/bin:%s/zig:$PATH\"", depsDir, depsDir, depsDir)
	LogInfo("")
}

func installLocalDepsMacOSARM() {
	LogInfo("Installing local dependencies for macOS ARM (Apple Silicon)...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		LogFatal("Failed to get home directory: %v", err)
	}
	depsDir := filepath.Join(homeDir, ".dialtone_env")
	os.MkdirAll(depsDir, 0755)

	// 1. Install Go 1.25.5 for darwin-arm64
	goVersion := "1.25.5"
	goDir := filepath.Join(depsDir, "go")
	if _, err := os.Stat(filepath.Join(goDir, "bin", "go")); err != nil {
		LogInfo("Step 1: Installing Go %s for macOS ARM64...", goVersion)
		goTarball := fmt.Sprintf("go%s.darwin-arm64.tar.gz", goVersion)
		downloadUrl := fmt.Sprintf("https://go.dev/dl/%s", goTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, goTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("tar -C %s -xzf %s/%s", depsDir, depsDir, goTarball))
		os.Remove(filepath.Join(depsDir, goTarball))
	} else {
		LogInfo("Go is already installed in %s", goDir)
	}

	// 2. Install Node.js for darwin-arm64
	nodeDir := filepath.Join(depsDir, "node")
	if _, err := os.Stat(filepath.Join(nodeDir, "bin", "node")); err != nil {
		LogInfo("Step 2: Installing Node.js for macOS ARM64...")
		nodeVersion := "22.13.0" // Current LTS
		nodeTarball := fmt.Sprintf("node-v%s-darwin-arm64.tar.gz", nodeVersion)
		downloadUrl := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", nodeVersion, nodeTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, nodeTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xzf %s/%s", nodeDir, nodeDir, depsDir, nodeTarball))
		os.Remove(filepath.Join(depsDir, nodeTarball))
	} else {
		LogInfo("Node.js is already installed in %s", nodeDir)
	}

	// 3. Install Zig for darwin-arm64 (portable C compiler for CGO cross-compilation)
	zigDir := filepath.Join(depsDir, "zig")
	if _, err := os.Stat(filepath.Join(zigDir, "zig")); err != nil {
		LogInfo("Step 3: Installing Zig (portable C compiler) for macOS ARM64...")
		zigVersion := "0.13.0"
		zigTarball := fmt.Sprintf("zig-macos-aarch64-%s.tar.xz", zigVersion)
		downloadUrl := fmt.Sprintf("https://ziglang.org/download/%s/%s", zigVersion, zigTarball)
		runSimpleShell(fmt.Sprintf("curl -L -o %s/%s %s", depsDir, zigTarball, downloadUrl))
		runSimpleShell(fmt.Sprintf("mkdir -p %s && tar -C %s --strip-components=1 -xJf %s/%s", zigDir, zigDir, depsDir, zigTarball))
		os.Remove(filepath.Join(depsDir, zigTarball))
	} else {
		LogInfo("Zig is already installed in %s", zigDir)
	}

	printInstallComplete(depsDir)
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runSudoShell(command string) {
	LogInfo("Running with sudo: %s", command)
	cmd := exec.Command("sudo", "bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}

func RunSyncCode(args []string) {
	fs := flag.NewFlagSet("sync-code", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for sync-code")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteDir == "" {
		remoteDir = "/home/tim/dialtone_src"
	}

	LogInfo("Syncing code to %s on %s...", remoteDir, *host)

	// Clean remote src dir partially? Or just overwrite.
	// We definitely need to ensure directories exist.
	_, _ = runCommand(client, fmt.Sprintf("mkdir -p %s/src/web", remoteDir))

	// Sync root files
	filesToUpload := []string{"go.mod", "go.sum", "dialtone.go", "build.sh", "build.ps1", "README.md"}
	for _, file := range filesToUpload {
		if _, err := os.Stat(file); err == nil {
			LogInfo("Uploading %s...", file)
			if err := uploadFile(client, file, path.Join(remoteDir, file)); err != nil {
				LogFatal("Failed to upload %s: %v", file, err)
			}
		}
	}

	// Sync src/*.go
	srcFiles, _ := filepath.Glob("src/*.go")
	for _, f := range srcFiles {
		LogInfo("Uploading %s...", f)
		if err := uploadFile(client, f, path.Join(remoteDir, filepath.ToSlash(f))); err != nil {
			LogFatal("Failed to upload %s: %v", f, err)
		}
	}

	// Sync src/web (excluding node_modules and dist)
	LogInfo("Uploading src/web source...")
	uploadDirFiltered(client, filepath.Join("src", "web"), path.Join(remoteDir, "src", "web"), []string{"node_modules", "dist", ".git"})

	// Sync test directory
	LogInfo("Uploading test directory...")
	uploadDirFiltered(client, "test", path.Join(remoteDir, "test"), []string{".git"})

	// Sync mavlink directory
	LogInfo("Uploading mavlink directory...")
	uploadDirFiltered(client, "mavlink", path.Join(remoteDir, "mavlink"), []string{".git", "__pycache__"})

	LogInfo("Code sync complete.")
}

func RunRemoteBuild(args []string) {
	fs := flag.NewFlagSet("remote-build", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for remote-build")
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteDir == "" {
		home, err := getRemoteHome(client)
		if err != nil {
			LogFatal("Failed to get remote home: %v", err)
		}
		remoteDir = path.Join(home, "dialtone_src")
	}

	LogInfo("Building on remote %s...", *host)

	// Build Web
	webCmd := fmt.Sprintf(`
		export PATH=$PATH:/usr/local/go/bin
		cd %s/src/web
		echo "Installing npm dependencies..."
		npm install
		echo "Building web assets..."
		npm run build
		cd ..
		rm -rf web_build
		mkdir -p web_build
		cp -r web/dist/* web_build/
	`, remoteDir)

	output, err := runCommand(client, webCmd)
	if err != nil {
		LogFatal("Remote web build failed: %v\nOutput: %s", err, output)
	}
	LogInfo(output)

	// Build Go
	goCmd := fmt.Sprintf(`
		export PATH=$PATH:/usr/local/go/bin
		cd %s
		echo "Building Go binary..."
		go build -v -o dialtone .
	`, remoteDir)

	output, err = runCommand(client, goCmd)
	if err != nil {
		LogFatal("Remote Go build failed: %v\nOutput: %s", err, output)
	}
	LogInfo(output)
	LogInfo("Remote build successful.")
}

func uploadDirFiltered(client *ssh.Client, localDir, remoteDir string, ignore []string) {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		LogInfo("Failed to create SFTP client: %v", err)
		return
	}
	defer sftpClient.Close()

	// Ensure remote dir exists
	_ = sftpClient.MkdirAll(remoteDir)

	entries, err := os.ReadDir(localDir)
	if err != nil {
		LogFatal("Failed to read local dir %s: %v", localDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		shouldIgnore := false
		for _, ig := range ignore {
			if name == ig {
				shouldIgnore = true
				break
			}
		}
		if shouldIgnore {
			continue
		}

		srcPath := filepath.Join(localDir, name)
		dstPath := path.Join(remoteDir, name)

		if entry.IsDir() {
			uploadDirFiltered(client, srcPath, dstPath, ignore)
		} else {
			// Upload file
			localFile, err := os.Open(srcPath)
			if err != nil {
				LogFatal("Failed to open %s: %v", srcPath, err)
			}
			defer localFile.Close()

			remoteFile, err := sftpClient.Create(dstPath)
			if err != nil {
				LogFatal("Failed to create remote file %s: %v", dstPath, err)
			}
			defer remoteFile.Close()

			if _, err := io.Copy(remoteFile, localFile); err != nil {
				LogFatal("Failed to copy %s: %v", srcPath, err)
			}
		}
	}
}

func provisionKey(token string) {
	LogInfo("Generating new Tailscale Auth Key...")

	url := "https://api.tailscale.com/api/v2/tailnet/-/keys"
	payload := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"devices": map[string]interface{}{
				"create": map[string]interface{}{
					"reusable":      false,
					"ephemeral":     true,
					"preauthorized": true,
				},
			},
		},
		"expirySeconds": 86400,
		"description":   "Dialtone Auto-Provisioned Key",
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		LogFatal("Failed to create request: %v", err)
	}

	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		LogFatal("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		LogFatal("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Key string `json:"key"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	LogInfo("Successfully generated key: %s...", result.Key[:10])
	updateEnv("TS_AUTHKEY", result.Key)
	LogInfo("Updated .env with new TS_AUTHKEY.")
}

func updateEnv(key, value string) {
	// Update current process environment
	os.Setenv(key, value)

	envFile := ".env"
	content, _ := os.ReadFile(envFile)
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	_ = os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}
