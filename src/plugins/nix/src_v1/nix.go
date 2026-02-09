package nix

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

type ProcessInfo struct {
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Logs   []string `json:"logs"`
}

type NixPlugin struct {
	Addr string
	mu   sync.Mutex
	cmds map[string]*exec.Cmd
	logs map[string][]string
}

func NewNixPlugin(addr string) *NixPlugin {
	return &NixPlugin{
		Addr: addr,
		cmds: make(map[string]*exec.Cmd),
		logs: make(map[string][]string),
	}
}

func (p *NixPlugin) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "Online"})
	})

	mux.HandleFunc("/api/processes", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()

		if r.Method == http.MethodPost {
			id := fmt.Sprintf("proc-%d", len(p.cmds)+1)
			// Using nix-shell to run a simple loop
			cmd := exec.Command("nix-shell", "-p", "hello", "--run", "while true; do echo 'hello dialtone from " + id + "'; sleep 2; done")
			
			stdout, _ := cmd.StdoutPipe()
			stderr, _ := cmd.StderrPipe()

			if err := cmd.Start(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			p.cmds[id] = cmd
			p.logs[id] = []string{"Process started..."}

			go p.captureLogs(id, stdout)
			go p.captureLogs(id, stderr)

			json.NewEncoder(w).Encode(ProcessInfo{ID: id, Status: "running"})
			return
		}

		var list []ProcessInfo
		for id, cmd := range p.cmds {
			status := "running"
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				status = "stopped"
			}
			list = append(list, ProcessInfo{ID: id, Status: status, Logs: p.logs[id]})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	})

	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		p.mu.Lock()
		defer p.mu.Unlock()

		if cmd, ok := p.cmds[id]; ok {
			cmd.Process.Kill()
			p.logs[id] = append(p.logs[id], "Process terminated.")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", 404)
	})

	workDir, _ := os.Getwd()
	uiPath := filepath.Join(workDir, "src/plugins/nix/src_v1/ui/dist")
	
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(uiPath, r.URL.Path)
		if r.URL.Path == "/" {
			path = filepath.Join(uiPath, "index.html")
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Fprintf(w, "UI not built. Run 'bun run build' in src/plugins/nix/src_v1/ui")
			return
		}
		http.ServeFile(w, r, path)
	})

	fmt.Printf("Nix Plugin starting on %s\n", p.Addr)
	return http.ListenAndServe(p.Addr, mux)
}

func (p *NixPlugin) captureLogs(id string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		p.mu.Lock()
		line := scanner.Text()
		p.logs[id] = append(p.logs[id], line)
		if len(p.logs[id]) > 20 {
			p.logs[id] = p.logs[id][1:]
		}
		p.mu.Unlock()
	}
}