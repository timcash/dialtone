package chrome

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func openSSHDebugTunnel(nodeInfo sshv1.MeshNode, remotePort int) (io.Closer, int, error) {
	client, err := sshv1.DialSSH(nodeInfo.Host, nodeInfo.Port, nodeInfo.User, "")
	if err != nil {
		if closer, port, xerr := openExternalSSHTunnel(nodeInfo, remotePort); xerr == nil {
			return closer, port, nil
		}
		return nil, 0, err
	}
	localPort, err := allocateLocalPort()
	if err != nil {
		_ = client.Close()
		return nil, 0, err
	}
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	remoteAddr := fmt.Sprintf("127.0.0.1:%d", remotePort)
	if err := sshv1.ForwardRemoteToLocal(client, remoteAddr, localAddr); err != nil {
		_ = client.Close()
		return nil, 0, err
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if canDialHostPort("127.0.0.1", localPort, 150*time.Millisecond) {
			return client, localPort, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	_ = client.Close()
	return nil, 0, fmt.Errorf("local tunnel %s did not become ready", localAddr)
}

type processCloser struct {
	cmd *exec.Cmd
}

func (p *processCloser) Close() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Kill()
	_, _ = p.cmd.Process.Wait()
	return nil
}

func openExternalSSHTunnel(nodeInfo sshv1.MeshNode, remotePort int) (io.Closer, int, error) {
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, 0, err
	}
	localPort, err := allocateLocalPort()
	if err != nil {
		return nil, 0, err
	}
	target := fmt.Sprintf("%s@%s", nodeInfo.User, nodeInfo.Host)
	localSpec := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", localPort, remotePort)
	args := []string{"-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-o", "ConnectTimeout=6", "-p", nodeInfo.Port, "-N", "-L", localSpec, target}
	cmd := exec.Command(sshPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = "ssh tunnel exited early"
			}
			return nil, 0, errors.New(msg)
		}
		if canDialHostPort("127.0.0.1", localPort, 150*time.Millisecond) {
			return &processCloser{cmd: cmd}, localPort, nil
		}
		time.Sleep(60 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = "ssh tunnel did not become ready"
	}
	return nil, 0, errors.New(msg)
}

func allocateLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok || addr.Port <= 0 {
		return 0, fmt.Errorf("failed to allocate local port")
	}
	return addr.Port, nil
}
