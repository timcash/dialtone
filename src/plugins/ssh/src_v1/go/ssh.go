package ssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func DialSSH(host, port, user, pass string) (*ssh.Client, error) {
	return DialSSHWithTimeout(host, port, user, pass, 10*time.Second)
}

func DialSSHWithTimeout(host, port, user, pass string, timeout time.Duration) (*ssh.Client, error) {
	return DialSSHWithAuth(host, port, user, pass, "", timeout)
}

func DialSSHWithPrivateKeyPath(host, port, user, keyPath string, timeout time.Duration) (*ssh.Client, error) {
	return DialSSHWithAuth(host, port, user, "", keyPath, timeout)
}

func DialSSHWithAuth(host, port, user, pass, keyPath string, timeout time.Duration) (*ssh.Client, error) {
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
		User:            username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	if strings.TrimSpace(keyPath) != "" {
		key, err := os.ReadFile(strings.TrimSpace(keyPath))
		if err != nil {
			return nil, fmt.Errorf("read private key %s: %w", strings.TrimSpace(keyPath), err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse private key %s: %w", strings.TrimSpace(keyPath), err)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	if strings.TrimSpace(pass) != "" {
		config.Auth = append(config.Auth, ssh.Password(pass))
	}
	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("no SSH auth methods available (set password or key path)")
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
				<-done
			}()
		}
	}()

	return nil
}
