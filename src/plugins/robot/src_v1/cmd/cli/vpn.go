package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"dialtone/dev/plugins/logs/src_v1/go"
	"tailscale.com/tsnet"
)

func RunVPNTest(args []string) error {
	hostname := "robot-debug-vpn"
	if len(args) > 0 {
		hostname = args[0]
	}

	logs.Info("[VPN-TEST] Starting tsnet connectivity test as %s...", hostname)

	authKey := os.Getenv("TS_AUTHKEY")
	if authKey == "" {
		return fmt.Errorf("TS_AUTHKEY environment variable is not set")
	}

	s := &tsnet.Server{
		Hostname: hostname,
		AuthKey:  authKey,
		Logf: func(format string, args ...any) {
			logs.Debug(fmt.Sprintf("tsnet: "+format, args...))
		},
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logs.Info("[VPN-TEST] Waiting for Tailscale to come up (timeout 60s)...")
	status, err := s.Up(ctx)
	if err != nil {
		return fmt.Errorf("failed to start tsnet server: %w", err)
	}

	logs.Info("[VPN-TEST] SUCCESS: tsnet is UP")
	logs.Info("[VPN-TEST] Hostname: %s", status.Self.HostName)
	logs.Info("[VPN-TEST] Tailscale IPs: %v", status.TailscaleIPs)

	return nil
}
