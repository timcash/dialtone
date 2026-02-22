package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

type Hooks struct {
	RunSubtoneWithEvents func(args []string, onEvent proc.SubtoneEventHandler) int
	ListManaged          func() []proc.ManagedProcessSnapshot
	KillManagedProcess   func(pid int) error
}

var (
	runSubtoneWithEventsFn = proc.RunSubtoneWithEvents
	listManagedFn          = proc.ListManagedProcesses
	killManagedProcessFn   = proc.KillManagedProcess
)

// SetHooksForTest overrides REPL side-effect functions and returns a restore function.
func SetHooksForTest(h Hooks) func() {
	prevRunSubtoneWithEvents := runSubtoneWithEventsFn
	prevListManaged := listManagedFn
	prevKillManaged := killManagedProcessFn

	if h.RunSubtoneWithEvents != nil {
		runSubtoneWithEventsFn = h.RunSubtoneWithEvents
	}
	if h.ListManaged != nil {
		listManagedFn = h.ListManaged
	}
	if h.KillManagedProcess != nil {
		killManagedProcessFn = h.KillManagedProcess
	}
	return func() {
		runSubtoneWithEventsFn = prevRunSubtoneWithEvents
		listManagedFn = prevListManaged
		killManagedProcessFn = prevKillManaged
	}
}

func Start(logFn func(category, msg string)) error {
	if logFn == nil {
		logFn = func(string, string) {}
	}

	say := func(msg string) {
		fmt.Println("DIALTONE> " + msg)
		logs.Info("[REPL] DIALTONE> %s", msg)
		logFn("REPL", "DIALTONE> "+msg)
	}
	saySubtone := func(pid int, msg string) {
		line := fmt.Sprintf("DIALTONE:%d> %s", pid, msg)
		fmt.Println(line)
		logs.Info("[REPL] %s", line)
		logFn("REPL", line)
	}

	say("Virtual Librarian online.")
	say("Type 'help' for commands, or 'exit' to quit.")

	scanner := bufio.NewScanner(os.Stdin)
	tty := isTTY()

	for {
		fmt.Print("USER-1> ")
		if !scanner.Scan() {
			say("Session closed.")
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if !tty {
			fmt.Println(line)
		}
		logFn("REPL", "USER-1> "+line)

		if line == "exit" || line == "quit" {
			say("Goodbye.")
			break
		}
		if line == "help" {
			printHelp(logFn)
			continue
		}
		if line == "ps" {
			printManagedProcesses(say)
			continue
		}
		if strings.HasPrefix(line, "kill ") {
			pidText := strings.TrimSpace(strings.TrimPrefix(line, "kill"))
			pid := 0
			if _, err := fmt.Sscanf(pidText, "%d", &pid); err != nil || pid <= 0 {
				say("Usage: kill <pid>")
				continue
			}
			if err := killManagedProcessFn(pid); err != nil {
				say(fmt.Sprintf("Failed to kill process %d: %v", pid, err))
			} else {
				say(fmt.Sprintf("Killed managed process %d.", pid))
			}
			continue
		}

		args := strings.Fields(line)
		if len(args) == 0 {
			continue
		}

		cmdName := args[0]
		if len(args) > 1 {
			cmdName += " " + args[1]
		}

		isBackground := false
		if len(args) > 0 && args[len(args)-1] == "&" {
			isBackground = true
			args = args[:len(args)-1]
			cmdName = strings.TrimSuffix(cmdName, " &")
		}

		say(fmt.Sprintf("Request received. Spawning subtone for %s...", cmdName))
		onEvent := func(ev proc.SubtoneEvent) {
			switch ev.Type {
			case proc.SubtoneEventStarted:
				if ev.PID <= 0 {
					return
				}
				saySubtone(ev.PID, fmt.Sprintf("Started at %s", ev.StartedAt.Format(time.RFC3339)))
				saySubtone(ev.PID, fmt.Sprintf("Command: %v", ev.Args))
				if strings.TrimSpace(ev.LogPath) != "" {
					saySubtone(ev.PID, fmt.Sprintf("Log: %s", ev.LogPath))
				}
			case proc.SubtoneEventStdout:
				if ev.PID <= 0 {
					return
				}
				if hasStructuredLevel(ev.Line) {
					saySubtone(ev.PID, ev.Line)
				}
			case proc.SubtoneEventStderr:
				if ev.PID <= 0 {
					return
				}
				line := strings.TrimSpace(ev.Line)
				if line == "" {
					return
				}
				saySubtone(ev.PID, "[ERROR] "+line)
			}
		}
		if isBackground {
			go runSubtoneWithEventsFn(args, onEvent)
		} else {
			exitCode := runSubtoneWithEventsFn(args, onEvent)
			say(fmt.Sprintf("Subtone for %s exited with code %d.", cmdName, exitCode))
		}
	}
	return scanner.Err()
}

func isTTY() bool {
	fi, _ := os.Stdin.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func printHelp(logFn func(category, msg string)) {
	content := `Help

### Bootstrap
` + "`" + `dev install` + "`" + `
Install latest Go and bootstrap dev.go command scaffold

### Plugins
` + "`" + `robot src_v1 install` + "`" + `
Install robot src_v1 dependencies

` + "`" + `dag src_v3 install` + "`" + `
Install dag src_v3 dependencies

` + "`" + `logs src_v1 test` + "`" + `
Run logs plugin tests on a subtone

### System
` + "`" + `ps` + "`" + `
List active subtones

` + "`" + `kill <pid>` + "`" + `
Kill a managed subtone process by PID

` + "`" + `<any command>` + "`" + `
Run any dialtone command on a managed subtone`

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if i == 0 {
			fmt.Println("DIALTONE> " + line)
			logFn("REPL", "DIALTONE> "+line)
		} else {
			fmt.Println(line)
			logFn("REPL", line)
		}
	}
}

func printManagedProcesses(say func(msg string)) {
	procs := listManagedFn()
	if len(procs) == 0 {
		say("No active subtones.")
		return
	}
	say("Active Subtones:")
	say(fmt.Sprintf("%-8s %-8s %-10s %-8s %s", "PID", "UPTIME", "CPU%", "PORTS", "COMMAND"))
	for _, p := range procs {
		say(fmt.Sprintf("%-8d %-8s %-10.1f %-8d %s", p.PID, p.StartedAgo, p.CPUPercent, p.PortCount, p.Command))
	}
}

func hasStructuredLevel(line string) bool {
	trimmed := strings.TrimSpace(line)
	for _, prefix := range []string{"[INFO]", "[WARN]", "[ERROR]", "[COST]", "[T+"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}
