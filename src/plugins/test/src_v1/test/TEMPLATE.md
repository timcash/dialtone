# test/src_v1 Template Example

This folder includes a copyable example for plugin authors (and LLM agents):

- `02_example_plugin_template/main.go`

Pattern:
1. Import `dialtone/dev/plugins/test/src_v1/go`.
2. Define `[]testv1.Step`.
3. Use `ctx.Logf(...)` and `ctx.Errorf(...)` in each step.
4. Run `testv1.RunSuite(testv1.SuiteOptions{ ... })` with `NATSURL` and `NATSSubject`.
5. Verify output with a listener (`logs.ListenToFile`) in your plugin tests.
