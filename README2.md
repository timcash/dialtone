# Dialtone quickstart: `dialtone_mod` and `mods`

This repo uses two entry points:

- `./dialtone_mod` — main orchestrator and mesh-aware launcher
- `./src/mods/mod/v1/main.go` via `mods` — mod lifecycle + sync tooling

Always run mod commands through `dialtone_mod` so they execute in the project’s Nix environment.

## 1) Start working with Nix

```sh
cd /home/user/dialtone
./dialtone_mod
```

Typical interactive usage:

- `./dialtone_mod` (show top-level help)
- `./dialtone_mod help` (if supported by your shell)

## 2) Mesh nodes

Mesh config is loaded from `env/mesh.json` and used by `mods v1` / related workflows.

- Hosts are addressed by name (for example: `gold`, `wsl`).
- `./dialtone_mod mods v1 <command> ...` runs the selected command from the current repo context.

## 3) `mods v1` command set

Core commands:

```sh
./dialtone_mod mods v1 list
./dialtone_mod mods v1 status [--name <mod-name>] [--short]
./dialtone_mod mods v1 new <mod-name> [--repo ...] [--path src/mods/<name>] [--branch main]
./dialtone_mod mods v1 add --mod <mod-name> <paths...>
```

Lifecycle:

```sh
./dialtone_mod mods v1 commit --all --message "..."
./dialtone_mod mods v1 push
```

- `commit` stages and commits local changes (or only selected mod when `--mod` is used).
- `push` pushes the parent repo and dirty submodules in the same workflow.

Pulling and syncing:

```sh
./dialtone_mod mods v1 pull --host all
./dialtone_mod mods v1 sync --host gold --mod mesh
./dialtone_mod mods v1 rsync --host gold --mod mosh
./dialtone_mod mods v1 rsync --host gold --all-repo --dry-run
./dialtone_mod mods v1 rsync --host gold --all-repo
```

- `sync` updates tracked submodule paths on target hosts.
- `rsync` performs rsync-based sync and honors `.gitignore` (and standard git exclude rules) through a generated `--exclude-from`.
- `--dry-run` prints actions only.

## 4) `mesh` and `tmux` helpers

```sh
./dialtone_mod mesh v1 list
./dialtone_mod tmux v1 logs --host gold
```

## 5) Typical flow

1. Pull latest state from all nodes.
2. Apply local changes.
3. Sync/update targets as needed.
4. Commit then push from local.

```sh
./dialtone_mod mods v1 pull --host all --dry-run
./dialtone_mod mods v1 pull --host all
./dialtone_mod mods v1 rsync gold --mod mesh
./dialtone_mod mods v1 commit --all --message "Update mesh tools"
./dialtone_mod mods v1 push
```

## 6) Notes

- `mods` and `plugins` are separate systems; they do not replace each other.
- Dialtone 2 sync behavior prefers git-clean paths and excludes ignored files.
- `mods v1` should typically be run from the repo root.
