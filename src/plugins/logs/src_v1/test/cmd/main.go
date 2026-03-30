package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	infra "dialtone/dev/plugins/logs/src_v1/test/01_infra"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("logs src_v1 test", flag.ContinueOnError)
	filter := fs.String("filter", "", "Run only matching test steps")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("logs test parse failed: %v", err)
		os.Exit(1)
	}

	paths, err := logs.ResolvePaths("", "src_v1")
	if err != nil {
		logs.Error("logs test init failed: %v", err)
		os.Exit(1)
	}
	reportPath := paths.TestReport

	reg := testv1.NewRegistry()
	infra.Register(reg)
	if filtered := filterSteps(reg.Steps, strings.TrimSpace(*filter)); len(filtered) > 0 {
		reg.Steps = filtered
	}
	logs.Info("Running logs src_v1 tests in single process (%d steps)", len(reg.Steps))
	rawReportPath := reportPath
	if ext := filepath.Ext(reportPath); ext != "" {
		rawReportPath = strings.TrimSuffix(reportPath, ext) + "_RAW" + ext
	} else if strings.TrimSpace(reportPath) != "" {
		rawReportPath = reportPath + "_RAW.md"
	}

	if err := reg.Run(testv1.SuiteOptions{
		Version:       "logs-src-v1",
		ReportPath:    reportPath,
		RawReportPath: rawReportPath,
		ReportFormat:  "template",
		ReportTitle:   "Logs Plugin src_v1 Test Report",
		ReportRunner:  "test/src_v1",
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.logs-src-v1",
		AutoStartNATS: true,
	}); err != nil {
		logs.Error("logs src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("logs src_v1 tests passed")
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
		sectionID := strings.ToLower(strings.TrimSpace(step.SectionID))
		for _, token := range tokens {
			if strings.Contains(name, token) || strings.Contains(sectionID, token) {
				out = append(out, step)
				break
			}
		}
	}
	if len(out) == 0 {
		logs.Warn("logs src_v1 --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("logs src_v1 --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
