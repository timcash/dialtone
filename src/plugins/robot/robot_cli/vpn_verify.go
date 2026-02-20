package robot_cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"dialtone/dev/logger"
	"tailscale.com/tsnet"
)

func RunVPNTest(args []string) error {
	hostname := "robot-debug-vpn"
	if len(args) > 0 {
		hostname = args[0]
	}

	logger.LogInfo("[VPN-TEST] Starting tsnet connectivity test as %s...", hostname)

	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		return fmt.Errorf("TS_AUTHKEY environment variable is not set")
	}

	s := &tsnet.Server{
		Hostname: hostname,
		AuthKey:  authKey,
		Logf: func(format string, args ...any) {
			logger.LogDebug(fmt.Sprintf("tsnet: "+format, args...))
		},
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.LogInfo("[VPN-TEST] Waiting for Tailscale to come up (timeout 60s)...")
	status, err := s.Up(ctx)
	if err != nil {
		return fmt.Errorf("failed to start tsnet server: %w", err)
	}

	logger.LogInfo("[VPN-TEST] SUCCESS: tsnet is UP")
	logger.LogInfo("[VPN-TEST] Hostname: %s", status.Self.HostName)
	logger.LogInfo("[VPN-TEST] Tailscale IPs: %v", status.TailscaleIPs)

	return nil
}
