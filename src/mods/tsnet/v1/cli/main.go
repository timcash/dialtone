package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return
	case "keepalive":
		if err := runKeepalive(args); err != nil {
			exitIfErr(err, "tsnet keepalive")
		}
	case "bootstrap":
		if err := runBootstrap(args); err != nil {
			exitIfErr(err, "tsnet bootstrap")
		}
	case "install":
		if err := runInstall(args); err != nil {
			exitIfErr(err, "tsnet install")
		}
	case "status":
		if err := runStatus(args); err != nil {
			exitIfErr(err, "tsnet status")
		}
	case "hosts":
		if err := runHosts(args); err != nil {
			exitIfErr(err, "tsnet hosts")
		}
	default:
		printUsage()
		exitIfErr(fmt.Errorf("unknown tsnet v1 command: %s", command), "tsnet v1")
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod tsnet v1 <bootstrap|install|keepalive|status|hosts> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  bootstrap   Ensure TS_AUTHKEY is available and start a persistent tsnet keepalive process")
	fmt.Println("    --host NAME        Hostname override")
	fmt.Println("    --env-file FILE    Env file to store/read TS_AUTHKEY")
	fmt.Println("    --state-dir PATH   tsnet state directory override")
	fmt.Println("    --no-keepalive     Skip tsnet keepalive process")
	fmt.Println("    --skip-acl         Skip ACL updates for mosh ports")
	fmt.Println("    --prefer-native     Do not start keepalive if local tailscale daemon is already running")
	fmt.Println("  keepalive   Start a foreground embedded tsnet keepalive process")
	fmt.Println("    --host NAME        Hostname override")
	fmt.Println("    --state-dir PATH   tsnet state directory override")
	fmt.Println("  install     Ensure tailscale CLI is available via Nix")
	fmt.Println("    --nixpkgs-url URL  Optional nix expr URL for fixed inputs")
	fmt.Println("    --ensure            Install tailscale to nix profile")
	fmt.Println("  status      Show runtime diagnostics for tsnet/tailscale bootstrap")
	fmt.Println("  hosts       List hostnames from local tailscale status")
	fmt.Println("    --format text|json  Output format (default: text)")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
