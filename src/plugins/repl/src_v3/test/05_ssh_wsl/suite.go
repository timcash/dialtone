package sshwsl

import (
	"fmt"
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

			fixture := support.ResolveSSHFixture()
			hostName := fixture.Alias
			hostAddr := fixture.Host
			hostUser := fixture.User
			hostPort := fixture.Port

			addHostCmd := fmt.Sprintf("/repl src_v3 add-host --name %s --host %s --user %s", hostName, hostAddr, hostUser)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: addHostCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf(`"message":"%s"`, addHostCmd)},
						support.StandardCommandRoomPatternGroups("repl src_v3", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{fmt.Sprintf("llm-codex> %s", addHostCmd)},
						support.StandardCommandOutputPatternGroups("repl src_v3", "", "")...,
					),
					Timeout: 35 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			resolveCmd := fmt.Sprintf("/ssh src_v1 resolve --host %s", hostName)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: resolveCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, resolveCmd),
							fmt.Sprintf(`ssh resolve: resolving %s`, hostName),
							fmt.Sprintf(`ssh resolve: transport=ssh preferred=%s`, hostAddr),
						},
						support.StandardCommandRoomPatternGroups("ssh src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf("llm-codex> %s", resolveCmd),
							fmt.Sprintf("dialtone> ssh resolve: resolving %s", hostName),
							fmt.Sprintf("dialtone> ssh resolve: transport=ssh preferred=%s", hostAddr),
						},
						support.StandardCommandOutputPatternGroups("ssh src_v1", "", "")...,
					),
					Timeout: 35 * time.Second,
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

			probeCmd := fmt.Sprintf("/ssh src_v1 probe --host %s --timeout 5s", hostName)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: probeCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, probeCmd),
							fmt.Sprintf(`ssh probe: checking transport/auth for %s`, hostName),
							fmt.Sprintf(`ssh probe: transport=ssh preferred=%s`, hostAddr),
							fmt.Sprintf(`Probe target=%s transport=ssh user=`, hostName),
							`candidate=`,
							`auth=PASS`,
							fmt.Sprintf(`ssh probe: auth checks passed for %s`, hostName),
						},
						support.StandardCommandRoomPatternGroups("ssh src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf("llm-codex> %s", probeCmd),
							fmt.Sprintf("dialtone> ssh probe: checking transport/auth for %s", hostName),
							fmt.Sprintf("dialtone> ssh probe: transport=ssh preferred=%s", hostAddr),
							fmt.Sprintf("dialtone> ssh probe: auth checks passed for %s", hostName),
						},
						support.StandardCommandOutputPatternGroups("ssh src_v1", "", "")...,
					),
					Timeout: 20 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("ssh probe via REPL failed before remote command run: %w", err)
			}

			sshCmd := fmt.Sprintf("/ssh src_v1 run --host %s --cmd whoami", hostName)
			if err := rt.RunTranscript([]support.TranscriptStep{
				{
					Send: sshCmd,
					ExpectRoomAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf(`"message":"%s"`, sshCmd),
							fmt.Sprintf(`ssh run: executing remote command on %s`, hostName),
							fmt.Sprintf(`ssh run: command completed on %s`, hostName),
							`"message":"user"`,
						},
						support.StandardCommandRoomPatternGroups("ssh src_v1", "", "")...,
					),
					ExpectOutputAny: support.CombinePatternGroups(
						[]string{
							fmt.Sprintf("llm-codex> %s", sshCmd),
							fmt.Sprintf("dialtone> ssh run: executing remote command on %s", hostName),
							fmt.Sprintf("dialtone> ssh run: command completed on %s", hostName),
						},
						support.StandardCommandOutputPatternGroups("ssh src_v1", "", "")...,
					),
					Timeout: 35 * time.Second,
				},
			}); err != nil {
				return testv1.StepRunResult{}, err
			}

			ctx.TestPassf("ssh remote command routed through llm-codex REPL prompt path via alias %s", hostName)
			return testv1.StepRunResult{
				Report: fmt.Sprintf("Joined REPL as llm-codex, added the sample SSH host alias `%s` through the REPL prompt flow, exercised SSH resolve through the prompt, verified the SSH resolve report selected the expected host/user/port plus a usable auth source and host-key mode, confirmed reachability and auth with the REPL SSH probe, then ran `whoami` through the REPL SSH task and verified the remote user output.", hostName),
			}, nil
		},
	})
}
