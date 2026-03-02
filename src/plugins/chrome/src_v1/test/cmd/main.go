package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type workflowStep struct {
	Name string
	Args []string
}

type remoteProc struct {
	PID    int    `json:"PID"`
	Node   string `json:"Node"`
	Origin string `json:"Origin"`
	Role   string `json:"Role"`
}

func main() {
	fs := flag.NewFlagSet("chrome test", flag.ExitOnError)
	host := fs.String("host", "all", "Target host, csv, or all")
	role := fs.String("role", "test", "Role to test")
	url := fs.String("url", "dialtone.earth", "Target URL")
	requiredHosts := fs.String("required-hosts", "darkmac,gold,legion", "Comma-separated hosts that must be running in policy checks")
	attach := fs.Bool("attach", false, "Attach to existing role sessions; do not cleanup/start/stop")
	filter := fs.String("filter", "", "Optional step name contains filter")
	_ = fs.Parse(os.Args[1:])

	targetHost := strings.TrimSpace(*host)
	targetRole := strings.TrimSpace(*role)
	targetURL := strings.TrimSpace(*url)
	required := parseCSVList(*requiredHosts)
	steps := buildWorkflowSteps(targetHost, targetRole, targetURL, *attach)

	report := &strings.Builder{}
	fmt.Fprintf(report, "# Chrome src_v1 Workflow Test\n\n")
	fmt.Fprintf(report, "Date: %s\n\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(report, "Host: `%s`  Role: `%s`  URL: `%s`  Attach: `%t`\n\n", targetHost, targetRole, targetURL, *attach)

	failed := 0
	baselineState := map[string]remoteProc{}
	for i, step := range steps {
		if !stepSelected(step.Name, strings.TrimSpace(*filter)) {
			continue
		}
		fmt.Fprintf(report, "## %d. %s\n\n", i+1, step.Name)
		fmt.Fprintf(report, "Command: `./dialtone.sh %s`\n\n", strings.Join(step.Args, " "))

		out, err := runDialtone(step.Args...)
		if strings.TrimSpace(out) != "" {
			fmt.Fprintf(report, "```\n%s\n```\n\n", strings.TrimSpace(out))
		}

		if err == nil {
			switch {
			case strings.Contains(step.Name, "count role before start"):
				state, parseErr := parseRemoteRoleState(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if len(state) != 0 {
					err = fmt.Errorf("expected zero role instances before start, got %d (%s)", len(state), formatState(state))
				}
			case strings.Contains(step.Name, "count role after first start"):
				state, parseErr := parseRemoteRoleState(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if len(state) == 0 {
					err = fmt.Errorf("expected at least one role instance after start")
				} else {
					baselineState = state
					fmt.Fprintf(report, "Assertion: started role PIDs = `%s`\n\n", formatState(state))
				}
			case strings.Contains(step.Name, "count role after reuse start"):
				state, parseErr := parseRemoteRoleState(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if !sameRoleState(baselineState, state) {
					err = fmt.Errorf("expected reuse to keep same PID per host; before=%s after=%s", formatState(baselineState), formatState(state))
				}
			case strings.Contains(step.Name, "count role after cleanup"):
				state, parseErr := parseRemoteRoleState(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if len(state) != 0 {
					err = fmt.Errorf("expected zero role instances after cleanup, got %d (%s)", len(state), formatState(state))
				}
			case strings.Contains(step.Name, "count role before attach"):
				state, parseErr := parseRemoteRoleState(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if len(state) == 0 {
					err = fmt.Errorf("attach mode requires existing role=%s browser", targetRole)
				} else {
					baselineState = state
					fmt.Fprintf(report, "Assertion: attach baseline PIDs = `%s`\n\n", formatState(state))
				}
			case strings.Contains(step.Name, "count role after attach open"):
				state, parseErr := parseRemoteRoleState(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if !sameRoleState(baselineState, state) {
					err = fmt.Errorf("attach mode must not create new instances; before=%s after=%s", formatState(baselineState), formatState(state))
				}
			case strings.Contains(step.Name, "list"):
				tabsByHost, parseErr := parseListTabsForRole(out, targetRole)
				if parseErr != nil {
					err = parseErr
				} else if len(tabsByHost) == 0 {
					err = fmt.Errorf("expected list output to include role=%s", targetRole)
				} else if hasTabMismatch(tabsByHost, 1) {
					err = fmt.Errorf("expected one tab per role instance, got %s", formatIntMap(tabsByHost))
				}
			case strings.Contains(step.Name, "policy hosts running and gold non-kiosk"):
				cmdByNode, parseErr := parseRemoteVerboseCommands(out)
				if parseErr != nil {
					err = parseErr
					break
				}
				missing := missingHosts(required, cmdByNode)
				if len(missing) > 0 {
					err = fmt.Errorf("required hosts missing running chrome: %s", strings.Join(missing, ","))
					break
				}
				if goldCmd, ok := cmdByNode["gold"]; ok && strings.Contains(strings.ToLower(goldCmd), "--kiosk") {
					err = fmt.Errorf("gold chrome must not run in kiosk mode")
				}
			}
		}

		if err != nil {
			failed++
			fmt.Fprintf(report, "Result: FAIL (`%v`)\n\n", err)
		} else {
			fmt.Fprintf(report, "Result: PASS\n\n")
		}
	}

	if writeErr := os.WriteFile("plugins/chrome/src_v1/TEST.md", []byte(report.String()), 0644); writeErr != nil {
		fmt.Fprintf(os.Stderr, "failed to write TEST.md: %v\n", writeErr)
		os.Exit(1)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "chrome workflow test failed (%d step(s))\n", failed)
		os.Exit(1)
	}
	fmt.Println("chrome workflow test passed")
}

func buildWorkflowSteps(host, role, url string, attach bool) []workflowStep {
	startCmd := []string{"chrome", "src_v1", "open", "--host", host, "--role", role, "--fullscreen", "--url", url}
	reuseCmd := []string{"chrome", "src_v1", "open", "--host", host, "--role", role, "--fullscreen", "--url", url}
	if !strings.EqualFold(strings.TrimSpace(host), "all") && !strings.Contains(host, ",") {
		startCmd = []string{"chrome", "src_v1", "remote-new", "--host", host, "--role", role, "--url", url, "--reuse-existing=false"}
		reuseCmd = []string{"chrome", "src_v1", "remote-new", "--host", host, "--role", role, "--url", url, "--reuse-existing=true"}
	}
	if attach {
		return []workflowStep{
			{Name: "count role before attach", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--json"}},
			{Name: "list before attach open", Args: []string{"chrome", "src_v1", "list", "--host", host}},
			{Name: "attach open on existing role", Args: reuseCmd},
			{Name: "list after attach open", Args: []string{"chrome", "src_v1", "list", "--host", host}},
			{Name: "count role after attach open", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--json"}},
		}
	}
	return []workflowStep{
		{Name: "cleanup role before test", Args: []string{"chrome", "src_v1", "remote-kill", "--nodes", host, "--role", role}},
		{Name: "count role before start", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--json"}},
		{Name: "start headed session on all hosts", Args: startCmd},
		{Name: "list after first start", Args: []string{"chrome", "src_v1", "list", "--host", host}},
		{Name: "count role after first start", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--json"}},
		{Name: "start again and expect reuse", Args: reuseCmd},
		{Name: "list after reuse start", Args: []string{"chrome", "src_v1", "list", "--host", host}},
		{Name: "count role after reuse start", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--json"}},
		{Name: "policy hosts running and gold non-kiosk", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--verbose"}},
		{Name: "stop role on all hosts", Args: []string{"chrome", "src_v1", "remote-kill", "--nodes", host, "--role", role}},
		{Name: "count role after cleanup", Args: []string{"chrome", "src_v1", "remote-list", "--nodes", host, "--origin", "dialtone", "--role", role, "--json"}},
	}
}

func parseCSVList(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.ToLower(strings.TrimSpace(p))
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func parseRemoteVerboseCommands(raw string) (map[string]string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	foundDash := false
	out := map[string]string{}
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "----") {
			foundDash = true
			continue
		}
		if !foundDash || strings.HasPrefix(line, "[T+") {
			continue
		}
		fields := strings.Fields(line)
		// NODE HOST PID PPID HEADLESS GPU PORT ORIGIN ROLE COMMAND...
		if len(fields) < 10 {
			continue
		}
		node := strings.ToLower(strings.TrimSpace(fields[0]))
		command := strings.Join(fields[9:], " ")
		out[node] = command
	}
	return out, nil
}

func missingHosts(required []string, cmdByNode map[string]string) []string {
	if len(required) == 0 {
		return nil
	}
	missing := make([]string, 0)
	for _, host := range required {
		if _, ok := cmdByNode[strings.ToLower(strings.TrimSpace(host))]; !ok {
			missing = append(missing, host)
		}
	}
	return missing
}

func parseRemoteRoleState(raw string, expectedRole string) (map[string]remoteProc, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return map[string]remoteProc{}, nil
	}
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start < 0 || end < start {
		return nil, fmt.Errorf("remote-list output missing JSON array")
	}
	var procs []remoteProc
	if err := json.Unmarshal([]byte(text[start:end+1]), &procs); err != nil {
		return nil, fmt.Errorf("parse remote-list json failed: %w", err)
	}
	out := map[string]remoteProc{}
	for _, p := range procs {
		if !strings.EqualFold(strings.TrimSpace(p.Origin), "dialtone") {
			continue
		}
		if expectedRole != "" && !strings.EqualFold(strings.TrimSpace(p.Role), expectedRole) {
			continue
		}
		node := strings.TrimSpace(p.Node)
		if node == "" {
			node = "unknown"
		}
		if _, exists := out[node]; exists {
			return nil, fmt.Errorf("expected max one dialtone role process per host, found duplicate on %s", node)
		}
		out[node] = p
	}
	return out, nil
}

func parseListTabsForRole(raw, expectedRole string) (map[string]int, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	role := strings.ToLower(strings.TrimSpace(expectedRole))
	foundDash := false
	out := map[string]int{}
	for _, ln := range lines {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "----") {
			foundDash = true
			continue
		}
		if !foundDash || strings.HasPrefix(line, "[T+") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		host := strings.TrimSpace(fields[0])
		lineRole := strings.ToLower(strings.TrimSpace(fields[3]))
		if role != "" && lineRole != role {
			continue
		}
		tabs, err := strconv.Atoi(strings.TrimSpace(fields[len(fields)-1]))
		if err != nil {
			return nil, fmt.Errorf("parse tabs failed for line: %s", line)
		}
		out[host] = tabs
	}
	return out, nil
}

func sameRoleState(a, b map[string]remoteProc) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va.PID != vb.PID {
			return false
		}
	}
	return true
}

func hasTabMismatch(values map[string]int, expected int) bool {
	for _, v := range values {
		if v != expected {
			return true
		}
	}
	return false
}

func formatState(state map[string]remoteProc) string {
	if len(state) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(state))
	for k := range state {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s(pid=%d)", k, state[k].PID))
	}
	return strings.Join(parts, ", ")
}

func formatIntMap(values map[string]int) string {
	if len(values) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, values[k]))
	}
	return strings.Join(parts, ", ")
}

func runDialtone(args ...string) (string, error) {
	script, err := resolveDialtoneScript()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(script, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	return out.String(), runErr
}

func stepSelected(name, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(name)), filter)
}

func resolveDialtoneScript() (string, error) {
	candidates := []string{"./dialtone.sh", "../dialtone.sh", "../../dialtone.sh"}
	for _, c := range candidates {
		p := filepath.Clean(c)
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("dialtone.sh not found from current working directory")
}
