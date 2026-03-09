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
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	if deploy {
		if err := deployRemoteBinary(node, role, true); err != nil {
			return nil, err
		}
	}
	resp, err := sendRemoteCommand(node, commandRequest{
		Command: "status",
		Role:    role,
	})
	if err == nil {
		return resp, nil
	}
	if err := startRemoteService(node, role); err != nil {
		return nil, err
	}
	return sendRemoteCommand(node, commandRequest{
		Command: "status",
		Role:    role,
	})
}

func EnsureServiceByTarget(host, role string, deploy bool) (*CommandResponse, error) {
	if isLocalHost(host) {
		if deploy {
			if err := deployTarget(host, role, true); err != nil {
				return nil, err
			}
		}
		if err := ensureLocalService(role); err != nil {
			return nil, err
		}
		return sendLocalCommand(commandRequest{
			Command: "status",
			Role:    strings.TrimSpace(role),
		})
	}
	return EnsureRemoteServiceByHost(host, role, deploy)
}

func RestartRemoteServiceByHost(host, role string) (*CommandResponse, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return nil, err
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	if err := startRemoteService(node, role); err != nil {
		return nil, err
	}
	return sendRemoteCommand(node, commandRequest{
		Command: "status",
		Role:    role,
	})
}

func RestartServiceByTarget(host, role string) (*CommandResponse, error) {
	if isLocalHost(host) {
		if err := stopLocalService(role); err != nil {
			return nil, err
		}
		if err := startLocalService(role); err != nil {
			return nil, err
		}
		return sendLocalCommand(commandRequest{
			Command: "status",
			Role:    strings.TrimSpace(role),
		})
	}
	return RestartRemoteServiceByHost(host, role)
}

func ReadRemoteLogsByHost(host string, lines int) (string, string, error) {
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(host))
	if err != nil {
		return "", "", err
	}
	return readRemoteLogs(node, lines)
}

func ReadLogsByTarget(host, role string, lines int) (string, string, error) {
	return readTargetLogs(host, role, lines)
}
