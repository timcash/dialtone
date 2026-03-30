package localuimocke2e

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

const (
	mockPort   = "18083"
	mockNATSP  = "18224"
	mockNATSWP = "18225"
)

type mockUISession struct {
	repo           string
	baseURL        string
	browserBaseURL string
	remoteNode     string
}

func Register(reg *testv1.Registry) {
	reg.Add(testv1.Step{
		Name:    "04-ui-section-navigation",
		Timeout: uiStepTimeout(120 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			mock, err := prepareMockUI(ctx, "robot-hero-stage", "Hero Section", "nav")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			sections := []struct {
				Menu  string
				Aria  string
				Extra string
			}{
				{Menu: "Navigate Hero", Aria: "Hero Section"},
				{Menu: "Navigate Docs", Aria: "Docs Section"},
				{Menu: "Navigate Telemetry", Aria: "Telemetry Section", Extra: "Robot Table"},
				{Menu: "Navigate Steering Settings", Aria: "Steering Settings Section", Extra: "Steering Settings Table"},
				{Menu: "Navigate Key Params", Aria: "Key Params Section", Extra: "Key Params Table"},
				{Menu: "Navigate Three", Aria: "Three Section"},
				{Menu: "Navigate Terminal", Aria: "Xterm Section", Extra: "Xterm Terminal"},
				{Menu: "Navigate Camera", Aria: "Video Section"},
				{Menu: "Navigate Settings", Aria: "Settings Section", Extra: "Robot Version Button"},
			}
			for _, s := range sections {
				if err := navigateMenuToSection(ctx, s.Menu, s.Aria, s.Extra); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			return testv1.StepRunResult{Report: fmt.Sprintf("UI section navigation verified on %s", mock.browserBaseURL)}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "05-ui-table-buttons",
		Timeout: uiStepTimeout(45 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := prepareMockUI(ctx, "robot-table-table", "Telemetry Section", "table"); err != nil {
				return testv1.StepRunResult{}, err
			}
			nc, err := connectMockNATS()
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()
			if err := publishHeartbeat(nc); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Robot Table", "data-last-heartbeat-ts", "12345", 20*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Robot Table", "data-last-heartbeat-mav-type", "rover", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Table Mode Form", "Table Thumb 1"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Table Mode Form", "data-last-button-aria", "Table Thumb 1", 3*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Table Mode Form", "Table Thumb 2"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Robot Table", "data-last-clear-row-count", "0", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := publishHeartbeat(nc); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Robot Table", "data-last-heartbeat-ts", "12345", 20*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Telemetry section buttons verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "05a-ui-table-repeat-click-sequence",
		Timeout: uiStepTimeout(45 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := prepareMockUI(ctx, "robot-table-table", "Telemetry Section", "table-click-seq"); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("table repeat-click: first refresh click")
			if err := clickFormButton(ctx, "Table Mode Form", "Table Thumb 1"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Table Mode Form", "data-click-seq", "1", uiStepTimeout(6*time.Second)); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("table repeat-click: second refresh click")
			if err := clickFormButton(ctx, "Table Mode Form", "Table Thumb 1"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Table Mode Form", "data-click-seq", "2", uiStepTimeout(6*time.Second)); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("table repeat-click: repeated refresh advanced data-click-seq to 2")
			return testv1.StepRunResult{Report: "Telemetry table repeated-button click sequence verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "06-ui-steering-settings-buttons",
		Timeout: uiStepTimeout(75 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := prepareMockUI(ctx, "robot-steering-settings-table", "Steering Settings Section", "steering"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Table", "data-selected-key", "forwardThrottlePwm", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			for i := 0; i < 5; i++ {
				if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 2"); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Table", "data-selected-key", "forwardDurationMs", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 3"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Status", "data-status", "Forward Duration (ms) = 1900", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 4"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Status", "data-status", "Forward Duration (ms) = 1890", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 5"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Status", "data-status", "Forward Duration (ms) = 1900", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 6"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Status", "data-status", "Forward Duration (ms) = 2000", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 7"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Status", "data-status", "Saved", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 8"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Status", "data-status", "Reset to defaults", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Steering Settings Form", "Steering Settings Thumb 1"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Steering Settings Table", "data-selected-key", "rightSteeringPwm", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Steering settings buttons verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "07-ui-three-buttons-three-system-arm",
		Timeout: uiHeavyStepTimeout(60 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repo := ctx.RepoRoot()
			if _, err := prepareMockUI(ctx, "robot-three-stage", "Three Section", "three"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Three Mode Form", "data-current-mode", "Drive", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("three step: drive mode ready")
			driveChecks := []struct {
				ButtonAria string
				Command    string
				Extra      string
			}{
				{"Three Thumb 1", "drive_up", "throttlePwm=2000 steeringPwm=1000 durationMs=2000"},
				{"Three Thumb 2", "drive_up", "throttlePwm=2000 steeringPwm=1500 durationMs=2000"},
				{"Three Thumb 3", "drive_up", "throttlePwm=2000 steeringPwm=2000 durationMs=2000"},
				{"Three Thumb 4", "drive_down", "throttlePwm=1000 steeringPwm=1000 durationMs=2000"},
				{"Three Thumb 5", "drive_down", "throttlePwm=1000 steeringPwm=1500 durationMs=2000"},
				{"Three Thumb 6", "drive_down", "throttlePwm=1000 steeringPwm=2000 durationMs=2000"},
			}
			for _, check := range driveChecks {
				ctx.Infof("three drive: clicking %s expecting %s %s", check.ButtonAria, check.Command, check.Extra)
				if err := clickFormButton(ctx, "Three Mode Form", check.ButtonAria); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := waitForLastCommand(ctx, check.Command, "", check.Extra, 5*time.Second); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			ctx.Infof("three drive: issuing stop")
			if err := clickFormButton(ctx, "Three Mode Form", "Three Thumb 7"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForLastCommand(ctx, "guided_hold", "", "", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("three step: switching to system mode")
			if err := cycleFormMode(ctx, "Three Mode Form", "Three Mode", "System"); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("three step: system mode ready")
			systemChecks := []struct {
				ButtonAria string
				Command    string
				Mode       string
			}{
				{"Three Thumb 1", "arm", ""},
				{"Three Thumb 2", "disarm", ""},
				{"Three Thumb 3", "mode", "manual"},
				{"Three Thumb 4", "mode", "steering"},
				{"Three Thumb 5", "mode", "guided"},
				{"Three Thumb 6", "pulse_fwd", ""},
			}
			for _, check := range systemChecks {
				ctx.Infof("three system: clicking %s expecting %s mode=%s", check.ButtonAria, check.Command, check.Mode)
				if err := clickFormButton(ctx, "Three Mode Form", check.ButtonAria); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := waitForLastCommand(ctx, check.Command, check.Mode, "", 5*time.Second); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			ctx.Infof("three system: issuing stop")
			if err := clickFormButton(ctx, "Three Mode Form", "Three Thumb 7"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForLastCommand(ctx, "guided_hold", "", "", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("three step: switching to guided mode")
			if err := cycleFormMode(ctx, "Three Mode Form", "Three Mode", "Guided"); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("three step: guided mode ready")
			guidedChecks := []struct {
				ButtonAria string
				Command    string
				Mode       string
			}{
				{"Three Thumb 1", "mode", "guided"},
				{"Three Thumb 2", "guided_forward_1m", ""},
				{"Three Thumb 3", "guided_square_5m", ""},
				{"Three Thumb 4", "guided_hold", ""},
				{"Three Thumb 5", "mode", "manual"},
			}
			for _, check := range guidedChecks {
				ctx.Infof("three guided: clicking %s expecting %s mode=%s", check.ButtonAria, check.Command, check.Mode)
				if err := clickFormButton(ctx, "Three Mode Form", check.ButtonAria); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := waitForLastCommand(ctx, check.Command, check.Mode, "", 5*time.Second); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := clickFormButton(ctx, "Three Mode Form", "Three Thumb 6"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForLastCommand(ctx, "guided_hold", "", "", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			shotPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "test", "screenshots", "three_system_arm.png")
			if err := os.MkdirAll(filepath.Dir(shotPath), 0755); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.CaptureScreenshot(shotPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Three section buttons verified, including system arm flow"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "07a-ui-three-mode-cycle",
		Timeout: uiHeavyStepTimeout(45 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := prepareMockUI(ctx, "robot-three-stage", "Three Section", "three-cycle"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Three Mode Form", "data-current-mode", "Drive", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Three Mode Form", "Three Mode", "System"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Three Mode Form", "Three Mode", "Guided"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Three Mode Form", "Three Mode", "Drive"); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Three mode cycling verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "08-ui-terminal-routing-and-buttons",
		Timeout: uiHeavyStepTimeout(60 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			repo := ctx.RepoRoot()
			nc, err := prepareTerminalMockUI(ctx, "robot-xterm-xterm", "xterm")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()
			if err := navigateMenuToSection(ctx, "Navigate Telemetry", "Telemetry Section", "Robot Table"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Robot Table", "data-last-command-ack-result", "MAV_RESULT_FAILED", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := navigateMenuToSection(ctx, "Navigate Terminal", "Xterm Section", "Xterm Terminal"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 7"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-paused", "true", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 7"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-paused", "false", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, button := range []string{"Log Thumb 1", "Log Thumb 2", "Log Thumb 3", "Log Thumb 4", "Log Thumb 5", "Log Thumb 6", "Log Thumb 8"} {
				if err := clickFormButton(ctx, "Log Mode Form", button); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Filter"); err != nil {
				return testv1.StepRunResult{}, err
			}
			filterChecks := []struct {
				ButtonAria string
				Expected   string
			}{
				{"Log Thumb 2", "mavlink"},
				{"Log Thumb 3", "command"},
				{"Log Thumb 4", "ui"},
				{"Log Thumb 5", "camera"},
				{"Log Thumb 6", "service"},
				{"Log Thumb 7", "error"},
				{"Log Thumb 1", "all"},
			}
			for _, check := range filterChecks {
				if err := clickFormButton(ctx, "Log Mode Form", check.ButtonAria); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-filter", check.Expected, 5*time.Second); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 8"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-ui-action", "clear", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Command"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.TypeAriaLabel("Log Command Input", "arm"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 1"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForLastCommand(ctx, "arm", "", "", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			commandChecks := []struct {
				ButtonAria string
				Command    string
				Mode       string
			}{
				{"Log Thumb 2", "arm", ""},
				{"Log Thumb 3", "disarm", ""},
				{"Log Thumb 4", "mode", "manual"},
				{"Log Thumb 5", "mode", "guided"},
			}
			for _, check := range commandChecks {
				if err := clickFormButton(ctx, "Log Mode Form", check.ButtonAria); err != nil {
					return testv1.StepRunResult{}, err
				}
				if err := waitForLastCommand(ctx, check.Command, check.Mode, "", 5*time.Second); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 6"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitForLastCommand(ctx, "stop", "", "", 3*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 7"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Log Mode Form", "data-current-mode", "Tail", 3*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Select"); err != nil {
				return testv1.StepRunResult{}, err
			}
			for _, button := range []string{"Log Thumb 1", "Log Thumb 2", "Log Thumb 3", "Log Thumb 4", "Log Thumb 5", "Log Thumb 7"} {
				if err := clickFormButton(ctx, "Log Mode Form", button); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 6"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Log Mode Form", "data-current-mode", "Tail", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Select"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 8"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Log Mode Form", "data-current-mode", "Tail", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			shotPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "test", "screenshots", "arm_failure_xterm.png")
			if err := os.MkdirAll(filepath.Dir(shotPath), 0755); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.CaptureScreenshot(shotPath); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Terminal section buttons and MAVLink routing verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "08a-ui-terminal-clear-action-state",
		Timeout: uiHeavyStepTimeout(45 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			nc, err := prepareTerminalMockUI(ctx, "robot-xterm-xterm", "xterm-clear")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Filter"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 4"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-filter", "ui", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 8"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-ui-action", "clear", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-log-category", "ui", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Terminal clear action state verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "08b-ui-terminal-mode-cycle",
		Timeout: uiHeavyStepTimeout(45 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			nc, err := prepareTerminalMockUI(ctx, "robot-xterm-xterm", "xterm-mode-cycle")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer nc.Close()
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Filter"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Command"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Select"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Log Mode Form", "Log Thumb 8"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Log Mode Form", "data-current-mode", "Tail", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := cycleFormMode(ctx, "Log Mode Form", "Log Mode", "Select"); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Terminal mode cycling verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "09-ui-video-buttons",
		Timeout: uiStepTimeout(45 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := prepareMockUI(ctx, "robot-video-video", "Video Section", "video"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Video Section", "data-feed-source", "Primary", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Video Mode Form", "Video Thumb 2"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Video Section", "data-feed-source", "Secondary", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Video Mode Form", "Video Thumb 1"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Video Section", "data-feed-source", "Primary", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Video Mode Form", "Video Thumb 3"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Video Section", "data-last-bookmark-status", "saved", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := clickFormButton(ctx, "Video Mode Form", "Video Mode"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Video Mode Form", "data-current-mode", "View", 3*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Video section buttons verified"}, nil
		},
	})

	reg.Add(testv1.Step{
		Name:    "10-ui-settings-and-keyparams",
		Timeout: uiStepTimeout(75 * time.Second),
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			if _, err := prepareMockUI(ctx, "robot-settings-button-list", "Settings Section", "settings"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabel("Robot Version Button", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabel("Toggle Chatlog Button", 8*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("settings step: toggling chatlog overlay on")
			if err := ctx.ClickAriaLabel("Toggle Chatlog Button"); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("settings step: navigating to three section for overlay check")
			if err := navigateMenuToSection(ctx, "Navigate Three", "Three Section", "Three Chatlog Overlay"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Three Chatlog Overlay", "data-enabled", "true", 5*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			ctx.Infof("settings step: navigating to key params section")
			if err := navigateMenuToSection(ctx, "Navigate Key Params", "Key Params Section", "Key Params Table"); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := ctx.WaitForAriaLabelAttrEquals("Key Params Table", "data-row-count", "10", 15*time.Second); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "Settings and key params sections verified"}, nil
		},
	})
}

func ensureUIBuild(ctx *testv1.StepContext, repo string) error {
	return ctx.WaitForStepMessageAfterAction("ui build complete", 60*time.Second, func() error {
		cmd := exec.Command("./dialtone.sh", "robot", "src_v2", "build")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			ctx.Errorf("ui build failed: %s", strings.TrimSpace(string(out)))
			return err
		}
		ctx.Infof("ui build complete")
		return nil
	})
}

func prepareMockUI(ctx *testv1.StepContext, hashID, sectionAria, roleSuffix string) (mockUISession, error) {
	repo := ctx.RepoRoot()
	uiDist := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "ui", "dist")
	if err := ensureUIBuild(ctx, repo); err != nil {
		return mockUISession{}, err
	}
	baseURL, browserBaseURL, remoteNode, err := startLocalMockServer(repo, uiDist, mockPort)
	if err != nil {
		return mockUISession{}, err
	}
	userDataDir := filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-robot-%s-%d", roleSuffix, time.Now().UnixNano()))
	_, err = ctx.EnsureBrowser(testv1.BrowserOptions{
		Headless:    false,
		GPU:         true,
		Role:        "robot-src-v2-" + roleSuffix,
		RemoteNode:  remoteNode,
		UserDataDir: userDataDir,
		URL:         browserBaseURL + fmt.Sprintf("/?step=%s-%d#%s", roleSuffix, time.Now().UnixNano(), hashID),
	})
	if err != nil {
		return mockUISession{}, err
	}
	if sectionAria == "Hero Section" {
		if err := waitForHeroReadyByID(ctx, 12*time.Second); err != nil {
			return mockUISession{}, err
		}
	} else {
		if err := waitForSectionReady(ctx, sectionAria, 12*time.Second); err != nil {
			return mockUISession{}, err
		}
	}
	if err := ctx.WaitForAriaLabelAttrEquals("App Header", "data-nats-connected", "true", 12*time.Second); err != nil {
		return mockUISession{}, err
	}
	return mockUISession{
		repo:           repo,
		baseURL:        baseURL,
		browserBaseURL: browserBaseURL,
		remoteNode:     remoteNode,
	}, nil
}

func prepareTerminalMockUI(ctx *testv1.StepContext, hashID, roleSuffix string) (*nats.Conn, error) {
	if _, err := prepareMockUI(ctx, hashID, "Xterm Section", roleSuffix); err != nil {
		return nil, err
	}
	if err := ctx.WaitForAriaLabel("Xterm Terminal", 8*time.Second); err != nil {
		return nil, err
	}
	if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-ready", "true", 8*time.Second); err != nil {
		return nil, err
	}
	nc, err := connectMockNATS()
	if err != nil {
		return nil, err
	}
	if err := publishTerminalEvents(nc); err != nil {
		nc.Close()
		return nil, err
	}
	if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-command-ack-result", "MAV_RESULT_FAILED", 8*time.Second); err != nil {
		nc.Close()
		return nil, err
	}
	if err := ctx.WaitForAriaLabelAttrEquals("Xterm Terminal", "data-last-status-text", "Arm: Radio failsafe on", 8*time.Second); err != nil {
		nc.Close()
		return nil, err
	}
	return nc, nil
}

func waitForSectionReady(ctx *testv1.StepContext, sectionAria string, timeout time.Duration) error {
	if err := ctx.WaitForAriaLabel(sectionAria, timeout); err != nil {
		return err
	}
	return ctx.WaitForAriaLabelAttrEquals(sectionAria, "data-active", "true", timeout)
}

func navigateMenuToSection(ctx *testv1.StepContext, menuAria, sectionAria, extraAria string) error {
	if err := ctx.ClickAriaLabelAfterWait("Toggle Global Menu", 12*time.Second); err != nil {
		return err
	}
	if err := ctx.ClickAriaLabelAfterWait(menuAria, 12*time.Second); err != nil {
		return err
	}
	if err := waitForSectionReady(ctx, sectionAria, 12*time.Second); err != nil {
		return err
	}
	if extraAria != "" {
		if err := ctx.WaitForAriaLabel(extraAria, 12*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func clickFormButton(ctx *testv1.StepContext, formAria, buttonAria string) error {
	if err := ctx.WaitForAriaLabel(formAria, 8*time.Second); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabelAttrEquals(formAria, "data-buttons-ready", "true", 8*time.Second); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabel(buttonAria, 8*time.Second); err != nil {
		return err
	}
	beforeSeq, err := ctx.ReadAriaLabelAttr(formAria, "data-click-seq")
	if err != nil {
		return err
	}
	ctx.Infof("%s click start button=%s seq=%s", formAria, buttonAria, beforeSeq)
	if err := ctx.ClickAriaLabel(buttonAria); err != nil {
		return err
	}
	afterSeq, err := waitForAriaLabelAttrChange(ctx, formAria, "data-click-seq", beforeSeq, uiStepTimeout(6*time.Second))
	if err != nil {
		return err
	}
	ctx.Infof("%s click complete button=%s seq=%s", formAria, buttonAria, afterSeq)
	return ctx.WaitForAriaLabelAttrEquals(formAria, "data-last-button-aria", buttonAria, uiStepTimeout(6*time.Second))
}

func cycleFormMode(ctx *testv1.StepContext, formAria, modeButtonAria, expectedMode string) error {
	deadline := time.Now().Add(uiStepTimeout(30 * time.Second))
	for time.Now().Before(deadline) {
		currentMode, err := ctx.ReadAriaLabelAttr(formAria, "data-current-mode")
		if err != nil {
			return err
		}
		if currentMode == expectedMode {
			ctx.Infof("%s mode is now %s", formAria, expectedMode)
			return nil
		}
		ctx.Infof("%s mode=%s target=%s", formAria, currentMode, expectedMode)
		if err := clickFormButton(ctx, formAria, modeButtonAria); err != nil {
			return err
		}
		if nextMode, err := waitForAriaLabelAttrChange(ctx, formAria, "data-current-mode", currentMode, uiStepTimeout(4*time.Second)); err == nil {
			ctx.Infof("%s mode advanced to %s", formAria, nextMode)
			continue
		}
	}
	return fmt.Errorf("timed out waiting for %s current mode=%s", formAria, expectedMode)
}

func waitForAriaLabelAttrChange(ctx *testv1.StepContext, label, attr, before string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		value, err := ctx.ReadAriaLabelAttr(label, attr)
		if err == nil {
			last = value
			if value != before {
				return value, nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	if last == "" {
		last = before
	}
	return "", fmt.Errorf("timed out waiting for %s attr %s to change from %q (last=%q)", label, attr, before, last)
}

func waitForLastCommand(ctx *testv1.StepContext, cmd, mode, extra string, timeout time.Duration) error {
	if err := ctx.WaitForAriaLabelAttrEquals("App Header", "data-last-rover-command", cmd, timeout); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabelAttrEquals("App Header", "data-last-rover-command-mode", mode, timeout); err != nil {
		return err
	}
	if extra == "" {
		return nil
	}
	return ctx.WaitForAriaLabelAttrEquals("App Header", "data-last-rover-command-extra", extra, timeout)
}

func uiStepTimeout(base time.Duration) time.Duration {
	if strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode) != "" && base < 90*time.Second {
		return base * 2
	}
	return base
}

func uiHeavyStepTimeout(base time.Duration) time.Duration {
	timeout := uiStepTimeout(base)
	if strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode) != "" && timeout < 8*time.Minute {
		return 8 * time.Minute
	}
	return timeout
}

func connectMockNATS() (*nats.Conn, error) {
	return nats.Connect("nats://127.0.0.1:"+mockNATSP, nats.Timeout(2*time.Second))
}

func publishHeartbeat(nc *nats.Conn) error {
	msg := `{"type":"HEARTBEAT","custom_mode":4,"mav_type":"rover","timestamp":12345}`
	if err := nc.Publish("mavlink.heartbeat", []byte(msg)); err != nil {
		return err
	}
	return nc.Flush()
}

func publishTerminalEvents(nc *nats.Conn) error {
	messages := map[string]string{
		"mavlink.heartbeat":      `{"type":"HEARTBEAT","custom_mode":4,"mav_type":"rover","timestamp":12345}`,
		"mavlink.command_ack":    `{"type":"COMMAND_ACK","command":"MAV_CMD_COMPONENT_ARM_DISARM","result":"MAV_RESULT_FAILED","timestamp":12346}`,
		"mavlink.statustext":     `{"type":"STATUSTEXT","severity":"MAV_SEVERITY_CRITICAL","text":"Arm: Radio failsafe on","timestamp":12347}`,
		"camera.status":          `{"message":"camera stream ready","timestamp":12348}`,
		"robot.service":          `{"type":"SERVICE","source":"robot_src_v2","uptime":"5s","connections":1,"timestamp":12349,"errors":[]}`,
		"robot.autoswap.runtime": `{"type":"AUTOSWAP_RUNTIME","source":"autoswap","listen":":18086","running_count":4,"process_count":4,"process_names":"robot,camera,mavlink,repl","timestamp":12350,"errors":[]}`,
	}
	for subject, payload := range messages {
		if err := nc.Publish(subject, []byte(payload)); err != nil {
			return err
		}
	}
	return nc.Flush()
}

func startLocalMockServer(repo, uiDist, port string) (baseURL, browserBaseURL, remoteNode string, err error) {
	baseURL = "http://127.0.0.1:" + port
	browserBaseURL = baseURL
	remoteNode = strings.TrimSpace(testv1.RuntimeConfigSnapshot().BrowserNode)
	opts := GetOptions()
	if remoteNode != "" {
		if v := strings.TrimSpace(opts.BrowserBaseURL); v != "" {
			browserBaseURL = strings.TrimRight(v, "/")
		} else if isWSL() && strings.EqualFold(remoteNode, "legion") {
			if hostIP := wslHostIPForWindowsBrowser(); hostIP != "" {
				browserBaseURL = "http://" + hostIP + ":" + port
			} else {
				browserBaseURL = baseURL
			}
		} else if dnsName := tailscaleSelfDNSName(); dnsName != "" {
			browserBaseURL = "http://" + dnsName + ":" + port
		} else if host, herr := os.Hostname(); herr == nil && strings.TrimSpace(host) != "" {
			browserBaseURL = "http://" + strings.TrimSpace(host) + ":" + port
		}
	} else if isWSL() {
		if hostIP := wslHostIPForWindowsBrowser(); hostIP != "" {
			browserBaseURL = "http://" + hostIP + ":" + port
		}
	}
	_ = exec.Command("pkill", "-f", `dialtone_robot_v2.*--listen :`+port).Run()
	cmd := exec.Command(
		configv1.PluginBinaryPath(configv1.Runtime{RepoRoot: repo}, "robot", "src_v2", "dialtone_robot_v2"),
		"--listen", ":"+port,
		"--nats-port", mockNATSP,
		"--nats-ws-port", mockNATSWP,
		"--ui-dist", uiDist,
	)
	cmd.Dir = repo
	logPath := filepath.Join(repo, "src", "plugins", "robot", "src_v2", "test", "local_ui_mock_server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return "", "", "", err
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return "", "", "", err
	}
	_ = cmd.Process.Release()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp, reqErr := http.Get(baseURL + "/")
		if reqErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return baseURL, browserBaseURL, remoteNode, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", "", "", fmt.Errorf("ui root did not return 200")
}

func tailscaleSelfDNSName() string {
	cmd := exec.Command("tailscale", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
}

func isWSL() bool {
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), "microsoft")
}

func wslHostIPForWindowsBrowser() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet == nil {
				continue
			}
			ip := ipnet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func waitForHeroReadyByID(ctx *testv1.StepContext, timeout time.Duration) error {
	if err := ctx.WaitForAriaLabel("Hero Section", timeout); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabel("Hero Canvas", timeout); err != nil {
		return err
	}
	if err := ctx.WaitForAriaLabelAttrEquals("Hero Section", "data-ready", "true", timeout); err != nil {
		return err
	}
	return ctx.WaitForAriaLabelAttrEquals("Hero Section", "data-active", "true", timeout)
}
