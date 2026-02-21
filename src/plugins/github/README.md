# GitHub Plugin (`src/plugins/github`)

This plugin is now `src_v1`-style and uses:
- `logs` library: `dialtone/dev/plugins/logs/src_v1/go`
- `test` library: `dialtone/dev/plugins/test/src_v1/go`

## Layout

```text
src/plugins/github/
  README.md
  scaffold/main.go
  src_v1/
    go/github.go
    issues/
    test/
      01_self_check/main.go
      02_example_library/main.go
```

## Commands

```bash
./dialtone.sh github help
./dialtone.sh github test src_v1
```

### Issue Commands

Download repository issues into task-style markdown files:

```bash
./dialtone.sh github issue sync src_v1
./dialtone.sh github issue sync src_v1 --state all --limit 500
./dialtone.sh github issue sync src_v1 --out plugins/github/src_v1/issues
```

Each generated issue file:
- lives at `src/plugins/github/src_v1/issues/<issue_id>.md`
- starts with a `signature` block
- always starts with `- status: wait`

This is intended for a later LLM pass that upgrades each issue into full task format and flips status to `ready`.

Quick issue passthrough commands:

```bash
./dialtone.sh github issue list src_v1 --state open --limit 30
./dialtone.sh github issue view src_v1 123
```

### Pull Request Commands

Simple PR flow:

```bash
./dialtone.sh github pr src_v1
```

Behavior:
- pushes current branch (`git push -u origin <branch>`)
- creates PR if missing
- updates existing PR if one already exists

Additional PR actions:

```bash
./dialtone.sh github pr src_v1 review   # mark PR ready for review
./dialtone.sh github pr src_v1 view
./dialtone.sh github pr src_v1 merge
./dialtone.sh github pr src_v1 close
```

## Tests

Run:

```bash
./dialtone.sh github test src_v1
```

Covers:
- issue markdown render includes `status: wait`
- library example runs and prints pass marker
- task-file output shape is valid for downstream task-upgrade workflows
