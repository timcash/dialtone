package improve_cli_build_commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dialtone "dialtone/cli/src"
)

// TestWebBuildDirectoryHasContent verifies web_build has actual content
func TestWebBuildDirectoryHasContent(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	projectRoot := filepath.Join(cwd, "..", "..")
	webBuildDir := filepath.Join(projectRoot, "src", "web_build")

	dialtone.LogInfo("Checking web_build directory: %s", webBuildDir)

	// Check if directory exists
	info, err := os.Stat(webBuildDir)
	if os.IsNotExist(err) {
		t.Fatalf("web_build directory does not exist: %s", webBuildDir)
	}
	if !info.IsDir() {
		t.Fatalf("web_build is not a directory: %s", webBuildDir)
	}

	// Check for index.html
	indexPath := filepath.Join(webBuildDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("index.html not found in web_build - run 'dialtone build --full' or 'npm run build' in src/web")
		dialtone.LogInfo("MISSING: %s", indexPath)
	} else {
		// Check file has content
		content, err := os.ReadFile(indexPath)
		if err != nil {
			t.Errorf("Failed to read index.html: %v", err)
		} else if len(content) < 100 {
			t.Errorf("index.html appears to be a placeholder (only %d bytes)", len(content))
			dialtone.LogInfo("index.html content: %s", string(content))
		} else {
			dialtone.LogInfo("index.html found with %d bytes", len(content))
		}
	}

	// List all files in web_build
	entries, err := os.ReadDir(webBuildDir)
	if err != nil {
		t.Fatalf("Failed to read web_build directory: %v", err)
	}

	dialtone.LogInfo("Files in web_build:")
	fileCount := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") { // Skip hidden files
			dialtone.LogInfo("  - %s", entry.Name())
			fileCount++
		}
	}

	if fileCount == 0 {
		t.Errorf("web_build directory is empty (no non-hidden files)")
	}
}

// TestAPIInitEndpoint tests the /api/init endpoint
func TestAPIInitEndpoint(t *testing.T) {
	baseURL := getTestServerURL()
	if baseURL == "" {
		t.Skip("No test server URL available - set DIALTONE_TEST_URL or start server first")
	}

	url := baseURL + "/api/init"
	dialtone.LogInfo("Testing API endpoint: %s", url)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Failed to connect to %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	dialtone.LogInfo("Response: %s", string(body))

	// Parse JSON response
	var initData map[string]interface{}
	if err := json.Unmarshal(body, &initData); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	// Check expected fields
	expectedFields := []string{"hostname", "nats_port", "ws_port", "web_port"}
	for _, field := range expectedFields {
		if _, ok := initData[field]; !ok {
			t.Errorf("Missing expected field in /api/init response: %s", field)
		}
	}
}

// TestAPIStatusEndpoint tests the /api/status endpoint
func TestAPIStatusEndpoint(t *testing.T) {
	baseURL := getTestServerURL()
	if baseURL == "" {
		t.Skip("No test server URL available - set DIALTONE_TEST_URL or start server first")
	}

	url := baseURL + "/api/status"
	dialtone.LogInfo("Testing API endpoint: %s", url)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Failed to connect to %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	dialtone.LogInfo("Response: %s", string(body))

	// Parse JSON response
	var statusData map[string]interface{}
	if err := json.Unmarshal(body, &statusData); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	// Check expected fields
	expectedFields := []string{"hostname", "uptime", "platform", "arch", "nats"}
	for _, field := range expectedFields {
		if _, ok := statusData[field]; !ok {
			t.Errorf("Missing expected field in /api/status response: %s", field)
		}
	}
}

// TestRootPathServesContent tests that / returns content (not 404)
func TestRootPathServesContent(t *testing.T) {
	baseURL := getTestServerURL()
	if baseURL == "" {
		t.Skip("No test server URL available - set DIALTONE_TEST_URL or start server first")
	}

	url := baseURL + "/"
	dialtone.LogInfo("Testing root path: %s", url)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("Failed to connect to %s: %v", url, err)
	}
	defer resp.Body.Close()

	dialtone.LogInfo("Root path status code: %d", resp.StatusCode)
	dialtone.LogInfo("Content-Type: %s", resp.Header.Get("Content-Type"))

	if resp.StatusCode == http.StatusNotFound {
		t.Errorf("Root path returned 404 - web_build likely missing index.html")
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	dialtone.LogInfo("Response body length: %d bytes", len(body))

	if len(body) < 100 {
		t.Errorf("Root path returned very short content (%d bytes) - likely placeholder", len(body))
		dialtone.LogInfo("Body: %s", string(body))
	}

	// Check if it looks like HTML
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "<!DOCTYPE html>") && !strings.Contains(bodyStr, "<html") {
		t.Errorf("Root path did not return HTML content")
	}
}

// getTestServerURL returns the URL of the running test server
func getTestServerURL() string {
	// Check environment variable first
	if url := os.Getenv("DIALTONE_TEST_URL"); url != "" {
		return url
	}

	// Try common local URLs
	testURLs := []string{
		"http://100.77.155.74:80",      // Tailscale IP from earlier
		"http://localhost:80",
		"http://127.0.0.1:80",
	}

	client := &http.Client{Timeout: 2 * time.Second}
	for _, url := range testURLs {
		resp, err := client.Get(url + "/api/init")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("Found test server at: %s\n", url)
				return url
			}
		}
	}

	return ""
}
