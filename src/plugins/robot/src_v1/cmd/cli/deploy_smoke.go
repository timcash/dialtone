package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func RunPostDeployUIValidation() error {
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		hostname = "drone-1"
	}
	url := fmt.Sprintf("http://%s", hostname)

	logs.Info("   [SMOKE] Strictly verifying robot UI at %s (via embedded tsnet)...", url)

	tsHost := "dialtone-deployer-" + hostname
	cfg, err := tsnetv1.ResolveConfig(tsHost, "")
	if err != nil {
		return err
	}

	if !cfg.AuthKeyPresent {
		apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
		}
		if apiKey != "" {
			logs.Info("   [SMOKE] Provisioning ephemeral auth key for deployer...")
			key, perr := provisionRobotAuthKeyWithAPI(apiKey, cfg.Tailnet, "dialtone-deployer-"+hostname, []string{"dialtone", "deployer", "ephemeral"})
			if perr != nil && strings.Contains(strings.ToLower(perr.Error()), "requested tags") {
				logs.Warn("   [SMOKE] Tailnet does not allow requested tags; retrying without tags")
				key, perr = provisionRobotAuthKeyWithAPI(apiKey, cfg.Tailnet, "dialtone-deployer-"+hostname, nil)
			}
			if perr == nil {
				_ = os.Setenv("TS_AUTHKEY", key)
				cfg.AuthKeyPresent = true
				cfg.AuthKeyEnv = "TS_AUTHKEY"
				logs.Info("   [SMOKE] Successfully provisioned ephemeral auth key.")
			} else {
				logs.Error("   [SMOKE] Failed to provision auth key: %v", perr)
			}
		} else {
			logs.Warn("   [SMOKE] TS_API_KEY missing; cannot auto-provision auth key for verification")
		}
	}

	srv := tsnetv1.BuildServer(cfg)
	srv.Ephemeral = true
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logs.Info("   [SMOKE] Joining Tailnet as %s ...", tsHost)
	if _, err := srv.Up(ctx); err != nil {
		return fmt.Errorf("failed to join Tailnet for verification: %w", err)
	}

	time.Sleep(3 * time.Second)
	client := srv.HTTPClient()
	logs.Info("   [SMOKE] Probing %s ...", url)

	var bodyStr string
	var lastErr error
	for attempt := 1; attempt <= 15; attempt++ {
		targetURL := url
		if attempt > 5 {
			ip, iperr := getTailscaleIP(hostname)
			if iperr == nil && ip != "" {
				if attempt == 6 {
					logs.Info("   [SMOKE] Hostname resolution sluggish; switching to IP probe: http://%s", ip)
				}
				targetURL = "http://" + ip
			} else if attempt == 6 {
				logs.Warn("   [SMOKE] Could not resolve IP via API: %v", iperr)
			}
		}

		resp, err := client.Get(targetURL)
		if err != nil {
			lastErr = err
			if attempt%3 == 0 {
				logs.Warn("   [SMOKE] Probe attempt %d failed: %v", attempt, err)
			}
			time.Sleep(3 * time.Second)
			continue
		}

		body, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			lastErr = rerr
			time.Sleep(1 * time.Second)
			continue
		}
		bodyStr = string(body)
		lastErr = nil
		break
	}

	if lastErr != nil {
		return fmt.Errorf("failed to reach robot at %s via Tailscale after retries: %w", url, lastErr)
	}
	if strings.Contains(strings.ToLower(bodyStr), "sleeping ...") {
		return fmt.Errorf("VERIFICATION FAILED: Reached the relay sleep page instead of the robot UI at %s", url)
	}
	if !strings.Contains(strings.ToLower(bodyStr), "dialtone.robot") {
		return fmt.Errorf("VERIFICATION FAILED: Robot UI content not found at %s. (Response length: %d)", url, len(bodyStr))
	}

	logs.Info("   [SMOKE] Verification SUCCESS: Robot UI is reachable and active at %s", url)
	return nil
}
