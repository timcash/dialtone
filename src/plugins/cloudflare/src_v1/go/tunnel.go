package cloudflare

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ProvisionRequest struct {
	TunnelName string
	Domain     string
	APIToken   string
	AccountID  string
	EnvPath    string
}

type ProvisionResult struct {
	FullHostname string
	TunnelID     string
	EnvVarName   string
	RunToken     string
	DNSCreated   bool
}

type CleanupRequest struct {
	TunnelName string
	Domain     string
	APIToken   string
	AccountID  string
	EnvPath    string
}

type CleanupResult struct {
	FullHostname       string
	TunnelID           string
	TokenEnvName       string
	DNSDeleted         bool
	ConnectionsCleared bool
	TunnelDeleted      bool
	TokenRemoved       bool
}

const defaultManagedZone = "dialtone.earth"

func TokenEnvKey(name string) string {
	upper := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
	return "CF_TUNNEL_TOKEN_" + upper
}

func ResolveTunnelToken(name, explicitToken string) string {
	if strings.TrimSpace(explicitToken) != "" {
		return strings.TrimSpace(explicitToken)
	}
	name = strings.TrimSpace(name)
	if name != "" {
		if tok := strings.TrimSpace(os.Getenv(TokenEnvKey(name))); tok != "" {
			return tok
		}
		if tok := strings.TrimSpace(readConfigVar(defaultConfigPath(), TokenEnvKey(name))); tok != "" {
			return tok
		}
	}
	if tok := strings.TrimSpace(os.Getenv("CF_TUNNEL_TOKEN")); tok != "" {
		return tok
	}
	return strings.TrimSpace(readConfigVar(defaultConfigPath(), "CF_TUNNEL_TOKEN"))
}

func BuildTunnelRunArgs(name, url, token string) ([]string, error) {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	token = strings.TrimSpace(token)

	if url == "" {
		return nil, fmt.Errorf("--url is required")
	}
	args := []string{"run"}
	if token != "" {
		args = append(args, "--token", token, "--url", url)
		return args, nil
	}
	if name == "" {
		return nil, fmt.Errorf("tunnel name is required when no token is provided")
	}
	args = append(args, "--url", url, name)
	return args, nil
}

func ProvisionTunnelAndDNS(in ProvisionRequest) (ProvisionResult, error) {
	name := strings.TrimSpace(in.TunnelName)
	domain := strings.TrimSpace(in.Domain)
	apiToken := strings.TrimSpace(in.APIToken)
	accountID := strings.TrimSpace(in.AccountID)
	envPath := strings.TrimSpace(in.EnvPath)

	if name == "" {
		return ProvisionResult{}, fmt.Errorf("tunnel name is required")
	}
	if domain == "" {
		return ProvisionResult{}, fmt.Errorf("domain is required")
	}
	if apiToken == "" {
		return ProvisionResult{}, fmt.Errorf("api token is required")
	}
	if accountID == "" {
		return ProvisionResult{}, fmt.Errorf("account id is required")
	}
	if envPath == "" {
		envPath = "env/dialtone.json"
	}

	client := &http.Client{}
	hostSpec, err := resolveManagedHostname(name, domain)
	if err != nil {
		return ProvisionResult{}, err
	}

	// 1) Resolve zone id.
	zoneID, err := resolveZoneID(client, apiToken, hostSpec.Zone)
	if err != nil {
		return ProvisionResult{}, err
	}

	// 2) Create tunnel.
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return ProvisionResult{}, fmt.Errorf("secret generation failed: %w", err)
	}
	tunnelSecret := base64.StdEncoding.EncodeToString(secretBytes)
	payload := map[string]string{"name": name, "tunnel_secret": tunnelSecret}
	body, _ := json.Marshal(payload)
	createURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/tunnels", accountID)
	createReq, _ := http.NewRequest("POST", createURL, bytes.NewBuffer(body))
	createReq.Header.Set("Authorization", "Bearer "+apiToken)
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := client.Do(createReq)
	if err != nil {
		return ProvisionResult{}, fmt.Errorf("create tunnel request failed: %w", err)
	}
	defer createResp.Body.Close()

	type createResponse struct {
		Result struct {
			ID string `json:"id"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	var cr createResponse
	if err := json.NewDecoder(createResp.Body).Decode(&cr); err != nil {
		return ProvisionResult{}, fmt.Errorf("decode create response failed: %w", err)
	}
	if createResp.StatusCode != http.StatusOK || !cr.Success || cr.Result.ID == "" {
		return ProvisionResult{}, fmt.Errorf("tunnel creation failed (status=%d)", createResp.StatusCode)
	}
	tunnelID := cr.Result.ID

	// 3) Create DNS CNAME.
	dnsCreated := false
	dnsPayload := map[string]any{
		"type":    "CNAME",
		"name":    hostSpec.RecordName,
		"content": fmt.Sprintf("%s.cfargotunnel.com", tunnelID),
		"proxied": true,
		"ttl":     1,
	}
	dnsBody, _ := json.Marshal(dnsPayload)
	dnsURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
	dnsReq, _ := http.NewRequest("POST", dnsURL, bytes.NewBuffer(dnsBody))
	dnsReq.Header.Set("Authorization", "Bearer "+apiToken)
	dnsReq.Header.Set("Content-Type", "application/json")
	dnsResp, err := client.Do(dnsReq)
	if err == nil {
		defer dnsResp.Body.Close()
		dnsCreated = dnsResp.StatusCode == http.StatusOK
	}

	// 4) Save run token in dialtone config.
	tokenData := map[string]string{"a": accountID, "t": tunnelID, "s": tunnelSecret}
	tokenJSON, _ := json.Marshal(tokenData)
	runToken := base64.StdEncoding.EncodeToString(tokenJSON)
	envVar := TokenEnvKey(name)
	if err := upsertConfigVar(envPath, envVar, runToken); err != nil {
		return ProvisionResult{}, fmt.Errorf("write config token failed: %w", err)
	}

	return ProvisionResult{
		FullHostname: hostSpec.FullHostname,
		TunnelID:     tunnelID,
		EnvVarName:   envVar,
		RunToken:     runToken,
		DNSCreated:   dnsCreated,
	}, nil
}

func CleanupTunnelAndDNS(in CleanupRequest) (CleanupResult, error) {
	name := strings.TrimSpace(in.TunnelName)
	domain := strings.TrimSpace(in.Domain)
	apiToken := strings.TrimSpace(in.APIToken)
	accountID := strings.TrimSpace(in.AccountID)
	envPath := strings.TrimSpace(in.EnvPath)

	if name == "" {
		return CleanupResult{}, fmt.Errorf("tunnel name is required")
	}
	if domain == "" {
		return CleanupResult{}, fmt.Errorf("domain is required")
	}
	if apiToken == "" {
		return CleanupResult{}, fmt.Errorf("api token is required")
	}
	if accountID == "" {
		return CleanupResult{}, fmt.Errorf("account id is required")
	}
	if envPath == "" {
		envPath = defaultConfigPath()
	}

	client := &http.Client{}
	hostSpec, err := resolveManagedHostname(name, domain)
	if err != nil {
		return CleanupResult{}, err
	}
	zoneID, err := resolveZoneID(client, apiToken, hostSpec.Zone)
	if err != nil {
		return CleanupResult{}, err
	}
	tunnelID := decodeTunnelIDFromToken(readConfigVar(envPath, TokenEnvKey(name)))
	if tunnelID == "" {
		tunnelID, err = resolveTunnelIDByName(client, apiToken, accountID, name)
		if err != nil {
			return CleanupResult{}, err
		}
	}
	if tunnelID == "" {
		return CleanupResult{}, fmt.Errorf("tunnel id for %s not found", name)
	}

	dnsDeleted, err := deleteDNSRecord(client, apiToken, zoneID, hostSpec.FullHostname)
	if err != nil {
		return CleanupResult{}, err
	}
	if err := deleteTunnelConnections(client, apiToken, accountID, tunnelID); err != nil {
		return CleanupResult{}, err
	}
	if err := deleteTunnel(client, apiToken, accountID, tunnelID); err != nil {
		return CleanupResult{}, err
	}
	tokenEnvName := TokenEnvKey(name)
	tokenRemoved, err := removeConfigVar(envPath, tokenEnvName)
	if err != nil {
		return CleanupResult{}, err
	}
	return CleanupResult{
		FullHostname:       hostSpec.FullHostname,
		TunnelID:           tunnelID,
		TokenEnvName:       tokenEnvName,
		DNSDeleted:         dnsDeleted,
		ConnectionsCleared: true,
		TunnelDeleted:      true,
		TokenRemoved:       tokenRemoved,
	}, nil
}

type managedHostname struct {
	Zone         string
	RecordName   string
	FullHostname string
}

func resolveManagedHostname(name string, domain string) (managedHostname, error) {
	name = strings.TrimSpace(name)
	domain = strings.TrimSpace(domain)
	if name == "" {
		return managedHostname{}, fmt.Errorf("tunnel name is required")
	}
	if domain == "" {
		domain = defaultManagedZone
	}

	zone := domain
	if !strings.Contains(zone, ".") {
		// Dialtone commonly stores a short public label like "rover-1" in DIALTONE_DOMAIN.
		// Cloudflare zone operations still target the managed apex zone.
		zone = defaultManagedZone
	}

	return managedHostname{
		Zone:         zone,
		RecordName:   name,
		FullHostname: fmt.Sprintf("%s.%s", name, zone),
	}, nil
}

func resolveZoneID(client *http.Client, apiToken string, domain string) (string, error) {
	type zoneResponse struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	zoneReq, _ := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s", domain), nil)
	zoneReq.Header.Set("Authorization", "Bearer "+apiToken)
	zoneResp, err := client.Do(zoneReq)
	if err != nil {
		return "", fmt.Errorf("fetch zone failed: %w", err)
	}
	defer zoneResp.Body.Close()
	var zr zoneResponse
	if err := json.NewDecoder(zoneResp.Body).Decode(&zr); err != nil {
		return "", fmt.Errorf("decode zone response failed: %w", err)
	}
	if !zr.Success || len(zr.Result) == 0 {
		return "", fmt.Errorf("zone %s not found", domain)
	}
	return zr.Result[0].ID, nil
}

func resolveTunnelIDByName(client *http.Client, apiToken string, accountID string, name string) (string, error) {
	type listResponse struct {
		Result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/tunnels?name=%s", accountID, name), nil)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("list tunnels failed: %w", err)
	}
	defer resp.Body.Close()
	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return "", fmt.Errorf("decode tunnel list failed: %w", err)
	}
	if !lr.Success {
		return "", fmt.Errorf("list tunnels failed for %s", name)
	}
	for _, item := range lr.Result {
		if strings.TrimSpace(item.Name) == name && strings.TrimSpace(item.ID) != "" {
			return strings.TrimSpace(item.ID), nil
		}
	}
	return "", nil
}

func deleteDNSRecord(client *http.Client, apiToken string, zoneID string, fullHostname string) (bool, error) {
	type recordResponse struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=CNAME&name=%s", zoneID, fullHostname), nil)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("list dns records failed: %w", err)
	}
	defer resp.Body.Close()
	var rr recordResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return false, fmt.Errorf("decode dns list failed: %w", err)
	}
	if !rr.Success {
		return false, fmt.Errorf("dns lookup failed for %s", fullHostname)
	}
	deleted := false
	for _, rec := range rr.Result {
		if strings.TrimSpace(rec.ID) == "" {
			continue
		}
		delReq, _ := http.NewRequest("DELETE", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, rec.ID), nil)
		delReq.Header.Set("Authorization", "Bearer "+apiToken)
		delResp, err := client.Do(delReq)
		if err != nil {
			return deleted, fmt.Errorf("delete dns record failed: %w", err)
		}
		delResp.Body.Close()
		if delResp.StatusCode >= 200 && delResp.StatusCode < 300 {
			deleted = true
		}
	}
	return deleted, nil
}

func deleteTunnel(client *http.Client, apiToken string, accountID string, tunnelID string) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/tunnels/%s", accountID, tunnelID), nil)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete tunnel failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		text := strings.TrimSpace(string(body))
		if text == "" {
			return fmt.Errorf("delete tunnel failed: status=%d", resp.StatusCode)
		}
		return fmt.Errorf("delete tunnel failed: status=%d body=%s", resp.StatusCode, text)
	}
	return nil
}

func deleteTunnelConnections(client *http.Client, apiToken string, accountID string, tunnelID string) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/connections", accountID, tunnelID), nil)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete tunnel connections failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Errorf("delete tunnel connections failed: status=%d", resp.StatusCode)
	}
	return fmt.Errorf("delete tunnel connections failed: status=%d body=%s", resp.StatusCode, text)
}

func defaultConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("DIALTONE_ENV_FILE")); path != "" {
		return path
	}
	if path := strings.TrimSpace(os.Getenv("DIALTONE_MESH_CONFIG")); path != "" {
		return path
	}
	return filepath.Join("env", "dialtone.json")
}

func readConfigVar(path string, key string) string {
	path = strings.TrimSpace(path)
	key = strings.TrimSpace(key)
	if path == "" || key == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		return ""
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	if v, ok := doc[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func removeConfigVar(path string, key string) (bool, error) {
	path = strings.TrimSpace(path)
	key = strings.TrimSpace(key)
	if path == "" || key == "" {
		return false, fmt.Errorf("missing config path or key")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false, err
	}
	if _, ok := doc[key]; !ok {
		return false, nil
	}
	delete(doc, key)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')
	return true, os.WriteFile(path, out, 0o644)
}

func decodeTunnelIDFromToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return ""
	}
	var doc map[string]string
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc["t"])
}

func upsertConfigVar(path, key, value string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "env/dialtone.json"
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("missing config key")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		doc := map[string]any{}
		if raw, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(raw)) != "" {
			if err := json.Unmarshal(raw, &doc); err != nil {
				return err
			}
		}
		doc[key] = value
		out, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		out = append(out, '\n')
		return os.WriteFile(path, out, 0o644)
	}
	existing := ""
	if raw, err := os.ReadFile(path); err == nil {
		existing = string(raw)
	}
	lines := []string{}
	if existing != "" {
		lines = strings.Split(existing, "\n")
	}
	prefix := key + "="
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			replaced = true
		}
	}
	if !replaced {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(path, []byte(out), 0o644)
}
