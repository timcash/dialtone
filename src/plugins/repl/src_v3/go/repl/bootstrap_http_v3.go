package repl

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunBootstrapHTTP(args []string) error {
	fs := flag.NewFlagSet("repl-v3-bootstrap-http", flag.ContinueOnError)
	bindHost := fs.String("host", "127.0.0.1", "Bind host")
	port := fs.Int("port", 8811, "Bind port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return err
	}
	tmpRoot, err := os.MkdirTemp("", "dialtone-repl-v3-http-*")
	if err != nil {
		return err
	}
	repoTar := filepath.Join(tmpRoot, "dialtone-main.tar.gz")
	if err := createRepoTarball(repoRoot, repoTar); err != nil {
		return err
	}
	srcDialtone := filepath.Join(repoRoot, "dialtone.sh")
	if _, err := os.Stat(srcDialtone); err != nil {
		return fmt.Errorf("dialtone.sh not found at %s", srcDialtone)
	}
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(*bindHost), *port)
	srv, err := startBootstrapServerAtAddress(addr, repoTar, srcDialtone, "shell.dialtone.earth", false)
	if err != nil {
		return err
	}
	if err := persistBootstrapHTTPConfig(strings.TrimSpace(*bindHost), *port); err != nil {
		return err
	}
	logs.Info("bootstrap HTTP server active at http://%s", addr)
	logs.Info("  /install.sh")
	logs.Info("  /dialtone.sh")
	logs.Info("  /dialtone-main.tar.gz")
	logs.Info("Cloudflare example:")
	logs.Info("  ./dialtone.sh cloudflare src_v1 tunnel start shell --url http://127.0.0.1:%d", *port)
	logs.Info("Remote bootstrap example:")
	logs.Info("  curl -fsSL https://shell.dialtone.earth/install.sh | bash -s -- repl src_v3 test")
	return srv.ListenAndServe()
}

func startLocalBootstrapServer(tarPath, dialtonePath string) (string, int, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", 0, nil, err
	}
	_, portText, splitErr := net.SplitHostPort(ln.Addr().String())
	if splitErr != nil {
		_ = ln.Close()
		return "", 0, nil, splitErr
	}
	port := 0
	if _, scanErr := fmt.Sscanf(portText, "%d", &port); scanErr != nil {
		_ = ln.Close()
		return "", 0, nil, scanErr
	}
	dialtoneURL := fmt.Sprintf("http://shell.dialtone.earth:%d/dialtone.sh", port)
	repoURL := fmt.Sprintf("http://127.0.0.1:%d/dialtone-main.tar.gz", port)
	mux := http.NewServeMux()
	mux.HandleFunc("/dialtone-main.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, tarPath)
	})
	mux.HandleFunc("/dialtone.sh", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, dialtonePath)
	})
	mux.HandleFunc("/install.sh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-shellscript")
		version := strconv.FormatInt(time.Now().UnixNano(), 10)
		dialtoneURLVersioned := dialtoneURL + "?v=" + version
		repoURLVersioned := repoURL + "?v=" + version
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
curl -fsSL --resolve shell.dialtone.earth:%d:127.0.0.1 %s -o dialtone.sh
chmod +x dialtone.sh
export DIALTONE_BOOTSTRAP_REPO_URL=%s
exec ./dialtone.sh "$@"
`, port, dialtoneURLVersioned, repoURLVersioned)
		_, _ = io.WriteString(w, script)
	})
	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()
	closeFn := func() {
		_ = srv.Close()
		_ = ln.Close()
	}
	baseURL := "http://" + ln.Addr().String()
	return baseURL, port, closeFn, nil
}

func startBootstrapServerAtAddress(addr, tarPath, dialtonePath, publicHost string, useResolve bool) (*http.Server, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("bootstrap server address is required")
	}
	publicHost = strings.TrimSpace(publicHost)
	if publicHost == "" {
		publicHost = "shell.dialtone.earth"
	}
	_, portText, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return nil, splitErr
	}
	port := 0
	if _, scanErr := fmt.Sscanf(portText, "%d", &port); scanErr != nil {
		return nil, scanErr
	}
	dialtoneURL := fmt.Sprintf("https://%s/dialtone.sh", publicHost)
	repoURL := fmt.Sprintf("https://%s/dialtone-main.tar.gz", publicHost)
	if useResolve {
		dialtoneURL = fmt.Sprintf("http://%s:%d/dialtone.sh", publicHost, port)
		repoURL = fmt.Sprintf("http://%s:%d/dialtone-main.tar.gz", publicHost, port)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/dialtone-main.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, tarPath)
	})
	mux.HandleFunc("/dialtone.sh", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, dialtonePath)
	})
	mux.HandleFunc("/install.sh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/x-shellscript")
		version := strconv.FormatInt(time.Now().UnixNano(), 10)
		dialtoneURLVersioned := dialtoneURL + "?v=" + version
		repoURLVersioned := repoURL + "?v=" + version
		curlLine := fmt.Sprintf("curl -fsSL %s -o dialtone.sh", dialtoneURL)
		if useResolve {
			curlLine = fmt.Sprintf("curl -fsSL --resolve %s:%d:127.0.0.1 %s -o dialtone.sh", publicHost, port, dialtoneURLVersioned)
		} else {
			curlLine = fmt.Sprintf("curl -fsSL %s -o dialtone.sh", dialtoneURLVersioned)
		}
		script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
%s
chmod +x dialtone.sh
export DIALTONE_BOOTSTRAP_REPO_URL=%s
exec ./dialtone.sh "$@"
`, curlLine, repoURLVersioned)
		_, _ = io.WriteString(w, script)
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	return srv, nil
}

func EnsureBootstrapHTTPRunning(host string, port int) error {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 8811
	}
	if bootstrapHTTPEndpointReachable(host, port, 700*time.Millisecond) {
		_ = persistBootstrapHTTPConfig(host, port)
		return nil
	}
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "bootstrap-http",
		"--host", host,
		"--port", strconv.Itoa(port),
	)
	cmd.Dir = srcRoot
	cmd.Env = append(os.Environ(),
		"DIALTONE_REPO_ROOT="+repoRoot,
		"DIALTONE_SRC_ROOT="+srcRoot,
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if bootstrapHTTPEndpointReachable(host, port, 700*time.Millisecond) {
			_ = persistBootstrapHTTPConfig(host, port)
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("repl v3 bootstrap HTTP server did not start at http://%s:%d/install.sh", host, port)
}

func persistBootstrapHTTPConfig(host string, port int) error {
	if port <= 0 {
		return fmt.Errorf("invalid bootstrap http port: %d", port)
	}
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	doc["DIALTONE_BOOTSTRAP_HTTP_HOST"] = strings.TrimSpace(host)
	doc["DIALTONE_BOOTSTRAP_HTTP_PORT"] = strconv.Itoa(port)
	doc["DIALTONE_BOOTSTRAP_HTTP_URL"] = fmt.Sprintf("http://%s:%d", strings.TrimSpace(host), port)
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return os.WriteFile(cfgPath, out, 0o644)
}

func bootstrapHTTPEndpointReachable(host string, port int, timeout time.Duration) bool {
	client := &http.Client{Timeout: timeout}
	u := fmt.Sprintf("http://%s:%d/install.sh", host, port)
	resp, err := client.Get(u)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
