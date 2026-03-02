# Chrome Plugin (src_v1)

Daemon-first Chrome control for local and mesh hosts.

## Command Workflow

```bash
# generic plugin workflow
./dialtone.sh chrome src_v1 install
./dialtone.sh chrome src_v1 format
./dialtone.sh chrome src_v1 lint
./dialtone.sh chrome src_v1 build
./dialtone.sh chrome src_v1 test --filter open

# deploy daemon binary to hosts
./dialtone.sh chrome src_v1 deploy --host darkmac --service --role dev
./dialtone.sh chrome src_v1 deploy --host legion --service --role dev
./dialtone.sh chrome src_v1 deploy --host gold --service --role dev

# inspect running instances
./dialtone.sh chrome src_v1 list --host darkmac,legion,gold --verbose
./dialtone.sh chrome src_v1 remote-list --nodes darkmac,legion,gold --origin dialtone --verbose

# open/reuse one headed browser per host (daemon path)
./dialtone.sh chrome src_v1 open --host darkmac,legion --role dev --url http://127.0.0.1:5177
./dialtone.sh chrome src_v1 open --host darkmac --role dev --kiosk --url https://dialtone.earth

# fallback when daemon /open is unavailable on a host
./dialtone.sh chrome src_v1 remote-new --host legion --role dev --url http://127.0.0.1:5177 --reuse-existing=false
```

## Core Rules

- One Dialtone Chrome browser process per host.
- One page tab per managed browser session.
- Host controls browser state; callers send control signals.
- `gold` should run non-kiosk unless explicitly requested.

## Daemon Debug Checklist

1. Verify daemon deployed and started:

```bash
./dialtone.sh chrome src_v1 deploy --host <host> --service --role dev
```

2. Verify process state:

```bash
./dialtone.sh chrome src_v1 list --host <host> --verbose
./dialtone.sh chrome src_v1 remote-list --nodes <host> --origin dialtone --verbose
```

3. If `open` fails with `remote chrome service unavailable`:

- redeploy daemon on that host
- run `remote-new` as fallback
- re-check tab/process count with `list --host`

4. If tabs drift above one:

- this should fail guard checks in daemon/CLI paths
- run policy test and investigate host-specific control endpoint failures

## Tests

```bash
# workflow
./dialtone.sh chrome src_v1 test
./dialtone.sh chrome src_v1 test --filter open

# policy: required hosts running + gold non-kiosk
./dialtone.sh chrome src_v1 test --filter policy --role dev --host darkmac,gold,legion

# gold health: verifies gold vite/daemon/url + non-kiosk + single-tab
./dialtone.sh chrome src_v1 test --host gold --role dev --required-hosts gold --filter gold
```

Test artifacts:

- `src/plugins/chrome/src_v1/TEST.md`
- `src/plugins/chrome/src_v1/ERRORS.md`

## Known Host Notes

- `legion`: daemon `/open` may intermittently fail from WSL; use `remote-new` fallback.
- `gold`: if local build/dev commands fail with `xcode-select` prompts, install Apple Command Line Tools on Gold.
- `wsl -> windows`: avoid loopback assumptions; control via daemon/host commands.
