package src_v3

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func runDaemon(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 daemon", flag.ExitOnError)
	role := fs.String("role", defaultRole, "Chrome role")
	chromePort := fs.Int("chrome-port", defaultChromePort, "Chrome debug port")
	natsPort := fs.Int("nats-port", defaultNATSPort, "NATS port")
	_ = fs.Parse(args)

	state := &daemonState{
		role:       strings.TrimSpace(*role),
		chromePort: *chromePort,
		natsPort:   *natsPort,
		profileDir: defaultProfileDir(strings.TrimSpace(*role)),
	}
	if state.role == "" {
		state.role = defaultRole
	}
	if err := state.init(); err != nil {
		return err
	}
	opts := &natsserver.Options{Host: "0.0.0.0", Port: state.natsPort}
	ns, err := natsserver.NewServer(opts)
	if err != nil {
		return err
	}
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("embedded nats failed to start on %d", state.natsPort)
	}
	nc, err := nats.Connect(fmt.Sprintf("nats://127.0.0.1:%d", state.natsPort))
	if err != nil {
		return err
	}
	defer nc.Close()
	subject := natsSubject(state.role)
	_, err = nc.Subscribe(subject, func(m *nats.Msg) {
		var req commandRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			writeReply(m, commandResponse{OK: false, Error: err.Error()})
			return
		}
		writeReply(m, state.handle(req))
	})
	if err != nil {
		return err
	}
	logs.Info("chrome src_v3 daemon ready role=%s nats=%d chrome=%d", state.role, state.natsPort, state.chromePort)
	select {}
}

func (d *daemonState) init() error {
	if d.chromePort <= 0 || d.natsPort <= 0 {
		return fmt.Errorf("invalid ports")
	}
	if err := os.MkdirAll(d.profileDir, 0755); err != nil {
		return err
	}
	path, err := findChromePath()
	if err != nil {
		return err
	}
	d.chromePath = path
	return nil
}

func (d *daemonState) handle(req commandRequest) commandResponse {
	logs.Info("chrome src_v3 daemon handle: %s", req.Command)
	resp := d.baseResponse()
	switch strings.TrimSpace(req.Command) {
	case "status":
	case "open", "goto":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		url := req.URL
		if url == "" && req.Command == "open" {
			url = "about:blank"
		}
		if url == "" {
			resp.OK = false
			resp.Error = "goto requires url"
			return resp
		}
		logs.Info("chrome src_v3 daemon navigating to: %s", url)
		if err := d.navigateManaged(url); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "get-url", "tabs":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
	case "tab-open":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		url := req.URL
		if url == "" {
			url = "about:blank"
		}
		if err := d.openNewTab(url); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "tab-close":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.closeTab(req.Index); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "click-aria":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.clickAriaLabel(req.AriaLabel); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "type-aria":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.typeAriaLabel(req.AriaLabel, req.Value); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "press-enter-aria":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.pressEnterAriaLabel(req.AriaLabel); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "wait-aria":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.waitForAriaLabel(req.AriaLabel, time.Duration(req.TimeoutMS)*time.Millisecond); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "wait-aria-attr":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.waitForAriaLabelAttrEquals(req.AriaLabel, req.Attr, req.Expected, time.Duration(req.TimeoutMS)*time.Millisecond); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "get-aria-attr":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		value, err := d.readAriaLabelAttr(req.AriaLabel, req.Attr)
		if err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
		resp.Value = value
	case "set-html":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		if err := d.setManagedHTML(req.Value); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "wait-log":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		lines, err := d.waitForConsoleContains(req.Contains, time.Duration(req.TimeoutMS)*time.Millisecond)
		if err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
		resp.ConsoleLines = lines
	case "console":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		resp.ConsoleLines = d.consoleSnapshot()
	case "screenshot":
		if err := d.ensureBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return resp
		}
		b64, err := d.captureScreenshotB64()
		if err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
		resp.ScreenshotB64 = b64
	case "close":
		if err := d.closeBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
	case "reset":
		if err := d.closeBrowser(); err != nil {
			resp.OK = false
			resp.Error = err.Error()
			return d.refreshResponse(resp)
		}
		if runtime.GOOS == "windows" {
			if err := cleanupChromeProfileLocks(d.profileDir); err != nil {
				logs.Error("chrome src_v3 reset cleanup failed: %v", err)
			}
		}
	default:
		resp.OK = false
		resp.Error = fmt.Sprintf("unsupported command: %s", req.Command)
		return resp
	}
	resp = d.refreshResponse(resp)
	resp.OK = true
	return resp
}

func (d *daemonState) baseResponse() commandResponse {
	d.mu.Lock()
	defer d.mu.Unlock()
	resp := commandResponse{
		ServicePID: os.Getpid(),
		BrowserPID: d.browserPID,
		ChromePort: d.chromePort,
		NATSPort:   d.natsPort,
		Role:       d.role,
		ProfileDir: d.profileDir,
	}
	if d.managedTarget != "" {
		resp.ManagedTarget = d.managedTarget
	}
	resp.CurrentURL = d.currentURL
	if d.unexpectedErr != nil {
		resp.Unhealthy = true
		resp.Error = d.unexpectedErr.Error()
	}
	return resp
}

func (d *daemonState) refreshResponse(resp commandResponse) commandResponse {
	_ = d.fillStatus(&resp)
	return resp
}

func (d *daemonState) fillStatus(resp *commandResponse) error {
	d.mu.Lock()
	pid := d.browserPID
	resp.BrowserPID = pid
	resp.ChromePort = d.chromePort
	resp.NATSPort = d.natsPort
	resp.CurrentURL = d.currentURL
	if d.managedTarget != "" {
		resp.ManagedTarget = d.managedTarget
	}
	if d.unexpectedErr != nil {
		resp.Unhealthy = true
		if resp.Error == "" {
			resp.Error = d.unexpectedErr.Error()
		}
	}
	d.mu.Unlock()

	if pid == 0 {
		return nil
	}
	tabs, err := d.listTabs()
	if err != nil {
		logs.Error("chrome src_v3 listTabs failed: %v", err)
		return err
	}
	resp.Tabs = tabs
	resp.CurrentURL = d.currentURLFromTabs(tabs)
	return nil
}

func sendRemoteCommand(node sshv1.MeshNode, req commandRequest) (*commandResponse, error) {
	subject := natsSubject(req.Role)
	natsURL := fmt.Sprintf("nats://%s:%d", preferredHost(node), defaultNATSPort)
	nc, err := nats.Connect(natsURL, nats.Timeout(defaultTimeout))
	if err != nil {
		return nil, err
	}
	defer nc.Close()
	raw, _ := json.Marshal(req)
	msg, err := nc.Request(subject, raw, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var resp commandResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, err
	}
	if !resp.OK && strings.TrimSpace(resp.Error) != "" {
		return &resp, fmt.Errorf(resp.Error)
	}
	return &resp, nil
}

func printResponse(resp *commandResponse) {
	if resp == nil {
		return
	}
	fmt.Printf("role=%s service_pid=%d browser_pid=%d chrome_port=%d nats_port=%d tabs=%d current_url=%s managed_target=%s unhealthy=%t error=%s\n",
		resp.Role, resp.ServicePID, resp.BrowserPID, resp.ChromePort, resp.NATSPort, len(resp.Tabs), strings.TrimSpace(resp.CurrentURL), strings.TrimSpace(resp.ManagedTarget), resp.Unhealthy, strings.TrimSpace(resp.Error))
	for i, tab := range resp.Tabs {
		fmt.Printf("TAB %d id=%s url=%s\n", i, tab.ID, tab.URL)
	}
	for i, line := range resp.ConsoleLines {
		fmt.Printf("LOG %d %s\n", i, line)
	}
	if strings.TrimSpace(resp.ScreenshotB64) != "" {
		fmt.Printf("SCREENSHOT_B64_LEN %d\n", len(resp.ScreenshotB64))
	}
}

func writeReply(m *nats.Msg, resp commandResponse) {
	data, _ := json.Marshal(resp)
	_ = m.Respond(data)
}

func natsSubject(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		role = defaultRole
	}
	return "chrome.src_v3." + role + ".cmd"
}
