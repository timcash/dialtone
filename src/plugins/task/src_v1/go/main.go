package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

var globalTasksDir string
var globalIssuesDir string

func main() {
	// CLI tools should print to stdout by default
	logs.SetOutput(os.Stdout)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// Default tasks directory
	globalTasksDir = filepath.Join("src", "plugins", "task", "src_v1", "tasks")
	globalIssuesDir = filepath.Join("src", "plugins", "github", "src_v1", "issues")

	// Simple global flag parsing
	var filteredArgs []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--tasks-dir" && i+1 < len(os.Args) {
			globalTasksDir = os.Args[i+1]
			i++
			continue
		}
		if arg == "--issues-dir" && i+1 < len(os.Args) {
			globalIssuesDir = os.Args[i+1]
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
	case "link":
		runLink(args)
	case "unlink":
		runUnlink(args)
	case "tree":
		runTree(args)
	case "resolve":
		runResolve(args)
	case "help":
		printUsage()
	default:
		logs.Error("Unknown task command: %s", command)
		printUsage()
	}
}

func printUsage() {
	logs.Info("Usage: task [global-options] <command> [arguments]")
	logs.Info("")
	logs.Info("Global Options:")
	logs.Info("  --tasks-dir <path>   Override default tasks directory")
	logs.Info("  --issues-dir <path>  Override default github issues directory")
	logs.Info("")
	logs.Info("Commands:")
	logs.Info("  create <task-name>   Create a new task in tasks/<name>/v1/root.md")
	logs.Info("  validate <task-name> Validate a task markdown file")
	logs.Info("  archive <task-name>  Promote v2 to v1 and prepare for next cycle")
	logs.Info("  sign <task-name> --role <role>  Sign a task in v2")
	logs.Info("  sync [issue-id]      Sync GitHub issues into tasks/ folder as <id>-root")
	logs.Info("  link <a<--b>         Link b as an input to a")
	logs.Info("  link <a-->b>         Link b as an output of a")
	logs.Info("  link <a-->b-->c>     Chain multiple links in one command")
	logs.Info("  link <a-->b,b-->c>   Comma-separated link expressions")
	logs.Info("  unlink <id> <dep-id> Remove link between <id> and <dep-id>")
	logs.Info("  tree [id]            Show input dependency tree for a task")
	logs.Info("  resolve <root-id> [--pr-url URL]  Verify input tree, sign root review, and sync completion")
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
			return "", logs.Errorf("repo root not found")
		}
		cwd = parent
	}
}

func getTasksDir() string {
	if envDir := os.Getenv("DIALTONE_TASKS_DIR"); envDir != "" {
		return envDir
	}
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

func getIssuesDir() string {
	if envDir := os.Getenv("DIALTONE_ISSUES_DIR"); envDir != "" {
		return envDir
	}
	defaultDir := filepath.Join("src", "plugins", "github", "src_v1", "issues")
	if globalIssuesDir != defaultDir {
		return globalIssuesDir
	}
	root, err := findRepoRoot()
	if err != nil {
		return globalIssuesDir
	}
	return filepath.Join(root, "src", "plugins", "github", "src_v1", "issues")
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

	content := "# " + taskName + `
### description:
TODO: Add description here.
### tags:
- todo
### token_est:
- none
### time_est:
- none
### inputs:
- none
### outputs:
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
`

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
	sectionRegex := regexp.MustCompile("^### ([a-z0-9_-]+):$")
	listRegex := regexp.MustCompile("^- .*")
	commentRegex := regexp.MustCompile("^# .*")
	sigRegex := regexp.MustCompile("^- [A-Z0-9:-]+> .+ :: .+")
	foundHeader := false

	h1Count := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(line, "# ") {
			h1Count++
			if lineNum != 1 {
				errors = append(errors, "Line "+strconv.FormatInt(int64(lineNum), 10)+": H1 header ('# ') is only allowed on line 1")
			}
			foundHeader = true
			continue
		}

		if lineNum == 1 && !strings.HasPrefix(line, "# ") {
			errors = append(errors, "Line 1: Must be a header '# task-name' (found: '"+line+"')")
		}
		if trimmed == "" {
			continue
		}
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 0 {
			currentSection = matches[1]
			continue
		}
		if currentSection == "description" || currentSection == "notes" {
			continue
		}
		if currentSection == "reviewed" || currentSection == "tested" {
			if commentRegex.MatchString(line) {
				continue
			}
			if line == "- none" {
				continue
			}
			if listRegex.MatchString(line) {
				if !sigRegex.MatchString(line) {
					errors = append(errors, "Line "+strconv.FormatInt(int64(lineNum), 10)+": Invalid signature format in '"+currentSection+"'. Expected '- ACTOR> timestamp :: key'")
				}
				continue
			}
			errors = append(errors, "Line "+strconv.FormatInt(int64(lineNum), 10)+": Invalid content in '"+currentSection+"'. Expected list item or comment.")
			continue
		}
		isList := listRegex.MatchString(line)
		isComment := commentRegex.MatchString(line)
		if !isList && !isComment {
			errors = append(errors, "Line "+strconv.FormatInt(int64(lineNum), 10)+": Invalid line in section '"+currentSection+"'. Must be bullet point ('- ') or comment ('# '). Found: '"+line+"'")
		}
	}
	if !foundHeader || h1Count != 1 {
		errors = append(errors, "Missing or multiple H1 headers")
	}

	if len(errors) > 0 {
		logs.Error("Validation FAILED:")
		for _, e := range errors {
			logs.Error("  - %s", e)
		}
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
	signature := "- " + strings.ToUpper(role) + "> " + timestamp + " :: sig-" + strconv.FormatInt(time.Now().UnixNano(), 10)

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

	issuesDir := getIssuesDir()
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

		// Root task folder is issue id.
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

		orig := string(content)
		deps := parseDependencies(orig)
		md := normalizeIssueAsTask(taskID, orig)

		if err := os.WriteFile(v1Path, []byte(md), 0644); err != nil {
			logs.Error("Error writing v1 root.md for %s: %v", taskID, err)
			continue
		}
		if err := os.WriteFile(v2Path, []byte(md), 0644); err != nil {
			logs.Error("Error writing v2 root.md for %s: %v", taskID, err)
			continue
		}

		// Build dependency tree links from issue task-dependencies.
		for _, dep := range deps {
			if err := ensureTaskExists(dep); err != nil {
				logs.Error("Failed to create dependency task %s: %v", dep, err)
				continue
			}
			if err := updateSection(taskID, "inputs", dep, "../../"+dep+"/v2/root.md"); err != nil {
				logs.Error("Failed to link dependency %s -> %s: %v", dep, taskID, err)
				continue
			}
			if err := updateSection(dep, "outputs", taskID, "../../"+taskID+"/v2/root.md"); err != nil {
				logs.Error("Failed to link reverse output %s -> %s: %v", dep, taskID, err)
				continue
			}
		}

		// Root tasks should have no outputs.
		if err := setSectionToNone(taskID, "outputs"); err != nil {
			logs.Warn("Could not force outputs:none on root task %s: %v", taskID, err)
		}

		logs.Info("Synced issue %s to task %s/v1/root.md and %s/v2/root.md", entry.Name(), taskID, taskID)

		if issueID != "" {
			break
		}
	}
}

func normalizeIssueAsTask(taskID, md string) string {
	lines := strings.Split(md, "\n")
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		lines[0] = "# " + taskID
	}
	md = strings.Join(lines, "\n")

	issueURL := bulletValue(md, "url")
	if issueURL == "" {
		issueURL = "none"
	}

	if !strings.Contains(md, "### token_est:") {
		md += "\n### token_est:\n- none\n"
	}
	if !strings.Contains(md, "### time_est:") {
		md += "\n### time_est:\n- none\n"
	}
	if !strings.Contains(md, "### inputs:") {
		if strings.Contains(md, "### task-dependencies:") {
			md = strings.Replace(md, "### task-dependencies:", "### inputs:\n- none", 1)
		} else {
			md += "\n### inputs:\n- none\n"
		}
	}
	if !strings.Contains(md, "### outputs:") {
		md += "\n### outputs:\n- none\n"
	}
	if !strings.Contains(md, "### issue:") {
		md += "\n### issue:\n"
		if issueURL == "none" {
			md += "- none\n"
		} else {
			md += "- [#" + taskID + "](" + issueURL + ")\n"
		}
	}
	if !strings.Contains(md, "### pr:") {
		md += "### pr:\n- none\n"
	}
	return md
}

func bulletValue(md, key string) string {
	lines := strings.Split(md, "\n")
	prefix := "- " + key + ":"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}

func parseDependencies(md string) []string {
	lines := strings.Split(md, "\n")
	inDeps := false
	out := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "### task-dependencies:" {
			inDeps = true
			continue
		}
		if inDeps {
			if strings.HasPrefix(trimmed, "### ") {
				break
			}
			if !strings.HasPrefix(trimmed, "- ") {
				continue
			}
			v := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if v == "" || v == "none" {
				continue
			}
			out = append(out, sanitizeTaskID(v))
		}
	}
	return unique(out)
}

func sanitizeTaskID(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "-")
	re := regexp.MustCompile(`[^a-z0-9\-_]+`)
	v = re.ReplaceAllString(v, "")
	for strings.Contains(v, "--") {
		v = strings.ReplaceAll(v, "--", "-")
	}
	return strings.Trim(v, "-")
}

func unique(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func ensureTaskExists(taskID string) error {
	baseDir := filepath.Join(getTasksDir(), taskID, "v1")
	v2Dir := filepath.Join(getTasksDir(), taskID, "v2")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(v2Dir, 0755); err != nil {
		return err
	}
	v1Path := filepath.Join(baseDir, "root.md")
	v2Path := filepath.Join(v2Dir, "root.md")
	if _, err := os.Stat(v1Path); err == nil {
		return nil
	}
	content := "# " + taskID + `
### description:
- TODO: task dependency generated from issue sync
### tags:
- task
### token_est:
- none
### time_est:
- none
### inputs:
- none
### outputs:
- none
### issue:
- none
### pr:
- none
### documentation:
- none
### test-condition-1:
- TODO
### test-command:
- TODO
### reviewed:
- none
### tested:
- none
### last-error-types:
- none
### last-error-times:
- none
### log-stream-command:
- TODO
### last-error-loglines:
- none
### notes:
`
	if err := os.WriteFile(v1Path, []byte(content), 0644); err != nil {
		return err
	}
	return os.WriteFile(v2Path, []byte(content), 0644)
}

func runLink(args []string) {
	if len(args) < 1 {
		logs.Error("Usage: task link <a<--b|a-->b|a-->b-->c|a-->b,b-->c>")
		return
	}

	specs, err := parseLinkSpecs(args)
	if err != nil {
		logs.Error("%v", err)
		return
	}
	for _, spec := range specs {
		if spec.direction == "input" {
			// 'to' is INPUT to 'from'; reverse OUTPUT on dependency.
			if err := updateSection(spec.from, "inputs", spec.to, "../../"+spec.to+"/v2/root.md"); err != nil {
				logs.Error("Failed to update inputs for %s: %v", spec.from, err)
				return
			}
			if err := updateSection(spec.to, "outputs", spec.from, "../../"+spec.from+"/v2/root.md"); err != nil {
				logs.Error("Failed to update outputs for %s: %v", spec.to, err)
				return
			}
			logs.Info("Linked %s as input to %s", spec.to, spec.from)
			continue
		}

		// 'to' is OUTPUT of 'from'; reverse INPUT on destination.
		if err := updateSection(spec.from, "outputs", spec.to, "../../"+spec.to+"/v2/root.md"); err != nil {
			logs.Error("Failed to update outputs for %s: %v", spec.from, err)
			return
		}
		if err := updateSection(spec.to, "inputs", spec.from, "../../"+spec.from+"/v2/root.md"); err != nil {
			logs.Error("Failed to update inputs for %s: %v", spec.to, err)
			return
		}
		logs.Info("Linked %s as output of %s", spec.to, spec.from)
	}
}

type linkSpec struct {
	from      string
	to        string
	direction string // input|output
}

func parseLinkSpecs(args []string) ([]linkSpec, error) {
	raw := strings.Join(args, " ")
	raw = strings.ReplaceAll(raw, " ", "")
	if raw == "" {
		return nil, logs.Errorf("missing link expression")
	}

	// Backward compatibility: task link a b  -> a<--b
	if !strings.Contains(raw, "-->") && !strings.Contains(raw, "<--") && len(args) >= 2 {
		return []linkSpec{{from: args[0], to: args[1], direction: "input"}}, nil
	}

	var specs []linkSpec
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		switch {
		case strings.Contains(token, "<--") && strings.Contains(token, "-->"):
			return nil, logs.Errorf("mixed arrow directions in one token are not supported: %s", token)
		case strings.Contains(token, "<--"):
			parts := strings.Split(token, "<--")
			if len(parts) < 2 {
				return nil, logs.Errorf("invalid link token: %s", token)
			}
			for i := 0; i < len(parts)-1; i++ {
				a := sanitizeTaskID(parts[i])
				b := sanitizeTaskID(parts[i+1])
				if a == "" || b == "" {
					return nil, logs.Errorf("invalid link token: %s", token)
				}
				specs = append(specs, linkSpec{from: a, to: b, direction: "input"})
			}
		case strings.Contains(token, "-->"):
			parts := strings.Split(token, "-->")
			if len(parts) < 2 {
				return nil, logs.Errorf("invalid link token: %s", token)
			}
			for i := 0; i < len(parts)-1; i++ {
				a := sanitizeTaskID(parts[i])
				b := sanitizeTaskID(parts[i+1])
				if a == "" || b == "" {
					return nil, logs.Errorf("invalid link token: %s", token)
				}
				specs = append(specs, linkSpec{from: a, to: b, direction: "output"})
			}
		default:
			return nil, logs.Errorf("invalid link token: %s", token)
		}
	}
	if len(specs) == 0 {
		return nil, logs.Errorf("missing link expression")
	}
	return specs, nil
}

func updateSection(id, section, targetID, relPath string) error {
	v2Path := filepath.Join(getTasksDir(), id, "v2", "root.md")
	content, err := os.ReadFile(v2Path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	newLines := []string{}
	foundHeader := false
	alreadyLinked := false

	linkItem := "- [" + targetID + "](" + relPath + ")"

	for _, line := range lines {
		if line == "### "+section+":" {
			foundHeader = true
			newLines = append(newLines, line)
			continue
		}
		if foundHeader {
			if strings.TrimSpace(line) == "- none" {
				newLines = append(newLines, linkItem)
				foundHeader = false
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(line), "- ") {
				if strings.Contains(line, "["+targetID+"]") {
					alreadyLinked = true
				}
				newLines = append(newLines, line)
				continue
			}
			// Reached next section
			if !alreadyLinked {
				newLines = append(newLines, linkItem)
			}
			foundHeader = false
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(v2Path, []byte(strings.Join(newLines, "\n")), 0644)
}

func runUnlink(args []string) {
	if len(args) < 2 {
		logs.Error("Usage: task unlink <task-id> <dep-task-id>")
		return
	}
	id, depID := args[0], args[1]

	if err := removeLink(id, "inputs", depID); err != nil {
		logs.Error("Failed to unlink input: %v", err)
	}
	if err := removeLink(depID, "outputs", id); err != nil {
		logs.Error("Failed to unlink output: %v", err)
	}
	logs.Info("Unlinked %s and %s", id, depID)
}

func removeLink(id, section, targetID string) error {
	v2Path := filepath.Join(getTasksDir(), id, "v2", "root.md")
	content, err := os.ReadFile(v2Path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	newLines := []string{}
	inSection := false
	removed := false
	for _, line := range lines {
		if line == "### "+section+":" {
			inSection = true
			newLines = append(newLines, line)
			continue
		}
		if inSection {
			if strings.Contains(line, "["+targetID+"]") {
				removed = true
				continue
			}
			if strings.HasPrefix(line, "### ") || line == "" {
				inSection = false
				if newLines[len(newLines)-1] == "### "+section+":" {
					newLines = append(newLines, "- none")
				}
			}
		}
		newLines = append(newLines, line)
	}
	_ = removed
	return os.WriteFile(v2Path, []byte(strings.Join(newLines, "\n")), 0644)
}

func runTree(args []string) {
	rootID := ""
	if len(args) > 0 {
		rootID = args[0]
	}

	if rootID != "" {
		printTaskTree(rootID, 0, make(map[string]bool))
	} else {
		entries, _ := os.ReadDir(getTasksDir())
		for _, e := range entries {
			if e.IsDir() {
				logs.Raw("")
				logs.Raw("--- Tree for %s ---", e.Name())
				printTaskTree(e.Name(), 0, make(map[string]bool))
			}
		}
	}
}

func runResolve(args []string) {
	if len(args) < 1 {
		logs.Error("Usage: task resolve <root-id> [--pr-url URL]")
		return
	}
	rootID := strings.TrimSpace(args[0])
	prURL := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--pr-url" && i+1 < len(args) {
			prURL = strings.TrimSpace(args[i+1])
			i++
		}
	}

	incomplete, err := collectIncompleteInputs(rootID, map[string]bool{})
	if err != nil {
		logs.Error("Resolve failed: %v", err)
		return
	}
	if len(incomplete) > 0 {
		logs.Error("Resolve blocked: inputs not complete for root %s: %s", rootID, strings.Join(incomplete, ", "))
		os.Exit(1)
	}
	if !isTaskDone(rootID) {
		logs.Error("Resolve blocked: root task %s is not done (needs reviewed+tested signatures)", rootID)
		os.Exit(1)
	}

	// Final review signature on root.
	runSign([]string{rootID, "--role", "REVIEW"})
	if err := setSignatureStatus(rootID, "done"); err != nil {
		logs.Warn("Failed setting root signature status done: %v", err)
	}
	if prURL != "" {
		if err := setSingleBullet(rootID, "pr", "- [PR]("+prURL+")"); err != nil {
			logs.Warn("Failed writing PR link: %v", err)
		}
	}
	if err := syncTaskDoneToIssue(rootID, prURL); err != nil {
		logs.Warn("Failed syncing completion back to issue markdown: %v", err)
	}
	logs.Info("Resolved root task %s: dependency tree done, review signed, issue sync updated", rootID)
}

func printTaskTree(id string, indent int, visited map[string]bool) {
	prefix := strings.Repeat("  ", indent)
	if visited[id] {
		logs.Raw("%s- %s (circular dependency!)", prefix, id)
		return
	}
	visited[id] = true
	logs.Raw("%s- %s", prefix, id)

	path := filepath.Join(getTasksDir(), id, "v2", "root.md")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(getTasksDir(), id, "v1", "root.md")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	inSection := false
	for _, line := range lines {
		if line == "### inputs:" {
			inSection = true
			continue
		}
		if inSection {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				// Parse [id](path)
				re := regexp.MustCompile(`\[(.*?)\]`)
				match := re.FindStringSubmatch(trimmed)
				if len(match) > 1 {
					dep := match[1]
					printTaskTree(dep, indent+1, visited)
				} else if strings.TrimPrefix(trimmed, "- ") != "none" {
					// Fallback for raw IDs
					printTaskTree(strings.TrimPrefix(trimmed, "- "), indent+1, visited)
				}
			} else if strings.HasPrefix(trimmed, "### ") {
				inSection = false
			}
		}
	}
	delete(visited, id)
}

func collectIncompleteInputs(taskID string, visited map[string]bool) ([]string, error) {
	if visited[taskID] {
		return nil, nil
	}
	visited[taskID] = true
	inputs, err := readLinkedTaskIDs(taskID, "inputs")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, in := range inputs {
		if in == "none" {
			continue
		}
		if !isTaskDone(in) {
			out = append(out, in)
		}
		sub, err := collectIncompleteInputs(in, visited)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return unique(out), nil
}

func readLinkedTaskIDs(taskID, section string) ([]string, error) {
	md, err := readTaskMarkdown(taskID)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(md, "\n")
	inSection := false
	var out []string
	re := regexp.MustCompile(`\[(.*?)\]`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "### "+section+":" {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			break
		}
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		if trimmed == "- none" {
			out = append(out, "none")
			continue
		}
		match := re.FindStringSubmatch(trimmed)
		if len(match) > 1 {
			out = append(out, strings.TrimSpace(match[1]))
			continue
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	}
	return unique(out), nil
}

func isTaskDone(taskID string) bool {
	md, err := readTaskMarkdown(taskID)
	if err != nil {
		return false
	}
	reviewed := sectionHasSignature(md, "reviewed")
	tested := sectionHasSignature(md, "tested")
	return reviewed && tested
}

func sectionHasSignature(md, section string) bool {
	lines := strings.Split(md, "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "### "+section+":" {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			break
		}
		if strings.HasPrefix(trimmed, "- ") && trimmed != "- none" {
			return true
		}
	}
	return false
}

func readTaskMarkdown(taskID string) (string, error) {
	v2Path := filepath.Join(getTasksDir(), taskID, "v2", "root.md")
	if _, err := os.Stat(v2Path); err == nil {
		raw, err := os.ReadFile(v2Path)
		return string(raw), err
	}
	v1Path := filepath.Join(getTasksDir(), taskID, "v1", "root.md")
	raw, err := os.ReadFile(v1Path)
	return string(raw), err
}

func setSignatureStatus(taskID, status string) error {
	path := filepath.Join(getTasksDir(), taskID, "v2", "root.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "- status:") {
			lines[i] = "- status: " + status
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
		}
	}
	return nil
}

func setSectionToNone(taskID, section string) error {
	return setSingleBullet(taskID, section, "- none")
}

func setSingleBullet(taskID, section, bulletLine string) error {
	path := filepath.Join(getTasksDir(), taskID, "v2", "root.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	out := []string{}
	inSection := false
	inserted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "### "+section+":" {
			inSection = true
			inserted = true
			out = append(out, line)
			out = append(out, bulletLine)
			continue
		}
		if inSection {
			if strings.HasPrefix(trimmed, "### ") {
				inSection = false
				out = append(out, line)
			}
			continue
		}
		out = append(out, line)
	}
	if !inserted {
		out = append(out, "### "+section+":")
		out = append(out, bulletLine)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0644)
}

func syncTaskDoneToIssue(rootID, prURL string) error {
	issuePath := filepath.Join(getIssuesDir(), rootID+".md")
	raw, err := os.ReadFile(issuePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	now := time.Now().UTC().Format(time.RFC3339)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "- status:") {
			lines[i] = "- status: done"
		}
	}
	comment := "- [sent " + now + "] task " + rootID + " resolved via task plugin"
	if prURL != "" {
		comment += " (pr: " + prURL + ")"
	}
	lines = addCommentToSection(lines, "comments-outbound", comment)
	return os.WriteFile(issuePath, []byte(strings.Join(lines, "\n")), 0644)
}

func addCommentToSection(lines []string, section, comment string) []string {
	out := []string{}
	inSection := false
	inserted := false
	for _, line := range lines {
		out = append(out, line)
		trimmed := strings.TrimSpace(line)
		if trimmed == "### "+section+":" {
			inSection = true
			continue
		}
		if inSection {
			if strings.HasPrefix(trimmed, "### ") {
				if !inserted {
					out = append(out[:len(out)-1], comment, line)
					inserted = true
				}
				inSection = false
				continue
			}
			if strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, "TODO: add a bullet comment") {
				out[len(out)-1] = comment
				inserted = true
			}
		}
	}
	if !inserted {
		out = append(out, "### "+section+":")
		out = append(out, comment)
	}
	return out
}

func copyDir(src string, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
