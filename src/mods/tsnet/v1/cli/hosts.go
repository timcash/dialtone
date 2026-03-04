package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

var tailscaleStatusOutput = runTailscaleStatusCommand

const (
	hostsOutputText = "text"
	hostsOutputJSON = "json"
)

type hostsArgs struct {
	outputFormat string
}

func runHosts(args []string) error {
	cfg, err := parseHostsArgs(args)
	if err != nil {
		return err
	}
	if cfg.outputFormat == "" {
		cfg.outputFormat = hostsOutputText
	}

	raw, err := tailscaleStatusOutput("status", "--json")
	if err != nil {
		return fmt.Errorf("failed to read tailscale status: %w", err)
	}

	hostnames, err := parseStatusHosts(raw)
	if err != nil {
		return err
	}

	if cfg.outputFormat == hostsOutputJSON {
		return printHostsJSON(hostnames)
	}

	for _, host := range hostnames {
		fmt.Println(host)
	}
	return nil
}

func parseHostsArgs(args []string) (hostsArgs, error) {
	cfg := hostsArgs{outputFormat: hostsOutputText}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			return cfg, nil
		case "--format":
			if i+1 >= len(args) {
				return hostsArgs{}, fmt.Errorf("tsnet hosts --format requires value")
			}
			value := strings.TrimSpace(args[i+1])
			i++
			switch value {
			case hostsOutputText, hostsOutputJSON:
				cfg.outputFormat = value
			default:
				return hostsArgs{}, fmt.Errorf("unknown hosts format: %s", value)
			}
		default:
			return hostsArgs{}, fmt.Errorf("tsnet hosts does not accept positional arguments")
		}
	}
	return cfg, nil
}

func printHostsJSON(hosts []string) error {
	data := struct {
		Hosts []string `json:"hosts"`
	}{
		Hosts: hosts,
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode hosts json failed: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func parseStatusHosts(raw []byte) ([]string, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("invalid tailscale status json: %w", err)
	}

	hostnames := make([]string, 0, 8)
	appendHostEntries(&hostnames, doc["Self"], true)
	appendHostEntries(&hostnames, doc["Peers"], false)
	appendHostEntries(&hostnames, doc["Peer"], false)

	deduped := dedupeNonEmptyStrings(hostnames)
	if len(deduped) == 0 {
		return nil, fmt.Errorf("no hostnames found in tailscale status")
	}
	sort.Strings(deduped)
	return deduped, nil
}

func appendHostEntries(hosts *[]string, raw any, includeSelf bool) {
	if raw == nil {
		return
	}
	switch typed := raw.(type) {
	case map[string]any:
		if includeSelf {
			appendHostMap(hosts, typed)
			return
		}
		for _, candidate := range typed {
			appendHostMap(hosts, candidate)
		}
	case []any:
		for _, candidate := range typed {
			appendHostMap(hosts, candidate)
		}
	default:
		appendHostMap(hosts, typed)
	}
}

func appendHostMap(hosts *[]string, raw any) {
	entry, ok := raw.(map[string]any)
	if !ok {
		return
	}
	if host := parseStatusHost(entry); host != "" {
		*hosts = append(*hosts, host)
	}
}

func parseStatusHost(raw map[string]any) string {
	for _, key := range []string{"DNSName", "HostName", "Name"} {
		rawHost := strings.TrimSpace(anyToString(raw[key]))
		if rawHost == "" {
			continue
		}
		host := sanitizeHost(rawHost)
		if host != "" {
			return host
		}
	}
	return ""
}

func runTailscaleStatusCommand(args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing tailscale arguments")
	}
	cmdPath, err := exec.LookPath("tailscale")
	if err != nil {
		return nil, fmt.Errorf("tailscale not found: %w", err)
	}
	cmd := exec.Command(cmdPath, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}
