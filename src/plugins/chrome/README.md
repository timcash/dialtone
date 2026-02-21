# Chrome Plugin

`src/plugins/chrome` manages local Chrome/Chromium/Edge instances for Dialtone.

It supports:
- verifying debug connectivity
- listing browser processes
- launching tagged browser sessions
- killing Dialtone-only (or all) browser processes
- running chrome plugin self-test

## CLI

Use scaffold-style commands:

```bash
./dialtone.sh chrome help
./dialtone.sh chrome verify src_v1 --port 9222
./dialtone.sh chrome list src_v1 --headed
./dialtone.sh chrome new src_v1 https://example.com --gpu --role dev
./dialtone.sh chrome kill src_v1 all
./dialtone.sh chrome test src_v1
```

Notes:
- `src_v1` is accepted as an optional version argument for all commands.
- Current runtime behavior is version-agnostic (single implementation path), but command shape is normalized to the versioned style.

## Commands

- `verify [src_v1] [--port N] [--debug]`
- `list [src_v1] [--headed|--headless] [--verbose|-v]`
- `new [src_v1] [URL] [--port N] [--gpu] [--headless] [--role NAME] [--reuse-existing] [--debug]`
- `kill [src_v1] [PID|all] [--all] [--windows]`
- `test [src_v1]`
- `install`

## WSL / Windows Host Support

When running under WSL:
- `chrome list` can show Windows-host browser processes.
- `chrome kill` auto-detects Windows process handling (or force via `--windows`).
- `chrome new` can launch host browser when needed.

## Examples

```bash
./dialtone.sh chrome verify src_v1 --port 9222
./dialtone.sh chrome list src_v1 --verbose
./dialtone.sh chrome new src_v1 --headless --role smoke
./dialtone.sh chrome kill src_v1 12345
./dialtone.sh chrome kill src_v1 all --all
./dialtone.sh chrome test src_v1
```
