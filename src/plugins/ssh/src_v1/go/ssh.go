package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func DialSSH(host, port, user, pass string) (*ssh.Client, error) {
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

	// Try to use SSH key if available
	keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
	if _, err := os.Stat(keyPath); err == nil {
		key, err := os.ReadFile(keyPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				config.Auth = append([]ssh.AuthMethod{ssh.PublicKeys(signer)}, config.Auth...)
			}
		}
	}

	addr := fmt.Sprintf("%s:%s", hostname, port)
	return ssh.Dial("tcp", addr, config)
}

func RunSSHCommand(client *ssh.Client, cmd string) (string, error) {
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

func UploadFile(client *ssh.Client, localPath, remotePath string) error {
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

func GetRemoteHome(client *ssh.Client) (string, error) {
	output, err := RunSSHCommand(client, "echo $HOME")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func ForwardRemoteToLocal(client *ssh.Client, remoteAddr, localAddr string) error {
	localListener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return err
	}

	go func() {
		defer localListener.Close()
		for {
			localConn, err := localListener.Accept()
			if err != nil {
				return
			}

			go func() {
				defer localConn.Close()
				remoteConn, err := client.Dial("tcp", remoteAddr)
				if err != nil {
					return
				}
				defer remoteConn.Close()

				done := make(chan struct{}, 2)
				go func() {
					io.Copy(remoteConn, localConn)
					done <- struct{}{}
				}()
				go func() {
					io.Copy(localConn, remoteConn)
					done <- struct{}{}
				}()
				<-done
			}()
		}
	}()

	return nil
}
