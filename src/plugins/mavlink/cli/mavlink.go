package cli

import (
	"fmt"
	"os"

	"dialtone/cli/src/plugins/mavlink/app"
)

func RunMavlink(args []string) {
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	// subArgs := args[1:]

	switch command {
	case "arm":
		runArm()
	case "disarm":
		runDisarm()
	default:
		fmt.Printf("Unknown mavlink command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev mavlink <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  arm      Arm the robot")
	fmt.Println("  disarm   Disarm the robot")
}

func runArm() {
	fmt.Println("Arming robot...")
	// This would normally go through NATS, but for direct CLI test:
	endpoint := os.Getenv("MAVLINK_ENDPOINT")
	if endpoint == "" {
		fmt.Println("Error: MAVLINK_ENDPOINT not set")
		return
	}

	svc, err := mavlink.NewMavlinkService(mavlink.MavlinkConfig{
		Endpoint: endpoint,
	})
	if err != nil {
		fmt.Printf("Error creating MAVLink service: %v\n", err)
		return
	}
	defer svc.Close()

	if err := svc.Arm(); err != nil {
		fmt.Printf("Error arming: %v\n", err)
	} else {
		fmt.Println("Arm command sent")
	}
}

func runDisarm() {
	fmt.Println("Disarming robot...")
	endpoint := os.Getenv("MAVLINK_ENDPOINT")
	if endpoint == "" {
		fmt.Println("Error: MAVLINK_ENDPOINT not set")
		return
	}

	svc, err := mavlink.NewMavlinkService(mavlink.MavlinkConfig{
		Endpoint: endpoint,
	})
	if err != nil {
		fmt.Printf("Error creating MAVLink service: %v\n", err)
		return
	}
	defer svc.Close()

	if err := svc.Disarm(); err != nil {
		fmt.Printf("Error disarming: %v\n", err)
	} else {
		fmt.Println("Disarm command sent")
	}
}
