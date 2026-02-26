package autoswap

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

type runtimeManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Runtime struct {
		Binary string `json:"binary"`
	} `json:"runtime"`
	Artifacts struct {
		Sync map[string]string `json:"sync"`
	} `json:"artifacts"`
}

type composeConfig struct {
	ManifestPath string
	RepoRoot     string
	Listen       string
	NATSPort     int
	NATSWSPort   int
	Timeout      time.Duration
}

type resolvedArtifacts struct {
	ManifestPath string
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
	art, err := loadAndValidateManifest(cfg.ManifestPath, cfg.RepoRoot)
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
	art, err := loadAndValidateManifest(cfg.ManifestPath, cfg.RepoRoot)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	robotCmd := exec.CommandContext(
		ctx,
		art.RobotBin,
		"--listen", cfg.Listen,
		"--nats-port", strconv.Itoa(cfg.NATSPort),
		"--nats-ws-port", strconv.Itoa(cfg.NATSWSPort),
		"--ui-dist", art.UIDist,
	)
	robotCmd.Stdout = os.Stdout
	robotCmd.Stderr = os.Stderr
	if err := robotCmd.Start(); err != nil {
		return fmt.Errorf("start robot failed: %w", err)
	}
	defer terminateProcess(robotCmd)

	baseURL := "http://127.0.0.1" + cfg.Listen
	if err := waitHTTP(ctx, baseURL+"/health", http.StatusOK); err != nil {
		return err
	}
	if err := waitHTTP(ctx, baseURL+"/", http.StatusOK); err != nil {
		return err
	}

	natsURL := fmt.Sprintf("nats://127.0.0.1:%d", cfg.NATSPort)
	cameraCmd := exec.CommandContext(ctx, art.CameraBin, "run", "--nats-url", natsURL, "--subject", "camera.heartbeat", "--interval", "250ms")
	cameraCmd.Stdout = os.Stdout
	cameraCmd.Stderr = os.Stderr
	if err := cameraCmd.Start(); err != nil {
		return fmt.Errorf("start camera sidecar failed: %w", err)
	}
	defer terminateProcess(cameraCmd)

	mavlinkCmd := exec.CommandContext(ctx, art.MavlinkBin, "run", "--nats-url", natsURL, "--subject", "mavlink.heartbeat", "--interval", "250ms")
	mavlinkCmd.Stdout = os.Stdout
	mavlinkCmd.Stderr = os.Stderr
	if err := mavlinkCmd.Start(); err != nil {
		return fmt.Errorf("start mavlink sidecar failed: %w", err)
	}
	defer terminateProcess(mavlinkCmd)

	if err := verifyReplBinary(ctx, art.ReplBin); err != nil {
		return err
	}
	if err := waitHeartbeats(ctx, natsURL); err != nil {
		return err
	}

	logs.Info("autoswap run OK: robot+ui+camera+mavlink composition healthy")
	return nil
}

func parseComposeFlags(name string, args []string) (composeConfig, error) {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return composeConfig{}, err
	}
	defaultManifest := filepath.Join(rt.RepoRoot, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	manifest := fs.String("manifest", defaultManifest, "Path to robot composition manifest")
	repoRoot := fs.String("repo-root", rt.RepoRoot, "Repo root for <repo_root> substitutions")
	listen := fs.String("listen", ":18084", "Robot listen address for compose run")
	natsPort := fs.Int("nats-port", 18226, "Robot embedded NATS port for compose run")
	natsWSPort := fs.Int("nats-ws-port", 18227, "Robot embedded NATS websocket port for compose run")
	timeout := fs.Duration("timeout", 25*time.Second, "Compose timeout")
	if err := fs.Parse(args); err != nil {
		return composeConfig{}, err
	}
	return composeConfig{
		ManifestPath: strings.TrimSpace(*manifest),
		RepoRoot:     strings.TrimSpace(*repoRoot),
		Listen:       strings.TrimSpace(*listen),
		NATSPort:     *natsPort,
		NATSWSPort:   *natsWSPort,
		Timeout:      *timeout,
	}, nil
}

func loadAndValidateManifest(manifestPath, repoRoot string) (resolvedArtifacts, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return resolvedArtifacts{}, fmt.Errorf("read manifest failed: %w", err)
	}
	var mf runtimeManifest
	if err := json.Unmarshal(raw, &mf); err != nil {
		return resolvedArtifacts{}, fmt.Errorf("manifest parse failed: %w", err)
	}

	resolve := func(p string) string {
		return strings.ReplaceAll(strings.TrimSpace(p), "<repo_root>", repoRoot)
	}
	get := func(key string) (string, error) {
		v := resolve(mf.Artifacts.Sync[key])
		if v == "" {
			return "", fmt.Errorf("manifest missing artifacts.sync.%s", key)
		}
		return v, nil
	}
	autoswapBin, err := get("autoswap")
	if err != nil {
		return resolvedArtifacts{}, err
	}
	robotBin, err := get("robot")
	if err != nil {
		return resolvedArtifacts{}, err
	}
	replBin, err := get("repl")
	if err != nil {
		return resolvedArtifacts{}, err
	}
	cameraBin, err := get("camera")
	if err != nil {
		return resolvedArtifacts{}, err
	}
	mavlinkBin, err := get("mavlink")
	if err != nil {
		return resolvedArtifacts{}, err
	}
	wlanBundle, err := get("wlan")
	if err != nil {
		return resolvedArtifacts{}, err
	}
	uiDist, err := get("ui_dist")
	if err != nil {
		return resolvedArtifacts{}, err
	}

	out := resolvedArtifacts{
		ManifestPath: manifestPath,
		RobotBin:     robotBin,
		AutoswapBin:  autoswapBin,
		ReplBin:      replBin,
		CameraBin:    cameraBin,
		MavlinkBin:   mavlinkBin,
		WLANBundle:   wlanBundle,
		UIDist:       uiDist,
	}
	for _, bin := range []string{out.AutoswapBin, out.RobotBin, out.ReplBin, out.CameraBin, out.MavlinkBin} {
		if err := requireExecutable(bin); err != nil {
			return resolvedArtifacts{}, err
		}
	}
	if err := requireDirWithIndex(out.UIDist); err != nil {
		return resolvedArtifacts{}, err
	}
	if err := requirePath(out.WLANBundle); err != nil {
		return resolvedArtifacts{}, err
	}
	return out, nil
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
