package test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestE2E_WwwHelp(t *testing.T) {
	// Need to find dialtone.sh. 
	// specific path depending on where test runs.
	// Usually tests run in their directory.
	// src/plugins/www/test -> ../../../../dialtone.sh
	
	wd, _ := os.Getwd()
	dialtoneSh := filepath.Join(wd, "../../../../dialtone.sh")
	projectRoot := filepath.Join(wd, "../../../../")

	cmd := exec.Command(dialtoneSh, "www", "--help")
	cmd.Dir = projectRoot // Run from project root so dialtone-dev.go is found
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run dialtone.sh www --help: %v\nOutput: %s", err, output)
	}
	
	outStr := string(output)
	if !strings.Contains(outStr, "Usage: dialtone-dev www") {
		t.Errorf("Expected usage info, got: %s", outStr)
	}
}

func TestE2E_WwwLogsNoArgs(t *testing.T) {
	wd, _ := os.Getwd()
	dialtoneSh := filepath.Join(wd, "../../../../dialtone.sh")
	projectRoot := filepath.Join(wd, "../../../../")

	cmd := exec.Command(dialtoneSh, "www", "logs")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	
	// Expect failure (exit status 1)
	if err == nil {
		t.Error("Expected error when running logs without args, but got success")
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Usage: dialtone-dev www logs") {
		t.Errorf("Expected usage error message, got: %s", outStr)
	}
}

func TestE2E_PublishAndVerify(t *testing.T) {
	// 1. Setup
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "../../../../")
	dialtoneSh := filepath.Join(projectRoot, "dialtone.sh")
	
	// Skip if VERCEL_TOKEN not set (likely in CI without secrets, or just local run without auth)
	// We assume user has auth for this test request.
	
	// 2. Modify version in page.tsx
	pagePath := filepath.Join(projectRoot, "src/plugins/www/app/app/page.tsx")
	originalContent, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatalf("Failed to read page.tsx: %v", err)
	}
	
	newVersion := fmt.Sprintf("v1.0.1-test-%d", time.Now().Unix())
	t.Logf("Updating version to: %s", newVersion)
	
	// Replace version string using regex
	re := regexp.MustCompile(`v[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9-]+)?`)
	newContent := re.ReplaceAll(originalContent, []byte(newVersion))
	
	if err := os.WriteFile(pagePath, newContent, 0644); err != nil {
		t.Fatalf("Failed to write page.tsx: %v", err)
	}
	
	// Ensure we revert changes even if test fails
	defer func() {
		os.WriteFile(pagePath, originalContent, 0644)
	}()

	// 3. Publish
	// Note: using 'publish' (prod) might be slow. We'll use 'publish' as requested.
	t.Log("Running dialtone.sh www publish --yes...")
	cmd := exec.Command(dialtoneSh, "www", "publish", "--yes")
	cmd.Dir = projectRoot
	// We need to capture stdout to get the URL
	output, err := cmd.CombinedOutput()
	outStr := string(output)
	
	if err != nil {
		t.Fatalf("Publish failed: %v\nOutput: %s", err, outStr)
	}
	
	// 4. Extract Deployment URL
	// Vercel CLI usually outputs the URL. 
	// The output might contain ANSI codes or extra text.
	// We look for https://dialtone-earth-git-*-timcashs-projects.vercel.app or similar
	// Or just the main production URL if it was a prod deploy: https://dialtone.earth
	// If it was a preview, it's specific.
	// `www publish` runs `vercel deploy --prod`, so it deploys to the production domain.
	// However, it also prints the specific deployment URL.
	
	// Regex to find a vercel app URL
	// Example: https://dialtone-earth-789xyz.vercel.app
	urlRe := regexp.MustCompile(`https://[a-zA-Z0-9-]+\.vercel\.app`)
	urls := urlRe.FindAllString(outStr, -1)
	
	var targetURL string
	if len(urls) > 0 {
		targetURL = urls[0] 
		// Ideally we pick the one that looks like a deployment alias or the main one.
		// For verification of *this* build, we ideally want the specific deployment ID URL to avoid caching issues,
		// but --prod usually implies the main domain.
		// Let's rely on finding *any* valid URL in the output and checking it.
		// If multiple, maybe check the last one?
		targetURL = urls[len(urls)-1]
	} else {
		// Fallback to main domain if not found (though risky if not updated yet)
		targetURL = "https://dialtone.earth" 
	}
	
	t.Logf("Verifying deployment at: %s", targetURL)
	
	// 5. Verify with net/http (Chromedp failed due to missing Chrome binary in this env)
	// We wait a bit for propagation if needed, though Vercel is usually instant after deploy command returns.
	
	// Retry loop for robustness
	var bodyBytes []byte
	var res string // Declare outside loop
	success := false
	for i := 0; i < 5; i++ {
		t.Logf("Attempt %d: Fetching %s...", i+1, targetURL)
		resp, err := http.Get(targetURL)
		if err != nil {
			t.Logf("Fetch failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		defer resp.Body.Close()
		
		bodyBytes, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Logf("Read body failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		
		res = string(bodyBytes)
		if strings.Contains(res, newVersion) {
			success = true
			break
		}
		
		t.Logf("Version not found yet. waiting...")
		time.Sleep(5 * time.Second)
	}

	if !success {
		t.Errorf("Deployment verification failed!\nExpected version: %s\nFound response (partial): %s", newVersion, res[:min(len(res), 500)])
	} else {
		t.Log("Verification successful! Version found on live site.")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
