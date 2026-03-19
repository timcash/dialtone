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

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func chromeServiceName(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	return "chrome-" + role
}

func managerNATSURL() string {
	return strings.TrimSpace(os.Getenv("DIALTONE_REPL_NATS_URL"))
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
		if advertise := localAdvertiseIP(); advertise != "" {
			parsed.Host = net.JoinHostPort(advertise, parsed.Port())
			return parsed.String()
		}
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
	if node.PreferWSLPowerShell {
		return false
	}
	return strings.TrimSpace(managerNATSURL()) != ""
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

func deployRemoteBinary(node sshv1.MeshNode, role string, startService bool) error {
	if role == "" {
		role = defaultRole
	}
	goos := mapNodeGOOS(node.OS)
	goarch := detectRemoteGOARCH(node)
	localBin := filepath.Join(resolveRepoRoot(), "bin", binaryName(goos, goarch))
	if err := buildBinaryFor(localBin, goos, goarch); err != nil {
		return err
	}
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
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
		cmd := fmt.Sprintf(`$bin=%s; New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($bin)) -Force | Out-Null; if(Test-Path $bin){ Remove-Item -Force $bin }; Move-Item -Force %s $bin`, psQuote(remoteBin), psQuote(remoteBin+".upload"))
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
	if role == "" {
		role = defaultRole
	}
	_ = stopRemoteService(node, role)
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
		return err
	}
	natsURL := managerNATSURLForNode(node)
	useManagerNATS := shouldUseLocalManagerNATS(node) && strings.TrimSpace(natsURL) != ""
	if node.PreferWSLPowerShell && strings.TrimSpace(readManagerLeaderState().TSNetNATSURL) == "" {
		useManagerNATS = false
		natsURL = ""
	}
	role = strings.TrimSpace(role)
	serviceName := chromeServiceName(role)
	logs.Info("chrome src_v3 remote service start host=%s role=%s prefer_wsl_powershell=%t use_manager_nats=%t manager_nats_url=%q",
		node.Name, role, node.PreferWSLPowerShell, useManagerNATS, natsURL)
	if strings.EqualFold(node.OS, "windows") {
		stdoutPath := windowsPath(filepath.Join(filepath.Dir(remoteBin), "dialtone_chrome_v3.out.log"))
		stderrPath := windowsPath(filepath.Join(filepath.Dir(remoteBin), "dialtone_chrome_v3.err.log"))
		cmdPath := windowsPath(filepath.Join(filepath.Dir(remoteBin), "dialtone_chrome_v3.cmd"))
		args := fmt.Sprintf("src_v3 daemon --role %s --chrome-port %d --host-id %s", role, defaultChromePort, node.Name)
		if useManagerNATS {
			args += " --nats-url " + natsURL
		} else {
			args += fmt.Sprintf(" --nats-port %d", defaultNATSPort)
		}
		script := fmt.Sprintf("@echo off\r\n\"%s\" %s 1>> \"%s\" 2>> \"%s\"\r\n", remoteBin, args, stdoutPath, stderrPath)
		scriptB64 := base64.StdEncoding.EncodeToString([]byte(script))
		cmd := fmt.Sprintf(`$cmdPath=%s; New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($cmdPath)) -Force | Out-Null; [IO.File]::WriteAllBytes($cmdPath, [Convert]::FromBase64String(%s)); Unblock-File -LiteralPath $cmdPath -ErrorAction SilentlyContinue; Start-Process -FilePath $cmdPath -WindowStyle Hidden`,
			psQuote(cmdPath), psQuote(scriptB64))
		logs.Info("chrome src_v3 windows launcher command: %s", cmd)
		if _, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{}); err != nil {
			return err
		}
	} else {
		args := fmt.Sprintf("src_v3 daemon --role %s --chrome-port %d --host-id %s", shellQuote(role), defaultChromePort, shellQuote(node.Name))
		if useManagerNATS {
			args += " --nats-url " + shellQuote(natsURL)
		} else {
			args += fmt.Sprintf(" --nats-port %d", defaultNATSPort)
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
		cmdPath := windowsPath(filepath.Join(filepath.Dir(remoteBin), "dialtone_chrome_v3.cmd"))
		cmd := fmt.Sprintf(`Get-CimInstance Win32_Process | Where-Object {
  ($_.Name -eq 'dialtone_chrome_v3.exe' -and $_.ExecutablePath -eq %s) -or
  ($_.Name -eq 'cmd.exe' -and $_.CommandLine -like %s)
} | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }`,
			psQuote(remoteBin),
			psQuote("*"+cmdPath+"*"))
		_, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		return err
	}
	cmd := fmt.Sprintf("pkill -f %s || true", shellQuote(remoteBin))
	_, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	return err
}

func waitForRemoteService(node sshv1.MeshNode, role string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := sendRemoteCommand(node, commandRequest{Command: "status", Role: role}); err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for remote chrome service on %s role=%s", node.Name, role)
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

func readRemoteLogs(node sshv1.MeshNode, lines int) (string, string, error) {
	if lines <= 0 {
		lines = 80
	}
	if !strings.EqualFold(node.OS, "windows") {
		return "", "", fmt.Errorf("logs currently implemented for windows hosts only")
	}
	outCmd := fmt.Sprintf("Get-Content -Tail %d $env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.out.log", lines)
	errCmd := fmt.Sprintf("Get-Content -Tail %d $env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.err.log", lines)
	stdout, outErr := sshv1.RunNodeCommand(node.Name, outCmd, sshv1.CommandOptions{})
	stderr, errErr := sshv1.RunNodeCommand(node.Name, errCmd, sshv1.CommandOptions{})
	if outErr != nil && errErr != nil {
		return "", "", outErr
	}
	return strings.TrimSpace(stdout), strings.TrimSpace(stderr), nil
}
