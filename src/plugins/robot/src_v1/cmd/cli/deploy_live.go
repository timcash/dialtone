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
	if strings.TrimSpace(*user) == "" {
		return fmt.Errorf("sync-code requires --user (or ROBOT_USER in env/.env)")
	}
	if strings.TrimSpace(*remoteDir) == "" {
		*remoteDir = path.Join("/home", strings.TrimSpace(*user), "dialtone", "src")
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	repoRoot := rt.RepoRoot
	client, err := ssh_plugin.DialSSH(strings.TrimSpace(*host), strings.TrimSpace(*port), strings.TrimSpace(*user), *pass)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	remoteRoot := path.Dir(*remoteDir)
	if _, err := ssh_plugin.RunSSHCommand(client, "mkdir -p "+shellQuote(*remoteDir)); err != nil {
		return fmt.Errorf("failed creating remote dir: %w", err)
	}

	localSrc := rt.SrcRoot
	for _, p := range []string{"go.mod", "go.sum"} {
		if err := syncPath(client, localSrc, *remoteDir, p); err != nil {
			return err
		}
	}

	if err := ssh_plugin.UploadFile(client, filepath.Join(repoRoot, "dialtone.sh"), path.Join(remoteRoot, "dialtone.sh")); err != nil {
		return fmt.Errorf("failed to sync dialtone.sh: %w", err)
	}
	if _, err := ssh_plugin.RunSSHCommand(client, "chmod +x "+shellQuote(path.Join(remoteRoot, "dialtone.sh"))); err != nil {
		return fmt.Errorf("failed to chmod dialtone.sh: %w", err)
	}

	robotBase := path.Join("plugins", "robot", versionDir)
	robotSyncCandidates := []string{
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
		if err := syncPath(client, localSrc, *remoteDir, p); err != nil {
			return err
		}
	}

	buildHint := "cd " + shellQuote(*remoteDir) + " && go build ./plugins/robot/" + versionDir + "/cmd/server/main.go"
	logs.Info("[SYNC-CODE] Complete. Remote build command:")
	logs.Raw("  %s", buildHint)
	return nil
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
