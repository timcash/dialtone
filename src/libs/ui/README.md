# Log 2026-02-16 13:36:14 -0800

- Design-language update:
  - a UI is section-based
  - each section is modeled with overlays
  - overlay terms in active DAG work are `menu`, `thumb`, `legend`, plus one primary surface overlay (`stage` / `table` / `xterm` / `docs`)
- `src/libs/ui` remains legacy/shared runtime for older plugin UIs.
- New overlay-first work should prefer `src/libs/ui_v2`, while preserving compatibility for existing `src/libs/ui` consumers.

# `ui`

`src/libs/ui` is the shared front-end runtime utility library for Dialtone plugin UIs.

## Scope

- Section lifecycle orchestration (`SectionManager`)
- Shared menu controls (`Menu`)
- Common telemetry helpers (`FpsCounter`, `GpuTimer`)
- Typing utility (`startTyping`)
- Shared base styles (`style.css`)
- App bootstrap helper (`setupApp`)

## Section Visibility API

Per-section visibility for header/menu is configured through `SectionConfig.header`.

```ts
sections.register('home', {
  containerId: 'home',
  header: { visible: true, menuVisible: true },
  load: async () => { /* ... */ }
})

sections.register('docs', {
  containerId: 'docs',
  header: { visible: false, menuVisible: false },
  load: async () => { /* ... */ }
})
```

Supported `header` fields:

- `visible?: boolean` controls `.header-title`
- `menuVisible?: boolean` controls `.top-right-controls`
- `title?: string`
- `subtitle?: string`
- `telemetry?: boolean`
- `version?: boolean`

At runtime, `SectionManager` also toggles:

- `body.hide-header` when `header.visible === false`
- `body.hide-menu` when `header.menuVisible === false`

These classes are part of `src/libs/ui/style.css` and can be extended by plugin-local CSS.

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

## DOM Contract

`setupApp`/`SectionManager` assumes these shared elements exist:

- `.header-title`
- `#header-subtitle` (optional but recommended)
- `.top-right-controls`
- `#global-menu-toggle`
- `#global-menu-panel`

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
