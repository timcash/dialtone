# Test Plugin

The `test` plugin provides a centralized test runner for `dialtone`. It supports running ticket-specific tests, plugin-specific tests, and global tag-based test filtering.

## Commands

### `./dialtone.sh test ticket <ticket-name>`
Runs all tests located in `tickets/<ticket-name>/test/`. It uses `go test` internally.
- Use `--subtask <subtask-name>` to run a specific subtask test.
- Use `--list` to see what would run without executing.

### `./dialtone.sh test plugin <plugin-name>`
Runs tests for a specific plugin. Plugins must register a `RunAll() error` function in their `test/` package and connect it to `src/plugins/test/cli/test.go`.

### `./dialtone.sh test tags [tag1 tag2 ...]`
Runs all tests registered in the global registry that match any of the specified tags.
- Example: `./dialtone.sh test tags metadata camera-filters`

### `./dialtone.sh test`
Runs all registered tests across all plugins and tickets.

### `./dialtone.sh test --list`
Lists all tests that would be executed by the current command without actually running them.

## Tagging System

To support tagging, tests must be explicitly registered with the core test registry in `src/core/test/registry.go`.

### Registering a Test

In your test file (e.g., `tickets/my-ticket/test/test.go`), use the `init()` function to register your tests:

```go
import (
    "dialtone/cli/src/core/test"
)

func init() {
    test.Register("my-metadata-test", []string{"metadata", "core"}, RunMyTest)
}

func RunMyTest() error {
    // Test logic here
    return nil
}
```

### How Tag Filtering Works

When you run `./dialtone.sh test tags <tags>`, the runner:
1. Iterates through the global registry.
2. Checks if the test has at least one of the tags requested.
3. Executes the registered function for each matching test.
