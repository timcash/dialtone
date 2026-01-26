package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var logFile *os.File

func init() {
	var err error
	logFile, err = os.OpenFile("dialtone.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Warning: Failed to open dialtone.log: %v\n", err)
	}

	RedirectStandardLogger()
}

// LoggerWriter implements io.Writer to capture standard log output
type LoggerWriter struct{}

func (w *LoggerWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		LogMsgWithDepth(5, "INFO", "%s", msg)
	}
	return len(p), nil
}

// RedirectStandardLogger redirects the default Go log package output to our custom formatter
func RedirectStandardLogger() {
	log.SetFlags(0)
	log.SetOutput(&LoggerWriter{})
}

// LogMsgWithDepth is the internal helper for formatted logging with custom stack depth
func LogMsgWithDepth(depth int, level string, format string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	pc, file, line, ok := runtime.Caller(depth)
	details := "unknown:unknown:0"
	if ok {
		fn := runtime.FuncForPC(pc)
		fnName := "unknown"
		if fn != nil {
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
	out := fmt.Sprintf("[%s | %s | %s] %s\n", timestamp, level, details, msg)
	fmt.Print(out)
	if logFile != nil {
		logFile.WriteString(out)
	}
}

// LogInfo logs an informational message
func LogInfo(format string, args ...interface{}) {
	LogMsgWithDepth(3, "INFO", format, args...)
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	LogMsgWithDepth(3, "ERROR", format, args...)
}

// LogFatal logs a fatal error and exits
func LogFatal(format string, args ...interface{}) {
	LogMsgWithDepth(3, "FATAL", format, args...)
	os.Exit(1)
}

// Noticef satisfies nats-server Logger interface
func (w *LoggerWriter) Noticef(format string, v ...interface{}) {
	LogMsgWithDepth(4, "INFO", format, v...)
}

// Fatalf satisfies nats-server Logger interface
func (w *LoggerWriter) Fatalf(format string, v ...interface{}) {
	LogMsgWithDepth(4, "FATAL", format, v...)
	os.Exit(1)
}

// Errorf satisfies nats-server Logger interface
func (w *LoggerWriter) Errorf(format string, v ...interface{}) {
	LogMsgWithDepth(4, "ERROR", format, v...)
}

// Warnf satisfies nats-server Logger interface
func (w *LoggerWriter) Warnf(format string, v ...interface{}) {
	LogMsgWithDepth(4, "WARN", format, v...)
}

// Debugf satisfies nats-server Logger interface
func (w *LoggerWriter) Debugf(format string, v ...interface{}) {
	LogMsgWithDepth(4, "DEBUG", format, v...)
}

// Tracef satisfies nats-server Logger interface
func (w *LoggerWriter) Tracef(format string, v ...interface{}) {
	LogMsgWithDepth(4, "TRACE", format, v...)
}

// GetNATSLogger returns a logger suitable for NATS server
func GetNATSLogger() *LoggerWriter {
	return &LoggerWriter{}
}
