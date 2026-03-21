package main

import (
	"bytes"
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

	if err := ensureSplitLayout(repoRoot, strings.TrimSpace(*session), strings.TrimSpace(*shellName), strings.TrimSpace(*rightTitle), time.Duration(*waitSeconds)*time.Second); err != nil {
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
	pane := opts.String("pane", "", "Explicit tmux prompt pane override (default: sqlite tmux.prompt_target)")
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
	target := strings.TrimSpace(*pane)
	if target == "" {
		target, err = loadStateTarget(repoRoot, sqlitestate.TmuxPromptTargetKey)
		if err != nil {
			return err
		}
	}
	if target == "" {
		return errors.New("no codex prompt target configured")
	}
	text := strings.TrimSpace(strings.Join(opts.Args(), " "))
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return err
	}
	defer db.Close()
	queueID, err := submitPrompt(db, repoRoot, target, text)
	if err != nil {
		return err
	}
	fmt.Printf("submitted prompt to %s [queue_id=%d]\n", target, queueID)
	return nil
}

func runDemoProtocol(argv []string) error {
	opts := flag.NewFlagSet("shell v1 demo-protocol", flag.ContinueOnError)
	session := opts.String("session", "codex-view", "tmux session name to use for the protocol demo")
	shellName := opts.String("shell", "default", "flake shell for the right-side dialtone pane")
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
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(record.Value), nil
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
	fmt.Println("  start [--session codex-view] [--shell default|repl-v1|ssh-v1] [--reasoning medium] [--model gpt-5.4] [--wait-seconds 20] [--run-tests=true]")
	fmt.Println("       Quit Ghostty, create one fresh window with one tab, start/attach codex-view, split dialtone-view on the right, launch Codex left, and optionally run the sqlite test plan on the right")
	fmt.Println("  split-vertical [--session codex-view] [--shell default|repl-v1|ssh-v1] [--right-title dialtone-view] [--wait-seconds 10]")
	fmt.Println("       Split the tmux window so Codex stays on the left and a dialtone shell pane titled dialtone-view appears on the right")
	fmt.Println("  prompt [--pane codex-view:0:0] <text...>")
	fmt.Println("       Submit a prompt into the sqlite-configured codex prompt pane and record it in SQLite")
	fmt.Println("  demo-protocol [--bootstrap=true|false] [--wait-seconds 20] [--command 'env DIALTONE_TMUX_PROXY_ACTIVE=1 ./dialtone_mod mods v1 db graph --format outline']")
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
