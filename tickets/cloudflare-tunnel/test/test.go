package test

import (
	"fmt"
	"os"
	"path/filepath"
	"dialtone/cli/src/core/test"
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/config"
)

func init() {
	test.Register("cloudflare-plugin-add", "cloudflare-tunnel", []string{"scaffold"}, RunScaffoldTest)
	test.Register("cloudflare-install", "cloudflare-tunnel", []string{"install"}, RunInstallTest)
	test.Register("cloudflare-login", "cloudflare-tunnel", []string{"login"}, RunLoginTest)
	test.Register("cloudflare-tunnel-mgmt", "cloudflare-tunnel", []string{"tunnel"}, RunTunnelMgmtTest)
	test.Register("cloudflare-serve", "cloudflare-tunnel", []string{"serve"}, RunServeTest)
	test.Register("cloudflare-hostname-subdomain", "cloudflare-tunnel", []string{"hostname"}, RunHostnameSubdomainTest)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running cloudflare-tunnel suite...")
	return test.RunTicket("cloudflare-tunnel")
}

func RunScaffoldTest() error {
	readmePath := filepath.Join("src", "plugins", "cloudflare", "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		return fmt.Errorf("plugin scaffold not found at %s", readmePath)
	}
	fmt.Println("PASS: [scaffold] cloudflare plugin structure verified")
	return nil
}

func RunInstallTest() error {
	depsDir := config.GetDialtoneEnv()
	if depsDir == "" {
		return fmt.Errorf("DIALTONE_ENV is not set")
	}
	binPath := filepath.Join(depsDir, "cloudflare", "cloudflared")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return fmt.Errorf("cloudflared binary not found; run install first")
	}
	fmt.Println("PASS: [install] cloudflared installation verified")
	return nil
}

func RunLoginTest() error {
	fmt.Println("TODO: Implement login verification logic")
	return nil
}

func RunTunnelMgmtTest() error {
	fmt.Println("TODO: Implement tunnel management verification logic")
	return nil
}

func RunServeTest() error {
	fmt.Println("TODO: Implement serve verification logic")
	return nil
}

func RunHostnameSubdomainTest() error {
	config.LoadConfig()
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	if hostname == "" {
		return fmt.Errorf("DIALTONE_HOSTNAME not set; please check .env")
	}
	fmt.Printf("PASS: [hostname] correctly identified DIALTONE_HOSTNAME: %s\n", hostname)
	return nil
}
