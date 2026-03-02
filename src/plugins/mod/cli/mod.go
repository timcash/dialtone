package cli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

const defaultVersion = "v1"

type modEntry struct {
	Name string
	Path string
}

func Run(args []string) error {
	version, command, rest, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		printUsage()
		return err
	}
	if version != defaultVersion {
		return fmt.Errorf("unsupported mod version %s (expected %s)", version, defaultVersion)
	}
	if warnedOldOrder {
		fmt.Println("[WARN] old mod CLI order is deprecated. Use: ./dialtone.sh mod v1 <command> [args]")
	}

	switch command {
	case "help", "--help", "-h":
		printUsage()
		return nil
	case "add":
		return runAdd(rest)
	case "list":
		return runList(rest)
	case "status":
		return runStatus(rest)
	case "sync":
		return runSync(rest)
	case "sync-ui":
		return runSyncUI(rest)
	case "gh-create":
		return runGitHubCreate(rest)
	default:
		printUsage()
		return fmt.Errorf("unknown mod command: %s", command)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return defaultVersion, "help", nil, false, nil
	}
	if isHelp(args[0]) {
		return defaultVersion, "help", nil, false, nil
	}
	if strings.EqualFold(strings.TrimSpace(args[0]), "v1") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh mod v1 <command> [args])")
		}
		return defaultVersion, args[1], args[2:], false, nil
	}
	if strings.EqualFold(strings.TrimSpace(args[0]), "src_v1") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh mod v1 <command> [args])")
		}
		return defaultVersion, args[1], args[2:], true, nil
	}
	if len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[1]), "v1") {
		return defaultVersion, args[0], args[2:], true, nil
	}
	if len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[1]), "src_v1") {
		return defaultVersion, args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first mod argument (usage: ./dialtone.sh mod v1 <command> [args])")
}

func isHelp(v string) bool {
	switch strings.TrimSpace(v) {
	case "help", "--help", "-h":
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh mod v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  add <name> [--repo <url|owner/repo|path>] [--owner <github-owner>] [--repo-name <name>]")
	fmt.Println("      [--path src/mods/<name>] [--branch main] [--with-ui=true|false] [--ui-from PATH] [--dry-run]")
	fmt.Println("      [--commit=true|false] [--push=true|false] [--message \"Add mod <name>\"] [--public|--private]")
	fmt.Println("      One command to create/seed GitHub repo, add submodule, and commit/push pointer")
	fmt.Println("      Default mapping: <owner>/dialtone-<name> -> src/mods/<name>")
	fmt.Println("  list")
	fmt.Println("      List plugin mods discovered from .gitmodules")
	fmt.Println("  status [--name <mod-name>] [--short]")
	fmt.Println("      Show git submodule status for plugin mods")
	fmt.Println("  sync [--host <name|all|local>] [--repo-dir PATH] [--mod NAME|PATH ...] [--skip-self=true|false] [--strict-scaffold=true|false]")
	fmt.Println("      Source of truth for mod sync: git submodule sync/update (no rsync assumptions)")
	fmt.Println("  sync-ui [--mod NAME|PATH ...] [--from PATH] [--dry-run] [--commit] [--push]")
	fmt.Println("      Copy UI template into selected mods at v1/ui (no nested submodules)")
	fmt.Println("  gh-create <name> --owner <github-owner> [--repo-name <name>] [--private|--public]")
	fmt.Println("      Create GitHub repo for a plugin mod (default: dialtone-<name>)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ./dialtone.sh mod v1 add mod-name")
	fmt.Println("  ./dialtone.sh mod v1 add mod-name --owner timcash --public")
	fmt.Println("  ./dialtone.sh mod v1 sync --host all")
	fmt.Println("  ./dialtone.sh mod v1 sync-ui --mod mod-name")
}

func runAdd(args []string) error {
	if len(args) == 0 {
		return errors.New("mod add requires <name>")
	}
	name := strings.TrimSpace(args[0])
	flagArgs := args[1:]
	if !isValidModName(name) {
		return fmt.Errorf("invalid mod name %q (allowed: a-z 0-9 - _)", name)
	}

	fs := flag.NewFlagSet("mod add", flag.ContinueOnError)
	repo := fs.String("repo", "", "Repository URL, owner/repo, or local path")
	owner := fs.String("owner", "", "GitHub owner/org used when --repo is omitted")
	repoName := fs.String("repo-name", "", "Repo name used with --owner (default: dialtone-<name>)")
	pathFlag := fs.String("path", "", "Submodule destination path (default: src/mods/<name>)")
	branch := fs.String("branch", "", "Optional branch name")
	withUI := fs.Bool("with-ui", true, "Seed default UI folder v1/ui when creating empty mod repos")
	uiFrom := fs.String("ui-from", "", "UI template source path (default: src/plugins/ui/src_v1/ui)")
	dryRun := fs.Bool("dry-run", false, "Print commands without executing")
	commit := fs.Bool("commit", true, "Stage and commit .gitmodules + submodule path after add")
	push := fs.Bool("push", true, "Push current branch after commit (implies --commit)")
	message := fs.String("message", "", "Commit message for --commit")
	private := fs.Bool("private", true, "Create GitHub repo as private if auto-creating")
	public := fs.Bool("public", false, "Create GitHub repo as public if auto-creating")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if *public {
		*private = false
	}
	if *push {
		*commit = true
	}

	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	destPath := strings.TrimSpace(*pathFlag)
	if destPath == "" {
		destPath = filepath.ToSlash(filepath.Join("src", "mods", name))
	}
	absDest := filepath.Join(repoRoot, filepath.FromSlash(destPath))
	if _, statErr := os.Stat(absDest); statErr == nil {
		return fmt.Errorf("destination already exists: %s", destPath)
	}

	uiSource := resolveUISourcePath(repoRoot, strings.TrimSpace(*uiFrom))
	resolvedRepo, err := resolveAddRepo(repoRoot, name, strings.TrimSpace(*repo), strings.TrimSpace(*owner), strings.TrimSpace(*repoName))
	if err != nil {
		return err
	}
	resolvedOwner, resolvedRepoName := splitOwnerRepo(resolvedRepo)
	if strings.TrimSpace(*repo) == "" && resolvedOwner != "" && resolvedRepoName != "" {
		if *dryRun {
			fmt.Printf("[DRY-RUN] ensure github repo exists: %s/%s (public=%t)\n", resolvedOwner, resolvedRepoName, !*private)
			fmt.Printf("[DRY-RUN] seed scaffold if repo is empty (with-ui=%t source=%s)\n", *withUI, uiSource)
		} else {
			if err := ensureGitHubRepoWithScaffold(resolvedOwner, resolvedRepoName, name, !*private, *withUI, uiSource); err != nil {
				return err
			}
		}
	}

	repoSpec := normalizeRepoSpec(resolvedRepo)
	addCmd := []string{"git", "-C", repoRoot, "submodule", "add"}
	if strings.TrimSpace(*branch) != "" {
		addCmd = append(addCmd, "-b", strings.TrimSpace(*branch))
	}
	addCmd = append(addCmd, repoSpec, destPath)
	updateCmd := []string{"git", "-C", repoRoot, "submodule", "update", "--init", "--recursive", "--", destPath}

	if *dryRun {
		fmt.Printf("[DRY-RUN] %s\n", shellJoin(addCmd))
		fmt.Printf("[DRY-RUN] %s\n", shellJoin(updateCmd))
		fmt.Printf("[DRY-RUN] git -C %s status --short .gitmodules %s\n", repoRoot, destPath)
		if *commit {
			msg := strings.TrimSpace(*message)
			if msg == "" {
				msg = fmt.Sprintf("Add mod %s", name)
			}
			fmt.Printf("[DRY-RUN] git -C %s add .gitmodules %s\n", repoRoot, destPath)
			fmt.Printf("[DRY-RUN] git -C %s commit -m %s\n", repoRoot, shellQuote(msg))
			if *push {
				branchOut, berr := runCapture("git", "-C", repoRoot, "branch", "--show-current")
				branchName := strings.TrimSpace(branchOut)
				if berr != nil || branchName == "" {
					fmt.Printf("[DRY-RUN] git -C %s push   # branch detection unavailable\n", repoRoot)
				} else {
					_, upErr := runCapture("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
					if upErr != nil {
						fmt.Printf("[DRY-RUN] git -C %s push -u origin %s\n", repoRoot, branchName)
					} else {
						fmt.Printf("[DRY-RUN] git -C %s push\n", repoRoot)
					}
				}
			}
		}
		return nil
	}

	if err := runCommand(addCmd...); err != nil {
		return err
	}
	if err := runCommand(updateCmd...); err != nil {
		return err
	}
	if err := ensureScaffoldExists(repoRoot, destPath); err != nil {
		return err
	}

	if *commit {
		msg := strings.TrimSpace(*message)
		if msg == "" {
			msg = fmt.Sprintf("Add mod %s", name)
		}
		if err := runCommand("git", "-C", repoRoot, "add", ".gitmodules", destPath); err != nil {
			return err
		}
		if err := runCommand("git", "-C", repoRoot, "commit", "-m", msg); err != nil {
			return err
		}
		if *push {
			if err := pushWithUpstream(repoRoot); err != nil {
				return err
			}
		}
	}

	fmt.Printf("mod added: %s -> %s\n", name, destPath)
	if !*commit {
		fmt.Printf("next: git -C %s add .gitmodules %s && git -C %s commit -m \"Add mod %s\"\n", repoRoot, destPath, repoRoot, name)
	}
	return nil
}

func runList(args []string) error {
	fs := flag.NewFlagSet("mod list", flag.ContinueOnError)
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
	if len(mods) == 0 {
		fmt.Println("No plugin mods found in .gitmodules")
		return nil
	}
	for _, m := range mods {
		fmt.Printf("%s\t%s\n", m.Name, m.Path)
	}
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("mod status", flag.ContinueOnError)
	name := fs.String("name", "", "Optional plugin mod name")
	short := fs.Bool("short", false, "Short status output")
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
		fmt.Println("No plugin mods selected")
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
	fs := flag.NewFlagSet("mod sync", flag.ContinueOnError)
	host := fs.String("host", "all", "Target host: local, mesh node, or all")
	repoDir := fs.String("repo-dir", "", "Remote repo dir override (default: mesh repo candidate)")
	skipSelf := fs.Bool("skip-self", true, "When --host all, skip current local mesh node")
	strictScaffold := fs.Bool("strict-scaffold", true, "Require scaffold/main.go or scaffold.sh in each mod after sync")
	var modFilter multiValueFlag
	fs.Var(&modFilter, "mod", "Mod name or path to sync (repeatable)")
	name := fs.String("name", "", "Alias for a single --mod")
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
	filters := append([]string{}, modFilter.values...)
	if strings.TrimSpace(*name) != "" {
		filters = append(filters, strings.TrimSpace(*name))
	}
	paths, err := selectModPaths(mods, filters)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no plugin mods selected (add one with: ./dialtone.sh mod v1 add <name>)")
	}

	target := strings.ToLower(strings.TrimSpace(*host))
	switch target {
	case "local":
		return syncLocalMods(repoRoot, paths, *strictScaffold)
	case "all":
		return syncAllMeshMods(repoRoot, paths, strings.TrimSpace(*repoDir), *skipSelf, *strictScaffold)
	default:
		node, err := sshv1.ResolveMeshNode(target)
		if err != nil {
			return err
		}
		return syncNodeMods(node, strings.TrimSpace(*repoDir), paths, *strictScaffold)
	}
}

func runSyncUI(args []string) error {
	fs := flag.NewFlagSet("mod sync-ui", flag.ContinueOnError)
	from := fs.String("from", "", "UI template source path (default: src/plugins/ui/src_v1/ui)")
	dryRun := fs.Bool("dry-run", false, "Print actions without changing files")
	commit := fs.Bool("commit", false, "Commit UI changes inside each mod repo")
	push := fs.Bool("push", false, "Push mod repo branch after commit (implies --commit)")
	message := fs.String("message", "Sync UI template from dialtone ui plugin", "Commit message for --commit")
	var modFilter multiValueFlag
	fs.Var(&modFilter, "mod", "Mod name or path to sync UI into (repeatable)")
	name := fs.String("name", "", "Alias for a single --mod")
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
	filters := append([]string{}, modFilter.values...)
	if strings.TrimSpace(*name) != "" {
		filters = append(filters, strings.TrimSpace(*name))
	}
	paths, err := selectModPaths(mods, filters)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no plugin mods selected")
	}

	for _, modPath := range paths {
		absMod := filepath.Join(repoRoot, filepath.FromSlash(modPath))
		targetUI := filepath.Join(absMod, "v1", "ui")
		fmt.Printf("== %s ==\n", modPath)
		if *dryRun {
			fmt.Printf("[DRY-RUN] copy %s -> %s\n", source, targetUI)
			if *commit {
				fmt.Printf("[DRY-RUN] git -C %s add -A v1/ui && git -C %s commit -m %s\n", absMod, absMod, shellQuote(strings.TrimSpace(*message)))
				if *push {
					fmt.Printf("[DRY-RUN] git -C %s push\n", absMod)
				}
			}
			continue
		}
		if err := os.RemoveAll(targetUI); err != nil {
			return err
		}
		if err := copyDir(source, targetUI); err != nil {
			return err
		}
		if *commit {
			if err := runCommand("git", "-C", absMod, "add", "-A", "v1/ui"); err != nil {
				return err
			}
			if err := runCommand("git", "-C", absMod, "commit", "-m", strings.TrimSpace(*message)); err != nil {
				// no changes is acceptable
				if !strings.Contains(strings.ToLower(err.Error()), "nothing to commit") {
					return err
				}
			}
			if *push {
				if err := pushWithUpstream(absMod); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runGitHubCreate(args []string) error {
	if len(args) == 0 {
		return errors.New("mod gh-create requires <name>")
	}
	name := strings.TrimSpace(args[0])
	if !isValidModName(name) {
		return fmt.Errorf("invalid mod name %q", name)
	}
	flagArgs := args[1:]

	fs := flag.NewFlagSet("mod gh-create", flag.ContinueOnError)
	owner := fs.String("owner", "", "GitHub owner/org")
	repoName := fs.String("repo-name", "", "GitHub repo name (default: dialtone-<name>)")
	private := fs.Bool("private", true, "Create private repo")
	public := fs.Bool("public", false, "Create public repo")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if strings.TrimSpace(*owner) == "" {
		return errors.New("mod gh-create requires --owner")
	}

	repoShort := strings.TrimSpace(*repoName)
	if repoShort == "" {
		repoShort = "dialtone-" + name
	}
	repo := strings.TrimSpace(*owner) + "/" + repoShort
	visibility := "--private"
	if *public || !*private {
		visibility = "--public"
	}

	_ = runCommand("./dialtone.sh", "github", "src_v1", "install")
	return runCommand("gh", "repo", "create", repo, visibility, "--confirm")
}

func syncLocalMods(repoRoot string, modPaths []string, strictScaffold bool) error {
	syncCmd := []string{"git", "-C", repoRoot, "submodule", "sync", "--recursive", "--"}
	syncCmd = append(syncCmd, modPaths...)
	updateCmd := []string{"git", "-C", repoRoot, "submodule", "update", "--init", "--recursive", "--"}
	updateCmd = append(updateCmd, modPaths...)

	fmt.Printf("== local ==\n")
	if err := runCommand(syncCmd...); err != nil {
		return err
	}
	if err := runCommand(updateCmd...); err != nil {
		return err
	}
	if strictScaffold {
		for _, p := range modPaths {
			if err := ensureScaffoldExists(repoRoot, p); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncAllMeshMods(repoRoot string, modPaths []string, repoDirOverride string, skipSelf bool, strictScaffold bool) error {
	nodes := sshv1.ListMeshNodes()
	failed := 0
	for _, node := range nodes {
		if skipSelf && isSelfMeshNode(node) {
			fmt.Printf("== %s ==\nSKIP self node\n", node.Name)
			continue
		}
		if err := syncNodeMods(node, repoDirOverride, modPaths, strictScaffold); err != nil {
			failed++
			fmt.Printf("== %s ==\nERROR: %v\n", node.Name, err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("mod sync finished with %d host failures", failed)
	}
	return nil
}

func syncNodeMods(node sshv1.MeshNode, repoDirOverride string, modPaths []string, strictScaffold bool) error {
	repoDir := strings.TrimSpace(repoDirOverride)
	if repoDir == "" {
		repoDir = defaultRepoDirForNode(node)
	}
	if repoDir == "" {
		return fmt.Errorf("cannot resolve repo dir for node %s", node.Name)
	}

	fmt.Printf("== %s ==\n", node.Name)
	cmd := buildRemoteSyncCommand(repoDir, modPaths, strictScaffold)
	out, err := sshv1.RunNodeCommand(node.Name, cmd, sshv1.CommandOptions{})
	if strings.TrimSpace(out) != "" {
		fmt.Print(strings.TrimRight(out, "\n"))
		fmt.Println()
	}
	return err
}

func buildRemoteSyncCommand(repoDir string, modPaths []string, strictScaffold bool) string {
	quotedPaths := make([]string, 0, len(modPaths))
	for _, p := range modPaths {
		quotedPaths = append(quotedPaths, shellQuote(p))
	}
	pathArgs := strings.Join(quotedPaths, " ")

	lines := []string{
		"set -e",
		"cd " + shellQuote(repoDir),
		"if [ ! -d .git ]; then echo \"missing git repo at " + strings.ReplaceAll(repoDir, "\"", "\\\"") + "\" >&2; exit 2; fi",
		"git submodule sync --recursive -- " + pathArgs,
		"git submodule update --init --recursive -- " + pathArgs,
	}
	if strictScaffold {
		lines = append(lines, "for p in "+pathArgs+"; do if [ ! -f \"$p/scaffold/main.go\" ] && [ ! -f \"$p/scaffold.sh\" ]; then echo \"missing plugin scaffold in $p\" >&2; exit 14; fi; done")
	}
	return strings.Join(lines, " && ")
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := cwd
	for {
		if fileExists(filepath.Join(cur, "dialtone.sh")) && fileExists(filepath.Join(cur, "src", "dev.go")) {
			return cur, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("could not find dialtone repo root from %s", cwd)
		}
		cur = parent
	}
}

func discoverMods(repoRoot string) ([]modEntry, error) {
	gitmodulesPath := filepath.Join(repoRoot, ".gitmodules")
	if !fileExists(gitmodulesPath) {
		return nil, nil
	}
	out, err := runCapture("git", "-C", repoRoot, "config", "-f", ".gitmodules", "--get-regexp", `^submodule\..*\.path$`)
	if err != nil {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	mods := make([]modEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		p := filepath.ToSlash(parts[len(parts)-1])
		if !isPluginModPath(p) {
			continue
		}
		mods = append(mods, modEntry{Name: filepath.Base(p), Path: p})
	}
	sort.SliceStable(mods, func(i, j int) bool { return mods[i].Path < mods[j].Path })
	return mods, nil
}

func isPluginModPath(p string) bool {
	p = filepath.ToSlash(strings.TrimSpace(p))
	if !strings.HasPrefix(p, "src/mods/") {
		return false
	}
	rest := strings.TrimPrefix(p, "src/mods/")
	return rest != "" && !strings.Contains(rest, "/")
}

func selectModPaths(mods []modEntry, filters []string) ([]string, error) {
	paths := make([]string, 0, len(mods))
	if len(filters) == 0 {
		for _, m := range mods {
			paths = append(paths, m.Path)
		}
		return paths, nil
	}

	byName := map[string]string{}
	byPath := map[string]string{}
	for _, m := range mods {
		byName[strings.ToLower(strings.TrimSpace(m.Name))] = m.Path
		byPath[filepath.ToSlash(strings.TrimSpace(m.Path))] = m.Path
	}
	seen := map[string]struct{}{}
	for _, f := range filters {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if p, ok := byName[strings.ToLower(f)]; ok {
			if _, exists := seen[p]; !exists {
				paths = append(paths, p)
				seen[p] = struct{}{}
			}
			continue
		}
		f = filepath.ToSlash(f)
		if p, ok := byPath[f]; ok {
			if _, exists := seen[p]; !exists {
				paths = append(paths, p)
				seen[p] = struct{}{}
			}
			continue
		}
		return nil, fmt.Errorf("unknown mod filter %q", f)
	}
	return paths, nil
}

func ensureScaffoldExists(repoRoot, modPath string) error {
	abs := filepath.Join(repoRoot, filepath.FromSlash(modPath))
	if fileExists(filepath.Join(abs, "scaffold", "main.go")) || fileExists(filepath.Join(abs, "scaffold.sh")) {
		return nil
	}
	return fmt.Errorf("missing plugin scaffold in %s (expected scaffold/main.go or scaffold.sh)", modPath)
}

func defaultRepoDirForNode(node sshv1.MeshNode) string {
	if len(node.RepoCandidates) > 0 {
		return node.RepoCandidates[0]
	}
	if strings.TrimSpace(node.User) == "" {
		return "~/dialtone"
	}
	if strings.EqualFold(node.OS, "macos") || strings.EqualFold(node.OS, "darwin") {
		return filepath.ToSlash(filepath.Join("/Users", node.User, "dialtone"))
	}
	return filepath.ToSlash(filepath.Join("/home", node.User, "dialtone"))
}

func isSelfMeshNode(node sshv1.MeshNode) bool {
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

func normalizeHost(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.TrimSuffix(v, ".")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

func pushWithUpstream(repoPath string) error {
	branchOut, err := runCapture("git", "-C", repoPath, "branch", "--show-current")
	if err != nil {
		return err
	}
	branch := strings.TrimSpace(branchOut)
	if branch == "" {
		return fmt.Errorf("cannot push: detached HEAD")
	}
	_, upErr := runCapture("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if upErr != nil {
		return runCommand("git", "-C", repoPath, "push", "-u", "origin", branch)
	}
	return runCommand("git", "-C", repoPath, "push")
}

func resolveAddRepo(repoRoot, modName, explicitRepo, explicitOwner, explicitRepoName string) (string, error) {
	if strings.TrimSpace(explicitRepo) != "" {
		return strings.TrimSpace(explicitRepo), nil
	}
	owner := strings.TrimSpace(explicitOwner)
	if owner == "" {
		inferred, err := inferGitHubOwner(repoRoot)
		if err != nil {
			return "", fmt.Errorf("cannot infer GitHub owner; pass --owner or --repo: %w", err)
		}
		owner = inferred
	}
	repoName := strings.TrimSpace(explicitRepoName)
	if repoName == "" {
		repoName = "dialtone-" + modName
	}
	return owner + "/" + repoName, nil
}

func splitOwnerRepo(v string) (owner, repo string) {
	v = strings.TrimSpace(v)
	if strings.Contains(v, "://") || strings.HasPrefix(v, "git@") || strings.HasPrefix(v, "/") || strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../") {
		return "", ""
	}
	parts := strings.Split(v, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func ensureGitHubRepoWithScaffold(owner, repoName, modName string, public bool, withUI bool, uiSource string) error {
	full := owner + "/" + repoName
	if _, err := runCapture("gh", "repo", "view", full, "--json", "name"); err != nil {
		visibility := "--private"
		if public {
			visibility = "--public"
		}
		if err := runCommand("gh", "repo", "create", full, visibility, "--confirm"); err != nil {
			return err
		}
	}

	empty := false
	if _, err := runCapture("gh", "api", "repos/"+full+"/contents"); err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "repository is empty") || strings.Contains(lower, "http 404") {
			empty = true
		} else {
			return err
		}
	}
	if !empty {
		return nil
	}
	return seedRepoScaffold(full, modName, withUI, uiSource)
}

func seedRepoScaffold(fullRepo, modName string, withUI bool, uiSource string) error {
	tmp, err := os.MkdirTemp("", "mod-seed-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := runCommand("gh", "repo", "clone", fullRepo, tmp); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tmp, "scaffold"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tmp, "v1", "go"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tmp, "v1", "cmd"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(tmp, "v1", "test", "cmd"), 0o755); err != nil {
		return err
	}

	readme := "# " + filepath.Base(fullRepo) + "\n\nDialtone mod scaffold for `" + modName + "`.\n"
	if err := os.WriteFile(filepath.Join(tmp, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}
	scaffold := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"" + modName + " v1 scaffold\")\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "scaffold", "main.go"), []byte(scaffold), 0o644); err != nil {
		return err
	}

	if withUI {
		if !fileExists(uiSource) {
			return fmt.Errorf("ui source path missing: %s", uiSource)
		}
		if err := copyDir(uiSource, filepath.Join(tmp, "v1", "ui")); err != nil {
			return err
		}
	}

	if err := runCommand("git", "-C", tmp, "add", "."); err != nil {
		return err
	}
	if err := runCommand("git", "-C", tmp, "commit", "-m", "Initial mod scaffold"); err != nil {
		return err
	}
	if err := runCommand("git", "-C", tmp, "branch", "-M", "main"); err != nil {
		return err
	}
	return runCommand("git", "-C", tmp, "push", "-u", "origin", "main")
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
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
		return nil
	})
}

func inferGitHubOwner(repoRoot string) (string, error) {
	out, err := runCapture("git", "-C", repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	u := strings.TrimSuffix(strings.TrimSpace(out), ".git")
	if u == "" {
		return "", fmt.Errorf("origin URL is empty")
	}
	if strings.Contains(u, "github.com/") {
		tail := strings.Split(u, "github.com/")[1]
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) >= 2 && strings.TrimSpace(parts[0]) != "" {
			return parts[0], nil
		}
	}
	if strings.Contains(u, "github.com:") {
		tail := strings.Split(u, "github.com:")[1]
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) >= 2 && strings.TrimSpace(parts[0]) != "" {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("unsupported origin URL format: %s", u)
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

func isValidModName(v string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	return re.MatchString(strings.TrimSpace(v))
}

func shellJoin(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.ContainsAny(p, " \t\n\"'") {
			out = append(out, "'"+strings.ReplaceAll(p, "'", "'\"'\"'")+"'")
		} else {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

type multiValueFlag struct {
	values []string
}

func (m *multiValueFlag) String() string { return strings.Join(m.values, ",") }

func (m *multiValueFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		m.values = append(m.values, value)
	}
	return nil
}
