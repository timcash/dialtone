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
	if IsREPLContext() {
		if isTest {
			message = TestPrefix(message)
		}
		return strings.TrimSpace(message)
	}
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

func FormatDialtoneMessage(prefix, message string) string {
	message = strings.TrimSpace(message)
	if IsREPLContext() {
		return message
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "dialtone"
	} else if strings.HasPrefix(strings.ToUpper(prefix), "DIALTONE") {
		prefix = "dialtone" + prefix[len("DIALTONE"):]
	}
	return prefix + "> " + message
}

func FormatSystemMessage(message string) string {
	return FormatDialtoneMessage("dialtone", message)
}

func FormatUserMessage(message string) string {
	return FormatDialtoneMessage("dialtone", message)
}

func formatSystemMessage(message string) string {
	return FormatSystemMessage(message)
}

func formatUserMessage(message string) string {
	return FormatUserMessage(message)
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
	msg := fmt.Sprintf(format, args...)
	publishPrimary("INFO", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatPlain("INFO", msg))
}

func InfoFrom(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("INFO", msg, source, false)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("INFO", source, msg, false))
}

func InfoFromTest(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("INFO", msg, source, true)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("INFO", source, msg, true))
}

func Raw(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("INFO", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", msg)
}

func System(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimaryWithKind("INFO", "status", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatSystemMessage(msg))
}

func User(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimaryWithKind("INFO", "chat", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatUserMessage(msg))
}

func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("ERROR", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatPlain("ERROR", msg))
}

func ErrorFrom(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("ERROR", msg, source, false)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("ERROR", source, msg, false))
}

func ErrorFromTest(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("ERROR", msg, source, true)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("ERROR", source, msg, true))
}

func Errorf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	Error("%s", msg)
	return fmt.Errorf("%s", msg)
}

func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("WARN", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatPlain("WARN", msg))
}

func WarnFrom(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("WARN", msg, source, false)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("WARN", source, msg, false))
}

func WarnFromTest(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("WARN", msg, source, true)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("WARN", source, msg, true))
}

func Debug(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("DEBUG", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatPlain("DEBUG", msg))
}

func DebugFrom(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("DEBUG", msg, source, false)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("DEBUG", source, msg, false))
}

func DebugFromTest(source, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("DEBUG", msg, source, true)
	fmt.Fprintf(logOutput, "%s\n", formatPlainWithSource("DEBUG", source, msg, true))
}

func Fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	publishPrimary("FATAL", msg, callerSourceLocation(), false)
	fmt.Fprintf(logOutput, "%s\n", formatPlain("FATAL", msg))
	os.Exit(1)
}
