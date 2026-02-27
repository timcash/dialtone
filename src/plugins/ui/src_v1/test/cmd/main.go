package main

import (
	"errors"
	"flag"
	"os"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"dialtone/dev/plugins/ui/src_v1/test"
	qualitychecks "dialtone/dev/plugins/ui/src_v1/test/00_quality_checks"
	buildserve "dialtone/dev/plugins/ui/src_v1/test/01_build_and_serve"
	navigation "dialtone/dev/plugins/ui/src_v1/test/02_sections_navigation"
	components "dialtone/dev/plugins/ui/src_v1/test/03_component_actions"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("ui test", flag.ContinueOnError)
	attachNode := fs.String("attach", "", "Attach test browser to headed browser on mesh node (example: chroma)")
	targetURL := fs.String("url", "", "URL for browser steps (defaults to local served dist or inferred dev URL when --attach is set)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("ui test parse failed: %v", err)
		os.Exit(1)
	}
	attach := strings.TrimSpace(*attachNode)
	url := strings.TrimSpace(*targetURL)
	test.SetOptions(test.Options{
		AttachNode: attach,
		TargetURL:  url,
	})
	if attach != "" {
		_ = os.Setenv("DIALTONE_TEST_BROWSER_NODE", attach)
		logs.Info("ui test attach mode node=%s url=%s", attach, url)
	} else {
		_ = os.Unsetenv("DIALTONE_TEST_BROWSER_NODE")
	}

	reg := test.NewRegistry()
	qualitychecks.Register(reg)
	buildserve.Register(reg)
	navigation.Register(reg)
	components.Register(reg)

	logs.Info("Starting UI src_v1 suite with %d registered steps", len(reg.Steps))
	preserveAttachBrowser := attach != ""
	runErr := reg.Run(testv1.SuiteOptions{
		Version:               "ui-src-v1",
		ReportPath:            "plugins/ui/src_v1/test/TEST.md",
		RawReportPath:         "plugins/ui/src_v1/test/TEST_RAW.md",
		ReportFormat:          "template",
		ReportTitle:           "UI Plugin src_v1 Test Report",
		ReportRunner:          "test/src_v1",
		LogPath:               "plugins/ui/src_v1/test/test.log",
		ErrorLogPath:          "plugins/ui/src_v1/test/error.log",
		NATSURL:               "nats://127.0.0.1:4222",
		NATSSubject:           "logs.test.ui.src-v1",
		AutoStartNATS:         true,
		BrowserCleanupRole:    "ui-test",
		PreserveSharedBrowser: preserveAttachBrowser,
		SkipBrowserCleanup:    preserveAttachBrowser,
	})
	if runErr != nil {
		logs.Error("UI src_v1 suite failed: %v", runErr)
		os.Exit(1)
	}
	logs.Info("UI src_v1 suite passed")
}
