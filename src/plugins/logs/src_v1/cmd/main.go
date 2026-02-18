package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	port := "8080"
	cwd, _ := os.Getwd()
	uiPath := filepath.Join(cwd, "ui", "dist")
	if _, err := os.Stat(uiPath); err != nil {
		uiPath = filepath.Join(cwd, "src", "plugins", "logs", "src_v1", "ui", "dist")
	}
	logPath := resolveTestLogPath(cwd)

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	http.HandleFunc("/api/test-log", func(w http.ResponseWriter, r *http.Request) {
		offset := int64(0)
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed >= 0 {
				offset = parsed
			}
		}
		nextOffset, lines, err := readLogDelta(logPath, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"offset": nextOffset,
			"lines":  lines,
		})
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rel := r.URL.Path
		if len(rel) > 0 && rel[0] == '/' {
			rel = rel[1:]
		}
		path := filepath.Join(uiPath, rel)
		if r.URL.Path == "/" {
			path = filepath.Join(uiPath, "index.html")
		}
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(uiPath, "index.html")
		}
		http.ServeFile(w, r, path)
	})

	fmt.Printf("Logs Server starting on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}

func resolveTestLogPath(cwd string) string {
	local := filepath.Join(cwd, "test", "test.log")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return filepath.Join(cwd, "src", "plugins", "logs", "src_v1", "test", "test.log")
}

func readLogDelta(path string, offset int64) (int64, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, []string{}, nil
		}
		return offset, nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return offset, nil, err
	}
	size := info.Size()
	if offset > size {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return offset, nil, err
	}
	next := offset + int64(len(data))
	if len(data) == 0 {
		return next, []string{}, nil
	}
	chunks := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c == "" {
			continue
		}
		lines = append(lines, c)
	}
	return next, lines, nil
}
