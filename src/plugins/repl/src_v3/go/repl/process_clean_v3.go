package repl

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunProcessClean(args []string) error {
	fs := flag.NewFlagSet("repl-v3-process-clean", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "List matching processes without killing them")
	includeChrome := fs.Bool("include-chrome", false, "Also kill chrome-v1 service processes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	type pattern struct {
		label string
		expr  string
	}
	patterns := []pattern{
		{label: "repl-v3-run-go", expr: `go run dev\.go repl src_v3 run`},
		{label: "repl-v3-run", expr: `plugins/repl/scaffold/main.go src_v3 run`},
		{label: "repl-v3-watch-go", expr: `go run dev\.go repl src_v3 watch`},
		{label: "repl-v3-watch-bin", expr: `src_v3 watch --nats-url`},
		{label: "repl-v3-join-go", expr: `go run dev\.go repl src_v3 join`},
		{label: "repl-v3-join-bin", expr: `src_v3 join --nats-url`},
		{label: "repl-v3-leader-go", expr: `go run dev\.go repl src_v3 leader`},
		{label: "repl-v3-leader", expr: `plugins/repl/scaffold/main.go src_v3 leader`},
		{label: "repl-v3-leader-bin", expr: `src_v3 leader --embedded-nats`},
		{label: "repl-v3-bootstrap-http", expr: `plugins/repl/scaffold/main.go src_v3 bootstrap-http`},
		{label: "repl-v3-bootstrap-http-bin", expr: `src_v3 bootstrap-http --host`},
		{label: "dialtone-tap", expr: `dialtone-tap`},
		{label: "cloudflare-shell-up", expr: `go run dev\.go cloudflare src_v1 shell up`},
		{label: "cloudflare-tunnel-runner", expr: `go run dev\.go cloudflare src_v1 tunnel (start|run)`},
		{label: "cloudflared-tunnel", expr: `cloudflared.*tunnel run`},
		{label: "stuck-tsnet-shell", expr: `dialtone\.sh tsnet src_v1 up`},
		{label: "stuck-tsnet-go", expr: `go run dev\.go tsnet src_v1 up`},
	}
	if *includeChrome {
		patterns = append(patterns,
			pattern{label: "chrome-v1-service", expr: `/tmp/dialtone/chrome-v1/chrome-v1-service`},
			pattern{label: "chrome-v1-role", expr: `--dialtone-role=chrome-v1-service`},
		)
	}

	totalFound := 0
	totalKilled := 0
	for _, p := range patterns {
		found, err := pgrepCount(p.expr)
		if err != nil {
			return err
		}
		if found == 0 {
			continue
		}
		totalFound += found
		if *dryRun {
			logs.Info("process-clean dry-run: %s matched %d process(es)", p.label, found)
			continue
		}
		killed, err := pkillCount(p.expr)
		if err != nil {
			return err
		}
		totalKilled += killed
		logs.Info("process-clean: %s killed %d process(es)", p.label, killed)
	}

	if *dryRun {
		logs.Info("process-clean dry-run complete: %d process(es) matched", totalFound)
		agentFound, _, err := stopKnownLaunchAgents(true)
		if err != nil {
			return err
		}
		if agentFound > 0 {
			logs.Info("process-clean dry-run launch agents: %d matched", agentFound)
		}
		return nil
	}

	agentFound, agentStopped, err := stopKnownLaunchAgents(false)
	if err != nil {
		return err
	}
	if agentFound > 0 {
		logs.Info("process-clean launch agents: %d matched, %d stopped/disabled", agentFound, agentStopped)
	}
	logs.Info("process-clean complete: %d processes matched, %d killed", totalFound, totalKilled)
	return nil
}

func pgrepCount(expr string) (int, error) {
	cmd := exec.Command("pgrep", "-f", expr)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return 0, nil
		}
		return 0, err
	}
	return countNonEmptyLines(string(out)), nil
}

func pkillCount(expr string) (int, error) {
	before, err := pgrepCount(expr)
	if err != nil {
		return 0, err
	}
	if before == 0 {
		return 0, nil
	}
	cmd := exec.Command("pkill", "-f", expr)
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return 0, nil
		}
		return 0, err
	}
	after, err := pgrepCount(expr)
	if err != nil {
		return 0, err
	}
	if before < after {
		return 0, nil
	}
	return before - after, nil
}

func countNonEmptyLines(raw string) int {
	n := 0
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

func stopKnownLaunchAgents(dryRun bool) (found int, stopped int, err error) {
	if runtime.GOOS != "darwin" {
		return 0, 0, nil
	}
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return 0, 0, homeErr
	}
	uid := strconv.Itoa(os.Getuid())
	specs := []launchAgentSpec{
		{label: "dev.dialtone.dialtone_tsnet", plistRel: filepath.Join("Library", "LaunchAgents", "dev.dialtone.dialtone_tsnet.plist")},
	}

	for _, spec := range specs {
		plistPath := filepath.Join(home, spec.plistRel)
		active := launchctlLabelKnown(uid, spec.label)
		_, statErr := os.Stat(plistPath)
		plistExists := statErr == nil
		if !active && !plistExists {
			continue
		}
		found++
		if dryRun {
			logs.Info("process-clean dry-run launch agent: %s (active=%t plist=%t)", spec.label, active, plistExists)
			continue
		}

		_ = exec.Command("launchctl", "bootout", "gui/"+uid+"/"+spec.label).Run()
		if plistExists {
			_ = exec.Command("launchctl", "unload", "-w", plistPath).Run()
		}
		_ = exec.Command("launchctl", "disable", "gui/"+uid+"/"+spec.label).Run()

		stopped++
		logs.Info("process-clean launch agent stopped: %s", spec.label)
	}
	return found, stopped, nil
}

func launchctlLabelKnown(uid, label string) bool {
	cmd := exec.Command("launchctl", "print", "gui/"+uid+"/"+label)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}
