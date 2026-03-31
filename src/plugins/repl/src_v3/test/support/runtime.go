package support

import (
	"bufio"
	"bytes"
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
	"sync"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

const runtimeLogSource = "src/plugins/repl/src_v3/test/support/runtime.go"

var (
	sharedRuntimeMu      sync.Mutex
	sharedRuntimeEnabled bool
	sharedRuntime        *Runtime
)

type TranscriptStep struct {
	Send            string
	ExpectRoom      []string
	ExpectRoomAny   [][]string
	ExpectOutput    []string
	ExpectOutputAny [][]string
	Timeout         time.Duration
}

type Runtime struct {
	Ctx           *testv1.StepContext
	RepoRoot      string
	SrcRoot       string
	NATSURL       string
	Room          string
	shared        bool
	startupLogged bool

	leader   *exec.Cmd
	join     *exec.Cmd
	joinName string
	joinIn   io.WriteCloser
	joinDone chan error
	nc       *nats.Conn
	msgCh    chan string
	outCh    chan string
	bufMu    sync.Mutex

	recentRoom   []string
	recentOutput []string
	recentEvents []RoomEvent
	roomEvents   []sequencedMessage
	outputEvents []sequencedMessage
	roomSeq      int
	outputSeq    int
}

type RoomEvent struct {
	Subject string
	Message string
}

type sequencedMessage struct {
	Seq     int
	Message string
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
	if !SharedRuntimeEnabled() {
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

	sharedRuntimeMu.Lock()
	defer sharedRuntimeMu.Unlock()
	if sharedRuntime == nil {
		sharedRuntime = &Runtime{
			shared:   true,
			joinDone: make(chan error, 1),
			msgCh:    make(chan string, 4096),
			outCh:    make(chan string, 4096),
		}
	}
	sharedRuntime.prepareForStep(ctx, repoRoot, srcRoot, natsURL)
	return sharedRuntime, nil
}

func EnableSharedRuntime() {
	sharedRuntimeMu.Lock()
	defer sharedRuntimeMu.Unlock()
	sharedRuntimeEnabled = true
}

func SharedRuntimeEnabled() bool {
	sharedRuntimeMu.Lock()
	defer sharedRuntimeMu.Unlock()
	return sharedRuntimeEnabled
}

func CloseSharedRuntime() {
	sharedRuntimeMu.Lock()
	rt := sharedRuntime
	sharedRuntime = nil
	sharedRuntimeEnabled = false
	sharedRuntimeMu.Unlock()
	if rt != nil {
		rt.forceStop()
	}
}

func (rt *Runtime) prepareForStep(ctx *testv1.StepContext, repoRoot, srcRoot, natsURL string) {
	rt.Ctx = ctx
	rt.RepoRoot = repoRoot
	rt.SrcRoot = srcRoot
	rt.NATSURL = natsURL
	rt.Room = "index"
	rt.resetBuffers()
}

func (rt *Runtime) resetBuffers() {
	rt.bufMu.Lock()
	rt.recentRoom = nil
	rt.recentOutput = nil
	rt.recentEvents = nil
	rt.bufMu.Unlock()
	drainStringChan(rt.msgCh)
	drainStringChan(rt.outCh)
}

func drainStringChan(ch chan string) {
	if ch == nil {
		return
	}
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func (rt *Runtime) forceStop() {
	if rt == nil {
		return
	}
	rt.startupLogged = false
	if rt.Ctx != nil {
		rt.Ctx.SetStatusPublisher(nil)
	}
	if rt.joinIn != nil {
		_, _ = io.WriteString(rt.joinIn, "quit\n")
		_ = rt.joinIn.Close()
		rt.joinIn = nil
	}
	if rt.join != nil && rt.join.Process != nil {
		_ = rt.join.Process.Kill()
		rt.join = nil
	}
	rt.joinName = ""
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

func (rt *Runtime) leaderReady() bool {
	if rt == nil || rt.leader == nil {
		return false
	}
	return waitForEndpoint(rt.NATSURL, 300*time.Millisecond) == nil
}

func (rt *Runtime) joinReady() bool {
	if rt == nil || rt.join == nil || rt.joinIn == nil {
		return false
	}
	return rt.checkJoinExited() == nil
}

func (rt *Runtime) ensureNATSSubscription() error {
	if rt.nc != nil && rt.nc.IsConnected() {
		return nil
	}
	nc, err := nats.Connect(rt.NATSURL, nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return err
	}
	sub, err := nc.Subscribe("repl.>", func(m *nats.Msg) {
		line := string(m.Data)
		rt.rememberRoom(strings.TrimSpace(m.Subject), line)
		rt.debugf("[REPL][ROOM][%s] %s", strings.TrimSpace(m.Subject), strings.TrimSpace(line))
		select {
		case rt.msgCh <- line:
		default:
		}
	})
	if err != nil {
		nc.Close()
		return err
	}
	_ = sub
	if err := nc.Flush(); err != nil {
		nc.Close()
		return err
	}
	rt.nc = nc
	return nil
}

func NewIsolatedRuntime(ctx *testv1.StepContext) (*Runtime, error) {
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
	if rt.shared && rt.leaderReady() {
		return rt.ensureNATSSubscription()
	}
	if rt.shared && rt.leader != nil && !rt.leaderReady() {
		rt.forceStop()
		rt.prepareForStep(rt.Ctx, rt.RepoRoot, rt.SrcRoot, rt.NATSURL)
	}
	cleanCmd := rt.newDialtoneCommand("repl", "src_v3", "process-clean")
	var cleanOut bytes.Buffer
	cleanCmd.Stdout = &cleanOut
	cleanCmd.Stderr = &cleanOut
	if err := cleanCmd.Run(); err != nil {
		rt.infof("[REPL][SETUP] process-clean returned nonzero before leader start: %v", err)
		if raw := strings.TrimSpace(cleanOut.String()); raw != "" {
			rt.debugf("[REPL][SETUP] process-clean output:\n%s", raw)
		}
	}

	listenURL := listenURLFromClientURL(rt.NATSURL)
	rt.leader = rt.newDialtoneCommand("repl", "src_v3", "leader",
		"--embedded-nats",
		"--nats-url", listenURL,
		"--topic", rt.Room,
		"--hostname", "DIALTONE-SERVER",
	)
	if err := rt.leader.Start(); err != nil {
		return err
	}
	if err := waitForEndpoint(rt.NATSURL, 10*time.Second); err != nil {
		return err
	}
	if err := rt.ensureNATSSubscription(); err != nil {
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
	patterns := []string{
		"Leader active on",
		"Leader online on",
	}
	deadline := time.Now().Add(12 * time.Second)
	for _, msg := range rt.recentRoomTail() {
		for _, p := range patterns {
			if strings.Contains(msg, p) {
				return nil
			}
		}
	}
	for time.Now().Before(deadline) {
		select {
		case msg := <-rt.msgCh:
			for _, p := range patterns {
				if strings.Contains(msg, p) {
					return nil
				}
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	timeoutErr := fmt.Errorf("timeout waiting for any pattern: %s", strings.Join(patterns, ", "))
	if rt.nc != nil && rt.nc.IsConnected() {
		if flushErr := rt.nc.FlushTimeout(1500 * time.Millisecond); flushErr == nil {
			rt.debugf("[REPL][SETUP] leader probe lifecycle line missing, but NATS connection stayed healthy: %v", timeoutErr)
			return nil
		}
	}
	if waitErr := waitForEndpoint(rt.NATSURL, 5*time.Second); waitErr == nil {
		rt.debugf("[REPL][SETUP] leader probe lifecycle line missing, but endpoint stayed reachable: %v", timeoutErr)
		return nil
	}
	return timeoutErr
}

func (rt *Runtime) StartJoin(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = DefaultPromptName()
	}
	if rt.shared && rt.joinReady() && strings.TrimSpace(rt.joinName) == name {
		rt.attachStatusPublisher()
		return rt.emitStartupDialog(name)
	}
	if rt.shared && rt.join != nil && !rt.joinReady() {
		if rt.join.Process != nil {
			_ = rt.join.Process.Kill()
		}
		rt.join = nil
		rt.joinIn = nil
		rt.joinDone = make(chan error, 1)
		rt.joinName = ""
	}
	if rt.shared && rt.joinReady() && strings.TrimSpace(rt.joinName) != name {
		if rt.joinIn != nil {
			_, _ = io.WriteString(rt.joinIn, "quit\n")
			_ = rt.joinIn.Close()
			rt.joinIn = nil
		}
		if rt.join != nil && rt.join.Process != nil {
			_ = rt.join.Process.Kill()
		}
		rt.join = nil
		rt.joinDone = make(chan error, 1)
		rt.joinName = ""
	}
	cmd := rt.newDialtoneCommand("repl", "src_v3", "join",
		"--nats-url", rt.NATSURL,
		"--name", name,
		"--topic", rt.Room,
	)
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	rt.join = cmd
	rt.joinIn = in
	if err := cmd.Start(); err != nil {
		return err
	}
	doneCh := rt.joinDone
	go func() {
		doneCh <- cmd.Wait()
	}()
	go rt.captureOutput(stdout)
	go rt.captureOutput(stderr)
	p := fmt.Sprintf(`"type":"join"`)
	if err := rt.WaitForPatterns(12*time.Second, []string{p, fmt.Sprintf(`"from":"%s"`, name)}); err != nil {
		return err
	}
	rt.joinName = name
	rt.attachStatusPublisher()
	return rt.emitStartupDialog(name)
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

func (rt *Runtime) attachStatusPublisher() {
	if rt == nil || rt.Ctx == nil || rt.nc == nil {
		return
	}
	rt.Ctx.SetStatusPublisher(func(kind string, msg string) {
		if err := rt.publishIndexLine(kind, msg); err != nil {
			rt.debugf("[REPL][STATUS] publish failed kind=%s msg=%q err=%v", strings.TrimSpace(kind), strings.TrimSpace(msg), err)
		}
	})
}

func (rt *Runtime) publishIndexLine(kind string, msg string) error {
	if rt == nil || rt.nc == nil {
		return fmt.Errorf("nats not connected")
	}
	frame := map[string]string{
		"type":    "line",
		"scope":   "index",
		"kind":    strings.TrimSpace(kind),
		"room":    rt.Room,
		"message": strings.TrimSpace(msg),
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := rt.nc.Publish(rt.RoomSubject(), raw); err != nil {
		return err
	}
	return rt.nc.FlushTimeout(1500 * time.Millisecond)
}

func (rt *Runtime) emitStartupDialog(name string) error {
	if rt == nil || rt.startupLogged {
		return nil
	}
	cfgPath := filepath.Join(rt.RepoRoot, "env", "dialtone.json")
	roomName := strings.TrimSpace(rt.Room)
	if roomName == "" {
		roomName = "index"
	}
	lines := []string{
		fmt.Sprintf("Shared REPL session ready on topic %s.", roomName),
		"Checking required files.",
		describePathLine("repo root", rt.RepoRoot),
		describePathLine("src/dev.go", filepath.Join(rt.SrcRoot, "dev.go")),
		describePathLine("env/dialtone.json", cfgPath),
		"Checking runtime variables.",
		describeVarLine("DIALTONE_REPO_ROOT", rt.RepoRoot),
		describeVarLine("DIALTONE_ENV_FILE", cfgPath),
		describeVarLine("DIALTONE_REPL_NATS_URL", rt.NATSURL),
		dialtoneMetadataLine(cfgPath),
	}
	for _, line := range lines {
		if err := rt.publishIndexLine("status", line); err != nil {
			return err
		}
	}
	rt.startupLogged = true
	return nil
}

func describePathLine(label string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Sprintf("%s: missing", strings.TrimSpace(label))
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("%s: %s (missing)", strings.TrimSpace(label), path)
	}
	kind := "file"
	if info.IsDir() {
		kind = "dir"
	}
	return fmt.Sprintf("%s: %s (%s)", strings.TrimSpace(label), path, kind)
}

func describeVarLine(label string, value string) string {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Sprintf("%s: set=false", label)
	}
	return fmt.Sprintf("%s: set=true value=%s", label, value)
}

func dialtoneMetadataLine(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "dialtone.json metadata: unavailable (config path empty)"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("dialtone.json metadata: unavailable (read error: %v)", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Sprintf("dialtone.json metadata: unavailable (json error: %v)", err)
	}
	meshCount, meshNames := configMeshNodeSummary(doc["mesh_nodes"])
	return fmt.Sprintf(
		"dialtone.json metadata: mesh_nodes=%d names=%s tailscale_keys=%t cloudflare_keys=%t",
		meshCount,
		strings.Join(meshNames, ","),
		configHasTailscaleKeys(doc),
		configHasCloudflareKeys(doc),
	)
}

func configMeshNodeSummary(raw any) (int, []string) {
	nodes, ok := raw.([]any)
	if !ok || len(nodes) == 0 {
		return 0, []string{"none"}
	}
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		m, ok := node.(map[string]any)
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return len(nodes), []string{"unlabeled"}
	}
	return len(nodes), out
}

func configHasTailscaleKeys(doc map[string]any) bool {
	return configHasNonEmptyString(doc, "TS_AUTHKEY") ||
		(configHasNonEmptyString(doc, "TS_API_KEY") && configHasNonEmptyString(doc, "TS_TAILNET"))
}

func configHasCloudflareKeys(doc map[string]any) bool {
	return configHasNonEmptyString(doc, "CF_TUNNEL_TOKEN_SHELL") ||
		(configHasNonEmptyString(doc, "CLOUDFLARE_API_TOKEN") && configHasNonEmptyString(doc, "CLOUDFLARE_ACCOUNT_ID"))
}

func configHasNonEmptyString(doc map[string]any, key string) bool {
	value, ok := doc[strings.TrimSpace(key)]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v)) != ""
	}
}

func (rt *Runtime) RunTranscript(steps []TranscriptStep) error {
	for i, step := range steps {
		timeout := step.Timeout
		if timeout <= 0 {
			timeout = 12 * time.Second
		}
		rt.infof("[REPL][STEP %d] send=%q expect_room=%d expect_room_any=%d expect_output=%d expect_output_any=%d timeout=%s", i+1, strings.TrimSpace(step.Send), len(step.ExpectRoom), len(step.ExpectRoomAny), len(step.ExpectOutput), len(step.ExpectOutputAny), timeout)
		roomSeq, outputSeq := rt.currentSeqs()
		deadline := time.Now().Add(timeout)
		if len(step.ExpectRoom) > 0 {
			if strings.TrimSpace(step.Send) != "" {
				if err := rt.SendJoinLine(step.Send); err != nil {
					return fmt.Errorf("transcript step %d send failed: %w", i+1, err)
				}
			}
			if err := rt.waitForPatternsAfter(remainingUntil(deadline), step.ExpectRoom, roomSeq); err != nil {
				return fmt.Errorf("transcript step %d topic expect failed: %w", i+1, err)
			}
		} else if strings.TrimSpace(step.Send) != "" {
			if err := rt.SendJoinLine(step.Send); err != nil {
				return fmt.Errorf("transcript step %d send failed: %w", i+1, err)
			}
		}
		if len(step.ExpectRoomAny) > 0 {
			if err := rt.waitForAnyPatternGroupAfter(remainingUntil(deadline), step.ExpectRoomAny, roomSeq); err != nil {
				return fmt.Errorf("transcript step %d topic-any expect failed: %w", i+1, err)
			}
		}
		if len(step.ExpectOutput) > 0 {
			if err := rt.waitForOutputAfter(remainingUntil(deadline), step.ExpectOutput, outputSeq); err != nil {
				return fmt.Errorf("transcript step %d output expect failed: %w", i+1, err)
			}
		}
		if len(step.ExpectOutputAny) > 0 {
			if err := rt.waitForAnyOutputGroupAfter(remainingUntil(deadline), step.ExpectOutputAny, outputSeq); err != nil {
				return fmt.Errorf("transcript step %d output-any expect failed: %w", i+1, err)
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
	return "repl.topic." + room
}

func (rt *Runtime) Inject(user string, args ...string) error {
	user = strings.TrimSpace(user)
	if user == "" {
		user = DefaultPromptName()
	}
	if len(args) == 0 {
		return fmt.Errorf("inject command args are required")
	}
	injectArgs := []string{
		"repl", "src_v3", "inject",
		"--nats-url", rt.NATSURL,
		"--topic", rt.Room,
		"--user", user,
	}
	injectArgs = append(injectArgs, args...)
	cmd := rt.newDialtoneCommand(injectArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (rt *Runtime) WaitForPatterns(timeout time.Duration, patterns []string) error {
	return rt.waitForPatternsAfter(timeout, patterns, 0)
}

func (rt *Runtime) WaitForPatternsAfter(timeout time.Duration, patterns []string, afterSeq int) error {
	return rt.waitForPatternsAfter(timeout, patterns, afterSeq)
}

func (rt *Runtime) WaitForAnyPatternGroupAfter(timeout time.Duration, groups [][]string, afterSeq int) error {
	return rt.waitForAnyPatternGroupAfter(timeout, groups, afterSeq)
}

func (rt *Runtime) waitForPatternsAfter(timeout time.Duration, patterns []string, afterSeq int) error {
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, msg := range rt.recentRoomTailAfter(afterSeq) {
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
	err := fmt.Errorf("timeout waiting for topic patterns: %s\nrecent topic messages:\n%s", strings.Join(missing, ", "), strings.Join(rt.recentRoomTail(), "\n"))
	rt.errorf("%v", err)
	return err
}

func (rt *Runtime) waitForAnyPatternGroupAfter(timeout time.Duration, groups [][]string, afterSeq int) error {
	groups = trimPatternGroups(groups)
	if len(groups) == 0 {
		return nil
	}
	seen := initializeGroupMatches(groups, rt.recentRoomTailAfter(afterSeq))
	if completeGroupIndex(seen, groups) >= 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := rt.checkJoinExited(); err != nil {
			return err
		}
		select {
		case msg := <-rt.msgCh:
			updateGroupMatches(seen, groups, msg)
			if completeGroupIndex(seen, groups) >= 0 {
				return nil
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	err := fmt.Errorf("timeout waiting for any topic pattern group: %s\nrecent topic messages:\n%s", describePatternGroups(groups, seen), strings.Join(rt.recentRoomTail(), "\n"))
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
	return rt.waitForOutputAfter(timeout, patterns, 0)
}

func (rt *Runtime) waitForOutputAfter(timeout time.Duration, patterns []string, afterSeq int) error {
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, line := range rt.recentOutputTailAfter(afterSeq) {
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

func (rt *Runtime) waitForAnyOutputGroupAfter(timeout time.Duration, groups [][]string, afterSeq int) error {
	groups = trimPatternGroups(groups)
	if len(groups) == 0 {
		return nil
	}
	seen := initializeGroupMatches(groups, rt.recentOutputTailAfter(afterSeq))
	if completeGroupIndex(seen, groups) >= 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := rt.checkJoinExited(); err != nil {
			return err
		}
		select {
		case line := <-rt.outCh:
			updateGroupMatches(seen, groups, line)
			if completeGroupIndex(seen, groups) >= 0 {
				return nil
			}
		case <-time.After(120 * time.Millisecond):
		}
	}
	err := fmt.Errorf("timeout waiting for any output pattern group: %s\nrecent visible output:\n%s", describePatternGroups(groups, seen), strings.Join(rt.recentOutputTail(), "\n"))
	rt.errorf("%v", err)
	return err
}

func (rt *Runtime) LatestTaskWorkerPID() (int, error) {
	rt.bufMu.Lock()
	recentRoom := append([]string(nil), rt.recentRoom...)
	rt.bufMu.Unlock()
	for i := len(recentRoom) - 1; i >= 0; i-- {
		msg := strings.TrimSpace(recentRoom[i])
		if msg == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(msg), &frame); err == nil {
			if raw, ok := frame["pid"]; ok {
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
	return 0, fmt.Errorf("no task pid found in recent topic messages")
}

func (rt *Runtime) LatestTaskID() (string, error) {
	rt.bufMu.Lock()
	recentRoom := append([]string(nil), rt.recentRoom...)
	rt.bufMu.Unlock()
	for i := len(recentRoom) - 1; i >= 0; i-- {
		msg := strings.TrimSpace(recentRoom[i])
		if msg == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(msg), &frame); err != nil {
			continue
		}
		raw, ok := frame["task_id"]
		if !ok {
			continue
		}
		taskID := strings.TrimSpace(fmt.Sprint(raw))
		if strings.HasPrefix(taskID, "task-") {
			return taskID, nil
		}
	}
	return "", fmt.Errorf("no task id found in recent topic messages")
}

func (rt *Runtime) LatestTaskIDForCommand(command string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	rt.bufMu.Lock()
	roomEvents := append([]sequencedMessage(nil), rt.roomEvents...)
	rt.bufMu.Unlock()
	for i := len(roomEvents) - 1; i >= 0; i-- {
		msg := strings.TrimSpace(roomEvents[i].Message)
		if msg == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(msg), &frame); err != nil {
			continue
		}
		message, _ := frame["message"].(string)
		if !strings.Contains(message, "Command: ["+command+"]") {
			continue
		}
		raw, ok := frame["task_id"]
		if !ok {
			continue
		}
		taskID := strings.TrimSpace(fmt.Sprint(raw))
		if strings.HasPrefix(taskID, "task-") {
			return taskID, nil
		}
	}
	return "", fmt.Errorf("no task id found for command %q", command)
}

func (rt *Runtime) WaitForTaskIDForCommand(command string, timeout time.Duration) (string, error) {
	return rt.waitForTaskIDForCommandAfter(command, timeout, 0)
}

func (rt *Runtime) WaitForTaskIDForCommandAfter(command string, timeout time.Duration, afterSeq int) (string, error) {
	return rt.waitForTaskIDForCommandAfter(command, timeout, afterSeq)
}

func (rt *Runtime) waitForTaskIDForCommandAfter(command string, timeout time.Duration, afterSeq int) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	if taskID, err := rt.latestTaskIDForCommandAfter(command, afterSeq); err == nil && strings.HasPrefix(taskID, "task-") {
		return taskID, nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := rt.checkJoinExited(); err != nil {
			return "", err
		}
		if taskID, err := rt.latestTaskIDForCommandAfter(command, afterSeq); err == nil && strings.HasPrefix(taskID, "task-") {
			return taskID, nil
		}
		select {
		case <-rt.msgCh:
		case <-time.After(120 * time.Millisecond):
		}
	}
	return "", fmt.Errorf("timeout waiting for task id for command %q", command)
}

func (rt *Runtime) LatestTaskWorkerPIDForCommand(command string) (int, error) {
	return rt.latestTaskWorkerPIDForCommandAfter(command, 0)
}

func (rt *Runtime) latestTaskIDForCommandAfter(command string, afterSeq int) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	rt.bufMu.Lock()
	roomEvents := append([]sequencedMessage(nil), rt.roomEvents...)
	rt.bufMu.Unlock()
	for i := len(roomEvents) - 1; i >= 0; i-- {
		if roomEvents[i].Seq <= afterSeq {
			continue
		}
		msg := strings.TrimSpace(roomEvents[i].Message)
		if msg == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(msg), &frame); err != nil {
			continue
		}
		message, _ := frame["message"].(string)
		if !strings.Contains(message, "Command: ["+command+"]") {
			continue
		}
		raw, ok := frame["task_id"]
		if !ok {
			continue
		}
		taskID := strings.TrimSpace(fmt.Sprint(raw))
		if strings.HasPrefix(taskID, "task-") {
			return taskID, nil
		}
	}
	return "", fmt.Errorf("no task id found for command %q", command)
}

func (rt *Runtime) latestTaskWorkerPIDForCommandAfter(command string, afterSeq int) (int, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return 0, fmt.Errorf("command is required")
	}
	rt.bufMu.Lock()
	roomEvents := append([]sequencedMessage(nil), rt.roomEvents...)
	rt.bufMu.Unlock()
	for i := len(roomEvents) - 1; i >= 0; i-- {
		if roomEvents[i].Seq <= afterSeq {
			continue
		}
		msg := strings.TrimSpace(roomEvents[i].Message)
		if msg == "" {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(msg), &frame); err != nil {
			continue
		}
		message, _ := frame["message"].(string)
		if !strings.Contains(message, "Command: ["+command+"]") {
			continue
		}
		raw, ok := frame["pid"]
		if !ok {
			continue
		}
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
	return 0, fmt.Errorf("no task pid found for command %q", command)
}

func (rt *Runtime) WaitForTaskWorkerPIDForCommand(command string, timeout time.Duration) (int, error) {
	return rt.waitForTaskWorkerPIDForCommandAfter(command, timeout, 0)
}

func (rt *Runtime) WaitForTaskWorkerPIDForCommandAfter(command string, timeout time.Duration, afterSeq int) (int, error) {
	return rt.waitForTaskWorkerPIDForCommandAfter(command, timeout, afterSeq)
}

func (rt *Runtime) LatestTaskPID() (int, error) {
	return rt.LatestTaskWorkerPID()
}

func (rt *Runtime) WaitForTaskPIDForCommand(command string, timeout time.Duration) (int, error) {
	return rt.waitForTaskWorkerPIDForCommandAfter(command, timeout, 0)
}

func (rt *Runtime) WaitForTaskPIDForCommandAfter(command string, timeout time.Duration, afterSeq int) (int, error) {
	return rt.waitForTaskWorkerPIDForCommandAfter(command, timeout, afterSeq)
}

func (rt *Runtime) waitForTaskWorkerPIDForCommandAfter(command string, timeout time.Duration, afterSeq int) (int, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return 0, fmt.Errorf("command is required")
	}
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	if pid, err := rt.latestTaskWorkerPIDForCommandAfter(command, afterSeq); err == nil && pid > 0 {
		return pid, nil
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := rt.checkJoinExited(); err != nil {
			return 0, err
		}
		if pid, err := rt.latestTaskWorkerPIDForCommandAfter(command, afterSeq); err == nil && pid > 0 {
			return pid, nil
		}
		select {
		case <-rt.msgCh:
		case <-time.After(120 * time.Millisecond):
		}
	}
	return 0, fmt.Errorf("timeout waiting for task pid for command %q", command)
}

func (rt *Runtime) RunDialtone(args ...string) (string, error) {
	cmd := rt.newDialtoneCommand(args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (rt *Runtime) WaitForSubjectPatterns(subject string, timeout time.Duration, patterns []string) error {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return fmt.Errorf("subject is required")
	}
	if len(patterns) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, event := range rt.recentEventTail(subject) {
		for _, p := range patterns {
			if !seen[p] && strings.Contains(event.Message, p) {
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
		case <-rt.msgCh:
			for _, event := range rt.recentEventTail(subject) {
				for _, p := range patterns {
					if !seen[p] && strings.Contains(event.Message, p) {
						seen[p] = true
					}
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
	return fmt.Errorf("timeout waiting for subject %s patterns: %s\nrecent subject messages:\n%s", subject, strings.Join(missing, ", "), strings.Join(rt.SubjectMessages(subject), "\n"))
}

func (rt *Runtime) SubjectMessages(subject string) []string {
	subject = strings.TrimSpace(subject)
	rt.bufMu.Lock()
	events := append([]RoomEvent(nil), rt.recentEvents...)
	rt.bufMu.Unlock()
	out := make([]string, 0, len(events))
	for _, event := range events {
		if strings.TrimSpace(event.Subject) == subject {
			out = append(out, event.Message)
		}
	}
	if len(out) == 0 {
		return []string{"<empty>"}
	}
	return out
}

func (rt *Runtime) newDialtoneCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("./dialtone.sh", args...)
	cmd.Dir = rt.RepoRoot
	envFile := filepath.Join(rt.RepoRoot, "env", "dialtone.json")
	cmd.Env = append(os.Environ(),
		"DIALTONE_USE_NIX=0",
		"DIALTONE_LOG_STDOUT=0",
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

func StandardTaskWorkerRoomPatterns(cmdName string, exitPattern string) []string {
	cmdName = strings.TrimSpace(cmdName)
	exitPattern = strings.TrimSpace(exitPattern)
	if exitPattern == "" && cmdName != "" {
		exitPattern = fmt.Sprintf("Task worker for %s exited with code 0.", cmdName)
	}
	patterns := []string{
		`"scope":"index"`,
		fmt.Sprintf("Request received. Starting task worker for %s", cmdName),
		`Task worker started as pid `,
		`Task topic: task-worker-`,
		`Task log: `,
	}
	if exitPattern != "" {
		patterns = append(patterns, exitPattern)
	}
	return patterns
}

func StandardTaskWorkerOutputPatterns(cmdName string, exitPattern string) []string {
	cmdName = strings.TrimSpace(cmdName)
	exitPattern = strings.TrimSpace(exitPattern)
	if exitPattern == "" && cmdName != "" {
		exitPattern = fmt.Sprintf("Task worker for %s exited with code 0.", cmdName)
	}
	patterns := []string{
		fmt.Sprintf("dialtone> Request received. Starting task worker for %s", cmdName),
		"dialtone> Task worker started as pid ",
		"dialtone> Task topic: task-worker-",
		"dialtone> Task log: ",
	}
	if exitPattern != "" {
		patterns = append(patterns, exitPattern)
	}
	return patterns
}

func StandardTaskRoomPatterns(exitPattern string) []string {
	exitPattern = strings.TrimSpace(exitPattern)
	if exitPattern == "" {
		exitPattern = "exited with code 0."
	}
	patterns := []string{
		`"scope":"index"`,
		`Request received.`,
		`Task queued as task-`,
		`Task topic: task.task-`,
		`Task task-`,
		`assigned pid `,
		`Task log: `,
	}
	if exitPattern != "" {
		patterns = append(patterns, exitPattern)
	}
	return patterns
}

func StandardTaskOutputPatterns(exitPattern string) []string {
	exitPattern = strings.TrimSpace(exitPattern)
	if exitPattern == "" {
		exitPattern = "exited with code 0."
	}
	patterns := []string{
		`dialtone> Request received.`,
		`dialtone> Task queued as task-`,
		`dialtone> Task topic: task.task-`,
		`dialtone> Task task-`,
		`assigned pid `,
		`dialtone> Task log: `,
	}
	if exitPattern != "" {
		patterns = append(patterns, exitPattern)
	}
	return patterns
}

func StandardCommandRoomPatternGroups(cmdName string, taskWorkerExitPattern string, taskExitPattern string) [][]string {
	return [][]string{
		StandardTaskRoomPatterns(taskExitPattern),
	}
}

func StandardCommandOutputPatternGroups(cmdName string, taskWorkerExitPattern string, taskExitPattern string) [][]string {
	return [][]string{
		StandardTaskOutputPatterns(taskExitPattern),
	}
}

func StandardShellTaskOutputPatterns() []string {
	return []string{
		`dialtone> Request received.`,
		`dialtone> Task queued as task-`,
		`dialtone> Task topic: task.task-`,
		`dialtone> Task log: `,
		`dialtone> To view the last 10 log lines: ./dialtone.sh repl src_v3 task log --task-id task-`,
		` --lines 10`,
	}
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

func CombinePatternGroups(base []string, groups ...[]string) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		out = append(out, CombinePatterns(base, group))
	}
	return out
}

func MatchAnyPatternGroupText(body string, groups [][]string) error {
	groups = trimPatternGroups(groups)
	if len(groups) == 0 {
		return nil
	}
	seen := initializeGroupMatches(groups, []string{body})
	if completeGroupIndex(seen, groups) >= 0 {
		return nil
	}
	return fmt.Errorf("text did not match any pattern group: %s", describePatternGroups(groups, seen))
}

func ParseWorkLogPath(output string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		const prefix = "dialtone> Task log: "
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if path != "" {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("work log path not found in shell output")
}

func trimPatternGroups(groups [][]string) [][]string {
	out := make([][]string, 0, len(groups))
	for _, group := range groups {
		trimmed := CombinePatterns(group)
		if len(trimmed) > 0 {
			out = append(out, trimmed)
		}
	}
	return out
}

func initializeGroupMatches(groups [][]string, existing []string) []map[string]bool {
	seen := make([]map[string]bool, len(groups))
	for i := range groups {
		seen[i] = map[string]bool{}
	}
	for _, msg := range existing {
		updateGroupMatches(seen, groups, msg)
	}
	return seen
}

func updateGroupMatches(seen []map[string]bool, groups [][]string, msg string) {
	for i, group := range groups {
		for _, pattern := range group {
			if !seen[i][pattern] && strings.Contains(msg, pattern) {
				seen[i][pattern] = true
			}
		}
	}
}

func completeGroupIndex(seen []map[string]bool, groups [][]string) int {
	for i, group := range groups {
		if len(seen[i]) == len(group) {
			return i
		}
	}
	return -1
}

func describePatternGroups(groups [][]string, seen []map[string]bool) string {
	parts := make([]string, 0, len(groups))
	for i, group := range groups {
		missing := make([]string, 0, len(group))
		for _, pattern := range group {
			if !seen[i][pattern] {
				missing = append(missing, pattern)
			}
		}
		if len(missing) == 0 {
			parts = append(parts, fmt.Sprintf("group %d matched", i+1))
			continue
		}
		parts = append(parts, fmt.Sprintf("group %d missing [%s]", i+1, strings.Join(missing, ", ")))
	}
	return strings.Join(parts, "; ")
}

func remainingUntil(deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 100 * time.Millisecond
	}
	return remaining
}

func (rt *Runtime) Stop() {
	if rt == nil {
		return
	}
	if rt.shared {
		if rt.Ctx != nil {
			rt.Ctx.SetStatusPublisher(nil)
		}
		return
	}
	rt.forceStop()
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
		if shouldPrintTranscriptLine(line) {
			_, _ = fmt.Fprintln(os.Stdout, line)
		}
		rt.debugf("[REPL][OUT] %s", line)
		select {
		case rt.outCh <- line:
		default:
		}
	}
}

func shouldPrintTranscriptLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	return strings.HasPrefix(line, "dialtone>") ||
		strings.HasPrefix(line, "dialtone:") ||
		isPromptLine(line)
}

func DefaultPromptName() string {
	host, err := os.Hostname()
	if err != nil {
		return "USER-1"
	}
	host = strings.TrimSpace(strings.ReplaceAll(host, " ", "-"))
	if host == "" {
		return "USER-1"
	}
	return host
}

func PromptLine(command string) string {
	return fmt.Sprintf("%s> %s", DefaultPromptName(), strings.TrimSpace(command))
}

func PromptFromPattern() string {
	return fmt.Sprintf(`"from":"%s"`, DefaultPromptName())
}

func isPromptLine(line string) bool {
	idx := strings.Index(line, ">")
	if idx <= 0 || idx > 64 {
		return false
	}
	prefix := strings.TrimSpace(line[:idx])
	if prefix == "" {
		return false
	}
	for _, ch := range prefix {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '-', '_', '.':
			continue
		default:
			return false
		}
	}
	return strings.TrimSpace(line[idx+1:]) != ""
}

func (rt *Runtime) rememberRoom(subject string, msg string) {
	msg = strings.TrimSpace(msg)
	rt.bufMu.Lock()
	rt.recentRoom = appendBounded(rt.recentRoom, msg, 30)
	rt.roomSeq++
	if msg == "" {
		rt.bufMu.Unlock()
		return
	}
	rt.roomEvents = appendBoundedSequenced(rt.roomEvents, sequencedMessage{
		Seq:     rt.roomSeq,
		Message: msg,
	}, 200)
	rt.recentEvents = appendBoundedEvent(rt.recentEvents, RoomEvent{
		Subject: strings.TrimSpace(subject),
		Message: msg,
	}, 200)
	rt.bufMu.Unlock()
}

func (rt *Runtime) rememberOutput(line string) {
	rt.bufMu.Lock()
	rt.outputSeq++
	rt.recentOutput = appendBounded(rt.recentOutput, strings.TrimSpace(line), 30)
	rt.outputEvents = appendBoundedSequenced(rt.outputEvents, sequencedMessage{
		Seq:     rt.outputSeq,
		Message: strings.TrimSpace(line),
	}, 200)
	rt.bufMu.Unlock()
}

func (rt *Runtime) recentRoomTail() []string {
	rt.bufMu.Lock()
	defer rt.bufMu.Unlock()
	if len(rt.recentRoom) == 0 {
		return []string{"<empty>"}
	}
	return append([]string(nil), rt.recentRoom...)
}

func (rt *Runtime) recentOutputTail() []string {
	rt.bufMu.Lock()
	defer rt.bufMu.Unlock()
	if len(rt.recentOutput) == 0 {
		return []string{"<empty>"}
	}
	return append([]string(nil), rt.recentOutput...)
}

func (rt *Runtime) recentRoomTailAfter(afterSeq int) []string {
	rt.bufMu.Lock()
	defer rt.bufMu.Unlock()
	if len(rt.roomEvents) == 0 {
		return []string{"<empty>"}
	}
	out := make([]string, 0, len(rt.roomEvents))
	for _, event := range rt.roomEvents {
		if event.Seq > afterSeq {
			out = append(out, event.Message)
		}
	}
	if len(out) == 0 {
		return []string{"<empty>"}
	}
	return out
}

func (rt *Runtime) recentOutputTailAfter(afterSeq int) []string {
	rt.bufMu.Lock()
	defer rt.bufMu.Unlock()
	if len(rt.outputEvents) == 0 {
		return []string{"<empty>"}
	}
	out := make([]string, 0, len(rt.outputEvents))
	for _, event := range rt.outputEvents {
		if event.Seq > afterSeq {
			out = append(out, event.Message)
		}
	}
	if len(out) == 0 {
		return []string{"<empty>"}
	}
	return out
}

func (rt *Runtime) currentSeqs() (int, int) {
	rt.bufMu.Lock()
	defer rt.bufMu.Unlock()
	return rt.roomSeq, rt.outputSeq
}

func (rt *Runtime) CurrentSeqs() (int, int) {
	return rt.currentSeqs()
}

func (rt *Runtime) recentEventTail(subject string) []RoomEvent {
	subject = strings.TrimSpace(subject)
	rt.bufMu.Lock()
	defer rt.bufMu.Unlock()
	if len(rt.recentEvents) == 0 {
		return nil
	}
	out := make([]RoomEvent, 0, len(rt.recentEvents))
	for _, event := range rt.recentEvents {
		if subject == "" || strings.TrimSpace(event.Subject) == subject {
			out = append(out, event)
		}
	}
	return out
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

func appendBoundedEvent(items []RoomEvent, value RoomEvent, max int) []RoomEvent {
	value.Subject = strings.TrimSpace(value.Subject)
	value.Message = strings.TrimSpace(value.Message)
	if value.Message == "" {
		return items
	}
	items = append(items, value)
	if len(items) > max {
		items = items[len(items)-max:]
	}
	return items
}

func appendBoundedSequenced(items []sequencedMessage, value sequencedMessage, max int) []sequencedMessage {
	value.Message = strings.TrimSpace(value.Message)
	if value.Message == "" {
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
