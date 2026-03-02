package cli

import (
	"flag"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func handleClickCmd(args []string) {
	selector := ""
	parseArgs := make([]string, 0, len(args))
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if selector == "" && !strings.HasPrefix(a, "-") {
			selector = a
			continue
		}
		parseArgs = append(parseArgs, a)
	}

	fs := flag.NewFlagSet("chrome click", flag.ExitOnError)
	host := fs.String("host", "", "Target host name, or 'all'")
	role := fs.String("role", "dev", "Role tag to reuse/start")
	url := fs.String("url", "about:blank", "Optional URL to ensure before click")
	servicePort := fs.Int("service-port", defaultChromeServicePort, "Remote chrome service command port")
	_ = fs.Parse(parseArgs)

	if strings.TrimSpace(selector) == "" {
		logs.Fatal("click requires selector (example: form-submit-button or #form-submit-button)")
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		logs.Fatal("click requires --host")
	}

	nodes, err := resolveChromeHosts(target)
	if err != nil {
		logs.Fatal("click --host: %v", err)
	}

	ok := 0
	fail := 0
	targetURL := normalizeOpenURL(strings.TrimSpace(*url))
	for _, node := range nodes {
		t := strings.TrimSpace(node.Name)
		resp, err := requestRemoteServiceAction(t, *servicePort, actionRequest{
			debugURLRequest: debugURLRequest{
				Role:         strings.TrimSpace(*role),
				Headless:     false,
				URL:          targetURL,
				Reuse:        true,
				DebugAddress: "0.0.0.0",
			},
			Action:   "click",
			Selector: selector,
		})
		if err != nil {
			fail++
			logs.Warn("click host=%s failed: %v", t, err)
			continue
		}
		for _, l := range resp.Logs {
			if strings.TrimSpace(l) != "" {
				logs.Info("click host=%s browser-log: %s", t, strings.TrimSpace(l))
			}
		}
		ok++
		logs.Info("click host=%s ok pid=%d port=%d", t, resp.PID, resp.Port)
	}
	if ok == 0 {
		logs.Fatal("click failed on all targets (%d)", fail)
	}
	if fail > 0 {
		logs.Warn("click completed with partial failures: ok=%d fail=%d", ok, fail)
	} else {
		logs.Info("click completed: ok=%d", ok)
	}
}
