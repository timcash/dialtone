# Test Plugin

The `test` plugin provides a centralized test runner for `dialtone`. It supports running ticket-specific tests, plugin-specific tests, and global tag-based test filtering.

## Commands

```bash
# Runs all tests for a specific plugin.
# Use `--subtask <subtask-name>` for a specific test, or `--list` to dry-run.
./dialtone.sh plugin test <plugin-name>

# Runs tests for a specific plugin. Plugins must register a `RunAll() error`
# function in their test/ package and connect it to src/plugins/test/cli/test.go.
./dialtone.sh plugin test <plugin-name>

# Runs tests matching any of the specified tags.
# Example: ./dialtone.sh test tags metadata camera-filters
./dialtone.sh test tags <tag-name>...

# Runs all registered tests across all plugins and tickets.
./dialtone.sh test

# Lists all tests that would be executed by the current command.
./dialtone.sh test tags [<tag-name>...] --list
```



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
