package main

import (
	"fmt"
	"os"
	"strings"

	cloudflare_ops "dialtone/dev/plugins/cloudflare/src_v1/cmd/ops"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, args, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old cloudflare CLI order is deprecated. Use: ./dialtone.sh cloudflare src_v1 <command> [args]")
	}

	if version != "src_v1" {
		logs.Error("unsupported cloudflare version: %s", version)
		os.Exit(1)
	}

	if err := runSrcV1(command, args); err != nil {
		logs.Error("cloudflare error: %v", err)
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh cloudflare src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first cloudflare argument (for example: ./dialtone.sh cloudflare src_v1 tunnel list)")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runSrcV1(command string, args []string) error {
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return cloudflare_ops.Install()
	case "fmt":
		return cloudflare_ops.Fmt()
	case "format":
		return cloudflare_ops.Format()
	case "vet":
		return cloudflare_ops.Vet()
	case "go-build":
		return cloudflare_ops.GoBuild()
	case "lint":
		return cloudflare_ops.Lint()
	case "build":
		return cloudflare_ops.Build()
	case "dev":
		return cloudflare_ops.Dev()
	case "ui-run":
		port, err := cloudflare_ops.ParseUIRunPort(args)
		if err != nil {
			return err
		}
		return cloudflare_ops.UIRun(port)
	case "test":
		return cloudflare_ops.Test("src_v1")
	case "serve":
		// src_v1 serve without args runs plugin HTTP UI.
		if len(args) == 0 {
			return cloudflare_ops.Serve()
		}
		// src_v1 serve with args delegates runtime tunnel serve behavior.
		return cloudflare_ops.RunRuntime("serve", args)
	case "login", "tunnel", "robot", "proxy", "provision", "setup-service":
		return cloudflare_ops.RunRuntime(command, args)
	default:
		return fmt.Errorf("unknown cloudflare command: %s", command)
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh cloudflare src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install      Install UI dependencies")
	logs.Raw("  fmt          Run formatting checks/fixes")
	logs.Raw("  format       Run UI format checks")
	logs.Raw("  vet          Run go vet checks")
	logs.Raw("  go-build     Run go build checks")
	logs.Raw("  lint         Run lint checks")
	logs.Raw("  build        Build UI assets")
	logs.Raw("  dev          Run UI in development mode")
	logs.Raw("  ui-run       Run UI dev server")
	logs.Raw("  test         Run automated tests")
	logs.Raw("  serve        Run cloudflare UI server (no args) or tunnel serve (with args)")
	logs.Raw("  login        Authenticate with Cloudflare")
	logs.Raw("  tunnel       Manage Cloudflare tunnels (create/list/status/run/start/route/cleanup/stop)")
	logs.Raw("  robot        Expose a remote robot via tunnel")
	logs.Raw("  proxy        Start local TCP proxy")
	logs.Raw("  provision    Create tunnel + DNS and store token in env/.env")
	logs.Raw("  setup-service Install cloudflare robot proxy as a service")
}
