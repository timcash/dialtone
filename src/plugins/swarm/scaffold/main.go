package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	swarmv3 "dialtone/dev/plugins/swarm/src_v3/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if err := swarmv3.Run(os.Args[1:]); err != nil {
		logs.Error("swarm error: %v", err)
		os.Exit(1)
	}
}
