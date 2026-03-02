package cli

import (
	"flag"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func handleInstallCmd(args []string) {
	fs := flag.NewFlagSet("chrome install", flag.ExitOnError)
	host := fs.String("host", "", "Optional mesh host target (or 'all') to install runtime dependencies")
	_ = fs.Parse(args)

	target := strings.TrimSpace(*host)
	if target == "" {
		if err := ensureLocalBun(); err != nil {
			logs.Fatal("chrome install failed: %v", err)
		}
		logs.Info("chrome install complete (local)")
		return
	}

	nodes, err := resolveChromeHosts(target)
	if err != nil {
		logs.Fatal("chrome install --host: %v", err)
	}

	fail := 0
	ok := 0
	for _, node := range nodes {
		if strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
			logs.Info("chrome install host=%s skipped bun install on windows", node.Name)
			ok++
			continue
		}
		cmd := `if command -v bun >/dev/null 2>&1; then echo "bun-ok"; exit 0; fi; curl -fsSL https://bun.sh/install | bash`
		out, rerr := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
		if rerr != nil {
			fail++
			logs.Warn("chrome install host=%s failed: %v output=%s", node.Name, rerr, strings.TrimSpace(out))
			continue
		}
		ok++
		logs.Info("chrome install host=%s ok", node.Name)
	}

	if ok == 0 {
		logs.Fatal("chrome install failed on all targets (%d)", fail)
	}
	if fail > 0 {
		logs.Fatal("chrome install completed with failures: ok=%d fail=%d", ok, fail)
	}
	logs.Info("chrome install completed: ok=%d", ok)
}

func ensureLocalBun() error {
	if _, err := exec.LookPath("bun"); err == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return fmt.Errorf("bun not found on windows; install bun manually for this shell")
	}
	cmd := exec.Command("bash", "-lc", "curl -fsSL https://bun.sh/install | bash")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bun install failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}
