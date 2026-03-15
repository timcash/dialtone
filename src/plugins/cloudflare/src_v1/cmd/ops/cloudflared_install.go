package ops

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func ensureCloudflaredInstalled(rt configv1.Runtime, force bool) (string, error) {
	dst := cloudflaredInstallPath(rt)
	if !force {
		if info, err := os.Stat(dst); err == nil && info.Mode().IsRegular() {
			return dst, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	asset, archiveType, err := cloudflaredDownloadSpec()
	if err != nil {
		return "", err
	}
	url := "https://github.com/cloudflare/cloudflared/releases/latest/download/" + asset
	logs.Info("cloudflare src_v1 install: downloading %s", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download cloudflared failed: status=%d", resp.StatusCode)
	}
	tmpFile := dst + ".tmp"
	defer os.Remove(tmpFile)
	switch archiveType {
	case "tgz":
		if err := writeCloudflaredFromTGZ(resp.Body, tmpFile); err != nil {
			return "", err
		}
	case "bin":
		if err := writeExecutableFile(tmpFile, resp.Body); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported cloudflared archive type %q", archiveType)
	}
	if err := os.Rename(tmpFile, dst); err != nil {
		return "", err
	}
	if err := os.Chmod(dst, 0o755); err != nil {
		return "", err
	}
	return dst, nil
}

func cloudflaredInstallPath(rt configv1.Runtime) string {
	name := "cloudflared"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(strings.TrimSpace(rt.DialtoneEnv), "cloudflare", name)
}

func cloudflaredDownloadSpec() (asset string, archiveType string, err error) {
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "cloudflared-darwin-arm64.tgz", "tgz", nil
		case "amd64":
			return "cloudflared-darwin-amd64.tgz", "tgz", nil
		}
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "cloudflared-linux-amd64", "bin", nil
		case "arm64":
			return "cloudflared-linux-arm64", "bin", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "cloudflared-windows-amd64.exe", "bin", nil
		}
	}
	return "", "", fmt.Errorf("cloudflared install unsupported on %s/%s", runtime.GOOS, runtime.GOARCH)
}

func writeExecutableFile(path string, src io.Reader) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, src)
	return err
}

func writeCloudflaredFromTGZ(src io.Reader, path string) error {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr == nil || hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(strings.TrimSpace(hdr.Name))
		if name != "cloudflared" && name != "cloudflared.exe" {
			continue
		}
		return writeExecutableFile(path, tr)
	}
	return fmt.Errorf("cloudflared binary not found in archive")
}
