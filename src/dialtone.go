package dialtone

import (
	"fmt"
	"os"

	"dialtone/cli/src/core/config"
	dag_cli "dialtone/cli/src/plugins/dag/cli"
	nix_cli "dialtone/cli/src/plugins/nix/cli"
	template_cli "dialtone/cli/src/plugins/template/cli"
	"dialtone/cli/src/plugins/robot"
	"dialtone/cli/src/plugins/vpn"
)

func Execute() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	config.LoadConfig()

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "start":
		robot.RunRobot(append([]string{"start"}, args...))
	case "robot":
		robot.RunRobot(args)
	case "vpn":
		vpn.RunVPN(args)
	case "nix":
		if err := nix_cli.Run(args); err != nil {
			fmt.Printf("Nix command error: %v\n", err)
			os.Exit(1)
		}
	case "dag":
		if err := dag_cli.Run(args); err != nil {
			fmt.Printf("DAG command error: %v\n", err)
			os.Exit(1)
		}
	case "template":
		if err := template_cli.Run(args); err != nil {
			fmt.Printf("Template command error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  start         Start the robot services (NATS, Web, Mavlink)")
	fmt.Println("  robot         Robot plugin commands (deploy, sync-code, test)")
	fmt.Println("  vpn           VPN plugin commands")
	fmt.Println("  nix           Nix plugin commands")
	fmt.Println("  dag           DAG plugin commands")
	fmt.Println("  template      Template plugin commands")
}
