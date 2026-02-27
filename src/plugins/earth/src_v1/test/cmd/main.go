package main

import (
	"flag"
	"os"
	"strings"

	buildui "dialtone/dev/plugins/earth/src_v1/test/01_build_and_ui"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("earth test", flag.ContinueOnError)
	attachNode := fs.String("attach", "", "Attach test browser to headed browser on mesh node (example: chroma)")
	targetURL := fs.String("url", "", "URL for browser step (defaults to local served dist or inferred dev URL when --attach is set)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		logs.Error("earth test parse failed: %v", err)
		os.Exit(1)
	}

	attach := strings.TrimSpace(*attachNode)
	url := strings.TrimSpace(*targetURL)
	if attach != "" {
		_ = os.Setenv("DIALTONE_TEST_BROWSER_NODE", attach)
		logs.Info("earth test attach mode node=%s url=%s", attach, url)
	} else {
		_ = os.Unsetenv("DIALTONE_TEST_BROWSER_NODE")
	}

	reg := testv1.NewRegistry()
	buildui.Register(reg, buildui.Options{
		AttachNode: attach,
		TargetURL:  url,
	})
	preserveAttachBrowser := attach != ""
	if err := reg.Run(testv1.SuiteOptions{
		Version:               "earth-src-v1",
		NATSURL:               "nats://127.0.0.1:4222",
		NATSSubject:           "logs.test.earth-src-v1",
		AutoStartNATS:         true,
		ReportPath:            "plugins/earth/src_v1/test/TEST.md",
		BrowserCleanupRole:    "earth-test",
		PreserveSharedBrowser: preserveAttachBrowser,
		SkipBrowserCleanup:    preserveAttachBrowser,
	}); err != nil {
		logs.Error("earth src_v1 suite failed: %v", err)
		os.Exit(1)
	}
}
