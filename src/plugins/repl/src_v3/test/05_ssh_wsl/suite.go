package sshwsl

import (
	"fmt"
	"os"
	"strings"
	"time"

	"dialtone/dev/plugins/repl/src_v3/test/support"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "interactive-ssh-wsl-command",
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

			hostName := "wsl"
			hostAddr := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_HOST"))
			if hostAddr == "" {
				hostAddr = "127.0.0.1"
			}
			hostUser := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_USER"))
			if hostUser == "" {
				hostUser = "user"
			}
			hostPort := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_WSL_PORT"))
			if hostPort == "" {
				hostPort = "22"
			}

			addHostCmd := fmt.Sprintf("/repl src_v3 add-host --name %s --host %s --user %s", hostName, hostAddr, hostUser)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: addHostCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, addHostCmd),
					}, support.StandardSubtoneRoomPatterns("repl src_v3", "")),
					ExpectOutput: support.StandardSubtoneOutputPatterns("repl src_v3", ""),
					Timeout:      35 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			resolveCmd := "/ssh src_v1 resolve --host wsl"
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: resolveCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, resolveCmd),
						`"scope":"subtone"`,
						`Subtone for ssh src_v1 exited with code 0.`,
					}, support.StandardSubtoneRoomPatterns("ssh src_v1", "")),
					ExpectOutput: support.StandardSubtoneOutputPatterns("ssh src_v1", ""),
					Timeout:      35 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			report, err := sshv1.BuildResolveReport(hostName, sshv1.CommandOptions{})
			if err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("build ssh resolve report: %w", err)
			}
			if strings.TrimSpace(report.Name) != hostName {
				return testv1.StepRunResult{}, fmt.Errorf("expected resolve name %s, got %s", hostName, report.Name)
			}
			if strings.TrimSpace(report.PreferredHost) != hostAddr {
				return testv1.StepRunResult{}, fmt.Errorf("expected preferred host %s, got %s", hostAddr, report.PreferredHost)
			}
			if strings.TrimSpace(report.User) != hostUser {
				return testv1.StepRunResult{}, fmt.Errorf("expected user %s, got %s", hostUser, report.User)
			}
			if strings.TrimSpace(report.Port) != hostPort {
				return testv1.StepRunResult{}, fmt.Errorf("expected port %s, got %s", hostPort, report.Port)
			}
			if strings.TrimSpace(report.AuthSource) == "" || strings.TrimSpace(report.AuthSource) == "none" {
				return testv1.StepRunResult{}, fmt.Errorf("expected usable auth source, got %s", report.AuthSource)
			}
			if strings.TrimSpace(report.HostKeyMode) != "insecure-ignore" {
				return testv1.StepRunResult{}, fmt.Errorf("expected host key mode insecure-ignore, got %s", report.HostKeyMode)
			}

			probeCmd := "/ssh src_v1 probe --host wsl --timeout 5s"
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: probeCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, probeCmd),
						`"scope":"subtone"`,
						`Probe target=wsl transport=ssh user=`,
						`candidate=`,
						`auth=PASS`,
						`Subtone for ssh src_v1 exited with code 0.`,
					}, support.StandardSubtoneRoomPatterns("ssh src_v1", "")),
					ExpectOutput: support.StandardSubtoneOutputPatterns("ssh src_v1", ""),
					Timeout:      20 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("ssh probe via REPL failed before remote command run: %w", err)
			}

			sshCmd := "/ssh src_v1 run --host wsl --cmd whoami"
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: sshCmd,
					ExpectRoom: support.CombinePatterns([]string{
						fmt.Sprintf(`"message":"%s"`, sshCmd),
						`"scope":"subtone"`,
						`"message":"user"`,
						`Subtone for ssh src_v1 exited with code`,
					}, support.StandardSubtoneRoomPatterns("ssh src_v1", `Subtone for ssh src_v1 exited with code`)),
					ExpectOutput: support.StandardSubtoneOutputPatterns("ssh src_v1", `DIALTONE> Subtone for ssh src_v1 exited with code`),
					Timeout:      35 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("ssh wsl command routed through llm-codex REPL prompt path")
			return testv1.StepRunResult{
				Report: "Joined REPL as llm-codex, added the sample wsl host through the REPL prompt flow, exercised SSH resolve through the prompt, verified the SSH resolve report selected the expected host/user/port plus a usable auth source and host-key mode, confirmed reachability and auth with the REPL SSH probe, then ran `whoami` through the REPL SSH subtone and verified the remote user output.",
			}, nil
		},
	})
}
