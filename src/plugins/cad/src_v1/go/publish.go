package cad

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

type PublishOptions struct {
	BackendPort     int
	TunnelName      string
	Domain          string
	SiteSubpath     string
	OutputDir       string
	BackendOrigin   string
	RepositoryOwner string
	RepositoryName  string
	PagesOnly       bool
}

type PublishPlan struct {
	RepositoryOwner string `json:"repository_owner"`
	RepositoryName  string `json:"repository_name"`
	SiteSubpath     string `json:"site_subpath"`
	PagesBasePath   string `json:"pages_base_path"`
	PagesURL        string `json:"pages_url"`
	TunnelName      string `json:"tunnel_name"`
	Domain          string `json:"domain"`
	BackendHostname string `json:"backend_hostname"`
	BackendOrigin   string `json:"backend_origin"`
	LocalBackendURL string `json:"local_backend_url"`
	BackendPort     int    `json:"backend_port"`
	OutputRoot      string `json:"output_root"`
	AppDir          string `json:"app_dir"`
	AppIndex        string `json:"app_index"`
	App404          string `json:"app_404"`
	RootIndex       string `json:"root_index"`
	NoJekyllPath    string `json:"no_jekyll_path"`
	MetadataPath    string `json:"metadata_path"`
	StateDir        string `json:"state_dir"`
	StatePath       string `json:"state_path"`
}

func RunPublish(args []string) (PublishPlan, error) {
	opts, err := parsePublishOptions(args)
	if err != nil {
		return PublishPlan{}, err
	}

	paths, err := ResolvePaths("", "src_v1")
	if err != nil {
		return PublishPlan{}, err
	}
	_ = configv1.LoadEnvFile(paths.Runtime)
	_ = configv1.ApplyRuntimeEnv(paths.Runtime)

	plan, err := ResolvePublishPlan(paths, opts)
	if err != nil {
		return PublishPlan{}, err
	}

	if !opts.PagesOnly && strings.TrimSpace(opts.BackendOrigin) == "" {
		if err := ensurePublishServer(paths, plan); err != nil {
			return PublishPlan{}, err
		}
		if err := ensurePublishTunnel(paths, plan); err != nil {
			return PublishPlan{}, err
		}
	}

	if err := buildPublishPages(paths, plan); err != nil {
		return PublishPlan{}, err
	}
	if err := writePublishState(plan); err != nil {
		return PublishPlan{}, err
	}
	return plan, nil
}

func ResolvePublishPlan(paths Paths, opts PublishOptions) (PublishPlan, error) {
	owner := strings.TrimSpace(opts.RepositoryOwner)
	repo := strings.TrimSpace(opts.RepositoryName)
	if owner == "" || repo == "" {
		resolvedOwner, resolvedRepo, err := resolveGitHubRepository(paths.Runtime)
		if err != nil {
			return PublishPlan{}, err
		}
		if owner == "" {
			owner = resolvedOwner
		}
		if repo == "" {
			repo = resolvedRepo
		}
	}

	siteSubpath := NormalizeSiteSubpath(opts.SiteSubpath)
	if siteSubpath == "" {
		siteSubpath = "cad-src-v1"
	}
	tunnelName := sanitizeHostLabel(opts.TunnelName)
	if tunnelName == "" {
		tunnelName = "cad-src-v1"
	}
	domain := normalizePublishDomain(opts.Domain)
	if domain == "" {
		domain = "dialtone.earth"
	}
	backendHostname := BuildBackendHostname(tunnelName, domain)
	backendOrigin := strings.TrimSpace(opts.BackendOrigin)
	if backendOrigin == "" {
		backendOrigin = "https://" + backendHostname
	}
	backendPort := opts.BackendPort
	if backendPort <= 0 {
		backendPort = 8081
	}

	outputRoot := strings.TrimSpace(opts.OutputDir)
	if outputRoot == "" {
		outputRoot = filepath.Join(paths.UIDir, "dist-pages")
	}
	outputRoot, _ = filepath.Abs(outputRoot)
	appDir := filepath.Join(outputRoot, siteSubpath)
	stateDir := filepath.Join(paths.StateDir, "publish")

	return PublishPlan{
		RepositoryOwner: owner,
		RepositoryName:  repo,
		SiteSubpath:     siteSubpath,
		PagesBasePath:   BuildPagesBasePath(repo, siteSubpath),
		PagesURL:        BuildPagesURL(owner, repo, siteSubpath),
		TunnelName:      tunnelName,
		Domain:          domain,
		BackendHostname: backendHostname,
		BackendOrigin:   backendOrigin,
		LocalBackendURL: fmt.Sprintf("http://127.0.0.1:%d", backendPort),
		BackendPort:     backendPort,
		OutputRoot:      outputRoot,
		AppDir:          appDir,
		AppIndex:        filepath.Join(appDir, "index.html"),
		App404:          filepath.Join(appDir, "404.html"),
		RootIndex:       filepath.Join(outputRoot, "index.html"),
		NoJekyllPath:    filepath.Join(outputRoot, ".nojekyll"),
		MetadataPath:    filepath.Join(appDir, "publish.json"),
		StateDir:        stateDir,
		StatePath:       filepath.Join(stateDir, "publish-state.json"),
	}, nil
}

func NormalizeSiteSubpath(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	raw = strings.Trim(raw, "/")
	parts := []string{}
	for _, part := range strings.Split(raw, "/") {
		part = sanitizeHostLabel(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/")
}

func BuildPagesBasePath(repoName, siteSubpath string) string {
	repoName = strings.TrimSpace(repoName)
	siteSubpath = NormalizeSiteSubpath(siteSubpath)
	if repoName == "" {
		return "/"
	}
	base := "/" + repoName + "/"
	if siteSubpath == "" {
		return base
	}
	return base + siteSubpath + "/"
}

func BuildPagesURL(owner, repoName, siteSubpath string) string {
	owner = strings.TrimSpace(owner)
	repoName = strings.TrimSpace(repoName)
	siteSubpath = NormalizeSiteSubpath(siteSubpath)
	if owner == "" || repoName == "" {
		return ""
	}
	base := fmt.Sprintf("https://%s.github.io/%s/", owner, repoName)
	if siteSubpath == "" {
		return base
	}
	return base + siteSubpath + "/"
}

func BuildBackendHostname(tunnelName, domain string) string {
	tunnelName = sanitizeHostLabel(tunnelName)
	domain = normalizePublishDomain(domain)
	if tunnelName == "" {
		tunnelName = "cad-src-v1"
	}
	if domain == "" {
		domain = "dialtone.earth"
	}
	return tunnelName + "." + domain
}

func BuildPublishLandingHTML(plan PublishPlan) string {
	target := "./" + NormalizeSiteSubpath(plan.SiteSubpath) + "/"
	title := "Dialtone CAD Publish"
	return strings.TrimSpace(fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta http-equiv="refresh" content="0; url=%s" />
    <title>%s</title>
    <style>
      :root { color-scheme: dark; }
      body {
        margin: 0;
        min-height: 100vh;
        display: grid;
        place-items: center;
        background: radial-gradient(circle at top, #2d2d2d 0%%, #050505 58%%);
        color: #f5f5f5;
        font: 16px/1.5 "Segoe UI", sans-serif;
      }
      main {
        width: min(36rem, calc(100vw - 3rem));
        padding: 2rem;
        border: 1px solid rgba(255,255,255,0.12);
        background: rgba(8,8,8,0.8);
        box-shadow: 0 20px 50px rgba(0,0,0,0.35);
      }
      a { color: #ffffff; }
      p { color: rgba(255,255,255,0.72); }
    </style>
  </head>
  <body>
    <main>
      <h1>%s</h1>
      <p>Redirecting to the published CAD application.</p>
      <p><a href="%s">Open the CAD PWA</a></p>
    </main>
  </body>
</html>`, target, title, title, target))
}

func MarshalPublishMetadata(plan PublishPlan) ([]byte, error) {
	return json.MarshalIndent(plan, "", "  ")
}

func parsePublishOptions(args []string) (PublishOptions, error) {
	opts := PublishOptions{BackendPort: 8081}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--pages-only":
			opts.PagesOnly = true
		case arg == "--backend-port":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--backend-port requires a value")
			}
			value, err := strconvAtoi(args[i])
			if err != nil {
				return PublishOptions{}, fmt.Errorf("invalid --backend-port: %w", err)
			}
			opts.BackendPort = value
		case strings.HasPrefix(arg, "--backend-port="):
			value, err := strconvAtoi(strings.TrimPrefix(arg, "--backend-port="))
			if err != nil {
				return PublishOptions{}, fmt.Errorf("invalid --backend-port: %w", err)
			}
			opts.BackendPort = value
		case arg == "--tunnel-name":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--tunnel-name requires a value")
			}
			opts.TunnelName = args[i]
		case strings.HasPrefix(arg, "--tunnel-name="):
			opts.TunnelName = strings.TrimPrefix(arg, "--tunnel-name=")
		case arg == "--domain":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--domain requires a value")
			}
			opts.Domain = args[i]
		case strings.HasPrefix(arg, "--domain="):
			opts.Domain = strings.TrimPrefix(arg, "--domain=")
		case arg == "--site-subpath":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--site-subpath requires a value")
			}
			opts.SiteSubpath = args[i]
		case strings.HasPrefix(arg, "--site-subpath="):
			opts.SiteSubpath = strings.TrimPrefix(arg, "--site-subpath=")
		case arg == "--output-dir":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--output-dir requires a value")
			}
			opts.OutputDir = args[i]
		case strings.HasPrefix(arg, "--output-dir="):
			opts.OutputDir = strings.TrimPrefix(arg, "--output-dir=")
		case arg == "--backend-origin":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--backend-origin requires a value")
			}
			opts.BackendOrigin = args[i]
		case strings.HasPrefix(arg, "--backend-origin="):
			opts.BackendOrigin = strings.TrimPrefix(arg, "--backend-origin=")
		case arg == "--repo-owner":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--repo-owner requires a value")
			}
			opts.RepositoryOwner = args[i]
		case strings.HasPrefix(arg, "--repo-owner="):
			opts.RepositoryOwner = strings.TrimPrefix(arg, "--repo-owner=")
		case arg == "--repo-name":
			i++
			if i >= len(args) {
				return PublishOptions{}, fmt.Errorf("--repo-name requires a value")
			}
			opts.RepositoryName = args[i]
		case strings.HasPrefix(arg, "--repo-name="):
			opts.RepositoryName = strings.TrimPrefix(arg, "--repo-name=")
		default:
			return PublishOptions{}, fmt.Errorf("unknown publish flag: %s", arg)
		}
	}
	return opts, nil
}

func strconvAtoi(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	return value, nil
}

func normalizePublishDomain(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		raw = strings.TrimSpace(configv1.LookupEnvString("DIALTONE_DOMAIN"))
	}
	raw = strings.Trim(raw, ".")
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, ".") {
		return "dialtone.earth"
	}
	return raw
}

func sanitizeHostLabel(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "-", "_", "-", ".", "-", "/", "-", "\\", "-")
	raw = replacer.Replace(raw)
	out := make([]rune, 0, len(raw))
	lastDash := false
	for _, r := range raw {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if isLetter || isDigit {
			out = append(out, r)
			lastDash = false
			continue
		}
		if !lastDash {
			out = append(out, '-')
			lastDash = true
		}
	}
	return strings.Trim(string(out), "-")
}

func resolveGitHubRepository(rt configv1.Runtime) (string, string, error) {
	if repoEnv := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); repoEnv != "" {
		parts := strings.Split(repoEnv, "/")
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
		}
	}

	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = rt.RepoRoot
	raw, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("resolve git remote origin: %w", err)
	}
	owner, repo, ok := ParseGitHubRemoteURL(strings.TrimSpace(string(raw)))
	if !ok {
		return "", "", fmt.Errorf("parse github remote: %s", strings.TrimSpace(string(raw)))
	}
	return owner, repo, nil
}

func ParseGitHubRemoteURL(raw string) (string, string, bool) {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if raw == "" {
		return "", "", false
	}
	if strings.HasPrefix(raw, "git@github.com:") {
		parts := strings.Split(strings.TrimPrefix(raw, "git@github.com:"), "/")
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
		return "", "", false
	}
	for _, prefix := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(raw, prefix) {
			parts := strings.Split(strings.TrimPrefix(raw, prefix), "/")
			if len(parts) >= 2 {
				return parts[0], parts[1], true
			}
		}
	}
	return "", "", false
}
