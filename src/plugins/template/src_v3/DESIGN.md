# Template Plugin v3 Design (Bottom-Up)

This document rewrites the v3 design from the ground up, mirroring every capability described in `src/plugins/template/README.md` while simplifying the new implementation.

## 0. Entry Point: CLI Usage (Ground Truth)
All v3 workflows are invoked via `./dialtone.sh` and remain consistent with existing template capabilities.

Examples (v3 target):
```bash
# Build UI assets
./dialtone.sh template build src_v3

# Run the test suite
./dialtone.sh template test src_v3

# Start dev server
./dialtone.sh template dev src_v3

# Scaffold a new version
./dialtone.sh template src --n 4
```

These commands stay aligned with the template plugin’s current build/dev/test flows and versioned `src_vN` structure.

## 1. Foundation: Versioned Source Layout (`src_vN`)
We keep the versioned directory pattern from `src/plugins/template/README.md` intact.

Expected structure for v3:
- `src/plugins/template/src_v3/cmd/` (Go entrypoint)
- `src/plugins/template/src_v3/ui/` (Vite UI)
- `src/plugins/template/src_v3/test/` (automation harness)
- `src/plugins/template/src_v3/install.go` (v3-local install workflow)
- `src/plugins/template/src_v3/build.go` (v3-local build workflow)
- `src/plugins/template/src_v3/DESIGN.md`
- `src/plugins/template/src_v3/TEST_PLAN.md`

This preserves:
- Side-by-side versions.
- Safe experimentation without breaking older versions.
- Comparable automation runs across versions.

## 2. Shared Library Layer (v3 uses new libs only)
We keep the same architectural intent as the original template README, but use new libraries instead of modifying old ones.

### UI Library (`src/plugins/ui`)
Replaces `src/libs/ui` in v3.

Core capabilities retained:
- `SectionManager`-style lifecycle handling.
- Global menu controls.
- Visibility and header/menu toggles.
- Shared styles and DOM contract.
- Lifecycle logging that is consumed by tests.

Behavioral requirements for v3:
- No transitions or fades between sections.
- Section swaps are immediate to speed automation.
- Logs are deterministic and structured for test_v2 parsing.

### Test Library (`src/libs/test_v2`)
Replaces `src/libs/dialtest` in v3.

Core capabilities retained:
- Browser lifecycle management.
- Preflight checks for Go/UI.
- Consolidated logging.
- Screenshot capture per step.
- Lifecycle/invariant verification.

Naming changes:
- `smoke` becomes `test` everywhere.
- `test.log`, `TEST.md`, `test_step_N.png`.

## 3. UI Layer: Section Model (Template Capabilities Preserved)
The v3 UI implements the same scope of template sections, but with simpler code and faster swaps.

### Required Section Types (UI Templates)
Each section must have a dedicated UI template in `src_v3/ui` using the shared naming rule:

- `<plugin-name>-<subname>-<underlay-type>`

Template `src_v3` section ids:

- `template-hero-stage` (hero stage underlay)
- `template-docs-docs` (docs underlay)
- `template-meta-table` (table underlay)
- `template-three-stage` (three scene stage underlay)
- `template-log-xterm` (log xterm underlay)
- `template-demo-video` (video underlay)

Overlay terminology follows the shared `ui_v2` section model:

- overlays: `menu`, `mode-form`, `legend`, `chatlog` (optional), `status-bar` (optional)
- underlays: `stage`, `table`, `docs`, `xterm`, `video`

### Shared Section Behaviors
- Each section can optionally show/hide:
  - header
  - menu
- Sections must be discoverable by ARIA labels for tests.
- Use ARIA labels and explicit tab ordering wherever possible to keep the UI accessible and testable.
- All click testing and element validation in tests will use `aria-label` selectors; tests should locate elements exclusively via these labels.
- Sections are lazy-loaded on first navigation and should pause any animation/render loops when unfocused.
- No transitions or animations between sections; navigation should flip instantly.
- Mouse wheel up/down should move directly to the previous/next section.
- Lifecycle logs must be emitted for each transition:
  - `LOADING`, `LOADED`, `START`, `RESUME`, `PAUSE`, `NAVIGATING TO`, `NAVIGATE TO`, `NAVIGATE AWAY`

### Section State Rules + Logging
Each section follows a strict state machine. Every transition must emit a log line.

States:
- `UNLOADED` (not yet fetched)
- `LOADING`
- `LOADED`
- `STARTED`
- `RESUMED`
- `PAUSED`

Rules:
- `UNLOADED -> LOADING -> LOADED -> STARTED` happens once per section.
- `RESUMED` and `PAUSED` can toggle multiple times after `STARTED`.
- `RESUMED` is only valid if the section is visible.
- `PAUSED` must run when the section is no longer visible.
- `NAVIGATING TO` must be logged before any visible change.
- `NAVIGATE TO` must be logged after the section becomes active.
- `NAVIGATE AWAY` must be logged before the previous section is fully hidden.
- The previous section must `PAUSE` immediately after `NAVIGATE AWAY`.

Required logs:
- `[SectionManager] 📦 LOADING #<id>`
- `[SectionManager] ✅ LOADED #<id>`
- `[SectionManager] ✨ START #<id>`
- `[SectionManager] 🚀 RESUME #<id>`
- `[SectionManager] 💤 PAUSE #<id>`
- `[SectionManager] 🧭 NAVIGATING TO #<id>`
- `[SectionManager] 🧭 NAVIGATE TO #<id>`
- `[SectionManager] 🧭 NAVIGATE AWAY #<id>`

Example timeline (first visit):
1. `[SectionManager] 🧭 NAVIGATING TO #docs`
2. `[SectionManager] 📦 LOADING #docs`
3. `[SectionManager] ✅ LOADED #docs`
4. `[SectionManager] ✨ START #docs`
5. `[SectionManager] 🧭 NAVIGATE TO #docs`
6. `[SectionManager] 🚀 RESUME #docs`

Example timeline (return visit):
1. `[SectionManager] 🧭 NAVIGATING TO #docs`
2. `[SectionManager] 🧭 NAVIGATE AWAY #hero`
3. `[SectionManager] 🧭 NAVIGATE TO #docs`
4. `[SectionManager] 🚀 RESUME #docs`
5. `[SectionManager] 💤 PAUSE #hero`

### Menu + Modal
- Menu includes a modal selector for section switching.
- Modal interactions are tested via Chromedp (click + keyboard).

## 4. Runtime Logging & Invariants (Parity with README)
We keep the same invariant expectations as `src/libs/ui`:
- Only one active section at a time.
- Active section must be visible.
- Resume must occur after load.

Violations are surfaced as `console.error` and cause test failure in v3.

## 5. Test Layer: Bottom-Up Test Flow
v3 test execution is strictly bottom-up and deterministic.

Order:
1. Preflight checks
2. Server startup
3. Browser connect
4. Section-by-section validation
5. Lifecycle/invariant summary
6. Report generation

### Dev Server Coexistence
- The UI dev server can run while tests are executing so a developer can watch changes live.
- Each test run should trigger a UI rebuild or refresh so the dev server reflects the latest code.

### Dev Server (Debug + Chromedp Attach)
v3 adds a dedicated dev server experience that is compatible with testing:

CLI example:
```bash
./dialtone.sh template dev src_v3
```

Requirements:
- Dev server runs in debug mode.
- Chromedp attaches to the dev server browser session for live inspection.
- All browser console logs stream to stdout and to `dev.log` in `src/plugins/template/src_v3/`.
- Dev server remains running while tests execute, and test runs force a UI refresh/rebuild so visible UI is always current.

### Per-Section Test Loop
Each UI section runs through the same loop. Every step writes to both `test.log` and `TEST.md`, and each section produces a screenshot artifact.

Loop (per section):
1. `test.log`: append step start (timestamp, section id, intent)
2. `TEST.md`: append step header (section id, intent)
3. Navigate to section (fast swap, no animation)
4. Capture console logs and errors
5. Capture section-specific assertions
6. Screenshot: `test_step_<N>.png`
7. `test.log`: append results (duration, errors, performance)
8. `TEST.md`: append summary (logs, errors, metrics, screenshot ref)

### Preflight (Explicit Commands)
- Go: `fmt`, `vet`, `build`
- UI: `lint`, `format`, `build`

### Report (`TEST.md`)
The report is richer than `SMOKE.md`:
- Preflight results (exit code + duration)
- Process timeline (server, browser, shutdown)
- Per-step console logs
- Errors/warnings grouped by step
- Performance per section
- Screenshot index

## 6. Implementation Notes
- v3 will be created by copying v2, then simplifying:
  - Replace `dialtone-ui` import with `ui_v2` entrypoint.
  - Replace `smoke` with `test` harness.
  - Strip UI transitions and fades.
- All v3 changes are local to `src_v3` and the new libraries.
- `src_v3` owns its own setup/build orchestration files:
  - `install.go` installs only the dependencies needed for `src_v3`.
  - `build.go` executes only the build steps needed for `src_v3`.

## 7. Decisions (Confirmed)
- `ui_v2` will use a new base stylesheet that targets common HTML elements (`section`, `header`, `button`, etc.) so components “just work” without extra classes.
- Three.js will be updated (not pinned to the v2 version).
- The section modal will be a standalone overlay (independent of the header).
- Sections must support hiding the header, the menu, or both.
