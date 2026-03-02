package ssh

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

var defaultSyncExcludes = []string{
	".git/",
	"bin/",
	"node_modules/",
	"**/node_modules/",
	".pixi/",
	"**/.pixi/",
	".direnv/",
	"**/.direnv/",
	".DS_Store",
	"dist/",
	"**/dist/",
	"tmp/",
	"**/tmp/",
	".dialtone/",
	"**/.dialtone/",
}

type SyncCodeOptions struct {
	Node     string
	Source   string
	Dest     string
	Delete   bool
	Excludes []string
}

func SyncCode(opts SyncCodeOptions) error {
	src := strings.TrimSpace(opts.Source)
	if src == "" {
		cwd, _ := os.Getwd()
		src = cwd
	}
	src = strings.TrimRight(src, "/")
	if src == "" {
		return fmt.Errorf("source path is empty")
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source path missing: %s", src)
	}

	node := strings.TrimSpace(opts.Node)
	if node == "" {
		return fmt.Errorf("host is required")
	}
	if node == "all" {
		return syncCodeAll(opts, src)
	}

	target, err := ResolveMeshNode(node)
	if err != nil {
		return err
	}
	dest := strings.TrimSpace(opts.Dest)
	if dest == "" {
		dest = defaultSyncDestForNode(target)
	}
	return runRsyncToNode(src, target, dest, opts.Delete, normalizeExcludes(opts.Excludes))
}

func syncCodeAll(opts SyncCodeOptions, src string) error {
	failed := 0
	for _, node := range ListMeshNodes() {
		dest := strings.TrimSpace(opts.Dest)
		if dest == "" {
			dest = defaultSyncDestForNode(node)
		}
		logs.Raw("== %s ==", node.Name)
		if err := runRsyncToNode(src, node, dest, opts.Delete, normalizeExcludes(opts.Excludes)); err != nil {
			failed++
			logs.Raw("ERROR: %v", err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("sync-code finished with %d failures", failed)
	}
	return nil
}

func normalizeExcludes(ex []string) []string {
	if len(ex) == 0 {
		return append([]string{}, defaultSyncExcludes...)
	}
	out := append([]string{}, defaultSyncExcludes...)
	for _, e := range ex {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func defaultSyncDestForNode(node MeshNode) string {
	if node.Name == "legion" {
		// Keep a dedicated Windows working copy for native tooling.
		return "/mnt/c/Users/timca/dialtone"
	}
	if len(node.RepoCandidates) > 0 {
		return node.RepoCandidates[0]
	}
	return "~/dialtone"
}

func runRsyncToNode(src string, node MeshNode, dest string, del bool, excludes []string) error {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return fmt.Errorf("dest is required for node %s", node.Name)
	}
	if node.Name == "wsl" {
		return runLocalRsync(src, dest, del, excludes)
	}
	// From WSL on Legion, sync directly to /mnt/c/... without SSH.
	if node.Name == "legion" && isWSLFunc() {
		// Local WSL -> Windows copy path (no SSH hop needed).
		return runLocalRsync(src, dest, del, excludes)
	}
	return runRemoteRsync(src, node, dest, del, excludes)
}

func runLocalRsync(src, dest string, del bool, excludes []string) error {
	if strings.HasPrefix(dest, "~") {
		home, _ := os.UserHomeDir()
		dest = filepath.Join(home, strings.TrimPrefix(dest, "~/"))
	}
	args := []string{"-az"}
	if del {
		args = append(args, "--delete")
	}
	args = append(args, "--filter=:- .gitignore")
	for _, e := range excludes {
		args = append(args, "--exclude", e)
	}
	args = append(args, src+"/", dest+"/")
	logs.Raw("rsync local: %s -> %s", src, dest)
	cmd := exec.Command("rsync", args...)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		logs.Raw("%s", strings.TrimSpace(string(out)))
	}
	if err != nil {
		return fmt.Errorf("local rsync failed: %w", err)
	}
	return nil
}

func runRemoteRsync(src string, node MeshNode, dest string, del bool, excludes []string) error {
	args := []string{"-az"}
	if del {
		args = append(args, "--delete")
	}
	args = append(args, "--filter=:- .gitignore")
	for _, e := range excludes {
		args = append(args, "--exclude", e)
	}
	sshTarget := fmt.Sprintf("%s@%s", node.User, node.Host)
	sshCmd := "ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new"
	if strings.TrimSpace(node.Port) != "" && node.Port != "22" {
		sshCmd = fmt.Sprintf("%s -p %s", sshCmd, node.Port)
	}
	args = append(args, "-e", sshCmd, src+"/", fmt.Sprintf("%s:%s/", sshTarget, dest))
	logs.Raw("rsync remote: %s -> %s:%s", src, sshTarget, dest)
	cmd := exec.Command("rsync", args...)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		logs.Raw("%s", strings.TrimSpace(string(out)))
	}
	if err != nil {
		return fmt.Errorf("remote rsync failed for %s: %w", node.Name, err)
	}
	return nil
}
