package src_v3

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func handleDoctor(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 doctor", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("doctor requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
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

func handleLogs(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 logs", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	lines := fs.Int("lines", 80, "Lines to tail")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("logs requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	if !strings.EqualFold(node.OS, "windows") {
		return fmt.Errorf("logs currently implemented for windows hosts only")
	}
	outCmd := fmt.Sprintf("Get-Content -Tail %d $env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.out.log", *lines)
	errCmd := fmt.Sprintf("Get-Content -Tail %d $env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.err.log", *lines)
	if out, err := sshv1.RunNodeCommand(node.Name, outCmd, sshv1.CommandOptions{}); err == nil {
		fmt.Println("STDOUT LOG")
		fmt.Println(strings.TrimSpace(out))
	}
	if out, err := sshv1.RunNodeCommand(node.Name, errCmd, sshv1.CommandOptions{}); err == nil {
		fmt.Println("STDERR LOG")
		fmt.Println(strings.TrimSpace(out))
	}
	return nil
}

func handleReset(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 reset", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("reset requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	_ = stopRemoteService(node)
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf(`Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and ($_.CommandLine -like '*dialtone*' -or $_.CommandLine -like '*--remote-debugging-port=%d*') } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`, defaultChromePort)
		_, _ = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		profile := defaultProfileDir(defaultRole)
		wipeCmd := fmt.Sprintf(`if(Test-Path %s){ Remove-Item -Path %s -Recurse -Force -ErrorAction SilentlyContinue }`, psQuote(strings.ReplaceAll(profile, `/`, `\`)), psQuote(strings.ReplaceAll(profile, `/`, `\`)))
		_, _ = sshv1.RunNodeCommand(node.Name, wipeCmd, sshv1.CommandOptions{})
	}
	logs.Info("chrome src_v3 reset ok host=%s", node.Name)
	return nil
}

func defaultProfileDir(role string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone", "chrome-v3", role)
}

func resolveRepoRoot() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); v != "" {
		return v
	}
	cwd, _ := os.Getwd()
	return cwd
}

func resolveSrcRoot() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_SRC_ROOT")); v != "" {
		return v
	}
	return filepath.Join(resolveRepoRoot(), "src")
}

func binaryName(goos, goarch string) string {
	name := fmt.Sprintf("dialtone_chrome_v3-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func remoteBinaryPath(node sshv1.MeshNode) (string, error) {
	homeCmd := "$HOME"
	if strings.EqualFold(node.OS, "windows") {
		homeCmd = "$env:USERPROFILE"
	}
	home, err := sshv1.RunNodeCommand(node.Name, homeCmd, sshv1.CommandOptions{})
	if err != nil {
		return "", err
	}
	base := strings.TrimSpace(home)
	if strings.EqualFold(node.OS, "windows") {
		return windowsPath(filepath.Join(base, ".dialtone", "bin", "dialtone_chrome_v3.exe")), nil
	}
	return filepath.Join(base, ".dialtone", "bin", "dialtone_chrome_v3"), nil
}

func startRemoteService(node sshv1.MeshNode, role string) error {
	if role == "" {
		role = defaultRole
	}
	remoteBin, err := remoteBinaryPath(node)
	if err != nil {
		return err
	}
	if strings.EqualFold(node.OS, "windows") {
		cmd := fmt.Sprintf("$out=\"$env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.out.log\"\n"+
			"$err=\"$env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.err.log\"\n"+
			"$runner=\"$env:USERPROFILE\\.dialtone\\bin\\dialtone_chrome_v3.cmd\"\n"+
			"try { Get-Process -Name 'dialtone_chrome_v3','dialtone_chrome_v1' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue } catch {}\n"+
			"try { schtasks /Delete /TN DialtoneChromeService-dev /F *> $null } catch {}\n"+
			"try { Get-CimInstance Win32_Process | Where-Object { $_.Name -eq 'chrome.exe' -and ($_.CommandLine -like '*dialtone-role=%s*' -or $_.CommandLine -like '*chrome-v3\\\\%s*' -or $_.CommandLine -like '*--remote-debugging-port=%d*') } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue } } catch {}\n"+
			"if(Test-Path $out){ Remove-Item -Force $out }\n"+
			"if(Test-Path $err){ Remove-Item -Force $err }\n"+
			"Set-Content -Path $runner -Encoding ASCII -Value ('@echo off' + \"`r`n\" + '\"' + %s + '\" src_v3 daemon --role ' + %s + ' --chrome-port %d --nats-port %d 1>>\"' + $out + '\" 2>>\"' + $err + '\"')\n"+
			"Start-Process -FilePath 'cmd.exe' -ArgumentList @('/c',$runner) -WindowStyle Hidden\n"+
			"Start-Sleep -Seconds 2",
			role, role, defaultChromePort, psQuote(remoteBin), psQuote(role), defaultChromePort, defaultNATSPort)
		_, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		return err
	}
	cmd := fmt.Sprintf("pkill -f %s >/dev/null 2>&1 || true\nnohup %s src_v3 daemon --role %s --chrome-port %d --nats-port %d >/tmp/dialtone_chrome_v3.log 2>&1 </dev/null &",
		shellQuote("dialtone_chrome_v3 src_v3 daemon"), shellQuote(remoteBin), shellQuote(role), defaultChromePort, defaultNATSPort)
	_, err = sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	return err
}

func stopRemoteService(node sshv1.MeshNode) error {
	if strings.EqualFold(node.OS, "windows") {
		cmd := "Get-Process -Name 'dialtone_chrome_v3','dialtone_chrome_v1' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue"
		_, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		if err != nil && !strings.Contains(err.Error(), "exit status 1") {
			return err
		}
		return nil
	}
	_, err := sshv1.RunNodeCommand(node.Name, "pkill -f 'dialtone_chrome_v3 src_v3 daemon' >/dev/null 2>&1 || true", sshv1.CommandOptions{})
	return err
}

func preferredHost(node sshv1.MeshNode) string {
	if node.PreferWSLPowerShell && runtime.GOOS == "linux" {
		return "127.0.0.1"
	}
	return strings.TrimSpace(node.Host)
}

func mapNodeGOOS(nodeOS string) string {
	switch strings.ToLower(strings.TrimSpace(nodeOS)) {
	case "windows":
		return "windows"
	case "macos", "darwin":
		return "darwin"
	default:
		return "linux"
	}
}

func detectRemoteGOARCH(node sshv1.MeshNode) string {
	if strings.EqualFold(node.OS, "windows") {
		out, err := sshv1.RunNodeCommand(node.Name, "$env:PROCESSOR_ARCHITECTURE", sshv1.CommandOptions{})
		if err == nil && strings.Contains(strings.ToLower(strings.TrimSpace(out)), "arm64") {
			return "arm64"
		}
		return "amd64"
	}
	out, err := sshv1.RunNodeCommand(node.Name, "uname -m", sshv1.CommandOptions{})
	if err == nil {
		v := strings.ToLower(strings.TrimSpace(out))
		if strings.Contains(v, "arm64") || strings.Contains(v, "aarch64") {
			return "arm64"
		}
	}
	return "amd64"
}

func psQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "''") + "'"
}

func shellQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", "'\"'\"'") + "'"
}

func windowsPath(raw string) string {
	raw = strings.ReplaceAll(raw, "/", `\`)
	return strings.ReplaceAll(raw, `\\`, `\`)
}

func shellEscapeGrep(raw string) string {
	return strings.ReplaceAll(raw, `'`, `'\''`)
}
