package cli

import (
	"strings"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
)

func normalizeDebugRequestDefaults(req *debugURLRequest, defaultRole, defaultDebugAddress string) {
	if req == nil {
		return
	}
	req.Role = strings.TrimSpace(req.Role)
	req.URL = strings.TrimSpace(req.URL)
	req.DebugAddress = strings.TrimSpace(req.DebugAddress)
	req.UserDataDir = strings.TrimSpace(req.UserDataDir)
	if req.Role == "" {
		req.Role = strings.TrimSpace(defaultRole)
	}
	if req.URL == "" {
		req.URL = "about:blank"
	}
	if req.DebugAddress == "" {
		req.DebugAddress = strings.TrimSpace(defaultDebugAddress)
	}
	if req.UserDataDir == "" {
		req.UserDataDir = defaultServiceUserDataDir(req.Role, req.Headless)
	}
}

func startDaemonManagedSession(req debugURLRequest, kiosk bool) (*chrome.Session, error) {
	sess, err := chrome.StartSession(chrome.SessionOptions{
		RequestedPort: req.Port,
		GPU:           true,
		Headless:      req.Headless,
		Kiosk:         kiosk,
		TargetURL:     req.URL,
		Role:          req.Role,
		ReuseExisting: req.Reuse,
		UserDataDir:   req.UserDataDir,
		DebugAddress:  req.DebugAddress,
	})
	if err != nil {
		return nil, err
	}
	sess = ensureSessionPageReady(req, sess)
	enforceSingleRoleInstance(req.Role, req.Headless, sess.PID)
	if err := ensureSinglePageTab(sess.Port); err != nil {
		return nil, err
	}
	return sess, nil
}
