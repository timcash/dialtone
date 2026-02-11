package wsl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type InstanceInfo struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Version   string `json:"version"`
	Memory    string `json:"memory"`
	Disk      string `json:"disk"`
	Persisted bool   `json:"persisted"`
}

type WslPlugin struct {
	Addr    string
	mu      sync.Mutex
	clients map[*websocket.Conn]bool
}

func NewWslPlugin(addr string) *WslPlugin {
	return &WslPlugin{
		Addr:    addr,
		clients: make(map[*websocket.Conn]bool),
	}
}

// wslExec runs a wsl.exe command with a 30-second timeout. Logs the command and result.
func wslExec(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	log.Printf("[WSL] exec: wsl.exe %s", strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, "wsl.exe", args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[WSL] exec TIMEOUT (30s): wsl.exe %s", strings.Join(args, " "))
		return out, fmt.Errorf("wsl.exe %s timed out after 30s", strings.Join(args, " "))
	}
	if err != nil {
		log.Printf("[WSL] exec ERROR: wsl.exe %s -> %v (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	} else {
		log.Printf("[WSL] exec OK: wsl.exe %s (%d bytes output)", strings.Join(args, " "), len(out))
	}
	return out, err
}

func (p *WslPlugin) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "Online"})
	})

	mux.HandleFunc("/ws", p.handleWebSocket)

	mux.HandleFunc("/api/instances", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[WSL] API: %s /api/instances", r.Method)
		if r.Method == http.MethodPost {
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				log.Printf("[WSL] API Error: failed to decode create request: %v", err)
				http.Error(w, err.Error(), 400)
				return
			}
			log.Printf("[WSL] API: triggering creation of %s", req.Name)
			go p.createInstance(req.Name)
			w.WriteHeader(http.StatusAccepted)
			return
		}

		instances, err := p.listInstances()
		if err != nil {
			log.Printf("[WSL] API Error: failed to list instances: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		log.Printf("[WSL] API: returning %d instances", len(instances))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(instances)
	})

	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		go p.stopInstance(name)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		go p.deleteInstance(name)
		w.WriteHeader(http.StatusAccepted)
	})

	// Smart UI Path detection
	workDir, _ := os.Getwd()
	possiblePaths := []string{
		filepath.Join(workDir, "ui/dist"),
		filepath.Join(workDir, "src/plugins/wsl/src_v1/ui/dist"),
	}

	var uiPath string
	for _, path := range possiblePaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			uiPath = path
			break
		}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if uiPath == "" {
			fmt.Fprintf(w, "UI not found. Searching in: %v", possiblePaths)
			return
		}
		path := filepath.Join(uiPath, r.URL.Path)
		if r.URL.Path == "/" {
			path = filepath.Join(uiPath, "index.html")
		}
		http.ServeFile(w, r, path)
	})

	// Start telemetry loop
	go p.telemetryLoop()

	// Intelligent port selection
	host, portStr, err := net.SplitHostPort(p.Addr)
	if err != nil {
		host = "0.0.0.0"
		portStr = "8080"
	}

	var listener net.Listener
	var finalPort int
	startPort := 8080
	fmt.Sscanf(portStr, "%d", &startPort)

	for i := 0; i < 100; i++ {
		tryAddr := fmt.Sprintf("%s:%d", host, startPort+i)
		l, err := net.Listen("tcp", tryAddr)
		if err == nil {
			listener = l
			finalPort = startPort + i
			break
		}
	}

	if listener == nil {
		return fmt.Errorf("could not find available port after 100 attempts")
	}

	p.Addr = fmt.Sprintf("%s:%d", host, finalPort)
	fmt.Printf("WSL Plugin starting on %s\n", p.Addr)

	// Write port to file for smoke test
	os.WriteFile("smoke_port.txt", []byte(fmt.Sprintf("%d", finalPort)), 0644)

	return http.Serve(listener, mux)
}

func (p *WslPlugin) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	p.mu.Lock()
	p.clients[c] = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.clients, c)
		p.mu.Unlock()
	}()

	for {
		_, _, err := c.Read(context.Background())
		if err != nil {
			break
		}
	}
}

func (p *WslPlugin) broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Marshal error: %v", err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.clients {
		c.Write(context.Background(), websocket.MessageText, data)
	}
}

func (p *WslPlugin) listInstances() ([]InstanceInfo, error) {
	out, err := wslExec("-l", "-v")
	if err != nil {
		return nil, err
	}

	cleanOut := strings.ReplaceAll(string(out), "\x00", "")
	lines := strings.Split(cleanOut, "\n")
	if len(lines) <= 1 {
		return []InstanceInfo{}, nil
	}

	var instances []InstanceInfo
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			inst := InstanceInfo{
				Name:    parts[0],
				State:   parts[1],
				Version: parts[2],
				Memory:  "--",
				Disk:    "--",
			}
			if inst.State == "Running" {
				inst.Memory, inst.Disk = p.getStats(inst.Name)
			}
			instances = append(instances, inst)
		}
	}
	return instances, nil
}

func (p *WslPlugin) getStats(name string) (string, string) {
	mem := "--"
	disk := "--"
	out, err := wslExec("-d", name, "--", "sh", "-c", "free -m | grep Mem:; df -h / | grep /$")
	if err != nil {
		return mem, disk
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "Mem:") {
			f := strings.Fields(l)
			if len(f) >= 3 {
				mem = fmt.Sprintf("%sMB / %sMB", f[2], f[1])
			}
		} else if strings.HasSuffix(l, "/") {
			f := strings.Fields(l)
			if len(f) >= 3 {
				disk = fmt.Sprintf("%s / %s", f[2], f[1])
			}
		}
	}
	return mem, disk
}

func (p *WslPlugin) telemetryLoop() {
	for {
		time.Sleep(3 * time.Second)
		instances, err := p.listInstances()
		if err == nil {
			p.broadcast(map[string]interface{}{
				"type": "list",
				"data": instances,
			})
		}
	}
}

func (p *WslPlugin) wslBaseDir() string {
	env := os.Getenv("DIALTONE_ENV")
	if env == "" {
		home, _ := os.UserHomeDir()
		env = filepath.Join(home, "dialtone_dependencies")
	}
	// Expand ~ on Windows
	if strings.HasPrefix(env, "~") {
		home, _ := os.UserHomeDir()
		env = filepath.Join(home, env[1:])
	}
	// Resolve relative paths from cwd
	if !filepath.IsAbs(env) {
		cwd, _ := os.Getwd()
		env = filepath.Join(cwd, env)
	}
	return filepath.Join(env, "wsl")
}

func (p *WslPlugin) createInstance(name string) {
	if name == "" {
		log.Printf("[WSL] createInstance: rejecting empty name")
		return
	}
	wslDir := p.wslBaseDir()
	baseDir := filepath.Join(wslDir, "_bases")
	basePath := filepath.Join(baseDir, "alpine.tar.gz")
	installPath := filepath.Join(wslDir, name)

	log.Printf("[WSL] createInstance: name=%s wslDir=%s", name, wslDir)
	os.MkdirAll(baseDir, 0755)

	// Download Alpine rootfs if not cached (or if previous download was corrupt/empty)
	info, statErr := os.Stat(basePath)
	if statErr != nil || info.Size() < 1024 {
		if statErr == nil && info.Size() < 1024 {
			log.Printf("[WSL] Removing corrupt/empty Alpine tarball (%d bytes)", info.Size())
			os.Remove(basePath)
		}
		log.Printf("[WSL] Downloading Alpine rootfs...")
		alpineURL := "https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-minirootfs-3.21.2-x86_64.tar.gz"
		dlCmd := exec.Command("curl.exe", "-L", "-o", basePath, alpineURL)
		dlCmd.Stdout = os.Stdout
		dlCmd.Stderr = os.Stderr
		if err := dlCmd.Run(); err != nil {
			log.Printf("[WSL] Alpine download failed: %v", err)
			return
		}
		if fi, err := os.Stat(basePath); err != nil || fi.Size() < 1024 {
			log.Printf("[WSL] Alpine download produced invalid file")
			os.Remove(basePath)
			return
		}
		log.Printf("[WSL] Alpine rootfs downloaded to %s", basePath)
	} else {
		log.Printf("[WSL] Using cached Alpine rootfs (%d bytes) at %s", info.Size(), basePath)
	}

	// Cleanup if exists
	log.Printf("[WSL] Unregistering old %s if exists...", name)
	wslExec("--unregister", name)
	os.RemoveAll(installPath)

	// Import
	log.Printf("[WSL] Importing %s from %s into %s...", name, basePath, installPath)
	os.MkdirAll(installPath, 0755)
	importOut, err := wslExec("--import", name, installPath, basePath)
	if err != nil {
		log.Printf("[WSL] Import failed: %v\nOutput: %s", err, string(importOut))
		return
	}
	log.Printf("[WSL] Import succeeded for %s", name)

	// Start the instance with a keep-alive process
	log.Printf("[WSL] Starting %s...", name)
	go func() {
		startOut, err := wslExec("-d", name, "-u", "root", "--", "sh", "-c", "echo STARTED && while true; do sleep 3600; done")
		if err != nil {
			log.Printf("[WSL] Start background for %s ended: %v (output: %s)", name, err, strings.TrimSpace(string(startOut)))
		}
	}()
}

func (p *WslPlugin) stopInstance(name string) {
	log.Printf("[WSL] Stopping instance %s...", name)
	wslExec("--terminate", name)
	log.Printf("[WSL] Stop complete for %s", name)
}

func (p *WslPlugin) deleteInstance(name string) {
	log.Printf("[WSL] Deleting instance %s...", name)
	p.stopInstance(name)
	wslExec("--unregister", name)
	installPath := filepath.Join(p.wslBaseDir(), name)
	os.RemoveAll(installPath)
	log.Printf("[WSL] Delete complete for %s", name)
}
