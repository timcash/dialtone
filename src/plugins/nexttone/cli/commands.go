package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const toneTestTemplate = `package test

import (
	"os"
	"testing"
)

func TestSubtone(t *testing.T) {
	if os.Getenv("NEXTTONE_SUBTONE") == "" {
		t.Skip("no subtone provided")
	}
}
`

func RunAdd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh nexttone add <tone-name> [--desc \"...\"]")
		return
	}
	name := args[0]
	if err := validateToneName(name); err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	desc := parseFlagValue(args[1:], "--desc")

	dbPath := filepath.Join(toneRootDir(), name, name+".duckdb")
	db, err := openNexttoneDBAt(dbPath)
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	defer db.Close()

	if err := addTone(db, name, desc); err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	if err := ensureToneFiles(name); err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	printActionPrompt("tone-add", []string{
		fmt.Sprintf("TONE: %s", name),
	}, []string{
		"export NEXTTONE_TONE=" + name,
		"./dialtone.sh nexttone",
	})
}

func RunSubtone(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ./dialtone.sh nexttone subtone <add|set> [args]")
		return
	}
	switch args[0] {
	case "add":
		RunSubtoneAdd(args[1:])
	case "set":
		RunSubtoneSet(args[1:])
	default:
		fmt.Printf("Unknown nexttone subtone subcommand: %s\n", args[0])
		fmt.Println("Usage: ./dialtone.sh nexttone subtone <add|set> [args]")
	}
}

func RunSubtoneAdd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: ./dialtone.sh nexttone subtone add <name> [--desc \"...\"]")
		return
	}
	name := args[0]
	flags := args[1:]
	desc := parseFlagValue(flags, "--desc")
	testConditions, _ := parseFlagValues(flags, "--test-condition")
	testCommand := parseFlagValue(flags, "--test-command")
	dependsOn := parseFlagValue(flags, "--depends-on")
	testOutput := parseFlagValue(flags, "--test-output")
	agentNotes := parseFlagValue(flags, "--agent-notes")

	db, err := openNexttoneDB()
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	defer db.Close()

	position, err := nextSubtonePosition(db)
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	if err := insertSubtone(db, subtone{
		Name:          name,
		Description:   desc,
		Position:      position,
		TestCondition: firstCondition(testConditions),
		TestConditions: testConditions,
		TestCommand:   testCommand,
		DependsOn:     dependsOn,
		TestOutput:    testOutput,
		AgentNotes:    agentNotes,
	}); err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	printActionPrompt("subtone-add", []string{
		fmt.Sprintf("SUBTONE: %s", name),
	}, []string{
		fmt.Sprintf("./dialtone.sh nexttone subtone set %s --desc \"...\"", name),
		"./dialtone.sh nexttone",
	})
}

func RunSubtoneSet(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: ./dialtone.sh nexttone subtone set <name> --<field> \"...\"")
		return
	}
	name := args[0]
	flags := args[1:]
	updates := parseSubtoneUpdates(flags)
	if updates.isEmpty() {
		fmt.Println("Usage: ./dialtone.sh nexttone subtone set <name> --<field> \"...\"")
		return
	}

	db, err := openNexttoneDB()
	if err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}
	defer db.Close()

	if err := updateSubtone(db, name, updates); err != nil {
		fmt.Printf("[error] %v\n", err)
		return
	}

	printActionPrompt("subtone-set", []string{
		fmt.Sprintf("SUBTONE: %s", updates.targetName(name)),
	}, []string{
		"./dialtone.sh nexttone",
	})
}

type subtoneUpdates struct {
	Name          string
	Description   string
	TestCondition string
	TestConditions []string
	TestCommand   string
	DependsOn     string
	TestOutput    string
	AgentNotes    string
}

func (u subtoneUpdates) isEmpty() bool {
	return u.Name == "" &&
		u.Description == "" &&
		u.TestCondition == "" &&
		len(u.TestConditions) == 0 &&
		u.TestCommand == "" &&
		u.DependsOn == "" &&
		u.TestOutput == "" &&
		u.AgentNotes == ""
}

func (u subtoneUpdates) targetName(current string) string {
	if u.Name != "" {
		return u.Name
	}
	return current
}

func parseSubtoneUpdates(args []string) subtoneUpdates {
	updates := subtoneUpdates{}
	testConditions, hasTestConditions := parseFlagValues(args, "--test-condition")
	if hasTestConditions {
		updates.TestConditions = testConditions
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--name="):
			updates.Name = strings.TrimPrefix(arg, "--name=")
		case arg == "--name" && i+1 < len(args):
			i++
			updates.Name = args[i]
		case strings.HasPrefix(arg, "--desc="):
			updates.Description = strings.TrimPrefix(arg, "--desc=")
		case arg == "--desc" && i+1 < len(args):
			i++
			updates.Description = args[i]
		case strings.HasPrefix(arg, "--test-condition="), arg == "--test-condition":
			// handled by parseFlagValues
		case strings.HasPrefix(arg, "--test-command="):
			updates.TestCommand = strings.TrimPrefix(arg, "--test-command=")
		case arg == "--test-command" && i+1 < len(args):
			i++
			updates.TestCommand = args[i]
		case strings.HasPrefix(arg, "--depends-on="):
			updates.DependsOn = strings.TrimPrefix(arg, "--depends-on=")
		case arg == "--depends-on" && i+1 < len(args):
			i++
			updates.DependsOn = args[i]
		case strings.HasPrefix(arg, "--test-output="):
			updates.TestOutput = strings.TrimPrefix(arg, "--test-output=")
		case arg == "--test-output" && i+1 < len(args):
			i++
			updates.TestOutput = args[i]
		case strings.HasPrefix(arg, "--agent-notes="):
			updates.AgentNotes = strings.TrimPrefix(arg, "--agent-notes=")
		case arg == "--agent-notes" && i+1 < len(args):
			i++
			updates.AgentNotes = args[i]
		}
	}
	return updates
}

func parseFlagValues(args []string, flag string) ([]string, bool) {
	values := []string{}
	found := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, flag+"=") {
			found = true
			values = append(values, strings.TrimPrefix(arg, flag+"="))
			continue
		}
		if arg == flag && i+1 < len(args) {
			found = true
			i++
			values = append(values, args[i])
		}
	}
	return values, found
}

func parseFlagValue(args []string, flag string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, flag+"=") {
			return strings.TrimPrefix(arg, flag+"=")
		}
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func firstCondition(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return conditions[0]
}

func addTone(db *sql.DB, name, description string) error {
	if name == "" {
		return fmt.Errorf("tone name is required")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_tones WHERE name = ?`, name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("tone already exists: %s", name)
	}
	_, err := db.Exec(
		`INSERT INTO nexttone_tones (name, description, created_at) VALUES (?, ?, ?)`,
		name,
		description,
		time.Now().Format(time.RFC3339),
	)
	return err
}

func ensureToneFiles(name string) error {
	if name == "" {
		return fmt.Errorf("tone name is required")
	}
	toneDir := filepath.Join(toneRootDir(), name, "test")
	if err := os.MkdirAll(toneDir, 0755); err != nil {
		return err
	}
	testPath := filepath.Join(toneDir, "test.go")
	if _, err := os.Stat(testPath); err == nil {
		return nil
	}
	return os.WriteFile(testPath, []byte(toneTestTemplate), 0644)
}

func validateToneName(name string) error {
	if name == "" {
		return fmt.Errorf("tone name is required")
	}
	if !regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+){2,4}$`).MatchString(name) {
		return fmt.Errorf("tone name must be 3 to 5 kebab-case words (example: \"nexttone-graph-demo\")")
	}
	return nil
}

func nextSubtonePosition(db *sql.DB) (int, error) {
	var maxPos sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(position) FROM nexttone_subtones`).Scan(&maxPos); err != nil {
		return 0, err
	}
	if !maxPos.Valid {
		return 1, nil
	}
	return int(maxPos.Int64) + 1, nil
}

func insertSubtone(db *sql.DB, st subtone) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_subtones WHERE name = ?`, st.Name).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("subtone already exists: %s", st.Name)
	}
	_, err := db.Exec(
		`INSERT INTO nexttone_subtones (name, position, description, test_condition, test_command, depends_on, test_output, agent_notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		st.Name,
		st.Position,
		st.Description,
		st.TestCondition,
		st.TestCommand,
		st.DependsOn,
		st.TestOutput,
		st.AgentNotes,
	)
	if err != nil {
		return err
	}
	return replaceSubtoneConditions(db, st.Name, st.TestConditions)
}

func updateSubtone(db *sql.DB, name string, updates subtoneUpdates) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_subtones WHERE name = ?`, name).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("unknown subtone: %s", name)
	}

	if updates.Name != "" && updates.Name != name {
		var renameCount int
		if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_subtones WHERE name = ?`, updates.Name).Scan(&renameCount); err != nil {
			return err
		}
		if renameCount > 0 {
			return fmt.Errorf("subtone already exists: %s", updates.Name)
		}
		if _, err := db.Exec(`UPDATE nexttone_subtones SET name = ? WHERE name = ?`, updates.Name, name); err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE nexttone_sessions SET current_subtone = ? WHERE current_subtone = ?`, updates.Name, name); err != nil {
			return err
		}
		name = updates.Name
	}

	setClauses := []string{}
	args := []any{}
	if updates.Description != "" {
		setClauses = append(setClauses, "description = ?")
		args = append(args, updates.Description)
	}
	if updates.TestCondition != "" || len(updates.TestConditions) > 0 {
		primary := updates.TestCondition
		if primary == "" && len(updates.TestConditions) > 0 {
			primary = updates.TestConditions[0]
		}
		setClauses = append(setClauses, "test_condition = ?")
		args = append(args, primary)
	}
	if updates.TestCommand != "" {
		setClauses = append(setClauses, "test_command = ?")
		args = append(args, updates.TestCommand)
	}
	if updates.DependsOn != "" {
		setClauses = append(setClauses, "depends_on = ?")
		args = append(args, updates.DependsOn)
	}
	if updates.TestOutput != "" {
		setClauses = append(setClauses, "test_output = ?")
		args = append(args, updates.TestOutput)
	}
	if updates.AgentNotes != "" {
		setClauses = append(setClauses, "agent_notes = ?")
		args = append(args, updates.AgentNotes)
	}
	if len(setClauses) > 0 {
		args = append(args, name)
		query := fmt.Sprintf("UPDATE nexttone_subtones SET %s WHERE name = ?", strings.Join(setClauses, ", "))
		if _, err := db.Exec(query, args...); err != nil {
			return err
		}
	}
	if len(updates.TestConditions) > 0 {
		if err := replaceSubtoneConditions(db, name, updates.TestConditions); err != nil {
			return err
		}
	}
	return nil
}

func replaceSubtoneConditions(db *sql.DB, name string, conditions []string) error {
	if len(conditions) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM nexttone_subtone_conditions WHERE subtone_name = ?`, name); err != nil {
		return err
	}
	for i, condition := range conditions {
		if strings.TrimSpace(condition) == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO nexttone_subtone_conditions (subtone_name, position, condition) VALUES (?, ?, ?)`,
			name,
			i + 1,
			condition,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func getSubtoneConditions(db *sql.DB, name string) ([]string, error) {
	conditions := []string{}
	rows, err := db.Query(
		`SELECT condition FROM nexttone_subtone_conditions WHERE subtone_name = ? ORDER BY position`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var condition string
		if err := rows.Scan(&condition); err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}
	return conditions, nil
}

func printActionPrompt(tag string, context []string, commands []string) {
	fmt.Printf("DIALTONE [%s]:\n", tag)
	for _, line := range context {
		fmt.Println(line)
	}
	fmt.Println("DIALTONE: What is the next step?")
	for _, cmd := range commands {
		fmt.Printf("  %s\n", cmd)
	}
}
