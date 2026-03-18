package autoswap

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshplugin "dialtone/dev/plugins/ssh/src_v1/go"
)

type deployOptions struct {
	Host          string
	Port          string
	User          string
	Pass          string
	RemoteRepo    string
	InstallDir    string
	Service       bool
	Repo          string
	CheckInterval time.Duration
	ManifestPath  string
	ManifestURL   string
	Listen        string
	NATSPort      int
	NATSWSPort    int
}

func RunDeploy(args []string) error {
	fs := flag.NewFlagSet("autoswap-deploy", flag.ContinueOnError)
	host := fs.String("host", strings.TrimSpace(os.Getenv("ROBOT_HOST")), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", strings.TrimSpace(os.Getenv("ROBOT_USER")), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password (optional when SSH key auth is configured)")
	remoteRepo := fs.String("remote-repo", "", "Remote repo root (default: /home/<user>/dialtone)")
	installDir := fs.String("install-dir", "", "Remote autoswap install dir (default: /home/<user>/.dialtone/autoswap)")
	service := fs.Bool("service", false, "Install/start autoswap service on remote host")
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name for update polling")
	checkInterval := fs.Duration("check-interval", 5*time.Minute, "GitHub poll interval")
	manifest := fs.String("manifest", "src/plugins/robot/src_v2/config/composition.manifest.json", "Manifest path relative to remote repo src/")
	manifestURL := fs.String("manifest-url", "", "Manifest URL for autoswap service (overrides --manifest)")
	listen := fs.String("listen", ":18086", "Robot listen address used by autoswap run")
	natsPort := fs.Int("nats-port", 18236, "Robot embedded NATS port used by autoswap run")
	natsWSPort := fs.Int("nats-ws-port", 18237, "Robot embedded NATS websocket port used by autoswap run")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := deployOptions{
		Host:          strings.TrimSpace(*host),
		Port:          strings.TrimSpace(*port),
		User:          strings.TrimSpace(*user),
		Pass:          *pass,
		RemoteRepo:    strings.TrimSpace(*remoteRepo),
		InstallDir:    strings.TrimSpace(*installDir),
		Service:       *service,
		Repo:          strings.TrimSpace(*repo),
		CheckInterval: *checkInterval,
		ManifestPath:  strings.TrimSpace(*manifest),
		ManifestURL:   strings.TrimSpace(*manifestURL),
		Listen:        strings.TrimSpace(*listen),
		NATSPort:      *natsPort,
		NATSWSPort:    *natsWSPort,
	}
	if opts.Port == "" {
		opts.Port = "22"
	}
	if opts.Host == "" {
		return fmt.Errorf("deploy requires --host (or ROBOT_HOST env)")
	}
	replIndexInfof("autoswap deploy: preparing service on %s", opts.Host)

	node, err := sshplugin.ResolveMeshNode(opts.Host)
	if err != nil {
		return fmt.Errorf("deploy requires a mesh node alias for --host (e.g. rover/chroma/darkmac/legion): %w", err)
	}
	if opts.User == "" {
		opts.User = node.User
	}
	if opts.Port == "" {
		opts.Port = node.Port
	}
	if opts.RemoteRepo == "" {
		switch strings.ToLower(node.OS) {
		case "macos":
			opts.RemoteRepo = filepath.ToSlash(filepath.Join("/Users", opts.User, "dialtone"))
		default:
			opts.RemoteRepo = filepath.ToSlash(filepath.Join("/home", opts.User, "dialtone"))
		}
	}
	if opts.InstallDir == "" {
		switch strings.ToLower(node.OS) {
		case "macos":
			opts.InstallDir = filepath.ToSlash(filepath.Join("/Users", opts.User, ".dialtone", "autoswap"))
		default:
			opts.InstallDir = filepath.ToSlash(filepath.Join("/home", opts.User, ".dialtone", "autoswap"))
		}
	}
	if strings.TrimSpace(opts.User) == "" {
		return fmt.Errorf("deploy requires --user or a mesh node with a default user")
	}
	if opts.Service && strings.TrimSpace(opts.ManifestURL) == "" {
		opts.ManifestURL = fmt.Sprintf("https://github.com/%s/releases/latest/download/robot_src_v2_channel.json", opts.Repo)
		logs.Info("[DEPLOY] manifest-url not provided; using latest release URL: %s", opts.ManifestURL)
	}
	if opts.Service {
		if normalized, changed := normalizeManifestURLForAutoUpdate(opts.ManifestURL, opts.Repo); changed {
			logs.Info("[DEPLOY] normalized manifest-url to auto-update latest: %s -> %s", opts.ManifestURL, normalized)
			opts.ManifestURL = normalized
		}
	}

	cmdOpts := sshplugin.CommandOptions{
		User:     opts.User,
		Port:     opts.Port,
		Password: opts.Pass,
	}
	logs.Info("[DEPLOY] Connecting to mesh node=%s as %s", node.Name, opts.User)
	replIndexInfof("autoswap deploy: connecting to %s", node.Name)

	goos, goarch, err := detectRemoteTarget(node.Name, cmdOpts)
	if err != nil {
		return err
	}
	logs.Info("[DEPLOY] Remote target detected: %s/%s", goos, goarch)
	replIndexInfof("autoswap deploy: remote target is %s/%s", goos, goarch)

	localBinary, err := buildDeployBinary(goos, goarch)
	if err != nil {
		return err
	}
	remoteBinDir := filepath.ToSlash(filepath.Join(opts.InstallDir, "bin"))
	remoteBin := filepath.ToSlash(filepath.Join(remoteBinDir, "dialtone_autoswap_v1"))
	remoteTmpBin := filepath.ToSlash(filepath.Join(remoteBinDir, fmt.Sprintf("dialtone_autoswap_v1.upload-%d", time.Now().UnixNano())))
	if _, err := sshplugin.RunNodeCommand(node.Name, "mkdir -p "+shellQuote(remoteBinDir), cmdOpts); err != nil {
		return fmt.Errorf("failed to create remote bin dir: %w", err)
	}
	if err := sshplugin.UploadNodeFile(node.Name, localBinary, remoteTmpBin, cmdOpts); err != nil {
		return fmt.Errorf("failed to upload binary: %w", err)
	}
	if _, err := sshplugin.RunNodeCommand(node.Name, "chmod +x "+shellQuote(remoteTmpBin)+" && mv -f "+shellQuote(remoteTmpBin)+" "+shellQuote(remoteBin), cmdOpts); err != nil {
		return fmt.Errorf("failed to activate remote binary: %w", err)
	}
	logs.Info("[DEPLOY] Uploaded %s", remoteBin)
	replIndexInfof("autoswap deploy: uploaded runtime to %s", node.Name)

	if !opts.Service {
		replIndexInfof("autoswap deploy: completed")
		return nil
	}

	remoteSrc := filepath.ToSlash(filepath.Join(opts.RemoteRepo, "src"))
	manifestFlag := ""
	repoRootFlag := ""
	if strings.TrimSpace(opts.ManifestURL) != "" {
		manifestFlag = "--manifest-url " + shellQuoteArg(opts.ManifestURL)
	} else {
		remoteManifest := filepath.ToSlash(filepath.Join("plugins", strings.TrimPrefix(strings.TrimSpace(opts.ManifestPath), "src/plugins/")))
		if strings.HasPrefix(opts.ManifestPath, "plugins/") {
			remoteManifest = opts.ManifestPath
		}
		if strings.HasPrefix(opts.ManifestPath, "src/") {
			remoteManifest = strings.TrimPrefix(opts.ManifestPath, "src/")
		}
		remoteManifestAbs := filepath.ToSlash(filepath.Join(remoteSrc, remoteManifest))
		manifestFlag = "--manifest " + shellQuoteArg(remoteManifestAbs)
		repoRootFlag = "--repo-root " + shellQuoteArg(opts.RemoteRepo)
	}
	if repoRootFlag == "" && strings.TrimSpace(opts.RemoteRepo) != "" {
		repoRootFlag = "--repo-root " + shellQuoteArg(opts.RemoteRepo)
	}

	serviceCmdParts := []string{
		"mkdir -p " + shellQuote(opts.InstallDir),
		shellQuote(remoteBin) + " service --mode install " +
			"--repo " + shellQuoteArg(opts.Repo) + " " +
			"--check-interval " + shellQuoteArg(opts.CheckInterval.String()) + " " +
			"--install-dir " + shellQuoteArg(opts.InstallDir) + " " +
			manifestFlag + " " +
			repoRootFlag + " " +
			"--listen " + shellQuoteArg(opts.Listen) + " " +
			fmt.Sprintf("--nats-port %d ", opts.NATSPort) +
			fmt.Sprintf("--nats-ws-port %d ", opts.NATSWSPort) +
			"--require-stream=true",
	}
	serviceCmd := strings.Join(serviceCmdParts, " && ")
	if _, err := sshplugin.RunNodeCommand(node.Name, serviceCmd, cmdOpts); err != nil {
		return fmt.Errorf("remote service install failed: %w", err)
	}
	logs.Info("[DEPLOY] Installed autoswap service on %s", opts.Host)
	if strings.TrimSpace(opts.ManifestURL) != "" {
		replIndexInfof("autoswap deploy: manifest source is %s", opts.ManifestURL)
	}
	replIndexInfof("autoswap deploy: service installed on %s", node.Name)
	return nil
}

func buildDeployBinary(goos, goarch string) (string, error) {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return "", err
	}
	out := filepath.Join(rt.RepoRoot, "bin", fmt.Sprintf("dialtone_autoswap-%s-%s", goos, goarch))
	if goos == "windows" {
		out += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		fallback, lookErr := exec.LookPath("go")
		if lookErr != nil {
			return "", fmt.Errorf("managed go binary not found at %s and fallback go not in PATH", goBin)
		}
		goBin = fallback
	}
	cmd := exec.Command(goBin, "build", "-o", out, "./plugins/autoswap/src_v1/cmd/main.go")
	cmd.Dir = filepath.Join(rt.RepoRoot, "src")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out, nil
}

func detectRemoteTarget(node string, opts sshplugin.CommandOptions) (string, string, error) {
	osOut, err := sshplugin.RunNodeCommand(node, "uname -s", opts)
	if err != nil {
		return "", "", fmt.Errorf("detect remote os failed: %w", err)
	}
	archOut, err := sshplugin.RunNodeCommand(node, "uname -m", opts)
	if err != nil {
		return "", "", fmt.Errorf("detect remote arch failed: %w", err)
	}
	osName := strings.ToLower(strings.TrimSpace(osOut))
	archName := strings.ToLower(strings.TrimSpace(archOut))
	goos := "linux"
	switch osName {
	case "linux":
		goos = "linux"
	case "darwin":
		goos = "darwin"
	default:
		return "", "", fmt.Errorf("unsupported remote OS %q", osName)
	}
	goarch := "arm64"
	switch archName {
	case "aarch64", "arm64":
		goarch = "arm64"
	case "armv7l", "arm":
		goarch = "arm"
	case "x86_64", "amd64":
		goarch = "amd64"
	default:
		return "", "", fmt.Errorf("unsupported remote arch %q", archName)
	}
	return goos, goarch, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func shellQuoteArg(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "'", "")
}
