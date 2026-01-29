# Test Plugin

The `test` plugin provides a centralized test runner for `dialtone`. It supports running plugin-specific tests and global tag-based test filtering.

## Commands

```shell
# Run all tests for a specific plugin
./dialtone.sh plugin test <plugin-name>

# Run a specific subtask test
./dialtone.sh plugin test <plugin-name> --subtask <subtask-name>

# List available tests for a plugin
./dialtone.sh plugin test <plugin-name> --list

# Run tests matching any of the specified tags
./dialtone.sh plugin test tags <tag-name>...

# Run all registered tests
./dialtone.sh plugin test

# List all tests that would be executed
./dialtone.sh plugin test --list
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

When you run `./dialtone.sh plugin test tags <tags>`, the runner:
1. Iterates through the global registry.
2. Checks if the test has at least one of the tags requested.
3. Executes the registered function for each matching test.
