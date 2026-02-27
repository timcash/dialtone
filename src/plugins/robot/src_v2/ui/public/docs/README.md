# Robot src_v2 Docs

ROBOT_UI_DOCS_VERSION: robot-src_v2-docs-v4

This page documents the `robot src_v2` runtime and operator workflow.

## Quick Commands

```bash
# Local development
./dialtone.sh robot src_v2 install
./dialtone.sh robot src_v2 build
./dialtone.sh robot src_v2 test

# Publish artifacts to GitHub release (publish-only)
./dialtone.sh robot src_v2 publish --repo timcash/dialtone

# Install/update autoswap service on robot from manifest URL
./dialtone.sh autoswap src_v1 deploy \
  --host rover \
  --user tim \
  --service \
  --manifest-url https://raw.githubusercontent.com/timcash/dialtone/main/src/plugins/robot/src_v2/config/composition.manifest.json \
  --repo timcash/dialtone

# Force autoswap refresh check immediately
./dialtone.sh autoswap src_v1 update --host rover --user tim

# Verify robot runtime + UI checks
./dialtone.sh robot src_v2 diagnostic --host rover --user tim

# Start/refresh local WSL relay service
./dialtone.sh robot src_v2 relay --subdomain rover-1 --robot-ui-url http://rover-1:18086 --service
```

## Runtime Model

`src_v2` is manifest-driven. Autoswap manages processes and artifacts for:
- `dialtone_robot_v2`
- `dialtone_camera_v1`
- `dialtone_mavlink_v1`
- `dialtone_repl_v1`
- `robot_src_v2_ui_dist`

Only autoswap is installed as a host service. Autoswap then manages all robot composition processes.

## Required Verification

After changes:
1. `./dialtone.sh robot src_v2 test` passes locally.
2. `./dialtone.sh robot src_v2 publish --repo timcash/dialtone` completes.
3. `./dialtone.sh autoswap src_v1 update --host rover --user tim` triggers refresh.
4. `./dialtone.sh robot src_v2 diagnostic --host rover --user tim` passes.
5. Public relay URL serves the UI through local WSL relay service.

## Troubleshooting

- Check autoswap state:
```bash
./dialtone.sh autoswap src_v1 service --mode status --host rover --user tim
./dialtone.sh autoswap src_v1 service --mode list --host rover --user tim
```

- Reset robot runtime state:
```bash
./dialtone.sh robot src_v2 clean --host rover --user tim
```
