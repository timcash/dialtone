package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dialtone/dev/internal/modcli"
	"dialtone/dev/internal/modstate"
	"dialtone/dev/internal/tmuxcmd"
	"dialtone/dev/mods/shared/dispatch"
	"dialtone/dev/mods/shared/nixplan"
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
			if err := routeCommandViaShell(repoRoot, os.Args[1:]); err != nil {
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
	if hasGoPackage(filepath.Join(modDir, "cli")) {
		return relativeFrom(srcRoot, filepath.Join(modDir, "cli"))
	}
	return ""
}

func routeCommandViaShell(repoRoot string, args []string) error {
	cmdArgs := append([]string{"dialtone", "v1", "queue"}, args...)
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone_mod"), cmdArgs...)
	cmd.Dir = repoRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return fmt.Errorf("failed to queue routed command via dialtone: %w", err)
	}
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
	repoRoot, err := findRepoRoot()
	if err != nil {
		repoRoot = ""
	}
	return modcli.CurrentTmuxTarget(os.Getenv("DIALTONE_TMUX_TARGET"), os.Getenv("TMUX_PANE"), func(paneID string) (string, error) {
		out, err := tmuxcmd.Command(repoRoot, "display-message", "-p", "-t", paneID, "#{session_name}:#{window_index}:#{pane_index}").Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	})
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
