package test

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func getDialtoneCmd(repoRoot string) *exec.Cmd {
	return configv1.DialtoneCommand(repoRoot)
}

func cleanupPort(port int) error {
	portStr := strconv.Itoa(port)
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
			fmt.Sprintf("$connections = Get-NetTCPConnection -LocalPort %s -State Listen -ErrorAction SilentlyContinue; if ($connections) { $connections | Select-Object -ExpandProperty OwningProcess -Unique | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue } }", portStr))
		return cmd.Run()
	}
	if err := exec.Command("fuser", "-k", "-n", "tcp", portStr).Run(); err == nil {
		return nil
	}
	return exec.Command("sh", "-lc", "pids=$(lsof -ti tcp:"+portStr+" -sTCP:LISTEN 2>/dev/null); [ -z \"$pids\" ] || kill -9 $pids").Run()
}
