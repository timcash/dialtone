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
	case "serve":
		return runServe(args[1:])
	case "ensure":
		return runEnsure(args[1:])
	case "status", "state", "bootstrap":
		return runStatus(args[1:])
	case "queue":
		return runQueue(args[1:])
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
		if healthy, err := router.ShellWorkerHealthy(db, 5*time.Second); err == nil && healthy {
			_ = modstate.DeleteStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey)
			_ = modstate.DeleteStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey)
			_ = modstate.DeleteStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellEnsureStartedAtKey)
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
	fmt.Fprintf(&out, "state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
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
	fmt.Fprintf(&out, "inspect\t./dialtone_mod shell v1 status --row-id %d --full --sync=false\n", rowID)
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
	fmt.Println("  ensure")
	fmt.Println("       Ensure the standalone dialtone daemon process is running outside Nix")
	fmt.Println("  serve")
	fmt.Println("       Run the standalone dialtone daemon loop outside Nix")
	fmt.Println("  status [--row-id <id>] [--full] [--sync=false]")
	fmt.Println("       Print the cached SQLite-backed control-plane status and latest command row")
	fmt.Println("  queue <plain ./dialtone_mod args...>")
	fmt.Println("       Queue one routed plain dialtone_mod command into SQLite and ensure dialtone is running")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s error: %v\n", context, err)
	os.Exit(1)
}
