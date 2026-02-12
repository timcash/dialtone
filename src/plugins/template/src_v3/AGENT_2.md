# src_v3 Agent Handoff (AGENT_2)

## Legacy Agent Docs
- `src/plugins/template/src_v3/DESIGN.md` (older design notes from previous agents)
- `src/plugins/template/src_v3/AGENT.md` (older implementation workflow notes)

## Current Status
- Test runner is regular Go code (not `testing` package): `src/plugins/template/src_v3/test/main.go`.
- Run tests with:
  - `./dialtone.sh template test src_v3`
- Steps `01` through `18` currently pass.

Implemented/passing steps:
1. Go Format
2. Go Vet
3. Go Build
4. UI Lint
5. UI Format
6. UI Build
7. Go Run
8. UI Run
9. Expected Errors (Proof of Life)
10. Dev Server Running (latest UI)
11. Hero Section Validation
12. Docs Section Validation
13. Table Section Validation
14. Three Section Validation
15. Xterm Section Validation
16. Video Section Validation
17. Lifecycle / Invariants
18. Cleanup Verification

## Non-Negotiable Command Rules
- Do not run `go`, `bun`, `vite`, `tsc`, or `gofmt` directly.
- Use `./dialtone.sh` for all Go/Bun/Vite work.
- Prefer `./dialtone.sh template <cmd> src_v3` when available.

## Architecture Notes
- `src/libs/test_v2` now wraps `src/libs/dialtest` browser session patterns.
- Shared helpers in `src/libs/test_v2`:
  - `test.go` for browser session abstraction (`StartBrowser`, `Run`, `CaptureScreenshot`, log capture)
  - `browser_actions.go` for `NavigateToSection`, `WaitForAriaLabel`, `AssertElementHidden`
  - `ports.go` for `WaitForPort`, `PickFreePort`
- Keep adding shared behavior in `test_v2` instead of duplicating per-step logic.

Inspiration guidance:
- Use `src/libs/ui/` and `src/libs/dialtest/` for inspiration on required capabilities and patterns.
- Do not copy legacy code exactly.
- Re-abstract behavior into `src/libs/ui_v2/` and `src/libs/test_v2/` with cleaner APIs.
- Use `src/plugins/template/src_v3/` as the proving ground to discover what belongs in `ui_v2`/`test_v2`.

## Lessons Learned
1. Serve path bug:
- `cmd/main.go` initially served wrong path when launched from repo root.
- Fix was required to resolve `src/plugins/template/src_v3/ui/dist` when cwd is repo root.

2. Asset path bug:
- `filepath.Join(uiPath, r.URL.Path)` breaks if URL path starts with `/`.
- Must strip leading slash before join.

3. Port cleanup side effects:
- `browser.CleanupPort(8080)` can print noisy "killed" logs from subprocess trees.
- Tests still pass, but future runner polish should capture/quiet expected cleanup noise.

4. Avoid hidden stage coupling:
- Build originally called install implicitly.
- This was removed so install/lint/format/build are explicit steps in `src_v3/test`.

5. Keep tests incremental:
- Add one step only after previous sequence is green.
- Do not add future section tests until current one is stable.

## What Next Agent Should Do (In Order)
Use `src/plugins/template/src_v3/TEST_EXAMPLE.md` ordering and continue one test at a time.

Hard rule:
- Build each test in its own file (`01.go`, `02.go`, ...), one step at a time.
- Do not batch multiple new tests in one change.
- Only move to the next numbered file after the full suite is green with the new step.

Immediate next targets:
- Expand lifecycle assertions from token-level checks to per-section event ordering checks.
- Move repeated serve/browser startup teardown logic from `src_v3/test` into `src/libs/test_v2` runner helpers.
- Add explicit assertions for header/menu visibility toggles per section configuration.

For each new step:
1. Add minimal UI and ARIA labels needed for only that step.
2. Add one numbered step file in `src/plugins/template/src_v3/test/`.
3. Register that step in `test/main.go`.
4. Run `./dialtone.sh template test src_v3`.
5. Confirm pass and artifacts (`test_step_N.png`, logs/report updates if implemented).

## UI Scope Still Missing
Sections implemented in UI:
- `table`
- `three`
- `xterm`
- `video`

Keep behavior aligned with DESIGN.md:
- no transitions
- ARIA-first selectors
- deterministic logs
- fast section swaps

## Suggested Near-Term Refactor
- Move test step bookkeeping/report writing into `src/libs/test_v2` (closer to `dialtest/smoke.go` runner model) so `src_v3/test/main.go` is mostly scenario definitions.
- Consolidate shared test actions with `test_v2` helpers (`NavigateToSection`, `WaitForAriaLabel`, `ClickAriaLabel`, `AssertAriaLabelTextContains`, `WaitForAriaLabelAttrEquals`).
