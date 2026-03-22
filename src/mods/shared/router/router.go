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
	commandTarget, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey)
	if err != nil {
		return 0, err
	}
	if !ok || strings.TrimSpace(commandTarget.Value) == "" {
		return 0, fmt.Errorf("shell workflow is not ready: tmux command target is missing")
	}
	session := "codex-view"
	if parts := strings.Split(strings.TrimSpace(commandTarget.Value), ":"); len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		session = strings.TrimSpace(parts[0])
	}
	innerCommand := dispatch.BuildDialtoneCommand(args)
	initialBody, err := dispatch.EncodeIntentBody(dispatch.ShellCommandIntent{
		InnerCommand: innerCommand,
		Target:       strings.TrimSpace(commandTarget.Value),
	})
	if err != nil {
		return 0, err
	}
	rowID, err := modstate.EnqueueShellBus(db, "shell", "desired", "command", "run", "dialtone_mod", session, strings.TrimSpace(commandTarget.Value), initialBody)
	if err != nil {
		return 0, err
	}
	command, expect, innerCommand := dispatch.BuildTrackedVisibleCommand(repoRoot, args, rowID)
	finalBody, err := dispatch.EncodeIntentBody(dispatch.ShellCommandIntent{
		Command:      command,
		Expect:       expect,
		InnerCommand: innerCommand,
		Target:       strings.TrimSpace(commandTarget.Value),
	})
	if err != nil {
		return 0, err
	}
	if err := modstate.UpdateShellBusStatus(db, rowID, "queued", 0, finalBody); err != nil {
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
	rowID := time.Now().UnixNano()
	command, expect, _ := dispatch.BuildTrackedVisibleCommand(repoRoot, args, rowID)
	return []string{"run", "--wait-seconds", fmt.Sprintf("%d", waitSeconds), "--expect", expect, command}
}

func RunCommandViaShell(repoRoot, goBin string, runner GoPackageRunner, args []string, waitSeconds int) error {
	if runner == nil {
		return fmt.Errorf("go package runner is required")
	}
	return runner.Run(repoRoot, goBin, "./mods/shell/v1/cli", BuildShellRunArgs(repoRoot, args, waitSeconds)...)
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
