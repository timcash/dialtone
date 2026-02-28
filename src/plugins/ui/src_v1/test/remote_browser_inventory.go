package test

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type RemoteDebugEndpoint struct {
	PID          int
	DebugPort    int
	WebSocketURL string
}

type RemoteChromeProcess struct {
	PID     int
	Command string
}

type RemoteBrowserInventory struct {
	Node        string
	ChromeCount int
	Processes   []RemoteChromeProcess
	Endpoints   []RemoteDebugEndpoint
}

func LogRemoteBrowserInventory(node, phase string) (*RemoteBrowserInventory, error) {
	node = strings.TrimSpace(node)
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = "inventory"
	}
	if node == "" {
		return &RemoteBrowserInventory{
			Node:        "",
			ChromeCount: 0,
		}, nil
	}
	nodeInfo, err := sshv1.ResolveMeshNode(node)
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
    echo "DIALTONE_REMOTE_CHROME_PID=${pid}|${cmd}"
    arg_port="$(printf '%s' "$cmd" | sed -n 's/.*--remote-debugging-port=\([0-9][0-9]*\).*/\1/p' | head -n1)"
    listen_ports="$(lsof -nP -a -p "$pid" -iTCP -sTCP:LISTEN 2>/dev/null | awk 'NR>1 {print $9}' | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' || true)"
    ports="$(printf '%s\n%s\n' "$arg_port" "$listen_ports" | sed '/^[[:space:]]*$/d' | sort -u || true)"
    if [ -z "$ports" ]; then
      continue
    fi
    printf '%s\n' "$ports" | while IFS= read -r p; do
      [ -n "$p" ] || continue
      resp="$(curl -fsS --max-time 1 "http://127.0.0.1:${p}/json/version" 2>/dev/null || true)"
      if [ -n "$resp" ] && printf '%s' "$resp" | grep -q "webSocketDebuggerUrl"; then
        ws="$(printf '%s' "$resp" | tr -d '\n' | sed -n 's/.*"webSocketDebuggerUrl"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
        echo "DIALTONE_REMOTE_DEBUG=${pid}|${p}|${ws}"
      fi
    done
  done
fi
`

	out, err := sshv1.RunNodeCommand(nodeInfo.Name, script, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote browser inventory on %s failed: %w", nodeInfo.Name, err)
	}

	count := -1
	var procs []RemoteChromeProcess
	var endpoints []RemoteDebugEndpoint
	seen := make(map[string]struct{})
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "DIALTONE_REMOTE_CHROME_COUNT=") {
			raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_CHROME_COUNT=")
			if n, nerr := strconv.Atoi(strings.TrimSpace(raw)); nerr == nil {
				count = n
			}
			continue
		}
		if strings.HasPrefix(line, "DIALTONE_REMOTE_DEBUG=") {
			raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_DEBUG=")
			parts := strings.SplitN(raw, "|", 3)
			if len(parts) < 3 {
				continue
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			port, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			ws := strings.TrimSpace(parts[2])
			key := fmt.Sprintf("%d|%d|%s", pid, port, ws)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			endpoints = append(endpoints, RemoteDebugEndpoint{
				PID:          pid,
				DebugPort:    port,
				WebSocketURL: ws,
			})
		}
		if strings.HasPrefix(line, "DIALTONE_REMOTE_CHROME_PID=") {
			raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_CHROME_PID=")
			parts := strings.SplitN(raw, "|", 2)
			if len(parts) < 2 {
				continue
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			cmd := strings.TrimSpace(parts[1])
			procs = append(procs, RemoteChromeProcess{
				PID:     pid,
				Command: cmd,
			})
		}
	}

	if count < 0 {
		logs.Info("ui attach %s node=%s chrome_count=unknown debug_endpoints=%d", phase, nodeInfo.Name, len(endpoints))
	} else {
		logs.Info("ui attach %s node=%s chrome_count=%d debug_endpoints=%d", phase, nodeInfo.Name, count, len(endpoints))
	}
	for _, p := range procs {
		logs.Info("ui attach %s chrome pid=%d cmd=%s", phase, p.PID, p.Command)
	}
	for _, ep := range endpoints {
		logs.Info("ui attach %s debug endpoint pid=%d port=%d ws=%s", phase, ep.PID, ep.DebugPort, ep.WebSocketURL)
	}
	return &RemoteBrowserInventory{
		Node:        nodeInfo.Name,
		ChromeCount: count,
		Processes:   procs,
		Endpoints:   endpoints,
	}, nil
}
