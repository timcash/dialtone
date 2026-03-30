package test

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

type CommonTestCLIOptions struct {
	AttachNode        string
	AttachRole        string
	DefaultAttachNode string
	TargetURL         string
	FilterExpr        string
	ActionsPerMinute  float64
	ClicksPerSecond   float64
	ForceLocalBrowser bool
	NoSSH             bool
	RemoteNoLaunch    bool
	RemoteDebugPort   int
	RemoteDebugPorts  []int
	RemoteBrowserPID  int
}

type CommonTestCLIBindings struct {
	attachNode        *string
	attachRole        *string
	defaultAttachNode *string
	targetURL         *string
	filterExpr        *string
	actionsPerMinute  *float64
	clicksPerSecond   *float64
	forceLocalBrowser *bool
	noSSH             *bool
	remoteNoLaunch    *bool
	remoteDebugPort   *int
	remoteDebugPorts  *string
	remoteBrowserPID  *int
}

func BindCommonTestFlags(fs *flag.FlagSet, defaults CommonTestCLIOptions) CommonTestCLIBindings {
	return CommonTestCLIBindings{
		attachNode:        fs.String("attach", strings.TrimSpace(defaults.AttachNode), "Attach test browser to headed browser on mesh node (example: legion)"),
		attachRole:        fs.String("attach-role", strings.TrimSpace(defaults.AttachRole), "Browser role to reuse on attach node (default: dev)"),
		defaultAttachNode: fs.String("default-attach", strings.TrimSpace(defaults.DefaultAttachNode), "Default remote browser node when running from WSL without --attach (use none/off/local to disable)"),
		targetURL:         fs.String("url", strings.TrimSpace(defaults.TargetURL), "URL for browser steps"),
		filterExpr:        fs.String("filter", strings.TrimSpace(defaults.FilterExpr), "Run only matching steps"),
		actionsPerMinute: fs.Float64(
			"apm",
			defaults.ActionsPerMinute,
			"Throttle browser actions in actions per minute across goto/click/type/enter/screenshot (example: --apm 60)",
		),
		clicksPerSecond: fs.Float64(
			"cps",
			defaults.ClicksPerSecond,
			"Deprecated alias for click pacing in clicks per second; prefer --apm",
		),
		forceLocalBrowser: fs.Bool("force-local-browser", defaults.ForceLocalBrowser, "Disable the WSL auto-attach fallback and keep browser tests local unless --attach is provided"),
		noSSH:             fs.Bool("no-ssh", defaults.NoSSH, "Disable SSH fallback and use direct attach only"),
		remoteNoLaunch:    fs.Bool("remote-no-launch", defaults.RemoteNoLaunch, "Do not launch remote browser when attach probe cannot reuse one"),
		remoteDebugPort:   fs.Int("remote-debug-port", defaults.RemoteDebugPort, "Preferred remote debugging port for attach"),
		remoteDebugPorts:  fs.String("remote-debug-ports", joinInts(defaults.RemoteDebugPorts), "Comma-separated remote debug ports to probe first"),
		remoteBrowserPID:  fs.Int("remote-browser-pid", defaults.RemoteBrowserPID, "Preferred remote browser PID for attach selection"),
	}
}

func (b CommonTestCLIBindings) Resolve() (CommonTestCLIOptions, error) {
	opts := CommonTestCLIOptions{}
	if b.attachNode != nil {
		opts.AttachNode = strings.TrimSpace(*b.attachNode)
	}
	if b.attachRole != nil {
		opts.AttachRole = strings.TrimSpace(*b.attachRole)
	}
	if b.defaultAttachNode != nil {
		opts.DefaultAttachNode = strings.TrimSpace(*b.defaultAttachNode)
	}
	if b.targetURL != nil {
		opts.TargetURL = strings.TrimSpace(*b.targetURL)
	}
	if b.filterExpr != nil {
		opts.FilterExpr = strings.TrimSpace(*b.filterExpr)
	}
	if b.actionsPerMinute != nil {
		opts.ActionsPerMinute = *b.actionsPerMinute
	}
	if b.clicksPerSecond != nil {
		opts.ClicksPerSecond = *b.clicksPerSecond
	}
	if b.forceLocalBrowser != nil {
		opts.ForceLocalBrowser = *b.forceLocalBrowser
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
	if opts.ActionsPerMinute < 0 {
		return CommonTestCLIOptions{}, fmt.Errorf("--apm must be >= 0")
	}
	if opts.ClicksPerSecond < 0 {
		return CommonTestCLIOptions{}, fmt.Errorf("--cps must be >= 0")
	}
	if opts.ActionsPerMinute <= 0 && opts.ClicksPerSecond > 0 {
		opts.ActionsPerMinute = opts.ClicksPerSecond * 60
	}
	return opts, nil
}

func ResolveConfiguredAttachNode(configured string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(configured)) {
	case "", "default":
		return "", false
	case "none", "off", "local":
		return "", true
	default:
		return strings.TrimSpace(configured), false
	}
}

func ResolveDefaultAttachNode(configured string) string {
	if node, disabled := ResolveConfiguredAttachNode(configured); disabled {
		return ""
	} else if node != "" {
		return node
	}
	if !IsWSLRuntime() {
		return ""
	}
	for _, node := range sshv1.ListMeshNodes() {
		if strings.EqualFold(strings.TrimSpace(node.OS), "windows") && node.PreferWSLPowerShell {
			return strings.TrimSpace(node.Name)
		}
	}
	if _, err := sshv1.ResolveMeshNode("legion"); err == nil {
		return "legion"
	}
	return ""
}

func ApplyDefaultBrowserAttach(o *CommonTestCLIOptions, defaultRole string) bool {
	if o == nil || strings.TrimSpace(o.AttachNode) != "" || o.ForceLocalBrowser {
		return false
	}
	node := ResolveDefaultAttachNode(o.DefaultAttachNode)
	if node == "" {
		return false
	}
	o.AttachNode = node
	if strings.TrimSpace(o.AttachRole) == "" {
		o.AttachRole = strings.TrimSpace(defaultRole)
		if o.AttachRole == "" {
			o.AttachRole = "test"
		}
	}
	return true
}

func (o CommonTestCLIOptions) ApplyRuntimeConfig() {
	cfg := RuntimeConfig{
		BrowserNode:       strings.TrimSpace(o.AttachNode),
		RemoteRequireRole: strings.TrimSpace(o.AttachRole) != "",
		NoSSH:             o.NoSSH,
		RemoteNoLaunch:    o.RemoteNoLaunch,
		RemoteDebugPort:   o.RemoteDebugPort,
		RemoteDebugPorts:  append([]int(nil), o.RemoteDebugPorts...),
		RemoteBrowserPID:  o.RemoteBrowserPID,
		ActionsPerMinute:  o.ActionsPerMinute,
	}
	if cfg.BrowserNode != "" {
		cfg.RemoteBrowserRole = strings.TrimSpace(o.AttachRole)
		if cfg.RemoteBrowserRole == "" {
			cfg.RemoteBrowserRole = "dev"
			cfg.RemoteRequireRole = true
		}
	}
	SetActionsPerMinute(o.ActionsPerMinute)
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
