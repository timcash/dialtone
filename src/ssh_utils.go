package dialtone

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

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
		output, err := runSSHCommand(client, *cmd)
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

func runSSHCommand(client *ssh.Client, cmd string) (string, error) {
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

func runSSHCommandNoWait(client *ssh.Client, cmd string) error {
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
	output, err := runSSHCommand(client, "echo $HOME")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
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
		dstPath := filepath.Join(remoteDir, name)
		dstPath = filepath.ToSlash(dstPath)

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
