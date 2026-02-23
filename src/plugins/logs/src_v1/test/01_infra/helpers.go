package infra

import (
	"fmt"
	"os"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func findRepoRoot() (string, error) {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return "", err
	}
	if rt.RepoRoot == "" {
		return "", fmt.Errorf("repo root not found")
	}
	return rt.RepoRoot, nil
}

func testReportPath(repoRoot string) string {
	paths, err := logs.ResolvePaths(repoRoot, "src_v1")
	if err != nil {
		return ""
	}
	return paths.TestReport
}

func waitForContains(path, pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), pattern) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q in %s", pattern, path)
}
