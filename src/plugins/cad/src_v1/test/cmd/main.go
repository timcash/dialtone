package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	selfcheck "dialtone/dev/plugins/cad/src_v1/test/01_self_check"
	browsercheck "dialtone/dev/plugins/cad/src_v1/test/02_browser_smoke"
	chromev3 "dialtone/dev/plugins/chrome/src_v3"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("cad-src-v1-test", flag.ExitOnError)
	commonBindings := testv1.BindCommonTestFlags(fs, testv1.CommonTestCLIOptions{
		AttachRole: "cad-smoke",
	})
	_ = fs.Parse(os.Args[1:])
	common, err := commonBindings.Resolve()
	if err != nil {
		logs.Error("cad src_v1 test flag parse failed: %v", err)
		os.Exit(1)
	}
	common.ApplyRuntimeConfig()
	if attach := strings.TrimSpace(common.AttachNode); attach != "" {
		if err := ensureAttachBrowser(attach); err != nil {
			logs.Error("cad src_v1 attach preflight failed: %v", err)
			os.Exit(1)
		}
	}

	reg := testv1.NewRegistry()
	selfcheck.Register(reg)
	browsercheck.Register(reg)
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(common.FilterExpr)); len(filtered) > 0 {
		reg.Steps = filtered
	}

	logs.Info("DIALTONE_INDEX: cad test: running %d suite steps", len(reg.Steps))
	err = reg.Run(common.ApplySuiteOptions(testv1.SuiteOptions{
		Version:       "cad-src-v1",
		NATSURL:       resolveSuiteNATSURL(),
		NATSSubject:   "logs.test.cad-src-v1",
		AutoStartNATS: true,
	}))
	if err != nil {
		logs.Error("cad src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("DIALTONE_INDEX: cad test: suite passed")
	logs.Info("cad src_v1 tests passed")
}

func resolveSuiteNATSURL() string {
	if v := strings.TrimSpace(os.Getenv("DIALTONE_REPL_NATS_URL")); v != "" {
		return v
	}
	return "nats://127.0.0.1:4222"
}

func filterSteps(steps []testv1.Step, filterExpr string) []testv1.Step {
	filterExpr = strings.TrimSpace(strings.ToLower(filterExpr))
	if filterExpr == "" {
		return nil
	}
	parts := strings.Split(filterExpr, ",")
	var out []testv1.Step
	for _, step := range steps {
		name := strings.ToLower(strings.TrimSpace(step.Name))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if strings.Contains(name, part) {
				out = append(out, step)
				break
			}
		}
	}
	if len(out) == 0 {
		logs.Warn("cad src_v1 --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	return out
}

func ensureAttachBrowser(node string) error {
	node = strings.TrimSpace(node)
	if node == "" {
		return fmt.Errorf("attach node is required")
	}
	role := strings.TrimSpace(testv1.RuntimeConfigSnapshot().RemoteBrowserRole)
	if role == "" {
		role = "dev"
	}
	logs.Info("DIALTONE_INDEX: cad test: ensuring chrome src_v3 role=%s on %s", role, node)
	if _, err := chromev3.EnsureRemoteServiceByHost(node, role, true); err != nil {
		return err
	}
	_, err := chromev3.SendCommandByHost(node, chromev3.CommandRequest{
		Command: "open",
		Role:    role,
		URL:     "about:blank",
	})
	return err
}
