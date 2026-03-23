package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
)

func tmuxBinary() string {
	if value := strings.TrimSpace(os.Getenv("DIALTONE_TMUX_BIN")); value != "" {
		return value
	}
	return "tmux"
}

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
	case "clear":
		if err := runClear(args); err != nil {
			exitIfErr(err, "tmux clear")
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
	case "split":
		if err := runSplit(args); err != nil {
			exitIfErr(err, "tmux split")
		}
	case "shell":
		if err := runShell(args); err != nil {
			exitIfErr(err, "tmux shell")
		}
	case "target":
		if err := runTarget(args); err != nil {
			exitIfErr(err, "tmux target")
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
	cmd := exec.Command(tmuxBinary(), "list-sessions", "-F", format)
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
	if out, err := exec.Command(tmuxBinary(), "send-keys", "-t", tmuxTarget, "--", text).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys failed: %s", strings.TrimSpace(string(out)))
	}
	if *enter {
		// Raw terminal UIs such as Codex can miss an immediate Enter after a large paste.
		time.Sleep(300 * time.Millisecond)
		if out, err := exec.Command(tmuxBinary(), "send-keys", "-t", tmuxTarget, "C-m").CombinedOutput(); err != nil {
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

	cmd := exec.Command(tmuxBinary(), "capture-pane", "-pt", target.target(), "-S", fmt.Sprintf("-%d", *lines))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux capture-pane failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Print(string(out))
	return nil
}

func runClear(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 clear", flag.ContinueOnError)
	pane := opts.String("pane", "dialtone:0:0", "tmux target in session:window:pane form")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("clear does not accept positional arguments")
	}
	target, err := parsePaneTarget(*pane)
	if err != nil {
		return err
	}

	tmuxTarget := target.target()
	if out, err := exec.Command(tmuxBinary(), "clear-history", "-t", tmuxTarget).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux clear-history failed: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command(tmuxBinary(), "send-keys", "-t", tmuxTarget, "C-l").CombinedOutput(); err != nil {
		return fmt.Errorf("tmux send-keys clear failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("cleared tmux pane %s\n", tmuxTarget)
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
		out, _ := exec.Command(tmuxBinary(), "display-message", "-p", "#S").Output()
		resolveSession = strings.TrimSpace(string(out))
	}
	if resolveSession == "" {
		out, _ := exec.Command(tmuxBinary(), "list-sessions", "-F", "#{session_name}").Output()
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
	cmd := exec.Command(tmuxBinary(), "rename-session", "-t", resolveSession, newName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux rename-session failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("renamed tmux session: %s -> %s\n", resolveSession, newName)
	return nil
}

func runSplit(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 split", flag.ContinueOnError)
	pane := opts.String("pane", "dialtone:0:0", "tmux target in session:window:pane form")
	direction := opts.String("direction", "right", "Split direction: right|left|down|up")
	title := opts.String("title", "", "Optional title for the new pane")
	cwd := opts.String("cwd", "", "Working directory for the new pane")
	command := opts.String("command", "", "Optional command to run in the new pane")
	focus := opts.Bool("focus", true, "Focus the new pane after splitting")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("split does not accept positional arguments")
	}

	target, err := parsePaneTarget(*pane)
	if err != nil {
		return err
	}

	tmuxArgs := []string{
		"split-window",
		"-P",
		"-F", "#{session_name}:#{window_index}:#{pane_index}",
		"-t", target.target(),
	}
	switch strings.TrimSpace(*direction) {
	case "right":
		tmuxArgs = append(tmuxArgs, "-h")
	case "left":
		tmuxArgs = append(tmuxArgs, "-h", "-b")
	case "down":
		tmuxArgs = append(tmuxArgs, "-v")
	case "up":
		tmuxArgs = append(tmuxArgs, "-v", "-b")
	default:
		return fmt.Errorf("invalid --direction %q (expected right|left|down|up)", *direction)
	}
	if strings.TrimSpace(*cwd) != "" {
		tmuxArgs = append(tmuxArgs, "-c", strings.TrimSpace(*cwd))
	}
	if strings.TrimSpace(*command) != "" {
		tmuxArgs = append(tmuxArgs, strings.TrimSpace(*command))
	}

	out, err := exec.Command(tmuxBinary(), tmuxArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tmux split-window failed: %s", strings.TrimSpace(string(out)))
	}
	newPane := strings.TrimSpace(string(out))
	newTarget, err := parsePaneTarget(newPane)
	if err != nil {
		return fmt.Errorf("invalid tmux split result %q: %w", newPane, err)
	}

	if strings.TrimSpace(*title) != "" {
		if out, err := exec.Command(tmuxBinary(), "select-pane", "-t", newTarget.target(), "-T", strings.TrimSpace(*title)).CombinedOutput(); err != nil {
			return fmt.Errorf("tmux select-pane title failed: %s", strings.TrimSpace(string(out)))
		}
	}
	if !*focus {
		if out, err := exec.Command(tmuxBinary(), "select-pane", "-t", target.target()).CombinedOutput(); err != nil {
			return fmt.Errorf("tmux select-pane restore failed: %s", strings.TrimSpace(string(out)))
		}
	}

	fmt.Printf("split %s %s -> %s\n", target.target(), strings.TrimSpace(*direction), newTarget.target())
	return nil
}

func runShell(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 shell", flag.ContinueOnError)
	pane := opts.String("pane", "dialtone:0:0", "tmux target in session:window:pane form")
	shellName := opts.String("shell", "default", "flake shell to enter (default|repl-v1|ssh-v1)")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	target, err := parsePaneTarget(*pane)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	switch strings.TrimSpace(*shellName) {
	case "default", "repl-v1", "ssh-v1":
	default:
		return fmt.Errorf("unsupported --shell %q", *shellName)
	}

	tmuxTarget := target.target()
	command := fmt.Sprintf(
		"cd %s && nix --extra-experimental-features %s develop %s --command zsh -l",
		shellQuote(repoRoot),
		shellQuote("nix-command flakes"),
		shellQuote(".#"+strings.TrimSpace(*shellName)),
	)
	if out, err := exec.Command(tmuxBinary(), "clear-history", "-t", tmuxTarget).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux clear-history failed: %s", strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command(tmuxBinary(), "respawn-pane", "-k", "-t", tmuxTarget, "zsh", "-lc", command).CombinedOutput(); err != nil {
		return fmt.Errorf("tmux respawn-pane shell failed: %s", strings.TrimSpace(string(out)))
	}
	fmt.Printf("entered nix shell %s in %s\n", strings.TrimSpace(*shellName), tmuxTarget)
	return nil
}

func runTarget(argv []string) error {
	opts := flag.NewFlagSet("tmux v1 target", flag.ContinueOnError)
	setPane := opts.String("set", "", "Persist the dialtone_mod tmux target as session:window:pane")
	setPromptPane := opts.String("set-prompt", "", "Persist the codex prompt tmux target as session:window:pane")
	clear := opts.Bool("clear", false, "Clear the persisted dialtone_mod tmux target")
	clearPrompt := opts.Bool("clear-prompt", false, "Clear the persisted codex prompt tmux target")
	showPrompt := opts.Bool("prompt", false, "Print the persisted codex prompt tmux target")
	showAll := opts.Bool("all", false, "Print both command and prompt tmux targets")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}

	if *clear && *clearPrompt {
		if err := clearPersistedTarget(repoRoot); err != nil {
			return err
		}
		if err := clearPersistedPromptTarget(repoRoot); err != nil {
			return err
		}
		fmt.Println("cleared dialtone_mod tmux command target")
		fmt.Println("cleared dialtone_mod tmux prompt target")
		return nil
	}
	if *clear {
		if err := clearPersistedTarget(repoRoot); err != nil {
			return err
		}
		fmt.Println("cleared dialtone_mod tmux command target")
		return nil
	}
	if *clearPrompt {
		if err := clearPersistedPromptTarget(repoRoot); err != nil {
			return err
		}
		fmt.Println("cleared dialtone_mod tmux prompt target")
		return nil
	}

	if strings.TrimSpace(*setPane) != "" {
		target, err := parsePaneTarget(*setPane)
		if err != nil {
			return err
		}
		value := target.Session + ":" + target.Window + ":" + target.Pane
		if err := storePersistedTarget(repoRoot, value); err != nil {
			return err
		}
		fmt.Printf("set dialtone_mod tmux command target: %s\n", value)
		return nil
	}
	if strings.TrimSpace(*setPromptPane) != "" {
		target, err := parsePaneTarget(*setPromptPane)
		if err != nil {
			return err
		}
		value := target.Session + ":" + target.Window + ":" + target.Pane
		if err := storePersistedPromptTarget(repoRoot, value); err != nil {
			return err
		}
		fmt.Printf("set dialtone_mod tmux prompt target: %s\n", value)
		return nil
	}

	if *showAll {
		commandValue, commandErr := loadPersistedTarget(repoRoot)
		promptValue, promptErr := loadPersistedPromptTarget(repoRoot)
		if commandErr != nil && !errors.Is(commandErr, os.ErrNotExist) {
			return commandErr
		}
		if promptErr != nil && !errors.Is(promptErr, os.ErrNotExist) {
			return promptErr
		}
		if commandValue == "" {
			commandValue = "(unset)"
		}
		if promptValue == "" {
			promptValue = "(unset)"
		}
		fmt.Printf("command\t%s\nprompt\t%s\n", commandValue, promptValue)
		return nil
	}
	if *showPrompt {
		raw, err := loadPersistedPromptTarget(repoRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Println("no dialtone_mod tmux prompt target configured")
				return nil
			}
			return err
		}
		fmt.Print(strings.TrimSpace(raw))
		fmt.Println()
		return nil
	}

	raw, err := loadPersistedTarget(repoRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("no dialtone_mod tmux command target configured")
			return nil
		}
		return err
	}
	fmt.Print(strings.TrimSpace(string(raw)))
	fmt.Println()
	return nil
}

func loadPersistedTarget(repoRoot string) (string, error) {
	value, ok, err := loadPersistedTargetFromState(repoRoot)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", os.ErrNotExist
	}
	return value, nil
}

func storePersistedTarget(repoRoot, target string) error {
	return storeStateTarget(repoRoot, sqlitestate.TmuxTargetKey, target)
}

func clearPersistedTarget(repoRoot string) error {
	return clearStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
}

func loadPersistedPromptTarget(repoRoot string) (string, error) {
	value, ok, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", os.ErrNotExist
	}
	return value, nil
}

func storePersistedPromptTarget(repoRoot, target string) error {
	return storeStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey, target)
}

func clearPersistedPromptTarget(repoRoot string) error {
	return clearStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
}

func loadPersistedTargetFromState(repoRoot string) (string, bool, error) {
	return loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
}

func loadStateTarget(repoRoot, key string) (string, bool, error) {
	var value string
	var found bool
	err := withStateDB(repoRoot, func(db *sql.DB) error {
		record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
		if err != nil {
			return err
		}
		value = strings.TrimSpace(record.Value)
		found = ok && value != ""
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return value, found, nil
}

func storeStateTarget(repoRoot, key, target string) error {
	value := strings.TrimSpace(target)
	if value == "" {
		return errors.New("target value is required")
	}
	if err := withStateDB(repoRoot, func(db *sql.DB) error {
		return modstate.UpsertStateValue(db, sqlitestate.SystemScope, key, value)
	}); err != nil {
		return err
	}
	return nil
}

func clearStateTarget(repoRoot, key string) error {
	if err := withStateDB(repoRoot, func(db *sql.DB) error {
		return modstate.DeleteStateValue(db, sqlitestate.SystemScope, key)
	}); err != nil {
		return err
	}
	return nil
}

func withStateDB(repoRoot string, fn func(*sql.DB) error) error {
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
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
	fmt.Println("  clear [--pane codex-view:0:0]")
	fmt.Println("       Clear tmux history and redraw the target pane")
	fmt.Println("  rename [--session NAME] [--to dialtone]")
	fmt.Println("       Rename tmux session (defaults to current/first session)")
	fmt.Println("  split [--pane codex-view:0:0] [--direction right|left|down|up] [--title NAME] [--cwd PATH] [--command CMD] [--focus=true|false]")
	fmt.Println("       Split a tmux pane and optionally title or initialize the new pane")
	fmt.Println("  shell [--pane codex-view:1:1] [--shell default|repl-v1|ssh-v1]")
	fmt.Println("       Put the target tmux pane into the repo nix develop shell without starting Codex")
	fmt.Println("  target [--set codex-view:1:1] [--clear]")
	fmt.Println("       Persist or clear the default tmux pane that dialtone_mod should proxy non-control commands into")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
