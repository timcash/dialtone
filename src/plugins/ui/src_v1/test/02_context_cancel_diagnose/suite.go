package contextcanceldiagnose

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	uitest "dialtone/dev/plugins/ui/src_v1/test"
	"github.com/chromedp/chromedp"
)

var ctx = uitest.SharedContext()

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "ui-attach-context-cancel-diagnose",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			return run(sc)
		},
	})
}

func run(sc *testv1.StepContext) (testv1.StepRunResult, error) {
	ctx.BeginStep(sc)
	opts := uitest.GetOptions()
	attach := strings.TrimSpace(opts.AttachNode) != ""
	if !attach {
		return testv1.StepRunResult{Report: "skipped (not running in --attach mode)"}, nil
	}
	if err := ctx.EnsureBuiltAndServed(); err != nil {
		return testv1.StepRunResult{}, err
	}

	defaultURL := ctx.AppURL("/#hero")
	browserOpts, _, err := uitest.BrowserOptionsFor(defaultURL)
	if err != nil {
		return testv1.StepRunResult{}, err
	}
	if _, err := sc.EnsureBrowser(browserOpts); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("ensure browser: %w", err)
	}
	b, err := sc.Browser()
	if err != nil {
		return testv1.StepRunResult{}, err
	}

	const iterations = 24
	for i := 1; i <= iterations; i++ {
		var href string
		runErr := sc.RunBrowser(chromedp.Evaluate(`window.location.href`, &href))
		if runErr != nil {
			msg := strings.ToLower(strings.TrimSpace(runErr.Error()))
			if strings.Contains(msg, "context canceled") {
				ctxDone := false
				ctxErr := ""
				select {
				case <-b.Context().Done():
					ctxDone = true
					if err := b.Context().Err(); err != nil {
						ctxErr = err.Error()
					}
				default:
				}
				sess := b.ChromeSession()
				pid := 0
				port := 0
				ws := ""
				reachable := false
				targetsSnapshot := ""
				if sess != nil {
					pid = sess.PID
					port = sess.Port
					ws = strings.TrimSpace(sess.WebSocketURL)
					reachable = tcpReachable(port)
					targetsSnapshot = snapshotTargets(port)
				}
				return testv1.StepRunResult{}, fmt.Errorf(
					"context canceled during diagnostic iteration=%d/%d pid=%d port=%d ws=%q ctx_done=%t ctx_err=%q tcp_reachable=%t targets=%s",
					i, iterations, pid, port, ws, ctxDone, ctxErr, reachable, targetsSnapshot,
				)
			}
			return testv1.StepRunResult{}, fmt.Errorf("diagnostic iteration %d failed: %w", i, runErr)
		}
		time.Sleep(80 * time.Millisecond)
		_ = href
	}

	return testv1.StepRunResult{Report: "attach diagnostic passed: no context canceled across repeated browser evals"}, nil
}

func tcpReachable(port int) bool {
	if port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func snapshotTargets(port int) string {
	if port <= 0 {
		return "[]"
	}
	u := fmt.Sprintf("http://127.0.0.1:%d/json/list", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(u)
	if err != nil {
		return fmt.Sprintf("[error:%q]", err.Error())
	}
	defer resp.Body.Close()
	var rows []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		return fmt.Sprintf("[decode-error:%q]", err.Error())
	}
	trim := make([]string, 0, len(rows))
	for i, r := range rows {
		if i >= 5 {
			break
		}
		trim = append(trim, fmt.Sprintf("{id:%q type:%q url:%q}", r.ID, r.Type, r.URL))
	}
	return "[" + strings.Join(trim, " ") + "]"
}
