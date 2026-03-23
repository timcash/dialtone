package dispatch

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
)

type ShellCommandIntent struct {
	Command        string   `json:"command,omitempty"`
	Expect         string   `json:"expect,omitempty"`
	InnerCommand   string   `json:"inner_command,omitempty"`
	DisplayCommand string   `json:"display_command,omitempty"`
	Args           []string `json:"args,omitempty"`
	Target         string   `json:"target,omitempty"`
	Summary        string   `json:"summary,omitempty"`
	Error          string   `json:"error,omitempty"`
	Output         string   `json:"output,omitempty"`
	StartedAt      string   `json:"started_at,omitempty"`
	FinishedAt     string   `json:"finished_at,omitempty"`
	PID            int      `json:"pid,omitempty"`
	ExitCode       int      `json:"exit_code"`
	RuntimeMS      int64    `json:"runtime_ms"`
}

func ShellReady(db *sql.DB) (bool, error) {
	if db == nil {
		return false, nil
	}
	if err := modstate.EnsureSchema(db); err != nil {
		return false, err
	}
	commandTarget, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxTargetKey)
	if err != nil {
		return false, err
	}
	if !ok || strings.TrimSpace(commandTarget.Value) == "" {
		return false, nil
	}
	promptTarget, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, sqlitestate.TmuxPromptTargetKey)
	if err != nil {
		return false, err
	}
	if !ok || strings.TrimSpace(promptTarget.Value) == "" {
		return false, nil
	}
	return true, nil
}

func ShouldRouteViaShell(modName, command string) bool {
	mod := strings.ToLower(strings.TrimSpace(modName))
	if mod == "" {
		return false
	}
	if mod == "shell" || mod == "ghostty" || mod == "tmux" || mod == "test" || mod == "dialtone" {
		return false
	}
	return true
}

func BuildDialtoneCommand(args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, "./dialtone_mod")
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func BuildTrackedDialtoneCommand(args []string, rowID int64) (string, string) {
	inner := BuildDialtoneCommand(args)
	expect := fmt.Sprintf("DIALTONE_CMD_DONE_%d", rowID)
	command := fmt.Sprintf("%s; __dialtone_status=$?; printf '%s exit=%%s\\n' \"$__dialtone_status\"", inner, expect)
	return command, expect
}

func BuildTrackedVisibleCommand(repoRoot string, args []string, rowID int64) (string, string, string) {
	inner := BuildDialtoneCommand(args)
	command, expect := BuildTrackedDialtoneCommand(args, rowID)
	return fmt.Sprintf("cd %s && %s", shellQuote(repoRoot), command), expect, inner
}

func EncodeIntentBody(body ShellCommandIntent) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func DecodeIntentBody(raw string) (ShellCommandIntent, error) {
	if strings.TrimSpace(raw) == "" {
		return ShellCommandIntent{}, nil
	}
	var body ShellCommandIntent
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return ShellCommandIntent{}, err
	}
	return body, nil
}

func ShouldExecuteDirectInPane(db *sql.DB, args []string, currentPane string) (bool, error) {
	if strings.TrimSpace(currentPane) == "" || db == nil {
		return false, nil
	}
	if err := modstate.EnsureSchema(db); err != nil {
		return false, err
	}
	inner := BuildDialtoneCommand(args)
	rows, err := modstate.LoadShellBus(db, "desired", 50)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Subject != "command" || row.Action != "run" || row.Status != "running" {
			continue
		}
		if strings.TrimSpace(row.Pane) != "" && strings.TrimSpace(row.Pane) != strings.TrimSpace(currentPane) {
			continue
		}
		body, err := DecodeIntentBody(row.BodyJSON)
		if err != nil {
			continue
		}
		if strings.TrimSpace(body.InnerCommand) == inner || strings.TrimSpace(body.Command) == inner {
			return true, nil
		}
	}
	return false, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"$`;&|()<>*?[]{}!") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
