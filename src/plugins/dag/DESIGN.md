# DAG Plugin Design (Buildout Spec)

This document captures the DAG behavior represented in `../dag_viz` (`src/`, `src3/`, `src4/`, `src5/`, `summary.md`, `v2.md`) and organizes it into a single product spec for `src/plugins/dag`.

## Scope

- 3D DAG visualization with nested sub-layers.
- Left-to-right rank-based graph flow per layer.
- Interactive node/edge editing.
- Layer navigation with back/forward semantics.
- UI/HUD overlays for context and control.
- Testability through deterministic logs, screenshots, and pixel checks.

## Core Concepts

- `Layer` (aka `Plane` / `DSP`): A 2D graph surface in 3D (`THREE.Group`).
- `Node`: 3D object with id, label, rank, inputs, outputs, and optional nested sub-layer.
- `Edge`: Directed connection within a layer from node output to node input (rank N -> N+1+).
- `VILL` (Vertical Inter-Layer Link): Visual link from a parent node to nodes in its sub-layer.
- `Current Layer`: Only this layer accepts creation/link/delete actions.
- `Navigation Path`: Ordered stack of selected nodes representing current drill-down depth.
- `History`: Forward stack used after going back (scrub-up / breadcrumb back).

## Graph Model

### Nodes

- Required fields:
  - `id: string` (stable key)
  - `label: string`
  - `rank: number`
  - `inputs: string[]`
  - `outputs: string[]`
  - `subLayer?: Layer`
- Visual defaults:
  - Dark body, white label, border/wireframe.
  - Hover/selection emissive highlight.
- Node may own a sub-layer positioned below parent (`Y` offset).

### Edges

- Directed, curved links (Bezier/Catmull style) with left-to-right flow.
- Style supports weight/importance:
  - High weight brighter/less transparent.
  - Low weight dimmer/more transparent.
- Recomputed after layout changes.

### Layers

- Each layer has:
  - `id`, `group`, `nodes`, `edges`
  - optional grid/ground plane for interaction anchoring.
- Layers may remain visible in background while navigating deeper (context mode).
- Sublayers are transient by default and can be revealed on hover/select.

## Layout Rules

- Rank constraint: destination node rank must be `> source node rank`.
- Node placement baseline:
  - `x = rank * rankSpacing`
  - `z = ordinalInRank * rowSpacing`
- Auto-layout is re-applied after:
  - add/remove node
  - add/remove edge
  - re-rank operations
- Sublayer placement:
  - anchored beneath parent node world position
  - must avoid overlap with sibling sublayers (DSP spatial separation).

## Navigation Model

- `Dive` (click/select node):
  - push node id onto `path`
  - clear forward `history`
  - transition camera to selected node/sub-layer context.
- `Back` (scroll up, breadcrumb click, or back action):
  - pop from `path`
  - push popped id into `history`
  - move camera toward parent context.
- `Forward` (scroll down after back):
  - pop id from `history`
  - append back to `path`
  - restore child context.
- Breadcrumb displays `Root > ... > Current` and supports jump navigation.

## Interaction Categories

### 1) Hover & Selection

- Mouse move raycasts all visible nodes (including nested).
- Hover enter/leave updates visual state and logs deterministic events.
- Hover can reveal sub-layer preview.
- Single click selects node and may trigger dive.
- Selection state must be queryable for tests.

### 2) Camera & Exploration

- Scroll wheel:
  - zoom / scrub path backward-forward
- WASD (or equivalent) pans current view.
- Smooth transitions for dive/back/teleport.
- Optional ride/path spline visualization for orientation.

### 3) Node Editing

- Add node:
  - double-click empty ground or use UI action.
  - create node at cursor-to-layer intersection.
- Remove node:
  - delete selected node via key or menu action.
  - remove connected incoming/outgoing edges.
  - if node has sub-layer, enforce delete policy (cascade or block with prompt).
- Edit node metadata:
  - label/id/attributes via inspector panel.

### 4) Edge Editing

- Add edge (manual link mode):
  - select source then destination (e.g., Ctrl+Click flow).
  - enforce DAG/rank constraints.
  - prevent duplicate edges.
- Remove edge:
  - select edge and delete via key/menu.
- Visual confirmation:
  - hover/selection highlight on edge.

### 5) Layer & Hierarchy Management

- Create sub-layer for node on demand.
- Add/remove sub-nodes and sub-edges within sub-layer.
- Render VILLs from parent to sub-layer nodes.
- Color-code VILL sets by parent group when multiple DSPs are active.
- Ensure multi-DSP spacing remains collision-free.

### 6) HUD & Productivity UI

- Breadcrumb bar (clickable path).
- Minimap showing global layout + current focus.
- Thumb/floating menus for mobile-friendly quick actions.
- Toolbox actions:
  - add node
  - link mode
  - delete selected
  - reset camera
  - toggle overlays (grid, labels, splines, VILLs)
  - save/load state
- Search + teleport to node id/label.
- Node inspector for properties and metrics.
- Legend explaining colors and link styles.

## User Workflows

### Create and Link a Node

1. Enter target layer (or stay at root).
2. Add node at cursor location.
3. Enter link mode and select source node.
4. Select destination node.
5. System validates DAG constraint, applies rank update, reflows layout, redraws edges.

### Remove a Node or Edge

1. Select target node/edge.
2. Trigger delete action.
3. System removes object and dependent links.
4. Layout and links recompute.
5. Breadcrumb/minimap/state update.

### Dive Into Hierarchy and Go Back

1. Click node with sub-layer to dive.
2. Breadcrumb path extends.
3. Scroll up or click breadcrumb ancestor to go back.
4. Scroll down to re-enter forward history when applicable.

## Rendering & Visual Requirements

- Dark background and grid for depth cues.
- Nodes readable at default camera distance.
- Link curvature and opacity communicate flow/weight.
- Hover spline / focus aids optional but supported.
- Parent layer context remains visible or dimmed while deep.

## Testability Requirements

- Deterministic console logs for key events:
  - hover enter/exit
  - node selected/dive
  - path back/forward
  - add/remove node
  - add/remove edge
- Screenshot artifacts and markdown report generation.
- Pixel checks for:
  - hover/selection color changes
  - camera framing correctness
  - edge visibility at expected regions
- Metrics API for tests:
  - node count, edge count, layer depth, path, selected/hovered ids.

## Acceptance Baseline

- Root DAG renders with rank-based layout and curved directed edges.
- Nodes support nested sub-layers with visible parent-child vertical links.
- User can add/remove nodes and edges in current layer.
- User can dive, go back, and go forward through path history.
- HUD provides breadcrumb + minimap + quick actions.
- Automated tests produce `TEST.md` with screenshots and pass key interaction checks.
