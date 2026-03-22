package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const tmuxRetryAttempts = 6

var (
	tmuxOutputRunner     = tmuxOutput
	defaultTmuxRetrySleep = func() { time.Sleep(250 * time.Millisecond) }
	tmuxRetrySleep       = defaultTmuxRetrySleep
)

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
	case "start":
		if err := runStart(args); err != nil {
			exitIfErr(err, "codex start")
		}
	case "status":
		if err := runStatus(args); err != nil {
			exitIfErr(err, "codex status")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown codex command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runStart(argv []string) error {
	opts := flag.NewFlagSet("codex v1 start", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name on the default tmux server")
	pane := opts.String("pane", "", "Explicit tmux pane target to launch Codex into")
	shellName := opts.String("shell", "default", "flake shell to enter before launching Codex")
	reasoning := opts.String("reasoning", "medium", "reasoning label for startup banner")
	model := opts.String("model", "gpt-5.4", "Codex model to launch")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	startCmd := buildStartCommand(repoRoot, *shellName, *reasoning, *model)
	explicitPane := strings.TrimSpace(*pane)
	if explicitPane != "" {
		target := normalizePaneTarget(explicitPane)
		if err := respawnPane(target, startCmd); err != nil {
			return err
		}
		fmt.Printf("started codex in tmux pane %s\n", target)
		return nil
	}
	if isProxyActive() {
		target, err := currentPaneTarget()
		if err != nil {
			return err
		}
		if err := clearPaneHistory(target); err != nil {
			return err
		}
		return runInCurrentPane(startCmd)
	}
	if err := ensureSession(*session, repoRoot); err != nil {
		return err
	}
	target, err := activePaneTarget(*session)
	if err != nil {
		return err
	}
	if err := respawnPane(target, startCmd); err != nil {
		return err
	}
	fmt.Printf("started codex in tmux session %s via %s\n", *session, target)
	return nil
}

func runStatus(argv []string) error {
	opts := flag.NewFlagSet("codex v1 status", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name on the default tmux server")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if isProxyActive() {
		target, err := currentPaneTarget()
		if err != nil {
			return err
		}
		out, err := tmuxOutput("display-message", "-p", "-t", target, "#{session_name}\t#{window_index}.#{pane_index}\t#{pane_current_command}\t#{pane_current_path}")
		if err != nil {
			return err
		}
		fmt.Println(strings.TrimSpace(out))
		return nil
	}

	target, err := activePaneTarget(*session)
	if err != nil {
		return err
	}
	out, err := tmuxOutput("display-message", "-p", "-t", target, "#{session_name}\t#{window_index}.#{pane_index}\t#{pane_current_command}\t#{pane_current_path}")
	if err != nil {
		return err
	}
	fmt.Println(strings.TrimSpace(out))
	return nil
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
		if _, err := os.Stat(filepath.Join(dir, "src", "mods.go")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("cannot locate repo root")
}

func ensureSession(session, cwd string) error {
	if strings.TrimSpace(session) == "" {
		return errors.New("session is required")
	}
	if _, err := tmuxOutput("has-session", "-t", session); err == nil {
		return nil
	}
	if _, err := tmuxOutput("new-session", "-d", "-s", session, "-c", cwd); err != nil {
		return err
	}
	return nil
}

func activePaneTarget(session string) (string, error) {
	out, err := tmuxOutput("list-panes", "-t", session, "-F", "#{session_name}:#{window_index}.#{pane_index}")
	if err != nil {
		return "", err
	}
	target := strings.TrimSpace(strings.Split(strings.TrimSpace(out), "\n")[0])
	if target == "" {
		return "", fmt.Errorf("could not resolve active pane for session %s", session)
	}
	return target, nil
}

func currentPaneTarget() (string, error) {
	pane := strings.TrimSpace(os.Getenv("TMUX_PANE"))
	if pane == "" {
		return "", errors.New("TMUX_PANE is not set")
	}
	return pane, nil
}

func buildStartCommand(repoRoot, shellName, reasoning, model string) string {
	codexCmd := buildCodexExecCommand(strings.TrimSpace(model))
	inner := fmt.Sprintf(
		"clear; printf 'Starting Codex CLI with %s (requested reasoning: %s) and skipping confirmations...\\n'; %s",
		shellSingleQuoteLiteral(strings.TrimSpace(model)),
		shellSingleQuoteLiteral(strings.TrimSpace(reasoning)),
		codexCmd,
	)
	return fmt.Sprintf(
		"cd %s && exec nix --extra-experimental-features %s develop %s --command bash -lc %s",
		shellQuote(repoRoot),
		shellQuote("nix-command flakes"),
		shellQuote(".#"+strings.TrimSpace(shellName)),
		shellQuote(inner),
	)
}

func buildCodexExecCommand(model string) string {
	args := strings.Join([]string{
		"-c", shellQuote("check_for_update_on_startup=false"),
		"-m", shellQuote(model),
		"-a", "never",
		"-s", "danger-full-access",
	}, " ")
	return fmt.Sprintf(
		"if command -v codex >/dev/null 2>&1; then exec env CI=1 codex %s; else exec env CI=1 npx --yes @openai/codex %s; fi",
		args,
		args,
	)
}

func isProxyActive() bool {
	return strings.TrimSpace(os.Getenv("DIALTONE_TMUX_PROXY_ACTIVE")) == "1"
}

func normalizePaneTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	parts := strings.Split(trimmed, ":")
	if len(parts) == 3 {
		return fmt.Sprintf("%s:%s.%s", parts[0], parts[1], parts[2])
	}
	return trimmed
}

func runInCurrentPane(command string) error {
	cmd := exec.Command("bash", "-lc", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resetPane(target string) error {
	if _, err := tmuxOutput("send-keys", "-t", target, "C-c"); err != nil {
		return err
	}
	return nil
}

func clearPaneHistory(target string) error {
	if _, err := tmuxOutputWithRetry("clear-history", "-t", target); err != nil {
		return err
	}
	return nil
}

func respawnPane(target, command string) error {
	if err := clearPaneHistory(target); err != nil {
		return err
	}
	if _, err := tmuxOutputWithRetry("respawn-pane", "-k", "-t", target, "bash", "-lc", command); err != nil {
		return err
	}
	return nil
}

func sendKeys(target, text string, enter bool) error {
	if _, err := tmuxOutput("send-keys", "-t", target, "--", text); err != nil {
		return err
	}
	if enter {
		if _, err := tmuxOutput("send-keys", "-t", target, "C-m"); err != nil {
			return err
		}
	}
	return nil
}

func tmuxOutput(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		text = strings.TrimSpace(text)
		if text == "" {
			text = err.Error()
		}
		return "", fmt.Errorf("tmux %s failed: %s", strings.Join(args, " "), text)
	}
	return text, nil
}

func tmuxOutputWithRetry(args ...string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= tmuxRetryAttempts; attempt++ {
		out, err := tmuxOutputRunner(args...)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !isTransientTmuxError(err) || attempt == tmuxRetryAttempts {
			return "", err
		}
		tmuxRetrySleep()
	}
	return "", lastErr
}

func isTransientTmuxError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "server exited unexpectedly") ||
		strings.Contains(text, "lost server") ||
		strings.Contains(text, "no server running")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func shellSingleQuoteLiteral(value string) string {
	return strings.ReplaceAll(value, "'", "'\"'\"'")
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod codex v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start [--session codex-view] [--shell default|repl-v1|ssh-v1] [--reasoning medium] [--model gpt-5.4]")
	fmt.Println("       Ensure the tmux session exists on the default server and launch Codex there via nix develop")
	fmt.Println("  status [--session codex-view]")
	fmt.Println("       Show the current pane command and cwd for the tmux session")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
