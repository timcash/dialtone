package test

import (
	"bytes"
	"dialtone/dev/core/logger"
	"dialtone/dev/core/test"
	"dialtone/dev/plugins/cad/app"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
)

func init() {
	test.Register("cad-object-creation", "cad", []string{"plugin", "cad"}, RunCADObjectTest)
	test.Register("cad-http-api", "cad", []string{"plugin", "cad"}, RunCADHTTPSTest)
}

// RunAll is the standard entry point required by project rules.
func RunAll() error {
	logger.LogInfo("Running cad plugin suite...")
	return test.RunPlugin("cad")
}

func RunCADObjectTest() error {
	gear := app.NewGearObject(80.0, 20.0, 20)
	if gear.Type != "gear" {
		return fmt.Errorf("unexpected gear type: %s", gear.Type)
	}
	if gear.Parameters["num_teeth"] != 20 {
		return fmt.Errorf("unexpected teeth count: %v", gear.Parameters["num_teeth"])
	}
	if gear.Parameters["outer_diameter"] != 80.0 {
		return fmt.Errorf("unexpected outer diameter: %v", gear.Parameters["outer_diameter"])
	}
	fmt.Println("PASS: [cad] Plugin logic verified")
	return nil
}

func RunCADHTTPSTest() error {
	fmt.Println(">> [cad] Running HTTP API Verification...")

	mux := http.NewServeMux()
	app.RegisterHandlers(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Test POST /api/cad/generate
	fmt.Println("   Checking /api/cad/generate...")
	params := map[string]interface{}{
		"num_teeth":      15,
		"outer_diameter": 60,
	}
	body, _ := json.Marshal(params)
	resp, err := http.Post(ts.URL+"/api/cad/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("POST /api/cad/generate failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/sla" {
		return fmt.Errorf("unexpected content type: %s", resp.Header.Get("Content-Type"))
	}
	fmt.Println("   [PASS] STL generation successful over HTTP")

	// 2. Test GET /api/cad (Metadata & Source)
	fmt.Println("   Checking /api/cad (Metadata)...")
	resp, err = http.Get(ts.URL + "/api/cad?num_teeth=15&outer_diameter=60")
	if err != nil {
		return fmt.Errorf("GET /api/cad failed: %v", err)
	}
	defer resp.Body.Close()

	var metadata struct {
		Type       string                 `json:"type"`
		Parameters map[string]interface{} `json:"parameters"`
		SourceCode string                 `json:"source_code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return fmt.Errorf("failed to decode metadata: %v", err)
	}

	if metadata.Type != "gear" {
		return fmt.Errorf("unexpected type in metadata: %s", metadata.Type)
	}
	// Note: strconv.ParseFloat/Atoi in cad.go handles types, but json.Unmarshal uses float64 for all numbers by default
	if metadata.Parameters["num_teeth"].(float64) != 15 {
		return fmt.Errorf("unexpected teeth in metadata: %v", metadata.Parameters["num_teeth"])
	}
	if len(metadata.SourceCode) < 100 {
		return fmt.Errorf("source code looks too short or missing: %d bytes", len(metadata.SourceCode))
	}
	fmt.Println("   [PASS] Metadata API verified (matches cad.ts expectations)")

	fmt.Println("PASS: [cad] HTTP API verified")
	return nil
}
