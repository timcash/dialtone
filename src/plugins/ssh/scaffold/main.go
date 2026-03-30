package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if err := sshv1.Run(os.Args[1:]); err != nil {
		logs.Error("ssh error: %v", err)
		os.Exit(1)
	}
}
