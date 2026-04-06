package ssh

import (
	"fmt"
	"net"
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
	SkipSelf bool
}

func syncCodeSummary(node, src, dest string, del bool) string {
	parts := make([]string, 0, 4)
	if node = strings.TrimSpace(node); node != "" {
		parts = append(parts, fmt.Sprintf("host=%s", node))
	}
	if src = strings.TrimSpace(src); src != "" {
		parts = append(parts, fmt.Sprintf("src=%s", src))
	}
	if dest = strings.TrimSpace(dest); dest != "" {
		parts = append(parts, fmt.Sprintf("dest=%s", dest))
	}
	if del {
		parts = append(parts, "delete=true")
	}
	return strings.Join(parts, " ")
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
		summary := syncCodeSummary("all", src, strings.TrimSpace(opts.Dest), opts.Delete)
		replIndexInfof("ssh sync-code: syncing %s", summary)
		err := syncCodeAll(opts, src)
		if err != nil {
			replIndexInfof("ssh sync-code: failed %s: %v", summary, err)
			return err
		}
		replIndexInfof("ssh sync-code: completed %s", summary)
		return nil
	}

	target, err := ResolveMeshNode(node)
	if err != nil {
		return err
	}
	dest := strings.TrimSpace(opts.Dest)
	if dest == "" {
		dest = defaultSyncDestForNode(target)
	}
	summary := syncCodeSummary(target.Name, src, dest, opts.Delete)
	replIndexInfof("ssh sync-code: syncing %s", summary)
	if err := runRsyncToNode(src, target, dest, opts.Delete, normalizeExcludes(opts.Excludes)); err != nil {
		replIndexInfof("ssh sync-code: failed %s: %v", summary, err)
		return err
	}
	replIndexInfof("ssh sync-code: completed %s", summary)
	return nil
}

func syncCodeAll(opts SyncCodeOptions, src string) error {
	failed := 0
	skipped := 0
	selfNodes := detectSelfMeshNodes()
	for _, node := range ListMeshNodes() {
		if opts.SkipSelf && selfNodes[node.Name] {
			skipped++
			logs.Raw("== %s ==", node.Name)
			logs.Raw("SKIP self node")
			continue
		}
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
		replIndexInfof("ssh sync-code: all-nodes sync finished with %d failure(s) and %d self skip(s)", failed, skipped)
		return fmt.Errorf("sync-code finished with %d failures", failed)
	}
	if skipped > 0 {
		replIndexInfof("ssh sync-code: all-nodes sync skipped %d self node(s)", skipped)
		logs.Raw("sync-code skipped %d self node(s)", skipped)
	}
	return nil
}

func detectSelfMeshNodes() map[string]bool {
	out := map[string]bool{}
	if isWSLFunc() {
		out["wsl"] = true
		return out
	}

	localHosts := localHostNames()
	for _, node := range ListMeshNodes() {
		if hostMatchAny(node.Name, localHosts) {
			out[node.Name] = true
			continue
		}
		for _, alias := range node.Aliases {
			if hostMatchAny(alias, localHosts) {
				out[node.Name] = true
				break
			}
		}
	}
	return out
}

func localHostNames() map[string]struct{} {
	out := map[string]struct{}{}
	if h, err := os.Hostname(); err == nil {
		addHostVariants(out, h)
	}
	if ips, err := net.LookupHost("localhost"); err == nil {
		for _, h := range ips {
			addHostVariants(out, h)
		}
	}
	return out
}

func addHostVariants(out map[string]struct{}, raw string) {
	v := normalizeHost(raw)
	if v == "" {
		return
	}
	out[v] = struct{}{}
	if i := strings.IndexByte(v, '.'); i > 0 {
		out[v[:i]] = struct{}{}
	}
}

func hostMatchAny(candidate string, set map[string]struct{}) bool {
	v := normalizeHost(candidate)
	if v == "" {
		return false
	}
	if _, ok := set[v]; ok {
		return true
	}
	if i := strings.IndexByte(v, '.'); i > 0 {
		_, ok := set[v[:i]]
		return ok
	}
	return false
}

func normalizeHost(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.TrimSuffix(v, ".")
	return v
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
	host := PreferredHost(node, node.Port)
	sshTarget := fmt.Sprintf("%s@%s", node.User, host)
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
