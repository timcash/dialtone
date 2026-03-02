package cli

import (
	"flag"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func handleDashboardCmd(args []string) {
	fs := flag.NewFlagSet("chrome dashboard", flag.ExitOnError)
	host := fs.String("host", "", "Target host name, or 'all'")
	role := fs.String("role", "dev", "Role tag to reuse/start")
	port := fs.Int("port", 9333, "Preferred debug port")
	_ = fs.Parse(args)

	target := strings.TrimSpace(*host)
	if target == "" {
		logs.Fatal("dashboard requires --host")
	}
	url := "http://127.0.0.1:19444/process-ui"
	nodes, err := resolveChromeHosts(target)
	if err != nil {
		logs.Fatal("dashboard --host: %v", err)
	}
	ok := 0
	for _, node := range nodes {
		if strings.EqualFold(strings.TrimSpace(node.Name), "local") {
			ws, err := openOnHost("local", url, strings.TrimSpace(*role), defaultChromeServicePort, false, false)
			if err != nil {
				logs.Warn("dashboard host=%s failed: %v", node.Name, err)
				continue
			}
			ok++
			logs.Info("dashboard host=%s ok ws=%s", node.Name, strings.TrimSpace(ws))
			continue
		}
		err := startRemoteChrome(node, remoteStartOptions{
			URL:           url,
			Port:          *port,
			Role:          strings.TrimSpace(*role),
			Headless:      false,
			GPU:           true,
			DebugAddress:  "0.0.0.0",
			ReuseExisting: true,
		})
		if err != nil {
			logs.Warn("dashboard host=%s failed: %v", node.Name, err)
			continue
		}
		ok++
		logs.Info("dashboard host=%s opened %s", node.Name, url)
	}
	if ok == 0 {
		logs.Fatal("dashboard failed on all targets")
	}
}
