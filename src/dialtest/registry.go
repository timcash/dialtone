package dialtest

import (
	"fmt"
	"sync"
)

// SubtaskTestFunc is the signature for a subtask test function.
type SubtaskTestFunc func() error

// SubtaskTest represents a registered subtask test.
type SubtaskTest struct {
	Name       string
	TicketName string
	Tags       []string
	Fn         SubtaskTestFunc
}

var (
	registry = make(map[string]map[string]SubtaskTest)
	mu       sync.RWMutex
	currentTicket string
)

// RegisterTicket sets the current ticket context for subsequent AddSubtaskTest calls.
func RegisterTicket(ticketID string) {
	mu.Lock()
	defer mu.Unlock()
	currentTicket = ticketID
	if _, ok := registry[ticketID]; !ok {
		registry[ticketID] = make(map[string]SubtaskTest)
	}
}

// AddSubtaskTest registers a test function for a specific subtask within the current ticket.
func AddSubtaskTest(name string, fn SubtaskTestFunc, tags []string) {
	mu.Lock()
	defer mu.Unlock()
	if currentTicket == "" {
		panic("dialtest: RegisterTicket must be called before AddSubtaskTest")
	}
	registry[currentTicket][name] = SubtaskTest{
		Name:       name,
		TicketName: currentTicket,
		Fn:         fn,
		Tags:       tags,
	}
}

// GetTest returns a registered subtask test for a given ticket and subtask name.
func GetTest(ticketID, subtaskName string) (SubtaskTest, error) {
	mu.RLock()
	defer mu.RUnlock()
	ticketTests, ok := registry[ticketID]
	if !ok {
		return SubtaskTest{}, fmt.Errorf("no tests registered for ticket: %s", ticketID)
	}
	test, ok := ticketTests[subtaskName]
	if !ok {
		return SubtaskTest{}, fmt.Errorf("no test registered for subtask: %s", subtaskName)
	}
	return test, nil
}

// GetAllTests returns all registered tests for a ticket.
func GetAllTests(ticketID string) []SubtaskTest {
	mu.RLock()
	defer mu.RUnlock()
	var tests []SubtaskTest
	if ticketTests, ok := registry[ticketID]; ok {
		for _, t := range ticketTests {
			tests = append(tests, t)
		}
	}
	return tests
}
