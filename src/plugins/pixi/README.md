# Pixi Plugin

Managed Pixi runtime for Dialtone.

## Commands

```bash
./dialtone.sh pixi src_v1 install
./dialtone.sh pixi src_v1 exec <pixi-args...>
./dialtone.sh pixi src_v1 run <pixi-args...>
./dialtone.sh pixi src_v1 version
./dialtone.sh pixi src_v1 test
```

`install` uses the official Pixi installer with `PIXI_HOME` pointed at `DIALTONE_ENV/pixi` and disables shell `PATH` mutation so the runtime stays managed inside Dialtone.
