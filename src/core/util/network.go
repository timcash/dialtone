package util

import (
	"dialtone/cli/src/core/logger"
	"net"
)

func CheckStaleHostname(hostname string) {
	if hostname == "" {
		return
	}
	// Try to resolve the hostname. If it resolves before we start tsnet, it's likely stale.
	ips, err := net.LookupIP(hostname)
	if err == nil && len(ips) > 0 {
		logger.LogInfo("[Pre-flight] WARNING: Hostname %s already resolves to %v. This might be a stale MagicDNS entry or another node. This can cause 'operation timed out' errors.", hostname, ips)
	} else {
		logger.LogInfo("[Pre-flight] Hostname %s is not currently resolvable (this is good).", hostname)
	}
}
