package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type remoteListRow struct {
	Node        string
	Host        string
	PID         int
	Role        string
	Headless    bool
	DebugPort   int
	DebugActive bool
	PageTabs    int
}

func handleListWithHost(hostArg string, headedOnly, headlessOnly, verbose bool) {
	target := strings.TrimSpace(hostArg)
	if target == "" {
		handleList(headedOnly, headlessOnly, verbose)
		return
	}
	nodes, err := resolveListHosts(target)
	if err != nil {
		logs.Fatal("list --host: %v", err)
	}

	rows := make([]remoteListRow, 0)
	seen := map[string]struct{}{}
	for _, node := range nodes {
		procs, perr := listRemoteNodeChrome(node)
		if perr != nil {
			logs.Warn("list --host node=%s failed: %v", node.Name, perr)
			continue
		}
		for _, p := range procs {
			if p.DebugPort <= 0 && strings.EqualFold(strings.TrimSpace(p.Role), "unknown") {
				continue
			}
			if headedOnly && p.Headless {
				continue
			}
			if headlessOnly && !p.Headless {
				continue
			}
			active := false
			tabs := 0
			if p.DebugPort > 0 {
				active, tabs = probeRemoteDebugState(node, p.DebugPort)
			}
			key := fmt.Sprintf("%s|%s|%t|%d", node.Name, strings.ToLower(strings.TrimSpace(p.Role)), p.Headless, p.DebugPort)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			rows = append(rows, remoteListRow{
				Node:        node.Name,
				Host:        node.Host,
				PID:         p.PID,
				Role:        p.Role,
				Headless:    p.Headless,
				DebugPort:   p.DebugPort,
				DebugActive: active,
				PageTabs:    tabs,
			})
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Node == rows[j].Node {
			return rows[i].PID < rows[j].PID
		}
		return rows[i].Node < rows[j].Node
	})

	if len(rows) == 0 {
		logs.Info("list --host: no chrome instances found")
		return
	}
	fmt.Printf("%-10s %-15s %-7s %-16s %-8s %-7s %-8s %-6s\n", "HOST", "IP", "PID", "ROLE", "HEADLESS", "PORT", "ACTIVE", "TABS")
	fmt.Println(strings.Repeat("-", 86))
	for _, r := range rows {
		port := "-"
		if r.DebugPort > 0 {
			port = strconv.Itoa(r.DebugPort)
		}
		fmt.Printf("%-10s %-15s %-7d %-16s %-8t %-7s %-8t %-6d\n", r.Node, r.Host, r.PID, r.Role, r.Headless, port, r.DebugActive, r.PageTabs)
	}
	logs.Info("list --host: %d instance(s) across %d host(s)", len(rows), len(nodes))
}

func resolveListHosts(target string) ([]sshv1.MeshNode, error) {
	return resolveChromeHosts(target)
}

func probeRemoteDebugState(node sshv1.MeshNode, port int) (bool, int) {
	_, remoteVersion := probeRemoteNodePort(node, port)
	if !remoteVersion {
		return false, 0
	}
	return true, probeRemotePageTabs(node, port)
}

func probeRemotePageTabs(node sshv1.MeshNode, port int) int {
	if port <= 0 {
		return 0
	}
	cmd := buildRemoteTabsProbeCommand(node, port)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return 0
	}
	raw := strings.TrimSpace(out)
	if raw == "" {
		return 0
	}
	type tinfo struct {
		Type string `json:"type"`
	}
	var targets []tinfo
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		var one tinfo
		if err2 := json.Unmarshal([]byte(raw), &one); err2 != nil {
			return 0
		}
		targets = append(targets, one)
	}
	tabs := 0
	for _, t := range targets {
		if strings.EqualFold(strings.TrimSpace(t.Type), "page") {
			tabs++
		}
	}
	return tabs
}

func buildRemoteTabsProbeCommand(node sshv1.MeshNode, port int) string {
	if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
		return fmt.Sprintf(`$p=%d; try { $v=Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/json/list" -f $p) -TimeoutSec 2; $v | ConvertTo-Json -Compress } catch { "[]" }`, port)
	}
	return fmt.Sprintf(`curl -fsS --max-time 2 "http://127.0.0.1:%d/json/list" 2>/dev/null || echo '[]'`, port)
}
