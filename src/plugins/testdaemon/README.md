# testdaemon

`testdaemon` is the generic local task and service fixture for `repl src_v3`.

Use it to prove shared control-plane behavior without depending on Chrome, robot, or any other plugin-specific daemon.

## `src_v1`

```bash
./dialtone.sh testdaemon src_v1 format
./dialtone.sh testdaemon src_v1 build
./dialtone.sh testdaemon src_v1 test

./dialtone.sh testdaemon src_v1 run --mode once
./dialtone.sh testdaemon src_v1 emit-progress --steps 5
./dialtone.sh testdaemon src_v1 sleep --seconds 10
./dialtone.sh testdaemon src_v1 exit-code --code 17
./dialtone.sh testdaemon src_v1 panic
./dialtone.sh testdaemon src_v1 crash
./dialtone.sh testdaemon src_v1 hang

./dialtone.sh testdaemon src_v1 service --mode start --name demo
./dialtone.sh testdaemon src_v1 service --mode status --name demo
./dialtone.sh testdaemon src_v1 heartbeat --name demo
./dialtone.sh testdaemon src_v1 heartbeat --name demo --mode stop
./dialtone.sh testdaemon src_v1 heartbeat --name demo --mode resume
./dialtone.sh testdaemon src_v1 shutdown --name demo
./dialtone.sh testdaemon src_v1 service --mode stop --name demo
```

The fixture writes logs under the shared Dialtone home logs directory and keeps simple per-service state under `~/.dialtone/testdaemon/services/<name>`.
