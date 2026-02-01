package main

import (
	"database/sql"
	"fmt"
)

func TestSubtoneSetUpdatesFields() error {
	logLine("step", "Update subtone fields")
	if err := resetToneDB(); err != nil {
		return err
	}
	subtoneName := "subtone-alpha"
	runCmd("./dialtone.sh", "nexttone", "subtone", "add", subtoneName, "--desc", "demo subtone", "--test-condition", "condition one")
	runCmd("./dialtone.sh", "nexttone", "subtone", "set", subtoneName,
		"--desc", "updated desc",
		"--test-condition", "condition two",
		"--test-command", "./dialtone.sh plugin test nexttone",
		"--depends-on", "subtone-beta",
		"--test-output", "status 200",
		"--agent-notes", "checked",
	)

	db, err := openTestDB()
	if err != nil {
		return err
	}

	var description string
	var testCommand string
	var dependsOn string
	var testOutput string
	var agentNotes string
	if err := db.QueryRow(
		`SELECT description, test_command, depends_on, test_output, agent_notes FROM nexttone_subtones WHERE name = ?`,
		subtoneName,
	).Scan(&description, &testCommand, &dependsOn, &testOutput, &agentNotes); err != nil {
		return err
	}
	if description != "updated desc" {
		return fmt.Errorf("expected description to update")
	}
	if testCommand != "./dialtone.sh plugin test nexttone" {
		return fmt.Errorf("expected test_command to update")
	}
	if dependsOn != "subtone-beta" {
		return fmt.Errorf("expected depends_on to update")
	}
	if testOutput != "status 200" {
		return fmt.Errorf("expected test_output to update")
	}
	if agentNotes != "checked" {
		return fmt.Errorf("expected agent_notes to update")
	}

	var conditionCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM nexttone_subtone_conditions WHERE subtone_name = ?`,
		subtoneName,
	).Scan(&conditionCount); err != nil {
		return err
	}
	if conditionCount != 1 {
		return fmt.Errorf("expected 1 test condition, got %d", conditionCount)
	}
	return nil
}
