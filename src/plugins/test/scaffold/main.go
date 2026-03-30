package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1"
)

func main() {
	if err := testv1.Run(os.Args[1:]); err != nil {
		logs.Error("test error: %v", err)
		os.Exit(1)
	}
}
