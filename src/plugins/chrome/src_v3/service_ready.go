package src_v3

import (
	"fmt"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func chromeServiceReady(resp *commandResponse) bool {
	return resp != nil && resp.BrowserPID > 0 && !resp.Unhealthy
}

func chromeServiceNotReadyError(host, role string, resp *commandResponse) error {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "local"
	}
	role = normalizeRole(role)
	if resp == nil {
		return fmt.Errorf("chrome service on %s role=%s did not return status", host, role)
	}
	if strings.TrimSpace(resp.Error) != "" {
		return fmt.Errorf("chrome service on %s role=%s is not ready: %s", host, role, strings.TrimSpace(resp.Error))
	}
	return fmt.Errorf("chrome service on %s role=%s is not ready (browser_pid=%d unhealthy=%t)", host, role, resp.BrowserPID, resp.Unhealthy)
}

func warmLocalChromeService(role string) (*commandResponse, error) {
	role = normalizeRole(role)
	resp, err := sendLocalCommand(commandRequest{
		Command: "open",
		Role:    role,
		URL:     "about:blank",
	})
	if err != nil {
		return nil, err
	}
	if !chromeServiceReady(resp) {
		return nil, chromeServiceNotReadyError("", role, resp)
	}
	resp.IsNew = true
	return resp, nil
}

func warmRemoteChromeService(node sshv1.MeshNode, role string) (*commandResponse, error) {
	role = normalizeRole(role)
	resp, err := sendRemoteCommand(node, commandRequest{
		Command: "open",
		Role:    role,
		URL:     "about:blank",
	})
	if err != nil {
		return nil, err
	}
	if !chromeServiceReady(resp) {
		return nil, chromeServiceNotReadyError(node.Name, role, resp)
	}
	resp.IsNew = true
	return resp, nil
}
