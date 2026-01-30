package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"
)

const gofmtBatchSize = 200

// Run handles the 'format' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	if err := formatGoFiles(); err != nil {
		logger.LogFatal("Format failed: %v", err)
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone format")
	fmt.Println()
	fmt.Println("Formats Go code using the Dialtone Go toolchain.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  help   Show help for format command")
	fmt.Println("  --help Show help for format command")
}

func formatGoFiles() error {
	root, err := findRepoRoot()
	if err != nil {
		return err
	}

	gofmtBin, err := resolveGofmt()
	if err != nil {
		return err
	}
	setupGoEnv()

	var files []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if len(files) == 0 {
		logger.LogInfo("No Go files found to format.")
		return nil
	}

	for i := 0; i < len(files); i += gofmtBatchSize {
		end := i + gofmtBatchSize
		if end > len(files) {
			end = len(files)
		}
		args := append([]string{"-w"}, files[i:end]...)
		cmd := exec.Command(gofmtBin, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("gofmt failed: %v", err)
		}
	}

	logger.LogInfo("Formatted %d Go files.", len(files))
	return nil
}

func resolveGofmt() (string, error) {
	envDir := config.GetDialtoneEnv()
	if envDir == "" {
		return "", fmt.Errorf("DIALTONE_ENV not set (run ./dialtone.sh go install)")
	}
	candidate := filepath.Join(envDir, "go", "bin", "gofmt")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("gofmt not found in DIALTONE_ENV (run ./dialtone.sh go install)")
}

func setupGoEnv() {
	envDir := config.GetDialtoneEnv()
	if envDir == "" {
		return
	}
	goDir := filepath.Join(envDir, "go")
	os.Setenv("GOROOT", goDir)
	newPath := filepath.Join(goDir, "bin") + string(os.PathListSeparator) + os.Getenv("PATH")
	os.Setenv("PATH", newPath)
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found from %s", cwd)
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "dist", "bin", "vendor", ".cursor":
		return true
	case "dialtone_dependencies", "test_dialtone_env", ".dialtone_env", ".dialtone_env_test":
		return true
	}
	return strings.HasPrefix(name, ".git")
}
