package ops

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	robot_cli "dialtone/dev/plugins/robot/src_v1/cmd/cli"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
)

type remoteOptions struct {
	Remote    bool
	Host      string
	Port      string
	User      string
	Pass      string
	RemoteDir string
}

func parseRemoteOptions(cmdName string, args []string) (remoteOptions, []string, error) {
	opts := remoteOptions{}
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	fs.SetOutput(nil)
	fs.BoolVar(&opts.Remote, "remote", false, "run on remote robot")
	fs.StringVar(&opts.Host, "host", "", "remote host")
	fs.StringVar(&opts.Port, "port", "22", "remote ssh port")
	fs.StringVar(&opts.User, "user", "", "remote ssh user")
	fs.StringVar(&opts.Pass, "pass", "", "remote ssh password")
	fs.StringVar(&opts.RemoteDir, "remote-dir", "", "remote source directory")
	if err := fs.Parse(args); err != nil {
		return remoteOptions{}, nil, err
	}
	rest := fs.Args()
	opts.Host = strings.TrimSpace(chooseNonEmpty(opts.Host, getenvTrim("ROBOT_HOST")))
	opts.User = strings.TrimSpace(chooseNonEmpty(opts.User, getenvTrim("ROBOT_USER")))
	opts.Pass = chooseNonEmpty(opts.Pass, getenvTrim("ROBOT_PASSWORD"))
	if opts.Port == "" {
		opts.Port = "22"
	}
	if opts.RemoteDir == "" && opts.User != "" {
		opts.RemoteDir = path.Join("/home", opts.User, "dialtone_src")
	}
	return opts, rest, nil
}

func runRemoteInstall(opts remoteOptions) error {
	if err := validateRemoteOptions(opts); err != nil {
		return err
	}
	if err := syncSourceForRemote(opts); err != nil {
		return err
	}
	cmd := remoteBootstrapAndInstallCommand(opts.RemoteDir)
	logs.Info("[ROBOT INSTALL] running install on remote host %s", opts.Host)
	_, err := runRemoteCommand(opts, cmd)
	return err
}

func runRemoteBuild(opts remoteOptions) error {
	if err := validateRemoteOptions(opts); err != nil {
		return err
	}
	if err := syncSourceForRemote(opts); err != nil {
		return err
	}
	cmd := remoteBuildCommand(opts.RemoteDir)
	logs.Info("[ROBOT BUILD] running build on remote host %s", opts.Host)
	_, err := runRemoteCommand(opts, cmd)
	return err
}

func runRemoteServe(opts remoteOptions) error {
	if err := validateRemoteOptions(opts); err != nil {
		return err
	}
	hostname := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	if hostname == "" {
		hostname = "drone-1"
	}
	authKey := strings.TrimSpace(os.Getenv("ROBOT_TS_AUTHKEY"))
	if authKey == "" {
		authKey = strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	}
	mavEndpoint := strings.TrimSpace(os.Getenv("ROBOT_MAVLINK_ENDPOINT"))
	if mavEndpoint == "" {
		mavEndpoint = strings.TrimSpace(os.Getenv("MAVLINK_ENDPOINT"))
	}
	cmd := remoteServeCommand(opts.RemoteDir, hostname, authKey, mavEndpoint)
	logs.Info("[ROBOT SERVE] starting remote server on host %s", opts.Host)
	_, err := runRemoteCommand(opts, cmd)
	return err
}

func validateRemoteOptions(opts remoteOptions) error {
	if !opts.Remote {
		return nil
	}
	if opts.Host == "" || opts.User == "" || opts.Pass == "" {
		return fmt.Errorf("--remote requires host/user/pass (or ROBOT_HOST/ROBOT_USER/ROBOT_PASSWORD in env/dialtone.json)")
	}
	if opts.RemoteDir == "" {
		return fmt.Errorf("missing remote source dir")
	}
	return nil
}

func syncSourceForRemote(opts remoteOptions) error {
	syncArgs := []string{
		"--host", opts.Host,
		"--port", opts.Port,
		"--user", opts.User,
		"--pass", opts.Pass,
		"--remote-dir", opts.RemoteDir,
	}
	return robot_cli.RunSyncCode("src_v1", syncArgs)
}

func runRemoteCommand(opts remoteOptions, remoteCmd string) (string, error) {
	client, err := ssh_plugin.DialSSH(opts.Host, opts.Port, opts.User, opts.Pass)
	if err != nil {
		return "", fmt.Errorf("remote ssh connect failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("remote create session failed: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("remote stdout pipe failed: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("remote stderr pipe failed: %w", err)
	}

	var (
		wg       sync.WaitGroup
		combined bytes.Buffer
		mu       sync.Mutex
	)
	stream := func(prefix string, r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			logs.Raw("[REMOTE %s] %s", prefix, line)
			mu.Lock()
			combined.WriteString(line)
			combined.WriteByte('\n')
			mu.Unlock()
		}
	}

	wg.Add(2)
	go stream("OUT", stdout)
	go stream("ERR", stderr)

	if err := session.Start(remoteCmd); err != nil {
		return "", fmt.Errorf("remote command start failed: %w", err)
	}
	waitErr := session.Wait()
	wg.Wait()
	out := combined.String()
	if waitErr != nil {
		return out, fmt.Errorf("remote command failed: %w", waitErr)
	}
	return out, nil
}

func remoteBootstrapAndInstallCommand(remoteDir string) string {
	return strings.Join([]string{
		"set -euo pipefail",
		"REMOTE_DIR=" + shellQuote(remoteDir),
		`mkdir -p "$REMOTE_DIR/plugins/robot/src_v1/ui"`,
		`cd "$REMOTE_DIR"`,
		`./dialtone.sh install`,
		`./dialtone.sh bun src_v1 install --cwd "$REMOTE_DIR/plugins/robot/src_v1/ui" --frozen-lockfile`,
	}, "\n")
}

func remoteBuildCommand(remoteDir string) string {
	return strings.Join([]string{
		remoteBootstrapAndInstallCommand(remoteDir),
		`./dialtone.sh bun src_v1 exec --cwd "$REMOTE_DIR/plugins/robot/src_v1/ui" run build`,
		`mkdir -p "$REMOTE_DIR/plugins/robot/src_v1/bin"`,
		`./dialtone.sh go src_v1 exec build -o "$REMOTE_DIR/plugins/robot/src_v1/bin/robot-src_v1" ./plugins/robot/src_v1/cmd/server/main.go`,
		`ls -lh "$REMOTE_DIR/plugins/robot/src_v1/bin/robot-src_v1"`,
	}, "\n")
}

func remoteServeCommand(remoteDir, hostname, authKey, mavEndpoint string) string {
	lines := []string{
		"set -euo pipefail",
		"REMOTE_DIR=" + shellQuote(remoteDir),
		`APP_DIR="$REMOTE_DIR/plugins/robot/src_v1"`,
		`BIN="$REMOTE_DIR/plugins/robot/src_v1/bin/robot-src_v1"`,
		`HOSTNAME=` + shellQuote(hostname),
		`AUTHKEY=` + shellQuote(authKey),
		`MAV_ENDPOINT=` + shellQuote(mavEndpoint),
		`if [ ! -x "$BIN" ]; then`,
		`  echo "missing remote binary: $BIN" >&2`,
		`  echo "run: ./dialtone.sh robot src_v1 build --remote" >&2`,
		`  exit 1`,
		`fi`,
		`pkill -x robot-src_v1 || true`,
		`sleep 1`,
		`if [ -n "$AUTHKEY" ]; then`,
		`  (cd "$APP_DIR" && ROBOT_TSNET=1 DIALTONE_HOSTNAME="$HOSTNAME" ROBOT_TS_AUTHKEY="$AUTHKEY" ROBOT_MAVLINK_ENDPOINT="$MAV_ENDPOINT" nohup "$BIN" >/dev/null 2>&1 < /dev/null &)`,
		`else`,
		`  (cd "$APP_DIR" && ROBOT_MAVLINK_ENDPOINT="$MAV_ENDPOINT" nohup "$BIN" >/dev/null 2>&1 < /dev/null &)`,
		`fi`,
		`sleep 1`,
		`echo "pids:"`,
		`pgrep -af "robot-src_v1" || true`,
		`echo "listen-8080:"`,
		`(ss -ltnp 2>/dev/null | grep ':8080' || netstat -ltnp 2>/dev/null | grep ':8080' || true)`,
		`echo "health:"`,
		`curl -fsS --max-time 5 http://127.0.0.1:8080/health || true`,
		`echo`,
	}
	return strings.Join(lines, "\n")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func chooseNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func getenvTrim(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
