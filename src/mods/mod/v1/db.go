package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/sqlitestate"
)

func runDB(args []string) error {
	if len(args) == 0 {
		printDBUsage()
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		printDBUsage()
		return nil
	case "path":
		return runDBPath(args[1:])
	case "init":
		return runDBInit(args[1:])
	case "sync":
		return runDBSync(args[1:])
	case "graph":
		return runDBGraph(args[1:])
	case "env":
		return runDBEnv(args[1:])
	case "state":
		return runDBState(args[1:])
	case "queue":
		return runDBQueue(args[1:])
	case "runs":
		return runDBRuns(args[1:])
	case "run":
		return runDBRun(args[1:])
	case "topo":
		return runDBTopo(args[1:])
	case "test-plan":
		return runDBTestPlan(args[1:])
	case "test-run":
		return runDBTestRun(args[1:])
	case "test-runs":
		return runDBTestRuns(args[1:])
	case "test-run-steps":
		return runDBTestRunSteps(args[1:])
	case "protocol-runs":
		return runDBProtocolRuns(args[1:])
	case "protocol-events":
		return runDBProtocolEvents(args[1:])
	default:
		return fmt.Errorf("unknown db subcommand: %s", args[0])
	}
}

func runDBPath(args []string) error {
	fs := flag.NewFlagSet("mods db path", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db path does not accept positional arguments")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	fmt.Println(resolveStateDBPath(repoRoot, *dbPath))
	return nil
}

func runDBInit(args []string) error {
	fs := flag.NewFlagSet("mods db init", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db init does not accept positional arguments")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	path := resolveStateDBPath(repoRoot, *dbPath)
	db, err := modstate.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := modstate.EnsureSchema(db); err != nil {
		return err
	}
	fmt.Printf("initialized sqlite state db: %s\n", path)
	return nil
}

func runDBSync(args []string) error {
	fs := flag.NewFlagSet("mods db sync", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db sync does not accept positional arguments")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	path := resolveStateDBPath(repoRoot, *dbPath)
	db, err := modstate.Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	summary, err := modstate.SyncRepo(db, repoRoot, modstate.CaptureRuntimeEnv())
	if err != nil {
		return err
	}
	fmt.Printf("synced sqlite state db: %s\n", path)
	fmt.Printf("mods=%d manifests=%d dependencies=%d nix_packages=%d env_vars=%d topology=%d test_steps=%d\n",
		summary.Mods, summary.Manifests, summary.Dependencies, summary.NixPackages, summary.EnvVars, summary.Topology, summary.TestSteps)
	return nil
}

func runDBGraph(args []string) error {
	fs := flag.NewFlagSet("mods db graph", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	format := fs.String("format", "text", "Output format: text|mermaid|outline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db graph does not accept positional arguments")
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
	edges, err := modstate.LoadGraph(db)
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "text":
		fmt.Println(modstate.RenderGraphText(edges))
	case "mermaid":
		fmt.Println(modstate.RenderGraphMermaid(edges))
	case "outline":
		topology, err := modstate.LoadTopology(db)
		if err != nil {
			return err
		}
		fmt.Println(modstate.RenderGraphOutline(topology, edges))
	default:
		return fmt.Errorf("unsupported graph format %q", *format)
	}
	return nil
}

func runDBEnv(args []string) error {
	fs := flag.NewFlagSet("mods db env", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	scope := fs.String("scope", "process", "Runtime env scope to print")
	setValue := fs.String("set", "", "Persist a KEY=VALUE runtime env entry into sqlite")
	unsetKey := fs.String("unset", "", "Delete a runtime env key from sqlite")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db env does not accept positional arguments")
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
	if err := modstate.EnsureSchema(db); err != nil {
		return err
	}
	if strings.TrimSpace(*setValue) != "" && strings.TrimSpace(*unsetKey) != "" {
		return fmt.Errorf("db env accepts only one of --set or --unset")
	}
	if strings.TrimSpace(*setValue) != "" {
		key, value, err := sqlitestate.ParseAssignment(*setValue)
		if err != nil {
			return err
		}
		if err := sqlitestate.UpsertRuntimeEnv(db, strings.TrimSpace(*scope), key, value); err != nil {
			return err
		}
		fmt.Printf("set sqlite runtime env %s=%s\n", key, value)
		return nil
	}
	if strings.TrimSpace(*unsetKey) != "" {
		if err := sqlitestate.DeleteRuntimeEnv(db, strings.TrimSpace(*scope), strings.TrimSpace(*unsetKey)); err != nil {
			return err
		}
		fmt.Printf("deleted sqlite runtime env %s\n", strings.TrimSpace(*unsetKey))
		return nil
	}
	rows, err := modstate.LoadRuntimeEnv(db, strings.TrimSpace(*scope))
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%s=%s\n", row.Key, row.Value)
	}
	return nil
}

func runDBState(args []string) error {
	fs := flag.NewFlagSet("mods db state", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	scope := fs.String("scope", sqlitestate.SystemScope, "State scope to inspect")
	key := fs.String("key", "", "Specific state key to print")
	setValue := fs.String("set", "", "Persist a KEY=VALUE state entry into sqlite")
	unsetKey := fs.String("unset", "", "Delete a state key from sqlite")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db state does not accept positional arguments")
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
	if strings.TrimSpace(*setValue) != "" && strings.TrimSpace(*unsetKey) != "" {
		return fmt.Errorf("db state accepts only one of --set or --unset")
	}
	if strings.TrimSpace(*setValue) != "" {
		stateKey, value, err := sqlitestate.ParseAssignment(*setValue)
		if err != nil {
			return err
		}
		if err := modstate.UpsertStateValue(db, strings.TrimSpace(*scope), stateKey, value); err != nil {
			return err
		}
		fmt.Printf("set sqlite state %s=%s\n", stateKey, value)
		return nil
	}
	if strings.TrimSpace(*unsetKey) != "" {
		if err := modstate.DeleteStateValue(db, strings.TrimSpace(*scope), strings.TrimSpace(*unsetKey)); err != nil {
			return err
		}
		fmt.Printf("deleted sqlite state %s\n", strings.TrimSpace(*unsetKey))
		return nil
	}
	if strings.TrimSpace(*key) != "" {
		record, ok, err := modstate.LoadStateValue(db, strings.TrimSpace(*scope), strings.TrimSpace(*key))
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("state key not found: %s", strings.TrimSpace(*key))
		}
		fmt.Printf("%s=%s\n", record.Key, record.Value)
		return nil
	}
	rows, err := modstate.LoadStateValues(db, strings.TrimSpace(*scope))
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%s=%s\n", row.Key, row.Value)
	}
	return nil
}

func runDBQueue(args []string) error {
	fs := flag.NewFlagSet("mods db queue", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	name := fs.String("name", "tmux", "Queue name to inspect")
	limit := fs.Int("limit", 20, "Maximum queue rows to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db queue does not accept positional arguments")
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
	rows, err := modstate.LoadQueue(db, strings.TrimSpace(*name), *limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.ID, row.Status, row.Kind, row.Target, row.CommandText, row.CreatedAt, row.FinishedAt)
	}
	return nil
}

func runDBRuns(args []string) error {
	fs := flag.NewFlagSet("mods db runs", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	limit := fs.Int("limit", 20, "Maximum command runs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db runs does not accept positional arguments")
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
	rows, err := modstate.LoadCommandRuns(db, *limit)
	if err != nil {
		return err
	}
	fmt.Println("id\tmod_name\tmod_version\tverb\ttransport\tstatus\tshell_bus_id\tpid\texit_code\truntime_ms\tflake_shell\ttarget\tlog_path\tcreated_at\tfinished_at\tcommand")
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.ID, row.ModName, row.ModVersion, row.Verb, row.Transport, row.Status, row.ShellBusID, row.PID,
			row.ExitCode, row.RuntimeMS, row.FlakeShell, row.Target, row.LogPath, row.CreatedAt, row.FinishedAt, row.CommandText)
	}
	return nil
}

func runDBRun(args []string) error {
	fs := flag.NewFlagSet("mods db run", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	runID := fs.Int64("id", 0, "Command run id to inspect")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db run does not accept positional arguments")
	}
	if *runID <= 0 {
		return fmt.Errorf("db run requires --id <id>")
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
	row, ok, err := modstate.LoadCommandRun(db, *runID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("command run %d not found", *runID)
	}
	fmt.Printf("run_id\t%d\n", row.ID)
	fmt.Printf("mod_name\t%s\n", row.ModName)
	fmt.Printf("mod_version\t%s\n", row.ModVersion)
	fmt.Printf("verb\t%s\n", row.Verb)
	fmt.Printf("transport\t%s\n", row.Transport)
	fmt.Printf("status\t%s\n", row.Status)
	fmt.Printf("shell_bus_id\t%d\n", row.ShellBusID)
	fmt.Printf("target\t%s\n", row.Target)
	fmt.Printf("flake_shell\t%s\n", row.FlakeShell)
	fmt.Printf("pid\t%d\n", row.PID)
	fmt.Printf("exit_code\t%d\n", row.ExitCode)
	fmt.Printf("runtime_ms\t%d\n", row.RuntimeMS)
	fmt.Printf("log_path\t%s\n", row.LogPath)
	fmt.Printf("created_at\t%s\n", row.CreatedAt)
	fmt.Printf("started_at\t%s\n", row.StartedAt)
	fmt.Printf("heartbeat_at\t%s\n", row.HeartbeatAt)
	fmt.Printf("finished_at\t%s\n", row.FinishedAt)
	fmt.Printf("command\t%s\n", row.CommandText)
	fmt.Printf("args_json\t%s\n", row.ArgsJSON)
	fmt.Printf("package_refs_json\t%s\n", row.PackageRefsJSON)
	if strings.TrimSpace(row.ResultText) != "" {
		fmt.Printf("result_text\t%s\n", row.ResultText)
	}
	if strings.TrimSpace(row.ErrorText) != "" {
		fmt.Printf("error_text\t%s\n", row.ErrorText)
	}
	return nil
}

func runDBTopo(args []string) error {
	fs := flag.NewFlagSet("mods db topo", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db topo does not accept positional arguments")
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
	rows, err := modstate.LoadTopology(db)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\n", row.TopoRank, row.ModName, row.ModVersion)
	}
	return nil
}

func runDBTestPlan(args []string) error {
	fs := flag.NewFlagSet("mods db test-plan", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	planName := fs.String("name", "default", "Test plan name to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db test-plan does not accept positional arguments")
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
	rows, err := modstate.LoadTestPlan(db, strings.TrimSpace(*planName))
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%s\t%t\t%t\t%s\n",
			row.StepIndex, row.ModName, row.ModVersion, row.SerialGroup, row.RequiresNix, row.VisibleTmux, row.CommandText)
	}
	return nil
}

func runDBProtocolRuns(args []string) error {
	fs := flag.NewFlagSet("mods db protocol-runs", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	limit := fs.Int("limit", 20, "Maximum protocol runs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db protocol-runs does not accept positional arguments")
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
	rows, err := modstate.LoadProtocolRuns(db, *limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.ID, row.Name, row.Status, row.PromptTarget, row.CommandTarget, row.StartedAt, row.FinishedAt, row.ResultText)
	}
	return nil
}

func runDBProtocolEvents(args []string) error {
	fs := flag.NewFlagSet("mods db protocol-events", flag.ContinueOnError)
	dbPath := fs.String("db", "", "SQLite database path (default: DIALTONE_STATE_DB or ~/.dialtone/state.sqlite)")
	runID := fs.Int64("run", 0, "Protocol run id to inspect")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("db protocol-events does not accept positional arguments")
	}
	if *runID <= 0 {
		return fmt.Errorf("db protocol-events requires --run <id>")
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
	rows, err := modstate.LoadProtocolEvents(db, *runID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		fmt.Printf("%d\t%d\t%s\t%s\t%d\t%s\t%s\t%s\n",
			row.RunID, row.EventIndex, row.EventType, row.QueueName, row.QueueRowID, row.PaneTarget, row.CommandText, row.MessageText)
	}
	return nil
}

func resolveStateDBPath(repoRoot, raw string) string {
	if strings.TrimSpace(raw) != "" {
		return filepath.Clean(strings.TrimSpace(raw))
	}
	return sqlitestate.ResolveStateDBPath(repoRoot)
}

func printDBUsage() {
	fmt.Println("Usage: ./dialtone_mod mods v1 db <subcommand> [args]")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  path [--db PATH]")
	fmt.Println("       Print the resolved sqlite state database path")
	fmt.Println("  init [--db PATH]")
	fmt.Println("       Create the sqlite state database schema if it does not exist")
	fmt.Println("  sync [--db PATH]")
	fmt.Println("       Sync mods, DAG manifests, nix packages, and current DIALTONE_* env vars into sqlite")
	fmt.Println("  graph [--db PATH] [--format text|mermaid|outline]")
	fmt.Println("       Print the mod dependency graph from sqlite")
	fmt.Println("  env [--db PATH] [--scope process]")
	fmt.Println("       Print the captured runtime environment from sqlite")
	fmt.Println("  env [--db PATH] [--scope process] --set KEY=VALUE")
	fmt.Println("       Persist a runtime env entry into sqlite")
	fmt.Println("  env [--db PATH] [--scope process] --unset KEY")
	fmt.Println("       Delete a runtime env entry from sqlite")
	fmt.Println("  state [--db PATH] [--scope system] [--key KEY]")
	fmt.Println("       Print sqlite-backed system state entries")
	fmt.Println("  state [--db PATH] [--scope system] --set KEY=VALUE")
	fmt.Println("       Persist a sqlite-backed system state entry")
	fmt.Println("  state [--db PATH] [--scope system] --unset KEY")
	fmt.Println("       Delete a sqlite-backed system state entry")
	fmt.Println("  queue [--db PATH] [--name tmux] [--limit 20]")
	fmt.Println("       Print queue rows with timestamps from sqlite")
	fmt.Println("  runs [--db PATH] [--limit 20]")
	fmt.Println("       Print canonical SQLite command runs for routed mod execution")
	fmt.Println("  run [--db PATH] --id ID")
	fmt.Println("       Print one canonical SQLite command run in detail")
	fmt.Println("  topo [--db PATH]")
	fmt.Println("       Print the validated topological order for the mod DAG")
	fmt.Println("  test-plan [--db PATH] [--name default]")
	fmt.Println("       Print the sequential Go test plan derived from the SQLite DAG")
	fmt.Println("  test-run [--db PATH] [--name default] [--sync=true] [--update-readmes=true]")
	fmt.Println("       Execute the SQLite test plan step by step, record results, and update mod READMEs")
	fmt.Println("  test-runs [--db PATH] [--limit 20]")
	fmt.Println("       Print recorded SQLite test runs")
	fmt.Println("  test-run-steps [--db PATH] --run ID")
	fmt.Println("       Print recorded SQLite test steps for a specific run")
	fmt.Println("  protocol-runs [--db PATH] [--limit 20]")
	fmt.Println("       Print recorded protocol runs that tie codex-view, dialtone-view, and SQLite together")
	fmt.Println("  protocol-events [--db PATH] --run ID")
	fmt.Println("       Print the ordered SQLite protocol events for a specific protocol run")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ./dialtone_mod mods v1 db sync")
	fmt.Println("  ./dialtone_mod mods v1 db runs --limit 10")
	fmt.Println("  ./dialtone_mod mods v1 db run --id 42")
	fmt.Println("  ./dialtone_mod mods v1 db graph --format outline")
	fmt.Println("  ./dialtone_mod mods v1 db queue --limit 20")
	fmt.Println("  ./dialtone_mod mods v1 db protocol-runs --limit 10")
	fmt.Println("  ./dialtone_mod mods v1 db test-run --name default")
}
