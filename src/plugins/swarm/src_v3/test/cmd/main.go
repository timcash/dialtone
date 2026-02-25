package main

import (
	"flag"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	localtest "dialtone/dev/plugins/swarm/src_v3/test/01_local"
	rendezvoustest "dialtone/dev/plugins/swarm/src_v3/test/02_rendezvous"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	fs := flag.NewFlagSet("swarm-src-v3-test", flag.ContinueOnError)
	mode := fs.String("mode", "all", "local|rendezvous|all")
	rendezvousURL := fs.String("rendezvous-url", "https://relay.dialtone.earth", "Rendezvous URL")
	if err := fs.Parse(os.Args[1:]); err != nil {
		logs.Error("swarm src_v3 tests argument error: %v", err)
		os.Exit(1)
	}

	reg := testv1.NewRegistry()
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "local":
		localtest.Register(reg)
	case "rendezvous":
		rendezvoustest.Register(reg, *rendezvousURL)
	case "all":
		localtest.Register(reg)
		rendezvoustest.Register(reg, *rendezvousURL)
	default:
		logs.Error("unsupported mode %s (expected local|rendezvous|all)", *mode)
		os.Exit(1)
	}

	logs.Info("Running swarm src_v3 tests in single process (%d steps, mode=%s)", len(reg.Steps), strings.TrimSpace(*mode))
	err := reg.Run(testv1.SuiteOptions{
		Version:       "swarm-src-v3",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.swarm-src-v3",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("swarm src_v3 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("swarm src_v3 tests passed")
}
