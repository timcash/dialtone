package modstate

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const schemaVersion = "2"

type ModRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ModEnvVar struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}

type TestPolicy struct {
	RequiresNix bool   `json:"requires_nix"`
	SerialGroup string `json:"serial_group"`
	VisibleTmux bool   `json:"visible_tmux"`
}

type NixPolicy struct {
	FlakeShell string `json:"flake_shell"`
}

type Manifest struct {
	Name        string      `json:"name"`
	Version     string      `json:"version"`
	DependsOn   []ModRef    `json:"depends_on"`
	Environment []ModEnvVar `json:"env_vars"`
	Testing     TestPolicy  `json:"testing"`
	Nix         NixPolicy   `json:"nix"`
}

type ModRecord struct {
	Name         string
	Version      string
	Path         string
	ReadmePath   string
	ManifestPath string
	HasMain      bool
	HasCLI       bool
}

type DependencyRecord struct {
	FromName    string
	FromVersion string
	ToName      string
	ToVersion   string
	Source      string
}

type NixPackageRecord struct {
	ModName    string
	ModVersion string
	Selector   string
	PackageRef string
}

type LaunchConfigRecord struct {
	ModName    string
	ModVersion string
	FlakeShell string
}

type EnvRecord struct {
	Scope string
	Key   string
	Value string
}

type GraphNode struct {
	Name    string
	Version string
}

type GraphEdge struct {
	From   GraphNode
	To     GraphNode
	Source string
}

type StateRecord struct {
	Scope string
	Key   string
	Value string
}

type QueueRecord struct {
	ID           int64
	CommandRunID int64
	QueueName    string
	Status       string
	Kind         string
	Target       string
	CommandText  string
	PayloadJSON  string
	ResultText   string
	ErrorText    string
	CreatedAt    string
	StartedAt    string
	FinishedAt   string
}

type TopologyRecord struct {
	ModName    string
	ModVersion string
	TopoRank   int
}

type TestStepRecord struct {
	PlanName    string
	StepIndex   int
	ModName     string
	ModVersion  string
	TopoRank    int
	SerialGroup string
	RequiresNix bool
	VisibleTmux bool
	CommandText string
}

type ProtocolRunRecord struct {
	ID            int64
	Name          string
	Status        string
	PromptText    string
	PromptTarget  string
	CommandTarget string
	ResultText    string
	ErrorText     string
	StartedAt     string
	FinishedAt    string
}

type ProtocolEventRecord struct {
	RunID       int64
	EventIndex  int
	EventType   string
	QueueName   string
	QueueRowID  int64
	PaneTarget  string
	CommandText string
	MessageText string
	CreatedAt   string
}

type ModTestRunRecord struct {
	ID           int64
	PlanName     string
	Status       string
	TotalSteps   int
	PassedSteps  int
	FailedSteps  int
	SkippedSteps int
	StopOnError  bool
	ErrorText    string
	StartedAt    string
	FinishedAt   string
}

type ModTestRunStepRecord struct {
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

type ShellBusRecord struct {
	ID           int64
	System       string
	Scope        string
	Subject      string
	Action       string
	Status       string
	Actor        string
	Session      string
	Pane         string
	RefID        int64
	CommandRunID int64
	BodyJSON     string
	CreatedAt    string
	UpdatedAt    string
}

type CommandRunRecord struct {
	ID              int64
	ModName         string
	ModVersion      string
	Verb            string
	CommandText     string
	ArgsJSON        string
	Transport       string
	Status          string
	Target          string
	FlakeShell      string
	PackageRefsJSON string
	ShellBusID      int64
	PID             int
	ExitCode        int
	RuntimeMS       int64
	LogPath         string
	ResultText      string
	ErrorText       string
	CreatedAt       string
	StartedAt       string
	HeartbeatAt     string
	FinishedAt      string
}

type SyncSummary struct {
	Mods         int
	Dependencies int
	NixPackages  int
	EnvVars      int
	Manifests    int
	Topology     int
	TestSteps    int
}

type Entrypoint struct {
	Name    string
	Version string
	Path    string
	HasMain bool
	HasCLI  bool
}

var volatileRuntimeEnvKeys = map[string]struct{}{
	"DIALTONE_TMUX_PROXY_ACTIVE": {},
	"DIALTONE_TMUX_TARGET":       {},
	"DIALTONE_TMUX_TARGET_FILE":  {},
	"DIALTONE_NIX_ACTIVE":        {},
	"DIALTONE_NIX_BASE_PATH":     {},
	"DIALTONE_NIX_PACKAGES_SIG":  {},
	"DIALTONE_NIX_SHELL":         {},
}

func DefaultStateDir(repoRoot string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(strings.TrimSpace(home), ".dialtone")
	}
	return filepath.Join(os.TempDir(), "dialtone")
}

func DefaultDBPath(repoRoot string) string {
	return filepath.Join(DefaultStateDir(repoRoot), "state.sqlite")
}

func Open(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
	}
	for _, stmt := range pragmas {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return db, nil
}

func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`create table if not exists meta (
			key text primary key,
			value text not null,
			updated_at text not null
		);`,
		`create table if not exists mods (
			name text not null,
			version text not null,
			path text not null,
			readme_path text not null default '',
			manifest_path text not null default '',
			has_main integer not null default 0,
			has_cli integer not null default 0,
			updated_at text not null,
			primary key (name, version)
		);`,
		`create table if not exists mod_dependencies (
			from_name text not null,
			from_version text not null,
			to_name text not null,
			to_version text not null,
			source text not null,
			updated_at text not null,
			primary key (from_name, from_version, to_name, to_version)
		);`,
		`create table if not exists mod_nix_packages (
			mod_name text not null,
			mod_version text not null,
			selector text not null,
			package_ref text not null,
			updated_at text not null,
			primary key (mod_name, mod_version, selector, package_ref)
		);`,
		`create table if not exists mod_env_vars (
			mod_name text not null,
			mod_version text not null,
			key text not null,
			required integer not null default 1,
			updated_at text not null,
			primary key (mod_name, mod_version, key)
		);`,
		`create table if not exists mod_test_policies (
			mod_name text not null,
			mod_version text not null,
			requires_nix integer not null default 0,
			serial_group text not null default '',
			visible_tmux integer not null default 0,
			updated_at text not null,
			primary key (mod_name, mod_version)
		);`,
		`create table if not exists mod_launch_configs (
			mod_name text not null,
			mod_version text not null,
			flake_shell text not null default '',
			updated_at text not null,
			primary key (mod_name, mod_version)
		);`,
		`create table if not exists runtime_env (
			scope text not null,
			key text not null,
			value text not null,
			updated_at text not null,
			primary key (scope, key)
		);`,
		`create table if not exists state_values (
			scope text not null,
			key text not null,
			value text not null,
			updated_at text not null,
			primary key (scope, key)
		);`,
		`create table if not exists command_queue (
			id integer primary key autoincrement,
			command_run_id integer not null default 0,
			queue_name text not null,
			status text not null,
			kind text not null,
			target text not null default '',
			command_text text not null default '',
			payload_json text not null default '',
			result_text text not null default '',
			error_text text not null default '',
			created_at text not null,
			started_at text not null default '',
			finished_at text not null default ''
		);`,
		`create table if not exists command_runs (
			id integer primary key autoincrement,
			mod_name text not null,
			mod_version text not null,
			verb text not null default '',
			command_text text not null,
			args_json text not null default '[]',
			transport text not null default '',
			status text not null,
			target text not null default '',
			flake_shell text not null default '',
			package_refs_json text not null default '[]',
			shell_bus_id integer not null default 0,
			pid integer not null default 0,
			exit_code integer not null default 0,
			runtime_ms integer not null default 0,
			log_path text not null default '',
			result_text text not null default '',
			error_text text not null default '',
			created_at text not null,
			started_at text not null default '',
			heartbeat_at text not null default '',
			finished_at text not null default ''
		);`,
		`create index if not exists idx_command_runs_status on command_runs(status, id desc);`,
		`create index if not exists idx_command_runs_mod on command_runs(mod_name, mod_version, id desc);`,
		`create table if not exists mod_topology (
			mod_name text not null,
			mod_version text not null,
			topo_rank integer not null,
			updated_at text not null,
			primary key (mod_name, mod_version)
		);`,
		`create table if not exists mod_test_steps (
			plan_name text not null,
			step_index integer not null,
			mod_name text not null,
			mod_version text not null,
			topo_rank integer not null,
			serial_group text not null default '',
			requires_nix integer not null default 0,
			visible_tmux integer not null default 0,
			command_text text not null,
			updated_at text not null,
			primary key (plan_name, step_index)
		);`,
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
		`create table if not exists protocol_runs (
			id integer primary key autoincrement,
			name text not null,
			status text not null,
			prompt_text text not null default '',
			prompt_target text not null default '',
			command_target text not null default '',
			result_text text not null default '',
			error_text text not null default '',
			started_at text not null,
			finished_at text not null default ''
		);`,
		`create table if not exists protocol_events (
			run_id integer not null,
			event_index integer not null,
			event_type text not null,
			queue_name text not null default '',
			queue_row_id integer not null default 0,
			pane_target text not null default '',
			command_text text not null default '',
			message_text text not null default '',
			created_at text not null,
			primary key (run_id, event_index)
		);`,
		`create table if not exists shell_bus (
			id integer primary key autoincrement,
			system text not null,
			scope text not null,
			subject text not null,
			action text not null,
			status text not null,
			actor text not null,
			session text not null default '',
			pane text not null default '',
			ref_id integer not null default 0,
			command_run_id integer not null default 0,
			body_json text not null default '',
			created_at text not null,
			updated_at text not null
		);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	if err := ensureTableColumn(db, "shell_bus", "command_run_id", "integer not null default 0"); err != nil {
		return err
	}
	if err := ensureTableColumn(db, "command_queue", "command_run_id", "integer not null default 0"); err != nil {
		return err
	}
	if _, err := db.Exec(`create index if not exists idx_shell_bus_command_run_id on shell_bus(command_run_id, id desc)`); err != nil {
		return err
	}
	if _, err := db.Exec(`create index if not exists idx_command_queue_command_run_id on command_queue(command_run_id, id desc)`); err != nil {
		return err
	}
	_, err := db.Exec(`insert into meta(key, value, updated_at) values('schema_version', ?, ?)
		on conflict(key) do update set value=excluded.value, updated_at=excluded.updated_at`, schemaVersion, nowRFC3339())
	return err
}

func ensureTableColumn(db *sql.DB, tableName, columnName, columnDef string) error {
	rows, err := db.Query(fmt.Sprintf("pragma table_info(%s)", quoteSQLiteIdent(tableName)))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(columnName)) {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("alter table %s add column %s %s", quoteSQLiteIdent(tableName), quoteSQLiteIdent(columnName), strings.TrimSpace(columnDef)))
	return err
}

func quoteSQLiteIdent(value string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(value), `"`, `""`) + `"`
}

func SyncRepo(db *sql.DB, repoRoot string, env map[string]string) (SyncSummary, error) {
	if err := EnsureSchema(db); err != nil {
		return SyncSummary{}, err
	}
	mods, manifests, deps, nixPkgs, err := ScanRepo(repoRoot)
	if err != nil {
		return SyncSummary{}, err
	}
	topology, err := BuildTopology(mods, deps)
	if err != nil {
		return SyncSummary{}, err
	}
	testSteps := BuildTestPlan(mods, manifests, topology)
	tx, err := db.Begin()
	if err != nil {
		return SyncSummary{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, stmt := range []string{
		"delete from mods",
		"delete from mod_dependencies",
		"delete from mod_nix_packages",
		"delete from mod_env_vars",
		"delete from mod_test_policies",
		"delete from mod_launch_configs",
		"delete from mod_topology",
		"delete from mod_test_steps",
	} {
		if _, err = tx.Exec(stmt); err != nil {
			return SyncSummary{}, err
		}
	}
	if _, err = tx.Exec("delete from runtime_env where scope = ?", "process"); err != nil {
		return SyncSummary{}, err
	}

	timestamp := nowRFC3339()
	for _, mod := range mods {
		if _, err = tx.Exec(`insert into mods(name, version, path, readme_path, manifest_path, has_main, has_cli, updated_at)
			values(?, ?, ?, ?, ?, ?, ?, ?)`,
			mod.Name, mod.Version, mod.Path, mod.ReadmePath, mod.ManifestPath, boolToInt(mod.HasMain), boolToInt(mod.HasCLI), timestamp); err != nil {
			return SyncSummary{}, err
		}
	}
	for _, dep := range deps {
		if _, err = tx.Exec(`insert into mod_dependencies(from_name, from_version, to_name, to_version, source, updated_at)
			values(?, ?, ?, ?, ?, ?)`,
			dep.FromName, dep.FromVersion, dep.ToName, dep.ToVersion, dep.Source, timestamp); err != nil {
			return SyncSummary{}, err
		}
	}
	for _, pkg := range nixPkgs {
		if _, err = tx.Exec(`insert into mod_nix_packages(mod_name, mod_version, selector, package_ref, updated_at)
			values(?, ?, ?, ?, ?)`,
			pkg.ModName, pkg.ModVersion, pkg.Selector, pkg.PackageRef, timestamp); err != nil {
			return SyncSummary{}, err
		}
	}
	for key, manifest := range manifests {
		name, version, found := strings.Cut(key, ":")
		if !found {
			continue
		}
		for _, envVar := range manifest.Environment {
			if strings.TrimSpace(envVar.Name) == "" {
				continue
			}
			if _, err = tx.Exec(`insert into mod_env_vars(mod_name, mod_version, key, required, updated_at)
				values(?, ?, ?, ?, ?)`,
				name, version, strings.TrimSpace(envVar.Name), boolToInt(envVar.Required), timestamp); err != nil {
				return SyncSummary{}, err
			}
		}
		if _, err = tx.Exec(`insert into mod_test_policies(mod_name, mod_version, requires_nix, serial_group, visible_tmux, updated_at)
			values(?, ?, ?, ?, ?, ?)`,
			name, version, boolToInt(manifest.Testing.RequiresNix), strings.TrimSpace(manifest.Testing.SerialGroup), boolToInt(manifest.Testing.VisibleTmux), timestamp); err != nil {
			return SyncSummary{}, err
		}
		if _, err = tx.Exec(`insert into mod_launch_configs(mod_name, mod_version, flake_shell, updated_at)
			values(?, ?, ?, ?)`,
			name, version, strings.TrimSpace(manifest.Nix.FlakeShell), timestamp); err != nil {
			return SyncSummary{}, err
		}
	}
	for _, item := range topology {
		if _, err = tx.Exec(`insert into mod_topology(mod_name, mod_version, topo_rank, updated_at)
			values(?, ?, ?, ?)`,
			item.ModName, item.ModVersion, item.TopoRank, timestamp); err != nil {
			return SyncSummary{}, err
		}
	}
	for _, step := range testSteps {
		if _, err = tx.Exec(`insert into mod_test_steps(plan_name, step_index, mod_name, mod_version, topo_rank, serial_group, requires_nix, visible_tmux, command_text, updated_at)
			values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			step.PlanName, step.StepIndex, step.ModName, step.ModVersion, step.TopoRank, step.SerialGroup, boolToInt(step.RequiresNix), boolToInt(step.VisibleTmux), step.CommandText, timestamp); err != nil {
			return SyncSummary{}, err
		}
	}

	keys := sortedRuntimeEnvKeys(env)
	for _, key := range keys {
		if _, err = tx.Exec(`insert into runtime_env(scope, key, value, updated_at) values(?, ?, ?, ?)`,
			"process", key, env[key], timestamp); err != nil {
			return SyncSummary{}, err
		}
	}
	for _, item := range []struct {
		Key   string
		Value string
	}{
		{Key: "repo_root", Value: filepath.Clean(repoRoot)},
		{Key: "state_db", Value: DefaultDBPath(repoRoot)},
		{Key: "last_sync_at", Value: timestamp},
	} {
		if _, err = tx.Exec(`insert into meta(key, value, updated_at) values(?, ?, ?)
			on conflict(key) do update set value=excluded.value, updated_at=excluded.updated_at`,
			item.Key, item.Value, timestamp); err != nil {
			return SyncSummary{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return SyncSummary{}, err
	}
	return SyncSummary{
		Mods:         len(mods),
		Dependencies: len(deps),
		NixPackages:  len(nixPkgs),
		EnvVars:      len(keys),
		Manifests:    len(manifests),
		Topology:     len(topology),
		TestSteps:    len(testSteps),
	}, nil
}

func ScanRepo(repoRoot string) ([]ModRecord, map[string]Manifest, []DependencyRecord, []NixPackageRecord, error) {
	modRoot := filepath.Join(repoRoot, "src", "mods")
	entries, err := os.ReadDir(modRoot)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	mods := []ModRecord{}
	manifests := map[string]Manifest{}
	deps := []DependencyRecord{}
	nixPkgs := []NixPackageRecord{}
	for _, modEntry := range entries {
		if !modEntry.IsDir() {
			continue
		}
		modName := strings.TrimSpace(modEntry.Name())
		versionEntries, err := os.ReadDir(filepath.Join(modRoot, modName))
		if err != nil {
			return nil, nil, nil, nil, err
		}
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() || !strings.HasPrefix(versionEntry.Name(), "v") {
				continue
			}
			version := strings.TrimSpace(versionEntry.Name())
			versionDir := filepath.Join(modRoot, modName, version)
			relVersionDir := filepath.ToSlash(relative(repoRoot, versionDir))
			record := ModRecord{
				Name:         modName,
				Version:      version,
				Path:         relVersionDir,
				ReadmePath:   relOrEmpty(repoRoot, filepath.Join(versionDir, "README.md")),
				ManifestPath: relOrEmpty(repoRoot, filepath.Join(versionDir, "mod.json")),
				HasMain:      hasGoPackage(versionDir),
				HasCLI:       hasGoPackage(filepath.Join(versionDir, "cli")),
			}
			mods = append(mods, record)

			pkgs, err := readNixPackages(versionDir, modName, version)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			nixPkgs = append(nixPkgs, pkgs...)

			manifest, ok, err := readManifest(versionDir)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if !ok {
				continue
			}
			if manifest.Name == "" {
				manifest.Name = modName
			}
			if manifest.Version == "" {
				manifest.Version = version
			}
			key := manifest.Name + ":" + manifest.Version
			manifests[key] = manifest
			for _, dep := range manifest.DependsOn {
				deps = append(deps, DependencyRecord{
					FromName:    manifest.Name,
					FromVersion: manifest.Version,
					ToName:      strings.TrimSpace(dep.Name),
					ToVersion:   strings.TrimSpace(dep.Version),
					Source:      "mod.json",
				})
			}
		}
	}
	sort.Slice(mods, func(i, j int) bool {
		if mods[i].Name == mods[j].Name {
			return mods[i].Version < mods[j].Version
		}
		return mods[i].Name < mods[j].Name
	})
	sort.Slice(deps, func(i, j int) bool {
		left := deps[i].FromName + ":" + deps[i].FromVersion + "->" + deps[i].ToName + ":" + deps[i].ToVersion
		right := deps[j].FromName + ":" + deps[j].FromVersion + "->" + deps[j].ToName + ":" + deps[j].ToVersion
		return left < right
	})
	sort.Slice(nixPkgs, func(i, j int) bool {
		left := nixPkgs[i].ModName + ":" + nixPkgs[i].ModVersion + ":" + nixPkgs[i].Selector + ":" + nixPkgs[i].PackageRef
		right := nixPkgs[j].ModName + ":" + nixPkgs[j].ModVersion + ":" + nixPkgs[j].Selector + ":" + nixPkgs[j].PackageRef
		return left < right
	})
	return mods, manifests, deps, nixPkgs, nil
}

func LoadGraph(db *sql.DB) ([]GraphEdge, error) {
	rows, err := db.Query(`select from_name, from_version, to_name, to_version, source
		from mod_dependencies
		order by from_name, from_version, to_name, to_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var edges []GraphEdge
	for rows.Next() {
		var edge GraphEdge
		if err := rows.Scan(&edge.From.Name, &edge.From.Version, &edge.To.Name, &edge.To.Version, &edge.Source); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

func LoadRuntimeEnv(db *sql.DB, scope string) ([]EnvRecord, error) {
	rows, err := db.Query(`select scope, key, value from runtime_env where scope = ? order by key`, strings.TrimSpace(scope))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EnvRecord
	for rows.Next() {
		var item EnvRecord
		if err := rows.Scan(&item.Scope, &item.Key, &item.Value); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func UpsertStateValue(db *sql.DB, scope, key, value string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`insert into state_values(scope, key, value, updated_at) values(?, ?, ?, ?)
		on conflict(scope, key) do update set value=excluded.value, updated_at=excluded.updated_at`,
		strings.TrimSpace(scope), strings.TrimSpace(key), value, nowRFC3339())
	return err
}

func DeleteStateValue(db *sql.DB, scope, key string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`delete from state_values where scope = ? and key = ?`, strings.TrimSpace(scope), strings.TrimSpace(key))
	return err
}

func LoadStateValue(db *sql.DB, scope, key string) (StateRecord, bool, error) {
	var record StateRecord
	err := db.QueryRow(`select scope, key, value
		from state_values
		where scope = ? and key = ?`,
		strings.TrimSpace(scope), strings.TrimSpace(key),
	).Scan(&record.Scope, &record.Key, &record.Value)
	if err != nil {
		if err == sql.ErrNoRows {
			return StateRecord{}, false, nil
		}
		return StateRecord{}, false, err
	}
	return record, true, nil
}

func LoadStateValues(db *sql.DB, scope string) ([]StateRecord, error) {
	rows, err := db.Query(`select scope, key, value
		from state_values
		where scope = ?
		order by key`, strings.TrimSpace(scope))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StateRecord{}
	for rows.Next() {
		var record StateRecord
		if err := rows.Scan(&record.Scope, &record.Key, &record.Value); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func EnqueueCommand(db *sql.DB, queueName, kind, target, commandText, payloadJSON string) (int64, error) {
	if err := EnsureSchema(db); err != nil {
		return 0, err
	}
	result, err := db.Exec(`insert into command_queue(command_run_id, queue_name, status, kind, target, command_text, payload_json, created_at)
		values(0, ?, 'queued', ?, ?, ?, ?, ?)`,
		strings.TrimSpace(queueName),
		strings.TrimSpace(kind),
		strings.TrimSpace(target),
		commandText,
		payloadJSON,
		nowRFC3339(),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func StartCommandRun(db *sql.DB, record CommandRunRecord) (int64, error) {
	if err := EnsureSchema(db); err != nil {
		return 0, err
	}
	status := strings.TrimSpace(record.Status)
	if status == "" {
		status = "queued"
	}
	argsJSON := strings.TrimSpace(record.ArgsJSON)
	if argsJSON == "" {
		argsJSON = "[]"
	}
	packageRefsJSON := strings.TrimSpace(record.PackageRefsJSON)
	if packageRefsJSON == "" {
		packageRefsJSON = "[]"
	}
	result, err := db.Exec(`insert into command_runs(
			mod_name, mod_version, verb, command_text, args_json, transport, status, target,
			flake_shell, package_refs_json, shell_bus_id, pid, exit_code, runtime_ms, log_path,
			result_text, error_text, created_at, started_at, heartbeat_at, finished_at
		) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(record.ModName),
		strings.TrimSpace(record.ModVersion),
		strings.TrimSpace(record.Verb),
		record.CommandText,
		argsJSON,
		strings.TrimSpace(record.Transport),
		status,
		strings.TrimSpace(record.Target),
		strings.TrimSpace(record.FlakeShell),
		packageRefsJSON,
		record.ShellBusID,
		record.PID,
		record.ExitCode,
		record.RuntimeMS,
		strings.TrimSpace(record.LogPath),
		record.ResultText,
		record.ErrorText,
		nowRFC3339(),
		strings.TrimSpace(record.StartedAt),
		strings.TrimSpace(record.HeartbeatAt),
		strings.TrimSpace(record.FinishedAt),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateCommandRunQueued(db *sql.DB, id, shellBusID int64, target, logPath string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update command_runs
		set status = 'queued',
			target = ?,
			shell_bus_id = ?,
			log_path = ?,
			heartbeat_at = ?
		where id = ?`,
		strings.TrimSpace(target),
		shellBusID,
		strings.TrimSpace(logPath),
		nowRFC3339(),
		id,
	)
	return err
}

func MarkCommandRunRunning(db *sql.DB, id int64, pid int, target, logPath string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	now := nowRFC3339()
	_, err := db.Exec(`update command_runs
		set status = 'running',
			pid = ?,
			target = case when ? <> '' then ? else target end,
			log_path = case when ? <> '' then ? else log_path end,
			started_at = case when started_at = '' then ? else started_at end,
			heartbeat_at = ?
		where id = ?`,
		pid,
		strings.TrimSpace(target), strings.TrimSpace(target),
		strings.TrimSpace(logPath), strings.TrimSpace(logPath),
		now,
		now,
		id,
	)
	return err
}

func HeartbeatCommandRun(db *sql.DB, id int64, pid int, logPath string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update command_runs
		set pid = case when ? > 0 then ? else pid end,
			log_path = case when ? <> '' then ? else log_path end,
			heartbeat_at = ?
		where id = ?`,
		pid, pid,
		strings.TrimSpace(logPath), strings.TrimSpace(logPath),
		nowRFC3339(),
		id,
	)
	return err
}

func FinishCommandRun(db *sql.DB, id int64, status string, pid, exitCode int, runtimeMS int64, target, logPath, resultText, errorText string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	now := nowRFC3339()
	_, err := db.Exec(`update command_runs
		set status = ?,
			pid = case when ? > 0 then ? else pid end,
			exit_code = ?,
			runtime_ms = ?,
			target = case when ? <> '' then ? else target end,
			log_path = case when ? <> '' then ? else log_path end,
			result_text = ?,
			error_text = ?,
			heartbeat_at = ?,
			finished_at = ?
		where id = ?`,
		strings.TrimSpace(status),
		pid, pid,
		exitCode,
		runtimeMS,
		strings.TrimSpace(target), strings.TrimSpace(target),
		strings.TrimSpace(logPath), strings.TrimSpace(logPath),
		resultText,
		errorText,
		now,
		now,
		id,
	)
	return err
}

func LoadCommandRuns(db *sql.DB, limit int) ([]CommandRunRecord, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`select id, mod_name, mod_version, verb, command_text, args_json, transport, status, target,
		flake_shell, package_refs_json, shell_bus_id, pid, exit_code, runtime_ms, log_path, result_text, error_text,
		created_at, started_at, heartbeat_at, finished_at
		from command_runs
		order by id desc
		limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CommandRunRecord{}
	for rows.Next() {
		var record CommandRunRecord
		if err := rows.Scan(
			&record.ID, &record.ModName, &record.ModVersion, &record.Verb, &record.CommandText, &record.ArgsJSON,
			&record.Transport, &record.Status, &record.Target, &record.FlakeShell, &record.PackageRefsJSON,
			&record.ShellBusID, &record.PID, &record.ExitCode, &record.RuntimeMS, &record.LogPath, &record.ResultText,
			&record.ErrorText, &record.CreatedAt, &record.StartedAt, &record.HeartbeatAt, &record.FinishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadCommandRun(db *sql.DB, id int64) (CommandRunRecord, bool, error) {
	if err := EnsureSchema(db); err != nil {
		return CommandRunRecord{}, false, err
	}
	var record CommandRunRecord
	err := db.QueryRow(`select id, mod_name, mod_version, verb, command_text, args_json, transport, status, target,
		flake_shell, package_refs_json, shell_bus_id, pid, exit_code, runtime_ms, log_path, result_text, error_text,
		created_at, started_at, heartbeat_at, finished_at
		from command_runs
		where id = ?`, id).Scan(
		&record.ID, &record.ModName, &record.ModVersion, &record.Verb, &record.CommandText, &record.ArgsJSON,
		&record.Transport, &record.Status, &record.Target, &record.FlakeShell, &record.PackageRefsJSON,
		&record.ShellBusID, &record.PID, &record.ExitCode, &record.RuntimeMS, &record.LogPath, &record.ResultText,
		&record.ErrorText, &record.CreatedAt, &record.StartedAt, &record.HeartbeatAt, &record.FinishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return CommandRunRecord{}, false, nil
		}
		return CommandRunRecord{}, false, err
	}
	return record, true, nil
}

func LinkShellBusCommandRun(db *sql.DB, shellBusID, commandRunID int64) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update shell_bus set command_run_id = ?, updated_at = ? where id = ?`,
		commandRunID,
		nowRFC3339(),
		shellBusID,
	)
	return err
}

func MarkCommandStarted(db *sql.DB, id int64) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update command_queue set status = 'running', started_at = ? where id = ?`, nowRFC3339(), id)
	return err
}

func MarkCommandFinished(db *sql.DB, id int64, status, resultText, errorText string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update command_queue
		set status = ?, result_text = ?, error_text = ?, finished_at = ?
		where id = ?`,
		strings.TrimSpace(status), resultText, errorText, nowRFC3339(), id)
	return err
}

func LoadQueue(db *sql.DB, queueName string, limit int) ([]QueueRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`select id, command_run_id, queue_name, status, kind, target, command_text, payload_json, result_text, error_text, created_at, started_at, finished_at
		from command_queue
		where queue_name = ?
		order by id desc
		limit ?`, strings.TrimSpace(queueName), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QueueRecord{}
	for rows.Next() {
		var record QueueRecord
		if err := rows.Scan(
			&record.ID, &record.CommandRunID, &record.QueueName, &record.Status, &record.Kind, &record.Target,
			&record.CommandText, &record.PayloadJSON, &record.ResultText, &record.ErrorText,
			&record.CreatedAt, &record.StartedAt, &record.FinishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func StartProtocolRun(db *sql.DB, name, promptText, promptTarget, commandTarget string) (int64, error) {
	if err := EnsureSchema(db); err != nil {
		return 0, err
	}
	result, err := db.Exec(`insert into protocol_runs(name, status, prompt_text, prompt_target, command_target, started_at)
		values(?, 'running', ?, ?, ?, ?)`,
		strings.TrimSpace(name),
		promptText,
		strings.TrimSpace(promptTarget),
		strings.TrimSpace(commandTarget),
		nowRFC3339(),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func FinishProtocolRun(db *sql.DB, id int64, status, resultText, errorText string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update protocol_runs
		set status = ?, result_text = ?, error_text = ?, finished_at = ?
		where id = ?`,
		strings.TrimSpace(status), resultText, errorText, nowRFC3339(), id)
	return err
}

func AppendProtocolEvent(db *sql.DB, event ProtocolEventRecord) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`insert into protocol_events(run_id, event_index, event_type, queue_name, queue_row_id, pane_target, command_text, message_text, created_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.RunID,
		event.EventIndex,
		strings.TrimSpace(event.EventType),
		strings.TrimSpace(event.QueueName),
		event.QueueRowID,
		strings.TrimSpace(event.PaneTarget),
		event.CommandText,
		event.MessageText,
		nowRFC3339(),
	)
	return err
}

func LoadProtocolRuns(db *sql.DB, limit int) ([]ProtocolRunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`select id, name, status, prompt_text, prompt_target, command_target, result_text, error_text, started_at, finished_at
		from protocol_runs
		order by id desc
		limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProtocolRunRecord{}
	for rows.Next() {
		var record ProtocolRunRecord
		if err := rows.Scan(
			&record.ID, &record.Name, &record.Status, &record.PromptText, &record.PromptTarget,
			&record.CommandTarget, &record.ResultText, &record.ErrorText, &record.StartedAt, &record.FinishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadProtocolEvents(db *sql.DB, runID int64) ([]ProtocolEventRecord, error) {
	rows, err := db.Query(`select run_id, event_index, event_type, queue_name, queue_row_id, pane_target, command_text, message_text, created_at
		from protocol_events
		where run_id = ?
		order by event_index`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProtocolEventRecord{}
	for rows.Next() {
		var record ProtocolEventRecord
		if err := rows.Scan(
			&record.RunID, &record.EventIndex, &record.EventType, &record.QueueName, &record.QueueRowID,
			&record.PaneTarget, &record.CommandText, &record.MessageText, &record.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadModTestRuns(db *sql.DB, limit int) ([]ModTestRunRecord, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`select id, plan_name, status, total_steps, passed_steps, failed_steps, skipped_steps, stop_on_error, error_text, started_at, finished_at
		from mod_test_runs
		order by id desc
		limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModTestRunRecord{}
	for rows.Next() {
		var record ModTestRunRecord
		var stopOnError int
		if err := rows.Scan(
			&record.ID, &record.PlanName, &record.Status, &record.TotalSteps, &record.PassedSteps,
			&record.FailedSteps, &record.SkippedSteps, &stopOnError, &record.ErrorText, &record.StartedAt, &record.FinishedAt,
		); err != nil {
			return nil, err
		}
		record.StopOnError = stopOnError == 1
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadModTestRun(db *sql.DB, runID int64) (ModTestRunRecord, bool, error) {
	if err := EnsureSchema(db); err != nil {
		return ModTestRunRecord{}, false, err
	}
	var record ModTestRunRecord
	var stopOnError int
	err := db.QueryRow(`select id, plan_name, status, total_steps, passed_steps, failed_steps, skipped_steps, stop_on_error, error_text, started_at, finished_at
		from mod_test_runs
		where id = ?`, runID).Scan(
		&record.ID, &record.PlanName, &record.Status, &record.TotalSteps, &record.PassedSteps,
		&record.FailedSteps, &record.SkippedSteps, &stopOnError, &record.ErrorText, &record.StartedAt, &record.FinishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return ModTestRunRecord{}, false, nil
		}
		return ModTestRunRecord{}, false, err
	}
	record.StopOnError = stopOnError == 1
	return record, true, nil
}

func LoadModTestRunSteps(db *sql.DB, runID int64) ([]ModTestRunStepRecord, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`select run_id, step_index, mod_name, mod_version, serial_group, visible_tmux, requires_nix,
		status, exit_code, queue_id, command_text, output_text, error_text, started_at, finished_at, runtime_ms
		from mod_test_run_steps
		where run_id = ?
		order by step_index`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ModTestRunStepRecord{}
	for rows.Next() {
		var record ModTestRunStepRecord
		var visibleTmux int
		var requiresNix int
		if err := rows.Scan(
			&record.RunID, &record.StepIndex, &record.ModName, &record.ModVersion, &record.SerialGroup, &visibleTmux, &requiresNix,
			&record.Status, &record.ExitCode, &record.QueueID, &record.CommandText, &record.OutputText, &record.ErrorText,
			&record.StartedAt, &record.FinishedAt, &record.RuntimeMS,
		); err != nil {
			return nil, err
		}
		record.VisibleTmux = visibleTmux == 1
		record.RequiresNix = requiresNix == 1
		out = append(out, record)
	}
	return out, rows.Err()
}

func EnqueueShellBus(db *sql.DB, system, scope, subject, action, actor, session, pane, bodyJSON string) (int64, error) {
	if err := EnsureSchema(db); err != nil {
		return 0, err
	}
	now := nowRFC3339()
	result, err := db.Exec(`insert into shell_bus(system, scope, subject, action, status, actor, session, pane, ref_id, command_run_id, body_json, created_at, updated_at)
		values(?, ?, ?, ?, 'queued', ?, ?, ?, 0, 0, ?, ?, ?)`,
		strings.TrimSpace(system),
		strings.TrimSpace(scope),
		strings.TrimSpace(subject),
		strings.TrimSpace(action),
		strings.TrimSpace(actor),
		strings.TrimSpace(session),
		strings.TrimSpace(pane),
		bodyJSON,
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func AppendShellBusObserved(db *sql.DB, system, subject, action, actor, session, pane string, refID int64, bodyJSON string) (int64, error) {
	if err := EnsureSchema(db); err != nil {
		return 0, err
	}
	now := nowRFC3339()
	result, err := db.Exec(`insert into shell_bus(system, scope, subject, action, status, actor, session, pane, ref_id, body_json, created_at, updated_at)
		values(?, 'observed', ?, ?, 'done', ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(system),
		strings.TrimSpace(subject),
		strings.TrimSpace(action),
		strings.TrimSpace(actor),
		strings.TrimSpace(session),
		strings.TrimSpace(pane),
		refID,
		bodyJSON,
		now,
		now,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateShellBusStatus(db *sql.DB, id int64, status string, refID int64, bodyJSON string) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	_, err := db.Exec(`update shell_bus set status = ?, ref_id = ?, body_json = ?, updated_at = ? where id = ?`,
		strings.TrimSpace(status), refID, bodyJSON, nowRFC3339(), id)
	return err
}

func LoadShellBus(db *sql.DB, scope string, limit int) ([]ShellBusRecord, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`select id, system, scope, subject, action, status, actor, session, pane, ref_id, command_run_id, body_json, created_at, updated_at
		from shell_bus
		where (? = '' or scope = ?)
		order by id desc
		limit ?`, strings.TrimSpace(scope), strings.TrimSpace(scope), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShellBusRecord{}
	for rows.Next() {
		var record ShellBusRecord
		if err := rows.Scan(&record.ID, &record.System, &record.Scope, &record.Subject, &record.Action, &record.Status, &record.Actor, &record.Session, &record.Pane, &record.RefID, &record.CommandRunID, &record.BodyJSON, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadQueuedShellBus(db *sql.DB, limit int) ([]ShellBusRecord, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`select id, system, scope, subject, action, status, actor, session, pane, ref_id, command_run_id, body_json, created_at, updated_at
		from shell_bus
		where scope = 'desired' and status = 'queued'
		order by id asc
		limit ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ShellBusRecord{}
	for rows.Next() {
		var record ShellBusRecord
		if err := rows.Scan(&record.ID, &record.System, &record.Scope, &record.Subject, &record.Action, &record.Status, &record.Actor, &record.Session, &record.Pane, &record.RefID, &record.CommandRunID, &record.BodyJSON, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadShellBusRecord(db *sql.DB, id int64) (ShellBusRecord, bool, error) {
	if err := EnsureSchema(db); err != nil {
		return ShellBusRecord{}, false, err
	}
	var record ShellBusRecord
	err := db.QueryRow(`select id, system, scope, subject, action, status, actor, session, pane, ref_id, command_run_id, body_json, created_at, updated_at
		from shell_bus
		where id = ?`, id).
		Scan(&record.ID, &record.System, &record.Scope, &record.Subject, &record.Action, &record.Status, &record.Actor, &record.Session, &record.Pane, &record.RefID, &record.CommandRunID, &record.BodyJSON, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return ShellBusRecord{}, false, nil
		}
		return ShellBusRecord{}, false, err
	}
	return record, true, nil
}

func LoadTopology(db *sql.DB) ([]TopologyRecord, error) {
	rows, err := db.Query(`select mod_name, mod_version, topo_rank
		from mod_topology
		order by topo_rank, mod_name, mod_version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TopologyRecord{}
	for rows.Next() {
		var record TopologyRecord
		if err := rows.Scan(&record.ModName, &record.ModVersion, &record.TopoRank); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadTestPlan(db *sql.DB, planName string) ([]TestStepRecord, error) {
	rows, err := db.Query(`select plan_name, step_index, mod_name, mod_version, topo_rank, serial_group, requires_nix, visible_tmux, command_text
		from mod_test_steps
		where plan_name = ?
		order by step_index`, strings.TrimSpace(planName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TestStepRecord{}
	for rows.Next() {
		var record TestStepRecord
		var requiresNix int
		var visibleTmux int
		if err := rows.Scan(&record.PlanName, &record.StepIndex, &record.ModName, &record.ModVersion, &record.TopoRank, &record.SerialGroup, &requiresNix, &visibleTmux, &record.CommandText); err != nil {
			return nil, err
		}
		record.RequiresNix = requiresNix == 1
		record.VisibleTmux = visibleTmux == 1
		out = append(out, record)
	}
	return out, rows.Err()
}

func BuildTopology(mods []ModRecord, deps []DependencyRecord) ([]TopologyRecord, error) {
	type node struct {
		name    string
		version string
	}
	nodeKey := func(name, version string) string {
		return strings.TrimSpace(name) + ":" + strings.TrimSpace(version)
	}
	nodes := map[string]node{}
	keys := make([]string, 0, len(mods))
	for _, mod := range mods {
		key := nodeKey(mod.Name, mod.Version)
		nodes[key] = node{name: mod.Name, version: mod.Version}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	adj := map[string][]string{}
	indegree := map[string]int{}
	for _, key := range keys {
		indegree[key] = 0
	}
	for _, dep := range deps {
		from := nodeKey(dep.FromName, dep.FromVersion)
		to := nodeKey(dep.ToName, dep.ToVersion)
		if _, ok := nodes[from]; !ok {
			continue
		}
		if _, ok := nodes[to]; !ok {
			continue
		}
		adj[to] = append(adj[to], from)
		indegree[from]++
	}
	for key := range adj {
		sort.Strings(adj[key])
	}
	ready := []string{}
	for _, key := range keys {
		if indegree[key] == 0 {
			ready = append(ready, key)
		}
	}
	sort.Strings(ready)
	out := make([]TopologyRecord, 0, len(nodes))
	for len(ready) > 0 {
		current := ready[0]
		ready = ready[1:]
		item := nodes[current]
		out = append(out, TopologyRecord{
			ModName:    item.name,
			ModVersion: item.version,
			TopoRank:   len(out),
		})
		for _, next := range adj[current] {
			indegree[next]--
			if indegree[next] == 0 {
				ready = append(ready, next)
				sort.Strings(ready)
			}
		}
	}
	if len(out) != len(nodes) {
		remaining := []string{}
		for _, key := range keys {
			if indegree[key] > 0 {
				remaining = append(remaining, key)
			}
		}
		return nil, fmt.Errorf("mod dependency cycle detected: %s", strings.Join(remaining, ", "))
	}
	return out, nil
}

func BuildTestPlan(mods []ModRecord, manifests map[string]Manifest, topology []TopologyRecord) []TestStepRecord {
	modByKey := map[string]ModRecord{}
	for _, mod := range mods {
		modByKey[strings.TrimSpace(mod.Name)+":"+strings.TrimSpace(mod.Version)] = mod
	}
	out := make([]TestStepRecord, 0, len(topology))
	for idx, item := range topology {
		key := strings.TrimSpace(item.ModName) + ":" + strings.TrimSpace(item.ModVersion)
		mod, ok := modByKey[key]
		if !ok {
			continue
		}
		manifest := manifests[key]
		commandPath := "./" + strings.TrimPrefix(filepath.ToSlash(mod.Path), "src/") + "/..."
		out = append(out, TestStepRecord{
			PlanName:    "default",
			StepIndex:   idx + 1,
			ModName:     item.ModName,
			ModVersion:  item.ModVersion,
			TopoRank:    item.TopoRank,
			SerialGroup: strings.TrimSpace(manifest.Testing.SerialGroup),
			RequiresNix: manifest.Testing.RequiresNix,
			VisibleTmux: manifest.Testing.VisibleTmux,
			CommandText: "go test " + commandPath,
		})
	}
	return out
}

func LoadMods(db *sql.DB) ([]ModRecord, error) {
	rows, err := db.Query(`select name, version, path, readme_path, manifest_path, has_main, has_cli
		from mods
		order by name, version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ModRecord
	for rows.Next() {
		var record ModRecord
		var hasMain int
		var hasCLI int
		if err := rows.Scan(&record.Name, &record.Version, &record.Path, &record.ReadmePath, &record.ManifestPath, &hasMain, &hasCLI); err != nil {
			return nil, err
		}
		record.HasMain = hasMain == 1
		record.HasCLI = hasCLI == 1
		out = append(out, record)
	}
	return out, rows.Err()
}

func ResolveEntrypoint(db *sql.DB, srcRoot, modName, version, command string) (Entrypoint, error) {
	var path string
	var hasMain int
	var hasCLI int
	err := db.QueryRow(`select path, has_main, has_cli
		from mods
		where name = ? and version = ?`,
		strings.TrimSpace(modName), strings.TrimSpace(version),
	).Scan(&path, &hasMain, &hasCLI)
	if err != nil {
		if err == sql.ErrNoRows {
			return Entrypoint{}, fmt.Errorf("mod not found in sqlite registry: %s %s", modName, version)
		}
		return Entrypoint{}, err
	}
	entry := Entrypoint{
		Name:    strings.TrimSpace(modName),
		Version: strings.TrimSpace(version),
		Path:    srcRelativePath(srcRoot, path),
		HasMain: hasMain == 1,
		HasCLI:  hasCLI == 1,
	}
	_ = command
	if entry.HasCLI {
		entry.Path = srcRelativePath(srcRoot, filepath.ToSlash(filepath.Join(path, "cli")))
		return entry, nil
	}
	if entry.HasMain {
		return Entrypoint{}, fmt.Errorf("mod is missing required cli/main.go wrapper in sqlite registry: %s %s", modName, version)
	}
	return Entrypoint{}, fmt.Errorf("mod has no runnable cli entrypoint in sqlite registry: %s %s", modName, version)
}

func LoadNixPackages(db *sql.DB, modName, version string) ([]NixPackageRecord, error) {
	rows, err := db.Query(`select mod_name, mod_version, selector, package_ref
		from mod_nix_packages
		where mod_name = ? and mod_version = ?
		order by selector, package_ref`,
		strings.TrimSpace(modName), strings.TrimSpace(version))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NixPackageRecord
	for rows.Next() {
		var record NixPackageRecord
		if err := rows.Scan(&record.ModName, &record.ModVersion, &record.Selector, &record.PackageRef); err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

func LoadLaunchConfig(db *sql.DB, modName, version string) (LaunchConfigRecord, error) {
	var record LaunchConfigRecord
	err := db.QueryRow(`select mod_name, mod_version, flake_shell
		from mod_launch_configs
		where mod_name = ? and mod_version = ?`,
		strings.TrimSpace(modName), strings.TrimSpace(version),
	).Scan(&record.ModName, &record.ModVersion, &record.FlakeShell)
	if err != nil {
		if err == sql.ErrNoRows {
			return LaunchConfigRecord{}, fmt.Errorf("launch config not found in sqlite registry: %s %s", modName, version)
		}
		return LaunchConfigRecord{}, err
	}
	return record, nil
}

func RenderGraphText(edges []GraphEdge) string {
	if len(edges) == 0 {
		return "no mod dependencies recorded"
	}
	lines := make([]string, 0, len(edges))
	for _, edge := range edges {
		lines = append(lines, fmt.Sprintf("%s:%s -> %s:%s (%s)", edge.From.Name, edge.From.Version, edge.To.Name, edge.To.Version, edge.Source))
	}
	return strings.Join(lines, "\n")
}

func RenderGraphMermaid(edges []GraphEdge) string {
	lines := []string{"graph TD"}
	for _, edge := range edges {
		lines = append(lines, fmt.Sprintf("  %s --> %s", mermaidNode(edge.From), mermaidNode(edge.To)))
	}
	if len(edges) == 0 {
		lines = append(lines, "  empty[\"no dependencies recorded\"]")
	}
	return strings.Join(lines, "\n")
}

func RenderGraphOutline(topology []TopologyRecord, edges []GraphEdge) string {
	type node struct {
		name    string
		version string
		rank    int
	}
	nodeKey := func(name, version string) string {
		return strings.TrimSpace(name) + ":" + strings.TrimSpace(version)
	}
	nodeLabel := func(name, version string) string {
		return strings.TrimSpace(name) + ":" + strings.TrimSpace(version)
	}

	nodes := map[string]node{}
	rootKeys := make([]string, 0, len(topology))
	depsByFrom := map[string][]string{}
	incoming := map[string]int{}
	for _, item := range topology {
		key := nodeKey(item.ModName, item.ModVersion)
		nodes[key] = node{name: item.ModName, version: item.ModVersion, rank: item.TopoRank}
		rootKeys = append(rootKeys, key)
		incoming[key] = 0
	}
	for _, edge := range edges {
		fromKey := nodeKey(edge.From.Name, edge.From.Version)
		toKey := nodeKey(edge.To.Name, edge.To.Version)
		if _, ok := nodes[fromKey]; !ok {
			continue
		}
		if _, ok := nodes[toKey]; !ok {
			continue
		}
		depsByFrom[fromKey] = append(depsByFrom[fromKey], toKey)
		incoming[toKey]++
	}
	sort.Slice(rootKeys, func(i, j int) bool {
		left := nodes[rootKeys[i]]
		right := nodes[rootKeys[j]]
		if left.rank == right.rank {
			if left.name == right.name {
				return left.version < right.version
			}
			return left.name < right.name
		}
		return left.rank < right.rank
	})
	for key, deps := range depsByFrom {
		sort.Slice(deps, func(i, j int) bool {
			left := nodes[deps[i]]
			right := nodes[deps[j]]
			if left.rank == right.rank {
				if left.name == right.name {
					return left.version < right.version
				}
				return left.name < right.name
			}
			return left.rank < right.rank
		})
		depsByFrom[key] = deps
	}

	lines := []string{}
	visitedRoots := 0
	var walk func(key string, depth int)
	walk = func(key string, depth int) {
		item, ok := nodes[key]
		if !ok {
			return
		}
		indent := strings.Repeat("  ", depth)
		lines = append(lines, fmt.Sprintf("%s- %s", indent, nodeLabel(item.name, item.version)))
		for _, depKey := range depsByFrom[key] {
			walk(depKey, depth+1)
		}
	}
	for _, key := range rootKeys {
		if incoming[key] != 0 {
			continue
		}
		walk(key, 0)
		visitedRoots++
	}
	if visitedRoots == 0 {
		if len(topology) == 0 {
			return "no mod dependencies recorded"
		}
		for _, key := range rootKeys {
			walk(key, 0)
		}
	}
	return strings.Join(lines, "\n")
}

func CaptureRuntimeEnv() map[string]string {
	result := map[string]string{}
	for _, item := range os.Environ() {
		key, value, found := strings.Cut(item, "=")
		if !found {
			continue
		}
		if ShouldPersistRuntimeEnvKey(key) {
			result[key] = value
		}
	}
	return result
}

func ShouldPersistRuntimeEnvKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if !strings.HasPrefix(trimmed, "DIALTONE_") && trimmed != "NIXPKGS_FLAKE" {
		return false
	}
	_, blocked := volatileRuntimeEnvKeys[trimmed]
	return !blocked
}

func readManifest(versionDir string) (Manifest, bool, error) {
	path := filepath.Join(versionDir, "mod.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return manifest, true, nil
}

func readNixPackages(versionDir, modName, version string) ([]NixPackageRecord, error) {
	path := filepath.Join(versionDir, "nix.packages")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var out []NixPackageRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		selector := "all"
		pkg := raw
		if left, right, found := strings.Cut(raw, ":"); found && !strings.HasPrefix(raw, "nixpkgs#") {
			selector = strings.TrimSpace(left)
			pkg = strings.TrimSpace(right)
		}
		out = append(out, NixPackageRecord{
			ModName:    modName,
			ModVersion: version,
			Selector:   selector,
			PackageRef: pkg,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func hasGoPackage(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			return true
		}
	}
	return false
}

func relOrEmpty(repoRoot, path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return filepath.ToSlash(relative(repoRoot, path))
}

func relative(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

func mermaidNode(node GraphNode) string {
	return strings.ReplaceAll(node.Name+"_"+node.Version, "-", "_")
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func sortedRuntimeEnvKeys(env map[string]string) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func srcRelativePath(srcRoot, repoRelative string) string {
	target := filepath.Join(filepath.Dir(srcRoot), filepath.FromSlash(strings.TrimSpace(repoRelative)))
	rel, err := filepath.Rel(srcRoot, target)
	if err != nil {
		return filepath.ToSlash(repoRelative)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return "."
	}
	if strings.HasPrefix(rel, ".") {
		return rel
	}
	return "./" + rel
}
