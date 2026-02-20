package logs

import (
	"os"
	"path/filepath"
	"strings"
)

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
	// Fallback to default
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone_env")
}
