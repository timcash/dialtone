# DAG UI Design Notes (`src_v3`)

## Goals

- Reduce CSS surface area and selector churn.
- Prefer semantic HTML (`section`, `nav`, `form`, `aside`) over class-heavy wrappers.
- Make `mode-form` the primary control abstraction for section overlays.
- Keep behavior stable by preserving aria labels used by tests.

## Naming Rules

- Section IDs use: `<plugin-name>-<subname>-<underlay-type>`.
- Valid underlay types in UI v2: `stage`, `table`, `docs`, `xterm`, `video`.
- Current DAG section IDs:
  - `dag-meta-table`
  - `dag-3d-stage`
  - `dag-log-xterm`
- Control overlays use `mode-form` (not `thumbs`).

## Implemented Changes

- Stage and log control overlays are now semantic forms:
  - `form.mode-form[data-mode-form='dag']`
  - `form.mode-form[data-mode-form='log']`
- Removed per-button class dependency in markup (`dag-thumb`, `dag-action-btn`, etc.).
- CSS now styles controls primarily through structure:
  - `form.mode-form > button`
  - `form.mode-form > input`
  - `form.mode-form > button[aria-label$='Mode']`
- Overlay binding in section registration now targets forms directly.
- Existing aria labels were kept so test flows continue to work.

## CSS Simplification Pattern

- Prefer low-specificity structural selectors.
- Avoid one-off button classes when aria already identifies role.
- Keep only domain-specific classes where structure alone is insufficient:
  - `dag-history`
  - `dag-chatlog`
  - `dag-chatlog-xterm`

## Recommended Next Steps

- Replace remaining DAG-prefixed spacing token names with generic UI tokens.
- Move shared `mode-form` styles to `src/plugins/ui/style.css` once another plugin adopts it.
- Normalize underlay section aria labels to include section ID for easier debugging.
- Add a tiny section-style checklist to PR template:
  - semantic wrapper tag
  - `mode-form` for mode-driven controls
  - no redundant button classes
  - aria stability for tests

## Testing Expectations

After UI structure changes, run:

```bash
./dialtone.sh dag lint src_v3
./dialtone.sh dag build src_v3
./dialtone.sh dag test src_v3 --attach
```

This validates compile/build and live section navigation/control behavior.
