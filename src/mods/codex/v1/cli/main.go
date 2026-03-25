package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/internal/tmuxcmd"
)

const tmuxRetryAttempts = 20

var (
	tmuxOutputRunner      = tmuxOutput
	defaultTmuxRetrySleep = func() { time.Sleep(500 * time.Millisecond) }
	tmuxRetrySleep        = defaultTmuxRetrySleep
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
	case "install":
		if err := runInstall(args); err != nil {
			exitIfErr(err, "codex install")
		}
	case "build":
		if err := runBuild(args); err != nil {
			exitIfErr(err, "codex build")
		}
	case "format":
		if err := runFormat(args); err != nil {
			exitIfErr(err, "codex format")
		}
	case "test":
		if err := runTest(args); err != nil {
			exitIfErr(err, "codex test")
		}
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
	reasoning := opts.String("reasoning", "medium", "Codex reasoning effort to request")
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
		trimPaneStartupScrollback(target)
		fmt.Printf("started codex in tmux pane %s\n", target)
		return nil
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
	trimPaneStartupScrollback(target)
	fmt.Printf("started codex in tmux session %s via %s\n", *session, target)
	return nil
}

func runStatus(argv []string) error {
	opts := flag.NewFlagSet("codex v1 status", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name on the default tmux server")
	if err := opts.Parse(argv); err != nil {
		return err
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

func buildStartCommand(repoRoot, shellName, reasoning, model string) string {
	codexCmd := buildCodexExecCommand(strings.TrimSpace(model), strings.TrimSpace(reasoning))
	inner := fmt.Sprintf(
		"clear; printf 'Starting Codex CLI with %s (requested reasoning: %s) and skipping confirmations...\\n'; %s",
		shellSingleQuoteLiteral(strings.TrimSpace(model)),
		shellSingleQuoteLiteral(strings.TrimSpace(reasoning)),
		codexCmd,
	)
	return fmt.Sprintf(
		"cd %s && exec env DIALTONE_NIX_SHELL_BANNER=0 nix --extra-experimental-features %s --no-warn-dirty develop %s --command bash -lc %s",
		shellQuote(repoRoot),
		shellQuote("nix-command flakes"),
		shellQuote(".#"+strings.TrimSpace(shellName)),
		shellQuote(inner),
	)
}

func buildCodexExecCommand(model, reasoning string) string {
	effort := strings.TrimSpace(reasoning)
	if effort == "" {
		effort = "medium"
	}
	args := strings.Join([]string{
		"-c", shellQuote("check_for_update_on_startup=false"),
		"-c", shellQuote(fmt.Sprintf("model_reasoning_effort=%q", effort)),
		"-c", shellQuote(fmt.Sprintf("plan_mode_reasoning_effort=%q", effort)),
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

func normalizePaneTarget(value string) string {
	trimmed := strings.TrimSpace(value)
	parts := strings.Split(trimmed, ":")
	if len(parts) == 3 {
		return fmt.Sprintf("%s:%s.%s", parts[0], parts[1], parts[2])
	}
	return trimmed
}

func clearPaneHistory(target string) error {
	if _, err := tmuxOutputWithRetry("clear-history", "-t", target); err != nil {
		return err
	}
	return nil
}

func respawnPane(target, command string) error {
	// A stale pane history is cosmetic. The restart itself is what must succeed.
	_ = clearPaneHistory(target)
	if _, err := tmuxOutputWithRetry("respawn-pane", "-k", "-t", target, "bash", "-lc", command); err != nil {
		return err
	}
	return nil
}

func trimPaneStartupScrollback(target string) {
	time.Sleep(750 * time.Millisecond)
	_ = clearPaneHistory(target)
}

func tmuxOutput(args ...string) (string, error) {
	repoRoot, err := locateRepoRoot()
	if err != nil {
		repoRoot = ""
	}
	cmd := tmuxcmd.Command(repoRoot, args...)
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
	fmt.Println("  install")
	fmt.Println("       Verify tmux plus codex or npx are available in the default nix shell")
	fmt.Println("  build")
	fmt.Println("       Build the codex v1 CLI wrapper to <repo-root>/bin/mods/codex/v1/codex")
	fmt.Println("  format [--dir DIR]")
	fmt.Println("       Run gofmt on codex v1 Go files")
	fmt.Println("  test")
	fmt.Println("       Run go test for codex v1 plus direct-routing helpers")
	fmt.Println("  start [--session codex-view] [--shell default|ssh-v1] [--reasoning medium] [--model gpt-5.4]")
	fmt.Println("       Launch Codex directly in the target tmux pane via nix develop; shell v1 owns the broader workflow")
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
