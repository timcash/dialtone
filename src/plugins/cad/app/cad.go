package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

// CADObject represents a parametric 3D object
type CADObject struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewGearObject creates a new parametric gear object matching gear_generator.py logic
func NewGearObject(outer float64, inner float64, teeth int) CADObject {
	return CADObject{
		Type: "gear",
		Parameters: map[string]interface{}{
			"outer_diameter":         outer,
			"inner_diameter":         inner,
			"num_teeth":              teeth,
			"thickness":              8.0,
			"tooth_height":           6.0,
			"tooth_width":            4.0,
			"num_mounting_holes":     4,
			"mounting_hole_diameter": 6.0,
		},
	}
}

// ToJSON converts the CAD object to a JSON string
func (o CADObject) ToJSON() (string, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GenerateSTL calls the Python CLI to generate STL data
func GenerateSTL(params map[string]interface{}) ([]byte, error) {
	args := []string{"run", "python", "main.py"}
	
	for k, v := range params {
		args = append(args, "--"+k, fmt.Sprintf("%v", v))
	}

	cmd := exec.Command("pixi", args...)
	cmd.Dir = "src/plugins/cad/backend"
	
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("python cli failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	
	return output, nil
}

// HandleGenerate handles STL generation requests
func HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var params map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	stl, err := GenerateSTL(params)
	if err != nil {
		http.Error(w, fmt.Sprintf("Generation failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/sla")
	w.Write(stl)
}

// HandleMetadata returns CAD object metadata and source code
func HandleMetadata(w http.ResponseWriter, r *http.Request) {
	// Parse parameters from query string
	params := make(map[string]interface{})
	query := r.URL.Query()
	for k := range query {
		val := query.Get(k)
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			params[k] = f
		} else if i, err := strconv.Atoi(val); err == nil {
			params[k] = i
		} else {
			params[k] = val
		}
	}

	// Read source code
	sourcePath := "src/plugins/cad/backend/main.py"
	sourceCode, err := os.ReadFile(sourcePath)
	if err != nil {
		sourceCode = []byte(fmt.Sprintf("# Error reading source: %v", err))
	}

	resp := map[string]interface{}{
		"type":        "gear",
		"parameters":  params,
		"source_code": string(sourceCode),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleDownload handles STL download requests
func HandleDownload(w http.ResponseWriter, r *http.Request) {
	params := make(map[string]interface{})
	query := r.URL.Query()
	for k := range query {
		params[k] = query.Get(k)
	}

	stl, err := GenerateSTL(params)
	if err != nil {
		http.Error(w, fmt.Sprintf("Generation failed: %v", err), http.StatusInternalServerError)
		return
	}

	numTeeth := query.Get("num_teeth")
	if numTeeth == "" {
		numTeeth = "unknown"
	}

	w.Header().Set("Content-Type", "application/sla")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=gear_%st.stl", numTeeth))
	w.Write(stl)
}

// RegisterHandlers registers the CAD API handlers to the given mux
func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/cad/generate", HandleGenerate)
	mux.HandleFunc("/api/cad", HandleMetadata)
	mux.HandleFunc("/api/cad/download", HandleDownload)
}
