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
	"time"
)

type ProcessInfo struct {
	ID string `json:"id"`

	PID int `json:"pid"`

	Status string `json:"status"`

	StartTime string `json:"start_time"`

	Logs []string `json:"logs"`
}

type NixPlugin struct {
	Addr string

	mu sync.Mutex

	cmds map[string]*exec.Cmd

	logs map[string][]string

	status map[string]string

	startTime map[string]time.Time
}

func NewNixPlugin(addr string) *NixPlugin {

	return &NixPlugin{

		Addr: addr,

		cmds: make(map[string]*exec.Cmd),

		logs: make(map[string][]string),

		status: make(map[string]string),

		startTime: make(map[string]time.Time),
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

			cmd := exec.Command("bash", "-c", "while true; do echo 'hello dialtone from "+id+"'; sleep 2; done")

			stdout, _ := cmd.StdoutPipe()

			stderr, _ := cmd.StderrPipe()

			if err := cmd.Start(); err != nil {

				http.Error(w, err.Error(), 500)

				return

			}

			p.cmds[id] = cmd

			p.logs[id] = []string{"Process started..."}

			p.status[id] = "running"

			p.startTime[id] = time.Now()

			go p.captureLogs(id, stdout)

			go p.captureLogs(id, stderr)

			go func() {

				cmd.Wait()

				p.mu.Lock()

				p.status[id] = "stopped"

				p.mu.Unlock()

			}()

			json.NewEncoder(w).Encode(ProcessInfo{

				ID: id,

				PID: cmd.Process.Pid,

				Status: "running",

				StartTime: p.startTime[id].Format(time.Kitchen),
			})

			return

		}

		list := []ProcessInfo{}

		for id := range p.cmds {

			pid := 0

			if p.cmds[id].Process != nil {

				pid = p.cmds[id].Process.Pid

			}

			list = append(list, ProcessInfo{

				ID: id,

				PID: pid,

				Status: p.status[id],

				StartTime: p.startTime[id].Format(time.Kitchen),

				Logs: p.logs[id],
			})

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

			p.status[id] = "stopped"

			p.logs[id] = append(p.logs[id], "Process terminated.")

			w.WriteHeader(http.StatusOK)

			return

		}

		http.Error(w, "not found", 404)

	})

	// Smart UI Path detection
	workDir, _ := os.Getwd()
	possiblePaths := []string{
		filepath.Join(workDir, "ui/dist"),                        // If running from src/plugins/nix/src_v1
		filepath.Join(workDir, "src/plugins/nix/src_v1/ui/dist"), // If running from root
	}

	var uiPath string
	for _, p := range possiblePaths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			uiPath = p
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
