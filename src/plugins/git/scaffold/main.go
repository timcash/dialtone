package main

import (
	"os"

	gitv1 "dialtone/dev/plugins/git/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if err := gitv1.Run(os.Args[1:]); err != nil {
		logs.Error("git error: %v", err)
		os.Exit(1)
	}
}
