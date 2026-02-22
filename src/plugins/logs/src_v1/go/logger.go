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
	src := callerSourceLocation()
	return formatPlainWithSource(level, src, message, false)
}

func formatPlainWithSource(level, source, message string, isTest bool) string {
	src := normalizeSourcePath(source)
	if src == "" {
		src = "unknown"
	}
	if isTest {
		message = TestPrefix(message)
	}
	elapsed := elapsedSeconds("default")
	return fmt.Sprintf("[T+%04ds|%s|%s] %s", elapsed, strings.ToUpper(level), src, message)
}

func TestPrefix(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "[TEST]") {
		return line
	}
	return "[TEST] " + line
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

func callerSourceLocation() string {
	for i := 2; i < 12; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		norm := filepath.ToSlash(file)
		if strings.Contains(norm, "/plugins/logs/src_v1/go/logger.go") || strings.Contains(norm, "/plugins/logs/src_v1/go/nats.go") {
			continue
		}
		src := normalizeSourcePath(file)
		if src == "" {
			src = "unknown"
		}
		if line > 0 {
			return fmt.Sprintf("%s:%d", src, line)
		}
		return src
	}
	return "unknown"
}

func normalizeSourcePath(source string) string {
	s := strings.TrimSpace(filepath.ToSlash(source))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "src/") {
		return s
	}
	if idx := strings.Index(s, "/src/"); idx >= 0 {
		return s[idx+1:]
	}
	return filepath.Base(s)
}

func Info(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("INFO", fmt.Sprintf(format, args...)))
}

func InfoFrom(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("INFO", source, fmt.Sprintf(format, args...), false))
}

func InfoFromTest(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("INFO", source, fmt.Sprintf(format, args...), true))
}

func Raw(format string, args ...any) {
	fmt.Fprintf(logOutput, format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("ERROR", fmt.Sprintf(format, args...)))
}

func ErrorFrom(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("ERROR", source, fmt.Sprintf(format, args...), false))
}

func ErrorFromTest(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("ERROR", source, fmt.Sprintf(format, args...), true))
}

func Errorf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	Error("%s", msg)
	return fmt.Errorf("%s", msg)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("WARN", fmt.Sprintf(format, args...)))
}

func WarnFrom(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("WARN", source, fmt.Sprintf(format, args...), false))
}

func WarnFromTest(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("WARN", source, fmt.Sprintf(format, args...), true))
}

func Debug(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("DEBUG", fmt.Sprintf(format, args...)))
}

func DebugFrom(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("DEBUG", source, fmt.Sprintf(format, args...), false))
}

func DebugFromTest(source, format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("DEBUG", source, fmt.Sprintf(format, args...), true))
}

func Fatal(format string, args ...any) {
	fmt.Fprintf(logOutput, "%s\n", formatPlain("FATAL", fmt.Sprintf(format, args...)))
	os.Exit(1)
}
