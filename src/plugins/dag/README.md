# DAG Plugin

A section-based Dialtone plugin for exploring DAGs with a Three.js hero toy, clear documentation, and the nested layer visualization migrated from `dag_viz`.

## CLI Commands

```bash
# 🛠️ Development: Start Go host (serves built UI)
./dialtone.sh dag dev

# 🏗️ Build: Compile UI assets
./dialtone.sh dag build src_v1

# ✅ Lint: Go + TypeScript checks
./dialtone.sh dag lint src_v1

# 💨 Smoke Test: Build + Automated UI verification (Generates SMOKE.md)
./dialtone.sh dag smoke src_v1
```

## Smoke Test Plan (Build Order)

1. **Hero Section Renders**
   Pass criteria: `dag-hero` is navigable via `dialtest.NavigateToSection`, the hero title is visible (`aria-label="DAG Hero Title"`), and a Three.js canvas is mounted in the hero container (`aria-label="DAG Hero Canvas"`).

2. **Docs Section Content**
   Pass criteria: `dag-docs` is navigable, the docs title is visible (`aria-label="DAG Docs Title"`), and the command snippet block is present (`aria-label="DAG Docs Commands"`).

3. **Layer Nest Visualization**
   Pass criteria: `dag-layer-nest` is navigable, the visualization root is visible (`aria-label="DAG Layer Nest"`), and a Three.js canvas is mounted (`aria-label="DAG Layer Canvas"`).

4. **Header/Menu Visibility Rules**
   Pass criteria: Header and menu are visible on `dag-hero` and hidden on `dag-docs` and `dag-layer-nest` via `body.hide-header` / `body.hide-menu`.

Each step captures a screenshot and appends results to `src/plugins/dag/src_v1/SMOKE.md` with server logs stored in `smoke_server.log`.
The smoke test also records lint/build output in `SMOKE.md` under preflight sections.
