package repl

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
	"github.com/nats-io/nats.go"
)

func Run(args []string) error {
	fs := flag.NewFlagSet("repl-v3-run", flag.ContinueOnError)
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL")
	room := fs.String("room", defaultRoom, "Shared room name")
	name := fs.String("name", DefaultPromptName(), "Prompt name for this client")
	if err := fs.Parse(args); err != nil {
		return err
	}
	wasReachable := endpointReachable(strings.TrimSpace(*natsURL), 700*time.Millisecond)
	if err := EnsureLeaderRunning(strings.TrimSpace(*natsURL), strings.TrimSpace(*room)); err != nil {
		return err
	}
	if bootstrapHTTPAutostartEnabled() {
		host := strings.TrimSpace(os.Getenv("DIALTONE_REPL_BOOTSTRAP_HTTP_HOST"))
		if host == "" {
			host = "127.0.0.1"
		}
		port := 8811
		if raw := strings.TrimSpace(os.Getenv("DIALTONE_REPL_BOOTSTRAP_HTTP_PORT")); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
				port = parsed
			}
		}
		if err := EnsureBootstrapHTTPRunning(host, port); err != nil {
			return err
		}
		logs.Info("bootstrap installer available at http://%s:%d/install.sh", host, port)
	}
	startupMode := "autostarted"
	if wasReachable {
		startupMode = "connected"
	}
	logREPLRunStartupState(strings.TrimSpace(*natsURL), strings.TrimSpace(*room), startupMode)
	joinArgs := []string{
		"--nats-url", strings.TrimSpace(*natsURL),
		"--room", strings.TrimSpace(*room),
		"--name", strings.TrimSpace(*name),
	}
	return RunJoin(joinArgs)
}

func bootstrapHTTPAutostartEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("DIALTONE_REPL_BOOTSTRAP_HTTP_AUTOSTART")))
	switch v {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func RunInstall(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "version")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logs.Info("repl src_v3 install: verifying Go toolchain at %s", goBin)
	return cmd.Run()
}

func RunWatch(args []string) error {
	fs := flag.NewFlagSet("repl-v3-watch", flag.ContinueOnError)
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL")
	subject := fs.String("subject", "repl.>", "NATS subject")
	filter := fs.String("filter", "", "Only print messages containing this text")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := EnsureLeaderRunning(strings.TrimSpace(*natsURL), defaultRoom); err != nil {
		return err
	}
	nc, err := nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(1500*time.Millisecond))
	if err != nil {
		return err
	}
	defer nc.Close()

	f := strings.TrimSpace(*filter)
	sub, err := nc.Subscribe(strings.TrimSpace(*subject), func(m *nats.Msg) {
		line := strings.TrimSpace(string(m.Data))
		if f != "" && !strings.Contains(line, f) {
			return
		}
		logs.Raw("[%s] %s", strings.TrimSpace(m.Subject), line)
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	logs.Info("watching NATS subject %q on %s", strings.TrimSpace(*subject), strings.TrimSpace(*natsURL))
	select {}
}

func RunFormat(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "fmt", "./plugins/repl/...")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunBuild(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "build",
		"./plugins/repl/scaffold",
		"./plugins/repl/src_v3/go/repl",
		"./plugins/repl/src_v3/test/cmd",
	)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunLint(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "vet", "./plugins/repl/...")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunCheck(args []string) error {
	_, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin, err := resolveGoBin()
	if err != nil {
		return err
	}
	cmd := exec.Command(goBin, "test",
		"./plugins/repl/src_v3/go/repl",
		"./plugins/repl/scaffold",
	)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunBootstrap(args []string) error {
	fs := flag.NewFlagSet("repl-v3-bootstrap", flag.ContinueOnError)
	apply := fs.Bool("apply", false, "Apply host onboarding changes")
	wslHost := fs.String("wsl-host", "wsl.shad-artichoke.ts.net", "WSL host DNS name")
	wslUser := fs.String("wsl-user", "user", "WSL ssh username")
	if err := fs.Parse(args); err != nil {
		return err
	}
	logs.Raw("DIALTONE v3 bootstrap guide")
	logs.Raw("  1. ./dialtone.sh")
	logs.Raw("  2. ./dialtone.sh repl src_v3 inject --user llm-codex repl src_v3 bootstrap --apply --wsl-host %s --wsl-user %s", strings.TrimSpace(*wslHost), strings.TrimSpace(*wslUser))
	logs.Raw("  3. ./dialtone.sh repl src_v3 inject --user llm-codex ssh src_v1 run --host wsl --cmd whoami")
	if !*apply {
		logs.Raw("  (dry-run) pass --apply to create/update mesh host entry named 'wsl'")
		return nil
	}
	return AddHost([]string{
		"--name", "wsl",
		"--host", strings.TrimSpace(*wslHost),
		"--user", strings.TrimSpace(*wslUser),
		"--port", "22",
		"--os", "linux",
		"--alias", "wsl",
		"--route", "tailscale,private",
	})
}

func logREPLRunStartupState(natsURL, room, mode string) {
	hostName := strings.TrimSpace(DefaultPromptName())
	if hostName == "" {
		hostName = "unknown"
	}
	cpuCores := runtime.NumCPU()
	memText := humanizeBytes(detectHostMemoryBytes())
	if memText == "" {
		memText = "unknown"
	}

	logs.System("Startup state:")
	logs.System("- repl version=%s host=%s os=%s arch=%s cpu_cores=%d mem_total=%s", strings.TrimSpace(BuildVersion), hostName, runtime.GOOS, runtime.GOARCH, cpuCores, memText)
	logs.System("- repl mode=%s", strings.TrimSpace(mode))
	logs.System("- room=%s nats=%s reachable=%t", strings.TrimSpace(room), strings.TrimSpace(natsURL), endpointReachable(strings.TrimSpace(natsURL), 700*time.Millisecond))

	pid := replLeaderPID()
	if pid <= 0 {
		logs.System("- repl leader pid=<none>")
	} else {
		logs.System("- repl leader pid=%d", pid)
		cpuPct, rssKB, etime := replProcessStats(pid)
		rssText := "-"
		if rssKB > 0 {
			rssText = humanizeBytes(uint64(rssKB) * 1024)
		}
		logs.System("  cpu=%s%% rss=%s etime=%s", cpuPct, rssText, etime)
		if secs, ok := parsePSElapsed(etime); ok && secs > 0 {
			logs.System("- repl uptime=%s", formatUptime(secs))
		}
	}

	if active, provider, tailnet := tsnetv1.NativeTailnetConnected(); active {
		logs.System("- native tailscale active=true provider=%s tailnet=%s", provider, strings.TrimSpace(tailnet))
	} else {
		logs.System("- native tailscale active=false")
	}
	if cfg, err := tsnetv1.ResolveConfig(hostName, ""); err == nil {
		logs.System("- tsnet config hostname=%s tailnet=%s auth_key=%t api_key=%t", strings.TrimSpace(cfg.Hostname), strings.TrimSpace(cfg.Tailnet), cfg.AuthKeyPresent, cfg.APIKeyPresent)
	}
}

func replLeaderPID() int {
	if runtime.GOOS == "windows" {
		return 0
	}
	patterns := []string{
		`plugins/repl/scaffold/main.go src_v3 leader`,
		`src_v3 leader --embedded-nats`,
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, 2)
	for _, p := range patterns {
		cmd := exec.Command("pgrep", "-f", p)
		raw, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			pid, err := strconv.Atoi(line)
			if err != nil || pid <= 0 {
				continue
			}
			if _, ok := seen[pid]; ok {
				continue
			}
			seen[pid] = struct{}{}
			out = append(out, pid)
		}
	}
	if len(out) == 0 {
		return 0
	}
	sort.Ints(out)
	// Prefer the actual leader binary over the go run wrapper.
	for i := len(out) - 1; i >= 0; i-- {
		pid := out[i]
		if !isGoRunWrapperPID(pid) {
			return pid
		}
	}
	return out[len(out)-1]
}

func isGoRunWrapperPID(pid int) bool {
	if pid <= 0 || runtime.GOOS == "windows" {
		return false
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	cmdline := strings.TrimSpace(string(out))
	return strings.Contains(cmdline, "go run") && strings.Contains(cmdline, "plugins/repl/scaffold/main.go")
}

func replProcessStats(pid int) (cpuPct string, rssKB int, etime string) {
	cpuPct = "-"
	etime = "-"
	if pid <= 0 || runtime.GOOS == "windows" {
		return cpuPct, 0, etime
	}
	if out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "pcpu=").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			cpuPct = v
		}
	}
	if out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "rss=").Output(); err == nil {
		if parsed, perr := strconv.Atoi(strings.TrimSpace(string(out))); perr == nil && parsed > 0 {
			rssKB = parsed
		}
	}
	if out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "etime=").Output(); err == nil {
		v := strings.TrimSpace(string(out))
		if v != "" {
			etime = v
		}
	}
	return cpuPct, rssKB, etime
}

func detectHostMemoryBytes() uint64 {
	switch runtime.GOOS {
	case "darwin":
		if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			if v, perr := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); perr == nil {
				return v
			}
		}
	case "linux":
		if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				line = strings.TrimSpace(line)
				if !strings.HasPrefix(line, "MemTotal:") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, perr := strconv.ParseUint(fields[1], 10, 64); perr == nil {
						return kb * 1024
					}
				}
				break
			}
		}
	}
	return 0
}

func humanizeBytes(v uint64) string {
	if v == 0 {
		return ""
	}
	const unit = 1024
	if v < unit {
		return strconv.FormatUint(v, 10) + " B"
	}
	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	if exp < 0 || exp >= len(suffixes) {
		return strconv.FormatUint(v, 10) + " B"
	}
	return fmt.Sprintf("%.1f %s", float64(v)/float64(div), suffixes[exp])
}

func parsePSElapsed(raw string) (int64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "-" {
		return 0, false
	}
	dayParts := strings.SplitN(s, "-", 2)
	days := int64(0)
	timePart := s
	if len(dayParts) == 2 {
		d, err := strconv.ParseInt(strings.TrimSpace(dayParts[0]), 10, 64)
		if err != nil || d < 0 {
			return 0, false
		}
		days = d
		timePart = strings.TrimSpace(dayParts[1])
	}
	parts := strings.Split(timePart, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	toInt := func(v string) (int64, bool) {
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil || n < 0 {
			return 0, false
		}
		return n, true
	}
	var hours, minutes, seconds int64
	if len(parts) == 2 {
		m, ok := toInt(parts[0])
		if !ok {
			return 0, false
		}
		sec, ok := toInt(parts[1])
		if !ok {
			return 0, false
		}
		minutes, seconds = m, sec
	} else {
		h, ok := toInt(parts[0])
		if !ok {
			return 0, false
		}
		m, ok := toInt(parts[1])
		if !ok {
			return 0, false
		}
		sec, ok := toInt(parts[2])
		if !ok {
			return 0, false
		}
		hours, minutes, seconds = h, m, sec
	}
	total := days*24*3600 + hours*3600 + minutes*60 + seconds
	return total, true
}

func formatUptime(seconds int64) string {
	if seconds <= 0 {
		return "0s"
	}
	d := time.Duration(seconds) * time.Second
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Second).String()
	}
	return d.Round(time.Second).String()
}
