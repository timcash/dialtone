package cli

import (
	test_v2 "dialtone/cli/src/libs/test_v2"
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

	// Helper to get directory with latest default
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
		return RunUIRun(getDir(), args[2:])
	case "serve":
		return RunServe(getDir())
	case "smoke":
		dir := getDir()
		cwd, _ := os.Getwd()
		smokeFile := filepath.Join(cwd, "src", "plugins", "template", dir, "smoke", "smoke.go")
		if _, err := os.Stat(smokeFile); os.IsNotExist(err) {
			return fmt.Errorf("smoke test file not found: %s", smokeFile)
		}

		fmt.Printf(">> [TEMPLATE] Running Smoke Test for %s...\n", dir)
		cmd := exec.Command("go", "run", smokeFile, dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "test":
		dir := getDir()
		cwd, _ := os.Getwd()
		testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "template", dir, "test"))
		if _, err := os.Stat(filepath.Join(cwd, "src", "plugins", "template", dir, "test", "main.go")); os.IsNotExist(err) {
			return fmt.Errorf("test runner not found: %s/main.go", testPkg)
		}
		fmt.Printf(">> [TEMPLATE] Running Test Suite for %s...\n", dir)
		cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
		cmd.Dir = cwd
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "build":
		return RunBuild(getDir())
	case "src":
		n := 0
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			n, _ = strconv.Atoi(args[1])
		} else {
			srcFlags := flag.NewFlagSet("template src", flag.ExitOnError)
			nFlag := srcFlags.Int("n", 0, "Version number to create")
			srcFlags.Parse(args[1:])
			n = *nFlag
		}

		if n == 0 {
			return fmt.Errorf("usage: template src <N> or template src --n <N>")
		}
		return RunCreateVersion(n)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func getLatestVersionDir() string {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "template")
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
		return "src_v1"
	}
	return fmt.Sprintf("src_v%d", maxVer)
}

func RunInstall(versionDir string) error {
	return runTemplateInstall(versionDir)
}

func RunFmt(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/template/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Format: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/template/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Go Build: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/template/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Lint: %s\n", versionDir)

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")

	fmt.Println("   [LINT] Running tsc...")
	cmd := runBun(cwd, uiDir, "run", "lint")
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Serve: %s\n", versionDir)

	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "template", versionDir, "cmd", "main.go")))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [TEMPLATE] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunDev(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Dev: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")
	versionDirPath := filepath.Join(cwd, "src", "plugins", "template", versionDir)
	devPort := 3000
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	devSession, err := test_v2.NewDevSession(test_v2.DevSessionOptions{
		VersionDirPath: versionDirPath,
		Port:           devPort,
		URL:            devURL,
		ConsoleWriter:  os.Stdout,
	})
	if err != nil {
		return err
	}
	defer devSession.Close()

	fmt.Println("   [DEV] Running vite dev...")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort))
	cmd.Stdout = devSession.Writer()
	cmd.Stderr = devSession.Writer()
	if err := cmd.Start(); err != nil {
		return err
	}
	devSession.StartBrowserAttach()

	waitErr := cmd.Wait()
	return waitErr
}

func RunBuild(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "template", versionDir, "ui")

	if err := RunInstall(versionDir); err != nil {
		return err
	}

	fmt.Println("   [BUILD] Running UI build...")
	cmd := runBun(cwd, uiDir, "run", "build")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %v", err)
	}

	fmt.Println(">> [TEMPLATE] Build successful")
	return nil
}

func RunCreateVersion(newVer int) error {
	cwd, _ := os.Getwd()
	pluginDir := filepath.Join(cwd, "src", "plugins", "template")

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

	fmt.Printf(">> [TEMPLATE] Creating new version: src_v%d from src_v%d\n", newVer, maxVer)

	// Simple copy using cp -r
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

	fmt.Printf(">> [TEMPLATE] New version created at: %s\n", destDir)
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

	srcTemplateLabel := fmt.Sprintf("Template v%d", srcVerNum)
	destTemplateLabel := fmt.Sprintf("Template v%d", destVerNum)
	srcPackageLabel := fmt.Sprintf("template-ui-v%d", srcVerNum)
	destPackageLabel := fmt.Sprintf("template-ui-v%d", destVerNum)

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
		updated = strings.ReplaceAll(updated, srcTemplateLabel, destTemplateLabel)
		updated = strings.ReplaceAll(updated, srcPackageLabel, destPackageLabel)

		if updated == content {
			return nil
		}
		return os.WriteFile(path, []byte(updated), 0644)
	})
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh template <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  install <dir>  Install UI dependencies")
	fmt.Println("  fmt <dir>      Run formatting checks/fixes")
	fmt.Println("  format <dir>   Run UI format checks")
	fmt.Println("  vet <dir>      Run go vet checks")
	fmt.Println("  go-build <dir> Run go build checks")
	fmt.Println("  lint <dir>     Run lint checks")
	fmt.Println("  dev <dir>      Run UI in development mode")
	fmt.Println("  ui-run <dir>   Run UI dev server")
	fmt.Println("  serve <dir>    Run plugin Go server")
	fmt.Println("  build <dir>    Build everything needed (UI assets)")
	fmt.Println("  test <dir>     Run automated tests and write TEST.md artifacts")
	fmt.Println("  smoke <dir>    Run robust automated UI tests")
	fmt.Println("  src --n <N>    Generate next src_vN folder")
}
