package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
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
			exitIfErr(err, "shell start")
		}
	case "split-vertical", "split-verticle":
		if err := runSplitVertical(args); err != nil {
			exitIfErr(err, "shell split-vertical")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown shell command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runStart(argv []string) error {
	opts := flag.NewFlagSet("shell v1 start", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name to start or attach")
	shellName := opts.String("shell", "default", "flake shell to use before launching Codex")
	reasoning := opts.String("reasoning", "medium", "reasoning label for the Codex startup banner")
	model := opts.String("model", "gpt-5.4", "Codex model to launch")
	waitSeconds := opts.Int("wait-seconds", 20, "Seconds to wait for the tmux pane to become readable")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("start does not accept positional arguments")
	}
	if strings.TrimSpace(*session) == "" {
		return errors.New("--session is required")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}

	startTmuxCmd := fmt.Sprintf("tmux new-session -A -s %s", strings.TrimSpace(*session))
	if _, err := runDialtoneMod(repoRoot,
		"ghostty", "v1", "fresh-window",
		"--cwd", repoRoot,
	); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot,
		"ghostty", "v1", "write",
		"--terminal", "1",
		"--focus",
		startTmuxCmd,
	); err != nil {
		return err
	}

	pane := fmt.Sprintf("%s:0:0", strings.TrimSpace(*session))
	if err := waitForPane(repoRoot, pane, time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}

	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "target", "--set", pane); err != nil {
		return err
	}

	if _, err := runDialtoneMod(repoRoot,
		"codex", "v1", "start",
		"--session", strings.TrimSpace(*session),
		"--shell", strings.TrimSpace(*shellName),
		"--reasoning", strings.TrimSpace(*reasoning),
		"--model", strings.TrimSpace(*model),
	); err != nil {
		return err
	}

	fmt.Printf("started shell workflow: ghostty one-window/one-tab -> %s -> codex %s\n",
		pane,
		strings.TrimSpace(*model),
	)
	return nil
}

func runSplitVertical(argv []string) error {
	opts := flag.NewFlagSet("shell v1 split-vertical", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name whose primary pane should remain Codex on the left")
	shellName := opts.String("shell", "default", "flake shell to use in the right-side dialtone pane")
	rightTitle := opts.String("right-title", "dialtone-view", "Title for the right-side tmux pane")
	waitSeconds := opts.Int("wait-seconds", 10, "Seconds to wait for the new tmux pane to become readable")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("split-vertical does not accept positional arguments")
	}
	if strings.TrimSpace(*session) == "" {
		return errors.New("--session is required")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}

	leftPane := fmt.Sprintf("%s:0:0", strings.TrimSpace(*session))
	rightPane := fmt.Sprintf("%s:0:1", strings.TrimSpace(*session))

	if !paneExists(repoRoot, rightPane) {
		if _, err := runDialtoneMod(repoRoot,
			"tmux", "v1", "split",
			"--pane", leftPane,
			"--direction", "right",
			"--title", strings.TrimSpace(*rightTitle),
			"--cwd", repoRoot,
		); err != nil {
			return err
		}
	}
	if err := waitForPane(repoRoot, rightPane, time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot,
		"tmux", "v1", "shell",
		"--pane", rightPane,
		"--shell", strings.TrimSpace(*shellName),
	); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "clear", "--pane", rightPane); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "target", "--set", leftPane); err != nil {
		return err
	}

	fmt.Printf("split shell workflow: codex on %s (left), %s on %s (right)\n",
		leftPane,
		strings.TrimSpace(*rightTitle),
		rightPane,
	)
	return nil
}

func waitForPane(repoRoot, pane string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", pane, "--lines", "1"); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr != nil {
		return fmt.Errorf("timed out waiting for tmux pane %s: %w", pane, lastErr)
	}
	return fmt.Errorf("timed out waiting for tmux pane %s", pane)
}

func paneExists(repoRoot, pane string) bool {
	_, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", pane, "--lines", "1")
	return err == nil
}

func runDialtoneMod(repoRoot string, args ...string) (string, error) {
	output, err := runDialtoneModQuiet(repoRoot, args...)
	if strings.TrimSpace(output) != "" {
		fmt.Print(output)
		if !strings.HasSuffix(output, "\n") {
			fmt.Println()
		}
	}
	return output, err
}

func runDialtoneModQuiet(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone_mod"), args...)
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		text := strings.TrimSpace(stdout.String())
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		if text != "" {
			errText = strings.TrimSpace(text + "\n" + errText)
		}
		return stdout.String(), fmt.Errorf("./dialtone_mod %s failed: %s", strings.Join(args, " "), errText)
	}
	return stdout.String(), nil
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

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod shell v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start [--session codex-view] [--shell default|repl-v1|ssh-v1] [--reasoning medium] [--model gpt-5.4] [--wait-seconds 20]")
	fmt.Println("       Quit Ghostty, create one fresh window with one tab, start/attach codex-view, set the tmux target, and launch Codex")
	fmt.Println("  split-vertical [--session codex-view] [--shell default|repl-v1|ssh-v1] [--right-title dialtone-view] [--wait-seconds 10]")
	fmt.Println("       Split the tmux window so Codex stays on the left and a dialtone shell pane titled dialtone-view appears on the right")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
