package test

import (
	"fmt"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

func (sc *StepContext) Logf(format string, args ...any) {
	sc.Infof(format, args...)
}

func (sc *StepContext) Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepLog("INFO", msg)
	source := sc.callerLocation()
	if !sc.quietConsole {
		logs.InfoFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepLog("WARN", msg)
	source := sc.callerLocation()
	if !sc.quietConsole {
		logs.WarnFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
	if sc.logger != nil {
		_ = sc.logger.WarnfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepLog("DEBUG", msg)
	source := sc.callerLocation()
	if !sc.quietConsole {
		logs.DebugFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	sc.appendStepError("ERROR", msg)
	source := sc.callerLocation()
	logs.ErrorFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	if sc.logger != nil {
		_ = sc.logger.ErrorfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
	if sc.errorLogger != nil {
		_ = sc.errorLogger.ErrorfFromTest(source, "[STEP:%s] %s", sc.Name, msg)
	}
}

func (sc *StepContext) TestPassf(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		msg = "step passed"
	}
	source := sc.callerLocation()
	line := fmt.Sprintf("[TEST][PASS] [STEP:%s] %s", sc.Name, msg)
	sc.appendStepLog("PASS", line)
	if !sc.quietConsole {
		logs.InfoFromTest(source, "%s", line)
	}
	if sc.passLogger != nil {
		_ = sc.passLogger.InfofFromTest(source, "%s", line)
		return
	}
	if sc.logger != nil {
		_ = sc.logger.InfofFromTest(source, "%s", line)
	}
	if strings.HasPrefix(msg, "report: ") || msg == "completed" {
		return
	}
	sc.publishStatus("validation", fmt.Sprintf("Validation passed for %s: %s", sc.Name, msg))
}

func (sc *StepContext) TestFailf(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		msg = "step failed"
	}
	source := sc.callerLocation()
	line := fmt.Sprintf("[TEST][FAIL] [STEP:%s] %s", sc.Name, msg)
	sc.appendStepError("FAIL", line)
	logs.ErrorFromTest(source, "%s", line)
	if sc.failLogger != nil {
		_ = sc.failLogger.ErrorfFromTest(source, "%s", line)
		return
	}
	if sc.errorLogger != nil {
		_ = sc.errorLogger.ErrorfFromTest(source, "%s", line)
		return
	}
	if sc.logger != nil {
		_ = sc.logger.ErrorfFromTest(source, "%s", line)
	}
	sc.publishStatus("validation", fmt.Sprintf("Validation failed for %s: %s", sc.Name, msg))
}

func (sc *StepContext) WaitForMessage(subject string, pattern string, timeout time.Duration) error {
	if sc.logger == nil || sc.logger.Conn() == nil {
		return fmt.Errorf("NATS not available in this test context")
	}
	nc := sc.logger.Conn()

	msgCh := make(chan string, 100)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		msgCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	deadline := time.Now().Add(timeout)
	for {
		select {
		case data := <-msgCh:
			if strings.Contains(data, pattern) {
				return nil
			}
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for %q on %s", pattern, subject)
		}
	}
}

func (sc *StepContext) NATSConn() *nats.Conn {
	if sc.logger == nil {
		return nil
	}
	return sc.logger.Conn()
}

func (sc *StepContext) NATSURL() string {
	return strings.TrimSpace(sc.natsURL)
}

func (sc *StepContext) NATSURLForHost(host string) (string, error) {
	base := strings.TrimSpace(sc.natsURL)
	host = strings.TrimSpace(host)
	if base == "" {
		return "", fmt.Errorf("NATS not configured in this test context")
	}
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	trimmed := strings.TrimSpace(strings.TrimPrefix(base, "nats://"))
	parts := strings.Split(trimmed, ":")
	port := "4222"
	if len(parts) > 1 {
		port = strings.TrimSpace(parts[len(parts)-1])
	}
	if port == "" {
		port = "4222"
	}
	return fmt.Sprintf("nats://%s:%s", host, port), nil
}

func (sc *StepContext) RepoRoot() string {
	return strings.TrimSpace(sc.repoRoot)
}

func (sc *StepContext) NewTopicLogger(subject string) (*logs.NATSLogger, error) {
	nc := sc.NATSConn()
	if nc == nil {
		return nil, fmt.Errorf("NATS not available in this test context")
	}
	return logs.NewNATSLogger(nc, subject)
}

func (sc *StepContext) WaitForMessageAfterAction(subject, pattern string, timeout time.Duration, action func() error) error {
	if sc.logger == nil || sc.logger.Conn() == nil {
		return fmt.Errorf("NATS not available in this test context")
	}
	nc := sc.logger.Conn()
	msgCh := make(chan string, 100)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		msgCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return err
	}
	deadline := time.Now().Add(timeout)
	for {
		select {
		case data := <-msgCh:
			if strings.Contains(data, pattern) {
				return nil
			}
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for %q on %s", pattern, subject)
		}
	}
}

func (sc *StepContext) WaitForAllMessagesAfterAction(subject string, patterns []string, timeout time.Duration, action func() error) error {
	if sc.logger == nil || sc.logger.Conn() == nil {
		return fmt.Errorf("NATS not available in this test context")
	}
	if len(patterns) == 0 {
		return fmt.Errorf("no patterns provided")
	}
	nc := sc.logger.Conn()
	msgCh := make(chan string, 100)
	sub, err := nc.Subscribe(subject, func(m *nats.Msg) {
		msgCh <- logs.FormatMessage(m.Subject, m.Data)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}
	if err := action(); err != nil {
		return err
	}
	seen := map[string]bool{}
	deadline := time.Now().Add(timeout)
	for len(seen) < len(patterns) {
		select {
		case data := <-msgCh:
			for _, p := range patterns {
				if !seen[p] && strings.Contains(data, p) {
					seen[p] = true
				}
			}
		case <-time.After(time.Until(deadline)):
			missing := []string{}
			for _, p := range patterns {
				if !seen[p] {
					missing = append(missing, p)
				}
			}
			return fmt.Errorf("timeout waiting for patterns on %s: %s", subject, strings.Join(missing, ", "))
		}
	}
	return nil
}

func (sc *StepContext) WaitForStepMessage(pattern string, timeout time.Duration) error {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return fmt.Errorf("step subject not available in this test context")
	}
	return sc.WaitForMessage(sc.StepSubject, pattern, timeout)
}

func (sc *StepContext) WaitForBrowserMessage(pattern string, timeout time.Duration) error {
	if strings.TrimSpace(sc.BrowserSubject) == "" {
		return fmt.Errorf("browser subject not available in this test context")
	}
	return sc.WaitForMessage(sc.BrowserSubject, pattern, timeout)
}

func (sc *StepContext) WaitForErrorMessage(pattern string, timeout time.Duration) error {
	if strings.TrimSpace(sc.ErrorSubject) == "" {
		return fmt.Errorf("error subject not available in this test context")
	}
	return sc.WaitForMessage(sc.ErrorSubject, pattern, timeout)
}

func (sc *StepContext) WaitForErrorMessageAfterAction(pattern string, timeout time.Duration, action func() error) error {
	if strings.TrimSpace(sc.ErrorSubject) == "" {
		return fmt.Errorf("error subject not available in this test context")
	}
	return sc.WaitForMessageAfterAction(sc.ErrorSubject, pattern, timeout, action)
}

func (sc *StepContext) WaitForStepMessageAfterAction(pattern string, timeout time.Duration, action func() error) error {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return fmt.Errorf("step subject not available in this test context")
	}
	return sc.WaitForMessageAfterAction(sc.StepSubject, pattern, timeout, action)
}

func (sc *StepContext) WaitForBrowserMessageAfterAction(pattern string, timeout time.Duration, action func() error) error {
	if strings.TrimSpace(sc.BrowserSubject) == "" {
		return fmt.Errorf("browser subject not available in this test context")
	}
	return sc.WaitForMessageAfterAction(sc.BrowserSubject, pattern, timeout, action)
}

func (sc *StepContext) ResetStepLogClock() {
	if strings.TrimSpace(sc.StepSubject) == "" {
		return
	}
	logs.ResetTopicClock(sc.StepSubject)
}

func (sc *StepContext) callerLocation() string {
	for i := 2; i < 14; i++ {
		_, file, line, ok := stdruntime.Caller(i)
		if !ok {
			break
		}
		norm := filepath.ToSlash(file)
		if strings.Contains(norm, "/plugins/test/src_v1/go/test.go") {
			continue
		}
		if idx := strings.Index(norm, "/src/"); idx >= 0 {
			if line > 0 {
				return fmt.Sprintf("%s:%d", norm[idx+1:], line)
			}
			return norm[idx+1:]
		}
		base := filepath.Base(file)
		if line > 0 {
			return fmt.Sprintf("%s:%d", base, line)
		}
		return base
	}
	return "unknown"
}

func (sc *StepContext) appendStepLog(level, msg string) {
	line := strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(level), strings.TrimSpace(msg)))
	if line == "" {
		return
	}
	sc.logMu.Lock()
	sc.stepLogs = append(sc.stepLogs, line)
	sc.logMu.Unlock()
}

func (sc *StepContext) appendStepError(level, msg string) {
	line := strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(level), strings.TrimSpace(msg)))
	if line == "" {
		return
	}
	sc.logMu.Lock()
	sc.stepErrors = append(sc.stepErrors, line)
	sc.logMu.Unlock()
}

func (sc *StepContext) snapshotStepLogs() ([]string, []string, []string) {
	sc.logMu.Lock()
	defer sc.logMu.Unlock()
	logCopy := append([]string(nil), sc.stepLogs...)
	errCopy := append([]string(nil), sc.stepErrors...)
	browserCopy := append([]string(nil), sc.browserLogs...)
	return logCopy, errCopy, browserCopy
}

func (sc *StepContext) appendBrowserLog(kind, msg string, isError bool) {
	line := strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(kind), strings.TrimSpace(msg)))
	if line == "" {
		return
	}
	if isError {
		line = "ERROR: " + line
	} else {
		line = "INFO: " + line
	}
	sc.logMu.Lock()
	sc.browserLogs = append(sc.browserLogs, line)
	sc.logMu.Unlock()
}
