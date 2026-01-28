package cli

type TestCondition struct {
	Condition string `json:"condition"`
}

type Subtask struct {
	Name           string          `json:"name"`
	Tags           []string        `json:"tags"`
	Dependencies   []string        `json:"dependencies"`
	Description    string          `json:"description"`
	TestConditions []TestCondition `json:"test_conditions"`
	AgentNotes     string          `json:"agent_notes"`
	PassTimestamp  string          `json:"pass_timestamp"`
	FailTimestamp  string          `json:"fail_timestamp"`
	Status         string          `json:"status"` // todo, progress, done, failed
}

type Ticket struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Subtasks    []Subtask `json:"subtasks"`
	Tags        []string  `json:"tags"`
}
