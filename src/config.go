package dialtone

import (
	"os"
	"os/exec"

	"github.com/joho/godotenv"
)

// LoadConfig loads environment variables from .env
func LoadConfig() {
	if err := godotenv.Load(); err != nil {
		LogInfo("Warning: godotenv.Load() failed: %v", err)
	}
}

func validateRequiredVars(vars []string) {
	for _, v := range vars {
		if os.Getenv(v) == "" {
			LogFatal("ERROR: Environment variable %s is not set. Please check your .env file.", v)
		}
	}
}

func runShell(dir string, name string, args ...string) {
	LogInfo("Running: %s %v in %s", name, args, dir)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}

func copyDir(src, dst string) {
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
			copyDir(srcPath, dstPath)
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

func runSimpleShell(command string) {
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

func runSudoShell(command string) {
	LogInfo("Running with sudo: %s", command)
	cmd := exec.Command("sudo", "bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		LogFatal("Command failed: %v", err)
	}
}
