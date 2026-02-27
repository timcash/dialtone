package ssh

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run(args []string) error {
	if len(args) == 0 {
		PrintUsage()
		return nil
	}
	switch strings.TrimSpace(args[0]) {
	case "help", "--help", "-h":
		PrintUsage()
		return nil
	case "mesh", "nodes", "list":
		return runMeshList(args[1:])
	case "run":
		return runCommand(args[1:])
	case "run-all":
		return runCommandAll(args[1:])
	case "sync-repos":
		return runSyncRepos(args[1:])
	case "sync-code":
		return runSyncCode(args[1:])
	case "bootstrap":
		return runBootstrap(args[1:])
	default:
		PrintUsage()
		return fmt.Errorf("unknown ssh command: %s", args[0])
	}
}

func PrintUsage() {
	logs.Raw("Usage: ./dialtone.sh ssh src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  mesh|nodes|list                       List canonical mesh nodes and transport mode")
	logs.Raw("  run --node N --cmd C [--user U --port P --password X]")
	logs.Raw("                                        Run command on a mesh node")
	logs.Raw("  run-all --cmd C [--user U --port P --password X]")
	logs.Raw("                                        Run command on every mesh node")
	logs.Raw("  sync-repos [--branch B] [--allow-dirty]")
	logs.Raw("                                        Sync dialtone repo on every mesh node to one branch")
	logs.Raw("                                        Per-node repo override: --repo-<node> /path/to/repo")
	logs.Raw("  sync-code --node <name|all> [--src P] [--dest P] [--delete] [--exclude PATTERN]")
	logs.Raw("            [--service] [--interval 30s] [--service-stop] [--service-status]")
	logs.Raw("                                        Rsync code without git, excludes node_modules/.pixi by default")
	logs.Raw("  bootstrap --node <name|all> [--src P] [--dest P] [--delete] [--install-cmd C]")
	logs.Raw("                                        Sync code + run install command(s) on target node(s)")
	logs.Raw("  test                                  Run ssh plugin self-check suite")
}

func runMeshList(_ []string) error {
	nodes := ListMeshNodes()
	logs.Raw("NAME      USER   HOST                                   PORT  OS       TRANSPORT")
	for _, n := range nodes {
		transport := "ssh"
		if shouldUseLocalPowerShell(n) {
			transport = "powershell"
		}
		logs.Raw("%-9s %-6s %-38s %-5s %-8s %s", n.Name, n.User, n.Host, n.Port, n.OS, transport)
	}
	return nil
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("ssh run", flag.ContinueOnError)
	fs.SetOutput(nil)
	node := fs.String("node", "", "Mesh node name or alias")
	cmd := fs.String("cmd", "", "Command to execute")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional SSH password")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*node) == "" {
		return errors.New("--node is required")
	}
	if strings.TrimSpace(*cmd) == "" {
		return errors.New("--cmd is required")
	}
	out, err := RunNodeCommand(*node, *cmd, CommandOptions{
		User:     *user,
		Port:     *port,
		Password: *pass,
	})
	if strings.TrimSpace(out) != "" {
		logs.Raw("%s", strings.TrimRight(out, "\n"))
	}
	return err
}

func runCommandAll(args []string) error {
	fs := flag.NewFlagSet("ssh run-all", flag.ContinueOnError)
	fs.SetOutput(nil)
	cmd := fs.String("cmd", "", "Command to execute on all nodes")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional SSH password")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*cmd) == "" {
		return errors.New("--cmd is required")
	}

	failures := 0
	for _, node := range ListMeshNodes() {
		logs.Raw("== %s ==", node.Name)
		out, err := RunNodeCommand(node.Name, *cmd, CommandOptions{
			User:     *user,
			Port:     *port,
			Password: *pass,
		})
		if strings.TrimSpace(out) != "" {
			logs.Raw("%s", strings.TrimRight(out, "\n"))
		}
		if err != nil {
			failures++
			logs.Raw("ERROR: %v", err)
		}
	}
	if failures > 0 {
		return fmt.Errorf("run-all finished with %d node failures", failures)
	}
	return nil
}

func runSyncRepos(args []string) error {
	fs := flag.NewFlagSet("ssh sync-repos", flag.ContinueOnError)
	fs.SetOutput(nil)
	branch := fs.String("branch", "main", "Branch to sync")
	allowDirty := fs.Bool("allow-dirty", false, "Allow sync even if node repo has local changes")
	repoByNode := map[string]*string{}
	for _, node := range ListMeshNodes() {
		key := "repo-" + node.Name
		repoByNode[node.Name] = fs.String(key, "", "Repo path override for "+node.Name)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	repoPaths := map[string]string{}
	for k, v := range repoByNode {
		if strings.TrimSpace(*v) != "" {
			repoPaths[k] = strings.TrimSpace(*v)
		}
	}

	results := SyncReposAll(RepoSyncOptions{
		Branch:        *branch,
		AllowDirty:    *allowDirty,
		NodeRepoPaths: repoPaths,
	})
	failed := 0
	skipped := 0
	for _, r := range results {
		logs.Raw("== %s ==", r.Node)
		logs.Raw("repo=%s branch=%s", r.Repo, r.Branch)
		if strings.TrimSpace(r.Output) != "" {
			logs.Raw("%s", r.Output)
		}
		if r.Skipped {
			skipped++
		}
		if r.Err != nil {
			failed++
			logs.Raw("ERROR: %v", r.Err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("sync-repos finished with %d failures (%d skipped dirty)", failed, skipped)
	}
	if skipped > 0 {
		logs.Warn("sync-repos completed with %d dirty-skip node(s)", skipped)
	}
	return nil
}

func runSyncCode(args []string) error {
	fs := flag.NewFlagSet("ssh sync-code", flag.ContinueOnError)
	fs.SetOutput(nil)
	node := fs.String("node", "", "Target mesh node or 'all'")
	src := fs.String("src", "", "Source path (defaults to current working directory)")
	dest := fs.String("dest", "", "Destination path on target")
	del := fs.Bool("delete", false, "Delete files on dest that are missing in src")
	service := fs.Bool("service", false, "Install/start persistent user systemd sync service")
	serviceStop := fs.Bool("service-stop", false, "Stop/disable persistent user systemd sync service")
	serviceStatus := fs.Bool("service-status", false, "Show persistent user systemd sync service status")
	interval := fs.Duration("interval", 30*time.Second, "Sync interval used with --service")
	var excludes multiValueFlag
	fs.Var(&excludes, "exclude", "Extra exclude pattern (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := SyncCodeOptions{
		Node:     *node,
		Source:   *src,
		Dest:     *dest,
		Delete:   *del,
		Excludes: excludes.values,
	}
	if *serviceStop {
		return StopSyncCodeService()
	}
	if *serviceStatus {
		return StatusSyncCodeService()
	}
	if *service {
		if *interval <= 0 {
			return fmt.Errorf("--interval must be greater than 0")
		}
		return InstallSyncCodeService(opts, *interval)
	}
	return SyncCode(opts)
}

func runBootstrap(args []string) error {
	fs := flag.NewFlagSet("ssh bootstrap", flag.ContinueOnError)
	fs.SetOutput(nil)
	node := fs.String("node", "", "Target mesh node or 'all'")
	src := fs.String("src", "", "Source path (defaults to current working directory)")
	dest := fs.String("dest", "", "Destination path on target")
	del := fs.Bool("delete", false, "Delete files on dest that are missing in src")
	noSync := fs.Bool("no-sync", false, "Skip rsync and run install/verify only")
	verifyCmd := fs.String("verify-cmd", "./dialtone.sh go src_v1 exec version", "Post-install verify command")
	var installCmds multiValueFlag
	fs.Var(&installCmds, "install-cmd", "Install command to run on target (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cmds := installCmds.values
	if len(cmds) == 0 {
		cmds = []string{"printf 'y\\n' | ./dialtone.sh go src_v1 install"}
	}
	return Bootstrap(BootstrapOptions{
		Node:        *node,
		Source:      *src,
		Dest:        *dest,
		Delete:      *del,
		NoSync:      *noSync,
		InstallCmds: cmds,
		VerifyCmd:   *verifyCmd,
	})
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
