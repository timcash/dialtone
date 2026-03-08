package main

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"dialtone/dev/plugins/ui/src_v1/test"
	qualitychecks "dialtone/dev/plugins/ui/src_v1/test/00_quality_checks"
	buildserve "dialtone/dev/plugins/ui/src_v1/test/01_build_and_serve"
	contextdiag "dialtone/dev/plugins/ui/src_v1/test/02_context_cancel_diagnose"
	sectionhero "dialtone/dev/plugins/ui/src_v1/test/02_section_hero"
	sectionthreefullscreen "dialtone/dev/plugins/ui/src_v1/test/03_section_three_fullscreen"
	sectionthreecalculator "dialtone/dev/plugins/ui/src_v1/test/04_section_three_calculator"
	sectiontable "dialtone/dev/plugins/ui/src_v1/test/05_section_table"
	sectioncamera "dialtone/dev/plugins/ui/src_v1/test/06_section_camera"
	sectiondocs "dialtone/dev/plugins/ui/src_v1/test/07_section_docs"
	sectionterminal "dialtone/dev/plugins/ui/src_v1/test/08_section_terminal"
	sectionsettings "dialtone/dev/plugins/ui/src_v1/test/09_section_settings"
)

func main() {
	logs.SetOutput(os.Stdout)
	fs := flag.NewFlagSet("ui test", flag.ContinueOnError)
	commonFlags := testv1.BindCommonTestFlags(fs, testv1.CommonTestCLIOptions{
		ActionsPerMinute: 300,
	})
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logs.Error("ui test parse failed: %v", err)
		os.Exit(1)
	}
	common, err := commonFlags.Resolve()
	if err != nil {
		logs.Error("ui test parse failed: %v", err)
		os.Exit(1)
	}
	attach := strings.TrimSpace(common.AttachNode)
	url := strings.TrimSpace(common.TargetURL)
	test.SetOptions(test.Options{
		AttachNode:       attach,
		TargetURL:        url,
		ActionsPerMinute: common.ActionsPerMinute,
		ClicksPerSecond:  common.ClicksPerSecond,
	})
	common.ApplyRuntimeConfig()
	if attach != "" {
		if meshNode, err := sshv1.ResolveMeshNode(attach); err == nil && strings.EqualFold(strings.TrimSpace(meshNode.OS), "windows") {
			testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
				cfg.NoSSH = true
			})
			logs.Info("ui test auto-enabled --no-ssh for windows attach node=%s", attach)
		}
		logs.Info("ui test remote attach mode (headed) node=%s url=%s apm=%.3f", attach, url, common.ActionsPerMinute)
		if err := ensureAttachBrowser(attach, url); err != nil {
			logs.Error("ui test attach preflight failed: %v", err)
			os.Exit(1)
		}
		testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
			cfg.BrowserAllowCreateTarget = true
			cfg.RemoteNoLaunch = true
			cfg.RemoteBrowserPID = 0
		})
	} else {
		testv1.UpdateRuntimeConfig(func(cfg *testv1.RuntimeConfig) {
			cfg.BrowserAllowCreateTarget = false
			cfg.BrowserNewTargetURL = ""
			cfg.RemoteNoLaunch = false
			cfg.RemoteBrowserPID = 0
		})
		logs.Info("ui test local mode url=%s apm=%.3f", url, common.ActionsPerMinute)
	}

	reg := test.NewRegistry()
	qualitychecks.Register(reg)
	buildserve.Register(reg)
	contextdiag.Register(reg)
	sectionhero.Register(reg)
	sectionthreefullscreen.Register(reg)
	sectionthreecalculator.Register(reg)
	sectiontable.Register(reg)
	sectioncamera.Register(reg)
	sectiondocs.Register(reg)
	sectionterminal.Register(reg)
	sectionsettings.Register(reg)

	if filtered := filterSteps(reg.Steps, strings.TrimSpace(common.FilterExpr)); len(filtered) > 0 {
		reg.Steps = filtered
	}
	logs.Info("Starting UI src_v1 suite with %d registered steps", len(reg.Steps))
	runErr := reg.Run(common.ApplySuiteOptions(testv1.SuiteOptions{
		Version:            "ui-src-v1",
		ReportPath:         "plugins/ui/src_v1/TEST.md",
		RawReportPath:      "plugins/ui/src_v1/TEST_RAW.md",
		ReportFormat:       "template",
		ReportTitle:        "UI Plugin src_v1 Test Report",
		ReportRunner:       "test/src_v1",
		NATSURL:            "nats://127.0.0.1:4222",
		NATSSubject:        "logs.test.ui.src-v1",
		AutoStartNATS:      true,
		BrowserCleanupRole: "ui-test",
	}))
	if runErr != nil {
		logs.Error("UI src_v1 suite failed: %v", runErr)
		os.Exit(1)
	}
	logs.Info("UI src_v1 suite passed")
}

func ensureAttachBrowser(node, url string) error {
	node = strings.TrimSpace(node)
	if node == "" {
		return errors.New("attach node is required")
	}
	targetURL := strings.TrimSpace(url)
	if targetURL == "" {
		targetURL = "about:blank"
	}
	args := []string{
		dialtoneScriptPath(),
		"chrome", "src_v1", "remote-new",
		"--node", node,
		"--port", "9333",
		"--role", "test",
		"--reuse-existing",
		"--debug-address", "0.0.0.0",
		"--url", targetURL,
	}
	// Legion Windows attach is notably more stable in headless mode.
	if meshNode, err := sshv1.ResolveMeshNode(node); err == nil && strings.EqualFold(strings.TrimSpace(meshNode.OS), "windows") {
		args = append(args, "--headless")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dialtoneScriptPath() string {
	if p := os.Getenv("DIALTONE_SCRIPT"); strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "./dialtone.sh"
	}
	cur := cwd
	for i := 0; i < 6; i++ {
		candidate := filepath.Join(cur, "dialtone.sh")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return filepath.Join(cwd, "dialtone.sh")
}

func filterSteps(steps []testv1.Step, filterExpr string) []testv1.Step {
	filterExpr = strings.TrimSpace(strings.ToLower(filterExpr))
	if filterExpr == "" {
		return nil
	}

	presetStepNames := map[string][]string{
		"remote-browser-dev": {
			"ui-attach-context-cancel-diagnose",
		},
	}

	parts := strings.Split(filterExpr, ",")
	tokens := make([]string, 0, len(parts))
	exact := map[string]struct{}{}
	for _, part := range parts {
		token := strings.TrimSpace(strings.ToLower(part))
		if token == "" {
			continue
		}
		if names, ok := presetStepNames[token]; ok {
			for _, n := range names {
				exact[strings.ToLower(strings.TrimSpace(n))] = struct{}{}
			}
			continue
		}
		tokens = append(tokens, token)
	}

	out := make([]testv1.Step, 0, len(steps))
	for _, step := range steps {
		name := strings.ToLower(strings.TrimSpace(step.Name))
		if _, ok := exact[name]; ok {
			out = append(out, step)
			continue
		}
		for _, token := range tokens {
			if strings.Contains(name, token) {
				out = append(out, step)
				break
			}
		}
	}

	if len(out) == 0 {
		logs.Warn("ui test --filter=%q matched no steps; running all steps", filterExpr)
		return nil
	}
	names := make([]string, 0, len(out))
	for _, step := range out {
		names = append(names, step.Name)
	}
	logs.Info("ui test --filter=%q selected steps: %s", filterExpr, strings.Join(names, ", "))
	return out
}
