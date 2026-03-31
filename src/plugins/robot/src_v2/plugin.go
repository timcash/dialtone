package robotv2

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	test_plugin "dialtone/dev/plugins/test/src_v1/go"
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

func runSrcV2Publish(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-publish", flag.ContinueOnError)
	repo := fs.String("repo", "timcash/dialtone", "GitHub repo owner/name")
	version := fs.String("version", "", "Release version/tag (default: current git tag or robot-src-v2-<sha>)")
	skipRelease := fs.Bool("skip-release", false, "Skip GitHub release publish check/upload")
	targetFlag := fs.String("target", "linux-arm64", "Release target GOOS-GOARCH (default: linux-arm64)")
	allTargets := fs.Bool("all-targets", false, "Build/publish all release targets (linux/darwin/windows variants)")
	uiOnly := fs.Bool("ui", false, "Publish only robot src_v2 UI dist artifacts (and manifest); skip binary builds")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := runDialtone(repoRoot, "robot", "src_v2", "build"); err != nil {
		return err
	}
	replIndexInfof("robot publish: local UI build complete")
	if *uiOnly {
		logs.Info("robot src_v2 publish: local UI artifacts built (--ui mode)")
	} else {
		logs.Info("robot src_v2 publish: local UI artifacts built; release asset builds handled by publish stage")
	}
	if !*skipRelease {
		resolvedVersion, err := resolveRobotPublishVersion(repoRoot, strings.TrimSpace(*version))
		if err != nil {
			return err
		}
		replIndexInfof("robot publish: preparing release assets for %s", strings.TrimSpace(*repo))
		targets, err := resolvePublishTargets(strings.TrimSpace(*targetFlag), *allTargets)
		if err != nil {
			return err
		}
		if err := publishRobotSrcV2Release(repoRoot, strings.TrimSpace(*repo), resolvedVersion, targets, *uiOnly); err != nil {
			return err
		}
		replIndexInfof("robot publish: release assets ready for version %s", resolvedVersion)
		logs.Info("robot src_v2 publish: release assets up to date version=%s repo=%s", resolvedVersion, strings.TrimSpace(*repo))
	}
	return nil
}

func ensureRobotUIDeps(repoRoot, uiDir string, force bool) error {
	if strings.TrimSpace(uiDir) == "" {
		return fmt.Errorf("robot ui directory is empty")
	}
	if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err != nil {
		return fmt.Errorf("robot ui package.json missing in %s: %w", uiDir, err)
	}

	logs.Info("robot install dependency: camera src_v1 install")
	if err := runDialtone(repoRoot, "camera", "src_v1", "install"); err != nil {
		return fmt.Errorf("camera dependency install failed: %w", err)
	}
	logs.Info("robot install dependency: github src_v1 install")
	if err := runDialtone(repoRoot, "github", "src_v1", "install"); err != nil {
		return fmt.Errorf("github dependency install failed: %w", err)
	}
	logs.Info("robot install dependency: bun src_v1 install")
	if err := runDialtone(repoRoot, "bun", "src_v1", "install"); err != nil {
		return fmt.Errorf("bun runtime install failed: %w", err)
	}

	viteBin := filepath.Join(uiDir, "node_modules", ".bin", "vite")
	if runtime.GOOS == "windows" {
		viteBin += ".cmd"
	}
	if !force {
		if _, err := os.Stat(viteBin); err == nil {
			return nil
		}
	}

	installArgs := []string{"bun", "src_v1", "install", "--cwd", uiDir, "--frozen-lockfile"}
	if force {
		installArgs = append(installArgs, "--force")
	}
	if err := runDialtone(repoRoot, installArgs...); err != nil {
		return fmt.Errorf("robot ui dependency install failed: %w", err)
	}
	if _, err := os.Stat(viteBin); err != nil {
		return fmt.Errorf("robot ui vite binary missing after install: %s", viteBin)
	}
	return nil
}

func ensureRobotLocalArtifacts(repoRoot string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	required := []string{
		configv1.PluginBinaryPath(rt, "autoswap", "src_v1", "dialtone_autoswap_v1"),
		configv1.PluginBinaryPath(rt, "robot", "src_v2", "dialtone_robot_v2"),
		configv1.PluginBinaryPath(rt, "camera", "src_v1", "dialtone_camera_v1"),
		configv1.PluginBinaryPath(rt, "mavlink", "src_v1", "dialtone_mavlink_v1"),
		configv1.PluginBinaryPath(rt, "repl", "src_v1", "dialtone_repl_v1"),
		filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "ui", "dist", "index.html"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			logs.Info("robot local artifacts missing; running ./dialtone.sh robot src_v2 build")
			return runDialtone(repoRoot, "robot", "src_v2", "build")
		}
	}
	return nil
}

func buildRobotLocalArtifacts(repoRoot string) error {
	rt, err := configv1.ResolveRuntime(repoRoot)
	if err != nil {
		return err
	}
	srcRoot := rt.SrcRoot
	goBin, err := resolveGoBinary()
	if err != nil {
		return err
	}
	specs := []struct {
		outPath   string
		mainPath  string
		useCamera bool
	}{
		{outPath: configv1.PluginBinaryPath(rt, "autoswap", "src_v1", "dialtone_autoswap_v1"), mainPath: "./plugins/autoswap/src_v1/cmd/main.go"},
		{outPath: configv1.PluginBinaryPath(rt, "robot", "src_v2", "dialtone_robot_v2"), mainPath: "./plugins/robot/src_v2/cmd/server/main.go"},
		{outPath: configv1.PluginBinaryPath(rt, "camera", "src_v1", "dialtone_camera_v1"), useCamera: true},
		{outPath: configv1.PluginBinaryPath(rt, "mavlink", "src_v1", "dialtone_mavlink_v1"), mainPath: "./plugins/mavlink/src_v1/cmd/main.go"},
		{outPath: configv1.PluginBinaryPath(rt, "repl", "src_v1", "dialtone_repl_v1"), mainPath: "./plugins/repl/src_v1/cmd/repld/main.go"},
	}
	for _, spec := range specs {
		if spec.useCamera {
			if err := runDialtone(repoRoot, "camera", "src_v1", "build", "--goos", runtime.GOOS, "--goarch", runtime.GOARCH, "--out", spec.outPath, "--podman=false"); err != nil {
				return err
			}
			continue
		}
		if err := buildGoBinary(goBin, srcRoot, spec.mainPath, spec.outPath, runtime.GOOS, runtime.GOARCH, ""); err != nil {
			return err
		}
	}
	return nil
}

type releaseAssetInfo struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type manifestChannelDoc struct {
	SchemaVersion  string `json:"schema_version"`
	Name           string `json:"name"`
	Channel        string `json:"channel"`
	Repo           string `json:"repo,omitempty"`
	ReleaseVersion string `json:"release_version"`
	ManifestURL    string `json:"manifest_url"`
	ManifestSHA256 string `json:"manifest_sha256,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
}

type releaseView struct {
	TagName string             `json:"tagName"`
	Assets  []releaseAssetInfo `json:"assets"`
}

type buildTarget struct {
	GOOS   string
	GOARCH string
}

func publishRobotSrcV2Release(repoRoot, repo, version string, targets []buildTarget, uiOnly bool) error {
	if strings.TrimSpace(repo) == "" {
		return fmt.Errorf("repo is required (owner/name)")
	}
	if len(targets) == 0 {
		return fmt.Errorf("robot src_v2 publish: no release targets selected")
	}
	srcRoot := filepath.Join(repoRoot, "src")
	outDir := configv1.PluginBinaryDir(configv1.Runtime{RepoRoot: repoRoot}, "robot", "src_v2", "releases", sanitizeVersion(version))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	goBin, err := resolveGoBinary()
	if err != nil {
		return err
	}
	if err := runDialtone(repoRoot, "robot", "src_v2", "build"); err != nil {
		return err
	}
	uiDist := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "ui", "dist")
	manifestSrc := filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json")

	specs := []struct {
		AssetPrefix string
		MainPath    string
	}{
		{AssetPrefix: "dialtone_autoswap", MainPath: "./plugins/autoswap/src_v1/cmd/main.go"},
		{AssetPrefix: "dialtone_robot_v2", MainPath: "./plugins/robot/src_v2/cmd/server/main.go"},
		{AssetPrefix: "dialtone_camera_v1", MainPath: "./plugins/camera/src_v1/cmd/main.go"},
		{AssetPrefix: "dialtone_mavlink_v1", MainPath: "./plugins/mavlink/src_v1/cmd/main.go"},
		{AssetPrefix: "dialtone_repl", MainPath: "./plugins/repl/src_v1/cmd/repld/main.go"},
	}

	existing, exists, err := githubReleaseAssets(repoRoot, repo, version)
	if err != nil {
		return err
	}

	assetPathByName := map[string]string{}
	for _, t := range targets {
		if !uiOnly {
			for _, s := range specs {
				name := s.AssetPrefix + "-" + t.GOOS + "-" + t.GOARCH
				if t.GOOS == "windows" {
					name += ".exe"
				}
				out := filepath.Join(outDir, name)
				if s.AssetPrefix == "dialtone_camera_v1" {
					if err := runDialtone(repoRoot, "camera", "src_v1", "build", "--goos", t.GOOS, "--goarch", t.GOARCH, "--out", out, "--podman=false"); err != nil {
						logs.Warn("robot src_v2 publish: skip asset %s (%s/%s camera build failed: %v)", name, t.GOOS, t.GOARCH, err)
						continue
					}
					assetPathByName[name] = out
					continue
				}
				ldflags := ""
				if s.AssetPrefix == "dialtone_robot_v2" {
					ldflags = fmt.Sprintf("-X main.embeddedAppVersion=%s", strings.TrimSpace(version))
				}
				if err := buildGoBinary(goBin, srcRoot, s.MainPath, out, t.GOOS, t.GOARCH, ldflags); err != nil {
					logs.Warn("robot src_v2 publish: skip asset %s (%s/%s build failed: %v)", name, t.GOOS, t.GOARCH, err)
					continue
				}
				assetPathByName[name] = out
			}
		}
		uiName := "robot_src_v2_ui_dist-" + t.GOOS + "-" + t.GOARCH + ".tar.gz"
		uiArchive := filepath.Join(outDir, uiName)
		if err := createTarGzFromDir(uiArchive, uiDist); err != nil {
			return err
		}
		assetPathByName[uiName] = uiArchive
	}
	manifestAssetName := "robot_src_v2_composition_manifest.json"
	manifestVersionedAssetName := "robot_src_v2_composition_manifest-" + sanitizeVersion(version) + ".json"
	manifestAssetPath := filepath.Join(outDir, manifestAssetName)
	manifestVersionedAssetPath := filepath.Join(outDir, manifestVersionedAssetName)
	manifestRaw, err := os.ReadFile(manifestSrc)
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: read manifest failed: %w", err)
	}
	var manifestDoc map[string]any
	if err := json.Unmarshal(manifestRaw, &manifestDoc); err != nil {
		return fmt.Errorf("robot src_v2 publish: parse manifest failed: %w", err)
	}
	assetSHA := map[string]string{}
	for name, digest := range existing {
		d := strings.TrimSpace(strings.TrimPrefix(digest, "sha256:"))
		if d != "" {
			assetSHA[name] = d
		}
	}
	for name, p := range assetPathByName {
		sum, serr := fileSHA256(p)
		if serr != nil {
			return fmt.Errorf("robot src_v2 publish: asset sha failed for %s: %w", name, serr)
		}
		assetSHA[name] = sum
	}
	manifestDoc["release_version"] = strings.TrimSpace(version)
	manifestDoc["release_published_at"] = time.Now().UTC().Format(time.RFC3339)
	manifestDoc["release_asset_sha256"] = assetSHA
	manifestDoc["manifest_asset"] = manifestVersionedAssetName
	if artifactsRaw, ok := manifestDoc["artifacts"].(map[string]any); ok {
		if releaseRaw, ok := artifactsRaw["release"].(map[string]any); ok {
			for depKey, bindingRaw := range releaseRaw {
				binding, ok := bindingRaw.(map[string]any)
				if !ok {
					continue
				}
				assetTpl, _ := binding["asset"].(string)
				assetTpl = strings.TrimSpace(assetTpl)
				if assetTpl == "" {
					continue
				}
				byTarget := map[string]string{}
				for _, t := range targets {
					targetKey := t.GOOS + "-" + t.GOARCH
					assetName := renderReleaseAssetTemplate(assetTpl, t.GOOS, t.GOARCH)
					if sha, ok := assetSHA[assetName]; ok && strings.TrimSpace(sha) != "" {
						byTarget[targetKey] = sha
					}
				}
				if len(byTarget) == 0 {
					continue
				}
				binding["sha256_by_target"] = byTarget
				hostKey := runtime.GOOS + "-" + runtime.GOARCH
				if v, ok := byTarget[hostKey]; ok {
					binding["sha256"] = v
				}
				releaseRaw[depKey] = binding
			}
			artifactsRaw["release"] = releaseRaw
		}
		manifestDoc["artifacts"] = artifactsRaw
	}
	manifestOut, err := json.MarshalIndent(manifestDoc, "", "  ")
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: marshal manifest failed: %w", err)
	}
	manifestOut = append(manifestOut, '\n')
	if err := os.WriteFile(manifestAssetPath, manifestOut, 0o644); err != nil {
		return fmt.Errorf("robot src_v2 publish: write manifest asset failed: %w", err)
	}
	if err := os.WriteFile(manifestVersionedAssetPath, manifestOut, 0o644); err != nil {
		return fmt.Errorf("robot src_v2 publish: write versioned manifest asset failed: %w", err)
	}
	assetPathByName[manifestAssetName] = manifestAssetPath
	assetPathByName[manifestVersionedAssetName] = manifestVersionedAssetPath
	manifestDigest, err := fileSHA256(manifestVersionedAssetPath)
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: manifest sha failed: %w", err)
	}
	channelAssetName := "robot_src_v2_channel.json"
	channelAssetPath := filepath.Join(outDir, channelAssetName)
	channelDoc := manifestChannelDoc{
		SchemaVersion:  "v1",
		Name:           "robot-src_v2",
		Channel:        "latest",
		Repo:           strings.TrimSpace(repo),
		ReleaseVersion: strings.TrimSpace(version),
		ManifestURL:    fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", strings.TrimSpace(repo), strings.TrimSpace(version), manifestVersionedAssetName),
		ManifestSHA256: manifestDigest,
		PublishedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	channelRaw, err := json.MarshalIndent(channelDoc, "", "  ")
	if err != nil {
		return fmt.Errorf("robot src_v2 publish: marshal channel asset failed: %w", err)
	}
	channelRaw = append(channelRaw, '\n')
	if err := os.WriteFile(channelAssetPath, channelRaw, 0o644); err != nil {
		return fmt.Errorf("robot src_v2 publish: write channel asset failed: %w", err)
	}
	assetPathByName[channelAssetName] = channelAssetPath

	if len(assetPathByName) == 0 {
		return fmt.Errorf("robot src_v2 publish: no release assets were built")
	}

	needsUpload := make([]string, 0, len(assetPathByName))
	for name, localPath := range assetPathByName {
		remoteDigest, ok := existing[name]
		if !ok {
			needsUpload = append(needsUpload, name)
			continue
		}
		localDigest, derr := fileSHA256(localPath)
		if derr != nil {
			return fmt.Errorf("robot src_v2 publish: digest failed for %s: %w", name, derr)
		}
		remoteDigest = strings.TrimSpace(strings.TrimPrefix(remoteDigest, "sha256:"))
		if remoteDigest == "" || !strings.EqualFold(remoteDigest, localDigest) {
			needsUpload = append(needsUpload, name)
		}
	}
	sort.Strings(needsUpload)
	if len(needsUpload) == 0 {
		replIndexInfof("robot publish: release %s already has matching assets", version)
		logs.Info("robot src_v2 publish: release %s already has all required assets with matching digests; skipping upload", version)
		return nil
	}

	gh, err := resolveGHCli(repoRoot)
	if err != nil {
		return err
	}
	assetPaths := make([]string, 0, len(needsUpload))
	for _, name := range needsUpload {
		assetPaths = append(assetPaths, assetPathByName[name])
	}
	replIndexInfof("robot publish: uploading %d release assets for %s", len(assetPaths), version)
	if !exists {
		args := []string{"release", "create", version, "--repo", repo, "--title", "Robot src_v2 " + version, "--notes", "Automated robot src_v2 publish " + version}
		args = append(args, assetPaths...)
		cmd := exec.Command(gh, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		logs.Info("robot src_v2 publish: created release %s with %d assets", version, len(assetPaths))
		return nil
	}

	args := []string{"release", "upload", version, "--repo", repo, "--clobber"}
	args = append(args, assetPaths...)
	cmd := exec.Command(gh, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	logs.Info("robot src_v2 publish: uploaded %d changed/missing assets to %s", len(assetPaths), version)
	return nil
}

func resolvePublishTargets(target string, all bool) ([]buildTarget, error) {
	if all {
		return []buildTarget{
			{GOOS: "linux", GOARCH: "amd64"},
			{GOOS: "linux", GOARCH: "arm64"},
			{GOOS: "darwin", GOARCH: "amd64"},
			{GOOS: "darwin", GOARCH: "arm64"},
			{GOOS: "windows", GOARCH: "amd64"},
		}, nil
	}
	v := strings.TrimSpace(strings.ToLower(target))
	if v == "" {
		v = "linux-arm64"
	}
	parts := strings.Split(v, "-")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return nil, fmt.Errorf("invalid --target %q (expected <goos>-<goarch>, e.g. linux-arm64)", target)
	}
	return []buildTarget{{GOOS: strings.TrimSpace(parts[0]), GOARCH: strings.TrimSpace(parts[1])}}, nil
}

func resolveRobotPublishVersion(repoRoot, requested string) (string, error) {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested), nil
	}
	if v := strings.TrimSpace(os.Getenv("ROBOT_SRC_V2_PUBLISH_VERSION")); v != "" {
		return v, nil
	}
	tagCmd := exec.Command("git", "describe", "--tags", "--exact-match")
	tagCmd.Dir = repoRoot
	if out, err := tagCmd.CombinedOutput(); err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			return v, nil
		}
	}
	shaCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	shaCmd.Dir = repoRoot
	out, err := shaCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve publish version failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("resolve publish version failed: empty git sha")
	}
	return "robot-src-v2-" + sha, nil
}

func resolveGoBinary() (string, error) {
	candidate := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return exec.LookPath("go")
}

func renderReleaseAssetTemplate(raw, goos, goarch string) string {
	v := strings.TrimSpace(raw)
	v = strings.ReplaceAll(v, "${goos}", goos)
	v = strings.ReplaceAll(v, "${goarch}", goarch)
	v = strings.ReplaceAll(v, "<goos>", goos)
	v = strings.ReplaceAll(v, "<goarch>", goarch)
	return v
}

func buildGoBinary(goBin, srcRoot, mainPath, out, goos, goarch, ldflags string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	args := []string{"build"}
	if strings.TrimSpace(ldflags) != "" {
		args = append(args, "-ldflags", strings.TrimSpace(ldflags))
	}
	args = append(args, "-o", out, mainPath)
	cmd := exec.Command(goBin, args...)
	cmd.Dir = srcRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createTarGzFromDir(outFile, srcDir string) error {
	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer f.Close()
	gzw := gzip.NewWriter(f)
	defer gzw.Close()
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		h, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		h.Name = rel
		if err := tw.WriteHeader(h); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(tw, in)
		return err
	})
}

func githubReleaseAssets(repoRoot, repo, version string) (map[string]string, bool, error) {
	gh, err := resolveGHCli(repoRoot)
	if err != nil {
		return nil, false, err
	}
	cmd := exec.Command(gh, "release", "view", version, "--repo", repo, "--json", "tagName,assets")
	out, err := cmd.CombinedOutput()
	if err != nil {
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no release found") {
			return map[string]string{}, false, nil
		}
		return nil, false, fmt.Errorf("gh release view failed: %v: %s", err, strings.TrimSpace(string(out)))
	}
	var rv releaseView
	if err := json.Unmarshal(out, &rv); err != nil {
		return nil, false, err
	}
	m := map[string]string{}
	for _, a := range rv.Assets {
		m[strings.TrimSpace(a.Name)] = strings.TrimSpace(a.Digest)
	}
	return m, true, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func resolveGHCli(repoRoot string) (string, error) {
	if p, err := exec.LookPath("gh"); err == nil {
		return p, nil
	}
	candidate := filepath.Join(logs.GetDialtoneEnv(), "gh", "bin", "gh")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	if strings.TrimSpace(repoRoot) != "" {
		if err := runDialtone(repoRoot, "github", "src_v1", "install"); err == nil {
			if p, err := exec.LookPath("gh"); err == nil {
				return p, nil
			}
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("gh cli not found; run ./dialtone.sh github src_v1 install")
}

func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "/", "-")
	v = strings.ReplaceAll(v, "\\", "-")
	v = strings.ReplaceAll(v, " ", "-")
	if v == "" {
		return time.Now().UTC().Format("20060102-150405")
	}
	return v
}

func runSrcV2Diagnostic(repoRoot string, args []string) error {
	fs := flag.NewFlagSet("robot-src-v2-diagnostic", flag.ContinueOnError)
	host := fs.String("host", strings.TrimSpace(os.Getenv("ROBOT_HOST")), "Robot SSH host")
	port := fs.String("port", "22", "Robot SSH port")
	user := fs.String("user", strings.TrimSpace(os.Getenv("ROBOT_USER")), "Robot SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "Robot SSH password")
	remoteRepo := fs.String("remote-repo", "", "Remote repo root (default: <remote-home>/dialtone)")
	manifest := fs.String("manifest", "src/plugins/robot/src_v2/config/composition.manifest.json", "Remote manifest path (absolute or repo-relative)")
	uiURL := fs.String("ui-url", "", "Robot UI URL for public checks + browser checks (default: "+defaultRobotPublicUIURL+")")
	browserNode := fs.String("browser-node", defaultRobotDevBrowserNode(), "Mesh node for remote browser (for example legion; use none/off/local to disable)")
	skipUI := fs.Bool("skip-ui", false, "Skip chromedp UI menu checks")
	publicCheck := fs.Bool("public-check", true, "Verify public UI endpoint is reachable")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := ensureRobotLocalArtifacts(repoRoot); err != nil {
		return err
	}

	required := []string{
		configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repoRoot}, "autoswap", "src_v1", "dialtone_autoswap_v1"),
		configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repoRoot}, "robot", "src_v2", "dialtone_robot_v2"),
		configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repoRoot}, "camera", "src_v1", "dialtone_camera_v1"),
		configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repoRoot}, "mavlink", "src_v1", "dialtone_mavlink_v1"),
		configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repoRoot}, "repl", "src_v1", "dialtone_repl_v1"),
		filepath.Join(repoRoot, "src", "plugins", "robot", "src_v2", "ui", "dist", "index.html"),
	}
	for _, p := range required {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("diagnostic missing local artifact: %s", p)
		}
	}
	replIndexInfof("robot diagnostic: checking local artifacts")
	logs.Info("robot src_v2 diagnostic: local artifact check passed")

	targetHost := strings.TrimSpace(*host)
	targetUser := strings.TrimSpace(*user)
	if targetHost == "" {
		logs.Warn("robot src_v2 diagnostic: no --host provided; skipped remote checks")
		return nil
	}
	if targetUser == "" {
		if node, err := ssh_plugin.ResolveMeshNode(targetHost); err == nil {
			targetUser = strings.TrimSpace(node.User)
			if targetUser != "" {
				*user = targetUser
			}
		}
	}
	if targetUser == "" {
		logs.Warn("robot src_v2 diagnostic: no --user provided and mesh node has no default user; skipped remote checks")
		return nil
	}
	replIndexInfof("robot diagnostic: checking robot runtime on %s", targetHost)

	targetPort := strings.TrimSpace(*port)
	targetPass := *pass
	client, node, _, _, meshErr := ssh_plugin.DialMeshNode(targetHost, ssh_plugin.CommandOptions{
		User:     targetUser,
		Port:     targetPort,
		Password: targetPass,
	})
	if meshErr != nil {
		// Fallback to direct host dial for non-mesh targets.
		directClient, err := ssh_plugin.DialSSH(targetHost, targetPort, targetUser, targetPass)
		if err != nil {
			return err
		}
		client = directClient
	} else {
		if strings.TrimSpace(*user) == "" {
			*user = node.User
		}
	}
	defer client.Close()

	remoteHomeOut, err := ssh_plugin.RunSSHCommand(client, "printf '%s' \"$HOME\"")
	if err != nil {
		return fmt.Errorf("remote home lookup failed: %w", err)
	}
	remoteHome := strings.TrimSpace(remoteHomeOut)
	if remoteHome == "" {
		return fmt.Errorf("remote home lookup returned empty value")
	}
	resolvedRemoteRepo := strings.TrimSpace(*remoteRepo)
	if resolvedRemoteRepo == "" {
		resolvedRemoteRepo = filepath.ToSlash(filepath.Join(remoteHome, "dialtone"))
	}
	autoswapRoot := filepath.ToSlash(filepath.Join(remoteHome, ".dialtone", "autoswap"))
	manifestAbs := resolveRemoteManifestPath(resolvedRemoteRepo, strings.TrimSpace(*manifest))
	remoteExecutableExists := func(path string) bool {
		if strings.TrimSpace(path) == "" {
			return false
		}
		_, err := ssh_plugin.RunSSHCommand(client, "test -x "+shellSingleQuote(path))
		return err == nil
	}
	remoteFileExists := func(path string) bool {
		if strings.TrimSpace(path) == "" {
			return false
		}
		_, err := ssh_plugin.RunSSHCommand(client, "test -f "+shellSingleQuote(path))
		return err == nil
	}
	selectExecutable := func(candidates []string) (string, error) {
		for _, c := range candidates {
			c = filepath.ToSlash(strings.TrimSpace(c))
			if remoteExecutableExists(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("no executable candidate exists: %v", candidates)
	}
	selectFile := func(candidates []string) (string, error) {
		for _, c := range candidates {
			c = filepath.ToSlash(strings.TrimSpace(c))
			if remoteFileExists(c) {
				return c, nil
			}
		}
		return "", fmt.Errorf("no file candidate exists: %v", candidates)
	}

	autoswapBin, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "plugins", "autoswap", "src_v1", "dialtone_autoswap_v1"),
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_autoswap_v1"),
		filepath.Join(autoswapRoot, "bin", "dialtone_autoswap_v1"),
	})
	if err != nil {
		return fmt.Errorf("diagnostic remote autoswap binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "plugins", "robot", "src_v2", "dialtone_robot_v2"),
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_robot_v2"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_robot_v2"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote robot binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "plugins", "camera", "src_v1", "dialtone_camera_v1"),
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_camera_v1"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_camera_v1"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote camera binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "plugins", "mavlink", "src_v1", "dialtone_mavlink_v1"),
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_mavlink_v1"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_mavlink_v1"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote mavlink binary check failed: %w", err)
	}
	if _, err := selectExecutable([]string{
		filepath.Join(resolvedRemoteRepo, "bin", "plugins", "repl", "src_v1", "dialtone_repl_v1"),
		filepath.Join(resolvedRemoteRepo, "bin", "dialtone_repl_v1"),
		filepath.Join(autoswapRoot, "artifacts", "dialtone_repl_v1"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote repl binary check failed: %w", err)
	}
	if _, err := selectFile([]string{
		filepath.Join(resolvedRemoteRepo, "src", "plugins", "robot", "src_v2", "ui", "dist", "index.html"),
		filepath.Join(autoswapRoot, "artifacts", "robot_src_v2_ui_dist", "index.html"),
	}); err != nil {
		return fmt.Errorf("diagnostic remote ui dist check failed: %w", err)
	}
	if !remoteFileExists(manifestAbs) {
		candidates := []string{
			filepath.ToSlash(filepath.Join(autoswapRoot, "manifests", "robot-src_v2.manifest.json")),
		}
		found := ""
		for _, c := range candidates {
			if remoteFileExists(c) {
				found = c
				break
			}
		}
		if found == "" {
			manifestDir := filepath.ToSlash(filepath.Join(autoswapRoot, "manifests"))
			latestManifestOut, lerr := ssh_plugin.RunSSHCommand(client, "find "+shellSingleQuote(manifestDir)+" -maxdepth 1 -type f -name 'manifest-*.json' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n1 | awk '{print $2}'")
			if lerr == nil {
				latestManifest := strings.TrimSpace(latestManifestOut)
				if latestManifest != "" && remoteFileExists(latestManifest) {
					found = latestManifest
				}
			}
		}
		if found == "" {
			return fmt.Errorf("diagnostic remote manifest check failed: %s", manifestAbs)
		}
		manifestAbs = found
	}
	logs.Info("robot src_v2 diagnostic: remote artifact check passed")

	activeOut, err := ssh_plugin.RunSSHCommand(client, "systemctl --user is-active dialtone_autoswap.service")
	if err != nil {
		return fmt.Errorf("autoswap service active check failed: %w", err)
	}
	if strings.TrimSpace(activeOut) != "active" {
		return fmt.Errorf("autoswap service is not active: %s", strings.TrimSpace(activeOut))
	}

	execOut, err := ssh_plugin.RunSSHCommand(client, "systemctl --user show dialtone_autoswap.service --property=ExecStart --no-pager")
	if err != nil {
		return fmt.Errorf("autoswap service ExecStart check failed: %w", err)
	}
	manifestURL := strings.TrimSpace(extractFlagValue(execOut, "--manifest-url"))
	if !strings.Contains(execOut, "dialtone_autoswap_v1") {
		return fmt.Errorf("autoswap service ExecStart does not reference dialtone_autoswap_v1")
	}
	if !strings.Contains(execOut, manifestAbs) && !strings.Contains(execOut, "--manifest-url") {
		return fmt.Errorf("autoswap service ExecStart does not reference manifest path %s or --manifest-url", manifestAbs)
	}
	logs.Info("robot src_v2 diagnostic: autoswap service is active and uses expected manifest")
	replIndexInfof("robot diagnostic: autoswap service and manifest look healthy")

	repoRootForList := ""
	if _, err := ssh_plugin.RunSSHCommand(client, "test -d "+shellSingleQuote(resolvedRemoteRepo)); err == nil {
		repoRootForList = resolvedRemoteRepo
	}
	runtimePath := filepath.ToSlash(filepath.Join(remoteHome, ".dialtone", "autoswap", "state", "runtime.json"))
	var runtimeState struct {
		ManifestPath string `json:"manifest_path"`
		Processes    []struct {
			Name   string `json:"name"`
			PID    int    `json:"pid"`
			Status string `json:"status"`
		} `json:"processes"`
	}
	loadRuntimeState := func() error {
		runtimeRaw, err := ssh_plugin.RunSSHCommand(client, "cat "+shellSingleQuote(runtimePath))
		if err != nil {
			return fmt.Errorf("autoswap runtime state read failed: %w", err)
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(runtimeRaw)), &runtimeState); err != nil {
			return fmt.Errorf("autoswap runtime state parse failed: %w", err)
		}
		if rp := strings.TrimSpace(runtimeState.ManifestPath); rp != "" {
			if remoteFileExists(rp) {
				manifestAbs = filepath.ToSlash(rp)
				logs.Info("robot src_v2 diagnostic: using runtime manifest path from state: %s", manifestAbs)
				return nil
			}
		}
		if manifestURL == "" {
			return nil
		}
		// When using --manifest-url, the manifest is typically downloaded under autoswap/manifests/.
		candidates := []string{
			filepath.ToSlash(filepath.Join(autoswapRoot, "manifests", "robot-src_v2.manifest.json")),
			filepath.ToSlash(filepath.Join(autoswapRoot, "manifests")),
		}
		found := ""
		for _, c := range candidates {
			if c == filepath.ToSlash(filepath.Join(autoswapRoot, "manifests")) {
				if latestManifestOut, lerr := ssh_plugin.RunSSHCommand(client, "find "+shellSingleQuote(c)+" -maxdepth 1 -type f -name 'manifest-*.json' -printf '%T@ %p\\n' 2>/dev/null | sort -nr | head -n1 | awk '{print $2}'"); lerr == nil {
					latestManifest := strings.TrimSpace(latestManifestOut)
					if latestManifest != "" && remoteFileExists(latestManifest) {
						found = latestManifest
						break
					}
				}
				continue
			}
			if remoteFileExists(c) {
				found = c
				break
			}
		}
		if found == "" {
			return fmt.Errorf("diagnostic could not resolve active autoswap manifest for --manifest-url mode")
		}
		manifestAbs = found
		return nil
	}
	runtimeStateSettlingLogged := false
	loadRuntimeStateWithRetry := func() error {
		var lastErr error
		for attempt := 0; attempt < 5; attempt++ {
			if err := loadRuntimeState(); err == nil {
				return nil
			} else {
				lastErr = err
				if !strings.Contains(err.Error(), "autoswap runtime state parse failed") {
					return err
				}
				if !runtimeStateSettlingLogged {
					replIndexInfof("robot diagnostic: waiting for autoswap runtime state to settle")
					runtimeStateSettlingLogged = true
				}
				time.Sleep(750 * time.Millisecond)
			}
		}
		return lastErr
	}
	if err := loadRuntimeStateWithRetry(); err != nil {
		return err
	}
	listCmd := shellSingleQuote(autoswapBin) + " service --mode list --manifest " + shellSingleQuote(manifestAbs)
	if strings.TrimSpace(repoRootForList) != "" {
		listCmd += " --repo-root " + shellSingleQuote(repoRootForList)
	}
	listOut, err := ssh_plugin.RunSSHCommand(client, listCmd)
	if err != nil {
		return fmt.Errorf("autoswap service --mode list failed: %w", err)
	}
	for _, token := range []string{"runtime", "supervisor"} {
		if !strings.Contains(strings.ToLower(listOut), token) {
			return fmt.Errorf("autoswap list output missing expected token %q", token)
		}
	}
	logs.Info("robot src_v2 diagnostic: autoswap list output looks valid")

	manifestRaw, err := ssh_plugin.RunSSHCommand(client, "cat "+shellSingleQuote(manifestAbs))
	if err != nil {
		return fmt.Errorf("autoswap manifest read failed: %w", err)
	}
	type manifestReleaseBinding struct {
		Asset          string            `json:"asset"`
		Type           string            `json:"type"`
		SHA256         string            `json:"sha256"`
		SHA256ByTarget map[string]string `json:"sha256_by_target"`
	}
	var manifestState struct {
		ReleaseVersion string `json:"release_version"`
		Runtime        struct {
			Binary    string `json:"binary"`
			Processes []struct {
				Name     string `json:"name"`
				Artifact string `json:"artifact"`
			} `json:"processes"`
		} `json:"runtime"`
		Artifacts struct {
			Sync    map[string]string                 `json:"sync"`
			Release map[string]manifestReleaseBinding `json:"release"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(manifestRaw)), &manifestState); err != nil {
		return fmt.Errorf("autoswap manifest parse failed: %w", err)
	}
	expectedProc := make(map[string]bool)
	expectedArtifactKeyByProc := make(map[string]string)
	for _, p := range manifestState.Runtime.Processes {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		expectedProc[name] = false
		expectedArtifactKeyByProc[name] = strings.TrimSpace(p.Artifact)
	}
	if len(expectedProc) == 0 {
		return fmt.Errorf("manifest has no runtime.processes entries")
	}
	if manifestURL == "" {
		return fmt.Errorf("autoswap service must use --manifest-url so diagnostic can prove the newest robot release is active")
	}
	if manifestURL != "" {
		if strings.TrimSpace(runtimeState.ManifestPath) == "" {
			return fmt.Errorf("active autoswap manifest is empty while service uses --manifest-url")
		}
		if strings.TrimSpace(manifestAbs) == "" || filepath.Clean(runtimeState.ManifestPath) != filepath.Clean(manifestAbs) {
			logs.Info("robot src_v2 diagnostic: active manifest path resolved from runtime state: %s", strings.TrimSpace(runtimeState.ManifestPath))
			manifestAbs = filepath.ToSlash(strings.TrimSpace(runtimeState.ManifestPath))
		}
	}
	if filepath.Clean(runtimeState.ManifestPath) != filepath.Clean(manifestAbs) {
		if strings.Contains(execOut, "--manifest-url") {
			if strings.TrimSpace(runtimeState.ManifestPath) == "" {
				return fmt.Errorf("active autoswap manifest is empty while service uses --manifest-url")
			}
			logs.Info("robot src_v2 diagnostic: active manifest path is %s, using this for checks", strings.TrimSpace(runtimeState.ManifestPath))
		} else {
			return fmt.Errorf("active autoswap manifest mismatch: got=%s expected=%s", runtimeState.ManifestPath, manifestAbs)
		}
	}
	for _, p := range runtimeState.Processes {
		name := strings.TrimSpace(p.Name)
		if _, ok := expectedProc[name]; !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(p.Status), "running") && p.PID > 0 {
			expectedProc[name] = true
		}
	}
	for name, ok := range expectedProc {
		if !ok {
			return fmt.Errorf("autoswap runtime state process is not running: %s", name)
		}
	}
	logs.Info("robot src_v2 diagnostic: autoswap runtime matches manifest processes")

	if manifestURL != "" {
		latestManifestHash, err := fetchResolvedManifestSHA256(manifestURL, 15*time.Second)
		if err != nil {
			return fmt.Errorf("latest manifest fetch failed (%s): %w", manifestURL, err)
		}
		remoteManifestHash := ""
		deadline := time.Now().Add(45 * time.Second)
		for {
			remoteManifestHashOut, herr := ssh_plugin.RunSSHCommand(client, "sha256sum "+shellSingleQuote(manifestAbs)+" | awk '{print $1}'")
			if herr != nil {
				return fmt.Errorf("active manifest hash read failed: %w", herr)
			}
			remoteManifestHash = strings.TrimSpace(remoteManifestHashOut)
			if strings.EqualFold(remoteManifestHash, latestManifestHash) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("active manifest is not latest from manifest-url: active=%s latest=%s", remoteManifestHash, latestManifestHash)
			}
			time.Sleep(2 * time.Second)
			if err := loadRuntimeState(); err != nil {
				return err
			}
		}
		logs.Info("robot src_v2 diagnostic: active manifest hash matches latest manifest-url")
		replIndexInfof("robot diagnostic: active manifest matches latest release channel")
	}
	expandAutoswapPath := func(raw string) string {
		v := filepath.ToSlash(strings.TrimSpace(raw))
		if v == "" {
			return ""
		}
		v = strings.ReplaceAll(v, "<autoswap_root>", autoswapRoot)
		return filepath.ToSlash(v)
	}
	remoteTargetKey, err := func() (string, error) {
		osOut, err := ssh_plugin.RunSSHCommand(client, "uname -s")
		if err != nil {
			return "", fmt.Errorf("detect remote os failed: %w", err)
		}
		archOut, err := ssh_plugin.RunSSHCommand(client, "uname -m")
		if err != nil {
			return "", fmt.Errorf("detect remote arch failed: %w", err)
		}
		osName := strings.ToLower(strings.TrimSpace(osOut))
		archName := strings.ToLower(strings.TrimSpace(archOut))
		goos := ""
		switch osName {
		case "linux":
			goos = "linux"
		case "darwin":
			goos = "darwin"
		default:
			return "", fmt.Errorf("unsupported remote os %q", osName)
		}
		goarch := ""
		switch archName {
		case "aarch64", "arm64":
			goarch = "arm64"
		case "armv7l", "arm":
			goarch = "arm"
		case "x86_64", "amd64":
			goarch = "amd64"
		default:
			return "", fmt.Errorf("unsupported remote arch %q", archName)
		}
		return goos + "-" + goarch, nil
	}()
	if err != nil {
		return err
	}
	manifestReleaseVersion := strings.TrimSpace(manifestState.ReleaseVersion)
	expectedArtifactPathByProc := make(map[string]string)
	for procName, artifactKey := range expectedArtifactKeyByProc {
		if strings.TrimSpace(artifactKey) == "" {
			return fmt.Errorf("manifest runtime process %s is missing an artifact binding", procName)
		}
		syncPath := expandAutoswapPath(manifestState.Artifacts.Sync[artifactKey])
		if syncPath == "" {
			return fmt.Errorf("manifest artifacts.sync is missing %s", artifactKey)
		}
		if !remoteFileExists(syncPath) {
			return fmt.Errorf("remote artifact missing for %s: %s", procName, syncPath)
		}
		releaseBinding, ok := manifestState.Artifacts.Release[artifactKey]
		if !ok {
			return fmt.Errorf("manifest artifacts.release is missing %s", artifactKey)
		}
		expectedSHA := strings.TrimSpace(releaseBinding.SHA256ByTarget[remoteTargetKey])
		if expectedSHA == "" {
			expectedSHA = strings.TrimSpace(releaseBinding.SHA256)
		}
		if expectedSHA == "" {
			return fmt.Errorf("manifest release digest is missing for %s target %s", artifactKey, remoteTargetKey)
		}
		actualSHAOut, err := ssh_plugin.RunSSHCommand(client, "sha256sum "+shellSingleQuote(syncPath)+" | awk '{print $1}'")
		if err != nil {
			return fmt.Errorf("remote artifact hash read failed for %s: %w", syncPath, err)
		}
		actualSHA := strings.TrimSpace(actualSHAOut)
		if !strings.EqualFold(actualSHA, expectedSHA) {
			return fmt.Errorf("remote artifact digest mismatch for %s: path=%s got=%s expected=%s", procName, syncPath, actualSHA, expectedSHA)
		}
		expectedArtifactPathByProc[procName] = syncPath
	}
	runtimeBinaryPath := expandAutoswapPath(manifestState.Runtime.Binary)
	if runtimeBinaryPath == "" {
		return fmt.Errorf("manifest runtime.binary is empty")
	}
	if robotArtifactPath, ok := expectedArtifactPathByProc["robot"]; ok && filepath.Clean(filepath.FromSlash(robotArtifactPath)) != filepath.Clean(filepath.FromSlash(runtimeBinaryPath)) {
		return fmt.Errorf("manifest runtime.binary mismatch: runtime.binary=%s robot.artifact=%s", runtimeBinaryPath, robotArtifactPath)
	}
	logs.Info("robot src_v2 diagnostic: release artifact digests match latest manifest target=%s", remoteTargetKey)

	procsOut := ""
	processesMatchReleaseArtifacts := false
	processCheckErr := ""
	for attempt := 0; attempt < 10; attempt++ {
		if err := loadRuntimeState(); err != nil {
			return err
		}
		pidArgs := make([]string, 0, len(runtimeState.Processes))
		missingRunning := ""
		expectedProcessNameByPID := make(map[string]string)
		expectedArtifactPathByPID := make(map[string]string)
		for name := range expectedProc {
			expectedProc[name] = false
		}
		for _, p := range runtimeState.Processes {
			name := strings.TrimSpace(p.Name)
			if _, ok := expectedProc[name]; ok && strings.EqualFold(strings.TrimSpace(p.Status), "running") && p.PID > 0 {
				expectedProc[name] = true
				pid := strconv.Itoa(p.PID)
				pidArgs = append(pidArgs, pid)
				expectedProcessNameByPID[pid] = name
				expectedArtifactPathByPID[pid] = expectedArtifactPathByProc[name]
			}
		}
		for name, ok := range expectedProc {
			if !ok {
				missingRunning = name
				break
			}
		}
		if missingRunning != "" || len(pidArgs) == 0 {
			time.Sleep(1 * time.Second)
			continue
		}
		nextOut, perr := ssh_plugin.RunSSHCommand(client, "ps -p "+strings.Join(pidArgs, ",")+" -o pid= -o args= || true")
		if perr != nil {
			return fmt.Errorf("remote process list failed: %w", perr)
		}
		procsOut = nextOut
		procLineByPID := make(map[string]string)
		for _, rawLine := range strings.Split(strings.TrimSpace(procsOut), "\n") {
			line := strings.TrimSpace(rawLine)
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			procLineByPID[fields[0]] = line
		}
		processesMatchReleaseArtifacts = true
		processCheckErr = ""
		for _, pid := range pidArgs {
			line := strings.TrimSpace(procLineByPID[pid])
			if line == "" {
				processesMatchReleaseArtifacts = false
				processCheckErr = fmt.Sprintf("remote process list missing expected managed pid %s", pid)
				break
			}
			expectedPath := strings.TrimSpace(expectedArtifactPathByPID[pid])
			if expectedPath != "" && !strings.Contains(line, expectedPath) {
				processesMatchReleaseArtifacts = false
				processCheckErr = fmt.Sprintf("managed process %s pid=%s is not running the latest artifact %s", expectedProcessNameByPID[pid], pid, expectedPath)
				break
			}
		}
		if processesMatchReleaseArtifacts {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !processesMatchReleaseArtifacts {
		return fmt.Errorf("%s", chooseNonEmpty(processCheckErr, fmt.Sprintf("remote process list missing expected managed pids: %s", strings.TrimSpace(procsOut))))
	}
	logs.Info("robot src_v2 diagnostic: latest release artifacts are the running managed processes version=%s target=%s", chooseNonEmpty(manifestReleaseVersion, "unknown"), remoteTargetKey)
	replIndexInfof("robot diagnostic: newest robot release artifacts are running")

	healthOut, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:18086/health")
	if err != nil {
		return fmt.Errorf("remote /health check failed: %w", err)
	}
	if strings.TrimSpace(healthOut) != "ok" {
		return fmt.Errorf("remote /health expected ok, got %q", strings.TrimSpace(healthOut))
	}
	initOut, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:18086/api/init")
	if err != nil {
		return fmt.Errorf("remote /api/init check failed: %w", err)
	}
	if !strings.Contains(initOut, "/natsws") {
		return fmt.Errorf("remote /api/init missing /natsws")
	}
	var remoteInit struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(initOut)), &remoteInit); err != nil {
		return fmt.Errorf("remote /api/init json parse failed: %w", err)
	}
	remoteRobotVersion := strings.TrimSpace(remoteInit.Version)
	if remoteRobotVersion == "" {
		return fmt.Errorf("remote /api/init returned empty version")
	}
	if manifestReleaseVersion != "" && remoteRobotVersion != manifestReleaseVersion {
		return fmt.Errorf("remote /api/init version mismatch: got=%s expected=%s", remoteRobotVersion, manifestReleaseVersion)
	}
	integOut, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:18086/api/integration-health")
	if err != nil {
		return fmt.Errorf("remote /api/integration-health check failed: %w", err)
	}
	if !strings.Contains(integOut, "\"camera\":{\"status\":\"configured\"}") {
		return fmt.Errorf("remote /api/integration-health missing configured camera")
	}
	if !strings.Contains(integOut, "\"mavlink\":{\"status\":\"ok\"}") {
		return fmt.Errorf("remote /api/integration-health missing live mavlink ok status")
	}
	streamCodeOut, err := ssh_plugin.RunSSHCommand(client, "curl -sS -I --max-time 5 -o /dev/null -w '%{http_code}' http://127.0.0.1:18086/stream")
	if err != nil {
		return fmt.Errorf("remote /stream status check failed: %w", err)
	}
	if strings.TrimSpace(streamCodeOut) != "200" {
		return fmt.Errorf("remote /stream expected HTTP 200, got %s", strings.TrimSpace(streamCodeOut))
	}
	natswsProbe, err := ssh_plugin.RunSSHCommand(client, "python3 - <<'PY'\nimport socket\n\ns = socket.create_connection(('127.0.0.1', 18086), timeout=2)\nreq = (\n    'GET /natsws HTTP/1.1\\r\\n'\n    'Host: 127.0.0.1:18086\\r\\n'\n    'Connection: Upgrade\\r\\n'\n    'Upgrade: websocket\\r\\n'\n    'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\\r\\n'\n    'Sec-WebSocket-Version: 13\\r\\n\\r\\n'\n)\ns.sendall(req.encode())\nresp = s.recv(64)\ns.close()\nprint(resp.decode(errors='replace'))\nPY")
	if err != nil {
		return fmt.Errorf("remote /natsws websocket handshake failed: %w", err)
	}
	natswsStatusLine := strings.TrimSpace(strings.SplitN(strings.TrimSpace(natswsProbe), "\n", 2)[0])
	if !strings.HasPrefix(natswsStatusLine, "HTTP/1.1 101") {
		return fmt.Errorf("remote /natsws expected websocket upgrade 101, got %s", natswsStatusLine)
	}
	cameraSidecarHealth, err := ssh_plugin.RunSSHCommand(client, "curl -fsS --max-time 5 http://127.0.0.1:19090/health")
	if err != nil {
		return fmt.Errorf("remote camera sidecar /health check failed: %w", err)
	}
	if strings.TrimSpace(cameraSidecarHealth) != "ok" {
		return fmt.Errorf("remote camera sidecar /health expected ok, got %q", strings.TrimSpace(cameraSidecarHealth))
	}
	streamProbeOut, err := ssh_plugin.RunSSHCommand(client, "python3 - <<'PY'\nimport urllib.request\nreq=urllib.request.Request('http://127.0.0.1:19090/stream')\nwith urllib.request.urlopen(req, timeout=8) as r:\n    ct=(r.headers.get('Content-Type') or '').lower()\n    chunk=r.read(2048)\nok='multipart/x-mixed-replace' in ct and b'--frame' in chunk\nprint('ok' if ok else 'bad')\nPY")
	if err != nil {
		return fmt.Errorf("remote camera stream payload probe failed: %w", err)
	}
	if strings.TrimSpace(streamProbeOut) != "ok" {
		return fmt.Errorf("remote camera stream payload probe did not return multipart frame boundary")
	}
	mavlinkLiveProbe, err := ssh_plugin.RunSSHCommand(client, "journalctl --user -u dialtone_autoswap.service --since '2 minutes ago' --no-pager | egrep '\\[MAVLINK-RAW\\] (HEARTBEAT|GLOBALPOSITIONINT)' | tail -n 8")
	if err != nil {
		return fmt.Errorf("remote mavlink telemetry liveness check failed: %w", err)
	}
	if !strings.Contains(mavlinkLiveProbe, "MAVLINK-RAW") {
		return fmt.Errorf("remote mavlink telemetry liveness check found no recent MAVLINK-RAW HEARTBEAT/GLOBALPOSITIONINT")
	}
	logs.Info("robot src_v2 diagnostic: remote endpoints passed (/health, /api/init, /api/integration-health, /stream, sidecar camera stream, mavlink telemetry liveness)")
	replIndexInfof("robot diagnostic: robot API and telemetry endpoints passed")

	resolvedUIURL := strings.TrimSpace(*uiURL)
	if resolvedUIURL == "" {
		resolvedUIURL = defaultRobotPublicUIURL
	}
	expectedRobotVersion := remoteRobotVersion
	if !*publicCheck {
		logs.Info("robot src_v2 diagnostic: skipping public UI verification (pass --public-check=true to re-enable)")
	} else {
		if !strings.Contains(resolvedUIURL, "://") {
			resolvedUIURL = "https://" + resolvedUIURL
		}
		uiBase := strings.TrimRight(resolvedUIURL, "/")
		publicHealthBody, err := fetchURLText(uiBase+"/health", 10*time.Second)
		if err != nil {
			return fmt.Errorf("public ui /health check failed (%s): %w", uiBase, err)
		}
		if strings.TrimSpace(publicHealthBody) != "ok" {
			return fmt.Errorf("public ui /health expected ok, got %q", strings.TrimSpace(publicHealthBody))
		}
		publicInitBody, err := fetchURLText(uiBase+"/api/init", 10*time.Second)
		if err != nil {
			return fmt.Errorf("public ui /api/init check failed (%s): %w", uiBase, err)
		}
		if !strings.Contains(publicInitBody, "/natsws") {
			return fmt.Errorf("public ui /api/init missing /natsws")
		}
		var publicInit struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(publicInitBody)), &publicInit); err != nil {
			return fmt.Errorf("public ui /api/init json parse failed: %w", err)
		}
		publicRobotVersion := strings.TrimSpace(publicInit.Version)
		if publicRobotVersion == "" {
			return fmt.Errorf("public ui /api/init returned empty version")
		}
		if publicRobotVersion != remoteRobotVersion {
			return fmt.Errorf("public ui version mismatch: public=%s remote=%s", publicRobotVersion, remoteRobotVersion)
		}
		if manifestReleaseVersion != "" && publicRobotVersion != manifestReleaseVersion {
			return fmt.Errorf("public ui version mismatch: public=%s expected=%s", publicRobotVersion, manifestReleaseVersion)
		}
		expectedRobotVersion = publicRobotVersion
		logs.Info("robot src_v2 diagnostic: public UI passed (%s) version=%s", uiBase, publicRobotVersion)
	}
	if !strings.Contains(resolvedUIURL, "://") {
		resolvedUIURL = "https://" + resolvedUIURL
	}
	if !*skipUI {
		if err := runRobotSrcV2MenuDiagnostic(resolvedUIURL, strings.TrimSpace(*browserNode), repoRoot, expectedRobotVersion); err != nil {
			return err
		}
		logs.Info("robot src_v2 diagnostic: UI menu checks passed (%s)", resolvedUIURL)
	}

	replIndexInfof("robot diagnostic: completed")
	logs.Info("robot src_v2 diagnostic: remote checks completed")
	return nil
}

func fetchURLText(rawURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func fetchResolvedManifestSHA256(rawURL string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, cacheBustedLatestReleaseURL(rawURL), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(string(body))
	var probe map[string]any
	if err := json.Unmarshal([]byte(trimmed), &probe); err == nil {
		if manifestURL, ok := probe["manifest_url"].(string); ok && strings.TrimSpace(manifestURL) != "" && probe["runtime"] == nil {
			if manifestSHA, ok := probe["manifest_sha256"].(string); ok {
				manifestSHA = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(manifestSHA), "sha256:"))
				if manifestSHA != "" {
					return manifestSHA, nil
				}
			}
			return fetchResolvedManifestSHA256(strings.TrimSpace(manifestURL), timeout)
		}
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:]), nil
}

func cacheBustedLatestReleaseURL(rawURL string) string {
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

func extractFlagValue(execStart, flagName string) string {
	fields := strings.Fields(execStart)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != flagName {
			continue
		}
		v := strings.TrimSpace(fields[i+1])
		v = strings.Trim(v, "\"';")
		return v
	}
	return ""
}

func runRobotSrcV2MenuDiagnostic(uiURL, browserNode, repoRoot, expectedRobotVersion string) error {
	reg := test_plugin.NewRegistry()
	urlBase := strings.TrimRight(strings.TrimSpace(uiURL), "/")
	if urlBase == "" {
		return fmt.Errorf("ui url is empty")
	}
	reg.Add(test_plugin.Step{
		Name:    "robot-src-v2-diagnostic-ui-menu",
		Timeout: 45 * time.Second,
		RunWithContext: func(ctx *test_plugin.StepContext) (test_plugin.StepRunResult, error) {
			opts := test_plugin.BrowserOptions{
				Headless:   true,
				GPU:        true,
				Role:       "test",
				RemoteNode: strings.TrimSpace(browserNode),
				URL:        "about:blank",
			}
			ctx.Infof("[ACTION] ensure browser role=%s remote_node=%s url=%s", opts.Role, opts.RemoteNode, opts.URL)
			if _, err := ctx.EnsureBrowser(opts); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] browser ready")
			if err := ctx.Goto(urlBase + "/#hero"); err != nil {
				return test_plugin.StepRunResult{}, fmt.Errorf("navigate robot ui: %w", err)
			}
			ctx.Infof("[ACTION] navigated to robot ui")
			if err := ctx.WaitForAriaLabel("Hero Section", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] hero section visible")
			if err := ctx.WaitForAriaLabelAttrEquals("Hero Section", "data-active", "true", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] hero section active")
			if err := ctx.WaitForAriaLabel("Toggle Global Menu", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] menu toggle visible")
			if err := ctx.ClickAriaLabel("Toggle Global Menu"); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] menu opened")
			if err := ctx.WaitForAriaLabel("Navigate Settings", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings nav visible")
			if err := ctx.ClickAriaLabel("Navigate Settings"); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings nav clicked")
			if err := ctx.WaitForAriaLabel("Settings Section", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings section visible")
			if err := ctx.WaitForAriaLabelAttrEquals("Settings Section", "data-active", "true", 8*time.Second); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			ctx.Infof("[ACTION] settings section active")
			expectedPrefix := "version:" + strings.TrimSpace(expectedRobotVersion)
			if err := waitForBrowserJSCondition(
				ctx,
				12*time.Second,
				fmt.Sprintf(`(() => {
				  const section = document.querySelector("[aria-label='Settings Section']");
				  const byAria = document.querySelector("button[aria-label='Robot Version Button']");
				  const byText = section ? Array.from(section.querySelectorAll("button")).find((b) => /^version:\S+/i.test((b.textContent || "").trim())) : null;
				  const btn = byAria || byText;
				  if (!(btn instanceof HTMLButtonElement)) return false;
				  const text = (btn.textContent || "").trim();
				  return text.startsWith(%q) || text.includes(":update");
				})()`, expectedPrefix),
				"settings section version button did not converge to backend version",
			); err != nil {
				var debugInfo string
				_ = ctx.Evaluate(`(() => {
				  const section = document.querySelector("[aria-label='Settings Section']");
				  const active = section ? section.getAttribute("data-active") : "";
				  const buttons = Array.from(document.querySelectorAll("[aria-label='Settings Section'] button")).map((b) => ({
				    text: (b.textContent || "").trim(),
				    aria: b.getAttribute("aria-label") || ""
				  }));
				  const allButtons = Array.from(document.querySelectorAll("button")).slice(0, 12).map((b) => ({
				    text: (b.textContent || "").trim(),
				    aria: b.getAttribute("aria-label") || ""
				  }));
				  return JSON.stringify({ active, buttons, allButtons });
				})()`, &debugInfo)
				return test_plugin.StepRunResult{}, fmt.Errorf("%w; debug=%s", err, strings.TrimSpace(debugInfo))
			}
			var settingsVersionText string
			if err := ctx.Evaluate(`(() => {
			  const section = document.querySelector("[aria-label='Settings Section']");
			  const byAria = document.querySelector("button[aria-label='Robot Version Button']");
			  const byText = section ? Array.from(section.querySelectorAll("button")).find((b) => /^version:\S+/i.test((b.textContent || "").trim())) : null;
			  const btn = byAria || byText;
			  if (!(btn instanceof HTMLButtonElement)) return "";
			  return (btn.textContent || "").trim();
			})()`, &settingsVersionText); err != nil {
				return test_plugin.StepRunResult{}, err
			}
			settingsVersionText = strings.TrimSpace(settingsVersionText)
			if settingsVersionText == "" {
				return test_plugin.StepRunResult{}, fmt.Errorf("settings version button text is empty")
			}
			if !strings.HasPrefix(settingsVersionText, expectedPrefix) && !strings.Contains(settingsVersionText, ":update") {
				return test_plugin.StepRunResult{}, fmt.Errorf("settings version button mismatch: got=%q expected_prefix=%q", settingsVersionText, expectedPrefix)
			}
			ctx.Infof("[ACTION] settings version button text: %s", settingsVersionText)
			return test_plugin.StepRunResult{Report: "diagnostic settings version button passed"}, nil
		},
	})
	return reg.Run(test_plugin.SuiteOptions{
		Version:       "robot-src-v2-diagnostic-ui",
		RepoRoot:      repoRoot,
		ReportPath:    "plugins/robot/src_v2/test/DIAGNOSTIC_TEST.md",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.robot-src-v2-diagnostic-ui",
		AutoStartNATS: true,
	})
}

func waitForBrowserJSCondition(ctx *test_plugin.StepContext, timeout time.Duration, expr string, timeoutMsg string) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var ok bool
		if err := ctx.Evaluate(expr, &ok); err == nil && ok {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if strings.TrimSpace(timeoutMsg) == "" {
		timeoutMsg = "browser condition timed out"
	}
	return fmt.Errorf("%s", timeoutMsg)
}

func resolveRemoteManifestPath(remoteRepo, manifest string) string {
	m := strings.TrimSpace(manifest)
	if m == "" {
		return filepath.ToSlash(filepath.Join(remoteRepo, "src", "plugins", "robot", "src_v2", "config", "composition.manifest.json"))
	}
	if strings.HasPrefix(m, "/") {
		return filepath.ToSlash(filepath.Clean(m))
	}
	if strings.HasPrefix(m, "src/") {
		return filepath.ToSlash(filepath.Join(remoteRepo, m))
	}
	if strings.HasPrefix(m, "plugins/") {
		return filepath.ToSlash(filepath.Join(remoteRepo, "src", m))
	}
	return filepath.ToSlash(filepath.Join(remoteRepo, m))
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(strings.TrimSpace(s), "'", "'\\''") + "'"
}

func runDialtone(repoRoot string, args ...string) error {
	cmd := exec.Command(filepath.Join(repoRoot, "dialtone.sh"), args...)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
