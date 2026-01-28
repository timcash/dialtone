package cli

type TestCondition struct {
	Condition string `json:"condition"`
}

type Subtask struct {
	Name           string          `json:"name"`
	Tags           []string        `json:"tags,omitempty"`
	Dependencies   []string        `json:"dependencies,omitempty"`
	Description    string          `json:"description"`
	TestConditions []TestCondition `json:"test_conditions"`
	AgentNotes     string          `json:"agent_notes,omitempty"`
	PassTimestamp  string          `json:"pass_timestamp,omitempty"`
	FailTimestamp  string          `json:"fail_timestamp,omitempty"`
	Status         string          `json:"status"` // todo, progress, done, failed, skipped
}

type Ticket struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Tags        []string  `json:"tags,omitempty"`
	Description string    `json:"description"`
	Subtasks    []Subtask `json:"subtasks"`
}
