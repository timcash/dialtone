package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	procv1 "dialtone/dev/plugins/proc/src_v1/go/proc"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, rest, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old proc CLI order is deprecated. Use: ./dialtone.sh proc src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("Unsupported version %s", version)
		os.Exit(1)
	}

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "test":
		runTests()
	case "list", "ps":
		runList()
	case "kill":
		runKill(rest)
	case "sleep":
		runSleep(rest)
	case "emit":
		runEmit(rest)
	default:
		logs.Error("Unknown proc command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh proc src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first proc argument (usage: ./dialtone.sh proc src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh proc src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  test               Run proc src_v1 test suite")
	logs.Raw("  list|ps            List managed processes with metrics")
	logs.Raw("  kill <pid>         Kill a managed process by PID")
	logs.Raw("  sleep <seconds>    Sleep for N seconds (default 5)")
	logs.Raw("  emit <line...>     Echo a line")
	logs.Raw("  help               Show this help")
}

func runSleep(args []string) {
	duration := 5 * time.Second
	if len(args) > 0 {
		if d, err := strconv.Atoi(args[0]); err == nil {
			duration = time.Duration(d) * time.Second
		}
	}
	logs.Raw("Sleeping for %v...", duration)
	time.Sleep(duration)
	logs.Raw("Sleep complete.")
}

func runEmit(args []string) {
	if len(args) == 0 {
		logs.Error("Usage: ./dialtone.sh proc src_v1 emit <line>")
		os.Exit(1)
	}
	logs.Raw("%s", strings.Join(args, " "))
}

func runList() {
	items := procv1.ListManagedProcesses()
	if len(items) == 0 {
		logs.Raw("No active managed processes.")
		return
	}
	logs.Raw(fmt.Sprintf("%-8s %-8s %-10s %-12s %-8s %s", "PID", "UPTIME", "CPU%", "MEM", "PORTS", "COMMAND"))
	for _, p := range items {
		logs.Raw(fmt.Sprintf("%-8d %-8s %-10.1f %-12s %-8d %s", p.PID, p.StartedAgo, p.CPUPercent, formatBytes(p.MemRSS), p.PortCount, p.Command))
	}
}

func runKill(args []string) {
	if len(args) < 1 {
		logs.Error("Usage: ./dialtone.sh proc src_v1 kill <pid>")
		os.Exit(1)
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil || pid <= 0 {
		logs.Error("invalid pid: %s", args[0])
		os.Exit(1)
	}
	if err := procv1.KillManagedProcess(pid); err != nil {
		logs.Error("failed to kill pid %d: %v", pid, err)
		os.Exit(1)
	}
	logs.Raw("Killed managed process %d.", pid)
}

func runTests() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Error("%v", err)
		os.Exit(1)
	}
	cmd := exec.Command("go", "run", "./plugins/proc/src_v1/test/cmd/main.go")
	cmd.Dir = filepath.Join(repoRoot, "src")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
