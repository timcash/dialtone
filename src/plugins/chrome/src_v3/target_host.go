package src_v3

import (
	"os"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

var listChromeMeshNodesFunc = sshv1.ListMeshNodes

func effectiveChromeTargetHost(host string) string {
	host = strings.TrimSpace(host)
	if host != "" {
		return host
	}
	if fallback := defaultChromeTargetHost(); fallback != "" {
		return fallback
	}
	return ""
}

func defaultChromeTargetHost() string {
	for _, key := range []string{"DIALTONE_CHROME_DEFAULT_HOST", "DIALTONE_CHROME_TEST_HOST"} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			return raw
		}
		if raw := strings.TrimSpace(configv1.LookupEnvString(key)); raw != "" {
			return raw
		}
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) == "" {
		return ""
	}
	for _, node := range listChromeMeshNodesFunc() {
		if !strings.EqualFold(strings.TrimSpace(node.OS), "windows") {
			continue
		}
		if !node.PreferWSLPowerShell {
			continue
		}
		if name := strings.TrimSpace(node.Name); name != "" {
			return name
		}
	}
	return ""
}
