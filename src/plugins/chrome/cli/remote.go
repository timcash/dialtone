package cli

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type remoteChromeProcess struct {
	Node      string
	Host      string
	PID       int
	PPID      int
	Command   string
	Headless  bool
	GPU       bool
	DebugPort int
	Origin    string
	Role      string
}

func handleRemoteListCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-list", flag.ExitOnError)
	nodesArg := fs.String("nodes", "all", "Target nodes csv or 'all'")
	headed := fs.Bool("headed", false, "Show only headed processes")
	headless := fs.Bool("headless", false, "Show only headless processes")
	role := fs.String("role", "", "Filter role tag")
	origin := fs.String("origin", "", "Filter origin: dialtone|other")
	verbose := fs.Bool("verbose", false, "Show full command")
	asJSON := fs.Bool("json", false, "Emit JSON")
	_ = fs.Parse(args)

	nodes, err := resolveTargetNodes(*nodesArg)
	if err != nil {
		logs.Fatal("remote-list: %v", err)
	}

	rows := make([]remoteChromeProcess, 0)
	for _, node := range nodes {
		procs, perr := listRemoteNodeChrome(node)
		if perr != nil {
			logs.Warn("remote-list node=%s failed: %v", node.Name, perr)
			continue
		}
		for _, p := range procs {
			if *headed && p.Headless {
				continue
			}
			if *headless && !p.Headless {
				continue
			}
			if strings.TrimSpace(*role) != "" && !strings.EqualFold(strings.TrimSpace(*role), p.Role) {
				continue
			}
			if o := strings.TrimSpace(strings.ToLower(*origin)); o != "" {
				wantDialtone := o == "dialtone"
				if wantDialtone && !strings.EqualFold(p.Origin, "Dialtone") {
					continue
				}
				if !wantDialtone && strings.EqualFold(p.Origin, "Dialtone") {
					continue
				}
			}
			rows = append(rows, p)
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Node == rows[j].Node {
			return rows[i].PID < rows[j].PID
		}
		return rows[i].Node < rows[j].Node
	})

	if *asJSON {
		raw, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(raw))
		return
	}

	if len(rows) == 0 {
		logs.Info("remote-list: no matching Chrome processes")
		return
	}
	if *verbose {
		fmt.Printf("%-10s %-33s %-7s %-7s %-8s %-9s %-7s %-10s %-16s %s\n", "NODE", "HOST", "PID", "PPID", "HEADLESS", "GPU", "PORT", "ORIGIN", "ROLE", "COMMAND")
		fmt.Println(strings.Repeat("-", 190))
		for _, r := range rows {
			port := "-"
			if r.DebugPort > 0 {
				port = strconv.Itoa(r.DebugPort)
			}
			fmt.Printf("%-10s %-33s %-7d %-7d %-8t %-9t %-7s %-10s %-16s %s\n", r.Node, r.Host, r.PID, r.PPID, r.Headless, r.GPU, port, r.Origin, r.Role, r.Command)
		}
	} else {
		fmt.Printf("%-10s %-33s %-7s %-8s %-9s %-7s %-10s %-16s\n", "NODE", "HOST", "PID", "HEADLESS", "GPU", "PORT", "ORIGIN", "ROLE")
		fmt.Println(strings.Repeat("-", 112))
		for _, r := range rows {
			port := "-"
			if r.DebugPort > 0 {
				port = strconv.Itoa(r.DebugPort)
			}
			fmt.Printf("%-10s %-33s %-7d %-8t %-9t %-7s %-10s %-16s\n", r.Node, r.Host, r.PID, r.Headless, r.GPU, port, r.Origin, r.Role)
		}
	}
	logs.Info("remote-list: %d process(es) across %d node(s)", len(rows), len(nodes))
}

func handleRemoteNewCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-new", flag.ExitOnError)
	nodeName := fs.String("node", "", "Target mesh node")
	url := fs.String("url", "about:blank", "Initial URL")
	port := fs.Int("port", chrome.DefaultDebugPort, "Remote debugging port")
	role := fs.String("role", "dev", "Role tag (dev|test)")
	headless := fs.Bool("headless", false, "Headless mode")
	gpu := fs.Bool("gpu", true, "Enable GPU")
	debugAddress := fs.String("debug-address", "0.0.0.0", "Remote debug bind address")
	reuseExisting := fs.Bool("reuse-existing", true, "Reuse if /json/version is already available")
	_ = fs.Parse(args)

	n := strings.TrimSpace(*nodeName)
	if n == "" {
		logs.Fatal("remote-new: --node is required")
	}
	node, err := sshv1.ResolveMeshNode(n)
	if err != nil {
		logs.Fatal("remote-new: %v", err)
	}
	if err := startRemoteChrome(node, remoteStartOptions{
		URL:           strings.TrimSpace(*url),
		Port:          *port,
		Role:          strings.TrimSpace(*role),
		Headless:      *headless,
		GPU:           *gpu,
		DebugAddress:  strings.TrimSpace(*debugAddress),
		ReuseExisting: *reuseExisting,
	}); err != nil {
		logs.Fatal("remote-new node=%s failed: %v", node.Name, err)
	}
}

func handleRemoteProbeCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-probe", flag.ExitOnError)
	nodesArg := fs.String("nodes", "all", "Target nodes csv or 'all'")
	portsArg := fs.String("ports", fmt.Sprintf("%d,%d", chrome.DefaultDebugPort, chrome.DefaultDebugPort+1), "Ports csv")
	timeoutMS := fs.Int("timeout-ms", 800, "Dial timeout in milliseconds")
	_ = fs.Parse(args)

	nodes, err := resolveTargetNodes(*nodesArg)
	if err != nil {
		logs.Fatal("remote-probe: %v", err)
	}
	ports := parsePortsCSV(*portsArg)
	if len(ports) == 0 {
		logs.Fatal("remote-probe: no valid ports")
	}

	fmt.Printf("%-10s %-33s %-6s %-11s %-12s %-12s\n", "NODE", "HOST", "PORT", "LOCAL-DIAL", "REMOTE-LISTEN", "JSON/VERSION")
	fmt.Println(strings.Repeat("-", 96))
	for _, node := range nodes {
		for _, p := range ports {
			localDial := canDialHost(node.Host, p, time.Duration(*timeoutMS)*time.Millisecond)
			remoteListen, remoteVersion := probeRemoteNodePort(node, p)
			fmt.Printf("%-10s %-33s %-6d %-11t %-12t %-12t\n", node.Name, node.Host, p, localDial, remoteListen, remoteVersion)
		}
	}
}

func handleRemoteRelayCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-relay", flag.ExitOnError)
	nodeName := fs.String("node", "", "Target mesh node")
	listenPort := fs.Int("listen-port", 9223, "Relay listen port on remote host")
	targetPort := fs.Int("target-port", chrome.DefaultDebugPort, "Target debug port on remote localhost")
	stop := fs.Bool("stop", false, "Stop relay on remote host instead of starting")
	_ = fs.Parse(args)

	if strings.TrimSpace(*nodeName) == "" {
		logs.Fatal("remote-relay: --node is required")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*nodeName))
	if err != nil {
		logs.Fatal("remote-relay: %v", err)
	}
	if node.OS == "windows" {
		logs.Fatal("remote-relay: windows relay helper is not implemented yet")
	}
	cmd := buildRemoteRelayShell(*listenPort, *targetPort, *stop)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		logs.Fatal("remote-relay node=%s failed: %v output=%s", node.Name, err, strings.TrimSpace(out))
	}
	if *stop {
		logs.Info("remote-relay node=%s stopped on listen=%d", node.Name, *listenPort)
	} else {
		logs.Info("remote-relay node=%s listen=%d target=%d", node.Name, *listenPort, *targetPort)
	}
	if strings.TrimSpace(out) != "" {
		fmt.Println(strings.TrimSpace(out))
	}
}

func handleRemoteDoctorCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-doctor", flag.ExitOnError)
	nodesArg := fs.String("nodes", "all", "Target nodes csv or 'all'")
	portsArg := fs.String("ports", fmt.Sprintf("%d,%d", chrome.DefaultDebugPort, chrome.DefaultDebugPort+1), "Ports csv")
	timeoutMS := fs.Int("timeout-ms", 800, "Dial timeout in milliseconds")
	_ = fs.Parse(args)

	nodes, err := resolveTargetNodes(*nodesArg)
	if err != nil {
		logs.Fatal("remote-doctor: %v", err)
	}
	ports := parsePortsCSV(*portsArg)
	if len(ports) == 0 {
		logs.Fatal("remote-doctor: no valid ports")
	}

	for _, node := range nodes {
		fmt.Printf("\n== node=%s host=%s os=%s ==\n", node.Name, node.Host, node.OS)
		procs, perr := listRemoteNodeChrome(node)
		if perr != nil {
			fmt.Printf("chrome_processes: error=%v\n", perr)
		} else {
			fmt.Printf("chrome_processes: %d\n", len(procs))
			for _, p := range procs {
				if p.DebugPort > 0 {
					fmt.Printf("- pid=%d role=%s headless=%t port=%d origin=%s\n", p.PID, p.Role, p.Headless, p.DebugPort, p.Origin)
				}
			}
		}
		for _, p := range ports {
			localDial := canDialHost(node.Host, p, time.Duration(*timeoutMS)*time.Millisecond)
			remoteListen, remoteVersion := probeRemoteNodePort(node, p)
			bind := inspectRemoteListenerBind(node, p)
			fmt.Printf("port=%d local_dial=%t remote_listen=%t json_version=%t bind=%s\n", p, localDial, remoteListen, remoteVersion, bind)
			if remoteListen && !localDial {
				fmt.Printf("hint: listener exists but is not reachable from mesh; likely loopback-only bind or network ACL\n")
			}
			if remoteListen && bind != "-" && strings.Contains(bind, "127.0.0.1") {
				fmt.Printf("hint: listener bound to localhost; use --debug-address 0.0.0.0 or remote-relay\n")
			}
		}
	}
}

func handleRemoteKillCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-kill", flag.ExitOnError)
	nodesArg := fs.String("nodes", "all", "Target nodes csv or 'all'")
	roleArg := fs.String("role", "", "Optional role filter (dev|test)")
	killAll := fs.Bool("all", false, "Kill all chrome/msedge processes (not only Dialtone-origin)")
	_ = fs.Parse(args)

	nodes, err := resolveTargetNodes(*nodesArg)
	if err != nil {
		logs.Fatal("remote-kill: %v", err)
	}
	roleFilter := strings.ToLower(strings.TrimSpace(*roleArg))
	killed := 0
	for _, node := range nodes {
		procs, perr := listRemoteNodeChrome(node)
		if perr != nil {
			logs.Warn("remote-kill node=%s list failed: %v", node.Name, perr)
			continue
		}
		for _, p := range procs {
			if !*killAll && !strings.EqualFold(p.Origin, "Dialtone") {
				continue
			}
			if roleFilter != "" && strings.ToLower(strings.TrimSpace(p.Role)) != roleFilter {
				continue
			}
			if p.PID <= 0 {
				continue
			}
			if err := killRemoteNodePID(node, p.PID); err != nil {
				logs.Warn("remote-kill node=%s pid=%d failed: %v", node.Name, p.PID, err)
				continue
			}
			killed++
		}
	}
	logs.Info("remote-kill: terminated %d process(es) across %d node(s)", killed, len(nodes))
}

func handleRemoteWSLForwardCmd(args []string) {
	fs := flag.NewFlagSet("chrome remote-wsl-forward", flag.ExitOnError)
	nodeName := fs.String("node", "legion", "Windows mesh node to configure")
	portsArg := fs.String("ports", fmt.Sprintf("%d", chrome.DefaultDebugPort), "Ports csv")
	remove := fs.Bool("remove", false, "Remove forwarding/firewall rules instead of creating them")
	_ = fs.Parse(args)

	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*nodeName))
	if err != nil {
		logs.Fatal("remote-wsl-forward: %v", err)
	}
	if !strings.EqualFold(node.OS, "windows") {
		logs.Fatal("remote-wsl-forward: node=%s is not windows (os=%s)", node.Name, node.OS)
	}
	ports := parsePortsCSV(*portsArg)
	if len(ports) == 0 {
		logs.Fatal("remote-wsl-forward: no valid ports")
	}
	ps := buildRemoteWSLForwardPowerShell(ports, *remove)
	encoded := encodePowerShellUTF16Base64(ps)
	out, err := sshv1.RunNodeCommand(node.Name, "powershell -NoProfile -EncodedCommand "+encoded, sshv1.CommandOptions{})
	if err != nil {
		logs.Fatal("remote-wsl-forward node=%s failed: %v output=%s", node.Name, err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "" {
		fmt.Println(strings.TrimSpace(out))
	}
	if *remove {
		logs.Info("remote-wsl-forward removed on node=%s ports=%v", node.Name, ports)
	} else {
		logs.Info("remote-wsl-forward configured on node=%s ports=%v", node.Name, ports)
	}
}

func buildRemoteWSLForwardPowerShell(ports []int, remove bool) string {
	portVals := make([]string, 0, len(ports))
	for _, p := range ports {
		if p > 0 {
			portVals = append(portVals, strconv.Itoa(p))
		}
	}
	removePS := "$false"
	if remove {
		removePS = "$true"
	}
	return fmt.Sprintf(`$remove=%s; $ports=@(%s); foreach($port in $ports){ netsh interface portproxy delete v4tov4 listenport=$port listenaddress=0.0.0.0 | Out-Null; $name="Dialtone Chrome DevTools WSL $port"; if(Get-NetFirewallRule -DisplayName $name -ErrorAction SilentlyContinue){ Remove-NetFirewallRule -DisplayName $name | Out-Null }; if(-not $remove){ netsh interface portproxy add v4tov4 listenport=$port listenaddress=0.0.0.0 connectport=$port connectaddress=127.0.0.1 | Out-Null; New-NetFirewallRule -DisplayName $name -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any | Out-Null; Write-Output ("configured:"+$port) } else { Write-Output ("removed:"+$port) } }`, removePS, strings.Join(portVals, ","))
}

func encodePowerShellUTF16Base64(script string) string {
	u16 := utf16.Encode([]rune(script))
	buf := make([]byte, 0, len(u16)*2)
	for _, v := range u16 {
		buf = append(buf, byte(v), byte(v>>8))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

type remoteStartOptions struct {
	URL           string
	Port          int
	Role          string
	Headless      bool
	GPU           bool
	DebugAddress  string
	ReuseExisting bool
}

func startRemoteChrome(node sshv1.MeshNode, opts remoteStartOptions) error {
	if opts.Port <= 0 {
		opts.Port = chrome.DefaultDebugPort
	}
	// Keep remote launches consistent with Dialtone policy: always GPU-enabled.
	opts.GPU = true
	if opts.URL == "" {
		opts.URL = "about:blank"
	}
	if opts.Role == "" {
		opts.Role = "dev"
	}
	if opts.DebugAddress == "" {
		opts.DebugAddress = "0.0.0.0"
	}

	if node.OS == "windows" {
		reusePS := "$false"
		if opts.ReuseExisting {
			reusePS = "$true"
		}
		headlessPS := "$false"
		if opts.Headless {
			headlessPS = "$true"
		}
		gpuPS := "$false"
		if opts.GPU {
			gpuPS = "$true"
		}
		ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$paths=@("$env:ProgramFiles\Google\Chrome\Application\chrome.exe","$env:ProgramFiles(x86)\Google\Chrome\Application\chrome.exe","$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe")
$exe=$null
foreach($p in $paths){ if(Test-Path $p){ $exe=$p; break } }
if(-not $exe){ Write-Error "chrome executable not found"; exit 1 }
$port=%d
if (%s) {
  try { $v=Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/json/version" -f $port) -TimeoutSec 1; if($v.webSocketDebuggerUrl){ Write-Output ("reused debugger on :{0}" -f $port); Write-Output ($v | ConvertTo-Json -Compress); exit 0 } } catch {}
}
for($attempt=0;$attempt -lt 8;$attempt++){
  $used=Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
  if(-not $used){ break }
  $port=$port+1
}
$profile=Join-Path $env:TEMP ("dialtone-chrome-%s-port-"+$port)
$rule=("Dialtone Chrome DevTools "+$port)
try{
  if(-not (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName $rule -Direction Inbound -Action Allow -Protocol TCP -LocalPort $port -Profile Any | Out-Null
  }
}catch{}
$args=@("--remote-debugging-port=$port","--remote-debugging-address=%s","--remote-allow-origins=*","--no-first-run","--no-default-browser-check","--user-data-dir=$profile","--new-window","--dialtone-origin=true","--dialtone-role=%s")
if(%s){ $args += "--headless=new" }
if(%s){ } else { $args += "--disable-gpu" }
$args += %q
$proc=Start-Process -FilePath $exe -ArgumentList $args -PassThru
for($i=0;$i -lt 60;$i++){ try{ $v=Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/json/version" -f $port) -TimeoutSec 1; if($v.webSocketDebuggerUrl){ Write-Output ("started pid="+$proc.Id); Write-Output ($v | ConvertTo-Json -Compress); exit 0 } }catch{}; Start-Sleep -Milliseconds 200 }
Write-Error "debug endpoint not ready on :$port"; exit 2`,
			opts.Port, reusePS, opts.Role, opts.DebugAddress, opts.Role, headlessPS, gpuPS, opts.URL)
		out, err := sshv1.RunNodeCommand(node.Name, ps, sshv1.CommandOptions{})
		if err != nil {
			return fmt.Errorf("remote-new windows failed: %v output=%s", err, strings.TrimSpace(out))
		}
		fmt.Println(strings.TrimSpace(out))
		return nil
	}

	headlessFlag := ""
	if opts.Headless {
		headlessFlag = " --headless=new"
	}
	disableGPUFlag := ""
	if !opts.GPU {
		disableGPUFlag = " --disable-gpu"
	}
	cmd := fmt.Sprintf(`set -eu
port=%d
if [ %t = true ]; then
  if curl -fsS --max-time 1 "http://127.0.0.1:${port}/json/version" >/dev/null 2>&1; then
    echo "reused debugger on :${port}"
    curl -fsS --max-time 2 "http://127.0.0.1:${port}/json/version"
    exit 0
  fi
fi
bin=""
for c in "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" "google-chrome" "google-chrome-stable" "chromium-browser" "chromium" "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"; do
  if [ -x "$c" ] || command -v "$c" >/dev/null 2>&1; then bin="$c"; break; fi
done
if [ -z "$bin" ]; then echo "chrome executable not found"; exit 1; fi
profile="$HOME/.dialtone/chrome-%s-port-${port}"
mkdir -p "$HOME/.dialtone"
nohup "$bin" --remote-debugging-port="${port}" --remote-debugging-address=%s '--remote-allow-origins=*' --no-first-run --no-default-browser-check --user-data-dir="$profile" --new-window --dialtone-origin=true --dialtone-role=%s%s%s %q >"$HOME/.dialtone/chrome-%s-port-${port}.log" 2>&1 < /dev/null &
for _ in $(seq 1 60); do
  if curl -fsS --max-time 1 "http://127.0.0.1:${port}/json/version" >/dev/null 2>&1; then
    echo "started debugger on :${port}"
    curl -fsS --max-time 2 "http://127.0.0.1:${port}/json/version"
    exit 0
  fi
  sleep 0.2
done
echo "debug endpoint not ready on :${port}"
exit 2`,
		opts.Port, opts.ReuseExisting, opts.Role, opts.DebugAddress, opts.Role, headlessFlag, disableGPUFlag, opts.URL, opts.Role)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return fmt.Errorf("remote-new failed: %v output=%s", err, strings.TrimSpace(out))
	}
	fmt.Println(strings.TrimSpace(out))
	return nil
}

func killRemoteNodePID(node sshv1.MeshNode, pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	cmd := ""
	if strings.EqualFold(node.OS, "windows") {
		cmd = fmt.Sprintf("Stop-Process -Id %d -Force -ErrorAction Stop", pid)
	} else {
		cmd = fmt.Sprintf("kill -9 %d", pid)
	}
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return fmt.Errorf("%v output=%s", err, strings.TrimSpace(out))
	}
	return nil
}

func resolveTargetNodes(nodesArg string) ([]sshv1.MeshNode, error) {
	raw := strings.TrimSpace(nodesArg)
	if raw == "" || strings.EqualFold(raw, "all") {
		return sshv1.ListMeshNodes(), nil
	}
	parts := strings.Split(raw, ",")
	out := make([]sshv1.MeshNode, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		node, err := sshv1.ResolveMeshNode(p)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[node.Name]; ok {
			continue
		}
		seen[node.Name] = struct{}{}
		out = append(out, node)
	}
	return out, nil
}

func listRemoteNodeChrome(node sshv1.MeshNode) ([]remoteChromeProcess, error) {
	if node.OS == "windows" {
		ps := `Get-CimInstance Win32_Process | Where-Object { $_.Name -in @('chrome.exe','msedge.exe') } | Select-Object ProcessId,ParentProcessId,CommandLine | ConvertTo-Csv -NoTypeInformation`
		out, err := sshv1.RunNodeCommand(node.Name, ps, sshv1.CommandOptions{})
		if err != nil {
			return nil, err
		}
		reader := csv.NewReader(strings.NewReader(strings.ReplaceAll(out, "\r\n", "\n")))
		reader.FieldsPerRecord = -1
		_, _ = reader.Read()
		rows := make([]remoteChromeProcess, 0)
		for {
			rec, rerr := reader.Read()
			if rerr != nil {
				break
			}
			if len(rec) < 3 {
				continue
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(rec[0]))
			ppid, _ := strconv.Atoi(strings.TrimSpace(rec[1]))
			cmd := strings.TrimSpace(rec[2])
			if pid <= 0 || cmd == "" {
				continue
			}
			rows = append(rows, remoteChromeProcess{
				Node:      node.Name,
				Host:      node.Host,
				PID:       pid,
				PPID:      ppid,
				Command:   cmd,
				Headless:  strings.Contains(strings.ToLower(cmd), "--headless"),
				GPU:       !strings.Contains(strings.ToLower(cmd), "--disable-gpu"),
				DebugPort: debugPortFromCmd(cmd),
				Origin:    detectOriginFromCmd(cmd),
				Role:      detectRoleFromCmd(cmd),
			})
		}
		return rows, nil
	}

	cmd := `ps axww -o pid= -o ppid= -o command= | grep -Ei 'Google Chrome|google-chrome|chromium|msedge|microsoft edge' | grep -Ev 'grep|Crashpad|Helper \(Renderer\)|Helper \(Plugin\)|Helper \(GPU\)|--type=' || true`
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	rows := make([]remoteChromeProcess, 0, len(lines))
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		cmdline := strings.TrimSpace(strings.Join(fields[2:], " "))
		lc := strings.ToLower(cmdline)
		if strings.HasPrefix(lc, "zsh -c ") || strings.HasPrefix(lc, "bash -c ") || strings.HasPrefix(lc, "sh -c ") {
			continue
		}
		rows = append(rows, remoteChromeProcess{
			Node:      node.Name,
			Host:      node.Host,
			PID:       pid,
			PPID:      ppid,
			Command:   cmdline,
			Headless:  strings.Contains(lc, "--headless"),
			GPU:       !strings.Contains(lc, "--disable-gpu"),
			DebugPort: debugPortFromCmd(cmdline),
			Origin:    detectOriginFromCmd(cmdline),
			Role:      detectRoleFromCmd(cmdline),
		})
	}
	return rows, nil
}

func probeRemoteNodePort(node sshv1.MeshNode, port int) (remoteListen bool, remoteVersion bool) {
	if port <= 0 {
		return false, false
	}
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`$p=%d; if(Get-NetTCPConnection -State Listen -LocalPort $p -ErrorAction SilentlyContinue){ "LISTEN" } else { "NOLISTEN" }; try { $v=Invoke-RestMethod -Uri ("http://127.0.0.1:{0}/json/version" -f $p) -TimeoutSec 1; if($v.webSocketDebuggerUrl){ "VERSION" } else { "NOVERSION" } } catch { "NOVERSION" }`, port)
		out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		if err != nil {
			return false, false
		}
		text := strings.ToUpper(out)
		return strings.Contains(text, "LISTEN"), strings.Contains(text, "VERSION")
	}
	cmd := fmt.Sprintf(`if command -v lsof >/dev/null 2>&1; then lsof -nP -iTCP:%d -sTCP:LISTEN >/dev/null 2>&1 && echo LISTEN || echo NOLISTEN; else echo NOLISTEN; fi; if curl -fsS --max-time 1 http://127.0.0.1:%d/json/version >/dev/null 2>&1; then echo VERSION; else echo NOVERSION; fi`, port, port)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return false, false
	}
	text := strings.ToUpper(out)
	return strings.Contains(text, "LISTEN"), strings.Contains(text, "VERSION")
}

func parsePortsCSV(raw string) []int {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]int, 0, len(parts))
	seen := map[int]struct{}{}
	for _, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v <= 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func canDialHost(host string, port int, timeout time.Duration) bool {
	if strings.TrimSpace(host) == "" || port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func inspectRemoteListenerBind(node sshv1.MeshNode, port int) string {
	if port <= 0 {
		return "-"
	}
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`$p=%d; $c=Get-NetTCPConnection -State Listen -LocalPort $p -ErrorAction SilentlyContinue | Select-Object -First 1; if($c){ $c.LocalAddress }`, port)
		out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		if err != nil {
			return "-"
		}
		line := strings.TrimSpace(out)
		if line == "" {
			return "-"
		}
		return line
	}
	cmd := fmt.Sprintf(`if command -v lsof >/dev/null 2>&1; then lsof -nP -iTCP:%d -sTCP:LISTEN | awk 'NR>1{print $9}' | head -n 1; elif command -v netstat >/dev/null 2>&1; then netstat -an | grep LISTEN | grep '\.%d ' | head -n 1; fi`, port, port)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return "-"
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return "-"
	}
	return line
}

func buildRemoteRelayShell(listenPort, targetPort int, stop bool) string {
	return fmt.Sprintf(`set -eu
listen=%d
target=%d
base="$HOME/.dialtone"
mkdir -p "$base"
log="$base/chrome-relay-${listen}.log"
pidf="$base/chrome-relay-${listen}.pid"
if [ %t = true ]; then
  if [ -f "$pidf" ]; then
    pid="$(cat "$pidf" 2>/dev/null || true)"
    if [ -n "${pid}" ]; then kill "$pid" >/dev/null 2>&1 || true; fi
    rm -f "$pidf"
  fi
  pkill -f "chrome-relay.py ${listen} ${target}" >/dev/null 2>&1 || true
  pkill -f "socat .*TCP-LISTEN:${listen}" >/dev/null 2>&1 || true
  echo "stopped relay on :${listen}"
  exit 0
fi
if [ -f "$pidf" ]; then
  oldpid="$(cat "$pidf" 2>/dev/null || true)"
  if [ -n "${oldpid}" ] && kill -0 "${oldpid}" >/dev/null 2>&1; then
    echo "relay already running pid=${oldpid} listen=:${listen} -> 127.0.0.1:${target}"
    if command -v lsof >/dev/null 2>&1; then lsof -nP -iTCP:${listen} -sTCP:LISTEN || true; fi
    exit 0
  fi
fi
if command -v socat >/dev/null 2>&1; then
  nohup socat TCP-LISTEN:${listen},bind=0.0.0.0,reuseaddr,fork TCP:127.0.0.1:${target} >"$log" 2>&1 < /dev/null &
  pid=$!
else
  py="$base/chrome-relay.py"
  cat >"$py" <<'PY'
import socket, threading, sys
listen = int(sys.argv[1]); target = int(sys.argv[2])
def pump(src, dst):
    try:
        while True:
            data = src.recv(65536)
            if not data:
                break
            dst.sendall(data)
    except Exception:
        pass
    finally:
        try: dst.shutdown(socket.SHUT_WR)
        except Exception: pass
def handle(c):
    try:
        u = socket.create_connection(("127.0.0.1", target), timeout=2)
    except Exception:
        try: c.close()
        except Exception: pass
        return
    threading.Thread(target=pump, args=(c, u), daemon=True).start()
    threading.Thread(target=pump, args=(u, c), daemon=True).start()
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("0.0.0.0", listen))
s.listen(64)
while True:
    c, _ = s.accept()
    threading.Thread(target=handle, args=(c,), daemon=True).start()
PY
  nohup python3 "$py" "${listen}" "${target}" >"$log" 2>&1 < /dev/null &
  pid=$!
fi
echo "$pid" >"$pidf"
sleep 0.4
if command -v lsof >/dev/null 2>&1; then
  lsof -nP -iTCP:${listen} -sTCP:LISTEN || true
fi
echo "relay started pid=${pid} listen=:${listen} -> 127.0.0.1:${target}"`, listenPort, targetPort, stop)
}

func debugPortFromCmd(cmd string) int {
	re := regexp.MustCompile(`--remote-debugging-port=(\d+)`)
	m := re.FindStringSubmatch(cmd)
	if len(m) < 2 {
		return 0
	}
	v, _ := strconv.Atoi(m[1])
	return v
}

func detectRoleFromCmd(cmd string) string {
	re := regexp.MustCompile(`--dialtone-role=([a-zA-Z0-9_-]+)`)
	m := re.FindStringSubmatch(cmd)
	if len(m) < 2 {
		return "unknown"
	}
	return strings.TrimSpace(m[1])
}

func detectOriginFromCmd(cmd string) string {
	lc := strings.ToLower(cmd)
	if strings.Contains(lc, "--dialtone-origin=true") || strings.Contains(lc, "dialtone-chrome-") {
		return "Dialtone"
	}
	return "Other"
}
