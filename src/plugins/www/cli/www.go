package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func logInfo(format string, args ...interface{}) {
	fmt.Printf("[www] "+format+"\n", args...)
}

func logFatal(format string, args ...interface{}) {
	fmt.Printf("[www] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

type vercelProject struct {
	ProjectID string `json:"projectId"`
	OrgID     string `json:"orgId"`
}

type npmPackage struct {
	Version string `json:"version"`
}

func vercelProjectEnv(webDir string) ([]string, error) {
	projectPath := filepath.Join(webDir, ".vercel", "project.json")
	data, err := os.ReadFile(projectPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", projectPath, err)
	}

	var project vercelProject
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, fmt.Errorf("parse %s: %w", projectPath, err)
	}
	if project.ProjectID == "" || project.OrgID == "" {
		return nil, fmt.Errorf("missing project/org id in %s", projectPath)
	}

	return []string{
		"VERCEL_PROJECT_ID=" + project.ProjectID,
		"VERCEL_ORG_ID=" + project.OrgID,
	}, nil
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

func publishPrebuilt(webDir string, repoRoot string, vercelPath string, vercelEnv []string, args []string) {
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
	prebuildCmd.Dir = repoRoot
	prebuildCmd.Env = append(os.Environ(), vercelEnv...)
	prebuildCmd.Stdout = os.Stdout
	prebuildCmd.Stderr = os.Stderr
	prebuildCmd.Stdin = os.Stdin
	if err := prebuildCmd.Run(); err != nil {
		logFatal("Vercel build failed: %v", err)
	}

	vArgs := append([]string{"deploy", "--prebuilt", "--prod"}, args...)
	cmd := exec.Command(vercelPath, vArgs...)
	cmd.Dir = repoRoot
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
		return
	}

	subcommand := args[0]
	// Determine the directory where the webpage code is located
	// Used to be "dialtone-earth", now it is "src/plugins/www/app"
	// We need to resolve it relative to the project root (where dialtone-dev runs)
	webDir := filepath.Join("src", "plugins", "www", "app")
	repoRoot, _ := os.Getwd()
	vercelEnv, err := vercelProjectEnv(webDir)
	if err != nil {
		logFatal("Vercel project not linked in %s/.vercel. Run 'vercel link' there. (%v)", webDir, err)
	}

	switch subcommand {
	case "publish":
		publishPrebuilt(webDir, repoRoot, vercelPath, vercelEnv, args[1:])

	case "publish-prebuilt":
		publishPrebuilt(webDir, repoRoot, vercelPath, vercelEnv, args[1:])

	case "logs":
		if len(args) < 2 {
			logFatal("Usage: dialtone-dev www logs <deployment-url-or-id>\n   Run 'dialtone-dev www logs --help' for more info.")
		}
		vArgs := append([]string{"logs"}, args[1:]...)
		cmd := exec.Command(vercelPath, vArgs...)
		cmd.Dir = repoRoot
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
		cmd.Dir = repoRoot
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
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), vercelEnv...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			logFatal("Vercel command failed: %v", err)
		}
	}
}
