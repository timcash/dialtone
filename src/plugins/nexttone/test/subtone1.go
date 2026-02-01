package main

import (
	"database/sql"
	"fmt"
)

func TestSubtoneAddCreatesRow() error {
	logLine("step", "Add subtone")
	if err := resetToneDB(); err != nil {
		return err
	}
	subtoneName := "subtone-alpha"
	runCmd("./dialtone.sh", "nexttone", "subtone", "add", subtoneName, "--desc", "demo subtone", "--test-condition", "condition one")

	db, err := openTestDB()
	if err != nil {
		return err
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nexttone_subtones WHERE name = ?`, subtoneName).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("expected subtone %s to exist", subtoneName)
	}
	return nil
}
