# `ui`

`src/libs/ui` is the shared front-end runtime utility library for Dialtone plugin UIs.

## Scope

- Section lifecycle orchestration (`SectionManager`)
- Shared menu controls (`Menu`)
- Common telemetry helpers (`FpsCounter`, `GpuTimer`)
- Typing utility (`startTyping`)
- Shared base styles (`style.css`)
- App bootstrap helper (`setupApp`)

## Independence Contract

This library is intentionally plugin-agnostic:

- No imports from `src/plugins/...`
- No dependencies on `src/plugins/www/app`
- No plugin-specific route names or component references

It expects only a generic DOM structure (header/menu/sections) and plugin-provided section registrations.

## Lifecycle Logs

`SectionManager` emits consistent logs for section transitions:

- `LOADING`
- `LOADED`
- `START`
- `RESUME`
- `PAUSE`
- `NAVIGATING TO`
- `NAVIGATE TO`
- `NAVIGATE AWAY`

These logs are consumed by smoke tests and by runtime invariant checks.

## Live Invariant Checks

`setupApp(...)` enables runtime invariant validation while the UI is running. It reports `console.error` when state invariants break, including:

- Hash/active section mismatch
- More than one visible section
- More than one resumed section
- Active section not visible
- Section resumed before load
- Section resumed while not visible

These checks run continuously, not only in smoke tests.

## Known Problems

- `src/libs/ui` currently builds through plugin-local Vite configs (`src/plugins/*/src_vN/ui`), so build hangs in a plugin UI pipeline can block smoke before section assertions execute.
- In that case, lifecycle/invariant logging is still correct at runtime once the UI is served, but smoke coverage for section transitions may not run until the build issue is resolved.
