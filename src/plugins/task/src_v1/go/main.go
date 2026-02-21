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
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "create":
		runCreate(args)
	case "validate":
		runValidate(args)
	case "archive":
		runArchive(args)
	case "sign":
		runSign(args)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown task command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: task <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  create <task-name>   Create a new task in database/<name>/v1")
	fmt.Println("  validate <task-name> Validate a task markdown file")
	fmt.Println("  archive <task-name>  Promote v2 to v1 and prepare for next cycle")
	fmt.Println("  sign <task-name> --role <role>  Sign a task in v2")
}

func runCreate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: task create <task-name>")
		return
	}
	taskName := args[0]

	baseDir := filepath.Join("src", "plugins", "task", "database", taskName, "v1")
	v2Dir := filepath.Join("src", "plugins", "task", "database", taskName, "v2")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("Error creating directory %s: %v\n", baseDir, err)
		return
	}
	if err := os.MkdirAll(v2Dir, 0755); err != nil {
		fmt.Printf("Error creating directory %s: %v\n", v2Dir, err)
		return
	}

	filename := filepath.Join(baseDir, taskName+".md")
	v2filename := filepath.Join(v2Dir, taskName+".md")

	if _, err := os.Stat(filename); err == nil {
		fmt.Printf("Error: Task file already exists at %s\n", filename)
		return
	}

	content := fmt.Sprintf(`# %s
### description:
TODO: Add description here.
### tags:
- todo
### task-dependencies:
# None
### documentation:
# None
### test-condition-1:
- TODO: Add test condition
### test-command:
- TODO: Add test command
### reviewed:
# [Waiting for signatures]
### tested:
# [Waiting for tests]
### last-error-types:
# None
### last-error-times:
# None
### log-stream-command:
- TODO: Add log command
### last-error-loglines:
# None
### notes:
`, taskName)

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		fmt.Printf("Error writing file %s: %v\n", filename, err)
		return
	}
	if err := os.WriteFile(v2filename, []byte(content), 0644); err != nil {
		fmt.Printf("Error writing file %s: %v\n", v2filename, err)
		return
	}

	fmt.Printf("Created new task: %s and %s\n", filename, v2filename)
}

func runValidate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: task validate <task-name>")
		return
	}
	taskName := args[0]
	path := filepath.Join("src", "plugins", "task", "database", taskName, "v2", taskName+".md")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join("src", "plugins", "task", "database", taskName, "v1", taskName+".md")
	}

	if _, err := os.Stat(path); err != nil {
		fmt.Printf("Task %s not found in v1 or v2\n", taskName)
		return
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening file %s: %v\n", path, err)
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

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if lineNum == 1 {
			if strings.HasPrefix(line, "# ") {
				foundHeader = true
				continue
			} else {
				errors = append(errors, fmt.Sprintf("Line 1: Must be a header '# task-name' (found: '%s')", line))
			}
		}
		if trimmed == "" { continue }
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 0 {
			currentSection = matches[1]
			continue
		}
		if currentSection == "description" || currentSection == "notes" { continue }
		if currentSection == "reviewed" || currentSection == "tested" {
			if commentRegex.MatchString(line) { continue }
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
	if !foundHeader { errors = append(errors, "Missing Task Name header on line 1") }
	if len(errors) > 0 {
		fmt.Println("Validation FAILED:")
		for _, e := range errors { fmt.Printf("  - %s\n", e) }
		os.Exit(1)
	}
	fmt.Printf("Validation PASSED: %s\n", path)
}

func runArchive(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: task archive <task-name>")
		return
	}
	taskName := args[0]
	basePath := filepath.Join("src", "plugins", "task", "database", taskName)
	v1Dir := filepath.Join(basePath, "v1")
	v2Dir := filepath.Join(basePath, "v2")

	if _, err := os.Stat(v2Dir); err != nil {
		fmt.Printf("Error: v2 directory for task %s not found\n", taskName)
		return
	}

	fmt.Printf("Promoting %s/v2 to v1...\n", taskName)
	if err := os.RemoveAll(v1Dir); err != nil {
		fmt.Printf("Error removing v1: %v\n", err)
		return
	}

	if err := os.Rename(v2Dir, v1Dir); err != nil {
		fmt.Printf("Error renaming v2 to v1: %v\n", err)
		return
	}

	if err := copyDir(v1Dir, v2Dir); err != nil {
		fmt.Printf("Error copying v1 to v2: %v\n", err)
		return
	}

	fmt.Printf("Successfully archived task %s. v1 and v2 now match.\n", taskName)
}

func runSign(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: task sign <task-name> --role <role>")
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
		fmt.Println("Error: --role <role> is required")
		return
	}

	v2Path := filepath.Join("src", "plugins", "task", "database", taskName, "v2", taskName+".md")
	if _, err := os.Stat(v2Path); err != nil {
		fmt.Printf("Error: v2 task file not found at %s\n", v2Path)
		return
	}

	content, err := os.ReadFile(v2Path)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	lines := strings.Split(string(content), "\n")
	newLines := []string{}
	section := ""
	signed := false

	// Target section based on role
	targetSection := "reviewed"
	if strings.Contains(strings.ToLower(role), "test") {
		targetSection = "tested"
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	signature := fmt.Sprintf("- %s> %s :: sig-%d", strings.ToUpper(role), timestamp, time.Now().UnixNano())

	for _, line := range lines {
		newLines = append(newLines, line)
		if strings.HasPrefix(line, "### ") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "### "), ":")
		}
		if section == targetSection && !signed {
			// Add signature after the section header or after existing signatures
			// For simplicity, we'll just append it if the next line is a comment or empty
			// Actually, let's look for the next section header or end of file
		}
	}
	
	// Real implementation should be more precise.
	// Let's do a simple insertion.
	finalLines := []string{}
	sectionFound := false
	for _, line := range lines {
		finalLines = append(finalLines, line)
		if line == "### "+targetSection+":" {
			finalLines = append(finalLines, signature)
			signed = true
			sectionFound = true
		}
	}

	if !sectionFound {
		fmt.Printf("Error: section ### %s: not found in %s\n", targetSection, v2Path)
		return
	}

	if err := os.WriteFile(v2Path, []byte(strings.Join(finalLines, "\n")), 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("Successfully signed task %s as %s in v2\n", taskName, role)
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
