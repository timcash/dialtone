package chrome

import (
	"fmt"
	"net"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func resolveReachableDebugHost(port int, nodeInfo sshv1.MeshNode) string {
	hosts := make([]string, 0, 4)
	isWindows := strings.EqualFold(strings.TrimSpace(nodeInfo.OS), "windows")
	if h := strings.TrimSpace(nodeInfo.Host); h != "" {
		if isWindows {
			if ip := sanitizeNonLoopbackIP(h); ip != "" {
				hosts = append(hosts, ip)
			} else if ip := resolveIPv4Host(h); ip != "" {
				hosts = append(hosts, ip)
			}
		} else {
			hosts = append(hosts, h)
		}
	}
	if gw := detectWSLHostGatewayIP(); gw != "" && !isWindows {
		hosts = append(hosts, gw)
	}
	if !isWindows {
		hosts = append(hosts, "127.0.0.1")
	}
	for _, h := range hosts {
		if isWindows && strings.HasPrefix(h, "127.") {
			continue
		}
		if canDialHostPort(h, port, 1200*time.Millisecond) {
			return h
		}
	}
	return ""
}

func ensureWindowsDebugRelay(nodeInfo sshv1.MeshNode, listenPort, targetPort int) error {
	if listenPort <= 0 || targetPort <= 0 {
		return fmt.Errorf("invalid relay ports listen=%d target=%d", listenPort, targetPort)
	}
	ps := fmt.Sprintf(`$ErrorActionPreference='Stop'
$listen=%d
$target=%d
netsh interface portproxy delete v4tov4 listenaddress=0.0.0.0 listenport=$listen | Out-Null
netsh interface portproxy add v4tov4 listenaddress=0.0.0.0 listenport=$listen connectaddress=127.0.0.1 connectport=$target | Out-Null
$rule=("Dialtone Chrome Relay "+$listen)
try{
  if(-not (Get-NetFirewallRule -DisplayName $rule -ErrorAction SilentlyContinue)){
    New-NetFirewallRule -DisplayName $rule -Direction Inbound -Action Allow -Protocol TCP -LocalPort $listen -Profile Any | Out-Null
  }
}catch{}
Write-Output ("relay:"+$listen+"->"+$target)`, listenPort, targetPort)
	_, err := sshv1.RunNodeCommand(nodeInfo.Name, ps, sshv1.CommandOptions{})
	return err
}

func resolveIPv4Host(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			if out := sanitizeNonLoopbackIP(v4.String()); out != "" {
				return out
			}
		}
	}
	return ""
}

func sanitizeNonLoopbackIP(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ""
}

func canDialHostPort(host string, port int, timeout time.Duration) bool {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 {
		return false
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
