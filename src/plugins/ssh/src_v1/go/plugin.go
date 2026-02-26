package ssh

import (
	"errors"
	"flag"
	"fmt"
	"strings"

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
