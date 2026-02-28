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
	commonFlags := testv1.BindCommonTestFlags(fs, testv1.CommonTestCLIOptions{
		RemoteDebugPort:  9333,
		RemoteDebugPorts: []int{9333},
	})
	if err := fs.Parse(os.Args[1:]); err != nil {
		logs.Error("earth test parse failed: %v", err)
		os.Exit(1)
	}
	common, err := commonFlags.Resolve()
	if err != nil {
		logs.Error("earth test parse failed: %v", err)
		os.Exit(1)
	}

	attach := strings.TrimSpace(common.AttachNode)
	url := strings.TrimSpace(common.TargetURL)
	common.ApplyRuntimeConfig()
	if attach != "" {
		logs.Info("earth test attach mode node=%s url=%s", attach, url)
	}

	reg := testv1.NewRegistry()
	buildui.Register(reg, buildui.Options{
		AttachNode: attach,
		TargetURL:  url,
	})
	if err := reg.Run(common.ApplySuiteOptions(testv1.SuiteOptions{
		Version:            "earth-src-v1",
		NATSURL:            "nats://127.0.0.1:4222",
		NATSSubject:        "logs.test.earth-src-v1",
		AutoStartNATS:      true,
		ReportPath:         "plugins/earth/src_v1/test/TEST.md",
		BrowserCleanupRole: "earth-test",
	})); err != nil {
		logs.Error("earth src_v1 suite failed: %v", err)
		os.Exit(1)
	}
}
