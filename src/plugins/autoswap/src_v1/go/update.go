package autoswap

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshplugin "dialtone/dev/plugins/ssh/src_v1/go"
)

type updateOptions struct {
	Host string
	Port string
	User string
	Pass string
}

func RunUpdate(args []string) error {
	fs := flag.NewFlagSet("autoswap-update", flag.ContinueOnError)
	host := fs.String("host", strings.TrimSpace(os.Getenv("ROBOT_HOST")), "SSH mesh host (for example rover/chroma/darkmac/legion)")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", strings.TrimSpace(os.Getenv("ROBOT_USER")), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password (optional when SSH key auth is configured)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := updateOptions{
		Host: strings.TrimSpace(*host),
		Port: strings.TrimSpace(*port),
		User: strings.TrimSpace(*user),
		Pass: *pass,
	}
	if opts.Port == "" {
		opts.Port = "22"
	}
	if opts.Host == "" {
		return fmt.Errorf("update requires --host (or ROBOT_HOST env)")
	}

	node, err := sshplugin.ResolveMeshNode(opts.Host)
	if err != nil {
		return fmt.Errorf("update requires mesh node alias for --host: %w", err)
	}
	if opts.User == "" {
		opts.User = node.User
	}
	if opts.Port == "" {
		opts.Port = node.Port
	}
	if strings.TrimSpace(opts.User) == "" {
		return fmt.Errorf("update requires --user or a mesh node with a default user")
	}

	cmdOpts := sshplugin.CommandOptions{
		User:     opts.User,
		Port:     opts.Port,
		Password: opts.Pass,
	}
	logs.Info("[UPDATE] forcing autoswap refresh on mesh node=%s as %s", node.Name, opts.User)

	cmd := strings.Join([]string{
		"set -e",
		"systemctl --user restart dialtone_autoswap.service",
		"sleep 1",
		"systemctl --user is-active dialtone_autoswap.service",
		"systemctl --user show dialtone_autoswap.service --property=ExecStart --no-pager",
		"if [ -f \"$HOME/.dialtone/autoswap/state/supervisor.json\" ]; then cat \"$HOME/.dialtone/autoswap/state/supervisor.json\"; fi",
	}, " && ")
	out, err := sshplugin.RunNodeCommand(node.Name, cmd, cmdOpts)
	if err != nil {
		return fmt.Errorf("remote autoswap update failed: %w", err)
	}
	logs.Raw("%s", strings.TrimSpace(out))
	logs.Info("[UPDATE] autoswap refresh command completed on %s at %s", node.Name, time.Now().UTC().Format(time.RFC3339))
	return nil
}
