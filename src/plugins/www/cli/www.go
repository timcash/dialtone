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

	"dialtone/cli/src/core/browser"

	"github.com/chromedp/cdproto/runtime"
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
	// Determine the directory where the webpage code is located
	// All vercel commands run from webDir
	webDir := filepath.Join("src", "plugins", "www", "app")
	// Get project env vars (hardcoded for the app project serving dialtone.earth)
	vercelEnv := vercelProjectEnv()

	switch subcommand {
	case "vision":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www vision demo")
					fmt.Println("\nOrchestrates a local Vision section demo:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome on the Vision section (#s-vision).")
					return
				}
			}
			handleVisionDemo(webDir)
			return
		}
		logFatal("Unknown 'vision' command. Use 'dialtone www help' for usage.")
	case "about":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www about demo")
					fmt.Println("\nOrchestrates a local About section demo:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome on the About section (#s-about).")
					return
				}
			}
			handleAboutDemo(webDir)
			return
		}
		logFatal("Unknown 'about' command. Use 'dialtone www help' for usage.")
	case "cad":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www cad demo")
					fmt.Println("\nOrchestrates a full local CAD development environment:")
					fmt.Println("  1. Cleans up ports 5173 and 8081.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Go CAD server.")
					fmt.Println("  4. Starts the Vite WWW dev server.")
					fmt.Println("  5. Launches Chrome with GPU acceleration on the CAD section.")
					return
				}
			}
			handleCadDemo(webDir)
			return
		}
		logFatal("Unknown 'cad' command. Use 'dialtone www help' for usage.")
	case "earth":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www earth demo")
					fmt.Println("\nOrchestrates a full local Earth development environment:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome with GPU acceleration on the Earth section.")
					return
				}
			}
			handleEarthDemo(webDir)
			return
		}
		logFatal("Unknown 'earth' command. Use 'dialtone www help' for usage.")
	case "music":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www music demo")
					fmt.Println("\nOrchestrates a local Music section demo:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server on 0.0.0.0.")
					fmt.Println("  4. Launches Chrome on the Music section (#s-music).")
					return
				}
			}
			handleMusicDemo(webDir)
			return
		}
		logFatal("Unknown 'music' command. Use 'dialtone www help' for usage.")
	case "webgpu":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www webgpu demo")
					fmt.Println("\nOrchestrates a full local WebGPU development environment:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome with GPU acceleration on the WebGPU section.")
					return
				}
			}
			handleWebgpuDemo(webDir)
			return
		}
		if len(args) > 1 && args[1] == "debug" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www webgpu debug")
					fmt.Println("\nRuns a Chromedp debug pass for the WebGPU section:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome with WebGPU flags.")
					fmt.Println("  5. Captures WebGPU availability, canvas sizing, and console logs.")
					return
				}
			}
			handleWebgpuDebug(webDir)
			return
		}
		logFatal("Unknown 'webgpu' command. Use 'dialtone www help' for usage.")

	case "radio":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www radio demo")
					fmt.Println("\nOrchestrates a local Radio section demo:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome on the Radio section (#s-radio).")
					return
				}
			}
			handleRadioDemo(webDir)
			return
		}
		logFatal("Unknown 'radio' command. Use 'dialtone www help' for usage.")

	case "policy":
		if len(args) > 1 && args[1] == "demo" {
			for _, arg := range args[2:] {
				if arg == "--help" || arg == "-h" {
					fmt.Println("Usage: dialtone www policy demo")
					fmt.Println("\nOrchestrates a local Policy section demo:")
					fmt.Println("  1. Cleans up port 5173.")
					fmt.Println("  2. Kills existing Chrome debug instances.")
					fmt.Println("  3. Starts the Vite WWW dev server.")
					fmt.Println("  4. Launches Chrome on the Policy section (#s-policy).")
					return
				}
			}
			handlePolicyDemo(webDir)
			return
		}
		logFatal("Unknown 'policy' command. Use 'dialtone www help' for usage.")

	case "publish":
		publishPrebuilt(webDir, getVercel(), vercelEnv, args[1:])

	case "publish-prebuilt":
		publishPrebuilt(webDir, getVercel(), vercelEnv, args[1:])

	case "install":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www install")
				fmt.Println("\nInstalls dependencies for the WWW app.")
				fmt.Println("Prefers Bun from DIALTONE_ENV, then npm from DIALTONE_ENV.")
				return
			}
		}
		installWwwDeps(webDir)

	case "logs":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www logs <deployment-url-or-id>")
				fmt.Println("\nFetches the runtime logs for a specific Vercel deployment.")
				return
			}
		}
		if len(args) < 2 {
			logFatal("Usage: dialtone www logs <deployment-url-or-id>\n   Run 'dialtone www logs --help' for more info.")
		}
		vArgs := append([]string{"logs"}, args[1:]...)
		cmd := exec.Command(getVercel(), vArgs...)
		cmd.Dir = webDir
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to show logs: %v", err)
		}

	case "domain":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www domain [deployment-url]")
				fmt.Println("\nAliases a deployment URL to 'dialtone.earth'.")
				fmt.Println("If no URL is provided, it aliases the most recent deployment.")
				return
			}
		}
		// Usage: dialtone www domain [deployment-url]
		// If no deployment-url is given, it will attempt to alias the most recent deployment.
		vArgs := []string{"alias", "set"}
		vArgs = append(vArgs, args[1:]...)
		vArgs = append(vArgs, "dialtone.earth")
		cmd := exec.Command(getVercel(), vArgs...)
		cmd.Dir = webDir
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to set domain alias: %v", err)
		}

	case "login":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www login")
				fmt.Println("\nAuthenticates the local CLI with your Vercel account.")
				return
			}
		}
		cmd := exec.Command(getVercel(), "login")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Failed to login: %v", err)
		}

	case "dev":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www dev")
				fmt.Println("\nStarts the local Vite development server with hot module replacement.")
				fmt.Println("The server typically runs at http://127.0.0.1:5173")
				return
			}
		}
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

	case "lint":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www lint")
				fmt.Println("\nRuns static analysis (TypeScript type-checking) on the codebase.")
				return
			}
		}
		// Run 'npm run lint'
		logInfo("Running static analysis...")
		ensureNpmDeps(webDir)
		cmd := exec.Command("npm", "run", "lint")
		cmd.Dir = webDir // Keep running in webDir for NPM
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Lint check failed: %v", err)
		}

	case "test":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www test [cad] [--live]")
				fmt.Println("\nSubcommands:")
				fmt.Println("  (default)          Run all standard integration tests")
				fmt.Println("  cad                Run headed browser tests for the CAD flow")
				fmt.Println("\nFlags for 'test cad':")
				fmt.Println("  --live             Use the production CAD backend instead of local simulator")
				return
			}
		}
		if len(args) > 1 && args[1] == "cad" {
			logInfo("Running headed CAD test...")
			cmd := getDialtoneCmd("test", "plugin", "www-cad")
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
		cmd := getDialtoneCmd("test", "plugin", "www")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logFatal("Tests failed: %v", err)
		}

	case "smoke":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www smoke")
				fmt.Println("\nRuns a comprehensive smoke pass (performance + menu) across all WWW sections.")
				fmt.Println("Uses a single shared browser instance for efficiency.")
				return
			}
		}
		logInfo("Running comprehensive smoke tests...")
		cmd := getDialtoneCmd("test", "tags", "comprehensive")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			logFatal("Smoke tests failed: %v", err)
		}

	case "build":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www build")
				fmt.Println("\nCompiles the TypeScript and Vite project into static assets.")
				fmt.Println("Outputs are generated in: src/plugins/www/app/dist")
				return
			}
		}
		// Run 'npm run build' (which runs vite build)
		logInfo("Building project...")
		ensureNpmDeps(webDir)
		cmd := exec.Command("npm", "run", "build")
		cmd.Dir = webDir // Keep running in webDir for NPM
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Build failed: %v", err)
		}

	case "check-version", "validate":
		for _, arg := range args[1:] {
			if arg == "--help" || arg == "-h" {
				fmt.Println("Usage: dialtone www validate")
				fmt.Println("\nCompares the version on dialtone.earth with the local package.json.")
				return
			}
		}
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
		cmd := exec.Command(getVercel(), vArgs...)
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
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start CAD Server (Background)
	logInfo("Starting CAD Server...")
	cadCmd := getDialtoneCmd("cad", "server")
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
	chromeCmd := getDialtoneCmd("chrome", "new", "http://127.0.0.1:5173/#s-cad", "--gpu")
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

func handleWebgpuDemo(webDir string) {
	logInfo("Setting up WebGPU Demo Environment...")

	// 1. Aggressive Port Cleanup
	logInfo("Cleaning up port 5173...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server (Background)
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

	// 5. Launch GPU-enabled Chrome on WebGPU section
	logInfo("Launching GPU-enabled Chrome...")
	chromeCmd := getDialtoneCmd("chrome", "new", "http://127.0.0.1:5173/#s-webgpu-template", "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		logFatal("Failed to launch Chrome: %v", err)
	}

	logInfo("WebGPU Demo Environment is LIVE!")
	logInfo("Dev Server: http://127.0.0.1:5173/#s-webgpu-template")
	logInfo("Press Ctrl+C to stop...")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}

func handleAboutDemo(webDir string) {
	logInfo("Setting up About Demo Environment...")

	// 1. Port cleanup (same as earth demo)
	logInfo("Cleaning up port 5173...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server with piped output to detect port (same pattern as earth demo)
	logInfo("Starting WWW Dev Server...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	stdout, err := devCmd.StdoutPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stdout: %v", err)
	}
	stderr, err := devCmd.StderrPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stderr: %v", err)
	}
	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}

	// 4. Wait for dev server ready and detect port (like earth_demo.go)
	logInfo("Waiting for Dev Server...")
	port := 5173
	portCh := make(chan int, 1)
	go func() {
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		re := regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/`)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if match := re.FindStringSubmatch(line); len(match) == 2 {
				if p, err := strconv.Atoi(match[1]); err == nil {
					select {
					case portCh <- p:
					default:
					}
				}
			}
		}
	}()

	select {
	case detected := <-portCh:
		port = detected
	case <-time.After(10 * time.Second):
		logInfo("Dev server port not detected yet; falling back to %d", port)
	}

	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	// 5. Launch Chrome straight on About section
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/#s-about", port)
	logInfo("Launching Chrome on About section...")
	chromeCmd := getDialtoneCmd("chrome", "new", baseURL, "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		logFatal("Failed to launch Chrome: %v", err)
	}

	logInfo("About Demo Environment is LIVE!")
	logInfo("Dev Server: %s", baseURL)
	logInfo("Press Ctrl+C to stop...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}

func handleRadioDemo(webDir string) {
	logInfo("Setting up Radio Demo Environment...")

	// 1. Port cleanup (same as earth demo)
	logInfo("Cleaning up port 5173...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server with piped output to detect port (same pattern as earth demo)
	logInfo("Starting WWW Dev Server...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	stdout, err := devCmd.StdoutPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stdout: %v", err)
	}
	stderr, err := devCmd.StderrPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stderr: %v", err)
	}
	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}

	// 4. Wait for dev server ready and detect port (like earth_demo.go)
	logInfo("Waiting for Dev Server...")
	port := 5173
	portCh := make(chan int, 1)
	go func() {
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		re := regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/`)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if match := re.FindStringSubmatch(line); len(match) == 2 {
				if p, err := strconv.Atoi(match[1]); err == nil {
					select {
					case portCh <- p:
					default:
					}
				}
			}
		}
	}()

	select {
	case detected := <-portCh:
		port = detected
	case <-time.After(10 * time.Second):
		logInfo("Dev server port not detected yet; falling back to %d", port)
	}

	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	// 5. Launch Chrome straight on Radio section (like earth #s-home, cad #s-cad)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/#s-radio", port)
	logInfo("Launching Chrome on Radio section...")
	chromeCmd := getDialtoneCmd("chrome", "new", baseURL, "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		logFatal("Failed to launch Chrome: %v", err)
	}

	logInfo("Radio Demo Environment is LIVE!")
	logInfo("Dev Server: %s", baseURL)
	logInfo("Press Ctrl+C to stop...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}

func handlePolicyDemo(webDir string) {
	logInfo("Setting up Policy Demo Environment...")

	// 1. Port cleanup (same as earth demo)
	logInfo("Cleaning up port 5173...")
	_ = exec.Command("fuser", "-k", "5173/tcp").Run()
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server with piped output to detect port (same pattern as earth demo)
	logInfo("Starting WWW Dev Server...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	stdout, err := devCmd.StdoutPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stdout: %v", err)
	}
	stderr, err := devCmd.StderrPipe()
	if err != nil {
		logFatal("Failed to attach to dev server stderr: %v", err)
	}
	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}

	// 4. Wait for dev server ready and detect port (like earth_demo.go)
	logInfo("Waiting for Dev Server...")
	port := 5173
	portCh := make(chan int, 1)
	go func() {
		reader := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(reader)
		re := regexp.MustCompile(`http://127\.0\.0\.1:(\d+)/`)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)
			if match := re.FindStringSubmatch(line); len(match) == 2 {
				if p, err := strconv.Atoi(match[1]); err == nil {
					select {
					case portCh <- p:
					default:
					}
				}
			}
		}
	}()

	select {
	case detected := <-portCh:
		port = detected
	case <-time.After(10 * time.Second):
		logInfo("Dev server port not detected yet; falling back to %d", port)
	}

	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d", port))
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	// 5. Launch Chrome straight on Policy section
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/#s-policy", port)
	logInfo("Launching Chrome on Policy section...")
	chromeCmd := getDialtoneCmd("chrome", "new", baseURL, "--gpu")
	chromeCmd.Stdout = os.Stdout
	chromeCmd.Stderr = os.Stderr
	if err := chromeCmd.Run(); err != nil {
		logFatal("Failed to launch Chrome: %v", err)
	}

	logInfo("Policy Demo Environment is LIVE!")
	logInfo("Dev Server: %s", baseURL)
	logInfo("Press Ctrl+C to stop...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	logInfo("Shutting down...")
}

func handleWebgpuDebug(webDir string) {
	logInfo("Setting up WebGPU Debug Environment...")

	headless := os.Getenv("HEADLESS") == "true"

	// 1. Cleanup ports
	logInfo("Cleaning up port 5173...")
	_ = browser.CleanupPort(5173)
	time.Sleep(1500 * time.Millisecond)

	// 2. Kill existing Dialtone Chrome instances
	logInfo("Cleaning up Chrome processes...")
	_ = getDialtoneCmd("chrome", "kill", "all").Run()

	// 3. Start WWW Dev Server (Background)
	logInfo("Starting WWW Dev Server...")
	devCmd := exec.Command("npm", "run", "dev", "--", "--host", "127.0.0.1")
	devCmd.Dir = webDir
	devCmd.Stdout = os.Stdout
	devCmd.Stderr = os.Stderr
	if err := devCmd.Start(); err != nil {
		logFatal("Failed to start dev server: %v", err)
	}
	defer func() {
		if devCmd.Process != nil {
			logInfo("Stopping Dev Server...")
			_ = devCmd.Process.Kill()
		}
	}()

	// 4. Wait for dev server to be ready
	logInfo("Waiting for Dev Server...")
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get("http://127.0.0.1:5173")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !ready {
		logFatal("Dev server failed to start within 30 seconds")
	}

	chromePath := browser.FindChromePath()
	if chromePath == "" {
		logFatal("Chrome not found on this system")
	}

	logInfo("Launching Chrome (headless=%v) with WebGPU flags...", headless)
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("enable-unsafe-webgpu", true),
		chromedp.Flag("enable-features", "Vulkan,UseSkiaRenderer,WebGPU"),
		chromedp.Flag("use-angle", "metal"),
		chromedp.Flag("enable-gpu-rasterization", true),
		chromedp.Flag("enable-zero-copy", true),
		chromedp.Flag("dialtone-origin", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var consoleLogs []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			for _, arg := range ev.Args {
				consoleLogs = append(consoleLogs, fmt.Sprintf("[%s] %s", ev.Type, arg.Value))
			}
		case *runtime.EventExceptionThrown:
			consoleLogs = append(consoleLogs, fmt.Sprintf("[EXCEPTION] %s", ev.ExceptionDetails.Text))
		}
	})

	var hasGPU bool
	var canvasInfo struct {
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		ClientWidth  int    `json:"clientWidth"`
		ClientHeight int    `json:"clientHeight"`
		Text         string `json:"text"`
		Visible      bool   `json:"visible"`
		HasContext   bool   `json:"hasContext"`
	}
	var isVisible bool

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:5173/#s-webgpu-template"),
		chromedp.WaitReady("#webgpu-template-container"),
		chromedp.Evaluate(`(function(){
			const section = document.getElementById("s-webgpu-template");
			if (section) section.scrollIntoView({ behavior: "instant" });
			return true;
		})()`, nil),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`!!navigator.gpu`, &hasGPU),
		chromedp.Evaluate(`(function(){
			const container = document.getElementById("webgpu-template-container");
			const canvas = container ? container.querySelector("canvas") : null;
			const text = container ? container.textContent || "" : "";
			const rect = canvas ? canvas.getBoundingClientRect() : { width: 0, height: 0 };
			return {
				width: canvas ? canvas.width : 0,
				height: canvas ? canvas.height : 0,
				clientWidth: canvas ? canvas.clientWidth : 0,
				clientHeight: canvas ? canvas.clientHeight : 0,
				text: text.trim(),
				visible: rect.width > 0 && rect.height > 0,
				hasContext: canvas ? !!canvas.getContext("webgpu") : false,
			};
		})()`, &canvasInfo),
		chromedp.Evaluate(`(function(){
			const section = document.getElementById("s-webgpu-template");
			return !!(section && section.classList.contains("is-visible"));
		})()`, &isVisible),
	)

	if err != nil {
		logInfo("Chromedp run failed: %v", err)
	}

	logInfo("WebGPU availability: %v", hasGPU)
	logInfo("Section visible (.is-visible): %v", isVisible)
	logInfo("Canvas info: size=%dx%d client=%dx%d visible=%v context=%v",
		canvasInfo.Width,
		canvasInfo.Height,
		canvasInfo.ClientWidth,
		canvasInfo.ClientHeight,
		canvasInfo.Visible,
		canvasInfo.HasContext,
	)
	if canvasInfo.Text != "" {
		logInfo("WebGPU container text: %s", canvasInfo.Text)
	}

	if len(consoleLogs) > 0 {
		logInfo("Browser console logs:")
		for _, line := range consoleLogs {
			fmt.Printf("  %s\n", line)
		}
	} else {
		logInfo("No browser console output captured.")
	}
}
