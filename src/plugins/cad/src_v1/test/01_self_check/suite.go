package selfcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	cadv1 "dialtone/dev/plugins/cad/src_v1/go"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name: "cad-self-check-object-creation-src-v1",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			gear := cadv1.NewGearObject(80.0, 20.0, 20)
			if gear.Type != "gear" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected gear type: %s", gear.Type)
			}
			if gear.Parameters["num_teeth"] != 20 {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected teeth count: %v", gear.Parameters["num_teeth"])
			}
			if gear.Parameters["outer_diameter"] != 80.0 {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected outer diameter: %v", gear.Parameters["outer_diameter"])
			}
			if _, err := gear.ToJSON(); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("gear json failed: %w", err)
			}
			ctx.Infof("cad-self-check-object-creation-src-v1-ok")
			return testv1.StepRunResult{Report: "cad object creation verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "cad-self-check-install-layout-src-v1",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			paths, err := cadv1.ResolvePaths("", "src_v1")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cadv1.VerifyInstallLayout(paths); err != nil {
				return testv1.StepRunResult{}, err
			}

			tmpRoot, err := os.MkdirTemp("", "cad-install-layout-*")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer os.RemoveAll(tmpRoot)

			fakeDialtoneEnv := filepath.Join(tmpRoot, "dialtone-env")
			fakePixi := filepath.Join(fakeDialtoneEnv, "pixi", "bin", "pixi")
			if err := os.MkdirAll(filepath.Dir(fakePixi), 0o755); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := os.WriteFile(fakePixi, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				return testv1.StepRunResult{}, err
			}

			fakePaths := paths
			fakePaths.Runtime = configv1.Runtime{
				RepoRoot:     paths.Runtime.RepoRoot,
				SrcRoot:      paths.Runtime.SrcRoot,
				EnvFile:      paths.Runtime.EnvFile,
				DialtoneHome: paths.Runtime.DialtoneHome,
				DialtoneEnv:  fakeDialtoneEnv,
				PixiBin:      fakePixi,
			}
			pixiBin, err := cadv1.ResolvePixiBinary(fakePaths)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("resolve managed pixi failed: %w", err)
			}
			if pixiBin != fakePixi {
				return testv1.StepRunResult{}, fmt.Errorf("resolve managed pixi mismatch: got %s want %s", pixiBin, fakePixi)
			}

			fakeBun := filepath.Join(fakeDialtoneEnv, "bun", "bin", "bun")
			if err := os.MkdirAll(filepath.Dir(fakeBun), 0o755); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := os.WriteFile(fakeBun, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
				return testv1.StepRunResult{}, err
			}
			fakePaths.Runtime = configv1.Runtime{
				RepoRoot:     paths.Runtime.RepoRoot,
				SrcRoot:      paths.Runtime.SrcRoot,
				EnvFile:      paths.Runtime.EnvFile,
				DialtoneHome: paths.Runtime.DialtoneHome,
				DialtoneEnv:  fakeDialtoneEnv,
				PixiBin:      fakePixi,
			}
			bunBin, err := cadv1.ResolveBunBinary(fakePaths)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("resolve bun fallback failed: %w", err)
			}
			if bunBin != fakeBun {
				return testv1.StepRunResult{}, fmt.Errorf("resolve bun fallback mismatch: got %s want %s", bunBin, fakeBun)
			}

			ctx.Infof("cad-self-check-install-layout-src-v1-ok")
			return testv1.StepRunResult{Report: "cad install layout and managed tool resolution verified"}, nil
		},
	})

	r.Add(testv1.Step{
		Name: "cad-http-api-src-v1",
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			paths, err := cadv1.ResolvePaths("", "src_v1")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			mux := http.NewServeMux()
			cadv1.RegisterHandlers(mux, paths)
			ts := httptest.NewServer(mux)
			defer ts.Close()

			params := map[string]interface{}{
				"num_teeth":      15,
				"outer_diameter": 60,
			}
			body, _ := json.Marshal(params)
			resp, err := http.Post(ts.URL+"/api/cad/generate", "application/json", bytes.NewBuffer(body))
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("POST /api/cad/generate failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return testv1.StepRunResult{}, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
			}
			if resp.Header.Get("Content-Type") != "application/sla" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
			}

			resp, err = http.Get(ts.URL + "/api/cad?num_teeth=15&outer_diameter=60")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("GET /api/cad failed: %v", err)
			}
			defer resp.Body.Close()

			var metadata struct {
				Type       string                 `json:"type"`
				Parameters map[string]interface{} `json:"parameters"`
				SourceCode string                 `json:"source_code"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("failed to decode metadata: %v", err)
			}
			if metadata.Type != "gear" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected type in metadata: %s", metadata.Type)
			}
			if metadata.Parameters["num_teeth"].(float64) != 15 {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected teeth in metadata: %v", metadata.Parameters["num_teeth"])
			}
			if len(metadata.SourceCode) < 100 {
				return testv1.StepRunResult{}, fmt.Errorf("source code looks too short or missing: %d bytes", len(metadata.SourceCode))
			}

			ctx.Infof("cad-http-api-src-v1-ok")
			return testv1.StepRunResult{Report: "cad HTTP API verified"}, nil
		},
	})
}
