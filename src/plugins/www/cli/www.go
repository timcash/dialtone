package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dialtone/dev/browser"
	chrome "dialtone/dev/plugins/chrome/app"
	wwwtest "dialtone/dev/plugins/www/test"

	"github.com/chromedp/chromedp"
	stdruntime "runtime"
)

func getDialtoneCmd(args ...string) *exec.Cmd {
	if stdruntime.GOOS == "windows" {
		return exec.Command("powershell", append([]string{"-ExecutionPolicy", "Bypass", "-File", ".\\dialtone.ps1"}, args...)...)
	}
	return exec.Command("./dialtone.sh", args...)
}

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[www] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[www] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

func dialtoneBunPath() (string, bool) {
	envDir := os.Getenv("DIALTONE_ENV")
	if envDir == "" {
		return "", false
	}
	candidates := []string{
		filepath.Join(envDir, "bun", "bin", "bun"),
		filepath.Join(envDir, "bin", "bun"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func dialtoneNpmPath() (string, bool) {
	envDir := os.Getenv("DIALTONE_ENV")
	if envDir == "" {
		return "", false
	}
	candidate := filepath.Join(envDir, "node", "bin", "npm")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, true
	}
	return "", false
}

func installWwwDeps(webDir string) {
	if bunPath, ok := dialtoneBunPath(); ok {
		logInfo("Installing dependencies with DIALTONE_ENV bun: %s", bunPath)
		cmd := exec.Command(bunPath, "install")
		cmd.Dir = webDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("bun install failed: %v", err)
		}
		return
	}

	if npmPath, ok := dialtoneNpmPath(); ok {
		logInfo("Installing dependencies with DIALTONE_ENV npm: %s", npmPath)
		cmd := exec.Command(npmPath, "install")
		cmd.Dir = webDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("npm install failed: %v", err)
		}
		return
	}

	logFatal("Neither Bun nor npm found in DIALTONE_ENV. Run './dialtone.sh install' first.")
}

type npmPackage struct {
	Version string `json:"version"`
}

// Vercel project configuration for dialtone.earth (app project)
const (
	vercelProjectID = "prj_vynjSZFIhD8TlR8oOyuXTKjFUQxM"
	vercelOrgID     = "team_4tzswM6M6PoDxaszH2ZHs5J7"
)

func vercelProjectEnv() []string {
	return []string{
		"VERCEL_PROJECT_ID=" + vercelProjectID,
		"VERCEL_ORG_ID=" + vercelOrgID,
	}
}

func expectedVersion(webDir string) (string, error) {
	packagePath := filepath.Join(webDir, "package.json")
	data, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", packagePath, err)
	}

	var pkg npmPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("parse %s: %w", packagePath, err)
	}
	if pkg.Version == "" {
		return "", fmt.Errorf("missing version in %s", packagePath)
	}

	return pkg.Version, nil
}

func fetchDialtoneVersion() (string, error) {
	client := &http.Client{
		Timeout: 12 * time.Second,
	}
	req, err := http.NewRequest(http.MethodGet, "https://dialtone.earth", nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "dialtone-cli/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch dialtone.earth: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	re := regexp.MustCompile(`class="version">\s*([^<\s]+)\s*<`)
	match := re.FindSubmatch(body)
	if len(match) < 2 {
		return "", fmt.Errorf("version tag not found")
	}

	return strings.TrimSpace(string(match[1])), nil
}

func normalizeVersion(raw string) string {
	if strings.HasPrefix(raw, "v") {
		return raw[1:]
	}
	return raw
}

func ensureNpmDeps(webDir string) {
	tscPath := filepath.Join(webDir, "node_modules", ".bin", "tsc")
	if _, err := os.Stat(tscPath); err == nil {
		return
	}

	installWwwDeps(webDir)
}

func bumpPatch(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("unsupported version format: %s", version)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version: %s", parts[2])
	}
	parts[2] = strconv.Itoa(patch + 1)
	return strings.Join(parts, "."), nil
}

func bumpWwwVersion(webDir string) (string, error) {
	packagePath := filepath.Join(webDir, "package.json")
	packageData, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", packagePath, err)
	}

	versionRe := regexp.MustCompile(`"version"\s*:\s*"([^"]+)"`)
	versionMatch := versionRe.FindSubmatch(packageData)
	if len(versionMatch) < 2 {
		return "", fmt.Errorf("version not found in %s", packagePath)
	}

	currentVersion := string(versionMatch[1])
	nextVersion, err := bumpPatch(currentVersion)
	if err != nil {
		return "", err
	}

	updatedPackage := versionRe.ReplaceAll(packageData, []byte(`"version": "`+nextVersion+`"`))
	if err := os.WriteFile(packagePath, updatedPackage, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", packagePath, err)
	}

	indexPath := filepath.Join(webDir, "index.html")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", indexPath, err)
	}

	versionTagRe := regexp.MustCompile(`(<p class="version">)\s*v?[^<]*\s*(</p>)`)
	if !versionTagRe.Match(indexData) {
		return "", fmt.Errorf("version tag not found in %s", indexPath)
	}
	updatedIndex := versionTagRe.ReplaceAll(indexData, []byte("${1}v"+nextVersion+"${2}"))
	if err := os.WriteFile(indexPath, updatedIndex, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", indexPath, err)
	}

	return nextVersion, nil
}

func publishPrebuilt(webDir string, vercelPath string, vercelEnv []string, args []string) {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			fmt.Println("Usage: dialtone www publish [options]")
			fmt.Println("\nThis command performs a full production deployment pipeline:")
			fmt.Println("  1. Versioning: Bumps patch version in package.json & index.html")
			fmt.Println("  2. Building:   Runs 'npm run build' for optimized web assets")
			fmt.Println("  3. Prebuilding: Runs 'vercel build --prod' for edge compatibility")
			fmt.Println("  4. Deploying:  Runs 'vercel deploy --prebuilt --prod' to dialtone.earth")
			fmt.Println("\nOptions:")
			fmt.Println("  --help, -h         Show this help message")
			fmt.Println("\nArguments:")
			fmt.Println("  Extra arguments are passed directly to the 'vercel deploy' command.")
			fmt.Println("  Examples: --debug, --force, --no-wait, --token <T>")
			fmt.Println("\nDemo usage:")
			fmt.Println("  dialtone www publish --debug")
			return
		}
	}

	nextVersion, err := bumpWwwVersion(webDir)
	if err != nil {
		logFatal("Failed to bump version: %v", err)
	}
	logInfo("Bumped version to v%s", nextVersion)
	logInfo("Building and deploying prebuilt output...")
	ensureNpmDeps(webDir)
	buildCmd := exec.Command("npm", "run", "build")
	buildCmd.Dir = webDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdin = os.Stdin
	if err := buildCmd.Run(); err != nil {
		logFatal("Build failed: %v", err)
	}

	prebuildCmd := exec.Command(vercelPath, "build", "--prod")
	prebuildCmd.Dir = webDir
	prebuildCmd.Env = append(os.Environ(), vercelEnv...)
	prebuildCmd.Stdout = os.Stdout
	prebuildCmd.Stderr = os.Stderr
	prebuildCmd.Stdin = os.Stdin
	if err := prebuildCmd.Run(); err != nil {
		logFatal("Vercel build failed: %v", err)
	}

	vArgs := append([]string{"deploy", "--prebuilt", "--prod"}, args...)
	cmd := exec.Command(vercelPath, vArgs...)
	cmd.Dir = webDir
	cmd.Env = append(os.Environ(), vercelEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		logFatal("Failed to deploy prebuilt output: %v", err)
	}
	logInfo("Deployment successful!")
}

// RunWww handles 'www <subcommand>'
func RunWww(args []string) {
	// Lazy-load Vercel CLI path
	homeDir, _ := os.UserHomeDir()
	defaultVercelPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "vercel")

	getVercel := func() string {
		if _, err := os.Stat(defaultVercelPath); os.IsNotExist(err) {
			// Fallback to searching in PATH
			if p, err := exec.LookPath("vercel"); err == nil {
				return p
			}
			logFatal("Vercel CLI not found. Run 'dialtone install' to install dependencies.")
		}
		return defaultVercelPath
	}

	// Handle help explicitly
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: dialtone www <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  publish            Full deployment pipeline (version -> build -> deploy)")
		fmt.Println("  install            Install WWW dependencies using DIALTONE_ENV bun/npm")
		fmt.Println("  build              Vite build (generates /dist)")
		fmt.Println("  dev                Vite dev server (hot reload)")
		fmt.Println("  lint               Run static analysis (tsc --noEmit)")
		fmt.Println("  validate           Check live dialtone.earth version vs local pkg")
		fmt.Println("  logs <id|url>      Fetch Vercel deployment logs")
		fmt.Println("  domain             Alias production to dialtone.earth")
		fmt.Println("  login              Authenticate with Vercel CLI")
		fmt.Println("  test               Run standard WWW integration tests")
		fmt.Println("  smoke              Quick smoke pass across all sections")
		fmt.Println("  smoke-chrome       Verify Chrome browser reaping lifecycle")
		fmt.Println("  test cad [--live]  Run headed browser tests for CAD generator")
		fmt.Println("  about demo         Zero-config local About section demo")
		fmt.Println("  cad demo           Zero-config local CAD development environment")
		fmt.Println("  earth demo         Zero-config local Earth development environment")
		fmt.Println("  music demo         Zero-config local Music section demo")
		fmt.Println("  webgpu demo        Zero-config local WebGPU development environment")
		fmt.Println("  webgpu debug       Chromedp debug run for WebGPU rendering")
		fmt.Println("  policy demo        Zero-config local Policy section demo")
		fmt.Println("  radio demo         Zero-config local Radio section demo")
		fmt.Println("  vision demo        Zero-config local Vision section demo")
		fmt.Println("\nRun 'dialtone www <subcommand> --help' for specific details.")
		return
	}

	subcommand := args[0]
	webDir := filepath.Join("src", "plugins", "www", "app")
	vercelEnv := vercelProjectEnv()

	switch subcommand {
	case "vision":
		handleDemo(webDir, "vision", "#s-vision")
	case "about":
		handleDemo(webDir, "about", "#s-about")
	case "cad":
		if len(args) > 1 && args[1] == "demo" {
			handleCadDemo(webDir)
			return
		}
		logFatal("Unknown 'cad' command.")
	case "earth":
		handleDemo(webDir, "earth", "#s-home")
	case "music":
		handleDemo(webDir, "music", "#s-music")
	case "webgpu":
		if len(args) > 1 && args[1] == "demo" {
			handleDemo(webDir, "webgpu", "#s-webgpu-template")
			return
		}
		if len(args) > 1 && args[1] == "debug" {
			handleWebgpuDebug(webDir)
			return
		}
		logFatal("Unknown 'webgpu' command.")
	case "radio":
		handleDemo(webDir, "radio", "#s-radio")
	case "policy":
		handleDemo(webDir, "policy", "#s-policy")
	case "test":
		logInfo("Running integration tests...")
		if err := wwwtest.RunAll(); err != nil {
			logFatal("Tests failed: %v", err)
		}
	case "publish":
		publishPrebuilt(webDir, getVercel(), vercelEnv, args[1:])
	case "smoke":
		logInfo("Running comprehensive smoke tests...")
		if err := wwwtest.RunComprehensiveSmoke(); err != nil {
			logFatal("Smoke tests failed: %v", err)
		}
	case "smoke-chrome":
		logInfo("Running Chrome lifecycle smoke test...")
		if err := wwwtest.RunSmokeChromeLifecycleTest(); err != nil {
			logFatal("Chrome lifecycle test failed: %v", err)
		}
	case "dev":
		logInfo("Starting local development server...")
		cmd := exec.Command("npm", "run", "dev"); cmd.Dir = webDir
		cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr; cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil { logFatal("Dev server failed: %v", err) }
	default:
		// Pass-through to Vercel...
		vArgs := append([]string{subcommand}, args[1:]...)
		cmd := exec.Command(getVercel(), vArgs...); cmd.Dir = webDir
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr; cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil { logFatal("Vercel command failed: %v", err) }
	}
}

func handleDemo(webDir, name, anchor string) {
	logInfo("Setting up %s Demo Environment...", name)
	_ = browser.CleanupPort(5173)
	chrome.KillDialtoneResources()

	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	stdout, _ := devCmd.StdoutPipe(); stderr, _ := devCmd.StderrPipe()
	if err := devCmd.Start(); err != nil { logFatal("Failed to start dev server: %v", err) }

	port := 5173
	portCh := make(chan int, 1)
	go func() {
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		re := regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/`)
		for scanner.Scan() {
			line := scanner.Text(); fmt.Println(line)
			if match := re.FindStringSubmatch(line); len(match) == 2 {
				if p, _ := strconv.Atoi(match[1]); p > 0 { select { case portCh <- p: default: } }
			}
		}
	}()

	select {
	case detected := <-portCh: port = detected
	case <-time.After(10 * time.Second): logInfo("Timeout detecting port, using default %d", port)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/%s", port, anchor)
	session, err := chrome.StartSession(chrome.SessionOptions{Role: "demo", Headless: false, GPU: true, TargetURL: baseURL})
	if err != nil { logFatal("Failed to start chrome: %v", err) }
	defer chrome.CleanupSession(session)

	logInfo("%s Demo is LIVE at %s", strings.ToUpper(name), baseURL)
	sigChan := make(chan os.Signal, 1); signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}

func handleCadDemo(webDir string) {
	logInfo("Setting up CAD Demo Environment...")
	_ = browser.CleanupPort(5173); _ = browser.CleanupPort(8081)
	chrome.KillDialtoneResources()

	cadCmd := getDialtoneCmd("cad", "server")
	if err := cadCmd.Start(); err != nil { logFatal("Failed to start CAD server: %v", err) }

	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	if err := devCmd.Start(); err != nil { logFatal("Failed to start dev server: %v", err) }

	// Wait for servers
	time.Sleep(5 * time.Second)

	baseURL := "http://127.0.0.1:5173/#s-cad"
	session, err := chrome.StartSession(chrome.SessionOptions{Role: "demo", Headless: false, GPU: true, TargetURL: baseURL})
	if err != nil { logFatal("Failed to start chrome: %v", err) }
	defer chrome.CleanupSession(session)

	logInfo("CAD Demo is LIVE at %s", baseURL)
	sigChan := make(chan os.Signal, 1); signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
}

func handleWebgpuDebug(webDir string) {
	logInfo("Setting up WebGPU Debug Environment...")
	_ = browser.CleanupPort(5173)
	chrome.KillDialtoneResources()

	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	if err := devCmd.Start(); err != nil { logFatal("Failed to start dev server: %v", err) }
	time.Sleep(5 * time.Second)

	chromePath := browser.FindChromePath()
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-unsafe-webgpu", true),
		chromedp.Flag("dialtone-origin", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()
	defer chrome.KillDialtoneResources()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173/#s-webgpu-template"),
		chromedp.WaitReady("#webgpu-template-container"),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil { logInfo("Debug failed: %v", err) }
}
