package repl

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func createRepoTarball(repoRoot, tarPath string) error {
	lsCmd := exec.Command("git", "-C", repoRoot, "ls-files", "--cached", "--modified", "--others", "--exclude-standard", "-z")
	files, err := lsCmd.Output()
	paths := make([]string, 0, 2048)
	if err == nil && len(files) > 0 {
		entries := bytes.Split(files, []byte{0})
		for _, e := range entries {
			rel := strings.TrimSpace(string(e))
			if rel == "" {
				continue
			}
			paths = append(paths, rel)
		}
	} else {
		walkErr := filepath.WalkDir(repoRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if shouldSkipTarPath(rel, d.IsDir()) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type().IsRegular() || d.Type()&os.ModeSymlink != 0 {
				paths = append(paths, rel)
			}
			return nil
		})
		if walkErr != nil {
			return fmt.Errorf("fallback file walk failed: %w", walkErr)
		}
	}
	if len(paths) == 0 {
		return fmt.Errorf("no files discovered for bootstrap tarball in %s", repoRoot)
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

	for _, relRaw := range paths {
		rel := strings.TrimSpace(relRaw)
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

func shouldSkipTarPath(rel string, isDir bool) bool {
	if rel == "" {
		return false
	}
	base := filepath.Base(rel)
	if base == ".DS_Store" {
		return true
	}
	if strings.HasPrefix(rel, ".git/") || rel == ".git" {
		return true
	}
	if strings.HasPrefix(rel, ".dialtone/") || rel == ".dialtone" {
		return true
	}
	if strings.HasPrefix(rel, "dialtone_dependencies/") || rel == "dialtone_dependencies" {
		return true
	}
	if strings.HasPrefix(rel, "node_modules/") || rel == "node_modules" {
		return true
	}
	if isDir && strings.HasPrefix(base, ".tmp-") {
		return true
	}
	return false
}
