package autoswap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeManifestURLForAutoUpdateUsesChannelAsset(t *testing.T) {
	got, changed := normalizeManifestURLForAutoUpdate(
		"https://github.com/timcash/dialtone/releases/download/robot-src-v2-abcd1234/robot_src_v2_composition_manifest-robot-src-v2-abcd1234.json",
		"timcash/dialtone",
	)
	if !changed {
		t.Fatalf("expected normalizeManifestURLForAutoUpdate to rewrite versioned manifest URL")
	}
	want := "https://github.com/timcash/dialtone/releases/latest/download/robot_src_v2_channel.json"
	if got != want {
		t.Fatalf("unexpected normalized URL: got %q want %q", got, want)
	}
}

func TestResolveManifestPathDirectURLUsesContentAddressedPath(t *testing.T) {
	var body = `{"name":"robot","version":"src_v2","runtime":{"binary":"x","processes":[]},"artifacts":{"sync":{},"release":{}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	got, err := resolveManifestPath("", srv.URL+"/manifest.json", dir, "")
	if err != nil {
		t.Fatalf("resolveManifestPath failed: %v", err)
	}
	sum := sha256.Sum256([]byte(body))
	wantBase := fmt.Sprintf("manifest-%s.json", hex.EncodeToString(sum[:8]))
	if filepath.Base(got) != wantBase {
		t.Fatalf("unexpected manifest cache path: got %q want %q", filepath.Base(got), wantBase)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("resolved manifest path missing: %v", err)
	}
}

func TestResolveManifestPathChannelResolvesImmutableManifest(t *testing.T) {
	manifest := `{"name":"robot","version":"src_v2","release_version":"robot-src-v2-abcd1234","runtime":{"binary":"x","processes":[]},"artifacts":{"sync":{},"release":{}}}`
	sum := sha256.Sum256([]byte(manifest + "\n"))
	manifestSHA := hex.EncodeToString(sum[:])

	baseURL := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robot_src_v2_channel.json":
			_, _ = w.Write([]byte(`{
  "schema_version":"v1",
  "name":"robot-src_v2",
  "channel":"latest",
  "release_version":"robot-src-v2-abcd1234",
  "manifest_url":"` + baseURL + `/robot_src_v2_composition_manifest-robot-src-v2-abcd1234.json",
  "manifest_sha256":"` + manifestSHA + `"
}`))
		case "/robot_src_v2_composition_manifest-robot-src-v2-abcd1234.json":
			_, _ = w.Write([]byte(manifest + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	baseURL = srv.URL

	dir := t.TempDir()
	got, err := resolveManifestPath("", srv.URL+"/robot_src_v2_channel.json", dir, "")
	if err != nil {
		t.Fatalf("resolveManifestPath(channel) failed: %v", err)
	}
	raw, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read resolved manifest failed: %v", err)
	}
	if !strings.Contains(string(raw), `"release_version":"robot-src-v2-abcd1234"`) {
		t.Fatalf("resolved path does not contain expected immutable manifest contents")
	}
}
