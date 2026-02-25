# Swarm Plugin

Primary interface:

```bash
./dialtone.sh swarm src_v3 <command> [args]
```

## Commands

```bash
./dialtone.sh swarm src_v3 help
./dialtone.sh swarm src_v3 install
./dialtone.sh swarm src_v3 build --arch host
./dialtone.sh swarm src_v3 build --arch all
./dialtone.sh swarm src_v3 test --mode all
./dialtone.sh swarm src_v3 test --mode rendezvous --rendezvous-url https://relay.dialtone.earth
./dialtone.sh swarm src_v3 deploy --host <ip> --user <user> --pass <password>
./dialtone.sh swarm src_v3 relay serve --listen :8080
```

## Notes

- `src_v3` builds static binaries:
  - `dialtone_swarm_v3_x86_64`
  - `dialtone_swarm_v3_arm64`
- Build/deploy/test logic lives in Go:
  - `src/plugins/swarm/scaffold/main.go`
  - `src/plugins/swarm/src_v3/go/`
- `libudx` is tracked as a submodule:
  - `src/plugins/swarm/src_v3/libudx`

Legacy swarm CLI code under `src/plugins/swarm/cli` is not the source of truth for `src_v3`.

