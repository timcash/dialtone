package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func handleDeployCmd(args []string) {
	fs := flag.NewFlagSet("chrome deploy", flag.ExitOnError)
	host := fs.String("host", "", "Mesh node target (example: darkmac)")
	user := fs.String("user", "", "Override remote user")
	remotePath := fs.String("remote-path", "", "Remote binary path (default: ~/.dialtone/bin/dialtone_chrome_v1)")
	service := fs.Bool("service", false, "Start persistent chrome service on remote host")
	role := fs.String("role", "dev", "Role used by service-start")
	_ = fs.Parse(args)

	target := strings.TrimSpace(*host)
	if target == "" {
		logs.Fatal("deploy requires --host")
	}
	node, err := sshv1.ResolveMeshNode(target)
	if err != nil {
		logs.Fatal("deploy resolve host failed: %v", err)
	}
	client, _, _, _, err := sshv1.DialMeshNode(target, sshv1.CommandOptions{User: strings.TrimSpace(*user)})
	if err != nil {
		logs.Fatal("deploy ssh dial failed: %v", err)
	}
	defer client.Close()

	paths, err := chrome.ResolvePaths("")
	if err != nil {
		logs.Fatal("deploy resolve paths failed: %v", err)
	}
	targetGOOS := mapNodeGOOS(node.OS)
	targetGOARCH := detectRemoteGOARCH(target, node.OS)
	localBin := filepath.Join(paths.Runtime.RepoRoot, "bin", fmt.Sprintf("dialtone_chrome_v1-%s-%s", targetGOOS, targetGOARCH))
	if err := buildChromeBinary(paths.Runtime.SrcRoot, localBin, targetGOOS, targetGOARCH); err != nil {
		logs.Fatal("deploy build failed: %v", err)
	}

	home, err := sshv1.GetRemoteHome(client)
	if err != nil {
		logs.Fatal("deploy remote home failed: %v", err)
	}
	remoteBin := strings.TrimSpace(*remotePath)
	if remoteBin == "" {
		remoteBin = filepath.ToSlash(filepath.Join(home, ".dialtone", "bin", "dialtone_chrome_v1"))
	}
	remoteTmp := remoteBin + ".upload"

	mkdirCmd := fmt.Sprintf("mkdir -p %s", shellQuote(filepath.ToSlash(filepath.Dir(remoteBin))))
	if _, err := sshv1.RunSSHCommand(client, mkdirCmd); err != nil {
		logs.Fatal("deploy remote mkdir failed: %v", err)
	}
	if err := sshv1.UploadFile(client, localBin, remoteTmp); err != nil {
		logs.Fatal("deploy upload failed: %v", err)
	}
	installCmd := fmt.Sprintf("chmod +x %s && mv %s %s", shellQuote(remoteTmp), shellQuote(remoteTmp), shellQuote(remoteBin))
	if _, err := sshv1.RunSSHCommand(client, installCmd); err != nil {
		logs.Fatal("deploy install failed: %v", err)
	}
	logs.Info("deployed chrome binary to %s:%s", node.Name, remoteBin)

	if *service {
		if err := startRemoteChromeService(target, node.OS, remoteBin, strings.TrimSpace(*role)); err != nil {
			logs.Fatal("deploy service start failed: %v", err)
		}
		logs.Info("remote chrome service started on %s (role=%s)", node.Name, strings.TrimSpace(*role))
	}
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
		ps := fmt.Sprintf(`$name='dialtone_chrome_service_%s'
$bin=%s
Get-Process -Name $name -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
try{
  if(-not (Get-NetFirewallRule -DisplayName 'Dialtone Chrome Service 19444' -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName 'Dialtone Chrome Service 19444' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 19444 -Profile Any | Out-Null
  }
}catch{}
Start-Process -FilePath $bin -ArgumentList @('src_v1','service-daemon','--role',%s,'--debug-address','0.0.0.0','--listen-address','0.0.0.0','--listen-port','19444') -WindowStyle Hidden`, role, psQuote(remoteBin), psQuote(role))
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
