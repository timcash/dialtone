# Camera Plugin (`src_v1`)

```bash
# Build / run / test
./dialtone.sh camera src_v1 build
./dialtone.sh camera src_v1 run --listen :19090 --serve-stream=true
./dialtone.sh camera src_v1 test

# Remote stream smoke test over ssh mesh (no publish/UI required)
./dialtone.sh camera src_v1 stream --host rover --snapshot /tmp/rover-camera.jpg
```

## Purpose

`camera src_v1` serves the rover camera stream (`/stream`) and heartbeat.

Use `stream --host` to validate a remote host camera endpoint directly from this machine.
It tunnels to the remote camera port, checks `/health`, requests `/stream`, and saves one JPEG snapshot.
