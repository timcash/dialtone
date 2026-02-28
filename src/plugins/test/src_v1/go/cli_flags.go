package test

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
)

type CommonTestCLIOptions struct {
	AttachNode       string
	TargetURL        string
	FilterExpr       string
	ClicksPerSecond  float64
	NoSSH            bool
	RemoteNoLaunch   bool
	RemoteDebugPort  int
	RemoteDebugPorts []int
	RemoteBrowserPID int
}

type CommonTestCLIBindings struct {
	attachNode       *string
	targetURL        *string
	filterExpr       *string
	clicksPerSecond  *float64
	noSSH            *bool
	remoteNoLaunch   *bool
	remoteDebugPort  *int
	remoteDebugPorts *string
	remoteBrowserPID *int
}

func BindCommonTestFlags(fs *flag.FlagSet, defaults CommonTestCLIOptions) CommonTestCLIBindings {
	return CommonTestCLIBindings{
		attachNode: fs.String("attach", strings.TrimSpace(defaults.AttachNode), "Attach test browser to headed browser on mesh node (example: chroma)"),
		targetURL:  fs.String("url", strings.TrimSpace(defaults.TargetURL), "URL for browser steps"),
		filterExpr: fs.String("filter", strings.TrimSpace(defaults.FilterExpr), "Run only matching steps"),
		clicksPerSecond: fs.Float64(
			"cps",
			defaults.ClicksPerSecond,
			"Throttle UI clicks/taps in clicks per second (example: --cps 1)",
		),
		noSSH:            fs.Bool("no-ssh", defaults.NoSSH, "Disable SSH fallback and use direct attach only"),
		remoteNoLaunch:   fs.Bool("remote-no-launch", defaults.RemoteNoLaunch, "Do not launch remote browser when attach probe cannot reuse one"),
		remoteDebugPort:  fs.Int("remote-debug-port", defaults.RemoteDebugPort, "Preferred remote debugging port for attach"),
		remoteDebugPorts: fs.String("remote-debug-ports", joinInts(defaults.RemoteDebugPorts), "Comma-separated remote debug ports to probe first"),
		remoteBrowserPID: fs.Int("remote-browser-pid", defaults.RemoteBrowserPID, "Preferred remote browser PID for attach selection"),
	}
}

func (b CommonTestCLIBindings) Resolve() (CommonTestCLIOptions, error) {
	opts := CommonTestCLIOptions{}
	if b.attachNode != nil {
		opts.AttachNode = strings.TrimSpace(*b.attachNode)
	}
	if b.targetURL != nil {
		opts.TargetURL = strings.TrimSpace(*b.targetURL)
	}
	if b.filterExpr != nil {
		opts.FilterExpr = strings.TrimSpace(*b.filterExpr)
	}
	if b.clicksPerSecond != nil {
		opts.ClicksPerSecond = *b.clicksPerSecond
	}
	if b.noSSH != nil {
		opts.NoSSH = *b.noSSH
	}
	if b.remoteNoLaunch != nil {
		opts.RemoteNoLaunch = *b.remoteNoLaunch
	}
	if b.remoteDebugPort != nil {
		opts.RemoteDebugPort = *b.remoteDebugPort
	}
	if b.remoteDebugPorts != nil {
		ports, err := parseIntsCSV(*b.remoteDebugPorts)
		if err != nil {
			return CommonTestCLIOptions{}, err
		}
		opts.RemoteDebugPorts = ports
	}
	if b.remoteBrowserPID != nil {
		opts.RemoteBrowserPID = *b.remoteBrowserPID
	}
	if opts.ClicksPerSecond < 0 {
		return CommonTestCLIOptions{}, fmt.Errorf("--cps must be >= 0")
	}
	return opts, nil
}

func (o CommonTestCLIOptions) ApplyRuntimeConfig() {
	cfg := RuntimeConfig{
		BrowserNode:       strings.TrimSpace(o.AttachNode),
		RemoteRequireRole: false,
		NoSSH:             o.NoSSH,
		RemoteNoLaunch:    o.RemoteNoLaunch,
		RemoteDebugPort:   o.RemoteDebugPort,
		RemoteDebugPorts:  append([]int(nil), o.RemoteDebugPorts...),
		RemoteBrowserPID:  o.RemoteBrowserPID,
	}
	SetRuntimeConfig(cfg)
}

func (o CommonTestCLIOptions) ApplySuiteOptions(in SuiteOptions) SuiteOptions {
	out := in
	attach := strings.TrimSpace(o.AttachNode) != ""
	if attach {
		out.PreserveSharedBrowser = true
		out.SkipBrowserCleanup = true
	}
	return out
}

func parseIntsCSV(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		v, err := strconv.Atoi(s)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("invalid integer in --remote-debug-ports: %q", s)
		}
		out = append(out, v)
	}
	return out, nil
}

func joinInts(vals []int) string {
	if len(vals) == 0 {
		return ""
	}
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		if v > 0 {
			parts = append(parts, strconv.Itoa(v))
		}
	}
	return strings.Join(parts, ",")
}
