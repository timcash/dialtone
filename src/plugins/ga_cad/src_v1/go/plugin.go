package gacad

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run(command string, args []string) error {
	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "serve", "server":
		return runServe(args)
	case "status":
		return runStatus(args)
	case "stop":
		return runStop(args)
	default:
		printUsage()
		return fmt.Errorf("unknown ga_cad command: %s", command)
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh ga_cad src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  serve [--port <n>]   Start the GA CAD server")
	logs.Raw("  server [--port <n>]  Alias for serve")
	logs.Raw("  status [--port <n>]  Check local GA CAD server health")
	logs.Raw("  stop [--port <n>]    Stop the tracked local GA CAD server")
	logs.Raw("  install              Verify/install UI dependencies")
	logs.Raw("  build                Build the UI assets")
	logs.Raw("  format               Format Go and UI sources")
	logs.Raw("  lint                 Run Go and UI lint checks")
	logs.Raw("  help                 Show this help")
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("ga-cad-serve", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	port := fs.Int("port", 8082, "Port to listen on")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.StateDir, 0o755); err != nil {
		return err
	}

	logs.Info("DIALTONE_INDEX: ga_cad serve: checking ui bundle")
	logs.Info("DIALTONE_INDEX: ga_cad serve: starting server on 127.0.0.1:%d", *port)

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{Addr: addr, Handler: NewHandler(paths)}
	meta := serverState{
		PID:       os.Getpid(),
		Port:      *port,
		Listen:    "127.0.0.1:" + strconv.Itoa(*port),
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := writeServerState(paths, meta); err != nil {
		return err
	}
	defer removeServerState(paths)

	logs.Info("DIALTONE_INDEX: ga_cad serve: ready on 127.0.0.1:%d", *port)
	logs.Info("ga_cad src_v1 server listening on %s", addr)
	return srv.ListenAndServe()
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("ga-cad-status", flag.ContinueOnError)
	port := fs.Int("port", 8082, "Port to inspect")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad status: checking local server on 127.0.0.1:%d", *port)
	meta, hasMeta, err := readServerState(paths)
	if err != nil {
		return err
	}
	if hasMeta && meta.Port != *port {
		logs.Info("DIALTONE_INDEX: ga_cad status: tracked server is on 127.0.0.1:%d", meta.Port)
	}
	if hasMeta && isRunningPID(meta.PID) {
		ok, status := checkHealth(meta.Port)
		if ok {
			logs.Info("DIALTONE_INDEX: ga_cad status: server healthy on 127.0.0.1:%d", meta.Port)
			logs.Info("ga_cad src_v1 status: pid=%d started_at=%s health=%s", meta.PID, strings.TrimSpace(meta.StartedAt), status)
			return nil
		}
		return fmt.Errorf("ga_cad status health check failed for pid=%d port=%d: %s", meta.PID, meta.Port, status)
	}
	ok, status := checkHealth(*port)
	if ok {
		logs.Info("DIALTONE_INDEX: ga_cad status: server healthy on 127.0.0.1:%d", *port)
		logs.Info("ga_cad src_v1 status: health=%s", status)
		return nil
	}
	logs.Info("DIALTONE_INDEX: ga_cad status: no healthy server on 127.0.0.1:%d", *port)
	return fmt.Errorf("ga_cad server not running on 127.0.0.1:%d", *port)
}

func runStop(args []string) error {
	fs := flag.NewFlagSet("ga-cad-stop", flag.ContinueOnError)
	port := fs.Int("port", 8082, "Port to stop")
	if err := fs.Parse(args); err != nil {
		return err
	}

	paths, err := ResolvePaths("", "src_v1")
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad stop: checking for local server on 127.0.0.1:%d", *port)
	meta, hasMeta, err := readServerState(paths)
	if err != nil {
		return err
	}
	if !hasMeta {
		ok, _ := checkHealth(*port)
		if !ok {
			logs.Info("DIALTONE_INDEX: ga_cad stop: no tracked server was running")
			return nil
		}
		return fmt.Errorf("ga_cad stop: server responds on port %d but no tracked pid exists", *port)
	}
	if meta.Port != *port {
		return fmt.Errorf("ga_cad stop: tracked server is on port %d, not %d", meta.Port, *port)
	}
	if !isRunningPID(meta.PID) {
		removeServerState(paths)
		logs.Info("DIALTONE_INDEX: ga_cad stop: removed stale server state")
		return nil
	}

	proc, err := os.FindProcess(meta.PID)
	if err != nil {
		return err
	}
	logs.Info("DIALTONE_INDEX: ga_cad stop: stopping pid %d on 127.0.0.1:%d", meta.PID, meta.Port)
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !isRunningPID(meta.PID) {
			removeServerState(paths)
			logs.Info("DIALTONE_INDEX: ga_cad stop: server stopped")
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("ga_cad stop: pid %d did not exit in time", meta.PID)
}

func NewHandler(paths Paths) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	if stat, err := os.Stat(paths.UIDist); err == nil && stat.IsDir() {
		logs.Info("DIALTONE_INDEX: ga_cad serve: serving ui/dist from %s", paths.UIDist)
		mux.HandleFunc("/", makeStaticHandler(paths.UIDist))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "ga_cad src_v1 ui/dist not built", http.StatusServiceUnavailable)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			return
		}
		logs.Info("ga_cad serve: handling %s %s", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})
}

type serverState struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	Listen    string `json:"listen"`
	StartedAt string `json:"started_at"`
}

func writeServerState(paths Paths, meta serverState) error {
	if err := os.WriteFile(paths.ServerPID, []byte(strconv.Itoa(meta.PID)+"\n"), 0o644); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(paths.ServerMeta, append(raw, '\n'), 0o644)
}

func removeServerState(paths Paths) {
	_ = os.Remove(paths.ServerPID)
	_ = os.Remove(paths.ServerMeta)
}

func readServerState(paths Paths) (serverState, bool, error) {
	raw, err := os.ReadFile(paths.ServerMeta)
	if err != nil {
		if os.IsNotExist(err) {
			return serverState{}, false, nil
		}
		return serverState{}, false, err
	}
	var meta serverState
	if err := json.Unmarshal(raw, &meta); err != nil {
		return serverState{}, false, err
	}
	return meta, true, nil
}

func checkHealth(port int) (bool, string) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, resp.Status
	}
	return true, resp.Status
}

func isRunningPID(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func makeStaticHandler(root string) http.HandlerFunc {
	fs := http.FileServer(http.Dir(root))
	indexPath := root + "/index.html"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, indexPath)
			return
		}
		fs.ServeHTTP(w, r)
	}
}
