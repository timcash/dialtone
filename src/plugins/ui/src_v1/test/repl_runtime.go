package test

import (
	"os"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
)

func ResolveSuiteNATSURL() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPL_NATS_URL")); v != "" {
		return v
	}
	return "nats://127.0.0.1:4222"
}

func ReplIndexInfof(format string, args ...any) {
	logs.Info("DIALTONE_INDEX: "+format, args...)
}
