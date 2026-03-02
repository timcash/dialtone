# Mod Plugin

`mod` is a `v1` plugin for managing plugin mods as Git submodules.
It is the source of truth for mod sync behavior via Git (not `rsync`).

## Usage

```bash
./dialtone.sh mod v1 <command> [args]
```

## Mapping Convention

- Repo name: `dialtone-<mod-name>`
- Owner/repo: `<owner>/dialtone-<mod-name>`
- Parent repo path: `src/mods/<mod-name>`

Example: `timcash/dialtone-mod-name` maps to `src/mods/mod-name`.

## Commands

- `add <mod-name> [--repo <url|owner/repo|path>] [--owner <owner>] [--repo-name <name>] [--path src/mods/<name>] [--branch <branch>] [--with-ui=true|false] [--ui-from <path>] [--dry-run] [--commit=true|false] [--push=true|false] [--message "..."] [--private|--public]`
- `list`
- `status [--name <name>] [--short]`
- `sync [--host <name|all|local>] [--repo-dir <path>] [--mod <name|path> ...] [--skip-self=true|false] [--strict-scaffold=true|false]`
- `sync-ui [--mod <name|path> ...] [--from <path>] [--dry-run] [--commit] [--push]`
- `gh-create <mod-name> --owner <owner> [--repo-name <name>] [--private|--public]`

## One-command Add

`add` can do the full setup in one command:

1. Resolve repo target (default `<owner>/dialtone-<mod-name>`).
2. Ensure GitHub repo exists (create when needed).
3. Seed scaffold files when repo is empty.
4. Optionally seed default UI files into `v1/ui`.
5. Add as submodule to `src/mods/<mod-name>`.
6. Commit and push parent repo pointer updates.

## UI Sync Strategy (No Nested Submodules)

Mods do not depend on a nested UI submodule.
Instead, copy a baseline UI folder from the `ui` plugin into each mod:

- Default source: `src/plugins/ui/src_v1/ui`
- Mod destination: `src/mods/<mod-name>/v1/ui`

Use:

```bash
./dialtone.sh mod v1 sync-ui --mod mod-name
```

For all mods:

```bash
./dialtone.sh mod v1 sync-ui
```

`add` uses `--with-ui=true` by default so new mods get a starting UI folder.

## Examples

```bash
./dialtone.sh mod v1 add mod-name --owner timcash --public
./dialtone.sh mod v1 add mod-name --owner timcash --dry-run
./dialtone.sh mod v1 sync --host all
./dialtone.sh mod v1 sync-ui --mod mod-name --dry-run
```
