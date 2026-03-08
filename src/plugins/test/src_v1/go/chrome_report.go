package test

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

const (
	chromeReportStart = "<!-- DIALTONE_CHROME_REPORT_START -->"
	chromeReportEnd   = "<!-- DIALTONE_CHROME_REPORT_END -->"
)

type remoteChromeProcessInfo struct {
	PID       int
	Role      string
	DebugPort int
	Command   string
}

type remoteChromeReport struct {
	Node        string
	ChromeCount int
	Processes   []remoteChromeProcessInfo
}

var (
	roleEqualsRe = regexp.MustCompile(`--dialtone-role=([^\s]+)`)
	roleSplitRe  = regexp.MustCompile(`--dialtone-role\s+([^\s]+)`)
)

func appendSuiteChromeReport(opts SuiteOptions) error {
	reportPath := strings.TrimSpace(opts.ReportPath)
	if reportPath == "" {
		return nil
	}
	node := resolveChromeReportNode(opts)
	if node == "" {
		return nil
	}
	report, err := collectRemoteChromeReport(node)
	if err != nil {
		return appendChromeReportBlock(reportPath, renderChromeReportUnavailable(node, err))
	}
	return appendChromeReportBlock(reportPath, renderChromeReport(report))
}

func resolveChromeReportNode(opts SuiteOptions) string {
	if n := strings.TrimSpace(opts.ChromeReportNode); n != "" {
		return n
	}
	return strings.TrimSpace(RuntimeConfigSnapshot().BrowserNode)
}

func collectRemoteChromeReport(node string) (*remoteChromeReport, error) {
	nodeInfo, err := sshv1.ResolveMeshNode(strings.TrimSpace(node))
	if err != nil {
		return nil, err
	}
	script := `
set -eu
procs="$(ps axww -o pid= -o command= | grep -Ei 'Google Chrome|google-chrome|chromium|msedge|microsoft edge' | grep -Ev 'grep|Crashpad|--type=|Helper \(Plugin\)|Helper \(Renderer\)|Helper \(GPU\)|Helper \(Alerts\)|Helper \(EH\)' || true)"
proc_count="$(printf '%s\n' "$procs" | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
echo "DIALTONE_REMOTE_CHROME_COUNT=${proc_count}"
if [ -n "$procs" ]; then
  printf '%s\n' "$procs" | while IFS= read -r line; do
    [ -n "$line" ] || continue
    pid="$(printf '%s' "$line" | awk '{print $1}')"
    cmd="$(printf '%s' "$line" | cut -d' ' -f2-)"
    arg_port="$(printf '%s' "$cmd" | sed -n 's/.*--remote-debugging-port=\([0-9][0-9]*\).*/\1/p' | head -n1)"
    listen_port="$(lsof -nP -a -p "$pid" -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $9}' | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' | head -n1 || true)"
    echo "DIALTONE_REMOTE_CHROME_PROCESS=${pid}|${arg_port}|${listen_port}|${cmd}"
  done
fi
`
	if strings.EqualFold(nodeInfo.OS, "windows") {
		script = `
$procs = Get-CimInstance Win32_Process | Where-Object {
  $_.Name -eq 'chrome.exe' -and $_.CommandLine -match '--remote-debugging-port'
}
$count = @($procs).Count
Write-Output ("DIALTONE_REMOTE_CHROME_COUNT=" + $count)
foreach ($p in $procs) {
  $cmd = [string]$p.CommandLine
  $argPort = ''
  if ($cmd -match '--remote-debugging-port=([0-9]+)') { $argPort = $Matches[1] }
  $listenPort = ''
  if ($argPort -ne '') { $listenPort = $argPort }
  Write-Output ("DIALTONE_REMOTE_CHROME_PROCESS=" + $p.ProcessId + "|" + $argPort + "|" + $listenPort + "|" + $cmd)
}
`
	}
	out, err := sshv1.RunNodeCommand(nodeInfo.Name, script, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote browser inventory on %s failed: %w", nodeInfo.Name, err)
	}
	return parseRemoteChromeReport(nodeInfo.Name, out), nil
}

func parseRemoteChromeReport(node, raw string) *remoteChromeReport {
	out := &remoteChromeReport{
		Node:        strings.TrimSpace(node),
		ChromeCount: -1,
		Processes:   make([]remoteChromeProcessInfo, 0),
	}
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "DIALTONE_REMOTE_CHROME_COUNT=") {
			countRaw := strings.TrimSpace(strings.TrimPrefix(line, "DIALTONE_REMOTE_CHROME_COUNT="))
			if n, err := strconv.Atoi(countRaw); err == nil {
				out.ChromeCount = n
			}
			continue
		}
		if !strings.HasPrefix(line, "DIALTONE_REMOTE_CHROME_PROCESS=") {
			continue
		}
		rawProc := strings.TrimPrefix(line, "DIALTONE_REMOTE_CHROME_PROCESS=")
		parts := strings.SplitN(rawProc, "|", 4)
		if len(parts) < 4 {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		argPort, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		listenPort, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
		cmd := strings.TrimSpace(parts[3])
		port := argPort
		if port <= 0 {
			port = listenPort
		}
		out.Processes = append(out.Processes, remoteChromeProcessInfo{
			PID:       pid,
			Role:      extractChromeRole(cmd),
			DebugPort: port,
			Command:   cmd,
		})
	}
	sort.SliceStable(out.Processes, func(i, j int) bool { return out.Processes[i].PID < out.Processes[j].PID })
	return out
}

func extractChromeRole(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "unknown"
	}
	if m := roleEqualsRe.FindStringSubmatch(cmd); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	if m := roleSplitRe.FindStringSubmatch(cmd); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return "unlabeled"
}

func renderChromeReport(r *remoteChromeReport) string {
	var sb strings.Builder
	sb.WriteString(chromeReportStart)
	sb.WriteString("\n\n## Chrome Report\n\n")
	sb.WriteString(fmt.Sprintf("- hostnode: `%s`\n", strings.TrimSpace(r.Node)))
	if r.ChromeCount >= 0 {
		sb.WriteString(fmt.Sprintf("- chrome_count: `%d`\n", r.ChromeCount))
	} else {
		sb.WriteString("- chrome_count: `unknown`\n")
	}
	if len(r.Processes) == 0 {
		sb.WriteString("- processes: `<empty>`\n")
		sb.WriteString("\n")
		sb.WriteString(chromeReportEnd)
		sb.WriteString("\n")
		return sb.String()
	}
	sb.WriteString("\n| PID | ROLE | PORT |\n")
	sb.WriteString("| --- | --- | --- |\n")
	for _, p := range r.Processes {
		port := "unknown"
		if p.DebugPort > 0 {
			port = strconv.Itoa(p.DebugPort)
		}
		sb.WriteString(fmt.Sprintf("| %d | `%s` | %s |\n", p.PID, strings.TrimSpace(p.Role), port))
	}
	sb.WriteString("\n")
	sb.WriteString(chromeReportEnd)
	sb.WriteString("\n")
	return sb.String()
}

func renderChromeReportUnavailable(node string, err error) string {
	var sb strings.Builder
	sb.WriteString(chromeReportStart)
	sb.WriteString("\n\n## Chrome Report\n\n")
	sb.WriteString(fmt.Sprintf("- hostnode: `%s`\n", strings.TrimSpace(node)))
	sb.WriteString("- chrome_count: `unknown`\n")
	sb.WriteString(fmt.Sprintf("- error: `%s`\n\n", strings.TrimSpace(err.Error())))
	sb.WriteString(chromeReportEnd)
	sb.WriteString("\n")
	return sb.String()
}

func appendChromeReportBlock(reportPath, block string) error {
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		return err
	}
	content := stripChromeReportBlock(string(raw))
	content = strings.TrimRight(content, "\n")
	if strings.TrimSpace(block) != "" {
		content += "\n\n" + strings.TrimSpace(block) + "\n"
	}
	return os.WriteFile(reportPath, []byte(content), 0644)
}

func stripChromeReportBlock(content string) string {
	start := strings.Index(content, chromeReportStart)
	end := strings.Index(content, chromeReportEnd)
	if start < 0 || end < 0 || end < start {
		return content
	}
	end += len(chromeReportEnd)
	for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
		end++
	}
	return content[:start] + content[end:]
}
