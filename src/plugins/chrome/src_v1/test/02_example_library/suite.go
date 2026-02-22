package examplelibrary

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	chrome "dialtone/dev/plugins/chrome/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:           "example-library-metadata-and-helpers",
		Timeout:        20 * time.Second,
		RunWithContext: runExampleLibrary,
	})
}

func runExampleLibrary(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
	if chrome.FindChromePath() == "" {
		return testv1.StepRunResult{}, fmt.Errorf("chrome binary not found")
	}

	session := &chrome.Session{
		PID:          1234,
		Port:         9222,
		WebSocketURL: "ws://127.0.0.1:9222/devtools/browser/example",
		IsNew:        true,
	}
	meta := chrome.BuildSessionMetadata(session)
	if meta == nil {
		return testv1.StepRunResult{}, fmt.Errorf("expected session metadata")
	}
	if !strings.Contains(meta.DebugURL, "http://127.0.0.1:9222/devtools/browser/example") {
		return testv1.StepRunResult{}, fmt.Errorf("unexpected debug url: %s", meta.DebugURL)
	}

	out := filepath.Join(".chrome_data", fmt.Sprintf("meta-%d.json", time.Now().UnixNano()))
	if err := chrome.WriteSessionMetadata(out, session); err != nil {
		return testv1.StepRunResult{}, fmt.Errorf("write session metadata: %w", err)
	}
	ctx.Infof("wrote metadata file: %s", out)
	return testv1.StepRunResult{Report: "library metadata helpers validated"}, nil
}
