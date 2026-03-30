package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	logsv1 "dialtone/dev/plugins/logs/src_v1"
)

func main() {
	if err := logsv1.Run(os.Args[1:]); err != nil {
		logs.Error("logs error: %v", err)
		os.Exit(1)
	}
}
