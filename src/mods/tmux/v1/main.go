package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type tmuxMeshNode struct {
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	User           string   `json:"user"`
	Host           string   `json:"host"`
	Port           string   `json:"port"`
	HostCandidates []string `json:"host_candidates"`
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
	case "logs":
		if err := runLogs(args); err != nil {
			exitIfErr(err, "tmux logs")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown tmux command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runLogs(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 logs", flag.ContinueOnError)
	host := opts.String("host", sanitizeDialtoneHost(os.Getenv("DIALTONE_HOSTNAME")), "Mesh host alias/name")
	user := opts.String("user", "", "SSH user")
	port := opts.String("port", "", "SSH port")
	session := opts.String("session", "", "tmux session name")
	pane := opts.String("pane", "0.0", "tmux pane target")
	lines := opts.Int("lines", 10, "Number of lines to capture")
	dryRun := opts.Bool("dry-run", false, "Print generated command without running")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	node, err := resolveTmuxNode(repoRoot, strings.TrimSpace(*host))
	if err != nil {
		return err
	}

	targetUser := strings.TrimSpace(*user)
	if targetUser == "" {
		targetUser = strings.TrimSpace(node.User)
	}
	if targetUser == "" {
		targetUser = strings.TrimSpace(os.Getenv("USER"))
	}
	targetPort := strings.TrimSpace(*port)
	if targetPort == "" {
		targetPort = strings.TrimSpace(node.Port)
	}
	if targetPort == "" {
		targetPort = "22"
	}
	targetSession := strings.TrimSpace(*session)
	if targetSession == "" {
		targetSession = fmt.Sprintf("dialtone-%s", sanitizeDialtoneHost(node.Name))
	}

	hostForSSH := pickHostForTMux(node)
	if hostForSSH == "" {
		hostForSSH = strings.TrimSpace(node.Host)
	}
	if hostForSSH == "" {
		return errors.New("tmux host is missing")
	}

	linesValue := *lines
	if linesValue <= 0 {
		return errors.New("--lines must be a positive integer")
	}

	targetAddr := fmt.Sprintf("%s@%s", targetUser, hostForSSH)
	rawSession := strings.TrimSpace(targetSession)
	rawPane := strings.TrimSpace(*pane)

	remoteScript := buildRemoteTmuxLogsScript(rawSession, rawPane, linesValue)
	if *dryRun {
		fmt.Printf("tmux target: %s\n", targetAddr)
		fmt.Println("remote script:")
		fmt.Println(remoteScript)
		return nil
	}

	return runRemoteTmuxCommand(targetAddr, targetPort, remoteScript)
}

func runRemoteTmuxCommand(target string, port string, script string) error {
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
	args = append(args, target)
	args = append(args, "bash", "-lc", script)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildRemoteTmuxLogsScript(session string, pane string, lines int) string {
	if strings.TrimSpace(pane) == "" {
		pane = "0.0"
	}
	tmuxSession := shellQuote(session)
	tmuxPane := shellQuote(pane)

	script := fmt.Sprintf(`set -euo pipefail
find_tmux_command() {
  if command -v tmux >/dev/null 2>&1; then
    command -v tmux
    return 0
  fi

  if command -v lsof >/dev/null 2>&1; then
    local pid
    pid=$(pgrep -x tmux | head -n1 || true)
    if [ -n "$pid" ]; then
      local found
      found=$(lsof -p "$pid" -a -d txt 2>/dev/null | awk '/tmux/ {print $9; exit}')
      if [ -n "$found" ]; then
        echo "$found"
        return 0
      fi
    fi
  fi

  for p in /nix/store/*-tmux-*/bin/tmux; do
    if [ -x "$p" ]; then
      echo "$p"
      return 0
    fi
  done

  if [ -x "$HOME/.nix-profile/bin/tmux" ]; then
    echo "$HOME/.nix-profile/bin/tmux"
    return 0
  fi

  return 1
}

TMUX_CMD=$(find_tmux_command)
if [ -z "$TMUX_CMD" ]; then
  echo "tmux is not available on this host" >&2
  exit 1
fi

SESSION=%[1]s
PANE=%[2]s
LINES=%d

if [ -n "$PANE" ] && [ "$PANE" != "0.0" ]; then
  TARGET="$SESSION:$PANE"
else
  TARGET="$SESSION:0.0"
fi

if ! "$TMUX_CMD" has-session -t "$SESSION" 2>/dev/null; then
  FALLBACK_SESSION=$("$TMUX_CMD" list-sessions -F '#{session_name}' 2>/dev/null | head -n1 || true)
  if [ -n "$FALLBACK_SESSION" ]; then
    SESSION="$FALLBACK_SESSION"
    if [ -n "$PANE" ] && [ "$PANE" != "0.0" ]; then
      TARGET="$SESSION:$PANE"
    else
      TARGET="$SESSION:0.0"
    fi
  else
    echo "tmux session not found: $SESSION" >&2
    exit 1
  fi
fi

if ! "$TMUX_CMD" capture-pane -pt "$TARGET" -S "-$LINES"; then
  FIRST_PANE=$("$TMUX_CMD" list-panes -t "$SESSION" -F '#{window_index}.#{pane_index}' | head -n1 || true)
  if [ -n "$FIRST_PANE" ]; then
    "$TMUX_CMD" capture-pane -pt "$SESSION:$FIRST_PANE" -S "-$LINES"
    exit $?
  fi
  echo "tmux session $SESSION has no panes" >&2
  exit 1
fi
`, tmuxSession, tmuxPane, lines)

	return script
}

func resolveTmuxNode(repoRoot, rawHost string) (tmuxMeshNode, error) {
	if strings.TrimSpace(rawHost) == "" {
		return tmuxMeshNode{}, errors.New("host is required")
	}

	nodes, err := loadTmuxMeshConfig(repoRoot)
	if err == nil {
		if node, ok := findTmuxMeshNode(nodes, rawHost); ok {
			return node, nil
		}
	}

	return tmuxMeshNode{
		Name: rawHost,
		Host: rawHost,
		User: strings.TrimSpace(os.Getenv("USER")),
		Port: "22",
	}, nil
}

func loadTmuxMeshConfig(repoRoot string) ([]tmuxMeshNode, error) {
	configPath := filepath.Join(repoRoot, "env", "mesh.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	nodes := []tmuxMeshNode{}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func findTmuxMeshNode(nodes []tmuxMeshNode, rawHost string) (tmuxMeshNode, bool) {
	target := sanitizeDialtoneHost(rawHost)
	for _, node := range nodes {
		if sanitizeDialtoneHost(node.Name) == target {
			return node, true
		}
		for _, alias := range node.Aliases {
			if sanitizeDialtoneHost(alias) == target {
				return node, true
			}
		}
	}
	return tmuxMeshNode{}, false
}

func pickHostForTMux(node tmuxMeshNode) string {
	candidates := append([]string{}, node.HostCandidates...)
	candidates = append(candidates, node.Host)
	ordered := preferTailnetHostsForMods(candidates)
	for _, candidate := range ordered {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		return strings.TrimSuffix(candidate, ".")
	}
	return ""
}

func preferTailnetHostsForMods(candidates []string) []string {
	seen := map[string]struct{}{}
	tailnet := make([]string, 0, len(candidates))
	others := make([]string, 0, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = strings.TrimSuffix(strings.TrimSpace(c), ".")
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		if strings.HasSuffix(strings.ToLower(c), ".ts.net") {
			tailnet = append(tailnet, c)
		} else {
			others = append(others, c)
		}
	}
	out = append(out, tailnet...)
	out = append(out, others...)
	return out
}

func sanitizeDialtoneHost(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return ""
	}
	return strings.TrimSuffix(v, ".")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	repl := strings.ReplaceAll(value, "'", `'"'"'`)
	return "'" + repl + "'"
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

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod tmux v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  logs --host <name|ip> [--user USER] [--port PORT]")
	fmt.Println("       [--session dialtone-<host>] [--pane 0.0] [--lines 120] [--dry-run]")
	fmt.Println("       Capture tmux scrollback reliably for the target session/pane")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
