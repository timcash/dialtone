package cli

import (
	chrome_app "dialtone/dev/plugins/chrome/src_v1/go"
	test_v2 "dialtone/dev/plugins/test/src_v1/go"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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

	// Set default plugin dir if not already set by a wrapper
	if os.Getenv("DIALTONE_PLUGIN_DIR") == "" {
		cwd, _ := os.Getwd()
		os.Setenv("DIALTONE_PLUGIN_DIR", filepath.Join(cwd, "src", "plugins", "template"))
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
	case "test":
		dir := getDir()
		attach := false
		for _, arg := range args[2:] {
			if arg == "--attach" {
				attach = true
			}
		}
		return RunTest(dir, attach)
	case "build":
		return RunBuild(getDir())
	case "copy":
		if len(args) < 3 {
			return fmt.Errorf("usage: template copy <src_vN> <target_directory>")
		}
		return RunCopy(args[1], args[2])
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func RunTest(versionDir string, attach bool) error {
	cwd, _ := os.Getwd()
	testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "template", versionDir, "test"))
	if _, err := os.Stat(filepath.Join(cwd, "src", "plugins", "template", versionDir, "test", "main.go")); os.IsNotExist(err) {
		return fmt.Errorf("test runner not found: %s/main.go", testPkg)
	}

	baseURL := "http://127.0.0.1:8080"
	if attach {
		devSession, err := ensureTemplateDevServerAndHeadedBrowser(cwd, versionDir)
		if err != nil {
			return err
		}
		fmt.Printf(">> [TEMPLATE] Test: attach mode enabled (reusing headed dev browser session)\n")
		fmt.Printf(">> [TEMPLATE] Test: leaving dev preview running at http://127.0.0.1:%d after test completion\n", devSession.port)
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", devSession.port)
	}

	fmt.Printf(">> [TEMPLATE] Running Test Suite for %s...\n", versionDir)
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(
		os.Environ(),
		"TEMPLATE_TEST_ATTACH=0",
		"TEMPLATE_TEST_BASE_URL="+baseURL,
	)
	if attach {
		cmd.Env = append(cmd.Env, "TEMPLATE_TEST_ATTACH=1")
	}
	return cmd.Run()
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

func resolvePaths(versionDir string) (string, string) {
	cwd, _ := os.Getwd()

	// 1. Absolute path
	if filepath.IsAbs(versionDir) {
		return versionDir, filepath.Join(versionDir, "ui")
	}

	// 2. Relative to repo root
	if strings.HasPrefix(versionDir, "src/plugins/") {
		abs := filepath.Join(cwd, versionDir)
		return abs, filepath.Join(abs, "ui")
	}

	// 3. Relative to DIALTONE_PLUGIN_DIR (for when called from another plugin)
	pluginDir := os.Getenv("DIALTONE_PLUGIN_DIR")
	if pluginDir != "" {
		abs := filepath.Join(pluginDir, versionDir)
		if _, err := os.Stat(abs); err == nil {
			return abs, filepath.Join(abs, "ui")
		}
	}

	// 4. Default to template plugin
	pluginBase := filepath.Join(cwd, "src", "plugins", "template")
	versionDirPath := filepath.Join(pluginBase, versionDir)
	uiDir := filepath.Join(versionDirPath, "ui")

	return versionDirPath, uiDir
}

func RunFmt(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	versionDirPath, _ := resolvePaths(versionDir)
	relPath, _ := filepath.Rel(cwd, versionDirPath)

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./"+filepath.ToSlash(relPath)+"/...")
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
	_, uiDir := resolvePaths(versionDir)
	cmd := runBun(cwd, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	versionDirPath, _ := resolvePaths(versionDir)
	relPath, _ := filepath.Rel(cwd, versionDirPath)

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./"+filepath.ToSlash(relPath)+"/...")
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
	versionDirPath, _ := resolvePaths(versionDir)
	relPath, _ := filepath.Rel(cwd, versionDirPath)

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./"+filepath.ToSlash(relPath)+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Lint: %s\n", versionDir)

	cwd, _ := os.Getwd()
	_, uiDir := resolvePaths(versionDir)

	fmt.Println("   [LINT] Running tsc...")
	cmd := runBun(cwd, uiDir, "run", "lint")
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Serve: %s\n", versionDir)

	cwd, _ := os.Getwd()
	versionDirPath, _ := resolvePaths(versionDir)
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join(versionDirPath, "cmd", "main.go")))
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
	_, uiDir := resolvePaths(versionDir)
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunDev(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Dev: %s\n", versionDir)
	cwd, _ := os.Getwd()
	versionDirPath, uiDir := resolvePaths(versionDir)
	devPort := 3000
	if err := test_v2.WaitForPort(devPort, 400*time.Millisecond); err == nil {
		freePort, pickErr := test_v2.PickFreePort()
		if pickErr != nil {
			return fmt.Errorf("port %d is already in use and no free port could be picked: %w", devPort, pickErr)
		}
		fmt.Printf("   [DEV] Port %d is in use; using %d instead\n", devPort, freePort)
		devPort = freePort
	}
	devURL := fmt.Sprintf("http://127.0.0.1:%d", devPort)

	devSession, err := test_v2.NewDevSession(test_v2.DevSessionOptions{
		VersionDirPath: versionDirPath,
		Port:           devPort,
		URL:            devURL,
		ConsoleWriter:  os.Stdout,
		BrowserRole:    "template-dev",
	})
	if err != nil {
		return err
	}
	defer devSession.Close()

	fmt.Println("   [DEV] Running vite dev...")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(devPort), "--strictPort")
	cmd.Stdout = devSession.Writer()
	cmd.Stderr = devSession.Writer()
	if err := cmd.Start(); err != nil {
		return err
	}
	devSession.StartBrowserAttach()

	waitErr := cmd.Wait()
	return waitErr
}

type templateDevPreviewSession struct {
	port int
}

func ensureTemplateDevServerAndHeadedBrowser(repoRoot, versionDir string) (*templateDevPreviewSession, error) {
	_, uiDir := resolvePaths(versionDir)
	targetTitle, err := readHTMLTitle(filepath.Join(uiDir, "index.html"))
	if err != nil {
		return nil, err
	}

	port := 3000
	reuse := false
	if err := test_v2.WaitForPort(port, 800*time.Millisecond); err == nil {
		matched, probeErr := devServerMatchesVersion(port, targetTitle)
		if probeErr == nil && matched {
			reuse = true
			fmt.Printf(">> [TEMPLATE] Test: dev server already running for %s at http://127.0.0.1:%d\n", versionDir, port)
		} else {
			freePort, pickErr := test_v2.PickFreePort()
			if pickErr != nil {
				return nil, fmt.Errorf("dev server on %d is not %s and no free port could be picked: %w", port, versionDir, pickErr)
			}
			fmt.Printf(">> [TEMPLATE] Test: existing dev server on :%d is not %s; starting %s on :%d\n", port, versionDir, versionDir, freePort)
			port = freePort
		}
	}

	if !reuse {
		if err := startDetachedTemplateDevServer(repoRoot, versionDir, port); err != nil {
			return nil, err
		}
		if err := test_v2.WaitForPort(port, 30*time.Second); err != nil {
			return nil, fmt.Errorf("template dev server for %s did not become ready on :%d: %w", versionDir, port, err)
		}
		fmt.Printf(">> [TEMPLATE] Test: started dev server for %s at http://127.0.0.1:%d\n", versionDir, port)
	}

	previewURL := fmt.Sprintf("http://127.0.0.1:%d/#template-hero-stage", port)
	if err := openPersistentTemplateDevChrome(previewURL); err != nil {
		return nil, err
	}
	return &templateDevPreviewSession{port: port}, nil
}

func readHTMLTitle(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed reading %s: %w", path, err)
	}
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	m := re.FindSubmatch(raw)
	if len(m) < 2 {
		return "", fmt.Errorf("missing <title> in %s", path)
	}
	title := strings.TrimSpace(string(m[1]))
	if title == "" {
		return "", fmt.Errorf("empty <title> in %s", path)
	}
	return title, nil
}

func devServerMatchesVersion(port int, targetTitle string) (bool, error) {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}
	html := string(body)
	if strings.Contains(html, "<title>"+targetTitle+"</title>") || strings.Contains(html, targetTitle) {
		return true, nil
	}
	return false, nil
}

func startDetachedTemplateDevServer(repoRoot, versionDir string, port int) error {
	logDir := filepath.Join(repoRoot, ".dialtone", "run")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	logPath := filepath.Join(logDir, "template_dev_"+versionDir+".log")
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		filepath.Join(repoRoot, "dialtone.sh"),
		"template",
		"ui-run",
		versionDir,
		"--port",
		strconv.Itoa(port),
	)
	cmd.Dir = repoRoot
	cmd.Stdout = logf
	cmd.Stderr = logf
	if err := cmd.Start(); err != nil {
		_ = logf.Close()
		return err
	}
	_ = cmd.Process.Release()
	_ = logf.Close()
	return nil
}

func openPersistentTemplateDevChrome(url string) error {
	_, err := chrome_app.StartSession(chrome_app.SessionOptions{
		GPU:           true,
		Headless:      false,
		TargetURL:     url,
		Role:          "template-dev",
		ReuseExisting: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open template dev chrome preview at %s: %w", url, err)
	}
	return nil
}

func RunBuild(versionDir string) error {
	fmt.Printf(">> [TEMPLATE] Build: %s\n", versionDir)
	cwd, _ := os.Getwd()
	_, uiDir := resolvePaths(versionDir)

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

func copyDir(srcDir, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "dist" || d.Name() == ".pixi" || d.Name() == ".git" || d.Name() == ".chrome_data" || d.Name() == ".dialtone" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destDir, rel), 0755)
		}

		// Copy file
		input, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destDir, rel), input, 0644)
	})
}

func RunCopy(srcVersion, targetDir string) error {
	cwd, _ := os.Getwd()
	srcDir := filepath.Join(cwd, "src", "plugins", "template", srcVersion)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", srcDir)
	}

	destDir := targetDir
	if !filepath.IsAbs(destDir) {
		destDir = filepath.Join(cwd, targetDir)
	}

	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("target directory already exists: %s", destDir)
	}

	fmt.Printf(">> [TEMPLATE] Copying %s to %s...\n", srcVersion, targetDir)

	// Parse src version number
	srcVerNum := 0
	if strings.HasPrefix(srcVersion, "src_v") {
		srcVerNum, _ = strconv.Atoi(srcVersion[5:])
	}

	// Parse dest plugin name and version
	destPlugin := "template"
	destVersion := srcVersion
	destVerNum := srcVerNum

	// Example targetDir: src/plugins/my-plugin/src_v5
	absTarget, _ := filepath.Abs(destDir)
	relTarget, _ := filepath.Rel(cwd, absTarget)
	parts := strings.Split(filepath.ToSlash(relTarget), "/")

	// Check if it's in src/plugins/NAME/VERSION
	if len(parts) >= 3 && parts[0] == "src" && parts[1] == "plugins" {
		destPlugin = parts[2]
		if len(parts) >= 4 {
			destVersion = parts[3]
			if strings.HasPrefix(destVersion, "src_v") {
				destVerNum, _ = strconv.Atoi(destVersion[5:])
			}
		}
	}

	if err := copyDir(srcDir, destDir); err != nil {
		return err
	}

	if err := rewriteAllRefs(destDir, "template", destPlugin, srcVersion, destVersion, srcVerNum, destVerNum); err != nil {
		return fmt.Errorf("failed to rewrite references: %w", err)
	}

	fmt.Printf(">> [TEMPLATE] Successfully copied to: %s\n", destDir)
	return nil
}

func toTitle(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func rewriteAllRefs(destDir, srcPlugin, destPlugin, srcVersion, destVersion string, srcVerNum, destVerNum int) error {
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

	srcTemplateLabel := "Template"
	if srcVerNum > 0 {
		srcTemplateLabel = fmt.Sprintf("Template v%d", srcVerNum)
	}

	destPluginLabel := toTitle(destPlugin)
	if destVerNum > 0 {
		destPluginLabel = fmt.Sprintf("%s v%d", toTitle(destPlugin), destVerNum)
	}

	srcPackageLabel := fmt.Sprintf("%s-ui-%s", srcPlugin, strings.ReplaceAll(srcVersion, "src_", ""))
	destPackageLabel := fmt.Sprintf("%s-ui-%s", destPlugin, strings.ReplaceAll(destVersion, "src_", ""))

	// More replacements
	srcPluginPath := filepath.Join("src", "plugins", srcPlugin)
	destPluginPath := filepath.Join("src", "plugins", destPlugin)

	return filepath.WalkDir(destDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
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

		// Order matters: replace specific paths first
		updated = strings.ReplaceAll(updated, srcPluginPath, destPluginPath)
		updated = strings.ReplaceAll(updated, "/"+srcPlugin+"/", "/"+destPlugin+"/")
		updated = strings.ReplaceAll(updated, "\""+srcPlugin+"\"", "\""+destPlugin+"\"")
		updated = strings.ReplaceAll(updated, "\""+srcPlugin+"\",", "\""+destPlugin+"\",")
		updated = strings.ReplaceAll(updated, "\""+srcPlugin+" ", "\""+destPlugin+" ")
		updated = strings.ReplaceAll(updated, "\""+srcVersion+"\"", "\""+destVersion+"\"")
		updated = strings.ReplaceAll(updated, srcVersion, destVersion)
		updated = strings.ReplaceAll(updated, srcTemplateLabel, destPluginLabel)
		updated = strings.ReplaceAll(updated, srcPackageLabel, destPackageLabel)

		// Special case for Template Server in main.go
		updated = strings.ReplaceAll(updated, "Template Server", fmt.Sprintf("%s Server", toTitle(destPlugin)))

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
	fmt.Println("  test <dir> [--attach] Run automated tests and write TEST.md artifacts")
	fmt.Println("  copy <src_vN> <target_dir> Copy template version to target directory")
}
