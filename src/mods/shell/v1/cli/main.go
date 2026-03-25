package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/internal/tmuxcmd"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/sqlitestate"
)

type shellWorkflowState struct {
	PromptTarget       string
	CommandTarget      string
	PromptPanePresent  bool
	CommandPanePresent bool
	WorkerHealthy      bool
	WorkerPane         string
}

var (
	shellPaneExistsFn  = paneExists
	shellRunStartFn    func([]string) error
	shellStartWorkerFn func(string, string, string, time.Duration) error
)

func (state shellWorkflowState) readyForVisibleCommands() bool {
	if strings.TrimSpace(state.PromptTarget) == "" || strings.TrimSpace(state.CommandTarget) == "" {
		return false
	}
	if !state.PromptPanePresent || !state.CommandPanePresent || !state.WorkerHealthy {
		return false
	}
	workerPane := strings.TrimSpace(state.WorkerPane)
	commandTarget := strings.TrimSpace(state.CommandTarget)
	return workerPane != "" && workerPane == commandTarget
}

func (state shellWorkflowState) problems() []string {
	problems := make([]string, 0, 6)
	if strings.TrimSpace(state.PromptTarget) == "" {
		problems = append(problems, "missing prompt_target")
	} else if !state.PromptPanePresent {
		problems = append(problems, "prompt pane is not reachable")
	}
	if strings.TrimSpace(state.CommandTarget) == "" {
		problems = append(problems, "missing command_target")
	} else if !state.CommandPanePresent {
		problems = append(problems, "command pane is not reachable")
	}
	if !state.WorkerHealthy {
		problems = append(problems, "worker heartbeat is stale or stopped")
	}
	workerPane := strings.TrimSpace(state.WorkerPane)
	commandTarget := strings.TrimSpace(state.CommandTarget)
	switch {
	case workerPane == "":
		problems = append(problems, "worker pane is missing")
	case commandTarget != "" && workerPane != commandTarget:
		problems = append(problems, fmt.Sprintf("worker pane %s does not match command target %s", printableTarget(workerPane), printableTarget(commandTarget)))
	}
	return problems
}

func (state shellWorkflowState) problemSummary() string {
	problems := state.problems()
	if len(problems) == 0 {
		return "no workflow issues"
	}
	return strings.Join(problems, "; ")
}

func invokeShellRunStart(args []string) error {
	if shellRunStartFn != nil {
		return shellRunStartFn(args)
	}
	return runStart(args)
}

func invokeShellStartWorker(repoRoot, pane, shellName string, wait time.Duration) error {
	if shellStartWorkerFn != nil {
		return shellStartWorkerFn(repoRoot, pane, shellName, wait)
	}
	return startShellWorker(repoRoot, pane, shellName, wait)
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
	case "install":
		if err := runInstall(args); err != nil {
			exitIfErr(err, "shell install")
		}
	case "build":
		if err := runBuild(args); err != nil {
			exitIfErr(err, "shell build")
		}
	case "format":
		if err := runFormat(args); err != nil {
			exitIfErr(err, "shell format")
		}
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
	case "serve":
		if err := runServe(args); err != nil {
			exitIfErr(err, "shell serve")
		}
	case "ensure-worker":
		if err := runEnsureWorker(args); err != nil {
			exitIfErr(err, "shell ensure-worker")
		}
	case "state":
		if err := runState(args); err != nil {
			exitIfErr(err, "shell state")
		}
	case "status":
		if err := runState(args); err != nil {
			exitIfErr(err, "shell status")
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

	startTmuxCmd := buildTmuxStartCommand(repoRoot, strings.TrimSpace(*session))
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

	if err := ensureSplitLayout(repoRoot, strings.TrimSpace(*session), strings.TrimSpace(*dialtoneShell), strings.TrimSpace(*rightTitle), time.Duration(*waitSeconds)*time.Second, false); err != nil {
		return err
	}
	if err := startShellWorker(repoRoot, rightPane, strings.TrimSpace(*dialtoneShell), time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot, buildCodexStartArgs(strings.TrimSpace(*session), pane, strings.TrimSpace(*shellName), strings.TrimSpace(*reasoning), strings.TrimSpace(*model))...); err != nil {
		return err
	}
	if *runTests {
		if err := runRun([]string{
			"--pane", rightPane,
			"--wait-seconds", "240",
			"./dialtone_mod mods v1 db test-run",
		}); err != nil {
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

func buildTmuxStartCommand(repoRoot, session string) string {
	return tmuxcmd.ShellCommand(repoRoot, "new-session", "-A", "-s", strings.TrimSpace(session))
}

func runSplitVerticalWithOptions(session, shellName, rightTitle string, wait time.Duration) error {
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	if err := ensureSplitLayout(repoRoot, session, shellName, rightTitle, wait, true); err != nil {
		return err
	}
	fmt.Printf("split shell workflow: codex on %s:0:0 (left), %s on %s:0:1 (right)\n",
		session,
		rightTitle,
		session,
	)
	return nil
}

func ensureSplitLayout(repoRoot, session, shellName, rightTitle string, wait time.Duration, enterShell bool) error {
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
	if enterShell {
		if _, err := runDialtoneMod(repoRoot,
			"tmux", "v1", "shell",
			"--pane", rightPane,
			"--shell", strings.TrimSpace(shellName),
			"--banner=false",
		); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
		if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "clear", "--pane", rightPane); err != nil {
			return err
		}
	}
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "target", "--set", rightPane); err != nil {
		return err
	}
	if _, err := runDialtoneMod(repoRoot, "tmux", "v1", "target", "--set-prompt", leftPane); err != nil {
		return err
	}
	return nil
}

func startShellWorker(repoRoot, pane, shellName string, wait time.Duration) error {
	serveCommand := buildShellWorkerStartCommand(repoRoot, strings.TrimSpace(pane))
	if _, err := runDialtoneMod(repoRoot,
		"tmux", "v1", "shell",
		"--pane", strings.TrimSpace(pane),
		"--shell", strings.TrimSpace(shellName),
		"--banner=false",
		"--command", serveCommand,
	); err != nil {
		return err
	}
	if err := waitForShellWorkerReady(repoRoot, strings.TrimSpace(pane), wait); err != nil {
		return err
	}
	if _, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "clear", "--pane", strings.TrimSpace(pane)); err != nil {
		return err
	}
	return nil
}

func buildShellWorkerStartCommand(repoRoot, pane string) string {
	return fmt.Sprintf("cd %s && ./dialtone_mod shell v1 serve --pane %s", repoRoot, strings.TrimSpace(pane))
}

func waitForShellWorkerReady(repoRoot, pane string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
		if err == nil {
			healthy, checkErr := shellWorkerHealthy(db, 5*time.Second)
			if checkErr == nil && healthy {
				record, ok, loadErr := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerPaneKey)
				_ = db.Close()
				if loadErr == nil && ok && strings.TrimSpace(record.Value) == strings.TrimSpace(pane) {
					return nil
				}
			} else {
				_ = db.Close()
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for shell worker in %s", printableTarget(pane))
}

func runEnsureWorker(argv []string) error {
	opts := flag.NewFlagSet("shell v1 ensure-worker", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name to ensure")
	dialtoneShell := opts.String("dialtone-shell", defaultDialtoneShellName(), "flake shell to keep active in the right-side dialtone pane")
	reasoning := opts.String("reasoning", "medium", "reasoning label for the Codex startup banner")
	model := opts.String("model", "gpt-5.4", "Codex model to launch if the workflow is missing")
	rightTitle := opts.String("right-title", "dialtone-view", "Title for the right-side tmux pane")
	waitSeconds := opts.Int("wait-seconds", 20, "Seconds to wait for a new shell worker")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("ensure-worker does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	wait := time.Duration(*waitSeconds) * time.Second
	if err := ensureVisibleShellWorkflow(
		repoRoot,
		strings.TrimSpace(*session),
		strings.TrimSpace(*dialtoneShell),
		strings.TrimSpace(*reasoning),
		strings.TrimSpace(*model),
		strings.TrimSpace(*rightTitle),
		wait,
	); err != nil {
		return err
	}
	state, err := inspectShellWorkflowState(repoRoot)
	if err != nil {
		return err
	}
	if !state.readyForVisibleCommands() {
		return fmt.Errorf("shell workflow is not ready: %s", state.problemSummary())
	}
	fmt.Printf("shell worker ready\t%s\n", strings.TrimSpace(state.CommandTarget))
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
	if *syncNow {
		if err := ensureVisibleShellWorkflow(repoRoot, strings.TrimSpace(*session), defaultDialtoneShellName(), "medium", "gpt-5.4", "dialtone-view", 20*time.Second); err != nil {
			return err
		}
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
		if err := waitForShellBusRowExecution(db, repoRoot, rowID, 3*time.Second); err != nil {
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
	if *syncNow {
		if err := ensureVisibleShellWorkflow(repoRoot, strings.TrimSpace(*session), defaultDialtoneShellName(), "medium", "gpt-5.4", "dialtone-view", time.Duration(*waitSeconds)*time.Second); err != nil {
			return err
		}
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	rowID, err := enqueueShellBusCommand(db, repoRoot, "shell-cli", strings.TrimSpace(*session), strings.TrimSpace(*pane), shellBusIntentBody{
		Command: strings.TrimSpace(strings.Join(opts.Args(), " ")),
		Expect:  strings.TrimSpace(*expect),
	})
	if err != nil {
		return err
	}
	if *syncNow {
		if err := waitForShellBusRowExecution(db, repoRoot, rowID, time.Duration(*waitSeconds)*time.Second); err != nil {
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
	if err := ensureVisibleShellWorkflow(repoRoot, strings.TrimSpace(*session), defaultDialtoneShellName(), "medium", "gpt-5.4", "dialtone-view", time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	rowID, err := enqueueShellBusCommand(db, repoRoot, "shell-cli", strings.TrimSpace(*session), strings.TrimSpace(*pane), shellBusIntentBody{
		Command: strings.TrimSpace(strings.Join(opts.Args(), " ")),
		Expect:  strings.TrimSpace(*expect),
	})
	if err != nil {
		return err
	}
	if err := waitForShellBusRowExecution(db, repoRoot, rowID, time.Duration(*waitSeconds)*time.Second); err != nil {
		return err
	}
	_ = captureCurrentShellState(db, repoRoot, 0)
	fmt.Printf("ran command via shell bus [row_id=%d]\n", rowID)
	return nil
}

func runTest(argv []string) error {
	opts := flag.NewFlagSet("shell v1 test", flag.ContinueOnError)
	waitSeconds := opts.Int("wait-seconds", 60, "Seconds to wait for the shell workflow Go suite in dialtone-view")
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
	command := buildShellTestCommand(repoRoot)
	return runRun([]string{
		"--wait-seconds", fmt.Sprintf("%d", *waitSeconds),
		"--expect", "DIALTONE_SHELL_TEST_DONE",
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

func shellTestPackages() []string {
	return []string{
		"./internal/modcli",
		"./internal/modstate",
		"./mods/shared/dispatch",
		"./mods/shared/router",
		"./mods/shared/sqlitestate",
		"./mods/dialtone/v1",
		"./mods/codex/v1/...",
		"./mods/ghostty/v1/...",
		"./mods/shell/v1/...",
		"./mods/test/v1/...",
		"./mods/tmux/v1/...",
	}
}

func buildShellTestCommand(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	packages := strings.Join(shellTestPackages(), " ")
	return fmt.Sprintf("clear && cd %s/src && go test %s && printf 'DIALTONE_SHELL_TEST_DONE\\n'", root, packages)
}

func buildBasicTestCommand(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	return fmt.Sprintf("clear && cd %s/src && go test ./internal/modstate ./mods/shared/sqlitestate ./mods/mod/v1 ./mods/shell/v1/cli", root)
}

func buildAllModsTestCommand(repoRoot string) string {
	root := strings.TrimSpace(repoRoot)
	return fmt.Sprintf("clear && cd %s/src && go test ./mods/...", root)
}

func buildCodexStartArgs(session, pane, shellName, reasoning, model string) []string {
	return []string{
		"codex", "v1", "start",
		"--session", strings.TrimSpace(session),
		"--pane", strings.TrimSpace(pane),
		"--shell", strings.TrimSpace(shellName),
		"--reasoning", strings.TrimSpace(reasoning),
		"--model", strings.TrimSpace(model),
	}
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
	commandText := opts.String("command", "./dialtone_mod mods v1 db graph --format outline", "Visible command to run in dialtone-view")
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
	Text           string   `json:"text,omitempty"`
	Command        string   `json:"command,omitempty"`
	Expect         string   `json:"expect,omitempty"`
	InnerCommand   string   `json:"inner_command,omitempty"`
	DisplayCommand string   `json:"display_command,omitempty"`
	Args           []string `json:"args,omitempty"`
	Target         string   `json:"target,omitempty"`
	LogPath        string   `json:"log_path,omitempty"`
	Summary        string   `json:"summary,omitempty"`
	Error          string   `json:"error,omitempty"`
	Output         string   `json:"output,omitempty"`
	StartedAt      string   `json:"started_at,omitempty"`
	FinishedAt     string   `json:"finished_at,omitempty"`
	PID            int      `json:"pid,omitempty"`
	ExitCode       int      `json:"exit_code"`
	RuntimeMS      int64    `json:"runtime_ms"`
}

func enqueueShellBusCommand(db *sql.DB, repoRoot, actor, session, pane string, body shellBusIntentBody) (int64, error) {
	if db == nil {
		return 0, errors.New("enqueueShellBusCommand requires an open sqlite db handle")
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", strings.TrimSpace(actor), strings.TrimSpace(session), strings.TrimSpace(pane), string(raw))
	if err != nil {
		return 0, err
	}
	body.LogPath = sqlitestate.ResolveCommandLogPath(repoRoot, rowID)
	updated, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "queued", 0, string(updated)); err != nil {
		return 0, err
	}
	return rowID, nil
}

func ensureShellBusCommandLogPath(repoRoot string, rowID int64, body *shellBusIntentBody) {
	if body == nil || rowID <= 0 || strings.TrimSpace(repoRoot) == "" {
		return
	}
	if strings.TrimSpace(body.LogPath) == "" {
		body.LogPath = sqlitestate.ResolveCommandLogPath(repoRoot, rowID)
	}
}

func writeShellBusCommandLog(repoRoot string, rowID int64, body shellBusIntentBody, status string) error {
	ensureShellBusCommandLogPath(repoRoot, rowID, &body)
	logPath := strings.TrimSpace(body.LogPath)
	if logPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "row_id=%d\n", rowID)
	if strings.TrimSpace(body.DisplayCommand) != "" {
		fmt.Fprintf(&out, "display_command=%s\n", strings.TrimSpace(body.DisplayCommand))
	}
	if strings.TrimSpace(body.Command) != "" {
		fmt.Fprintf(&out, "command=%s\n", strings.TrimSpace(body.Command))
	}
	if strings.TrimSpace(body.Target) != "" {
		fmt.Fprintf(&out, "target=%s\n", strings.TrimSpace(body.Target))
	}
	if strings.TrimSpace(status) != "" {
		fmt.Fprintf(&out, "status=%s\n", strings.TrimSpace(status))
	}
	if strings.TrimSpace(body.StartedAt) != "" {
		fmt.Fprintf(&out, "started_at=%s\n", strings.TrimSpace(body.StartedAt))
	}
	if strings.TrimSpace(body.FinishedAt) != "" {
		fmt.Fprintf(&out, "finished_at=%s\n", strings.TrimSpace(body.FinishedAt))
	}
	if body.PID > 0 {
		fmt.Fprintf(&out, "pid=%d\n", body.PID)
	}
	if body.ExitCode != 0 || strings.TrimSpace(status) == "done" || strings.TrimSpace(status) == "failed" {
		fmt.Fprintf(&out, "exit_code=%d\n", body.ExitCode)
	}
	if body.RuntimeMS > 0 {
		fmt.Fprintf(&out, "runtime_ms=%d\n", body.RuntimeMS)
	}
	if strings.TrimSpace(body.Summary) != "" {
		fmt.Fprintf(&out, "summary=%s\n", strings.TrimSpace(body.Summary))
	}
	if strings.TrimSpace(body.Error) != "" {
		fmt.Fprintf(&out, "error=%s\n", strings.TrimSpace(body.Error))
	}
	if strings.TrimSpace(body.Output) != "" {
		fmt.Fprintf(&out, "\noutput\n%s\n", strings.TrimRight(body.Output, "\n"))
	}
	return os.WriteFile(logPath, []byte(out.String()), 0o644)
}

func runServe(argv []string) error {
	opts := flag.NewFlagSet("shell v1 serve", flag.ContinueOnError)
	pane := opts.String("pane", "", "Explicit dialtone-view pane target for this worker")
	pollIntervalMS := opts.Int("poll-interval-ms", 500, "Polling interval for queued SQLite shell bus rows")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("serve does not accept positional arguments")
	}
	if *pollIntervalMS <= 0 {
		return errors.New("--poll-interval-ms must be positive")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	workerPane := strings.TrimSpace(*pane)
	if workerPane == "" {
		workerPane, err = loadStateTarget(repoRoot, sqlitestate.TmuxTargetKey)
		if err != nil {
			return err
		}
	}
	if workerPane == "" {
		return errors.New("serve requires a dialtone-view pane target")
	}
	session := strings.TrimSpace(strings.Split(workerPane, ":")[0])
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	defer func() {
		_ = setShellWorkerState(db, map[string]string{
			sqlitestate.ShellWorkerStatusKey:         "stopped",
			sqlitestate.ShellWorkerPaneKey:           workerPane,
			sqlitestate.ShellWorkerHeartbeatKey:      time.Now().UTC().Format(time.RFC3339),
			sqlitestate.ShellWorkerCurrentRowIDKey:   "",
			sqlitestate.ShellWorkerCurrentCommandKey: "",
		})
	}()

	fmt.Printf("shell worker ready\t%s\n", workerPane)
	for {
		if err := setShellWorkerState(db, map[string]string{
			sqlitestate.ShellWorkerStatusKey:    "running",
			sqlitestate.ShellWorkerPaneKey:      workerPane,
			sqlitestate.ShellWorkerHeartbeatKey: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
		rows, err := modstate.LoadQueuedShellBus(db, 20)
		if err != nil {
			return err
		}
		for _, row := range rows {
			target, err := resolveShellBusTarget(repoRoot, row)
			if err != nil {
				return err
			}
			switch row.Subject {
			case "command":
				if strings.TrimSpace(target) != workerPane {
					continue
				}
			case "prompt":
				if strings.TrimSpace(target) == "" || strings.TrimSpace(strings.Split(target, ":")[0]) != session {
					continue
				}
			default:
				continue
			}
			if err := syncShellBusRowInWorker(db, repoRoot, row, workerPane); err != nil {
				return err
			}
		}
		time.Sleep(time.Duration(*pollIntervalMS) * time.Millisecond)
	}
}

func setShellWorkerState(db *sql.DB, values map[string]string) error {
	for key, value := range values {
		if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, key, value); err != nil {
			return err
		}
	}
	return nil
}

func shellWorkerHealthy(db *sql.DB, maxAge time.Duration) (bool, error) {
	if db == nil {
		return false, nil
	}
	statusRecord, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey)
	if err != nil || !ok || strings.TrimSpace(statusRecord.Value) != "running" {
		return false, err
	}
	heartbeatRecord, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey)
	if err != nil || !ok {
		return false, err
	}
	heartbeat, ok := parseSQLiteTime(heartbeatRecord.Value)
	if !ok {
		return false, nil
	}
	return time.Since(heartbeat) <= maxAge, nil
}

func inspectShellWorkflowState(repoRoot string) (shellWorkflowState, error) {
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return shellWorkflowState{}, err
	}
	defer db.Close()
	if err := modstate.EnsureSchema(db); err != nil {
		return shellWorkflowState{}, err
	}
	promptTarget, err := loadStateTargetFromDB(db, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return shellWorkflowState{}, err
	}
	commandTarget, err := loadStateTargetFromDB(db, sqlitestate.TmuxTargetKey)
	if err != nil {
		return shellWorkflowState{}, err
	}
	workerPane, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerPaneKey)
	healthy, err := shellWorkerHealthy(db, 5*time.Second)
	if err != nil {
		return shellWorkflowState{}, err
	}
	state := shellWorkflowState{
		PromptTarget:  strings.TrimSpace(promptTarget),
		CommandTarget: strings.TrimSpace(commandTarget),
		WorkerHealthy: healthy,
		WorkerPane:    strings.TrimSpace(workerPane),
	}
	if state.PromptTarget != "" {
		state.PromptPanePresent = shellPaneExistsFn(repoRoot, state.PromptTarget)
	}
	if state.CommandTarget != "" {
		state.CommandPanePresent = shellPaneExistsFn(repoRoot, state.CommandTarget)
	}
	return state, nil
}

func ensureShellWorkflowWorker(repoRoot, session, dialtoneShell, reasoning, model, rightTitle string, wait time.Duration) error {
	state, err := inspectShellWorkflowState(repoRoot)
	if err != nil {
		return err
	}
	if state.readyForVisibleCommands() {
		return nil
	}
	if strings.TrimSpace(state.PromptTarget) == "" ||
		strings.TrimSpace(state.CommandTarget) == "" ||
		!state.PromptPanePresent ||
		!state.CommandPanePresent {
		return invokeShellRunStart([]string{
			"--session", strings.TrimSpace(session),
			"--dialtone-shell", strings.TrimSpace(dialtoneShell),
			"--reasoning", strings.TrimSpace(reasoning),
			"--model", strings.TrimSpace(model),
			"--right-title", strings.TrimSpace(rightTitle),
			"--wait-seconds", fmt.Sprintf("%d", int(wait.Seconds())),
			"--run-tests=false",
		})
	}
	return invokeShellStartWorker(repoRoot, strings.TrimSpace(state.CommandTarget), strings.TrimSpace(dialtoneShell), wait)
}

func ensureVisibleShellWorkflow(repoRoot, session, dialtoneShell, reasoning, model, rightTitle string, wait time.Duration) error {
	if err := ensureShellWorkflowWorker(repoRoot, session, dialtoneShell, reasoning, model, rightTitle, wait); err != nil {
		return err
	}
	state, err := inspectShellWorkflowState(repoRoot)
	if err != nil {
		return err
	}
	if !state.readyForVisibleCommands() {
		return fmt.Errorf("shell workflow is not ready: %s", state.problemSummary())
	}
	return nil
}

func waitForShellBusRowExecution(db *sql.DB, repoRoot string, rowID int64, wait time.Duration) error {
	healthy, err := shellWorkerHealthy(db, 5*time.Second)
	if err != nil {
		return err
	}
	if !healthy {
		row, ok, err := modstate.LoadShellBusRecord(db, rowID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("shell bus row not found after enqueue: %d", rowID)
		}
		if err := syncShellBusRow(db, repoRoot, row, wait); err != nil {
			return err
		}
	} else {
		if _, err := waitForShellBusCompletion(db, rowID, wait); err != nil {
			return err
		}
	}
	record, ok, err := modstate.LoadShellBusRecord(db, rowID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("shell bus row not found after execution: %d", rowID)
	}
	if record.Status != "failed" {
		return nil
	}
	body, decodeErr := dispatch.DecodeIntentBody(record.BodyJSON)
	if decodeErr == nil && strings.TrimSpace(body.Error) != "" {
		return fmt.Errorf("dialtone-view command failed: %s", body.Error)
	}
	return fmt.Errorf("dialtone-view command failed [row_id=%d]", rowID)
}

func waitForShellBusCompletion(db *sql.DB, rowID int64, timeout time.Duration) (modstate.ShellBusRecord, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		record, ok, err := modstate.LoadShellBusRecord(db, rowID)
		if err != nil {
			return modstate.ShellBusRecord{}, err
		}
		if ok && record.Status != "queued" && record.Status != "running" {
			return record, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return modstate.ShellBusRecord{}, fmt.Errorf("timed out waiting for dialtone-view command row %d", rowID)
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
	if row.Subject == "command" && row.Action == "run" {
		ensureShellBusCommandLogPath(repoRoot, row.ID, &body)
	}

	switch {
	case row.Subject == "prompt" && row.Action == "submit":
		updated, _ := json.Marshal(body)
		if err := markShellBusRowRunning(db, row.ID, string(updated)); err != nil {
			return err
		}
		if _, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "write", "--pane", target, "--enter", body.Text); err != nil {
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
		if _, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "write", "--pane", target, "--enter", body.Command); err != nil {
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
		out, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", target, "--lines", "120")
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
	status := "done"
	if row.Subject == "command" {
		body.Output = snapshotText
		if exitCode, ok := trackedCommandExitStatus(snapshotText); ok && exitCode != 0 {
			status = "failed"
			body.Error = fmt.Sprintf("command exited with status %d", exitCode)
		}
		if err := writeShellBusCommandLog(repoRoot, row.ID, body, status); err != nil {
			return err
		}
	}
	updated, _ := json.Marshal(body)
	if err := modstate.UpdateShellBusStatus(db, row.ID, status, observedID, string(updated)); err != nil {
		return err
	}
	return nil
}

func syncShellBusRowInWorker(db *sql.DB, repoRoot string, row modstate.ShellBusRecord, workerPane string) error {
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
	if row.Subject == "command" && row.Action == "run" {
		ensureShellBusCommandLogPath(repoRoot, row.ID, &body)
	}
	body.StartedAt = time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(body.DisplayCommand) == "" {
		switch {
		case strings.TrimSpace(body.InnerCommand) != "":
			body.DisplayCommand = body.InnerCommand
		case strings.TrimSpace(body.Command) != "":
			body.DisplayCommand = body.Command
		}
	}
	updated, _ := json.Marshal(body)
	if err := markShellBusRowRunning(db, row.ID, string(updated)); err != nil {
		return err
	}

	switch {
	case row.Subject == "prompt" && row.Action == "submit":
		if err := setShellWorkerState(db, map[string]string{
			sqlitestate.ShellWorkerStatusKey:         "running",
			sqlitestate.ShellWorkerPaneKey:           workerPane,
			sqlitestate.ShellWorkerHeartbeatKey:      time.Now().UTC().Format(time.RFC3339),
			sqlitestate.ShellWorkerCurrentRowIDKey:   strconv.FormatInt(row.ID, 10),
			sqlitestate.ShellWorkerCurrentCommandKey: body.Text,
		}); err != nil {
			return err
		}
		if _, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "write", "--pane", target, "--enter", body.Text); err != nil {
			body.Error = err.Error()
			body.FinishedAt = time.Now().UTC().Format(time.RFC3339)
			updated, _ := json.Marshal(body)
			_ = modstate.UpdateShellBusStatus(db, row.ID, "failed", 0, string(updated))
			return err
		}
		body.Summary = "submitted prompt"
		body.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		updated, _ = json.Marshal(body)
		if err := modstate.UpdateShellBusStatus(db, row.ID, "done", 0, string(updated)); err != nil {
			return err
		}
		return setShellWorkerState(db, map[string]string{
			sqlitestate.ShellWorkerStatusKey:         "running",
			sqlitestate.ShellWorkerPaneKey:           workerPane,
			sqlitestate.ShellWorkerHeartbeatKey:      time.Now().UTC().Format(time.RFC3339),
			sqlitestate.ShellWorkerCurrentRowIDKey:   "",
			sqlitestate.ShellWorkerCurrentCommandKey: "",
			sqlitestate.ShellWorkerLastRowIDKey:      strconv.FormatInt(row.ID, 10),
			sqlitestate.ShellWorkerLastStatusKey:     "done",
			sqlitestate.ShellWorkerLastSummaryKey:    body.Summary,
			sqlitestate.ShellWorkerLastExitCodeKey:   "0",
		})
	case row.Subject == "command" && row.Action == "run":
		if strings.TrimSpace(target) != strings.TrimSpace(workerPane) {
			return fmt.Errorf("shell worker pane mismatch: target=%s worker=%s", target, workerPane)
		}
		if err := setShellWorkerState(db, map[string]string{
			sqlitestate.ShellWorkerStatusKey:         "running",
			sqlitestate.ShellWorkerPaneKey:           workerPane,
			sqlitestate.ShellWorkerHeartbeatKey:      time.Now().UTC().Format(time.RFC3339),
			sqlitestate.ShellWorkerCurrentRowIDKey:   strconv.FormatInt(row.ID, 10),
			sqlitestate.ShellWorkerCurrentCommandKey: body.DisplayCommand,
		}); err != nil {
			return err
		}
		if strings.TrimSpace(body.DisplayCommand) != "" {
			fmt.Printf("$ %s\n", strings.TrimSpace(body.DisplayCommand))
		}
		output, exitCode, pid, runtime, execErr := runVisibleCommand(repoRoot, workerPane, body.Command, func(pid int) error {
			body.PID = pid
			updated, _ := json.Marshal(body)
			return modstate.UpdateShellBusStatus(db, row.ID, "running", 0, string(updated))
		})
		body.Output = output
		body.PID = pid
		body.ExitCode = exitCode
		body.RuntimeMS = runtime.Milliseconds()
		body.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		body.Summary = summarizeSnapshot(output)
		status := "done"
		if execErr != nil {
			status = "failed"
			body.Error = deriveVisibleCommandError(output, execErr, exitCode)
		} else if exitCode != 0 {
			status = "failed"
			body.Error = deriveVisibleCommandError(output, nil, exitCode)
		}
		if err := writeShellBusCommandLog(repoRoot, row.ID, body, status); err != nil {
			return err
		}
		observedBody, _ := json.Marshal(map[string]any{
			"command":   body.DisplayCommand,
			"output":    body.Output,
			"summary":   body.Summary,
			"pid":       body.PID,
			"exitCode":  body.ExitCode,
			"runtimeMs": body.RuntimeMS,
			"logPath":   body.LogPath,
		})
		observedID, err := modstate.AppendShellBusObserved(db, "shell", "command", "result", "shell-worker", row.Session, target, row.ID, string(observedBody))
		if err != nil {
			return err
		}
		updated, _ = json.Marshal(body)
		if err := modstate.UpdateShellBusStatus(db, row.ID, status, observedID, string(updated)); err != nil {
			return err
		}
		if err := captureCurrentShellState(db, repoRoot, row.ID); err != nil {
			return err
		}
		return setShellWorkerState(db, map[string]string{
			sqlitestate.ShellWorkerStatusKey:         "running",
			sqlitestate.ShellWorkerPaneKey:           workerPane,
			sqlitestate.ShellWorkerHeartbeatKey:      time.Now().UTC().Format(time.RFC3339),
			sqlitestate.ShellWorkerCurrentRowIDKey:   "",
			sqlitestate.ShellWorkerCurrentCommandKey: "",
			sqlitestate.ShellWorkerLastRowIDKey:      strconv.FormatInt(row.ID, 10),
			sqlitestate.ShellWorkerLastStatusKey:     status,
			sqlitestate.ShellWorkerLastSummaryKey:    body.Summary,
			sqlitestate.ShellWorkerLastExitCodeKey:   strconv.Itoa(body.ExitCode),
		})
	default:
		return fmt.Errorf("unsupported shell bus action: %s/%s", row.Subject, row.Action)
	}
}

func buildVisibleCommandEnv(baseEnv []string, paneTarget string) []string {
	env := append([]string{}, baseEnv...)
	env = append(env, "TERM=xterm-256color")
	if target := strings.TrimSpace(paneTarget); target != "" {
		env = append(env,
			"DIALTONE_TMUX_PROXY_ACTIVE=1",
			"DIALTONE_TMUX_TARGET="+target,
		)
	}
	return env
}

func runVisibleCommand(repoRoot, paneTarget, command string, onStart func(pid int) error) (string, int, int, time.Duration, error) {
	cmd := exec.Command("zsh", "-lc", command)
	cmd.Dir = strings.TrimSpace(repoRoot)
	cmd.Env = buildVisibleCommandEnv(os.Environ(), paneTarget)
	var output bytes.Buffer
	stdout := io.MultiWriter(os.Stdout, &output)
	stderr := io.MultiWriter(os.Stderr, &output)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		return "", -1, 0, 0, err
	}
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	if onStart != nil {
		if err := onStart(pid); err != nil {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_ = cmd.Wait()
			return output.String(), -1, pid, time.Since(startedAt), err
		}
	}
	err := cmd.Wait()
	runtime := time.Since(startedAt)
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		return output.String(), exitCode, pid, runtime, err
	}
	return output.String(), exitCode, pid, runtime, nil
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
		return runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", target, "--lines", "120")
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
	rowID := opts.Int64("row-id", 0, "Optional exact shell_bus row id to print")
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
	workerStatus, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerStatusKey)
	workerPane, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerPaneKey)
	workerHeartbeat, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerHeartbeatKey)
	workerCurrentRow, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerCurrentRowIDKey)
	workerCurrentCommand, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerCurrentCommandKey)
	workerLastRow, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerLastRowIDKey)
	workerLastStatus, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerLastStatusKey)
	workerLastSummary, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerLastSummaryKey)
	workerLastExitCode, _ := loadOptionalStateValue(db, sqlitestate.ShellWorkerLastExitCodeKey)
	rows, err := modstate.LoadShellBus(db, "observed", *limit)
	if err != nil {
		return err
	}
	desiredRows, err := modstate.LoadShellBus(db, "desired", *limit)
	if err != nil {
		return err
	}
	promptSnapshot := latestPaneSnapshot(rows, promptTarget)
	commandSnapshot := latestPaneSnapshot(rows, commandTarget)
	latestCommandRow, latestCommand, hasLatestCommand := latestDesiredCommandRecord(desiredRows)
	if *rowID > 0 {
		record, ok, err := modstate.LoadShellBusRecord(db, *rowID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("shell bus row %d not found", *rowID)
		}
		latestCommandRow = record
		latestCommand, _ = decodeShellBusIntentBody(record)
		hasLatestCommand = true
	}
	fmt.Printf("prompt_target\t%s\ncommand_target\t%s\nqueued\t%d\n", printableTarget(promptTarget), printableTarget(commandTarget), len(queued))
	if workerStatus != "" {
		fmt.Printf("worker_status\t%s\n", workerStatus)
	}
	if workerPane != "" {
		fmt.Printf("worker_pane\t%s\n", workerPane)
	}
	if workerHeartbeat != "" {
		fmt.Printf("worker_heartbeat\t%s\n", workerHeartbeat)
	}
	if workerCurrentRow != "" {
		fmt.Printf("worker_current_row\t%s\n", workerCurrentRow)
	}
	if workerCurrentCommand != "" {
		fmt.Printf("worker_current_command\t%s\n", workerCurrentCommand)
	}
	if workerLastRow != "" {
		fmt.Printf("worker_last_row\t%s\n", workerLastRow)
	}
	if workerLastStatus != "" {
		fmt.Printf("worker_last_status\t%s\n", workerLastStatus)
	}
	if workerLastExitCode != "" {
		fmt.Printf("worker_last_exit_code\t%s\n", workerLastExitCode)
	}
	if workerLastSummary != "" {
		fmt.Printf("worker_last_summary\t%s\n", workerLastSummary)
	}
	if hasLatestCommand {
		printShellBusCommandStatus("command", latestCommandRow, latestCommand, *full)
		if strings.TrimSpace(latestCommand.DisplayCommand) != "" {
			fmt.Printf("last_command\t%s\n", latestCommand.DisplayCommand)
		}
	}
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
	_, err = runDialtoneModQuiet(repoRoot, "tmux", "v1", "write", "--pane", target, "--enter", text)
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
	_, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "write", "--pane", target, "--enter", command)
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

func decodeShellBusIntentBody(row modstate.ShellBusRecord) (shellBusIntentBody, bool) {
	body := shellBusIntentBody{}
	if err := json.Unmarshal([]byte(row.BodyJSON), &body); err != nil {
		return shellBusIntentBody{}, false
	}
	if strings.TrimSpace(body.DisplayCommand) == "" {
		if strings.TrimSpace(body.InnerCommand) != "" {
			body.DisplayCommand = body.InnerCommand
		} else {
			body.DisplayCommand = body.Command
		}
	}
	if strings.TrimSpace(body.Target) == "" {
		body.Target = strings.TrimSpace(row.Pane)
	}
	return body, true
}

func latestDesiredCommandRecord(rows []modstate.ShellBusRecord) (modstate.ShellBusRecord, shellBusIntentBody, bool) {
	for _, row := range rows {
		if row.Subject != "command" || row.Action != "run" {
			continue
		}
		body, ok := decodeShellBusIntentBody(row)
		if !ok {
			continue
		}
		return row, body, true
	}
	return modstate.ShellBusRecord{}, shellBusIntentBody{}, false
}

func printShellBusCommandStatus(prefix string, row modstate.ShellBusRecord, body shellBusIntentBody, full bool) {
	label := strings.TrimSpace(prefix)
	if label == "" {
		label = "command"
	}
	fmt.Printf("%s_row_id\t%d\n", label, row.ID)
	fmt.Printf("%s_status\t%s\n", label, strings.TrimSpace(row.Status))
	if strings.TrimSpace(body.DisplayCommand) != "" {
		fmt.Printf("%s\t%s\n", label, strings.TrimSpace(body.DisplayCommand))
	}
	if strings.TrimSpace(body.Target) != "" {
		fmt.Printf("%s_target\t%s\n", label, strings.TrimSpace(body.Target))
	}
	if strings.TrimSpace(body.LogPath) != "" {
		fmt.Printf("%s_log_path\t%s\n", label, strings.TrimSpace(body.LogPath))
	}
	if strings.TrimSpace(body.StartedAt) != "" {
		fmt.Printf("%s_started_at\t%s\n", label, strings.TrimSpace(body.StartedAt))
	}
	if strings.TrimSpace(body.FinishedAt) != "" {
		fmt.Printf("%s_finished_at\t%s\n", label, strings.TrimSpace(body.FinishedAt))
	}
	if body.PID > 0 {
		fmt.Printf("%s_pid\t%d\n", label, body.PID)
	} else {
		fmt.Printf("%s_pid\tpending\n", label)
	}
	if body.ExitCode != 0 || row.Status == "done" || row.Status == "failed" {
		fmt.Printf("%s_exit_code\t%d\n", label, body.ExitCode)
	} else {
		fmt.Printf("%s_exit_code\tpending\n", label)
	}
	if runtimeMS, ok := shellBusRuntimeMillis(row, body); ok {
		fmt.Printf("%s_runtime_ms\t%d\n", label, runtimeMS)
	} else {
		fmt.Printf("%s_runtime_ms\tpending\n", label)
	}
	if strings.TrimSpace(body.Summary) != "" {
		fmt.Printf("%s_summary\t%s\n", label, strings.TrimSpace(body.Summary))
	}
	if strings.TrimSpace(body.Error) != "" {
		fmt.Printf("%s_error\t%s\n", label, strings.TrimSpace(body.Error))
	}
	if full && strings.TrimSpace(body.Output) != "" {
		fmt.Printf("%s_output\n%s\n", label, strings.TrimSpace(body.Output))
		fmt.Printf("last_%s_output\n%s\n", label, strings.TrimSpace(body.Output))
	}
}

func shellBusRuntimeMillis(row modstate.ShellBusRecord, body shellBusIntentBody) (int64, bool) {
	if body.RuntimeMS > 0 {
		return body.RuntimeMS, true
	}
	startedAt, ok := parseSQLiteTime(body.StartedAt)
	if !ok {
		return 0, false
	}
	finishedAt, ok := parseSQLiteTime(body.FinishedAt)
	if !ok {
		if row.Status == "running" {
			finishedAt = time.Now().UTC()
			ok = true
		} else if updatedAt, updatedOK := parseSQLiteTime(row.UpdatedAt); updatedOK {
			finishedAt = updatedAt
			ok = true
		}
	}
	if !ok || finishedAt.Before(startedAt) {
		return 0, false
	}
	return finishedAt.Sub(startedAt).Milliseconds(), true
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

func deriveVisibleCommandError(output string, execErr error, exitCode int) string {
	fallback := ""
	switch {
	case execErr != nil:
		fallback = strings.TrimSpace(execErr.Error())
	case exitCode != 0:
		fallback = fmt.Sprintf("command exited with status %d", exitCode)
	}

	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if strings.HasPrefix(line, "probe_error\t") {
			return strings.TrimSpace(strings.TrimPrefix(line, "probe_error\t"))
		}
		if strings.HasPrefix(line, "error\t") {
			return strings.TrimSpace(strings.TrimPrefix(line, "error\t"))
		}
	}
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "exit status ") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "probe_mode\t"),
			strings.HasPrefix(line, "probe_label\t"),
			strings.HasPrefix(line, "probe_pid\t"),
			strings.HasPrefix(line, "probe_started_at\t"),
			strings.HasPrefix(line, "probe_sleep_ms\t"),
			strings.HasPrefix(line, "probe_finished_at\t"),
			strings.HasPrefix(line, "probe_result\t"),
			strings.HasPrefix(line, "probe_background_"):
			continue
		}
		return line
	}
	return fallback
}

func waitForPaneContains(repoRoot, pane, needle string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	lastOutput := ""
	for time.Now().Before(deadline) {
		out, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", pane, "--lines", "160")
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
		if strings.TrimSpace(promptTarget) != "" {
			if out, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", promptTarget, "--lines", fmt.Sprintf("%d", *readLines)); err == nil {
				fmt.Printf("prompt_pane_read\n%s\n", strings.TrimRight(out, "\n"))
			}
		}
		if strings.TrimSpace(commandTarget) != "" {
			if out, err := runDialtoneModQuiet(repoRoot, "tmux", "v1", "read", "--pane", commandTarget, "--lines", fmt.Sprintf("%d", *readLines)); err == nil {
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
	return loadStateTargetFromDB(db, key)
}

func loadStateTargetFromDB(db *sql.DB, key string) (string, error) {
	if db == nil {
		return "", errors.New("loadStateTargetFromDB requires an open sqlite db handle")
	}
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

func loadOptionalStateValue(db *sql.DB, key string) (string, error) {
	record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(record.Value), nil
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
	fmt.Println("  install")
	fmt.Println("       Verify bash, Go, and tmux are available in the default nix shell")
	fmt.Println("  build")
	fmt.Println("       Build the shell v1 CLI wrapper to <repo-root>/bin/mods/shell/v1/shell")
	fmt.Println("  format [--dir DIR]")
	fmt.Println("       Run gofmt on shell v1 Go files")
	fmt.Println("  start [--session codex-view] [--shell default|ssh-v1] [--dialtone-shell ssh-v1] [--reasoning medium] [--model gpt-5.4] [--wait-seconds 20] [--run-tests=true]")
	fmt.Println("       Quit Ghostty, create one fresh window with one tab, start/attach codex-view, split dialtone-view on the right, launch Codex left, and optionally run the sqlite test plan on the right")
	fmt.Println("  split-vertical [--session codex-view] [--shell default|ssh-v1] [--right-title dialtone-view] [--wait-seconds 10]")
	fmt.Println("       Split the tmux window so Codex stays on the left and a dialtone shell pane titled dialtone-view appears on the right")
	fmt.Println("  prompt [--session codex-view] [--pane codex-view:0:0] <text...>")
	fmt.Println("       Queue a prompt intent on the SQLite shell bus for codex-view")
	fmt.Println("  enqueue-command [--session codex-view] [--pane codex-view:0:1] [--expect TEXT] <command...>")
	fmt.Println("       Queue a visible command intent on the SQLite shell bus for dialtone-view")
	fmt.Println("  run [--session codex-view] [--pane codex-view:0:1] [--expect TEXT] [--wait-seconds 10] <command...>")
	fmt.Println("       Queue one visible command through SQLite and wait for the dialtone-view worker or fallback sync")
	fmt.Println("  test [--wait-seconds 60]")
	fmt.Println("       Run the shell workflow Go suite visibly in dialtone-view through the SQLite shell bus")
	fmt.Println("  test-basic")
	fmt.Println("       Run the core SQLite and shell Go tests first before starting the visible Ghostty workflow")
	fmt.Println("  test-all [--wait-seconds 120]")
	fmt.Println("       Run the full mod Go test sweep visibly in dialtone-view through the SQLite shell bus")
	fmt.Println("  workflow [--wait-seconds 120]")
	fmt.Println("       Run basic tests first, start the shell workflow, then run the remaining full test sweep in dialtone-view")
	fmt.Println("  sync-once [--limit 10] [--wait-seconds 10]")
	fmt.Println("       Reconcile queued SQLite shell bus intents into the live tmux panes once")
	fmt.Println("  serve [--pane codex-view:0:1] [--poll-interval-ms 500]")
	fmt.Println("       Run the long-lived SQLite shell worker inside dialtone-view")
	fmt.Println("  ensure-worker [--session codex-view] [--dialtone-shell default] [--wait-seconds 20]")
	fmt.Println("       Start the workflow or restart the dialtone-view worker if it is missing")
	fmt.Println("  state [--limit 40] [--row-id 123] [--full]")
	fmt.Println("       Auto-refresh and read prompt/command targets, worker status, and the latest or selected shell_bus command row from SQLite")
	fmt.Println("  status [--limit 40] [--row-id 123] [--full]")
	fmt.Println("       Alias for state")
	fmt.Println("  read [--role prompt|command] [--pane codex-view:0:1] [--full=true] [--sync=true]")
	fmt.Println("       Auto-refresh and print one pane snapshot from SQLite without reading tmux directly yourself")
	fmt.Println("  events [--scope desired|observed] [--limit 20]")
	fmt.Println("       Auto-refresh and print recent SQLite shell bus rows")
	fmt.Println("  demo-protocol [--bootstrap=true|false] [--dialtone-shell ssh-v1] [--wait-seconds 20] [--command './dialtone_mod mods v1 db graph --format outline']")
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
