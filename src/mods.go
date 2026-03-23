package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dialtone/dev/internal/modstate"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/nixplan"
	"dialtone/dev/mods/shared/router"
	"dialtone/dev/mods/shared/sqlitestate"
)

func main() {
	if err := runCLI(); err != nil {
		fmt.Fprintln(os.Stderr, "DIALTONE>", err)
		os.Exit(1)
	}
}

func runCLI() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	stateDB := openStateDBBestEffort(repoRoot)
	if stateDB != nil {
		_, _ = sqlitestate.HydrateRuntimeEnv(stateDB, "process", false)
	}
	setBaseEnv(repoRoot)
	srcRoot := filepath.Join(repoRoot, "src")
	stateDB = syncStateDBBestEffort(repoRoot, stateDB)
	if stateDB != nil {
		defer stateDB.Close()
	}

	if len(os.Args) < 3 {
		printUsage(repoRoot)
		return nil
	}
	if strings.TrimSpace(os.Args[1]) == "__nix-plan" {
		if len(os.Args) < 4 {
			return fmt.Errorf("__nix-plan requires <mod> <version>")
		}
		modName := mapCommandNameToMod(strings.TrimSpace(os.Args[2]))
		version := strings.TrimSpace(os.Args[3])
		plan, err := nixplan.BuildPlan(stateDB, repoRoot, modName, version, runtime.GOOS)
		if err != nil {
			return err
		}
		if strings.TrimSpace(plan.FlakeShell) != "" {
			fmt.Printf("flake_shell\t%s\n", strings.TrimSpace(plan.FlakeShell))
		}
		for _, pkg := range plan.Packages {
			fmt.Printf("package\t%s\n", strings.TrimSpace(pkg))
		}
		return nil
	}

	modName := mapCommandNameToMod(strings.TrimSpace(os.Args[1]))
	version := strings.TrimSpace(os.Args[2])
	if modName == "" || version == "" {
		printUsage(repoRoot)
		return nil
	}
	if !isVersionArg(version) {
		printUsage(repoRoot)
		return nil
	}

	if modName == "mesh" && version == "v3" {
		meshDir := filepath.Join(srcRoot, "mods", "mesh", "v3")
		return runMeshV3(meshDir, os.Args[3:])
	}
	if modName == "mesh" {
		return fmt.Errorf("unsupported mesh version %s; use mesh v3", version)
	}

	commandArg := ""
	if len(os.Args) > 3 {
		commandArg = strings.TrimSpace(os.Args[3])
	}

	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}

	if stateDB != nil && dispatch.ShouldRouteViaShell(modName, commandArg) {
		executeDirect, err := dispatch.ShouldExecuteDirectInPane(stateDB, os.Args[1:], currentTmuxPaneTarget())
		if err != nil {
			return err
		}
		if !executeDirect {
			if err := routeCommandViaShell(repoRoot, goBin, stateDB, os.Args[1:]); err != nil {
				return err
			}
			return nil
		}
	}

	entry := resolveModEntry(srcRoot, stateDB, modName, version, commandArg)
	if entry == "" {
		return fmt.Errorf("unknown mod entrypoint for %s %s", modName, version)
	}

	cmdArgs := append([]string{"run", entry}, os.Args[3:]...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = srcRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to launch %s %s: %w", modName, version, err)
	}
	return nil
}

func resolveModEntry(srcRoot string, stateDB *sql.DB, modName, version, command string) string {
	if stateDB != nil {
		if entry, err := resolveModEntryFromState(stateDB, srcRoot, modName, version, command); err == nil && strings.TrimSpace(entry) != "" {
			return entry
		}
	}
	modDir := filepath.Join(srcRoot, "mods", modName, version)
	if shouldUseModCLI(command) && hasGoPackage(filepath.Join(modDir, "cli")) {
		return relativeFrom(srcRoot, filepath.Join(modDir, "cli"))
	}
	if hasGoPackage(modDir) {
		return relativeFrom(srcRoot, modDir)
	}
	if hasGoPackage(filepath.Join(modDir, "cli")) {
		return relativeFrom(srcRoot, filepath.Join(modDir, "cli"))
	}
	return ""
}

func routeCommandViaShell(repoRoot, goBin string, stateDB *sql.DB, args []string) error {
	if stateDB == nil {
		return fmt.Errorf("sqlite state is required to route commands via shell")
	}
	rowID, err := router.QueueCommandViaShell(stateDB, repoRoot, args)
	if err != nil {
		return err
	}
	startedPID, startedLogPath, err := ensureShellWorkerAsync(repoRoot, stateDB)
	if err != nil {
		return err
	}
	printDialtoneRouteReport(repoRoot, rowID, startedPID, startedLogPath)
	return nil
}

type goRunner struct{}

func (goRunner) Run(repoRoot, goBin, entry string, args ...string) error {
	cmdArgs := append([]string{"run", entry}, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
	return fmt.Errorf("failed to launch %s: %w", entry, err)
	}
	return nil
}

func currentTmuxPaneTarget() string {
	paneID := strings.TrimSpace(os.Getenv("TMUX_PANE"))
	if paneID == "" {
		return ""
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", paneID, "#{session_name}:#{window_index}:#{pane_index}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ensureShellWorkerAsync(repoRoot string, stateDB *sql.DB) (int, string, error) {
	ready, err := dispatch.ShellReady(stateDB)
	if err != nil {
		return 0, "", err
	}
	healthy, err := router.ShellWorkerHealthy(stateDB, 5*time.Second)
	if err != nil {
		return 0, "", err
	}
	if ready && healthy {
		return existingEnsureProcess(stateDB)
	}
	if pid, logPath, ok, err := existingEnsureProcessIfAlive(stateDB); err != nil {
		return 0, "", err
	} else if ok {
		return pid, logPath, nil
	}
	pid, logPath, err := startDetachedDialtoneProcess(repoRoot, "shell", "v1", "ensure-worker", "--wait-seconds", "30")
	if err != nil {
		return 0, "", err
	}
	if err := modstate.UpsertStateValue(stateDB, sqlitestate.SystemScope, sqlitestate.ShellEnsurePIDKey, strconv.Itoa(pid)); err != nil {
		return 0, "", err
	}
	if err := modstate.UpsertStateValue(stateDB, sqlitestate.SystemScope, sqlitestate.ShellEnsureLogPathKey, logPath); err != nil {
		return 0, "", err
	}
	if err := modstate.UpsertStateValue(stateDB, sqlitestate.SystemScope, sqlitestate.ShellEnsureStartedAtKey, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return 0, "", err
	}
	return pid, logPath, nil
}

func existingEnsureProcess(stateDB *sql.DB) (int, string, error) {
	if stateDB == nil {
		return 0, "", nil
	}
	pidText, ok, err := loadOptionalStateValue(stateDB, sqlitestate.ShellEnsurePIDKey)
	if err != nil || !ok {
		return 0, "", err
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return 0, "", nil
	}
	logPath, _, err := loadOptionalStateValue(stateDB, sqlitestate.ShellEnsureLogPathKey)
	if err != nil {
		return 0, "", err
	}
	return pid, logPath, nil
}

func existingEnsureProcessIfAlive(stateDB *sql.DB) (int, string, bool, error) {
	pid, logPath, err := existingEnsureProcess(stateDB)
	if err != nil || pid <= 0 {
		return 0, "", false, err
	}
	return pid, logPath, processAlive(pid), nil
}

func loadOptionalStateValue(db *sql.DB, key string) (string, bool, error) {
	record, ok, err := modstate.LoadStateValue(db, sqlitestate.SystemScope, key)
	if err != nil || !ok {
		return "", ok, err
	}
	return strings.TrimSpace(record.Value), true, nil
}

func startDetachedDialtoneProcess(repoRoot string, args ...string) (int, string, error) {
	logDir := filepath.Join(sqlitestate.ResolveStateDir(repoRoot), "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return 0, "", err
	}
	logPath := filepath.Join(logDir, fmt.Sprintf("dialtone-%d.log", time.Now().UnixNano()))
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, "", err
	}
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone_mod"), args...)
	cmd.Dir = repoRoot
	cmd.Stdout = file
	cmd.Stderr = file
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = file.Close()
		return 0, "", err
	}
	_ = file.Close()
	return cmd.Process.Pid, logPath, nil
}

type dialtoneProcessRecord struct {
	PID        int
	PPID       int
	Stat       string
	TTY        string
	Role       string
	Command    string
	Background bool
}

func printDialtoneRouteReport(repoRoot string, rowID int64, startedPID int, startedLogPath string) {
	processes, _ := loadDialtoneProcessReport()
	backgroundCount := 0
	for _, process := range processes {
		if process.Background {
			backgroundCount++
		}
	}
	fmt.Printf("queued to dialtone-view [row_id=%d]\n", rowID)
	fmt.Printf("state_db\t%s\n", sqlitestate.ResolveStateDBPath(repoRoot))
	if startedPID > 0 {
		fmt.Printf("started_pid\t%d\n", startedPID)
	}
	if strings.TrimSpace(startedLogPath) != "" {
		fmt.Printf("started_log\t%s\n", startedLogPath)
	}
	fmt.Printf("dialtone_mod_running\t%d\n", len(processes))
	fmt.Printf("dialtone_mod_background\t%d\n", backgroundCount)
	fmt.Println("pid\tppid\tstat\ttty\trole\tbackground\tcommand")
	for _, process := range processes {
		background := "no"
		if process.Background {
			background = "yes"
		}
		fmt.Printf("%d\t%d\t%s\t%s\t%s\t%s\t%s\n",
			process.PID,
			process.PPID,
			process.Stat,
			process.TTY,
			process.Role,
			background,
			process.Command,
		)
	}
}

func loadDialtoneProcessReport() ([]dialtoneProcessRecord, error) {
	out, err := exec.Command("ps", "-axo", "pid=,ppid=,stat=,tt=,command=").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	records := make([]dialtoneProcessRecord, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 5 {
			continue
		}
		command := strings.Join(fields[4:], " ")
		if !strings.Contains(command, "dialtone_mod") {
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
		records = append(records, dialtoneProcessRecord{
			PID:        pid,
			PPID:       ppid,
			Stat:       stat,
			TTY:        tty,
			Role:       dialtoneProcessRole(command),
			Command:    command,
			Background: tty == "??" || !strings.Contains(stat, "+"),
		})
	}
	return records, nil
}

func dialtoneProcessRole(command string) string {
	switch {
	case strings.Contains(command, "shell v1 serve"):
		return "worker"
	case strings.Contains(command, "shell v1 ensure-worker"):
		return "ensure"
	default:
		return "dialtone_mod"
	}
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

func shouldUseModCLI(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "install", "build", "format", "test":
		return true
	default:
		return false
	}
}

func mapCommandNameToMod(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mods":
		return "mod"
	default:
		return strings.TrimSpace(name)
	}
}

func isVersionArg(v string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(v)), "v")
}

func printUsage(repoRoot string) {
	fmt.Println("Usage: ./dialtone_mod <mod-name> <version> <command> [args]")
	fmt.Println("Examples:")
	fmt.Println("  ./dialtone_mod mods v1 help")
	fmt.Println("  ./dialtone_mod mesh v3 help")
	stateDB := syncStateDBBestEffort(repoRoot, nil)
	if stateDB != nil {
		defer stateDB.Close()
	}
	listAvailableMods(filepath.Join(repoRoot, "src", "mods"), stateDB)
}

func listAvailableMods(modRoot string, stateDB *sql.DB) {
	if stateDB != nil {
		if records, err := listAvailableModsFromState(stateDB); err == nil && len(records) > 0 {
			fmt.Println("Available mods:")
			current := ""
			for _, record := range records {
				if record.Name != current {
					if current != "" {
						fmt.Println()
					}
					fmt.Printf("  %s %s", record.Name, record.Version)
					current = record.Name
					continue
				}
				fmt.Printf(" %s", record.Version)
			}
			fmt.Println()
			return
		}
	}
	entries, err := os.ReadDir(modRoot)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		fmt.Println("No mods directory found.")
		return
	}
	mods := []string{}
	versions := map[string][]string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if name == "" || name[0] == '.' {
			continue
		}
		mods = append(mods, name)
		modPath := filepath.Join(modRoot, name)
		versionEntries, err := os.ReadDir(modPath)
		if err != nil {
			continue
		}
		for _, v := range versionEntries {
			if !v.IsDir() {
				continue
			}
			ver := strings.TrimSpace(v.Name())
			if strings.HasPrefix(ver, "v") {
				mainPath := filepath.Join(modPath, ver)
				cliPath := filepath.Join(modPath, ver, "cli")
				hasEntry := hasGoPackage(mainPath) || hasGoPackage(cliPath) || hasCargoPackage(mainPath) || hasRustPackage(mainPath)
				if filepath.Base(modPath) == "mesh" {
					hasEntry = filepath.Base(ver) == "v3" && (hasCargoPackage(mainPath) || hasRustPackage(mainPath))
				}
				if hasEntry {
					versions[name] = append(versions[name], ver)
				}
			}
		}
	}
	if len(mods) == 0 {
		fmt.Println("No mod command roots found.")
		return
	}
	fmt.Println("Available mods:")
	for _, mod := range mods {
		if len(versions[mod]) == 0 {
			continue
		}
		fmt.Printf("  %s", mod)
		for _, v := range versions[mod] {
			fmt.Printf(" %s", v)
		}
		fmt.Println()
	}
}

func runMeshV3(meshRoot string, args []string) error {
	if len(args) == 0 {
		printMeshV3Usage()
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		printMeshV3Usage()
		return nil
	case "build", "install":
		if strings.ToLower(strings.TrimSpace(args[0])) == "install" {
			return runMeshV3Install(meshRoot, args[1:])
		}
		return runMeshV3Build(meshRoot, args[1:])
	case "format", "fmt":
		return runMeshV3Cargo(meshRoot, []string{"fmt"})
	case "lint":
		return runMeshV3Cargo(meshRoot, append([]string{"clippy", "--all-targets", "--all-features", "--", "-D", "warnings"}, args[1:]...))
	case "test":
		return runMeshV3Cargo(meshRoot, append([]string{"test"}, args[1:]...))
	case "logs":
		return runMeshV3Logs()
	default:
		return runMeshV3Binary(meshRoot, args)
	}
}

func printMeshV3Usage() {
	fmt.Println("Usage: ./dialtone_mod mesh v3 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  install                                       Prepare nix development dependencies for mesh v3")
	fmt.Println("  build [--rebuild] [--target native|rover]     Build mesh-v3 binary")
	fmt.Println("  format                                         Run cargo fmt")
	fmt.Println("  lint                                           Run cargo clippy")
	fmt.Println("  test [--all|--release|...]                     Run cargo test")
	fmt.Println("  node [--flags]                                 Run mesh node mode")
	fmt.Println("  index [--bind_addr]                            Run mesh index mode")
	fmt.Println("  hub [--bind_addr]                              Run mesh node + index")
	fmt.Println("  register/list/connect                           Run mesh runtime commands")
}

func runMeshV3Build(meshRoot string, args []string) error {
	cliDir := filepath.Join(filepath.Dir(meshRoot), "v1", "cli")
	return runMeshV1CliCommand(cliDir, "build", args)
}

func runMeshV3Install(meshRoot string, args []string) error {
	cliDir := filepath.Join(filepath.Dir(meshRoot), "v1", "cli")
	return runMeshV1CliCommand(cliDir, "install", args)
}

func runMeshV1CliCommand(cliDir, commandName string, args []string) error {
	command := []string{"run", "."}
	command = append(command, commandName)
	command = append(command, args...)
	return runCommandInDir(cliDir, "go", command...)
}

func runMeshV3Cargo(meshRoot string, args []string) error {
	nixArgs := []string{
		"--extra-experimental-features", "nix-command flakes",
		"develop", ".",
		"--command",
	}
	nixArgs = append(nixArgs, args...)
	return runNixCommand(meshRoot, nixArgs...)
}

func runMeshV3Binary(meshRoot string, args []string) error {
	bin := meshV3BinaryPath()
	if _, err := os.Stat(bin); err != nil {
		if buildErr := runMeshV3Build(meshRoot, nil); buildErr != nil {
			return buildErr
		}
	}
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("mesh-v3 binary not found after build: %s", bin)
	}
	return runCommand(bin, args, meshRoot)
}

func runMeshV3Logs() error {
	fmt.Println("mesh-v3 runtime logging is handled by the calling process/service")
	fmt.Println("Hint: capture logs with your process supervisor (for example: journalctl -u mesh-v3 --follow)")
	return nil
}

func meshV3BinaryPath() string {
	repoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	if strings.TrimSpace(repoRoot) == "" {
		home := os.Getenv("HOME")
		if home == "" {
			home = "/home/user"
		}
		repoRoot = filepath.Join(home, "dialtone")
	}
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	}
	return filepath.Join(repoRoot, "bin", "mesh-v3_"+arch)
}

func runNixCommand(dir string, args ...string) error {
	nixBin, err := findNixBinary()
	if err != nil {
		return err
	}
	return runCommandInDir(dir, nixBin, args...)
}

func runCommand(command string, args []string, workDir string) error {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to launch command %s: %w", command, err)
	}
	return nil
}

func runCommandInDir(dir, command string, args ...string) error {
	return runCommand(command, args, dir)
}

func hasCargoPackage(path string) bool {
	_, err := os.Stat(filepath.Join(path, "Cargo.toml"))
	return err == nil
}

func hasRustPackage(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.EqualFold(entry.Name(), "src") {
			return true
		}
	}
	return false
}

func findNixBinary() (string, error) {
	if path, err := exec.LookPath("nix"); err == nil {
		return path, nil
	}
	candidates := []string{
		"/nix/var/nix/profiles/default/bin/nix",
		filepath.Join(os.Getenv("HOME"), ".nix-profile/bin/nix"),
		"/run/current-system/sw/bin/nix",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	matches, err := filepath.Glob("/nix/store/*-nix-*/bin/nix")
	if err == nil && len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("nix binary not found")
}

func findRepoRoot() (string, error) {
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
	return "", fmt.Errorf("failed to locate repository root from %s", cwd)
}

func isRepoRoot(candidate string) bool {
	if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(candidate, "src", "go.mod"))
	return err == nil
}

func setBaseEnv(repoRoot string) {
	setEnvDefault("DIALTONE_REPO_ROOT", repoRoot)
	setEnvDefault("DIALTONE_SRC_ROOT", filepath.Join(repoRoot, "src"))
	setEnvDefault("DIALTONE_ENV_FILE", filepath.Join(repoRoot, "env", "dialtone.json"))
	setEnvDefault("DIALTONE_MESH_CONFIG", filepath.Join(repoRoot, "env", "dialtone.json"))
	stateDir := sqlitestate.ResolveStateDir(repoRoot)
	_ = os.MkdirAll(stateDir, 0o755)
	setEnvDefault("DIALTONE_STATE_DIR", stateDir)
	setEnvDefault("DIALTONE_STATE_DB", sqlitestate.ResolveStateDBPath(repoRoot))
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

func relativeFrom(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	rel = filepath.ToSlash(rel)
	if strings.TrimSpace(rel) == "." {
		return "."
	}
	if strings.HasPrefix(rel, ".") {
		return rel
	}
	return "./" + rel
}

func openStateDBBestEffort(repoRoot string) *sql.DB {
	db, err := modstate.Open(sqlitestate.ResolveStateDBPath(repoRoot))
	if err != nil {
		return nil
	}
	if err := modstate.EnsureSchema(db); err != nil {
		_ = db.Close()
		return nil
	}
	return db
}

func syncStateDBBestEffort(repoRoot string, stateDB *sql.DB) *sql.DB {
	db := stateDB
	if db == nil {
		db = openStateDBBestEffort(repoRoot)
		if db == nil {
			return nil
		}
	}
	if _, err := modstate.SyncRepo(db, repoRoot, modstate.CaptureRuntimeEnv()); err != nil {
		_ = db.Close()
		return nil
	}
	return db
}

func setEnvDefault(key, value string) {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, value)
}

func resolveModEntryFromState(stateDB *sql.DB, srcRoot, modName, version, command string) (string, error) {
	entry, err := modstate.ResolveEntrypoint(stateDB, srcRoot, modName, version, command)
	if err != nil {
		return "", err
	}
	return entry.Path, nil
}

func listAvailableModsFromState(stateDB *sql.DB) ([]modstate.ModRecord, error) {
	return modstate.LoadMods(stateDB)
}
