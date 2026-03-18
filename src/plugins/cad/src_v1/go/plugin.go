package cad

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run(command string, args []string) error {
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "serve", "server":
		return runServe(args)
	default:
		printUsage()
		return fmt.Errorf("unknown cad command: %s", command)
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh cad src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  serve [--port <n>]   Start the CAD backend server")
	logs.Raw("  server [--port <n>]  Alias for serve")
	logs.Raw("  help                 Show this help")
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("cad-serve", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	port := fs.Int("port", 8081, "Port to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}

	logs.Info("DIALTONE_INDEX: cad serve: starting backend on 127.0.0.1:%d", *port)

	handler := NewHandler(paths)

	addr := fmt.Sprintf(":%d", *port)
	logs.Info("cad src_v1 server listening on %s", addr)
	return http.ListenAndServe(addr, handler)
}

func NewHandler(paths Paths) http.Handler {
	mux := http.NewServeMux()
	RegisterHandlers(mux, paths)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	if stat, err := os.Stat(paths.UIDist); err == nil && stat.IsDir() {
		logs.Info("DIALTONE_INDEX: cad serve: serving ui/dist from %s", paths.UIDist)
		mux.HandleFunc("/", makeStaticHandler(paths.UIDist))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "cad src_v1 ui/dist not built", http.StatusServiceUnavailable)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			return
		}

		logs.Info("cad serve: handling %s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})
}

func makeStaticHandler(uiDist string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		path := filepath.Join(uiDist, filepath.FromSlash(rel))
		if r.URL.Path == "/" {
			path = filepath.Join(uiDist, "index.html")
		}
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(uiDist, "index.html"))
	}
}
