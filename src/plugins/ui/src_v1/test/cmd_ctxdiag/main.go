package main

import (
	"flag"
	"os"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"dialtone/dev/plugins/ui/src_v1/test"
	ctxdiag "dialtone/dev/plugins/ui/src_v1/test/02_context_cancel_diagnose"
)

func main() {
	logs.SetOutput(os.Stdout)

	fs := flag.NewFlagSet("ui-src-v1-ctxdiag", flag.ExitOnError)
	attachNode := fs.String("attach", "", "Attach to headed browser on mesh node")
	targetURL := fs.String("url", "", "Browser target URL")
	actionsPerMinute := fs.Float64("apm", 300, "Throttle browser actions in actions per minute")
	_ = fs.Parse(os.Args[1:])

	attach := strings.TrimSpace(*attachNode)
	url := strings.TrimSpace(*targetURL)
	test.SetOptions(test.Options{
		AttachNode:       attach,
		TargetURL:        url,
		ActionsPerMinute: *actionsPerMinute,
	})
	testv1.SetActionsPerMinute(*actionsPerMinute)
	if attach != "" {
		testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
			cfg.BrowserNode = attach
			cfg.RemoteRequireRole = true
		})
		if cfg, err := test.LoadBrowserDebugConfig(); err == nil && cfg != nil && cfg.PID > 0 {
			testv1.UpdateRuntimeConfig(func(rc *testv1.RuntimeConfig) {
				rc.RemoteBrowserPID = cfg.PID
			})
		}
	} else {
		testv1.SetRuntimeConfig(testv1.RuntimeConfig{})
	}

	reg := test.NewRegistry()
	ctxdiag.Register(reg)
	runErr := reg.Run(testv1.SuiteOptions{
		Version:               "ui-src-v1-ctxdiag",
		ReportPath:            "plugins/ui/src_v1/CTX_DIAG_TEST.md",
		RawReportPath:         "plugins/ui/src_v1/CTX_DIAG_TEST_RAW.md",
		ReportFormat:          "template",
		ReportTitle:           "UI src_v1 Context Canceled Diagnostic",
		ReportRunner:          "test/src_v1",
		NATSURL:               "nats://127.0.0.1:4222",
		NATSSubject:           "logs.test.ui.src-v1.ctxdiag",
		AutoStartNATS:         true,
		BrowserCleanupRole:    "ui-test",
		PreserveSharedBrowser: attach != "",
		SkipBrowserCleanup:    attach != "",
	})
	if runErr != nil {
		logs.Error("ui src_v1 context-cancel diagnostic failed: %v", runErr)
		os.Exit(1)
	}
	logs.Info("ui src_v1 context-cancel diagnostic passed")
}
