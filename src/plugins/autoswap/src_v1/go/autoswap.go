package autoswap

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

type runtimeManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Runtime struct {
		Binary    string            `json:"binary"`
		Processes []manifestProcess `json:"processes"`
	} `json:"runtime"`
	Artifacts struct {
		Sync    map[string]string         `json:"sync"`
		Release map[string]releaseBinding `json:"release"`
	} `json:"artifacts"`
}

type manifestProcess struct {
	Name      string            `json:"name"`
	Artifact  string            `json:"artifact"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	DependsOn []string          `json:"depends_on"`
}

type releaseBinding struct {
	Asset   string `json:"asset"`
	Type    string `json:"type"`
	Extract string `json:"extract"`
}

type composeConfig struct {
	ManifestPath  string
	ManifestURL   string
	RepoRoot      string
	Listen        string
	NATSPort      int
	NATSWSPort    int
	Timeout       time.Duration
	RequireStream bool
	StayRunning   bool
}

type resolvedArtifacts struct {
	ManifestPath string
	Manifest     runtimeManifest
	Sync         map[string]string
	RepoRoot     string
	ManifestDir  string
	AutoswapRoot string
	RobotBin     string
	AutoswapBin  string
	ReplBin      string
	CameraBin    string
	MavlinkBin   string
	WLANBundle   string
	UIDist       string
}

func Stage(args []string) error {
	cfg, err := parseComposeFlags("stage", args)
	if err != nil {
		return err
	}
	manifestPath, err := materializeManifestForCompose(cfg)
	if err != nil {
		return err
	}
	art, err := loadAndValidateManifest(manifestPath, cfg.RepoRoot)
	if err != nil {
		return err
	}
	logs.Info("autoswap stage OK manifest=%s", art.ManifestPath)
	logs.Info("autoswap stage artifacts robot=%s camera=%s mavlink=%s repl=%s ui=%s", art.RobotBin, art.CameraBin, art.MavlinkBin, art.ReplBin, art.UIDist)
	return nil
}

func Run(args []string) error {
	cfg, err := parseComposeFlags("run", args)
	if err != nil {
		return err
	}
	manifestPath, err := materializeManifestForCompose(cfg)
	if err != nil {
		return err
	}
	art, err := loadAndValidateManifest(manifestPath, cfg.RepoRoot)
	if err != nil {
		return err
	}
	if len(art.Manifest.Runtime.Processes) > 0 {
		cfg.ManifestPath = manifestPath
		return runManifestProcesses(cfg, art)
	}
	cfg.ManifestPath = manifestPath

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", cfg.NATSPort)
	cameraSidecarURL := "http://127.0.0.1:19090"
	runtimeStatePath := strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_STATE"))
	writeState := func(procs []managedProc) {
		if runtimeStatePath == "" {
			return
		}
		_ = writeRuntimeState(runtimeStatePath, runtimeState{
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
			ManifestPath: cfg.ManifestPath,
			RepoRoot:     cfg.RepoRoot,
			Listen:       cfg.Listen,
			NATSPort:     cfg.NATSPort,
			NATSWSPort:   cfg.NATSWSPort,
			Processes:    snapshotManagedProcesses(procs),
		})
	}
	buildRobot := func() *exec.Cmd {
		cmd := exec.CommandContext(
			ctx,
			art.RobotBin,
			"--listen", cfg.Listen,
			"--nats-port", strconv.Itoa(cfg.NATSPort),
			"--nats-ws-port", strconv.Itoa(cfg.NATSWSPort),
			"--ui-dist", art.UIDist,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(),
			"ROBOT_V2_CAMERA_STREAM_URL="+cameraSidecarURL,
			"ROBOT_V2_MAVLINK_ENABLED=1",
		)
		return cmd
	}
	buildCamera := func() *exec.Cmd {
		cmd := exec.CommandContext(
			ctx,
			art.CameraBin, "run",
			"--nats-url", natsURL,
			"--subject", "camera.heartbeat",
			"--interval", "250ms",
			"--listen", ":19090",
			"--serve-stream=true",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd
	}
	buildMavlink := func() *exec.Cmd {
		cmd := exec.CommandContext(
			ctx,
			art.MavlinkBin, "run",
			"--nats-url", natsURL,
			"--mock-if-no-endpoint=false",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "MAVLINK_ENDPOINT="+strings.TrimSpace(os.Getenv("MAVLINK_ENDPOINT")))
		return cmd
	}
	buildRepl := func() *exec.Cmd {
		cmd := exec.CommandContext(
			ctx,
			art.ReplBin, "leader",
			"--nats-url", natsURL,
			"--room", "robot",
			"--embedded-nats=false",
			"--hostname", "robot-v2",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd
	}

	robotProc := managedProc{Name: "robot", Build: buildRobot}
	if err := robotProc.Start(); err != nil {
		return fmt.Errorf("start robot failed: %w", err)
	}
	defer robotProc.Stop()

	baseURL := "http://127.0.0.1" + cfg.Listen
	if err := waitHTTP(ctx, baseURL+"/health", http.StatusOK); err != nil {
		return err
	}
	if err := waitHTTP(ctx, baseURL+"/", http.StatusOK); err != nil {
		return err
	}

	procs := []managedProc{
		robotProc,
		{Name: "camera", Build: buildCamera},
		{Name: "mavlink", Build: buildMavlink},
		{Name: "repl", Build: buildRepl},
	}
	for i := 1; i < len(procs); i++ {
		if err := procs[i].Start(); err != nil {
			return fmt.Errorf("start %s failed: %w", procs[i].Name, err)
		}
		defer procs[i].Stop()
	}
	writeState(procs)

	if err := verifyReplBinary(ctx, art.ReplBin); err != nil {
		return err
	}
	if err := waitHTTP(ctx, cameraSidecarURL+"/health", http.StatusOK); err != nil {
		return err
	}
	if cfg.RequireStream {
		if err := waitHTTP(ctx, baseURL+"/stream", http.StatusOK); err != nil {
			return err
		}
	}
	if err := waitHeartbeats(ctx, natsURL); err != nil {
		return err
	}

	logs.Info("autoswap run OK: robot+ui+camera+mavlink composition healthy")
	if cfg.StayRunning {
		logs.Info("autoswap stay-running enabled; supervising manifest processes")
		return superviseProcesses(ctx, procs)
	}
	writeState(procs)
	return nil
}

func runManifestProcesses(cfg composeConfig, art resolvedArtifacts) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	procs, err := buildManagedProcessesFromManifest(ctx, cfg, art)
	if err != nil {
		return err
	}
	for i := range procs {
		if err := procs[i].Start(); err != nil {
			return fmt.Errorf("start %s failed: %w", procs[i].Name, err)
		}
		defer procs[i].Stop()
	}

	baseURL := "http://127.0.0.1" + cfg.Listen
	if err := waitHTTP(ctx, baseURL+"/health", http.StatusOK); err != nil {
		return err
	}
	if cfg.RequireStream {
		_ = waitHTTP(ctx, baseURL+"/stream", http.StatusOK)
	}
	logs.Info("autoswap run OK: manifest composition healthy processes=%d", len(procs))
	if cfg.StayRunning {
		return superviseProcesses(ctx, procs)
	}
	return nil
}

func buildManagedProcessesFromManifest(ctx context.Context, cfg composeConfig, art resolvedArtifacts) ([]managedProc, error) {
	ordered, err := orderManifestProcesses(art.Manifest.Runtime.Processes)
	if err != nil {
		return nil, err
	}
	out := make([]managedProc, 0, len(ordered))
	for _, p := range ordered {
		proc := p
		cmdArgs, envVars, err := resolveProcessInvocation(proc, art, cfg)
		if err != nil {
			return nil, err
		}
		if len(cmdArgs) == 0 {
			return nil, fmt.Errorf("process %s has empty command", proc.Name)
		}
		out = append(out, managedProc{
			Name: proc.Name,
			Build: func() *exec.Cmd {
				cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Env = append(os.Environ(), envVars...)
				return cmd
			},
		})
	}
	return out, nil
}

func resolveProcessInvocation(p manifestProcess, art resolvedArtifacts, cfg composeConfig) ([]string, []string, error) {
	tokens := map[string]string{
		"listen":       cfg.Listen,
		"nats_port":    strconv.Itoa(cfg.NATSPort),
		"nats_ws_port": strconv.Itoa(cfg.NATSWSPort),
		"nats_url":     fmt.Sprintf("nats://127.0.0.1:%d", cfg.NATSPort),
	}
	for k, v := range art.Sync {
		tokens["artifact:"+k] = v
	}
	expand := func(v string) string {
		s := strings.TrimSpace(v)
		for tk, tv := range tokens {
			s = strings.ReplaceAll(s, "${"+tk+"}", tv)
		}
		for {
			start := strings.Index(s, "${env:")
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], "}")
			if end < 0 {
				break
			}
			end = start + end
			key := strings.TrimSpace(strings.TrimPrefix(s[start:end], "${env:"))
			s = s[:start] + os.Getenv(key) + s[end+1:]
		}
		return s
	}
	cmdArgs := []string{}
	if len(p.Command) > 0 {
		for _, c := range p.Command {
			cmdArgs = append(cmdArgs, expand(c))
		}
	} else if strings.TrimSpace(p.Artifact) != "" {
		target := strings.TrimSpace(art.Sync[p.Artifact])
		if target == "" {
			return nil, nil, fmt.Errorf("process %s references unknown artifact %s", p.Name, p.Artifact)
		}
		cmdArgs = append(cmdArgs, target)
	}
	for _, a := range p.Args {
		cmdArgs = append(cmdArgs, expand(a))
	}
	envVars := []string{}
	for k, v := range p.Env {
		envVars = append(envVars, strings.TrimSpace(k)+"="+expand(v))
	}
	return cmdArgs, envVars, nil
}

func orderManifestProcesses(in []manifestProcess) ([]manifestProcess, error) {
	byName := map[string]manifestProcess{}
	for _, p := range in {
		n := strings.TrimSpace(p.Name)
		if n == "" {
			return nil, fmt.Errorf("manifest process missing name")
		}
		if _, ok := byName[n]; ok {
			return nil, fmt.Errorf("duplicate process name %s", n)
		}
		byName[n] = p
	}
	visited := map[string]int{}
	out := make([]manifestProcess, 0, len(in))
	var visit func(string) error
	visit = func(name string) error {
		switch visited[name] {
		case 1:
			return fmt.Errorf("process dependency cycle at %s", name)
		case 2:
			return nil
		}
		visited[name] = 1
		p := byName[name]
		for _, dep := range p.DependsOn {
			d := strings.TrimSpace(dep)
			if d == "" {
				continue
			}
			if _, ok := byName[d]; !ok {
				return fmt.Errorf("process %s depends on unknown process %s", name, d)
			}
			if err := visit(d); err != nil {
				return err
			}
		}
		visited[name] = 2
		out = append(out, p)
		return nil
	}
	for _, p := range in {
		if err := visit(strings.TrimSpace(p.Name)); err != nil {
			return nil, err
		}
	}
	return out, nil
}

type managedProc struct {
	Name         string
	Build        func() *exec.Cmd
	Cmd          *exec.Cmd
	RestartCount int
	LastExit     string
	StartedAt    time.Time
	mu           sync.Mutex
}

type managedProcState struct {
	Name         string `json:"name"`
	PID          int    `json:"pid,omitempty"`
	RestartCount int    `json:"restart_count"`
	Status       string `json:"status"`
	StartedAt    string `json:"started_at,omitempty"`
	LastExit     string `json:"last_exit,omitempty"`
}

type runtimeState struct {
	UpdatedAt    string             `json:"updated_at"`
	ManifestPath string             `json:"manifest_path"`
	RepoRoot     string             `json:"repo_root"`
	Listen       string             `json:"listen"`
	NATSPort     int                `json:"nats_port"`
	NATSWSPort   int                `json:"nats_ws_port"`
	Processes    []managedProcState `json:"processes"`
}

func (p *managedProc) Start() error {
	if p == nil || p.Build == nil {
		return fmt.Errorf("invalid managed process")
	}
	cmd := p.Build()
	if err := cmd.Start(); err != nil {
		return err
	}
	p.mu.Lock()
	p.Cmd = cmd
	p.StartedAt = time.Now().UTC()
	p.mu.Unlock()
	return nil
}

func (p *managedProc) Stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	cmd := p.Cmd
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	p.mu.Lock()
	p.LastExit = "stopped"
	if p.Cmd == cmd {
		p.Cmd = nil
	}
	p.mu.Unlock()
}

func superviseProcesses(ctx context.Context, procs []managedProc) error {
	exitCh := make(chan string, len(procs)*2)
	watch := func(p *managedProc) {
		cmd := p.Cmd
		go func(name string, local *exec.Cmd) {
			_ = local.Wait()
			select {
			case exitCh <- name:
			default:
			}
		}(p.Name, cmd)
	}
	index := map[string]*managedProc{}
	for i := range procs {
		index[procs[i].Name] = &procs[i]
		watch(&procs[i])
	}
	statePath := strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_STATE"))
	write := func() {
		if statePath == "" {
			return
		}
		natsPort, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_NATS_PORT")))
		natsWSPort, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_NATS_WS_PORT")))
		_ = writeRuntimeState(statePath, runtimeState{
			UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
			ManifestPath: strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_MANIFEST")),
			RepoRoot:     strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_REPO_ROOT")),
			Listen:       strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_LISTEN")),
			NATSPort:     natsPort,
			NATSWSPort:   natsWSPort,
			Processes:    snapshotManagedProcesses(procs),
		})
	}
	write()
	for {
		select {
		case <-ctx.Done():
			for i := range procs {
				procs[i].Stop()
			}
			write()
			return nil
		case name := <-exitCh:
			if ctx.Err() != nil {
				return nil
			}
			p := index[name]
			if p == nil {
				continue
			}
			logs.Warn("managed process exited, restarting: %s", name)
			p.Stop()
			p.mu.Lock()
			p.RestartCount++
			p.LastExit = "exited"
			p.mu.Unlock()
			time.Sleep(600 * time.Millisecond)
			if err := p.Start(); err != nil {
				logs.Error("restart failed for %s: %v", name, err)
				time.Sleep(1 * time.Second)
				write()
				continue
			}
			watch(p)
			write()
		}
	}
}

func snapshotManagedProcesses(procs []managedProc) []managedProcState {
	out := make([]managedProcState, 0, len(procs))
	for i := range procs {
		p := &procs[i]
		p.mu.Lock()
		st := managedProcState{
			Name:         p.Name,
			RestartCount: p.RestartCount,
			LastExit:     p.LastExit,
		}
		if p.StartedAt.Unix() > 0 {
			st.StartedAt = p.StartedAt.Format(time.RFC3339)
		}
		if p.Cmd != nil && p.Cmd.Process != nil {
			st.PID = p.Cmd.Process.Pid
			st.Status = "running"
		} else {
			st.Status = "stopped"
		}
		p.mu.Unlock()
		out = append(out, st)
	}
	return out
}

func writeRuntimeState(path string, st runtimeState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func parseComposeFlags(name string, args []string) (composeConfig, error) {
	defaultManifest := "src/plugins/robot/src_v2/config/composition.manifest.json"
	defaultRepoRoot := ""
	if rt, err := configv1.ResolveRuntime(""); err == nil {
		defaultManifest = filepath.Join(rt.RepoRoot, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
		defaultRepoRoot = rt.RepoRoot
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	manifest := fs.String("manifest", defaultManifest, "Path to composition manifest")
	manifestURL := fs.String("manifest-url", "", "Manifest URL (if set, overrides --manifest)")
	repoRoot := fs.String("repo-root", defaultRepoRoot, "Optional repo root for <repo_root> substitutions")
	listen := fs.String("listen", ":18084", "Runtime listen address")
	natsPort := fs.Int("nats-port", 18226, "Embedded NATS port")
	natsWSPort := fs.Int("nats-ws-port", 18227, "Embedded NATS websocket port")
	timeout := fs.Duration("timeout", 25*time.Second, "Compose timeout")
	requireStream := fs.Bool("require-stream", false, "Require /stream endpoint to return HTTP 200")
	stayRunning := fs.Bool("stay-running", false, "Keep composition running until timeout/signal")
	if err := fs.Parse(args); err != nil {
		return composeConfig{}, err
	}
	return composeConfig{
		ManifestPath:  strings.TrimSpace(*manifest),
		ManifestURL:   strings.TrimSpace(*manifestURL),
		RepoRoot:      strings.TrimSpace(*repoRoot),
		Listen:        strings.TrimSpace(*listen),
		NATSPort:      *natsPort,
		NATSWSPort:    *natsWSPort,
		Timeout:       *timeout,
		RequireStream: *requireStream,
		StayRunning:   *stayRunning,
	}, nil
}

func materializeManifestForCompose(cfg composeConfig) (string, error) {
	token := strings.TrimSpace(os.Getenv("AUTOSWAP_MANIFEST_TOKEN"))
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	return resolveManifestPath(
		cfg.ManifestPath,
		cfg.ManifestURL,
		filepath.Join(userHomeDir(), ".dialtone", "autoswap", "manifests"),
		token,
	)
}

func resolveManifestPath(manifestPath, manifestURL, manifestDir, token string) (string, error) {
	url := strings.TrimSpace(manifestURL)
	if url == "" {
		if strings.TrimSpace(manifestPath) == "" {
			return "", fmt.Errorf("manifest source required: set --manifest or --manifest-url")
		}
		return strings.TrimSpace(manifestPath), nil
	}
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(url))
	fileName := fmt.Sprintf("manifest-%x.json", sum[:8])
	finalPath := filepath.Join(manifestDir, fileName)
	tmpPath := finalPath + ".tmp"
	if err := downloadFile(url, token, tmpPath); err != nil {
		return "", fmt.Errorf("download manifest failed: %w", err)
	}
	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	var mf runtimeManifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("manifest parse failed: %w", err)
	}
	if err := os.WriteFile(finalPath, raw, 0o644); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	_ = os.Remove(tmpPath)
	return finalPath, nil
}

func loadManifestResolved(manifestPath, repoRoot string, requireExisting bool) (resolvedArtifacts, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return resolvedArtifacts{}, fmt.Errorf("read manifest failed: %w", err)
	}
	var mf runtimeManifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return resolvedArtifacts{}, fmt.Errorf("manifest parse failed: %w", err)
	}

	absManifest, _ := filepath.Abs(strings.TrimSpace(manifestPath))
	manifestDir := filepath.Dir(absManifest)
	autoswapRoot := filepath.Join(userHomeDir(), ".dialtone", "autoswap")

	resolve := func(p string) string {
		v := strings.TrimSpace(p)
		v = strings.ReplaceAll(v, "<repo_root>", strings.TrimSpace(repoRoot))
		v = strings.ReplaceAll(v, "<manifest_dir>", manifestDir)
		v = strings.ReplaceAll(v, "<autoswap_root>", autoswapRoot)
		return v
	}
	syncResolved := map[string]string{}
	for k, v := range mf.Artifacts.Sync {
		syncResolved[strings.TrimSpace(k)] = resolve(v)
	}
	getOpt := func(key string) string {
		return strings.TrimSpace(syncResolved[key])
	}

	out := resolvedArtifacts{
		ManifestPath: absManifest,
		Manifest:     mf,
		Sync:         syncResolved,
		RepoRoot:     strings.TrimSpace(repoRoot),
		ManifestDir:  manifestDir,
		AutoswapRoot: autoswapRoot,
		RobotBin:     getOpt("robot"),
		AutoswapBin:  getOpt("autoswap"),
		ReplBin:      getOpt("repl"),
		CameraBin:    getOpt("camera"),
		MavlinkBin:   getOpt("mavlink"),
		WLANBundle:   getOpt("wlan"),
		UIDist:       getOpt("ui_dist"),
	}

	if !requireExisting {
		return out, nil
	}

	if len(out.Manifest.Runtime.Processes) > 0 {
		for _, p := range out.Manifest.Runtime.Processes {
			if strings.TrimSpace(p.Name) == "" {
				return resolvedArtifacts{}, fmt.Errorf("manifest runtime.processes contains unnamed process")
			}
			if strings.TrimSpace(p.Artifact) != "" {
				target := strings.TrimSpace(out.Sync[p.Artifact])
				if target == "" {
					return resolvedArtifacts{}, fmt.Errorf("process %s references unknown artifact %s", p.Name, p.Artifact)
				}
				if err := requirePath(target); err != nil {
					return resolvedArtifacts{}, err
				}
			}
		}
		return out, nil
	}

	requiredLegacy := []struct {
		key, path string
	}{
		{"autoswap", out.AutoswapBin},
		{"robot", out.RobotBin},
		{"repl", out.ReplBin},
		{"camera", out.CameraBin},
		{"mavlink", out.MavlinkBin},
	}
	for _, req := range requiredLegacy {
		if strings.TrimSpace(req.path) == "" {
			return resolvedArtifacts{}, fmt.Errorf("manifest missing artifacts.sync.%s", req.key)
		}
	}
	for _, bin := range []string{out.AutoswapBin, out.RobotBin, out.ReplBin, out.CameraBin, out.MavlinkBin} {
		if err := requireExecutable(bin); err != nil {
			return resolvedArtifacts{}, err
		}
	}
	if strings.TrimSpace(out.UIDist) == "" {
		return resolvedArtifacts{}, fmt.Errorf("manifest missing artifacts.sync.ui_dist")
	}
	if err := requireDirWithIndex(out.UIDist); err != nil {
		return resolvedArtifacts{}, err
	}
	if strings.TrimSpace(out.WLANBundle) == "" {
		return resolvedArtifacts{}, fmt.Errorf("manifest missing artifacts.sync.wlan")
	}
	if err := requirePath(out.WLANBundle); err != nil {
		return resolvedArtifacts{}, err
	}
	return out, nil
}

func loadAndValidateManifest(manifestPath, repoRoot string) (resolvedArtifacts, error) {
	return loadManifestResolved(manifestPath, repoRoot, true)
}

func waitHTTP(ctx context.Context, url string, expected int) error {
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == expected {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("http check timed out for %s expecting %d", url, expected)
		case <-time.After(180 * time.Millisecond):
		}
	}
}

func verifyReplBinary(ctx context.Context, replBin string) error {
	cmd := exec.CommandContext(ctx, replBin, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("repl binary verify failed: %w output=%s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitHeartbeats(ctx context.Context, natsURL string) error {
	nc, err := nats.Connect(natsURL, nats.Timeout(2*time.Second))
	if err != nil {
		return err
	}
	defer nc.Close()

	cameraCh := make(chan struct{}, 1)
	mavlinkCh := make(chan struct{}, 1)

	subA, err := nc.Subscribe("camera.>", func(_ *nats.Msg) {
		select {
		case cameraCh <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return err
	}
	defer subA.Unsubscribe()
	subB, err := nc.Subscribe("mavlink.>", func(_ *nats.Msg) {
		select {
		case mavlinkCh <- struct{}{}:
		default:
		}
	})
	if err != nil {
		return err
	}
	defer subB.Unsubscribe()
	if err := nc.Flush(); err != nil {
		return err
	}

	gotCamera := false
	gotMavlink := false
	for !(gotCamera && gotMavlink) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for camera+mavlink heartbeats")
		case <-cameraCh:
			gotCamera = true
		case <-mavlinkCh:
			gotMavlink = true
		}
	}
	return nil
}

func terminateProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

func requireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing executable artifact: %s", path)
	}
	if info.IsDir() {
		return fmt.Errorf("expected executable file, got directory: %s", path)
	}
	return nil
}

func requireDirWithIndex(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing ui_dist artifact: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("ui_dist is not a directory: %s", path)
	}
	index := filepath.Join(path, "index.html")
	if _, err := os.Stat(index); err != nil {
		return fmt.Errorf("ui_dist missing index.html: %s", index)
	}
	return nil
}

func requirePath(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("missing artifact path: %s", path)
	}
	return nil
}
