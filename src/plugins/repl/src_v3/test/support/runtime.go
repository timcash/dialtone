package support

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

const runtimeLogSource = "src/plugins/repl/src_v3/test/support/runtime.go"

type TranscriptStep struct {
	Send         string
	ExpectRoom   []string
	ExpectOutput []string
	Timeout      time.Duration
}

type Runtime struct {
	Ctx      *testv1.StepContext
	RepoRoot string
	SrcRoot  string
	NATSURL  string
	Room     string

	leader   *exec.Cmd
	join     *exec.Cmd
	joinIn   io.WriteCloser
	joinDone chan error
	nc       *nats.Conn
	msgCh    chan string
	outCh    chan string

	recentRoom   []string
	recentOutput []string
}

func NewRuntime(ctx *testv1.StepContext) (*Runtime, error) {
	repoRoot := strings.TrimSpace(ctx.RepoRoot())
	if strings.HasSuffix(filepath.ToSlash(repoRoot), "/src") {
		repoRoot = filepath.Dir(repoRoot)
	}
	srcRoot := filepath.Join(repoRoot, "src")
	natsURL := strings.TrimSpace(ctx.NATSURL())
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:46222"
	}
	return &Runtime{
		Ctx:      ctx,
		RepoRoot: repoRoot,
		SrcRoot:  srcRoot,
		NATSURL:  natsURL,
		Room:     "index",
		joinDone: make(chan error, 1),
		msgCh:    make(chan string, 4096),
		outCh:    make(chan string, 4096),
	}, nil
}

func (rt *Runtime) StartLeader() error {
	cleanCmd := rt.newDialtoneCommand("repl", "src_v3", "process-clean")
	cleanCmd.Stdout = os.Stdout
	cleanCmd.Stderr = os.Stderr
	if err := cleanCmd.Run(); err != nil {
		return fmt.Errorf("process-clean before leader start failed: %w", err)
	}

	listenURL := listenURLFromClientURL(rt.NATSURL)
	rt.leader = rt.newDialtoneCommand("repl", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--room", rt.Room,
		"--hostname", "DIALTONE-SERVER",
	)
	if err := rt.leader.Start(); err != nil {
		return err
	}
	if err := waitForEndpoint(rt.NATSURL, 10*time.Second); err != nil {
		return err
	}
	nc, err := nats.Connect(rt.NATSURL, nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return err
	}
	rt.nc = nc
	sub, err := rt.nc.Subscribe("repl.>", func(m *nats.Msg) {
		line := string(m.Data)
		rt.rememberRoom(line)
		rt.debugf("[REPL][ROOM][%s] %s", strings.TrimSpace(m.Subject), strings.TrimSpace(line))
		select {
		case rt.msgCh <- line:
		default:
		}
	})
	if err != nil {
		return err
	}
	_ = sub
	if err := rt.nc.Flush(); err != nil {
		return err
	}
	probe := map[string]string{
		"type":    "probe",
		"from":    "repl-src-v3-test",
		"room":    rt.Room,
		"message": "probe",
	}
	rawProbe, err := json.Marshal(probe)
	if err != nil {
		return err
	}
	if err := rt.nc.Publish("repl.cmd", rawProbe); err != nil {
		return err
	}
	if err := rt.nc.Flush(); err != nil {
		return err
	}
	_, err = rt.WaitForAnyPattern(12*time.Second, []string{
		"Leader active on",
		"Leader online on",
	})
	return err
}

func (rt *Runtime) StartJoin(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "observer"
	}
	rt.join = rt.newDialtoneCommand("repl", "src_v3", "join",
		"--nats-url", rt.NATSURL,
		"--name", name,
		"--room", rt.Room,
	)
	in, err := rt.join.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := rt.join.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := rt.join.StderrPipe()
	if err != nil {
		return err
	}
	rt.joinIn = in
	if err := rt.join.Start(); err != nil {
		return err
	}
	go func() {
		rt.joinDone <- rt.join.Wait()
	}()
	go rt.captureOutput(stdout)
	go rt.captureOutput(stderr)
	p := fmt.Sprintf(`"type":"join"`)
	return rt.WaitForPatterns(12*time.Second, []string{p, fmt.Sprintf(`"from":"%s"`, name)})
}

func (rt *Runtime) SendJoinLine(line string) error {
	if rt.joinIn == nil {
		return fmt.Errorf("join session is not active")
	}
	if err := rt.checkJoinExited(); err != nil {
		return err
	}
	rt.infof("[REPL][INPUT] %s", strings.TrimSpace(line))
	if _, err := io.WriteString(rt.joinIn, strings.TrimRight(line, "\r\n")+"\n"); err != nil {
		return err
	}
	return nil
}

func (rt *Runtime) RunTranscript(steps []TranscriptStep) error {
	for i, step := range steps {
		timeout := step.Timeout
		if timeout <= 0 {
			timeout = 12 * time.Second
		}
		rt.infof("[REPL][STEP %d] send=%q expect_room=%d expect_output=%d timeout=%s", i+1, strings.TrimSpace(step.Send), len(step.ExpectRoom), len(step.ExpectOutput), timeout)
		if len(step.ExpectRoom) > 0 {
			if rt.hasSuiteNATS() {
				if err := rt.Ctx.WaitForAllMessagesAfterAction(rt.RoomSubject(), step.ExpectRoom, timeout, func() error {
					if strings.TrimSpace(step.Send) == "" {
						return nil
					}
					return rt.SendJoinLine(step.Send)
				}); err != nil {
					return fmt.Errorf("transcript step %d room expect failed: %w", i+1, err)
				}
			} else {
				if strings.TrimSpace(step.Send) != "" {
					if err := rt.SendJoinLine(step.Send); err != nil {
						return fmt.Errorf("transcript step %d send failed: %w", i+1, err)
					}
				}
				if err := rt.WaitForPatterns(timeout, step.ExpectRoom); err != nil {
					return fmt.Errorf("transcript step %d room expect failed: %w", i+1, err)
				}
			}
		} else if strings.TrimSpace(step.Send) != "" {
			if err := rt.SendJoinLine(step.Send); err != nil {
				return fmt.Errorf("transcript step %d send failed: %w", i+1, err)
			}
		}
		if len(step.ExpectOutput) > 0 {
			if err := rt.WaitForOutput(timeout, step.ExpectOutput); err != nil {
				return fmt.Errorf("transcript step %d output expect failed: %w", i+1, err)
			}
		}
		rt.infof("[REPL][STEP %d] complete", i+1)
	}
	return nil
}

func (rt *Runtime) RoomSubject() string {
	room := strings.TrimSpace(rt.Room)
	if room == "" {
		room = "index"
	}
	return "repl.room." + room
}

func (rt *Runtime) Inject(user string, args ...string) error {
	user = strings.TrimSpace(user)
	if user == "" {
		user = "llm-codex"
	}
	if len(args) == 0 {
		return fmt.Errorf("inject command args are required")
	}
	injectArgs := []string{
		"repl", "src_v3", "inject",
		"--nats-url", rt.NATSURL,
		"--room", rt.Room,
		"--user", user,
	}
	injectArgs = append(injectArgs, args...)
	cmd := rt.newDialtoneCommand(injectArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (rt *Runtime) WaitForPatterns(timeout time.Duration, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, msg := range rt.recentRoomTail() {
		for _, p := range patterns {
			if !seen[p] && strings.Contains(msg, p) {
				seen[p] = true
			}
		}
	}
	if len(seen) == len(patterns) {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for len(seen) < len(patterns) && time.Now().Before(deadline) {
		if err := rt.checkJoinExited(); err != nil {
			return err
		}
		select {
		case msg := <-rt.msgCh:
			rt.rememberRoom(msg)
			for _, p := range patterns {
				if !seen[p] && strings.Contains(msg, p) {
					seen[p] = true
				}
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	if len(seen) == len(patterns) {
		return nil
	}
	missing := make([]string, 0, len(patterns)-len(seen))
	for _, p := range patterns {
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	err := fmt.Errorf("timeout waiting for room patterns: %s\nrecent room messages:\n%s", strings.Join(missing, ", "), strings.Join(rt.recentRoomTail(), "\n"))
	rt.errorf("%v", err)
	return err
}

func (rt *Runtime) WaitForAnyPattern(timeout time.Duration, patterns []string) (string, error) {
	if len(patterns) == 0 {
		return "", fmt.Errorf("patterns are required")
	}
	for _, msg := range rt.recentRoomTail() {
		for _, p := range patterns {
			if strings.Contains(msg, p) {
				return p, nil
			}
		}
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case msg := <-rt.msgCh:
			rt.rememberRoom(msg)
			for _, p := range patterns {
				if strings.Contains(msg, p) {
					return p, nil
				}
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	err := fmt.Errorf("timeout waiting for any pattern: %s", strings.Join(patterns, ", "))
	rt.errorf("%v", err)
	return "", err
}

func (rt *Runtime) WaitForOutput(timeout time.Duration, patterns []string) error {
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, line := range rt.recentOutputTail() {
		for _, p := range patterns {
			if !seen[p] && strings.Contains(line, p) {
				seen[p] = true
			}
		}
	}
	if len(seen) == len(patterns) {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for len(seen) < len(patterns) && time.Now().Before(deadline) {
		if err := rt.checkJoinExited(); err != nil {
			return err
		}
		select {
		case line := <-rt.outCh:
			rt.rememberOutput(line)
			for _, p := range patterns {
				if !seen[p] && strings.Contains(line, p) {
					seen[p] = true
				}
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	if len(seen) == len(patterns) {
		return nil
	}
	missing := make([]string, 0, len(patterns)-len(seen))
	for _, p := range patterns {
		if !seen[p] {
			missing = append(missing, p)
		}
	}
	err := fmt.Errorf("timeout waiting for output patterns: %s\nrecent visible output:\n%s", strings.Join(missing, ", "), strings.Join(rt.recentOutputTail(), "\n"))
	rt.errorf("%v", err)
	return err
}

func (rt *Runtime) LatestSubtonePID() (int, error) {
	for i := len(rt.recentRoom) - 1; i >= 0; i-- {
		msg := strings.TrimSpace(rt.recentRoom[i])
		if msg == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(msg), &frame); err == nil {
			if raw, ok := frame["subtone_pid"]; ok {
				switch v := raw.(type) {
				case float64:
					if int(v) > 0 {
						return int(v), nil
					}
				case string:
					parsed, err := strconv.Atoi(strings.TrimSpace(v))
					if err == nil && parsed > 0 {
						return parsed, nil
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("no subtone pid found in recent room messages")
}

func (rt *Runtime) RunDialtone(args ...string) (string, error) {
	cmd := rt.newDialtoneCommand(args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (rt *Runtime) newDialtoneCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("./dialtone.sh", args...)
	cmd.Dir = rt.RepoRoot
	envFile := filepath.Join(rt.RepoRoot, "env", "dialtone.json")
	cmd.Env = append(os.Environ(),
		"DIALTONE_USE_NIX=0",
		"DIALTONE_REPO_ROOT="+rt.RepoRoot,
		"DIALTONE_SRC_ROOT="+rt.SrcRoot,
		"DIALTONE_ENV_FILE="+envFile,
		"DIALTONE_MESH_CONFIG="+envFile,
		"DIALTONE_REPL_NATS_URL="+rt.NATSURL,
	)
	if envDir := strings.TrimSpace(readConfigTopLevelString(envFile, "DIALTONE_ENV")); envDir != "" {
		cmd.Env = append(cmd.Env, "DIALTONE_ENV="+envDir)
	}
	return cmd
}

func readConfigTopLevelString(path string, key string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	value, ok := doc[key]
	if !ok {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func StandardSubtoneRoomPatterns(cmdName string, exitPattern string) []string {
	cmdName = strings.TrimSpace(cmdName)
	exitPattern = strings.TrimSpace(exitPattern)
	if exitPattern == "" && cmdName != "" {
		exitPattern = fmt.Sprintf("Subtone for %s exited with code 0.", cmdName)
	}
	patterns := []string{
		`"scope":"index"`,
		fmt.Sprintf("Request received. Spawning subtone for %s", cmdName),
		`Subtone started as pid `,
		`Subtone room: subtone-`,
		`Subtone log file: `,
	}
	if exitPattern != "" {
		patterns = append(patterns, exitPattern)
	}
	return patterns
}

func StandardSubtoneOutputPatterns(cmdName string, exitPattern string) []string {
	cmdName = strings.TrimSpace(cmdName)
	exitPattern = strings.TrimSpace(exitPattern)
	if exitPattern == "" && cmdName != "" {
		exitPattern = fmt.Sprintf("Subtone for %s exited with code 0.", cmdName)
	}
	patterns := []string{
		fmt.Sprintf("DIALTONE> Request received. Spawning subtone for %s", cmdName),
		"DIALTONE> Subtone started as pid ",
		"DIALTONE> Subtone room: subtone-",
		"DIALTONE> Subtone log file: ",
	}
	if exitPattern != "" {
		patterns = append(patterns, exitPattern)
	}
	return patterns
}

func CombinePatterns(groups ...[]string) []string {
	combined := make([]string, 0, 8)
	seen := map[string]struct{}{}
	for _, group := range groups {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; ok {
				continue
			}
			seen[item] = struct{}{}
			combined = append(combined, item)
		}
	}
	return combined
}

func (rt *Runtime) Stop() {
	if rt.joinIn != nil {
		_, _ = io.WriteString(rt.joinIn, "quit\n")
		_ = rt.joinIn.Close()
		rt.joinIn = nil
	}
	if rt.join != nil && rt.join.Process != nil {
		_ = rt.join.Process.Kill()
		rt.join = nil
	}
	if rt.nc != nil {
		rt.nc.Close()
		rt.nc = nil
	}
	if rt.leader != nil && rt.leader.Process != nil {
		_ = rt.leader.Process.Kill()
		_, _ = rt.leader.Process.Wait()
		rt.leader = nil
	}
}

func (rt *Runtime) checkJoinExited() error {
	select {
	case err := <-rt.joinDone:
		if err == nil {
			return fmt.Errorf("join session exited unexpectedly")
		}
		return fmt.Errorf("join session exited unexpectedly: %w", err)
	default:
		return nil
	}
}

func (rt *Runtime) captureOutput(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rt.rememberOutput(line)
		rt.infof("[REPL][OUT] %s", line)
		select {
		case rt.outCh <- line:
		default:
		}
	}
}

func (rt *Runtime) rememberRoom(msg string) {
	rt.recentRoom = appendBounded(rt.recentRoom, strings.TrimSpace(msg), 30)
}

func (rt *Runtime) rememberOutput(line string) {
	rt.recentOutput = appendBounded(rt.recentOutput, strings.TrimSpace(line), 30)
}

func (rt *Runtime) recentRoomTail() []string {
	if len(rt.recentRoom) == 0 {
		return []string{"<empty>"}
	}
	return rt.recentRoom
}

func (rt *Runtime) recentOutputTail() []string {
	if len(rt.recentOutput) == 0 {
		return []string{"<empty>"}
	}
	return rt.recentOutput
}

func appendBounded(items []string, value string, max int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	items = append(items, value)
	if len(items) > max {
		items = items[len(items)-max:]
	}
	return items
}

func (rt *Runtime) infof(format string, args ...any) {
	if rt.Ctx != nil {
		rt.Ctx.Infof(format, args...)
		return
	}
	logs.InfoFrom(runtimeLogSource, format, args...)
}

func (rt *Runtime) debugf(format string, args ...any) {
	if rt.Ctx != nil {
		rt.Ctx.Debugf(format, args...)
		return
	}
	logs.DebugFrom(runtimeLogSource, format, args...)
}

func (rt *Runtime) errorf(format string, args ...any) {
	if rt.Ctx != nil {
		rt.Ctx.Errorf(format, args...)
		return
	}
	logs.ErrorFrom(runtimeLogSource, format, args...)
}

func (rt *Runtime) hasSuiteNATS() bool {
	return rt.Ctx != nil && rt.Ctx.NATSConn() != nil
}

func waitForEndpoint(natsURL string, timeout time.Duration) error {
	u, err := url.Parse(strings.TrimSpace(natsURL))
	if err != nil {
		return err
	}
	host := strings.TrimSpace(u.Hostname())
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort(host, port), 700*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("nats endpoint did not become reachable at %s", natsURL)
}

func listenURLFromClientURL(clientURL string) string {
	u, err := url.Parse(strings.TrimSpace(clientURL))
	if err != nil {
		return "nats://0.0.0.0:4222"
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "4222"
	}
	return "nats://0.0.0.0:" + port
}
