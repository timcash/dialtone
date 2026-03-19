package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type ghosttyTerminal struct {
	Index            int
	ID               string
	Name             string
	WorkingDirectory string
	Focused          bool
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
	case "list":
		if err := runList(args); err != nil {
			exitIfErr(err, "ghostty list")
		}
	case "write", "type":
		if err := runWrite(args); err != nil {
			exitIfErr(err, "ghostty write")
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown ghostty command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runList(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 list", flag.ContinueOnError)
	if err := opts.Parse(argv); err != nil {
		return err
	}

	terminals, err := listFrontTabTerminals()
	if err != nil {
		return err
	}
	if len(terminals) == 0 {
		fmt.Println("no ghostty terminals in selected tab")
		return nil
	}
	for _, terminal := range terminals {
		fmt.Printf("%d\tfocused=%t\tid=%s\tname=%s\tcwd=%s\n",
			terminal.Index,
			terminal.Focused,
			terminal.ID,
			terminal.Name,
			terminal.WorkingDirectory,
		)
	}
	return nil
}

func runWrite(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 write", flag.ContinueOnError)
	terminalIndex := opts.Int("terminal", 1, "1-based terminal index within the selected tab of the front Ghostty window")
	enter := opts.Bool("enter", true, "Send Enter after typing text")
	focus := opts.Bool("focus", false, "Focus the target terminal before typing")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if *terminalIndex <= 0 {
		return errors.New("--terminal must be a positive integer")
	}
	if opts.NArg() == 0 {
		return errors.New("write requires text to send")
	}
	text := strings.TrimSpace(strings.Join(opts.Args(), " "))
	if text == "" {
		return errors.New("write text is empty")
	}

	script := buildWriteScript(*terminalIndex, text, *enter, *focus)
	out, err := runAppleScript(script)
	if err != nil {
		return err
	}
	fmt.Printf("wrote to ghostty terminal %d", *terminalIndex)
	if strings.TrimSpace(out) != "" {
		fmt.Printf(" (id=%s)", strings.TrimSpace(out))
	}
	fmt.Printf(": %s\n", text)
	return nil
}

func listFrontTabTerminals() ([]ghosttyTerminal, error) {
	out, err := runAppleScript(buildListScript())
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(out)
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	terminals := make([]ghosttyTerminal, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 5 {
			return nil, fmt.Errorf("unexpected Ghostty terminal row: %q", line)
		}
		index, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid terminal index %q: %w", parts[0], err)
		}
		terminals = append(terminals, ghosttyTerminal{
			Index:            index,
			ID:               parts[1],
			Name:             parts[2],
			WorkingDirectory: parts[3],
			Focused:          strings.EqualFold(parts[4], "true"),
		})
	}
	return terminals, nil
}

func runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return "", fmt.Errorf("osascript failed: %s", text)
	}
	return text, nil
}

func buildListScript() string {
	return strings.Join([]string{
		`tell application "Ghostty"`,
		`	set win to front window`,
		`	set tabRef to selected tab of win`,
		`	set focusedTerm to focused terminal of tabRef`,
		`	set linesOut to {}`,
		`	set fieldSep to ASCII character 9`,
		`	set idx to 1`,
		`	repeat with t in (terminals of tabRef)`,
		`		set isFocused to false`,
		`		if (id of t as string) is equal to (id of focusedTerm as string) then`,
		`			set isFocused to true`,
		`		end if`,
		`		set lineText to (idx as string) & fieldSep & (id of t as string) & fieldSep & (name of t as string) & fieldSep & (working directory of t as string) & fieldSep & (isFocused as string)`,
		`		set end of linesOut to lineText`,
		`		set idx to idx + 1`,
		`	end repeat`,
		`	set AppleScript's text item delimiters to linefeed`,
		`	set joinedText to linesOut as text`,
		`	set AppleScript's text item delimiters to ""`,
		`	return joinedText`,
		`end tell`,
	}, "\n")
}

func buildWriteScript(terminalIndex int, text string, enter, focus bool) string {
	lines := []string{
		`tell application "Ghostty"`,
		`	set win to front window`,
		`	set tabRef to selected tab of win`,
		fmt.Sprintf(`	set targetTerm to terminal %d of tabRef`, terminalIndex),
	}
	if focus {
		lines = append(lines, `	focus targetTerm`)
	}
	lines = append(lines,
		fmt.Sprintf(`	input text %q to targetTerm`, text),
	)
	if enter {
		lines = append(lines, `	send key "enter" to targetTerm`)
	}
	lines = append(lines,
		`	return id of targetTerm as string`,
		`end tell`,
	)
	return strings.Join(lines, "\n")
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod ghostty v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list")
	fmt.Println("       List terminals in the selected tab of the front Ghostty window")
	fmt.Println("  write [--terminal 1] [--enter=true|false] [--focus=false] <text...>")
	fmt.Println("       Type text into a specific Ghostty terminal in the selected tab of the front window")
}

func exitIfErr(err error, context string) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", context, err)
	os.Exit(1)
}
