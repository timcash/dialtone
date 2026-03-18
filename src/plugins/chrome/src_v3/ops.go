package src_v3

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

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
	_ = stopRemoteService(node)
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
