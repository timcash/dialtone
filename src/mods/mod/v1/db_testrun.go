package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dialtone/dev/internal/modstate"
)

type testRunRecord struct {
	ID          int64
	PlanName    string
	Status      string
	TotalSteps  int
	PassedSteps int
	FailedSteps int
	Skipped     int
	StartedAt   string
	FinishedAt  string
	ErrorText   string
}

type testRunStepRecord struct {
	RunID       int64
	StepIndex   int
	ModName     string
	ModVersion  string
	SerialGroup string
	VisibleTmux bool
	RequiresNix bool
	Status      string
	ExitCode    int
	QueueID     int64
	CommandText string
	OutputText  string
	ErrorText   string
	StartedAt   string
	FinishedAt  string
	RuntimeMS   int64
}

func runDBTestRun(args []string) error {
	fs := flag.NewFlagSet("mods db test-run", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	planName := fs.String("name", "default", "Test plan name to execute")
	stopOnError := fs.Bool("stop-on-error", true, "Stop after the first failed step")
	syncBefore := fs.Bool("sync", true, "Sync repo state into sqlite before executing the plan")
	updateReadmes := fs.Bool("update-readmes", true, "Write quickstart/test results into mod READMEs after the run")
	limit := fs.Int("limit", 0, "Optional maximum number of test steps to execute")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db test-run does not accept positional arguments")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(resolveStateDBPath(repoRoot, *dbPath))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ensureDBTestRunSchema(db); err != nil {
		return err
	}
	if *syncBefore {
		if _, err := modstate.SyncRepo(db, repoRoot, modstate.CaptureRuntimeEnv()); err != nil {
			return err
		}
	}
	plan, err := modstate.LoadTestPlan(db, strings.TrimSpace(*planName))
	if err != nil {
		return err
	}
	if *limit > 0 && len(plan) > *limit {
		plan = plan[:*limit]
	}
	if len(plan) == 0 {
		return fmt.Errorf("sqlite test plan %q is empty", strings.TrimSpace(*planName))
	}
	mods, err := modstate.LoadMods(db)
	if err != nil {
		return err
	}
	modByKey := map[string]modstate.ModRecord{}
	for _, mod := range mods {
		modByKey[mod.Name+":"+mod.Version] = mod
	}
	runID, err := startTestRun(db, strings.TrimSpace(*planName), len(plan), *stopOnError)
	if err != nil {
		return err
	}
	fmt.Printf("started sqlite test run %d plan=%s steps=%d\n", runID, strings.TrimSpace(*planName), len(plan))
	passed := 0
	failed := 0
	skipped := 0
	runStatus := "passed"
	runError := ""
	for _, step := range plan {
		outcome, execErr := executeTestPlanStep(db, repoRoot, runID, modByKey, step)
		if err := insertTestRunStep(db, outcome); err != nil {
			return err
		}
		switch outcome.Status {
		case "passed":
			passed++
		case "skipped":
			skipped++
		default:
			failed++
			runStatus = "failed"
			if runError == "" {
				runError = fmt.Sprintf("%s %s failed at step %d", outcome.ModName, outcome.ModVersion, outcome.StepIndex)
			}
		}
		fmt.Printf("step %d\t%s\t%s\t%s\t%s\n", outcome.StepIndex, outcome.ModName, outcome.ModVersion, outcome.Status, outcome.CommandText)
		if execErr != nil && *stopOnError {
			break
		}
	}
	if err := finishTestRun(db, runID, runStatus, passed, failed, skipped, runError); err != nil {
		return err
	}
	if *updateReadmes {
		if err := writeTestRunReadmes(db, repoRoot, runID, modByKey); err != nil {
			return err
		}
	}
	fmt.Printf("finished sqlite test run %d status=%s total=%d passed=%d failed=%d skipped=%d\n", runID, runStatus, len(plan), passed, failed, skipped)
	return nil
}

func runDBTestRuns(args []string) error {
	fs := flag.NewFlagSet("mods db test-runs", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	limit := fs.Int("limit", 20, "Maximum test runs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db test-runs does not accept positional arguments")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(resolveStateDBPath(repoRoot, *dbPath))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ensureDBTestRunSchema(db); err != nil {
		return err
	}
	rows, err := loadTestRuns(db, *limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\n",
			row.ID, row.PlanName, row.Status, row.TotalSteps, row.PassedSteps, row.FailedSteps, row.Skipped, row.StartedAt, row.FinishedAt)
	}
	return nil
}

func runDBTestRunSteps(args []string) error {
	fs := flag.NewFlagSet("mods db test-run-steps", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	runID := fs.Int64("run", 0, "Test run id to inspect")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db test-run-steps does not accept positional arguments")
	}
	if *runID <= 0 {
		return fmt.Errorf("db test-run-steps requires --run <id>")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	db, err := modstate.Open(resolveStateDBPath(repoRoot, *dbPath))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ensureDBTestRunSchema(db); err != nil {
		return err
	}
	rows, err := loadTestRunSteps(db, *runID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%d\t%s\t%s\t%s\t%d\t%d\t%s\t%s\n",
			row.RunID, row.StepIndex, row.ModName, row.ModVersion, row.Status, row.ExitCode, row.QueueID, row.StartedAt, row.CommandText)
	}
	return nil
}

func ensureDBTestRunSchema(db *sql.DB) error {
	if err := modstate.EnsureSchema(db); err != nil {
		return err
	}
	stmts := []string{
		`create table if not exists mod_test_runs (
			id integer primary key autoincrement,
			plan_name text not null,
			status text not null,
			total_steps integer not null default 0,
			passed_steps integer not null default 0,
			failed_steps integer not null default 0,
			skipped_steps integer not null default 0,
			stop_on_error integer not null default 1,
			error_text text not null default '',
			started_at text not null,
			finished_at text not null default ''
		);`,
		`create table if not exists mod_test_run_steps (
			run_id integer not null,
			step_index integer not null,
			mod_name text not null,
			mod_version text not null,
			serial_group text not null default '',
			visible_tmux integer not null default 0,
			requires_nix integer not null default 0,
			status text not null,
			exit_code integer not null default 0,
			queue_id integer not null default 0,
			command_text text not null,
			output_text text not null default '',
			error_text text not null default '',
			started_at text not null,
			finished_at text not null default '',
			runtime_ms integer not null default 0,
			primary key (run_id, step_index)
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func startTestRun(db *sql.DB, planName string, totalSteps int, stopOnError bool) (int64, error) {
	if err := ensureDBTestRunSchema(db); err != nil {
		return 0, err
	}
	result, err := db.Exec(`insert into mod_test_runs(plan_name, status, total_steps, stop_on_error, started_at)
		values(?, 'running', ?, ?, ?)`,
		strings.TrimSpace(planName), totalSteps, boolToIntLocal(stopOnError), nowRFC3339Local())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func finishTestRun(db *sql.DB, runID int64, status string, passed, failed, skipped int, errorText string) error {
	if err := ensureDBTestRunSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update mod_test_runs
		set status = ?, passed_steps = ?, failed_steps = ?, skipped_steps = ?, error_text = ?, finished_at = ?
		where id = ?`,
		strings.TrimSpace(status), passed, failed, skipped, strings.TrimSpace(errorText), nowRFC3339Local(), runID)
	return err
}

func insertTestRunStep(db *sql.DB, step testRunStepRecord) error {
	if err := ensureDBTestRunSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`insert into mod_test_run_steps(
		run_id, step_index, mod_name, mod_version, serial_group, visible_tmux, requires_nix,
		status, exit_code, queue_id, command_text, output_text, error_text, started_at, finished_at, runtime_ms
	) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.RunID, step.StepIndex, step.ModName, step.ModVersion, step.SerialGroup, boolToIntLocal(step.VisibleTmux), boolToIntLocal(step.RequiresNix),
		step.Status, step.ExitCode, step.QueueID, step.CommandText, step.OutputText, step.ErrorText, step.StartedAt, step.FinishedAt, step.RuntimeMS,
	)
	return err
}

func loadTestRuns(db *sql.DB, limit int) ([]testRunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`select id, plan_name, status, total_steps, passed_steps, failed_steps, skipped_steps, started_at, finished_at, error_text
		from mod_test_runs
		order by id desc
		limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []testRunRecord{}
	for rows.Next() {
		var record testRunRecord
		if err := rows.Scan(&record.ID, &record.PlanName, &record.Status, &record.TotalSteps, &record.PassedSteps, &record.FailedSteps, &record.Skipped, &record.StartedAt, &record.FinishedAt, &record.ErrorText); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func loadTestRun(db *sql.DB, runID int64) (testRunRecord, error) {
	var record testRunRecord
	err := db.QueryRow(`select id, plan_name, status, total_steps, passed_steps, failed_steps, skipped_steps, started_at, finished_at, error_text
		from mod_test_runs
		where id = ?`, runID).Scan(
		&record.ID, &record.PlanName, &record.Status, &record.TotalSteps, &record.PassedSteps, &record.FailedSteps, &record.Skipped, &record.StartedAt, &record.FinishedAt, &record.ErrorText,
	)
	return record, err
}

func loadTestRunSteps(db *sql.DB, runID int64) ([]testRunStepRecord, error) {
	rows, err := db.Query(`select run_id, step_index, mod_name, mod_version, serial_group, visible_tmux, requires_nix,
		status, exit_code, queue_id, command_text, output_text, error_text, started_at, finished_at, runtime_ms
		from mod_test_run_steps
		where run_id = ?
		order by step_index`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []testRunStepRecord{}
	for rows.Next() {
		var record testRunStepRecord
		var visibleTmux int
		var requiresNix int
		if err := rows.Scan(
			&record.RunID, &record.StepIndex, &record.ModName, &record.ModVersion, &record.SerialGroup, &visibleTmux, &requiresNix,
			&record.Status, &record.ExitCode, &record.QueueID, &record.CommandText, &record.OutputText, &record.ErrorText, &record.StartedAt, &record.FinishedAt, &record.RuntimeMS,
		); err != nil {
			return nil, err
		}
		record.VisibleTmux = visibleTmux == 1
		record.RequiresNix = requiresNix == 1
		out = append(out, record)
	}
	return out, rows.Err()
}

func executeTestPlanStep(db *sql.DB, repoRoot string, runID int64, modByKey map[string]modstate.ModRecord, step modstate.TestStepRecord) (testRunStepRecord, error) {
	modKey := step.ModName + ":" + step.ModVersion
	mod, ok := modByKey[modKey]
	if !ok {
		now := nowRFC3339Local()
		return testRunStepRecord{
			RunID:       runID,
			StepIndex:   step.StepIndex,
			ModName:     step.ModName,
			ModVersion:  step.ModVersion,
			SerialGroup: step.SerialGroup,
			VisibleTmux: step.VisibleTmux,
			RequiresNix: step.RequiresNix,
			Status:      "skipped",
			ExitCode:    0,
			CommandText: "",
			OutputText:  "",
			ErrorText:   "mod missing from sqlite registry",
			StartedAt:   now,
			FinishedAt:  now,
			RuntimeMS:   0,
		}, nil
	}
	commandText, ok := resolveGoTestCommand(mod)
	start := time.Now().UTC()
	record := testRunStepRecord{
		RunID:       runID,
		StepIndex:   step.StepIndex,
		ModName:     step.ModName,
		ModVersion:  step.ModVersion,
		SerialGroup: step.SerialGroup,
		VisibleTmux: step.VisibleTmux,
		RequiresNix: step.RequiresNix,
		CommandText: commandText,
		StartedAt:   start.Format(time.RFC3339),
	}
	if !ok {
		record.Status = "skipped"
		record.FinishedAt = record.StartedAt
		record.ErrorText = "no Go package recorded for this mod version"
		return record, nil
	}
	queueID, err := modstate.EnqueueCommand(db, "tests", "go-test", modKey, commandText, "")
	if err == nil {
		record.QueueID = queueID
		_ = modstate.MarkCommandStarted(db, queueID)
	}
	launchConfig, launchErr := modstate.LoadLaunchConfig(db, step.ModName, step.ModVersion)
	flakeShell := "default"
	if launchErr == nil && strings.TrimSpace(launchConfig.FlakeShell) != "" {
		flakeShell = strings.TrimSpace(launchConfig.FlakeShell)
	}
	output, exitCode, execErr := runTestCommand(repoRoot, commandText, step.RequiresNix, flakeShell)
	record.OutputText = truncateText(output, 24000)
	record.ExitCode = exitCode
	record.RuntimeMS = time.Since(start).Milliseconds()
	record.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	if execErr != nil {
		record.Status = "failed"
		record.ErrorText = execErr.Error()
		if record.QueueID != 0 {
			_ = modstate.MarkCommandFinished(db, record.QueueID, "failed", record.OutputText, record.ErrorText)
		}
		return record, execErr
	}
	record.Status = "passed"
	if record.QueueID != 0 {
		_ = modstate.MarkCommandFinished(db, record.QueueID, "done", record.OutputText, "")
	}
	return record, nil
}

func runTestCommand(repoRoot, commandText string, requiresNix bool, flakeShell string) (string, int, error) {
	script := fmt.Sprintf("cd %s && %s", shellQuote(filepath.Join(repoRoot, "src")), commandText)
	var cmd *exec.Cmd
	if requiresNix {
		shellRef := ".#" + strings.TrimSpace(flakeShell)
		if strings.TrimSpace(flakeShell) == "" {
			shellRef = ".#default"
		}
		cmd = exec.Command("nix", "--extra-experimental-features", "nix-command flakes", "develop", shellRef, "--command", "zsh", "-lc", script)
		cmd.Dir = repoRoot
	} else {
		cmd = exec.Command("zsh", "-lc", script)
		cmd.Dir = repoRoot
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
		return output.String(), exitCode, err
	}
	return output.String(), exitCode, nil
}

func resolveGoTestCommand(mod modstate.ModRecord) (string, bool) {
	base := "./" + strings.TrimPrefix(filepath.ToSlash(mod.Path), "src/")
	if mod.HasMain {
		return "go test " + base, true
	}
	if mod.HasCLI {
		return "go test " + base + "/cli", true
	}
	return "", false
}

func writeTestRunReadmes(db *sql.DB, repoRoot string, runID int64, modByKey map[string]modstate.ModRecord) error {
	run, err := loadTestRun(db, runID)
	if err != nil {
		return err
	}
	steps, err := loadTestRunSteps(db, runID)
	if err != nil {
		return err
	}
	stepsByMod := map[string][]testRunStepRecord{}
	for _, step := range steps {
		key := step.ModName + ":" + step.ModVersion
		stepsByMod[key] = append(stepsByMod[key], step)
	}
	keys := make([]string, 0, len(stepsByMod))
	for key := range stepsByMod {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		mod, ok := modByKey[key]
		if !ok {
			continue
		}
		if err := writeModReadme(repoRoot, mod, run, stepsByMod[key]); err != nil {
			return err
		}
	}
	return nil
}

func writeModReadme(repoRoot string, mod modstate.ModRecord, run testRunRecord, steps []testRunStepRecord) error {
	readmePath := strings.TrimSpace(mod.ReadmePath)
	if readmePath == "" {
		readmePath = filepath.ToSlash(filepath.Join(mod.Path, "README.md"))
	}
	absPath := filepath.Join(repoRoot, filepath.FromSlash(readmePath))
	content, err := os.ReadFile(absPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	body := strings.TrimSpace(string(content))
	if body == "" {
		body = defaultReadmeTitle(mod)
	}
	body = upsertMarkdownSection(body, "Quick Start", renderQuickStart(repoRoot, mod))
	body = upsertMarkdownSection(body, "Test Results", renderTestResults(run, steps))
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(absPath, []byte(body), 0o644)
}

func renderQuickStart(repoRoot string, mod modstate.ModRecord) string {
	commandName := mod.Name
	if mod.Name == "mod" {
		commandName = "mods"
	}
	var lines []string
	lines = append(lines, "## Quick Start", "", "```sh")
	lines = append(lines, fmt.Sprintf("# Show the %s command surface from the sqlite-managed mods system.", mod.Name))
	lines = append(lines, fmt.Sprintf("./dialtone_mod %s %s help", commandName, mod.Version), "")
	if testCommand, ok := resolveGoTestCommand(mod); ok {
		lines = append(lines, "# Run the Go test package for this mod from the repo source tree.")
		lines = append(lines, fmt.Sprintf("cd %s/src", repoRoot))
		lines = append(lines, testCommand, "")
		lines = append(lines, "# Regenerate the sqlite DAG and the stepwise test plan before the next TDD loop.")
		lines = append(lines, "cd ..")
		lines = append(lines, "./dialtone_mod mods v1 db sync")
		lines = append(lines, "./dialtone_mod mods v1 db test-plan")
	} else {
		lines = append(lines, "# This mod has no Go package entrypoint recorded in sqlite yet.")
		lines = append(lines, "# Inspect it through the central registry before adding tests.")
		lines = append(lines, "./dialtone_mod mods v1 db topo")
		lines = append(lines, "./dialtone_mod mods v1 db graph --format text")
	}
	lines = append(lines, "```")
	return strings.Join(lines, "\n")
}

func renderTestResults(run testRunRecord, steps []testRunStepRecord) string {
	errorsText := "none"
	failures := []string{}
	for _, step := range steps {
		if step.Status == "failed" {
			failures = append(failures, fmt.Sprintf("%s %s step %d exit=%d", step.ModName, step.ModVersion, step.StepIndex, step.ExitCode))
		}
	}
	if len(failures) > 0 {
		errorsText = strings.Join(failures, "; ")
	}
	runtimeText := "unknown"
	if start, err := time.Parse(time.RFC3339, run.StartedAt); err == nil {
		if stop, err := time.Parse(time.RFC3339, run.FinishedAt); err == nil {
			runtimeText = stop.Sub(start).String()
		}
	}
	lines := []string{
		"## Test Results",
		"",
		"Most recent validation run:",
		"",
		fmt.Sprintf("- `<run-id>`: %d", run.ID),
		fmt.Sprintf("- `<plan-name>`: %s", run.PlanName),
		fmt.Sprintf("- `<timestamp-start>`: %s", run.StartedAt),
		fmt.Sprintf("- `<timestamp-stop>`: %s", run.FinishedAt),
		fmt.Sprintf("- `<runtime>`: %s", runtimeText),
		fmt.Sprintf("- `<status>`: %s", run.Status),
		fmt.Sprintf("- `<ERRORS>`: %s", errorsText),
		"- `<ui-screenshot-grid>`: not captured",
		"",
		"Most recent command set:",
		"",
		"```sh",
	}
	for _, step := range steps {
		if strings.TrimSpace(step.CommandText) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("# step %d -> %s %s (%s)", step.StepIndex, step.ModName, step.ModVersion, step.Status))
		lines = append(lines, step.CommandText, "")
	}
	lines = append(lines, "```", "", "Observed result summary:", "")
	for _, step := range steps {
		lines = append(lines, fmt.Sprintf("- step %d `%s %s` -> `%s` (exit=%d)", step.StepIndex, step.ModName, step.ModVersion, step.Status, step.ExitCode))
	}
	return strings.Join(lines, "\n")
}

func defaultReadmeTitle(mod modstate.ModRecord) string {
	title := strings.ToUpper(mod.Name[:1]) + mod.Name[1:]
	return fmt.Sprintf("# %s Mod (`%s`)\n\nThis README is synchronized by `./dialtone_mod mods v1 db test-run`.\n", title, mod.Version)
}

func upsertMarkdownSection(content, sectionTitle, replacement string) string {
	if strings.TrimSpace(content) == "" {
		return strings.TrimSpace(replacement)
	}
	heading := "## " + strings.TrimSpace(sectionTitle)
	start := strings.Index(content, heading)
	if start == -1 {
		return strings.TrimRight(content, "\n") + "\n\n" + strings.TrimSpace(replacement)
	}
	searchFrom := start + len(heading)
	nextOffset := strings.Index(content[searchFrom:], "\n## ")
	if nextOffset == -1 {
		return strings.TrimRight(content[:start], "\n") + "\n\n" + strings.TrimSpace(replacement)
	}
	end := searchFrom + nextOffset + 1
	return strings.TrimRight(content[:start], "\n") + "\n\n" + strings.TrimSpace(replacement) + "\n\n" + strings.TrimLeft(content[end:], "\n")
}

func truncateText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func nowRFC3339Local() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func boolToIntLocal(v bool) int {
	if v {
		return 1
	}
	return 0
}
