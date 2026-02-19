package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dialtone/dev/core/logger"
	test_cli "dialtone/dev/core/test/cli"
	github_cli "dialtone/dev/plugins/github/cli"
)

// RunAI is the entry point for the AI plugin
func RunAI(args []string) {
	if len(args) < 1 {
		PrintAIUsage()
		return
	}

	command := args[0]
	subArgs := args[1:]

	switch command {
	case "opencode":
		RunOpencode(subArgs)
	case "developer":
		RunDeveloper(subArgs)
	case "subagent":
		RunSubagent(subArgs)
	case "build":
		RunAIBuild(subArgs)
	case "install":
		RunAIInstall(subArgs)
	case "auth":
		RunAIAuth(subArgs)
	case "help", "-h", "--help":
		PrintAIUsage()
	case "--gemini":
		RunGemini(subArgs)
	default:
		fmt.Printf("Unknown AI command: %s\n", command)
		PrintAIUsage()
		os.Exit(1)
	}
}

func PrintAIUsage() {
	fmt.Println("Usage: dialtone ai <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  opencode <subcmd>  Manage opencode AI assistant (start, stop, status, ui)")
	fmt.Println("  developer          Start the autonomous developer loop")
	fmt.Println("  subagent <options> Interface for autonomous subagents")
	fmt.Println("  build              Build AI related components")
	fmt.Println("  install <tool>     Install AI related tools (e.g., geminicli)")
	fmt.Println("  auth               Authenticate with Google / Gemini API")
}

// RunAIBuild handles the building of AI related components
func RunAIBuild(args []string) {
	logger.LogInfo("Building AI components...")
	// For now, this might just be a no-op or verification that binaries exist
	// since opencode is usually a downloaded binary in $HOME/.opencode.
	// We'll add actual build logic if we add native Go AI code later.
	logger.LogInfo("AI components are ready.")
}

// RunOpencode handles the opencode command
func RunOpencode(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: dialtone ai opencode <subcommand> [options]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  start         Start the opencode server")
		fmt.Println("  stop          Stop the opencode server")
		fmt.Println("  status        Check server status")
		fmt.Println("  ui            Open the opencode UI in browser")
		return
	}

	subcommand := args[0]
	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")

	switch subcommand {
	case "start":
		logger.LogInfo("Starting opencode server on port 3000...")
		cmd := exec.Command(opencodePath, "--port", "3000")
		logFile, err := os.OpenFile("opencode.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.LogFatal("Failed to open opencode log: %v", err)
		}
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		if err := cmd.Start(); err != nil {
			logger.LogFatal("Failed to start opencode: %v", err)
		}
		logger.LogInfo("opencode started (PID: %d). Logs: opencode.log", cmd.Process.Pid)

	case "stop":
		logger.LogInfo("Stopping opencode server...")
		cmd := exec.Command("pkill", "-f", "opencode")
		if err := cmd.Run(); err != nil {
			logger.LogInfo("Opencode not running or failed to stop: %v", err)
		} else {
			logger.LogInfo("opencode stopped")
		}

	case "status":
		cmd := exec.Command("pgrep", "-f", "opencode")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			fmt.Printf("opencode is running (PIDs: %s)\n", strings.TrimSpace(string(output)))
		} else {
			fmt.Println("opencode is not running")
		}

	case "ui":
		logger.LogInfo("Opening opencode UI...")
		url := "http://127.0.0.1:3000"
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", "start", url)
		} else if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
		cmd.Run()

	default:
		fmt.Printf("Unknown opencode subcommand: %s\n", subcommand)
		RunOpencode([]string{})
	}
}

// RunDeveloper handles the developer command
func RunDeveloper(args []string) {
	logger.LogInfo("Starting autonomous developer loop...")

	var capabilities []string
	dryRun := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--dry-run" {
			dryRun = true
		} else if args[i] == "--capability" && i+1 < len(args) {
			capabilities = append(capabilities, args[i+1])
			i++
		}
	}

	if dryRun {
		logger.LogInfo("Running in DRY RUN mode. No changes will be made.")
	}

	logger.LogInfo("Fetching open tickets from GitHub...")
	cmd := exec.Command("gh", "ticket", "list", "--json", "number,title,labels", "--state", "open")
	output, err := cmd.Output()
	if err != nil {
		logger.LogFatal("Failed to fetch tickets: %v", err)
	}

	var tickets []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}

	if err := json.Unmarshal(output, &tickets); err != nil {
		logger.LogFatal("Failed to parse tickets: %v", err)
	}

	if len(tickets) == 0 {
		logger.LogInfo("No open tickets found.")
		return
	}

	bestTicketIdx := -1
	maxMatch := -1

	for i, ticket := range tickets {
		matchCount := 0
		for _, label := range ticket.Labels {
			for _, cap := range capabilities {
				if strings.Contains(strings.ToLower(label.Name), strings.ToLower(cap)) {
					matchCount++
				}
			}
		}
		if matchCount > maxMatch {
			maxMatch = matchCount
			bestTicketIdx = i
		}
	}

	selectedTicket := tickets[bestTicketIdx]
	logger.LogInfo("Selected ticket #%d: %s (Match score: %d)", selectedTicket.Number, selectedTicket.Title, maxMatch)

	branchName := fmt.Sprintf("ticket-%d", selectedTicket.Number)
	if dryRun {
		logger.LogInfo("DRY RUN: Would create branch %s and directory features/%s", branchName, branchName)
		return
	}

	// Create branch
	logger.LogInfo("Creating new branch '%s'...", branchName)
	cmd = exec.Command("git", "checkout", "-b", branchName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logger.LogFatal("Git operation failed: %v", err)
	}

	featureDir := filepath.Join("features", branchName)
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		logger.LogFatal("Failed to create feature directory: %v", err)
	}

	taskPath := filepath.Join(featureDir, "task.md")
	taskContent := fmt.Sprintf("# Task: Solve Ticket #%d\n\n- [ ] %s\n", selectedTicket.Number, selectedTicket.Title)
	if err := os.WriteFile(taskPath, []byte(taskContent), 0644); err != nil {
		logger.LogFatal("Failed to create task file: %v", err)
	}

	logger.LogInfo("Setup complete for %s. Task file: %s", branchName, taskPath)

	subCmd := StartSubagent([]string{"--task", taskPath})
	if subCmd == nil {
		logger.LogFatal("Failed to start subagent")
	}

	logger.LogInfo("Monitoring subagent progress...")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if subCmd.ProcessState != nil && subCmd.ProcessState.Exited() {
				if subCmd.ProcessState.Success() {
					logger.LogInfo("Subagent completed successfully.")
					goto verification
				} else {
					logger.LogInfo("Subagent failed. Attempting restart...")
					subCmd = StartSubagent([]string{"--task", taskPath})
					continue
				}
			}

			logger.LogInfo("Checking subagent logs for drift...")
			if !CheckSubagentProgress(branchName) {
				logger.LogInfo("Subagent seems off-track. Killing and restarting...")
				subCmd.Process.Kill()
				subCmd = StartSubagent([]string{"--task", taskPath})
			}
		}
		time.Sleep(1 * time.Second)
		if subCmd.ProcessState != nil && subCmd.ProcessState.Exited() {
			break
		}
	}

verification:
	logger.LogInfo("Subagent finished. Running verification tests...")
	test_cli.RunTest([]string{})

	logger.LogInfo("Tests passed. Creating pull request...")
	github_cli.RunGithub([]string{"pull-request", "--title", fmt.Sprintf("%s: autonomous fix", branchName), "--body", fmt.Sprintf("Autonomous fix for ticket #%d\n\nSee %s for details.", selectedTicket.Number, taskPath)})

	logger.LogInfo("Autonomous developer loop completed for ticket #%d", selectedTicket.Number)
}

func CheckSubagentProgress(branchName string) bool {
	logPath := "opencode.log"
	data, err := os.ReadFile(logPath)
	if err != nil {
		return true
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) < 10 {
		return true
	}

	lastLogs := lines[len(lines)-10:]
	logger.LogInfo("Prompt: Analyzing last 10 lines of %s...", logPath)
	for _, line := range lastLogs {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(trimmed, "stuck") ||
			strings.Contains(trimmed, "loop detected") ||
			strings.Contains(trimmed, "installing") ||
			strings.Contains(trimmed, "edit") ||
			strings.Contains(trimmed, "write") ||
			strings.Contains(trimmed, "illegal operation") {
			return false
		}
	}
	return true
}

func StartSubagent(args []string) *exec.Cmd {
	var taskFile string
	for i := 0; i < len(args); i++ {
		if args[i] == "--task" && i+1 < len(args) {
			taskFile = args[i+1]
			i++
		}
	}

	if taskFile == "" {
		logger.LogInfo("Usage: dialtone ai subagent --task <file>")
		return nil
	}

	logger.LogInfo("Subagent starting task: %s", taskFile)
	opencodePath := os.ExpandEnv("$HOME/.opencode/bin/opencode")
	if _, err := os.Stat(opencodePath); os.IsNotExist(err) {
		logger.LogInfo("Default subagent (opencode) not found.")
		return nil
	}

	taskContent, err := os.ReadFile(taskFile)
	if err != nil {
		logger.LogInfo("Failed to read task file %s: %v", taskFile, err)
		return nil
	}

	cmd := exec.Command(opencodePath, "run", string(taskContent))
	logFile, err := os.OpenFile("opencode.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		logger.LogInfo("Failed to open subagent log: %v", err)
		return nil
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logger.LogInfo("Failed to start subagent: %v", err)
		return nil
	}
	return cmd
}

func RunSubagent(args []string) {
	cmd := StartSubagent(args)
	if cmd != nil {
		cmd.Wait()
		logger.LogInfo("Subagent completed.")
	}
}
