package test_test

import (
	"fmt"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
)

func init() {
	test.Register("metadata-sync", "test-test-tags", []string{"metadata", "core"}, RunMetadataSync)
	test.Register("camera-filter-overlay", "test-test-tags", []string{"camera-filters", "imaging"}, RunCameraFilter)
	test.Register("red-team-pentest", "test-test-tags", []string{"red-team", "security"}, RunRedTeam)
	test.Register("secure-camera-config", "test-test-tags", []string{"camera-filters", "red-team"}, RunSecureCamera)
}

// RunAll is the standard entry point required by project rules.
// It now uses the registry to find and run all tests for this ticket.
func RunAll() error {
	logger.LogInfo("Running test-test-tags suite...")
	return test.RunTicket("test-test-tags")
}

func RunMetadataSync() error {
	fmt.Println("PASS: [metadata] Synchronizing system metadata")
	return nil
}

func RunCameraFilter() error {
	fmt.Println("PASS: [camera-filters] Applying gaussian blur overlay")
	return nil
}

func RunRedTeam() error {
	fmt.Println("PASS: [red-team] Perimeter breach simulation")
	return nil
}

func RunSecureCamera() error {
	fmt.Println("PASS: [camera-filters][red-team] HARDENING: Validating camera feed encryption")
	return nil
}