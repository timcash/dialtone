package dialtest

import (
	"fmt"
)

// RunSubtask executes a registered subtask test.
func RunSubtask(ticketID, subtaskName string) error {
	test, err := GetTest(ticketID, subtaskName)
	if err != nil {
		return err
	}

	fmt.Printf("[dialtest] Running test for subtask: %s\n", subtaskName)
	if err := test.Fn(); err != nil {
		fmt.Printf("[dialtest] FAIL: %s - %v\n", subtaskName, err)
		return err
	}

	fmt.Printf("[dialtest] PASS: %s\n", subtaskName)
	return nil
}
