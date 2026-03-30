package src_v3

import (
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func EnsureRemoteServiceByHost(host, role string, deploy bool) (*CommandResponse, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	role = normalizeRole(role)
	resp, err := sendRemoteCommand(node, commandRequest{
		Command:   "status",
		Role:      role,
		TimeoutMS: 1200,
	})
	if err == nil && chromeServiceReady(resp) {
		return resp, nil
	}
	if err == nil {
		return warmRemoteChromeService(node, role)
	}
	if deploy {
		if err := deployRemoteBinary(node, role, true); err != nil {
			return nil, err
		}
		return warmRemoteChromeService(node, role)
	}
	if err := startRemoteService(node, role); err != nil {
		if err := deployRemoteBinary(node, role, true); err != nil {
			return nil, err
		}
		return warmRemoteChromeService(node, role)
	}
	return warmRemoteChromeService(node, role)
}

func EnsureServiceByTarget(host, role string, deploy bool) (*CommandResponse, error) {
	if isLocalHost(host) {
		role = normalizeRole(role)
		if deploy {
			if err := deployTarget(host, role, true); err != nil {
				return nil, err
			}
		}
		if err := ensureLocalService(role); err != nil {
			return nil, err
		}
		resp, err := sendLocalCommand(commandRequest{
			Command: "status",
			Role:    role,
		})
		if err == nil && chromeServiceReady(resp) {
			return resp, nil
		}
		if err == nil {
			return warmLocalChromeService(role)
		}
		return nil, err
	}
	return EnsureRemoteServiceByHost(host, role, deploy)
}

func RestartRemoteServiceByHost(host, role string) (*CommandResponse, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	role = normalizeRole(role)
	if err := startRemoteService(node, role); err != nil {
		return nil, err
	}
	return warmRemoteChromeService(node, role)
}

func RestartServiceByTarget(host, role string) (*CommandResponse, error) {
	if isLocalHost(host) {
		role = normalizeRole(role)
		if err := stopLocalService(role); err != nil {
			return nil, err
		}
		if err := startLocalService(role); err != nil {
			return nil, err
		}
		return sendLocalCommand(commandRequest{
			Command: "status",
			Role:    role,
		})
	}
	return RestartRemoteServiceByHost(host, role)
}

func ReadRemoteLogsByHost(host string, lines int) (string, string, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return "", "", err
	}
	return readRemoteLogs(node, defaultRole, lines)
}

func ReadLogsByTarget(host, role string, lines int) (string, string, error) {
	return readTargetLogs(host, role, lines)
}

func CountChromeProcessesByTarget(host, role string) (int, error) {
	if isLocalHost(host) {
		return countLocalChromeProcesses(role)
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return 0, err
	}
	return countRemoteChromeProcesses(node, role)
}
