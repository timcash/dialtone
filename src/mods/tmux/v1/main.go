package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type tmuxPaneTarget struct {
	Session string
	Window  string
	Pane    string
}

func (t tmuxPaneTarget) paneRef() string {
	return fmt.Sprintf("%s.%s", t.Window, t.Pane)
}

func (t tmuxPaneTarget) target() string {
	return fmt.Sprintf("%s:%s", t.Session, t.paneRef())
}

func parsePaneTarget(raw string) (tmuxPaneTarget, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = "dialtone:0:0"
	}
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return tmuxPaneTarget{}, fmt.Errorf("invalid --pane %q (expected session:window:pane)", raw)
	}
	session := strings.TrimSpace(parts[0])
	window := strings.TrimSpace(parts[1])
	pane := strings.TrimSpace(parts[2])
	if session == "" || window == "" || pane == "" {
		return tmuxPaneTarget{}, fmt.Errorf("invalid --pane %q (expected session:window:pane)", raw)
	}
	return tmuxPaneTarget{Session: session, Window: window, Pane: pane}, nil
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
	case "list":
		if err := runList(args); err != nil {
			exitIfErr(err, "tmux list")
		}
	case "read":
		if err := runRead(args); err != nil {
			exitIfErr(err, "tmux read")
		}
	case "write":
		if err := runWrite(args); err != nil {
			exitIfErr(err, "tmux write")
		}
	case "inject":
		if err := runWrite(args); err != nil {
			exitIfErr(err, "tmux write")
		}
	case "rename":
		if err := runRename(args); err != nil {
			exitIfErr(err, "tmux rename")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown tmux command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runList(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 list", flag.ContinueOnError)
	short := opts.Bool("short", false, "Print only session names")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	format := "#{session_name}:#{session_windows}:#{session_attached}"
	if *short {
		format = "#{session_name}"
	}
	cmd := exec.Command("tmux", "list-sessions", "-F", format)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "no server running") || strings.Contains(lower, "no sessions") {
			fmt.Println("no tmux sessions")
			return nil
		}
		return fmt.Errorf("tmux list-sessions failed: %s", strings.TrimSpace(string(out)))
	}
	if text == "" {
		fmt.Println("no tmux sessions")
		return nil
	}
	fmt.Println(text)
	return nil
}

func runWrite(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 write", flag.ContinueOnError)
	pane := opts.String("pane", "dialtone:0:0", "tmux target in session:window:pane form")
	enter := opts.Bool("enter", false, "Press Enter after writing")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() == 0 {
		return errors.New("write requires text to send")
	}
	text := strings.TrimSpace(strings.Join(opts.Args(), " "))
	if text == "" {
		return errors.New("write text is empty")
	}
	target, err := parsePaneTarget(*pane)
	if err != nil {
		return err
	}

	tmuxTarget := target.target()
	if out, err := exec.Command("tmux", "send-keys", "-t", tmuxTarget, "--", text).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys failed: %s", strings.TrimSpace(string(out)))
	}
	if *enter {
		if out, err := exec.Command("tmux", "send-keys", "-t", tmuxTarget, "C-m").CombinedOutput(); err != nil {
			return fmt.Errorf("tmux send-keys enter failed: %s", strings.TrimSpace(string(out)))
		}
	}
	fmt.Printf("wrote to %s: %s\n", tmuxTarget, text)
	return nil
}

func runRead(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 read", flag.ContinueOnError)
	pane := opts.String("pane", "dialtone:0:0", "tmux target in session:window:pane form")
	lines := opts.Int("lines", 10, "Number of trailing lines to read")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if *lines <= 0 {
		return errors.New("--lines must be a positive integer")
	}
	target, err := parsePaneTarget(*pane)
	if err != nil {
		return err
	}

	cmd := exec.Command("tmux", "capture-pane", "-pt", target.target(), "-S", fmt.Sprintf("-%d", *lines))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux capture-pane failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Print(string(out))
	return nil
}

func runRename(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 rename", flag.ContinueOnError)
	session := opts.String("session", "", "Existing tmux session name (default: current or first)")
	to := opts.String("to", "dialtone", "New tmux session name")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	resolveSession := strings.TrimSpace(*session)
	if resolveSession == "" {
		out, _ := exec.Command("tmux", "display-message", "-p", "#S").Output()
		resolveSession = strings.TrimSpace(string(out))
	}
	if resolveSession == "" {
		out, _ := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
		resolveSession = strings.TrimSpace(strings.Split(strings.TrimSpace(string(out)), "\n")[0])
	}
	if resolveSession == "" {
		return errors.New("no tmux sessions")
	}
	newName := strings.TrimSpace(*to)
	if newName == "" {
		return errors.New("--to is required")
	}
	if resolveSession == newName {
		fmt.Printf("tmux session already named %s\n", newName)
		return nil
	}
	cmd := exec.Command("tmux", "rename-session", "-t", resolveSession, newName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux rename-session failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("renamed tmux session: %s -> %s\n", resolveSession, newName)
	return nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod tmux v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list [--short]")
	fmt.Println("       List local tmux sessions")
	fmt.Println("  write [--pane dialtone:0:0] [--enter] <text...>")
	fmt.Println("       Write text to a tmux pane (default target: dialtone:0:0)")
	fmt.Println("  read [--pane dialtone:0:0] [--lines 10]")
	fmt.Println("       Read trailing lines from a tmux pane (default: 10)")
	fmt.Println("  rename [--session NAME] [--to dialtone]")
	fmt.Println("       Rename tmux session (defaults to current/first session)")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
