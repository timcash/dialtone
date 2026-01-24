---
trigger: always_on
---

# Testing Rules
1. Do not use golang testing framework
2. Do not use any languages testing tools
3. Use basic code and assert statements instead
4. use the `dialtone.sh test` command to run all tests
5. import or somehow connect all tests to `src/plugins/test/cli/test.go`
6. improve `src/plugins/test/cli/test.go` when needed to allow filtering and searching for tests

## Procedure for Creating and Connecting New Tests

### 1. Create the Test Suite
- **Directory**: Place tests in a `test/` subdirectory within the relevant plugin (e.g., `src/plugins/myplugin/test/`).
- **File**: Create a Go file ending in `_suite.go` (e.g., `myplugin_suite.go`).
- **Package**: Use a separate package name like `myplugin_test` to avoid circular dependencies.
- **RunAll function**: Implement a public `RunAll() error` function that executes all tests in the file and returns the first error encountered, or `nil` if all pass.

### 2. Standard Logging & Assertions
- **Logger**: Always import and use `"dialtone/cli/src/core/logger"`.
- **Informations**: Use `logger.LogInfo("Your message")`.
- **Failures**: Return `fmt.Errorf("failure description: %v", err)` instead of using `t.Error` or `assert` tags.

### 3. Registering with the Test Runner
- **File**: Edit `src/plugins/test/cli/test.go`.
- **Import**: Import the new test suite package.
- **Global Run**: Call the `RunAll()` function from within the `runAllTests()` function.
- **Specific Command**: (Optional) Add a case to the `RunTest()` switch statement to allow running the suite individually (e.g., `./dialtone.sh test myplugin`).