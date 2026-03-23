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
	"regexp"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/sqlitestate"
)

const (
	defaultSession       = "codex-view"
	defaultWaitSeconds   = 30
	defaultRoutedCommand = "./dialtone_mod mods v1 db graph --format outline"
	defaultCommandExpect = "- shell:v1"
	commandPaneExpect    = "- test:v1"
	protocolRunName      = "test-v1-start"
)

var rowIDPattern = regexp.MustCompile(`row_id=(\d+)`)

type startOptions struct {
	session       string
	waitSeconds   int
	model         string
	reasoning     string
	dialtoneShell string
	rightTitle    string
	prompt        string
}

type commandContext struct {
	status     string
	command    string
	target     string
	summary    string
	errorText  string
	output     string
	startedAt  string
	finishedAt string
	pid        int
	exitCode   int
	runtimeMS  int64
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
	case "start":
		if err := runStart(args); err != nil {
			exitIfErr(err, "test start")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown test command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runStart(argv []string) error {
	opts, err := parseStartArgs(argv)
	if err != nil {
		return err
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

	var report bytes.Buffer
	writeLine := func(format string, args ...any) {
		fmt.Fprintf(&report, format, args...)
		if !strings.HasSuffix(format, "\n") {
			report.WriteByte('\n')
		}
	}
	writeSection := func(name, text string) {
		writeLine("")
		writeLine("[%s]", name)
		writeLine("%s", strings.TrimRight(text, "\n"))
	}

	token := fmt.Sprintf("DIALTONE_TEST_V1_%d", time.Now().UTC().Unix())
	promptText := buildPromptText(token, opts.prompt)

	runID := int64(0)
	finishStatus := "failed"
	finishResult := ""
	finishError := ""
	defer func() {
		if runID > 0 {
			_ = modstate.FinishProtocolRun(db, runID, finishStatus, finishResult, finishError)
		}
	}()

	ensureOutput, err := runDialtoneModCapture(repoRoot,
		"shell", "v1", "ensure-worker",
		"--session", opts.session,
		"--dialtone-shell", opts.dialtoneShell,
		"--reasoning", opts.reasoning,
		"--model", opts.model,
		"--right-title", opts.rightTitle,
		"--wait-seconds", strconv.Itoa(opts.waitSeconds),
	)
	if err != nil {
		writeSection("test.ensure-worker", ensureOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.ensure-worker", ensureOutput)

	promptTarget, err := requiredStateValue(db, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		fmt.Print(report.String())
		return err
	}
	commandTarget, err := requiredStateValue(db, sqlitestate.TmuxTargetKey)
	if err != nil {
		fmt.Print(report.String())
		return err
	}

	runID, err = modstate.StartProtocolRun(db, protocolRunName, promptText, promptTarget, commandTarget)
	if err != nil {
		fmt.Print(report.String())
		return err
	}
	eventIndex := 0
	appendEvent := func(eventType string, queueRowID int64, paneTarget, commandText, messageText string) error {
		eventIndex++
		return modstate.AppendProtocolEvent(db, modstate.ProtocolEventRecord{
			RunID:       runID,
			EventIndex:  eventIndex,
			EventType:   strings.TrimSpace(eventType),
			QueueName:   "shell_bus",
			QueueRowID:  queueRowID,
			PaneTarget:  strings.TrimSpace(paneTarget),
			CommandText: strings.TrimSpace(commandText),
			MessageText: strings.TrimSpace(messageText),
		})
	}
	if err := appendEvent("workflow_ready", 0, "", "", fmt.Sprintf("prompt=%s command=%s", promptTarget, commandTarget)); err != nil {
		fmt.Print(report.String())
		return err
	}

	promptOutput, err := runDialtoneModCapture(repoRoot,
		"shell", "v1", "prompt",
		"--session", opts.session,
		"--sync=true",
		promptText,
	)
	if err != nil {
		finishError = err.Error()
		writeSection("test.prompt", promptOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.prompt", promptOutput)
	promptRowID, err := parseBracketRowID(promptOutput)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	if err := appendEvent("prompt_submitted", promptRowID, promptTarget, promptText, "submitted prompt to codex-view through shell bus"); err != nil {
		fmt.Print(report.String())
		return err
	}

	routeOutput, err := runDialtoneModCapture(repoRoot, "mods", "v1", "db", "graph", "--format", "outline")
	if err != nil {
		finishError = err.Error()
		writeSection("test.route", routeOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.route", routeOutput)
	commandRowID, err := parseCommandID(routeOutput)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	if err := appendEvent("command_queued", commandRowID, commandTarget, defaultRoutedCommand, "queued plain routed mod command through dialtone"); err != nil {
		fmt.Print(report.String())
		return err
	}

	record, err := waitForShellBusRecord(db, commandRowID, time.Duration(opts.waitSeconds)*time.Second)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	commandBody, err := decodeCommandContext(record)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	if err := appendEvent("command_observed", commandRowID, commandTarget, defaultRoutedCommand, fmt.Sprintf("status=%s exit=%d pid=%d runtime_ms=%d", record.Status, commandBody.exitCode, commandBody.pid, commandBody.runtimeMS)); err != nil {
		fmt.Print(report.String())
		return err
	}

	statusOutput, err := runDialtoneModCapture(repoRoot,
		"shell", "v1", "status",
		"--row-id", strconv.FormatInt(commandRowID, 10),
		"--full",
		"--sync=false",
	)
	if err != nil {
		finishError = err.Error()
		writeSection("test.status", statusOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.status", statusOutput)

	promptReadOutput, err := runDialtoneModCapture(repoRoot, "shell", "v1", "read", "--role", "prompt", "--full", "--sync=true")
	if err != nil {
		finishError = err.Error()
		writeSection("test.prompt-pane", promptReadOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.prompt-pane", promptReadOutput)

	commandReadOutput, err := runDialtoneModCapture(repoRoot, "shell", "v1", "read", "--role", "command", "--full", "--sync=true")
	if err != nil {
		finishError = err.Error()
		writeSection("test.command-pane", commandReadOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.command-pane", commandReadOutput)

	if err := validateE2EOutputs(token, routeOutput, statusOutput, promptReadOutput, commandReadOutput, commandBody); err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}

	selectedState, err := loadSelectedSystemState(db)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	writeSection("test.sqlite-state", selectedState)

	events, err := modstate.LoadProtocolEvents(db, runID)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	writeSection("test.protocol-events", renderProtocolEvents(events))

	finishStatus = "passed"
	finishResult = fmt.Sprintf("prompt_row=%d command_row=%d exit_code=%d runtime_ms=%d", promptRowID, commandRowID, commandBody.exitCode, commandBody.runtimeMS)

	writeLine("test_result\tpassed")
	writeLine("protocol_run_id\t%d", runID)
	writeLine("prompt_row_id\t%d", promptRowID)
	writeLine("command_row_id\t%d", commandRowID)
	writeLine("command_status\t%s", record.Status)
	writeLine("command_pid\t%d", commandBody.pid)
	writeLine("command_exit_code\t%d", commandBody.exitCode)
	writeLine("command_runtime_ms\t%d", commandBody.runtimeMS)
	writeLine("state_db\t%s", sqlitestate.ResolveStateDBPath(repoRoot))
	writeLine("prompt_target\t%s", promptTarget)
	writeLine("command_target\t%s", commandTarget)
	writeLine("routed_command\t%s", defaultRoutedCommand)
	writeLine("inspect\t./dialtone_mod shell v1 status --row-id %d --full --sync=false", commandRowID)
	fmt.Print(report.String())
	return nil
}

func parseStartArgs(argv []string) (startOptions, error) {
	opts := flag.NewFlagSet("test v1 start", flag.ContinueOnError)
	session := opts.String("session", defaultSession, "tmux session name to use for the end-to-end test")
	waitSeconds := opts.Int("wait-seconds", defaultWaitSeconds, "Seconds to wait for the routed command to complete")
	model := opts.String("model", "gpt-5.4", "Codex model to launch if the workflow is missing")
	reasoning := opts.String("reasoning", "medium", "Reasoning label for the Codex startup banner")
	dialtoneShell := opts.String("dialtone-shell", "default", "flake shell to keep active in dialtone-view")
	rightTitle := opts.String("right-title", "dialtone-view", "Title for the right-side tmux pane")
	prompt := opts.String("prompt", "", "Optional explicit prompt text to submit to codex-view")
	if err := opts.Parse(argv); err != nil {
		return startOptions{}, err
	}
	if opts.NArg() != 0 {
		return startOptions{}, errors.New("start does not accept positional arguments")
	}
	if strings.TrimSpace(*session) == "" {
		return startOptions{}, errors.New("--session is required")
	}
	if *waitSeconds <= 0 {
		return startOptions{}, errors.New("--wait-seconds must be positive")
	}
	return startOptions{
		session:       strings.TrimSpace(*session),
		waitSeconds:   *waitSeconds,
		model:         strings.TrimSpace(*model),
		reasoning:     strings.TrimSpace(*reasoning),
		dialtoneShell: strings.TrimSpace(*dialtoneShell),
		rightTitle:    strings.TrimSpace(*rightTitle),
		prompt:        strings.TrimSpace(*prompt),
	}, nil
}

func buildPromptText(token, custom string) string {
	if strings.TrimSpace(custom) != "" {
		return strings.TrimSpace(custom)
	}
	return fmt.Sprintf("Dialtone test v1 start %s: this is the SQLite-backed end-to-end system test. No reply is required.", strings.TrimSpace(token))
}

func locateRepoRoot() (string, error) {
	if envRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); envRoot != "" {
		candidate := filepath.Clean(envRoot)
		if isRepoRoot(candidate) {
			return candidate, nil
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd = filepath.Clean(cwd)
	for {
		if isRepoRoot(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("unable to locate repo root from %s", cwd)
}

func isRepoRoot(candidate string) bool {
	if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(candidate, "src", "go.mod"))
	return err == nil
}

func runDialtoneModCapture(repoRoot string, args ...string) (string, error) {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone_mod"), args...)
	cmd.Dir = strings.TrimSpace(repoRoot)
	cmd.Env = os.Environ()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return strings.TrimRight(out.String(), "\n"), err
}

func requiredStateValue(db *sql.DB, key string) (string, error) {
	record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, strings.TrimSpace(key))
	if err != nil {
		return "", err
	}
	if !ok || strings.TrimSpace(record.Value) == "" {
		return "", fmt.Errorf("missing sqlite system state key %s", strings.TrimSpace(key))
	}
	return strings.TrimSpace(record.Value), nil
}

func parseCommandID(output string) (int64, error) {
	return parseKVInt64(output, "command_id")
}

func parseKVInt64(output, key string) (int64, error) {
	prefix := strings.TrimSpace(key) + "\t"
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		rowID, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s value %q: %w", key, value, err)
		}
		return rowID, nil
	}
	return 0, fmt.Errorf("%s not found in output", key)
}

func parseBracketRowID(output string) (int64, error) {
	match := rowIDPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, fmt.Errorf("row_id not found in output")
	}
	rowID, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid row_id %q: %w", match[1], err)
	}
	return rowID, nil
}

func waitForShellBusRecord(db *sql.DB, rowID int64, timeout time.Duration) (modstate.ShellBusRecord, error) {
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
	return modstate.ShellBusRecord{}, fmt.Errorf("timed out waiting for shell bus row %d", rowID)
}

func decodeCommandContext(row modstate.ShellBusRecord) (commandContext, error) {
	body, err := dispatch.DecodeIntentBody(row.BodyJSON)
	if err != nil {
		return commandContext{}, err
	}
	commandText := strings.TrimSpace(body.DisplayCommand)
	if commandText == "" {
		if strings.TrimSpace(body.InnerCommand) != "" {
			commandText = strings.TrimSpace(body.InnerCommand)
		} else {
			commandText = strings.TrimSpace(body.Command)
		}
	}
	target := strings.TrimSpace(body.Target)
	if target == "" {
		target = strings.TrimSpace(row.Pane)
	}
	return commandContext{
		status:     strings.TrimSpace(row.Status),
		command:    commandText,
		target:     target,
		summary:    strings.TrimSpace(body.Summary),
		errorText:  strings.TrimSpace(body.Error),
		output:     body.Output,
		startedAt:  strings.TrimSpace(body.StartedAt),
		finishedAt: strings.TrimSpace(body.FinishedAt),
		pid:        body.PID,
		exitCode:   body.ExitCode,
		runtimeMS:  body.RuntimeMS,
	}, nil
}

func validateE2EOutputs(promptToken, routeOutput, statusOutput, promptReadOutput, commandReadOutput string, command commandContext) error {
	if !strings.Contains(routeOutput, "route\tqueued") {
		return errors.New("route output did not report queued state")
	}
	if !strings.Contains(statusOutput, "dialtone_status\trunning") {
		return errors.New("status output did not report dialtone as running")
	}
	if !strings.Contains(statusOutput, "worker_status\trunning") {
		return errors.New("status output did not report shell worker as running")
	}
	if !strings.Contains(promptReadOutput, strings.TrimSpace(promptToken)) {
		return errors.New("prompt pane read did not contain the submitted test token")
	}
	if !strings.Contains(commandReadOutput, commandPaneExpect) {
		return errors.New("command pane read did not contain the routed command output")
	}
	if !strings.Contains(command.output, defaultCommandExpect) {
		return fmt.Errorf("command output did not contain %q", defaultCommandExpect)
	}
	if command.exitCode != 0 {
		return fmt.Errorf("command exit code = %d, want 0", command.exitCode)
	}
	return nil
}

func loadSelectedSystemState(db *sql.DB) (string, error) {
	keys := []string{
		"bootstrap.status",
		"bootstrap.mode",
		"tmux.prompt_target",
		"tmux.target",
		"dialtone.daemon.pid",
		"dialtone.daemon.status",
		"dialtone.daemon.heartbeat_at",
		sqlitestate.ShellWorkerStatusKey,
		sqlitestate.ShellWorkerPaneKey,
		sqlitestate.ShellWorkerHeartbeatKey,
		sqlitestate.ShellWorkerLastRowIDKey,
		sqlitestate.ShellWorkerLastStatusKey,
		sqlitestate.ShellWorkerLastExitCodeKey,
	}
	var out strings.Builder
	for _, key := range keys {
		record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		fmt.Fprintf(&out, "%s=%s\n", record.Key, record.Value)
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func renderProtocolEvents(events []modstate.ProtocolEventRecord) string {
	if len(events) == 0 {
		return ""
	}
	var out strings.Builder
	for _, event := range events {
		fmt.Fprintf(&out, "%d\t%s\t%s\t%d\t%s\t%s\n",
			event.EventIndex,
			strings.TrimSpace(event.EventType),
			strings.TrimSpace(event.QueueName),
			event.QueueRowID,
			strings.TrimSpace(event.PaneTarget),
			strings.TrimSpace(event.MessageText),
		)
	}
	return strings.TrimRight(out.String(), "\n")
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod test v1 <start> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start")
	fmt.Println("       Run the end-to-end dialtone system test: ensure the workflow, submit a prompt to codex-view, queue one plain routed command into dialtone-view, and print the SQLite-backed system report")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}
