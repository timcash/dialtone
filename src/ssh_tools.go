//go:build ignore

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func main() {
	host := flag.String("host", "", "SSH host (user@host or just host)")
	port := flag.String("port", "22", "SSH port")
	user := flag.String("user", "", "SSH username (overrides user@host)")
	pass := flag.String("pass", "", "SSH password")
	cmd := flag.String("cmd", "", "Command to execute remotely")
	upload := flag.String("upload", "", "Local file to upload")
	dest := flag.String("dest", "", "Remote destination path")
	download := flag.String("download", "", "Remote file to download")
	localDest := flag.String("local-dest", "", "Local destination for download")
	deploy := flag.Bool("deploy", false, "Deploy dialtone to Raspberry Pi (cross-compiles and restarts)")
	flag.Parse()

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
	fmt.Println("Starting deployment of Dialtone...")

	// 1. Cross-compile locally
	fmt.Println("Building for linux/arm64...")
	os.MkdirAll("bin", 0755)
	outputPath := filepath.Join("bin", "dialtone_linux_arm64")
	cmdBuild := exec.Command("go", "build", "-o", outputPath, "./src/dialtone.go")
	cmdBuild.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
	if output, err := cmdBuild.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\nOutput: %s\n", err, output)
		os.Exit(1)
	}
	defer os.Remove(outputPath)

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

	// 2. Kill existing process
	fmt.Println("Stopping remote dialtone...")
	runCommand(client, "pkill dialtone || true")

	// 3. Upload binary
	fmt.Println("Uploading new binary...")
	err = uploadFile(client, outputPath, "/home/tim/dialtone")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		os.Exit(1)
	}

	// 4. Start service
	fmt.Println("Restarting service...")
	// We use nohup and redirect to nats.log to match the existing setup
	startCmd := "chmod +x ~/dialtone && nohup ~/dialtone -hostname drone-nats > ~/nats.log 2>&1 &"
	_, err = runCommand(client, startCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nDeployment complete!")
}
