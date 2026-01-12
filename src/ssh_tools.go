//go:build ignore

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
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
	flag.Parse()

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
