package ops

import (
	"os"
	"os/exec"
	"path/filepath"
)

func getDialtoneCmd(repoRoot string) *exec.Cmd {
	script := filepath.Join(repoRoot, "dialtone.sh")
	if os.Getenv("OS") == "Windows_NT" || os.PathSeparator == '\\' {
		script = filepath.Join(repoRoot, "dialtone.ps1")
		return exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	}
	return exec.Command(script)
}
