package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"dialtone/dev/plugins/ui/src_v1/test"
	qualitychecks "dialtone/dev/plugins/ui/src_v1/test/00_quality_checks"
	buildserve "dialtone/dev/plugins/ui/src_v1/test/01_build_and_serve"
	sectionhero "dialtone/dev/plugins/ui/src_v1/test/02_section_hero"
	sectionthreefullscreen "dialtone/dev/plugins/ui/src_v1/test/03_section_three_fullscreen"
	sectionthreecalculator "dialtone/dev/plugins/ui/src_v1/test/04_section_three_calculator"
	sectiontable "dialtone/dev/plugins/ui/src_v1/test/05_section_table"
	sectioncamera "dialtone/dev/plugins/ui/src_v1/test/06_section_camera"
	sectiondocs "dialtone/dev/plugins/ui/src_v1/test/07_section_docs"
	sectionterminal "dialtone/dev/plugins/ui/src_v1/test/08_section_terminal"
	sectionsettings "dialtone/dev/plugins/ui/src_v1/test/09_section_settings"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("ui test", flag.ContinueOnError)
	attachNode := fs.String("attach", "", "Attach test browser to headed browser on mesh node (example: chroma)")
	targetURL := fs.String("url", "", "URL for browser steps (defaults to local served dist or inferred dev URL when --attach is set)")
	clicksPerSecond := fs.Float64("cps", 5, "Throttle UI clicks/taps in clicks per second (example: --cps 1)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("ui test parse failed: %v", err)
		os.Exit(1)
	}
	if *clicksPerSecond < 0 {
		logs.Error("ui test parse failed: --cps must be >= 0")
		os.Exit(1)
	}
	attach := strings.TrimSpace(*attachNode)
	url := strings.TrimSpace(*targetURL)
	testv1.SetClicksPerSecond(*clicksPerSecond)
	test.SetOptions(test.Options{
		AttachNode:      attach,
		TargetURL:       url,
		ClicksPerSecond: *clicksPerSecond,
	})
	if attach != "" {
		_ = os.Setenv("DIALTONE_TEST_BROWSER_NODE", attach)
		logs.Info("ui test remote attach mode (headed) node=%s url=%s cps=%.3f", attach, url, *clicksPerSecond)
		if cfg, err := test.LoadBrowserDebugConfig(); err == nil && cfg != nil && cfg.PID > 0 && strings.EqualFold(strings.TrimSpace(cfg.Role), "ui-dev") {
			_ = os.Setenv("DIALTONE_TEST_REMOTE_BROWSER_PID", fmt.Sprintf("%d", cfg.PID))
			logs.Info("ui test remote attach pid hint: %d", cfg.PID)
		} else {
			_ = os.Unsetenv("DIALTONE_TEST_REMOTE_BROWSER_PID")
		}
	} else {
		_ = os.Unsetenv("DIALTONE_TEST_BROWSER_NODE")
		_ = os.Unsetenv("DIALTONE_TEST_REMOTE_BROWSER_PID")
		logs.Info("ui test local mode url=%s cps=%.3f", url, *clicksPerSecond)
	}

	var inventoryBefore *test.RemoteBrowserInventory
	if attach != "" {
		before, err := test.LogRemoteBrowserInventory(attach, "before")
		if err != nil {
			logs.Warn("ui attach preflight inventory failed on node=%s: %v", attach, err)
		} else {
			inventoryBefore = before
		}
	}

	reg := test.NewRegistry()
	qualitychecks.Register(reg)
	buildserve.Register(reg)
	sectionhero.Register(reg)
	sectionthreefullscreen.Register(reg)
	sectionthreecalculator.Register(reg)
	sectiontable.Register(reg)
	sectioncamera.Register(reg)
	sectiondocs.Register(reg)
	sectionterminal.Register(reg)
	sectionsettings.Register(reg)

	logs.Info("Starting UI src_v1 suite with %d registered steps", len(reg.Steps))
	preserveBrowser := attach != ""
	skipBrowserCleanup := attach != ""
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
		PreserveSharedBrowser: preserveBrowser,
		SkipBrowserCleanup:    skipBrowserCleanup,
	})
	if attach != "" {
		inventoryAfter, err := test.LogRemoteBrowserInventory(attach, "after")
		if err != nil {
			logs.Warn("ui attach postflight inventory failed on node=%s: %v", attach, err)
		}
		if err := test.WriteAttachMetadataReport("plugins/ui/src_v1/test/TEST.md", attach, inventoryBefore, inventoryAfter); err != nil {
			logs.Warn("ui attach report metadata update failed: %v", err)
		}
	}
	if runErr != nil {
		logs.Error("UI src_v1 suite failed: %v", runErr)
		os.Exit(1)
	}
	logs.Info("UI src_v1 suite passed")
}
