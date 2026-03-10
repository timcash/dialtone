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
	case "tailnet-check":
		return runTailnetCheck(args[1:])
	case "resolve":
		return runResolve(args[1:])
	case "probe":
		return runProbe(args[1:])
	case "run":
		return runCommand(args[1:])
	case "run-all":
		return runCommandAll(args[1:])
	case "status":
		return runStatus(args[1:])
	case "sync-repos":
		return runSyncRepos(args[1:])
	case "sync-code":
		return runSyncCode(args[1:])
	case "bootstrap":
		return runBootstrap(args[1:])
	case "keygen":
		return runKeygen(args[1:])
	case "key-install":
		return runKeyInstall(args[1:])
	case "key-setup":
		return runKeySetup(args[1:])
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
	logs.Raw("  tailnet-check [--host H|all] [--timeout 5s]")
	logs.Raw("                                        Verify SSH handshake over each node's tailscale host")
	logs.Raw("  format                                Run go fmt for the ssh plugin")
	logs.Raw("  run --host H --cmd C [--user U --port P --password X --key-path P] [--node N]")
	logs.Raw("                                        Run command on a mesh host (preferred flag: --host)")
	logs.Raw("  resolve --host H [--user U --port P]")
	logs.Raw("                                        Show resolved host/user/port/candidates/transport for one mesh node")
	logs.Raw("  probe --host H [--user U --port P --password X --key-path P --timeout 5s]")
	logs.Raw("                                        Probe reachability/auth for a mesh node with detailed per-candidate results")
	logs.Raw("  run-all --cmd C [--user U --port P --password X --key-path P]")
	logs.Raw("                                        Run command on every mesh node")
	logs.Raw("  status [--host H|all] [--json]")
	logs.Raw("                                        Show cpu/mem-free/network/disk-free/battery for mesh nodes")
	logs.Raw("  sync-repos [--branch B] [--allow-dirty]")
	logs.Raw("                                        Sync dialtone repo on every mesh node to one branch")
	logs.Raw("                                        Per-node repo override: --repo-<node> /path/to/repo")
	logs.Raw("  sync-code --host <name|all> [--src P] [--dest P] [--delete] [--exclude PATTERN] [--skip-self=true|false] [--node <name|all>]")
	logs.Raw("            [--service] [--interval 30s] [--service-stop] [--service-status]")
	logs.Raw("                                        Rsync code without git, excludes node_modules/.pixi by default")
	logs.Raw("  bootstrap --host <name|all> [--src P] [--dest P] [--delete] [--install-cmd C] [--node <name|all>]")
	logs.Raw("                                        Sync code + run install command(s) on target node(s)")
	logs.Raw("  keygen --host H [--key-path P --force]")
	logs.Raw("                                        Generate local ed25519 keypair for host and write key path to dialtone.json")
	logs.Raw("  key-install --host H [--user U --port P --password X --key-path P --pub-key-path P]")
	logs.Raw("                                        Install local public key to remote ~/.ssh/authorized_keys for passwordless auth")
	logs.Raw("  key-setup --host H [--user U --port P --password X --key-path P]")
	logs.Raw("                                        Generate key if needed, install it remotely, verify auth, and save key path in dialtone.json")
	logs.Raw("  test                                  Run ssh plugin self-check suite")
}

func runMeshList(_ []string) error {
	nodes := ListMeshNodes()
	logs.Raw("NAME      USER   HOST                                   TAILNET                                PORT  OS       TRANSPORT")
	for _, n := range nodes {
		transport := "ssh"
		if shouldUseLocalPowerShell(n) {
			transport = "powershell"
		}
		logs.Raw("%-9s %-6s %-38s %-38s %-5s %-8s %s", n.Name, n.User, PreferredHost(n, n.Port), RouteHost(n, meshRouteTailnet, n.Port), n.Port, n.OS, transport)
	}
	return nil
}

func runTailnetCheck(args []string) error {
	fs := flag.NewFlagSet("ssh tailnet-check", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "all", "Target mesh host or 'all'")
	timeout := fs.Duration("timeout", 5*time.Second, "Per-node command timeout")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional SSH password")
	keyPath := fs.String("key-path", "", "Optional SSH private key path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	targets := make([]MeshNode, 0)
	rawHost := strings.TrimSpace(*host)
	if rawHost == "" || rawHost == "all" {
		targets = ListMeshNodes()
	} else {
		for _, part := range strings.Split(rawHost, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			node, err := ResolveMeshNode(part)
			if err != nil {
				return err
			}
			targets = append(targets, node)
		}
	}

	logs.Raw("NAME      USER   TAILNET                                PORT  RESULT   DETAILS")
	failures := 0
	for _, node := range targets {
		portValue := strings.TrimSpace(*port)
		if portValue == "" {
			portValue = strings.TrimSpace(node.Port)
		}
		if portValue == "" {
			portValue = "22"
		}
		hostValue := RouteHost(node, meshRouteTailnet, portValue)
		userValue := strings.TrimSpace(*user)
		if userValue == "" {
			userValue = node.User
		}
		if hostValue == "" {
			failures++
			logs.Raw("%-9s %-6s %-38s %-5s %-8s %s", node.Name, userValue, "-", portValue, "NO_ROUTE", "no tailscale host candidate")
			continue
		}

		started := time.Now()
		client, _, resolvedHost, resolvedPort, err := DialMeshNodeViaRoute(node.Name, meshRouteTailnet, CommandOptions{
			User:           userValue,
			Port:           portValue,
			Password:       *pass,
			PrivateKeyPath: *keyPath,
		})
		if err != nil {
			failures++
			detail := summarizeTailnetCheckError(err)
			logs.Raw("%-9s %-6s %-38s %-5s %-8s %s", node.Name, userValue, hostValue, portValue, "FAIL", detail)
			continue
		}

		commandTimeout := *timeout
		if commandTimeout <= 0 {
			commandTimeout = 5 * time.Second
		}
		resultCh := make(chan error, 1)
		go func() {
			defer client.Close()
			_, runErr := runSSHFunc(client, "printf tailnet-ok")
			resultCh <- runErr
		}()

		select {
		case runErr := <-resultCh:
			if runErr != nil {
				failures++
				logs.Raw("%-9s %-6s %-38s %-5s %-8s %s", node.Name, userValue, resolvedHost, resolvedPort, "FAIL", summarizeTailnetCheckError(runErr))
				continue
			}
			logs.Raw("%-9s %-6s %-38s %-5s %-8s %s", node.Name, userValue, resolvedHost, resolvedPort, "PASS", time.Since(started).Round(10*time.Millisecond))
		case <-time.After(commandTimeout):
			_ = client.Close()
			failures++
			logs.Raw("%-9s %-6s %-38s %-5s %-8s %s", node.Name, userValue, resolvedHost, resolvedPort, "TIMEOUT", commandTimeout)
		}
	}
	if failures > 0 {
		return fmt.Errorf("tailnet-check finished with %d failure(s)", failures)
	}
	return nil
}

func summarizeTailnetCheckError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	msg = strings.ReplaceAll(msg, "\n", " | ")
	if len(msg) > 96 {
		return msg[:93] + "..."
	}
	return msg
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("ssh run", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "", "Mesh host name or alias")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	cmd := fs.String("cmd", "", "Command to execute")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional SSH password")
	keyPath := fs.String("key-path", "", "Optional SSH private key path")
	connectTimeout := fs.Duration("connect-timeout", 10*time.Second, "SSH connect/auth timeout per attempt")
	debug := fs.Bool("debug", false, "Print resolved host selection/debug info")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	if target == "" {
		return errors.New("--host is required")
	}
	if strings.TrimSpace(*cmd) == "" {
		return errors.New("--cmd is required")
	}
	opts := CommandOptions{
		User:           *user,
		Port:           *port,
		Password:       *pass,
		PrivateKeyPath: *keyPath,
		Debug:          *debug,
	}
	opts.ConnectTimeout = *connectTimeout
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 10 * time.Second
	}
	if *debug {
		if report, err := BuildResolveReport(target, opts); err == nil {
			logs.Raw("Resolved node: %s", report.Name)
			logs.Raw("  transport: %s", report.Transport)
			logs.Raw("  user: %s", report.User)
			logs.Raw("  port: %s", report.Port)
			logs.Raw("  preferred: %s", report.PreferredHost)
			logs.Raw("  route[tailscale]: %s", report.RouteTailnet)
			logs.Raw("  route[private]: %s", report.RoutePrivate)
			logs.Raw("  candidates: %s", strings.Join(report.Candidates, ", "))
		}
	}
	started := time.Now()
	out, err := RunNodeCommand(target, *cmd, opts)
	if strings.TrimSpace(out) != "" {
		logs.Raw("%s", strings.TrimRight(out, "\n"))
	}
	if *debug {
		logs.Raw("Elapsed: %s", time.Since(started).Round(10*time.Millisecond))
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
	keyPath := fs.String("key-path", "", "Optional SSH private key path")
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
			User:           *user,
			Port:           *port,
			Password:       *pass,
			PrivateKeyPath: *keyPath,
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

func runResolve(args []string) error {
	fs := flag.NewFlagSet("ssh resolve", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "", "Mesh host name or alias")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	if target == "" {
		return errors.New("--host is required")
	}
	report, err := BuildResolveReport(target, CommandOptions{User: *user, Port: *port})
	if err != nil {
		return err
	}
	logs.Raw("name=%s", report.Name)
	logs.Raw("transport=%s", report.Transport)
	logs.Raw("user=%s", report.User)
	logs.Raw("port=%s", report.Port)
	logs.Raw("preferred=%s", report.PreferredHost)
	logs.Raw("route.tailscale=%s", report.RouteTailnet)
	logs.Raw("route.private=%s", report.RoutePrivate)
	logs.Raw("candidates=%s", strings.Join(report.Candidates, ","))
	return nil
}

func runProbe(args []string) error {
	fs := flag.NewFlagSet("ssh probe", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "", "Mesh host name or alias")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional SSH password")
	keyPath := fs.String("key-path", "", "Optional SSH private key path")
	timeout := fs.Duration("timeout", 5*time.Second, "Per-candidate probe timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	if target == "" {
		return errors.New("--host is required")
	}
	opts := CommandOptions{
		User:           *user,
		Port:           *port,
		Password:       *pass,
		PrivateKeyPath: *keyPath,
	}
	opts.ConnectTimeout = *timeout
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = 5 * time.Second
	}
	report, err := BuildResolveReport(target, opts)
	if err != nil {
		return err
	}
	logs.Raw("Probe target=%s transport=%s user=%s port=%s", report.Name, report.Transport, report.User, report.Port)
	if report.Transport != "ssh" {
		logs.Raw("transport %s does not use ssh candidate dialing", report.Transport)
		return nil
	}
	resolvedNode, err := ResolveMeshNode(target)
	if err != nil {
		return err
	}
	passValue := strings.TrimSpace(opts.Password)
	if passValue == "" {
		passValue = strings.TrimSpace(resolvedNode.Password)
	}
	keyPathValue := strings.TrimSpace(opts.PrivateKeyPath)
	if keyPathValue == "" {
		keyPathValue = strings.TrimSpace(resolvedNode.SSHPrivateKeyPath)
	}
	failures := 0
	for _, c := range report.Candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		tcpOK := CanReachHostPort(c, report.Port, opts.ConnectTimeout)
		tcpState := "unreachable"
		if tcpOK {
			tcpState = "reachable"
		}
		started := time.Now()
		client, err := DialSSHWithAuth(c, report.Port, report.User, passValue, keyPathValue, opts.ConnectTimeout)
		if err != nil {
			failures++
			logs.Raw("candidate=%s tcp=%s auth=FAIL elapsed=%s err=%s", c, tcpState, time.Since(started).Round(10*time.Millisecond), summarizeTailnetCheckError(err))
			continue
		}
		_ = client.Close()
		logs.Raw("candidate=%s tcp=%s auth=PASS elapsed=%s", c, tcpState, time.Since(started).Round(10*time.Millisecond))
	}
	if failures > 0 {
		return fmt.Errorf("probe finished with %d auth failure(s)", failures)
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
	host := fs.String("host", "", "Target mesh host or 'all'")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	src := fs.String("src", "", "Source path (defaults to current working directory)")
	dest := fs.String("dest", "", "Destination path on target")
	del := fs.Bool("delete", false, "Delete files on dest that are missing in src")
	skipSelf := fs.Bool("skip-self", true, "When --host all, skip syncing the current node")
	service := fs.Bool("service", false, "Install/start persistent user systemd sync service")
	serviceStop := fs.Bool("service-stop", false, "Stop/disable persistent user systemd sync service")
	serviceStatus := fs.Bool("service-status", false, "Show persistent user systemd sync service status")
	interval := fs.Duration("interval", 30*time.Second, "Sync interval used with --service")
	var excludes multiValueFlag
	fs.Var(&excludes, "exclude", "Extra exclude pattern (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	opts := SyncCodeOptions{
		Node:     target,
		Source:   *src,
		Dest:     *dest,
		Delete:   *del,
		SkipSelf: *skipSelf,
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
	host := fs.String("host", "", "Target mesh host or 'all'")
	node := fs.String("node", "", "Alias for --host (deprecated)")
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

	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	cmds := installCmds.values
	if len(cmds) == 0 {
		cmds = []string{"printf 'y\\n' | ./dialtone.sh go src_v1 install"}
	}
	return Bootstrap(BootstrapOptions{
		Node:        target,
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
