package src_v3

import (
	"encoding/base64"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func chromeServiceName(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	return "chrome-" + role
}

func remotePathDir(remotePath string, windows bool) string {
	if windows {
		normalized := strings.ReplaceAll(strings.TrimSpace(remotePath), "/", "\\")
		if idx := strings.LastIndex(normalized, `\`); idx > 0 {
			return normalized[:idx]
		}
		return "."
	}
	return filepath.Dir(remotePath)
}

func managerNATSURL() string {
	raw := strings.TrimSpace(os.Getenv("DIALTONE_REPL_NATS_URL"))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return raw
	}
	if host == "0.0.0.0" || host == "localhost" {
		parsed.Host = net.JoinHostPort("127.0.0.1", parsed.Port())
		return parsed.String()
	}
	return raw
}

type replLeaderStateDoc struct {
	NATSURL      string `json:"nats_url"`
	TSNetNATSURL string `json:"tsnet_nats_url,omitempty"`
}

func readManagerLeaderState() replLeaderStateDoc {
	path := filepath.Join(resolveRepoRoot(), ".dialtone", "repl-v3", "leader.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return replLeaderStateDoc{}
	}
	var doc replLeaderStateDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return replLeaderStateDoc{}
	}
	return doc
}

func managerNATSURLForNode(node sshv1.MeshNode) string {
	if raw := strings.TrimSpace(os.Getenv("DIALTONE_REPL_MANAGER_NATS_URL")); raw != "" {
		return raw
	}
	if st := readManagerLeaderState(); strings.TrimSpace(st.TSNetNATSURL) != "" {
		return strings.TrimSpace(st.TSNetNATSURL)
	}
	raw := managerNATSURL()
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return raw
	}
	if (host == "127.0.0.1" || host == "localhost" || host == "0.0.0.0") && node.PreferWSLPowerShell {
		parsed.Host = net.JoinHostPort("127.0.0.1", parsed.Port())
		return parsed.String()
	}
	return raw
}

func shouldUseLocalManagerNATS(node sshv1.MeshNode) bool {
	if raw := strings.TrimSpace(os.Getenv("DIALTONE_REPL_MANAGER_NATS_URL")); raw != "" {
		return true
	}
	st := readManagerLeaderState()
	if strings.TrimSpace(st.TSNetNATSURL) != "" {
		return true
	}
	return strings.TrimSpace(managerNATSURLForNode(node)) != ""
}

func localAdvertiseIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			ip = ip.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func remoteDialtoneCommand(node sshv1.MeshNode, args []string) string {
	if strings.EqualFold(node.OS, "windows") {
		return remoteDialtoneCommandWindows(node, args)
	}
	return remoteDialtoneCommandPOSIX(node, args)
}

func remoteDialtoneCommandPOSIX(node sshv1.MeshNode, args []string) string {
	run := make([]string, 0, len(args)+2)
	run = append(run, "./dialtone.sh", "--subtone-internal")
	run = append(run, args...)
	joined := shellJoinChrome(run)
	if len(node.RepoCandidates) > 0 && strings.TrimSpace(node.RepoCandidates[0]) != "" {
		repo := strings.TrimSpace(node.RepoCandidates[0])
		return fmt.Sprintf(
			"if [ -x %s/dialtone.sh ]; then cd %s && %s; elif [ -x ./dialtone.sh ]; then %s; elif [ -x \"$HOME/dialtone/dialtone.sh\" ]; then cd \"$HOME/dialtone\" && %s; else echo \"dialtone.sh not found in %s, $PWD, or $HOME/dialtone\" >&2; exit 127; fi",
			shellQuote(repo), shellQuote(repo), joined, joined, joined, shellQuote(repo),
		)
	}
	return fmt.Sprintf(
		"if [ -x ./dialtone.sh ]; then %s; elif [ -x \"$HOME/dialtone/dialtone.sh\" ]; then cd \"$HOME/dialtone\" && %s; else echo \"dialtone.sh not found in $PWD or $HOME/dialtone\" >&2; exit 127; fi",
		joined, joined,
	)
}

func shellJoinChrome(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func quotePSArgs(args []string) []string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, psQuote(arg))
	}
	return parts
}

func encodePowerShellCommand(script string) string {
	u16 := utf16.Encode([]rune(script))
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		buf[i*2] = byte(v)
		buf[i*2+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func remoteDialtoneCommandWindows(node sshv1.MeshNode, args []string) string {
	items := make([]string, 0, len(args)+2)
	items = append(items, "'--subtone-internal'")
	for _, arg := range append([]string{"repl"}, args...) {
		items = append(items, psQuote(arg))
	}
	repo := ""
	if len(node.RepoCandidates) > 0 {
		repo = strings.TrimSpace(node.RepoCandidates[0])
	}
	var b strings.Builder
	if repo != "" {
		b.WriteString(fmt.Sprintf("$repo=%s; ", psQuote(windowsPath(repo))))
		b.WriteString("$script=$null; ")
		b.WriteString("if($repo -and (Test-Path (Join-Path $repo 'dialtone.sh'))){ Set-Location $repo; $script = Join-Path $repo 'dialtone.sh' } ")
	} else {
		b.WriteString("$script=$null; ")
	}
	b.WriteString("if(-not $script -and (Test-Path './dialtone.sh')){ $script = (Resolve-Path './dialtone.sh').Path } ")
	b.WriteString("if(-not $script){ $homeRepo = Join-Path $HOME 'dialtone'; if(Test-Path (Join-Path $homeRepo 'dialtone.sh')){ Set-Location $homeRepo; $script = Join-Path $homeRepo 'dialtone.sh' } } ")
	b.WriteString("if(-not $script){ throw 'dialtone.sh not found in repo candidates, $PWD, or $HOME\\dialtone' } ")
	b.WriteString(fmt.Sprintf("$argv=@(%s); & $script @argv", strings.Join(items, ", ")))
	return b.String()
}

func startRemoteReplService(node sshv1.MeshNode, serviceName string, commandArgs []string) error {
	args := []string{
		"src_v3", "inject",
		"--user", "chrome-service",
		"service-start",
		"--name", strings.TrimSpace(serviceName),
		"--",
	}
	args = append(args, commandArgs...)
	_, err := sshv1.RunNodeCommand(node.Name, remoteDialtoneCommand(node, args), sshv1.CommandOptions{})
	return err
}

func stopRemoteReplService(node sshv1.MeshNode, serviceName string) error {
	args := []string{
		"src_v3", "inject",
		"--user", "chrome-service",
		"service-stop",
		"--name", strings.TrimSpace(serviceName),
	}
	_, err := sshv1.RunNodeCommand(node.Name, remoteDialtoneCommand(node, args), sshv1.CommandOptions{})
	return err
}

func buildBinaryFor(outPath, goos, goarch string) error {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	cmd := exec.Command(goBin, "build", "-o", outPath, "./plugins/chrome/scaffold/main.go")
	cmd.Dir = resolveSrcRoot()
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	logs.Info("chrome src_v3 build ok: %s", outPath)
	return nil
}

func preferredWindowsRepoPath(node sshv1.MeshNode) string {
	for _, candidate := range node.RepoCandidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.HasPrefix(candidate, "/mnt/") {
			parts := strings.Split(strings.TrimPrefix(candidate, "/mnt/"), "/")
			if len(parts) >= 2 && len(parts[0]) == 1 {
				drive := strings.ToUpper(parts[0])
				rest := strings.Join(parts[1:], `\`)
				return drive + `:\` + rest
			}
		}
		if len(candidate) >= 3 && candidate[1] == ':' && (candidate[2] == '\\' || candidate[2] == '/') {
			return windowsPath(candidate)
		}
	}
	home, err := sshv1.RunNodeCommand(node.Name, "$env:USERPROFILE", sshv1.CommandOptions{})
	if err != nil {
		return ""
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return ""
	}
	return windowsPath(home + `\dialtone`)
}

func buildRemoteBinaryOnWindows(node sshv1.MeshNode, remoteBin string) error {
	repo := preferredWindowsRepoPath(node)
	if repo == "" {
		return fmt.Errorf("chrome src_v3 native windows build requires repo candidate for %s", node.Name)
	}
	remoteBin = windowsPath(remoteBin)
	script := fmt.Sprintf(`$repo=%s; $src=Join-Path $repo 'src'; $bin=%s; $subject='CN=Dialtone Local Dev'; if(-not (Test-Path $src)){ throw "chrome src_v3 remote build requires repo at $src" }; New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($bin)) -Force | Out-Null; Set-Location $src; $env:GOOS='windows'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'; go build -o $bin ./plugins/chrome/scaffold/main.go; Unblock-File -LiteralPath $bin -ErrorAction SilentlyContinue; $cert=Get-ChildItem Cert:\CurrentUser\My | Where-Object { $_.Subject -eq $subject } | Select-Object -First 1; if($cert){ Set-AuthenticodeSignature -FilePath $bin -Certificate $cert -HashAlgorithm SHA256 | Out-Null }; Get-AuthenticodeSignature -FilePath $bin | Select-Object Status, @{Name='Subject';Expression={ if($_.SignerCertificate){ $_.SignerCertificate.Subject } else { '' } }} | Format-List`,
		psQuote(repo), psQuote(remoteBin))
	cmd := "powershell.exe -NoProfile -NonInteractive -EncodedCommand " + encodePowerShellCommand(script)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		trimmed := strings.TrimSpace(out)
		if trimmed != "" {
			logs.Info("chrome src_v3 native windows build error output on %s:\n%s", node.Name, trimmed)
			return fmt.Errorf("chrome src_v3 native windows build failed on %s: %w (%s)", node.Name, err, trimmed)
		}
		return fmt.Errorf("chrome src_v3 native windows build failed on %s: %w", node.Name, err)
	}
	logs.Info("chrome src_v3 native windows build output on %s:\n%s", node.Name, strings.TrimSpace(out))
	return nil
}

func deployRemoteBinary(node sshv1.MeshNode, role string, startService bool) error {
	if role == "" {
		role = defaultRole
	}
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
		return err
	}
	if strings.EqualFold(node.OS, "windows") && node.PreferWSLPowerShell && len(node.RepoCandidates) > 0 && strings.TrimSpace(node.RepoCandidates[0]) != "" {
		_ = stopRemoteService(node, strings.TrimSpace(role))
		if err := buildRemoteBinaryOnWindows(node, remoteBin); err != nil {
			return err
		}
		logs.Info("chrome src_v3 native windows deploy complete on %s:%s", node.Name, remoteBin)
		if startService {
			return startRemoteService(node, strings.TrimSpace(role))
		}
		return nil
	}
	goos := mapNodeGOOS(node.OS)
	goarch := detectRemoteGOARCH(node)
	localBin := filepath.Join(resolveRepoRoot(), "bin", binaryName(goos, goarch))
	if err := buildBinaryFor(localBin, goos, goarch); err != nil {
		return err
	}
	localHash, err := localFileSHA256(localBin)
	if err != nil {
		return err
	}
	remoteHash, err := remoteFileSHA256(node, remoteBin)
	if err != nil {
		return err
	}
	if localHash != "" && remoteHash != "" && strings.EqualFold(localHash, remoteHash) {
		logs.Info("chrome src_v3 deploy skipped; remote binary already current on %s", node.Name)
		if !startService {
			return nil
		}
		if _, err := sendRemoteCommand(node, commandRequest{Command: "status", Role: strings.TrimSpace(role)}); err == nil {
			return nil
		}
		return startRemoteService(node, strings.TrimSpace(role))
	}
	_ = stopRemoteService(node, strings.TrimSpace(role))
	if err := sshv1.UploadNodeFile(node.Name, localBin, remoteBin+".upload", sshv1.CommandOptions{}); err != nil {
		return err
	}
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`$bin=%s; New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($bin)) -Force | Out-Null; if(Test-Path $bin){ Remove-Item -Force $bin }; Move-Item -Force %s $bin; Unblock-File -LiteralPath $bin -ErrorAction SilentlyContinue`, psQuote(remoteBin), psQuote(remoteBin+".upload"))
		if _, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		}
	} else {
		cmd := fmt.Sprintf("mkdir -p %s && chmod +x %s && mv %s %s", shellQuote(filepath.Dir(remoteBin)), shellQuote(remoteBin+".upload"), shellQuote(remoteBin+".upload"), shellQuote(remoteBin))
		if _, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		}
	}
	logs.Info("chrome src_v3 deployed to %s:%s", node.Name, remoteBin)
	if startService {
		return startRemoteService(node, strings.TrimSpace(role))
	}
	return nil
}

func startRemoteService(node sshv1.MeshNode, role string) error {
	role = normalizeRole(role)
	_ = stopRemoteService(node, role)
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
		return err
	}
	natsURL := managerNATSURLForNode(node)
	useManagerNATS := shouldUseLocalManagerNATS(node) && strings.TrimSpace(natsURL) != ""
	role = strings.TrimSpace(role)
	serviceName := chromeServiceName(role)
	logs.Info("chrome src_v3 remote service start host=%s role=%s prefer_wsl_powershell=%t use_manager_nats=%t manager_nats_url=%q",
		node.Name, role, node.PreferWSLPowerShell, useManagerNATS, natsURL)
	if strings.EqualFold(node.OS, "windows") {
		remoteDir := remotePathDir(remoteBin, true)
		stdoutPath := windowsPath(remoteDir + `\` + serviceName + `.out.log`)
		stderrPath := windowsPath(remoteDir + `\` + serviceName + `.err.log`)
		workDir := windowsPath(remoteDir)
		args := []string{"src_v3", "daemon", "--role", role, "--chrome-port", fmt.Sprintf("%d", roleChromePort(role)), "--host-id", node.Name}
		if useManagerNATS {
			args = append(args, "--nats-url", natsURL)
		} else {
			args = append(args, "--nats-port", fmt.Sprintf("%d", roleNATSPort(role)))
		}
		cmdParts := make([]string, 0, len(args)+4)
		cmdParts = append(cmdParts, `"`+strings.ReplaceAll(remoteBin, `"`, `\"`)+`"`)
		for _, arg := range args {
			cmdParts = append(cmdParts, `"`+strings.ReplaceAll(arg, `"`, `\"`)+`"`)
		}
		cmdParts = append(cmdParts, `1>>"`+stdoutPath+`"`, `2>>"`+stderrPath+`"`)
		cmd := fmt.Sprintf(`$bin=%s; $workDir=%s; $stdout=%s; $stderr=%s; New-Item -ItemType Directory -Path $workDir -Force | Out-Null; Unblock-File -LiteralPath $bin -ErrorAction SilentlyContinue; $launch='start "" /b %s'; Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c', $launch) -WorkingDirectory $workDir -WindowStyle Hidden | Out-Null; Write-Output 'STARTED'`,
			psQuote(remoteBin), psQuote(workDir), psQuote(stdoutPath), psQuote(stderrPath), strings.Join(cmdParts, " "))
		logs.Info("chrome src_v3 windows launcher command: %s", cmd)
		if out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		} else {
			logs.Info("chrome src_v3 windows launcher output on %s:\n%s", node.Name, strings.TrimSpace(out))
		}
	} else {
		args := fmt.Sprintf("src_v3 daemon --role %s --chrome-port %d --host-id %s", shellQuote(role), roleChromePort(role), shellQuote(node.Name))
		if useManagerNATS {
			args += " --nats-url " + shellQuote(natsURL)
		} else {
			args += fmt.Sprintf(" --nats-port %d", roleNATSPort(role))
		}
		cmd := fmt.Sprintf("mkdir -p %s && nohup %s %s >> %s 2>> %s < /dev/null &",
			shellQuote(filepath.Dir(remoteBin)),
			shellQuote(remoteBin),
			args,
			shellQuote(filepath.Join(filepath.Dir(remoteBin), serviceName+".out.log")),
			shellQuote(filepath.Join(filepath.Dir(remoteBin), serviceName+".err.log")),
		)
		if _, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		}
	}
	return waitForRemoteService(node, role, 20*time.Second)
}

func stopRemoteService(node sshv1.MeshNode, role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	if _, err := sendRemoteCommand(node, commandRequest{Command: "shutdown", Role: role}); err == nil {
		return nil
	}
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
		return err
	}
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`$role=%s; Get-CimInstance Win32_Process | Where-Object {
  $_.Name -eq 'dialtone_chrome_v3.exe' -and $_.ExecutablePath -eq %s -and (
    $_.CommandLine -like ('*--role ' + $role + '*') -or
    $_.CommandLine -like ('*"--role" "' + $role + '"*')
  )
} | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`,
			psQuote(role), psQuote(remoteBin))
		_, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		return err
	}
	cmd := fmt.Sprintf("ps -eo pid,args | grep '[d]ialtone_chrome_v3' | grep -- %s | grep -- '--role %s' | awk '{print $1}' | xargs -r kill -9", shellQuote(remoteBin), shellQuote(role))
	_, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	return err
}

func waitForRemoteService(node sshv1.MeshNode, role string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	backoff := 200 * time.Millisecond
	serviceName := chromeServiceName(role)
	useManagerNATS := shouldUseLocalManagerNATS(node) && strings.TrimSpace(managerNATSURLForNode(node)) != ""
	statusReq := commandRequest{Command: "status", Role: role, TimeoutMS: 1200}
	var lastErr error
	for time.Now().Before(deadline) {
		if useManagerNATS {
			item, ok, err := lookupRemoteServiceState(node, serviceName)
			if err == nil && ok && item.Active {
				if _, err := sendRemoteCommand(node, statusReq); err == nil {
					return nil
				} else {
					lastErr = err
				}
			} else if err != nil {
				lastErr = err
			}
			if _, err := sendRemoteCommand(node, statusReq); err == nil {
				return nil
			} else {
				lastErr = err
			}
		} else {
			if _, err := sendRemoteCommand(node, statusReq); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
			if backoff > time.Second {
				backoff = time.Second
			}
		}
	}
	if lastErr != nil {
		return fmt.Errorf("timed out waiting for remote chrome service on %s role=%s: %w", node.Name, role, lastErr)
	}
	return fmt.Errorf("timed out waiting for remote chrome service on %s role=%s", node.Name, role)
}

type remoteServiceRegistryRequest struct {
	Count int `json:"count,omitempty"`
}

type remoteServiceRegistryItem struct {
	Name          string `json:"name,omitempty"`
	Host          string `json:"host,omitempty"`
	PID           int    `json:"pid,omitempty"`
	Room          string `json:"room,omitempty"`
	Command       string `json:"command,omitempty"`
	Mode          string `json:"mode,omitempty"`
	LogPath       string `json:"log_path,omitempty"`
	LastHeartbeat string `json:"last_heartbeat,omitempty"`
	Active        bool   `json:"active,omitempty"`
}

func lookupRemoteServiceState(node sshv1.MeshNode, serviceName string) (remoteServiceRegistryItem, bool, error) {
	managerURL := strings.TrimSpace(managerNATSURLForNode(node))
	if managerURL == "" {
		return remoteServiceRegistryItem{}, false, nil
	}
	nc, err := nats.Connect(managerURL, nats.Timeout(2*time.Second))
	if err != nil {
		return remoteServiceRegistryItem{}, false, err
	}
	defer nc.Close()
	raw, err := json.Marshal(remoteServiceRegistryRequest{Count: 64})
	if err != nil {
		return remoteServiceRegistryItem{}, false, err
	}
	msg, err := nc.Request("repl.registry.services", raw, 2*time.Second)
	if err != nil {
		return remoteServiceRegistryItem{}, false, err
	}
	var items []remoteServiceRegistryItem
	if err := json.Unmarshal(msg.Data, &items); err != nil {
		return remoteServiceRegistryItem{}, false, err
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Name), strings.TrimSpace(serviceName)) &&
			strings.EqualFold(strings.TrimSpace(item.Host), strings.TrimSpace(node.Name)) {
			return item, true, nil
		}
	}
	return remoteServiceRegistryItem{}, false, nil
}

func runRemoteDoctor(node sshv1.MeshNode) error {
	if resp, err := sendRemoteCommand(node, commandRequest{Command: "status", Role: defaultRole}); err == nil {
		printResponse(resp)
	} else {
		fmt.Printf("NATS status error: %v\n", err)
	}
	processCmd := `Get-Process dialtone_chrome_v3,chrome -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,StartTime,Path | Sort-Object StartTime | Format-Table -AutoSize`
	portCmd := `cmd /c "netstat -ano | findstr :19464 & netstat -ano | findstr :19465"`
	taskCmd := `cmd /c "schtasks /Query /FO TABLE | findstr /I Dialtone"`
	mitigationCmd := `Get-ProcessMitigation -Name chrome.exe,dialtone_chrome_v3.exe -ErrorAction SilentlyContinue | Format-List`
	defenderCmd := `try { $p = Get-MpPreference -ErrorAction Stop; [pscustomobject]@{ AttackSurfaceReductionRules_Actions = ($p.AttackSurfaceReductionRules_Actions -join ','); AttackSurfaceReductionRules_Ids = ($p.AttackSurfaceReductionRules_Ids -join ','); EnableControlledFolderAccess = $p.EnableControlledFolderAccess } | Format-List } catch { Write-Output $_.Exception.Message }`
	if strings.EqualFold(node.OS, "windows") {
		if out, err := sshv1.RunNodeCommand(node.Name, processCmd, sshv1.CommandOptions{}); err == nil {
			fmt.Println("PROCESS LIST")
			fmt.Println(strings.TrimSpace(out))
		}
		if out, err := sshv1.RunNodeCommand(node.Name, portCmd, sshv1.CommandOptions{}); err == nil {
			fmt.Println("PORT LISTENERS")
			fmt.Println(strings.TrimSpace(out))
		}
		if out, err := sshv1.RunNodeCommand(node.Name, taskCmd, sshv1.CommandOptions{}); err == nil {
			fmt.Println("SCHEDULED TASKS")
			fmt.Println(strings.TrimSpace(out))
		}
		if out, err := sshv1.RunNodeCommand(node.Name, mitigationCmd, sshv1.CommandOptions{}); err == nil {
			fmt.Println("PROCESS MITIGATIONS")
			fmt.Println(strings.TrimSpace(out))
		}
		if out, err := sshv1.RunNodeCommand(node.Name, defenderCmd, sshv1.CommandOptions{}); err == nil {
			fmt.Println("DEFENDER PREFERENCES")
			fmt.Println(strings.TrimSpace(out))
		}
	}
	return nil
}

func resetRemoteHost(node sshv1.MeshNode, role string) error {
	if role == "" {
		role = defaultRole
	}
	if _, err := sendRemoteCommand(node, commandRequest{Command: "reset", Role: role}); err == nil {
		logs.Info("chrome src_v3 reset ok host=%s role=%s profile_preserved=true", node.Name, role)
		return nil
	}
	if err := startRemoteService(node, role); err != nil {
		return err
	}
	if _, err := sendRemoteCommand(node, commandRequest{Command: "reset", Role: role}); err != nil {
		return err
	}
	logs.Info("chrome src_v3 reset ok host=%s role=%s profile_preserved=true", node.Name, role)
	return nil
}

func localFileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func remoteFileSHA256(node sshv1.MeshNode, path string) (string, error) {
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`$path=%s; if(!(Test-Path $path)){ exit 0 }; (Get-FileHash -Algorithm SHA256 -Path $path).Hash`, psQuote(path))
		out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	}
	cmd := fmt.Sprintf("if [ -f %s ]; then sha256sum %s | awk '{print $1}'; fi", shellQuote(path), shellQuote(path))
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func readRemoteLogs(node sshv1.MeshNode, role string, lines int) (string, string, error) {
	if lines <= 0 {
		lines = 80
	}
	role = normalizeRole(role)
	serviceName := chromeServiceName(role)
	if !strings.EqualFold(node.OS, "windows") {
		return "", "", fmt.Errorf("logs currently implemented for windows hosts only")
	}
	outCmd := fmt.Sprintf("Get-Content -Tail %d $env:USERPROFILE\\.dialtone\\bin\\%s.out.log", lines, serviceName)
	errCmd := fmt.Sprintf("Get-Content -Tail %d $env:USERPROFILE\\.dialtone\\bin\\%s.err.log", lines, serviceName)
	stdout, outErr := sshv1.RunNodeCommand(node.Name, outCmd, sshv1.CommandOptions{})
	stderr, errErr := sshv1.RunNodeCommand(node.Name, errCmd, sshv1.CommandOptions{})
	if outErr != nil && errErr != nil {
		return "", "", outErr
	}
	return strings.TrimSpace(stdout), strings.TrimSpace(stderr), nil
}
