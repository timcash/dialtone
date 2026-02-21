package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

var globalTasksDir string

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// Default tasks directory
	globalTasksDir = filepath.Join("src", "plugins", "task", "src_v1", "tasks")

	// Simple global flag parsing
	var filteredArgs []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--tasks-dir" && i+1 < len(os.Args) {
			globalTasksDir = os.Args[i+1]
			i++
			continue
		}
		filteredArgs = append(filteredArgs, arg)
	}

	if len(filteredArgs) < 1 {
		printUsage()
		return
	}

	command := filteredArgs[0]
	args := filteredArgs[1:]

	switch command {
	case "create":
		runCreate(args)
	case "validate":
		runValidate(args)
	case "archive":
		runArchive(args)
	case "sign":
		runSign(args)
	case "sync":
		runSync(args)
	case "help":
		printUsage()
	default:
		logs.Error("Unknown task command: %s", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: task [global-options] <command> [arguments]")
	fmt.Println("\nGlobal Options:")
	fmt.Println("  --tasks-dir <path>   Override default tasks directory")
	fmt.Println("\nCommands:")
	fmt.Println("  create <task-name>   Create a new task in tasks/<name>/v1/root.md")
	fmt.Println("  validate <task-name> Validate a task markdown file")
	fmt.Println("  archive <task-name>  Promote v2 to v1 and prepare for next cycle")
	fmt.Println("  sign <task-name> --role <role>  Sign a task in v2")
	fmt.Println("  sync [issue-id]      Sync GitHub issues into tasks/ folder")
}

func getTasksDir() string {
	if envDir := os.Getenv("DIALTONE_TASKS_DIR"); envDir != "" {
		return envDir
	}
	// If it's not the default, it means it was set via flag
	defaultDir := filepath.Join("src", "plugins", "task", "src_v1", "tasks")
	if globalTasksDir != defaultDir {
		return globalTasksDir
	}
	root, err := findRepoRoot()
	if err != nil {
		return globalTasksDir // Fallback
	}
	return filepath.Join(root, "src", "plugins", "task", "src_v1", "tasks")
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", fmt.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func runCreate(args []string) {
	if len(args) < 1 {
		logs.Error("Usage: task create <task-name>")
		return
	}
	taskName := args[0]

	baseDir := filepath.Join(getTasksDir(), taskName, "v1")
	v2Dir := filepath.Join(getTasksDir(), taskName, "v2")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		logs.Error("Error creating directory %s: %v", baseDir, err)
		return
	}
	if err := os.MkdirAll(v2Dir, 0755); err != nil {
		logs.Error("Error creating directory %s: %v", v2Dir, err)
		return
	}

	filename := filepath.Join(baseDir, "root.md")
	v2filename := filepath.Join(v2Dir, "root.md")

	if _, err := os.Stat(filename); err == nil {
		logs.Error("Error: Task file already exists at %s", filename)
		return
	}

	content := fmt.Sprintf(`# %s
### description:
TODO: Add description here.
### tags:
- todo
### task-dependencies:
- none
### documentation:
- none
### test-condition-1:
- TODO: Add test condition
### test-command:
- TODO: Add test command
### reviewed:
- none
### tested:
- none
### last-error-types:
- none
### last-error-times:
- none
### log-stream-command:
- TODO: Add log command
### last-error-loglines:
- none
### notes:
`, taskName)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		logs.Error("Error writing file %s: %v", filename, err)
		return
	}
	if err := os.WriteFile(v2filename, []byte(content), 0644); err != nil {
		logs.Error("Error writing file %s: %v", v2filename, err)
		return
	}

	logs.Info("Created new task: %s and %s", filename, v2filename)
}

func runValidate(args []string) {
	if len(args) < 1 {
		logs.Error("Usage: task validate <task-name>")
		return
	}
	taskName := args[0]
	path := filepath.Join(getTasksDir(), taskName, "v2", "root.md")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(getTasksDir(), taskName, "v1", "root.md")
	}

	if _, err := os.Stat(path); err != nil {
		logs.Error("Task %s not found in v1 or v2 (searched %s)", taskName, getTasksDir())
		return
	}

	file, err := os.Open(path)
	if err != nil {
		logs.Error("Error opening file %s: %v", path, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	errors := []string{}
	lineNum := 0
	var currentSection string
	sectionRegex := regexp.MustCompile(`^### ([a-z0-9-]+):$`)
	listRegex := regexp.MustCompile(`^- .*`)
	commentRegex := regexp.MustCompile(`^# .*`)
	sigRegex := regexp.MustCompile(`^- [A-Z0-9:-]+> .+ :: .+`)
	foundHeader := false

	h1Count := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "# ") {
			h1Count++
			if lineNum != 1 {
				errors = append(errors, fmt.Sprintf("Line %d: H1 header ('# ') is only allowed on line 1", lineNum))
			}
			foundHeader = true
			continue
		}

		if lineNum == 1 && !strings.HasPrefix(line, "# ") {
			errors = append(errors, fmt.Sprintf("Line 1: Must be a header '# task-name' (found: '%s')", line))
		}
		if trimmed == "" { continue }
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 0 {
			currentSection = matches[1]
			continue
		}
		if currentSection == "description" || currentSection == "notes" { continue }
		if currentSection == "reviewed" || currentSection == "tested" {
			if commentRegex.MatchString(line) { continue }
			if line == "- none" { continue }
			if listRegex.MatchString(line) {
				if !sigRegex.MatchString(line) {
					errors = append(errors, fmt.Sprintf("Line %d: Invalid signature format in '%s'. Expected '- ACTOR> timestamp :: key'", lineNum, currentSection))
				}
				continue
			}
			errors = append(errors, fmt.Sprintf("Line %d: Invalid content in '%s'. Expected list item or comment.", lineNum, currentSection))
			continue
		}
		isList := listRegex.MatchString(line)
		isComment := commentRegex.MatchString(line)
		if !isList && !isComment {
			errors = append(errors, fmt.Sprintf("Line %d: Invalid line in section '%s'. Must be bullet point ('- ') or comment ('# '). Found: '%s'", lineNum, currentSection, line))
		}
	}
	if !foundHeader || h1Count != 1 {
		errors = append(errors, fmt.Sprintf("Missing or multiple H1 headers (found %d, expected 1)", h1Count))
	}

	if len(errors) > 0 {
		logs.Error("Validation FAILED:")
		for _, e := range errors { logs.Error("  - %s", e) }
		os.Exit(1)
	}
	logs.Info("Validation PASSED: %s", path)
}

func runArchive(args []string) {
	if len(args) < 1 {
		logs.Error("Usage: task archive <task-name>")
		return
	}
	taskName := args[0]
	basePath := filepath.Join(getTasksDir(), taskName)
	v1Dir := filepath.Join(basePath, "v1")
	v2Dir := filepath.Join(basePath, "v2")

	if _, err := os.Stat(v2Dir); err != nil {
		logs.Error("Error: v2 directory for task %s not found in %s", taskName, getTasksDir())
		return
	}

	logs.Info("Promoting %s/v2 to v1...", taskName)
	if err := os.RemoveAll(v1Dir); err != nil {
		logs.Error("Error removing v1: %v", err)
		return
	}

	if err := os.Rename(v2Dir, v1Dir); err != nil {
		logs.Error("Error renaming v2 to v1: %v", err)
		return
	}

	if err := copyDir(v1Dir, v2Dir); err != nil {
		logs.Error("Error copying v1 to v2: %v", err)
		return
	}

	logs.Info("Successfully archived task %s. v1 and v2 now match.", taskName)
}

func runSign(args []string) {
	if len(args) < 3 {
		logs.Error("Usage: task sign <task-name> --role <role>")
		return
	}
	taskName := args[0]
	role := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--role" && i+1 < len(args) {
			role = args[i+1]
			break
		}
	}
	if role == "" {
		logs.Error("Error: --role <role> is required")
		return
	}

	v2Path := filepath.Join(getTasksDir(), taskName, "v2", "root.md")
	if _, err := os.Stat(v2Path); err != nil {
		logs.Error("Error: v2 task file not found at %s", v2Path)
		return
	}

	content, err := os.ReadFile(v2Path)
	if err != nil {
		logs.Error("Error reading file: %v", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	targetSection := "reviewed"
	if strings.Contains(strings.ToLower(role), "test") {
		targetSection = "tested"
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	signature := fmt.Sprintf("- %s> %s :: sig-%d", strings.ToUpper(role), timestamp, time.Now().UnixNano())

	finalLines := []string{}
	sectionFound := false
	for _, line := range lines {
		finalLines = append(finalLines, line)
		if line == "### "+targetSection+":" {
			finalLines = append(finalLines, signature)
			sectionFound = true
		}
	}

	if !sectionFound {
		logs.Error("Error: section ### %s: not found in %s", targetSection, v2Path)
		return
	}

	if err := os.WriteFile(v2Path, []byte(strings.Join(finalLines, "\n")), 0644); err != nil {
		logs.Error("Error writing file: %v", err)
		return
	}

	logs.Info("Successfully signed task %s as %s in v2", taskName, role)
}

func runSync(args []string) {
	issueID := ""
	if len(args) > 0 {
		issueID = args[0]
	}

	issuesDir := filepath.Join("src", "plugins", "github", "src_v1", "issues")
	entries, err := os.ReadDir(issuesDir)
	if err != nil {
		logs.Error("Error reading issues: %v", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == ".gitkeep" {
			continue
		}

		if issueID != "" && !strings.HasPrefix(entry.Name(), issueID) {
			continue
		}

		issuePath := filepath.Join(issuesDir, entry.Name())
		content, err := os.ReadFile(issuePath)
		if err != nil {
			logs.Error("Error reading issue %s: %v", entry.Name(), err)
			continue
		}

		// Use filename (minus .md) as task ID
		taskID := strings.TrimSuffix(entry.Name(), ".md")
		
		v1Dir := filepath.Join(getTasksDir(), taskID, "v1")
		v2Dir := filepath.Join(getTasksDir(), taskID, "v2")
		
		if err := os.MkdirAll(v1Dir, 0755); err != nil {
			logs.Error("Error creating v1 dir for %s: %v", taskID, err)
			continue
		}
		if err := os.MkdirAll(v2Dir, 0755); err != nil {
			logs.Error("Error creating v2 dir for %s: %v", taskID, err)
			continue
		}

		v1Path := filepath.Join(v1Dir, "root.md")
		v2Path := filepath.Join(v2Dir, "root.md")

		if err := os.WriteFile(v1Path, content, 0644); err != nil {
			logs.Error("Error writing v1 root.md for %s: %v", taskID, err)
			continue
		}
		if err := os.WriteFile(v2Path, content, 0644); err != nil {
			logs.Error("Error writing v2 root.md for %s: %v", taskID, err)
			continue
		}

		logs.Info("Synced issue %s to task %s/v1/root.md", entry.Name(), taskID)
		
		if issueID != "" {
			break 
		}
	}
}

func copyDir(src string, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil { return err }
	if err := os.MkdirAll(dst, 0755); err != nil { return err }
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil { return err }
		} else {
			if err := copyFile(srcPath, dstPath); err != nil { return err }
		}
	}
	return nil
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
