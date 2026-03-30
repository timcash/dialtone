package main

import (
	"errors"
	"flag"
	"os"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	selfcheck "dialtone/dev/plugins/ssh/src_v1/test/01_self_check"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("ssh src_v1 test", flag.ContinueOnError)
	filter := fs.String("filter", "", "Run only matching test steps")
	host := fs.String("host", "", "Mesh host to use for ssh self-checks")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("ssh src_v1 test parse failed: %v", err)
		os.Exit(1)
	}

	reg := testv1.NewRegistry()
	testHost := resolveTestHost(*host)
	selfcheck.Register(reg, selfcheck.Config{
		Host: testHost,
	})
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(*filter)); len(filtered) > 0 {
		reg.Steps = filtered
	}

	logs.Info("Running ssh src_v1 tests in single process (%d steps, host=%s)", len(reg.Steps), testHost)
	err := reg.Run(testv1.SuiteOptions{
		Version:       "ssh-src-v1",
		ReportPath:    "plugins/ssh/src_v1/TEST.md",
		RawReportPath: "plugins/ssh/src_v1/TEST_RAW.md",
		ReportFormat:  "template",
		ReportTitle:   "SSH Plugin src_v1 Test Report",
		ReportRunner:  "test/src_v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.ssh-src-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("ssh src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("ssh src_v1 tests passed")
}

func resolveTestHost(flagHost string) string {
	if host := strings.TrimSpace(flagHost); host != "" {
		return host
	}
	if host := strings.TrimSpace(configv1.LookupEnvString("DIALTONE_SSH_TEST_HOST")); host != "" {
		return host
	}
	return "grey"
}

func filterSteps(steps []testv1.Step, filterExpr string) []testv1.Step {
	filterExpr = strings.TrimSpace(strings.ToLower(filterExpr))
	if filterExpr == "" {
		return nil
	}
	parts := strings.Split(filterExpr, ",")
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		token := strings.TrimSpace(strings.ToLower(part))
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	if len(tokens) == 0 {
		return nil
	}
	out := make([]testv1.Step, 0, len(steps))
	for _, step := range steps {
		name := strings.ToLower(strings.TrimSpace(step.Name))
		for _, token := range tokens {
			if strings.Contains(name, token) {
				out = append(out, step)
				break
			}
		}
	}
	if len(out) == 0 {
		logs.Warn("ssh src_v1 --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("ssh src_v1 --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
