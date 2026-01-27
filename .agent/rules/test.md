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