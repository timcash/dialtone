package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const browserDebugConfigRel = "plugins/ui/src_v1/test/browser.debug.json"

type browserDebugConfig struct {
	WebSocketURL string `json:"websocket_url"`
	DebugPort    int    `json:"debug_port"`
	PID          int    `json:"pid"`
	Role         string `json:"role"`
	UpdatedAtUTC string `json:"updated_at_utc"`
}

func LoadBrowserDebugConfig() (*browserDebugConfig, error) {
	paths, err := resolvePaths()
	if err != nil {
		return nil, err
	}
	inPath := filepath.Join(paths.Runtime.SrcRoot, browserDebugConfigRel)
	raw, err := os.ReadFile(inPath)
	if err != nil {
		return nil, err
	}
	var cfg browserDebugConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveBrowserDebugConfig(sc *StepContext) error {
	b, err := sc.Browser()
	if err != nil {
		return err
	}
	if b == nil || b.Session == nil {
		return fmt.Errorf("browser session unavailable")
	}
	paths, err := resolvePaths()
	if err != nil {
		return err
	}
	outPath := filepath.Join(paths.Runtime.SrcRoot, browserDebugConfigRel)
	payload := browserDebugConfig{
		WebSocketURL: strings.TrimSpace(b.Session.WebSocketURL),
		DebugPort:    b.Session.Port,
		PID:          b.Session.PID,
		Role:         "ui-test",
		UpdatedAtUTC: time.Now().UTC().Format(time.RFC3339),
	}
	if strings.TrimSpace(GetOptions().AttachNode) != "" {
		payload.Role = "test"
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(outPath, raw, 0644); err != nil {
		return err
	}
	sc.Infof("saved browser debug config: %s", outPath)
	return nil
}
