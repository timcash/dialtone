package dialtone

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func init() {
	RedirectStandardLogger()
}

// logWriter implements io.Writer to capture standard log output
type logWriter struct{}

func (w *logWriter) Write(p []byte) (n int, err error) {
	// Standard log lines usually end with a newline, strip it if present
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		// Since this is coming from standard log, the caller depth is higher
		// We use LogMsg directly with appropriate depth
		logMsgWithDepth(3, "INFO", "%s", msg)
	}
	return len(p), nil
}

// RedirectStandardLogger redirects the default Go log package output to our custom formatter
func RedirectStandardLogger() {
	log.SetFlags(0) // Remove default timestamp/flags
	log.SetOutput(&logWriter{})
}

// logMsg is the internal helper for formatted logging
func logMsg(level string, format string, args ...interface{}) {
	logMsgWithDepth(3, level, format, args...)
}

func logMsgWithDepth(depth int, level string, format string, args ...interface{}) {
	// ISO 8601 timestamp (RFC3339)
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	// Caller info
	pc, file, line, ok := runtime.Caller(depth)
	details := "unknown:unknown:0"
	if ok {
		fn := runtime.FuncForPC(pc)
		fnName := "unknown"
		if fn != nil {
			// Extract function name from full path
			// e.g., "dialtone/src.runWithTailscale" -> "runWithTailscale"
			fullPath := fn.Name()
			parts := strings.Split(fullPath, ".")
			if len(parts) > 0 {
				fnName = parts[len(parts)-1]
			} else {
				fnName = fullPath
			}
		}
		fileName := filepath.Base(file)
		details = fmt.Sprintf("%s:%s:%d", fileName, fnName, line)
	}

	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s | %s | %s] %s\n", timestamp, level, details, msg)
}

// LogInfo logs an informational message
func LogInfo(format string, args ...interface{}) {
	logMsg("INFO", format, args...)
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	logMsg("ERROR", format, args...)
}

// LogFatal logs a fatal error and exits
func LogFatal(format string, args ...interface{}) {
	logMsg("FATAL", format, args...)
	os.Exit(1)
}

// LogPrintf is an alias for LogInfo to satisfy interfaces expecting a Printf signature
func LogPrintf(format string, args ...interface{}) {
	logMsg("INFO", format, args...)
}

// Noticef satisfies nats-server Logger interface
func (w *logWriter) Noticef(format string, v ...interface{}) {
	logMsgWithDepth(4, "INFO", format, v...)
}

// Fatalf satisfies nats-server Logger interface
func (w *logWriter) Fatalf(format string, v ...interface{}) {
	logMsgWithDepth(4, "FATAL", format, v...)
	os.Exit(1)
}

// Errorf satisfies nats-server Logger interface
func (w *logWriter) Errorf(format string, v ...interface{}) {
	logMsgWithDepth(4, "ERROR", format, v...)
}

// Warnf satisfies nats-server Logger interface
func (w *logWriter) Warnf(format string, v ...interface{}) {
	logMsgWithDepth(4, "WARN", format, v...)
}

// Debugf satisfies nats-server Logger interface
func (w *logWriter) Debugf(format string, v ...interface{}) {
	logMsgWithDepth(4, "DEBUG", format, v...)
}

// Tracef satisfies nats-server Logger interface
func (w *logWriter) Tracef(format string, v ...interface{}) {
	logMsgWithDepth(4, "TRACE", format, v...)
}

// GetNATSLogger returns a logger suitable for NATS server
func GetNATSLogger() *logWriter {
	return &logWriter{}
}
