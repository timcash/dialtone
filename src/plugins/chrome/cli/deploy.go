package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func handleDeployCmd(args []string) {
	fs := flag.NewFlagSet("chrome deploy", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host target (example: darkmac or all)")
	user := fs.String("user", "", "Override remote user")
	remotePath := fs.String("remote-path", "", "Remote binary path (default: ~/.dialtone/bin/dialtone_chrome_v1)")
	service := fs.Bool("service", false, "Start persistent chrome service on remote host")
	role := fs.String("role", "dev", "Role used by service-start")
	_ = fs.Parse(args)

	target := strings.TrimSpace(*host)
	if target == "" {
		logs.Fatal("deploy requires --host")
	}
	paths, err := chrome.ResolvePaths("")
	if err != nil {
		logs.Fatal("deploy resolve paths failed: %v", err)
	}

	targets := []string{target}
	if strings.EqualFold(target, "all") {
		targets = make([]string, 0)
		for _, n := range sshv1.ListMeshNodes() {
			targets = append(targets, n.Name)
		}
		sort.Strings(targets)
	}

	built := map[string]string{}
	failures := 0
	for _, t := range targets {
		node, nerr := sshv1.ResolveMeshNode(t)
		if nerr != nil {
			failures++
			logs.Warn("deploy resolve host failed target=%s: %v", t, nerr)
			continue
		}
		targetGOOS := mapNodeGOOS(node.OS)
		targetGOARCH := detectRemoteGOARCH(node.Name, node.OS)
		buildKey := targetGOOS + "/" + targetGOARCH
		localBin := built[buildKey]
		if strings.TrimSpace(localBin) == "" {
			localBin = filepath.Join(paths.Runtime.RepoRoot, "bin", fmt.Sprintf("dialtone_chrome_v1-%s-%s", targetGOOS, targetGOARCH))
			if berr := buildChromeBinary(paths.Runtime.SrcRoot, localBin, targetGOOS, targetGOARCH); berr != nil {
				failures++
				logs.Warn("deploy build failed host=%s target=%s: %v", node.Name, buildKey, berr)
				continue
			}
			built[buildKey] = localBin
		}
		if derr := deployToHost(node, localBin, strings.TrimSpace(*user), strings.TrimSpace(*remotePath), *service, strings.TrimSpace(*role)); derr != nil {
			failures++
			logs.Warn("deploy failed host=%s: %v", node.Name, derr)
			continue
		}
		logs.Info("deploy ok host=%s target=%s", node.Name, buildKey)
	}
	if failures > 0 {
		logs.Fatal("deploy finished with %d failure(s)", failures)
	}
}

func deployToHost(node sshv1.MeshNode, localBin, user, remotePath string, service bool, role string) error {
	client, _, _, _, err := sshv1.DialMeshNode(node.Name, sshv1.CommandOptions{User: user})
	if err != nil {
		return fmt.Errorf("ssh dial failed: %w", err)
	}
	defer client.Close()

	home, err := sshv1.GetRemoteHome(client)
	if err != nil {
		return fmt.Errorf("remote home failed: %w", err)
	}
	remoteBin := strings.TrimSpace(remotePath)
	if remoteBin == "" {
		remoteBin = filepath.ToSlash(filepath.Join(home, ".dialtone", "bin", "dialtone_chrome_v1"))
		if strings.EqualFold(node.OS, "windows") {
			remoteBin = windowsPath(remoteBin)
			if !strings.HasSuffix(strings.ToLower(remoteBin), ".exe") {
				remoteBin += ".exe"
			}
		}
	} else if strings.EqualFold(node.OS, "windows") {
		remoteBin = windowsPath(remoteBin)
	}
	remoteTmp := remoteBin + ".upload"
	remoteDir := filepath.ToSlash(filepath.Dir(remoteBin))
	if strings.EqualFold(node.OS, "windows") {
		remoteDir = windowsPath(remoteDir)
	}

	if strings.EqualFold(node.OS, "windows") {
		mkdirCmd := fmt.Sprintf(`$dir=%s; New-Item -ItemType Directory -Path $dir -Force | Out-Null`, psQuote(remoteDir))
		if _, err := sshv1.RunNodeCommand(node.Name, mkdirCmd, sshv1.CommandOptions{User: user}); err != nil {
			return fmt.Errorf("remote mkdir failed: %w", err)
		}
	} else {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", shellQuote(remoteDir))
		if _, err := sshv1.RunSSHCommand(client, mkdirCmd); err != nil {
			return fmt.Errorf("remote mkdir failed: %w", err)
		}
	}
	if err := sshv1.UploadFile(client, localBin, remoteTmp); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	if strings.EqualFold(node.OS, "windows") {
		installCmd := fmt.Sprintf(`$bin=%s; $name=[System.IO.Path]::GetFileNameWithoutExtension($bin); Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue; if(Test-Path %s){ Remove-Item -Force %s }; Move-Item -Path %s -Destination %s -Force`, psQuote(remoteBin), psQuote(remoteBin), psQuote(remoteBin), psQuote(remoteTmp), psQuote(remoteBin))
		if _, err := sshv1.RunNodeCommand(node.Name, installCmd, sshv1.CommandOptions{User: user}); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
	} else {
		installCmd := fmt.Sprintf("chmod +x %s && mv %s %s", shellQuote(remoteTmp), shellQuote(remoteTmp), shellQuote(remoteBin))
		if _, err := sshv1.RunSSHCommand(client, installCmd); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
	}
	logs.Info("deployed chrome binary to %s:%s", node.Name, remoteBin)
	if service {
		if err := startRemoteChromeService(node.Name, node.OS, remoteBin, role); err != nil {
			return fmt.Errorf("service start failed: %w", err)
		}
		logs.Info("remote chrome service started on %s (role=%s)", node.Name, role)
	}
	return nil
}

func buildChromeBinary(srcRoot, outPath, goos, goarch string) error {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "build", "-o", outPath, "./plugins/chrome/scaffold/main.go")
	cmd.Dir = srcRoot
	cmd.Env = append(os.Environ(),
		"GOOS="+shellEscapeEnv(goos),
		"GOARCH="+shellEscapeEnv(goarch),
		"CGO_ENABLED=0",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func shellEscapeEnv(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "\"", "")
	return s
}

func mapNodeGOOS(nodeOS string) string {
	switch strings.ToLower(strings.TrimSpace(nodeOS)) {
	case "macos", "darwin":
		return "darwin"
	case "windows":
		return "windows"
	default:
		return "linux"
	}
}

func detectRemoteGOARCH(target, nodeOS string) string {
	switch strings.ToLower(strings.TrimSpace(nodeOS)) {
	case "windows":
		out, err := sshv1.RunNodeCommand(target, "$env:PROCESSOR_ARCHITECTURE", sshv1.CommandOptions{})
		if err == nil {
			arch := strings.ToLower(strings.TrimSpace(out))
			if strings.Contains(arch, "arm64") {
				return "arm64"
			}
		}
	case "macos", "darwin", "linux":
		out, err := sshv1.RunNodeCommand(target, "uname -m", sshv1.CommandOptions{})
		if err == nil {
			arch := strings.ToLower(strings.TrimSpace(out))
			if strings.Contains(arch, "aarch64") || strings.Contains(arch, "arm64") {
				return "arm64"
			}
		}
	}
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "amd64"
}

func startRemoteChromeService(target, nodeOS, remoteBin, role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		role = "dev"
	}
	switch strings.ToLower(strings.TrimSpace(nodeOS)) {
	case "windows":
		ps := fmt.Sprintf(`$task='DialtoneChromeService-%s'
$bin=%s
$dir=[System.IO.Path]::GetDirectoryName($bin)
$cmd=Join-Path $dir 'dialtone_chrome_service.cmd'
Get-Process -Name 'dialtone_chrome_v1' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
schtasks /Delete /TN $task /F *> $null
$line='"' + $bin + '" src_v1 service-daemon --role %s --debug-address 127.0.0.1 --listen-address 127.0.0.1 --listen-port 19444'
Set-Content -Path $cmd -Encoding ASCII -Value $line
try{
  if(-not (Get-NetFirewallRule -DisplayName 'Dialtone Chrome Service 19444' -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName 'Dialtone Chrome Service 19444' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 19444 -Profile Any | Out-Null
  }
}catch{}
try{
  schtasks /Create /TN $task /SC ONCE /ST 00:00 /TR $cmd /F /RL HIGHEST /IT | Out-Null
  schtasks /Run /TN $task | Out-Null
}catch{
  Start-Process -FilePath $bin -ArgumentList @('src_v1','service-daemon','--role',%s,'--debug-address','127.0.0.1','--listen-address','127.0.0.1','--listen-port','19444') -WindowStyle Hidden
}`, role, psQuote(remoteBin), role, psQuote(role))
		_, err := sshv1.RunNodeCommand(target, ps, sshv1.CommandOptions{})
		return err
	default:
		cmd := fmt.Sprintf(`pkill -f %s >/dev/null 2>&1 || true
nohup %s src_v1 service-daemon --role %s --debug-address 0.0.0.0 --listen-address 0.0.0.0 --listen-port 19444 >"$HOME/.dialtone/chrome-service-%s.log" 2>&1 < /dev/null &`, shellQuote(remoteBin+" src_v1 service-daemon --role "+role), shellQuote(remoteBin), shellQuote(role), role)
		_, err := sshv1.RunNodeCommand(target, cmd, sshv1.CommandOptions{})
		return err
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func windowsPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, "/", "\\")
	return p
}
