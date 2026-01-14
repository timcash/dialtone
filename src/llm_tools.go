package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"os/exec"

	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func RunTools(args []string) {
	fs := flag.NewFlagSet("tools", flag.ExitOnError)
	host := fs.String("host", "", "SSH host (user@host or just host)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", "", "SSH username (overrides user@host)")
	pass := fs.String("pass", "", "SSH password")
	cmd := fs.String("cmd", "", "Command to execute remotely")
	upload := fs.String("upload", "", "Local file to upload")
	dest := fs.String("dest", "", "Remote destination path")
	download := fs.String("download", "", "Remote file to download")
	localDest := fs.String("local-dest", "", "Local destination for download")
	deploy := fs.Bool("deploy", false, "Deploy dialtone to Raspberry Pi (cross-compiles and restarts)")
	podmanBuild := fs.Bool("podman-build", false, "Build dialtone locally using Podman for Linux ARM64")
	fs.Parse(args)

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// Non-fatal, just log if verbose
		// fmt.Printf("No .env file found: %v\n", err)
	}

	// Validate required environment variables for deployment
	requiredVars := []string{"REMOTE_DIR_SRC", "REMOTE_DIR_DEPLOY", "DIALTONE_HOSTNAME", "TS_AUTHKEY"}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			fmt.Printf("ERROR: Environment variable %s is not set. Please check your .env file.\n", v)
			os.Exit(1)
		}
	}

	if *podmanBuild {
		buildWithPodman()
		if !*deploy {
			return
		}
	}

	if *deploy {
		if *host == "" {
			fmt.Println("Error: -host (user@host) is required for deployment")
			os.Exit(1)
		}
		if *pass == "" {
			fmt.Println("Error: -pass is required for deployment")
			os.Exit(1)
		}
		deployDialtone(*host, *port, *pass)
		return
	}

	if *host == "" {
		fmt.Println("Usage:")
		fmt.Println("  Execute command:  ssh_tools -host user@host -pass password -cmd 'ls -la'")
		fmt.Println("  Upload file:      ssh_tools -host user@host -pass password -upload local.txt -dest /remote/path/")
		fmt.Println("  Download file:    ssh_tools -host user@host -pass password -download /remote/file -local-dest local.txt")
		os.Exit(1)
	}

	// Parse user@host format
	username := *user
	hostname := *host
	if username == "" {
		for i, c := range *host {
			if c == '@' {
				username = (*host)[:i]
				hostname = (*host)[i+1:]
				break
			}
		}
	}
	if username == "" {
		username = os.Getenv("USER")
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(*pass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect
	addr := fmt.Sprintf("%s:%s", hostname, *port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Execute command
	if *cmd != "" {
		output, err := runCommand(client, *cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Command failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(output)
	}

	// Upload file
	if *upload != "" {
		if *dest == "" {
			*dest = filepath.Base(*upload)
		}
		err := uploadFile(client, *upload, *dest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Uploaded %s to %s\n", *upload, *dest)
	}

	// Download file
	if *download != "" {
		if *localDest == "" {
			*localDest = filepath.Base(*download)
		}
		err := downloadFile(client, *download, *localDest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Downloaded %s to %s\n", *download, *localDest)
	}
}

func buildWithPodman() {
	fmt.Println("Building Dialtone for Linux ARM64 using Podman...")

	// Use absolute path for mounting
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	// Create bin directory if it doesn't exist
	if err := os.MkdirAll("bin", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create bin directory: %v\n", err)
		os.Exit(1)
	}

	// Podman command to cross-compile
	// We mount the current directory and build for linux/arm64
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
	// We don't defer session.Close() here because we want it to stay alive
	// long enough for the command to start and background itself.
	// Actually, session.Start() returns immediately.

	err = session.Start(cmd)
	if err != nil {
		session.Close()
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Small sleep to let the command start
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

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get local file info for permissions
	localInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy content
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Set permissions
	err = sftpClient.Chmod(remotePath, localInfo.Mode())
	if err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to set permissions: %v\n", err)
	}

	return nil
}

func uploadDir(client *ssh.Client, localDir, remoteDir string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	return filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate remote path
		relPath, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return err
		}
		remotePath := path.Join(remoteDir, filepath.ToSlash(relPath))

		if info.IsDir() {
			return sftpClient.MkdirAll(remotePath)
		}

		// Upload file
		return uploadFile(client, localPath, remotePath)
	})
}

func downloadFile(client *ssh.Client, remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy content
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func deployDialtone(host, port, pass string) {
	fmt.Println("Starting deployment of Dialtone (Remote Build)...")

	// Parse user@host
	username := ""
	hostname := host
	for i, c := range host {
		if c == '@' {
			username = host[:i]
			hostname = host[i+1:]
			break
		}
	}

	// Check if we have a pre-built local binary
	localBinary := "bin/dialtone-arm64"
	usePrebuilt := false
	if _, err := os.Stat(localBinary); err == nil {
		usePrebuilt = true
		fmt.Println("Found pre-built binary, using it for deployment.")
	}

	// Connect SSH
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", hostname, port), config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	if usePrebuilt {
		// 1. Clean remote directory
		remoteDir := os.Getenv("REMOTE_DIR_DEPLOY")
		if remoteDir == "" {
			remoteDir = "/home/tim/dialtone_deploy"
		}
		fmt.Printf("Cleaning and creating remote directory %s...\n", remoteDir)
		runCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s", remoteDir, remoteDir))

		// 2. Upload only the binary
		fmt.Printf("Uploading pre-built binary %s...\n", localBinary)
		remotePath := path.Join(remoteDir, "dialtone")
		err = uploadFile(client, localBinary, remotePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to upload binary: %v\n", err)
			os.Exit(1)
		}

		// No external web assets needed, they are embedded
	} else {
		// 1. Create remote directory (clean slate)
		remoteDir := os.Getenv("REMOTE_DIR_SRC")
		if remoteDir == "" {
			remoteDir = "/home/tim/dialtone_src"
		}
		fmt.Printf("Cleaning and creating remote directory %s...\n", remoteDir)
		runCommand(client, fmt.Sprintf("rm -rf %s && mkdir -p %s/src", remoteDir, remoteDir))

		// 2. Upload source files
		filesToUpload := []string{
			"go.mod",
			"go.sum",
			"src/dialtone.go",
			"src/camera_linux.go",
			"src/camera_stub.go",
		}

		for _, file := range filesToUpload {
			fmt.Printf("Uploading %s...\n", file)
			remotePath := path.Join(remoteDir, file)
			// Ensure remote subdirectory exists
			remoteSubDir := path.Dir(remotePath)
			runCommand(client, fmt.Sprintf("mkdir -p %s", remoteSubDir))

			err = uploadFile(client, file, remotePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to upload %s: %v\n", file, err)
				os.Exit(1)
			}
		}

		// 3. Build remotely
		fmt.Println("Building on Raspberry Pi...")
		buildCmd := fmt.Sprintf("cd %s && /usr/local/go/bin/go build -o dialtone ./src", remoteDir)
		output, err := runCommand(client, buildCmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Remote build failed: %v\nOutput: %s\n", err, output)
			fmt.Println("\nTIP: Make sure 'go' is installed on the Raspberry Pi.")
			os.Exit(1)
		}
	}

	// 4. Stop existing process
	fmt.Println("Stopping remote dialtone...")
	runCommand(client, "pkill dialtone || true")

	// 5. Move binary to home and start
	fmt.Println("Starting service...")
	// Use < /dev/null to ensure the session doesn't wait for input
	remoteBaseDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteBaseDir == "" {
		remoteBaseDir = "/home/tim/dialtone_src"
	}
	if usePrebuilt {
		remoteBaseDir = os.Getenv("REMOTE_DIR_DEPLOY")
		if remoteBaseDir == "" {
			remoteBaseDir = "/home/tim/dialtone_deploy"
		}
	}
	remoteBinaryPath := path.Join(remoteBaseDir, "dialtone")

	hostnameParam := os.Getenv("DIALTONE_HOSTNAME")
	if hostnameParam == "" {
		hostnameParam = "drone-nats"
	}

	tsAuthKey := os.Getenv("TS_AUTHKEY")

	startCmd := fmt.Sprintf("cp %s ~/dialtone && chmod +x ~/dialtone && nohup TS_AUTHKEY=%s ~/dialtone -hostname %s > ~/nats.log 2>&1 < /dev/null &", remoteBinaryPath, tsAuthKey, hostnameParam)
	err = runCommandNoWait(client, startCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nDeployment complete!")
}
