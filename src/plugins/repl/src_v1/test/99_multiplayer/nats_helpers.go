package multiplayer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func resolveNATSAdvertiseHost(_ string) string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPL_TEST_NATS_HOST")); v != "" {
		return v
	}
	if dns := tailscaleSelfDNSName(); dns != "" {
		return dns
	}
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		base := strings.TrimSpace(host)
		if i := strings.Index(base, "."); i > 0 {
			base = base[:i]
		}
		tailnet := strings.TrimSpace(os.Getenv("TAILSCALE_TAILNET"))
		if tailnet == "" {
			tailnet = strings.TrimSpace(os.Getenv("TS_TAILNET"))
		}
		if tailnet == "" {
			tailnet = "shad-artichoke"
		}
		return fmt.Sprintf("%s.%s.ts.net", base, tailnet)
	}
	return "localhost"
}

func tailscaleSelfDNSName() string {
	out, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return ""
	}
	var v struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if json.Unmarshal(out, &v) != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(v.Self.DNSName), ".")
}
