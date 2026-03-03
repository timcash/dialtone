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
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	User           string   `json:"user"`
	Host           string   `json:"host"`
	Port           string   `json:"port"`
	OS             string   `json:"os"`
	RepoCandidates []string `json:"repo_candidates"`
}

var meshNodes []meshNode

func main() {
	if err := loadMeshConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load mesh config: %v\n", err)
	}
	cmd, args, err := parseTopLevel(os.Args[1:])
	if err != nil {
		printUsage()
		exitIfErr(err)
		return
	}
	switch cmd {
	case "help", "-h", "--help":
		printUsage()
	case "bootstrap":
		exitIfErr(runBootstrap(args))
	case "new":
		exitIfErr(runNew(args))
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
	case "setup":
		exitIfErr(runSetup(args))
	case "commit":
		exitIfErr(runCommit(args))
	case "push":
		exitIfErr(runPush(args))
	case "pull":
		exitIfErr(runPull(args))
	default:
		printUsage()
		exitIfErr(fmt.Errorf("unknown mods command: %s", cmd))
	}
}

func parseTopLevel(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, errors.New("missing mods command")
	}
	cmd := strings.TrimSpace(args[0])
	rest := args[1:]
	if strings.EqualFold(cmd, "v1") {
		if len(rest) == 0 {
			return "", nil, errors.New("missing command after v1")
		}
		cmd = strings.TrimSpace(rest[0])
		rest = rest[1:]
	} else if len(rest) > 0 && strings.EqualFold(strings.TrimSpace(rest[0]), "v1") {
		// Backward-compatible: ./dialtone.sh mods <command> v1 ...
		rest = rest[1:]
	}
	return cmd, rest, nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh mods v1 <command> [args]")
	fmt.Println("       ./dialtone.sh mods <command> [args]      # backward compatible")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  bootstrap [dev|binary]")
	fmt.Println("  new <mod-name> [--repo <url|owner/repo|path>] [--owner <owner>] [--repo-name <name>]")
	fmt.Println("      [--path src/mods/<name>] [--branch main] [--public|--private] [--dry-run]")
	fmt.Println("  add --mod <mod-name> <paths...>")
	fmt.Println("      Stage specific files inside a mod before committing")
	fmt.Println("  clone [--host <name|all|local>] [--from wsl] [--source PATH] [--dest PATH]")
	fmt.Println("      [--branch BRANCH] [--branch-map host=branch] [--skip-self=true|false] [--dry-run]")
	fmt.Println("  list")
	fmt.Println("  status [--name <mod-name>] [--short]")
	fmt.Println("  sync [--host <name|all|local>] [--repo-dir PATH] [--mod NAME|PATH ...] [--skip-self=true|false]")
	fmt.Println("  sync-ui [--mod NAME|PATH ...] [--from PATH] [--dry-run] [--commit] [--push]")
	fmt.Println("  gh-create <mod-name> --owner <owner> [--repo-name <name>] [--private|--public]")
	fmt.Println("  setup")
	fmt.Println("      Guide through setting up GitHub authentication and environment")
	fmt.Println("  commit --mod <mod-name> [--message <msg>] [--all]")

	fmt.Println("  push [--mod <mod-name>] [--message <msg>] [--dry-run]")
	fmt.Println("       Push one mod, or all dirty mods + parent submodule pointers to GitHub")
	fmt.Println("  pull [--host <name|all|local>] [--from <name>] [--branch BRANCH]")
	fmt.Println("       [--source PATH] [--dest PATH] [--repo-dir PATH] [--skip-self=true|false] [--dry-run]")
	fmt.Println("       Clone/update dialtone repo across mesh nodes and sync mod submodules")
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

func runNew(args []string) error {
	if len(args) == 0 {
		return errors.New("mods new requires <name>")
	}
	name := strings.TrimSpace(args[0])
	if !isValidModName(name) {
		return fmt.Errorf("invalid mod name %q", name)
	}
	fs := flag.NewFlagSet("mods new", flag.ContinueOnError)
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

func runAdd(args []string) error {
	fs := flag.NewFlagSet("mods add", flag.ContinueOnError)
	modName := fs.String("mod", "", "mod name (optional, defaults to parent repo)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	targetPath := repoRoot
	name := strings.TrimSpace(*modName)
	if name != "" {
		targetPath = filepath.Join(repoRoot, "src", "mods", name)
		if !fileExists(targetPath) {
			return fmt.Errorf("mod path missing: %s", targetPath)
		}
	}
	
	paths := fs.Args()
	if len(paths) == 0 {
		return errors.New("no paths provided to add")
	}
	
	addArgs := []string{"-C", targetPath, "add"}
	addArgs = append(addArgs, paths...)
	
	return runCommand(append([]string{"git"}, addArgs...)...)
}

func runCommit(args []string) error {
	fs := flag.NewFlagSet("mods commit", flag.ContinueOnError)
	modName := fs.String("mod", "", "mod name (optional, defaults to parent repo)")
	msg := fs.String("message", "", "commit message")
	fs.StringVar(msg, "m", "", "commit message")
	all := fs.Bool("all", false, "stage all changes before committing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	targetPath := repoRoot
	name := strings.TrimSpace(*modName)
	if name != "" {
		targetPath = filepath.Join(repoRoot, "src", "mods", name)
		if !fileExists(targetPath) {
			return fmt.Errorf("mod path missing: %s", targetPath)
		}
	}

	if *all {
		if err := runCommand("git", "-C", targetPath, "add", "-A"); err != nil {
			return err
		}
	}
	m := strings.TrimSpace(*msg)
	if m == "" {
		if name != "" {
			m = "Update mod " + name
		} else {
			m = "Update dialtone"
		}
	}
	return runCommand("git", "-C", targetPath, "commit", "-m", m)
}

func runPush(args []string) error {
	fs := flag.NewFlagSet("mods push", flag.ContinueOnError)
	modName := fs.String("mod", "", "mod name (optional, defaults to all mods + parent)")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	if err := fs.Parse(args); err != nil {
		return err
	}
	name := strings.TrimSpace(*modName)

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	parentOwner, _ := getRepoOwner(repoRoot)

	if name == "" {
		// Push all discovered mods first
		mods, err := discoverMods(repoRoot)
		if err != nil {
			return err
		}
		for _, m := range mods {
			modPath := filepath.Join(repoRoot, filepath.FromSlash(m.Path))

			modOwner, _ := getRepoOwner(modPath)
			if parentOwner != "" && modOwner != "" && modOwner != parentOwner {
				if *dryRun {
					fmt.Printf("[DRY-RUN] skip mod %s (owner mismatch: %s != %s)\n", m.Name, modOwner, parentOwner)
				} else {
					fmt.Printf("Skipping mod %s (external owner: %s)\n", m.Name, modOwner)
				}
				continue
			}

			if *dryRun {
				fmt.Printf("[DRY-RUN] push mod %s\n", m.Name)
				continue
			}
			if err := pushModRepo(modPath); err != nil {
				fmt.Printf("Warning: failed to push mod %s: %v\n", m.Name, err)
			} else {
				fmt.Printf("Pushed mod %s\n", m.Name)
			}
		}

		// Push parent repo
		if *dryRun {
			fmt.Printf("[DRY-RUN] push parent repo\n")
			return nil
		}
		fmt.Printf("Pushing parent repo...\n")
		return pushModRepo(repoRoot)
	}

	// Push specific mod
	modPath := filepath.Join(repoRoot, "src", "mods", name)
	if !fileExists(modPath) {
		return fmt.Errorf("mod path missing: %s", modPath)
	}
	if *dryRun {
		fmt.Printf("[DRY-RUN] push mod %s\n", name)
		return nil
	}
	return pushModRepo(modPath)
}

func runClone(args []string) error {
	fs := flag.NewFlagSet("mods clone", flag.ContinueOnError)
	host := fs.String("host", "all", "target host name|all|local")
	from := fs.String("from", "", "source mesh node (defaults to current host)")
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

	srcName := strings.TrimSpace(*from)
	if srcName == "" {
		for _, n := range meshNodes {
			if isSelfMeshNode(n) {
				srcName = n.Name
				break
			}
		}
	}
	if srcName == "" {
		srcName = "wsl" // final fallback
	}

	srcNode, err := resolveMeshNode(srcName)
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

	repoRoot, _ := findRepoRoot()
	originURL := ""
	if repoRoot != "" {
		if r, err := git.PlainOpen(repoRoot); err == nil {
			if rem, err := r.Remote("origin"); err == nil && len(rem.Config().URLs) > 0 {
				originURL = rem.Config().URLs[0]
			}
		}
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

		sources := []string{}
		// 1. Primary requested source
		primary := sourceURLForRemote(srcNode, srcPath)
		if strings.EqualFold(node.Name, srcNode.Name) {
			primary = srcPath
		}
		sources = append(sources, primary)

		// 2. Add other mesh nodes as fallbacks if this is a general pull
		if target == "all" || strings.HasSuffix(os.Args[0], "pull") {
			for _, mn := range meshNodes {
				if strings.EqualFold(mn.Name, node.Name) || strings.EqualFold(mn.Name, srcNode.Name) {
					continue
				}
				mPath := defaultRepoDirForNode(mn)
				if mPath != "" {
					sources = append(sources, sourceURLForRemote(mn, mPath))
				}
			}
		}

		// 3. GitHub origin as final fallback
		if originURL != "" {
			sources = append(sources, originURL)
		}

		cmd := buildCloneUpdateCommand(sources, dst, nodeBranch)
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
	host := fs.String("host", "local", "target host: local|name|all")
	name := fs.String("name", "", "optional mod name")
	short := fs.Bool("short", false, "short output")
	skipSelf := fs.Bool("skip-self", true, "skip self when host=all")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	target := strings.ToLower(strings.TrimSpace(*host))
	if target == "" || target == "local" || target == "self" {
		return doStatusLocal(repoRoot, *name, *short)
	}

	if target == "all" {
		failed := 0
		for _, node := range listMeshNodes() {
			if *skipSelf && isSelfMeshNode(node) {
				fmt.Printf("== %s ==\nSKIP self node\n\n", node.Name)
				continue
			}
			fmt.Printf("== %s ==\n", node.Name)
			if err := doStatusRemote(node, *name, *short); err != nil {
				failed++
				fmt.Printf("ERROR: %v\n", err)
			}
			fmt.Println()
		}
		if failed > 0 {
			return fmt.Errorf("status finished with %d host failures", failed)
		}
		return nil
	}

	node, err := resolveMeshNode(target)
	if err != nil {
		return err
	}
	return doStatusRemote(node, *name, *short)
}

func doStatusLocal(root, nameFilter string, short bool) error {
	fmt.Println("== Parent: dialtone ==")
	parentStatus, _ := runCapture("git", "-C", root, "status", "--short")
	if strings.TrimSpace(parentStatus) != "" {
		fmt.Println(strings.TrimSpace(parentStatus))
	} else {
		fmt.Println("clean")
	}
	fmt.Println()

	mods, err := discoverMods(root)
	if err != nil {
		return err
	}
	filters := []string{}
	if strings.TrimSpace(nameFilter) != "" {
		filters = append(filters, strings.TrimSpace(nameFilter))
	}
	paths, err := selectModPaths(mods, filters)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}

	for _, p := range paths {
		modName := filepath.Base(p)
		absPath := filepath.Join(root, filepath.FromSlash(p))
		fmt.Printf("== Mod: %s ==\n", modName)

		// 1. Version/Commit status
		subStatus, _ := runCapture("git", "-C", root, "submodule", "status", "--", p)
		fmt.Print(strings.TrimSpace(subStatus) + " ")

		// 2. File status
		if !short {
			modStatus, _ := runCapture("git", "-C", absPath, "status", "--short")
			if strings.TrimSpace(modStatus) != "" {
				fmt.Println("(dirty)")
				fmt.Println(strings.TrimSpace(modStatus))
			} else {
				fmt.Println("(clean)")
			}
		} else {
			dirty, _ := gitHasChanges(absPath)
			if dirty {
				fmt.Println("(dirty)")
			} else {
				fmt.Println("(clean)")
			}
		}
		fmt.Println()
	}
	return nil
}

func doStatusRemote(node meshNode, nameFilter string, short bool) error {
	repoDir := defaultRepoDirForNode(node)
	args := []string{"mods", "v1", "status"}
	if nameFilter != "" {
		args = append(args, "--name", nameFilter)
	}
	if short {
		args = append(args, "--short")
	}
	cmd := fmt.Sprintf("cd %s && if [ -x ./dialtone.sh ]; then ./dialtone.sh %s; else echo 'dialtone.sh not found'; fi",
		shellQuote(repoDir), strings.Join(args, " "))

	out, err := runSSH(node, cmd)
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	return err
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("mods sync", flag.ContinueOnError)
	host := fs.String("host", "all", "target host: local|name|all")
	from := fs.String("from", "", "source mesh node for fallbacks (defaults to current host)")
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

	srcName := strings.TrimSpace(*from)
	if srcName == "" {
		for _, n := range meshNodes {
			if isSelfMeshNode(n) {
				srcName = n.Name
				break
			}
		}
	}
	srcNode, _ := resolveMeshNode(srcName)

	doLocal := func(root string) error {
		for _, p := range paths {
			absPath := filepath.Join(root, filepath.FromSlash(p))
			// 1. Try standard submodule update
			syncCmd := []string{"git", "-C", root, "submodule", "sync", "--recursive", "--", p}
			updCmd := []string{"git", "-C", root, "submodule", "update", "--init", "--recursive", "--", p}
			_ = runCommand(syncCmd...)
			if err := runCommand(updCmd...); err == nil {
				continue
			}

			// 2. Fallback to mesh nodes if standard update fails
			fmt.Printf("Submodule update failed for %s, trying mesh fallbacks...\n", p)
			success := false
			
			// Build list of mesh sources for this specific mod
			sources := []string{}
			if srcNode.Name != "" {
				sources = append(sources, sourceURLForRemote(srcNode, filepath.ToSlash(filepath.Join(defaultRepoDirForNode(srcNode), p))))
			}
			for _, mn := range meshNodes {
				if mn.Name == srcNode.Name {
					continue
				}
				sources = append(sources, sourceURLForRemote(mn, filepath.ToSlash(filepath.Join(defaultRepoDirForNode(mn), p))))
			}

			for _, src := range sources {
				fmt.Printf("Trying mesh source: %s\n", src)
				if !fileExists(absPath) {
					if err := runCommand("git", "clone", src, absPath); err == nil {
						success = true
						break
					}
				} else {
					if err := runCommand("git", "-C", absPath, "pull", "--ff-only", src, "main"); err == nil {
						success = true
						break
					}
				}

			}
			if !success {
				return fmt.Errorf("failed to sync mod %s from github or mesh", p)
			}
		}
		return nil
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
			cmd := buildRemoteSubmoduleSync(rd, paths, srcName)
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
	out, err := runSSH(node, buildRemoteSubmoduleSync(rd, paths, srcName))
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

func runPull(args []string) error {
	fs := flag.NewFlagSet("mods pull", flag.ContinueOnError)
	host := fs.String("host", "all", "target host: local|name|all")
	from := fs.String("from", "", "source mesh node (defaults to current host)")
	source := fs.String("source", "", "source repo path on source node")
	dest := fs.String("dest", "", "destination repo path on target node")
	branch := fs.String("branch", "", "default branch")
	repoDir := fs.String("repo-dir", "", "remote repo dir override for sync step")
	skipSelf := fs.Bool("skip-self", true, "skip self when host=all")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	var branchMapVals multiValueFlag
	fs.Var(&branchMapVals, "branch-map", "host=branch (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cloneArgs := []string{
		"--host", strings.TrimSpace(*host),
		"--from", strings.TrimSpace(*from),
		fmt.Sprintf("--skip-self=%t", *skipSelf),
	}
	if strings.TrimSpace(*source) != "" {
		cloneArgs = append(cloneArgs, "--source", strings.TrimSpace(*source))
	}
	if strings.TrimSpace(*dest) != "" {
		cloneArgs = append(cloneArgs, "--dest", strings.TrimSpace(*dest))
	}
	if strings.TrimSpace(*branch) != "" {
		cloneArgs = append(cloneArgs, "--branch", strings.TrimSpace(*branch))
	}
	for _, bm := range branchMapVals.values {
		cloneArgs = append(cloneArgs, "--branch-map", bm)
	}
	if *dryRun {
		cloneArgs = append(cloneArgs, "--dry-run")
	}

	// Use the current process name or an environment hint to tell runClone
	// that we are in a 'pull' context so it adds all mesh nodes as fallbacks.
	if err := runClone(cloneArgs); err != nil {
		return err
	}
	if *dryRun {
		fmt.Println("[DRY-RUN] would run: mods sync --host", strings.TrimSpace(*host))
		return nil
	}
	syncArgs := []string{
		"--host", strings.TrimSpace(*host),
		fmt.Sprintf("--skip-self=%t", *skipSelf),
	}
	if strings.TrimSpace(*repoDir) != "" {
		syncArgs = append(syncArgs, "--repo-dir", strings.TrimSpace(*repoDir))
	}
	return runSync(syncArgs)
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

func buildCloneUpdateCommand(sourceSpecs []string, destPath, branch string) string {
	b := strings.TrimSpace(branch)
	branchExpr := ""
	if b == "" {
		branchExpr = "$(git -C " + shellQuote(destPath) + " rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)"
	} else {
		branchExpr = shellQuote(b)
	}

	destGit := filepath.ToSlash(filepath.Join(destPath, ".git"))

	var lines []string
	lines = append(lines, "set -e")
	lines = append(lines, "success=0")

	for i, src := range sourceSpecs {
		srcQuote := shellQuote(src)
		destQuote := shellQuote(destPath)

		if i == 0 {
			lines = append(lines, "if [ -d "+shellQuote(destGit)+" ]; then")
			lines = append(lines, "  if git -C "+destQuote+" pull --ff-only "+srcQuote+" "+branchExpr+" ; then success=1; fi")
			lines = append(lines, "else")
			lines = append(lines, "  mkdir -p "+shellQuote(filepath.ToSlash(filepath.Dir(destPath))))
			lines = append(lines, "  if git clone "+srcQuote+" "+destQuote+" ; then")
			lines = append(lines, "    if [ -n "+branchExpr+" ] && [ "+branchExpr+" != \"main\" ]; then git -C "+destQuote+" checkout "+branchExpr+"; fi")
			lines = append(lines, "    success=1")
			lines = append(lines, "  fi")
			lines = append(lines, "fi")
		} else {
			lines = append(lines, "if [ $success -eq 0 ]; then")
			lines = append(lines, "  if [ -d "+shellQuote(destGit)+" ]; then")
			lines = append(lines, "    if git -C "+destQuote+" pull --ff-only "+srcQuote+" "+branchExpr+" ; then success=1; fi")
			lines = append(lines, "  else")
			lines = append(lines, "    if git clone "+srcQuote+" "+destQuote+" ; then")
			lines = append(lines, "      if [ -n "+branchExpr+" ] && [ "+branchExpr+" != \"main\" ]; then git -C "+destQuote+" checkout "+branchExpr+"; fi")
			lines = append(lines, "      success=1")
			lines = append(lines, "    fi")
			lines = append(lines, "  fi")
			lines = append(lines, "fi")
		}
	}

	lines = append(lines, "if [ $success -eq 0 ]; then echo \"All mesh/origin sources failed\"; exit 1; fi")
	return strings.Join(lines, " ; ")
}

func buildRemoteSubmoduleSync(repoDir string, modPaths []string, from string) string {
	var args []string
	for _, p := range modPaths {
		args = append(args, "--mod", p)
	}
	if from != "" {
		args = append(args, "--from", from)
	}
	modArgs := ""
	if len(args) > 0 {
		modArgs = " " + strings.Join(args, " ")
	}

	return fmt.Sprintf("cd %s && if [ -x ./dialtone.sh ]; then ./dialtone.sh mods v1 sync --host local%s; else git submodule update --init --recursive; fi",
		shellQuote(repoDir), modArgs)
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

func runSetup(args []string) error {
	fmt.Println("=== Dialtone Mods Setup ===")
	fmt.Println("This guide will help you set up GitHub authentication for automated mod provisioning.")
	fmt.Println()

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}

	token := strings.TrimSpace(firstNonEmpty(os.Getenv("GH_TOKEN"), os.Getenv("GITHUB_TOKEN")))
	if token != "" {
		fmt.Printf("✓ GitHub token found in environment.\n")
	} else {
		fmt.Println("! GitHub token NOT found.")
		fmt.Println("To create new mods automatically, you need a GitHub Personal Access Token (PAT).")
		fmt.Println("1. Go to: https://github.com/settings/tokens/new")
		fmt.Println("2. Select scopes: 'repo' (full control of private repositories).")
		fmt.Println("3. Generate and copy the token.")
		fmt.Println()
		fmt.Print("Paste your GitHub token here (or press Enter to skip): ")
		
		var inputToken string
		fmt.Scanln(&inputToken)
		inputToken = strings.TrimSpace(inputToken)
		
		if inputToken != "" {
			err := saveToEnv(repoRoot, "GITHUB_TOKEN", inputToken)
			if err != nil {
				return fmt.Errorf("failed to save token to .env: %w", err)
			}
			fmt.Println("✓ Token saved to env/.env")
			os.Setenv("GITHUB_TOKEN", inputToken)
			token = inputToken
		} else {
			fmt.Println("Skipped token setup. You can still push manually if you have SSH/Git auth set up.")
		}
	}

	owner, err := getRepoOwner(repoRoot)
	if err == nil {
		fmt.Printf("✓ Detected GitHub owner: %s\n", owner)
	} else {
		fmt.Println("! Could not detect GitHub owner from 'origin' remote.")
		fmt.Println("Make sure you have an 'origin' remote pointing to your GitHub fork.")
	}

	fmt.Println()
	fmt.Println("Setup complete! You can now use './dialtone.sh mods v1 new <name>' to create new mods.")
	return nil
}

func saveToEnv(repoRoot, key, value string) error {
	envPath := filepath.Join(repoRoot, "env", ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(envPath, []byte(fmt.Sprintf("%s=%s\n", key, value)), 0644)
		}
		return err
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0644)
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
		fmt.Println("Error: GitHub authentication token missing.")
		fmt.Println("Run './dialtone.sh mods v1 setup' to configure your token.")
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
	u, err := getRepoRemoteURL(repoRoot)
	if err != nil {
		return "", err
	}
	return parseGitHubOwner(u)
}

func getRepoRemoteURL(path string) (string, error) {
	out, err := runCapture("git", "-C", path, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func getRepoOwner(path string) (string, error) {
	u, err := getRepoRemoteURL(path)
	if err != nil {
		return "", err
	}
	return parseGitHubOwner(u)
}

func parseGitHubOwner(u string) (string, error) {
	u = strings.TrimSuffix(strings.TrimSpace(u), ".git")
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
	return runCommand("git", "-C", path, "push", "origin", "HEAD")
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

func gitHasChanges(repoPath string) (bool, error) {
	out, err := runCapture("git", "-C", repoPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func gitHasStagedChanges(repoPath string) (bool, error) {
	out, err := runCapture("git", "-C", repoPath, "diff", "--cached", "--name-only")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
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

func loadMeshConfig() error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	configPath := filepath.Join(repoRoot, "env", "mesh.json")
	if !fileExists(configPath) {
		return fmt.Errorf("mesh config missing: %s", configPath)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &meshNodes)
}
