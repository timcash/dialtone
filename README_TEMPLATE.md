# Plugin README Template

Use this template for every plugin README at:

- `src/plugins/<plugin>/README.md`

## Test Summary
- Show status: `PASS` or `FAIL`.
- If `FAIL`, include the first error and stack trace.
- Keep this section near the top so contributors can quickly see plugin health.

## Section Screenshot Grid
- Provide a compact screenshot grid for each UI section.
- Label each screenshot with section id or section name.
- Use this section for fast visual verification after test runs.

## Shell help
- Include shell usage examples in a code block.
- Include at least:
- `dev`
- `build`
- `test`
- plugin-specific commands

Example:
```bash
./dialtone.sh <plugin> dev <src_vN>
./dialtone.sh <plugin> build <src_vN>
./dialtone.sh <plugin> test <src_vN>
```

## Workflow
- Explain how to develop and validate changes for this plugin.
- Include:
- local dev flow
- test flow
- how to read failures
- how to refresh screenshots/reports
- checks for shared libraries when relevant (`src/libs/ui_v2`, `src/libs/test_v2`, future `src/libs/logs_v1`)
- cross-plugin validation when shared patterns must stay aligned
