package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var moshConnectRunner = runMoshSession
var sshConnectRunner = runSSHSession
var setupRunner = runSetup
var moshSSHCommonOpts = []string{
	"-F", "/dev/null",
	"-o", "BatchMode=yes",
	"-o", "StrictHostKeyChecking=no",
	"-o", "UserKnownHostsFile=/dev/null",
	"-o", "GSSAPIAuthentication=no",
}

func runConnect(args []string) error {
	cfg := parseConnectArgs(args)
	if cfg.host == "" {
		return errors.New("connect requires --host or positional host argument")
	}

	repoRoot, err := resolveConnectRepoRoot(cfg.repoRoot)
	if err != nil {
		return err
	}

	session := cfg.session
	if session == "" {
		session = fmt.Sprintf("dialtone-%s", sanitizeDialtoneHost(cfg.host))
	}
	remoteCommand := strings.TrimSpace(cfg.command)
	if remoteCommand == "" {
		remoteCommand = buildDefaultTmuxCommand(session)
	}

	remoteShell := buildRemoteShellCommand(remoteCommand, cfg.host, repoRoot)

	if cfg.ensure {
		if err := setupRunner([]string{"--host", cfg.host, "--ensure"}); err != nil {
			return err
		}
	}
	if cfg.dryRun {
		fmt.Println(remoteShell)
		return nil
	}

	if err := checkConnectPrereqs(cfg.host); err != nil {
		return err
	}

	if err := moshConnectRunner(cfg.host, remoteShell); err != nil {
		if !cfg.fallbackSSH {
			return err
		}
		return sshConnectRunner(cfg.host, remoteShell)
	}
	return nil
}

type connectOptions struct {
	host        string
	command     string
	session     string
	repoRoot    string
	ensure      bool
	fallbackSSH bool
	dryRun      bool
}

func parseConnectArgs(argv []string) connectOptions {
	fs := flag.NewFlagSet("mosh v1 connect", flag.ContinueOnError)
	host := fs.String("host", "", "Remote host name")
	command := fs.String("command", "", "Command to run on remote shell")
	session := fs.String("session", "", "Session name for tmux fallback")
	repoRoot := fs.String("repo-root", "", "Remote repo root path")
	ensure := fs.Bool("ensure", false, "Run mosh setup before connect (install mosh-server if needed)")
	fallbackSSH := fs.Bool("fallback-ssh", false, "Fallback to SSH if mosh fails")
	dryRun := fs.Bool("dry-run", false, "Print generated remote command without connecting")
	_ = fs.Parse(argv)

	hostValue := strings.TrimSpace(*host)
	if hostValue == "" && fs.NArg() > 0 {
		hostValue = strings.TrimSpace(fs.Arg(0))
	}
	if hostValue == "" {
		hostValue = sanitizeDialtoneHost(os.Getenv("DIALTONE_HOSTNAME"))
	}

	return connectOptions{
		host:        hostValue,
		command:     strings.TrimSpace(*command),
		session:     strings.TrimSpace(*session),
		repoRoot:    strings.TrimSpace(*repoRoot),
		ensure:      *ensure,
		fallbackSSH: *fallbackSSH,
		dryRun:      *dryRun,
	}
}

func resolveConnectRepoRoot(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		abs := strings.TrimSpace(explicit)
		if !filepath.IsAbs(abs) {
			cwd, err := os.Getwd()
			if err != nil {
				return "", err
			}
			abs = filepath.Join(cwd, abs)
		}
		return abs, nil
	}
	return "", nil
}

func buildDefaultTmuxCommand(session string) string {
	session = strings.TrimSpace(session)
	if session == "" {
		session = "dialtone"
	}
	return fmt.Sprintf("tmux new-session -A -s %s -n %s", shellQuote(session), shellQuote(session))
}

func buildRemoteShellCommand(remoteCommand, host, repoRoot string) string {
	var script strings.Builder
	script.WriteString("export DIALTONE_HOSTNAME=")
	script.WriteString(shellQuote(sanitizeDialtoneHost(host)))
	script.WriteByte('\n')
	if strings.TrimSpace(repoRoot) != "" {
		script.WriteString("cd ")
		script.WriteString(shellQuote(repoRoot))
		script.WriteString(";\n")
	}
	script.WriteString(remoteCommand)
	return script.String()
}

func sanitizeDialtoneHost(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.ToLower(value)
	if value == "" {
		return "dialtone"
	}
	return strings.TrimSuffix(value, ".")
}

func checkConnectPrereqs(host string) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("missing connect host")
	}
	return nil
}

func runMoshSession(host, remoteShell string) error {
	cmd := exec.Command("mosh", "--ssh", joinSSHCommand("ssh", moshSSHCommonOpts...), host, "--", "bash", "-lc", remoteShell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runSSHSession(host, remoteShell string) error {
	if isLocalHost(host) {
		cmd := exec.Command("bash", "-lc", remoteShell)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	args := []string{}
	args = append(args, moshSSHCommonOpts...)
	args = append(args, host, "bash", "-lc", remoteShell)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	sanitized := strings.ReplaceAll(value, "'", `'"'"'"`)
	return "'" + sanitized + "'"
}

func isLocalHost(host string) bool {
	target := sanitizeDialtoneHost(host)
	if target == "" {
		return false
	}

	if target == "localhost" || target == "127.0.0.1" || target == "::1" {
		return true
	}

	localConfigured := sanitizeDialtoneHost(os.Getenv("DIALTONE_HOSTNAME"))
	if target == localConfigured {
		return true
	}
	localEnvHost := sanitizeDialtoneHost(os.Getenv("HOSTNAME"))
	if target == localEnvHost {
		return true
	}

	hostname, err := os.Hostname()
	if err != nil {
		return false
	}
	localHostname := sanitizeDialtoneHost(strings.SplitN(hostname, ".", 2)[0])
	if target == localHostname {
		return true
	}
	return sanitizeDialtoneHost(hostname) == target
}

func joinSSHCommand(binary string, opts ...string) string {
	parts := make([]string, 0, len(opts)+1)
	parts = append(parts, binary)
	for _, opt := range opts {
		parts = append(parts, shellQuote(strings.TrimSpace(opt)))
	}
	return strings.Join(parts, " ")
}
