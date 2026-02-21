# test/src_v1 Template Example

This folder includes a copyable example for plugin authors (and LLM agents):

- `02_example_plugin_template/suite.go`
- `cmd/main.go`

Pattern:
1. Import `dialtone/dev/plugins/test/src_v1/go`.
2. In each `NN_name` folder, export `Register(r *testv1.Registry)`.
3. Use only `ctx` methods in steps (`ctx.Infof/Warnf/Errorf`, `ctx.WaitForStepMessage...`).
4. In `test/cmd/main.go`, import each folder and register all steps.
5. Run one `r.Run(testv1.SuiteOptions{ ... })` call for single-process execution.
