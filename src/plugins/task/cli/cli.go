package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

)

// Run processes the "task" subcommand
func Run(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}

	command := args[0]
	rest := args[1:]

	switch command {
	case "create":
		runCreate(rest)
	case "validate":
		runValidate(rest)
	case "help":
		printUsage()
	default:
		fmt.Printf("Unknown task command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: ./dialtone.sh task <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  create <task-name>   Create a new task in src/plugins/task/database/<name>/v1")
	fmt.Println("  validate <file-path> Validate a task markdown file format")
}

func runCreate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh task create <task-name>")
		return
	}
	taskName := args[0]
	
	// Directory: src/plugins/task/database/<task-name>/v1
	// We assume we run from project root, so "src" is available.
	baseDir := filepath.Join("src", "plugins", "task", "database", taskName, "v1")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Printf("Error creating directory %s: %v\n", baseDir, err)
		return
	}

	filename := filepath.Join(baseDir, taskName+".md")
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

	fmt.Printf("Created new task: %s\n", filename)
}

func runValidate(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh task validate <file-path>")
		return
	}
	path := args[0]

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
	
	// Regex for standard sections: "### section-name:"
	sectionRegex := regexp.MustCompile(`^### ([a-z0-9-]+):$`)
	// Regex for list items: "- something"
	listRegex := regexp.MustCompile(`^- .*`)
	// Regex for comments/placeholders: "# .*"
	commentRegex := regexp.MustCompile(`^# .*`)
	// Regex for signatures: "- USER> timestamp :: key"
	// We loosen it slightly: "- [A-Z0-9-]+> .* :: .*"
	sigRegex := regexp.MustCompile(`^- [A-Z0-9:-]+> .+ :: .+`)

	// Flag to track if we found the main header
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

		if trimmed == "" {
			continue
		}

		// Check for section headers
		if matches := sectionRegex.FindStringSubmatch(line); len(matches) > 0 {
			currentSection = matches[1]
			continue
		}

		// Validation rules per section content
		// If it's not a section header, it must be a list item, a comment, or text (only in description?)
		// User rule: "bullet point or key value" implies strictly structured data or comments.
		// Description might be multiline text, but user said "simple format". Let's assume description allows text.
		
		if currentSection == "description" {
			// Description accepts free text.
			continue
		}

		// Special handling for signatures in reviewed/tested
		if currentSection == "reviewed" || currentSection == "tested" {
			if commentRegex.MatchString(line) {
				continue // Placeholder comment is fine
			}
			if listRegex.MatchString(line) {
				if !sigRegex.MatchString(line) {
					errors = append(errors, fmt.Sprintf("Line %d: Invalid signature format in '%s'. Expected '- ACTOR> timestamp :: key'", lineNum, currentSection))
				}
				continue
			}
			// If not empty, comment, or list, it's invalid
			errors = append(errors, fmt.Sprintf("Line %d: Invalid content in '%s'. Expected list item or comment.", lineNum, currentSection))
			continue
		}

		// Default rule for all other sections: must be list item or comment
		isList := listRegex.MatchString(line)
		isComment := commentRegex.MatchString(line)

		if !isList && !isComment {
			errors = append(errors, fmt.Sprintf("Line %d: Invalid line in section '%s'. Must be bullet point ('- ') or comment ('# '). Found: '%s'", lineNum, currentSection, line))
		}
	}

	if !foundHeader {
		errors = append(errors, "Missing Task Name header on line 1")
	}

	if len(errors) > 0 {
		fmt.Println("Validation FAILED:")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
		os.Exit(1)
	}

	fmt.Printf("Validation PASSED: %s\n", path)
}
