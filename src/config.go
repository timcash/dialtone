package dialtone

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// LoadConfig loads environment variables from .env
func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		LogInfo("Warning: godotenv.Load() failed: %v", err)
	}
}

// GetDialtoneEnv returns the directory where dependencies are installed.
// It checks the DIALTONE_ENV environment variable, falling back to:
// 1. dialtone_dependencies/ next to the dialtone.sh script (if in a repo)
// 2. ~/.dialtone_env (default)
func GetDialtoneEnv() string {
	env := os.Getenv("DIALTONE_ENV")
	if env != "" {
		if strings.HasPrefix(env, "~") {
			home, _ := os.UserHomeDir()
			env = filepath.Join(home, env[1:])
		}
		absEnv, _ := filepath.Abs(env)
		return absEnv
	}

	cwd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			localPath := filepath.Join(cwd, "dialtone_dependencies")
			if _, err := os.Stat(localPath); err == nil {
				absPath, _ := filepath.Abs(localPath)
				return absPath
			}
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}

	home, _ := os.UserHomeDir()
	defaultPath := filepath.Join(home, ".dialtone_env")
	absPath, _ := filepath.Abs(defaultPath)
	return absPath
}

func validateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			LogFatal("ERROR: Environment variable %s is not set. Please check your .env file.", v)
		}
	}
}

func RunShell(dir string, name string, args ...string) {
	LogInfo("Running: %s %v in %s", name, args, dir)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}

func CopyDir(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		LogFatal("Failed to read src dir %s: %v", src, err)
	}

	for _, entry := range entries {
		srcPath := src + "/" + entry.Name()
		dstPath := dst + "/" + entry.Name()

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				LogFatal("Failed to create dir %s: %v", dstPath, err)
			}
			CopyDir(srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				LogFatal("Failed to read file %s: %v", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				LogFatal("Failed to write file %s: %v", dstPath, err)
			}
		}
	}
}

func RunSimpleShell(command string) {
	LogInfo("Running: %s", command)
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func RunSudoShell(command string) {
	LogInfo("Running with sudo: %s", command)
	cmd := exec.Command("sudo", "bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}
