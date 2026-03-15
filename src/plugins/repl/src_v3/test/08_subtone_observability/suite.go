package subtoneobservability

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "subtone-list-and-log-match-real-command",
		Timeout: 180 * time.Second,
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

			hostName := "obs"
			hostAddr := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_HOST"))
			if hostAddr == "" {
				hostAddr = "127.0.0.1"
			}
			hostUser := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_USER"))
			if hostUser == "" {
				hostUser = "user"
			}

			cmdText := fmt.Sprintf("repl src_v3 add-host --name %s --host %s --user %s", hostName, hostAddr, hostUser)
			addHostCmd := "/" + cmdText
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: addHostCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, addHostCmd),
					fmt.Sprintf(`Verified mesh host %s persisted`, hostName),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", addHostCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      45 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl add-host observability setup failed: %w", err)
			}
			pid, err := rt.LatestSubtonePID()
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing add-host subtone pid: %w", err)
			}

			listCmd := "/repl src_v3 subtone-list --count 20"
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: listCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, listCmd),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", listCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl subtone-list failed: %w", err)
			}
			listPID, err := rt.WaitForSubtonePIDForCommand("repl src_v3 subtone-list --count 20", 20*time.Second)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing subtone-list pid: %w", err)
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.subtone.%d", listPID), 20*time.Second, []string{
				`STATE`,
				strconv.Itoa(pid),
				`repl src_v3 add-host`,
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-list output did not expose add-host pid %d: %w", pid, err)
			}

			logCmd := fmt.Sprintf("/repl src_v3 subtone-log --pid %d --lines 200", pid)
			if err := rt.RunTranscript([]support.TranscriptStep{{
				Send: logCmd,
				ExpectRoom: support.CombinePatterns([]string{
					fmt.Sprintf(`"message":"%s"`, logCmd),
				}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
				ExpectOutput: append([]string{fmt.Sprintf("llm-codex> %s", logCmd)}, support.StandardSubtoneOutputPatterns("repl src_v3", "")...),
				Timeout:      30 * time.Second,
			}}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("repl subtone-log failed for pid %d: %w", pid, err)
			}
			logPID, err := rt.WaitForSubtonePIDForCommand(fmt.Sprintf("repl src_v3 subtone-log --pid %d --lines 200", pid), 20*time.Second)
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("missing subtone-log pid: %w", err)
			}
			requiredLogBits := []string{
				"Subtone log:",
				"args=",
				"repl",
				"src_v3",
				"add-host",
				hostName,
			}
			if err := rt.WaitForSubjectPatterns(fmt.Sprintf("repl.subtone.%d", logPID), 20*time.Second, requiredLogBits); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-log output for pid %d missing required bits: %w", pid, err)
			}

			ctx.TestPassf("subtone-list and subtone-log resolved pid %d for %s", pid, cmdText)
			return testv1.StepRunResult{
				Report: "Ran a real add-host subtone as llm-codex, verified the standard DIALTONE subtone lifecycle in the REPL room, then used subtone-list and subtone-log --pid to map the PID back to the exact command log.",
			}, nil
		},
	})
}
