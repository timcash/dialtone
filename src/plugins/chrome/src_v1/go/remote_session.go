package chrome

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	"github.com/chromedp/chromedp"
)

type RemoteSessionOptions struct {
	Role               string
	URL                string
	Headless           bool
	GPU                bool
	PreferredDebugPort int
	DebugPorts         []int
	PreferredPID       int
	RequireRole        bool
	NoSSH              bool
	NoLaunch           bool
}

type RemoteSessionResult struct {
	Session *Session
	Closers []io.Closer
}

func StartRemoteSession(node string, opts RemoteSessionOptions) (*RemoteSessionResult, error) {
	// Dialtone remote browser sessions should always run with GPU enabled.
	opts.GPU = true
	nodeInfo, err := sshv1.ResolveMeshNode(strings.TrimSpace(node))
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(nodeInfo.OS), "windows") {
		return startRemoteSessionWindows(nodeInfo, opts)
	}
	if s, err := startRemoteSessionDirectTailnet(nodeInfo, opts); err == nil {
		return s, nil
	}
	if opts.NoSSH {
		return nil, fmt.Errorf("tailnet direct attach unavailable on %s in no-ssh mode", nodeInfo.Name)
	}
	if s, err := startRemoteSessionUnixNoRepo(nodeInfo, opts); err == nil {
		return s, nil
	}
	return nil, fmt.Errorf("remote session start failed on %s", nodeInfo.Name)
}

func startRemoteSessionDirectTailnet(nodeInfo sshv1.MeshNode, opts RemoteSessionOptions) (*RemoteSessionResult, error) {
	host := strings.TrimSpace(nodeInfo.Host)
	if host == "" {
		return nil, fmt.Errorf("tailnet host unavailable for node %s", nodeInfo.Name)
	}
	ports := normalizeRemoteDebugPorts(opts.PreferredDebugPort, opts.DebugPorts)
	if len(ports) == 0 {
		ports = []int{DefaultDebugPort, DefaultDebugPort + 1}
	}
	var lastErr error
	for _, p := range ports {
		if !canDialHostPort(host, p, 700*time.Millisecond) {
			lastErr = fmt.Errorf("cannot dial %s:%d", host, p)
			continue
		}
		ws, err := getWebsocketURLForHost(host, p)
		if err != nil {
			lastErr = err
			continue
		}
		ws = normalizeWebSocketURLHost(ws, host, p)
		if strings.TrimSpace(ws) == "" {
			lastErr = fmt.Errorf("empty websocket url")
			continue
		}
		return &RemoteSessionResult{Session: &Session{PID: 0, Port: p, WebSocketURL: ws, IsNew: false}}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no attachable tailnet debug endpoint")
	}
	return nil, lastErr
}

func startRemoteSessionUnixNoRepo(nodeInfo sshv1.MeshNode, opts RemoteSessionOptions) (*RemoteSessionResult, error) {
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = "about:blank"
	}
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	roleToken := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "'", "", "\"", "").Replace(role)
	roleToken = strings.Trim(roleToken, "-")
	if roleToken == "" {
		roleToken = "test"
	}
	preferredPID := opts.PreferredPID
	requireRole := opts.RequireRole

	probeScript := `
set -eu
procs="$(ps axww -o pid= -o command= | grep -Ei 'Google Chrome|google-chrome|chromium|msedge|microsoft edge' | grep -Ev 'grep|Crashpad|--type=|Helper \(Plugin\)|Helper \(Renderer\)|Helper \(GPU\)|Helper \(Alerts\)|Helper \(EH\)' || true)"
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
	if out, err := sshv1.RunNodeCommand(nodeInfo.Name, probeScript, sshv1.CommandOptions{}); err == nil {
		type candidate struct {
			meta SessionMetadata
			cmd  string
		}
		cmdByPID := make(map[int]string)
		candidates := make([]candidate, 0)
		seen := make(map[string]struct{})
		sc := bufio.NewScanner(strings.NewReader(out))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "DIALTONE_REMOTE_CHROME_PID=") {
				raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_CHROME_PID=")
				parts := strings.SplitN(raw, "|", 2)
				if len(parts) == 2 {
					if pid, perr := strconv.Atoi(strings.TrimSpace(parts[0])); perr == nil && pid > 0 {
						cmdByPID[pid] = strings.TrimSpace(parts[1])
					}
				}
				continue
			}
			if !strings.HasPrefix(line, "DIALTONE_REMOTE_DEBUG=") {
				continue
			}
			raw := strings.TrimPrefix(line, "DIALTONE_REMOTE_DEBUG=")
			parts := strings.SplitN(raw, "|", 3)
			if len(parts) < 3 {
				continue
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			port, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			ws := strings.TrimSpace(parts[2])
			if port <= 0 || ws == "" {
				continue
			}
			key := fmt.Sprintf("%d|%d|%s", pid, port, ws)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			candidates = append(candidates, candidate{meta: SessionMetadata{PID: pid, DebugPort: port, WebSocketURL: ws, WebSocketPath: WebSocketPathFromURL(ws), IsNew: false}, cmd: strings.ToLower(strings.TrimSpace(cmdByPID[pid]))})
		}
		candidateMatchesMode := func(c candidate) bool {
			cmd := strings.ToLower(strings.TrimSpace(c.cmd))
			if opts.Headless && !strings.Contains(cmd, "--headless") {
				return false
			}
			if !opts.Headless && strings.Contains(cmd, "--headless") {
				return false
			}
			if opts.GPU && strings.Contains(cmd, "--disable-gpu") {
				return false
			}
			return true
		}
		tryAttach := func(filter func(candidate) bool) (*RemoteSessionResult, bool) {
			filtered := make([]candidate, 0, len(candidates))
			for _, c := range candidates {
				if !candidateMatchesMode(c) {
					continue
				}
				if filter != nil && !filter(c) {
					continue
				}
				filtered = append(filtered, c)
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				score := func(c candidate) int {
					s := 0
					if preferredPID > 0 && c.meta.PID == preferredPID {
						s += 200
					}
					if strings.Contains(c.cmd, "--dialtone-role="+strings.ToLower(roleToken)) {
						s += 100
					}
					if !strings.Contains(c.cmd, "--headless") {
						s += 20
					}
					if !strings.Contains(c.cmd, "--disable-gpu") {
						s += 10
					}
					return s
				}
				return score(filtered[i]) > score(filtered[j])
			})
			for _, c := range filtered {
				if s, aerr := attachRemoteSession(nodeInfo, c.meta, opts.NoSSH); aerr == nil {
					if opts.GPU && !opts.Headless && !sessionSupportsWebGL(s.Session) {
						closeRemoteClosers(s.Closers)
						continue
					}
					return s, true
				}
			}
			return nil, false
		}
		roleNeedle := "--dialtone-role=" + strings.ToLower(roleToken)
		if preferredPID > 0 {
			if s, ok := tryAttach(func(c candidate) bool {
				if c.meta.PID != preferredPID {
					return false
				}
				if requireRole && !strings.Contains(c.cmd, roleNeedle) {
					return false
				}
				return true
			}); ok {
				return s, nil
			}
		}
		if s, ok := tryAttach(func(c candidate) bool { return strings.Contains(c.cmd, roleNeedle) }); ok {
			return s, nil
		}
		if !requireRole {
			if s, ok := tryAttach(nil); ok {
				return s, nil
			}
		}
	}

	if opts.NoLaunch {
		return nil, fmt.Errorf("remote launch disabled by options")
	}
	headlessFlag := ""
	if opts.Headless {
		headlessFlag = " --headless=new"
	}
	disableGPUFlag := ""
	if !opts.GPU {
		disableGPUFlag = " --disable-gpu"
	}
	launchScript := fmt.Sprintf(`
set -eu
url=%s
profile="$HOME/.dialtone-remote-%s-profile"
bin=""
for c in "google-chrome" "google-chrome-stable" "chromium-browser" "chromium" "microsoft-edge" "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"; do
  if [ -x "$c" ] || command -v "$c" >/dev/null 2>&1; then
    bin="$c"
    break
  fi
done
if [ -z "$bin" ]; then
  echo "no supported Chrome/Chromium executable found"
  exit 2
fi
port=%d
while lsof -nP -iTCP:${port} -sTCP:LISTEN >/dev/null 2>&1; do
  port=$((port+1))
done
cmd="$bin"
nohup "$cmd" --remote-debugging-port=${port} --remote-debugging-address=0.0.0.0 '--remote-allow-origins=*' --no-first-run --no-default-browser-check --user-data-dir="$profile" --new-window --dialtone-origin=true --dialtone-role=%s%s%s "$url" >/tmp/dialtone_remote_browser.log 2>&1 < /dev/null &
pid=$!
ok=0
for _ in $(seq 1 60); do
  if curl -fsS --max-time 1 "http://127.0.0.1:${port}/json/version" >/dev/null 2>&1; then
    ok=1
    break
  fi
  sleep 0.2
done
if [ "$ok" != "1" ]; then
  echo "remote browser debugger not ready"
  exit 4
fi
resp="$(curl -fsS --max-time 1 "http://127.0.0.1:${port}/json/version")"
ws="$(printf '%s' "$resp" | sed -n 's/.*"webSocketDebuggerUrl":"\([^"]*\)".*/\1/p' | head -n1)"
path="$(printf '%s' "$ws" | sed -n 's#^ws://[^/]*/\(.*\)$#/\1#p' | head -n1)"
echo "DIALTONE_CHROME_SESSION_JSON={\"pid\":${pid},\"debug_port\":${port},\"websocket_url\":\"${ws}\",\"websocket_path\":\"${path:-}\",\"is_new\":true}"
`, remoteShellQuote(url), roleToken, DefaultDebugPort, roleToken, headlessFlag, disableGPUFlag)
	out, err := sshv1.RunNodeCommand(nodeInfo.Name, launchScript, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote no-repo browser launch on %s failed: %v output=%s", nodeInfo.Name, err, strings.TrimSpace(out))
	}
	raw := extractChromeSessionJSON(outputTrim(out))
	if raw == "" {
		return nil, fmt.Errorf("remote no-repo browser launch missing metadata marker")
	}
	var meta SessionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("decode remote no-repo browser metadata: %w", err)
	}
	if meta.DebugPort <= 0 {
		return nil, fmt.Errorf("invalid remote no-repo debug port %d", meta.DebugPort)
	}
	return attachRemoteSession(nodeInfo, meta, opts.NoSSH)
}

func startRemoteSessionWindows(nodeInfo sshv1.MeshNode, opts RemoteSessionOptions) (*RemoteSessionResult, error) {
	role := strings.TrimSpace(opts.Role)
	if role == "" {
		role = "test"
	}
	if opts.NoSSH && strings.EqualFold(nodeInfo.OS, "windows") && !opts.Headless {
		opts.Headless = true
	}
	headless := "$true"
	if !opts.Headless {
		headless = "$false"
	}
	gpuDisabled := "$true"
	if opts.GPU {
		gpuDisabled = "$false"
	}
	url := strings.TrimSpace(opts.URL)
	if url == "" {
		url = "about:blank"
	}
	preferredPort := opts.PreferredDebugPort
	if preferredPort <= 0 {
		preferredPort = DefaultDebugPort
	}
	portCandidates := normalizeRemoteDebugPorts(preferredPort, opts.DebugPorts)
	allowPortBumpPS := "$true"
	if opts.NoSSH {
		allowPortBumpPS = "$false"
	}
	allowLaunchPS := "$true"
	if opts.NoLaunch {
		allowLaunchPS = "$false"
	}
	portCandidatesPS := psIntArray(portCandidates)
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$paths=@("$env:ProgramFiles\Google\Chrome\Application\chrome.exe","$env:ProgramFiles(x86)\Google\Chrome\Application\chrome.exe","$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe")
$exe=$null
foreach($p in $paths){ if(Test-Path $p){ $exe=$p; break } }
if(-not $exe){ Write-Error "chrome executable not found"; exit 1 }
$ports=%s
$allowPortBump=%s
$allowLaunch=%s
function Get-DialtoneDebugVersion([int]$p){
  try{
    $raw=& curl.exe -sS --max-time 1 ("http://127.0.0.1:{0}/json/version" -f $p) 2>$null
    if(-not $raw){ return $null }
    return ($raw | ConvertFrom-Json)
  }catch{
    return $null
  }
}
$port=$null
foreach($candidate in $ports){
  $v=Get-DialtoneDebugVersion([int]$candidate)
  if($v -and $v.webSocketDebuggerUrl){
    $path=([Uri]$v.webSocketDebuggerUrl).PathAndQuery
    $obj=[PSCustomObject]@{ pid=0; debug_port=$candidate; websocket_url=$v.webSocketDebuggerUrl; websocket_path=$path; debug_url=("http://127.0.0.1:{0}{1}" -f $candidate,$path); is_new=$false; generated_at_rfc3339=(Get-Date).ToUniversalTime().ToString("o") }
    $json=$obj | ConvertTo-Json -Compress
    Write-Output ("DIALTONE_CHROME_SESSION_JSON="+$json)
    exit 0
  }
  $used=Get-NetTCPConnection -State Listen -LocalPort $candidate -ErrorAction SilentlyContinue
  if(-not $used){ $port=[int]$candidate; break }
}
if(-not $allowLaunch){
  Write-Error "remote-no-launch enabled and no existing debugger found"
  exit 1
}
if($null -eq $port){
  if($allowPortBump){
    $base=[int]$ports[-1]
    for($attempt=0;$attempt -lt 20;$attempt++){
      $cand=$base+$attempt+1
      $used=Get-NetTCPConnection -State Listen -LocalPort $cand -ErrorAction SilentlyContinue
      if(-not $used){ $port=$cand; break }
    }
  }
}
if($null -eq $port){
  $port=[int]$ports[0]
}
$profile=Join-Path $env:TEMP ("dialtone-remote-%s-p"+$port)
$rule=("Dialtone Chrome DevTools "+$port)
try{
  if(-not (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName $rule -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any | Out-Null
  }
}catch{}
$args=@("--remote-debugging-port=$port","--remote-debugging-address=0.0.0.0","--remote-allow-origins=*","--no-first-run","--no-default-browser-check","--user-data-dir=$profile","--new-window","--dialtone-origin=true","--dialtone-role=%s")
if(%s){ $args += "--headless=new" }
if(%s){ $args += "--disable-gpu" }
$args += %s
$proc=Start-Process -FilePath $exe -ArgumentList $args -PassThru
$ws=$null
for($i=0;$i -lt 45;$i++){
  $v=Get-DialtoneDebugVersion([int]$port)
  if($v -and $v.webSocketDebuggerUrl){ $ws=$v.webSocketDebuggerUrl; break }
  Start-Sleep -Milliseconds 150
}
if(-not $ws){ Write-Error "debug websocket not ready"; exit 1 }
$path=([Uri]$ws).PathAndQuery
$obj=[PSCustomObject]@{ pid=$proc.Id; debug_port=$port; websocket_url=$ws; websocket_path=$path; debug_url=("http://127.0.0.1:{0}{1}" -f $port,$path); is_new=$true; generated_at_rfc3339=(Get-Date).ToUniversalTime().ToString("o") }
$json=$obj | ConvertTo-Json -Compress
Write-Output ("DIALTONE_CHROME_SESSION_JSON="+$json)`, portCandidatesPS, allowPortBumpPS, allowLaunchPS, role, role, headless, gpuDisabled, psLiteral(url))

	out, err := sshv1.RunNodeCommand(nodeInfo.Name, ps, sshv1.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("remote windows command on %s failed: %v output=%s", nodeInfo.Name, err, strings.TrimSpace(out))
	}
	raw := extractChromeSessionJSON(outputTrim(out))
	if raw == "" {
		return nil, fmt.Errorf("remote windows chrome session output missing metadata marker")
	}
	var meta SessionMetadata
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return nil, fmt.Errorf("decode remote windows chrome session metadata: %w", err)
	}
	if meta.DebugPort <= 0 {
		return nil, fmt.Errorf("invalid remote debug port %d", meta.DebugPort)
	}
	wsPath := strings.TrimSpace(meta.WebSocketPath)
	if wsPath == "" {
		wsPath = WebSocketPathFromURL(meta.WebSocketURL)
	}
	if wsPath == "" {
		return nil, fmt.Errorf("remote windows websocket path is empty")
	}
	attachHost := ""
	attachPort := meta.DebugPort
	var tunnelCloser io.Closer

	if h := resolveReachableDebugHost(meta.DebugPort, nodeInfo); h != "" {
		attachHost = h
	} else {
		if opts.NoSSH {
			relayPort := meta.DebugPort + 10000
			if h2 := resolveReachableDebugHost(relayPort, nodeInfo); h2 != "" {
				attachHost = h2
				attachPort = relayPort
			} else if rerr := ensureWindowsDebugRelay(nodeInfo, relayPort, meta.DebugPort); rerr == nil {
				if h3 := resolveReachableDebugHost(relayPort, nodeInfo); h3 != "" {
					attachHost = h3
					attachPort = relayPort
				}
			}
			if attachHost == "" {
				return nil, fmt.Errorf("remote windows debug port %d is not reachable from this node without SSH tunnel", meta.DebugPort)
			}
		} else if client, lport, err := openSSHDebugTunnel(nodeInfo, meta.DebugPort); err == nil {
			attachHost = "127.0.0.1"
			attachPort = lport
			tunnelCloser = client
		}
		if attachHost == "" {
			return nil, fmt.Errorf("remote windows debug port %d is not reachable from this node", meta.DebugPort)
		}
	}
	if attachHost == "127.0.0.1" && attachPort > 0 {
		if localWS, err := getWebsocketURL(attachPort); err == nil && strings.TrimSpace(localWS) != "" {
			if p := WebSocketPathFromURL(localWS); strings.TrimSpace(p) != "" {
				wsPath = p
			}
		}
	}
	if attachPort > 0 {
		if p := refreshWebSocketPathForAttach(attachHost, attachPort, wsPath); p != "" {
			wsPath = p
		}
	}
	sess := &Session{PID: meta.PID, Port: attachPort, WebSocketURL: fmt.Sprintf("ws://%s:%d%s", attachHost, attachPort, wsPath), IsNew: false}
	res := &RemoteSessionResult{Session: sess}
	if tunnelCloser != nil {
		res.Closers = append(res.Closers, tunnelCloser)
	}
	return res, nil
}

func sessionSupportsWebGL(s *Session) bool {
	if s == nil || strings.TrimSpace(s.WebSocketURL) == "" {
		return false
	}
	ctx, cancel, err := AttachToWebSocket(s.WebSocketURL)
	if err != nil {
		return false
	}
	defer cancel()
	var ok bool
	js := `(() => { try { const c = document.createElement("canvas"); return !!(c.getContext("webgl2") || c.getContext("webgl") || c.getContext("experimental-webgl")); } catch (_) { return false; } })()`
	if err := chromedp.Run(ctx, chromedp.Evaluate(js, &ok)); err != nil {
		return false
	}
	return ok
}

func attachRemoteSession(nodeInfo sshv1.MeshNode, meta SessionMetadata, noSSH bool) (*RemoteSessionResult, error) {
	wsPath := strings.TrimSpace(meta.WebSocketPath)
	if wsPath == "" {
		wsPath = WebSocketPathFromURL(meta.WebSocketURL)
	}
	if wsPath == "" {
		return nil, fmt.Errorf("remote websocket path is empty")
	}
	attachHost := strings.TrimSpace(nodeInfo.Host)
	attachPort := meta.DebugPort
	var tunnelCloser io.Closer
	if attachHost == "" || !canDialHostPort(attachHost, attachPort, 1500*time.Millisecond) {
		if noSSH {
			return nil, fmt.Errorf("tailnet direct attach to %s:%d unavailable (no-ssh mode)", attachHost, attachPort)
		}
		if closer, lport, err := openSSHDebugTunnel(nodeInfo, meta.DebugPort); err == nil {
			attachHost = "127.0.0.1"
			attachPort = lport
			tunnelCloser = closer
			if localWS, werr := getWebsocketURL(attachPort); werr == nil && strings.TrimSpace(localWS) != "" {
				if p := WebSocketPathFromURL(localWS); strings.TrimSpace(p) != "" {
					wsPath = p
				}
			}
		}
	}
	if strings.TrimSpace(attachHost) == "" {
		return nil, fmt.Errorf("remote attach host is empty")
	}
	if attachPort > 0 {
		if p := refreshWebSocketPathForAttach(attachHost, attachPort, wsPath); p != "" {
			wsPath = p
		}
	}
	sess := &Session{PID: meta.PID, Port: attachPort, WebSocketURL: fmt.Sprintf("ws://%s:%d%s", attachHost, attachPort, wsPath), IsNew: false}
	res := &RemoteSessionResult{Session: sess}
	if tunnelCloser != nil {
		res.Closers = append(res.Closers, tunnelCloser)
	}
	return res, nil
}

func refreshWebSocketPathForAttach(host string, port int, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if port <= 0 {
		return fallback
	}
	// Browser IDs can rotate on reconnect/restart; refresh from /json/version.
	for i := 0; i < 3; i++ {
		if strings.TrimSpace(host) == "" || host == "127.0.0.1" || strings.EqualFold(host, "localhost") {
			if ws, err := getWebsocketURL(port); err == nil && strings.TrimSpace(ws) != "" {
				if p := WebSocketPathFromURL(ws); strings.TrimSpace(p) != "" {
					return p
				}
			}
		} else {
			if ws, err := getWebsocketURLForHost(host, port); err == nil && strings.TrimSpace(ws) != "" {
				if p := WebSocketPathFromURL(ws); strings.TrimSpace(p) != "" {
					return p
				}
			}
		}
		time.Sleep(180 * time.Millisecond)
	}
	return fallback
}
