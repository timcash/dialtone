package main

import (
	"os"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	repl "dialtone/dev/plugins/repl/src_v1/go/repl"
)

func main() {
	logs.SetOutput(os.Stdout)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error

	switch cmd {
	case "run":
		err = repl.RunLocal(nil, args)
	case "serve":
		err = repl.RunServe(args)
	case "join":
		err = repl.RunJoin(args)
	case "status":
		err = repl.RunStatus(args)
	case "service":
		err = repl.RunService(args)
	case "version":
		logs.Raw("%s", repl.BuildVersion)
		return
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		logs.Error("repld %s failed: %v", cmd, err)
		os.Exit(1)
	}
}

func printUsage() {
	logs.Raw("Usage: repl-src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  run [--name HOST]")
	logs.Raw("  serve [--nats-url URL] [--room NAME] [--embedded-nats] [--tsnet] [--hostname HOST]")
	logs.Raw("  join [--nats-url URL] [--room NAME] [--name HOST]")
	logs.Raw("  status [--nats-url URL] [--room NAME]")
	logs.Raw("  service [--mode install|run|status] [--repo owner/repo] [--nats-url URL] [--room NAME] [--check-interval 3m]")
	logs.Raw("  version")
	logs.Raw("  help")
}
