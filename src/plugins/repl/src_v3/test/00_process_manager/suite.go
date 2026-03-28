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
				Report: "Published an external `service` heartbeat on `repl.host.legion.heartbeat.service.chrome-dev` and verified `/service-list` surfaced it in the index room as an active managed service on host `legion`.",
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
				ExpectOutput: append([]string{`llm-codex> /repl src_v3 help`}, support.StandardSubtoneOutputPatterns("repl src_v3", "dialtone> Subtone for repl src_v3 exited with code 0.")...),
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
				ExpectRoom:   []string{`"type":"input"`, `"message":"/ps"`, `"message":"Running Tasks:"`, fmt.Sprintf("%d", bgPID)},
				ExpectOutput: []string{`dialtone> Running Tasks:`, fmt.Sprintf("%d", bgPID)},
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

	r.Add(testv1.Step{
		Name:    "background-subtone-can-be-stopped-and-registry-shows-mode",
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

			bgPID, err := startBackgroundWatch(rt, "stopme")
			if err != nil {
				return testv1.StepRunResult{}, err
			}

			roomSeq, _ := rt.CurrentSeqs()
			listCmd := "/repl src_v3 subtone-list --count 20"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, listCmd),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", listCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("pre-stop subtone-list failed: %w", err)
			}
			listPID, err := rt.WaitForSubtonePIDForCommandAfter("repl src_v3 subtone-list --count 20", 20*time.Second, roomSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing pre-stop subtone-list pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.subtone.%d", listPID), 20*time.Second, []string{
				`STATE`,
				`MODE`,
				strconv.Itoa(bgPID),
				`active`,
				`background`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("pre-stop subtone-list missing background mode row for pid %d: %w", bgPID, err)
			}

			stopCmd := fmt.Sprintf("/subtone-stop --pid %d", bgPID)
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send:         stopCmd,
				ExpectRoom:   []string{fmt.Sprintf(`"message":"%s"`, stopCmd), fmt.Sprintf(`"message":"Stopping subtone-%d."`, bgPID), fmt.Sprintf(`"message":"Stopped subtone-%d."`, bgPID)},
				ExpectOutput: []string{fmt.Sprintf("llm-codex> %s", stopCmd), fmt.Sprintf("dialtone> Stopping subtone-%d.", bgPID), fmt.Sprintf("dialtone> Stopped subtone-%d.", bgPID)},
				Timeout:      20 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-stop failed for pid %d: %w", bgPID, err)
			}

			postSeq, _ := rt.CurrentSeqs()
			postListCmd := "/repl src_v3 subtone-list --count 20"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: postListCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, postListCmd),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", postListCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("post-stop subtone-list failed: %w", err)
			}
			postListPID, err := rt.WaitForSubtonePIDForCommandAfter("repl src_v3 subtone-list --count 20", 20*time.Second, postSeq)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing post-stop subtone-list pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.subtone.%d", postListPID), 20*time.Second, []string{
				`STATE`,
				`MODE`,
				strconv.Itoa(bgPID),
				`done`,
				`background`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("post-stop subtone-list missing done/background row for pid %d: %w", bgPID, err)
			}

			ctx.TestPassf("background pid %d stopped cleanly and registry preserved mode/state", bgPID)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Started background subtone pid %d, verified `subtone-list` showed it as `active background`, stopped it with `/subtone-stop --pid %d`, and then verified `subtone-list` preserved the row as `done background`.", bgPID, bgPID),
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

func startBackgroundWatch(rt *support.Runtime, filter string) (int, error) {
	roomSeq, _ := rt.CurrentSeqs()
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
	command := fmt.Sprintf("repl src_v3 watch --nats-url %s --subject repl.room.index --filter %s", rt.NATSURL, strings.TrimSpace(filter))
	pid, err := rt.WaitForSubtonePIDForCommandAfter(command, 10*time.Second, roomSeq)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("no new background subtone pid observed after %q: %w", cmd, err)
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
	roomSeq, _ := rt.CurrentSeqs()
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
	listPID, err := rt.WaitForSubtonePIDForCommandAfter("repl src_v3 subtone-list --count 50", 20*time.Second, roomSeq)
	if err != nil {
		return fmt.Errorf("missing cleanup subtone-list pid: %w", err)
	}
	out := strings.Join(rt.SubjectMessages(fmt.Sprintf("repl.subtone.%d", listPID)), "\n")
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
		if pid == listPID {
			continue
		}
		stopCmd := fmt.Sprintf("/subtone-stop --pid %d", pid)
		if err := rt.RunTranscript([]support.TranscriptStep{{
			Send:         stopCmd,
			ExpectRoom:   []string{fmt.Sprintf(`"message":"%s"`, stopCmd), fmt.Sprintf(`"message":"Stopping subtone-%d."`, pid), fmt.Sprintf(`"message":"Stopped subtone-%d."`, pid)},
			ExpectOutput: []string{fmt.Sprintf("llm-codex> %s", stopCmd), fmt.Sprintf("dialtone> Stopping subtone-%d.", pid), fmt.Sprintf("dialtone> Stopped subtone-%d.", pid)},
			Timeout:      20 * time.Second,
		}}); err != nil {
			return fmt.Errorf("stop active subtone pid %d: %w", pid, err)
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
