package test

import (
	"dialtone/cli/src/core/config"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/test"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	test.Register("multi-peer-connection", "swarm", []string{"plugin", "swarm", "network"}, RunMultiPeerConnection)
}

// RunAll is the standard entry point required by project rules.
// It uses the registry to find and run all tests for this plugin.
func RunAll() error {
	logger.LogInfo("Running swarm plugin suite...")
	return test.RunPlugin("swarm")
}

func RunMultiPeerConnection() error {
	fmt.Println("[swarm] Running multi-peer integration test...")

	// Use project root for appDir
	appDir := filepath.Join("src", "plugins", "swarm", "app")

	envPath := config.GetDialtoneEnv()
	if envPath == "" {
		return fmt.Errorf("DIALTONE_ENV is not set. Please set it in env/.env or pass --env.")
	}
	pearBin := filepath.Join(envPath, "node", "bin", "pear")
	if _, err := os.Stat(pearBin); err != nil {
		pearBin = filepath.Join(envPath, "bin", "pear")
		if _, err := os.Stat(pearBin); err != nil {
			return fmt.Errorf("pear not found in DIALTONE_ENV (%s). Run ./dialtone.sh swarm install to link it or install Pear into DIALTONE_ENV", envPath)
		}
	}

	// Run peer-a and peer-b in parallel
	fmt.Println("[swarm] Starting Peer A...")
	cmdA := exec.Command(pearBin, "run", "./test.js", "peer-a", "test-topic")
	cmdA.Dir = appDir
	cmdA.Stdout = os.Stdout
	cmdA.Stderr = os.Stderr

	fmt.Println("[swarm] Starting Peer B...")
	cmdB := exec.Command(pearBin, "run", "./test.js", "peer-b", "test-topic")
	cmdB.Dir = appDir
	cmdB.Stdout = os.Stdout
	cmdB.Stderr = os.Stderr

	// Start both
	errA := cmdA.Start()
	errB := cmdB.Start()

	if errA != nil || errB != nil {
		return fmt.Errorf("failed to start test processes: %v, %v", errA, errB)
	}

	// Wait for both to finish
	errA = cmdA.Wait()
	errB = cmdB.Wait()

	if errA != nil || errB != nil {
		return fmt.Errorf("one or more peers failed to complete test")
	}

	fmt.Println("[swarm] Multi-peer test passed!")
	return nil
}
