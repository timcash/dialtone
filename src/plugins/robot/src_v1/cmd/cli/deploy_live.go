package cli

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
)

func RunSyncCode(versionDir string, args []string) error {
	if versionDir == "" {
		versionDir = "src_v1"
	}
	fs := flag.NewFlagSet("robot-sync-code", flag.ContinueOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	remoteDir := fs.String("remote-dir", "", "Remote source root (default: /home/<user>/dialtone/src)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("sync-code requires --host (or ROBOT_HOST in env/.env)")
	}
	node, err := ssh_plugin.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return fmt.Errorf("sync-code requires a mesh node alias/hostname for --host: %w", err)
	}
	if strings.TrimSpace(*user) == "" {
		*user = node.User
	}
	if strings.TrimSpace(*port) == "" {
		*port = node.Port
	}
	if strings.TrimSpace(*remoteDir) == "" {
		remoteRepo := ""
		if len(node.RepoCandidates) > 0 {
			remoteRepo = strings.TrimSpace(node.RepoCandidates[0])
		}
		if remoteRepo == "" {
			switch strings.ToLower(strings.TrimSpace(node.OS)) {
			case "macos":
				remoteRepo = path.Join("/Users", strings.TrimSpace(*user), "dialtone")
			default:
				remoteRepo = path.Join("/home", strings.TrimSpace(*user), "dialtone")
			}
		}
		*remoteDir = path.Join(remoteRepo, "src")
	}
	if strings.TrimSpace(*user) == "" {
		return fmt.Errorf("sync-code requires --user or a mesh node with default user")
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	repoRoot := rt.RepoRoot
	cmdOpts := ssh_plugin.CommandOptions{
		User:     strings.TrimSpace(*user),
		Port:     strings.TrimSpace(*port),
		Password: *pass,
	}
	targetNode := node.Name

	remoteRoot := path.Dir(*remoteDir)
	if _, err := ssh_plugin.RunNodeCommand(targetNode, "mkdir -p "+shellQuote(*remoteDir), cmdOpts); err != nil {
		return fmt.Errorf("failed creating remote dir: %w", err)
	}

	localSrc := rt.SrcRoot
	for _, p := range []string{"go.mod", "go.sum"} {
		if err := syncPathMesh(targetNode, cmdOpts, localSrc, *remoteDir, p); err != nil {
			return err
		}
	}

	if err := ssh_plugin.UploadNodeFile(targetNode, filepath.Join(repoRoot, "dialtone.sh"), path.Join(remoteRoot, "dialtone.sh"), cmdOpts); err != nil {
		return fmt.Errorf("failed to sync dialtone.sh: %w", err)
	}
	if _, err := ssh_plugin.RunNodeCommand(targetNode, "chmod +x "+shellQuote(path.Join(remoteRoot, "dialtone.sh")), cmdOpts); err != nil {
		return fmt.Errorf("failed to chmod dialtone.sh: %w", err)
	}

	robotBase := path.Join("plugins", "robot", versionDir)
	robotSyncCandidates := []string{
		path.Join("plugins", "robot", "src_v1", "cmd", "cli"),
		path.Join(robotBase, "cmd"),
		path.Join(robotBase, "go"),
		path.Join(robotBase, "config"),
		path.Join(robotBase, "test"),
		path.Join(robotBase, "ui", "src"),
		path.Join(robotBase, "ui", "public"),
		path.Join(robotBase, "ui", "index.html"),
		path.Join(robotBase, "ui", "package.json"),
		path.Join(robotBase, "ui", "bun.lock"),
		path.Join(robotBase, "ui", "tsconfig.json"),
		path.Join(robotBase, "ui", "vite.config.ts"),
		path.Join("plugins", "mavlink"),
		path.Join("plugins", "camera"),
		path.Join("plugins", "autoswap"),
		path.Join("plugins", "repl", "src_v1"),
		path.Join("plugins", "config"),
		path.Join("plugins", "test"),
		path.Join("plugins", "logs"),
		path.Join("plugins", "ui", "src_v1", "ui"),
	}
	robotSyncPaths := make([]string, 0, len(robotSyncCandidates))
	for _, p := range robotSyncCandidates {
		if _, statErr := os.Stat(filepath.Join(localSrc, filepath.FromSlash(p))); statErr == nil {
			robotSyncPaths = append(robotSyncPaths, p)
		}
	}
	for _, p := range robotSyncPaths {
		if err := syncPathMesh(targetNode, cmdOpts, localSrc, *remoteDir, p); err != nil {
			return err
		}
	}

	buildHint := "cd " + shellQuote(*remoteDir) + " && go build ./plugins/robot/" + versionDir + "/cmd/server/main.go"
	logs.Info("[SYNC-CODE] Complete. Remote build command:")
	logs.Raw("  %s", buildHint)
	return nil
}

func syncPathMesh(node string, opts ssh_plugin.CommandOptions, localSrcRoot, remoteSrcRoot, rel string) error {
	localPath := filepath.Join(localSrcRoot, filepath.FromSlash(rel))
	remotePath := path.Join(remoteSrcRoot, rel)
	fi, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("sync missing local path %s: %w", localPath, err)
	}
	logs.Info("[SYNC-CODE] Sync %s", rel)
	if !fi.IsDir() {
		if _, err := ssh_plugin.RunNodeCommand(node, "mkdir -p "+shellQuote(path.Dir(remotePath)), opts); err != nil {
			return fmt.Errorf("failed preparing remote dir %s: %w", path.Dir(remotePath), err)
		}
		if err := ssh_plugin.UploadNodeFile(node, localPath, remotePath, opts); err != nil {
			return fmt.Errorf("failed syncing file %s: %w", rel, err)
		}
		return nil
	}
	if _, err := ssh_plugin.RunNodeCommand(node, "mkdir -p "+shellQuote(remotePath), opts); err != nil {
		return fmt.Errorf("failed preparing remote dir %s: %w", remotePath, err)
	}
	return filepath.WalkDir(localPath, func(lp string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rp, err := filepath.Rel(localPath, lp)
		if err != nil {
			return err
		}
		rp = filepath.ToSlash(rp)
		remoteChild := remotePath
		if rp != "." {
			remoteChild = path.Join(remotePath, rp)
		}
		if d.IsDir() {
			_, err := ssh_plugin.RunNodeCommand(node, "mkdir -p "+shellQuote(remoteChild), opts)
			return err
		}
		if _, err := ssh_plugin.RunNodeCommand(node, "mkdir -p "+shellQuote(path.Dir(remoteChild)), opts); err != nil {
			return err
		}
		return ssh_plugin.UploadNodeFile(node, lp, remoteChild, opts)
	})
}

func RunDeploy(versionDir string, args []string) error {
	if versionDir == "" {
		versionDir = "src_v1"
	}
	fs := flag.NewFlagSet("robot-deploy", flag.ContinueOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	ephemeral := fs.Bool("ephemeral", false, "Register as ephemeral node on Tailscale")
	relay := fs.Bool("relay", false, "Configure local Cloudflare relay for robot Web UI from this host")
	service := fs.Bool("service", false, "Install/restart dialtone-robot.service on the robot")
	smoke := fs.Bool("smoke-test", false, "Run UI smoke test against drone-1.dialtone.earth after deploy")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := deployOptions{
		Host:      strings.TrimSpace(*host),
		Port:      strings.TrimSpace(*port),
		User:      strings.TrimSpace(*user),
		Pass:      *pass,
		Ephemeral: *ephemeral,
		Relay:     *relay,
		Service:   *service,
		SmokeTest: *smoke,
	}
	if opts.Port == "" {
		opts.Port = "22"
	}
	if opts.Host == "" || opts.Pass == "" {
		return fmt.Errorf("deploy requires --host and --pass (or ROBOT_HOST/ROBOT_PASSWORD in env/.env)")
	}
	return deployRobot(versionDir, opts)
}
