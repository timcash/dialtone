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
			roomPatterns := support.CombinePatterns(
				[]string{
					`"type":"input"`,
					`"from":"llm-codex"`,
					fmt.Sprintf(`"message":"/%s"`, cmdText),
					fmt.Sprintf(`Verified mesh host %s persisted`, hostName),
				},
				support.StandardSubtoneRoomPatterns("repl src_v3", ""),
			)
			if err := rt.Inject("llm-codex", "repl", "src_v3", "add-host", "--name", hostName, "--host", hostAddr, "--user", hostUser); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("inject add-host observability setup failed: %w", err)
			}
			if err := rt.WaitForPatterns(45*time.Second, roomPatterns); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("inject add-host observability setup failed: %w", err)
			}
			if err := rt.WaitForOutput(45*time.Second, support.CombinePatterns(
				[]string{fmt.Sprintf("llm-codex> /%s", cmdText)},
				support.StandardSubtoneOutputPatterns("repl src_v3", ""),
			)); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("visible subtone lifecycle missing from REPL output: %w", err)
			}

			listOut, err := rt.RunDialtone("repl", "src_v3", "subtone-list", "--count", "20")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-list failed: %w\n%s", err, listOut)
			}
			if !strings.Contains(listOut, "STATE") {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-list did not return registry-backed output\n%s", listOut)
			}
			pid, err := findSubtonePID(listOut, "repl src_v3 add-host")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-list output did not expose add-host pid: %w\n%s", err, listOut)
			}

			logOut, err := rt.RunDialtone("repl", "src_v3", "subtone-log", "--pid", strconv.Itoa(pid), "--lines", "200")
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone-log failed for pid %d: %w\n%s", pid, err, logOut)
			}
			requiredLogBits := []string{
				"Subtone log:",
				"args=",
				"repl",
				"src_v3",
				"add-host",
				hostName,
			}
			for _, bit := range requiredLogBits {
				if !strings.Contains(logOut, bit) {
					return testv1.StepRunResult{}, fmt.Errorf("subtone-log output for pid %d missing %q\n%s", pid, bit, logOut)
				}
			}

			ctx.TestPassf("subtone-list and subtone-log resolved pid %d for %s", pid, cmdText)
			return testv1.StepRunResult{
				Report: "Ran a real add-host subtone as llm-codex, verified the standard DIALTONE subtone lifecycle in the REPL room, then used subtone-list and subtone-log --pid to map the PID back to the exact command log.",
			}, nil
		},
	})
}

func findSubtonePID(listOut string, commandNeedle string) (int, error) {
	lines := strings.Split(strings.ReplaceAll(listOut, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "DIALTONE>") || strings.HasPrefix(line, "PID ") {
			continue
		}
		if !strings.Contains(line, commandNeedle) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err == nil && pid > 0 {
			return pid, nil
		}
	}
	return 0, fmt.Errorf("no subtone-list row matched %q", commandNeedle)
}
