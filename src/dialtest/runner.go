package dialtest

import (
	"fmt"
)

func RunSubtaskTest(name string) error {
	fn, ok := GetSubtaskTest(name)
	if !ok {
		return fmt.Errorf("subtask test not found in registry: %s", name)
	}
	return fn()
}
