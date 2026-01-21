package dialtone

import (
	"dialtone/cli/src/core/logger"
	"github.com/nats-io/nats-server/v2/server"
)

// LogInfo logs an informational message
func LogInfo(format string, args ...interface{}) {
	logger.LogMsgWithDepth(3, "INFO", format, args...)
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	logger.LogMsgWithDepth(3, "ERROR", format, args...)
}

// LogFatal logs a fatal error and exits
func LogFatal(format string, args ...interface{}) {
	logger.LogMsgWithDepth(3, "FATAL", format, args...)
}

// LogPrintf is an alias for LogInfo to satisfy interfaces expecting a Printf signature
func LogPrintf(format string, args ...interface{}) {
	logger.LogMsgWithDepth(3, "INFO", format, args...)
}

// GetNATSLogger returns a logger suitable for NATS server
func GetNATSLogger() server.Logger {
	return logger.GetNATSLogger()
}
