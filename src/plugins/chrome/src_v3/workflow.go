package src_v3

func EnsureRemoteServiceByHost(host, role string, deploy bool) (*CommandResponse, error) {
	return defaultCommandController.ensureRemoteService(host, role, deploy)
}

func EnsureServiceByTarget(host, role string, deploy bool) (*CommandResponse, error) {
	return defaultCommandController.EnsureService(host, role, deploy)
}

func RestartRemoteServiceByHost(host, role string) (*CommandResponse, error) {
	return defaultCommandController.RestartService(host, role)
}

func RestartServiceByTarget(host, role string) (*CommandResponse, error) {
	return defaultCommandController.RestartService(host, role)
}

func ReadRemoteLogsByHost(host string, lines int) (string, string, error) {
	node, err := resolveMeshNode(host)
	if err != nil {
		return "", "", err
	}
	return readRemoteLogs(node, defaultRole, lines)
}

func ReadLogsByTarget(host, role string, lines int) (string, string, error) {
	return readTargetLogs(host, role, lines)
}

func CountChromeProcessesByTarget(host, role string) (int, error) {
	host = effectiveChromeTargetHost(host)
	if isLocalHost(host) {
		return countLocalChromeProcesses(role)
	}
	node, err := resolveMeshNode(host)
	if err != nil {
		return 0, err
	}
	return countRemoteChromeProcesses(node, role)
}
