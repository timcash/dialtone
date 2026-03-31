package ssh

import (
	"encoding/json"
	"flag"
	"sort"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type nodeStatus struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	OS       string `json:"os"`
	CPU      string `json:"cpu"`
	MemFree  string `json:"mem_free"`
	Network  string `json:"network"`
	DiskFree string `json:"disk_free"`
	Battery  string `json:"battery"`
	Error    string `json:"error,omitempty"`
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("ssh status", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "all", "Target host, csv list, or all")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	nodes, err := resolveStatusNodes(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	rows := make([]nodeStatus, 0, len(nodes))
	for _, node := range nodes {
		row := nodeStatus{
			Name:     node.Name,
			Host:     node.Host,
			OS:       node.OS,
			CPU:      "-",
			MemFree:  "-",
			Network:  "-",
			DiskFree: "-",
			Battery:  "-",
		}
		cmd := buildStatusProbeCommand(node)
		out, rerr := RunNodeCommand(node.Name, cmd, CommandOptions{})
		if rerr != nil {
			row.Error = rerr.Error()
			rows = append(rows, row)
			continue
		}
		parsed := parseStatusOutput(out)
		row.CPU = defaultDash(parsed["cpu"])
		row.MemFree = defaultDash(parsed["mem_free"])
		row.Network = defaultDash(parsed["network"])
		row.DiskFree = defaultDash(parsed["disk_free"])
		row.Battery = defaultDash(parsed["battery"])
		rows = append(rows, row)
	}

	if *asJSON {
		raw, _ := json.MarshalIndent(rows, "", "  ")
		logs.Raw("%s", string(raw))
	} else {
		logs.Raw("%-9s %-15s %-8s %-8s %-10s %-26s %-10s %-8s %s", "NAME", "HOST", "OS", "CPU", "MEM_FREE", "NETWORK", "DISK_FREE", "BATTERY", "ERROR")
		logs.Raw("%s", strings.Repeat("-", 120))
		for _, r := range rows {
			logs.Raw("%-9s %-15s %-8s %-8s %-10s %-26s %-10s %-8s %s",
				r.Name, r.Host, r.OS, r.CPU, r.MemFree, truncate(r.Network, 26), r.DiskFree, r.Battery, truncate(r.Error, 40))
		}
	}

	return nil
}

func resolveStatusNodes(target string) ([]MeshNode, error) {
	target = strings.TrimSpace(target)
	if target == "" || strings.EqualFold(target, "all") {
		nodes := ListMeshNodes()
		sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
		return nodes, nil
	}
	parts := strings.Split(target, ",")
	nodes := make([]MeshNode, 0, len(parts))
	for _, p := range parts {
		n, err := ResolveMeshNode(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	sort.SliceStable(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return nodes, nil
}

func parseStatusOutput(raw string) map[string]string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	line := ""
	for i := len(lines) - 1; i >= 0; i-- {
		cand := strings.TrimSpace(lines[i])
		if strings.Contains(cand, "cpu=") && strings.Contains(cand, "mem_free=") {
			line = cand
			break
		}
	}
	out := map[string]string{}
	if strings.TrimSpace(line) == "" {
		return out
	}
	fields := strings.Split(line, "|")
	for _, f := range fields {
		parts := strings.SplitN(strings.TrimSpace(f), "=", 2)
		if len(parts) != 2 {
			continue
		}
		out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return out
}

func buildStatusProbeCommand(node MeshNode) string {
	switch strings.ToLower(strings.TrimSpace(node.OS)) {
	case "windows":
		return `powershell -NoProfile -Command '$cpu=((Get-Counter ''\Processor(_Total)\% Processor Time'').CounterSamples | Select-Object -First 1).CookedValue; $os=Get-CimInstance Win32_OperatingSystem; $mem=[math]::Round($os.FreePhysicalMemory/1024,0).ToString()+''MB''; $drv=Get-PSDrive -Name C -ErrorAction SilentlyContinue; $disk=if($drv){([math]::Round($drv.Free/1GB,1)).ToString()+''G''}else{''-''}; $ip=Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -ne ''127.0.0.1'' -and $_.IPAddress -notlike ''169.254.*'' } | Select-Object -First 1; $net=if($ip){$ip.InterfaceAlias+'':''+$ip.IPAddress}else{''-''}; $b=Get-CimInstance Win32_Battery -ErrorAction SilentlyContinue | Select-Object -First 1; $bat=if($b){$b.EstimatedChargeRemaining.ToString()+''%''}else{''-''}; Write-Output (''cpu='' + (''{0:N1}%'' -f $cpu) + ''|mem_free='' + $mem + ''|network='' + $net + ''|disk_free='' + $disk + ''|battery='' + $bat)'`
	case "macos", "darwin":
		return `cpu="$(top -l 1 -n 0 2>/dev/null | awk -F'[:,% ]+' '/CPU usage:/{printf("%.1f%%",$3+$5); exit}')"; pages="$(vm_stat 2>/dev/null)"; psize="$(printf '%s\n' "$pages" | head -n1 | awk '{print $8}' | tr -d '.')"; freep="$(printf '%s\n' "$pages" | awk '/Pages free/{gsub("\\.","",$3); print $3; exit}')"; specp="$(printf '%s\n' "$pages" | awk '/Pages speculative/{gsub("\\.","",$3); print $3; exit}')"; [ -z "$psize" ] && psize=4096; [ -z "$freep" ] && freep=0; [ -z "$specp" ] && specp=0; mem=$(( (freep + specp) * psize / 1024 / 1024 )); iface="$(route -n get default 2>/dev/null | awk '/interface:/{print $2; exit}')"; ip="$(ipconfig getifaddr "$iface" 2>/dev/null || true)"; net="${iface:--}:${ip:--}"; disk="$(df -h / | awk 'NR==2{print $4}')"; bat="$(pmset -g batt 2>/dev/null | awk 'NR==2{for(i=1;i<=NF;i++){if($i ~ /%/){gsub(\";\",\"\",$i); print $i; exit}}} END{if(NR<2) print \"-\"}')"; echo "cpu=${cpu:--}|mem_free=${mem}MB|network=${net}|disk_free=${disk:--}|battery=${bat:--}"`
	default:
		return `cpu="$(awk 'NR==1{u1=$2+$3+$4;s1=$5+$6+$7+$8} END{print u1, s1}' /proc/stat | while read -r u1 s1; do sleep 0.2; read -r _ a b c d e f g h _ < /proc/stat; u2=$((a+b+c)); s2=$((d+e+f+g+h)); du=$((u2-u1)); ds=$((s2-s1)); if [ "$ds" -gt 0 ]; then awk "BEGIN{printf \"%.1f%%\", 100*$du/$ds}"; else echo "-"; fi; done)"; mem="$(free -m 2>/dev/null | awk '/^Mem:/{printf \"%dMB\", $7; exit}')"; [ -z "$mem" ] && mem="$(awk '/MemAvailable:/{printf \"%dMB\", int($2/1024); exit}' /proc/meminfo 2>/dev/null)"; [ -z "$mem" ] && mem="-"; net="$(ip -o -4 addr show scope global 2>/dev/null | awk 'NR==1{print $2\":\"$4}')"; [ -z "$net" ] && net="$(hostname -I 2>/dev/null | awk '{print $1}')"; [ -z "$net" ] && net="-"; disk="$(df -h / | awk 'NR==2{print $4}')"; [ -z "$disk" ] && disk="-"; bat="$(for b in /sys/class/power_supply/BAT*/capacity; do [ -f "$b" ] && { cat "$b"; break; }; done)"; [ -n "$bat" ] && bat="${bat}%"; [ -z "$bat" ] && bat="-"; [ -z "$cpu" ] && cpu="-"; echo "cpu=${cpu}|mem_free=${mem}|network=${net}|disk_free=${disk}|battery=${bat}"`
	}
}

func defaultDash(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "-"
	}
	return v
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
