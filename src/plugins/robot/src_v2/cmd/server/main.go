package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	listen := flag.String("listen", envOrDefault("ROBOT_V2_LISTEN", ":8080"), "HTTP listen address")
	uiDist := flag.String("ui-dist", envOrDefault("ROBOT_V2_UI_DIST", ""), "Path to robot src_v2 ui/dist")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(*uiDist) == "" {
			http.Error(w, "robot src_v2 server scaffold active; ui/dist not configured", http.StatusServiceUnavailable)
			return
		}
		index := filepath.Join(*uiDist, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, fmt.Sprintf("ui index missing at %s", index), http.StatusServiceUnavailable)
			return
		}
		http.FileServer(http.Dir(*uiDist)).ServeHTTP(w, r)
	})

	fmt.Printf("robot src_v2 scaffold server listening on %s\n", *listen)
	if err := http.ListenAndServe(*listen, mux); err != nil {
		fmt.Fprintf(os.Stderr, "robot src_v2 server failed: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
