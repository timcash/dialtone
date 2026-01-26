package test

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/util"
	"dialtone/cli/src/plugins/vpn/cli"
)

func init() {
	Register("verify-tsnet-connection", "core", []string{"core", "vpn", "tailscale"}, RunTsnetTest)
}

func RunTsnetTest() error {
	logger.LogInfo("Starting tsnet verification test (IP-first)...")

	// 0. Provision a fresh ephemeral key if API key is available
	apiKey := os.Getenv("TS_API_KEY")
	authKey := os.Getenv("TS_AUTHKEY")

	if apiKey != "" {
		logger.LogInfo("TS_API_KEY found, provisioning new ephemeral auth key...")
		key, err := cli.ProvisionKey(apiKey, false) // Don't overwrite .env for test
		if err != nil {
			logger.LogInfo("Failed to provision key: %v. Falling back to TS_AUTHKEY.", err)
		} else {
			authKey = key
			logger.LogInfo("Successfully provisioned fresh key.")
		}
	}

	if authKey == "" {
		return fmt.Errorf("neither TS_API_KEY nor TS_AUTHKEY environment variables are set")
	}

	// Generate random hostname
	targetHostname := util.GenerateCodename("test-drone")
	logger.LogInfo("Generated random test hostname: %s", targetHostname)

	// 1. Build the dialtone binary
	logger.LogInfo("Building dialtone binary...")
	buildCmd := exec.Command("go", "build", "-o", "./dialtone-test-bin", "./src/cmd/dialtone/main.go")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to build dialtone: %v, output: %s", err, string(output))
	}
	defer os.Remove("./dialtone-test-bin")

	// 2. Start dialtone in vpn mode
	serverStateDir, _ := os.MkdirTemp("", "tsnet-server-*")
	defer os.RemoveAll(serverStateDir)

	logger.LogInfo("Starting server: ./dialtone-test-bin vpn --hostname %s --ephemeral", targetHostname)
	vpnCmd := exec.Command("./dialtone-test-bin", "vpn", "--hostname", targetHostname, "--ephemeral", "--state-dir", serverStateDir)
	// Pass the fresh auth key to the subprocess
	env := os.Environ()
	env = append(env, fmt.Sprintf("TS_AUTHKEY=%s", authKey))
	vpnCmd.Env = env

	// Capture server logs for debugging and IP parsing
	serverLogs, _ := os.CreateTemp("", "dialtone-server-*.log")
	defer os.Remove(serverLogs.Name())
	vpnCmd.Stdout = serverLogs
	vpnCmd.Stderr = serverLogs

	if err := vpnCmd.Start(); err != nil {
		return fmt.Errorf("failed to start vpn mode: %w", err)
	}
	defer func() {
		logger.LogInfo("Shutting down VPN server process...")
		vpnCmd.Process.Kill()
		vpnCmd.Wait()
	}()

	// 3. Wait for IP to appear in logs
	logger.LogInfo("Waiting for server to report Tailscale IP...")
	var serverIP string
	timeout := time.After(1 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for serverIP == "" {
		select {
		case <-timeout:
			logData, _ := os.ReadFile(serverLogs.Name())
			return fmt.Errorf("timed out waiting for IP in logs. Logs:\n%s", string(logData))
		case <-ticker.C:
			logData, _ := os.ReadFile(serverLogs.Name())
			content := string(logData)
			// Look for "VPN Mode: Connected (IP: 100.x.y.z)"
			if idx := strings.Index(content, "VPN Mode: Connected (IP: "); idx != -1 {
				start := idx + len("VPN Mode: Connected (IP: ")
				end := strings.Index(content[start:], ")")
				if end != -1 {
					serverIP = content[start : start+end]
					logger.LogInfo("Detected server IP: %s", serverIP)
				}
			}
		}
	}

	// 4. Verify connectivity via IP first
	urlIP := fmt.Sprintf("http://%s", serverIP)
	logger.LogInfo("Stage 1: Verifying HTTP connectivity to IP %s...", urlIP)

	count := 0
	for {
		resp, err := http.Get(urlIP)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				logger.LogInfo("Stage 1 PASS: Reachable via IP!")
				break
			}
		}
		count++
		if count > 10 {
			return fmt.Errorf("failed to reach server via IP %s after 10 tries: %v", urlIP, err)
		}
		time.Sleep(2 * time.Second)
	}

	// 5. Verify connectivity via Domain
	urlDomain := fmt.Sprintf("http://%s", targetHostname)
	logger.LogInfo("Stage 2: Verifying HTTP connectivity to domain %s...", urlDomain)

	timeout = time.After(2 * time.Minute)
	ticker = time.NewTicker(5 * time.Second)
	for {
		select {
		case <-timeout:
			logData, _ := os.ReadFile(serverLogs.Name())
			logger.LogInfo("Server Logs:\n%s", string(logData))
			return fmt.Errorf("Stage 2 FAIL: timed out waiting for domain %s to be reachable", urlDomain)
		case <-ticker.C:
			ips, _ := net.LookupIP(targetHostname)
			resp, err := http.Get(urlDomain)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					logger.LogInfo("Stage 2 PASS: Successfully connected and verified %s via Tailscale!", urlDomain)
					return nil
				}
			}
			logger.LogInfo("Still waiting for domain %s (Resolves to: %v)... (%v)", urlDomain, ips, err)
		}
	}
}
