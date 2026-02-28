package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	earthv1 "dialtone/dev/plugins/earth/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	addr := flag.String("addr", ":8891", "listen address")
	flag.Parse()

	paths, err := earthv1.ResolvePaths("")
	if err != nil {
		logs.Error("resolve paths: %v", err)
		os.Exit(1)
	}
	dist := paths.Preset.UIDist
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		logs.Error("earth ui dist missing at %s (run ./dialtone.sh earth src_v1 build)", dist)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(dist))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(dist, "index.html"))
			return
		}
		if _, err := os.Stat(filepath.Join(dist, filepath.Clean(r.URL.Path))); err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	}))

	logs.Info("earth src_v1 serving %s on %s", dist, *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}
