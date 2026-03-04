package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	tsnetDefaultHostname = "dialtone-node"
	tsnetDefaultTailnet  = "shad-artichoke.ts.net"
)

// tsnetConfig mirrors a minimal subset of tsnet runtime config needed by this mod.
type tsnetConfig struct {
	Hostname       string
	StateDir       string
	AuthKeyPresent bool
	AuthKeyEnv     string
	Tailnet        string
	APIKeyPresent  bool
	APIKeyEnv      string
}

func resolveConfig(hostnameArg, stateDirArg string) (tsnetConfig, error) {
	hostname := strings.TrimSpace(hostnameArg)
	if hostname == "" {
		hostname = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	}
	if hostname == "" {
		hostname = tsnetDefaultHostname
	}
	hostname = sanitizeHost(hostname)
	if hostname == "" {
		return tsnetConfig{}, errors.New("resolved empty hostname")
	}

	stateDir := strings.TrimSpace(stateDirArg)
	if stateDir == "" {
		stateDir = strings.TrimSpace(os.Getenv("DIALTONE_TSNET_STATE_DIR"))
	}
	if stateDir == "" {
		stateDir = filepath.Join(".dialtone", "tsnet")
	}

	authVar := "TS_AUTHKEY"
	authVal := strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	if authVal == "" {
		authVal = strings.TrimSpace(os.Getenv("TAILSCALE_AUTHKEY"))
		if authVal != "" {
			authVar = "TAILSCALE_AUTHKEY"
		}
	}

	apiVar := "TS_API_KEY"
	apiVal := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiVal == "" {
		apiVal = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
		if apiVal != "" {
			apiVar = "TAILSCALE_API_KEY"
		}
	}

	tailnet := strings.TrimSpace(os.Getenv("TS_TAILNET"))
	if tailnet == "" {
		if detected, err := detectTailnetFromLocalStatus(); err == nil && strings.TrimSpace(detected) != "" {
			tailnet = strings.TrimSpace(detected)
		}
	}
	if tailnet == "" {
		tailnet = tsnetDefaultTailnet
	}

	return tsnetConfig{
		Hostname:       hostname,
		StateDir:       stateDir,
		AuthKeyPresent: authVal != "",
		AuthKeyEnv:     authVar,
		Tailnet:        tailnet,
		APIKeyPresent:  apiVal != "",
		APIKeyEnv:      apiVar,
	}, nil
}

func detectTailnetFromLocalStatus() (string, error) {
	path, err := exec.LookPath("tailscale")
	if err != nil {
		return "", err
	}

	out, err := exec.Command(path, "status", "--json").Output()
	if err != nil {
		return "", err
	}

	tailnet := parseTailnetFromStatusJSON(out)
	if tailnet == "" {
		return "", errors.New("tailnet not found in tailscale status")
	}
	return tailnet, nil
}

func parseTailnetFromStatusJSON(raw []byte) string {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	if current, ok := doc["CurrentTailnet"].(map[string]any); ok {
		if suffix := sanitizeTailnet(anyToString(current["MagicDNSSuffix"])); suffix != "" {
			return suffix
		}
		if name := sanitizeTailnet(anyToString(current["Name"])); name != "" {
			return name
		}
	}

	if suffix := sanitizeTailnet(anyToString(doc["MagicDNSSuffix"])); suffix != "" {
		return suffix
	}

	if self, ok := doc["Self"].(map[string]any); ok {
		dnsName := sanitizeTailnet(anyToString(self["DNSName"]))
		if dnsName != "" {
			parts := strings.Split(dnsName, ".")
			if len(parts) > 1 {
				return sanitizeTailnet(strings.Join(parts[1:], "."))
			}
		}
	}

	return ""
}

func sanitizeTailnet(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if strings.HasPrefix(v, ".") || strings.HasSuffix(v, ".") {
		v = strings.Trim(v, ".")
	}
	if v == "" {
		return ""
	}
	return v
}
