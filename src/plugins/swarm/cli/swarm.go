package cli

import (
	"dialtone/cli/src/core/config"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	swarm_test "dialtone/cli/src/plugins/swarm/test"
)

func RunSwarm(args []string) {
	if len(args) < 1 {
		printSwarmUsage()
		return
	}

	subcommand := args[0]
	// Current requirement is just `swarm <topic>`
	// But we can support `swarm join <topic>` or just direct topic if it's not a known subcommand

	subArgs := args[1:]

	switch subcommand {
	case "help", "-h", "--help":
		printSwarmUsage()
	case "install":
		runSwarmInstall(subArgs)
	case "build":
		runSwarmBuild(subArgs)
	case "test":
		runSwarmTest(subArgs)
	default:
		// Assume the first argument is the topic if no other subcommand matches
		runSwarmPear(args)
	}
}

func runSwarmInstall(args []string) {
	fmt.Println("[swarm] Installing dependencies...")
	appDir := filepath.Join("src", "plugins", "swarm", "app")

	npmBin := "npm"
	envPath := config.GetDialtoneEnv()
	if envPath != "" {
		npmBin = filepath.Join(envPath, "node", "bin", "npm")
	}

	cmd := exec.Command(npmBin, "install")
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] npm install failed: %v\n", err)
		os.Exit(1)
	}
}

func runSwarmBuild(args []string) {
	fmt.Println("[swarm] Building plugin (no-op for now)...")
}

func runSwarmTest(args []string) {
	if err := swarm_test.RunMultiPeerConnection(); err != nil {
		fmt.Printf("[swarm] Test failed: %v\n", err)
		os.Exit(1)
	}
}

func printSwarmUsage() {
	fmt.Println("Usage: ./dialtone.sh swarm <topic>")
	fmt.Println("\nConnects to a Hyperswarm topic using Pear runtime.")
}

func runSwarmPear(args []string) {
	topic := args[0]
	fmt.Printf("[swarm] Joining topic: %s\n", topic)

	// Pear app is in src/plugins/swarm/app
	appDir := filepath.Join("src", "plugins", "swarm", "app")

	pearBin := "pear"
	envPath := config.GetDialtoneEnv()
	if envPath != "" {
		// Use node bin for pear if it's installed there, but usually it's in the bin dir of the env or system
		// If installed via npm -g earlier, it might be in the node bin dir
		testPear := filepath.Join(envPath, "node", "bin", "pear")
		if _, err := os.Stat(testPear); err == nil {
			pearBin = testPear
		}
	}

	// Run pear run . <topic>
	cmd := exec.Command(pearBin, "run", ".", topic)
	cmd.Dir = appDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[swarm] Pear execution failed: %v\n", err)
		os.Exit(1)
	}
}
