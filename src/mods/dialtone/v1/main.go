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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/router"
	"dialtone/dev/mods/shared/sqlitestate"
)

type ensureResult struct {
	PID     int
	LogPath string
	State   string
}

type processRecord struct {
	PID        int
	PPID       int
	Stat       string
	TTY        string
	Role       string
	Command    string
	Background bool
}

type statusOptions struct {
	rowID int64
	full  bool
}

type commandsOptions struct {
	limit   int
	scope   string
	subject string
	status  string
	session string
	pane    string
}

type commandOptions struct {
	rowID int64
	full  bool
}

type logOptions struct {
	kind     string
	rowID    int64
	lines    int
	pathOnly bool
}

type protocolRunOptions struct {
	runID int64
	full  bool
}

type testRunOptions struct {
	runID int64
	full  bool
}

var (
	nowUTC              = func() time.Time { return time.Now().UTC() }
	processAliveFn      = processAlive
	processCommandFn    = processCommand
	terminateProcessFn  = terminateProcess
	startDetachedSelfFn = startDetachedSelf
	startDetachedCmdFn  = startDetachedCommand
	loadProcessReportFn = loadProcessReport
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		exitIfErr(err, "dialtone")
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch strings.TrimSpace(args[0]) {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "paths":
		return runPaths(args[1:])
	case "processes":
		return runProcesses(args[1:])
	case "serve":
		return runServe(args[1:])
	case "ensure":
		return runEnsure(args[1:])
	case "status", "state", "bootstrap":
		return runStatus(args[1:])
	case "commands":
		return runCommands(args[1:])
	case "command":
		return runCommand(args[1:])
	case "log", "logs":
		return runLog(args[1:])
	case "queue":
		return runQueue(args[1:])
	case "protocol-runs":
		return runProtocolRuns(args[1:])
	case "protocol-run":
		return runProtocolRun(args[1:])
	case "test-runs":
		return runTestRuns(args[1:])
	case "test-run":
		return runTestRun(args[1:])
	default:
		return fmt.Errorf("unknown dialtone command: %s", strings.TrimSpace(args[0]))
	}
}

func runEnsure(argv []string) error {
	if len(argv) != 0 {
		return errors.New("ensure does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	result, err := ensureDaemon(db, repoRoot)
	if err != nil {
		return err
	}
	fmt.Printf("%d\t%s\t%s\n", result.PID, strings.TrimSpace(result.LogPath), strings.TrimSpace(result.State))
	return nil
}

func runPaths(argv []string) error {
	if len(argv) != 0 {
		return errors.New("paths does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	fmt.Printf("repo_root\t%s\n", strings.TrimSpace(repoRoot))
	fmt.Printf("state_dir\t%s\n", sqlitestate.ResolveStateDir(repoRoot))
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Printf("logs_dir\t%s\n", sqlitestate.ResolveLogsDir(repoRoot))
	fmt.Printf("command_logs_dir\t%s\n", sqlitestate.ResolveCommandLogsDir(repoRoot))
	return nil
}

func runProcesses(argv []string) error {
	if len(argv) != 0 {
		return errors.New("processes does not accept positional arguments")
	}
	processes, err := loadProcessReportFn()
	if err != nil {
		return err
	}
	total, totalBackground, dialtoneModCount, dialtoneModBackground, dialtoneCount, workerCount, ensureCount := summarizeProcessRecords(processes)
	fmt.Printf("dialtone_processes_running\t%d\n", total)
	fmt.Printf("dialtone_processes_background\t%d\n", totalBackground)
	fmt.Printf("dialtone_mod_running\t%d\n", dialtoneModCount)
	fmt.Printf("dialtone_mod_background\t%d\n", dialtoneModBackground)
	fmt.Printf("dialtone_running\t%d\n", dialtoneCount)
	fmt.Printf("worker_running\t%d\n", workerCount)
	fmt.Printf("ensure_running\t%d\n", ensureCount)
	fmt.Println("pid\tppid\tstat\ttty\trole\tbackground\tcommand")
	for _, record := range processes {
		background := "no"
		if record.Background {
			background = "yes"
		}
		fmt.Printf("%d\t%d\t%s\t%s\t%s\t%s\t%s\n",
			record.PID,
			record.PPID,
			record.Stat,
			record.TTY,
			record.Role,
			background,
			record.Command,
		)
	}
	return nil
}

func runServe(argv []string) error {
	if len(argv) != 0 {
		return errors.New("serve does not accept positional arguments")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	daemonPID := os.Getpid()
	existingPIDText, _, err := loadOptionalStateValue(db, "dialtone.daemon.pid")
	if err != nil {
		return err
	}
	if existingPIDText != "" {
		if existingPID, convErr := strconv.Atoi(existingPIDText); convErr == nil && existingPID > 0 && existingPID != daemonPID && processAliveFn(existingPID) {
			fmt.Printf("dialtone already running\t%d\n", existingPID)
			return nil
		}
	}

	daemonLogPath := strings.TrimSpace(os.Getenv("DIALTONE_DAEMON_LOG_PATH"))
	if daemonLogPath == "" {
		daemonLogPath, _, _ = loadOptionalStateValue(db, "dialtone.daemon.log_path")
	}

	markStopped := func() {
		now := nowUTC().Format(time.RFC3339)
		_ = modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.status", "stopped")
		_ = modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.heartbeat_at", now)
	}
	defer markStopped()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		now := nowUTC().Format(time.RFC3339)
		if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.pid", strconv.Itoa(daemonPID)); err != nil {
			return err
		}
		if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.status", "running"); err != nil {
			return err
		}
		if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.heartbeat_at", now); err != nil {
			return err
		}
		if strings.TrimSpace(daemonLogPath) != "" {
			if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.log_path", strings.TrimSpace(daemonLogPath)); err != nil {
				return err
			}
		}
		if _, err := ensureWorkerAsync(repoRoot, db); err != nil {
			fmt.Fprintf(os.Stderr, "DIALTONE> ensure-worker warning: %v\n", err)
		}
		if _, err := clearEnsureStateIfShellReady(db, 5*time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "DIALTONE> ensure-state cleanup warning: %v\n", err)
		}

		select {
		case <-ticker.C:
		case <-signals:
			return nil
		}
	}
}

func runQueue(argv []string) error {
	if len(argv) == 0 {
		return errors.New("queue requires a dialtone_mod command")
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	rowID, err := router.QueueCommandViaShell(db, repoRoot, argv)
	if err != nil {
		return err
	}
	commandTarget, _ := loadStateValue(db, sqlitestate.TmuxTargetKey)
	result, err := ensureDaemon(db, repoRoot)
	if err != nil {
		return err
	}
	fmt.Print(renderRouteReport(repoRoot, db, rowID, dispatch.BuildDialtoneCommand(argv), commandTarget, result))
	return nil
}

func runStatus(argv []string) error {
	opts, err := parseStatusArgs(argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	promptTarget, _ := loadStateValue(db, sqlitestate.TmuxPromptTargetKey)
	commandTarget, _ := loadStateValue(db, sqlitestate.TmuxTargetKey)
	queuedCount, _ := countQueuedRows(db)
	desiredRows, _ := modstate.LoadShellBus(db, "desired", 200)
	observedRows, _ := modstate.LoadShellBus(db, "observed", 200)

	var commandRow modstate.ShellBusRecord
	var commandBody dispatch.ShellCommandIntent
	hasCommand := false
	if opts.rowID > 0 {
		record, ok, err := modstate.LoadShellBusRecord(db, opts.rowID)
		if err != nil {
			return err
		}
		if ok {
			body, bodyOK := decodeIntentBody(record)
			if bodyOK {
				commandRow = record
				commandBody = body
				hasCommand = true
			}
		}
	} else {
		commandRow, commandBody, hasCommand = latestDesiredCommandRecord(desiredRows)
	}

	processes, _ := loadProcessReportFn()
	total, totalBackground, dialtoneModCount, dialtoneModBackground, dialtoneCount, workerCount, ensureCount := summarizeProcessRecords(processes)

	var out bytes.Buffer
	fmt.Fprintf(&out, "state_source\tsqlite_cached\n")
	fmt.Fprintf(&out, "state_dir\t%s\n", sqlitestate.ResolveStateDir(repoRoot))
	fmt.Fprintf(&out, "state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Fprintf(&out, "logs_dir\t%s\n", sqlitestate.ResolveLogsDir(repoRoot))
	fmt.Fprintf(&out, "command_logs_dir\t%s\n", sqlitestate.ResolveCommandLogsDir(repoRoot))
	fmt.Fprintf(&out, "prompt_target\t%s\n", printableTarget(promptTarget))
	fmt.Fprintf(&out, "command_target\t%s\n", printableTarget(commandTarget))
	fmt.Fprintf(&out, "queued\t%d\n", queuedCount)
	writeStateKV(&out, db, "bootstrap_status", "bootstrap.status")
	writeStateKV(&out, db, "bootstrap_mode", "bootstrap.mode")
	writeStateKV(&out, db, "bootstrap_shell", "bootstrap.shell")
	writeStateKV(&out, db, "bootstrap_completed_at", "bootstrap.completed_at")
	writeStateKV(&out, db, "bootstrap_log", "bootstrap.log_path")
	writeStateKV(&out, db, "dialtone_pid", "dialtone.daemon.pid")
	writeStateKV(&out, db, "dialtone_status", "dialtone.daemon.status")
	writeStateKV(&out, db, "dialtone_started_at", "dialtone.daemon.started_at")
	writeStateKV(&out, db, "dialtone_heartbeat", "dialtone.daemon.heartbeat_at")
	writeStateKV(&out, db, "dialtone_log", "dialtone.daemon.log_path")
	writeStateKV(&out, db, "worker_status", sqlitestate.ShellWorkerStatusKey)
	writeStateKV(&out, db, "worker_pane", sqlitestate.ShellWorkerPaneKey)
	writeStateKV(&out, db, "worker_heartbeat", sqlitestate.ShellWorkerHeartbeatKey)
	writeStateKV(&out, db, "worker_current_row", sqlitestate.ShellWorkerCurrentRowIDKey)
	writeStateKV(&out, db, "worker_current_command", sqlitestate.ShellWorkerCurrentCommandKey)
	writeStateKV(&out, db, "worker_last_row", sqlitestate.ShellWorkerLastRowIDKey)
	writeStateKV(&out, db, "worker_last_status", sqlitestate.ShellWorkerLastStatusKey)
	writeStateKV(&out, db, "worker_last_exit_code", sqlitestate.ShellWorkerLastExitCodeKey)
	writeStateKV(&out, db, "worker_last_summary", sqlitestate.ShellWorkerLastSummaryKey)
	fmt.Fprintf(&out, "dialtone_processes_running\t%d\n", total)
	fmt.Fprintf(&out, "dialtone_processes_background\t%d\n", totalBackground)
	fmt.Fprintf(&out, "dialtone_mod_running\t%d\n", dialtoneModCount)
	fmt.Fprintf(&out, "dialtone_mod_background\t%d\n", dialtoneModBackground)
	fmt.Fprintf(&out, "dialtone_running\t%d\n", dialtoneCount)
	fmt.Fprintf(&out, "worker_running\t%d\n", workerCount)
	fmt.Fprintf(&out, "ensure_running\t%d\n", ensureCount)
	if hasCommand {
		writeCommandStatus(&out, commandRow, commandBody, opts.full)
	}
	if summary := latestPaneSnapshot(observedRows, promptTarget); summary != "" {
		fmt.Fprintf(&out, "prompt_snapshot\t%s\n", summary)
	}
	if summary := latestPaneSnapshot(observedRows, commandTarget); summary != "" {
		fmt.Fprintf(&out, "command_snapshot\t%s\n", summary)
	}
	if opts.full {
		if text := latestPaneSnapshotText(observedRows, promptTarget); text != "" {
			fmt.Fprintf(&out, "prompt_text\n%s\n", text)
		}
		if text := latestPaneSnapshotText(observedRows, commandTarget); text != "" {
			fmt.Fprintf(&out, "command_text\n%s\n", text)
		}
	}
	fmt.Print(strings.TrimRight(out.String(), "\n"))
	if out.Len() > 0 {
		fmt.Println()
	}
	return nil
}

func runCommands(argv []string) error {
	opts, err := parseCommandsArgs(argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	loadLimit := opts.limit
	if loadLimit < 100 {
		loadLimit = 100
	}
	scope := strings.TrimSpace(opts.scope)
	if scope == "all" {
		scope = ""
	}
	rows, err := modstate.LoadShellBus(db, scope, loadLimit)
	if err != nil {
		return err
	}
	filtered := make([]modstate.ShellBusRecord, 0, len(rows))
	for _, row := range rows {
		if !matchesCommandFilters(row, opts) {
			continue
		}
		filtered = append(filtered, row)
		if len(filtered) >= opts.limit {
			break
		}
	}
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Printf("rows\t%d\n", len(filtered))
	fmt.Println("id\tscope\tsubject\taction\tstatus\tactor\tsession\tpane\tref_id\texit_code\truntime_ms\tlog_path\tsummary\tcommand\tupdated_at")
	for _, row := range filtered {
		body, _ := decodeIntentBody(row)
		exitCode := "pending"
		if row.Status == "done" || row.Status == "failed" || body.ExitCode != 0 {
			exitCode = strconv.Itoa(body.ExitCode)
		}
		runtime := "pending"
		if runtimeMS, ok := commandRuntimeMillis(row, body); ok {
			runtime = strconv.FormatInt(runtimeMS, 10)
		}
		commandText := strings.TrimSpace(body.DisplayCommand)
		if commandText == "" {
			commandText = strings.TrimSpace(body.Command)
		}
		logPath := strings.TrimSpace(body.LogPath)
		if logPath == "" && row.Subject == "command" && row.ID > 0 {
			logPath = sqlitestate.ResolveCommandLogPath(repoRoot, row.ID)
		}
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.ID,
			row.Scope,
			row.Subject,
			row.Action,
			row.Status,
			row.Actor,
			printableTarget(row.Session),
			printableTarget(row.Pane),
			row.RefID,
			exitCode,
			runtime,
			logPath,
			strings.TrimSpace(body.Summary),
			commandText,
			row.UpdatedAt,
		)
	}
	return nil
}

func runCommand(argv []string) error {
	opts, err := parseCommandArgs(argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	var row modstate.ShellBusRecord
	var body dispatch.ShellCommandIntent
	found := false
	if opts.rowID > 0 {
		record, ok, err := modstate.LoadShellBusRecord(db, opts.rowID)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("shell bus row %d not found", opts.rowID)
		}
		row = record
		body, _ = decodeIntentBody(record)
		found = true
	} else {
		rows, err := modstate.LoadShellBus(db, "desired", 100)
		if err != nil {
			return err
		}
		row, body, found = latestDesiredCommandRecord(rows)
	}
	if !found {
		return errors.New("no command rows found")
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Fprintf(&out, "command_scope\t%s\n", row.Scope)
	fmt.Fprintf(&out, "command_subject\t%s\n", row.Subject)
	fmt.Fprintf(&out, "command_action\t%s\n", row.Action)
	fmt.Fprintf(&out, "command_actor\t%s\n", row.Actor)
	fmt.Fprintf(&out, "command_session\t%s\n", printableTarget(row.Session))
	fmt.Fprintf(&out, "command_pane\t%s\n", printableTarget(row.Pane))
	fmt.Fprintf(&out, "command_ref_id\t%d\n", row.RefID)
	writeCommandStatus(&out, row, body, opts.full)
	fmt.Print(out.String())
	return nil
}

func runLog(argv []string) error {
	opts, err := parseLogArgs(argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	kind := strings.TrimSpace(opts.kind)
	if kind == "" {
		kind = "daemon"
	}
	logPath, err := resolveDialtoneLogPath(db, repoRoot, kind, opts.rowID)
	if err != nil {
		return err
	}
	fmt.Printf("kind\t%s\n", kind)
	fmt.Printf("path\t%s\n", logPath)
	if opts.pathOnly {
		return nil
	}
	text, err := tailFileLines(logPath, opts.lines)
	if err != nil {
		return err
	}
	fmt.Printf("lines\t%d\n", opts.lines)
	fmt.Printf("text\n%s\n", text)
	return nil
}

func runProtocolRuns(argv []string) error {
	limit, err := parseLimitOnlyArgs("protocol-runs", argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := modstate.LoadProtocolRuns(db, limit)
	if err != nil {
		return err
	}
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Printf("rows\t%d\n", len(rows))
	fmt.Println("id\tname\tstatus\tprompt_target\tcommand_target\tstarted_at\tfinished_at\tresult_text\terror_text")
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.ID,
			row.Name,
			row.Status,
			printableTarget(row.PromptTarget),
			printableTarget(row.CommandTarget),
			row.StartedAt,
			row.FinishedAt,
			row.ResultText,
			row.ErrorText,
		)
	}
	return nil
}

func runProtocolRun(argv []string) error {
	opts, err := parseProtocolRunArgs(argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	run, found, err := loadProtocolRunByID(db, opts.runID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("protocol run %d not found", opts.runID)
	}
	events, err := modstate.LoadProtocolEvents(db, opts.runID)
	if err != nil {
		return err
	}
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Printf("run_id\t%d\n", run.ID)
	fmt.Printf("name\t%s\n", run.Name)
	fmt.Printf("status\t%s\n", run.Status)
	fmt.Printf("prompt_target\t%s\n", printableTarget(run.PromptTarget))
	fmt.Printf("command_target\t%s\n", printableTarget(run.CommandTarget))
	fmt.Printf("started_at\t%s\n", run.StartedAt)
	fmt.Printf("finished_at\t%s\n", run.FinishedAt)
	if strings.TrimSpace(run.ResultText) != "" {
		fmt.Printf("result_text\t%s\n", run.ResultText)
	}
	if strings.TrimSpace(run.ErrorText) != "" {
		fmt.Printf("error_text\t%s\n", run.ErrorText)
	}
	if opts.full && strings.TrimSpace(run.PromptText) != "" {
		fmt.Printf("prompt_text\n%s\n", strings.TrimSpace(run.PromptText))
	}
	fmt.Printf("events\t%d\n", len(events))
	fmt.Println("event_index\tevent_type\tqueue_name\tqueue_row_id\tpane_target\tcommand_text\tmessage_text\tcreated_at")
	for _, event := range events {
		fmt.Printf("%d\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
			event.EventIndex,
			event.EventType,
			event.QueueName,
			event.QueueRowID,
			printableTarget(event.PaneTarget),
			event.CommandText,
			event.MessageText,
			event.CreatedAt,
		)
	}
	return nil
}

func runTestRuns(argv []string) error {
	limit, err := parseLimitOnlyArgs("test-runs", argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := modstate.LoadModTestRuns(db, limit)
	if err != nil {
		return err
	}
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Printf("rows\t%d\n", len(rows))
	fmt.Println("id\tplan_name\tstatus\ttotal_steps\tpassed_steps\tfailed_steps\tskipped_steps\tstop_on_error\tstarted_at\tfinished_at\terror_text")
	for _, row := range rows {
		stopOnError := "false"
		if row.StopOnError {
			stopOnError = "true"
		}
		fmt.Printf("%d\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\n",
			row.ID,
			row.PlanName,
			row.Status,
			row.TotalSteps,
			row.PassedSteps,
			row.FailedSteps,
			row.SkippedSteps,
			stopOnError,
			row.StartedAt,
			row.FinishedAt,
			row.ErrorText,
		)
	}
	return nil
}

func runTestRun(argv []string) error {
	opts, err := parseTestRunArgs(argv)
	if err != nil {
		return err
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	db, err := openStateDB(repoRoot)
	if err != nil {
		return err
	}
	defer db.Close()

	run, ok, err := modstate.LoadModTestRun(db, opts.runID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("test run %d not found", opts.runID)
	}
	steps, err := modstate.LoadModTestRunSteps(db, opts.runID)
	if err != nil {
		return err
	}
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Printf("run_id\t%d\n", run.ID)
	fmt.Printf("plan_name\t%s\n", run.PlanName)
	fmt.Printf("status\t%s\n", run.Status)
	fmt.Printf("total_steps\t%d\n", run.TotalSteps)
	fmt.Printf("passed_steps\t%d\n", run.PassedSteps)
	fmt.Printf("failed_steps\t%d\n", run.FailedSteps)
	fmt.Printf("skipped_steps\t%d\n", run.SkippedSteps)
	fmt.Printf("stop_on_error\t%t\n", run.StopOnError)
	fmt.Printf("started_at\t%s\n", run.StartedAt)
	fmt.Printf("finished_at\t%s\n", run.FinishedAt)
	if strings.TrimSpace(run.ErrorText) != "" {
		fmt.Printf("error_text\t%s\n", run.ErrorText)
	}
	fmt.Printf("steps\t%d\n", len(steps))
	fmt.Println("step_index\tmod_name\tmod_version\tstatus\texit_code\tqueue_id\truntime_ms\tvisible_tmux\trequires_nix\tserial_group\tcommand_text\terror_text")
	for _, step := range steps {
		fmt.Printf("%d\t%s\t%s\t%s\t%d\t%d\t%d\t%t\t%t\t%s\t%s\t%s\n",
			step.StepIndex,
			step.ModName,
			step.ModVersion,
			step.Status,
			step.ExitCode,
			step.QueueID,
			step.RuntimeMS,
			step.VisibleTmux,
			step.RequiresNix,
			step.SerialGroup,
			step.CommandText,
			step.ErrorText,
		)
		if opts.full && strings.TrimSpace(step.OutputText) != "" {
			fmt.Printf("step_%d_output\n%s\n", step.StepIndex, strings.TrimSpace(step.OutputText))
		}
	}
	return nil
}

func parseCommandsArgs(argv []string) (commandsOptions, error) {
	fs := flag.NewFlagSet("dialtone commands", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "Maximum shell_bus rows to print")
	scope := fs.String("scope", "desired", "Scope filter: desired, observed, or all")
	subject := fs.String("subject", "command", "Subject filter: command, prompt, pane, or all")
	status := fs.String("status", "", "Optional exact status filter")
	session := fs.String("session", "", "Optional exact session filter")
	pane := fs.String("pane", "", "Optional exact pane filter")
	if err := fs.Parse(argv); err != nil {
		return commandsOptions{}, err
	}
	if fs.NArg() != 0 {
		return commandsOptions{}, errors.New("commands does not accept positional arguments")
	}
	if *limit <= 0 {
		return commandsOptions{}, errors.New("--limit must be positive")
	}
	return commandsOptions{
		limit:   *limit,
		scope:   strings.TrimSpace(*scope),
		subject: strings.TrimSpace(*subject),
		status:  strings.TrimSpace(*status),
		session: strings.TrimSpace(*session),
		pane:    strings.TrimSpace(*pane),
	}, nil
}

func parseCommandArgs(argv []string) (commandOptions, error) {
	fs := flag.NewFlagSet("dialtone command", flag.ContinueOnError)
	rowID := fs.Int64("row-id", 0, "Optional exact shell_bus row id to print")
	full := fs.Bool("full", false, "Print full command output")
	if err := fs.Parse(argv); err != nil {
		return commandOptions{}, err
	}
	if fs.NArg() != 0 {
		return commandOptions{}, errors.New("command does not accept positional arguments")
	}
	return commandOptions{rowID: *rowID, full: *full}, nil
}

func parseLogArgs(argv []string) (logOptions, error) {
	fs := flag.NewFlagSet("dialtone log", flag.ContinueOnError)
	kind := fs.String("kind", "daemon", "Log kind: daemon, ensure, bootstrap, or command")
	rowID := fs.Int64("row-id", 0, "Required for --kind command when multiple rows exist")
	lines := fs.Int("lines", 80, "Number of trailing lines to print")
	pathOnly := fs.Bool("path-only", false, "Print only the resolved log path")
	if err := fs.Parse(argv); err != nil {
		return logOptions{}, err
	}
	if fs.NArg() != 0 {
		return logOptions{}, errors.New("log does not accept positional arguments")
	}
	if *lines <= 0 {
		return logOptions{}, errors.New("--lines must be positive")
	}
	return logOptions{kind: strings.TrimSpace(*kind), rowID: *rowID, lines: *lines, pathOnly: *pathOnly}, nil
}

func parseProtocolRunArgs(argv []string) (protocolRunOptions, error) {
	fs := flag.NewFlagSet("dialtone protocol-run", flag.ContinueOnError)
	runID := fs.Int64("run", 0, "Protocol run id")
	full := fs.Bool("full", false, "Print the stored prompt text")
	if err := fs.Parse(argv); err != nil {
		return protocolRunOptions{}, err
	}
	if fs.NArg() != 0 {
		return protocolRunOptions{}, errors.New("protocol-run does not accept positional arguments")
	}
	if *runID <= 0 {
		return protocolRunOptions{}, errors.New("protocol-run requires --run <id>")
	}
	return protocolRunOptions{runID: *runID, full: *full}, nil
}

func parseTestRunArgs(argv []string) (testRunOptions, error) {
	fs := flag.NewFlagSet("dialtone test-run", flag.ContinueOnError)
	runID := fs.Int64("run", 0, "SQLite mod test run id")
	full := fs.Bool("full", false, "Print full step output text")
	if err := fs.Parse(argv); err != nil {
		return testRunOptions{}, err
	}
	if fs.NArg() != 0 {
		return testRunOptions{}, errors.New("test-run does not accept positional arguments")
	}
	if *runID <= 0 {
		return testRunOptions{}, errors.New("test-run requires --run <id>")
	}
	return testRunOptions{runID: *runID, full: *full}, nil
}

func parseLimitOnlyArgs(name string, argv []string) (int, error) {
	fs := flag.NewFlagSet("dialtone "+name, flag.ContinueOnError)
	limit := fs.Int("limit", 20, "Maximum rows to print")
	if err := fs.Parse(argv); err != nil {
		return 0, err
	}
	if fs.NArg() != 0 {
		return 0, fmt.Errorf("%s does not accept positional arguments", name)
	}
	if *limit <= 0 {
		return 0, errors.New("--limit must be positive")
	}
	return *limit, nil
}

func matchesCommandFilters(row modstate.ShellBusRecord, opts commandsOptions) bool {
	scope := strings.TrimSpace(opts.scope)
	if scope != "" && scope != "all" && strings.TrimSpace(row.Scope) != scope {
		return false
	}
	subject := strings.TrimSpace(opts.subject)
	if subject != "" && subject != "all" && strings.TrimSpace(row.Subject) != subject {
		return false
	}
	if strings.TrimSpace(opts.status) != "" && strings.TrimSpace(row.Status) != strings.TrimSpace(opts.status) {
		return false
	}
	if strings.TrimSpace(opts.session) != "" && strings.TrimSpace(row.Session) != strings.TrimSpace(opts.session) {
		return false
	}
	if strings.TrimSpace(opts.pane) != "" && strings.TrimSpace(row.Pane) != strings.TrimSpace(opts.pane) {
		return false
	}
	return true
}

func parseStatusArgs(argv []string) (statusOptions, error) {
	fs := flag.NewFlagSet("dialtone status", flag.ContinueOnError)
	rowID := fs.Int64("row-id", 0, "Optional exact shell_bus row id to print")
	full := fs.Bool("full", false, "Print the full cached prompt/command pane text and command output")
	_ = fs.Bool("sync", false, "Accepted for compatibility; dialtone status reads cached SQLite state only")
	_ = fs.Int("limit", 40, "Accepted for compatibility; dialtone status reads a fixed recent SQLite window")
	if err := fs.Parse(argv); err != nil {
		return statusOptions{}, err
	}
	if fs.NArg() != 0 {
		return statusOptions{}, errors.New("status does not accept positional arguments")
	}
	return statusOptions{rowID: *rowID, full: *full}, nil
}

func ensureDaemon(db *sql.DB, repoRoot string) (ensureResult, error) {
	logPath, _, err := loadOptionalStateValue(db, "dialtone.daemon.log_path")
	if err != nil {
		return ensureResult{}, err
	}
	pidText, _, err := loadOptionalStateValue(db, "dialtone.daemon.pid")
	if err != nil {
		return ensureResult{}, err
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(pidText))
	if healthy, err := daemonHealthy(db, 10*time.Second); err != nil {
		return ensureResult{}, err
	} else if healthy {
		return ensureResult{PID: pid, LogPath: logPath, State: "running"}, nil
	}
	if pid > 0 && processAliveFn(pid) {
		legacy, err := isLegacyWrapperDaemon(pid)
		if err != nil {
			return ensureResult{}, err
		}
		if !legacy {
			return ensureResult{PID: pid, LogPath: logPath, State: "existing"}, nil
		}
		if err := stopLegacyDaemon(pid); err != nil {
			return ensureResult{}, err
		}
	}

	pid, logPath, err = startDetachedSelfFn(repoRoot)
	if err != nil {
		return ensureResult{}, err
	}
	startedAt := nowUTC().Format(time.RFC3339)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.pid", strconv.Itoa(pid)); err != nil {
		return ensureResult{}, err
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.log_path", logPath); err != nil {
		return ensureResult{}, err
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.status", "starting"); err != nil {
		return ensureResult{}, err
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.started_at", startedAt); err != nil {
		return ensureResult{}, err
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, "dialtone.daemon.heartbeat_at", startedAt); err != nil {
		return ensureResult{}, err
	}
	return ensureResult{PID: pid, LogPath: logPath, State: "started"}, nil
}

func daemonHealthy(db *sql.DB, maxAge time.Duration) (bool, error) {
	statusText, _, err := loadOptionalStateValue(db, "dialtone.daemon.status")
	if err != nil {
		return false, err
	}
	pidText, _, err := loadOptionalStateValue(db, "dialtone.daemon.pid")
	if err != nil {
		return false, err
	}
	heartbeatText, _, err := loadOptionalStateValue(db, "dialtone.daemon.heartbeat_at")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(statusText) != "running" || strings.TrimSpace(pidText) == "" || strings.TrimSpace(heartbeatText) == "" {
		return false, nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidText))
	if err != nil || !processAliveFn(pid) {
		return false, nil
	}
	if legacy, err := isLegacyWrapperDaemon(pid); err == nil && legacy {
		return false, nil
	}
	heartbeat, ok := parseSQLiteTime(heartbeatText)
	if !ok {
		return false, nil
	}
	return nowUTC().Sub(heartbeat) <= maxAge, nil
}

func isLegacyWrapperDaemon(pid int) (bool, error) {
	command, err := processCommandFn(pid)
	if err != nil {
		return false, nil
	}
	return isLegacyWrapperDaemonCommand(command), nil
}

func isLegacyWrapperDaemonCommand(command string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(command)), "__dialtone serve")
}

func stopLegacyDaemon(pid int) error {
	if pid <= 0 {
		return nil
	}
	if err := terminateProcessFn(pid); err != nil {
		return err
	}
	deadline := nowUTC().Add(2 * time.Second)
	for nowUTC().Before(deadline) {
		if !processAliveFn(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("legacy dialtone daemon %d did not exit", pid)
}

func ensureWorkerAsync(repoRoot string, db *sql.DB) (ensureResult, error) {
	ready, err := dispatch.ShellReady(db)
	if err != nil {
		return ensureResult{}, err
	}
	healthy, err := router.ShellWorkerHealthy(db, 5*time.Second)
	if err != nil {
		return ensureResult{}, err
	}
	if ready && healthy {
		pid, logPath, err := existingEnsureProcess(db)
		if err != nil {
			return ensureResult{}, err
		}
		return ensureResult{PID: pid, LogPath: logPath, State: "worker"}, nil
	}
	if pid, logPath, ok, err := existingEnsureProcessIfAlive(db); err != nil {
		return ensureResult{}, err
	} else if ok {
		return ensureResult{PID: pid, LogPath: logPath, State: "existing"}, nil
	}
	pid, logPath, err := startDetachedCmdFn(repoRoot, "shell", "v1", "ensure-worker", "--wait-seconds", "30")
	if err != nil {
		return ensureResult{}, err
	}
	startedAt := nowUTC().Format(time.RFC3339)
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, strconv.Itoa(pid)); err != nil {
		return ensureResult{}, err
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, logPath); err != nil {
		return ensureResult{}, err
	}
	if err := modstate.UpsertStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureStartedAtKey, startedAt); err != nil {
		return ensureResult{}, err
	}
	return ensureResult{PID: pid, LogPath: logPath, State: "started"}, nil
}

func clearEnsureStateIfShellReady(db *sql.DB, maxAge time.Duration) (bool, error) {
	if db == nil {
		return false, nil
	}
	ready, err := dispatch.ShellReady(db)
	if err != nil || !ready {
		return false, err
	}
	healthy, err := router.ShellWorkerHealthy(db, maxAge)
	if err != nil || !healthy {
		return false, err
	}
	if err := modstate.DeleteStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey); err != nil {
		return false, err
	}
	if err := modstate.DeleteStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey); err != nil {
		return false, err
	}
	if err := modstate.DeleteStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureStartedAtKey); err != nil {
		return false, err
	}
	return true, nil
}

func existingEnsureProcess(db *sql.DB) (int, string, error) {
	pidText, ok, err := loadOptionalStateValue(db, sqlitestate.ShellEnsurePIDKey)
	if err != nil || !ok {
		return 0, "", err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(pidText))
	if err != nil {
		return 0, "", nil
	}
	logPath, _, err := loadOptionalStateValue(db, sqlitestate.ShellEnsureLogPathKey)
	if err != nil {
		return 0, "", err
	}
	return pid, logPath, nil
}

func existingEnsureProcessIfAlive(db *sql.DB) (int, string, bool, error) {
	pid, logPath, err := existingEnsureProcess(db)
	if err != nil || pid <= 0 {
		return 0, "", false, err
	}
	return pid, logPath, processAliveFn(pid), nil
}

func loadOptionalStateValue(db *sql.DB, key string) (string, bool, error) {
	record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
	if err != nil || !ok {
		return "", ok, err
	}
	return strings.TrimSpace(record.Value), true, nil
}

func loadStateValue(db *sql.DB, key string) (string, error) {
	value, _, err := loadOptionalStateValue(db, key)
	return value, err
}

func resolveDialtoneLogPath(db *sql.DB, repoRoot, kind string, rowID int64) (string, error) {
	switch strings.TrimSpace(kind) {
	case "daemon":
		value, ok, err := loadOptionalStateValue(db, "dialtone.daemon.log_path")
		if err != nil {
			return "", err
		}
		if !ok || strings.TrimSpace(value) == "" {
			return "", errors.New("no dialtone daemon log path recorded")
		}
		return strings.TrimSpace(value), nil
	case "ensure":
		value, ok, err := loadOptionalStateValue(db, sqlitestate.ShellEnsureLogPathKey)
		if err != nil {
			return "", err
		}
		if !ok || strings.TrimSpace(value) == "" {
			return "", errors.New("no ensure-worker log path recorded")
		}
		return strings.TrimSpace(value), nil
	case "bootstrap":
		value, ok, err := loadOptionalStateValue(db, "bootstrap.log_path")
		if err != nil {
			return "", err
		}
		if !ok || strings.TrimSpace(value) == "" {
			return "", errors.New("no bootstrap log path recorded")
		}
		return strings.TrimSpace(value), nil
	case "command":
		var row modstate.ShellBusRecord
		var ok bool
		var err error
		if rowID > 0 {
			row, ok, err = modstate.LoadShellBusRecord(db, rowID)
			if err != nil {
				return "", err
			}
			if !ok {
				return "", fmt.Errorf("shell bus row %d not found", rowID)
			}
		} else {
			rows, err := modstate.LoadShellBus(db, "desired", 100)
			if err != nil {
				return "", err
			}
			var body dispatch.ShellCommandIntent
			row, body, ok = latestDesiredCommandRecord(rows)
			if ok && strings.TrimSpace(body.LogPath) != "" {
				return strings.TrimSpace(body.LogPath), nil
			}
			if !ok {
				return "", errors.New("no command rows found")
			}
		}
		body, _ := decodeIntentBody(row)
		if strings.TrimSpace(body.LogPath) != "" {
			return strings.TrimSpace(body.LogPath), nil
		}
		return sqlitestate.ResolveCommandLogPath(repoRoot, row.ID), nil
	default:
		return "", fmt.Errorf("unsupported --kind %q", kind)
	}
}

func tailFileLines(path string, lines int) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("log path is required")
	}
	if lines <= 0 {
		lines = 80
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	text := strings.TrimRight(string(raw), "\n")
	if text == "" {
		return "", nil
	}
	parts := strings.Split(text, "\n")
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return strings.Join(parts, "\n"), nil
}

func loadProtocolRunByID(db *sql.DB, runID int64) (modstate.ProtocolRunRecord, bool, error) {
	var record modstate.ProtocolRunRecord
	err := db.QueryRow(`select id, name, status, prompt_text, prompt_target, command_target, result_text, error_text, started_at, finished_at
		from protocol_runs
		where id = ?`, runID).Scan(
		&record.ID, &record.Name, &record.Status, &record.PromptText, &record.PromptTarget,
		&record.CommandTarget, &record.ResultText, &record.ErrorText, &record.StartedAt, &record.FinishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return modstate.ProtocolRunRecord{}, false, nil
		}
		return modstate.ProtocolRunRecord{}, false, err
	}
	return record, true, nil
}

func renderRouteReport(repoRoot string, db *sql.DB, rowID int64, commandText, commandTarget string, result ensureResult) string {
	processes, _ := loadProcessReportFn()
	total, totalBackground, dialtoneModCount, dialtoneModBackground, dialtoneCount, workerCount, ensureCount := summarizeProcessRecords(processes)

	bootstrapStatus, _ := loadStateValue(db, "bootstrap.status")
	bootstrapMode, _ := loadStateValue(db, "bootstrap.mode")
	bootstrapShell, _ := loadStateValue(db, "bootstrap.shell")
	bootstrapLog, _ := loadStateValue(db, "bootstrap.log_path")
	persistedDaemonPID, _ := loadStateValue(db, "dialtone.daemon.pid")
	persistedDaemonStatus, _ := loadStateValue(db, "dialtone.daemon.status")
	persistedDaemonLog, _ := loadStateValue(db, "dialtone.daemon.log_path")
	workerStatus, _ := loadStateValue(db, sqlitestate.ShellWorkerStatusKey)
	workerPane, _ := loadStateValue(db, sqlitestate.ShellWorkerPaneKey)
	workerCurrentRow, _ := loadStateValue(db, sqlitestate.ShellWorkerCurrentRowIDKey)
	workerLastRow, _ := loadStateValue(db, sqlitestate.ShellWorkerLastRowIDKey)
	workerLastStatus, _ := loadStateValue(db, sqlitestate.ShellWorkerLastStatusKey)
	workerLastExitCode, _ := loadStateValue(db, sqlitestate.ShellWorkerLastExitCodeKey)
	workerLastSummary, _ := loadStateValue(db, sqlitestate.ShellWorkerLastSummaryKey)

	daemonPID := persistedDaemonPID
	if result.PID > 0 {
		daemonPID = strconv.Itoa(result.PID)
	}
	daemonStatus := persistedDaemonStatus
	if strings.TrimSpace(result.State) != "" {
		daemonStatus = strings.TrimSpace(result.State)
	}
	daemonLog := persistedDaemonLog
	if strings.TrimSpace(result.LogPath) != "" {
		daemonLog = strings.TrimSpace(result.LogPath)
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "route\tqueued\n")
	fmt.Fprintf(&out, "command_id\t%d\n", rowID)
	fmt.Fprintf(&out, "command_status\tqueued\n")
	fmt.Fprintf(&out, "command_pid\tpending\n")
	fmt.Fprintf(&out, "command_exit_code\tpending\n")
	fmt.Fprintf(&out, "command_runtime_ms\tpending\n")
	fmt.Fprintf(&out, "command\t%s\n", strings.TrimSpace(commandText))
	fmt.Fprintf(&out, "command_target\t%s\n", printableTarget(commandTarget))
	fmt.Fprintf(&out, "command_log_path\t%s\n", sqlitestate.ResolveCommandLogPath(repoRoot, rowID))
	fmt.Fprintf(&out, "state_dir\t%s\n", sqlitestate.ResolveStateDir(repoRoot))
	fmt.Fprintf(&out, "state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	fmt.Fprintf(&out, "logs_dir\t%s\n", sqlitestate.ResolveLogsDir(repoRoot))
	writeField(&out, "bootstrap_status", bootstrapStatus)
	writeField(&out, "bootstrap_mode", bootstrapMode)
	writeField(&out, "bootstrap_shell", bootstrapShell)
	writeField(&out, "bootstrap_log", bootstrapLog)
	writeField(&out, "dialtone_pid", daemonPID)
	writeField(&out, "dialtone_status", daemonStatus)
	writeField(&out, "dialtone_log", daemonLog)
	writeField(&out, "worker_status", workerStatus)
	writeField(&out, "worker_pane", workerPane)
	writeField(&out, "worker_current_row", workerCurrentRow)
	writeField(&out, "worker_last_row", workerLastRow)
	writeField(&out, "worker_last_status", workerLastStatus)
	writeField(&out, "worker_last_exit_code", workerLastExitCode)
	writeField(&out, "worker_last_summary", workerLastSummary)
	fmt.Fprintf(&out, "dialtone_processes_running\t%d\n", total)
	fmt.Fprintf(&out, "dialtone_processes_background\t%d\n", totalBackground)
	fmt.Fprintf(&out, "dialtone_mod_running\t%d\n", dialtoneModCount)
	fmt.Fprintf(&out, "dialtone_mod_background\t%d\n", dialtoneModBackground)
	fmt.Fprintf(&out, "dialtone_running\t%d\n", dialtoneCount)
	fmt.Fprintf(&out, "worker_running\t%d\n", workerCount)
	fmt.Fprintf(&out, "ensure_running\t%d\n", ensureCount)
	fmt.Fprintf(&out, "inspect\t./dialtone_mod dialtone v1 command --row-id %d --full\n", rowID)
	fmt.Fprintf(&out, "inspect_log\t./dialtone_mod dialtone v1 log --kind command --row-id %d\n", rowID)
	fmt.Fprintf(&out, "\n")
	fmt.Fprintf(&out, "%-8s %-8s %-6s %-6s %-12s %-10s %s\n", "PID", "PPID", "STAT", "TTY", "ROLE", "BACKGROUND", "COMMAND")
	for _, record := range processes {
		background := "no"
		if record.Background {
			background = "yes"
		}
		fmt.Fprintf(&out, "%-8d %-8d %-6s %-6s %-12s %-10s %s\n",
			record.PID,
			record.PPID,
			record.Stat,
			record.TTY,
			record.Role,
			background,
			record.Command,
		)
	}
	return out.String()
}

func writeField(out *bytes.Buffer, key, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(out, "%s\t%s\n", strings.TrimSpace(key), strings.TrimSpace(value))
}

func writeStateKV(out *bytes.Buffer, db *sql.DB, label, key string) {
	if db == nil {
		return
	}
	value, ok, err := loadOptionalStateValue(db, key)
	if err != nil || !ok || strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(out, "%s\t%s\n", strings.TrimSpace(label), strings.TrimSpace(value))
}

func countQueuedRows(db *sql.DB) (int, error) {
	if db == nil {
		return 0, nil
	}
	var count int
	err := db.QueryRow(`select count(*) from shell_bus where scope = 'desired' and status = 'queued'`).Scan(&count)
	return count, err
}

func decodeIntentBody(row modstate.ShellBusRecord) (dispatch.ShellCommandIntent, bool) {
	body, err := dispatch.DecodeIntentBody(row.BodyJSON)
	if err != nil {
		return dispatch.ShellCommandIntent{}, false
	}
	if strings.TrimSpace(body.DisplayCommand) == "" {
		if strings.TrimSpace(body.InnerCommand) != "" {
			body.DisplayCommand = strings.TrimSpace(body.InnerCommand)
		} else {
			body.DisplayCommand = strings.TrimSpace(body.Command)
		}
	}
	if strings.TrimSpace(body.Target) == "" {
		body.Target = strings.TrimSpace(row.Pane)
	}
	return body, true
}

func latestDesiredCommandRecord(rows []modstate.ShellBusRecord) (modstate.ShellBusRecord, dispatch.ShellCommandIntent, bool) {
	for _, row := range rows {
		if row.Subject != "command" || row.Action != "run" {
			continue
		}
		body, ok := decodeIntentBody(row)
		if ok {
			return row, body, true
		}
	}
	return modstate.ShellBusRecord{}, dispatch.ShellCommandIntent{}, false
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

func writeCommandStatus(out *bytes.Buffer, row modstate.ShellBusRecord, body dispatch.ShellCommandIntent, full bool) {
	fmt.Fprintf(out, "command_row_id\t%d\n", row.ID)
	fmt.Fprintf(out, "command_status\t%s\n", strings.TrimSpace(row.Status))
	if strings.TrimSpace(body.DisplayCommand) != "" {
		fmt.Fprintf(out, "command\t%s\n", strings.TrimSpace(body.DisplayCommand))
		fmt.Fprintf(out, "last_command\t%s\n", strings.TrimSpace(body.DisplayCommand))
	}
	if strings.TrimSpace(body.Target) != "" {
		fmt.Fprintf(out, "command_target\t%s\n", strings.TrimSpace(body.Target))
	}
	logPath := strings.TrimSpace(body.LogPath)
	if logPath == "" && row.Subject == "command" && row.ID > 0 {
		logPath = sqlitestate.ResolveCommandLogPath(strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")), row.ID)
	}
	if logPath != "" {
		fmt.Fprintf(out, "command_log_path\t%s\n", logPath)
	}
	if strings.TrimSpace(body.StartedAt) != "" {
		fmt.Fprintf(out, "command_started_at\t%s\n", strings.TrimSpace(body.StartedAt))
	}
	if strings.TrimSpace(body.FinishedAt) != "" {
		fmt.Fprintf(out, "command_finished_at\t%s\n", strings.TrimSpace(body.FinishedAt))
	}
	if body.PID > 0 {
		fmt.Fprintf(out, "command_pid\t%d\n", body.PID)
	} else {
		fmt.Fprintf(out, "command_pid\tpending\n")
	}
	if row.Status == "done" || row.Status == "failed" || body.ExitCode != 0 {
		fmt.Fprintf(out, "command_exit_code\t%d\n", body.ExitCode)
	} else {
		fmt.Fprintf(out, "command_exit_code\tpending\n")
	}
	if runtimeMS, ok := commandRuntimeMillis(row, body); ok {
		fmt.Fprintf(out, "command_runtime_ms\t%d\n", runtimeMS)
	} else {
		fmt.Fprintf(out, "command_runtime_ms\tpending\n")
	}
	if strings.TrimSpace(body.Summary) != "" {
		fmt.Fprintf(out, "command_summary\t%s\n", strings.TrimSpace(body.Summary))
	}
	if strings.TrimSpace(body.Error) != "" {
		fmt.Fprintf(out, "command_error\t%s\n", strings.TrimSpace(body.Error))
	}
	if full && strings.TrimSpace(body.Output) != "" {
		fmt.Fprintf(out, "command_output\n%s\n", strings.TrimSpace(body.Output))
		fmt.Fprintf(out, "last_command_output\n%s\n", strings.TrimSpace(body.Output))
	}
}

func commandRuntimeMillis(row modstate.ShellBusRecord, body dispatch.ShellCommandIntent) (int64, bool) {
	if body.RuntimeMS > 0 {
		return body.RuntimeMS, true
	}
	startedAt, ok := parseSQLiteTime(body.StartedAt)
	if !ok {
		return 0, false
	}
	finishedAt, ok := parseSQLiteTime(body.FinishedAt)
	if !ok {
		if updatedAt, updatedOK := parseSQLiteTime(row.UpdatedAt); updatedOK {
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
		for index := len(lines) - 1; index >= 0; index-- {
			if trimmed := strings.TrimSpace(lines[index]); trimmed != "" {
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

func parseSQLiteTime(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func summarizeProcessRecords(records []processRecord) (total, totalBackground, dialtoneModCount, dialtoneModBackground, dialtoneCount, workerCount, ensureCount int) {
	for _, record := range records {
		total++
		if record.Background {
			totalBackground++
		}
		switch strings.TrimSpace(record.Role) {
		case "dialtone_mod":
			dialtoneModCount++
			if record.Background {
				dialtoneModBackground++
			}
		case "dialtone":
			dialtoneCount++
		case "worker":
			workerCount++
		case "ensure":
			ensureCount++
		}
	}
	return
}

func loadProcessReport() ([]processRecord, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,stat=,tt=,command=").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	records := make([]processRecord, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}
		command := strings.Join(fields[4:], " ")
		role := dialtoneProcessRole(command)
		if role == "" {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		stat := fields[2]
		tty := fields[3]
		records = append(records, processRecord{
			PID:        pid,
			PPID:       ppid,
			Stat:       stat,
			TTY:        tty,
			Role:       role,
			Command:    command,
			Background: tty == "??" || !strings.Contains(stat, "+"),
		})
	}
	return records, nil
}

func dialtoneProcessRole(command string) string {
	trimmed := strings.TrimSpace(command)
	lower := strings.ToLower(trimmed)
	switch {
	case strings.Contains(lower, "shell v1 serve"):
		return "worker"
	case strings.Contains(lower, "shell v1 ensure-worker"):
		return "ensure"
	case strings.Contains(lower, "__dialtone serve"):
		return "dialtone"
	case strings.Contains(lower, " dialtone v1 serve"):
		return "dialtone"
	case strings.Contains(lower, "dialtone_mod"):
		return "dialtone_mod"
	}
	fields := strings.Fields(lower)
	if len(fields) >= 2 && filepath.Base(fields[0]) == "dialtone" && fields[1] == "serve" {
		return "dialtone"
	}
	return ""
}

func startDetachedSelf(repoRoot string) (int, string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return 0, "", err
	}
	return startDetachedExecutable(repoRoot, exePath, []string{"serve"}, "dialtone-daemon")
}

func startDetachedCommand(repoRoot string, args ...string) (int, string, error) {
	return startDetachedExecutable(repoRoot, filepath.Join(repoRoot, "dialtone_mod"), args, "dialtone-ensure")
}

func startDetachedExecutable(repoRoot, executable string, args []string, logPrefix string) (int, string, error) {
	logDir := filepath.Join(sqlitestate.ResolveStateDir(repoRoot), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, "", err
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("%s-%d.log", strings.TrimSpace(logPrefix), time.Now().UnixNano()))
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()

	cmd := exec.Command(executable, args...)
	cmd.Dir = strings.TrimSpace(repoRoot)
	cmd.Stdout = file
	cmd.Stderr = file
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(),
		"DIALTONE_REPO_ROOT="+strings.TrimSpace(repoRoot),
		"DIALTONE_STATE_DIR="+sqlitestate.ResolveStateDir(repoRoot),
		"DIALTONE_STATE_DB="+sqlitestate.ResolveStateDBPath(repoRoot),
		"DIALTONE_DAEMON_LOG_PATH="+logPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, "", err
	}
	if cmd.Process == nil {
		return 0, "", errors.New("process did not start")
	}
	return cmd.Process.Pid, logPath, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func processCommand(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func openStateDB(repoRoot string) (*sql.DB, error) {
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return nil, err
	}
	if err := modstate.EnsureSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
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

func printableTarget(value string) string {
	if strings.TrimSpace(value) == "" {
		return "missing"
	}
	return strings.TrimSpace(value)
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod dialtone v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  paths")
	fmt.Println("       Print repo/state/log paths used by the dialtone control plane")
	fmt.Println("  ensure")
	fmt.Println("       Ensure the standalone dialtone daemon process is running outside Nix")
	fmt.Println("  serve")
	fmt.Println("       Run the standalone dialtone daemon loop outside Nix")
	fmt.Println("  processes")
	fmt.Println("       Print the live dialtone/dialtone_mod/worker process table")
	fmt.Println("  status [--row-id <id>] [--full] [--sync=false]")
	fmt.Println("       Print the cached SQLite-backed control-plane status and latest command row")
	fmt.Println("  commands [--limit 20] [--scope desired|observed|all] [--subject command|prompt|pane|all] [--status STATUS]")
	fmt.Println("       List recent shell_bus rows without writing SQL")
	fmt.Println("  command [--row-id <id>] [--full]")
	fmt.Println("       Print one routed command row in detail")
	fmt.Println("  log [--kind daemon|ensure|bootstrap|command] [--row-id <id>] [--lines 80] [--path-only]")
	fmt.Println("       Resolve and print a known log file from cached state")
	fmt.Println("  queue <plain ./dialtone_mod args...>")
	fmt.Println("       Queue one routed plain dialtone_mod command into SQLite and ensure dialtone is running")
	fmt.Println("  protocol-runs [--limit 20]")
	fmt.Println("       List recorded end-to-end protocol test runs")
	fmt.Println("  protocol-run --run <id> [--full]")
	fmt.Println("       Print one protocol run and its ordered events")
	fmt.Println("  test-runs [--limit 20]")
	fmt.Println("       List recorded SQLite mod test runs")
	fmt.Println("  test-run --run <id> [--full]")
	fmt.Println("       Print one SQLite mod test run and its step outcomes")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}
