package main

import (
	"flag"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18741", "listen address")
	root := flag.String("root", ".", "directory to serve")
	flag.Parse()

	handler, err := newStaticHandler(*root)
	if err != nil {
		log.Fatalf("ui_server init failed: %v", err)
	}

	log.Printf("ui_server serving %s on http://%s", *root, *addr)
	server := &http.Server{
		Addr:    *addr,
		Handler: handler,
	}
	log.Fatal(server.ListenAndServe())
}

func newStaticHandler(root string) (http.Handler, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	indexPath := filepath.Join(absRoot, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := filepath.Clean("/" + strings.TrimSpace(r.URL.Path))
		target := filepath.Join(absRoot, cleanPath)
		if rel, err := filepath.Rel(absRoot, target); err != nil || strings.HasPrefix(rel, "..") {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(target)
		switch {
		case err == nil && info.IsDir():
			target = filepath.Join(target, "index.html")
		case err != nil || info.IsDir():
			target = indexPath
		}

		if _, err := os.Stat(target); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if ext := filepath.Ext(target); ext != "" {
			if ctype := mime.TypeByExtension(ext); ctype != "" {
				w.Header().Set("Content-Type", ctype)
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, target)
	}), nil
}
