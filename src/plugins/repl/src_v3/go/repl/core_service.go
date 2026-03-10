package repl

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

var BuildVersion = "dev"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type serviceManager struct {
	mu      sync.Mutex
	worker  *exec.Cmd
	version string
	path    string
}

type serviceOptions struct {
	Mode           string
	Repo           string
	NATSURL        string
	Room           string
	HostName       string
	CheckInterval  time.Duration
	InstallDir     string
	TokenEnv       string
	EmbeddedNATS   bool
	TSNet          bool
	TSNetNATSPort  int
	AllowDowngrade bool
}

func RunService(args []string) error {
	fs := flag.NewFlagSet("repl-service", flag.ContinueOnError)
	mode := fs.String("mode", "install", "Service mode: install|run|status")
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name")
	natsURL := fs.String("nats-url", defaultNATSURL, "NATS URL for worker leader")
	room := fs.String("room", defaultRoom, "REPL room for worker leader")
	hostname := fs.String("hostname", DefaultPromptName(), "Host name used by worker leader")
	checkInterval := fs.Duration("check-interval", 5*time.Minute, "Update check interval")
	installDir := fs.String("install-dir", filepath.Join(userHomeDir(), ".dialtone", "repl"), "Service install directory")
	tokenEnv := fs.String("token-env", "GITHUB_TOKEN", "Token env var used for GitHub API")
	embeddedNATS := fs.Bool("embedded-nats", true, "Pass --embedded-nats to worker")
	enableTSNet := fs.Bool("tsnet", true, "Pass --tsnet to worker leader (auto-skips when native tailscale is already connected)")
	tsnetNATSPort := fs.Int("tsnet-nats-port", 0, "Pass --tsnet-nats-port to worker leader")
	allowDowngrade := fs.Bool("allow-downgrade", false, "Allow replacing worker with older version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := serviceOptions{
		Mode:           strings.TrimSpace(*mode),
		Repo:           strings.TrimSpace(*repo),
		NATSURL:        strings.TrimSpace(*natsURL),
		Room:           sanitizeRoom(*room),
		HostName:       normalizePromptName(*hostname),
		CheckInterval:  *checkInterval,
		InstallDir:     strings.TrimSpace(*installDir),
		TokenEnv:       strings.TrimSpace(*tokenEnv),
		EmbeddedNATS:   *embeddedNATS,
		TSNet:          *enableTSNet,
		TSNetNATSPort:  *tsnetNATSPort,
		AllowDowngrade: *allowDowngrade,
	}
	if opts.Mode == "" {
		opts.Mode = "install"
	}

	switch opts.Mode {
	case "run":
		return runServiceSupervisor(opts)
	case "install":
		return installService(opts)
	case "status":
		return serviceStatus(opts)
	default:
		return fmt.Errorf("unsupported service mode %q (expected install|run|status)", opts.Mode)
	}
}

func runServiceSupervisor(opts serviceOptions) error {
	if err := os.MkdirAll(opts.InstallDir, 0o755); err != nil {
		return err
	}

	subdir := filepath.Join(opts.InstallDir, "releases")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		return err
	}
	currentLink := filepath.Join(opts.InstallDir, "current")
	assetName := localAssetName()
	token := strings.TrimSpace(os.Getenv(opts.TokenEnv))

	rel, asset, err := latestRelease(opts.Repo, token, assetName)
	workerVersion := ""
	workerPath := ""
	if err != nil || strings.TrimSpace(rel.TagName) == "" {
		if err != nil {
			logs.Warn("initial release lookup failed, starting local worker and retrying updates: %v", err)
		} else {
			logs.Warn("latest release has empty tag; starting local worker and retrying updates")
		}
		localPath, localErr := ensureLocalWorkerBinary(opts.InstallDir, assetName)
		if localErr != nil {
			return fmt.Errorf("query latest release failed (%v), and local worker seed failed: %w", err, localErr)
		}
		workerVersion = BuildVersion
		workerPath = localPath
	} else {
		workerPath, err = ensureReleaseBinary(opts.InstallDir, rel.TagName, assetName, asset.BrowserDownloadURL, token)
		if err != nil {
			return err
		}
		workerVersion = rel.TagName
	}
	if err := switchCurrentLink(currentLink, workerPath); err != nil {
		return err
	}

	mgr := &serviceManager{version: workerVersion, path: workerPath}
	if err := mgr.startWorker(currentLink, opts); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(opts.CheckInterval)
	defer ticker.Stop()

	logs.Info("REPL service active: repo=%s room=%s nats=%s version=%s asset=%s", opts.Repo, opts.Room, opts.NATSURL, mgr.version, assetName)
	stopPresence := startDaemonPresenceLoop(ctx, opts, mgr)
	defer stopPresence()

	for {
		select {
		case <-ctx.Done():
			mgr.stopWorker(10 * time.Second)
			return nil
		case <-ticker.C:
			latest, latestAsset, lerr := latestRelease(opts.Repo, token, assetName)
			if lerr != nil {
				logs.Warn("service update check failed: %v", lerr)
				continue
			}
			if latest.TagName == "" {
				continue
			}
			if !opts.AllowDowngrade && compareVersions(latest.TagName, mgr.version) <= 0 {
				continue
			}
			newPath, derr := ensureReleaseBinary(opts.InstallDir, latest.TagName, assetName, latestAsset.BrowserDownloadURL, token)
			if derr != nil {
				logs.Warn("service download failed: %v", derr)
				continue
			}
			logs.Info("service update: %s -> %s", mgr.version, latest.TagName)
			mgr.stopWorker(10 * time.Second)
			if lerr := switchCurrentLink(currentLink, newPath); lerr != nil {
				logs.Error("switch current link failed: %v", lerr)
				continue
			}
			mgr.version = latest.TagName
			mgr.path = newPath
			if serr := mgr.startWorker(currentLink, opts); serr != nil {
				logs.Error("restart updated worker failed: %v", serr)
			}
		}
	}
}

func ensureLocalWorkerBinary(installDir, assetName string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return "", err
	}
	dstDir := filepath.Join(installDir, "releases", sanitizeTag("local-"+BuildVersion))
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}
	dstPath := filepath.Join(dstDir, assetName)
	if _, err := os.Stat(dstPath); err == nil {
		return dstPath, nil
	}
	if err := copyFile(exe, dstPath, 0o755); err != nil {
		return "", err
	}
	return dstPath, nil
}

func installService(opts serviceOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, _ = filepath.Abs(exe)
	runArgs := serviceRunArgs(opts)

	switch runtime.GOOS {
	case "linux":
		return installSystemdUserService(exe, runArgs)
	case "darwin":
		return installLaunchdUserService(exe, runArgs)
	default:
		return fmt.Errorf("service install unsupported on %s", runtime.GOOS)
	}
}

func serviceStatus(opts serviceOptions) error {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("systemctl", "--user", "status", "--no-pager", "dialtone_repl.service")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "darwin":
		uid := os.Getuid()
		label := "dev.dialtone.dialtone_repl"
		cmd := exec.Command("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, label))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("service status unsupported on %s", runtime.GOOS)
	}
}

func serviceRunArgs(opts serviceOptions) []string {
	args := []string{
		"service", "--mode", "run",
		"--repo", opts.Repo,
		"--nats-url", opts.NATSURL,
		"--room", opts.Room,
		"--hostname", opts.HostName,
		"--check-interval", opts.CheckInterval.String(),
		"--install-dir", opts.InstallDir,
		"--token-env", opts.TokenEnv,
	}
	if opts.EmbeddedNATS {
		args = append(args, "--embedded-nats")
	} else {
		args = append(args, "--embedded-nats=false")
	}
	if opts.AllowDowngrade {
		args = append(args, "--allow-downgrade")
	}
	if opts.TSNet {
		args = append(args, "--tsnet")
	}
	if opts.TSNetNATSPort > 0 {
		args = append(args, "--tsnet-nats-port", strconv.Itoa(opts.TSNetNATSPort))
	}
	return args
}

func installSystemdUserService(exe string, runArgs []string) error {
	home := userHomeDir()
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return err
	}
	unitPath := filepath.Join(unitDir, "dialtone_repl.service")

	execStart := exe + " " + strings.Join(runArgs, " ")
	unit := strings.Join([]string{
		"[Unit]",
		"Description=Dialtone REPL Service Supervisor",
		"After=default.target network-online.target",
		"",
		"[Service]",
		"Type=simple",
		"ExecStart=" + execStart,
		"Restart=always",
		"RestartSec=2",
		"StandardOutput=journal",
		"StandardError=journal",
		"",
		"[Install]",
		"WantedBy=default.target",
		"",
	}, "\n")
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{
		{"--user", "daemon-reload"},
		{"--user", "enable", "--now", "dialtone_repl.service"},
	} {
		cmd := exec.Command("systemctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	logs.Info("Installed systemd user service: %s", unitPath)
	return nil
}

func installLaunchdUserService(exe string, runArgs []string) error {
	home := userHomeDir()
	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(plistDir, 0o755); err != nil {
		return err
	}
	label := "dev.dialtone.dialtone_repl"
	plistPath := filepath.Join(plistDir, label+".plist")

	argsXML := "<string>" + xmlEscape(exe) + "</string>\n"
	for _, a := range runArgs {
		argsXML += "\t\t<string>" + xmlEscape(a) + "</string>\n"
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + label + `</string>
	<key>ProgramArguments</key>
	<array>
		` + strings.TrimSpace(argsXML) + `
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
</dict>
</plist>
`
	if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil {
		return err
	}

	uid := os.Getuid()
	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, label)).Run()
	for _, args := range [][]string{
		{"bootstrap", fmt.Sprintf("gui/%d", uid), plistPath},
		{"enable", fmt.Sprintf("gui/%d/%s", uid, label)},
		{"kickstart", "-k", fmt.Sprintf("gui/%d/%s", uid, label)},
	} {
		cmd := exec.Command("launchctl", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	logs.Info("Installed launchd user service: %s", plistPath)
	return nil
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

func (m *serviceManager) startWorker(currentPath string, opts serviceOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.worker != nil && m.worker.Process != nil {
		return nil
	}
	workerPath := currentPath
	if runtime.GOOS == "windows" {
		if strings.HasSuffix(strings.ToLower(workerPath), ".exe") {
			// no-op
		} else if _, exErr := os.Stat(workerPath + ".exe"); exErr == nil {
			workerPath = workerPath + ".exe"
		} else if _, err := os.Stat(workerPath); err == nil {
			// On Windows, executable paths generally require a .exe suffix.
			if cpErr := copyFile(workerPath, workerPath+".exe", 0o755); cpErr == nil {
				workerPath = workerPath + ".exe"
			}
		}
	}
	args := []string{"leader", "--nats-url", opts.NATSURL, "--room", opts.Room}
	if opts.EmbeddedNATS {
		args = append(args, "--embedded-nats")
	} else {
		args = append(args, "--embedded-nats=false")
	}
	if strings.TrimSpace(opts.HostName) != "" {
		args = append(args, "--hostname", opts.HostName)
	}
	if opts.TSNet {
		args = append(args, "--tsnet")
	}
	if opts.TSNetNATSPort > 0 {
		args = append(args, "--tsnet-nats-port", strconv.Itoa(opts.TSNetNATSPort))
	}
	cmd := exec.Command(workerPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	m.worker = cmd
	logs.Info("service started repl worker pid=%d version=%s", cmd.Process.Pid, m.version)
	go func(local *exec.Cmd, version string) {
		err := local.Wait()
		if err != nil {
			logs.Warn("worker exited version=%s: %v", version, err)
		} else {
			logs.Warn("worker exited version=%s", version)
		}
		m.mu.Lock()
		if m.worker == local {
			m.worker = nil
		}
		m.mu.Unlock()
	}(cmd, m.version)
	return nil
}

func (m *serviceManager) stopWorker(timeout time.Duration) {
	m.mu.Lock()
	cmd := m.worker
	m.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
	}
	m.mu.Lock()
	if m.worker == cmd {
		m.worker = nil
	}
	m.mu.Unlock()
}

func (m *serviceManager) workerVersion() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strings.TrimSpace(m.version)
}

func startDaemonPresenceLoop(ctx context.Context, opts serviceOptions, mgr *serviceManager) func() {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		dialURL := serviceDialNATSURL(opts.NATSURL)
		publish := func() {
			nc, err := nats.Connect(dialURL, nats.Timeout(1200*time.Millisecond))
			if err != nil {
				return
			}
			defer nc.Close()
			_ = publishFrame(nc, replRoomSubject(opts.Room), BusFrame{
				Type:      frameTypeDaemon,
				From:      opts.HostName,
				Room:      sanitizeRoom(opts.Room),
				DaemonVer: BuildVersion,
				ReplVer:   mgr.workerVersion(),
				OS:        runtime.GOOS,
				Arch:      runtime.GOARCH,
				Message:   "alive",
			})
			_ = nc.FlushTimeout(700 * time.Millisecond)
		}
		publish()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-t.C:
				publish()
			}
		}
	}()
	return func() {
		close(stop)
	}
}

func serviceDialNATSURL(raw string) string {
	targetAddr, port, err := natsProxyTarget(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	host, _, splitErr := net.SplitHostPort(targetAddr)
	if splitErr != nil || strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("nats://%s:%d", host, port)
}

func ensureReleaseBinary(installDir, tag, assetName, downloadURL, token string) (string, error) {
	dstDir := filepath.Join(installDir, "releases", sanitizeTag(tag))
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}
	dstPath := filepath.Join(dstDir, assetName)
	if _, err := os.Stat(dstPath); err == nil {
		return dstPath, nil
	}
	if strings.TrimSpace(downloadURL) == "" {
		return "", fmt.Errorf("release asset %s has empty download URL", assetName)
	}
	if err := downloadFile(downloadURL, token, dstPath+".tmp"); err != nil {
		return "", err
	}
	if err := os.Chmod(dstPath+".tmp", 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(dstPath+".tmp", dstPath); err != nil {
		return "", err
	}
	return dstPath, nil
}

func downloadFile(url, token, outPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("download failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func latestRelease(repo, token, assetName string) (releaseInfo, releaseAsset, error) {
	url := "https://api.github.com/repos/" + strings.TrimSpace(repo) + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return releaseInfo{}, releaseAsset{}, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return releaseInfo{}, releaseAsset{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return releaseInfo{}, releaseAsset{}, fmt.Errorf("github latest release failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return releaseInfo{}, releaseAsset{}, err
	}
	for _, a := range rel.Assets {
		if a.Name == assetName {
			return rel, a, nil
		}
	}
	assetNames := make([]string, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		assetNames = append(assetNames, a.Name)
	}
	sort.Strings(assetNames)
	return rel, releaseAsset{}, fmt.Errorf("asset %s not found in release %s (assets=%v)", assetName, rel.TagName, assetNames)
}

func localAssetName() string {
	name := fmt.Sprintf("dialtone_repl-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func compareVersions(a, b string) int {
	aa := parseVersionParts(a)
	bb := parseVersionParts(b)
	for i := 0; i < len(aa) || i < len(bb); i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func parseVersionParts(v string) []int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	segments := strings.Split(v, ".")
	parts := make([]int, 0, len(segments))
	for _, s := range segments {
		n := strings.Builder{}
		for _, r := range s {
			if r >= '0' && r <= '9' {
				n.WriteRune(r)
			} else {
				break
			}
		}
		if n.Len() == 0 {
			parts = append(parts, 0)
			continue
		}
		iv, err := strconv.Atoi(n.String())
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, iv)
	}
	return parts
}

func sanitizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.ReplaceAll(tag, "/", "-")
	tag = strings.ReplaceAll(tag, "\\", "-")
	if tag == "" {
		return "unknown"
	}
	return tag
}

func userHomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

func switchCurrentLink(currentLink, target string) error {
	_ = os.Remove(currentLink)
	if err := os.Symlink(target, currentLink); err == nil {
		return nil
	}
	// Symlink fallback for platforms/filesystems without symlink support.
	in, err := os.Open(target)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(currentLink)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(currentLink, 0o755)
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}
