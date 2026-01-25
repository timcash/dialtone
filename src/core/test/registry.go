package test

type TestCase struct {
	Name       string
	TicketName string
	Tags       []string
	Fn         func() error
}

var testRegistry []TestCase

// Register adds a test to the registry
func Register(name string, ticketName string, tags []string, fn func() error) {
	testRegistry = append(testRegistry, TestCase{
		Name:       name,
		TicketName: ticketName,
		Tags:       tags,
		Fn:         fn,
	})
}

// GetRegistry returns the current test registry
func GetRegistry() []TestCase {
	return testRegistry
}

// RunTicket executes all registered tests for a specific ticket name
func RunTicket(ticketName string) error {
	for _, t := range testRegistry {
		if t.TicketName == ticketName {
			if err := t.Fn(); err != nil {
				return err
			}
		}
	}
	return nil
}

// HasAnyTag checks if a test has any of the specified tags
func HasAnyTag(t TestCase, tags []string) bool {
	for _, tag := range tags {
		for _, tTag := range t.Tags {
			if tag == tTag {
				return true
			}
		}
	}
	return false
}
