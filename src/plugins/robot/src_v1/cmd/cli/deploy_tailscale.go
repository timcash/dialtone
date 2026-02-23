package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func ensureRobotAuthKey(repoRoot string) error {
	current := strings.TrimSpace(os.Getenv("ROBOT_TS_AUTHKEY"))
	if current != "" {
		return nil
	}

	envPath := filepath.Join(repoRoot, "env", ".env")
	apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
	}
	hostname := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	if hostname == "" {
		hostname = "drone-1"
	}
	tailnet := resolveRobotTailnet()

	if apiKey != "" {
		logs.Info("[DEPLOY] Provisioning dedicated ROBOT_TS_AUTHKEY for %s on %s...", hostname, tailnet)
		key, err := provisionRobotAuthKeyWithAPI(apiKey, tailnet, "dialtone-robot-"+hostname, []string{"dialtone", "robot", hostname})
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "requested tags") {
			logs.Warn("[DEPLOY] Tailnet does not allow requested tags; retrying ROBOT_TS_AUTHKEY provisioning without tags")
			key, err = provisionRobotAuthKeyWithAPI(apiKey, tailnet, "dialtone-robot-"+hostname, nil)
		}
		if err != nil {
			return fmt.Errorf("failed to provision ROBOT_TS_AUTHKEY: %w", err)
		}
		if err := tsnetv1.UpsertEnvVar(envPath, "ROBOT_TS_AUTHKEY", key); err != nil {
			return fmt.Errorf("failed writing ROBOT_TS_AUTHKEY to %s: %w", envPath, err)
		}
		_ = os.Setenv("ROBOT_TS_AUTHKEY", key)
		logs.Info("[DEPLOY] Wrote ROBOT_TS_AUTHKEY to %s", envPath)
		return nil
	}

	fallback := strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	if fallback == "" {
		fallback = strings.TrimSpace(os.Getenv("TAILSCALE_AUTHKEY"))
	}
	if fallback == "" {
		return fmt.Errorf("missing ROBOT_TS_AUTHKEY and cannot provision one: set TS_API_KEY locally")
	}
	logs.Warn("[DEPLOY] TS_API_KEY missing; reusing existing TS_AUTHKEY for ROBOT_TS_AUTHKEY")
	if err := tsnetv1.UpsertEnvVar(envPath, "ROBOT_TS_AUTHKEY", fallback); err != nil {
		return fmt.Errorf("failed writing fallback ROBOT_TS_AUTHKEY to %s: %w", envPath, err)
	}
	_ = os.Setenv("ROBOT_TS_AUTHKEY", fallback)
	return nil
}

func resolveRobotTailnet() string {
	for _, key := range []string{"ROBOT_TS_TAILNET", "TS_TAILNET", "TAILSCALE_TAILNET"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	if tailnet, err := tsnetv1.DetectTailnetFromLocalStatus(); err == nil && strings.TrimSpace(tailnet) != "" {
		return strings.TrimSpace(tailnet)
	}
	return "shad-artichoke.ts.net"
}

func provisionRobotAuthKeyWithAPI(apiKey, tailnet, description string, tags []string) (string, error) {
	tagValues := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "tag:") {
			t = "tag:" + t
		}
		tagValues = append(tagValues, t)
	}
	reqBody := map[string]any{
		"capabilities": map[string]any{
			"devices": map[string]any{
				"create": map[string]any{
					"reusable":      true,
					"ephemeral":     true,
					"preauthorized": true,
					"tags":          tagValues,
				},
			},
		},
		"expirySeconds": 24 * 30 * 3600,
		"description":   description,
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/keys", url.PathEscape(strings.TrimSpace(tailnet)))
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(strings.TrimSpace(apiKey), "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("tailscale api POST %s failed: %s", endpoint, strings.TrimSpace(string(body)))
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}

	keyVal := strings.TrimSpace(extractAuthKey(parsed))
	if keyVal == "" {
		return "", fmt.Errorf("tailscale api returned empty auth key payload")
	}
	return keyVal, nil
}

func getTailscaleIP(hostname string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
	}
	if apiKey == "" {
		return "", fmt.Errorf("TS_API_KEY not set")
	}
	tailnet := resolveRobotTailnet()

	endpoint := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", url.PathEscape(tailnet))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(apiKey, "")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to list devices: %s", resp.Status)
	}

	var data struct {
		Devices []struct {
			Hostname  string   `json:"hostname"`
			Name      string   `json:"name"`
			Addresses []string `json:"addresses"`
		} `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	for _, d := range data.Devices {
		if d.Hostname == hostname || strings.HasPrefix(d.Name, hostname+".") {
			if len(d.Addresses) > 0 {
				return d.Addresses[0], nil
			}
		}
	}
	return "", fmt.Errorf("device not found")
}

func pruneTailscaleNodes(hostname string) error {
	apiKey := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
	}
	if apiKey == "" {
		return fmt.Errorf("TS_API_KEY not set; cannot prune nodes")
	}
	tailnet := resolveRobotTailnet()

	endpoint := fmt.Sprintf("https://api.tailscale.com/api/v2/tailnet/%s/devices", url.PathEscape(tailnet))
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(apiKey, "")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("TS_API_KEY is invalid or expired. Please update it in env/.env (HTTP 401)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to list devices: %s", resp.Status)
	}

	var data struct {
		Devices []struct {
			ID       string   `json:"id"`
			Hostname string   `json:"hostname"`
			Name     string   `json:"name"`
			Tags     []string `json:"tags"`
		} `json:"devices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	for _, d := range data.Devices {
		if d.Hostname == hostname || strings.HasPrefix(d.Name, hostname+".") {
			isDialtone := false
			for _, t := range d.Tags {
				if t == "tag:dialtone" {
					isDialtone = true
					break
				}
			}
			if !isDialtone {
				logs.Warn("   [PRUNE] Skipping node %s (id=%s) because it lacks tag:dialtone (could be OS node)", d.Name, d.ID)
				continue
			}

			logs.Info("   [PRUNE] Deleting conflicting node: %s (id=%s)", d.Name, d.ID)
			deleteURL := fmt.Sprintf("https://api.tailscale.com/api/v2/device/%s", url.PathEscape(d.ID))
			delReq, _ := http.NewRequest("DELETE", deleteURL, nil)
			delReq.SetBasicAuth(apiKey, "")
			delResp, err := client.Do(delReq)
			if err != nil {
				logs.Warn("   [PRUNE] Failed to delete %s: %v", d.ID, err)
				continue
			}
			delResp.Body.Close()
			if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
				logs.Warn("   [PRUNE] Unexpected status deleting %s: %s", d.ID, delResp.Status)
			} else {
				logs.Info("   [PRUNE] Successfully deleted %s", d.ID)
			}
		}
	}
	return nil
}

func extractAuthKey(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if v, ok := payload["key"].(string); ok {
		return v
	}
	if keyObj, ok := payload["key"].(map[string]any); ok {
		if v, ok := keyObj["key"].(string); ok {
			return v
		}
	}
	if authKey, ok := payload["authKey"].(string); ok {
		return authKey
	}
	return ""
}
