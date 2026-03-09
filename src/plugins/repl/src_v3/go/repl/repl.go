package repl

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	replv1 "dialtone/dev/plugins/repl/src_v1/go/repl"
	"github.com/nats-io/nats.go"
)

const (
	defaultNATSURL = "nats://127.0.0.1:4222"
	defaultRoom    = "index"
	commandSubject = "repl.cmd"
)

type busFrame struct {
	Type      string `json:"type"`
	From      string `json:"from,omitempty"`
	Room      string `json:"room,omitempty"`
	Version   string `json:"version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type dialtoneConfig struct {
	DialtoneEnv      string     `json:"DIALTONE_ENV,omitempty"`
	DialtoneRepoRoot string     `json:"DIALTONE_REPO_ROOT,omitempty"`
	DialtoneUseNix   string     `json:"DIALTONE_USE_NIX,omitempty"`
	MeshNodes        []meshNode `json:"mesh_nodes,omitempty"`
}

type meshNode struct {
	Name                string   `json:"name"`
	Aliases             []string `json:"aliases,omitempty"`
	User                string   `json:"user"`
	Host                string   `json:"host"`
	HostCandidates      []string `json:"host_candidates,omitempty"`
	RoutePreference     []string `json:"route_preference,omitempty"`
	Port                string   `json:"port,omitempty"`
	OS                  string   `json:"os,omitempty"`
	PreferWSLPowerShell bool     `json:"prefer_wsl_powershell,omitempty"`
	RepoCandidates      []string `json:"repo_candidates,omitempty"`
}

func Run(args []string) error {
	fs := flag.NewFlagSet("repl-v3-run", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared room name")
	name := fs.String("name", replv1.DefaultPromptName(), "Prompt name for this client")
	isTest := fs.Bool("test", false, "Run REPL v3 end-to-end tests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *isTest {
		return RunTest(fs.Args())
	}
	if err := EnsureLeaderRunning(strings.TrimSpace(*natsURL), strings.TrimSpace(*room)); err != nil {
		return err
	}
	joinArgs := []string{
		"--nats-url", strings.TrimSpace(*natsURL),
		"--room", strings.TrimSpace(*room),
		"--name", strings.TrimSpace(*name),
	}
	return replv1.RunJoin(joinArgs)
}

func RunLeader(args []string) error {
	return replv1.RunLeader(args)
}

func RunJoin(args []string) error {
	return replv1.RunJoin(args)
}

func RunStatus(args []string) error {
	return replv1.RunStatus(args)
}

func RunService(args []string) error {
	return replv1.RunService(args)
}

func RunInstall(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "version")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logs.Info("repl src_v3 install: verifying Go toolchain at %s", goBin)
	return cmd.Run()
}

func RunFormat(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "fmt", "./plugins/repl/...")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunBuild(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "build",
		"./plugins/repl/scaffold",
		"./plugins/repl/src_v1/cmd/repld",
		"./plugins/repl/src_v3/go/repl",
		"./plugins/repl/src_v3/test/cmd",
	)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "vet", "./plugins/repl/...")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunCheck(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "test",
		"./plugins/repl/src_v3/go/repl",
		"./plugins/repl/scaffold",
	)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunBootstrap(args []string) error {
	fs := flag.NewFlagSet("repl-v3-bootstrap", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "Apply host onboarding changes")
	wslHost := fs.String("wsl-host", "wsl.shad-artichoke.ts.net", "WSL host DNS name")
	wslUser := fs.String("wsl-user", "user", "WSL ssh username")
	if err := fs.Parse(args); err != nil {
		return err
	}
	logs.Raw("DIALTONE v3 bootstrap guide")
	logs.Raw("  1. ./dialtone.sh")
	logs.Raw("  2. ./dialtone.sh repl src_v3 inject --user llm-codex repl src_v3 bootstrap --apply --wsl-host %s --wsl-user %s", strings.TrimSpace(*wslHost), strings.TrimSpace(*wslUser))
	logs.Raw("  3. ./dialtone.sh repl src_v3 inject --user llm-codex ssh src_v1 run --host wsl --cmd whoami")
	if !*apply {
		logs.Raw("  (dry-run) pass --apply to create/update mesh host entry named 'wsl'")
		return nil
	}
	return AddHost([]string{
		"--name", "wsl",
		"--host", strings.TrimSpace(*wslHost),
		"--user", strings.TrimSpace(*wslUser),
		"--port", "22",
		"--os", "linux",
		"--alias", "wsl",
		"--route", "tailscale,private",
	})
}

func AddHost(args []string) error {
	fs := flag.NewFlagSet("repl-v3-add-host", flag.ContinueOnError)
	name := fs.String("name", "", "Mesh host alias")
	host := fs.String("host", "", "Host or DNS")
	user := fs.String("user", "", "SSH user")
	port := fs.String("port", "22", "SSH port")
	osName := fs.String("os", "linux", "Host OS")
	alias := fs.String("alias", "", "Comma-separated aliases")
	route := fs.String("route", "tailscale,private", "Comma-separated route preference")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*host) == "" || strings.TrimSpace(*user) == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 add-host --name wsl --host <host> --user <user>")
	}
	cfgPath, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = dialtoneConfig{
				DialtoneEnv:      strings.TrimSpace(os.Getenv("DIALTONE_ENV")),
				DialtoneRepoRoot: strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")),
				DialtoneUseNix:   strings.TrimSpace(os.Getenv("DIALTONE_USE_NIX")),
			}
		} else {
			return err
		}
	}
	n := meshNode{
		Name:            strings.TrimSpace(*name),
		User:            strings.TrimSpace(*user),
		Host:            strings.TrimSpace(*host),
		Port:            strings.TrimSpace(*port),
		OS:              strings.TrimSpace(*osName),
		Aliases:         parseCSV(*alias),
		RoutePreference: parseCSV(*route),
	}
	if len(n.Aliases) == 0 {
		n.Aliases = []string{n.Name}
	}
	if len(n.RoutePreference) == 0 {
		n.RoutePreference = []string{"tailscale", "private"}
	}
	n.HostCandidates = []string{n.Host}
	upserted := false
	for i := range cfg.MeshNodes {
		if strings.EqualFold(strings.TrimSpace(cfg.MeshNodes[i].Name), n.Name) {
			cfg.MeshNodes[i] = n
			upserted = true
			break
		}
	}
	if !upserted {
		cfg.MeshNodes = append(cfg.MeshNodes, n)
	}
	if err := saveConfig(cfgPath, cfg); err != nil {
		return err
	}
	if upserted {
		logs.Info("Updated mesh host %s (%s@%s:%s)", n.Name, n.User, n.Host, n.Port)
	} else {
		logs.Info("Added mesh host %s (%s@%s:%s)", n.Name, n.User, n.Host, n.Port)
	}
	logs.Info("You can now run: ./dialtone.sh ssh src_v1 run --host %s --cmd whoami", n.Name)
	return nil
}

func Inject(args []string) error {
	fs := flag.NewFlagSet("repl-v3-inject", flag.ContinueOnError)
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL")
	room := fs.String("room", defaultRoom, "Shared room name")
	user := fs.String("user", "llm-codex", "Logical user name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if command == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 inject --user <name> [--nats-url URL] [--room ROOM] <command>")
	}
	return InjectCommand(strings.TrimSpace(*natsURL), strings.TrimSpace(*room), strings.TrimSpace(*user), command)
}

func InjectCommand(natsURL, room, user, command string) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("command is required")
	}
	if strings.TrimSpace(natsURL) == "" {
		natsURL = defaultNATSURL
	}
	if strings.TrimSpace(room) == "" {
		room = defaultRoom
	}
	if strings.TrimSpace(user) == "" {
		user = "llm-codex"
	}
	command = strings.TrimPrefix(strings.TrimSpace(command), "/")
	if err := EnsureLeaderRunning(natsURL, room); err != nil {
		return err
	}
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	frame := busFrame{
		Type:      "command",
		From:      strings.TrimSpace(user),
		Room:      strings.TrimSpace(room),
		Version:   "src_v3",
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Message:   command,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := nc.Publish(commandSubject, raw); err != nil {
		return err
	}
	return nc.FlushTimeout(1500 * time.Millisecond)
}

func RunTest(args []string) error {
	if strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_MODE")) == "inside" {
		return runInRepoTest(args)
	}
	return runTmpBootstrapTest(args)
}

func RunTestClean(args []string) error {
	fs := flag.NewFlagSet("repl-v3-test-clean", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "List temp folders without deleting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	tmpRoot := strings.TrimSpace(os.TempDir())
	if tmpRoot == "" {
		return fmt.Errorf("temp directory is empty")
	}
	entries, err := os.ReadDir(tmpRoot)
	if err != nil {
		return err
	}
	const prefix = "dialtone-repl-v3-bootstrap-"
	matches := make([]string, 0, 16)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		matches = append(matches, filepath.Join(tmpRoot, name))
	}
	if len(matches) == 0 {
		logs.Info("test-clean: no %s* folders found in %s", prefix, tmpRoot)
		return nil
	}
	if *dryRun {
		for _, p := range matches {
			logs.Info("test-clean dry-run: %s", p)
		}
		logs.Info("test-clean dry-run complete: %d folder(s) matched", len(matches))
		return nil
	}
	removed := 0
	for _, p := range matches {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		removed++
		logs.Info("test-clean removed: %s", p)
	}
	logs.Info("test-clean complete: %d folder(s) removed", removed)
	return nil
}

func RunProcessClean(args []string) error {
	fs := flag.NewFlagSet("repl-v3-process-clean", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "List matching processes without killing them")
	includeChrome := fs.Bool("include-chrome", false, "Also kill chrome-v1 service processes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	type pattern struct {
		label string
		expr  string
	}
	patterns := []pattern{
		{label: "repl-v3-leader", expr: `plugins/repl/scaffold/main.go src_v3 leader`},
		{label: "repl-v3-leader-bin", expr: `src_v3 leader --embedded-nats`},
		{label: "dialtone-tap", expr: `dialtone-tap`},
		{label: "stuck-tsnet-shell", expr: `dialtone\.sh tsnet src_v1 up`},
		{label: "stuck-tsnet-go", expr: `go run dev\.go tsnet src_v1 up`},
	}
	if *includeChrome {
		patterns = append(patterns,
			pattern{label: "chrome-v1-service", expr: `/tmp/dialtone/chrome-v1/chrome-v1-service`},
			pattern{label: "chrome-v1-role", expr: `--dialtone-role=chrome-v1-service`},
		)
	}

	totalFound := 0
	totalKilled := 0
	for _, p := range patterns {
		found, err := pgrepCount(p.expr)
		if err != nil {
			return err
		}
		if found == 0 {
			continue
		}
		totalFound += found
		if *dryRun {
			logs.Info("process-clean dry-run: %s matched %d process(es)", p.label, found)
			continue
		}
		killed, err := pkillCount(p.expr)
		if err != nil {
			return err
		}
		totalKilled += killed
		logs.Info("process-clean: %s killed %d process(es)", p.label, killed)
	}

	if *dryRun {
		logs.Info("process-clean dry-run complete: %d process(es) matched", totalFound)
		return nil
	}
	logs.Info("process-clean complete: %d matched, %d killed", totalFound, totalKilled)
	return nil
}

func runInRepoTest(args []string) error {
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmdArgs := []string{"run", "./plugins/repl/src_v3/test/cmd/main.go"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "DIALTONE_REPO_ROOT="+repoRoot, "DIALTONE_SRC_ROOT="+srcRoot)
	return cmd.Run()
}

func runTmpBootstrapTest(args []string) error {
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return err
	}
	tmpRoot, err := os.MkdirTemp("", "dialtone-repl-v3-bootstrap-*")
	if err != nil {
		return err
	}
	tmpRepo := filepath.Join(tmpRoot, "repo")
	tmpEnv := filepath.Join(tmpRoot, "dialtone_env")
	if err := os.MkdirAll(tmpRepo, 0o755); err != nil {
		return err
	}

	srcDialtone := filepath.Join(repoRoot, "dialtone.sh")
	dstDialtone := filepath.Join(tmpRepo, "dialtone.sh")
	if err := copyFile(srcDialtone, dstDialtone, 0o755); err != nil {
		return err
	}

	repoTar := filepath.Join(tmpRoot, "dialtone-local.tar.gz")
	if err := createRepoTarball(repoRoot, repoTar); err != nil {
		return err
	}
	repoURL, closeServer, err := startLocalTarServer(repoTar)
	if err != nil {
		return err
	}
	defer closeServer()

	logs.Info("REPL v3 bootstrap test temp root: %s", tmpRoot)
	logs.Info("REPL v3 bootstrap test command: (cd %s && ./dialtone.sh --test)", tmpRepo)
	logs.Info("REPL v3 bootstrap test repo URL: %s", repoURL)
	logs.Info("REPL v3 bootstrap inject demo command:")
	logs.Info("  ./dialtone.sh repl src_v3 inject --user llm-codex repl src_v3 bootstrap --apply --wsl-host wsl.shad-artichoke.ts.net --wsl-user user")

	cmd := exec.Command("./dialtone.sh", "--test")
	cmd.Dir = tmpRepo
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"TEST_ANS_ENV="+tmpEnv,
		"TEST_ANS_REPO="+tmpRepo,
		"DIALTONE_USE_NIX=0",
		"DIALTONE_BOOTSTRAP_REPO_URL="+repoURL,
		"DIALTONE_REPL_V3_TEST_MODE=inside",
		"DIALTONE_REPL_NATS_URL=nats://127.0.0.1:47222",
	)
	cmd.Args = append(cmd.Args, args...)
	return cmd.Run()
}

func resolveRoots() (repoRoot, srcRoot string, err error) {
	cwd, e := os.Getwd()
	if e != nil {
		return "", "", e
	}
	abs, _ := filepath.Abs(cwd)
	if filepath.Base(abs) == "src" {
		return filepath.Dir(abs), abs, nil
	}
	repoGuess := abs
	if _, statErr := os.Stat(filepath.Join(repoGuess, "src")); statErr != nil {
		return "", "", fmt.Errorf("unable to resolve repo root from %s", abs)
	}
	return repoGuess, filepath.Join(repoGuess, "src"), nil
}

func EnsureLeaderRunning(clientNATSURL, room string) error {
	clientNATSURL = strings.TrimSpace(clientNATSURL)
	if clientNATSURL == "" {
		clientNATSURL = defaultNATSURL
	}
	if strings.TrimSpace(room) == "" {
		room = defaultRoom
	}
	if endpointReachable(clientNATSURL, 700*time.Millisecond) {
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
	listenURL := listenURLFromClientURL(clientNATSURL)
	cmd := exec.Command(goBin, "run", "./plugins/repl/scaffold/main.go", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--room", room,
		"--hostname", "DIALTONE-SERVER",
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
		if endpointReachable(clientNATSURL, 600*time.Millisecond) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("repl v3 leader did not start nats endpoint at %s", clientNATSURL)
}

func endpointReachable(natsURL string, timeout time.Duration) bool {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return false
	}
	host := strings.TrimSpace(u.Hostname())
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func listenURLFromClientURL(clientURL string) string {
	u, err := url.Parse(strings.TrimSpace(clientURL))
	if err != nil {
		return "nats://0.0.0.0:4222"
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	return "nats://0.0.0.0:" + port
}

func resolveConfigPath() (string, error) {
	raw := strings.TrimSpace(os.Getenv("DIALTONE_MESH_CONFIG"))
	if raw != "" {
		return raw, nil
	}
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, "env", "dialtone.json"), nil
}

func loadConfig(path string) (dialtoneConfig, error) {
	var cfg dialtoneConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func saveConfig(path string, cfg dialtoneConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func parseCSV(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func copyFile(src, dst string, mode os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, mode)
}

func createRepoTarball(repoRoot, tarPath string) error {
	lsCmd := exec.Command("git", "-C", repoRoot, "ls-files", "--cached", "--modified", "--others", "--exclude-standard", "-z")
	files, err := lsCmd.Output()
	if err != nil {
		return fmt.Errorf("git ls-files failed: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("git ls-files returned no files for tarball")
	}

	out, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	entries := bytes.Split(files, []byte{0})
	for _, e := range entries {
		rel := strings.TrimSpace(string(e))
		if rel == "" {
			continue
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimPrefix(rel, "./")
		if rel == "" || strings.HasPrefix(rel, "../") {
			continue
		}
		absPath := filepath.Join(repoRoot, filepath.FromSlash(rel))
		info, statErr := os.Lstat(absPath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				logs.Warn("repl src_v3 tar skip missing path: %s", rel)
				continue
			}
			return statErr
		}

		hdrName := "dialtone-main/" + rel
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(absPath)
			if readErr != nil {
				return readErr
			}
			hdr := &tar.Header{
				Name:     hdrName,
				Mode:     0o777,
				Typeflag: tar.TypeSymlink,
				Linkname: target,
				ModTime:  info.ModTime(),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		hdr, hdrErr := tar.FileInfoHeader(info, "")
		if hdrErr != nil {
			return hdrErr
		}
		hdr.Name = hdrName
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, openErr := os.Open(absPath)
		if openErr != nil {
			return openErr
		}
		if _, copyErr := io.Copy(tw, f); copyErr != nil {
			_ = f.Close()
			return copyErr
		}
		_ = f.Close()
	}
	return nil
}

func startLocalTarServer(tarPath string) (string, func(), error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/dialtone-main.tar.gz", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, tarPath)
	})
	srv := &http.Server{Handler: mux}
	go func() {
		_ = srv.Serve(ln)
	}()
	closeFn := func() {
		_ = srv.Close()
		_ = ln.Close()
	}
	url := "http://" + ln.Addr().String() + "/dialtone-main.tar.gz"
	return url, closeFn, nil
}

func resolveGoBin() (string, error) {
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin != "" {
		return goBin, nil
	}
	path, err := exec.LookPath("go")
	if err != nil {
		return "", fmt.Errorf("go binary not found (DIALTONE_GO_BIN unset and go not in PATH)")
	}
	return path, nil
}

func pgrepCount(expr string) (int, error) {
	cmd := exec.Command("pgrep", "-f", expr)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return 0, nil
		}
		return 0, err
	}
	return countNonEmptyLines(string(out)), nil
}

func pkillCount(expr string) (int, error) {
	before, err := pgrepCount(expr)
	if err != nil {
		return 0, err
	}
	if before == 0 {
		return 0, nil
	}
	cmd := exec.Command("pkill", "-f", expr)
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return 0, nil
		}
		return 0, err
	}
	after, err := pgrepCount(expr)
	if err != nil {
		return 0, err
	}
	if before < after {
		return 0, nil
	}
	return before - after, nil
}

func countNonEmptyLines(raw string) int {
	n := 0
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}
