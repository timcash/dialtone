package logs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var logOutput io.Writer = io.Discard
var plainLogMu sync.Mutex
var plainLogEpoch = map[string]time.Time{}

func SetOutput(w io.Writer) {
	logOutput = w
}

func ResetClock(topic string) {
	key := strings.TrimSpace(topic)
	if key == "" {
		key = "default"
	}
	plainLogMu.Lock()
	delete(plainLogEpoch, key)
	plainLogMu.Unlock()
}

func formatPlain(level, message string) string {
	src := callerSourceFile()
	elapsed := elapsedSeconds("default")
	return fmt.Sprintf("[T+%04ds|%s|%s] %s", elapsed, strings.ToUpper(level), src, message)
}

func elapsedSeconds(topic string) int {
	key := strings.TrimSpace(topic)
	if key == "" {
		key = "default"
	}
	now := time.Now()
	plainLogMu.Lock()
	start, ok := plainLogEpoch[key]
	if !ok {
		start = now
		plainLogEpoch[key] = now
	}
	plainLogMu.Unlock()
	sec := int(now.Sub(start).Seconds())
	if sec < 0 {
		return 0
	}
	return sec
}

func callerSourceFile() string {
	for i := 2; i < 12; i++ {
		_, file, _, ok := runtime.Caller(i)
		if !ok {
			break
		}
		norm := filepath.ToSlash(file)
		if strings.Contains(norm, "/plugins/logs/src_v1/go/logger.go") || strings.Contains(norm, "/plugins/logs/src_v1/go/nats.go") {
			continue
		}
		return filepath.Base(file)
	}
	return "unknown"
}

func Info(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("INFO", fmt.Sprintf(format, args...)))
}

func Raw(format string, args ...any) {
	fmt.Fprintf(logOutput, format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("ERROR", fmt.Sprintf(format, args...)))
}

func Errorf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	Error("%s", msg)
	return fmt.Errorf("%s", msg)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("WARN", fmt.Sprintf(format, args...)))
}

func Debug(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("DEBUG", fmt.Sprintf(format, args...)))
}

func Fatal(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("FATAL", fmt.Sprintf(format, args...)))
	os.Exit(1)
}
