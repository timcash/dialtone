package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/cli/src/core/logger"

	"github.com/joho/godotenv"
)

// LoadConfig loads environment variables from a custom file or defaults to .env
func LoadConfig() {
	envFile := os.Getenv("DIALTONE_ENV_FILE")
	if envFile == "" {
		envFile = "env/.env"
	}

	if err := godotenv.Load(envFile); err != nil {
		logger.LogInfo("Warning: godotenv.Load(%s) failed: %v", envFile, err)
	}
}

// GetDialtoneEnv returns the directory where dependencies are installed.
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
	// Strict mode: no fallback
	return ""
}

func ValidateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			logger.LogFatal("ERROR: Environment variable %s is not set. Please check your .env file.", v)
		}
	}
}

func RunShell(dir string, name string, args ...string) {
	logger.LogInfo("Running: %s %v in %s", name, args, dir)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %v", err)
	}
}

func CopyDir(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		logger.LogFatal("Failed to read src dir %s: %v", src, err)
	}

	for _, entry := range entries {
		srcPath := src + "/" + entry.Name()
		dstPath := dst + "/" + entry.Name()

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				logger.LogFatal("Failed to create dir %s: %v", dstPath, err)
			}
			CopyDir(srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				logger.LogFatal("Failed to read file %s: %v", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				logger.LogFatal("Failed to write file %s: %v", dstPath, err)
			}
		}
	}
}

func RunSimpleShell(command string) {
	logger.LogInfo("Running: %s", command)
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %v", err)
	}
}

func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func RunSudoShell(command string) {
	logger.LogInfo("Running with sudo: %s", command)
	cmd := exec.Command("sudo", "bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Command failed: %v", err)
	}
}
