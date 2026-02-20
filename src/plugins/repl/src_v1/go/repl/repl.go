package repl

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"dialtone/dev/plugins/proc/src_v1/go/proc"
)

func Start(logFn func(category, msg string)) error {
	if logFn == nil {
		logFn = func(string, string) {}
	}

	say := func(msg string) {
		fmt.Println("DIALTONE> " + msg)
		logs.Info("[REPL] DIALTONE> %s", msg)
		logFn("REPL", "DIALTONE> "+msg)
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
			proc.ListProcesses()
			continue
		}

		cmdStr := line
		if strings.HasPrefix(line, "@DIALTONE ") {
			cmdStr = line[len("@DIALTONE "):]
		} else if strings.HasPrefix(line, "@dialtone.sh ") {
			cmdStr = line[len("@dialtone.sh "):]
		}

		args := strings.Fields(cmdStr)
		if len(args) == 0 {
			continue
		}

		cmdName := args[0]
		if len(args) > 1 {
			cmdName += " " + args[1]
		}

		if strings.Join(args, " ") == "proc test src_v1" {
			proc.RunTestSrcV1()
			continue
		}

		isBackground := false
		if len(args) > 0 && args[len(args)-1] == "&" {
			isBackground = true
			args = args[:len(args)-1]
			cmdName = strings.TrimSuffix(cmdName, " &")
		}

		say(fmt.Sprintf("Request received. Spawning subtone for %s...", cmdName))
		if isBackground {
			go proc.RunSubtone(args)
		} else {
			proc.RunSubtone(args)
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
` + "`" + `@DIALTONE dev install` + "`" + `
Install latest Go and bootstrap dev.go command scaffold

### Plugins
` + "`" + `robot install src_v1` + "`" + `
Install robot src_v1 dependencies

` + "`" + `dag install src_v3` + "`" + `
Install dag src_v3 dependencies

### System
` + "`" + `ps` + "`" + `
List active subtones

` + "`" + `<any command>` + "`" + `
Forward to @./dialtone.sh <command>`

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
