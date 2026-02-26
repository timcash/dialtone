package main

import (
	"os"

	autoswap "dialtone/dev/plugins/autoswap/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "run":
		err = autoswap.Run(args)
	case "stage":
		err = autoswap.Stage(args)
	case "service":
		err = autoswap.RunService(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		logs.Error("unknown command: %s", cmd)
		usage()
		os.Exit(1)
	}
	if err != nil {
		logs.Error("autoswap failed: %v", err)
		os.Exit(1)
	}
}

func usage() {
	logs.Raw("Usage: dialtone_autoswap_v1 <command> [args]")
	logs.Raw("Commands:")
	logs.Raw("  stage [--manifest PATH] [--repo-root PATH]")
	logs.Raw("  run [--manifest PATH] [--repo-root PATH] [--listen :18084] [--nats-port 18226] [--nats-ws-port 18227]")
	logs.Raw("  service [--mode install|run|start|stop|restart|status|is-active|list] [--repo owner/repo] [--check-interval 5m]")
}
