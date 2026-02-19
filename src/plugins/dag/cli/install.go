package cli

import (
	"archive/zip"
	"dialtone/dev/core/config"
	core_install "dialtone/dev/core/install"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var installRequirements = []core_install.Requirement{
	{Tool: core_install.ToolGo, Version: core_install.GoVersion},
	{Tool: core_install.ToolBun, Version: core_install.BunVersion},
}

func RunInstall(versionDir string) error {
	return runDagInstall(versionDir)
}

func runDagInstall(versionDir string) error {
	fmt.Printf(">> [DAG] Install: %s\n", versionDir)

	if err := core_install.EnsureRequirements(installRequirements); err != nil {
		return err
	}
	if err := ensureDuckDBInstalled(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("ui package.json not found for %s: %w", versionDir, err)
	}
	if err := runVersionInstallHook(cwd, versionDir); err != nil {
		return err
	}

	cmd := runBun(cwd, uiDir, "install", "--force")
	return cmd.Run()
}

func runVersionInstallHook(repoRoot, versionDir string) error {
	hookPath := filepath.Join(repoRoot, "src", "plugins", "dag", versionDir, "cmd", "ops", "install.go")
	if _, err := os.Stat(hookPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat version install hook: %w", err)
	}

	fmt.Printf("   [DAG] Running version install hook: %s\n", filepath.ToSlash(filepath.Join("src", "plugins", "dag", versionDir, "cmd", "ops", "install.go")))
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "dag", versionDir, "cmd", "ops", "install.go")))
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("version install hook failed: %w", err)
	}
	return nil
}

func ensureDuckDBInstalled() error {
	envDir := config.GetDialtoneEnv()
	if envDir == "" {
		return fmt.Errorf("DIALTONE_ENV is not set")
	}
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return fmt.Errorf("create DIALTONE_ENV: %w", err)
	}

	duckDir := filepath.Join(envDir, "duckdb")
	binDir := filepath.Join(duckDir, "bin")
	binaryName := "duckdb"
	if runtime.GOOS == "windows" {
		binaryName = "duckdb.exe"
	}
	duckBin := filepath.Join(binDir, binaryName)

	if info, err := os.Stat(duckBin); err == nil && !info.IsDir() {
		fmt.Printf("   [DAG] duckdb already installed at %s\n", duckBin)
		return ensureDuckDBBinLink(envDir, duckBin, binaryName)
	}

	archiveName, err := duckDBArchiveName()
	if err != nil {
		return err
	}

	downloadURL := "https://github.com/duckdb/duckdb/releases/latest/download/" + archiveName
	fmt.Printf("   [DAG] Installing duckdb from %s\n", downloadURL)

	tmpDir, err := os.MkdirTemp("", "dag-duckdb-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, archiveName)
	if err := downloadFile(downloadURL, zipPath); err != nil {
		return err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create duckdb bin dir: %w", err)
	}
	if err := extractDuckDBBinary(zipPath, duckBin, binaryName); err != nil {
		return err
	}
	if err := os.Chmod(duckBin, 0o755); err != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("chmod duckdb binary: %w", err)
	}

	fmt.Printf("   [DAG] duckdb installed at %s\n", duckBin)
	return ensureDuckDBBinLink(envDir, duckBin, binaryName)
}

func duckDBArchiveName() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return "duckdb_cli-osx-arm64.zip", nil
	case "darwin/amd64":
		return "duckdb_cli-osx-universal.zip", nil
	case "linux/amd64":
		return "duckdb_cli-linux-amd64.zip", nil
	case "linux/arm64":
		return "duckdb_cli-linux-arm64.zip", nil
	case "windows/amd64":
		return "duckdb_cli-windows-amd64.zip", nil
	default:
		return "", fmt.Errorf("unsupported platform for duckdb install: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download duckdb archive: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download duckdb archive: unexpected status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write archive file: %w", err)
	}
	return nil
}

func extractDuckDBBinary(zipPath, outPath, binaryName string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open duckdb archive: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		name := strings.ToLower(filepath.Base(f.Name))
		if name != strings.ToLower(binaryName) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open duckdb binary in archive: %w", err)
		}

		out, err := os.Create(outPath)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create duckdb binary: %w", err)
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rcCloseErr := rc.Close()
		if copyErr != nil {
			return fmt.Errorf("extract duckdb binary: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close duckdb binary: %w", closeErr)
		}
		if rcCloseErr != nil {
			return fmt.Errorf("close archive stream: %w", rcCloseErr)
		}
		return nil
	}

	return fmt.Errorf("duckdb binary %s not found in archive", binaryName)
}

func ensureDuckDBBinLink(envDir, duckBin, binaryName string) error {
	envBinDir := filepath.Join(envDir, "bin")
	if err := os.MkdirAll(envBinDir, 0o755); err != nil {
		return fmt.Errorf("create env bin dir: %w", err)
	}
	linkPath := filepath.Join(envBinDir, binaryName)

	_ = os.Remove(linkPath)
	if runtime.GOOS == "windows" {
		src, err := os.Open(duckBin)
		if err != nil {
			return fmt.Errorf("open duckdb source binary: %w", err)
		}
		defer src.Close()
		dst, err := os.Create(linkPath)
		if err != nil {
			return fmt.Errorf("create duckdb destination binary: %w", err)
		}
		if _, err := io.Copy(dst, src); err != nil {
			_ = dst.Close()
			return fmt.Errorf("copy duckdb binary into env bin: %w", err)
		}
		if err := dst.Close(); err != nil {
			return fmt.Errorf("close copied duckdb binary: %w", err)
		}
		return nil
	}
	if err := os.Symlink(duckBin, linkPath); err != nil {
		return fmt.Errorf("symlink duckdb into env bin: %w", err)
	}
	return nil
}
