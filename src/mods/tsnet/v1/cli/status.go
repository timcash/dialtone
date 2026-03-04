package main

import (
	"fmt"
	"os"
	"strings"
)

func runStatus(args []string) error {
	cfg, err := resolveConfig("", "")
	if err != nil {
		return err
	}

	nativeRunning, nativeTailnet, _ := detectNativeTailscale(cfg.Tailnet)
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if strings.TrimSpace(apiKey) == "" {
		apiKey = os.Getenv("TS_API_KEY")
	}

	fmt.Println("tsnet status:")
	fmt.Printf("  Hostname:    %s\n", cfg.Hostname)
	fmt.Printf("  Tailnet:     %s\n", cfg.Tailnet)
	fmt.Printf("  State dir:   %s\n", cfg.StateDir)
	fmt.Printf("  Auth key:    %t (%s)\n", cfg.AuthKeyPresent, cfg.AuthKeyEnv)
	fmt.Printf("  API key:     %t (%s)\n", strings.TrimSpace(apiKey) != "", cfg.APIKeyEnv)
	fmt.Printf("  Native CLI:  %t\n", nativeRunning)
	fmt.Printf("  Native tail: %s\n", nativeTailnet)
	if nativeRunning {
		fmt.Println("  Keepalive:   skipped (native tailscale detected)")
	} else {
		fmt.Println("  Keepalive:   not detected yet")
	}
	return nil
}
