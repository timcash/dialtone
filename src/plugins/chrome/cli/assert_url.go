package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type debugPageTarget struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

func handleAssertURLCmd(args []string) {
	fs := flag.NewFlagSet("chrome assert-url", flag.ExitOnError)
	host := fs.String("host", "", "Target host name, csv, or 'all'")
	role := fs.String("role", "dev", "Role to verify")
	wantURL := fs.String("url", "", "Expected URL (prefix match by default)")
	exact := fs.Bool("exact", false, "Require exact URL equality")
	_ = fs.Parse(args)

	target := strings.TrimSpace(*host)
	if target == "" {
		logs.Fatal("assert-url requires --host")
	}
	expected := strings.TrimSpace(*wantURL)
	if expected == "" {
		logs.Fatal("assert-url requires --url")
	}

	nodes, err := resolveChromeHosts(target)
	if err != nil {
		logs.Fatal("assert-url --host: %v", err)
	}

	fail := 0
	ok := 0
	for _, node := range nodes {
		proc, perr := findRoleProcess(node, strings.TrimSpace(*role))
		if perr != nil {
			fail++
			logs.Warn("assert-url host=%s failed: %v", node.Name, perr)
			continue
		}
		pages, gerr := probeRemotePageURLs(node, proc.DebugPort)
		if gerr != nil {
			fail++
			logs.Warn("assert-url host=%s failed: %v", node.Name, gerr)
			continue
		}
		actual := firstNonBlank(pages)
		if actual == "" {
			fail++
			logs.Warn("assert-url host=%s failed: no page URL found", node.Name)
			continue
		}
		match := false
		if *exact {
			match = strings.TrimSpace(actual) == expected
		} else {
			match = strings.HasPrefix(strings.TrimSpace(actual), expected)
		}
		if !match {
			fail++
			logs.Warn("assert-url host=%s mismatch expected=%q actual=%q", node.Name, expected, actual)
			continue
		}
		ok++
		logs.Info("assert-url host=%s ok actual=%q", node.Name, actual)
	}

	if ok == 0 {
		logs.Fatal("assert-url failed on all targets (%d)", fail)
	}
	if fail > 0 {
		logs.Fatal("assert-url completed with failures: ok=%d fail=%d", ok, fail)
	}
	logs.Info("assert-url completed: ok=%d", ok)
}

func findRoleProcess(node sshv1.MeshNode, role string) (*remoteChromeProcess, error) {
	role = strings.TrimSpace(role)
	procs, err := listRemoteNodeChrome(node)
	if err != nil {
		return nil, err
	}
	for _, p := range procs {
		if p.DebugPort <= 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(p.Origin), "dialtone") {
			continue
		}
		if role != "" && !strings.EqualFold(strings.TrimSpace(p.Role), role) {
			continue
		}
		cp := p
		return &cp, nil
	}
	return nil, fmt.Errorf("no dialtone process found for role=%s", role)
}

func probeRemotePageURLs(node sshv1.MeshNode, port int) ([]string, error) {
	if port <= 0 {
		return nil, fmt.Errorf("invalid debug port")
	}
	cmd := buildRemoteTabsProbeCommand(node, port)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return nil, fmt.Errorf("empty json/list output")
	}

	var targets []debugPageTarget
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		var one debugPageTarget
		if err2 := json.Unmarshal([]byte(raw), &one); err2 != nil {
			return nil, fmt.Errorf("parse json/list failed: %w", err)
		}
		targets = append(targets, one)
	}

	outURLs := make([]string, 0, len(targets))
	for _, t := range targets {
		if !strings.EqualFold(strings.TrimSpace(t.Type), "page") {
			continue
		}
		outURLs = append(outURLs, strings.TrimSpace(t.URL))
	}
	return outURLs, nil
}

func firstNonBlank(values []string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}
