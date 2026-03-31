package autoswap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

func replIndexInfof(format string, args ...any) {
	msg := strings.TrimSpace(fmt.Sprintf(format, args...))
	if msg == "" {
		return
	}
	if logs.IsREPLContext() {
		logs.Info("DIALTONE_INDEX: %s", msg)
		return
	}
	logs.Info("%s", msg)
}

type runtimeManifest struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	ReleaseVersion   string `json:"release_version,omitempty"`
	ManifestAsset    string `json:"manifest_asset,omitempty"`
	ManifestSHA256   string `json:"manifest_sha256,omitempty"`
	ReleasePublished string `json:"release_published_at,omitempty"`
	Runtime          struct {
		Binary    string            `json:"binary"`
		Processes []manifestProcess `json:"processes"`
	} `json:"runtime"`
	Artifacts struct {
		Sync    map[string]string         `json:"sync"`
		Release map[string]releaseBinding `json:"release"`
	} `json:"artifacts"`
}

type manifestChannel struct {
	SchemaVersion  string `json:"schema_version"`
	Name           string `json:"name"`
	Channel        string `json:"channel"`
	Repo           string `json:"repo,omitempty"`
	ReleaseVersion string `json:"release_version"`
	ManifestURL    string `json:"manifest_url"`
	ManifestSHA256 string `json:"manifest_sha256,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
}

type manifestProcess struct {
	Name      string            `json:"name"`
	Artifact  string            `json:"artifact"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	DependsOn []string          `json:"depends_on"`
	Nix       *manifestNix      `json:"nix,omitempty"`
}

type manifestNix struct {
	Installable string   `json:"installable,omitempty"`
	Develop     string   `json:"develop,omitempty"`
	Command     []string `json:"command,omitempty"`
	Impure      bool     `json:"impure,omitempty"`
}

type releaseBinding struct {
	Asset          string            `json:"asset"`
	Type           string            `json:"type"`
	Extract        string            `json:"extract"`
	SHA256         string            `json:"sha256,omitempty"`
	SHA256ByTarget map[string]string `json:"sha256_by_target,omitempty"`
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
	replIndexInfof("autoswap stage: validating manifest inputs")
	manifestPath, err := materializeManifestForCompose(cfg)
	if err != nil {
		return err
	}
	replIndexInfof("autoswap stage: resolved manifest %s", manifestPath)
	art, err := loadAndValidateManifest(manifestPath, cfg.RepoRoot)
	if err != nil {
		return err
	}
	replIndexInfof("autoswap stage: artifact graph validated")
	logs.Info("autoswap stage OK manifest=%s", art.ManifestPath)
	logs.Info("autoswap stage artifacts robot=%s camera=%s mavlink=%s repl=%s ui=%s", art.RobotBin, art.CameraBin, art.MavlinkBin, art.ReplBin, art.UIDist)
	return nil
}

func Run(args []string) error {
	cfg, err := parseComposeFlags("run", args)
	if err != nil {
		return err
	}
	replIndexInfof("autoswap run: preparing manifest runtime")
	manifestPath, err := materializeManifestForCompose(cfg)
	if err != nil {
		return err
	}
	replIndexInfof("autoswap run: using manifest %s", manifestPath)
	art, err := loadAndValidateManifest(manifestPath, cfg.RepoRoot)
	if err != nil {
		return err
	}
	if len(art.Manifest.Runtime.Processes) > 0 {
		replIndexInfof("autoswap run: starting manifest-defined process graph")
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
	replIndexInfof("autoswap run: robot process started")
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
	replIndexInfof("autoswap run: sidecar processes started")
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
	replIndexInfof("autoswap run: composition healthy")
	if cfg.StayRunning {
		logs.Info("autoswap stay-running enabled; supervising manifest processes")
		replIndexInfof("autoswap run: supervising managed processes")
		return superviseProcesses(ctx, procs)
	}
	writeState(procs)
	return nil
}

func runManifestProcesses(cfg composeConfig, art resolvedArtifacts) error {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	expand := makeManifestExpander(art, cfg)
	if err := prebuildManifestNixInstallables(expand, art.Manifest.Runtime.Processes); err != nil {
		return err
	}

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
	runtimeStatePath := strings.TrimSpace(os.Getenv("AUTOSWAP_RUNTIME_STATE"))
	writeState := func() {
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
	writeState()

	baseURL := "http://127.0.0.1" + cfg.Listen
	if err := waitHTTP(ctx, baseURL+"/health", http.StatusOK); err != nil {
		return err
	}
	if cfg.RequireStream {
		_ = waitHTTP(ctx, baseURL+"/stream", http.StatusOK)
	}
	logs.Info("autoswap run OK: manifest composition healthy processes=%d", len(procs))
	replIndexInfof("autoswap run: manifest composition healthy processes=%d", len(procs))
	if cfg.StayRunning {
		replIndexInfof("autoswap run: supervising managed processes")
		return superviseProcesses(ctx, procs)
	}
	writeState()
	return nil
}

func buildManagedProcessesFromManifest(ctx context.Context, cfg composeConfig, art resolvedArtifacts) ([]managedProc, error) {
	ordered, err := orderManifestProcesses(art.Manifest.Runtime.Processes)
	if err != nil {
		return nil, err
	}
	expand := makeManifestExpander(art, cfg)
	out := make([]managedProc, 0, len(ordered))
	for _, p := range ordered {
		proc := p
		cmdArgs, envVars, err := resolveProcessInvocation(proc, expand, art)
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

func resolveProcessInvocation(p manifestProcess, expand func(string) string, art resolvedArtifacts) ([]string, []string, error) {
	cmdArgs := []string{}
	needsArgSeparator := false
	if p.Nix != nil {
		var err error
		cmdArgs, needsArgSeparator, err = resolveNixProcessInvocation(*p.Nix, expand)
		if err != nil {
			return nil, nil, fmt.Errorf("process %s nix invocation invalid: %w", p.Name, err)
		}
	} else if len(p.Command) > 0 {
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
	if needsArgSeparator && len(p.Args) > 0 {
		cmdArgs = append(cmdArgs, "--")
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

func makeManifestExpander(art resolvedArtifacts, cfg composeConfig) func(string) string {
	tokens := map[string]string{
		"listen":       cfg.Listen,
		"nats_port":    strconv.Itoa(cfg.NATSPort),
		"nats_ws_port": strconv.Itoa(cfg.NATSWSPort),
		"nats_url":     fmt.Sprintf("nats://127.0.0.1:%d", cfg.NATSPort),
		"repo_root":    art.RepoRoot,
	}
	for k, v := range art.Sync {
		tokens["artifact:"+k] = v
	}
	return func(v string) string {
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
}

func prebuildManifestNixInstallables(expand func(string) string, processes []manifestProcess) error {
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	installables := make([]string, 0, len(processes))
	for _, p := range processes {
		if p.Nix == nil {
			continue
		}
		installable := strings.TrimSpace(expand(p.Nix.Installable))
		if installable == "" || seen[installable] {
			continue
		}
		seen[installable] = true
		installables = append(installables, installable)
	}
	if len(installables) == 0 {
		return nil
	}
	args := []string{"--extra-experimental-features", "nix-command flakes", "build"}
	args = append(args, installables...)
	cmd := exec.Command(nixBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	return cmd.Run()
}

func resolveNixProcessInvocation(spec manifestNix, expand func(string) string) ([]string, bool, error) {
	nixBin, err := exec.LookPath("nix")
	if err != nil {
		return nil, false, fmt.Errorf("nix executable not found in PATH")
	}
	if installable := strings.TrimSpace(expand(spec.Installable)); installable != "" {
		args := []string{
			nixBin,
			"--extra-experimental-features", "nix-command flakes",
			"run",
		}
		if spec.Impure {
			args = append(args, "--impure")
		}
		args = append(args, installable)
		return args, true, nil
	}
	develop := strings.TrimSpace(expand(spec.Develop))
	if develop == "" {
		return nil, false, fmt.Errorf("set nix.installable or nix.develop")
	}
	if len(spec.Command) == 0 {
		return nil, false, fmt.Errorf("nix.develop requires nix.command")
	}
	args := []string{
		nixBin,
		"--extra-experimental-features", "nix-command flakes",
		"develop",
		develop,
	}
	if spec.Impure {
		args = append(args, "--impure")
	}
	args = append(args, "--command")
	for _, part := range spec.Command {
		args = append(args, expand(part))
	}
	return args, false, nil
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
		resolveLocalManifestPath(cfg.ManifestPath, cfg.RepoRoot),
		cfg.ManifestURL,
		filepath.Join(userHomeDir(), ".dialtone", "autoswap", "manifests"),
		token,
	)
}

func resolveLocalManifestPath(manifestPath, repoRoot string) string {
	manifestPath = strings.TrimSpace(manifestPath)
	if manifestPath == "" || filepath.IsAbs(manifestPath) {
		return manifestPath
	}
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return manifestPath
	}
	return filepath.Join(repoRoot, manifestPath)
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
	return resolveManifestPathRecursive(url, manifestDir, token, 0)
}

func resolveManifestPathRecursive(sourceURL, manifestDir, token string, depth int) (string, error) {
	if depth > 4 {
		return "", fmt.Errorf("manifest resolution exceeded redirect depth")
	}
	tmpPath := filepath.Join(manifestDir, fmt.Sprintf("manifest-fetch-%d.tmp", time.Now().UnixNano()))
	if err := downloadFile(sourceURL, token, tmpPath); err != nil {
		return "", fmt.Errorf("download manifest failed: %w", err)
	}
	raw, err := os.ReadFile(tmpPath)
	_ = os.Remove(tmpPath)
	if err != nil {
		return "", err
	}
	return materializeRemoteManifest(raw, sourceURL, manifestDir, token, depth)
}

func materializeRemoteManifest(raw []byte, sourceURL, manifestDir, token string, depth int) (string, error) {
	trimmed := []byte(strings.TrimSpace(string(raw)))
	if len(trimmed) == 0 {
		return "", fmt.Errorf("manifest download from %s was empty", sourceURL)
	}

	var probe map[string]any
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return "", fmt.Errorf("manifest parse failed: %w", err)
	}
	if manifestURL, ok := probe["manifest_url"].(string); ok && strings.TrimSpace(manifestURL) != "" && probe["runtime"] == nil {
		var channel manifestChannel
		if err := json.Unmarshal(trimmed, &channel); err != nil {
			return "", fmt.Errorf("manifest channel parse failed: %w", err)
		}
		nextURL := strings.TrimSpace(channel.ManifestURL)
		if nextURL == "" {
			return "", fmt.Errorf("manifest channel missing manifest_url")
		}
		resolvedPath, err := resolveManifestPathRecursive(nextURL, manifestDir, token, depth+1)
		if err != nil {
			return "", err
		}
		if expected := normalizeSHA256(channel.ManifestSHA256); expected != "" {
			got, err := sha256File(resolvedPath)
			if err != nil {
				return "", err
			}
			if !strings.EqualFold(expected, got) {
				return "", fmt.Errorf("manifest channel sha256 mismatch: expected=%s got=%s", expected, got)
			}
		}
		return resolvedPath, nil
	}

	var mf runtimeManifest
	if err := json.Unmarshal(trimmed, &mf); err != nil {
		return "", fmt.Errorf("manifest parse failed: %w", err)
	}
	sum := sha256.Sum256(trimmed)
	fileName := fmt.Sprintf("manifest-%s.json", hex.EncodeToString(sum[:8]))
	finalPath := filepath.Join(manifestDir, fileName)
	if err := os.WriteFile(finalPath, append(trimmed, '\n'), 0o644); err != nil {
		return "", err
	}
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
