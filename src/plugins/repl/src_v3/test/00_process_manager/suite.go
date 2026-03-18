package processmanager

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

type leaderState struct {
	PID           int    `json:"pid"`
	NATSURL       string `json:"nats_url"`
	Room          string `json:"room"`
	ServerID      string `json:"server_id"`
	Running       bool   `json:"running"`
	LastHealthyAt string `json:"last_healthy_at"`
}

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "shell-routed-command-autostarts-leader-when-missing",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewIsolatedRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			rt.NATSURL = allocateLocalNATSURL()

			cleanOut, cleanErr := rt.RunDialtone("repl", "src_v3", "process-clean")
			if cleanErr != nil && strings.TrimSpace(cleanOut) == "" {
				return testv1.StepRunResult{}, fmt.Errorf("pre-test process-clean failed: %w", cleanErr)
			}

			out, err := rt.RunDialtone("proc", "src_v1", "emit", "shell-autostart-ok")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("shell routed proc emit failed: %w\n%s", err, out)
			}
			required := []string{
				"Request received. Spawning subtone for proc src_v1",
				"Subtone started as pid ",
				"Subtone room: subtone-",
				"Subtone log file: ",
				"Subtone for proc src_v1 exited with code 0.",
			}
			for _, needle := range required {
				if !strings.Contains(out, needle) {
					return testv1.StepRunResult{}, fmt.Errorf("shell autostart output missing %q\n%s", needle, out)
				}
			}
			if strings.Contains(out, "\nDIALTONE> shell-autostart-ok") || strings.Contains(out, "\nshell-autostart-ok\n") {
				return testv1.StepRunResult{}, fmt.Errorf("shell routed output leaked subtone payload into index/shell output\n%s", out)
			}
			logPath, err := parseSubtoneLogPath(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			logBody, err := os.ReadFile(logPath)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("read subtone log %s: %w", logPath, err)
			}
			if !strings.Contains(string(logBody), "shell-autostart-ok") {
				return testv1.StepRunResult{}, fmt.Errorf("subtone log missing emitted payload\n%s", string(logBody))
			}
			st, err := readLeaderState(rt.RepoRoot)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if st.PID <= 0 || !st.Running {
				return testv1.StepRunResult{}, fmt.Errorf("leader state invalid after shell autostart: %+v", st)
			}

			ctx.TestPassf("shell routed command autostarted leader pid %d and kept payload in subtone log", st.PID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Ran `./dialtone.sh proc src_v1 emit shell-autostart-ok` against a fresh local NATS URL, verified the shell path produced the normal routed subtone lifecycle, wrote leader pid %d to `leader.json`, and kept the emitted payload in the subtone log instead of leaking it into shell/index output.", st.PID),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "leader-state-file-persists-and-startleader-reuses-worker",
		Timeout: 90 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			first, err := readLeaderState(rt.RepoRoot)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if first.PID <= 0 || !first.Running {
				return testv1.StepRunResult{}, fmt.Errorf("leader state invalid after first start: %+v", first)
			}
			if strings.TrimSpace(first.NATSURL) == "" {
				return testv1.StepRunResult{}, fmt.Errorf("leader state missing nats url: %+v", first)
			}
			if !strings.Contains(strings.TrimSpace(first.NATSURL), ":46222") {
				return testv1.StepRunResult{}, fmt.Errorf("leader state nats url missing expected port: %+v", first)
			}
			if strings.TrimSpace(first.Room) != "index" {
				return testv1.StepRunResult{}, fmt.Errorf("leader state room mismatch: %+v", first)
			}

			time.Sleep(300 * time.Millisecond)
			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			second, err := readLeaderState(rt.RepoRoot)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if second.PID != first.PID {
				return testv1.StepRunResult{}, fmt.Errorf("StartLeader reused a different worker pid: first=%d second=%d", first.PID, second.PID)
			}
			if !second.Running {
				return testv1.StepRunResult{}, fmt.Errorf("leader state not running after second start: %+v", second)
			}

			ctx.TestPassf("leader state persisted and StartLeader reused pid %d", first.PID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started the shared REPL leader, verified `.dialtone/repl-v3/leader.json` was written with pid %d and the expected NATS URL, then called StartLeader again and confirmed the same worker pid was reused instead of restarting the leader.", first.PID),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "shell-routed-command-reuses-running-leader",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewIsolatedRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()
			rt.NATSURL = allocateLocalNATSURL()

			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			first, err := readLeaderState(rt.RepoRoot)
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			out, err := rt.RunDialtone("proc", "src_v1", "emit", "shell-reuse-ok")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("shell routed proc emit failed with running leader: %w\n%s", err, out)
			}
			if strings.Contains(out, "No REPL leader detected on") {
				return testv1.StepRunResult{}, fmt.Errorf("shell path unexpectedly autostarted a new leader despite existing healthy leader\n%s", out)
			}
			required := []string{
				"Request received. Spawning subtone for proc src_v1",
				"Subtone started as pid ",
				"Subtone room: subtone-",
				"Subtone log file: ",
				"Subtone for proc src_v1 exited with code 0.",
			}
			for _, needle := range required {
				if !strings.Contains(out, needle) {
					return testv1.StepRunResult{}, fmt.Errorf("shell reuse output missing %q\n%s", needle, out)
				}
			}
			if strings.Contains(out, "\nDIALTONE> shell-reuse-ok") || strings.Contains(out, "\nshell-reuse-ok\n") {
				return testv1.StepRunResult{}, fmt.Errorf("shell routed output leaked subtone payload into index/shell output\n%s", out)
			}
			second, err := readLeaderState(rt.RepoRoot)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if first.PID != second.PID {
				return testv1.StepRunResult{}, fmt.Errorf("leader pid changed across routed shell command: before=%d after=%d", first.PID, second.PID)
			}

			ctx.TestPassf("shell routed command reused existing leader pid %d", first.PID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started the REPL leader first, then ran `./dialtone.sh proc src_v1 emit shell-reuse-ok` and verified the shell path reused leader pid %d without printing a new autostart message while still routing the command into a subtone.", first.PID),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "background-subtone-does-not-block-later-foreground-command",
		Timeout: 120 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			rt, err := support.NewRuntime(ctx)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer rt.Stop()

			if err := rt.StartLeader(); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			bgPID, err := startBackgroundWatch(rt, "pm")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			roomSeq, _ := rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/repl src_v3 help",
				ExpectRoom:   support.StandardSubtoneRoomPatterns("repl src_v3", ""),
				ExpectOutput: append([]string{`llm-codex> /repl src_v3 help`}, support.StandardSubtoneOutputPatterns("repl src_v3", "DIALTONE> Subtone for repl src_v3 exited with code 0.")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("foreground help command failed while background pid %d was active: %w", bgPID, err)
			}
			helpPID, err := rt.WaitForSubtonePIDForCommandAfter("repl src_v3 help", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing foreground help subtone pid after background start: %w", err)
			}
			if helpPID == bgPID {
				return testv1.StepRunResult{}, fmt.Errorf("foreground help reused background pid %d unexpectedly", bgPID)
			}
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"type":"input"`, `"message":"/ps"`, `"message":"Active Subtones:"`, fmt.Sprintf(`"message":"%-8d`, bgPID)},
				ExpectOutput: []string{`DIALTONE> Active Subtones:`, fmt.Sprintf("%d", bgPID)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("background pid %d stopped being visible after foreground help: %w", bgPID, err)
			}
			if err := cleanupManagedSubtones(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("background pid %d stayed active while foreground help subtone pid %d completed", bgPID, helpPID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started a background REPL watch subtone as pid %d, then ran `/repl src_v3 help` as a new foreground subtone pid %d and verified the later foreground command still completed cleanly before cleaning active managed subtones.", bgPID, helpPID),
			}, nil
		},
	})
}

func readLeaderState(repoRoot string) (leaderState, error) {
	var st leaderState
	path := filepath.Join(strings.TrimSpace(repoRoot), ".dialtone", "repl-v3", "leader.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return st, fmt.Errorf("read leader state %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, fmt.Errorf("parse leader state %s: %w", path, err)
	}
	return st, nil
}

func startBackgroundWatch(rt *support.Runtime, filter string) (int, error) {
	prevPID, _ := rt.LatestSubtonePID()
	cmd := fmt.Sprintf("/repl src_v3 watch --nats-url %s --subject repl.room.index --filter %s &", rt.NATSURL, strings.TrimSpace(filter))
	if err := rt.SendJoinLine(cmd); err != nil {
		return 0, err
	}
	if err := rt.WaitForPatterns(20*time.Second, []string{
		fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
		`"scope":"index"`,
		`Request received. Spawning subtone for repl src_v3`,
		`Subtone started as pid `,
		`Subtone room: subtone-`,
		`Subtone log file: `,
		`Subtone for repl src_v3 is running in background.`,
	}); err != nil {
		return 0, fmt.Errorf("background watch subtone did not start cleanly: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	pid := 0
	for time.Now().Before(deadline) {
		nextPID, err := rt.LatestSubtonePID()
		if err == nil && nextPID > 0 && nextPID != prevPID {
			pid = nextPID
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	if pid <= 0 {
		return 0, fmt.Errorf("no new background subtone pid observed after %q", cmd)
	}
	if err := waitForSubjectContains(rt, fmt.Sprintf("repl.subtone.%d", pid), 10*time.Second, []string{
		`"scope":"subtone"`,
		`watching NATS subject`,
		fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
	}); err != nil {
		return 0, err
	}
	return pid, nil
}

func cleanupManagedSubtones(rt *support.Runtime) error {
	listCmd := "/repl src_v3 subtone-list --count 50"
	if err := rt.RunTranscript([]support.TranscriptStep{{
		Send: listCmd,
		ExpectRoom: support.CombinePatterns([]string{
			fmt.Sprintf(`"message":"%s"`, listCmd),
		}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
		ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", listCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
		Timeout:      30 * time.Second,
	}}); err != nil {
		return fmt.Errorf("repl subtone-list cleanup failed: %w", err)
	}
	listPID, err := rt.WaitForSubtonePIDForCommand("repl src_v3 subtone-list --count 50", 20*time.Second)
	if err != nil {
		return fmt.Errorf("missing cleanup subtone-list pid: %w", err)
	}
	out := strings.Join(rt.SubjectMessages(fmt.Sprintf("repl.subtone.%d", listPID)), "\n")
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		pidText := strings.TrimSpace(fields[0])
		stateText := strings.TrimSpace(fields[2])
		if pidText == "" || stateText != "active" {
			continue
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 {
			continue
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("find active subtone pid %s: %w", pidText, err)
		}
		if err := proc.Kill(); err != nil {
			return fmt.Errorf("kill active subtone pid %s: %w", pidText, err)
		}
	}
	return nil
}

func waitForSubjectContains(rt *support.Runtime, subject string, timeout time.Duration, needles []string) error {
	deadline := time.Now().Add(timeout)
	for {
		body := strings.Join(rt.SubjectMessages(subject), "\n")
		missing := make([]string, 0, len(needles))
		for _, needle := range needles {
			if !strings.Contains(body, needle) {
				missing = append(missing, needle)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("subject %s missing %s\n%s", subject, strings.Join(missing, ", "), body)
		}
		time.Sleep(120 * time.Millisecond)
	}
}

func parseSubtoneLogPath(output string) (string, error) {
	for _, line := range strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "DIALTONE> Subtone log file: "
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if path != "" {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("subtone log path not found in shell output")
}

func allocateLocalNATSURL() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "nats://127.0.0.1:46322"
	}
	defer ln.Close()
	addr := ln.Addr().String()
	return "nats://" + strings.TrimSpace(addr)
}
