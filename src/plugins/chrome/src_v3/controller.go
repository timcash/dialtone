package src_v3

import (
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type commandController struct{}

var defaultCommandController commandController

func (commandController) EnsureService(host, role string, deploy bool) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	role = normalizeRole(role)
	if isLocalHost(host) {
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
	return defaultCommandController.ensureRemoteService(host, role, deploy)
}

func (commandController) ensureRemoteService(host, role string, deploy bool) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	node, err := resolveMeshNode(host)
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

func (commandController) RestartService(host, role string) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	role = normalizeRole(role)
	if isLocalHost(host) {
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
	node, err := resolveMeshNode(host)
	if err != nil {
		return nil, err
	}
	if err := startRemoteService(node, role); err != nil {
		return nil, err
	}
	return warmRemoteChromeService(node, role)
}

func (commandController) StartService(host, role string) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	role = normalizeRole(role)
	if isLocalHost(host) {
		if err := startLocalService(role); err != nil {
			return nil, err
		}
		return sendLocalCommand(commandRequest{Command: "status", Role: role})
	}
	node, err := resolveMeshNode(host)
	if err != nil {
		return nil, err
	}
	if err := startRemoteService(node, role); err != nil {
		return nil, err
	}
	return sendRemoteCommand(node, commandRequest{Command: "status", Role: role})
}

func (commandController) StopService(host, role string) error {
	host = effectiveChromeTargetHost(host)
	role = normalizeRole(role)
	if isLocalHost(host) {
		return stopLocalService(role)
	}
	node, err := resolveMeshNode(host)
	if err != nil {
		return err
	}
	return stopRemoteService(node, role)
}

func (c commandController) Status(host, role string) (*CommandResponse, error) {
	return c.Send(host, CommandRequest{
		Command: "status",
		Role:    normalizeRole(role),
	}, false)
}

func (commandController) Send(host string, req CommandRequest, autoStart bool) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	req.Role = normalizeRole(req.Role)
	return sendCommandByTarget(host, req, autoStart)
}

func (c commandController) SendManaged(host string, req CommandRequest) (*CommandResponse, error) {
	req.Role = normalizeRole(req.Role)
	autoStart := shouldAutoStartManagedCommand(req.Command)
	resp, err := c.Send(host, req, autoStart)
	if err == nil {
		return resp, nil
	}
	if !isRecoverableServiceCommandError(err) || !shouldRetryManagedCommand(req.Command) {
		return nil, err
	}
	timeoutMS := req.TimeoutMS
	if timeoutMS < 5000 {
		timeoutMS = 5000
	}
	if _, resetErr := c.Send(host, CommandRequest{
		Command:   "reset",
		Role:      req.Role,
		TimeoutMS: timeoutMS,
	}, autoStart); resetErr == nil {
		return c.Send(host, req, autoStart)
	}
	if _, waitErr := c.WaitHealthy(host, req.Role, 5*time.Second); waitErr != nil {
		return nil, err
	}
	return c.Send(host, req, autoStart)
}

func (c commandController) EnsureManagedPage(host, role string) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	role = normalizeRole(role)
	resp, err := c.Status(host, role)
	if err == nil && chromeServiceReady(resp) {
		return resp, nil
	}
	resp, err = c.Send(host, CommandRequest{
		Command: "open",
		Role:    role,
		URL:     "about:blank",
	}, true)
	if err != nil {
		if isRecoverableServiceCommandError(err) {
			if recovered, recoverErr := c.WaitHealthy(host, role, 5*time.Second); recoverErr == nil {
				return recovered, nil
			}
		}
		return nil, err
	}
	if chromeServiceReady(resp) {
		return resp, nil
	}
	if recovered, recoverErr := c.WaitHealthy(host, role, 5*time.Second); recoverErr == nil {
		return recovered, nil
	}
	return resp, chromeServiceNotReadyError(strings.TrimSpace(host), role, resp)
}

func (c commandController) WaitHealthy(host, role string, timeout time.Duration) (*CommandResponse, error) {
	host = effectiveChromeTargetHost(host)
	role = normalizeRole(role)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := c.Send(host, CommandRequest{
			Command:   "status",
			Role:      role,
			TimeoutMS: 1200,
		}, false)
		if err == nil && chromeServiceReady(resp) {
			return resp, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = chromeServiceNotReadyError(strings.TrimSpace(host), role, resp)
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, chromeServiceNotReadyError(strings.TrimSpace(host), role, nil)
}

func resolveMeshNode(host string) (sshv1.MeshNode, error) {
	return sshv1.ResolveMeshNode(effectiveChromeTargetHost(host))
}

func shouldAutoStartManagedCommand(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "status", "close", "shutdown":
		return false
	default:
		return true
	}
}

func shouldRetryManagedCommand(command string) bool {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "", "status", "open", "close", "reset", "shutdown":
		return false
	default:
		return true
	}
}

func isRecoverableServiceCommandError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(text, "no browser is open") ||
		strings.Contains(text, "failed to open new tab") ||
		strings.Contains(text, "target closed") ||
		strings.Contains(text, "context canceled") ||
		strings.Contains(text, "invalid context") ||
		strings.Contains(text, "no target with given id found") ||
		strings.Contains(text, "(-32602)")
}
