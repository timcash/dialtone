package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunSyncWatch(versionDir string, args []string) error {
	if versionDir == "" {
		versionDir = "src_v1"
	}
	fs := flag.NewFlagSet("robot-sync-watch", flag.ContinueOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	remoteSrc := fs.String("remote-src", "", "Remote src root (default: /home/<user>/dialtone/src)")
	interval := fs.Int("interval", 2, "Sync interval in seconds")
	if err := fs.Parse(args); err != nil {
		return err
	}

	action := "start"
	if fs.NArg() > 0 {
		action = strings.ToLower(strings.TrimSpace(fs.Arg(0)))
	}

	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("sync-watch requires --host (or ROBOT_HOST in env/dialtone.json)")
	}
	if strings.TrimSpace(*user) == "" {
		return fmt.Errorf("sync-watch requires --user (or ROBOT_USER in env/dialtone.json)")
	}
	if strings.TrimSpace(*remoteSrc) == "" {
		*remoteSrc = path.Join("/home", strings.TrimSpace(*user), "dialtone", "src")
	}
	if *interval <= 0 {
		*interval = 2
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}

	scriptPath := filepath.Join(rt.SrcRoot, "plugins", "robot", versionDir, "scripts", "sync_watch.sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("sync-watch script missing for %s at %s", versionDir, scriptPath)
	}

	stateDir := filepath.Join(logs.GetDialtoneEnv(), "robot", versionDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	pidPath := filepath.Join(stateDir, "sync-watch.pid")
	logPath := filepath.Join(stateDir, "sync-watch.log")

	switch action {
	case "start":
		if running, pid := isRunningPID(pidPath); running {
			logs.Info("[SYNC-WATCH] already running (pid=%d)", pid)
			logs.Raw("log: %s", logPath)
			return nil
		}
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer logFile.Close()

		cmd := exec.Command("bash", scriptPath)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		cmd.Env = append(os.Environ(),
			"ROVER_HOST="+strings.TrimSpace(*host),
			"ROVER_USER="+strings.TrimSpace(*user),
			"LOCAL_REPO="+rt.RepoRoot,
			"REMOTE_SRC="+strings.TrimSpace(*remoteSrc),
			"INTERVAL="+strconv.Itoa(*interval),
			"ROBOT_VERSION="+versionDir,
		)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			return err
		}
		if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
			return err
		}
		logs.Info("[SYNC-WATCH] started for %s (pid=%d)", versionDir, cmd.Process.Pid)
		logs.Raw("pid: %s", pidPath)
		logs.Raw("log: %s", logPath)
		return nil
	case "stop":
		pid, err := readPID(pidPath)
		if err != nil {
			logs.Info("[SYNC-WATCH] not running (no pid file)")
			return nil
		}
		proc, err := os.FindProcess(pid)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
		_ = os.Remove(pidPath)
		logs.Info("[SYNC-WATCH] stopped (pid=%d)", pid)
		return nil
	case "status":
		if running, pid := isRunningPID(pidPath); running {
			logs.Info("[SYNC-WATCH] running (pid=%d)", pid)
			logs.Raw("pid: %s", pidPath)
			logs.Raw("log: %s", logPath)
			return nil
		}
		logs.Info("[SYNC-WATCH] not running")
		logs.Raw("pid: %s", pidPath)
		logs.Raw("log: %s", logPath)
		return nil
	default:
		return fmt.Errorf("unsupported sync-watch action %q (use: start|stop|status)", action)
	}
}

func readPID(pidPath string) (int, error) {
	b, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return 0, fmt.Errorf("empty pid file")
	}
	return strconv.Atoi(raw)
}

func isRunningPID(pidPath string) (bool, int) {
	pid, err := readPID(pidPath)
	if err != nil {
		return false, 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, pid
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid
	}
	return true, pid
}
