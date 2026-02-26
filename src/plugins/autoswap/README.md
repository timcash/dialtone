# Autoswap Plugin

`autoswap src_v1` stages manifest-defined robot composition artifacts and can run a local composition smoke cycle.

## Commands

```bash
./dialtone.sh autoswap src_v1 help
./dialtone.sh autoswap src_v1 stage --manifest src/plugins/robot/src_v2/config/composition.manifest.json
./dialtone.sh autoswap src_v1 run --manifest src/plugins/robot/src_v2/config/composition.manifest.json
```

`run` verifies manifest artifacts, starts `robot_v2` with `ui/dist`, starts `camera_v1` + `mavlink_v1` sidecars, and validates heartbeats over NATS.

