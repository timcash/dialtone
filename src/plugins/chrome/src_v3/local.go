package src_v3

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/nats-io/nats.go"
)

func isLocalHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	return host == "" || host == "local" || host == "localhost" || host == "127.0.0.1"
}

func localServiceRoot(role string) string {
	return filepath.Join(defaultProfileDir(role), "service")
}

func localServicePIDPath(role string) string {
	return filepath.Join(localServiceRoot(role), "daemon.pid")
}

func localServiceStdoutPath(role string) string {
	return filepath.Join(localServiceRoot(role), "daemon.out.log")
}

func localServiceStderrPath(role string) string {
	return filepath.Join(localServiceRoot(role), "daemon.err.log")
}

func localBinaryPath() string {
	return filepath.Join(resolveRepoRoot(), "bin", binaryName(runtime.GOOS, runtime.GOARCH))
}

func ensureLocalBinary() (string, error) {
	bin := localBinaryPath()
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}
	if err := buildBinaryFor(bin, runtime.GOOS, runtime.GOARCH); err != nil {
		return "", err
	}
	return bin, nil
}

func readPIDFile(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(raw)))
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil || proc == nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func localServiceRunning(role string) bool {
	pid, err := readPIDFile(localServicePIDPath(role))
	if err != nil {
		return false
	}
	return processAlive(pid)
}

func waitForLocalNATS(role string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := sendLocalCommand(commandRequest{Command: "status", Role: role}); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for local chrome src_v3 daemon")
}

func startLocalService(role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	if localServiceRunning(role) {
		return nil
	}
	bin, err := ensureLocalBinary()
	if err != nil {
		return err
	}
	root := localServiceRoot(role)
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	stdout, err := os.OpenFile(localServiceStdoutPath(role), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(localServiceStderrPath(role), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer stderr.Close()

	cmd := exec.Command(bin, "src_v3", "daemon", "--role", role, "--chrome-port", strconv.Itoa(defaultChromePort), "--nats-port", strconv.Itoa(defaultNATSPort))
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	prepareBackgroundCommand(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(localServicePIDPath(role), []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); err != nil {
		_ = killPID(cmd.Process.Pid)
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		logs.Warn("chrome src_v3 local daemon release failed: %v", err)
	}
	return waitForLocalNATS(role, 15*time.Second)
}

func stopLocalService(role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	if resp, err := sendLocalCommand(commandRequest{Command: "status", Role: role}); err == nil {
		if resp.BrowserPID > 0 {
			_ = killPID(resp.BrowserPID)
		}
	}
	pidPath := localServicePIDPath(role)
	pid, err := readPIDFile(pidPath)
	if err != nil {
		return nil
	}
	_ = killPID(pid)
	_ = os.Remove(pidPath)
	return nil
}

func ensureLocalService(role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	if _, err := sendLocalCommand(commandRequest{Command: "status", Role: role}); err == nil {
		return nil
	}
	return startLocalService(role)
}

func sendLocalCommand(req commandRequest) (*commandResponse, error) {
	if strings.TrimSpace(req.Role) == "" {
		req.Role = defaultRole
	}
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", defaultNATSPort), nats.Timeout(defaultTimeout))
	if err != nil {
		return nil, err
	}
	defer nc.Close()
	raw, _ := json.Marshal(req)
	msg, err := nc.Request(natsSubject(req.Role), raw, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var resp commandResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, err
	}
	if !resp.OK && strings.TrimSpace(resp.Error) != "" {
		return &resp, errors.New(strings.TrimSpace(resp.Error))
	}
	return &resp, nil
}

func sendCommandByTarget(host string, req commandRequest, autoStart bool) (*commandResponse, error) {
	if isLocalHost(host) {
		if autoStart {
			if err := ensureLocalService(req.Role); err != nil {
				return nil, err
			}
		}
		return sendLocalCommand(req)
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	return sendRemoteCommand(node, req)
}

func deployTarget(host, role string, startService bool) error {
	if isLocalHost(host) {
		if _, err := ensureLocalBinary(); err != nil {
			return err
		}
		if startService {
			return startLocalService(role)
		}
		return nil
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return err
	}
	return deployRemoteBinary(node, strings.TrimSpace(role), startService)
}

func serviceTarget(host, mode, role string) (*commandResponse, error) {
	if isLocalHost(host) {
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "start":
			if err := startLocalService(role); err != nil {
				return nil, err
			}
			return sendLocalCommand(commandRequest{Command: "status", Role: role})
		case "stop":
			return nil, stopLocalService(role)
		case "status":
			return sendLocalCommand(commandRequest{Command: "status", Role: role})
		default:
			return nil, fmt.Errorf("unsupported service mode: %s", mode)
		}
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "start":
		return nil, startRemoteService(node, strings.TrimSpace(role))
	case "stop":
		return nil, stopRemoteService(node)
	case "status":
		return sendRemoteCommand(node, commandRequest{Command: "status", Role: strings.TrimSpace(role)})
	default:
		return nil, fmt.Errorf("unsupported service mode: %s", mode)
	}
}

func tailText(path string, lines int) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if lines <= 0 || len(parts) <= lines {
		return strings.Join(parts, "\n")
	}
	return strings.Join(parts[len(parts)-lines:], "\n")
}

func readTargetLogs(host, role string, lines int) (string, string, error) {
	if isLocalHost(host) {
		role = strings.TrimSpace(role)
		if role == "" {
			role = defaultRole
		}
		return tailText(localServiceStdoutPath(role), lines), tailText(localServiceStderrPath(role), lines), nil
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return "", "", err
	}
	return readRemoteLogs(node, lines)
}

func doctorTarget(host, role string) error {
	if isLocalHost(host) {
		resp, err := sendLocalCommand(commandRequest{Command: "status", Role: role})
		if err != nil {
			return err
		}
		printResponse(resp)
		stdout, stderr, _ := readTargetLogs("", role, 80)
		if strings.TrimSpace(stdout) != "" {
			fmt.Println("STDOUT LOG")
			fmt.Println(stdout)
		}
		if strings.TrimSpace(stderr) != "" {
			fmt.Println("STDERR LOG")
			fmt.Println(stderr)
		}
		return nil
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return err
	}
	return runRemoteDoctor(node)
}

func resetTarget(host, role string) error {
	if isLocalHost(host) {
		_, err := sendLocalCommand(commandRequest{Command: "reset", Role: role})
		return err
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return err
	}
	return resetRemoteHost(node)
}

func localListenPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
