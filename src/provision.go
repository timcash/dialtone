package dialtone

import (
	"bytes"
	"encoding/json"
	"flag"
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
			LogInfo("TS_API_KEY not found, skipping provisioning.")
			return
		}
		LogFatal("Error: --api-key flag or TS_API_KEY environment variable is required.")
	}

	provisionKey(token)
}

func provisionKey(token string) {
	LogInfo("Generating new Tailscale Auth Key...")

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
		LogFatal("Failed to create request: %v", err)
	}

	req.SetBasicAuth(token, "")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		LogFatal("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		LogFatal("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Key string `json:"key"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	LogInfo("Successfully generated key: %s...", result.Key[:10])
	updateEnv("TS_AUTHKEY", result.Key)
	LogInfo("Updated .env with new TS_AUTHKEY.")
}

func updateEnv(key, value string) {
	// Update current process environment
	os.Setenv(key, value)

	envFile := ".env"
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
