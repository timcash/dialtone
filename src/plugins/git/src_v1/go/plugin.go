package gitv1

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	transport "github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

const defaultVersion = "src_v1"

func Run(args []string) error {
	version, command, rest, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		printUsage()
		return err
	}
	if warnedOldOrder {
		logs.Warn("old git CLI order is deprecated. Use: ./dialtone.sh git src_v1 <command> [args]")
	}
	if version == "" {
		version = defaultVersion
	}
	if version != defaultVersion {
		return fmt.Errorf("unsupported version %s (expected %s)", version, defaultVersion)
	}

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "clone":
		return runClone(rest)
	case "mesh-clone":
		return runMeshClone(rest)
	default:
		printUsage()
		return fmt.Errorf("unknown git command: %s", command)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return defaultVersion, "help", nil, false, nil
	}
	if isHelp(args[0]) {
		return defaultVersion, "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh git src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return defaultVersion, args[0], args[1:], false, nil
}

func isHelp(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh git src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  clone [--from wsl] [--source PATH] [--dest PATH] [--branch BRANCH] [--depth N]")
	logs.Raw("        Clone a repo from a mesh node over SSH (default source node: wsl)")
	logs.Raw("  mesh-clone [--host <name|all>] [--from wsl] [--source PATH] [--dest PATH]")
	logs.Raw("            [--branch BRANCH] [--branch-map host=branch] [--skip-self=true|false] [--dry-run]")
	logs.Raw("        Clone/sync on mesh target host(s) from another mesh node, sequential and self-aware")
	logs.Raw("  help")
	logs.Raw("")
	logs.Raw("Examples:")
	logs.Raw("  ./dialtone.sh git clone")
	logs.Raw("  ./dialtone.sh git clone --from rover --source /home/tim/dialtone --dest ./rover-clone")
	logs.Raw("  ./dialtone.sh git mesh-clone --host all --from wsl --branch main")
	logs.Raw("  ./dialtone.sh git mesh-clone --host all --branch-map gold=main --branch-map darkmac=feature-x")
}

func runClone(args []string) error {
	for _, a := range args {
		if isHelp(a) {
			fs := flag.NewFlagSet("git-clone", flag.ContinueOnError)
			fs.String("from", "wsl", "Mesh source node name")
			fs.String("source", "", "Source repo path on source node (default: node repo candidate)")
			fs.String("dest", "", "Local destination directory (default: <repo>-clone)")
			fs.String("branch", "", "Optional branch to checkout")
			fs.Int("depth", 0, "Optional shallow depth")
			fs.PrintDefaults()
			return nil
		}
	}
	fs := flag.NewFlagSet("git-clone", flag.ContinueOnError)
	from := fs.String("from", "wsl", "Mesh source node name")
	source := fs.String("source", "", "Source repo path on source node (default: node repo candidate)")
	dest := fs.String("dest", "", "Local destination directory (default: <repo>-clone)")
	branch := fs.String("branch", "", "Optional branch to checkout")
	depth := fs.Int("depth", 0, "Optional shallow depth")
	if err := fs.Parse(args); err != nil {
		return err
	}

	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*from))
	if err != nil {
		return err
	}

	srcPath := strings.TrimSpace(*source)
	if srcPath == "" {
		srcPath = defaultSourcePathForNode(node)
	}
	if srcPath == "" {
		return errors.New("source path is empty")
	}

	dstPath := strings.TrimSpace(*dest)
	if dstPath == "" {
		name := filepath.Base(strings.TrimRight(srcPath, "/"))
		if name == "" || name == "." || name == "/" {
			name = "dialtone"
		}
		dstPath = "./" + name + "-clone"
	}
	if _, err := os.Stat(dstPath); err == nil {
		return fmt.Errorf("destination already exists: %s", dstPath)
	}

	srcURL := sourceURL(node, srcPath)

	logs.Info("git clone from %s (%s) -> %s", node.Name, srcURL, dstPath)
	cloneOpts := &git.CloneOptions{
		URL:      srcURL,
		Progress: os.Stdout,
	}
	if *depth > 0 {
		cloneOpts.Depth = *depth
	}
	if strings.TrimSpace(*branch) != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(strings.TrimSpace(*branch))
		cloneOpts.SingleBranch = true
	}
	if auth := authForURL(srcURL); auth != nil {
		cloneOpts.Auth = auth
	}
	_, err = git.PlainClone(dstPath, false, cloneOpts)
	return err
}

func runMeshClone(args []string) error {
	fs := flag.NewFlagSet("git-mesh-clone", flag.ContinueOnError)
	host := fs.String("host", "all", "Target mesh host name or all")
	from := fs.String("from", "wsl", "Source mesh node name")
	source := fs.String("source", "", "Source repo path on source node (default: source repo candidate)")
	dest := fs.String("dest", "", "Destination repo path on target node (default: target repo candidate)")
	branch := fs.String("branch", "", "Default branch for target checkout/update")
	skipSelf := fs.Bool("skip-self", true, "When --host all, skip current local mesh node")
	dryRun := fs.Bool("dry-run", false, "Print commands without executing")
	var branchMapVals multiValueFlag
	fs.Var(&branchMapVals, "branch-map", "Per-host branch mapping host=branch (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	srcNode, err := sshv1.ResolveMeshNode(strings.TrimSpace(*from))
	if err != nil {
		return err
	}
	srcPath := strings.TrimSpace(*source)
	if srcPath == "" {
		srcPath = defaultSourcePathForNode(srcNode)
	}
	if srcPath == "" {
		return fmt.Errorf("source path is empty for source node %s", srcNode.Name)
	}

	branchMap, err := parseBranchMap(branchMapVals.values)
	if err != nil {
		return err
	}

	target := strings.ToLower(strings.TrimSpace(*host))
	if target == "" {
		target = "all"
	}
	if target == "all" {
		nodes := sshv1.ListMeshNodes()
		failed := 0
		for _, node := range nodes {
			if *skipSelf && isSelfMeshNode(node) {
				logs.Raw("== %s ==\nSKIP self node", node.Name)
				continue
			}
			if err := runMeshCloneForNode(node, srcNode, srcPath, strings.TrimSpace(*dest), pickBranchForNode(node.Name, strings.TrimSpace(*branch), branchMap), *dryRun); err != nil {
				failed++
				logs.Raw("== %s ==\nERROR: %v", node.Name, err)
			}
		}
		if failed > 0 {
			return fmt.Errorf("mesh-clone finished with %d host failures", failed)
		}
		return nil
	}

	node, err := sshv1.ResolveMeshNode(target)
	if err != nil {
		return err
	}
	return runMeshCloneForNode(node, srcNode, srcPath, strings.TrimSpace(*dest), pickBranchForNode(node.Name, strings.TrimSpace(*branch), branchMap), *dryRun)
}

func runMeshCloneForNode(targetNode, sourceNode sshv1.MeshNode, sourcePath, destOverride, branch string, dryRun bool) error {
	destPath := strings.TrimSpace(destOverride)
	if destPath == "" {
		if len(targetNode.RepoCandidates) > 0 {
			destPath = targetNode.RepoCandidates[0]
		}
	}
	if destPath == "" {
		return fmt.Errorf("cannot resolve destination path for %s; pass --dest", targetNode.Name)
	}

	sourceSpec := sourceURLForRemote(sourceNode, sourcePath)
	if strings.EqualFold(targetNode.Name, sourceNode.Name) {
		sourceSpec = sourcePath
	}

	cmd := buildRemoteCloneOrUpdateCommand(sourceSpec, destPath, branch)
	logs.Raw("== %s ==", targetNode.Name)
	if dryRun {
		logs.Raw("[DRY-RUN] %s", cmd)
		return nil
	}
	out, err := sshv1.RunNodeCommand(targetNode.Name, cmd, sshv1.CommandOptions{})
	if strings.TrimSpace(out) != "" {
		logs.Raw("%s", strings.TrimRight(out, "\n"))
	}
	return err
}

func buildRemoteCloneOrUpdateCommand(sourceSpec, destPath, branch string) string {
	b := strings.TrimSpace(branch)
	branchClone := ""
	branchFetch := ""
	branchCheckout := ""
	if b != "" {
		branchClone = "--branch " + shellQuote(b) + " --single-branch "
		branchFetch = "git -C " + shellQuote(destPath) + " fetch origin " + shellQuote(b)
		branchCheckout = "git -C " + shellQuote(destPath) + " checkout " + shellQuote(b)
	} else {
		branchFetch = "git -C " + shellQuote(destPath) + " fetch --all --prune"
		branchCheckout = "git -C " + shellQuote(destPath) + " checkout $(git -C " + shellQuote(destPath) + " rev-parse --abbrev-ref HEAD)"
	}
	branchPull := "git -C " + shellQuote(destPath) + " pull --ff-only"
	if b != "" {
		branchPull = "git -C " + shellQuote(destPath) + " pull --ff-only origin " + shellQuote(b)
	}

	lines := []string{
		"set -e",
		"if [ -d " + shellQuote(filepath.ToSlash(filepath.Join(destPath, ".git"))) + " ]; then",
		"  " + branchFetch,
		"  " + branchCheckout,
		"  " + branchPull,
		"else",
		"  mkdir -p " + shellQuote(filepath.ToSlash(filepath.Dir(destPath))),
		"  git clone " + branchClone + shellQuote(sourceSpec) + " " + shellQuote(destPath),
		"fi",
	}
	return strings.Join(lines, " ; ")
}

func parseBranchMap(values []string) (map[string]string, error) {
	out := map[string]string{}
	for _, v := range values {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid --branch-map value %q (expected host=branch)", s)
		}
		host := strings.ToLower(strings.TrimSpace(parts[0]))
		branch := strings.TrimSpace(parts[1])
		if host == "" || branch == "" {
			return nil, fmt.Errorf("invalid --branch-map value %q (expected host=branch)", s)
		}
		out[host] = branch
	}
	return out, nil
}

func pickBranchForNode(nodeName, defaultBranch string, branchMap map[string]string) string {
	key := strings.ToLower(strings.TrimSpace(nodeName))
	if b, ok := branchMap[key]; ok {
		return b
	}
	return defaultBranch
}

func defaultSourcePathForNode(node sshv1.MeshNode) string {
	if len(node.RepoCandidates) > 0 {
		return node.RepoCandidates[0]
	}
	if strings.EqualFold(node.OS, "windows") {
		return ""
	}
	home := "/home/" + strings.TrimSpace(node.User)
	if strings.EqualFold(node.OS, "macos") || strings.EqualFold(node.OS, "darwin") {
		home = "/Users/" + strings.TrimSpace(node.User)
	}
	return filepath.ToSlash(filepath.Join(home, "dialtone"))
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
	sort.Strings(candidates)
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

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

type multiValueFlag struct {
	values []string
}

func (m *multiValueFlag) String() string {
	return strings.Join(m.values, ",")
}

func (m *multiValueFlag) Set(value string) error {
	m.values = append(m.values, value)
	return nil
}

func sourceURL(node sshv1.MeshNode, srcPath string) string {
	srcPath = strings.TrimSpace(srcPath)
	if node.Name == "wsl" && runtime.GOOS == "linux" && strings.HasPrefix(srcPath, "/") {
		return srcPath
	}
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

func sourceURLForRemote(node sshv1.MeshNode, srcPath string) string {
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

func authForURL(rawURL string) transport.AuthMethod {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)), "ssh://") {
		return nil
	}
	ep, err := transport.NewEndpoint(rawURL)
	if err != nil {
		return nil
	}
	user := strings.TrimSpace(ep.User)
	if user == "" {
		return nil
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
