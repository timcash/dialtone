package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	neturl "net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	nserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type typingMeshNode struct {
	Name           string   `json:"name"`
	Aliases        []string `json:"aliases"`
	User           string   `json:"user"`
	Host           string   `json:"host"`
	Port           string   `json:"port"`
	OS             string   `json:"os"`
	PreferWSLPowerShell bool `json:"prefer_wsl_powershell"`
	HostCandidates []string `json:"host_candidates"`
	RepoCandidates []string `json:"repo_candidates"`
}

type terminalProcRow struct {
	Process   string
	PID       string
	User      string
	Session   string
	SessionID string
	Windowed  string
	Title     string
}

type terminalServiceExecRequest struct {
	Command string `json:"command"`
	Repo    string `json:"repo,omitempty"`
	LogPath string `json:"log_path,omitempty"`
}

type terminalServiceListRequest struct {
	LogPath string `json:"log_path,omitempty"`
}

type terminalServiceStopRequest struct {
	All        bool `json:"all,omitempty"`
	WSLPrompts bool `json:"wsl_prompts,omitempty"`
	LogPath    string `json:"log_path,omitempty"`
}

type terminalTrackedRow struct {
	PID     int    `json:"pid"`
	Alive   bool   `json:"alive"`
	Visible bool   `json:"visible"`
	Title   string `json:"title"`
	WSL     string `json:"wsl"`
}

type terminalServiceResponse struct {
	OK      bool            `json:"ok"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "help", "-h", "--help":
		printUsage()
	case "type":
		if err := runTerminal(args); err != nil {
			exitIfErr(err, "terminal type")
		}
	case "service":
		if err := runTerminalService(args); err != nil {
			exitIfErr(err, "terminal service")
		}
	case "test":
		if err := runTerminalTest(args); err != nil {
			exitIfErr(err, "terminal test")
		}
	case "list":
		if err := runTerminalList(args); err != nil {
			exitIfErr(err, "terminal list")
		}
	case "close":
		if err := runTerminalClose(args); err != nil {
			exitIfErr(err, "terminal close")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown terminal command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runGhostty(argv []string) error {
	opts := flag.NewFlagSet("typing v1 ghostty", flag.ContinueOnError)
	host := opts.String("host", "gold", "Mesh host alias/name for target machine")
	user := opts.String("user", "", "SSH user for the target host")
	port := opts.String("port", "", "SSH port for the target host")
	repoPath := opts.String("repo", "", "Repository path to cd into before typing command")
	command := opts.String("command", "./dialtone_mod", "Command to type in Ghostty (default from positional args)")
	dryRun := opts.Bool("dry-run", false, "Print generated script without typing it")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	if strings.TrimSpace(*command) == "" && opts.NArg() > 0 {
		*command = strings.Join(opts.Args(), " ")
	}

	commandText := strings.TrimSpace(*command)
	if commandText == "" {
		return errors.New("typing command is required (use --command or positional args)")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	target, typedRepoPath, err := resolveTargetNode(repoRoot, strings.TrimSpace(*host), strings.TrimSpace(*user), strings.TrimSpace(*repoPath))
	if err != nil {
		return err
	}

	if strings.TrimSpace(*port) != "" {
		target.Port = strings.TrimSpace(*port)
	}
	if target.Port == "" {
		target.Port = "22"
	}
	if target.Host == "" {
		target.Host = strings.TrimSpace(*host)
	}

	hostForSSH := pickHostForSSH(target)
	if strings.TrimSpace(typedRepoPath) != "" {
		commandText = "cd " + typedRepoPath + " && " + commandText
	}
	targetAddr := target.User + "@" + hostForSSH

	scriptText, typedCommandText := buildGhosttyAppleScriptText(commandText)
	if *dryRun {
		fmt.Printf("typing target: %s\n", targetAddr)
		fmt.Println("appleScript:")
		fmt.Println(scriptText)
		fmt.Printf("typed command: %s\n", typedCommandText)
		return nil
	}

	return runTypingSSH(targetAddr, target.Port, scriptText)
}

func runTerminal(argv []string) error {
	opts := flag.NewFlagSet("terminal v1 type", flag.ContinueOnError)
	repoPath := opts.String("repo", "", "Repository path to cd into before opening terminal")
	command := opts.String("command", "", "Command to run/type in local terminal")
	logPath := opts.String("log-path", "C:\\Users\\Public\\dialtone-typing-terminal.log", "Windows log file path for local terminal launch events")
	powershellPath := opts.String("powershell-path", defaultPowerShellPath(), "Path to powershell.exe for local launcher")
	wslPath := opts.String("wsl-path", defaultWSLPath(), "Path to wsl.exe when launching local Windows terminal")
	windowsTerminalPath := opts.String("wt-path", defaultWindowsTerminalPath(), "Path to wt.exe when running locally on Windows from WSL")
	windowsTerminalProfile := opts.String("wt-profile", "", "Windows Terminal profile to use for local WSL terminal launches")
	serviceURL := opts.String("service-url", "nats://127.0.0.1:47222", "NATS URL for terminal service request/reply")
	injectIfBlocked := opts.Bool("inject-if-blocked", true, "If queued command does not start quickly, inject keystrokes into tagged terminal window")
	dryRun := opts.Bool("dry-run", false, "Print generated command without opening terminal")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	if strings.TrimSpace(*command) == "" && opts.NArg() > 0 {
		*command = strings.Join(opts.Args(), " ")
	}

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	isLocalMacTerminal := runtime.GOOS == "darwin"
	localRepoPath := strings.TrimSpace(*repoPath)
	typedLocalCommand := buildLocalTypedCommand(strings.TrimSpace(*command), localRepoPath)
	trimmedWTP := strings.TrimSpace(*windowsTerminalProfile)
	localPowerShellPath := strings.TrimSpace(*powershellPath)
	localWSLPath := resolveWSLPath(*wslPath)
	localTerminalPath := strings.TrimSpace(*windowsTerminalPath)
	localLogPath := strings.TrimSpace(*logPath)
	serviceURLTrimmed := strings.TrimSpace(*serviceURL)

	if *dryRun {
		if isLocalMacTerminal {
			scriptText, typedText := buildGhosttyAppleScriptText(typedLocalCommand)
			fmt.Println("appleScript:")
			fmt.Println(scriptText)
			fmt.Printf("typed command: %s\n", typedText)
		} else {
			scriptCommand, err := buildLocalLauncherScriptCommand(repoRoot, localWSLPath, localTerminalPath, trimmedWTP, localLogPath, typedLocalCommand)
			if err != nil {
				return err
			}
			fmt.Printf("powershell script command:\n%s\n", scriptCommand)
		}
		return nil
	}

	if os.Getenv("DIALTONE_TERMINAL_SERVICE_INTERNAL") != "1" && strings.TrimSpace(typedLocalCommand) != "" {
		handled, err := maybeProxyTypeViaService(serviceURLTrimmed, localLogPath, localRepoPath, typedLocalCommand)
		if err != nil {
			return err
		}
		if handled {
			return nil
		}
		return fmt.Errorf("terminal service not reachable at %s; run `./dialtone_mod terminal v1 service start`", serviceURLTrimmed)
	}

	if isLocalMacTerminal {
		return runTypingLocalGhosttySession(typedLocalCommand)
	}
	if err := runTypingLocalPowerShellSession(repoRoot, typedLocalCommand, localPowerShellPath, localTerminalPath, localWSLPath, trimmedWTP, localLogPath); err != nil {
		return err
	}
	if strings.TrimSpace(typedLocalCommand) != "" {
		if warnErr := waitForCommandExecStart(localLogPath, typedLocalCommand, 3*time.Second); warnErr != nil {
			fmt.Fprintf(os.Stderr, "warning: command not observed in terminal log yet: %v\n", warnErr)
			if *injectIfBlocked && !isLocalMacTerminal {
				if injectErr := injectCommandIntoTaggedTerminal(localLogPath, typedLocalCommand, localPowerShellPath); injectErr != nil {
					fmt.Fprintf(os.Stderr, "warning: typing injection failed: %v\n", injectErr)
				} else {
					fmt.Fprintf(os.Stderr, "info: command injected into active tagged terminal window\n")
				}
			}
		}
	}
	return nil
}

func runTerminalTest(argv []string) error {
	opts := flag.NewFlagSet("terminal v1 test", flag.ContinueOnError)
	logPath := opts.String("log-path", "C:\\Users\\Public\\dialtone-typing-terminal.log", "Windows log file path used by local launcher")
	timeoutSeconds := opts.Int("timeout-seconds", 20, "Max seconds to wait for transcript token")
	testWSL := opts.Bool("wsl", false, "Also enter an interactive WSL shell in the same terminal and verify it")
	if err := opts.Parse(argv); err != nil {
		return err
	}

	if runtime.GOOS != "windows" && !isTypingLocalWSL() {
		return errors.New("terminal test requires Windows or WSL")
	}

	launchPath := wslPathForLog(*logPath)
	launchLogPath := launchPath
	eventLogPath := launchPath + ".events.log"
	statePath := launchPath + ".state.json"
	tagPath := launchPath + ".tagged-pids.txt"
	queuePath := launchPath + ".queue.txt"
	windowLogPath := launchPath + ".window.log"
	token := fmt.Sprintf("DIALTONE_TERMINAL_TEST_%d", time.Now().Unix())

	if tagged, tagErr := readTaggedTerminalPIDs(tagPath); tagErr == nil {
		for _, pid := range tagged {
			_ = stopProcessByPID(pid)
		}
		time.Sleep(700 * time.Millisecond)
	}
	if oldPID, err := readStatePID(statePath); err == nil && oldPID > 0 {
		_ = stopProcessByPID(oldPID)
		time.Sleep(600 * time.Millisecond)
	}
	if err := stopDialtonePowerShellWindows(); err != nil {
		return fmt.Errorf("cleanup existing Dialtone terminals failed: %w", err)
	}
	time.Sleep(800 * time.Millisecond)
	beforeVisible, err := countVisibleDialtonePowerShellWindows()
	if err != nil {
		return fmt.Errorf("count before launch failed: %w", err)
	}
	if beforeVisible != 0 {
		return fmt.Errorf("expected 0 visible DialtoneTyping windows before test, got %d", beforeVisible)
	}

	_ = os.Remove(statePath)
	_ = os.Remove(tagPath)
	_ = os.Remove(queuePath)
	_ = os.Remove(launchLogPath)
	_ = os.Remove(eventLogPath)
	_ = os.Remove(windowLogPath)
	_ = os.WriteFile(launchLogPath, []byte{}, 0644)
	_ = os.WriteFile(eventLogPath, []byte{}, 0644)
	_ = os.WriteFile(windowLogPath, []byte{}, 0644)

	step1 := fmt.Sprintf("Write-Host '%s step=1'", token)
	step2 := fmt.Sprintf("Write-Host '%s step=2'", token)
	wslToken := fmt.Sprintf("DIALTONE_WSL_TEST_%d", time.Now().Unix())
	injectToken := fmt.Sprintf("DIALTONE_INJECT_TEST_%d", time.Now().Unix())
	injectProofPath := "/mnt/c/Users/Public/dialtone-terminal-inject-proof.log"
	wslCommandMarker := ""
	if err := runTerminalInternalDirect([]string{"--log-path", *logPath, "--command", step1}); err != nil {
		return fmt.Errorf("step1 failed: %w", err)
	}
	time.Sleep(1 * time.Second)
	if err := runTerminalInternalDirect([]string{"--log-path", *logPath, "--command", step2}); err != nil {
		return fmt.Errorf("step2 failed: %w", err)
	}
	requiredReuse := 1
	requiredCommands := []string{step1, step2}
	wslCommand := ""
	if *testWSL {
		wslBinary := windowsPathForCommand(resolveWSLPath(""))
		if strings.TrimSpace(wslBinary) == "" {
			wslBinary = "C:\\Windows\\System32\\wsl.exe"
		}
		wslBootstrap := "export XDG_RUNTIME_DIR=/tmp/xdg-runtime-$UID; mkdir -p \"$XDG_RUNTIME_DIR\"; chmod 700 \"$XDG_RUNTIME_DIR\"; echo " + wslToken + "; exec /bin/bash --noprofile --norc -i"
		wslCmd := "& " + powerShellSingleQuote(wslBinary) + " --cd ~ -e bash --noprofile --norc -ic " + powerShellSingleQuote(wslBootstrap)
		wslCommandMarker = "--cd ~ -e bash --noprofile --norc -ic"
		wslCommand = wslCmd
		time.Sleep(500 * time.Millisecond)
		if err := runTerminalInternalDirect([]string{"--log-path", *logPath, "--command", wslCmd}); err != nil {
			return fmt.Errorf("wsl step failed: %w", err)
		}
		requiredReuse = 2
		requiredCommands = append(requiredCommands, wslCmd)

		_ = os.Remove(injectProofPath)
		injectCmd := "echo " + injectToken + " >> " + injectProofPath
		if err := runTerminalInternalDirect([]string{"--log-path", *logPath, "--command", injectCmd}); err != nil {
			return fmt.Errorf("wsl injection step failed: %w", err)
		}
		requiredReuse = 3
		requiredCommands = append(requiredCommands, injectCmd)
	}

	deadline := time.Now().Add(time.Duration(*timeoutSeconds) * time.Second)
	lastLaunch := 0
	lastReuse := 0
	lastQueueOK := false
	lastStepOK := false
	lastWSLOK := !*testWSL
	lastInjectOK := !*testWSL
	for time.Now().Before(deadline) {
		queueOK, _ := queueContainsAllCommands(queuePath, requiredCommands)
		lastQueueOK = queueOK

		launchCount := 0
		reuseCount := 0
		launchText := ""
		eventText := ""
		if launchRaw, launchErr := os.ReadFile(launchLogPath); launchErr == nil {
			launchText = string(launchRaw)
			launchCount = strings.Count(launchText, "launched powershell window pid=")
			reuseCount = strings.Count(launchText, "reusing existing window pid=")
		}
		if eventRaw, eventErr := os.ReadFile(eventLogPath); eventErr == nil {
			eventText = string(eventRaw)
		}
		lastLaunch = launchCount
		lastReuse = reuseCount

		stepOK := strings.Contains(eventText, "exec-start "+step1) &&
			strings.Contains(eventText, "exec-end "+step1) &&
			strings.Contains(eventText, "exec-start "+step2) &&
			strings.Contains(eventText, "exec-end "+step2)
		lastStepOK = stepOK

		wslOK := true
		if *testWSL {
			wslOK = false
			wslOK = strings.Contains(eventText, "exec-start "+wslCommand) &&
				!strings.Contains(eventText, "exec-end "+wslCommand)
		}
		lastWSLOK = wslOK
		injectOK := true
		if *testWSL {
			injectOK, _ = fileContainsToken(injectProofPath, injectToken)
		}
		lastInjectOK = injectOK

		if queueOK && stepOK && launchCount == 1 && reuseCount >= requiredReuse && wslOK && injectOK {
			if *testWSL {
				fmt.Printf("PASS token=%s wsl_token=%s inject_token=%s wsl_interactive=true injected=true reused=true launched=%d reused=%d marker=%s queue=%s inject_log=%s\n", token, wslToken, injectToken, launchCount, reuseCount, wslCommandMarker, queuePath, injectProofPath)
			} else {
				fmt.Printf("PASS token=%s reused=true launched=%d reused=%d queue=%s\n", token, launchCount, reuseCount, queuePath)
			}
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for terminal validation queue_ok=%t step_ok=%t launched=%d reused=%d wsl_ok=%t inject_ok=%t queue=%s launch_log=%s transcript=%s inject_log=%s", lastQueueOK, lastStepOK, lastLaunch, lastReuse, lastWSLOK, lastInjectOK, queuePath, launchLogPath, windowLogPath, injectProofPath)
}

func runTerminalList(argv []string) error {
	opts := flag.NewFlagSet("terminal v1 list", flag.ContinueOnError)
	logPath := opts.String("log-path", "C:\\Users\\Public\\dialtone-typing-terminal.log", "Windows log file path used by local launcher")
	showAll := opts.Bool("all", false, "Also list all visible terminal windows (PowerShell/CMD/WindowsTerminal)")
	useAdmin := opts.Bool("admin", false, "Use elevated Windows PowerShell for all-process terminal listing")
	serviceURL := opts.String("service-url", "nats://127.0.0.1:47222", "NATS URL for terminal service request/reply")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if runtime.GOOS != "windows" && !isTypingLocalWSL() {
		return errors.New("terminal list requires Windows or WSL")
	}
	if os.Getenv("DIALTONE_TERMINAL_SERVICE_INTERNAL") != "1" {
		if handled, err := maybeProxyListViaService(strings.TrimSpace(*serviceURL), strings.TrimSpace(*logPath), *showAll, *useAdmin); handled {
			return err
		}
		return fmt.Errorf("terminal service not reachable at %s; run `./dialtone_mod terminal v1 service start`", strings.TrimSpace(*serviceURL))
	}

	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return errors.New("powershell not found")
	}

	launchLog := wslPathForLog(*logPath)
	pids, err := readTaggedTerminalPIDs(launchLog + ".tagged-pids.txt")
	if err != nil || len(pids) == 0 {
		raw, readErr := os.ReadFile(launchLog)
		if readErr != nil {
			return fmt.Errorf("failed reading launch log %s: %w", launchLog, readErr)
		}
		re := regexp.MustCompile(`launched powershell window pid=(\d+)`)
		matches := re.FindAllStringSubmatch(string(raw), -1)
		seen := map[int]struct{}{}
		pids = make([]int, 0, len(matches))
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			pid, convErr := strconv.Atoi(strings.TrimSpace(m[1]))
			if convErr != nil || pid <= 0 {
				continue
			}
			if _, ok := seen[pid]; ok {
				continue
			}
			seen[pid] = struct{}{}
			pids = append(pids, pid)
		}
	}

	type trackedRow struct {
		PID     int
		Visible bool
		Title   string
		WSL     string
	}
	rows := []trackedRow{}
	for _, pid := range pids {
		title, visible, alive, infoErr := getProcessWindowInfo(pid)
		if infoErr != nil || !alive {
			continue
		}
		wslState := "false"
		hasWSL, wslErr := hasChildWSLProcess(pid)
		if wslErr != nil {
			wslState = "unknown"
		} else if hasWSL {
			wslState = "true"
		}
		rows = append(rows, trackedRow{
			PID:     pid,
			Visible: visible,
			Title:   title,
			WSL:     wslState,
		})
	}

	fmt.Printf("open=%d tracked_pids=%d\n", len(rows), len(pids))
	if len(rows) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PID\tVISIBLE\tWSL\tTITLE")
		for _, row := range rows {
			fmt.Fprintf(w, "%d\t%t\t%s\t%s\n", row.PID, row.Visible, row.WSL, row.Title)
		}
		_ = w.Flush()
	}

	if *showAll {
		allEntries, err := listAllVisibleTerminalWindows()
		if err != nil {
			return err
		}
		fmt.Printf("all_visible=%d\n", len(allEntries))
		printTerminalProcTable(allEntries)
		taskEntries, err := listAllTerminalProcessesFromTasklist()
		if *useAdmin {
			taskEntries, err = listAllTerminalProcessesFromTasklistAdmin()
		}
		if err != nil {
			return err
		}
		fmt.Printf("all_terminal_processes=%d\n", len(taskEntries))
		printTerminalProcTable(taskEntries)
	}
	return nil
}

func runTypingSSH(targetAddress, port string, scriptText string) error {
	args := []string{
		"-F", "/dev/null",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	}
	if strings.TrimSpace(port) != "" && strings.TrimSpace(port) != "22" {
		args = append(args, "-p", strings.TrimSpace(port))
	}
	remoteCmd := buildGhosttyTypingCommand(scriptText)
	args = append(args, targetAddress, remoteCmd)

	cmd := exec.Command("ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTypingSSHInteractive(targetAddress, port string, shellCommand string) error {
	args := buildSSHArgsForInteractiveTyping(targetAddress, port)
	args = append(args, "bash", "-lc", shellCommand)

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTypingPowerShellSession(targetAddress, port, shellCommand, powershellPath string) error {
	powerShellPath := resolvePowerShellPath(powershellPath)
	if powerShellPath == "" {
		return errors.New("powershell not found; set powershell.exe in PATH or use linux/darwin terminal type host")
	}
	command := buildPowerShellTerminalCommand(targetAddress, port, shellCommand)
	cmd := exec.Command(powerShellPath,
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		command,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runTypingLocalPowerShellSession(repoRoot, shellCommand, powershellPath, wtPathOverride, wslPath, wtProfile, logPath string) error {
	command := trimForLocalPowerShellTerminal(shellCommand)
	localPowerShellPath := resolvePowerShellPath(powershellPath)
	if localPowerShellPath == "" {
		return errors.New("powershell not found; set powershell.exe in PATH or use linux/darwin terminal type host")
	}
	scriptCommand, err := buildLocalLauncherScriptCommand(repoRoot, wslPath, wtPathOverride, wtProfile, logPath, command)
	if err != nil {
		return err
	}
	cmd := exec.Command(localPowerShellPath,
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy",
		"Bypass",
		"-Command",
		scriptCommand,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildLocalLauncherScriptCommand(repoRoot, wslPath, wtPathOverride, wtProfile, logPath, command string) (string, error) {
	scriptPath := filepath.Join(repoRoot, "src", "mods", "terminal", "v1", "launch_local_terminal.ps1")
	scriptBody, err := os.ReadFile(scriptPath)
	if err != nil {
		return "", fmt.Errorf("failed reading local launcher script: %w", err)
	}
	wslBinary := strings.TrimSpace(windowsPathForCommand(wslPath))
	if wslBinary == "" {
		wslBinary = strings.TrimSpace(windowsPathForCommand(defaultWSLPath()))
	}
	if wslBinary == "" {
		wslBinary = "wsl.exe"
	}
	wtBinary := strings.TrimSpace(windowsPathForCommand(resolveWindowsTerminalPath(wtPathOverride)))
	if wtBinary == "" {
		wtBinary = strings.TrimSpace(windowsPathForCommand(wtPathOverride))
	}
	trimmedLogPath := strings.TrimSpace(logPath)
	if trimmedLogPath == "" {
		trimmedLogPath = "C:\\Users\\Public\\dialtone-typing-terminal.log"
	}
	cmd := strings.TrimSpace(string(scriptBody)) + "\n" +
		"Start-DialtoneLocalTerminal " +
		"-WslPath " + powerShellSingleQuote(wslBinary) + " " +
		"-CommandText " + powerShellSingleQuote(command) + " " +
		"-LogPath " + powerShellSingleQuote(trimmedLogPath) + " " +
		"-WtPath " + powerShellSingleQuote(wtBinary) + " " +
		"-WtProfile " + powerShellSingleQuote(strings.TrimSpace(wtProfile))
	return cmd, nil
}

func resolveTargetNode(repoRoot, host, explicitUser, explicitRepo string) (typingMeshNode, string, error) {
	nodes, _ := loadTypingMeshConfig(repoRoot)
	selected := typingMeshNode{Name: host}
	if host == "" {
		return selected, explicitRepo, errors.New("host is required")
	}
	if len(nodes) > 0 {
		if match, ok := findTypingMeshNode(nodes, host); ok {
			selected = match
		}
	}

	if strings.TrimSpace(explicitUser) != "" {
		selected.User = strings.TrimSpace(explicitUser)
	}
	if strings.TrimSpace(selected.User) == "" {
		selected.User = os.Getenv("USER")
	}

	repoPath := strings.TrimSpace(explicitRepo)
	if repoPath == "" && len(selected.RepoCandidates) > 0 {
		repoPath = selected.RepoCandidates[0]
	}
	if strings.TrimSpace(repoPath) == "" {
		repoPath = filepath.Join(os.Getenv("HOME"), "dialtone")
	}
	return selected, repoPath, nil
}

func loadTypingMeshConfig(repoRoot string) ([]typingMeshNode, error) {
	configPath := filepath.Join(repoRoot, "env", "mesh.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	nodes := []typingMeshNode{}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

func findTypingMeshNode(nodes []typingMeshNode, rawHost string) (typingMeshNode, bool) {
	target := strings.ToLower(strings.TrimSpace(rawHost))
	target = strings.TrimSuffix(target, ".")
	for _, node := range nodes {
		if normalizeTypingHost(node.Name) == target {
			return node, true
		}
		for _, alias := range node.Aliases {
			if normalizeTypingHost(alias) == target {
				return node, true
			}
		}
	}
	return typingMeshNode{}, false
}

func pickHostForSSH(node typingMeshNode) string {
	candidates := append([]string{}, node.HostCandidates...)
	candidates = append(candidates, node.Aliases...)
	candidates = append(candidates, node.Host)
	for _, candidate := range preferTailnetHostsInTyping(candidates) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if isIPOnly(candidate) {
			continue
		}
		return strings.TrimSuffix(candidate, ".")
	}
	if node.Host != "" {
		return strings.TrimSuffix(node.Host, ".")
	}
	return ""
}

func preferTailnetHostsInTyping(candidates []string) []string {
	seen := map[string]struct{}{}
	tailnet := make([]string, 0, len(candidates))
	others := make([]string, 0, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSuffix(strings.TrimSpace(candidate), ".")
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		if strings.HasSuffix(strings.ToLower(candidate), ".ts.net") {
			tailnet = append(tailnet, candidate)
		} else {
			others = append(others, candidate)
		}
	}
	out = append(out, tailnet...)
	out = append(out, others...)
	return out
}

func locateRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Clean(cwd)
	for i := 0; i < 30; i++ {
		if dir == "" || dir == "." {
			break
		}
		if _, err := os.Stat(filepath.Join(dir, "env", "mesh.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("cannot locate repo root (env/mesh.json missing)")
}

func normalizeTypingHost(v string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(v)), ".")
}

func isIPOnly(value string) bool {
	v := strings.TrimSpace(value)
	return net.ParseIP(v) != nil
}

func buildTerminalCommand(command, repoPath string) string {
	setup := []string{}
	if repoPath != "" {
		setup = append(setup, "cd "+shellQuoteForShell(repoPath)+" || true")
	}
	if command != "" {
		setup = append(setup, command)
	}
	tail := "bind 'set bell-style none' >/dev/null 2>&1; exec ${SHELL:-/bin/bash} -i"
	if len(setup) == 0 {
		return tail
	}
	return strings.Join(setup, " && ") + "; " + tail
}

func buildLocalTypedCommand(command, repoPath string) string {
	parts := []string{}
	if repoPath != "" {
		parts = append(parts, "cd "+shellQuoteForShell(repoPath))
	}
	if strings.TrimSpace(command) != "" {
		parts = append(parts, strings.TrimSpace(command))
	}
	return strings.Join(parts, " && ")
}

func shouldUsePowerShellForTypingNode(node typingMeshNode) bool {
	return strings.EqualFold(strings.TrimSpace(node.OS), "windows") &&
		node.PreferWSLPowerShell &&
		isTypingLocalWSL()
}

func isTypingLocalWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
		return true
	}
	raw, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(raw))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

func shouldUseLocalWindowsTerminalHost(node typingMeshNode) bool {
	if !shouldUsePowerShellForTypingNode(node) {
		return false
	}
	for _, candidate := range append(append([]string{}, node.HostCandidates...), node.Host) {
		if isTypingHostLocal(candidate) {
			return true
		}
	}
	return false
}

func shouldUseLocalMacTerminalHost(node typingMeshNode) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(node.OS), "darwin") && !strings.EqualFold(strings.TrimSpace(node.OS), "macos") {
		return false
	}
	for _, candidate := range append(append([]string{}, node.HostCandidates...), node.Host) {
		if isTypingHostLocal(candidate) {
			return true
		}
	}
	return false
}

func isTypingHostLocal(rawHost string) bool {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return isTypingLocalIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if isTypingLocalIP(ip) {
			return true
		}
	}
	return false
}

func isTypingLocalIP(target net.IP) bool {
	if target == nil {
		return false
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		var candidate net.IP
		switch value := addr.(type) {
		case *net.IPNet:
			candidate = value.IP
		case *net.IPAddr:
			candidate = value.IP
		default:
			continue
		}
		if candidate == nil {
			continue
		}
		if candidate.Equal(target) {
			return true
		}
		if target4 := target.To4(); target4 != nil {
			if candidate4 := candidate.To4(); candidate4 != nil && target4.Equal(candidate4) {
				return true
			}
		}
	}
	return false
}

func resolvePowerShellPath(explicit string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if detected := defaultPowerShellPath(); detected != "" {
		return detected
	}
	powerShell := "powershell.exe"
	if _, err := exec.LookPath(powerShell); err == nil {
		return powerShell
	}
	return ""
}

func resolveWindowsTerminalPath(explicit string) string {
	if explicit == "" {
		explicit = strings.TrimSpace(os.Getenv("WT_PATH"))
	}
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if explicitPath := defaultWindowsTerminalPath(); explicitPath != "" {
		return explicitPath
	}
	if terminal, err := exec.LookPath("wt.exe"); err == nil {
		return terminal
	}
	return ""
}

func defaultWindowsTerminalPath() string {
	paths := []string{
		"/mnt/c/Users/timca/AppData/Local/Microsoft/WindowsApps/wt.exe",
	}
	for _, candidate := range paths {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func defaultPowerShellPath() string {
	candidates := []string{
		"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func defaultWSLPath() string {
	candidates := []string{
		"/mnt/c/Windows/System32/wsl.exe",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func resolveWSLPath(explicit string) string {
	if explicit == "" {
		explicit = strings.TrimSpace(os.Getenv("WSL_PATH"))
	}
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
	}
	if path := defaultWSLPath(); path != "" {
		return path
	}
	return "wsl.exe"
}

func windowsPathForCommand(value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "/mnt/") && len(path) >= 7 {
		drive := strings.ToUpper(path[5:6])
		remainder := path[6:]
		remainder = strings.TrimPrefix(remainder, "/")
		if remainder == "" {
			return drive + ":\\"
		}
		return drive + ":\\" + strings.ReplaceAll(remainder, "/", "\\")
	}
	return path
}

func wslPathForLog(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/mnt/c/Users/Public/dialtone-typing-terminal.log"
	}
	if strings.HasPrefix(trimmed, "/mnt/") {
		return trimmed
	}
	if len(trimmed) >= 3 && trimmed[1] == ':' && (trimmed[2] == '\\' || trimmed[2] == '/') {
		drive := strings.ToLower(trimmed[0:1])
		rest := strings.ReplaceAll(trimmed[3:], "\\", "/")
		rest = strings.TrimPrefix(rest, "/")
		if rest == "" {
			return "/mnt/" + drive
		}
		return "/mnt/" + drive + "/" + rest
	}
	return trimmed
}

func countVisibleDialtonePowerShellWindows() (int, error) {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return 0, errors.New("powershell not found")
	}
	cmd := exec.Command(
		powerShellPath,
		"-NoProfile",
		"-Command",
		"(Get-Process -Name powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 -and $_.MainWindowTitle -like 'DialtoneTyping*' }).Count",
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse window count %q: %w", value, err)
	}
	return n, nil
}

func runTerminalClose(argv []string) error {
	opts := flag.NewFlagSet("terminal v1 close", flag.ContinueOnError)
	logPath := opts.String("log-path", "C:\\Users\\Public\\dialtone-typing-terminal.log", "Windows log file path used by local launcher")
	closeAll := opts.Bool("all", false, "Close all visible PowerShell windows (not just Dialtone-tracked)")
	closeWSLPrompts := opts.Bool("wsl-prompts", false, "Close WSL prompt windows (wsl.exe with title user@...:~)")
	serviceURL := opts.String("service-url", "nats://127.0.0.1:47222", "NATS URL for terminal service request/reply")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if runtime.GOOS != "windows" && !isTypingLocalWSL() {
		return errors.New("terminal close requires Windows or WSL")
	}
	if os.Getenv("DIALTONE_TERMINAL_SERVICE_INTERNAL") != "1" {
		if handled, err := maybeProxyStopViaService(strings.TrimSpace(*serviceURL), strings.TrimSpace(*logPath), *closeAll, *closeWSLPrompts); handled {
			return err
		}
		return fmt.Errorf("terminal service not reachable at %s; run `./dialtone_mod terminal v1 service start`", strings.TrimSpace(*serviceURL))
	}
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return errors.New("powershell not found")
	}
	if *closeWSLPrompts {
		rows, err := listAllTerminalProcessesFromTasklistAdmin()
		if err != nil {
			rows, err = listAllTerminalProcessesFromTasklist()
			if err != nil {
				return err
			}
		}
		closed := 0
		for _, row := range rows {
			if strings.ToLower(strings.TrimSpace(row.Process)) != "wsl.exe" {
				continue
			}
			title := strings.ToLower(strings.TrimSpace(row.Title))
			if !strings.Contains(title, "@") || !strings.Contains(title, ":~") {
				continue
			}
			pid, convErr := strconv.Atoi(strings.TrimSpace(row.PID))
			if convErr != nil || pid <= 0 {
				continue
			}
			if err := stopProcessByPID(pid); err == nil {
				closed++
			}
		}
		fmt.Printf("closed=%d mode=wsl-prompts\n", closed)
		return nil
	}
	if *closeAll {
		command := "$targets = Get-Process -Name powershell,pwsh -ErrorAction SilentlyContinue; " +
			"$n = @($targets).Count; " +
			"if ($n -gt 0) { $targets | Stop-Process -Force -ErrorAction SilentlyContinue }; " +
			"Write-Output $n"
		cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", command)
		output, err := cmd.Output()
		if err != nil {
			return err
		}
		count := strings.TrimSpace(string(output))
		if count == "" {
			count = "0"
		}
		fmt.Printf("closed=%s mode=all\n", count)
		return nil
	}

	launchLog := wslPathForLog(*logPath)
	raw, err := os.ReadFile(launchLog)
	if err != nil {
		return fmt.Errorf("failed reading launch log %s: %w", launchLog, err)
	}
	re := regexp.MustCompile(`launched powershell window pid=(\d+)`)
	matches := re.FindAllStringSubmatch(string(raw), -1)
	seen := map[int]struct{}{}
	closed := 0
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		pid, convErr := strconv.Atoi(strings.TrimSpace(m[1]))
		if convErr != nil || pid <= 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		if err := stopProcessByPID(pid); err == nil {
			closed++
		}
	}
	fmt.Printf("closed=%d mode=tracked\n", closed)
	return nil
}

func stopDialtonePowerShellWindows() error {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return errors.New("powershell not found")
	}
	cmd := exec.Command(
		powerShellPath,
		"-NoProfile",
		"-Command",
		"Get-Process -Name powershell -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowTitle -like 'DialtoneTyping*' } | Stop-Process -Force -ErrorAction SilentlyContinue",
	)
	return cmd.Run()
}

func stopProcessByPID(pid int) error {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return errors.New("powershell not found")
	}
	command := fmt.Sprintf("Stop-Process -Id %d -Force -ErrorAction SilentlyContinue", pid)
	cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", command)
	return cmd.Run()
}

func readStatePID(statePath string) (int, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return 0, err
	}
	var payload struct {
		Pid int `json:"Pid"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return 0, err
	}
	return payload.Pid, nil
}

func hasChildWSLProcess(parentPID int) (bool, error) {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return false, errors.New("powershell not found")
	}
	filter := fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"Name = 'wsl.exe'\" | Where-Object { $_.ParentProcessId -eq %d }).Count", parentPID)
	cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", filter)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return false, nil
	}
	count, err := strconv.Atoi(value)
	if err != nil {
		return false, fmt.Errorf("parse wsl child count %q: %w", value, err)
	}
	return count > 0, nil
}

func isProcessAlive(pid int) (bool, error) {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return false, errors.New("powershell not found")
	}
	command := fmt.Sprintf("$p = Get-Process -Id %d -ErrorAction SilentlyContinue; if ($p) { '1' } else { '0' }", pid)
	cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", command)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "1", nil
}

func getProcessWindowInfo(pid int) (string, bool, bool, error) {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return "", false, false, errors.New("powershell not found")
	}
	command := fmt.Sprintf("$p = Get-Process -Id %d -ErrorAction SilentlyContinue; if (-not $p) { 'MISSING' } else { \"{0}`t{1}`t{2}\" -f $p.Id, $p.MainWindowHandle, $p.MainWindowTitle }", pid)
	cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", command)
	output, err := cmd.Output()
	if err != nil {
		return "", false, false, err
	}
	text := strings.TrimSpace(string(output))
	if text == "" || text == "MISSING" {
		return "", false, false, nil
	}
	parts := strings.SplitN(text, "\t", 3)
	if len(parts) < 2 {
		return "", false, true, nil
	}
	handleValue := strings.TrimSpace(parts[1])
	title := ""
	if len(parts) > 2 {
		title = strings.TrimSpace(parts[2])
	}
	handle, _ := strconv.ParseInt(handleValue, 10, 64)
	return title, handle != 0, true, nil
}

func listAllVisibleTerminalWindows() ([]terminalProcRow, error) {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return nil, errors.New("powershell not found")
	}
	script := `
Add-Type -TypeDefinition @"
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class Win32 {
  public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc lpEnumFunc, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, StringBuilder lpString, int nMaxCount);
  [DllImport("user32.dll")] public static extern int GetWindowTextLength(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out int lpdwProcessId);
}
"@
$allowed = @("powershell","pwsh","cmd","windowsterminal","openconsole","conhost")
$rows = New-Object System.Collections.Generic.List[Object]
[Win32]::EnumWindows({
  param($hWnd, $lParam)
  if (-not [Win32]::IsWindowVisible($hWnd)) { return $true }
  $len = [Win32]::GetWindowTextLength($hWnd)
  if ($len -le 0) { return $true }
  $sb = New-Object System.Text.StringBuilder ($len + 1)
  [Win32]::GetWindowText($hWnd, $sb, $sb.Capacity) | Out-Null
  $title = $sb.ToString()
  if ([string]::IsNullOrWhiteSpace($title)) { return $true }
  $pid = 0
  [Win32]::GetWindowThreadProcessId($hWnd, [ref]$pid) | Out-Null
  try { $p = Get-Process -Id $pid -ErrorAction Stop } catch { return $true }
  $name = $p.ProcessName.ToLower()
  if ($allowed -notcontains $name) { return $true }
  $owner = "N/A"
  try {
    $c = Get-CimInstance Win32_Process -Filter ("ProcessId={0}" -f $pid)
    if ($c) {
      $o = Invoke-CimMethod -InputObject $c -MethodName GetOwner
      if ($o -and $o.User) { $owner = ("{0}\{1}" -f $o.Domain, $o.User) }
    }
  } catch {}
  $rows.Add([pscustomobject]@{
    Process = $name
    Pid = $pid
    User = $owner
    SessionId = $p.SessionId
    Title = $title
  }) | Out-Null
  return $true
}, [IntPtr]::Zero) | Out-Null
$rows | Sort-Object Pid -Unique | ForEach-Object {
  "{0}|{1}|{2}|{3}|{4}" -f $_.Process, $_.Pid, $_.User, $_.SessionId, $_.Title
}
`
	cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", script)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	entries := make([]terminalProcRow, 0, len(lines))
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "|", 5)
		if len(parts) < 5 {
			continue
		}
		entries = append(entries, terminalProcRow{
			Process:   strings.TrimSpace(parts[0]),
			PID:       strings.TrimSpace(parts[1]),
			User:      strings.TrimSpace(parts[2]),
			Session:   "N/A",
			SessionID: strings.TrimSpace(parts[3]),
			Windowed:  "true",
			Title:     strings.TrimSpace(parts[4]),
		})
	}
	return entries, nil
}

func listAllTerminalProcessesFromTasklist() ([]terminalProcRow, error) {
	cmdPath := "/mnt/c/Windows/System32/cmd.exe"
	if _, err := os.Stat(cmdPath); err != nil {
		return nil, fmt.Errorf("cmd.exe not found at %s", cmdPath)
	}
	cmd := exec.Command(cmdPath, "/C", "tasklist", "/v", "/fo", "csv", "/nh")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseTasklistTerminalCSV(output), nil
}

func listAllTerminalProcessesFromTasklistAdmin() ([]terminalProcRow, error) {
	powerShellPath := resolvePowerShellPath(defaultPowerShellPath())
	if powerShellPath == "" {
		return nil, errors.New("powershell not found")
	}
	outPath := "C:\\Users\\Public\\dialtone-terminal-list-admin.csv"
	adminScript := "$out = " + powerShellSingleQuote(outPath) + "; " +
		"tasklist /v /fo csv /nh | Set-Content -LiteralPath $out -Encoding UTF8"
	command := "Start-Process -FilePath " + powerShellSingleQuote(windowsPathForCommand(powerShellPath)) +
		" -Verb RunAs -Wait -ArgumentList @('-NoProfile','-Command'," + powerShellSingleQuote(adminScript) + "); " +
		"Get-Content -LiteralPath " + powerShellSingleQuote(outPath) + " -Raw"
	cmd := exec.Command(powerShellPath, "-NoProfile", "-Command", command)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseTasklistTerminalCSV(output), nil
}

func parseTasklistTerminalCSV(raw []byte) []terminalProcRow {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := []terminalProcRow{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\",\"")
		if len(parts) < 9 {
			continue
		}
		normalize := func(v string) string {
			v = strings.TrimSpace(v)
			v = strings.TrimPrefix(v, "\"")
			v = strings.TrimSuffix(v, "\"")
			return v
		}
		image := strings.ToLower(normalize(parts[0]))
		if image != "powershell.exe" && image != "pwsh.exe" && image != "cmd.exe" && image != "windowsterminal.exe" && image != "conhost.exe" && image != "openconsole.exe" && image != "wsl.exe" {
			continue
		}
		pid := normalize(parts[1])
		sessionName := normalize(parts[2])
		sessionID := normalize(parts[3])
		user := normalize(parts[6])
		title := normalize(parts[8])
		if user == "" {
			user = "N/A"
		}
		if title == "" {
			title = "N/A"
		}
		windowed := title != "N/A"
		windowedText := "false"
		if windowed {
			windowedText = "true"
		}
		entries = append(entries, terminalProcRow{
			Process:   image,
			PID:       pid,
			User:      user,
			Session:   sessionName,
			SessionID: sessionID,
			Windowed:  windowedText,
			Title:     title,
		})
	}
	return entries
}

func readTaggedTerminalPIDs(tagPath string) ([]int, error) {
	raw, err := os.ReadFile(tagPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	seen := map[int]struct{}{}
	pids := make([]int, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" {
			continue
		}
		pid, convErr := strconv.Atoi(value)
		if convErr != nil || pid <= 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		pids = append(pids, pid)
	}
	return pids, nil
}

func waitForCommandExecStart(logPath, command string, timeout time.Duration) error {
	target := strings.TrimSpace(command)
	if target == "" {
		return nil
	}
	launchLog := wslPathForLog(logPath)
	eventLog := launchLog + ".events.log"
	transcriptLog := launchLog + ".window.log"
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(eventLog); err == nil {
			if strings.Contains(string(raw), "exec-start "+target) {
				return nil
			}
		}
		if raw, err := os.ReadFile(launchLog); err == nil {
			if strings.Contains(string(raw), "exec-start "+target) {
				return nil
			}
		}
		if raw, err := os.ReadFile(transcriptLog); err == nil {
			if strings.Contains(string(raw), target) {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("missing exec-start for command=%q within %s (likely blocked by a currently running interactive command)", target, timeout.String())
}

func injectCommandIntoTaggedTerminal(logPath, commandText, powershellPath string) error {
	target := strings.TrimSpace(commandText)
	if target == "" {
		return nil
	}
	powerShell := resolvePowerShellPath(powershellPath)
	if strings.TrimSpace(powerShell) == "" {
		return errors.New("powershell not found for typing injection")
	}
	tagPath := wslPathForLog(logPath) + ".tagged-pids.txt"
	winTagPath := windowsPathForCommand(tagPath)
	script := "$tagPath = " + powerShellSingleQuote(winTagPath) + "; " +
		"$cmdText = " + powerShellSingleQuote(target) + "; " +
		"if (-not (Test-Path -LiteralPath $tagPath)) { throw 'tag file not found' }; " +
		"$rows = Get-Content -LiteralPath $tagPath | ForEach-Object { $_.Trim() } | Where-Object { $_ -match '^[0-9]+$' }; " +
		"$pids = @($rows | ForEach-Object { [int]$_ } | Sort-Object -Descending -Unique); " +
		"Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public static class DTWin { [DllImport(\"user32.dll\")] public static extern bool SetForegroundWindow(IntPtr hWnd); [DllImport(\"user32.dll\")] public static extern bool ShowWindowAsync(IntPtr hWnd, int nCmdShow); }'; " +
		"$ws = New-Object -ComObject WScript.Shell; " +
		"$targets = @(); " +
		"foreach ($pid in $pids) { $p = Get-Process -Id $pid -ErrorAction SilentlyContinue; if ($p -and $p.MainWindowHandle -ne 0) { $targets += [pscustomobject]@{ Pid = $pid; Hwnd = $p.MainWindowHandle; Title = $p.MainWindowTitle } } }; " +
		"if (@($targets).Count -eq 0) { throw 'no active tagged terminal window' }; " +
		"$ok = $false; " +
		"for ($i = 0; $i -lt 16 -and -not $ok; $i++) { " +
		"foreach ($t in $targets) { " +
		"try { [DTWin]::ShowWindowAsync([IntPtr]$t.Hwnd, 5) | Out-Null; [DTWin]::SetForegroundWindow([IntPtr]$t.Hwnd) | Out-Null } catch {}; " +
		"$activated = $false; " +
		"if ($t.Title -and $ws.AppActivate($t.Title)) { $activated = $true } " +
		"elseif ($ws.AppActivate($t.Pid)) { $activated = $true }; " +
		"if ($activated) { " +
		"Start-Sleep -Milliseconds 180; " +
		"Set-Clipboard -Value $cmdText; Start-Sleep -Milliseconds 120; " +
		"$ws.SendKeys('^v'); Start-Sleep -Milliseconds 120; " +
		"$ws.SendKeys('~'); Start-Sleep -Milliseconds 260; " +
		"$ok = $true; break } } ; " +
		"Start-Sleep -Milliseconds 220 } ; " +
		"if (-not $ok) { throw 'failed to activate tagged terminal for injection' }"
	cmd := exec.Command(powerShell, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	return cmd.Run()
}

func fileContainsToken(path, token string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(raw), token), nil
}

func printTerminalProcTable(rows []terminalProcRow) {
	if len(rows) == 0 {
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROCESS\tPID\tUSER\tSESSION\tSESSION_ID\tWINDOWED\tTITLE")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row.Process, row.PID, row.User, row.Session, row.SessionID, row.Windowed, row.Title)
	}
	_ = w.Flush()
}

func queueContainsAllCommands(queuePath string, commands []string) (bool, error) {
	raw, err := os.ReadFile(queuePath)
	if err != nil {
		return false, err
	}
	text := string(raw)
	for _, command := range commands {
		encoded := base64.StdEncoding.EncodeToString([]byte(command))
		if !strings.Contains(text, encoded) {
			return false, nil
		}
	}
	return true, nil
}

func runTerminalService(argv []string) error {
	if len(argv) == 0 {
		return errors.New("service command required: start|run|stop|status")
	}
	subcommand := strings.TrimSpace(argv[0])
	fs := flag.NewFlagSet("terminal v1 service", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:47222", "NATS URL for terminal service")
	if err := fs.Parse(argv[1:]); err != nil {
		return err
	}
	switch subcommand {
	case "run":
		return runTerminalServiceLoop(strings.TrimSpace(*natsURL))
	case "start":
		return startTerminalServiceProcess(strings.TrimSpace(*natsURL))
	case "stop":
		return stopTerminalServiceProcess()
	case "status":
		return statusTerminalServiceProcess(strings.TrimSpace(*natsURL))
	default:
		return fmt.Errorf("unknown service subcommand: %s", subcommand)
	}
}

func maybeProxyTypeViaService(natsURL, logPath, repoPath, command string) (bool, error) {
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(350*time.Millisecond), nats.Name("dialtone-terminal-v1-type-proxy"))
	if err != nil {
		return false, nil
	}
	defer nc.Close()

	reqBody, _ := json.Marshal(terminalServiceExecRequest{
		Command: strings.TrimSpace(command),
		Repo:    strings.TrimSpace(repoPath),
		LogPath: strings.TrimSpace(logPath),
	})
	reply, err := nc.Request("terminal.v1.exec", reqBody, 15*time.Second)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no responders") {
			return false, nil
		}
		return true, fmt.Errorf("terminal service exec request failed: %w", err)
	}
	resp := terminalServiceResponse{}
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return true, fmt.Errorf("terminal service exec response decode failed: %w", err)
	}
	if !resp.OK {
		if strings.TrimSpace(resp.Message) == "" {
			resp.Message = "terminal service returned failure"
		}
		return true, errors.New(resp.Message)
	}
	return true, nil
}

func maybeProxyListViaService(natsURL, logPath string, showAll, useAdmin bool) (bool, error) {
	_ = showAll
	_ = useAdmin
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(350*time.Millisecond), nats.Name("dialtone-terminal-v1-list-proxy"))
	if err != nil {
		return false, nil
	}
	defer nc.Close()
	reqBody, _ := json.Marshal(terminalServiceListRequest{LogPath: strings.TrimSpace(logPath)})
	reply, err := nc.Request("terminal.v1.list", reqBody, 4*time.Second)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no responders") {
			return false, nil
		}
		return true, fmt.Errorf("terminal service list request failed: %w", err)
	}
	resp := terminalServiceResponse{}
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return true, fmt.Errorf("terminal service list response decode failed: %w", err)
	}
	if !resp.OK {
		if strings.TrimSpace(resp.Message) == "" {
			resp.Message = "terminal service list failed"
		}
		return true, errors.New(resp.Message)
	}
	rows := []terminalTrackedRow{}
	_ = json.Unmarshal(resp.Data, &rows)
	fmt.Printf("open=%d tracked_pids=%d\n", len(rows), len(rows))
	if len(rows) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PID\tVISIBLE\tWSL\tTITLE")
		for _, row := range rows {
			fmt.Fprintf(w, "%d\t%t\t%s\t%s\n", row.PID, row.Visible, row.WSL, row.Title)
		}
		_ = w.Flush()
	}
	return true, nil
}

func maybeProxyStopViaService(natsURL, logPath string, closeAll, closeWSLPrompts bool) (bool, error) {
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(350*time.Millisecond), nats.Name("dialtone-terminal-v1-stop-proxy"))
	if err != nil {
		return false, nil
	}
	defer nc.Close()
	reqBody, _ := json.Marshal(terminalServiceStopRequest{
		All:        closeAll,
		WSLPrompts: closeWSLPrompts,
		LogPath:    strings.TrimSpace(logPath),
	})
	reply, err := nc.Request("terminal.v1.stop", reqBody, 4*time.Second)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no responders") {
			return false, nil
		}
		return true, fmt.Errorf("terminal service stop request failed: %w", err)
	}
	resp := terminalServiceResponse{}
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return true, fmt.Errorf("terminal service stop response decode failed: %w", err)
	}
	if !resp.OK {
		if strings.TrimSpace(resp.Message) == "" {
			resp.Message = "terminal service stop failed"
		}
		return true, errors.New(resp.Message)
	}
	if strings.TrimSpace(resp.Message) != "" {
		fmt.Println(resp.Message)
	}
	return true, nil
}

func runTerminalInternalDirect(args []string) error {
	previous, had := os.LookupEnv("DIALTONE_TERMINAL_SERVICE_INTERNAL")
	_ = os.Setenv("DIALTONE_TERMINAL_SERVICE_INTERNAL", "1")
	defer func() {
		if had {
			_ = os.Setenv("DIALTONE_TERMINAL_SERVICE_INTERNAL", previous)
		} else {
			_ = os.Unsetenv("DIALTONE_TERMINAL_SERVICE_INTERNAL")
		}
	}()
	return runTerminal(args)
}

func runTerminalCloseInternal(args []string) error {
	previous, had := os.LookupEnv("DIALTONE_TERMINAL_SERVICE_INTERNAL")
	_ = os.Setenv("DIALTONE_TERMINAL_SERVICE_INTERNAL", "1")
	defer func() {
		if had {
			_ = os.Setenv("DIALTONE_TERMINAL_SERVICE_INTERNAL", previous)
		} else {
			_ = os.Unsetenv("DIALTONE_TERMINAL_SERVICE_INTERNAL")
		}
	}()
	return runTerminalClose(args)
}

func terminalServicePIDPath() string { return "/mnt/c/Users/Public/dialtone-terminal-v1-service.pid" }
func terminalServiceLogPath() string { return "/mnt/c/Users/Public/dialtone-terminal-v1-service.log" }

func startTerminalServiceProcess(natsURL string) error {
	pidPath := terminalServicePIDPath()
	if pid, err := readPIDFile(pidPath); err == nil && pid > 0 {
		if alive, _ := isLocalPIDAlive(pid); alive {
			fmt.Printf("service already running pid=%d\n", pid)
			return nil
		}
	}
	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	logPath := terminalServiceLogPath()
	launch := fmt.Sprintf("cd %s && DIALTONE_TERMINAL_SERVICE_INTERNAL=1 ./dialtone_mod terminal v1 service run --nats-url %s >> %s 2>&1",
		shellQuoteForShell(repoRoot),
		shellQuoteForShell(natsURL),
		shellQuoteForShell(logPath),
	)
	cmd := exec.Command("bash", "-lc", launch)
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0644); err != nil {
		return err
	}
	fmt.Printf("service started pid=%d nats=%s\n", cmd.Process.Pid, natsURL)
	return nil
}

func stopTerminalServiceProcess() error {
	pid, err := readPIDFile(terminalServicePIDPath())
	if err != nil {
		return fmt.Errorf("service not running")
	}
	if proc, findErr := os.FindProcess(pid); findErr == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}
	_ = os.Remove(terminalServicePIDPath())
	fmt.Printf("service stopped pid=%d\n", pid)
	return nil
}

func statusTerminalServiceProcess(natsURL string) error {
	pid, _ := readPIDFile(terminalServicePIDPath())
	alive := false
	if pid > 0 {
		alive, _ = isLocalPIDAlive(pid)
	}
	nc, err := nats.Connect(strings.TrimSpace(natsURL), nats.Timeout(350*time.Millisecond), nats.Name("dialtone-terminal-v1-status"))
	reachable := err == nil
	if err == nil {
		nc.Close()
	}
	fmt.Printf("service_pid=%d service_alive=%t nats=%s reachable=%t\n", pid, alive, natsURL, reachable)
	return nil
}

func runTerminalServiceLoop(natsURL string) error {
	srv, clientURL, err := startEmbeddedNATS(natsURL)
	if err != nil {
		return err
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(clientURL, nats.Name("dialtone-terminal-v1-service"))
	if err != nil {
		return err
	}
	defer nc.Close()

	_, err = nc.Subscribe("terminal.v1.exec", func(msg *nats.Msg) {
		fmt.Println("service request: terminal.v1.exec")
		req := terminalServiceExecRequest{}
		_ = json.Unmarshal(msg.Data, &req)
		args := []string{}
		if strings.TrimSpace(req.LogPath) != "" {
			args = append(args, "--log-path", strings.TrimSpace(req.LogPath))
		}
		if strings.TrimSpace(req.Repo) != "" {
			args = append(args, "--repo", strings.TrimSpace(req.Repo))
		}
		args = append(args, "--command", strings.TrimSpace(req.Command))
		err := runTerminalInternalDirect(args)
		replyServiceResponse(nc, msg.Reply, err, nil)
	})
	if err != nil {
		return err
	}
	_, err = nc.Subscribe("terminal.v1.list", func(msg *nats.Msg) {
		fmt.Println("service request: terminal.v1.list")
		req := terminalServiceListRequest{}
		_ = json.Unmarshal(msg.Data, &req)
		logPath := strings.TrimSpace(req.LogPath)
		if logPath == "" {
			logPath = "C:\\Users\\Public\\dialtone-typing-terminal.log"
		}
		rows, listErr := collectTrackedRows(logPath)
		if listErr != nil {
			replyServiceResponse(nc, msg.Reply, listErr, nil)
			return
		}
		payload, _ := json.Marshal(rows)
		replyServiceResponse(nc, msg.Reply, nil, payload)
	})
	if err != nil {
		return err
	}
	_, err = nc.Subscribe("terminal.v1.stop", func(msg *nats.Msg) {
		fmt.Println("service request: terminal.v1.stop")
		req := terminalServiceStopRequest{}
		_ = json.Unmarshal(msg.Data, &req)
		args := []string{}
		if strings.TrimSpace(req.LogPath) != "" {
			args = append(args, "--log-path", strings.TrimSpace(req.LogPath))
		}
		if req.All {
			args = append(args, "--all")
		}
		if req.WSLPrompts {
			args = append(args, "--wsl-prompts")
		}
		err := runTerminalCloseInternal(args)
		msgText := "closed=ok"
		if err != nil {
			replyServiceResponse(nc, msg.Reply, err, nil)
			return
		}
		replyServiceResponse(nc, msg.Reply, nil, []byte(msgText))
	})
	if err != nil {
		return err
	}
	if err := nc.Flush(); err != nil {
		return err
	}

	fmt.Printf("terminal service running nats=%s\n", clientURL)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals
	return nil
}

func startEmbeddedNATS(rawURL string) (*nserver.Server, string, error) {
	u, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, "", err
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		host = "127.0.0.1"
	}
	port := 47222
	if p := strings.TrimSpace(u.Port()); p != "" {
		parsed, parseErr := strconv.Atoi(p)
		if parseErr != nil {
			return nil, "", parseErr
		}
		port = parsed
	}
	opts := &nserver.Options{Host: host, Port: port}
	srv, err := nserver.NewServer(opts)
	if err != nil {
		return nil, "", err
	}
	go srv.Start()
	if !srv.ReadyForConnections(5 * time.Second) {
		return nil, "", errors.New("embedded nats did not become ready")
	}
	return srv, fmt.Sprintf("nats://%s:%d", host, port), nil
}

func replyServiceResponse(nc *nats.Conn, reply string, err error, data []byte) {
	if strings.TrimSpace(reply) == "" {
		return
	}
	resp := terminalServiceResponse{OK: err == nil}
	if err != nil {
		resp.Message = err.Error()
	}
	if len(data) > 0 {
		resp.Data = data
	}
	raw, _ := json.Marshal(resp)
	_ = nc.Publish(reply, raw)
}

func readPIDFile(path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return 0, errors.New("empty pid")
	}
	pid, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func isLocalPIDAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func collectTrackedRows(logPath string) ([]terminalTrackedRow, error) {
	launchLog := wslPathForLog(logPath)
	pids, err := readTaggedTerminalPIDs(launchLog + ".tagged-pids.txt")
	if err != nil || len(pids) == 0 {
		return []terminalTrackedRow{}, nil
	}
	rows := make([]terminalTrackedRow, 0, len(pids))
	for _, pid := range pids {
		title, visible, alive, infoErr := getProcessWindowInfo(pid)
		if infoErr != nil {
			continue
		}
		wslState := "false"
		hasWSL, wslErr := hasChildWSLProcess(pid)
		if wslErr != nil {
			wslState = "unknown"
		} else if hasWSL {
			wslState = "true"
		}
		rows = append(rows, terminalTrackedRow{
			PID:     pid,
			Alive:   alive,
			Visible: visible,
			Title:   title,
			WSL:     wslState,
		})
	}
	return rows, nil
}


func buildPowerShellTerminalCommand(targetAddress, port, posixCommand string) string {
	args := []string{"ssh"}
	args = append(args, buildSSHArgsForInteractiveTyping(targetAddress, port)...)
	remoteCommand := "bash -lc " + shellQuoteForShell(posixCommand)
	args = append(args, remoteCommand)
	wslBinary := windowsPathForCommand(resolveWSLPath(""))
	if wslBinary == "" {
		wslBinary = "wsl.exe"
	}
	command := "Set-Location C:\\; " + wslBinary + " --cd ~ -e bash -lc " + powerShellSingleQuote(strings.Join(args, " "))
	return command
}

func trimForLocalPowerShellTerminal(command string) string {
	trimmed := strings.TrimSpace(command)
	return strings.ReplaceAll(trimmed, "exec ${SHELL:-/bin/bash}", "exec /bin/bash")
}

func powerShellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func shellFormatCommand(name string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, name)
	parts = append(parts, args...)
	output := make([]string, len(parts))
	for idx, part := range parts {
		if strings.IndexAny(part, " \t\r\n") >= 0 {
			output[idx] = fmt.Sprintf("%q", part)
			continue
		}
		output[idx] = part
	}
	return strings.Join(output, " ")
}

func buildSSHArgsForInteractiveTyping(targetAddress, port string) []string {
	args := []string{
		"-F", "/dev/null",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-tt",
	}
	if strings.TrimSpace(port) != "" && strings.TrimSpace(port) != "22" {
		args = append(args, "-p", strings.TrimSpace(port))
	}
	args = append(args, targetAddress)
	return args
}

func shellQuoteForShell(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "'\"'\"'") + "'"
}

func escapeAppleString(value string) string {
	v := strings.ReplaceAll(value, "\\", "\\\\")
	v = strings.ReplaceAll(v, "\"", `\"`)
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "\\n")
	v = strings.ReplaceAll(v, "\t", "\\t")
	return v
}

func buildGhosttyAppleScriptText(command string) (string, string) {
	text := escapeAppleString(strings.TrimSpace(command))
	script := strings.Join([]string{
		`tell application "Ghostty" to activate`,
		"delay 0.2",
		`tell application "System Events"`,
		`	tell process "ghostty"`,
		fmt.Sprintf(`		keystroke "%s"`, text),
		`		key code 36`,
		`	end tell`,
		`end tell`,
	}, "\n")
	return script, text
}

func buildGhosttyTypingCommand(script string) string {
	payload := base64.StdEncoding.EncodeToString([]byte(script))
	return fmt.Sprintf(
		"printf %s %s | base64 -D | osascript",
		shellQuoteForShell("%s"),
		shellQuoteForShell(payload),
	)
}

func runTypingLocalGhosttySession(command string) error {
	scriptText, _ := buildGhosttyAppleScriptText(command)
	cmd := exec.Command("osascript", "-e", scriptText)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod terminal v1 <type|service|test|list|close> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  type [--repo PATH]")
	fmt.Println("      [--command \"./dialtone_mod ...\"]")
	fmt.Println("      [--powershell-path PATH]")
	fmt.Println("      [--wsl-path PATH]")
	fmt.Println("      [--wt-path PATH]")
	fmt.Println("      [--wt-profile PROFILE]")
	fmt.Println("      [--service-url nats://127.0.0.1:47222]")
	fmt.Println("      [--log-path WINDOWS_PATH]")
	fmt.Println("      [--dry-run]")
	fmt.Println("      Uses terminal v1 service request/reply for type/list/close")
	fmt.Println("  service <start|run|stop|status> [--nats-url URL]")
	fmt.Println("      Starts/stops terminald-style embedded NATS service and request/reply handlers")
	fmt.Println("  test [--log-path WINDOWS_PATH] [--timeout-seconds N]")
	fmt.Println("      Sends two commands through `type`, verifies transcript token, and checks visible DialtoneTyping window")
	fmt.Println("      Add --wsl to enter interactive WSL in that same terminal and verify token + wsl.exe child process")
	fmt.Println("  list")
	fmt.Println("      Lists currently open DialtoneTyping terminal windows and whether each has a wsl.exe child process")
	fmt.Println("      Add --all to also list visible PowerShell/CMD/Windows Terminal windows")
	fmt.Println("      Add --admin with --all to enumerate terminal processes via elevated Windows PowerShell")
	fmt.Println("  close")
	fmt.Println("      Closes Dialtone-tracked PowerShell windows")
	fmt.Println("      Add --all to close all visible PowerShell windows")
	fmt.Println("      Add --wsl-prompts to close WSL prompt windows created during terminal tests")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
