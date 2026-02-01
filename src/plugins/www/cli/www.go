package cli

import (
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
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[www] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[www] FATAL: "+format+"\n", args...)
	os.Exit(1)
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
			fmt.Println("Usage: dialtone-dev www publish [options]")
			fmt.Println("\nOptions:")
			fmt.Println("  --help, -h         Show this help message")
			fmt.Println("\nAll other options are passed through to 'vercel deploy'.")
			fmt.Println("Example:")
			fmt.Println("  dialtone-dev www publish --debug")
			return
		}
	}

	nextVersion, err := bumpWwwVersion(webDir)
	if err != nil {
		logFatal("Failed to bump version: %v", err)
	}
	logInfo("Bumped version to v%s", nextVersion)
	logInfo("Building and deploying prebuilt output...")
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
	// Check if vercel CLI is available
	homeDir, _ := os.UserHomeDir()
	vercelPath := filepath.Join(homeDir, ".dialtone_env", "node", "bin", "vercel")
	if _, err := os.Stat(vercelPath); os.IsNotExist(err) {
		// Fallback to searching in PATH
		if p, err := exec.LookPath("vercel"); err == nil {
			vercelPath = p
		} else {
			logFatal("Vercel CLI not found. Run 'dialtone install' to install dependencies.")
		}
	}

	// Handle help explicitly
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Println("Usage: dialtone-dev www <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  publish            Deploy using prebuilt output")
		fmt.Println("  publish-prebuilt   Alias for publish")
		fmt.Println("  build              Build the project locally")
		fmt.Println("  dev                Start local development server")
		fmt.Println("  validate           Verify dialtone.earth version")
		fmt.Println("  check-version      Alias for validate")
		fmt.Println("  logs               View deployment logs")
		fmt.Println("  domain             Manage the dialtone.earth domain")
		fmt.Println("  login              Login to Vercel")
		fmt.Println("  test               Run WWW integration tests")
		fmt.Println("  test cad           Run headed browser test for CAD section")
		fmt.Println("  cad demo           Start CAD server + WWW dev + GPU Chrome")
		return
	}

	subcommand := args[0]
	// Determine the directory where the webpage code is located
	// All vercel commands run from webDir
	webDir := filepath.Join("src", "plugins", "www", "app")
	// Get project env vars (hardcoded for the app project serving dialtone.earth)
	vercelEnv := vercelProjectEnv()

	switch subcommand {
	case "cad":
		if len(args) > 1 && args[1] == "demo" {
			handleCadDemo(webDir)
			return
		}
		logFatal("Unknown 'cad' command. Use 'dialtone www help' for usage.")

	case "publish":
		publishPrebuilt(webDir, vercelPath, vercelEnv, args[1:])

	case "publish-prebuilt":
		publishPrebuilt(webDir, vercelPath, vercelEnv, args[1:])

	case "logs":
		if len(args) < 2 {
			logFatal("Usage: dialtone-dev www logs <deployment-url-or-id>\n   Run 'dialtone-dev www logs --help' for more info.")
		}
		vArgs := append([]string{"logs"}, args[1:]...)
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to show logs: %v", err)
		}

	case "domain":
		// Usage: dialtone-dev www domain [deployment-url]
		// If no deployment-url is given, it will attempt to alias the most recent deployment.
		vArgs := []string{"alias", "set"}
		vArgs = append(vArgs, args[1:]...)
		vArgs = append(vArgs, "dialtone.earth")
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to set domain alias: %v", err)
		}

	case "login":
		cmd := exec.Command(vercelPath, "login")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to login: %v", err)
		}

	case "dev":
		// Run 'npm run dev' (which runs vite)
		logInfo("Starting local development server...")
		cmd := exec.Command("npm", "run", "dev")
		cmd.Dir = webDir // Keep running in webDir for NPM
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Dev server failed: %v", err)
		}

		if err := cmd.Run(); err != nil {
			logFatal("Tests failed: %v", err)
		}

	case "test":
		if len(args) > 1 && args[1] == "cad" {
			logInfo("Running headed CAD test...")
			cmd := exec.Command("./dialtone.sh", "test", "plugin", "www-cad")
			// Check for --live flag
			for _, arg := range args[2:] {
				if arg == "--live" {
					logInfo("  Using live backend (CAD_LIVE=true)")
					cmd.Env = append(os.Environ(), "CAD_LIVE=true")
					break
				}
			}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				logFatal("Headed CAD test failed: %v", err)
			}
			return
		}
		logInfo("Running integration tests...")
		cmd := exec.Command("./dialtone.sh", "test", "plugin", "www")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logFatal("Tests failed: %v", err)
		}

	case "build":
		// Run 'npm run build' (which runs vite build)
		logInfo("Building project...")
		cmd := exec.Command("npm", "run", "build")
		cmd.Dir = webDir // Keep running in webDir for NPM
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Build failed: %v", err)
		}

	case "check-version", "validate":
		expected, err := expectedVersion(webDir)
		if err != nil {
			logFatal("Failed to read expected version: %v", err)
		}
		actual, err := fetchDialtoneVersion()
		if err != nil {
			logFatal("Failed to fetch site version: %v", err)
		}
		if normalizeVersion(actual) != normalizeVersion(expected) {
			logFatal("Version mismatch: site=%s expected=v%s", actual, expected)
		}
		logInfo("Version OK: site=%s expected=v%s", actual, expected)

	default:
		// Generic pass-through to vercel CLI
		logInfo("Running: vercel %s %s", subcommand, strings.Join(args[1:], " "))
		vArgs := append([]string{subcommand}, args[1:]...)
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = webDir
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Vercel command failed: %v", err)
		}
	}
}

func handleCadDemo(webDir string) {
	logInfo("Setting up CAD Demo Environment...")

	// 1. Aggressive Port Cleanup
	logInfo("Cleaning up ports 5173 and 8081...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	_ = exec.Command("fuser", "-k", "8081/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = exec.Command("./dialtone.sh", "chrome", "kill", "all").Run()

	// 3. Start CAD Server (Background)
	logInfo("Starting CAD Server...")
	cadCmd := exec.Command("./dialtone.sh", "cad", "server")
	cadCmd.Stdout = os.Stdout
	cadCmd.Stderr = os.Stderr
	if err := cadCmd.Start(); err != nil {
		logFatal("Failed to start CAD server: %v", err)
	}

	// Wait for cad to be alive
	logInfo("Waiting for CAD Server...")
	cadReady := false
	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		r, err := http.Get("http://127.0.0.1:8081/api/cad")
		if err == nil {
			r.Body.Close()
			cadReady = true
			break
		}
	}
	if !cadReady {
		logFatal("CAD server failed to respond within 10 seconds")
	}

	// 4. Start WWW Dev Server (Background)
	logInfo("Starting WWW Dev Server...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	devCmd.Stdout = os.Stdout
	devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}

	// 4. Wait for dev server to be ready
	logInfo("Waiting for Dev Server...")
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get("http://127.0.0.1:5173")
		if err == nil && resp.StatusCode == 200 {
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	// 5. Launch GPU-enabled Chrome
	logInfo("Launching GPU-enabled Chrome...")
	chromeCmd := exec.Command("./dialtone.sh", "chrome", "new", "http://127.0.0.1:5173/#s-cad", "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		logFatal("Failed to launch Chrome: %v", err)
	}

	logInfo("CAD Demo Environment is LIVE!")
	logInfo("Dev Server: http://127.0.0.1:5173/#s-cad")
	logInfo("CAD Server: http://127.0.0.1:8081")
	logInfo("Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}
