package cad

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
)

// CADObject represents a parametric 3D object.
type CADObject struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

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

func (o CADObject) ToJSON() (string, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func GenerateSTL(paths Paths, params map[string]interface{}) ([]byte, error) {
	pixiBin, err := ResolvePixiBinary(paths)
	if err != nil {
		return nil, err
	}

	args := []string{"run", "python", "main.py"}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--"+k, fmt.Sprintf("%v", params[k]))
	}

	cmd := exec.Command(pixiBin, args...)
	cmd.Dir = paths.BackendDir
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("python cli failed (pixi=%s): %s", pixiBin, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("pixi command failed (pixi=%s): %w", pixiBin, err)
	}
	return output, nil
}

func HandleGenerate(paths Paths) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var params map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		stl, err := GenerateSTL(paths, params)
		if err != nil {
			http.Error(w, fmt.Sprintf("Generation failed: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/sla")
		_, _ = w.Write(stl)
	}
}

func HandleMetadata(paths Paths) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		sourceCode, err := os.ReadFile(paths.BackendMain)
		if err != nil {
			sourceCode = []byte(fmt.Sprintf("# Error reading source: %v", err))
		}

		resp := map[string]interface{}{
			"type":        "gear",
			"parameters":  params,
			"source_code": string(sourceCode),
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func HandleDownload(paths Paths) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := make(map[string]interface{})
		query := r.URL.Query()
		for k := range query {
			params[k] = query.Get(k)
		}

		stl, err := GenerateSTL(paths, params)
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
		_, _ = w.Write(stl)
	}
}

func RegisterHandlers(mux *http.ServeMux, paths Paths) {
	mux.HandleFunc("/api/cad/generate", HandleGenerate(paths))
	mux.HandleFunc("/api/cad", HandleMetadata(paths))
	mux.HandleFunc("/api/cad/download", HandleDownload(paths))
}
