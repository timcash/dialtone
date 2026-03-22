package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
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
	case "prompt":
		if err := runPrompt(args); err != nil {
			exitIfErr(err, "shell prompt")
		}
	case "enqueue-command":
		if err := runEnqueueCommand(args); err != nil {
			exitIfErr(err, "shell enqueue-command")
		}
	case "run":
		if err := runRun(args); err != nil {
			exitIfErr(err, "shell run")
		}
	case "test":
		if err := runTest(args); err != nil {
			exitIfErr(err, "shell test")
		}
	case "test-basic":
		if err := runTestBasic(args); err != nil {
			exitIfErr(err, "shell test-basic")
		}
	case "test-all":
		if err := runTestAll(args); err != nil {
			exitIfErr(err, "shell test-all")
		}
	case "workflow":
		if err := runWorkflow(args); err != nil {
			exitIfErr(err, "shell workflow")
		}
	case "sync-once":
		if err := runSyncOnce(args); err != nil {
			exitIfErr(err, "shell sync-once")
		}
	case "state":
		if err := runState(args); err != nil {
			exitIfErr(err, "shell state")
		}
	case "read":
		if err := runRead(args); err != nil {
			exitIfErr(err, "shell read")
		}
	case "events":
		if err := runEvents(args); err != nil {
			exitIfErr(err, "shell events")
		}
	case "demo-protocol":
		if err := runDemoProtocol(args); err != nil {
			exitIfErr(err, "shell demo-protocol")
		}
	case "supervise":
		if err := runSupervise(args); err != nil {
			exitIfErr(err, "shell supervise")
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
	dialtoneShell := opts.String("dialtone-shell", defaultDialtoneShellName(), "flake shell to keep active in the right-side dialtone pane")
	reasoning := opts.String("reasoning", "medium", "reasoning label for the Codex startup banner")
	model := opts.String("model", "gpt-5.4", "Codex model to launch")
	rightTitle := opts.String("right-title", "dialtone-view", "Title for the right-side tmux pane")
	waitSeconds := opts.Int("wait-seconds", 20, "Seconds to wait for the tmux pane to become readable")
	runTests := opts.Bool("run-tests", true, "Run the sqlite-driven mods test plan in the right-side dialtone pane")
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
	rightPane := fmt.Sprintf("%s:0:1", strings.TrimSpace(*session))
	if err := waitForPane(repoRoot, pane, time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}

	if err := ensureSplitLayout(repoRoot, strings.TrimSpace(*session), strings.TrimSpace(*dialtoneShell), strings.TrimSpace(*rightTitle), time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot,
		"codex", "v1", "start",
		"--session", strings.TrimSpace(*session),
		"--pane", pane,
		"--shell", strings.TrimSpace(*shellName),
		"--reasoning", strings.TrimSpace(*reasoning),
		"--model", strings.TrimSpace(*model),
	); err != nil {
		return err
	}
	if *runTests {
		if _, err := runDialtoneMod(repoRoot,
			"tmux", "v1", "write",
			"--pane", rightPane,
			"--enter",
			fmt.Sprintf("cd %s && env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db test-run", repoRoot),
		); err != nil {
			return err
		}
	}

	fmt.Printf("started shell workflow: ghostty one-window/one-tab -> %s (codex) + %s (%s) -> codex %s\n",
		pane,
		rightPane,
		strings.TrimSpace(*rightTitle),
		strings.TrimSpace(*model),
	)
	return nil
}

func runSplitVertical(argv []string) error {
	opts := flag.NewFlagSet("shell v1 split-vertical", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name whose primary pane should remain Codex on the left")
	shellName := opts.String("shell", defaultDialtoneShellName(), "flake shell to use in the right-side dialtone pane")
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
	return runSplitVerticalWithOptions(strings.TrimSpace(*session), strings.TrimSpace(*shellName), strings.TrimSpace(*rightTitle), time.Duration(*waitSeconds)*time.Second)
}

func runSplitVerticalWithOptions(session, shellName, rightTitle string, wait time.Duration) error {
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	if err := ensureSplitLayout(repoRoot, session, shellName, rightTitle, wait); err != nil {
		return err
	}
	fmt.Printf("split shell workflow: codex on %s:0:0 (left), %s on %s:0:1 (right)\n",
		session,
		rightTitle,
		session,
	)
	return nil
}

func ensureSplitLayout(repoRoot, session, shellName, rightTitle string, wait time.Duration) error {
	leftPane := fmt.Sprintf("%s:0:0", strings.TrimSpace(session))
	rightPane := fmt.Sprintf("%s:0:1", strings.TrimSpace(session))
	if !paneExists(repoRoot, rightPane) {
		if _, err := runDialtoneMod(repoRoot,
			"tmux", "v1", "split",
			"--pane", leftPane,
			"--direction", "right",
			"--title", strings.TrimSpace(rightTitle),
			"--cwd", repoRoot,
		); err != nil {
			return err
		}
	}
	if err := waitForPane(repoRoot, rightPane, wait); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot,
		"tmux", "v1", "shell",
		"--pane", rightPane,
		"--shell", strings.TrimSpace(shellName),
	); err != nil {
		return err
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "clear", "--pane", rightPane); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "target", "--set", rightPane); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "target", "--set-prompt", leftPane); err != nil {
		return err
	}
	return nil
}

func runPrompt(argv []string) error {
	opts := flag.NewFlagSet("shell v1 prompt", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "logical session name to associate with the prompt")
	pane := opts.String("pane", "", "Optional explicit pane override to store on the queued prompt row")
	syncNow := opts.Bool("sync", true, "Immediately reconcile this prompt into the live tmux pane")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() == 0 {
		return errors.New("prompt requires text to submit")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	text := strings.TrimSpace(strings.Join(opts.Args(), " "))
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	body, err := json.Marshal(shellBusIntentBody{Text: text})
	if err != nil {
		return err
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "prompt", "submit", "shell-cli", strings.TrimSpace(*session), strings.TrimSpace(*pane), string(body))
	if err != nil {
		return err
	}
	if *syncNow {
		row, ok, err := modstate.LoadShellBusRecord(db, rowID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("shell bus row not found after enqueue: %d", rowID)
		}
		if err := syncShellBusRow(db, repoRoot, row, 3*time.Second); err != nil {
			return err
		}
		_ = captureCurrentShellState(db, repoRoot, rowID)
		fmt.Printf("submitted prompt via shell bus [row_id=%d]\n", rowID)
		return nil
	}
	fmt.Printf("queued prompt on shell bus [row_id=%d]\n", rowID)
	return nil
}

func runEnqueueCommand(argv []string) error {
	opts := flag.NewFlagSet("shell v1 enqueue-command", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "logical session name to associate with the command")
	pane := opts.String("pane", "", "Optional explicit pane override to store on the queued command row")
	expect := opts.String("expect", "", "Optional text that sync-once should wait for in the target pane")
	syncNow := opts.Bool("sync", true, "Immediately reconcile this command into the live tmux pane")
	waitSeconds := opts.Int("wait-seconds", 10, "Seconds to wait for expected command output when --sync=true")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() == 0 {
		return errors.New("enqueue-command requires command text")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	body, err := json.Marshal(shellBusIntentBody{
		Command: strings.TrimSpace(strings.Join(opts.Args(), " ")),
		Expect:  strings.TrimSpace(*expect),
	})
	if err != nil {
		return err
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "shell-cli", strings.TrimSpace(*session), strings.TrimSpace(*pane), string(body))
	if err != nil {
		return err
	}
	if *syncNow {
		row, ok, err := modstate.LoadShellBusRecord(db, rowID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("shell bus row not found after enqueue: %d", rowID)
		}
		if err := syncShellBusRow(db, repoRoot, row, time.Duration(*waitSeconds)*time.Second); err != nil {
			return err
		}
		_ = captureCurrentShellState(db, repoRoot, rowID)
		fmt.Printf("ran command via shell bus [row_id=%d]\n", rowID)
		return nil
	}
	fmt.Printf("queued command on shell bus [row_id=%d]\n", rowID)
	return nil
}

func runRun(argv []string) error {
	opts := flag.NewFlagSet("shell v1 run", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "logical session name to associate with the command")
	pane := opts.String("pane", "", "Optional explicit pane override to store on the queued command row")
	expect := opts.String("expect", "", "Optional text that sync-once should wait for in the target pane")
	waitSeconds := opts.Int("wait-seconds", 10, "Seconds to wait for expected command output")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() == 0 {
		return errors.New("run requires command text")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	body, err := json.Marshal(shellBusIntentBody{
		Command: strings.TrimSpace(strings.Join(opts.Args(), " ")),
		Expect:  strings.TrimSpace(*expect),
	})
	if err != nil {
		return err
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "shell-cli", strings.TrimSpace(*session), strings.TrimSpace(*pane), string(body))
	if err != nil {
		return err
	}
	row, ok, err := modstate.LoadShellBusRecord(db, rowID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("shell bus row not found after enqueue: %d", rowID)
	}
	if err := syncShellBusRow(db, repoRoot, row, time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}
	_ = captureCurrentShellState(db, repoRoot, 0)
	fmt.Printf("ran command via shell bus [row_id=%d]\n", rowID)
	return nil
}

func runTest(argv []string) error {
	opts := flag.NewFlagSet("shell v1 test", flag.ContinueOnError)
	waitSeconds := opts.Int("wait-seconds", 20, "Seconds to wait for the go test result in dialtone-view")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("test does not accept positional arguments")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	command := buildGoTestCommand(repoRoot, "shell", "v1")
	expect := "ok  \tdialtone/dev/mods/shell/v1"
	return runRun([]string{
		"--wait-seconds", fmt.Sprintf("%d", *waitSeconds),
		"--expect", expect,
		command,
	})
}

func runTestBasic(argv []string) error {
	opts := flag.NewFlagSet("shell v1 test-basic", flag.ContinueOnError)
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("test-basic does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	return runRepoCommand(repoRoot, buildBasicTestCommand(repoRoot))
}

func runTestAll(argv []string) error {
	opts := flag.NewFlagSet("shell v1 test-all", flag.ContinueOnError)
	waitSeconds := opts.Int("wait-seconds", 120, "Seconds to wait for the full mod test sweep in dialtone-view")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("test-all does not accept positional arguments")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	command := buildAllModsTestCommand(repoRoot) + " && printf 'DIALTONE_TEST_ALL_DONE\\n'"
	return runRun([]string{
		"--wait-seconds", fmt.Sprintf("%d", *waitSeconds),
		"--expect", "DIALTONE_TEST_ALL_DONE",
		command,
	})
}

func runWorkflow(argv []string) error {
	opts := flag.NewFlagSet("shell v1 workflow", flag.ContinueOnError)
	waitSeconds := opts.Int("wait-seconds", 120, "Seconds to wait for the full mod test sweep in dialtone-view")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("workflow does not accept positional arguments")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}
	if err := runTestBasic(nil); err != nil {
		return err
	}
	if err := runStart([]string{"--run-tests=false"}); err != nil {
		return err
	}
	return runTestAll([]string{"--wait-seconds", fmt.Sprintf("%d", *waitSeconds)})
}

func buildGoTestCommand(repoRoot, modName, version string) string {
	root := strings.TrimSpace(repoRoot)
	mod := strings.TrimSpace(modName)
	ver := strings.TrimSpace(version)
	return fmt.Sprintf("clear && cd %s/src && go test ./mods/%s/%s/...", root, mod, ver)
}

func buildBasicTestCommand(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	return fmt.Sprintf("clear && cd %s/src && go test ./internal/modstate ./mods/shared/sqlitestate ./mods/mod/v1 ./mods/shell/v1/cli", root)
}

func buildAllModsTestCommand(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	return fmt.Sprintf("clear && cd %s/src && go test ./mods/...", root)
}

func defaultDialtoneShellName() string {
	return "default"
}

func runRepoCommand(repoRoot, command string) error {
	cmd := exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "develop", ".#default", "--command", "zsh", "-lc", command)
	cmd.Dir = strings.TrimSpace(repoRoot)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run repo command in nix shell: %w", err)
	}
	return nil
}

func runDemoProtocol(argv []string) error {
	opts := flag.NewFlagSet("shell v1 demo-protocol", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name to use for the protocol demo")
	shellName := opts.String("shell", "default", "flake shell for the right-side dialtone pane")
	dialtoneShell := opts.String("dialtone-shell", "ssh-v1", "flake shell for the right-side dialtone pane")
	reasoning := opts.String("reasoning", "medium", "reasoning label for the Codex startup banner")
	model := opts.String("model", "gpt-5.4", "Codex model to launch")
	rightTitle := opts.String("right-title", "dialtone-view", "Title for the right-side tmux pane")
	waitSeconds := opts.Int("wait-seconds", 20, "Seconds to wait for panes and expected output")
	bootstrap := opts.Bool("bootstrap", true, "Run the full shell bootstrap before the protocol demo")
	promptText := opts.String("prompt", "Protocol demo: the controller is recording this run in SQLite. A visible dialtone_mod command will run in dialtone-view while this prompt is shown in codex-view.", "Prompt to submit into codex-view")
	commandText := opts.String("command", "env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline", "Visible command to run in dialtone-view")
	expectText := opts.String("expect", "- shell:v1", "Substring expected in dialtone-view output to mark the demo successful")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("demo-protocol does not accept positional arguments")
	}
	if strings.TrimSpace(*session) == "" {
		return errors.New("--session is required")
	}
	if *waitSeconds <= 0 {
		return errors.New("--wait-seconds must be positive")
	}

	if *bootstrap {
		if err := runStart([]string{
			"--session", strings.TrimSpace(*session),
			"--shell", strings.TrimSpace(*shellName),
			"--dialtone-shell", strings.TrimSpace(*dialtoneShell),
			"--reasoning", strings.TrimSpace(*reasoning),
			"--model", strings.TrimSpace(*model),
			"--right-title", strings.TrimSpace(*rightTitle),
			"--wait-seconds", fmt.Sprintf("%d", *waitSeconds),
			"--run-tests=false",
		}); err != nil {
			return err
		}
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	promptTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return err
	}
	commandTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
	if err != nil {
		return err
	}
	runID, err := modstate.StartProtocolRun(db, "demo-protocol", strings.TrimSpace(*promptText), promptTarget, commandTarget)
	if err != nil {
		return err
	}
	eventIndex := 0
	appendEvent := func(eventType, queueName string, queueRowID int64, paneTarget, commandText, messageText string) error {
		eventIndex++
		return modstate.AppendProtocolEvent(db, modstate.ProtocolEventRecord{
			RunID:       runID,
			EventIndex:  eventIndex,
			EventType:   eventType,
			QueueName:   queueName,
			QueueRowID:  queueRowID,
			PaneTarget:  paneTarget,
			CommandText: commandText,
			MessageText: messageText,
		})
	}
	if err := appendEvent("workflow_ready", "", 0, "", "", fmt.Sprintf("prompt=%s command=%s", printableTarget(promptTarget), printableTarget(commandTarget))); err != nil {
		return err
	}
	queueID, err := submitPrompt(db, repoRoot, promptTarget, strings.TrimSpace(*promptText))
	if err != nil {
		_ = modstate.FinishProtocolRun(db, runID, "failed", "", err.Error())
		return err
	}
	if err := appendEvent("prompt_submitted", "prompts", queueID, promptTarget, strings.TrimSpace(*promptText), "submitted prompt to codex-view"); err != nil {
		return err
	}
	if err := writeVisibleCommand(repoRoot, commandTarget, strings.TrimSpace(*commandText)); err != nil {
		_ = modstate.FinishProtocolRun(db, runID, "failed", "", err.Error())
		return err
	}
	if err := appendEvent("command_written", "tmux", 0, commandTarget, strings.TrimSpace(*commandText), "wrote visible command to dialtone-view"); err != nil {
		return err
	}
	commandOutput, err := waitForPaneContains(repoRoot, commandTarget, strings.TrimSpace(*expectText), time.Duration(*waitSeconds)*time.Second)
	if err != nil {
		_ = appendEvent("command_wait_failed", "tmux", 0, commandTarget, strings.TrimSpace(*commandText), err.Error())
		_ = modstate.FinishProtocolRun(db, runID, "failed", commandOutput, err.Error())
		return err
	}
	if err := appendEvent("command_observed", "tmux", 0, commandTarget, strings.TrimSpace(*commandText), strings.TrimSpace(*expectText)); err != nil {
		return err
	}
	resultText := fmt.Sprintf("observed %q in %s", strings.TrimSpace(*expectText), commandTarget)
	if err := modstate.FinishProtocolRun(db, runID, "passed", resultText, ""); err != nil {
		return err
	}
	fmt.Printf("demo protocol run %d passed\n", runID)
	fmt.Printf("prompt_target\t%s\ncommand_target\t%s\n", printableTarget(promptTarget), printableTarget(commandTarget))
	fmt.Printf("command\t%s\n", strings.TrimSpace(*commandText))
	fmt.Printf("result\t%s\n", resultText)
	return nil
}

type shellBusIntentBody struct {
	Text    string `json:"text,omitempty"`
	Command string `json:"command,omitempty"`
	Expect  string `json:"expect,omitempty"`
	Target  string `json:"target,omitempty"`
	Summary string `json:"summary,omitempty"`
	Error   string `json:"error,omitempty"`
}

func runSyncOnce(argv []string) error {
	opts := flag.NewFlagSet("shell v1 sync-once", flag.ContinueOnError)
	limit := opts.Int("limit", 10, "Maximum queued shell bus rows to process")
	waitSeconds := opts.Int("wait-seconds", 10, "Seconds to wait for expected command output")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("sync-once does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	rows, err := modstate.LoadQueuedShellBus(db, *limit)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		if err := captureCurrentShellState(db, repoRoot, 0); err != nil {
			return err
		}
		fmt.Println("shell bus is idle; captured current shell state")
		return nil
	}
	for _, row := range rows {
		if err := syncShellBusRow(db, repoRoot, row, time.Duration(*waitSeconds)*time.Second); err != nil {
			return err
		}
	}
	if err := captureCurrentShellState(db, repoRoot, 0); err != nil {
		return err
	}
	fmt.Printf("processed %d shell bus row(s)\n", len(rows))
	return nil
}

func syncShellBusRow(db *sql.DB, repoRoot string, row modstate.ShellBusRecord, wait time.Duration) error {
	if err := modstate.UpdateShellBusStatus(db, row.ID, "running", row.RefID, row.BodyJSON); err != nil {
		return err
	}
	body := shellBusIntentBody{}
	if strings.TrimSpace(row.BodyJSON) != "" {
		if err := json.Unmarshal([]byte(row.BodyJSON), &body); err != nil {
			body.Error = err.Error()
		}
	}
	target, err := resolveShellBusTarget(repoRoot, row)
	if err != nil {
		body.Error = err.Error()
		updated, _ := json.Marshal(body)
		_ = modstate.UpdateShellBusStatus(db, row.ID, "failed", 0, string(updated))
		return err
	}
	body.Target = target

	switch {
	case row.Subject == "prompt" && row.Action == "submit":
		updated, _ := json.Marshal(body)
		if err := markShellBusRowRunning(db, row.ID, string(updated)); err != nil {
			return err
		}
		if _, err := runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
			"DIALTONE_TMUX_PROXY_ACTIVE": "1",
		}, "tmux", "v1", "write", "--pane", target, "--enter", body.Text); err != nil {
			body.Error = err.Error()
			updated, _ := json.Marshal(body)
			_ = modstate.UpdateShellBusStatus(db, row.ID, "failed", 0, string(updated))
			return err
		}
	case row.Subject == "command" && row.Action == "run":
		updated, _ := json.Marshal(body)
		if err := markShellBusRowRunning(db, row.ID, string(updated)); err != nil {
			return err
		}
		if _, err := runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
			"DIALTONE_TMUX_PROXY_ACTIVE": "1",
		}, "tmux", "v1", "write", "--pane", target, "--enter", body.Command); err != nil {
			body.Error = err.Error()
			updated, _ := json.Marshal(body)
			_ = modstate.UpdateShellBusStatus(db, row.ID, "failed", 0, string(updated))
			return err
		}
	default:
		return fmt.Errorf("unsupported shell bus action: %s/%s", row.Subject, row.Action)
	}

	snapshotText := ""
	if row.Subject == "command" && strings.TrimSpace(body.Expect) != "" {
		if out, err := waitForPaneContains(repoRoot, target, strings.TrimSpace(body.Expect), wait); err == nil {
			snapshotText = out
		} else {
			body.Error = err.Error()
			body.Summary = summarizeSnapshot(out)
			updated, _ := json.Marshal(body)
			_ = modstate.UpdateShellBusStatus(db, row.ID, "failed", 0, string(updated))
			return err
		}
	} else {
		time.Sleep(300 * time.Millisecond)
	}
	if snapshotText == "" {
		out, err := runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
			"DIALTONE_TMUX_PROXY_ACTIVE": "1",
		}, "tmux", "v1", "read", "--pane", target, "--lines", "120")
		if err == nil {
			snapshotText = out
		}
	}
	body.Summary = summarizeSnapshot(snapshotText)
	observedBody, _ := json.Marshal(map[string]string{
		"text":    snapshotText,
		"summary": body.Summary,
	})
	observedID, err := modstate.AppendShellBusObserved(db, "tmux", "pane", "snapshot", "shell-sync", row.Session, target, row.ID, string(observedBody))
	if err != nil {
		return err
	}
	updated, _ := json.Marshal(body)
	status := "done"
	if row.Subject == "command" {
		if exitCode, ok := trackedCommandExitStatus(snapshotText); ok && exitCode != 0 {
			status = "failed"
			body.Error = fmt.Sprintf("command exited with status %d", exitCode)
			updated, _ = json.Marshal(body)
		}
	}
	if err := modstate.UpdateShellBusStatus(db, row.ID, status, observedID, string(updated)); err != nil {
		return err
	}
	return nil
}

func markShellBusRowRunning(db *sql.DB, rowID int64, bodyJSON string) error {
	return modstate.UpdateShellBusStatus(db, rowID, "running", 0, bodyJSON)
}

func resolveShellBusTarget(repoRoot string, row modstate.ShellBusRecord) (string, error) {
	if strings.TrimSpace(row.Pane) != "" {
		return strings.TrimSpace(row.Pane), nil
	}
	switch row.Subject {
	case "prompt":
		target, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(target) == "" {
			return "", errors.New("no sqlite prompt target configured")
		}
		return target, nil
	case "command":
		target, err := loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(target) == "" {
			return "", errors.New("no sqlite command target configured")
		}
		return target, nil
	default:
		return "", fmt.Errorf("no target mapping for subject %q", row.Subject)
	}
}

func captureCurrentShellState(db *sql.DB, repoRoot string, refID int64) error {
	promptTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return err
	}
	commandTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
	if err != nil {
		return err
	}
	session := "codex-view"
	if strings.TrimSpace(promptTarget) != "" {
		session = strings.TrimSpace(strings.Split(promptTarget, ":")[0])
		if err := capturePaneSnapshot(db, repoRoot, session, promptTarget, refID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(commandTarget) != "" {
		session = strings.TrimSpace(strings.Split(commandTarget, ":")[0])
		if err := capturePaneSnapshot(db, repoRoot, session, commandTarget, refID); err != nil {
			return err
		}
	}
	return nil
}

func capturePaneSnapshot(db *sql.DB, repoRoot, session, pane string, refID int64) error {
	return capturePaneSnapshotWithReader(db, session, pane, refID, func(target string) (string, error) {
		return runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
			"DIALTONE_TMUX_PROXY_ACTIVE": "1",
		}, "tmux", "v1", "read", "--pane", target, "--lines", "120")
	})
}

func capturePaneSnapshotWithReader(db *sql.DB, session, pane string, refID int64, reader func(string) (string, error)) error {
	if db == nil {
		return errors.New("capturePaneSnapshotWithReader requires an open sqlite db handle")
	}
	target := strings.TrimSpace(pane)
	if target == "" {
		return nil
	}
	text, err := reader(target)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]string{
		"text":    text,
		"summary": summarizeSnapshot(text),
	})
	if err != nil {
		return err
	}
	_, err = modstate.AppendShellBusObserved(db, "tmux", "pane", "snapshot", "shell-sync", strings.TrimSpace(session), target, refID, string(payload))
	return err
}

func runState(argv []string) error {
	opts := flag.NewFlagSet("shell v1 state", flag.ContinueOnError)
	limit := opts.Int("limit", 40, "Maximum observed rows to scan for pane snapshots")
	full := opts.Bool("full", false, "Print the full latest pane text captured in SQLite")
	syncNow := opts.Bool("sync", true, "Refresh pane snapshots from tmux before printing SQLite state")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("state does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	if *syncNow {
		if err := captureCurrentShellState(db, repoRoot, 0); err != nil {
			return err
		}
	}
	commandTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
	if err != nil {
		return err
	}
	promptTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return err
	}
	queued, err := modstate.LoadQueuedShellBus(db, 100)
	if err != nil {
		return err
	}
	rows, err := modstate.LoadShellBus(db, "observed", *limit)
	if err != nil {
		return err
	}
	promptSnapshot := latestPaneSnapshot(rows, promptTarget)
	commandSnapshot := latestPaneSnapshot(rows, commandTarget)
	fmt.Printf("prompt_target\t%s\ncommand_target\t%s\nqueued\t%d\n", printableTarget(promptTarget), printableTarget(commandTarget), len(queued))
	if promptSnapshot != "" {
		fmt.Printf("prompt_snapshot\t%s\n", promptSnapshot)
	}
	if commandSnapshot != "" {
		fmt.Printf("command_snapshot\t%s\n", commandSnapshot)
	}
	if *full {
		if text := latestPaneSnapshotText(rows, promptTarget); text != "" {
			fmt.Printf("prompt_text\n%s\n", text)
		}
		if text := latestPaneSnapshotText(rows, commandTarget); text != "" {
			fmt.Printf("command_text\n%s\n", text)
		}
	}
	return nil
}

func runRead(argv []string) error {
	opts := flag.NewFlagSet("shell v1 read", flag.ContinueOnError)
	role := opts.String("role", "command", "Logical pane role to read from SQLite: prompt or command")
	pane := opts.String("pane", "", "Optional explicit pane override")
	limit := opts.Int("limit", 40, "Maximum observed rows to scan for pane snapshots")
	full := opts.Bool("full", true, "Print the full latest pane text captured in SQLite")
	syncNow := opts.Bool("sync", true, "Refresh pane snapshots from tmux before reading from SQLite")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("read does not accept positional arguments")
	}
	if strings.TrimSpace(*role) != "prompt" && strings.TrimSpace(*role) != "command" {
		return errors.New("--role must be prompt or command")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	if *syncNow {
		if err := captureCurrentShellState(db, repoRoot, 0); err != nil {
			return err
		}
	}
	target, err := resolveReadPane(repoRoot, strings.TrimSpace(*role), strings.TrimSpace(*pane))
	if err != nil {
		return err
	}
	rows, err := modstate.LoadShellBus(db, "observed", *limit)
	if err != nil {
		return err
	}
	fmt.Printf("role\t%s\npane\t%s\n", strings.TrimSpace(*role), printableTarget(target))
	if *full {
		if text := latestPaneSnapshotText(rows, target); text != "" {
			fmt.Printf("text\n%s\n", text)
		}
		return nil
	}
	if summary := latestPaneSnapshot(rows, target); summary != "" {
		fmt.Printf("summary\t%s\n", summary)
	}
	return nil
}

func runEvents(argv []string) error {
	opts := flag.NewFlagSet("shell v1 events", flag.ContinueOnError)
	scope := opts.String("scope", "", "Optional shell bus scope filter")
	limit := opts.Int("limit", 20, "Maximum shell bus rows to print")
	syncNow := opts.Bool("sync", true, "Refresh pane snapshots from tmux before printing SQLite events")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("events does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	if *syncNow {
		if err := captureCurrentShellState(db, repoRoot, 0); err != nil {
			return err
		}
	}
	rows, err := modstate.LoadShellBus(db, strings.TrimSpace(*scope), *limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			row.ID, row.Scope, row.Subject, row.Action, row.Status, row.Actor,
			printableTarget(row.Session), printableTarget(row.Pane), summarizeSnapshot(row.BodyJSON), row.RefID, row.UpdatedAt)
	}
	return nil
}

func submitPrompt(db *sql.DB, repoRoot, target, text string) (int64, error) {
	if db == nil {
		return 0, errors.New("submitPrompt requires an open sqlite db handle")
	}
	queueID, err := modstate.EnqueueCommand(db, "prompts", "prompt", target, text, "")
	if err != nil {
		return 0, err
	}
	if err := modstate.MarkCommandStarted(db, queueID); err != nil {
		return 0, err
	}
	_, err = runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
		"DIALTONE_TMUX_PROXY_ACTIVE": "1",
	}, "tmux", "v1", "write", "--pane", target, "--enter", text)
	if err != nil {
		_ = modstate.MarkCommandFinished(db, queueID, "failed", "", err.Error())
		return 0, err
	}
	if err := modstate.MarkCommandFinished(db, queueID, "done", "submitted", ""); err != nil {
		return 0, err
	}
	return queueID, nil
}

func writeVisibleCommand(repoRoot, target, command string) error {
	_, err := runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
		"DIALTONE_TMUX_PROXY_ACTIVE": "1",
	}, "tmux", "v1", "write", "--pane", target, "--enter", command)
	return err
}

func latestPaneSnapshot(rows []modstate.ShellBusRecord, pane string) string {
	target := strings.TrimSpace(pane)
	if target == "" {
		return ""
	}
	for _, row := range rows {
		if row.Subject != "pane" || row.Action != "snapshot" {
			continue
		}
		if strings.TrimSpace(row.Pane) != target {
			continue
		}
		payload := map[string]string{}
		if err := json.Unmarshal([]byte(row.BodyJSON), &payload); err == nil {
			return summarizeSnapshot(payload["summary"] + "\n" + payload["text"])
		}
		return summarizeSnapshot(row.BodyJSON)
	}
	return ""
}

func latestPaneSnapshotText(rows []modstate.ShellBusRecord, pane string) string {
	target := strings.TrimSpace(pane)
	if target == "" {
		return ""
	}
	for _, row := range rows {
		if row.Subject != "pane" || row.Action != "snapshot" {
			continue
		}
		if strings.TrimSpace(row.Pane) != target {
			continue
		}
		payload := map[string]string{}
		if err := json.Unmarshal([]byte(row.BodyJSON), &payload); err == nil {
			return strings.TrimSpace(payload["text"])
		}
		return strings.TrimSpace(row.BodyJSON)
	}
	return ""
}

func summarizeSnapshot(text string) string {
	value := strings.TrimSpace(text)
	if value == "" {
		return ""
	}
	lines := strings.Split(value, "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		for i := len(lines) - 1; i >= 0; i-- {
			if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
				last = trimmed
				break
			}
		}
	}
	if len(last) > 100 {
		last = last[:100]
	}
	return last
}

func trackedCommandExitStatus(text string) (int, bool) {
	value := strings.TrimSpace(text)
	if value == "" {
		return 0, false
	}
	lines := strings.Split(value, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		const marker = " exit="
		index := strings.LastIndex(line, marker)
		if index < 0 || !strings.Contains(line, "DIALTONE_CMD_DONE_") {
			continue
		}
		statusText := strings.TrimSpace(line[index+len(marker):])
		statusCode, err := strconv.Atoi(statusText)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	}
	return 0, false
}

func waitForPaneContains(repoRoot, pane, needle string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	lastOutput := ""
	for time.Now().Before(deadline) {
		out, err := runDialtoneModWithEnvQuiet(repoRoot, map[string]string{
			"DIALTONE_TMUX_PROXY_ACTIVE": "1",
		}, "tmux", "v1", "read", "--pane", pane, "--lines", "160")
		if err == nil {
			lastOutput = out
			if strings.Contains(out, needle) {
				return out, nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastOutput, fmt.Errorf("timed out waiting for %q in tmux pane %s", needle, pane)
}

func runSupervise(argv []string) error {
	opts := flag.NewFlagSet("shell v1 supervise", flag.ContinueOnError)
	thresholdSeconds := opts.Int("threshold-seconds", 30, "Age threshold to consider a queued/running row stuck")
	limit := opts.Int("limit", 5, "Number of recent queue rows to inspect per queue")
	readLines := opts.Int("read-lines", 20, "Pane lines to read when inspecting targets")
	inspectPanes := opts.Bool("inspect-panes", true, "Read prompt and command panes as part of supervision output")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("supervise does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	commandTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
	if err != nil {
		return err
	}
	promptTarget, err := loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return err
	}
	fmt.Printf("command_target\t%s\nprompt_target\t%s\n", printableTarget(commandTarget), printableTarget(promptTarget))
	for _, queueName := range []string{"prompts", "tmux", "tests"} {
		rows, err := modstate.LoadQueue(db, queueName, *limit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			fmt.Printf("queue\t%s\tempty\n", queueName)
			continue
		}
		status := classifyQueue(rows, time.Duration(*thresholdSeconds)*time.Second)
		head := rows[0]
		fmt.Printf("queue\t%s\t%s\tid=%d\tstatus=%s\ttarget=%s\tcommand=%s\n", queueName, status, head.ID, head.Status, head.Target, head.CommandText)
	}
	if *inspectPanes {
		env := map[string]string{"DIALTONE_TMUX_PROXY_ACTIVE": "1"}
		if strings.TrimSpace(promptTarget) != "" {
			if out, err := runDialtoneModWithEnvQuiet(repoRoot, env, "tmux", "v1", "read", "--pane", promptTarget, "--lines", fmt.Sprintf("%d", *readLines)); err == nil {
				fmt.Printf("prompt_pane_read\n%s\n", strings.TrimRight(out, "\n"))
			}
		}
		if strings.TrimSpace(commandTarget) != "" {
			if out, err := runDialtoneModWithEnvQuiet(repoRoot, env, "tmux", "v1", "read", "--pane", commandTarget, "--lines", fmt.Sprintf("%d", *readLines)); err == nil {
				fmt.Printf("command_pane_read\n%s\n", strings.TrimRight(out, "\n"))
			}
		}
	}
	return nil
}

func loadStateTarget(repoRoot, key string) (string, error) {
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return "", err
	}
	defer db.Close()
	record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
	if err != nil {
		return "", err
	}
	if ok && strings.TrimSpace(record.Value) != "" {
		return strings.TrimSpace(record.Value), nil
	}
	recovered, ok, err := recoverStateTargetFromObserved(db, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, key, recovered); err != nil {
		return "", err
	}
	return recovered, nil
}

func recoverStateTargetFromObserved(db *sql.DB, key string) (string, bool, error) {
	rows, err := modstate.LoadShellBus(db, "observed", 200)
	if err != nil {
		return "", false, err
	}
	suffix := ""
	switch strings.TrimSpace(key) {
	case sqlitestate.TmuxPromptTargetKey:
		suffix = ":0:0"
	case sqlitestate.TmuxTargetKey:
		suffix = ":0:1"
	default:
		return "", false, nil
	}
	for _, row := range rows {
		pane := strings.TrimSpace(row.Pane)
		if row.Subject != "pane" || row.Action != "snapshot" || pane == "" {
			continue
		}
		if strings.HasSuffix(pane, suffix) {
			return pane, true, nil
		}
	}
	return "", false, nil
}

func resolveReadPane(repoRoot, role, explicitPane string) (string, error) {
	if strings.TrimSpace(explicitPane) != "" {
		return strings.TrimSpace(explicitPane), nil
	}
	switch strings.TrimSpace(role) {
	case "prompt":
		return loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
	case "command":
		return loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
	default:
		return "", fmt.Errorf("unsupported read role %q", role)
	}
}

func printableTarget(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unset)"
	}
	return value
}

func classifyQueue(rows []modstate.QueueRecord, threshold time.Duration) string {
	head := rows[0]
	if (head.Status == "queued" || head.Status == "running") && queueRowAge(head) > threshold {
		return "stuck"
	}
	if len(rows) >= 3 &&
		rows[0].CommandText != "" &&
		rows[0].CommandText == rows[1].CommandText &&
		rows[1].CommandText == rows[2].CommandText &&
		rows[0].Target == rows[1].Target &&
		rows[1].Target == rows[2].Target {
		return "looping"
	}
	return "healthy"
}

func queueRowAge(row modstate.QueueRecord) time.Duration {
	for _, raw := range []string{row.StartedAt, row.CreatedAt} {
		if ts, ok := parseSQLiteTime(raw); ok {
			return time.Since(ts)
		}
	}
	return 0
}

func parseSQLiteTime(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
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
	return runDialtoneModWithEnvQuiet(repoRoot, nil, args...)
}

func runDialtoneModWithEnvQuiet(repoRoot string, extraEnv map[string]string, args ...string) (string, error) {
	if len(args) >= 2 && shouldUseBackendCLI(args[0], args[1]) && backendCLIAvailable(repoRoot, args[0], args[1]) {
		return runBackendCLIQuiet(repoRoot, extraEnv, args[0], args[1], args[2:]...)
	}
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone_mod"), args...)
	cmd.Dir = repoRoot
	if len(extraEnv) > 0 {
		env := os.Environ()
		for key, value := range extraEnv {
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}
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

func shouldUseBackendCLI(modName, version string) bool {
	switch strings.TrimSpace(modName) + ":" + strings.TrimSpace(version) {
	case "ghostty:v1", "tmux:v1", "codex:v1":
		return true
	default:
		return false
	}
}

func backendCLIEntry(modName, version string) string {
	return "./" + filepath.ToSlash(filepath.Join("mods", strings.TrimSpace(modName), strings.TrimSpace(version), "cli"))
}

func backendCLIAvailable(repoRoot, modName, version string) bool {
	path := filepath.Join(repoRoot, "src", "mods", strings.TrimSpace(modName), strings.TrimSpace(version), "cli", "main.go")
	_, err := os.Stat(path)
	return err == nil
}

func runBackendCLIQuiet(repoRoot string, extraEnv map[string]string, modName, version string, args ...string) (string, error) {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmdArgs := append([]string{"run", backendCLIEntry(modName, version)}, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = filepath.Join(repoRoot, "src")
	if len(extraEnv) > 0 {
		env := os.Environ()
		for key, value := range extraEnv {
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}
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
		return stdout.String(), fmt.Errorf("backend cli %s %s failed: %s", modName, version, errText)
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
	fmt.Println("  start [--session codex-view] [--shell default|repl-v1|ssh-v1] [--dialtone-shell ssh-v1] [--reasoning medium] [--model gpt-5.4] [--wait-seconds 20] [--run-tests=true]")
	fmt.Println("       Quit Ghostty, create one fresh window with one tab, start/attach codex-view, split dialtone-view on the right, launch Codex left, and optionally run the sqlite test plan on the right")
	fmt.Println("  split-vertical [--session codex-view] [--shell default|repl-v1|ssh-v1] [--right-title dialtone-view] [--wait-seconds 10]")
	fmt.Println("       Split the tmux window so Codex stays on the left and a dialtone shell pane titled dialtone-view appears on the right")
	fmt.Println("  prompt [--session codex-view] [--pane codex-view:0:0] <text...>")
	fmt.Println("       Queue a prompt intent on the SQLite shell bus for codex-view")
	fmt.Println("  enqueue-command [--session codex-view] [--pane codex-view:0:1] [--expect TEXT] <command...>")
	fmt.Println("       Queue a visible command intent on the SQLite shell bus for dialtone-view")
	fmt.Println("  run [--session codex-view] [--pane codex-view:0:1] [--expect TEXT] [--wait-seconds 10] <command...>")
	fmt.Println("       Queue and immediately reconcile one visible command through the SQLite shell bus")
	fmt.Println("  test [--wait-seconds 20]")
	fmt.Println("       Run the shell/v1 Go test visibly in dialtone-view through the SQLite shell bus")
	fmt.Println("  test-basic")
	fmt.Println("       Run the core SQLite and shell Go tests first before starting the visible Ghostty workflow")
	fmt.Println("  test-all [--wait-seconds 120]")
	fmt.Println("       Run the full mod Go test sweep visibly in dialtone-view through the SQLite shell bus")
	fmt.Println("  workflow [--wait-seconds 120]")
	fmt.Println("       Run basic tests first, start the shell workflow, then run the remaining full test sweep in dialtone-view")
	fmt.Println("  sync-once [--limit 10] [--wait-seconds 10]")
	fmt.Println("       Reconcile queued SQLite shell bus intents into the live tmux panes once")
	fmt.Println("  state [--limit 40] [--full]")
	fmt.Println("       Auto-refresh and read prompt/command targets and latest pane snapshots from SQLite")
	fmt.Println("  read [--role prompt|command] [--pane codex-view:0:1] [--full=true] [--sync=true]")
	fmt.Println("       Auto-refresh and print one pane snapshot from SQLite without reading tmux directly yourself")
	fmt.Println("  events [--scope desired|observed] [--limit 20]")
	fmt.Println("       Auto-refresh and print recent SQLite shell bus rows")
	fmt.Println("  demo-protocol [--bootstrap=true|false] [--dialtone-shell ssh-v1] [--wait-seconds 20] [--command 'env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline']")
	fmt.Println("       Record a protocol run in SQLite by submitting a prompt to codex-view and observing one visible ./dialtone_mod command in dialtone-view")
	fmt.Println("  supervise [--threshold-seconds 30] [--limit 5] [--read-lines 20] [--inspect-panes=true]")
	fmt.Println("       Poll SQLite queue state and inspect prompt/command panes for stuck or looping work")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
