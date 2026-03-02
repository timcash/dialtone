package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

type modEntry struct {
	Name string
	Path string
}

type meshNode struct {
	Name           string
	Aliases        []string
	User           string
	Host           string
	Port           string
	OS             string
	RepoCandidates []string
}

var meshNodes = []meshNode{
	{Name: "wsl", Aliases: []string{"wsl", "legion-wsl-1", "legion-wsl-1.shad-artichoke.ts.net"}, User: "user", Host: "192.168.4.52", Port: "22", OS: "linux", RepoCandidates: []string{"/home/user/dialtone"}},
	{Name: "gold", Aliases: []string{"gold", "gold.shad-artichoke.ts.net"}, User: "user", Host: "192.168.4.53", Port: "22", OS: "macos", RepoCandidates: []string{"/Users/user/dialtone", "/Users/user/Documents/dialtone"}},
	{Name: "darkmac", Aliases: []string{"darkmac", "darkmac.shad-artichoke.ts.net"}, User: "tim", Host: "192.168.4.31", Port: "22", OS: "macos", RepoCandidates: []string{"/Users/tim/dialtone", "/Users/tim/Documents/dialtone"}},
	{Name: "rover", Aliases: []string{"rover", "rover-1", "rover-1.shad-artichoke.ts.net"}, User: "tim", Host: "192.168.4.36", Port: "22", OS: "linux", RepoCandidates: []string{"/home/tim/dialtone", "/home/user/dialtone"}},
	{Name: "legion", Aliases: []string{"legion", "legion.shad-artichoke.ts.net"}, User: "timca", Host: "192.168.4.52", Port: "2223", OS: "windows", RepoCandidates: []string{"/home/user/dialtone", "/mnt/c/Users/timca/dialtone", "/mnt/c/Users/timca/code3/dialtone"}},
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	cmd := strings.TrimSpace(os.Args[1])
	args := os.Args[2:]
	switch cmd {
	case "help", "-h", "--help":
		printUsage()
	case "bootstrap":
		exitIfErr(runBootstrap(args))
	case "add":
		exitIfErr(runAdd(args))
	case "clone":
		exitIfErr(runClone(args))
	case "list":
		exitIfErr(runList(args))
	case "status":
		exitIfErr(runStatus(args))
	case "sync":
		exitIfErr(runSync(args))
	case "sync-ui":
		exitIfErr(runSyncUI(args))
	case "gh-create":
		exitIfErr(runGitHubCreate(args))
	case "commit":
		exitIfErr(runCommit(args))
	case "push":
		exitIfErr(runPush(args))
	default:
		printUsage()
		exitIfErr(fmt.Errorf("unknown mods command: %s", cmd))
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh mods <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  bootstrap [dev|binary]")
	fmt.Println("  add <mod-name> [--repo <url|owner/repo|path>] [--owner <owner>] [--repo-name <name>]")
	fmt.Println("      [--path src/mods/<name>] [--branch main] [--public|--private] [--dry-run]")
	fmt.Println("  clone [--host <name|all|local>] [--from wsl] [--source PATH] [--dest PATH]")
	fmt.Println("      [--branch BRANCH] [--branch-map host=branch] [--skip-self=true|false] [--dry-run]")
	fmt.Println("  list")
	fmt.Println("  status [--name <mod-name>] [--short]")
	fmt.Println("  sync [--host <name|all|local>] [--repo-dir PATH] [--mod NAME|PATH ...] [--skip-self=true|false]")
	fmt.Println("  sync-ui [--mod NAME|PATH ...] [--from PATH] [--dry-run] [--commit] [--push]")
	fmt.Println("  gh-create <mod-name> --owner <owner> [--repo-name <name>] [--private|--public]")
	fmt.Println("  commit --mod <mod-name> [--message <msg>] [--all]")
	fmt.Println("  push --mod <mod-name> [--host <name|local>] [--writer <name>] [--skip-self=true|false]")
}

func runBootstrap(args []string) error {
	mode := "dev"
	if len(args) > 0 {
		mode = strings.ToLower(strings.TrimSpace(args[0]))
	}
	switch mode {
	case "dev":
		return runDialtone("dev", "install")
	case "binary":
		fmt.Println("binary bootstrap path is reserved; use app-specific binary installers")
		return nil
	default:
		return fmt.Errorf("unknown bootstrap mode: %s", mode)
	}
}

func runAdd(args []string) error {
	if len(args) == 0 {
		return errors.New("mods add requires <name>")
	}
	name := strings.TrimSpace(args[0])
	if !isValidModName(name) {
		return fmt.Errorf("invalid mod name %q", name)
	}
	fs := flag.NewFlagSet("mods add", flag.ContinueOnError)
	repo := fs.String("repo", "", "repo URL, owner/repo, or local path")
	owner := fs.String("owner", "", "GitHub owner")
	repoName := fs.String("repo-name", "", "GitHub repo name (default: dialtone-<name>)")
	pathFlag := fs.String("path", "", "submodule destination (default: src/mods/<name>)")
	branch := fs.String("branch", "", "Optional branch")
	dryRun := fs.Bool("dry-run", false, "Print actions only")
	private := fs.Bool("private", true, "Create private repo when auto-creating")
	public := fs.Bool("public", false, "Create public repo when auto-creating")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *public {
		*private = false
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	destPath := strings.TrimSpace(*pathFlag)
	if destPath == "" {
		destPath = filepath.ToSlash(filepath.Join("src", "mods", name))
	}
	if _, err := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(destPath))); err == nil {
		return fmt.Errorf("destination already exists: %s", destPath)
	}

	repoSpec := strings.TrimSpace(*repo)
	if repoSpec == "" {
		own := strings.TrimSpace(*owner)
		if own == "" {
			own, err = inferGitHubOwner(repoRoot)
			if err != nil {
				return fmt.Errorf("cannot infer owner; pass --owner or --repo: %w", err)
			}
		}
		rn := strings.TrimSpace(*repoName)
		if rn == "" {
			rn = "dialtone-" + name
		}
		repoSpec = own + "/" + rn
		if *dryRun {
			fmt.Printf("[DRY-RUN] ensure github repo exists: %s (public=%t)\n", repoSpec, !*private)
		} else {
			_ = ensureGitHubRepo(repoSpec, !*private)
		}
	}
	remote := normalizeRepoSpec(repoSpec)
	cmd := []string{"submodule", "add"}
	if strings.TrimSpace(*branch) != "" {
		cmd = append(cmd, "-b", strings.TrimSpace(*branch))
	}
	cmd = append(cmd, remote, destPath)
	upd := []string{"submodule", "update", "--init", "--recursive", "--", destPath}
	if *dryRun {
		fmt.Printf("[DRY-RUN] git -C %s %s\n", repoRoot, shellJoin(cmd))
		fmt.Printf("[DRY-RUN] git -C %s %s\n", repoRoot, shellJoin(upd))
		return nil
	}
	if err := addSubmoduleWithCLI(repoRoot, cmd...); err != nil {
		return err
	}
	return addSubmoduleWithCLI(repoRoot, upd...)
}

func runClone(args []string) error {
	fs := flag.NewFlagSet("mods clone", flag.ContinueOnError)
	host := fs.String("host", "all", "target host name|all|local")
	from := fs.String("from", "wsl", "source mesh node")
	source := fs.String("source", "", "source repo path on source node")
	dest := fs.String("dest", "", "destination repo path on target node")
	branch := fs.String("branch", "", "default branch")
	skipSelf := fs.Bool("skip-self", true, "skip self when --host all")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	var branchMapVals multiValueFlag
	fs.Var(&branchMapVals, "branch-map", "host=branch (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srcNode, err := resolveMeshNode(strings.TrimSpace(*from))
	if err != nil {
		return err
	}
	srcPath := strings.TrimSpace(*source)
	if srcPath == "" {
		srcPath = defaultRepoDirForNode(srcNode)
	}
	if srcPath == "" {
		return fmt.Errorf("source path is empty for %s", srcNode.Name)
	}
	branchMap, err := parseBranchMap(branchMapVals.values)
	if err != nil {
		return err
	}
	target := strings.ToLower(strings.TrimSpace(*host))
	if target == "" {
		target = "all"
	}

	runForNode := func(node meshNode) error {
		if *skipSelf && target == "all" && isSelfMeshNode(node) {
			fmt.Printf("== %s ==\nSKIP self node\n", node.Name)
			return nil
		}
		dst := strings.TrimSpace(*dest)
		if dst == "" {
			dst = defaultRepoDirForNode(node)
		}
		if dst == "" {
			return fmt.Errorf("cannot resolve dest path for %s", node.Name)
		}
		nodeBranch := pickBranch(node.Name, strings.TrimSpace(*branch), branchMap)
		srcSpec := sourceURLForRemote(srcNode, srcPath)
		if strings.EqualFold(node.Name, srcNode.Name) {
			srcSpec = srcPath
		}
		cmd := buildCloneUpdateCommand(srcSpec, dst, nodeBranch)
		fmt.Printf("== %s ==\n", node.Name)
		if *dryRun {
			fmt.Printf("[DRY-RUN] %s\n", cmd)
			return nil
		}
		if strings.EqualFold(node.Name, "local") || strings.EqualFold(node.Name, "self") {
			return runCommand("bash", "-lc", cmd)
		}
		out, err := runSSH(node, cmd)
		if strings.TrimSpace(out) != "" {
			fmt.Print(strings.TrimRight(out, "\n"))
			fmt.Println()
		}
		return err
	}

	if target == "local" {
		local := meshNode{Name: "local", User: os.Getenv("USER"), Host: "127.0.0.1", Port: "22", OS: "linux", RepoCandidates: []string{"./"}}
		return runForNode(local)
	}
	if target == "all" {
		nodes := listMeshNodes()
		failed := 0
		for _, n := range nodes {
			if err := runForNode(n); err != nil {
				failed++
				fmt.Printf("ERROR %s: %v\n", n.Name, err)
			}
		}
		if failed > 0 {
			return fmt.Errorf("clone finished with %d host failures", failed)
		}
		return nil
	}
	node, err := resolveMeshNode(target)
	if err != nil {
		return err
	}
	return runForNode(node)
}

func runList(args []string) error {
	fs := flag.NewFlagSet("mods list", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	mods, err := discoverMods(repoRoot)
	if err != nil {
		return err
	}
	for _, m := range mods {
		fmt.Printf("%s\t%s\n", m.Name, m.Path)
	}
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("mods status", flag.ContinueOnError)
	name := fs.String("name", "", "optional mod name")
	short := fs.Bool("short", false, "short output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	mods, err := discoverMods(repoRoot)
	if err != nil {
		return err
	}
	filters := []string{}
	if strings.TrimSpace(*name) != "" {
		filters = append(filters, strings.TrimSpace(*name))
	}
	paths, err := selectModPaths(mods, filters)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		fmt.Println("No mods selected")
		return nil
	}
	cmd := []string{"git", "-C", repoRoot, "submodule", "status", "--recursive", "--"}
	cmd = append(cmd, paths...)
	if err := runCommand(cmd...); err != nil {
		return err
	}
	if !*short {
		return runCommand("git", "-C", repoRoot, "status", "--short", ".gitmodules")
	}
	return nil
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("mods sync", flag.ContinueOnError)
	host := fs.String("host", "all", "target host: local|name|all")
	repoDir := fs.String("repo-dir", "", "remote repo dir override")
	skipSelf := fs.Bool("skip-self", true, "skip self when host=all")
	var modFilter multiValueFlag
	fs.Var(&modFilter, "mod", "mod name or path (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	mods, err := discoverMods(repoRoot)
	if err != nil {
		return err
	}
	paths, err := selectModPaths(mods, modFilter.values)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("no mods selected")
	}
	doLocal := func(root string) error {
		syncCmd := append([]string{"git", "-C", root, "submodule", "sync", "--recursive", "--"}, paths...)
		updCmd := append([]string{"git", "-C", root, "submodule", "update", "--init", "--recursive", "--"}, paths...)
		if err := runCommand(syncCmd...); err != nil {
			return err
		}
		return runCommand(updCmd...)
	}
	target := strings.ToLower(strings.TrimSpace(*host))
	if target == "" || target == "local" {
		return doLocal(repoRoot)
	}
	if target == "all" {
		failed := 0
		for _, node := range listMeshNodes() {
			if *skipSelf && isSelfMeshNode(node) {
				fmt.Printf("== %s ==\nSKIP self node\n", node.Name)
				continue
			}
			rd := strings.TrimSpace(*repoDir)
			if rd == "" {
				rd = defaultRepoDirForNode(node)
			}
			cmd := buildRemoteSubmoduleSync(rd, paths)
			fmt.Printf("== %s ==\n", node.Name)
			out, err := runSSH(node, cmd)
			if strings.TrimSpace(out) != "" {
				fmt.Print(strings.TrimRight(out, "\n"))
				fmt.Println()
			}
			if err != nil {
				failed++
				fmt.Printf("ERROR: %v\n", err)
			}
		}
		if failed > 0 {
			return fmt.Errorf("sync finished with %d host failures", failed)
		}
		return nil
	}
	node, err := resolveMeshNode(target)
	if err != nil {
		return err
	}
	rd := strings.TrimSpace(*repoDir)
	if rd == "" {
		rd = defaultRepoDirForNode(node)
	}
	out, err := runSSH(node, buildRemoteSubmoduleSync(rd, paths))
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	return err
}

func runSyncUI(args []string) error {
	fs := flag.NewFlagSet("mods sync-ui", flag.ContinueOnError)
	from := fs.String("from", "", "UI template source path (default: src/plugins/ui/src_v1/ui)")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	commit := fs.Bool("commit", false, "commit UI updates in each mod")
	push := fs.Bool("push", false, "push after commit")
	msg := fs.String("message", "Sync UI template from ui plugin", "commit message")
	var modFilter multiValueFlag
	fs.Var(&modFilter, "mod", "mod name or path (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *push {
		*commit = true
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	source := resolveUISourcePath(repoRoot, strings.TrimSpace(*from))
	if !fileExists(source) {
		return fmt.Errorf("ui source path missing: %s", source)
	}
	mods, err := discoverMods(repoRoot)
	if err != nil {
		return err
	}
	paths, err := selectModPaths(mods, modFilter.values)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("no mods selected")
	}
	for _, p := range paths {
		abs := filepath.Join(repoRoot, filepath.FromSlash(p))
		targetUI := filepath.Join(abs, "v1", "ui")
		fmt.Printf("== %s ==\n", p)
		if *dryRun {
			fmt.Printf("[DRY-RUN] copy %s -> %s\n", source, targetUI)
			continue
		}
		if err := os.RemoveAll(targetUI); err != nil {
			return err
		}
		if err := copyDir(source, targetUI); err != nil {
			return err
		}
		if *commit {
			if err := runCommand("git", "-C", abs, "add", "-A", "v1/ui"); err != nil {
				return err
			}
			if err := runCommand("git", "-C", abs, "commit", "-m", strings.TrimSpace(*msg)); err != nil && !strings.Contains(strings.ToLower(err.Error()), "nothing to commit") {
				return err
			}
			if *push {
				if err := pushModRepo(abs); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runGitHubCreate(args []string) error {
	if len(args) == 0 {
		return errors.New("mods gh-create requires <name>")
	}
	name := strings.TrimSpace(args[0])
	if !isValidModName(name) {
		return fmt.Errorf("invalid mod name %q", name)
	}
	fs := flag.NewFlagSet("mods gh-create", flag.ContinueOnError)
	owner := fs.String("owner", "", "GitHub owner")
	repoName := fs.String("repo-name", "", "repo name (default: dialtone-<name>)")
	private := fs.Bool("private", true, "create private repo")
	public := fs.Bool("public", false, "create public repo")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*owner) == "" {
		return errors.New("--owner is required")
	}
	rn := strings.TrimSpace(*repoName)
	if rn == "" {
		rn = "dialtone-" + name
	}
	repo := strings.TrimSpace(*owner) + "/" + rn
	return ensureGitHubRepo(repo, *public || !*private)
}

func runCommit(args []string) error {
	fs := flag.NewFlagSet("mods commit", flag.ContinueOnError)
	modName := fs.String("mod", "", "mod name")
	msg := fs.String("message", "", "commit message")
	fs.StringVar(msg, "m", "", "commit message")
	all := fs.Bool("all", true, "stage all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := strings.TrimSpace(*modName)
	if name == "" {
		return errors.New("--mod is required")
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	modPath := filepath.Join(repoRoot, "src", "mods", name)
	if !fileExists(modPath) {
		return fmt.Errorf("mod path missing: %s", modPath)
	}
	if *all {
		if err := runCommand("git", "-C", modPath, "add", "-A"); err != nil {
			return err
		}
	}
	m := strings.TrimSpace(*msg)
	if m == "" {
		m = "Update mod " + name
	}
	return runCommand("git", "-C", modPath, "commit", "-m", m)
}

func runPush(args []string) error {
	fs := flag.NewFlagSet("mods push", flag.ContinueOnError)
	modName := fs.String("mod", "", "mod name")
	host := fs.String("host", "local", "local|mesh node")
	writer := fs.String("writer", "", "single writer override")
	skipSelf := fs.Bool("skip-self", true, "skip self if remote resolves to self")
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := strings.TrimSpace(*modName)
	if name == "" {
		return errors.New("--mod is required")
	}
	target := strings.ToLower(strings.TrimSpace(*host))
	if strings.TrimSpace(*writer) != "" {
		target = strings.ToLower(strings.TrimSpace(*writer))
	}
	if target == "all" {
		return errors.New("refusing push on --host all; use --writer <host>")
	}
	if target == "" || target == "local" {
		repoRoot, err := findRepoRoot()
		if err != nil {
			return err
		}
		return pushModRepo(filepath.Join(repoRoot, "src", "mods", name))
	}
	node, err := resolveMeshNode(target)
	if err != nil {
		return err
	}
	if *skipSelf && isSelfMeshNode(node) {
		fmt.Printf("SKIP self node: %s\n", node.Name)
		return nil
	}
	remoteModPath := filepath.ToSlash(filepath.Join(defaultRepoDirForNode(node), "src", "mods", name))
	cmd := "set -e && cd " + shellQuote(remoteModPath) + " && " +
		"b=$(git branch --show-current) && [ -n \"$b\" ] && " +
		"(git rev-parse --abbrev-ref --symbolic-full-name @{u} >/dev/null 2>&1 || git push -u origin \"$b\") && git push"
	out, err := runSSH(node, cmd)
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	return err
}

func discoverMods(repoRoot string) ([]modEntry, error) {
	gm := filepath.Join(repoRoot, ".gitmodules")
	if !fileExists(gm) {
		return nil, nil
	}
	data, err := os.ReadFile(gm)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	mods := []modEntry{}
	var section string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if !strings.HasPrefix(section, `submodule "`) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key != "path" {
			continue
		}
		p := filepath.ToSlash(val)
		if !strings.HasPrefix(p, "src/mods/") {
			continue
		}
		rest := strings.TrimPrefix(p, "src/mods/")
		if rest == "" || strings.Contains(rest, "/") {
			continue
		}
		mods = append(mods, modEntry{Name: rest, Path: p})
	}
	sort.SliceStable(mods, func(i, j int) bool { return mods[i].Path < mods[j].Path })
	return mods, nil
}

func selectModPaths(mods []modEntry, filters []string) ([]string, error) {
	if len(filters) == 0 {
		out := make([]string, 0, len(mods))
		for _, m := range mods {
			out = append(out, m.Path)
		}
		return out, nil
	}
	byName := map[string]string{}
	byPath := map[string]string{}
	for _, m := range mods {
		byName[strings.ToLower(m.Name)] = m.Path
		byPath[m.Path] = m.Path
	}
	out := []string{}
	seen := map[string]struct{}{}
	for _, f := range filters {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		key := strings.ToLower(f)
		if p, ok := byName[key]; ok {
			if _, ex := seen[p]; !ex {
				out = append(out, p)
				seen[p] = struct{}{}
			}
			continue
		}
		fp := filepath.ToSlash(f)
		if p, ok := byPath[fp]; ok {
			if _, ex := seen[p]; !ex {
				out = append(out, p)
				seen[p] = struct{}{}
			}
			continue
		}
		return nil, fmt.Errorf("unknown mod filter %q", f)
	}
	return out, nil
}

func findRepoRoot() (string, error) {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); v != "" && fileExists(filepath.Join(v, "dialtone.sh")) {
		return v, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := cwd
	for {
		if fileExists(filepath.Join(cur, "dialtone.sh")) {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("could not find repo root from %s", cwd)
		}
		cur = parent
	}
}

func listMeshNodes() []meshNode {
	out := make([]meshNode, len(meshNodes))
	copy(out, meshNodes)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func resolveMeshNode(target string) (meshNode, error) {
	t := normalizeHost(target)
	if t == "" {
		return meshNode{}, errors.New("mesh target is required")
	}
	for _, n := range meshNodes {
		if normalizeHost(n.Name) == t {
			return n, nil
		}
		for _, a := range n.Aliases {
			if normalizeHost(a) == t {
				return n, nil
			}
		}
	}
	return meshNode{}, fmt.Errorf("unknown mesh node %q", target)
}

func runSSH(node meshNode, command string) (string, error) {
	port := strings.TrimSpace(node.Port)
	if port == "" {
		port = "22"
	}
	user := strings.TrimSpace(node.User)
	if user == "" {
		user = "git"
	}
	host := strings.TrimSpace(node.Host)
	if host == "" {
		return "", fmt.Errorf("mesh node %s has empty host", node.Name)
	}
	authMethods := sshAuthMethods(user)
	if len(authMethods) == 0 {
		return "", errors.New("no SSH auth methods available (agent/key)")
	}
	hostCB := ssh.InsecureIgnoreHostKey()
	if home, err := os.UserHomeDir(); err == nil {
		kh := filepath.Join(home, ".ssh", "known_hosts")
		if cb, err := knownhosts.New(kh); err == nil {
			hostCB = cb
		}
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostCB,
		Timeout:         8 * time.Second,
	}
	client, err := ssh.Dial("tcp", host+":"+port, cfg)
	if err != nil {
		return "", err
	}
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	var out bytes.Buffer
	sess.Stdout = &out
	sess.Stderr = &out
	err = sess.Run(command)
	return out.String(), err
}

func defaultRepoDirForNode(node meshNode) string {
	if len(node.RepoCandidates) > 0 {
		return node.RepoCandidates[0]
	}
	if strings.EqualFold(node.OS, "macos") || strings.EqualFold(node.OS, "darwin") {
		return filepath.ToSlash(filepath.Join("/Users", node.User, "dialtone"))
	}
	return filepath.ToSlash(filepath.Join("/home", node.User, "dialtone"))
}

func sourceURLForRemote(node meshNode, srcPath string) string {
	srcPath = strings.TrimSpace(srcPath)
	if strings.HasPrefix(srcPath, "~/") {
		base := "/home/" + strings.TrimSpace(node.User)
		if strings.EqualFold(node.OS, "macos") || strings.EqualFold(node.OS, "darwin") {
			base = "/Users/" + strings.TrimSpace(node.User)
		}
		srcPath = filepath.ToSlash(filepath.Join(base, strings.TrimPrefix(srcPath, "~/")))
	}
	if !strings.HasPrefix(srcPath, "/") {
		srcPath = "/" + srcPath
	}
	host := strings.TrimSpace(node.Host)
	user := strings.TrimSpace(node.User)
	port := strings.TrimSpace(node.Port)
	if host == "" || user == "" {
		return srcPath
	}
	if port == "" || port == "22" {
		return fmt.Sprintf("ssh://%s@%s%s", user, host, srcPath)
	}
	return fmt.Sprintf("ssh://%s@%s:%s%s", user, host, port, srcPath)
}

func buildCloneUpdateCommand(sourceSpec, destPath, branch string) string {
	b := strings.TrimSpace(branch)
	cloneFlags := ""
	fetch := "git -C " + shellQuote(destPath) + " fetch --all --prune"
	checkout := "git -C " + shellQuote(destPath) + " checkout $(git -C " + shellQuote(destPath) + " rev-parse --abbrev-ref HEAD)"
	pull := "git -C " + shellQuote(destPath) + " pull --ff-only"
	if b != "" {
		cloneFlags = "--branch " + shellQuote(b) + " --single-branch "
		fetch = "git -C " + shellQuote(destPath) + " fetch origin " + shellQuote(b)
		checkout = "git -C " + shellQuote(destPath) + " checkout " + shellQuote(b)
		pull = "git -C " + shellQuote(destPath) + " pull --ff-only origin " + shellQuote(b)
	}
	lines := []string{
		"set -e",
		"if [ -d " + shellQuote(filepath.ToSlash(filepath.Join(destPath, ".git"))) + " ]; then",
		"  " + fetch,
		"  " + checkout,
		"  " + pull,
		"else",
		"  mkdir -p " + shellQuote(filepath.ToSlash(filepath.Dir(destPath))),
		"  git clone " + cloneFlags + shellQuote(sourceSpec) + " " + shellQuote(destPath),
		"fi",
	}
	return strings.Join(lines, " ; ")
}

func buildRemoteSubmoduleSync(repoDir string, modPaths []string) string {
	quoted := make([]string, 0, len(modPaths))
	for _, p := range modPaths {
		quoted = append(quoted, shellQuote(p))
	}
	pathArgs := strings.Join(quoted, " ")
	lines := []string{
		"set -e",
		"cd " + shellQuote(repoDir),
		"git submodule sync --recursive -- " + pathArgs,
		"git submodule update --init --recursive -- " + pathArgs,
	}
	return strings.Join(lines, " && ")
}

func parseBranchMap(values []string) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range values {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --branch-map value %q", s)
		}
		host := normalizeHost(parts[0])
		branch := strings.TrimSpace(parts[1])
		if host == "" || branch == "" {
			return nil, fmt.Errorf("invalid --branch-map value %q", s)
		}
		out[host] = branch
	}
	return out, nil
}

func pickBranch(nodeName, defaultBranch string, branchMap map[string]string) string {
	if b, ok := branchMap[normalizeHost(nodeName)]; ok {
		return b
	}
	return defaultBranch
}

func isSelfMeshNode(node meshNode) bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" && strings.EqualFold(node.Name, "wsl") {
		return true
	}
	hn, err := os.Hostname()
	if err != nil {
		return false
	}
	local := normalizeHost(hn)
	if local == "" {
		return false
	}
	candidates := []string{node.Name}
	candidates = append(candidates, node.Aliases...)
	for _, c := range candidates {
		n := normalizeHost(c)
		if n == local || strings.Split(n, ".")[0] == strings.Split(local, ".")[0] {
			return true
		}
	}
	return false
}

func resolveUISourcePath(repoRoot, from string) string {
	p := strings.TrimSpace(from)
	if p == "" {
		return filepath.Join(repoRoot, "src", "plugins", "ui", "src_v1", "ui")
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(repoRoot, filepath.FromSlash(p))
}

func copyDir(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func ensureGitHubRepo(ownerRepo string, public bool) error {
	parts := strings.Split(strings.TrimSpace(ownerRepo), "/")
	if len(parts) != 2 {
		return fmt.Errorf("owner/repo required, got %q", ownerRepo)
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	token := strings.TrimSpace(firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")))
	if token == "" {
		return errors.New("GH_TOKEN or GITHUB_TOKEN is required for GitHub repo creation")
	}

	if _, status, err := githubAPI("GET", "/repos/"+owner+"/"+repo, token, nil); err == nil && status == http.StatusOK {
		return nil
	}

	// Try org repo creation first, then fallback to user repo creation.
	body := map[string]any{
		"name":    repo,
		"private": !public,
	}
	if _, status, err := githubAPI("POST", "/orgs/"+owner+"/repos", token, body); err == nil && (status == http.StatusCreated || status == http.StatusOK) {
		return nil
	}
	if _, status, err := githubAPI("POST", "/user/repos", token, body); err == nil && (status == http.StatusCreated || status == http.StatusOK) {
		return nil
	}
	return fmt.Errorf("failed to create repo %s (check owner/token scope)", ownerRepo)
}

func inferGitHubOwner(repoRoot string) (string, error) {
	cfgPath := filepath.Join(repoRoot, ".git", "config")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	inOrigin := false
	remoteURL := ""
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inOrigin = t == `[remote "origin"]`
			continue
		}
		if !inOrigin {
			continue
		}
		parts := strings.SplitN(t, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[0]) == "url" {
			remoteURL = strings.TrimSpace(parts[1])
			break
		}
	}
	u := strings.TrimSuffix(strings.TrimSpace(remoteURL), ".git")
	if strings.Contains(u, "github.com/") {
		tail := strings.Split(u, "github.com/")[1]
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	if strings.Contains(u, "github.com:") {
		tail := strings.Split(u, "github.com:")[1]
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[0]), nil
		}
	}
	return "", fmt.Errorf("unsupported origin URL: %s", u)
}

func normalizeRepoSpec(v string) string {
	v = strings.TrimSpace(v)
	if strings.Contains(v, "://") || strings.HasPrefix(v, "git@") || strings.HasPrefix(v, "/") || strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") {
		return v
	}
	if strings.Count(v, "/") == 1 {
		return "https://github.com/" + v + ".git"
	}
	return v
}

func pushModRepo(path string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	head, err := repo.Head()
	if err != nil {
		return err
	}
	branch := head.Name().Short()
	if branch == "" {
		return fmt.Errorf("cannot push: detached HEAD in %s", path)
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}
	remoteURL := ""
	if cfg := remote.Config(); cfg != nil && len(cfg.URLs) > 0 {
		remoteURL = cfg.URLs[0]
	}
	auth := gitAuthForURL(remoteURL)
	refspec := gitcfg.RefSpec("refs/heads/" + branch + ":refs/heads/" + branch)
	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		Auth:       auth,
		RefSpecs:   []gitcfg.RefSpec{refspec},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}
	return nil
}

func isValidModName(v string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	return re.MatchString(strings.TrimSpace(v))
}

func runDialtone(command string, args ...string) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	script := filepath.Join(repoRoot, "dialtone.sh")
	all := append([]string{command}, args...)
	cmd := exec.Command(script, all...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runCommand(args ...string) error {
	if len(args) == 0 {
		return nil
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runCapture(args ...string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	cmd := exec.Command(args[0], args[1:]...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg != "" {
			return stdout.String(), fmt.Errorf("%w: %s", err, msg)
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func normalizeHost(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.TrimSuffix(v, ".")
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

func shellJoin(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, shellQuote(p))
	}
	return strings.Join(out, " ")
}

// addSubmoduleWithCLI is the only intentional CLI fallback in mods.
// go-git does not provide a robust equivalent to `git submodule add`.
func addSubmoduleWithCLI(repoRoot string, args ...string) error {
	all := append([]string{"git", "-C", repoRoot}, args...)
	return runCommand(all...)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func githubAPI(method, path, token string, body any) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "https://api.github.com"+path, r)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return data, resp.StatusCode, fmt.Errorf("github %s %s failed: %s", method, path, strings.TrimSpace(string(data)))
	}
	return data, resp.StatusCode, nil
}

func sshAuthMethods(user string) []ssh.AuthMethod {
	methods := []ssh.AuthMethod{}
	if sock := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK")); sock != "" {
		if conn, err := netDial("unix", sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		}
		for _, keyPath := range candidates {
			b, err := os.ReadFile(keyPath)
			if err != nil {
				continue
			}
			if signer, err := ssh.ParsePrivateKey(b); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}
	return methods
}

var netDial = func(network, address string) (io.ReadWriteCloser, error) {
	return net.Dial(network, address)
}

func gitAuthForURL(rawURL string) gittransport.AuthMethod {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	if strings.HasPrefix(u, "ssh://") || strings.HasPrefix(u, "git@") {
		endpoint, err := gittransport.NewEndpoint(rawURL)
		if err != nil {
			return nil
		}
		user := strings.TrimSpace(endpoint.User)
		if user == "" {
			user = "git"
		}
		auth, err := gitssh.NewSSHAgentAuth(user)
		if err != nil {
			return nil
		}
		if auth.HostKeyCallback == nil {
			auth.HostKeyCallback, _ = gitssh.NewKnownHostsCallback()
		}
		return auth
	}
	token := strings.TrimSpace(firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")))
	if token == "" {
		return nil
	}
	return &githttp.BasicAuth{Username: "token", Password: token}
}

type multiValueFlag struct {
	values []string
}

func (m *multiValueFlag) String() string { return strings.Join(m.values, ",") }
func (m *multiValueFlag) Set(v string) error {
	m.values = append(m.values, v)
	return nil
}

func exitIfErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
