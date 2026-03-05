package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type typingMeshNode struct {
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	User           string   `json:"user"`
	Host           string   `json:"host"`
	Port           string   `json:"port"`
	OS             string   `json:"os"`
	PreferWSLPowerShell bool `json:"prefer_wsl_powershell"`
	HostCandidates []string `json:"host_candidates"`
	RepoCandidates []string `json:"repo_candidates"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "ghostty":
		if err := runGhostty(args); err != nil {
			exitIfErr(err, "typing ghostty")
		}
	case "terminal":
		if err := runTerminal(args); err != nil {
			exitIfErr(err, "typing terminal")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown typing command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runGhostty(argv []string) error {
	opts := flag.NewFlagSet("typing v1 ghostty", flag.ContinueOnError)
	host := opts.String("host", "gold", "Mesh host alias/name for target machine")
	user := opts.String("user", "", "SSH user for the target host")
	port := opts.String("port", "", "SSH port for the target host")
	repoPath := opts.String("repo", "", "Repository path to cd into before typing command")
	command := opts.String("command", "./dialtone_mod", "Command to type in Ghostty (default from positional args)")
	dryRun := opts.Bool("dry-run", false, "Print generated script without typing it")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	if strings.TrimSpace(*command) == "" && opts.NArg() > 0 {
		*command = strings.Join(opts.Args(), " ")
	}

	commandText := strings.TrimSpace(*command)
	if commandText == "" {
		return errors.New("typing command is required (use --command or positional args)")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	target, typedRepoPath, err := resolveTargetNode(repoRoot, strings.TrimSpace(*host), strings.TrimSpace(*user), strings.TrimSpace(*repoPath))
	if err != nil {
		return err
	}

	if strings.TrimSpace(*port) != "" {
		target.Port = strings.TrimSpace(*port)
	}
	if target.Port == "" {
		target.Port = "22"
	}
	if target.Host == "" {
		target.Host = strings.TrimSpace(*host)
	}

	hostForSSH := pickHostForSSH(target)
	if strings.TrimSpace(typedRepoPath) != "" {
		commandText = "cd " + typedRepoPath + " && " + commandText
	}
	targetAddr := target.User + "@" + hostForSSH

	scriptText, typedCommandText := buildGhosttyAppleScriptText(commandText)
	if *dryRun {
		fmt.Printf("typing target: %s\n", targetAddr)
		fmt.Println("appleScript:")
		fmt.Println(scriptText)
		fmt.Printf("typed command: %s\n", typedCommandText)
		return nil
	}

	return runTypingSSH(targetAddr, target.Port, scriptText)
}

func runTerminal(argv []string) error {
	opts := flag.NewFlagSet("typing v1 terminal", flag.ContinueOnError)
	host := opts.String("host", "legion", "Mesh host alias/name for target machine")
	user := opts.String("user", "", "SSH user for the target host")
	port := opts.String("port", "", "SSH port for the target host")
	repoPath := opts.String("repo", "", "Repository path to cd into before opening terminal")
	command := opts.String("command", "", "Command to run before opening an interactive shell")
	logPath := opts.String("log-path", "C:\\Users\\Public\\dialtone-typing-terminal.log", "Windows log file path for local terminal launch events")
	powershellPath := opts.String("powershell-path", defaultPowerShellPath(), "Path to powershell.exe when connecting via WSL/PowerShell")
	wslPath := opts.String("wsl-path", defaultWSLPath(), "Path to wsl.exe when launching local Windows terminal")
	windowsTerminalPath := opts.String("wt-path", defaultWindowsTerminalPath(), "Path to wt.exe when running locally on Windows from WSL")
	windowsTerminalProfile := opts.String("wt-profile", "", "Windows Terminal profile to use for local WSL terminal launches")
	forceLocalWindows := opts.Bool("local", false, "Force local Windows terminal launch for localhost/legion host")
	dryRun := opts.Bool("dry-run", false, "Print generated command without opening terminal")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	if strings.TrimSpace(*command) == "" && opts.NArg() > 0 {
		*command = strings.Join(opts.Args(), " ")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	target, typedRepoPath, err := resolveTargetNode(repoRoot, strings.TrimSpace(*host), strings.TrimSpace(*user), strings.TrimSpace(*repoPath))
	if err != nil {
		return err
	}

	if strings.TrimSpace(*port) != "" {
		target.Port = strings.TrimSpace(*port)
	}
	if target.Port == "" {
		target.Port = "22"
	}
	if target.Host == "" {
		target.Host = strings.TrimSpace(*host)
	}
	if target.User == "" {
		target.User = os.Getenv("USER")
	}

	hostForSSH := pickHostForSSH(target)
	if strings.TrimSpace(hostForSSH) == "" {
		return fmt.Errorf("no SSH host found for %q", target.Name)
	}
	targetAddr := target.User + "@" + hostForSSH
	isLocalWindowsTerminal := shouldUseLocalWindowsTerminalHost(target) || *forceLocalWindows

	localRepoPath := strings.TrimSpace(typedRepoPath)
	if isLocalWindowsTerminal && strings.TrimSpace(*repoPath) == "" {
		localRepoPath = ""
	}
	typedLocalCommand := buildLocalTypedCommand(strings.TrimSpace(*command), localRepoPath)
	terminalCommand := buildTerminalCommand(strings.TrimSpace(*command), localRepoPath)
	trimmedWTP := strings.TrimSpace(*windowsTerminalProfile)
	localPowerShellPath := strings.TrimSpace(*powershellPath)
	localWSLPath := resolveWSLPath(*wslPath)
	localTerminalPath := strings.TrimSpace(*windowsTerminalPath)
	localLogPath := strings.TrimSpace(*logPath)

	if *dryRun {
		fmt.Printf("typing target: %s\n", targetAddr)
		if isLocalWindowsTerminal {
			scriptCommand, err := buildLocalLauncherScriptCommand(repoRoot, localWSLPath, localTerminalPath, trimmedWTP, localLogPath, typedLocalCommand)
			if err != nil {
				return err
			}
			fmt.Printf("powershell script command:\n%s\n", scriptCommand)
		} else if shouldUsePowerShellForTypingNode(target) {
			fmt.Printf("powershell command:\n%s\n", buildPowerShellTerminalCommand(targetAddr, target.Port, terminalCommand))
		} else {
			args := buildSSHArgsForInteractiveTyping(targetAddr, target.Port)
			args = append(args, "bash", "-lc", terminalCommand)
			fmt.Printf("ssh command:\nssh")
			for _, arg := range args {
				fmt.Printf(" %q", arg)
			}
			fmt.Println()
		}
		return nil
	}

	if isLocalWindowsTerminal {
		return runTypingLocalPowerShellSession(repoRoot, typedLocalCommand, localPowerShellPath, localTerminalPath, localWSLPath, trimmedWTP, localLogPath)
	}
	if shouldUsePowerShellForTypingNode(target) {
		return runTypingPowerShellSession(targetAddr, target.Port, terminalCommand, localPowerShellPath)
	}
	return runTypingSSHInteractive(targetAddr, target.Port, terminalCommand)
}

func runTypingSSH(targetAddress, port string, scriptText string) error {
	args := []string{
		"-F", "/dev/null",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	}
	if strings.TrimSpace(port) != "" && strings.TrimSpace(port) != "22" {
		args = append(args, "-p", strings.TrimSpace(port))
	}
	remoteCmd := buildGhosttyTypingCommand(scriptText)
	args = append(args, targetAddress, remoteCmd)

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTypingSSHInteractive(targetAddress, port string, shellCommand string) error {
	args := buildSSHArgsForInteractiveTyping(targetAddress, port)
	args = append(args, "bash", "-lc", shellCommand)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTypingPowerShellSession(targetAddress, port, shellCommand, powershellPath string) error {
	powerShellPath := resolvePowerShellPath(powershellPath)
	if powerShellPath == "" {
		return errors.New("powershell not found; set powershell.exe in PATH or use linux/darwin typing terminal host")
	}
	command := buildPowerShellTerminalCommand(targetAddress, port, shellCommand)
	cmd := exec.Command(powerShellPath,
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		command,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTypingLocalPowerShellSession(repoRoot, shellCommand, powershellPath, wtPathOverride, wslPath, wtProfile, logPath string) error {
	command := trimForLocalPowerShellTerminal(shellCommand)
	localPowerShellPath := resolvePowerShellPath(powershellPath)
	if localPowerShellPath == "" {
		return errors.New("powershell not found; set powershell.exe in PATH or use linux/darwin typing terminal host")
	}
	scriptCommand, err := buildLocalLauncherScriptCommand(repoRoot, wslPath, wtPathOverride, wtProfile, logPath, command)
	if err != nil {
		return err
	}
	cmd := exec.Command(localPowerShellPath,
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		scriptCommand,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildLocalLauncherScriptCommand(repoRoot, wslPath, wtPathOverride, wtProfile, logPath, command string) (string, error) {
	scriptPath := filepath.Join(repoRoot, "src", "mods", "typing", "v1", "launch_local_terminal.ps1")
	scriptBody, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed reading local launcher script: %w", err)
	}
	wslBinary := strings.TrimSpace(windowsPathForCommand(wslPath))
	if wslBinary == "" {
		wslBinary = strings.TrimSpace(windowsPathForCommand(defaultWSLPath()))
	}
	if wslBinary == "" {
		wslBinary = "wsl.exe"
	}
	wtBinary := strings.TrimSpace(windowsPathForCommand(resolveWindowsTerminalPath(wtPathOverride)))
	if wtBinary == "" {
		wtBinary = strings.TrimSpace(windowsPathForCommand(wtPathOverride))
	}
	trimmedLogPath := strings.TrimSpace(logPath)
	if trimmedLogPath == "" {
		trimmedLogPath = "C:\\Users\\Public\\dialtone-typing-terminal.log"
	}
	cmd := strings.TrimSpace(string(scriptBody)) + "\n" +
		"Start-DialtoneLocalTerminal " +
		"-WslPath " + powerShellSingleQuote(wslBinary) + " " +
		"-CommandText " + powerShellSingleQuote(command) + " " +
		"-LogPath " + powerShellSingleQuote(trimmedLogPath) + " " +
		"-WtPath " + powerShellSingleQuote(wtBinary) + " " +
		"-WtProfile " + powerShellSingleQuote(strings.TrimSpace(wtProfile))
	return cmd, nil
}

func resolveTargetNode(repoRoot, host, explicitUser, explicitRepo string) (typingMeshNode, string, error) {
	nodes, _ := loadTypingMeshConfig(repoRoot)
	selected := typingMeshNode{Name: host}
	if host == "" {
		return selected, explicitRepo, errors.New("host is required")
	}
	if len(nodes) > 0 {
		if match, ok := findTypingMeshNode(nodes, host); ok {
			selected = match
		}
	}

	if strings.TrimSpace(explicitUser) != "" {
		selected.User = strings.TrimSpace(explicitUser)
	}
	if strings.TrimSpace(selected.User) == "" {
		selected.User = os.Getenv("USER")
	}

	repoPath := strings.TrimSpace(explicitRepo)
	if repoPath == "" && len(selected.RepoCandidates) > 0 {
		repoPath = selected.RepoCandidates[0]
	}
	if strings.TrimSpace(repoPath) == "" {
		repoPath = filepath.Join(os.Getenv("HOME"), "dialtone")
	}
	return selected, repoPath, nil
}

func loadTypingMeshConfig(repoRoot string) ([]typingMeshNode, error) {
	configPath := filepath.Join(repoRoot, "env", "mesh.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	nodes := []typingMeshNode{}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func findTypingMeshNode(nodes []typingMeshNode, rawHost string) (typingMeshNode, bool) {
	target := strings.ToLower(strings.TrimSpace(rawHost))
	target = strings.TrimSuffix(target, ".")
	for _, node := range nodes {
		if normalizeTypingHost(node.Name) == target {
			return node, true
		}
		for _, alias := range node.Aliases {
			if normalizeTypingHost(alias) == target {
				return node, true
			}
		}
	}
	return typingMeshNode{}, false
}

func pickHostForSSH(node typingMeshNode) string {
	candidates := append([]string{}, node.HostCandidates...)
	candidates = append(candidates, node.Aliases...)
	candidates = append(candidates, node.Host)
	for _, candidate := range preferTailnetHostsInTyping(candidates) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if isIPOnly(candidate) {
			continue
		}
		return strings.TrimSuffix(candidate, ".")
	}
	if node.Host != "" {
		return strings.TrimSuffix(node.Host, ".")
	}
	return ""
}

func preferTailnetHostsInTyping(candidates []string) []string {
	seen := map[string]struct{}{}
	tailnet := make([]string, 0, len(candidates))
	others := make([]string, 0, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSuffix(strings.TrimSpace(candidate), ".")
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if strings.HasSuffix(strings.ToLower(candidate), ".ts.net") {
			tailnet = append(tailnet, candidate)
		} else {
			others = append(others, candidate)
		}
	}
	out = append(out, tailnet...)
	out = append(out, others...)
	return out
}

func locateRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Clean(cwd)
	for i := 0; i < 30; i++ {
		if dir == "" || dir == "." {
			break
		}
		if _, err := os.Stat(filepath.Join(dir, "env", "mesh.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("cannot locate repo root (env/mesh.json missing)")
}

func normalizeTypingHost(v string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(v)), ".")
}

func isIPOnly(value string) bool {
	v := strings.TrimSpace(value)
	return net.ParseIP(v) != nil
}

func buildTerminalCommand(command, repoPath string) string {
	setup := []string{}
	if repoPath != "" {
		setup = append(setup, "cd "+shellQuoteForShell(repoPath)+" || true")
	}
	if command != "" {
		setup = append(setup, command)
	}
	tail := "bind 'set bell-style none' >/dev/null 2>&1; exec ${SHELL:-/bin/bash} -i"
	if len(setup) == 0 {
		return tail
	}
	return strings.Join(setup, " && ") + "; " + tail
}

func buildLocalTypedCommand(command, repoPath string) string {
	parts := []string{}
	if repoPath != "" {
		parts = append(parts, "cd "+shellQuoteForShell(repoPath))
	}
	if strings.TrimSpace(command) != "" {
		parts = append(parts, strings.TrimSpace(command))
	}
	return strings.Join(parts, " && ")
}

func shouldUsePowerShellForTypingNode(node typingMeshNode) bool {
	return strings.EqualFold(strings.TrimSpace(node.OS), "windows") &&
		node.PreferWSLPowerShell &&
		isTypingLocalWSL()
}

func isTypingLocalWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(raw))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

func shouldUseLocalWindowsTerminalHost(node typingMeshNode) bool {
	if !shouldUsePowerShellForTypingNode(node) {
		return false
	}
	for _, candidate := range append(append([]string{}, node.HostCandidates...), node.Host) {
		if isTypingHostLocal(candidate) {
			return true
		}
	}
	return false
}

func isTypingHostLocal(rawHost string) bool {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return isTypingLocalIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if isTypingLocalIP(ip) {
			return true
		}
	}
	return false
}

func isTypingLocalIP(target net.IP) bool {
	if target == nil {
		return false
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		var candidate net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			candidate = value.IP
		case *net.IPAddr:
			candidate = value.IP
		default:
			continue
		}
		if candidate == nil {
			continue
		}
		if candidate.Equal(target) {
			return true
		}
		if target4 := target.To4(); target4 != nil {
			if candidate4 := candidate.To4(); candidate4 != nil && target4.Equal(candidate4) {
				return true
			}
		}
	}
	return false
}

func resolvePowerShellPath(explicit string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if fallback := defaultPowerShellPath(); fallback != "" {
		return fallback
	}
	powerShell := "powershell.exe"
	if _, err := exec.LookPath(powerShell); err == nil {
		return powerShell
	}
	return ""
}

func resolveWindowsTerminalPath(explicit string) string {
	if explicit == "" {
		explicit = strings.TrimSpace(os.Getenv("WT_PATH"))
	}
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if explicitPath := defaultWindowsTerminalPath(); explicitPath != "" {
		return explicitPath
	}
	if terminal, err := exec.LookPath("wt.exe"); err == nil {
		return terminal
	}
	return ""
}

func defaultWindowsTerminalPath() string {
	paths := []string{
		"/mnt/c/Users/timca/AppData/Local/Microsoft/WindowsApps/wt.exe",
	}
	for _, candidate := range paths {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func defaultPowerShellPath() string {
	candidates := []string{
		"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func defaultWSLPath() string {
	candidates := []string{
		"/mnt/c/Windows/System32/wsl.exe",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func resolveWSLPath(explicit string) string {
	if explicit == "" {
		explicit = strings.TrimSpace(os.Getenv("WSL_PATH"))
	}
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if path := defaultWSLPath(); path != "" {
		return path
	}
	return "wsl.exe"
}

func windowsPathForCommand(value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "/mnt/") && len(path) >= 7 {
		drive := strings.ToUpper(path[5:6])
		remainder := path[6:]
		remainder = strings.TrimPrefix(remainder, "/")
		if remainder == "" {
			return drive + ":\\"
		}
		return drive + ":\\" + strings.ReplaceAll(remainder, "/", "\\")
	}
	return path
}

func buildPowerShellTerminalCommand(targetAddress, port, posixCommand string) string {
	args := []string{"ssh"}
	args = append(args, buildSSHArgsForInteractiveTyping(targetAddress, port)...)
	remoteCommand := "bash -lc " + shellQuoteForShell(posixCommand)
	args = append(args, remoteCommand)
	wslBinary := windowsPathForCommand(resolveWSLPath(""))
	if wslBinary == "" {
		wslBinary = "wsl.exe"
	}
	command := "Set-Location C:\\; " + wslBinary + " --cd ~ -e bash -lc " + powerShellSingleQuote(strings.Join(args, " "))
	return command
}

func trimForLocalPowerShellTerminal(command string) string {
	trimmed := strings.TrimSpace(command)
	return strings.ReplaceAll(trimmed, "exec ${SHELL:-/bin/bash}", "exec /bin/bash")
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shellFormatCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	parts = append(parts, args...)
	output := make([]string, len(parts))
	for idx, part := range parts {
		if strings.IndexAny(part, " \t\r\n") >= 0 {
			output[idx] = fmt.Sprintf("%q", part)
			continue
		}
		output[idx] = part
	}
	return strings.Join(output, " ")
}

func buildSSHArgsForInteractiveTyping(targetAddress, port string) []string {
	args := []string{
		"-F", "/dev/null",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-tt",
	}
	if strings.TrimSpace(port) != "" && strings.TrimSpace(port) != "22" {
		args = append(args, "-p", strings.TrimSpace(port))
	}
	args = append(args, targetAddress)
	return args
}

func shellQuoteForShell(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "'\"'\"'") + "'"
}

func escapeAppleString(value string) string {
	v := strings.ReplaceAll(value, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", `\"`)
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "\\n")
	v = strings.ReplaceAll(v, "\t", "\\t")
	return v
}

func buildGhosttyAppleScriptText(command string) (string, string) {
	text := escapeAppleString(strings.TrimSpace(command))
	script := strings.Join([]string{
		`tell application "Ghostty" to activate`,
		"delay 0.2",
		`tell application "System Events"`,
		`	tell process "ghostty"`,
		fmt.Sprintf(`		keystroke "%s"`, text),
		`		key code 36`,
		`	end tell`,
		`end tell`,
	}, "\n")
	return script, text
}

func buildGhosttyTypingCommand(script string) string {
	payload := base64.StdEncoding.EncodeToString([]byte(script))
	return fmt.Sprintf(
		"printf %s %s | base64 -D | osascript",
		shellQuoteForShell("%s"),
		shellQuoteForShell(payload),
	)
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod typing v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  ghostty --host <name|ip> [--user USER] [--port PORT] [--repo PATH]")
	fmt.Println("      [--command \"./dialtone_mod ...\"] [positional command fallback]")
	fmt.Println("      [--dry-run]")
	fmt.Println("      Send a command into the active Ghostty window on the target host via AppleScript")
	fmt.Println("  terminal --host <name|ip> [--user USER] [--port PORT] [--repo PATH]")
	fmt.Println("      [--command \"./dialtone_mod ...\"] [positional command fallback]")
	fmt.Println("      [--powershell-path PATH]")
	fmt.Println("      [--wsl-path PATH]")
	fmt.Println("      [--wt-path PATH]")
	fmt.Println("      [--wt-profile PROFILE]")
	fmt.Println("      [--local]")
	fmt.Println("      Use --wt-profile to force the Windows Terminal profile (for example: Ubuntu).")
	fmt.Println("      [--dry-run]")
	fmt.Println("      Open an interactive SSH session to the target host; for windows+prefer_wsl_powershell")
	fmt.Println("      nodes, launch local WSL/PowerShell terminal only when that target matches the local host")
	fmt.Println("      (uses Windows Terminal via wt.exe when available, fallback to PowerShell)")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
