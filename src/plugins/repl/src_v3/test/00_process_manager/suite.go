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

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

type leaderState struct {
	PID           int    `json:"pid"`
	NATSURL       string `json:"nats_url"`
	Topic         string `json:"topic"`
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
			if err := support.MatchAnyPatternGroupText(out, [][]string{support.StandardShellTaskOutputPatterns()}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("shell autostart output missed the routed lifecycle contract: %w\n%s", err, out)
			}
			if strings.Contains(out, "\ndialtone> shell-autostart-ok") || strings.Contains(out, "\nshell-autostart-ok\n") {
				return testv1.StepRunResult{}, fmt.Errorf("shell routed output leaked subtone payload into index/shell output\n%s", out)
			}
			for _, forbidden := range []string{
				`assigned pid `,
				`exited with code `,
				` is running in background.`,
				`Service `,
			} {
				if strings.Contains(out, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("shell routed output should return before later lifecycle line %q\n%s", forbidden, out)
				}
			}
			logPath, err := support.ParseWorkLogPath(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if base := filepath.Base(logPath); !strings.HasPrefix(base, "task-") {
				return testv1.StepRunResult{}, fmt.Errorf("expected task-first log name, got %s", logPath)
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

			ctx.TestPassf("shell routed command autostarted leader pid %d and kept payload in the routed work log", st.PID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Ran `./dialtone.sh proc src_v1 emit shell-autostart-ok` against a fresh local NATS URL, verified the shell path produced the normal routed command lifecycle, wrote leader pid %d to `leader.json`, and kept the emitted payload in the routed work log instead of leaking it into shell/index output.", st.PID),
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
			firstTopic := strings.TrimSpace(first.Topic)
			if firstTopic == "" {
				firstTopic = strings.TrimSpace(first.Room)
			}
			if firstTopic != "index" {
				return testv1.StepRunResult{}, fmt.Errorf("leader state topic mismatch: %+v", first)
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
				Report: fmt.Sprintf("Started the shared REPL leader, verified `%s` was written with pid %d and the expected NATS URL, then called StartLeader again and confirmed the same worker pid was reused instead of restarting the leader.", filepath.Join(configv1.DefaultDialtoneHome(), "repl-v3", "leader.json"), first.PID),
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
			if err := support.MatchAnyPatternGroupText(out, [][]string{support.StandardShellTaskOutputPatterns()}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("shell reuse output missed the routed lifecycle contract: %w\n%s", err, out)
			}
			if strings.Contains(out, "\ndialtone> shell-reuse-ok") || strings.Contains(out, "\nshell-reuse-ok\n") {
				return testv1.StepRunResult{}, fmt.Errorf("shell routed output leaked subtone payload into index/shell output\n%s", out)
			}
			for _, forbidden := range []string{
				`assigned pid `,
				`exited with code `,
				` is running in background.`,
				`Service `,
			} {
				if strings.Contains(out, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("shell routed output should return before later lifecycle line %q\n%s", forbidden, out)
				}
			}
			logPath, err := support.ParseWorkLogPath(out)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if base := filepath.Base(logPath); !strings.HasPrefix(base, "task-") {
				return testv1.StepRunResult{}, fmt.Errorf("expected task-first log name, got %s", logPath)
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
				Report: fmt.Sprintf("Started the REPL leader first, then ran `./dialtone.sh proc src_v1 emit shell-reuse-ok` and verified the shell path reused leader pid %d without printing a new autostart message while still routing the command through the indexed lifecycle.", first.PID),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "shell-foreground-query-autostarts-leader-and-prints-direct-output",
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

			out, err := rt.RunDialtone("proc", "src_v1", "ps")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("foreground proc ps failed: %w\n%s", err, out)
			}
			for _, expected := range []string{
				"No active managed processes.",
			} {
				if !strings.Contains(out, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("foreground proc ps output missing %q\n%s", expected, out)
				}
			}
			for _, forbidden := range []string{
				"Request received.",
				"Task queued as task-",
				"Task topic: task.task-",
				"Task log: ",
				"To view the last 10 log lines:",
			} {
				if strings.Contains(out, forbidden) {
					return testv1.StepRunResult{}, fmt.Errorf("foreground proc ps unexpectedly queued through task transcript via %q\n%s", forbidden, out)
				}
			}
			st, err := readLeaderState(rt.RepoRoot)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if st.PID <= 0 || !st.Running {
				return testv1.StepRunResult{}, fmt.Errorf("leader state invalid after foreground query autostart: %+v", st)
			}

			ctx.TestPassf("foreground proc query autostarted leader pid %d and printed direct output", st.PID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Ran `./dialtone.sh proc src_v1 ps` against a fresh local NATS URL, verified the shell query path autostarted leader pid %d, and confirmed the command stayed foreground with direct process output instead of the queued task transcript.", st.PID),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "service-start-publishes-heartbeat-and-service-registry-state",
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

			serviceName := "pm-svc"
			roomSeq, _ := rt.CurrentSeqs()
			startCmd := "/service-start --name pm-svc -- proc src_v1 sleep 30"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: startCmd,
				ExpectRoom: []string{
					fmt.Sprintf(`"message":"%s"`, startCmd),
					`"message":"Request received."`,
					`"message":"Task queued as task-`,
					`"message":"Task topic: task.task-`,
					`"message":"Task log: `,
					`"message":"Task task-`,
					`assigned pid `,
					`"message":"Service pm-svc is running."`,
				},
				ExpectOutput: []string{
					fmt.Sprintf("llm-codex> %s", startCmd),
					`dialtone> Request received.`,
					`dialtone> Task queued as task-`,
					`dialtone> Task topic: task.task-`,
					`dialtone> Task log: `,
					`dialtone> Task task-`,
					`assigned pid `,
					`dialtone> Service pm-svc is running.`,
				},
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("service-start transcript failed: %w", err)
			}
			servicePID, err := rt.WaitForSubtonePIDForCommandAfter("proc src_v1 sleep 30", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing service pid after start: %w", err)
			}

			listCmd := "/service-list"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: []string{
					fmt.Sprintf(`"message":"%s"`, listCmd),
					`"message":"Managed Services:"`,
					serviceName,
					strconv.Itoa(servicePID),
					`active`,
					`service`,
					`proc src_v1 sleep 30`,
				},
				ExpectOutput: []string{
					fmt.Sprintf("llm-codex> %s", listCmd),
					`dialtone> Managed Services:`,
					serviceName,
					strconv.Itoa(servicePID),
					`active`,
					`service`,
					`proc src_v1 sleep 30`,
				},
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("service-list transcript failed: %w", err)
			}

			heartbeatSubject := serviceHeartbeatSubject(rt.RepoRoot, serviceName)
			if err := rt.WaitForSubjectPatterns(heartbeatSubject, 15*time.Second, []string{
				`"kind":"service"`,
				`"name":"pm-svc"`,
				`"mode":"service"`,
				`"state":"running"`,
				fmt.Sprintf(`"pid":%d`, servicePID),
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("service heartbeat missing running payload on %s: %w", heartbeatSubject, err)
			}

			stopCmd := "/service-stop --name pm-svc"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: stopCmd,
				ExpectRoom: []string{
					fmt.Sprintf(`"message":"%s"`, stopCmd),
					fmt.Sprintf(`"message":"Stopping service %s (pid %d)."`, serviceName, servicePID),
					fmt.Sprintf(`"message":"Stopped service %s."`, serviceName),
				},
				ExpectOutput: []string{
					fmt.Sprintf("llm-codex> %s", stopCmd),
					fmt.Sprintf("dialtone> Stopping service %s (pid %d).", serviceName, servicePID),
					fmt.Sprintf("dialtone> Stopped service %s.", serviceName),
				},
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("service-stop transcript failed: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(heartbeatSubject, 15*time.Second, []string{
				`"state":"stopped"`,
				fmt.Sprintf(`"pid":%d`, servicePID),
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("service heartbeat missing stopped payload on %s: %w", heartbeatSubject, err)
			}

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: []string{
					fmt.Sprintf(`"message":"%s"`, listCmd),
					`"message":"Managed Services:"`,
					serviceName,
					strconv.Itoa(servicePID),
					`done`,
					`service`,
				},
				ExpectOutput: []string{
					fmt.Sprintf("llm-codex> %s", listCmd),
					`dialtone> Managed Services:`,
					serviceName,
					strconv.Itoa(servicePID),
					`done`,
					`service`,
				},
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("post-stop service-list failed: %w", err)
			}

			ctx.TestPassf("service %s pid %d emitted heartbeats and stayed visible in service registry", serviceName, servicePID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started named service %s as pid %d through the REPL, verified `service-list` showed it as `active service`, observed its NATS heartbeat subject transition to `running`, stopped it with `/service-stop --name %s`, observed a `stopped` heartbeat, and verified `service-list` preserved the row as `done service`.", serviceName, servicePID, serviceName),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "external-service-heartbeat-appears-in-service-list",
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
			if err := rt.StartJoin("llm-codex"); err != nil {
				return testv1.StepRunResult{}, err
			}

			nc, err := nats.Connect(rt.NATSURL, nats.Timeout(1200*time.Millisecond))
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()

			payload := map[string]any{
				"host":       "legion",
				"kind":       "service",
				"name":       "chrome-dev",
				"mode":       "service",
				"pid":        42424,
				"room":       "service:chrome-dev",
				"command":    "chrome src_v3 daemon --role dev --nats-url nats://127.0.0.1:46222 --host-id legion",
				"state":      "running",
				"log_path":   "C:/Users/test/.dialtone/bin/dialtone_chrome_v3.out.log",
				"started_at": time.Now().UTC().Add(-10 * time.Second).Format(time.RFC3339),
				"last_ok_at": time.Now().UTC().Format(time.RFC3339),
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := nc.Publish("repl.host.legion.heartbeat.service.chrome-dev", raw); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := nc.Flush(); err != nil {
				return testv1.StepRunResult{}, err
			}

			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: "/service-list",
				ExpectRoom: []string{
					`"message":"/service-list"`,
					`"message":"Managed Services:"`,
					`chrome-dev`,
					`legion`,
					`42424`,
					`active`,
					`service`,
					`chrome src_v3 daemon --role dev`,
				},
				ExpectOutput: []string{
					`llm-codex> /service-list`,
					`dialtone> Managed Services:`,
					`chrome-dev`,
					`legion`,
					`42424`,
					`active`,
					`service`,
				},
				Timeout: 20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("service-list did not surface external heartbeat: %w", err)
			}

			ctx.TestPassf("external service heartbeat for chrome-dev appeared in service-list as host legion")
			return testv1.StepRunResult{
				Report: "Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index topic as an active managed service on host `legion`.",
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "background-task-does-not-block-later-foreground-command",
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

			bgTaskID, bgPID, err := startBackgroundWatch(rt, "pm")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			roomSeq, _ := rt.CurrentSeqs()
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: "/repl src_v3 help",
				ExpectRoom: support.CombinePatterns(
					[]string{`"message":"/repl src_v3 help"`},
					support.StandardTaskRoomPatterns("exited with code 0."),
				),
				ExpectOutput: support.CombinePatterns(
					[]string{`llm-codex> /repl src_v3 help`},
					support.StandardTaskOutputPatterns("exited with code 0."),
				),
				Timeout: 30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("foreground help command failed while background pid %d was active: %w", bgPID, err)
			}
			helpTaskID, err := rt.WaitForTaskIDForCommandAfter("repl src_v3 help", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing foreground help task id after background start: %w", err)
			}
			helpPID, err := rt.WaitForTaskPIDForCommandAfter("repl src_v3 help", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing foreground help task pid after background start: %w", err)
			}
			if helpTaskID == bgTaskID {
				return testv1.StepRunResult{}, fmt.Errorf("foreground help reused background task id %s unexpectedly", bgTaskID)
			}
			if helpPID == bgPID {
				return testv1.StepRunResult{}, fmt.Errorf("foreground help reused background pid %d unexpectedly", bgPID)
			}
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         "/ps",
				ExpectRoom:   []string{`"type":"input"`, `"message":"/ps"`, `"message":"Running Tasks:"`, fmt.Sprintf("%d", bgPID)},
				ExpectOutput: []string{`dialtone> Running Tasks:`, fmt.Sprintf("%d", bgPID)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("background pid %d stopped being visible after foreground help: %w", bgPID, err)
			}
			if _, err := waitForDialtoneContains(rt, 20*time.Second,
				[]string{"repl", "src_v3", "task", "show", "--task-id", bgTaskID},
				"Task: "+bgTaskID,
				fmt.Sprintf("PID: %d", bgPID),
				"State: running",
				"Mode: background",
			); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("background task %s stopped being visible after foreground help: %w", bgTaskID, err)
			}
			if err := cleanupManagedTasks(rt); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("background task %s pid %d stayed active while foreground help task %s pid %d completed", bgTaskID, bgPID, helpTaskID, helpPID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started a background REPL watch task as task %s pid %d, then ran `/repl src_v3 help` as a new foreground task %s pid %d and verified the later foreground command still completed cleanly before cleaning active managed tasks.", bgTaskID, bgPID, helpTaskID, helpPID),
			}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "background-task-can-be-stopped-and-registry-shows-mode",
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

			bgTaskID, bgPID, err := startBackgroundWatch(rt, "stopme")
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			if _, err := waitForDialtoneContains(rt, 20*time.Second,
				[]string{"repl", "src_v3", "task", "show", "--task-id", bgTaskID},
				"Task: "+bgTaskID,
				fmt.Sprintf("PID: %d", bgPID),
				"State: running",
				"Mode: background",
			); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("pre-stop task show missing background mode row for task %s pid %d: %w", bgTaskID, bgPID, err)
			}
			if _, err := waitForDialtoneContains(rt, 20*time.Second,
				[]string{"repl", "src_v3", "task", "list", "--state", "all", "--count", "20"},
				bgTaskID,
				strconv.Itoa(bgPID),
				"running",
				"background",
			); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("pre-stop task list missing background row for task %s pid %d: %w", bgTaskID, bgPID, err)
			}

			stopOut, err := rt.RunDialtone("repl", "src_v3", "task", "kill", "--task-id", bgTaskID)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("task kill failed for %s: %w\n%s", bgTaskID, err, stopOut)
			}
			for _, expected := range []string{
				fmt.Sprintf("Stopping task %s (pid %d)", bgTaskID, bgPID),
				fmt.Sprintf("Stop signal sent to task %s.", bgTaskID),
			} {
				if !strings.Contains(stopOut, expected) {
					return testv1.StepRunResult{}, fmt.Errorf("task kill output for %s missing %q\n%s", bgTaskID, expected, stopOut)
				}
			}
			if _, err := waitForDialtoneContains(rt, 20*time.Second,
				[]string{"repl", "src_v3", "task", "show", "--task-id", bgTaskID},
				"Task: "+bgTaskID,
				fmt.Sprintf("PID: %d", bgPID),
				"State: done",
				"Mode: background",
			); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("post-stop task show missing done/background row for task %s pid %d: %w", bgTaskID, bgPID, err)
			}
			if _, err := waitForDialtoneContains(rt, 20*time.Second,
				[]string{"repl", "src_v3", "task", "list", "--state", "all", "--count", "20"},
				bgTaskID,
				strconv.Itoa(bgPID),
				"done",
				"background",
			); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("post-stop task list missing done/background row for task %s pid %d: %w", bgTaskID, bgPID, err)
			}

			ctx.TestPassf("background task %s pid %d stopped cleanly and registry preserved mode/state", bgTaskID, bgPID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started background task %s pid %d, verified `task show` and `task list` showed it as `running background`, stopped it with `task kill --task-id %s`, and then verified the registry preserved the row as `done background`.", bgTaskID, bgPID, bgTaskID),
			}, nil
		},
	})
}

func readLeaderState(repoRoot string) (leaderState, error) {
	var st leaderState
	path := filepath.Join(configv1.DefaultDialtoneHome(), "repl-v3", "leader.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return st, fmt.Errorf("read leader state %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, fmt.Errorf("parse leader state %s: %w", path, err)
	}
	return st, nil
}

func startBackgroundWatch(rt *support.Runtime, filter string) (string, int, error) {
	prevPID, _ := rt.LatestSubtonePID()
	command := fmt.Sprintf("repl src_v3 watch --nats-url %s --subject repl.topic.index --filter %s", rt.NATSURL, strings.TrimSpace(filter))
	cmd := "/" + command + " &"
	roomSeq, _ := rt.CurrentSeqs()
	if err := rt.SendJoinLine(cmd); err != nil {
		return "", 0, err
	}
	if err := rt.WaitForPatterns(20*time.Second, support.CombinePatterns(
		[]string{
			fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
		},
		support.StandardTaskRoomPatterns("is running in background."),
	)); err != nil {
		return "", 0, fmt.Errorf("background watch task did not start cleanly: %w", err)
	}
	taskID, err := rt.WaitForTaskIDForCommandAfter(command, 10*time.Second, roomSeq)
	if err != nil {
		return "", 0, fmt.Errorf("background watch task id missing for command %q: %w", command, err)
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
		return "", 0, fmt.Errorf("no new background runtime pid observed after %q", cmd)
	}
	if err := waitForSubjectContains(rt, fmt.Sprintf("repl.topic.task.%s", taskID), 10*time.Second, []string{
		`"scope":"task"`,
		`watching NATS subject`,
		fmt.Sprintf(`--filter %s`, strings.TrimSpace(filter)),
	}); err != nil {
		return "", 0, err
	}
	return taskID, pid, nil
}

func cleanupManagedTasks(rt *support.Runtime) error {
	listCmd := "/repl src_v3 task list --count 50 --state running"
	roomSeq, _ := rt.CurrentSeqs()
	if err := rt.RunTranscript([]support.TranscriptStep{{
		Send: listCmd,
		ExpectRoom: support.CombinePatterns([]string{
			fmt.Sprintf(`"message":"%s"`, listCmd),
		}, support.StandardTaskRoomPatterns("exited with code 0.")),
		ExpectOutput: support.CombinePatterns(
			[]string{fmt.Sprintf("llm-codex> %s", listCmd)},
			support.StandardTaskOutputPatterns("exited with code 0."),
		),
		Timeout: 30 * time.Second,
	}}); err != nil {
		return fmt.Errorf("repl task list cleanup failed: %w", err)
	}
	listTaskID, err := rt.WaitForTaskIDForCommandAfter("repl src_v3 task list --count 50 --state running", 20*time.Second, roomSeq)
	if err != nil {
		return fmt.Errorf("missing cleanup task list task id: %w", err)
	}
	listPID, err := rt.WaitForTaskPIDForCommandAfter("repl src_v3 task list --count 50 --state running", 20*time.Second, roomSeq)
	if err != nil {
		return fmt.Errorf("missing cleanup task list pid: %w", err)
	}
	out := strings.Join(rt.SubjectMessages(fmt.Sprintf("repl.topic.task.%s", listTaskID)), "\n")
	stoppedAny := false
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		var frame map[string]any
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			continue
		}
		msgText, ok := frame["message"].(string)
		if !ok {
			continue
		}
		fields := strings.Fields(strings.TrimSpace(msgText))
		if len(fields) < 4 {
			continue
		}
		runningTaskID := strings.TrimSpace(fields[0])
		pidText := strings.TrimSpace(fields[1])
		stateText := strings.TrimSpace(fields[3])
		if runningTaskID == "" || pidText == "" || stateText != "running" {
			continue
		}
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 {
			continue
		}
		if pid == listPID || runningTaskID == listTaskID || runningTaskID == "-" {
			continue
		}
		stopCmd := fmt.Sprintf("/repl src_v3 task kill --task-id %s", runningTaskID)
		if err := rt.RunTranscript([]support.TranscriptStep{{
			Send: stopCmd,
			ExpectRoom: support.CombinePatterns(
				[]string{fmt.Sprintf(`"message":"%s"`, stopCmd)},
				support.StandardTaskRoomPatterns("exited with code 0."),
			),
			ExpectOutput: support.CombinePatterns(
				[]string{fmt.Sprintf("llm-codex> %s", stopCmd)},
				support.StandardTaskOutputPatterns("exited with code 0."),
			),
			Timeout: 20 * time.Second,
		}}); err != nil {
			return fmt.Errorf("stop active task %s pid %d: %w", runningTaskID, pid, err)
		}
		stoppedAny = true
	}
	if stoppedAny {
		if err := rt.RunTranscript([]support.TranscriptStep{{
			Send:         "/ps",
			ExpectRoom:   []string{`"type":"input"`, `"message":"/ps"`, `"message":"No running tasks."`},
			ExpectOutput: []string{`dialtone> No running tasks.`},
			Timeout:      20 * time.Second,
		}}); err != nil {
			return fmt.Errorf("verify no running tasks after cleanup: %w", err)
		}
	}
	return nil
}

func waitForDialtoneContains(rt *support.Runtime, timeout time.Duration, args []string, patterns ...string) (string, error) {
	deadline := time.Now().Add(timeout)
	lastOut := ""
	for time.Now().Before(deadline) {
		out, err := rt.RunDialtone(args...)
		lastOut = out
		if err == nil {
			matched := true
			for _, pattern := range patterns {
				if !strings.Contains(out, pattern) {
					matched = false
					break
				}
			}
			if matched {
				return out, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastOut, fmt.Errorf("timed out waiting for dialtone output to contain %s\n%s", strings.Join(patterns, ", "), lastOut)
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

func serviceHeartbeatSubject(repoRoot, serviceName string) string {
	st, err := readLeaderState(repoRoot)
	host := "local"
	if err == nil {
		raw := strings.TrimSpace(st.ServerID)
		if idx := strings.Index(raw, "@"); idx > 0 {
			host = raw[:idx]
		} else if raw != "" {
			host = raw
		}
	}
	return fmt.Sprintf("repl.host.%s.heartbeat.service.%s", subjectTokenForTest(host), subjectTokenForTest(serviceName))
}

func subjectTokenForTest(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown"
	}
	return out
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
