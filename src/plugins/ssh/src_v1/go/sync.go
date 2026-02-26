package ssh

import (
	"fmt"
	"strings"
)

type RepoSyncOptions struct {
	Branch        string
	AllowDirty    bool
	NodeRepoPaths map[string]string
}

type RepoSyncResult struct {
	Node    string
	Repo    string
	Branch  string
	Skipped bool
	Output  string
	Err     error
}

func SyncReposAll(opts RepoSyncOptions) []RepoSyncResult {
	nodes := ListMeshNodes()
	results := make([]RepoSyncResult, 0, len(nodes))
	for _, node := range nodes {
		repo := resolveRepoPath(node, opts.NodeRepoPaths)
		cmd := buildRepoSyncCommand(repo, opts.Branch, opts.AllowDirty)
		out, err := RunNodeCommand(node.Name, cmd, CommandOptions{})
		skipped := false
		if strings.Contains(out, "DIALTONE_SYNC_SKIPPED_DIRTY") {
			skipped = true
		}
		results = append(results, RepoSyncResult{
			Node:    node.Name,
			Repo:    repo,
			Branch:  opts.Branch,
			Skipped: skipped,
			Output:  strings.TrimSpace(out),
			Err:     err,
		})
	}
	return results
}

func resolveRepoPath(node MeshNode, explicit map[string]string) string {
	if explicit != nil {
		if v := strings.TrimSpace(explicit[node.Name]); v != "" {
			return v
		}
		for _, a := range node.Aliases {
			if v := strings.TrimSpace(explicit[a]); v != "" {
				return v
			}
		}
	}
	if len(node.RepoCandidates) > 0 {
		return node.RepoCandidates[0]
	}
	return "/home/user/dialtone"
}

func buildRepoSyncCommand(repo string, branch string, allowDirty bool) string {
	repo = strings.TrimSpace(repo)
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	if repo == "" {
		repo = "/home/user/dialtone"
	}

	dirtyCheck := "if [ -n \"$(git status --porcelain)\" ]; then echo DIALTONE_SYNC_SKIPPED_DIRTY; exit 0; fi;"
	if allowDirty {
		dirtyCheck = ""
	}
	return fmt.Sprintf(
		"repo=%s; if [ ! -d \"$repo/.git\" ]; then echo \"DIALTONE_SYNC_MISSING_REPO:$repo\"; exit 1; fi; cd \"$repo\"; %s git fetch --all --prune; git checkout %s; git pull --ff-only origin %s; git rev-parse --abbrev-ref HEAD; git rev-parse --short HEAD",
		shellQuoteSync(repo),
		dirtyCheck,
		shellQuoteSync(branch),
		shellQuoteSync(branch),
	)
}

func shellQuoteSync(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `'\''`)
	return "'" + v + "'"
}
