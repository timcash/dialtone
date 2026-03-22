package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ghosttyTerminal struct {
	Index            int
	ID               string
	Name             string
	WorkingDirectory string
	Focused          bool
}

type ghosttySurfaceConfig struct {
	WorkingDirectory string
	Command          string
	InitialInput     string
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
	case "new-tab":
		if err := runNewTab(args); err != nil {
			exitIfErr(err, "ghostty new-tab")
		}
	case "new-window":
		if err := runNewWindow(args); err != nil {
			exitIfErr(err, "ghostty new-window")
		}
	case "fresh-window":
		if err := runFreshWindow(args); err != nil {
			exitIfErr(err, "ghostty fresh-window")
		}
	case "split":
		if err := runSplit(args); err != nil {
			exitIfErr(err, "ghostty split")
		}
	case "focus":
		if err := runFocus(args); err != nil {
			exitIfErr(err, "ghostty focus")
		}
	case "fullscreen":
		if err := runFullscreen(args); err != nil {
			exitIfErr(err, "ghostty fullscreen")
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

func runNewTab(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 new-tab", flag.ContinueOnError)
	config := bindSurfaceConfigFlags(opts)
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("new-tab does not accept positional arguments")
	}

	out, err := runAppleScript(buildNewTabScript(config))
	if err != nil {
		return err
	}
	tabID, tabIndex, terminalID, err := parseCreatedTabResult(out)
	if err != nil {
		return err
	}
	fmt.Printf("created ghostty tab %d (id=%s) terminal (id=%s)\n", tabIndex, tabID, terminalID)
	return nil
}

func runNewWindow(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 new-window", flag.ContinueOnError)
	config := bindSurfaceConfigFlags(opts)
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("new-window does not accept positional arguments")
	}

	out, err := runAppleScript(buildNewWindowScript(config))
	if err != nil {
		return err
	}
	windowID, tabID, terminalID, err := parseCreatedWindowResult(out)
	if err != nil {
		return err
	}
	fmt.Printf("created ghostty window (id=%s) tab (id=%s) terminal (id=%s)\n", windowID, tabID, terminalID)
	return nil
}

func runFreshWindow(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 fresh-window", flag.ContinueOnError)
	config := bindSurfaceConfigFlags(opts)
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("fresh-window does not accept positional arguments")
	}

	// Best effort reset so the workflow always lands in one window with one tab.
	_, _ = runAppleScript(buildQuitScript())
	time.Sleep(500 * time.Millisecond)

	out, err := runAppleScript(buildNewWindowScript(config))
	if err != nil {
		return err
	}
	windowID, tabID, terminalID, err := parseCreatedWindowResult(out)
	if err != nil {
		return err
	}
	fmt.Printf("created fresh ghostty window (id=%s) tab (id=%s) terminal (id=%s)\n", windowID, tabID, terminalID)
	return nil
}

func runSplit(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 split", flag.ContinueOnError)
	terminalIndex := opts.Int("terminal", 1, "1-based terminal index within the selected tab of the front Ghostty window")
	direction := opts.String("direction", "right", "Split direction: right|left|down|up")
	focus := opts.Bool("focus", true, "Focus the new terminal after splitting")
	config := bindSurfaceConfigFlags(opts)
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("split does not accept positional arguments")
	}
	if *terminalIndex <= 0 {
		return errors.New("--terminal must be a positive integer")
	}
	if !isValidSplitDirection(*direction) {
		return fmt.Errorf("invalid --direction %q (expected right|left|down|up)", *direction)
	}

	out, err := runAppleScript(buildSplitScript(*terminalIndex, *direction, *focus, config))
	if err != nil {
		return err
	}
	fmt.Printf("split ghostty terminal %d %s -> terminal (id=%s)\n", *terminalIndex, *direction, strings.TrimSpace(out))
	return nil
}

func runFocus(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 focus", flag.ContinueOnError)
	terminalIndex := opts.Int("terminal", 1, "1-based terminal index within the selected tab of the front Ghostty window")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("focus does not accept positional arguments")
	}
	if *terminalIndex <= 0 {
		return errors.New("--terminal must be a positive integer")
	}

	out, err := runAppleScript(buildFocusScript(*terminalIndex))
	if err != nil {
		return err
	}
	fmt.Printf("focused ghostty terminal %d (id=%s)\n", *terminalIndex, strings.TrimSpace(out))
	return nil
}

func runFullscreen(argv []string) error {
	opts := flag.NewFlagSet("ghostty v1 fullscreen", flag.ContinueOnError)
	on := opts.Bool("on", true, "Set fullscreen on or off for the front Ghostty window")
	if err := opts.Parse(argv); err != nil {
		return err
	}
	if opts.NArg() != 0 {
		return errors.New("fullscreen does not accept positional arguments")
	}

	out, err := runAppleScript(buildFullscreenScript(*on))
	if err != nil {
		return err
	}
	fmt.Printf("set ghostty fullscreen=%t for window (id=%s)\n", *on, strings.TrimSpace(out))
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

func bindSurfaceConfigFlags(opts *flag.FlagSet) ghosttySurfaceConfig {
	config := ghosttySurfaceConfig{}
	opts.StringVar(&config.WorkingDirectory, "cwd", "", "Initial working directory for a new tab/window/split terminal")
	opts.StringVar(&config.Command, "command", "", "Command to execute instead of the default shell for a new tab/window/split terminal")
	opts.StringVar(&config.InitialInput, "input", "", "Initial input to send after creating a new tab/window/split terminal")
	return config
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

func (c ghosttySurfaceConfig) isZero() bool {
	return c.WorkingDirectory == "" && c.Command == "" && c.InitialInput == ""
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

func buildQuitScript() string {
	return strings.Join([]string{
		`do shell script "pkill -9 -x Ghostty >/dev/null 2>&1 || true"`,
	}, "\n")
}

func buildNewTabScript(config ghosttySurfaceConfig) string {
	lines := []string{
		`tell application "Ghostty"`,
		`	set win to front window`,
	}
	lines = append(lines, buildSurfaceConfigScriptLines("cfg", config)...)
	if config.isZero() {
		lines = append(lines, `	set newTab to new tab in win`)
	} else {
		lines = append(lines, `	set newTab to new tab in win with configuration cfg`)
	}
	lines = append(lines,
		`	select tab newTab`,
		`	set fieldSep to ASCII character 9`,
		`	return (id of newTab as string) & fieldSep & (index of newTab as string) & fieldSep & (id of focused terminal of newTab as string)`,
		`end tell`,
	)
	return strings.Join(lines, "\n")
}

func buildNewWindowScript(config ghosttySurfaceConfig) string {
	lines := []string{
		`tell application "Ghostty"`,
	}
	lines = append(lines, buildSurfaceConfigScriptLines("cfg", config)...)
	if config.isZero() {
		lines = append(lines, `	set newWin to new window`)
	} else {
		lines = append(lines, `	set newWin to new window with configuration cfg`)
	}
	lines = append(lines,
		`	activate window newWin`,
		`	set fieldSep to ASCII character 9`,
		`	return (id of newWin as string) & fieldSep & (id of selected tab of newWin as string) & fieldSep & (id of focused terminal of selected tab of newWin as string)`,
		`end tell`,
	)
	return strings.Join(lines, "\n")
}

func buildSplitScript(terminalIndex int, direction string, focus bool, config ghosttySurfaceConfig) string {
	lines := []string{
		`tell application "Ghostty"`,
		`	set win to front window`,
		`	set tabRef to selected tab of win`,
		fmt.Sprintf(`	set targetTerm to terminal %d of tabRef`, terminalIndex),
	}
	lines = append(lines, buildSurfaceConfigScriptLines("cfg", config)...)
	if config.isZero() {
		lines = append(lines, fmt.Sprintf(`	set newTerm to split targetTerm direction %s`, direction))
	} else {
		lines = append(lines, fmt.Sprintf(`	set newTerm to split targetTerm direction %s with configuration cfg`, direction))
	}
	if focus {
		lines = append(lines, `	focus newTerm`)
	}
	lines = append(lines,
		`	return id of newTerm as string`,
		`end tell`,
	)
	return strings.Join(lines, "\n")
}

func buildFocusScript(terminalIndex int) string {
	return strings.Join([]string{
		`tell application "Ghostty"`,
		`	set win to front window`,
		`	set tabRef to selected tab of win`,
		fmt.Sprintf(`	set targetTerm to terminal %d of tabRef`, terminalIndex),
		`	focus targetTerm`,
		`	return id of targetTerm as string`,
		`end tell`,
	}, "\n")
}

func buildFullscreenScript(on bool) string {
	return strings.Join([]string{
		`tell application "Ghostty"`,
		`	set win to front window`,
		`end tell`,
		`tell application "Ghostty" to activate`,
		`delay 0.1`,
		`tell application "System Events"`,
		`	tell process "Ghostty"`,
		fmt.Sprintf(`		set value of attribute "AXFullScreen" of window 1 to %t`, on),
		`	end tell`,
		`end tell`,
		`tell application "Ghostty"`,
		`	return id of front window as string`,
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

func buildSurfaceConfigScriptLines(varName string, config ghosttySurfaceConfig) []string {
	if config.isZero() {
		return nil
	}

	lines := []string{
		fmt.Sprintf(`	set %s to new surface configuration`, varName),
	}
	if config.WorkingDirectory != "" {
		lines = append(lines, fmt.Sprintf(`	set initial working directory of %s to %q`, varName, config.WorkingDirectory))
	}
	if config.Command != "" {
		lines = append(lines, fmt.Sprintf(`	set command of %s to %q`, varName, config.Command))
	}
	if config.InitialInput != "" {
		lines = append(lines, fmt.Sprintf(`	set initial input of %s to %q`, varName, config.InitialInput))
	}
	return lines
}

func parseCreatedTabResult(out string) (string, int, string, error) {
	parts := strings.Split(strings.TrimSpace(out), "\t")
	if len(parts) != 3 {
		return "", 0, "", fmt.Errorf("unexpected new tab result: %q", out)
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", fmt.Errorf("invalid tab index %q: %w", parts[1], err)
	}
	return parts[0], index, parts[2], nil
}

func parseCreatedWindowResult(out string) (string, string, string, error) {
	parts := strings.Split(strings.TrimSpace(out), "\t")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("unexpected new window result: %q", out)
	}
	return parts[0], parts[1], parts[2], nil
}

func isValidSplitDirection(value string) bool {
	switch value {
	case "right", "left", "down", "up":
		return true
	default:
		return false
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone_mod ghostty v1 <command> [args]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  list")
	fmt.Println("       List terminals in the selected tab of the front Ghostty window")
	fmt.Println("  new-tab [--cwd PATH] [--command CMD] [--input TEXT]")
	fmt.Println("       Create and select a new tab in the front Ghostty window")
	fmt.Println("  new-window [--cwd PATH] [--command CMD] [--input TEXT]")
	fmt.Println("       Create and activate a new Ghostty window")
	fmt.Println("  fresh-window [--cwd PATH] [--command CMD] [--input TEXT]")
	fmt.Println("       Quit Ghostty, then create one fresh Ghostty window with one tab")
	fmt.Println("  split [--terminal 1] [--direction right|left|down|up] [--focus=true|false] [--cwd PATH] [--command CMD] [--input TEXT]")
	fmt.Println("       Split a terminal in the selected tab using Ghostty's native split API")
	fmt.Println("  focus [--terminal 1]")
	fmt.Println("       Focus a specific terminal in the selected tab")
	fmt.Println("  fullscreen [--on=true|false]")
	fmt.Println("       Set fullscreen on the front Ghostty window")
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
