package main

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

	LoadConfig()

	if *full {
		buildEverything()
	} else {
		buildWithPodman()
	}
}

// RunDeploy handles deployment to remote robot
func RunDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	host := fs.String("host", "", "SSH host (user@host)")
	port := fs.String("port", "22", "SSH port")
	pass := fs.String("pass", "", "SSH password")
	ephemeral := fs.Bool("ephemeral", true, "Register as ephemeral node on Tailscale")
	fs.Parse(args)

	LoadConfig()

	if *host == "" || *pass == "" {
		fmt.Println("Error: -host (user@host) and -pass are required for deployment")
		os.Exit(1)
	}

	validateRequiredVars([]string{"REMOTE_DIR_SRC", "REMOTE_DIR_DEPLOY", "DIALTONE_HOSTNAME", "TS_AUTHKEY"})

	deployDialtone(*host, *port, *pass, *ephemeral)
}

// RunSSH handles general SSH tools (upload, download, cmd)
func RunSSH(args []string) {
	fs := flag.NewFlagSet("ssh", flag.ExitOnError)
	host := fs.String("host", "", "SSH host (user@host)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", "", "SSH username (overrides user@host)")
	pass := fs.String("pass", "", "SSH password")
	cmd := fs.String("cmd", "", "Command to execute remotely")
	upload := fs.String("upload", "", "Local file to upload")
	dest := fs.String("dest", "", "Remote destination path")
	download := fs.String("download", "", "Remote file to download")
	localDest := fs.String("local-dest", "", "Local destination for download")
	fs.Parse(args)

	LoadConfig()

	if *host == "" || *pass == "" {
		fmt.Println("Error: -host and -pass are required for SSH tools")
		os.Exit(1)
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SSH connection failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if *cmd != "" {
		output, err := runCommand(client, *cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(output)
	}

	if *upload != "" {
		if *dest == "" {
			*dest = filepath.Base(*upload)
		}
		if err := uploadFile(client, *upload, *dest); err != nil {
			fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Uploaded %s to %s\n", *upload, *dest)
	}

	if *download != "" {
		if *localDest == "" {
			*localDest = filepath.Base(*download)
		}
		if err := downloadFile(client, *download, *localDest); err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Downloaded %s to %s\n", *download, *localDest)
	}
}

func validateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			fmt.Printf("ERROR: Environment variable %s is not set. Please check your .env file.\n", v)
			os.Exit(1)
		}
	}
}

func buildWithPodman() {
	fmt.Println("Building Dialtone for Linux ARM64 using Podman...")

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("bin", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bin directory: %v\n", err)
		os.Exit(1)
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
		"bash", "-c", "apt-get update && apt-get install -y gcc-aarch64-linux-gnu && go build -buildvcs=false -o bin/dialtone-arm64 ./src",
	}

	fmt.Printf("Running: podman %v\n", buildCmd)
	cmd := exec.Command("podman", buildCmd...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Podman build failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Build successful: bin/dialtone-arm64")
}

func buildEverything() {
	fmt.Println("Starting Full Build Process...")

	// 1. Build Web UI
	fmt.Println("Building Web UI...")
	webDir := filepath.Join("src", "web")
	runShell(webDir, "npm", "install")
	runShell(webDir, "npm", "run", "build")

	// 2. Sync web assets
	fmt.Println("Syncing web assets to src/web_build...")
	webBuildDir := filepath.Join("src", "web_build")
	os.RemoveAll(webBuildDir)
	if err := os.MkdirAll(webBuildDir, 0755); err != nil {
		fmt.Printf("Failed to create web_build dir: %v\n", err)
		os.Exit(1)
	}
	copyDir(filepath.Join("src", "web", "dist"), webBuildDir)

	// 3. Build Dialtone locally (the tool itself)
	BuildSelf()

	// 4. Build for ARM64 using Podman
	buildWithPodman()

	fmt.Println("Full build successful!")
}

// BuildSelf rebuilds the current binary and replaces it
func BuildSelf() {
	fmt.Println("Building Dialtone CLI (Self)...")

	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("Warning: Could not determine executable path, defaulting to bin/dialtone.exe: %v\n", err)
		exePath = filepath.Join("bin", "dialtone.exe")
	}

	oldExePath := exePath + ".old"

	// Rename old exe if it exists (allows overwriting while running on Windows)
	os.Remove(oldExePath) // Clean up any previous old file
	if _, err := os.Stat(exePath); err == nil {
		if err := os.Rename(exePath, oldExePath); err != nil {
			fmt.Printf("Warning: Failed to rename current exe, build might fail: %v\n", err)
		} else {
			fmt.Printf("Renamed current binary to %s\n", filepath.Base(oldExePath))
		}
	}

	runShell(".", "go", "build", "-o", exePath, "./src")
	fmt.Printf("Successfully built %s\n", exePath)
}

func runShell(dir string, name string, args ...string) {
	fmt.Printf("Running: %s %v in %s\n", name, args, dir)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Command failed: %v\n", err)
		os.Exit(1)
	}
}

func copyDir(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		fmt.Printf("Failed to read src dir %s: %v\n", src, err)
		os.Exit(1)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				fmt.Printf("Failed to create dir %s: %v\n", dstPath, err)
				os.Exit(1)
			}
			copyDir(srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				fmt.Printf("Failed to read file %s: %v\n", srcPath, err)
				os.Exit(1)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				fmt.Printf("Failed to write file %s: %v\n", dstPath, err)
				os.Exit(1)
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

func deployDialtone(host, port, pass string, ephemeral bool) {
	fmt.Println("Starting deployment of Dialtone (Remote Build)...")

	localBinary := "bin/dialtone-arm64"
	usePrebuilt := false
	if _, err := os.Stat(localBinary); err == nil {
		usePrebuilt = true
		fmt.Println("Found pre-built binary, using it for deployment.")
	}

	client, err := dialSSH(host, port, "", pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if usePrebuilt {
		remoteDir := os.Getenv("REMOTE_DIR_DEPLOY")
		if remoteDir == "" {
			remoteDir = "/home/tim/dialtone_deploy"
		}
		fmt.Printf("Cleaning and creating remote directory %s...\n", remoteDir)
		_, _ = runCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDir, remoteDir))

		fmt.Printf("Uploading pre-built binary %s...\n", localBinary)
		remotePath := path.Join(remoteDir, "dialtone")
		if err := uploadFile(client, localBinary, remotePath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to upload binary: %v\n", err)
			os.Exit(1)
		}
	} else {
		remoteDir := os.Getenv("REMOTE_DIR_SRC")
		if remoteDir == "" {
			remoteDir = "/home/tim/dialtone_src"
		}
		fmt.Printf("Cleaning and creating remote directory %s...\n", remoteDir)
		_, _ = runCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s/src", remoteDir, remoteDir))

		filesToUpload := []string{"go.mod", "go.sum", "src/dialtone.go", "src/camera_linux.go", "src/camera_stub.go"}
		for _, file := range filesToUpload {
			fmt.Printf("Uploading %s...\n", file)
			remotePath := path.Join(remoteDir, file)
			_, _ = runCommand(client, fmt.Sprintf("mkdir -p %s", path.Dir(remotePath)))
			if err := uploadFile(client, file, remotePath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to upload %s: %v\n", file, err)
				os.Exit(1)
			}
		}

		fmt.Println("Building on Raspberry Pi...")
		buildCmd := fmt.Sprintf("cd %s && /usr/local/go/bin/go build -o dialtone ./src", remoteDir)
		output, err := runCommand(client, buildCmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Remote build failed: %v\nOutput: %s\n", err, output)
			os.Exit(1)
		}
	}

	fmt.Println("Stopping remote dialtone...")
	_, _ = runCommand(client, "pkill dialtone || true")

	fmt.Println("Starting service...")
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
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nDeployment complete!")
}

func runLogs(args []string) {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	host := fs.String("host", "", "SSH host (user@host)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", "", "SSH username")
	pass := fs.String("pass", "", "SSH password")
	fs.Parse(args)

	LoadConfig()

	if *host == "" || *pass == "" {
		fmt.Println("Error: -host and -pass are required for logs")
		os.Exit(1)
	}

	client, err := dialSSH(*host, *port, *user, *pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	fmt.Printf("Tailing logs on %s...\n", *host)
	_ = session.Run("tail -f ~/nats.log")
}

func RunProvision(args []string) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Tailscale API Access Token")
	fs.Parse(args)

	LoadConfig()

	token := *apiKey
	if token == "" {
		token = os.Getenv("TS_API_KEY")
	}

	if token == "" {
		fmt.Println("Error: --api-key flag or TS_API_KEY environment variable is required.")
		os.Exit(1)
	}

	fmt.Println("Generating new Tailscale Auth Key...")

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
		fmt.Printf("Failed to create request: %v\n", err)
		os.Exit(1)
	}

	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("API request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("API error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result struct {
		Key string `json:"key"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("Successfully generated key: %s...\n", result.Key[:10])
	updateEnv("TS_AUTHKEY", result.Key)
	fmt.Println("Updated .env with new TS_AUTHKEY.")
}

func updateEnv(key, value string) {
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
