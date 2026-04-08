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

	"dialtone/dev/internal/modstate"
	git "github.com/go-git/go-git/v5"
	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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
	HostCandidates []string `json:"host_candidates"`
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
	case "new":
		exitIfErr(runNew(args))
	case "probe":
		exitIfErr(runProbe(args))
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
	case "rsync":
		exitIfErr(runRsync(args))
	case "sync-ui":
		exitIfErr(runSyncUI(args))
	case "gh-create":
		exitIfErr(runGitHubCreate(args))
	case "commit":
		exitIfErr(runCommit(args))
	case "push":
		exitIfErr(runPush(args))
	case "pull":
		exitIfErr(runPull(args))
	case "clean":
		exitIfErr(runClean(args))
	case "reset":
		exitIfErr(runReset(args))
	case "db":
		exitIfErr(runDB(args))
	default:
		printUsage()
		exitIfErr(fmt.Errorf("unknown mods command: %s", cmd))
	}
}

func parseTopLevel(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, errors.New("missing mods command")
	}
	if strings.EqualFold(strings.TrimSpace(args[0]), "v1") {
		return "", nil, errors.New("version must be provided by ./src/mods.go, not mods argument")
	}
	cmd := strings.TrimSpace(args[0])
	rest := args[1:]
	return cmd, rest, nil
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod mods v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  new <mod-name> [--repo <url|owner/repo|path>] [--owner <owner>] [--repo-name <name>]")
	fmt.Println("      [--path src/mods/<name>] [--branch main] [--public|--private] [--dry-run]")
	fmt.Println("  probe [--mode success|sleep|fail|background] [--sleep-ms N] [--label TEXT] [--background-file PATH]")
	fmt.Println("      Deterministic routed probe command for validating dialtone queue/worker behavior")
	fmt.Println("  add --mod <mod-name> <paths...>")
	fmt.Println("      Stage specific files inside a mod before committing")
	fmt.Println("  clone [--host <name|all|local>] [--from wsl] [--source PATH] [--dest PATH]")
	fmt.Println("      [--branch BRANCH] [--branch-map host=branch] [--skip-self=true|false] [--dry-run]")
	fmt.Println("  list")
	fmt.Println("  status [--name <mod-name>] [--short]")
	fmt.Println("  sync [--host <name|all|local>] [--repo-dir PATH] [--mod NAME|PATH ...] [--skip-self=true|false]")
	fmt.Println("  rsync [--host local|name|all] [--all-repo] [--mod NAME|PATH ...] [--repo-dir PATH] [--skip-self=true|false] [--dry-run]")
	fmt.Println("  sync-ui [--mod NAME|PATH ...] [--from PATH] [--dry-run] [--commit] [--push]")
	fmt.Println("  gh-create <mod-name> --owner <owner> [--repo-name <name>] [--private|--public]")
	fmt.Println("  commit --mod <mod-name> [--message <msg>] [--all]")

	fmt.Println("  push [--mod <mod-name>] [--message <msg>] [--dry-run]")
	fmt.Println("       Push one mod, or all dirty mods + parent submodule pointers to GitHub")
	fmt.Println("  pull [--host <name|all|local>] [--from <name>] [--branch BRANCH]")
	fmt.Println("       [--source PATH] [--dest PATH] [--repo-dir PATH] [--skip-self=true|false] [--dry-run]")
	fmt.Println("       Clone/update dialtone repo across mesh nodes and sync mod submodules")
	fmt.Println("  clean [--host <name|all|local>] [--repo-dir PATH] [--skip-self=true|false] [--dry-run] [--force]")
	fmt.Println("       Discard local edits and hard-reset target repo(s) to origin/<current branch>, then clean submodule worktrees")
	fmt.Println("  reset [--host <name|all|local>] [--from <name>] [--branch BRANCH]")
	fmt.Println("       [--source PATH] [--dest PATH] [--repo-dir PATH] [--skip-self=true|false]")
	fmt.Println("       [--branch-map host=branch ...] [--dry-run] [--force]")
	fmt.Println("       Run `clean --force` then `pull` for the same host target")
	fmt.Println("  db <path|init|sync|graph|env|state|queue|runs|run|topo|test-plan|test-run|protocol-runs|protocol-events> [args]")
	fmt.Println("       Manage the central sqlite state database for the mod DAG, canonical command runs, transport rows, protocol runs, and TDD test execution")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ./dialtone_mod mods v1 db sync")
	fmt.Println("  ./dialtone_mod mods v1 db graph --format outline")
	fmt.Println("  ./dialtone_mod mods v1 db runs --limit 10")
	fmt.Println("  ./dialtone_mod mods v1 db run --id 42")
	fmt.Println("  ./dialtone_mod mods v1 db queue --limit 20")
	fmt.Println("  ./dialtone_mod mods v1 db test-run --name default")
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
			if err := ensureGitHubRepo(repoSpec, !*private); err != nil {
				return fmt.Errorf("ensure repo failed: %w", err)
			}
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
	msgText := strings.TrimSpace(*msg)
	if name != "" {
		targetPath = filepath.Join(repoRoot, "src", "mods", name)
		if !fileExists(targetPath) {
			return fmt.Errorf("mod path missing: %s", targetPath)
		}
	}

	if *all {
		if name != "" {
			if err := runCommand("git", "-C", targetPath, "add", "-A"); err != nil {
				return err
			}
			commitMsg := msgText
			if commitMsg == "" {
				commitMsg = "Update mod " + name
			}
			if _, err := runCommitIfChanged(targetPath, commitMsg); err != nil {
				return err
			}
			return nil
		} else {
			mods, err := discoverMods(repoRoot)
			if err != nil {
				return err
			}
			for _, mod := range mods {
				modPath := filepath.Join(repoRoot, filepath.FromSlash(mod.Path))
				if err := runCommand("git", "-C", modPath, "add", "-A"); err != nil {
					return err
				}
				modMsg := msgText
				if modMsg == "" {
					modMsg = "Update mod " + mod.Name
				}
				if _, err := runCommitIfChanged(modPath, modMsg); err != nil {
					return err
				}
			}

			if err := runCommand("git", "-C", repoRoot, "add", "-A"); err != nil {
				return err
			}
			parentMsg := msgText
			if parentMsg == "" {
				parentMsg = "Update dialtone"
			}
			_, err = runCommitIfChanged(repoRoot, parentMsg)
			return err
		}
	}
	m := msgText
	if m == "" {
		if name != "" {
			m = "Update mod " + name
		} else {
			m = "Update dialtone"
		}
	}
	_, err = runCommitIfChanged(targetPath, m)
	return err
}

func runCommitIfChanged(repoPath, message string) (bool, error) {
	changed, err := gitHasChanges(repoPath)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if err := runCommand("git", "-C", repoPath, "commit", "-m", message); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "nothing to commit") {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
	if err := pushModRepo(modPath); err != nil {
		return err
	}
	modRelPath := filepath.ToSlash(filepath.Join("src", "mods", name))
	if changed, err := hasSubmodulePointerChanges(repoRoot, modRelPath); err == nil && changed {
		fmt.Printf("Warning: parent repo has uncommitted submodule changes for %s. Run `mods v1 commit` then `mods v1 push` to sync pointer.\n", name)
	}
	return nil
}

func hasSubmodulePointerChanges(repoRoot, modRelPath string) (bool, error) {
	statusOut, err := runCapture("git", "-C", repoRoot, "status", "--short", "--", modRelPath)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(statusOut) != "" {
		return true, nil
	}
	cachedOut, err := runCapture("git", "-C", repoRoot, "diff", "--cached", "--name-only", "--", modRelPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(cachedOut) != "", nil
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
		root, _ := findRepoRoot()
		if root == "" {
			root = "./"
		}
		local := meshNode{Name: "local", User: os.Getenv("USER"), Host: "127.0.0.1", Port: "22", OS: "linux", RepoCandidates: []string{root}}
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
	cmd := fmt.Sprintf("cd %s && if [ -x ./dialtone_mod ]; then DIALTONE_USE_NIX=1 ./dialtone_mod %s; else echo \"dialtone_mod not found\"; exit 1; fi",
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
			cmd := buildRemoteSubmoduleSync(rd, paths, srcName, node.OS)
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
	out, err := runSSH(node, buildRemoteSubmoduleSync(rd, paths, srcName, node.OS))
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	return err
}

func runRsync(args []string) error {
	fs := flag.NewFlagSet("mods rsync", flag.ContinueOnError)
	host := fs.String("host", "all", "target host: local|name|all")
	allRepo := fs.Bool("all-repo", false, "sync entire repo root instead of only known mods")
	repoDir := fs.String("repo-dir", "", "destination repo dir override")
	skipSelf := fs.Bool("skip-self", true, "skip self when host=all")
	dryRun := fs.Bool("dry-run", false, "print commands only")
	var modFilter multiValueFlag
	fs.Var(&modFilter, "mod", "mod name or path (repeatable)")

	cmdArgs := args
	if len(cmdArgs) > 0 && !strings.HasPrefix(strings.TrimSpace(cmdArgs[0]), "--") {
		*host = strings.TrimSpace(cmdArgs[0])
		cmdArgs = cmdArgs[1:]
	}
	if err := fs.Parse(cmdArgs); err != nil {
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
	var paths []string
	if *allRepo {
		if len(modFilter.values) > 0 {
			return errors.New("cannot combine --all-repo with --mod")
		}
		paths = []string{"."}
	} else {
		paths, err = selectModPaths(mods, modFilter.values)
		if err != nil {
			return err
		}
	}
	if len(paths) == 0 {
		return errors.New("no mods selected")
	}

	runForNode := func(node meshNode) error {
		if strings.EqualFold(strings.TrimSpace(node.Name), "local") || strings.EqualFold(strings.TrimSpace(node.Name), "self") {
			return nil
		}
		dstBase := strings.TrimSpace(*repoDir)
		if dstBase == "" {
			dstBase = defaultRepoDirForNode(node)
		}
		if dstBase == "" {
			return fmt.Errorf("destination path unresolved for %s", node.Name)
		}
		return runRsyncToNode(node, repoRoot, dstBase, paths, *dryRun)
	}

	target := strings.ToLower(strings.TrimSpace(*host))
	if target == "" || target == "local" || target == "self" {
		return nil
	}
	if target == "all" {
		failed := 0
		for _, node := range listMeshNodes() {
			if *skipSelf && isSelfMeshNode(node) {
				fmt.Printf("== %s ==\nSKIP self node\n", node.Name)
				continue
			}
			fmt.Printf("== %s ==\n", node.Name)
			if err := runForNode(node); err != nil {
				failed++
				fmt.Printf("ERROR: %v\n", err)
			}
		}
		if failed > 0 {
			return fmt.Errorf("rsync finished with %d host failures", failed)
		}
		return nil
	}
	node, err := resolveMeshNode(target)
	if err != nil {
		return err
	}
	fmt.Printf("== %s ==\n", node.Name)
	return runForNode(node)
}

func runRsyncToNode(node meshNode, repoRoot, destinationBase string, paths []string, dryRun bool) error {
	destUser := strings.TrimSpace(node.User)
	if destUser == "" {
		destUser = strings.TrimSpace(os.Getenv("USER"))
	}
	destPort := strings.TrimSpace(node.Port)
	if destPort == "" {
		destPort = "22"
	}

	hosts := orderedMeshHosts(node.Host, node.HostCandidates)

	lastErr := error(nil)
	targetBase := filepath.Clean(destinationBase)
	for _, host := range hosts {
		if host == "" {
			continue
		}
		hostTarget := fmt.Sprintf("%s@%s", destUser, host)
		sshArgs := []string{"-F", "/dev/null", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR"}
		if destPort != "" && destPort != "22" {
			sshArgs = append(sshArgs, "-p", destPort)
		}
		sshCmd := "ssh " + strings.Join(sshArgs, " ")

		for _, p := range paths {
			relPath := filepath.ToSlash(filepath.Clean(p))
			src := filepath.ToSlash(filepath.Join(repoRoot, filepath.FromSlash(p)))
			dst := filepath.ToSlash(filepath.Join(targetBase, relPath))
			if relPath == "." {
				dst = filepath.ToSlash(filepath.Clean(targetBase))
			}
			excludeFile, err := gitIgnoreExcludeFile(repoRoot, relPath)
			if err != nil {
				return err
			}
			if excludeFile != "" {
				defer os.Remove(excludeFile)
			}
			args := []string{
				"--archive",
				"--compress",
				"--checksum",
				"--delete",
				"--prune-empty-dirs",
				"--filter=:- .gitignore",
				"--filter=:- .git/info/exclude",
				"--exclude=.git",
				"-e",
				sshCmd,
				src + "/",
				hostTarget + ":" + dst + "/",
			}
			if excludeFile != "" {
				args = append(args, "--exclude-from", excludeFile)
			}
			if dryRun {
				fmt.Printf("[DRY-RUN] rsync %s\n", strings.Join(args, " "))
				continue
			}
			rsyncArgs := append([]string{"rsync"}, args...)
			if err := runCommand(rsyncArgs...); err != nil {
				lastErr = err
				break
			}
		}
		if lastErr == nil {
			return nil
		}
		break
	}
	return lastErr
}

func gitIgnoreExcludeFile(repoRoot, relPath string) (string, error) {
	modRoot := filepath.ToSlash(filepath.Clean(filepath.Join(repoRoot, filepath.FromSlash(relPath))))
	insideSubmodule := false
	if out, err := runCapture("git", "-C", modRoot, "rev-parse", "--is-inside-work-tree"); err == nil && strings.TrimSpace(out) == "true" {
		insideSubmodule = true
	}

	var cmdArgs []string
	if insideSubmodule {
		cmdArgs = []string{"-C", modRoot, "ls-files", "-o", "-i", "-z", "--exclude-standard"}
	} else {
		cmdArgs = []string{"-C", repoRoot, "ls-files", "-o", "-i", "-z", "--exclude-standard", "--", relPath}
	}

	out, err := runCapture(append([]string{"git"}, cmdArgs...)...)
	if err != nil {
		return "", nil
	}
	out = strings.TrimSuffix(out, "\x00")
	if out == "" {
		return "", nil
	}
	lines := strings.Split(out, "\x00")
	trimmed := make([]string, 0, len(lines))
	prefix := relPath + "/"
	for _, line := range lines {
		entry := strings.TrimSpace(line)
		if entry == "" {
			continue
		}
		if !insideSubmodule && strings.HasPrefix(entry, prefix) {
			entry = strings.TrimPrefix(entry, prefix)
		}
		trimmed = append(trimmed, entry)
	}
	if len(trimmed) == 0 {
		return "", nil
	}

	tmp, err := os.CreateTemp("", "dialtone-rsync-exclude-")
	if err != nil {
		return "", err
	}
	for _, entry := range trimmed {
		_, _ = tmp.WriteString(entry + "\n")
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func runSyncUI(args []string) error {
	fs := flag.NewFlagSet("mods sync-ui", flag.ContinueOnError)
	from := fs.String("from", "", "UI template source path (required)")
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
	if strings.TrimSpace(*from) == "" {
		return errors.New("sync-ui requires --from")
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
	target := strings.TrimSpace(*host)
	syncArgs := []string{
		"--host", target,
		fmt.Sprintf("--skip-self=%t", *skipSelf),
	}
	if strings.TrimSpace(*repoDir) != "" {
		syncArgs = append(syncArgs, "--repo-dir", strings.TrimSpace(*repoDir))
	}

	if strings.ToLower(target) == "local" || strings.ToLower(target) == "self" {
		if err := runSync(syncArgs); err != nil {
			return err
		}
		return nil
	}

	if strings.ToLower(target) == "all" {
		if err := runSync(syncArgs); err != nil {
			return err
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

	root, err := findRepoRoot()
	if err != nil {
		return err
	}
	mods, err := discoverMods(root)
	if err != nil {
		return err
	}
	paths := []string{}
	for _, m := range mods {
		paths = append(paths, m.Path)
	}

	out, err := runSSH(node, buildRemoteSubmoduleSync(rd, paths, strings.TrimSpace(*from), node.OS))
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	if err != nil {
		return err
	}
	return nil
}

func runClean(args []string) error {
	fs := flag.NewFlagSet("mods clean", flag.ContinueOnError)
	host := fs.String("host", "local", "target host: local|name|all")
	repoDir := fs.String("repo-dir", "", "remote repo dir override")
	skipSelf := fs.Bool("skip-self", true, "skip self when host=all")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	force := fs.Bool("force", false, "required to allow destructive clean")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*force {
		return errors.New("mods v1 clean is destructive and requires --force")
	}

	target := strings.ToLower(strings.TrimSpace(*host))
	if target == "" || target == "local" || target == "self" {
		root := strings.TrimSpace(*repoDir)
		if root == "" {
			var err error
			root, err = findRepoRoot()
			if err != nil {
				return err
			}
		}
		return cleanRepoState(root, *dryRun)
	}
	if target == "all" {
		nodes := listMeshNodes()
		failed := 0
		for _, n := range nodes {
			if *skipSelf && isSelfMeshNode(n) {
				fmt.Printf("== %s ==\nSKIP self node\n", n.Name)
				continue
			}
			rd := strings.TrimSpace(*repoDir)
			if rd == "" {
				rd = defaultRepoDirForNode(n)
			}
			if rd == "" {
				failed++
				fmt.Printf("== %s ==\nERROR: unresolved repo path\n", n.Name)
				continue
			}
			fmt.Printf("== %s ==\n", n.Name)
			if *dryRun {
				fmt.Printf("[DRY-RUN] remote clean on %s (repo=%s)\n", n.Name, rd)
				continue
			}
			if err := runRemoteClean(n, rd); err != nil {
				failed++
				fmt.Printf("ERROR: %v\n", err)
			}
		}
		if failed > 0 {
			return fmt.Errorf("clean finished with %d host failures", failed)
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
	if rd == "" {
		return fmt.Errorf("destination path unresolved for %s", node.Name)
	}
	if *dryRun {
		fmt.Printf("[DRY-RUN] remote clean on %s (repo=%s)\n", node.Name, rd)
		return nil
	}
	return runRemoteClean(node, rd)
}

func runReset(args []string) error {
	fs := flag.NewFlagSet("mods reset", flag.ContinueOnError)
	host := fs.String("host", "all", "target host: local|name|all")
	from := fs.String("from", "", "source mesh node (defaults to current host)")
	source := fs.String("source", "", "source repo path on source node")
	dest := fs.String("dest", "", "destination repo path on target node")
	branch := fs.String("branch", "", "default branch")
	repoDir := fs.String("repo-dir", "", "remote repo dir override for sync step")
	skipSelf := fs.Bool("skip-self", true, "skip self when host=all")
	dryRun := fs.Bool("dry-run", false, "print actions only")
	force := fs.Bool("force", false, "required before destructive reset")
	var branchMapVals multiValueFlag
	fs.Var(&branchMapVals, "branch-map", "host=branch (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*force {
		return errors.New("mods v1 reset is destructive and requires --force")
	}

	cleanArgs := []string{
		"--host", strings.TrimSpace(*host),
		"--force",
		fmt.Sprintf("--skip-self=%t", *skipSelf),
	}
	if strings.TrimSpace(*repoDir) != "" {
		cleanArgs = append(cleanArgs, "--repo-dir", strings.TrimSpace(*repoDir))
	}
	if *dryRun {
		cleanArgs = append(cleanArgs, "--dry-run")
	}
	fmt.Println("== mods v1 reset: clean ==")
	if err := runClean(cleanArgs); err != nil {
		return err
	}

	pullArgs := []string{
		"--host", strings.TrimSpace(*host),
		fmt.Sprintf("--skip-self=%t", *skipSelf),
	}
	if strings.TrimSpace(*from) != "" {
		pullArgs = append(pullArgs, "--from", strings.TrimSpace(*from))
	}
	if strings.TrimSpace(*source) != "" {
		pullArgs = append(pullArgs, "--source", strings.TrimSpace(*source))
	}
	if strings.TrimSpace(*dest) != "" {
		pullArgs = append(pullArgs, "--dest", strings.TrimSpace(*dest))
	}
	if strings.TrimSpace(*branch) != "" {
		pullArgs = append(pullArgs, "--branch", strings.TrimSpace(*branch))
	}
	for _, bm := range branchMapVals.values {
		pullArgs = append(pullArgs, "--branch-map", bm)
	}
	if strings.TrimSpace(*repoDir) != "" {
		pullArgs = append(pullArgs, "--repo-dir", strings.TrimSpace(*repoDir))
	}
	if *dryRun {
		pullArgs = append(pullArgs, "--dry-run")
	}
	fmt.Println("== mods v1 reset: pull ==")
	return runPull(pullArgs)
}

func runRemoteClean(node meshNode, repoDir string) error {
	out, err := runSSH(node, buildRemoteCleanCommand(repoDir, false))
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	return err
}

func cleanRepoState(repoRoot string, dryRun bool) error {
	branch, err := runCapture("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || strings.TrimSpace(branch) == "" || strings.TrimSpace(branch) == "HEAD" {
		branch = "main"
	} else {
		branch = strings.TrimSpace(branch)
	}
	commands := [][]string{
		{"git", "-C", repoRoot, "fetch", "--all", "--prune"},
		{"git", "-C", repoRoot, "reset", "--hard", "origin/" + branch},
		{"git", "-C", repoRoot, "clean", "-fd"},
		{"git", "-C", repoRoot, "submodule", "foreach", "--recursive", "if [ -d .git ] || [ -f .git ]; then git reset --hard; git clean -fd .; fi"},
	}
	if dryRun {
		for _, cmd := range commands {
			fmt.Printf("[DRY-RUN] %s\n", shellJoin(cmd))
		}
		return nil
	}
	for _, cmd := range commands {
		if err := runCommand(cmd...); err != nil {
			return err
		}
	}
	return nil
}

func discoverMods(repoRoot string) ([]modEntry, error) {
	if mods, err := discoverModsFromState(repoRoot); err == nil && len(mods) > 0 {
		return mods, nil
	}
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

func discoverModsFromState(repoRoot string) ([]modEntry, error) {
	dbPath := strings.TrimSpace(os.Getenv("DIALTONE_STATE_DB"))
	if dbPath == "" {
		dbPath = modstate.DefaultDBPath(repoRoot)
	}
	db, err := modstate.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if _, err := modstate.SyncRepo(db, repoRoot, modstate.CaptureRuntimeEnv()); err != nil {
		return nil, err
	}
	records, err := modstate.LoadMods(db)
	if err != nil {
		return nil, err
	}
	mods := make([]modEntry, 0, len(records))
	seen := map[string]struct{}{}
	for _, record := range records {
		name := strings.TrimSpace(record.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		mods = append(mods, modEntry{
			Name: name,
			Path: filepath.ToSlash(filepath.Join("src", "mods", name)),
		})
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
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); v != "" && isRepoRoot(v) {
		return v, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := cwd
	for {
		if isRepoRoot(cur) {
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
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	dialtoneModPath := filepath.Join(repoRoot, "dialtone_mod")
	targetUser := strings.TrimSpace(node.User)
	if targetUser == "" {
		targetUser = strings.TrimSpace(os.Getenv("USER"))
	}
	remotePort := strings.TrimSpace(node.Port)
	if remotePort == "" {
		remotePort = "22"
	}

	hosts := orderedMeshHosts(node.Host, node.HostCandidates)

	var lastErr error
	for _, host := range hosts {
		if host == "" {
			continue
		}
		sshArgs := []string{"ssh", "v1", "run", "--host", host}
		if targetUser != "" {
			sshArgs = append(sshArgs, "--user", targetUser)
		}
		if remotePort != "" {
			sshArgs = append(sshArgs, "--port", remotePort)
		}
		sshArgs = append(sshArgs, "--command", command)

		goArgs := append([]string{"run", "./mods.go"}, sshArgs...)

		var cmd *exec.Cmd
		if fileExists(dialtoneModPath) {
			cmd = exec.Command(dialtoneModPath, sshArgs...)
		} else {
			cmd = exec.Command(goBin, goArgs...)
		}
		cmd.Dir = filepath.Join(repoRoot, "src")
		var out bytes.Buffer
		var errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		if err := cmd.Run(); err == nil {
			return out.String(), nil
		} else {
			msg := strings.TrimSpace(errOut.String())
			if msg == "" {
				msg = strings.TrimSpace(out.String())
			}
			if msg == "" {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("%w: %s", err, msg)
			}
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("ssh to %s failed", node.Name)
}

func orderedMeshHosts(host string, candidates []string) []string {
	all := make([]string, 0, len(candidates)+1)
	for _, c := range candidates {
		c = strings.TrimSuffix(strings.TrimSpace(c), ".")
		if c != "" {
			all = append(all, c)
		}
	}
	baseHost := strings.TrimSuffix(strings.TrimSpace(host), ".")
	if baseHost != "" {
		all = append(all, baseHost)
	}

	tailnetHosts := make([]string, 0, len(all))
	otherHosts := make([]string, 0, len(all))
	seen := map[string]struct{}{}
	out := make([]string, 0, len(all))
	for _, c := range all {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		if strings.HasSuffix(strings.ToLower(c), ".ts.net") {
			tailnetHosts = append(tailnetHosts, c)
		} else {
			otherHosts = append(otherHosts, c)
		}
	}
	out = append(out, tailnetHosts...)
	out = append(out, otherHosts...)
	return out
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
	return strings.Join(lines, "\n")
}

func buildRemoteSubmoduleSync(repoDir string, modPaths []string, from string, os string) string {
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

	return fmt.Sprintf("cd %s && if [ -x ./dialtone_mod ]; then DIALTONE_USE_NIX=1 ./dialtone_mod mods v1 sync --host local%s; else echo \"dialtone_mod not found\"; exit 1; fi",
		shellQuote(repoDir), modArgs)
}

func buildRemoteCleanCommand(repoDir string, dryRun bool) string {
	args := []string{"mods", "v1", "clean", "--host", "local", "--force"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return fmt.Sprintf("cd %s && if [ -x ./dialtone_mod ]; then DIALTONE_USE_NIX=1 ./dialtone_mod %s; else echo \"dialtone_mod not found\"; exit 1; fi",
		shellQuote(repoDir), strings.Join(args, " "))
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
		return ""
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
		fmt.Println("Error: GitHub authentication token missing.")
		fmt.Println("Please set GITHUB_TOKEN environment variable.")
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
	script := filepath.Join(repoRoot, "dialtone_mod")
	all := append([]string{command}, args...)
	cmd := exec.Command(script, all...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func isRepoRoot(candidate string) bool {
	return fileExists(filepath.Join(candidate, "dialtone_mod")) &&
		fileExists(filepath.Join(candidate, "src", "go.mod"))
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
