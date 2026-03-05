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
	"strings"
)

type typingMeshNode struct {
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	User           string   `json:"user"`
	Host           string   `json:"host"`
	Port           string   `json:"port"`
	OS             string   `json:"os"`
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
	command := opts.String("command", "./dialtone2.sh", "Command to type in Ghostty (default from positional args)")
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
	for _, candidate := range candidates {
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
	fmt.Println("Usage: ./dialtone2.sh typing v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  ghostty --host <name|ip> [--user USER] [--port PORT] [--repo PATH]")
	fmt.Println("      [--command \"./dialtone2.sh ...\"] [positional command fallback]")
	fmt.Println("      [--dry-run]")
	fmt.Println("      Send a command into the active Ghostty window on the target host via AppleScript")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
