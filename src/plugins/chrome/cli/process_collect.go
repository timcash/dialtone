package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func collectTopProcesses(limit int) ([]processStat, error) {
	switch runtime.GOOS {
	case "windows":
		return collectTopProcessesWindows(limit)
	default:
		return collectTopProcessesUnix(limit)
	}
}

func collectTopProcessesUnix(limit int) ([]processStat, error) {
	psCmd := ""
	switch runtime.GOOS {
	case "darwin":
		psCmd = fmt.Sprintf(`ps -axo pid=,pcpu=,rss=,command= -r | head -n %d | awk '{pid=$1; cpu=$2; rss=$3; $1=$2=$3=""; sub(/^ +/,"",$0); printf "%%s\t%%s\t%%s\t%%s\n", pid,cpu,rss,$0 }'`, limit)
	default:
		psCmd = fmt.Sprintf(`ps -eo pid=,pcpu=,rss=,command= --sort=-pcpu | head -n %d | awk '{pid=$1; cpu=$2; rss=$3; $1=$2=$3=""; sub(/^ +/,"",$0); printf "%%s\t%%s\t%%s\t%%s\n", pid,cpu,rss,$0 }'`, limit)
	}
	cmd := exec.Command("bash", "-lc", psCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ps failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	rows := make([]processStat, 0, len(lines))
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		cpu, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		rssKB, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		command := strings.TrimSpace(parts[3])
		if command == "" {
			continue
		}
		rows = append(rows, processStat{
			PID:     pid,
			Name:    deriveProcessName("", command),
			CPU:     cpu,
			MemMB:   rssKB / 1024.0,
			Command: command,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CPU == rows[j].CPU {
			return rows[i].MemMB > rows[j].MemMB
		}
		return rows[i].CPU > rows[j].CPU
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func collectTopProcessesWindows(limit int) ([]processStat, error) {
	script := fmt.Sprintf(`Get-Process | Sort-Object CPU -Descending | Select-Object -First %d Id,ProcessName,CPU,WS,Path | ConvertTo-Json -Compress`, limit)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Get-Process failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return []processStat{}, nil
	}
	type winProc struct {
		ID          int     `json:"Id"`
		ProcessName string  `json:"ProcessName"`
		CPU         float64 `json:"CPU"`
		WS          float64 `json:"WS"`
		Path        string  `json:"Path"`
	}
	one := winProc{}
	if err := json.Unmarshal([]byte(raw), &one); err == nil && one.ID > 0 {
		return []processStat{{
			PID:     one.ID,
			Name:    strings.TrimSpace(one.ProcessName),
			CPU:     one.CPU,
			MemMB:   one.WS / (1024.0 * 1024.0),
			Command: strings.TrimSpace(one.Path),
		}}, nil
	}
	var arr []winProc
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil, fmt.Errorf("parse process JSON failed: %w", err)
	}
	rows := make([]processStat, 0, len(arr))
	for _, p := range arr {
		rows = append(rows, processStat{
			PID:     p.ID,
			Name:    strings.TrimSpace(p.ProcessName),
			CPU:     p.CPU,
			MemMB:   p.WS / (1024.0 * 1024.0),
			Command: strings.TrimSpace(p.Path),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CPU == rows[j].CPU {
			return rows[i].MemMB > rows[j].MemMB
		}
		return rows[i].CPU > rows[j].CPU
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}
