package main

import (
	"errors"
	"flag"
	"os"

	"dialtone/dev/plugins/logs/src_v1/go"
	smoke "dialtone/dev/plugins/lyra3/src_v1/test/01_smoke"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("lyra3 test", flag.ContinueOnError)
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("lyra3 test parse failed: %v", err)
		os.Exit(1)
	}

	reg := testv1.NewRegistry()
	smoke.Register(reg)

	logs.Info("Starting Lyra3 src_v1 suite with %d registered steps", len(reg.Steps))
	runErr := reg.Run(testv1.SuiteOptions{
		Version:       "lyra3-src-v1",
		ReportPath:    "plugins/lyra3/src_v1/test/TEST.md",
		RawReportPath: "plugins/lyra3/src_v1/test/TEST_RAW.md",
		ReportFormat:  "template",
		ReportTitle:   "Lyra3 Plugin src_v1 Test Report",
		ReportRunner:  "test/src_v1",
		LogPath:       "plugins/lyra3/src_v1/test/test.log",
		ErrorLogPath:  "plugins/lyra3/src_v1/test/error.log",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.lyra3.src-v1",
		AutoStartNATS: true,
	})
	if runErr != nil {
		logs.Error("Lyra3 src_v1 suite failed: %v", runErr)
		os.Exit(1)
	}
	logs.Info("Lyra3 src_v1 suite passed")
}
