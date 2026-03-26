package logs

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"os"
	"path/filepath"
	"strings"
)

func DialtoneContext() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv("DIALTONE_CONTEXT")))
}

func IsREPLContext() bool {
	return DialtoneContext() == "repl"
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
	return configv1.DefaultDialtoneEnv()
}
