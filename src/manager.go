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
		// Non-fatal
	}
}

// RunBuild handles building for different platforms
func RunBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	full := fs.Bool("full", false, "Build Web UI, local CLI, and ARM64 binary")
	fs.Parse(args)

	if *full {
		buildEverything()
	} else {
		buildWithPodman()
	}
}

// RunDeploy handles deployment to remote robot
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", true, "Register as ephemeral node on Tailscale")
	fs.Parse(args)

	// If TS_API_KEY is available, provision a fresh key before deployment
	if os.Getenv("TS_API_KEY") != "" {
		LogInfo("TS_API_KEY found, auto-provisioning fresh TS_AUTHKEY for deployment...")
		provisionKey(os.Getenv("TS_API_KEY"))
	}

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for deployment")
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

func buildEverything() {
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

	// 4. Build for ARM64 using Podman
	buildWithPodman()

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
			remoteDir = "/home/tim/dialtone_deploy"
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
			remoteDir = "/home/tim/dialtone_src"
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

	startCmd := fmt.Sprintf("cp %s ~/dialtone && chmod +x ~/dialtone && nohup sh -c 'TS_AUTHKEY=%s ~/dialtone start -hostname %s %s' > ~/nats.log 2>&1 < /dev/null &", remoteBinaryPath, tsAuthKey, hostnameParam, ephemeralFlag)

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
