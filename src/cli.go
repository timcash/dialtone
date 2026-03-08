package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	setBaseEnv(repoRoot)
	srcRoot := filepath.Join(repoRoot, "src")

	if len(os.Args) < 3 {
		printUsage(repoRoot)
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
	entry := resolveModEntry(srcRoot, modName, version, commandArg)
	if entry == "" {
		return fmt.Errorf("unknown mod entrypoint for %s %s", modName, version)
	}

	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
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

func resolveModEntry(srcRoot, modName, version, command string) string {
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
	listAvailableMods(filepath.Join(repoRoot, "src", "mods"))
}

func listAvailableMods(modRoot string) {
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
	_ = os.Setenv("DIALTONE_REPO_ROOT", repoRoot)
	_ = os.Setenv("DIALTONE_SRC_ROOT", filepath.Join(repoRoot, "src"))
	_ = os.Setenv("DIALTONE_ENV_FILE", filepath.Join(repoRoot, "env", ".env"))
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
		if strings.HasSuffix(entry.Name(), ".go") {
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
