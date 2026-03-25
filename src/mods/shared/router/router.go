package router

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/sqlitestate"
)

type GoPackageRunner interface {
	Run(repoRoot, goBin, entry string, args ...string) error
}

func StartShellWorkflow(repoRoot, goBin string, runner GoPackageRunner) error {
	if runner == nil {
		return fmt.Errorf("go package runner is required")
	}
	return runner.Run(repoRoot, goBin, "./mods/shell/v1/cli", "start", "--run-tests=false")
}

func QueueCommandViaShell(db *sql.DB, repoRoot string, args []string) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("sqlite state is required to queue command via shell")
	}
	session := "codex-view"
	commandTarget := ""
	targetRecord, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey)
	if err != nil {
		return 0, err
	}
	if ok && strings.TrimSpace(targetRecord.Value) != "" {
		commandTarget = strings.TrimSpace(targetRecord.Value)
		if parts := strings.Split(commandTarget, ":"); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			session = strings.TrimSpace(parts[0])
		}
	}
	innerCommand := dispatch.BuildDialtoneCommand(args)
	body, err := dispatch.EncodeIntentBody(dispatch.ShellCommandIntent{
		Command:        innerCommand,
		InnerCommand:   innerCommand,
		DisplayCommand: innerCommand,
		Args:           append([]string(nil), args...),
		Target:         commandTarget,
	})
	if err != nil {
		return 0, err
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", session, commandTarget, body)
	if err != nil {
		return 0, err
	}
	updatedBody, err := dispatch.EncodeIntentBody(dispatch.ShellCommandIntent{
		Command:        innerCommand,
		InnerCommand:   innerCommand,
		DisplayCommand: innerCommand,
		Args:           append([]string(nil), args...),
		Target:         commandTarget,
		LogPath:        sqlitestate.ResolveCommandLogPath(repoRoot, rowID),
	})
	if err != nil {
		return 0, err
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "queued", 0, updatedBody); err != nil {
		return 0, err
	}
	return rowID, nil
}

func SyncShell(repoRoot, goBin string, runner GoPackageRunner, limit, waitSeconds int) error {
	if runner == nil {
		return fmt.Errorf("go package runner is required")
	}
	if limit <= 0 {
		limit = 20
	}
	if waitSeconds <= 0 {
		waitSeconds = 240
	}
	return runner.Run(repoRoot, goBin, "./mods/shell/v1/cli", "sync-once", "--limit", fmt.Sprintf("%d", limit), "--wait-seconds", fmt.Sprintf("%d", waitSeconds))
}

func BuildShellRunArgs(repoRoot string, args []string, waitSeconds int) []string {
	if waitSeconds <= 0 {
		waitSeconds = 240
	}
	return []string{"run", "--wait-seconds", fmt.Sprintf("%d", waitSeconds), dispatch.BuildDialtoneCommand(args)}
}

func RunCommandViaShell(repoRoot, goBin string, runner GoPackageRunner, args []string, waitSeconds int) error {
	if runner == nil {
		return fmt.Errorf("go package runner is required")
	}
	return runner.Run(repoRoot, goBin, "./mods/shell/v1/cli", BuildShellRunArgs(repoRoot, args, waitSeconds)...)
}

func ShellWorkerHealthy(db *sql.DB, maxAge time.Duration) (bool, error) {
	if db == nil {
		return false, nil
	}
	if err := modstate.EnsureSchema(db); err != nil {
		return false, err
	}
	statusRecord, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerStatusKey)
	if err != nil || !ok || strings.TrimSpace(statusRecord.Value) != "running" {
		return false, err
	}
	heartbeatRecord, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.ShellWorkerHeartbeatKey)
	if err != nil || !ok {
		return false, err
	}
	heartbeat, ok := parseRFC3339(heartbeatRecord.Value)
	if !ok {
		return false, nil
	}
	return time.Since(heartbeat) <= maxAge, nil
}

func parseRFC3339(raw string) (time.Time, bool) {
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

func WaitForShellBusCompletion(stateDB *sql.DB, rowID int64, timeout time.Duration) (modstate.ShellBusRecord, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		record, ok, err := modstate.LoadShellBusRecord(stateDB, rowID)
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
