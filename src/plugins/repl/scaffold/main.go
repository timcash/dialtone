package main

import (
	"fmt"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	replv3 "dialtone/dev/plugins/repl/src_v3/go/repl"
)

func main() {
	logs.SetOutput(os.Stdout)
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	version, command, rest, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old repl CLI order is deprecated. Use: ./dialtone.sh repl src_v3 <command> [args]")
	}
	if version == "src_v3" {
		switch command {
		case "install":
			if err := replv3.RunInstall(rest); err != nil {
				logs.Error("repl v3 install failed: %v", err)
				os.Exit(1)
			}
		case "format", "fmt":
			if err := replv3.RunFormat(rest); err != nil {
				logs.Error("repl v3 format failed: %v", err)
				os.Exit(1)
			}
		case "build":
			if err := replv3.RunBuild(rest); err != nil {
				logs.Error("repl v3 build failed: %v", err)
				os.Exit(1)
			}
		case "lint":
			if err := replv3.RunLint(rest); err != nil {
				logs.Error("repl v3 lint failed: %v", err)
				os.Exit(1)
			}
		case "check":
			if err := replv3.RunCheck(rest); err != nil {
				logs.Error("repl v3 check failed: %v", err)
				os.Exit(1)
			}
		case "run":
			if err := replv3.Run(rest); err != nil {
				logs.Error("repl v3 run failed: %v", err)
				os.Exit(1)
			}
		case "leader":
			if err := replv3.RunLeader(rest); err != nil {
				logs.Error("repl v3 leader failed: %v", err)
				os.Exit(1)
			}
		case "join":
			if err := replv3.RunJoin(rest); err != nil {
				logs.Error("repl v3 join failed: %v", err)
				os.Exit(1)
			}
		case "inject":
			if err := replv3.Inject(rest); err != nil {
				logs.Error("repl v3 inject failed: %v", err)
				os.Exit(1)
			}
		case "bootstrap":
			if err := replv3.RunBootstrap(rest); err != nil {
				logs.Error("repl v3 bootstrap failed: %v", err)
				os.Exit(1)
			}
		case "bootstrap-http":
			if err := replv3.RunBootstrapHTTP(rest); err != nil {
				logs.Error("repl v3 bootstrap-http failed: %v", err)
				os.Exit(1)
			}
		case "add-host":
			if err := replv3.AddHost(rest); err != nil {
				logs.Error("repl v3 add-host failed: %v", err)
				os.Exit(1)
			}
		case "status":
			if err := replv3.RunStatus(rest); err != nil {
				logs.Error("repl v3 status failed: %v", err)
				os.Exit(1)
			}
		case "service":
			if err := replv3.RunService(rest); err != nil {
				logs.Error("repl v3 service failed: %v", err)
				os.Exit(1)
			}
		case "test":
			if err := replv3.RunTest(rest); err != nil {
				logs.Error("repl v3 test failed: %v", err)
				os.Exit(1)
			}
		case "task":
			if err := replv3.RunTask(rest); err != nil {
				logs.Error("repl v3 task command failed: %v", err)
				os.Exit(1)
			}
		case "watch":
			if err := replv3.RunWatch(rest); err != nil {
				logs.Error("repl v3 watch failed: %v", err)
				os.Exit(1)
			}
		case "test-clean":
			if err := replv3.RunTestClean(rest); err != nil {
				logs.Error("repl v3 test-clean failed: %v", err)
				os.Exit(1)
			}
		case "process-clean":
			if err := replv3.RunProcessClean(rest); err != nil {
				logs.Error("repl v3 process-clean failed: %v", err)
				os.Exit(1)
			}
		case "version":
			logs.Raw("src_v3")
		case "help", "-h", "--help":
			printUsage()
		default:
			logs.Error("Unsupported repl src_v3 command: %s", command)
			os.Exit(1)
		}
		return
	}
	logs.Error("Unsupported repl version: %s (supported: src_v3)", version)
	os.Exit(1)
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v3", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh repl src_v3 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first repl argument (usage: ./dialtone.sh repl src_v3 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh repl src_v3 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands (src_v3):")
	logs.Raw("  install                                              Verify managed Go toolchain for REPL workflows")
	logs.Raw("  format|fmt                                           Run go fmt on REPL packages")
	logs.Raw("  lint                                                 Run go vet on REPL packages")
	logs.Raw("  check                                                Compile-check REPL v3 and scaffold packages")
	logs.Raw("  build                                                Build REPL scaffold/binaries/packages")
	logs.Raw("  run [--nats-url URL] [--room NAME] [--name USER]")
	logs.Raw("  leader [--nats-url URL] [--room NAME] [--embedded-nats] [--tsnet] [--tsnet-nats-port PORT] [--hostname HOST]")
	logs.Raw("  join [room-name] [--nats-url URL] [--name HOST] [--room NAME]")
	logs.Raw("  inject --user NAME [--host HOST] [--nats-url URL] [--room NAME] <command>")
	logs.Raw("  bootstrap [--apply] [--wsl-host HOST] [--wsl-user USER]  Show/apply first-host bootstrap guide")
	logs.Raw("  bootstrap-http [--host 127.0.0.1] [--port 8811]         Serve /install.sh + /dialtone.sh + /dialtone-main.tar.gz")
	logs.Raw("  add-host --name wsl --host HOST --user USER              Add/update mesh host in env/dialtone.json")
	logs.Raw("  status [--nats-url URL] [--room NAME]")
	logs.Raw("  service [--mode install|run|status] [--repo owner/repo] [--nats-url URL] [--room NAME] [--hostname HOST] [--check-interval 5m] [--embedded-nats] [--tsnet] [--tsnet-nats-port PORT]")
	logs.Raw("  task list [--count N] [--state all|running|done] [--nats-url URL]")
	logs.Raw("  task show --task-id TASK_ID [--nats-url URL]")
	logs.Raw("  task log --task-id TASK_ID [--lines N] [--nats-url URL]")
	logs.Raw("  task kill --task-id TASK_ID [--nats-url URL]")
	logs.Raw("  test [--filter EXPR] [--real] [--require-embedded-tsnet] [--wsl-host HOST] [--wsl-user USER] [--tunnel-name NAME] [--tunnel-url URL] [--install-url URL] [--bootstrap-repo-url URL]")
	logs.Raw("  watch [--nats-url URL] [--subject repl.>] [--filter TEXT]  Stream NATS room/events")
	logs.Raw("  test-clean [--dry-run]                               Remove REPL src_v3 /tmp bootstrap test folders")
	logs.Raw("  process-clean [--dry-run] [--include-chrome]        Stop REPL task workers, bootstrap-http, cloudflare, and known dialtone LaunchAgents")
}
