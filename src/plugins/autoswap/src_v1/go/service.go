package autoswap

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

var BuildVersion = "dev"

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type serviceManager struct {
	mu      sync.Mutex
	worker  *exec.Cmd
	version string
}

type serviceOptions struct {
	Mode           string
	Repo           string
	CheckInterval  time.Duration
	InstallDir     string
	TokenEnv       string
	AllowDowngrade bool

	ManifestPath  string
	ManifestURL   string
	RepoRoot      string
	Listen        string
	NATSPort      int
	NATSWSPort    int
	Timeout       time.Duration
	RequireStream bool
	ReleaseTag    string
}

type supervisorState struct {
	UpdatedAt      string `json:"updated_at"`
	Status         string `json:"status"`
	Repo           string `json:"repo"`
	ManifestPath   string `json:"manifest_path"`
	ManifestURL    string `json:"manifest_url,omitempty"`
	RepoRoot       string `json:"repo_root"`
	WorkerVersion  string `json:"worker_version"`
	WorkerPID      int    `json:"worker_pid,omitempty"`
	LastCheckAt    string `json:"last_check_at,omitempty"`
	LastError      string `json:"last_error,omitempty"`
	LastReleaseTag string `json:"last_release_tag,omitempty"`
}

func RunService(args []string) error {
	defaultManifest := "composition.manifest.json"
	defaultRepoRoot := ""
	if rt, err := configv1.ResolveRuntime(""); err == nil {
		defaultManifest = filepath.Join(rt.RepoRoot, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")
		defaultRepoRoot = rt.RepoRoot
	}

	fs := flag.NewFlagSet("autoswap-service", flag.ContinueOnError)
	mode := fs.String("mode", "install", "Service mode: install|run|status")
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name")
	checkInterval := fs.Duration("check-interval", 5*time.Minute, "Update check interval")
	installDir := fs.String("install-dir", filepath.Join(userHomeDir(), ".dialtone", "autoswap"), "Service install directory")
	tokenEnv := fs.String("token-env", "GITHUB_TOKEN", "Token env var used for GitHub API")
	allowDowngrade := fs.Bool("allow-downgrade", false, "Allow replacing worker with older version")

	manifest := fs.String("manifest", defaultManifest, "Path to composition manifest")
	manifestURL := fs.String("manifest-url", "", "Manifest URL (if set, overrides --manifest)")
	repoRoot := fs.String("repo-root", defaultRepoRoot, "Optional repo root for <repo_root> substitutions")
	listen := fs.String("listen", ":18086", "Runtime listen address for compose run")
	natsPort := fs.Int("nats-port", 18236, "Embedded NATS port for compose run")
	natsWSPort := fs.Int("nats-ws-port", 18237, "Embedded NATS websocket port for compose run")
	timeout := fs.Duration("timeout", 168*time.Hour, "Compose timeout when worker runs in service mode")
	requireStream := fs.Bool("require-stream", true, "Require /stream endpoint to return HTTP 200")

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := serviceOptions{
		Mode:           strings.TrimSpace(*mode),
		Repo:           strings.TrimSpace(*repo),
		CheckInterval:  *checkInterval,
		InstallDir:     strings.TrimSpace(*installDir),
		TokenEnv:       strings.TrimSpace(*tokenEnv),
		AllowDowngrade: *allowDowngrade,
		ManifestPath:   strings.TrimSpace(*manifest),
		ManifestURL:    strings.TrimSpace(*manifestURL),
		RepoRoot:       strings.TrimSpace(*repoRoot),
		Listen:         strings.TrimSpace(*listen),
		NATSPort:       *natsPort,
		NATSWSPort:     *natsWSPort,
		Timeout:        *timeout,
		RequireStream:  *requireStream,
	}
	if opts.Mode == "" {
		opts.Mode = "install"
	}
	switch opts.Mode {
	case "install":
		replIndexInfof("autoswap service: installing launcher")
	case "run":
		replIndexInfof("autoswap service: starting supervisor loop")
	case "start":
		replIndexInfof("autoswap service: starting launcher")
	case "stop":
		replIndexInfof("autoswap service: stopping launcher")
	case "restart":
		replIndexInfof("autoswap service: restarting launcher")
	case "is-active":
		replIndexInfof("autoswap service: checking active state")
	case "list":
		replIndexInfof("autoswap service: listing state files")
	case "status":
		replIndexInfof("autoswap service: reading launcher status")
	}

	switch opts.Mode {
	case "run":
		return runServiceSupervisor(opts)
	case "install":
		return installService(opts)
	case "start":
		return serviceStart()
	case "stop":
		return serviceStop()
	case "restart":
		return serviceRestart()
	case "is-active":
		return serviceIsActive()
	case "list":
		return serviceList(opts)
	case "status":
		return serviceStatus(opts)
	default:
		return fmt.Errorf("unsupported service mode %q (expected install|run|start|stop|restart|status|is-active|list)", opts.Mode)
	}
}

func runServiceSupervisor(opts serviceOptions) error {
	if err := os.MkdirAll(opts.InstallDir, 0o755); err != nil {
		return err
	}
	stateDir := filepath.Join(opts.InstallDir, "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	supervisorPath := filepath.Join(stateDir, "supervisor.json")
	releasesDir := filepath.Join(opts.InstallDir, "releases")
	if err := os.MkdirAll(releasesDir, 0o755); err != nil {
		return err
	}
	currentLink := filepath.Join(opts.InstallDir, "current")
	assetName := localAssetName()
	token := strings.TrimSpace(os.Getenv(opts.TokenEnv))
	resolvedManifestPath, err := resolveManifestPath(
		opts.ManifestPath,
		opts.ManifestURL,
		filepath.Join(opts.InstallDir, "manifests"),
		token,
	)
	if err != nil {
		return err
	}
	runOpts := opts
	runOpts.ManifestPath = resolvedManifestPath
	lastManifestFingerprint, _ := sha256File(runOpts.ManifestPath)
	if err := syncRepoRoot(runOpts.RepoRoot); err != nil {
		logs.Warn("initial repo sync failed: %v", err)
	}

	rel, asset, err := latestRelease(opts.Repo, token, assetName)
	workerVersion := ""
	workerPath := ""
	if err != nil || strings.TrimSpace(rel.TagName) == "" {
		if err != nil {
			logs.Warn("initial release lookup failed, using local seed worker: %v", err)
		}
		localPath, localErr := ensureLocalWorkerBinary(opts.InstallDir, assetName)
		if localErr != nil {
			return fmt.Errorf("release lookup failed (%v) and local seed failed: %w", err, localErr)
		}
		workerVersion = BuildVersion
		workerPath = localPath
	} else {
		workerPath, err = ensureReleaseBinary(opts.InstallDir, rel.TagName, asset, rel.Assets, token)
		if err != nil {
			return err
		}
		workerVersion = rel.TagName
	}
	if err := switchCurrentLink(currentLink, workerPath); err != nil {
		return err
	}
	lastManifestAssetsFingerprint := ""
	if strings.TrimSpace(rel.TagName) != "" {
		if err := syncManifestReleaseArtifacts(runOpts, rel, token); err != nil {
			logs.Warn("initial manifest artifact sync failed: %v", err)
		} else {
			lastManifestAssetsFingerprint = releaseAssetsFingerprint(rel.Assets)
		}
	}

	mgr := &serviceManager{version: workerVersion}
	runtimeStatePath := filepath.Join(stateDir, "runtime.json")
	if err := mgr.startWorker(currentLink, runOpts, runtimeStatePath); err != nil {
		return err
	}
	_ = writeSupervisorState(supervisorPath, supervisorState{
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
		Status:         "active",
		Repo:           opts.Repo,
		ManifestPath:   runOpts.ManifestPath,
		ManifestURL:    opts.ManifestURL,
		RepoRoot:       opts.RepoRoot,
		WorkerVersion:  mgr.version,
		WorkerPID:      mgr.workerPID(),
		LastReleaseTag: strings.TrimSpace(rel.TagName),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(opts.CheckInterval)
	defer ticker.Stop()

	logs.Info("autoswap service active: repo=%s version=%s asset=%s", opts.Repo, mgr.version, assetName)
	for {
		select {
		case <-ctx.Done():
			mgr.stopWorker(10 * time.Second)
			_ = writeSupervisorState(supervisorPath, supervisorState{
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
				Status:        "stopped",
				Repo:          opts.Repo,
				ManifestPath:  runOpts.ManifestPath,
				ManifestURL:   opts.ManifestURL,
				RepoRoot:      opts.RepoRoot,
				WorkerVersion: mgr.version,
			})
			return nil
		case <-ticker.C:
			checkAt := time.Now().UTC().Format(time.RFC3339)
			latest, latestAsset, lerr := latestRelease(opts.Repo, token, assetName)
			if lerr != nil {
				logs.Warn("service update check failed: %v", lerr)
				_ = writeSupervisorState(supervisorPath, supervisorState{
					UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
					Status:        "active",
					Repo:          opts.Repo,
					ManifestPath:  runOpts.ManifestPath,
					ManifestURL:   opts.ManifestURL,
					RepoRoot:      opts.RepoRoot,
					WorkerVersion: mgr.version,
					WorkerPID:     mgr.workerPID(),
					LastCheckAt:   checkAt,
					LastError:     lerr.Error(),
				})
				continue
			}
			if latest.TagName == "" {
				_ = writeSupervisorState(supervisorPath, supervisorState{
					UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
					Status:         "active",
					Repo:           opts.Repo,
					ManifestPath:   runOpts.ManifestPath,
					ManifestURL:    opts.ManifestURL,
					RepoRoot:       opts.RepoRoot,
					WorkerVersion:  mgr.version,
					WorkerPID:      mgr.workerPID(),
					LastCheckAt:    checkAt,
					LastReleaseTag: "",
				})
				continue
			}
			manifestChanged := false
			if strings.TrimSpace(opts.ManifestURL) != "" {
				updatedManifestPath, changed, merr := refreshManifestFromURL(runOpts.ManifestPath, opts.ManifestURL, token)
				if merr != nil {
					logs.Warn("manifest refresh failed: %v", merr)
					_ = writeSupervisorState(supervisorPath, supervisorState{
						UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
						Status:         "active",
						Repo:           opts.Repo,
						ManifestPath:   runOpts.ManifestPath,
						ManifestURL:    opts.ManifestURL,
						RepoRoot:       opts.RepoRoot,
						WorkerVersion:  mgr.version,
						WorkerPID:      mgr.workerPID(),
						LastCheckAt:    checkAt,
						LastError:      merr.Error(),
						LastReleaseTag: latest.TagName,
					})
					continue
				}
				runOpts.ManifestPath = updatedManifestPath
				if changed {
					manifestChanged = true
				}
			}
			if nextManifestFingerprint, ferr := sha256File(runOpts.ManifestPath); ferr == nil && strings.TrimSpace(nextManifestFingerprint) != "" {
				if strings.TrimSpace(lastManifestFingerprint) == "" {
					lastManifestFingerprint = nextManifestFingerprint
				} else if !strings.EqualFold(nextManifestFingerprint, lastManifestFingerprint) {
					manifestChanged = true
					lastManifestFingerprint = nextManifestFingerprint
				}
			}

			artifactsChanged := false
			nextManifestAssetsFingerprint := releaseAssetsFingerprint(latest.Assets)
			if manifestChanged || (nextManifestAssetsFingerprint != "" && nextManifestAssetsFingerprint != lastManifestAssetsFingerprint) {
				if err := syncRepoRoot(runOpts.RepoRoot); err != nil {
					logs.Warn("repo sync failed before manifest refresh: %v", err)
				}
				// Sync manifest artifacts when manifest content changes or release
				// assets changed, even if the autoswap worker release tag is unchanged.
				if syncErr := syncManifestReleaseArtifacts(runOpts, latest, token); syncErr != nil {
					logs.Warn("manifest artifact sync failed: %v", syncErr)
					_ = writeSupervisorState(supervisorPath, supervisorState{
						UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
						Status:         "active",
						Repo:           opts.Repo,
						ManifestPath:   runOpts.ManifestPath,
						ManifestURL:    opts.ManifestURL,
						RepoRoot:       opts.RepoRoot,
						WorkerVersion:  mgr.version,
						WorkerPID:      mgr.workerPID(),
						LastCheckAt:    checkAt,
						LastError:      syncErr.Error(),
						LastReleaseTag: latest.TagName,
					})
					continue
				}
				if nextManifestAssetsFingerprint != "" && nextManifestAssetsFingerprint != lastManifestAssetsFingerprint {
					lastManifestAssetsFingerprint = nextManifestAssetsFingerprint
					artifactsChanged = true
				}
			}
			cmp := compareVersions(latest.TagName, mgr.version)
			needsWorkerVersionUpdate := cmp > 0 || (opts.AllowDowngrade && cmp != 0)
			if !needsWorkerVersionUpdate {
				if manifestChanged || artifactsChanged {
					logs.Info("service refresh: manifest_changed=%t artifacts_changed=%t release=%s", manifestChanged, artifactsChanged, latest.TagName)
					mgr.stopWorker(10 * time.Second)
					if serr := mgr.startWorker(currentLink, runOpts, runtimeStatePath); serr != nil {
						logs.Error("restart refreshed worker failed: %v", serr)
						_ = writeSupervisorState(supervisorPath, supervisorState{
							UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
							Status:         "degraded",
							Repo:           opts.Repo,
							ManifestPath:   runOpts.ManifestPath,
							ManifestURL:    opts.ManifestURL,
							RepoRoot:       opts.RepoRoot,
							WorkerVersion:  mgr.version,
							LastCheckAt:    checkAt,
							LastError:      serr.Error(),
							LastReleaseTag: latest.TagName,
						})
						continue
					}
				}
				_ = writeSupervisorState(supervisorPath, supervisorState{
					UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
					Status:         "active",
					Repo:           opts.Repo,
					ManifestPath:   runOpts.ManifestPath,
					ManifestURL:    opts.ManifestURL,
					RepoRoot:       opts.RepoRoot,
					WorkerVersion:  mgr.version,
					WorkerPID:      mgr.workerPID(),
					LastCheckAt:    checkAt,
					LastReleaseTag: latest.TagName,
				})
				continue
			}
			newPath, derr := ensureReleaseBinary(opts.InstallDir, latest.TagName, latestAsset, latest.Assets, token)
			if derr != nil {
				logs.Warn("service download failed: %v", derr)
				_ = writeSupervisorState(supervisorPath, supervisorState{
					UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
					Status:         "active",
					Repo:           opts.Repo,
					ManifestPath:   opts.ManifestPath,
					RepoRoot:       opts.RepoRoot,
					WorkerVersion:  mgr.version,
					WorkerPID:      mgr.workerPID(),
					LastCheckAt:    checkAt,
					LastError:      derr.Error(),
					LastReleaseTag: latest.TagName,
				})
				continue
			}
			logs.Info("service update: %s -> %s", mgr.version, latest.TagName)
			mgr.stopWorker(10 * time.Second)
			if lerr := switchCurrentLink(currentLink, newPath); lerr != nil {
				logs.Error("switch current link failed: %v", lerr)
				_ = writeSupervisorState(supervisorPath, supervisorState{
					UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
					Status:         "degraded",
					Repo:           opts.Repo,
					ManifestPath:   runOpts.ManifestPath,
					ManifestURL:    opts.ManifestURL,
					RepoRoot:       opts.RepoRoot,
					WorkerVersion:  mgr.version,
					LastCheckAt:    checkAt,
					LastError:      lerr.Error(),
					LastReleaseTag: latest.TagName,
				})
				continue
			}
			mgr.version = latest.TagName
			if err := syncRepoRoot(runOpts.RepoRoot); err != nil {
				logs.Warn("repo sync failed before worker restart: %v", err)
			}
			if serr := mgr.startWorker(currentLink, runOpts, runtimeStatePath); serr != nil {
				logs.Error("restart updated worker failed: %v", serr)
				_ = writeSupervisorState(supervisorPath, supervisorState{
					UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
					Status:         "degraded",
					Repo:           opts.Repo,
					ManifestPath:   runOpts.ManifestPath,
					ManifestURL:    opts.ManifestURL,
					RepoRoot:       opts.RepoRoot,
					WorkerVersion:  mgr.version,
					LastCheckAt:    checkAt,
					LastError:      serr.Error(),
					LastReleaseTag: latest.TagName,
				})
				continue
			}
			_ = writeSupervisorState(supervisorPath, supervisorState{
				UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
				Status:         "active",
				Repo:           opts.Repo,
				ManifestPath:   runOpts.ManifestPath,
				ManifestURL:    opts.ManifestURL,
				RepoRoot:       opts.RepoRoot,
				WorkerVersion:  mgr.version,
				WorkerPID:      mgr.workerPID(),
				LastCheckAt:    checkAt,
				LastReleaseTag: latest.TagName,
			})
		}
	}
}

func syncRepoRoot(repoRoot string) error {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return nil
	}
	script := fmt.Sprintf(`set -e
cd %s
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  exit 0
fi
git fetch --all --tags --prune
branch="$(git symbolic-ref --quiet --short HEAD 2>/dev/null || true)"
if [ -n "$branch" ] && git rev-parse --abbrev-ref --symbolic-full-name "@{u}" >/dev/null 2>&1; then
  git pull --ff-only --tags
fi
rm -rf bin/releases
find . -path '*/ui/dist' -type d -prune -exec rm -rf {} + 2>/dev/null || true`, shellQuoteArg(repoRoot))
	cmd := exec.Command("bash", "-lc", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func releaseAssetsFingerprint(assets []releaseAsset) string {
	if len(assets) == 0 {
		return ""
	}
	rows := make([]string, 0, len(assets))
	for _, a := range assets {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			continue
		}
		digest := strings.TrimSpace(strings.ToLower(a.Digest))
		rows = append(rows, name+"|"+digest)
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])
}

func refreshManifestFromURL(currentPath, manifestURL, token string) (string, bool, error) {
	manifestURL = strings.TrimSpace(manifestURL)
	if manifestURL == "" {
		return strings.TrimSpace(currentPath), false, nil
	}
	prevPath := strings.TrimSpace(currentPath)
	prevSum := ""
	if prevPath != "" {
		if sum, err := sha256File(prevPath); err == nil {
			prevSum = sum
		}
	}
	manifestDir := filepath.Dir(prevPath)
	if strings.TrimSpace(manifestDir) == "" || manifestDir == "." {
		manifestDir = filepath.Join(userHomeDir(), ".dialtone", "autoswap", "manifests")
	}
	nextPath, err := resolveManifestPath(prevPath, manifestURL, manifestDir, token)
	if err != nil {
		return "", false, err
	}
	nextSum, err := sha256File(nextPath)
	if err != nil {
		return "", false, err
	}
	changed := strings.TrimSpace(prevSum) == "" || !strings.EqualFold(prevSum, nextSum)
	return nextPath, changed, nil
}

func syncManifestReleaseArtifacts(opts serviceOptions, rel releaseInfo, token string) error {
	if strings.TrimSpace(opts.ManifestPath) == "" || strings.TrimSpace(opts.RepoRoot) == "" {
		// Repo root is optional; allow manifests that do not use <repo_root>.
	}
	art, err := loadManifestResolved(opts.ManifestPath, opts.RepoRoot, false)
	if err != nil {
		return err
	}

	if len(art.Manifest.Artifacts.Release) == 0 {
		targets := map[string]string{
			"autoswap": art.AutoswapBin,
			"robot":    art.RobotBin,
			"repl":     art.ReplBin,
			"camera":   art.CameraBin,
			"mavlink":  art.MavlinkBin,
		}
		for key, target := range targets {
			if strings.TrimSpace(target) == "" {
				continue
			}
			asset, ok := pickManifestReleaseAsset(rel.Assets, key, target)
			if !ok {
				return fmt.Errorf("release %s missing asset for manifest key=%s target=%s", strings.TrimSpace(rel.TagName), key, target)
			}
			if err := placeReleaseFile(asset, rel.Assets, target, token, ""); err != nil {
				return err
			}
			logs.Info("synced manifest artifact %s <- %s", key, asset.Name)
		}
		return nil
	}

	for key, binding := range art.Manifest.Artifacts.Release {
		target := strings.TrimSpace(art.Sync[key])
		if target == "" {
			return fmt.Errorf("manifest release key %s missing artifacts.sync target", key)
		}
		assetName := renderReleaseAssetName(strings.TrimSpace(binding.Asset))
		if assetName == "" {
			return fmt.Errorf("manifest release key %s has empty asset name", key)
		}
		asset, ok := findReleaseAssetByName(rel.Assets, assetName)
		if !ok {
			return fmt.Errorf("release %s missing asset %s for key=%s", strings.TrimSpace(rel.TagName), assetName, key)
		}
		if err := placeReleaseArtifact(asset, rel.Assets, target, binding, token); err != nil {
			return err
		}
		logs.Info("synced manifest artifact %s <- %s", key, asset.Name)
	}
	return nil
}

func pickManifestReleaseAsset(assets []releaseAsset, key, targetPath string) (releaseAsset, bool) {
	candidates := manifestAssetCandidates(key, targetPath)
	for _, name := range candidates {
		for _, a := range assets {
			if strings.EqualFold(strings.TrimSpace(a.Name), name) {
				return a, true
			}
		}
	}
	return releaseAsset{}, false
}

func manifestAssetCandidates(key, targetPath string) []string {
	base := strings.TrimSpace(filepath.Base(targetPath))
	baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
	suffix := runtime.GOOS + "-" + runtime.GOARCH
	out := make([]string, 0, 8)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		for _, e := range out {
			if strings.EqualFold(e, v) {
				return
			}
		}
		out = append(out, v)
	}

	add(base + "-" + suffix)
	add(baseNoExt + "-" + suffix)
	add(base + "_" + suffix)
	add(baseNoExt + "_" + suffix)
	if runtime.GOOS == "windows" {
		add(base + "-" + suffix + ".exe")
		add(baseNoExt + "-" + suffix + ".exe")
	}
	if key == "autoswap" {
		add(localAssetName())
	}
	add(base)
	return out
}

func renderReleaseAssetName(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.ReplaceAll(v, "${goos}", runtime.GOOS)
	v = strings.ReplaceAll(v, "${goarch}", runtime.GOARCH)
	v = strings.ReplaceAll(v, "<goos>", runtime.GOOS)
	v = strings.ReplaceAll(v, "<goarch>", runtime.GOARCH)
	return v
}

func findReleaseAssetByName(assets []releaseAsset, name string) (releaseAsset, bool) {
	name = strings.TrimSpace(name)
	for _, a := range assets {
		if strings.EqualFold(strings.TrimSpace(a.Name), name) {
			return a, true
		}
	}
	return releaseAsset{}, false
}

func placeReleaseFile(asset releaseAsset, allAssets []releaseAsset, target, token, expectedDigest string) error {
	expectedDigest = normalizeSHA256(expectedDigest)
	if expectedDigest == "" {
		expectedDigest = releaseAssetDigest(asset)
	}
	if expectedDigest != "" {
		if sum, err := sha256File(target); err == nil && strings.EqualFold(strings.TrimSpace(sum), expectedDigest) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmpPath := target + ".tmp"
	if err := downloadFile(asset.BrowserDownloadURL, token, tmpPath); err != nil {
		return fmt.Errorf("download %s failed: %w", asset.Name, err)
	}
	if err := verifyReleaseAssetChecksum(asset, allAssets, tmpPath, token); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}

func placeReleaseArtifact(asset releaseAsset, allAssets []releaseAsset, target string, binding releaseBinding, token string) error {
	t := strings.ToLower(strings.TrimSpace(binding.Type))
	if t == "" {
		t = "file"
	}
	expectedDigest := bindingExpectedDigest(binding)
	switch t {
	case "file", "bin", "binary":
		return placeReleaseFile(asset, allAssets, target, token, expectedDigest)
	case "dir", "directory":
		return placeReleaseDir(asset, allAssets, target, binding, token, expectedDigest)
	default:
		return fmt.Errorf("unsupported release artifact type %q for asset %s", binding.Type, asset.Name)
	}
}

func placeReleaseDir(asset releaseAsset, allAssets []releaseAsset, target string, binding releaseBinding, token, expectedDigest string) error {
	expectedDigest = normalizeSHA256(expectedDigest)
	if expectedDigest == "" {
		expectedDigest = releaseAssetDigest(asset)
	}
	if expectedDigest != "" {
		stampPath := target + ".asset.sha256"
		if raw, err := os.ReadFile(stampPath); err == nil {
			if strings.EqualFold(strings.TrimSpace(string(raw)), expectedDigest) {
				if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
					return nil
				}
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".artifact.tmp"
	if err := downloadFile(asset.BrowserDownloadURL, token, tmp); err != nil {
		return fmt.Errorf("download %s failed: %w", asset.Name, err)
	}
	if err := verifyReleaseAssetChecksum(asset, allAssets, tmp, token); err != nil {
		return err
	}
	defer os.Remove(tmp)

	_ = os.RemoveAll(target + ".tmpdir")
	if err := os.MkdirAll(target+".tmpdir", 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(target + ".tmpdir")

	format := strings.ToLower(strings.TrimSpace(binding.Extract))
	if format == "" {
		l := strings.ToLower(asset.Name)
		switch {
		case strings.HasSuffix(l, ".tar.gz"), strings.HasSuffix(l, ".tgz"):
			format = "tar.gz"
		case strings.HasSuffix(l, ".zip"):
			format = "zip"
		}
	}

	switch format {
	case "tar.gz", "tgz":
		if err := extractTarGz(tmp, target+".tmpdir"); err != nil {
			return err
		}
	case "zip":
		if err := extractZip(tmp, target+".tmpdir"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("directory artifact %s requires extract format tar.gz|zip", asset.Name)
	}

	_ = os.RemoveAll(target)
	if err := os.Rename(target+".tmpdir", target); err != nil {
		return err
	}
	if expectedDigest != "" {
		_ = os.WriteFile(target+".asset.sha256", []byte(expectedDigest+"\n"), 0o644)
	}
	return nil
}

func releaseAssetDigest(asset releaseAsset) string {
	return normalizeSHA256(asset.Digest)
}

func normalizeSHA256(v string) string {
	digest := strings.TrimSpace(v)
	if digest == "" {
		return ""
	}
	digest = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(digest), "sha256:"))
	if !isHexDigest(digest) {
		return ""
	}
	return digest
}

func normalizeManifestURLForAutoUpdate(rawURL, repo string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, false
	}
	if !strings.EqualFold(strings.TrimSpace(u.Hostname()), "github.com") {
		return rawURL, false
	}
	parts := strings.Split(strings.Trim(strings.TrimSpace(u.Path), "/"), "/")
	if len(parts) < 6 {
		return rawURL, false
	}
	owner := strings.TrimSpace(parts[0])
	repoName := strings.TrimSpace(parts[1])
	if owner == "" || repoName == "" {
		return rawURL, false
	}
	pathRepo := owner + "/" + repoName
	if strings.TrimSpace(repo) != "" && !strings.EqualFold(pathRepo, strings.TrimSpace(repo)) {
		return rawURL, false
	}
	if parts[2] != "releases" || parts[3] != "download" {
		return rawURL, false
	}
	assetName := strings.TrimSpace(parts[len(parts)-1])
	switch assetName {
	case "robot_src_v2_channel.json", "robot_src_v2_composition_manifest.json":
	case "":
		return rawURL, false
	default:
		if !strings.HasPrefix(assetName, "robot_src_v2_composition_manifest-") {
			return rawURL, false
		}
	}
	if parts[4] == "latest" && assetName == "robot_src_v2_channel.json" {
		return rawURL, false
	}
	u.Path = "/" + owner + "/" + repoName + "/releases/latest/download/robot_src_v2_channel.json"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), true
}

func bindingExpectedDigest(binding releaseBinding) string {
	key := runtime.GOOS + "-" + runtime.GOARCH
	if v, ok := binding.SHA256ByTarget[key]; ok {
		if d := normalizeSHA256(v); d != "" {
			return d
		}
	}
	return normalizeSHA256(binding.SHA256)
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(h.Name)
		out := filepath.Join(dest, name)
		if !strings.HasPrefix(out, filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(out) != filepath.Clean(dest) {
			return fmt.Errorf("invalid tar path: %s", h.Name)
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			w, err := os.Create(out)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				_ = w.Close()
				return err
			}
			if err := w.Close(); err != nil {
				return err
			}
		}
	}
}

func extractZip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		name := filepath.Clean(f.Name)
		out := filepath.Join(dest, name)
		if !strings.HasPrefix(out, filepath.Clean(dest)+string(os.PathSeparator)) && filepath.Clean(out) != filepath.Clean(dest) {
			return fmt.Errorf("invalid zip path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(out)
		if err != nil {
			_ = r.Close()
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			_ = r.Close()
			_ = w.Close()
			return err
		}
		_ = r.Close()
		if err := w.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (m *serviceManager) startWorker(workerPath string, opts serviceOptions, runtimeStatePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.worker != nil && m.worker.Process != nil {
		return nil
	}
	args := []string{
		"run",
		"--listen", opts.Listen,
		"--nats-port", strconv.Itoa(opts.NATSPort),
		"--nats-ws-port", strconv.Itoa(opts.NATSWSPort),
		"--timeout", opts.Timeout.String(),
		"--stay-running=true",
	}
	if strings.TrimSpace(opts.ManifestPath) != "" {
		args = append(args, "--manifest", opts.ManifestPath)
	}
	if strings.TrimSpace(opts.RepoRoot) != "" {
		args = append(args, "--repo-root", opts.RepoRoot)
	}
	if opts.RequireStream {
		args = append(args, "--require-stream=true")
	} else {
		args = append(args, "--require-stream=false")
	}
	if strings.TrimSpace(runtimeStatePath) != "" {
		_ = os.Remove(strings.TrimSpace(runtimeStatePath))
	}

	cmd := exec.Command(workerPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.Env = append(
		os.Environ(),
		"AUTOSWAP_RUNTIME_STATE="+strings.TrimSpace(runtimeStatePath),
		"AUTOSWAP_RUNTIME_MANIFEST="+strings.TrimSpace(opts.ManifestPath),
		"AUTOSWAP_RUNTIME_REPO_ROOT="+strings.TrimSpace(opts.RepoRoot),
		"AUTOSWAP_RUNTIME_LISTEN="+strings.TrimSpace(opts.Listen),
		"AUTOSWAP_RUNTIME_NATS_PORT="+strconv.Itoa(opts.NATSPort),
		"AUTOSWAP_RUNTIME_NATS_WS_PORT="+strconv.Itoa(opts.NATSWSPort),
	)
	if err := cmd.Start(); err != nil {
		return err
	}
	m.worker = cmd
	logs.Info("service started autoswap worker pid=%d version=%s", cmd.Process.Pid, m.version)
	go func(local *exec.Cmd, version string) {
		err := local.Wait()
		if err != nil {
			logs.Warn("autoswap worker exited version=%s: %v", version, err)
		} else {
			logs.Warn("autoswap worker exited version=%s", version)
		}
		m.mu.Lock()
		if m.worker == local {
			m.worker = nil
		}
		m.mu.Unlock()
	}(cmd, m.version)
	return nil
}

func (m *serviceManager) workerPID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.worker == nil || m.worker.Process == nil {
		return 0
	}
	return m.worker.Process.Pid
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

func installService(opts serviceOptions) error {
	if normalized, changed := normalizeManifestURLForAutoUpdate(opts.ManifestURL, opts.Repo); changed {
		logs.Info("service install: normalized manifest-url to auto-update latest: %s -> %s", opts.ManifestURL, normalized)
		opts.ManifestURL = normalized
	}
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
	if err := serviceLauncherStatus(); err != nil {
		return err
	}
	return serviceList(opts)
}

func serviceStart() error {
	switch runtime.GOOS {
	case "linux":
		return runServiceCtl("systemctl", "--user", "start", "dialtone_autoswap.service")
	case "darwin":
		uid := os.Getuid()
		label := "dev.dialtone.dialtone_autoswap"
		plistPath := filepath.Join(userHomeDir(), "Library", "LaunchAgents", label+".plist")
		_ = exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), plistPath).Run()
		return runServiceCtl("launchctl", "kickstart", "-k", fmt.Sprintf("gui/%d/%s", uid, label))
	default:
		return fmt.Errorf("service start unsupported on %s", runtime.GOOS)
	}
}

func serviceStop() error {
	switch runtime.GOOS {
	case "linux":
		return runServiceCtl("systemctl", "--user", "stop", "dialtone_autoswap.service")
	case "darwin":
		uid := os.Getuid()
		label := "dev.dialtone.dialtone_autoswap"
		return runServiceCtl("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", uid, label))
	default:
		return fmt.Errorf("service stop unsupported on %s", runtime.GOOS)
	}
}

func serviceRestart() error {
	switch runtime.GOOS {
	case "linux":
		return runServiceCtl("systemctl", "--user", "restart", "dialtone_autoswap.service")
	case "darwin":
		if err := serviceStop(); err != nil {
			logs.Warn("launchctl bootout returned: %v", err)
		}
		return serviceStart()
	default:
		return fmt.Errorf("service restart unsupported on %s", runtime.GOOS)
	}
}

func serviceIsActive() error {
	switch runtime.GOOS {
	case "linux":
		return runServiceCtl("systemctl", "--user", "is-active", "dialtone_autoswap.service")
	case "darwin":
		uid := os.Getuid()
		label := "dev.dialtone.dialtone_autoswap"
		return runServiceCtl("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, label))
	default:
		return fmt.Errorf("service is-active unsupported on %s", runtime.GOOS)
	}
}

func serviceLauncherStatus() error {
	switch runtime.GOOS {
	case "linux":
		return runServiceCtl("systemctl", "--user", "status", "--no-pager", "dialtone_autoswap.service")
	case "darwin":
		uid := os.Getuid()
		label := "dev.dialtone.dialtone_autoswap"
		return runServiceCtl("launchctl", "print", fmt.Sprintf("gui/%d/%s", uid, label))
	default:
		return fmt.Errorf("service status unsupported on %s", runtime.GOOS)
	}
}

func serviceList(opts serviceOptions) error {
	stateDir := filepath.Join(strings.TrimSpace(opts.InstallDir), "state")
	supervisorPath := filepath.Join(stateDir, "supervisor.json")
	runtimePath := filepath.Join(stateDir, "runtime.json")
	logs.Raw("autoswap state files:")
	logs.Raw("  supervisor: %s", supervisorPath)
	logs.Raw("  runtime:    %s", runtimePath)
	for _, p := range []string{supervisorPath, runtimePath} {
		raw, err := os.ReadFile(p)
		if err != nil {
			logs.Raw("  - %s (missing)", p)
			continue
		}
		logs.Raw("---- %s ----", filepath.Base(p))
		logs.Raw("%s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func runServiceCtl(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func serviceRunArgs(opts serviceOptions) []string {
	args := []string{
		"service", "--mode", "run",
		"--repo", opts.Repo,
		"--check-interval", opts.CheckInterval.String(),
		"--install-dir", opts.InstallDir,
		"--token-env", opts.TokenEnv,
		"--listen", opts.Listen,
		"--nats-port", strconv.Itoa(opts.NATSPort),
		"--nats-ws-port", strconv.Itoa(opts.NATSWSPort),
		"--timeout", opts.Timeout.String(),
	}
	if opts.AllowDowngrade {
		args = append(args, "--allow-downgrade")
	}
	if strings.TrimSpace(opts.ManifestPath) != "" {
		args = append(args, "--manifest", opts.ManifestPath)
	}
	if strings.TrimSpace(opts.ManifestURL) != "" {
		args = append(args, "--manifest-url", opts.ManifestURL)
	}
	if strings.TrimSpace(opts.RepoRoot) != "" {
		args = append(args, "--repo-root", opts.RepoRoot)
	}
	if opts.RequireStream {
		args = append(args, "--require-stream")
	} else {
		args = append(args, "--require-stream=false")
	}
	return args
}

func installSystemdUserService(exe string, runArgs []string) error {
	home := userHomeDir()
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return err
	}
	unitPath := filepath.Join(unitDir, "dialtone_autoswap.service")
	execStart := exe + " " + strings.Join(runArgs, " ")
	unit := strings.Join([]string{
		"[Unit]",
		"Description=Dialtone Autoswap Service Supervisor",
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
		{"--user", "enable", "--now", "dialtone_autoswap.service"},
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
	label := "dev.dialtone.dialtone_autoswap"
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

func ensureReleaseBinary(installDir, tag string, asset releaseAsset, allAssets []releaseAsset, token string) (string, error) {
	assetName := strings.TrimSpace(asset.Name)
	dstDir := filepath.Join(installDir, "releases", sanitizeTag(tag))
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", err
	}
	dstPath := filepath.Join(dstDir, assetName)
	if _, err := os.Stat(dstPath); err == nil {
		if verr := verifyReleaseAssetChecksum(asset, allAssets, dstPath, token); verr == nil {
			return dstPath, nil
		}
		logs.Warn("release worker asset changed in-place for tag=%s asset=%s; refreshing local copy", sanitizeTag(tag), assetName)
		if rmErr := os.Remove(dstPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return "", rmErr
		}
	}
	if strings.TrimSpace(asset.BrowserDownloadURL) == "" {
		return "", fmt.Errorf("release asset %s has empty download URL", assetName)
	}
	tmpPath := dstPath + ".tmp"
	if err := downloadFile(asset.BrowserDownloadURL, token, tmpPath); err != nil {
		return "", err
	}
	if err := verifyReleaseAssetChecksum(asset, allAssets, tmpPath, token); err != nil {
		return "", err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		return "", err
	}
	return dstPath, nil
}

func verifyReleaseAssetChecksum(asset releaseAsset, allAssets []releaseAsset, localPath, token string) error {
	sum, err := sha256File(localPath)
	if err != nil {
		return err
	}
	digest := strings.TrimSpace(asset.Digest)
	if strings.HasPrefix(strings.ToLower(digest), "sha256:") {
		expected := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(digest), "sha256:"))
		if expected == "" {
			return fmt.Errorf("asset %s has empty sha256 digest", asset.Name)
		}
		if !strings.EqualFold(sum, expected) {
			return fmt.Errorf("checksum mismatch for %s: got=%s expected=%s", asset.Name, sum, expected)
		}
		return nil
	}

	checksumAsset, ok := findChecksumAssetFor(allAssets, asset.Name)
	if !ok {
		logs.Warn("release checksum metadata not found for %s; skipping checksum verification", asset.Name)
		return nil
	}
	tmp := localPath + ".checksum.tmp"
	if err := downloadFile(checksumAsset.BrowserDownloadURL, token, tmp); err != nil {
		return fmt.Errorf("download checksum asset %s failed: %w", checksumAsset.Name, err)
	}
	defer os.Remove(tmp)
	raw, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	expected, err := parseChecksumFile(string(raw), asset.Name)
	if err != nil {
		return fmt.Errorf("checksum parse failed for %s using %s: %w", asset.Name, checksumAsset.Name, err)
	}
	if !strings.EqualFold(sum, expected) {
		return fmt.Errorf("checksum mismatch for %s: got=%s expected=%s", asset.Name, sum, expected)
	}
	return nil
}

func findChecksumAssetFor(assets []releaseAsset, assetName string) (releaseAsset, bool) {
	candidates := []string{
		assetName + ".sha256",
		assetName + ".sha256sum",
		assetName + ".sha256.txt",
	}
	for _, c := range candidates {
		if a, ok := findReleaseAssetByName(assets, c); ok {
			return a, true
		}
	}
	return releaseAsset{}, false
}

func parseChecksumFile(content, assetName string) (string, error) {
	lines := strings.Split(content, "\n")
	assetName = strings.TrimSpace(assetName)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 1 && isHexDigest(fields[0]) {
			return strings.ToLower(fields[0]), nil
		}
		if len(fields) >= 2 && isHexDigest(fields[0]) {
			fileField := strings.TrimLeft(strings.TrimSpace(fields[len(fields)-1]), "*")
			fileField = filepath.Base(fileField)
			if assetName == "" || strings.EqualFold(fileField, filepath.Base(assetName)) {
				return strings.ToLower(fields[0]), nil
			}
		}
	}
	return "", fmt.Errorf("no checksum entry found for %s", assetName)
}

func isHexDigest(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) != 64 {
		return false
	}
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func downloadFile(url, token, outPath string) error {
	req, err := http.NewRequest(http.MethodGet, cacheBustedReleaseURL(url), nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
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

func cacheBustedReleaseURL(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return rawURL
	}
	if !strings.EqualFold(strings.TrimSpace(u.Hostname()), "github.com") {
		return rawURL
	}
	if !strings.Contains(strings.TrimSpace(u.Path), "/releases/latest/download/") {
		return rawURL
	}
	q := u.Query()
	q.Set("dialtone_ts", strconv.FormatInt(time.Now().UnixNano(), 10))
	u.RawQuery = q.Encode()
	return u.String()
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
	name := fmt.Sprintf("dialtone_autoswap-%s-%s", runtime.GOOS, runtime.GOARCH)
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

func writeSupervisorState(path string, st supervisorState) error {
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
