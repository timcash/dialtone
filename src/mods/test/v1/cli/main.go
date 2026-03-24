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
	defaultSession     = "codex-view"
	defaultWaitSeconds = 30
	protocolRunName    = "test-v1-start"
)

var (
	rowIDPattern             = regexp.MustCompile(`row_id=(\d+)`)
	wrappedTerminalJoinRegex = regexp.MustCompile(`([[:alnum:]<>])-\s+([[:alnum:]<>])`)
)

type startOptions struct {
	session          string
	waitSeconds      int
	codexWaitSeconds int
	model            string
	reasoning        string
	dialtoneShell    string
	rightTitle       string
	prompt           string
	verifyCodex      bool
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

type routedScenario struct {
	Name                 string
	Description          string
	Args                 []string
	ExpectStatus         string
	ExpectExitCode       int
	ExpectOutputContains []string
	ExpectErrorContains  string
	MinRuntimeMS         int64
	MaxRuntimeMS         int64
	ObserveRunning       bool
	BackgroundFile       string
	BackgroundContains   []string
	BackgroundWait       time.Duration
}

type routedScenarioResult struct {
	Scenario        routedScenario
	RouteOutput     string
	RowID           int64
	Record          modstate.ShellBusRecord
	Command         commandContext
	RunningObserved bool
	BackgroundText  string
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
			exitIfErr(err, "test install")
		}
	case "build":
		if err := runBuild(args); err != nil {
			exitIfErr(err, "test build")
		}
	case "format":
		if err := runFormat(args); err != nil {
			exitIfErr(err, "test format")
		}
	case "test":
		if err := runTest(args); err != nil {
			exitIfErr(err, "test")
		}
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
	codexScenario := buildCodexAgentScenario(token)
	promptText := buildPromptText(token, opts.prompt, codexScenario)
	scenarioDir := filepath.Join(sqlitestate.ResolveStateDir(repoRoot), "test-v1", sanitizeFileComponent(token))
	if err := os.RemoveAll(scenarioDir); err != nil {
		return err
	}
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		return err
	}
	scenarios := buildDefaultScenarios(scenarioDir)

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
	plannedScenarios := append([]routedScenario(nil), scenarios...)
	if opts.verifyCodex {
		plannedScenarios = append([]routedScenario{codexScenario}, plannedScenarios...)
	}
	writeSection("test.command-scenarios", renderScenarioPlan(plannedScenarios))
	clearCommandOutput, clearCommandErr := runDialtoneModCapture(repoRoot, "tmux", "v1", "clear", "--pane", commandTarget)
	switch {
	case clearCommandErr != nil && strings.TrimSpace(clearCommandOutput) != "":
		clearCommandOutput = strings.TrimSpace(clearCommandOutput) + "\nbest_effort_error\t" + clearCommandErr.Error()
	case clearCommandErr != nil:
		clearCommandOutput = "best_effort_error\t" + clearCommandErr.Error()
	}
	if strings.TrimSpace(clearCommandOutput) != "" {
		writeSection("test.clear-command-pane", clearCommandOutput)
	}
	if clearCommandErr == nil {
		if err := appendEvent("command_pane_cleared", 0, commandTarget, "", "cleared dialtone-view before routed scenarios"); err != nil {
			fmt.Print(report.String())
			return err
		}
	}

	codexStartCommand := buildCodexStartCommand(opts.session, promptTarget, opts.model, opts.reasoning)
	codexStartOutput, err := runDialtoneModCapture(repoRoot,
		"shell", "v1", "run",
		"--pane", commandTarget,
		"--wait-seconds", strconv.Itoa(opts.waitSeconds),
		codexStartCommand,
	)
	if err != nil {
		finishError = err.Error()
		writeSection("test.codex-start", codexStartOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.codex-start", codexStartOutput)
	codexStartRowID, err := parseBracketRowID(codexStartOutput)
	if err != nil {
		finishError = err.Error()
		fmt.Print(report.String())
		return err
	}
	if err := appendEvent("codex_started", codexStartRowID, promptTarget, codexStartCommand, "restarted codex prompt pane through the shell worker"); err != nil {
		fmt.Print(report.String())
		return err
	}
	codexReadyOutput, err := waitForPaneContains(repoRoot, promptTarget, []string{"OpenAI Codex", "model:"}, time.Duration(opts.codexWaitSeconds)*time.Second)
	if err != nil {
		finishError = err.Error()
		writeSection("test.codex-ready", codexReadyOutput)
		fmt.Print(report.String())
		return err
	}
	writeSection("test.codex-ready", codexReadyOutput)
	if err := appendEvent("codex_ready", codexStartRowID, promptTarget, codexStartCommand, "prompt pane showed the Codex CLI banner before prompt submission"); err != nil {
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
	results := make([]routedScenarioResult, 0, len(plannedScenarios))
	if opts.verifyCodex {
		result, scenarioErr := executeCodexPromptScenario(db, time.Duration(opts.codexWaitSeconds)*time.Second, promptRowID, commandTarget, codexScenario, appendEvent)
		writeSection(fmt.Sprintf("test.scenario.%s", codexScenario.Name), renderScenarioResult(result))
		if scenarioErr != nil {
			finishError = scenarioErr.Error()
			fmt.Print(report.String())
			return scenarioErr
		}
		results = append(results, result)
	}
	for _, scenario := range scenarios {
		result, scenarioErr := executeScenario(db, repoRoot, time.Duration(opts.waitSeconds)*time.Second, commandTarget, scenario, appendEvent)
		writeSection(fmt.Sprintf("test.scenario.%s", scenario.Name), renderScenarioResult(result))
		if scenarioErr != nil {
			finishError = scenarioErr.Error()
			fmt.Print(report.String())
			return scenarioErr
		}
		results = append(results, result)
	}
	lastScenario := results[len(results)-1]

	statusOutput, err := runDialtoneModCapture(repoRoot,
		"shell", "v1", "status",
		"--row-id", strconv.FormatInt(lastScenario.RowID, 10),
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

	writeSection("test.architecture", renderArchitectureSummary(token, promptTarget, commandTarget, codexStartRowID, promptRowID, promptReadOutput, commandReadOutput, statusOutput, results))

	if err := validateE2EOutputs(token, promptTarget, commandTarget, promptReadOutput, commandReadOutput, statusOutput, results); err != nil {
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
	finishResult = fmt.Sprintf("prompt_row=%d scenarios=%d last_row=%d last_exit_code=%d last_runtime_ms=%d", promptRowID, len(results), lastScenario.RowID, lastScenario.Command.exitCode, lastScenario.Command.runtimeMS)

	writeLine("test_result\tpassed")
	writeLine("protocol_run_id\t%d", runID)
	writeLine("prompt_row_id\t%d", promptRowID)
	writeLine("scenario_count\t%d", len(results))
	for _, result := range results {
		writeLine("scenario\t%s\trow_id=%d\tstatus=%s\texit_code=%d\truntime_ms=%d", result.Scenario.Name, result.RowID, result.Record.Status, result.Command.exitCode, result.Command.runtimeMS)
		if strings.TrimSpace(result.Scenario.BackgroundFile) != "" {
			writeLine("scenario_background\t%s\tfile=%s\tcomplete=%t", result.Scenario.Name, result.Scenario.BackgroundFile, strings.TrimSpace(result.BackgroundText) != "")
		}
	}
	writeLine("command_row_id\t%d", lastScenario.RowID)
	writeLine("command_status\t%s", lastScenario.Record.Status)
	writeLine("command_pid\t%d", lastScenario.Command.pid)
	writeLine("command_exit_code\t%d", lastScenario.Command.exitCode)
	writeLine("command_runtime_ms\t%d", lastScenario.Command.runtimeMS)
	writeLine("state_db\t%s", sqlitestate.ResolveStateDBPath(repoRoot))
	writeLine("prompt_target\t%s", promptTarget)
	writeLine("command_target\t%s", commandTarget)
	writeLine("routed_command\t%s", dispatch.BuildDialtoneCommand(lastScenario.Scenario.Args))
	writeLine("inspect\t./dialtone_mod shell v1 status --row-id %d --full --sync=false", lastScenario.RowID)
	fmt.Print(report.String())
	return nil
}

func parseStartArgs(argv []string) (startOptions, error) {
	opts := flag.NewFlagSet("test v1 start", flag.ContinueOnError)
	session := opts.String("session", defaultSession, "tmux session name to use for the end-to-end test")
	waitSeconds := opts.Int("wait-seconds", defaultWaitSeconds, "Seconds to wait for the routed command to complete")
	codexWaitSeconds := opts.Int("codex-wait-seconds", 60, "Seconds to wait for Codex to queue the prompted routed command")
	model := opts.String("model", "gpt-5.4", "Codex model to launch if the workflow is missing")
	reasoning := opts.String("reasoning", "medium", "Reasoning label for the Codex startup banner")
	dialtoneShell := opts.String("dialtone-shell", "default", "flake shell to keep active in dialtone-view")
	rightTitle := opts.String("right-title", "dialtone-view", "Title for the right-side tmux pane")
	prompt := opts.String("prompt", "", "Optional explicit prompt text to submit to codex-view")
	verifyCodex := opts.Bool("verify-codex", true, "Wait for Codex to issue one prompted plain routed ./dialtone_mod command before the harness queues the deterministic scenarios")
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
	if *codexWaitSeconds <= 0 {
		return startOptions{}, errors.New("--codex-wait-seconds must be positive")
	}
	return startOptions{
		session:          strings.TrimSpace(*session),
		waitSeconds:      *waitSeconds,
		codexWaitSeconds: *codexWaitSeconds,
		model:            strings.TrimSpace(*model),
		reasoning:        strings.TrimSpace(*reasoning),
		dialtoneShell:    strings.TrimSpace(*dialtoneShell),
		rightTitle:       strings.TrimSpace(*rightTitle),
		prompt:           strings.TrimSpace(*prompt),
		verifyCodex:      *verifyCodex,
	}, nil
}

func buildPromptText(token, custom string, codexScenario routedScenario) string {
	codexCommand := dispatch.BuildDialtoneCommand(codexScenario.Args)
	lines := []string{
		fmt.Sprintf("%s: this is the SQLite-backed end-to-end system test.", promptHeadline(token)),
	}
	if strings.TrimSpace(custom) != "" {
		lines = append(lines, strings.TrimSpace(custom))
	}
	lines = append(lines,
		"Run exactly one plain routed ./dialtone_mod command from this codex-view session so SQLite can queue it for dialtone-view.",
		"Run this command once exactly as written:",
		codexCommand,
		"After that command queues, stop. Do not run the example commands below and do not reply in chat.",
		"These additional plain routed ./dialtone_mod commands are examples of the behaviors the system should support in dialtone-view:",
	)
	for index, command := range promptExampleCommands() {
		lines = append(lines, fmt.Sprintf("%d. %s", index+1, command))
	}
	return strings.Join(lines, "\n")
}

func promptHeadline(token string) string {
	return fmt.Sprintf("Dialtone test v1 start %s", strings.TrimSpace(token))
}

func codexAgentLabel(token string) string {
	label := sanitizeFileComponent(strings.TrimSpace(token))
	if label == "" {
		label = "DIALTONE_TEST_V1"
	}
	return "CODEX_AGENT_" + label
}

func buildCodexAgentScenario(token string) routedScenario {
	label := codexAgentLabel(token)
	return routedScenario{
		Name:                 "codex_agent",
		Description:          "Prompt Codex in codex-view to run one plain routed probe command and verify SQLite queues it for dialtone-view where the worker executes it for real.",
		Args:                 []string{"mods", "v1", "probe", "--mode", "success", "--label", label},
		ExpectStatus:         "done",
		ExpectExitCode:       0,
		ExpectOutputContains: []string{"probe_mode\tsuccess", "probe_label\t" + label, "probe_result\tsuccess"},
	}
}

func buildCodexStartCommand(session, promptTarget, model, reasoning string) string {
	return dispatch.BuildDialtoneCommand([]string{
		"codex", "v1", "start",
		"--session", strings.TrimSpace(session),
		"--pane", strings.TrimSpace(promptTarget),
		"--shell", "default",
		"--reasoning", strings.TrimSpace(reasoning),
		"--model", strings.TrimSpace(model),
	})
}

func promptExampleCommands() []string {
	return []string{
		"./dialtone_mod mods v1 db graph --format outline",
		"./dialtone_mod mods v1 probe --mode sleep --sleep-ms 1500 --label TEST_LONG_RUNNING",
		"./dialtone_mod mods v1 probe --mode fail --label TEST_FAILURE",
		"./dialtone_mod mods v1 probe --mode invalid --label TEST_INVALID_MODE",
		"./dialtone_mod mods v1 probe --mode background --sleep-ms 4000 --label TEST_BACKGROUND --background-file <marker-file>",
	}
}

func buildDefaultScenarios(scenarioDir string) []routedScenario {
	backgroundFile := filepath.Join(scenarioDir, "background-marker.txt")
	return []routedScenario{
		{
			Name:                 "graph",
			Description:          "Queue a plain routed graph query and verify the visible outline appears in dialtone-view.",
			Args:                 []string{"mods", "v1", "db", "graph", "--format", "outline"},
			ExpectStatus:         "done",
			ExpectExitCode:       0,
			ExpectOutputContains: []string{"- test:v1", "- shell:v1"},
		},
		{
			Name:                 "long_running",
			Description:          "Queue a routed probe that sleeps long enough to observe SQLite move the row into running before it completes.",
			Args:                 []string{"mods", "v1", "probe", "--mode", "sleep", "--sleep-ms", "1500", "--label", "TEST_LONG_RUNNING"},
			ExpectStatus:         "done",
			ExpectExitCode:       0,
			ExpectOutputContains: []string{"probe_mode\tsleep", "probe_label\tTEST_LONG_RUNNING", "probe_result\tsuccess"},
			MinRuntimeMS:         1200,
			ObserveRunning:       true,
		},
		{
			Name:                 "failing",
			Description:          "Queue a routed probe that fails and verify SQLite records failed status, exit code, and error text without killing the worker.",
			Args:                 []string{"mods", "v1", "probe", "--mode", "fail", "--label", "TEST_FAILURE"},
			ExpectStatus:         "failed",
			ExpectExitCode:       1,
			ExpectOutputContains: []string{"probe_mode\tfail", "probe_label\tTEST_FAILURE", "probe_result\tfailure"},
			ExpectErrorContains:  "requested failure",
		},
		{
			Name:                "invalid_mode",
			Description:         "Queue a routed probe with invalid input and verify a fast CLI validation failure is recorded without wedging the worker.",
			Args:                []string{"mods", "v1", "probe", "--mode", "invalid", "--label", "TEST_INVALID_MODE"},
			ExpectStatus:        "failed",
			ExpectExitCode:      1,
			ExpectErrorContains: `unsupported --mode "invalid"`,
			MaxRuntimeMS:        2000,
		},
		{
			Name:                 "recovery",
			Description:          "Queue another routed probe after both failure modes and verify the worker continues handling new commands.",
			Args:                 []string{"mods", "v1", "probe", "--mode", "success", "--label", "TEST_RECOVERY"},
			ExpectStatus:         "done",
			ExpectExitCode:       0,
			ExpectOutputContains: []string{"probe_mode\tsuccess", "probe_label\tTEST_RECOVERY", "probe_result\tsuccess"},
		},
		{
			Name:                 "background",
			Description:          "Queue a routed probe that spawns detached background work, exits quickly, and writes a marker file later.",
			Args:                 []string{"mods", "v1", "probe", "--mode", "background", "--sleep-ms", "4000", "--label", "TEST_BACKGROUND", "--background-file", backgroundFile},
			ExpectStatus:         "done",
			ExpectExitCode:       0,
			ExpectOutputContains: []string{"probe_mode\tbackground", "probe_label\tTEST_BACKGROUND", "probe_result\tbackground-started", "probe_background_file\t" + backgroundFile},
			MaxRuntimeMS:         2500,
			BackgroundFile:       backgroundFile,
			BackgroundContains:   []string{"probe_background_done\tTEST_BACKGROUND", "probe_label\tTEST_BACKGROUND"},
			BackgroundWait:       6 * time.Second,
		},
	}
}

func renderScenarioPlan(scenarios []routedScenario) string {
	if len(scenarios) == 0 {
		return ""
	}
	var out strings.Builder
	for _, scenario := range scenarios {
		fmt.Fprintf(&out, "%s\t%s\t%s\n",
			strings.TrimSpace(scenario.Name),
			strings.TrimSpace(scenario.Description),
			dispatch.BuildDialtoneCommand(scenario.Args),
		)
	}
	return strings.TrimRight(out.String(), "\n")
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

func parseKVString(output, key string) (string, error) {
	prefix := strings.TrimSpace(key) + "\t"
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), nil
	}
	return "", fmt.Errorf("%s not found in output", key)
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

func executeScenario(
	db *sql.DB,
	repoRoot string,
	wait time.Duration,
	commandTarget string,
	scenario routedScenario,
	appendEvent func(eventType string, queueRowID int64, paneTarget, commandText, messageText string) error,
) (routedScenarioResult, error) {
	result := routedScenarioResult{Scenario: scenario}
	commandText := dispatch.BuildDialtoneCommand(scenario.Args)
	routeOutput, err := runDialtoneModCapture(repoRoot, scenario.Args...)
	result.RouteOutput = routeOutput
	if err != nil {
		return result, err
	}
	rowID, err := parseCommandID(routeOutput)
	if err != nil {
		return result, err
	}
	result.RowID = rowID
	if err := appendEvent("scenario_queued", rowID, commandTarget, commandText, fmt.Sprintf("%s queued", scenario.Name)); err != nil {
		return result, err
	}
	if scenario.ObserveRunning {
		if _, err := waitForShellBusStatus(db, rowID, "running", wait); err != nil {
			return result, err
		}
		result.RunningObserved = true
		if err := appendEvent("scenario_running", rowID, commandTarget, commandText, fmt.Sprintf("%s observed running", scenario.Name)); err != nil {
			return result, err
		}
	}
	record, err := waitForShellBusRecord(db, rowID, wait)
	if err != nil {
		return result, err
	}
	result.Record = record
	command, err := decodeCommandContext(record)
	if err != nil {
		return result, err
	}
	result.Command = command
	if strings.TrimSpace(result.Command.target) != strings.TrimSpace(commandTarget) {
		return result, fmt.Errorf("%s targeted %s, want %s", scenario.Name, result.Command.target, commandTarget)
	}
	if strings.TrimSpace(scenario.BackgroundFile) != "" {
		backgroundText, err := waitForFileContains(scenario.BackgroundFile, scenario.BackgroundContains, scenario.BackgroundWait)
		if err != nil {
			return result, err
		}
		result.BackgroundText = backgroundText
		if err := appendEvent("scenario_background_done", rowID, commandTarget, commandText, fmt.Sprintf("%s background marker observed", scenario.Name)); err != nil {
			return result, err
		}
	}
	if err := appendEvent("scenario_observed", rowID, commandTarget, commandText, fmt.Sprintf("%s status=%s exit=%d pid=%d runtime_ms=%d", scenario.Name, record.Status, command.exitCode, command.pid, command.runtimeMS)); err != nil {
		return result, err
	}
	if err := validateScenarioResult(result); err != nil {
		return result, err
	}
	return result, nil
}

func executeCodexPromptScenario(
	db *sql.DB,
	wait time.Duration,
	afterRowID int64,
	commandTarget string,
	scenario routedScenario,
	appendEvent func(eventType string, queueRowID int64, paneTarget, commandText, messageText string) error,
) (routedScenarioResult, error) {
	result := routedScenarioResult{Scenario: scenario}
	commandText := dispatch.BuildDialtoneCommand(scenario.Args)
	detected, err := waitForShellBusCommandByText(db, commandText, afterRowID, wait)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(detected.Actor) != "dialtone_mod" {
		return result, fmt.Errorf("%s actor = %s, want dialtone_mod", scenario.Name, strings.TrimSpace(detected.Actor))
	}
	result.RowID = detected.ID
	result.RouteOutput = fmt.Sprintf("route\tqueued\norigin\tcodex_prompt\ncommand_id\t%d\ninspect\t./dialtone_mod shell v1 status --row-id %d --full --sync=false", detected.ID, detected.ID)
	if err := appendEvent("codex_command_queued", detected.ID, commandTarget, commandText, fmt.Sprintf("%s queued by Codex after prompt row %d", scenario.Name, afterRowID)); err != nil {
		return result, err
	}
	record, err := waitForShellBusRecord(db, detected.ID, wait)
	if err != nil {
		return result, err
	}
	result.Record = record
	command, err := decodeCommandContext(record)
	if err != nil {
		return result, err
	}
	result.Command = command
	if strings.TrimSpace(result.Command.target) != strings.TrimSpace(commandTarget) {
		return result, fmt.Errorf("%s targeted %s, want %s", scenario.Name, result.Command.target, commandTarget)
	}
	if err := appendEvent("scenario_observed", detected.ID, commandTarget, commandText, fmt.Sprintf("%s status=%s exit=%d pid=%d runtime_ms=%d", scenario.Name, record.Status, command.exitCode, command.pid, command.runtimeMS)); err != nil {
		return result, err
	}
	if err := validateScenarioResult(result); err != nil {
		return result, err
	}
	return result, nil
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

func waitForShellBusStatus(db *sql.DB, rowID int64, want string, timeout time.Duration) (modstate.ShellBusRecord, error) {
	deadline := time.Now().Add(timeout)
	want = strings.TrimSpace(want)
	for time.Now().Before(deadline) {
		record, ok, err := modstate.LoadShellBusRecord(db, rowID)
		if err != nil {
			return modstate.ShellBusRecord{}, err
		}
		if ok && strings.TrimSpace(record.Status) == want {
			return record, nil
		}
		if ok && strings.TrimSpace(record.Status) != "queued" && strings.TrimSpace(record.Status) != "running" && strings.TrimSpace(record.Status) != want {
			return modstate.ShellBusRecord{}, fmt.Errorf("shell bus row %d finished as %s before reaching %s", rowID, strings.TrimSpace(record.Status), want)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return modstate.ShellBusRecord{}, fmt.Errorf("timed out waiting for shell bus row %d to reach %s", rowID, want)
}

func waitForShellBusCommandByText(db *sql.DB, commandText string, afterRowID int64, timeout time.Duration) (modstate.ShellBusRecord, error) {
	deadline := time.Now().Add(timeout)
	want := strings.TrimSpace(commandText)
	for time.Now().Before(deadline) {
		rows, err := modstate.LoadShellBus(db, "desired", 500)
		if err != nil {
			return modstate.ShellBusRecord{}, err
		}
		for _, row := range rows {
			if row.ID <= afterRowID {
				continue
			}
			if row.Subject != "command" || row.Action != "run" {
				continue
			}
			command, err := decodeCommandContext(row)
			if err != nil {
				continue
			}
			if strings.TrimSpace(command.command) == want {
				return row, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return modstate.ShellBusRecord{}, fmt.Errorf("timed out waiting for Codex to queue %q after row %d", want, afterRowID)
}

func waitForFileContains(path string, wants []string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			text := strings.TrimSpace(string(data))
			missing := false
			for _, want := range wants {
				if !strings.Contains(text, strings.TrimSpace(want)) {
					missing = true
					break
				}
			}
			if !missing {
				return text, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for background marker file %s", path)
}

func waitForPaneContains(repoRoot, pane string, wants []string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	target := strings.TrimSpace(pane)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		text, err := runDialtoneModCapture(repoRoot, "tmux", "v1", "read", "--pane", target, "--lines", "160")
		if err == nil {
			missing := false
			for _, want := range wants {
				if !strings.Contains(text, strings.TrimSpace(want)) {
					missing = true
					break
				}
			}
			if !missing {
				return text, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	text, _ := runDialtoneModCapture(repoRoot, "tmux", "v1", "read", "--pane", target, "--lines", "160")
	return text, fmt.Errorf("timed out waiting for tmux pane %s to contain %q", target, strings.Join(wants, ", "))
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

func validateScenarioResult(result routedScenarioResult) error {
	scenario := result.Scenario
	if !strings.Contains(result.RouteOutput, "route\tqueued") {
		return fmt.Errorf("%s route output did not report queued state", scenario.Name)
	}
	if strings.TrimSpace(result.Record.Status) != strings.TrimSpace(scenario.ExpectStatus) {
		return fmt.Errorf("%s status = %s, want %s", scenario.Name, result.Record.Status, scenario.ExpectStatus)
	}
	if result.Command.exitCode != scenario.ExpectExitCode {
		return fmt.Errorf("%s exit code = %d, want %d", scenario.Name, result.Command.exitCode, scenario.ExpectExitCode)
	}
	for _, want := range scenario.ExpectOutputContains {
		if !strings.Contains(result.Command.output, want) {
			return fmt.Errorf("%s output did not contain %q", scenario.Name, want)
		}
	}
	if strings.TrimSpace(scenario.ExpectErrorContains) != "" &&
		!strings.Contains(result.Command.errorText, strings.TrimSpace(scenario.ExpectErrorContains)) &&
		!strings.Contains(result.Command.output, strings.TrimSpace(scenario.ExpectErrorContains)) {
		return fmt.Errorf("%s failure detail did not contain %q", scenario.Name, scenario.ExpectErrorContains)
	}
	if scenario.MinRuntimeMS > 0 && result.Command.runtimeMS < scenario.MinRuntimeMS {
		return fmt.Errorf("%s runtime_ms = %d, want at least %d", scenario.Name, result.Command.runtimeMS, scenario.MinRuntimeMS)
	}
	if scenario.MaxRuntimeMS > 0 && result.Command.runtimeMS > scenario.MaxRuntimeMS {
		return fmt.Errorf("%s runtime_ms = %d, want at most %d", scenario.Name, result.Command.runtimeMS, scenario.MaxRuntimeMS)
	}
	if scenario.ObserveRunning && !result.RunningObserved {
		return fmt.Errorf("%s did not observe running status", scenario.Name)
	}
	if strings.TrimSpace(scenario.BackgroundFile) != "" && strings.TrimSpace(result.BackgroundText) == "" {
		return fmt.Errorf("%s background marker file was not observed", scenario.Name)
	}
	return nil
}

func validateE2EOutputs(promptToken, promptTarget, commandTarget, promptReadOutput, commandReadOutput, statusOutput string, results []routedScenarioResult) error {
	promptTextMarker := promptHeadline(promptToken)
	if !strings.Contains(statusOutput, "dialtone_status\trunning") {
		return errors.New("status output did not report dialtone as running")
	}
	if !strings.Contains(statusOutput, "worker_status\trunning") {
		return errors.New("status output did not report shell worker as running")
	}
	if !strings.Contains(statusOutput, "prompt_target\t"+strings.TrimSpace(promptTarget)) {
		return errors.New("status output did not report the expected prompt target")
	}
	if !strings.Contains(statusOutput, "command_target\t"+strings.TrimSpace(commandTarget)) {
		return errors.New("status output did not report the expected command target")
	}
	workerPane, err := parseKVString(statusOutput, "worker_pane")
	if err != nil {
		return errors.New("status output did not report the worker pane")
	}
	if strings.TrimSpace(workerPane) != strings.TrimSpace(commandTarget) {
		return fmt.Errorf("worker pane = %s, want %s", workerPane, commandTarget)
	}
	if !containsNormalizedText(promptReadOutput, promptTextMarker) {
		return errors.New("prompt pane read did not contain the submitted test token")
	}
	if codexAgent, ok := scenarioResultByName(results, "codex_agent"); ok {
		codexCommand := dispatch.BuildDialtoneCommand(codexAgent.Scenario.Args)
		if !containsNormalizedText(promptReadOutput, codexCommand) {
			return errors.New("prompt pane read did not contain the exact Codex-routed command")
		}
		if strings.TrimSpace(codexAgent.Record.Status) != "done" || codexAgent.Command.exitCode != 0 {
			return errors.New("codex-agent scenario did not report a successful routed command")
		}
		if !strings.Contains(codexAgent.Command.output, "probe_label\t"+codexAgentLabel(promptToken)) {
			return errors.New("codex-agent scenario did not carry the unique prompted label into dialtone-view output")
		}
	}
	if missing := missingPromptExampleCommands(promptReadOutput); len(missing) > 0 {
		return fmt.Errorf("prompt pane read did not contain the routed example commands: %s", strings.Join(missing, " | "))
	}
	if containsNormalizedText(commandReadOutput, promptTextMarker) {
		return errors.New("command pane read unexpectedly contained the submitted prompt token")
	}
	for _, result := range results {
		if strings.TrimSpace(result.Command.target) != strings.TrimSpace(commandTarget) {
			return fmt.Errorf("%s targeted %s, want %s", result.Scenario.Name, result.Command.target, commandTarget)
		}
	}
	graph, ok := scenarioResultByName(results, "graph")
	if !ok || !strings.Contains(graph.Command.output, "- shell:v1") {
		return errors.New("graph scenario did not contain the expected outline output")
	}
	longRunning, ok := scenarioResultByName(results, "long_running")
	if !ok || !longRunning.RunningObserved {
		return errors.New("long-running scenario did not observe SQLite running status")
	}
	failing, ok := scenarioResultByName(results, "failing")
	if !ok || strings.TrimSpace(failing.Record.Status) != "failed" || failing.Command.exitCode == 0 {
		return errors.New("failing scenario did not report a failed routed command")
	}
	invalidMode, ok := scenarioResultByName(results, "invalid_mode")
	if !ok || strings.TrimSpace(invalidMode.Record.Status) != "failed" || invalidMode.Command.exitCode == 0 {
		return errors.New("invalid-mode scenario did not report a failed routed command")
	}
	recovery, ok := scenarioResultByName(results, "recovery")
	if !ok || strings.TrimSpace(recovery.Record.Status) != "done" || recovery.Command.exitCode != 0 {
		return errors.New("recovery scenario did not prove the worker kept handling commands after the failure cases")
	}
	background, ok := scenarioResultByName(results, "background")
	if !ok || strings.TrimSpace(background.BackgroundText) == "" {
		return errors.New("background scenario did not produce the detached marker file")
	}
	if !strings.Contains(commandReadOutput, "TEST_BACKGROUND") {
		return errors.New("command pane read did not contain the final routed background probe output")
	}
	return nil
}

func renderArchitectureSummary(promptToken, promptTarget, commandTarget string, codexStartRowID, promptRowID int64, promptReadOutput, commandReadOutput, statusOutput string, results []routedScenarioResult) string {
	var out strings.Builder
	promptTextMarker := promptHeadline(promptToken)
	workerPane, _ := parseKVString(statusOutput, "worker_pane")
	routedTargetsMatch := true
	failureScenarios := []string{}
	for _, result := range results {
		if strings.TrimSpace(result.Command.target) != strings.TrimSpace(commandTarget) {
			routedTargetsMatch = false
		}
		if strings.TrimSpace(result.Record.Status) == "failed" || result.Command.exitCode != 0 {
			failureScenarios = append(failureScenarios, strings.TrimSpace(result.Scenario.Name))
		}
	}
	if len(failureScenarios) == 0 {
		failureScenarios = append(failureScenarios, "none")
	}
	codexAgent, hasCodexAgent := scenarioResultByName(results, "codex_agent")
	recovery, hasRecovery := scenarioResultByName(results, "recovery")
	background, hasBackground := scenarioResultByName(results, "background")
	fmt.Fprintf(&out, "codex_start_row_id\t%d\n", codexStartRowID)
	fmt.Fprintf(&out, "prompt_row_id\t%d\n", promptRowID)
	fmt.Fprintf(&out, "prompt_target\t%s\n", strings.TrimSpace(promptTarget))
	fmt.Fprintf(&out, "command_target\t%s\n", strings.TrimSpace(commandTarget))
	fmt.Fprintf(&out, "worker_pane\t%s\n", strings.TrimSpace(workerPane))
	fmt.Fprintf(&out, "worker_matches_command_target\t%t\n", strings.TrimSpace(workerPane) == strings.TrimSpace(commandTarget))
	fmt.Fprintf(&out, "prompt_visible_in_codex_view\t%t\n", containsNormalizedText(promptReadOutput, promptTextMarker))
	fmt.Fprintf(&out, "prompt_not_visible_in_dialtone_view\t%t\n", !containsNormalizedText(commandReadOutput, promptTextMarker))
	fmt.Fprintf(&out, "codex_initiated_command\t%t\n", hasCodexAgent && codexAgent.RowID > promptRowID)
	if hasCodexAgent {
		fmt.Fprintf(&out, "codex_command_row_id\t%d\n", codexAgent.RowID)
		fmt.Fprintf(&out, "codex_command_ran_in_dialtone_view\t%t\n", strings.TrimSpace(codexAgent.Command.target) == strings.TrimSpace(commandTarget))
	}
	fmt.Fprintf(&out, "routed_command_count\t%d\n", len(results))
	fmt.Fprintf(&out, "routed_targets_match_dialtone_view\t%t\n", routedTargetsMatch)
	fmt.Fprintf(&out, "failure_scenarios\t%s\n", strings.Join(failureScenarios, ","))
	fmt.Fprintf(&out, "recovery_after_failures\t%t\n", hasRecovery && strings.TrimSpace(recovery.Record.Status) == "done" && recovery.Command.exitCode == 0)
	fmt.Fprintf(&out, "background_completion\t%t\n", hasBackground && strings.TrimSpace(background.BackgroundText) != "")
	return strings.TrimRight(out.String(), "\n")
}

func scenarioResultByName(results []routedScenarioResult, name string) (routedScenarioResult, bool) {
	for _, result := range results {
		if strings.TrimSpace(result.Scenario.Name) == strings.TrimSpace(name) {
			return result, true
		}
	}
	return routedScenarioResult{}, false
}

func renderScenarioResult(result routedScenarioResult) string {
	var out strings.Builder
	scenario := result.Scenario
	fmt.Fprintf(&out, "description\t%s\n", strings.TrimSpace(scenario.Description))
	fmt.Fprintf(&out, "command\t%s\n", dispatch.BuildDialtoneCommand(scenario.Args))
	if result.RowID > 0 {
		fmt.Fprintf(&out, "command_id\t%d\n", result.RowID)
	}
	if strings.TrimSpace(result.RouteOutput) != "" {
		fmt.Fprintf(&out, "route_output\n%s\n", strings.TrimSpace(result.RouteOutput))
	}
	if strings.TrimSpace(result.Record.Status) != "" {
		fmt.Fprintf(&out, "status\t%s\n", strings.TrimSpace(result.Record.Status))
	}
	if result.Command.pid > 0 {
		fmt.Fprintf(&out, "pid\t%d\n", result.Command.pid)
	}
	if result.Command.runtimeMS > 0 {
		fmt.Fprintf(&out, "runtime_ms\t%d\n", result.Command.runtimeMS)
	}
	fmt.Fprintf(&out, "exit_code\t%d\n", result.Command.exitCode)
	fmt.Fprintf(&out, "running_observed\t%t\n", result.RunningObserved)
	if strings.TrimSpace(result.Command.errorText) != "" {
		fmt.Fprintf(&out, "error\t%s\n", strings.TrimSpace(result.Command.errorText))
	}
	if strings.TrimSpace(scenario.BackgroundFile) != "" {
		fmt.Fprintf(&out, "background_file\t%s\n", scenario.BackgroundFile)
		fmt.Fprintf(&out, "background_complete\t%t\n", strings.TrimSpace(result.BackgroundText) != "")
		if strings.TrimSpace(result.BackgroundText) != "" {
			fmt.Fprintf(&out, "background_text\n%s\n", strings.TrimSpace(result.BackgroundText))
		}
	}
	if strings.TrimSpace(result.Command.output) != "" {
		fmt.Fprintf(&out, "command_output\n%s\n", strings.TrimSpace(result.Command.output))
	}
	return strings.TrimRight(out.String(), "\n")
}

func normalizeWhitespace(value string) string {
	cleaned := strings.TrimSpace(value)
	cleaned = wrappedTerminalJoinRegex.ReplaceAllString(cleaned, "$1-$2")
	return strings.Join(strings.Fields(cleaned), " ")
}

func containsNormalizedText(haystack, needle string) bool {
	return strings.Contains(normalizeWhitespace(haystack), normalizeWhitespace(needle))
}

func missingPromptExampleCommands(promptReadOutput string) []string {
	missing := []string{}
	for _, command := range promptExampleCommands() {
		if containsNormalizedText(promptReadOutput, command) {
			continue
		}
		missing = append(missing, command)
	}
	return missing
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

func sanitizeFileComponent(value string) string {
	if strings.TrimSpace(value) == "" {
		return "test-v1"
	}
	var out strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			out.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			out.WriteRune(r)
		case r >= '0' && r <= '9':
			out.WriteRune(r)
		default:
			out.WriteByte('_')
		}
	}
	return strings.Trim(strings.TrimSpace(out.String()), "_")
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod test v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install")
	fmt.Println("       Verify Go, tmux, and codex or npx are available in the default nix shell")
	fmt.Println("  build")
	fmt.Println("       Build the test v1 CLI wrapper to <repo-root>/bin/mods/test/v1/test")
	fmt.Println("  format [--dir DIR]")
	fmt.Println("       Run gofmt on test v1 Go files")
	fmt.Println("  test")
	fmt.Println("       Run go test for test v1")
	fmt.Println("  start")
	fmt.Println("       Run the end-to-end dialtone system test: ensure the workflow, refresh codex-view, submit a prompt, wait for Codex to queue one plain routed ./dialtone_mod command, then queue deterministic routed commands into dialtone-view to validate success, long-running, failure, recovery, and background behavior")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}
