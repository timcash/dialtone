package wslv3

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	Addr      string
	mu        sync.Mutex
	clients   map[*websocket.Conn]bool
	keepAlive map[string]*exec.Cmd
}

func NewWslPlugin(addr string) *WslPlugin {
	return &WslPlugin{
		Addr:      addr,
		clients:   make(map[*websocket.Conn]bool),
		keepAlive: make(map[string]*exec.Cmd),
	}
}

func ListInstances() ([]InstanceInfo, error) {
	return NewWslPlugin("").listInstances()
}

func CreateInstance(name string) error {
	p := NewWslPlugin("")
	p.createInstance(strings.TrimSpace(name))
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("instance name is required")
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		instances, err := p.listInstances()
		if err == nil {
			for _, inst := range instances {
				if strings.EqualFold(strings.TrimSpace(inst.Name), strings.TrimSpace(name)) {
					return nil
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("instance %s did not appear after create", name)
}

func StopInstance(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("instance name is required")
	}
	NewWslPlugin("").stopInstance(name)
	return nil
}

func DeleteInstance(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("instance name is required")
	}
	NewWslPlugin("").deleteInstance(name)
	return nil
}

func ExecInstance(name string, args ...string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("instance name is required")
	}
	cmdArgs := []string{"-d", name, "-u", "root", "--"}
	if len(args) == 0 {
		cmdArgs = append(cmdArgs, "sh", "-lc", "cat /etc/alpine-release")
	} else {
		cmdArgs = append(cmdArgs, args...)
	}
	out, err := wslExecWithTimeout(45*time.Second, cmdArgs...)
	return strings.TrimSpace(cleanWSLOutput(out)), err
}

func resolveWSLExecutable() (string, error) {
	candidates := []string{
		"wsl.exe",
		"/mnt/c/WINDOWS/system32/wsl.exe",
		"/mnt/c/Windows/System32/wsl.exe",
		`C:\WINDOWS\system32\wsl.exe`,
		`C:\Windows\System32\wsl.exe`,
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, string(os.PathSeparator)) || strings.Contains(candidate, ":") {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
			continue
		}
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("wsl.exe not found on PATH or known Windows locations")
}

func cleanWSLOutput(raw []byte) string {
	return strings.ReplaceAll(string(raw), "\x00", "")
}

func toWindowsPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		return path
	}
	out, err := exec.Command("wslpath", "-w", path).CombinedOutput()
	if err != nil {
		return path
	}
	return strings.TrimSpace(cleanWSLOutput(out))
}

func downloadFile(url, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed: http status %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func wslExecWithTimeout(timeout time.Duration, args ...string) ([]byte, error) {
	wslExe, err := resolveWSLExecutable()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	log.Printf("[WSL] exec: %s %s", wslExe, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, wslExe, args...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[WSL] exec TIMEOUT (%s): %s %s", timeout, wslExe, strings.Join(args, " "))
		return out, fmt.Errorf("wsl.exe %s timed out after %s", strings.Join(args, " "), timeout)
	}
	if err != nil {
		log.Printf("[WSL] exec ERROR: %s %s -> %v (output: %s)", wslExe, strings.Join(args, " "), err, strings.TrimSpace(cleanWSLOutput(out)))
	} else {
		log.Printf("[WSL] exec OK: %s %s (%d bytes output)", wslExe, strings.Join(args, " "), len(out))
	}
	return out, err
}

// wslExec runs a wsl.exe command with a default timeout.
func wslExec(args ...string) ([]byte, error) {
	return wslExecWithTimeout(30*time.Second, args...)
}

func (p *WslPlugin) Start() error {
	paths, _ := ResolvePaths("")
	uiPath := paths.Preset.UIDist

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

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if uiPath == "" {
			fmt.Fprintf(w, "UI not found.")
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
	fmt.Printf("WSL Plugin (v3) starting on %s\n", p.Addr)

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

	cleanOut := cleanWSLOutput(out)
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
	out, err := wslExecWithTimeout(10*time.Second, "-d", name, "--", "sh", "-c", "free -m | grep Mem:; df -h / | grep /$")
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
	basePathWin := toWindowsPath(basePath)
	installPathWin := toWindowsPath(installPath)

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
		if err := downloadFile(alpineURL, basePath); err != nil {
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
	importOut, err := wslExecWithTimeout(2*time.Minute, "--import", name, installPathWin, basePathWin)
	if err != nil {
		log.Printf("[WSL] Import failed: %v\nOutput: %s", err, cleanWSLOutput(importOut))
		return
	}
	log.Printf("[WSL] Import succeeded for %s", name)

	if _, err := wslExecWithTimeout(45*time.Second, "-d", name, "-u", "root", "--", "sh", "-c", "test -f /etc/alpine-release && cat /etc/alpine-release"); err != nil {
		log.Printf("[WSL] Alpine verification for %s failed: %v", name, err)
		return
	}
	if err := p.startKeepAlive(name); err != nil {
		log.Printf("[WSL] Start keepalive for %s failed: %v", name, err)
		return
	}
	log.Printf("[WSL] Alpine instance %s is running", name)
}

func (p *WslPlugin) stopInstance(name string) {
	log.Printf("[WSL] Stopping instance %s...", name)
	p.stopKeepAlive(name)
	wslExecWithTimeout(20*time.Second, "--terminate", name)
	log.Printf("[WSL] Stop complete for %s", name)
}

func (p *WslPlugin) deleteInstance(name string) {
	log.Printf("[WSL] Deleting instance %s...", name)
	p.stopInstance(name)
	wslExecWithTimeout(30*time.Second, "--unregister", name)
	installPath := filepath.Join(p.wslBaseDir(), name)
	os.RemoveAll(installPath)
	log.Printf("[WSL] Delete complete for %s", name)
}

func (p *WslPlugin) startKeepAlive(name string) error {
	wslExe, err := resolveWSLExecutable()
	if err != nil {
		return err
	}
	p.stopKeepAlive(name)

	cmd := exec.Command(wslExe, "-d", name, "-u", "root", "--", "sh", "-c", "mkdir -p /run && touch /run/dialtone-host-started && exec tail -f /dev/null")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	p.mu.Lock()
	p.keepAlive[name] = cmd
	p.mu.Unlock()

	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		if current, ok := p.keepAlive[name]; ok && current == cmd {
			delete(p.keepAlive, name)
		}
		p.mu.Unlock()
		if err != nil {
			log.Printf("[WSL] Keepalive exited for %s: %v", name, err)
		}
	}()
	return nil
}

func (p *WslPlugin) stopKeepAlive(name string) {
	p.mu.Lock()
	cmd := p.keepAlive[name]
	delete(p.keepAlive, name)
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
