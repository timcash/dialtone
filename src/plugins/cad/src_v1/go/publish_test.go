package cad

import (
	"path/filepath"
	"strings"
	"testing"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func TestParseGitHubRemoteURL(t *testing.T) {
	owner, repo, ok := ParseGitHubRemoteURL("https://github.com/timcash/dialtone.git")
	if !ok || owner != "timcash" || repo != "dialtone" {
		t.Fatalf("unexpected https parse: ok=%t owner=%q repo=%q", ok, owner, repo)
	}

	owner, repo, ok = ParseGitHubRemoteURL("git@github.com:timcash/dialtone.git")
	if !ok || owner != "timcash" || repo != "dialtone" {
		t.Fatalf("unexpected ssh parse: ok=%t owner=%q repo=%q", ok, owner, repo)
	}
}

func TestResolvePublishPlanDefaults(t *testing.T) {
	paths := Paths{
		Runtime:  configv1.Runtime{},
		UIDir:    filepath.Join("ui"),
		StateDir: filepath.Join(".dialtone", "cad", "src_v1"),
	}
	plan, err := ResolvePublishPlan(paths, PublishOptions{
		RepositoryOwner: "timcash",
		RepositoryName:  "dialtone",
	})
	if err != nil {
		t.Fatalf("ResolvePublishPlan error: %v", err)
	}
	if plan.PagesBasePath != "/dialtone/cad-src-v1/" {
		t.Fatalf("unexpected pages base path: %q", plan.PagesBasePath)
	}
	if plan.PagesURL != "https://timcash.github.io/dialtone/cad-src-v1/" {
		t.Fatalf("unexpected pages url: %q", plan.PagesURL)
	}
	if plan.BackendOrigin != "https://cad-src-v1.dialtone.earth" {
		t.Fatalf("unexpected backend origin: %q", plan.BackendOrigin)
	}
}

func TestBuildPublishLandingHTMLPointsAtApp(t *testing.T) {
	html := BuildPublishLandingHTML(PublishPlan{SiteSubpath: "cad-src-v1"})
	if !strings.Contains(html, "./cad-src-v1/") {
		t.Fatalf("landing html missing app redirect: %s", html)
	}
}
