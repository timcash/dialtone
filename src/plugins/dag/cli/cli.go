package cli

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func runBun(repoRoot, uiDir string, args ...string) *exec.Cmd {
	bunArgs := append([]string{"bun", "exec", "--cwd", uiDir}, args...)
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), bunArgs...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	command := args[0]
	getDir := func() string {
		if len(args) > 1 {
			return args[1]
		}
		return getLatestVersionDir()
	}

	switch command {
	case "install":
		return RunInstall(getDir())
	case "fmt":
		return RunFmt(getDir())
	case "format":
		return RunFormat(getDir())
	case "vet":
		return RunVet(getDir())
	case "go-build":
		return RunGoBuild(getDir())
	case "lint":
		return RunLint(getDir())
	case "dev":
		return RunDev(getDir())
	case "ui-run":
		extraArgs := []string{}
		if len(args) > 2 {
			extraArgs = args[2:]
		}
		return RunUIRun(getDir(), extraArgs)
	case "serve":
		return RunServe(getDir())
	case "build":
		return RunBuild(getDir())
	case "test":
		testFlags := flag.NewFlagSet("dag test", flag.ContinueOnError)
		attach := testFlags.Bool("attach", false, "Attach to running headed dev browser session")
		dir := getDir()
		if len(args) > 1 && args[1] != "" && !strings.HasPrefix(args[1], "-") {
			dir = args[1]
			_ = testFlags.Parse(args[2:])
		} else {
			_ = testFlags.Parse(args[1:])
		}
		return RunTest(dir, *attach)
	case "src":
		n := 0
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			n, _ = strconv.Atoi(args[1])
		} else {
			srcFlags := flag.NewFlagSet("dag src", flag.ExitOnError)
			nFlag := srcFlags.Int("n", 0, "Version number to create")
			srcFlags.Parse(args[1:])
			n = *nFlag
		}
		if n == 0 {
			return fmt.Errorf("usage: dag src <N> or dag src --n <N>")
		}
		return RunCreateVersion(n)
	case "smoke":
		smokeFlags := flag.NewFlagSet("dag smoke", flag.ContinueOnError)
		timeout := smokeFlags.Int("smoke-timeout", 45, "Timeout in seconds for smoke test")

		dir := getDir()
		if len(args) > 1 && args[1] != "" && !strings.HasPrefix(args[1], "-") {
			dir = args[1]
			smokeFlags.Parse(args[2:])
		} else {
			smokeFlags.Parse(args[1:])
		}

		return runSmoke(dir, *timeout)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh dag <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  install <dir>  Install Go/Bun requirements, duckdb in DIALTONE_ENV, and UI deps")
	fmt.Println("  fmt <dir>      Run go fmt")
	fmt.Println("  format <dir>   Run UI format checks")
	fmt.Println("  vet <dir>      Run go vet")
	fmt.Println("  go-build <dir> Run go build")
	fmt.Println("  lint <dir>     Run TypeScript lint checks")
	fmt.Println("  dev <dir>      Start Vite + debug browser attach")
	fmt.Println("  ui-run <dir>   Run UI dev server")
	fmt.Println("  serve <dir>    Run plugin Go server")
	fmt.Println("  build <dir>    Build UI assets")
	fmt.Println("  test <dir>     Run automated tests and write TEST.md artifacts")
	fmt.Println("  smoke <dir>    Run legacy smoke test")
	fmt.Println("  src --n <N>    Generate next src_vN folder")
	fmt.Println("\nDefault <dir> is the latest src_vN folder.")
	fmt.Println("\nExamples:")
	fmt.Println("  ./dialtone.sh dag test src_v3")
	fmt.Println("  ./dialtone.sh dag test src_v3 --attach")
	fmt.Println("  ./dialtone.sh dag dev src_v3")
	fmt.Println("  ./dialtone.sh dag src --n 5    # creates src/plugins/dag/src_v5")
}

func runSmoke(versionDir string, timeoutSec int) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	smokeFile := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "smoke", "smoke.go")
	if _, err := os.Stat(smokeFile); os.IsNotExist(err) {
		return fmt.Errorf("smoke test file not found: %s", smokeFile)
	}

	cmd := exec.Command(
		filepath.Join(cwd, "dialtone.sh"),
		"go", "exec", "run", smokeFile, versionDir, strconv.Itoa(timeoutSec),
	)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getLatestVersionDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "src_v1"
	}
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag")
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return "src_v1"
	}

	maxVer := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "src_v") {
			continue
		}
		version, err := strconv.Atoi(strings.TrimPrefix(name, "src_v"))
		if err != nil {
			continue
		}
		if version > maxVer {
			maxVer = version
		}
	}

	if maxVer == 0 {
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

func RunCreateVersion(newVer int) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "dag")

	entries, _ := os.ReadDir(pluginDir)
	maxVer := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "src_v") {
			ver, _ := strconv.Atoi(e.Name()[5:])
			if ver > maxVer {
				maxVer = ver
			}
		}
	}

	if maxVer == 0 {
		return fmt.Errorf("no existing src_vN folders found to clone from")
	}

	srcDir := filepath.Join(pluginDir, fmt.Sprintf("src_v%d", maxVer))
	destDir := filepath.Join(pluginDir, fmt.Sprintf("src_v%d", newVer))
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("version directory already exists: %s", destDir)
	}

	fmt.Printf(">> [DAG] Creating new version: src_v%d from src_v%d\n", newVer, maxVer)

	cmd := exec.Command("cp", "-r", srcDir, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	srcVersion := fmt.Sprintf("src_v%d", maxVer)
	destVersion := fmt.Sprintf("src_v%d", newVer)
	if err := rewriteVersionRefs(destDir, srcVersion, destVersion, maxVer, newVer); err != nil {
		return fmt.Errorf("failed to rewrite version references: %w", err)
	}

	fmt.Printf(">> [DAG] New version created at: %s\n", destDir)
	return nil
}

func rewriteVersionRefs(destDir, srcVersion, destVersion string, srcVerNum, destVerNum int) error {
	allowedExt := map[string]bool{
		".go":   true,
		".ts":   true,
		".tsx":  true,
		".js":   true,
		".jsx":  true,
		".css":  true,
		".html": true,
		".json": true,
		".md":   true,
		".txt":  true,
		".yml":  true,
		".yaml": true,
		".d.ts": true,
	}

	srcLabel := fmt.Sprintf("DAG v%d", srcVerNum)
	destLabel := fmt.Sprintf("DAG v%d", destVerNum)
	srcPackageLabel := fmt.Sprintf("dag-ui-v%d", srcVerNum)
	destPackageLabel := fmt.Sprintf("dag-ui-v%d", destVerNum)

	return filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "dist" || name == ".pixi" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !allowedExt[ext] {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(raw)
		updated := content
		updated = strings.ReplaceAll(updated, srcVersion, destVersion)
		updated = strings.ReplaceAll(updated, srcLabel, destLabel)
		updated = strings.ReplaceAll(updated, srcPackageLabel, destPackageLabel)

		if updated == content {
			return nil
		}
		return os.WriteFile(path, []byte(updated), 0644)
	})
}
