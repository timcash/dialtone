package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	uiv1 "dialtone/dev/plugins/ui/src_v1"
)

func main() {
	if err := uiv1.Run(os.Args[1:]); err != nil {
		logs.Error("ui error: %v", err)
		os.Exit(1)
	}
}
