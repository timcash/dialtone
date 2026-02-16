# DAG Agent Plan

This document defines the next implementation phase for `src/plugins/dag/src_v3`.

## Objective

Ship a mobile-first DAG UI where:
1. The only UI controls are:
- node selection in the scene
- three thumb buttons
- one main menu button
2. All core DAG actions are reachable from those controls.
3. The full automated test suite passes using a mobile-sized viewport.

## Product Constraints

1. No desktop-first assumptions.
2. No floating debug panels as user-facing UI.
3. No drag-to-move nodes.
4. Rank-based layout remains left-to-right.
5. Nested layers remain distinct layer stacks.

## Required UI Surface

## 1) Node Selection (Scene)

Selecting a node is the primary context action.

Minimum behavior:
1. Tap node to select.
2. Selected node updates input/output highlighting.
3. Selected node can open nested layer.
4. Selected node is the target for thumb-button actions.

## 2) Three Thumb Buttons

Use three large fixed-position thumb buttons near the bottom edge (mobile thumb reach).

Recommended mapping:
1. Left thumb button: `Back`
- Goes back one layer in history if available.
2. Center thumb button: `Action`
- Primary context action for selected node:
  - If node has nested layer: open nested layer.
  - Else: create/connect behavior based on current flow mode.
3. Right thumb button: `Mode`
- Switches or cycles interaction mode for `Action` behavior (for example create node vs connect edge vs remove).

## 3) Main Menu Button

Separate persistent menu button (not part of thumb stack), typically top-right.

Menu button requirements:
1. Opens global actions/settings/help.
2. Does not block thumb controls.
3. Exposed with stable `aria-label` for tests.

Button requirements:
1. Touch target at least 56x56 px.
2. High contrast against scene.
3. Safe-area aware (`env(safe-area-inset-*)`).
4. Exposed with stable `aria-label`s for tests.
5. Each thumb button can expand vertically into a stack up to 3 buttons high.

Expansion behavior:
1. Single tap:
- Trigger primary action immediately.
2. Double tap:
- Expand/collapse that thumb button into its stacked secondary actions (max 3 high).
3. Expanded actions:
- Must remain within thumb reach.
- Must preserve 56x56 minimum touch targets.
- Must auto-collapse after action or explicit second double tap.

## Mobile Viewport Requirement

All tests must run with mobile viewport, not desktop default.

Target baseline:
1. Width: `390`
2. Height: `844`
3. Device scale factor: `2`

Use one consistent size across all DAG tests.

## Implementation Work Items

## A) UI Layer

1. Add a mobile control bar in `src/plugins/dag/src_v3/ui/index.html`:
- `button[aria-label="DAG Back"]`
- `button[aria-label="DAG Action"]`
- `button[aria-label="DAG Mode"]`
- `button[aria-label="DAG Menu"]`
2. Add mobile-first styles in `src/plugins/dag/src_v3/ui/src/style.css`.
3. Wire button handlers in `src/plugins/dag/src_v3/ui/src/components/three/index.ts`.
4. Ensure button handlers call the same state transitions as debug bridge actions.

## B) Three Interaction Model

1. Keep node selection as primary interaction.
2. Map `Back` button to layer-history back.
3. Map `Action` button to selected-node context behavior.
4. Map `Mode` button to context/action-mode selection.
5. Map `Menu` button to global UI menu actions.
6. Keep rank/grid constraints unchanged.
7. Keep nested layer offsets/stacking and camera fitting behavior.

## C) Test Harness Migration to Mobile

Update all `src/plugins/dag/src_v3/test/*.go` steps that open browser contexts:
1. Set/force mobile viewport for the shared browser session.
2. Replace desktop pixel assumptions with mobile-safe checks.
3. Keep screenshot assertions but with mobile frame.

Recommended updates:
1. In `session.go`, configure browser emulation once at session start.
2. Validate scene elements are inside `window.innerWidth/innerHeight` for mobile.
3. Update any hard-coded screenshot pixel checks to use projected coordinates only.

## D) Test Coverage Additions (Mobile)

Add/adjust tests to prove mobile usability:
1. Thumb buttons visible and tappable.
2. Menu button visible and tappable.
3. Back button works from nested layer.
4. Action button opens nested layer when node supports nesting.
5. Action button behavior with non-nested selected node is deterministic.
6. No overlap between thumb buttons and crucial node area in baseline scene.
7. Single tap and double tap behavior for expandable thumb stacks.

## Acceptance Criteria

All of the following must be true:
1. `./dialtone.sh dag test src_v3` passes fully on mobile viewport.
2. `TEST.md` includes logs proving button actions ran.
3. Node selection + three thumb buttons + one menu button are the only user-facing controls.
4. Nested layer navigation works without keyboard/mouse-only affordances.
5. Scene remains understandable on mobile screen size.

## Commands

```bash
./dialtone.sh dag install src_v3
./dialtone.sh dag dev src_v3
./dialtone.sh dag test src_v3
./dialtone.sh dag help
```

## Execution Order

1. Add mobile viewport emulation in test session setup.
2. Add thumb buttons and mobile styles.
3. Wire back/action behaviors in Three control.
4. Update existing tests to mobile-safe assertions.
5. Add button interaction tests.
6. Regenerate `TEST.md` and screenshots via `dag test src_v3`.
