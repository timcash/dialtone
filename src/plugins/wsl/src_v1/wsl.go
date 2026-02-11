package wsl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
			http.Error(w, err.Error(), 500)
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

	fmt.Printf("WSL Plugin starting on %s\n", p.Addr)
	return http.ListenAndServe(p.Addr, mux)
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
	cmd := exec.Command("wsl.exe", "-l", "-v")
	out, err := cmd.Output()
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
	cmd := exec.Command("wsl.exe", "-d", name, "--", "sh", "-c", "free -m | grep Mem:; df -h / | grep /$")
	out, err := cmd.CombinedOutput()
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

func (p *WslPlugin) createInstance(name string) {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, "WSL", "_bases")
	basePath := filepath.Join(baseDir, "alpine.tar.gz")
	installPath := filepath.Join(home, "WSL", name)

	os.MkdirAll(baseDir, 0755)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		log.Printf("Fetching Alpine for %s...", name)
		url := "https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/alpine-minirootfs-3.21.2-x86_64.tar.gz"
		cmd := exec.Command("powershell.exe", "-Command", fmt.Sprintf("Invoke-WebRequest -Uri %s -OutFile %s", url, basePath))
		cmd.Run()
	}

	log.Printf("Importing WSL instance %s...", name)
	// Cleanup if exists
	exec.Command("wsl.exe", "--unregister", name).Run()
	os.RemoveAll(installPath)

	os.MkdirAll(installPath, 0755)
	cmd := exec.Command("wsl.exe", "--import", name, installPath, basePath)
	err := cmd.Run()
	if err != nil {
		log.Printf("Import failed: %v", err)
		return
	}

	log.Printf("Starting %s...", name)
	go exec.Command("wsl.exe", "-d", name, "-u", "root", "--", "sh", "-c", "while true; do sleep 3600; done").Run()
}

func (p *WslPlugin) stopInstance(name string) {
	log.Printf("Stopping WSL instance %s...", name)
	exec.Command("wsl.exe", "--terminate", name).Run()
}

func (p *WslPlugin) deleteInstance(name string) {
	log.Printf("Deleting WSL instance %s...", name)
	p.stopInstance(name)
	exec.Command("wsl.exe", "--unregister", name).Run()
	home, _ := os.UserHomeDir()
	installPath := filepath.Join(home, "WSL", name)
	os.RemoveAll(installPath)
}
