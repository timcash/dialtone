package test

import (
	"strings"
	"sync"
)

type RuntimeConfig struct {
	BrowserNode              string
	RemoteBrowserRole        string
	RemoteRequireRole        bool
	BrowserAllowCreateTarget bool
	BrowserNewTargetURL      string
	ActionsPerMinute         float64
	NoSSH                    bool
	RemoteDebugPort          int
	RemoteDebugPorts         []int
	RemoteBrowserPID         int
	RemoteNoLaunch           bool
}

var (
	runtimeConfigMu sync.RWMutex
	runtimeConfig   RuntimeConfig
)

func SetRuntimeConfig(cfg RuntimeConfig) {
	cfg.BrowserNode = strings.TrimSpace(cfg.BrowserNode)
	cfg.BrowserNewTargetURL = strings.TrimSpace(cfg.BrowserNewTargetURL)
	cfg.RemoteDebugPorts = append([]int(nil), cfg.RemoteDebugPorts...)
	runtimeConfigMu.Lock()
	runtimeConfig = cfg
	runtimeConfigMu.Unlock()
}

func RuntimeConfigSnapshot() RuntimeConfig {
	runtimeConfigMu.RLock()
	defer runtimeConfigMu.RUnlock()
	out := runtimeConfig
	out.RemoteDebugPorts = append([]int(nil), out.RemoteDebugPorts...)
	return out
}

func UpdateRuntimeConfig(update func(*RuntimeConfig)) {
	runtimeConfigMu.Lock()
	defer runtimeConfigMu.Unlock()
	update(&runtimeConfig)
}
