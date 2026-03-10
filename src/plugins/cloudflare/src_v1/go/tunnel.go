package cloudflare

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	}
	return strings.TrimSpace(os.Getenv("CF_TUNNEL_TOKEN"))
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
	fullHostname := fmt.Sprintf("%s.%s", name, domain)

	// 1) Resolve zone id.
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
		return ProvisionResult{}, fmt.Errorf("fetch zone failed: %w", err)
	}
	defer zoneResp.Body.Close()

	var zr zoneResponse
	if err := json.NewDecoder(zoneResp.Body).Decode(&zr); err != nil {
		return ProvisionResult{}, fmt.Errorf("decode zone response failed: %w", err)
	}
	if !zr.Success || len(zr.Result) == 0 {
		return ProvisionResult{}, fmt.Errorf("zone %s not found", domain)
	}
	zoneID := zr.Result[0].ID

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
		"name":    name,
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
		FullHostname: fullHostname,
		TunnelID:     tunnelID,
		EnvVarName:   envVar,
		RunToken:     runToken,
		DNSCreated:   dnsCreated,
	}, nil
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
