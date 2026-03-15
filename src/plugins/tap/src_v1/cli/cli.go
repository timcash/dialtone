package cli

import (
	"flag"
	"fmt"
	"time"

	tapv1 "dialtone/dev/plugins/tap/src_v1/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing arguments, expected version as first argument")
	}

	version := args[0]
	if version != "src_v1" {
		if len(args) > 1 && args[1] == "src_v1" {
			version = args[1]
			args = append([]string{args[0]}, args[2:]...)
		} else {
			return fmt.Errorf("expected src_v1 as version argument")
		}
	} else {
		args = args[1:] // shift version
	}

	flags := flag.NewFlagSet("tap", flag.ExitOnError)
	upstream := flags.String("upstream", "nats://127.0.0.1:4222", "Upstream NATS URL")
	subjectsCSV := flags.String("subjects", "repl.>", "Comma-separated subject patterns")
	name := flags.String("name", "dialtone-tap", "NATS client name")
	reconnectWait := flags.Duration("reconnect-wait", 1200*time.Millisecond, "Reconnect wait interval")
	raw := flags.Bool("raw", false, "Print raw payloads instead of REPL-style formatting")
	showSubject := flags.Bool("show-subject", true, "Prefix output lines with subject")
	showReconnects := flags.Bool("show-reconnect-events", true, "Print reconnect lifecycle events")

	if err := flags.Parse(args); err != nil {
		return err
	}

	return tapv1.Run(*upstream, *subjectsCSV, *name, *reconnectWait, *raw, *showSubject, *showReconnects)
}
