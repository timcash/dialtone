package cli

import (
	"bytes"
	"dialtone/cli/src/core/logger"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func RunProvision(args []string) {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	apiKey := fs.String("api-key", "", "Tailscale API Access Token")
	optional := fs.Bool("optional", false, "Skip instead of failing if TS_API_KEY is missing")
	fs.Parse(args)

	token := *apiKey
	if token == "" {
		token = os.Getenv("TS_API_KEY")
	}

	if token == "" {
		if *optional {
			logger.LogInfo("TS_API_KEY not found, skipping provisioning.")
			return
		}
		logger.LogFatal("Error: --api-key flag or TS_API_KEY environment variable is required.")
	}

	key, err := ProvisionKey(token, true)
	if err != nil {
		logger.LogFatal("Failed to provision key: %v", err)
	}

	logger.LogInfo("Successfully generated key: %s...", key[:10])
	updateEnv("TS_AUTHKEY", key)
	logger.LogInfo("Updated .env with new TS_AUTHKEY.")
}

// ProvisionKey generates a new auth key using the Tailscale API.
// If updateEnvFile is true, it also updates the local .env file.
func ProvisionKey(token string, updateEnvFile bool) (string, error) {
	logger.LogInfo("Generating new Tailscale Auth Key...")

	url := "https://api.tailscale.com/api/v2/tailnet/-/keys"
	payload := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"devices": map[string]interface{}{
				"create": map[string]interface{}{
					"reusable":      true,
					"ephemeral":     false,
					"preauthorized": true,
				},
			},
		},
		"expirySeconds": 86400,
		"description":   "Dialtone Auto-Provisioned Key",
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Key, nil
}

func updateEnv(key, value string) {
	// Update current process environment
	os.Setenv(key, value)

	envFile := "env/.env"
	content, _ := os.ReadFile(envFile)
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	_ = os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644)
}
