package dialtest

import (
	"fmt"
	"strings"
)

type TestCase struct {
	Name       string
	TicketName string
	Tags       []string
	Fn         func() error
}

var testRegistry []TestCase

func RegisterTicket(ticketName string) {
	// Placeholder if needed for future logic
}

func AddSubtaskTest(name string, fn func() error, tags []string) {
	testRegistry = append(testRegistry, TestCase{
		Name: name,
		Fn:   fn,
		Tags: tags,
	})
}

func GetRegistry() []TestCase {
	return testRegistry
}

func GetSubtaskTest(name string) (func() error, bool) {
	for _, t := range testRegistry {
		if t.Name == name {
			return t.Fn, true
		}
	}
	return nil, false
}

func RunSubtask(name string) error {
	fn, ok := GetSubtaskTest(name)
	if !ok {
		return fmt.Errorf("subtask test not found: %s", name)
	}
	return fn()
}
