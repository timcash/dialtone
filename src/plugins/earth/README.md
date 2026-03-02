# earth Plugin

Three.js earth hero visualization packaged as a versioned Dialtone plugin.

## Commands

```bash
./dialtone.sh earth src_v1 install
./dialtone.sh earth src_v1 install --no-dev
./dialtone.sh earth src_v1 install --port 5181 --host 0.0.0.0 --browser-node legion --public-url http://192.168.4.52:5181
./dialtone.sh earth src_v1 dev --port 5181
./dialtone.sh earth src_v1 dev --port 5181 --browser-node chroma
./dialtone.sh earth src_v1 open-dev --hosts darkmac,legion,gold --port 5177
./dialtone.sh earth src_v1 build
./dialtone.sh earth src_v1 serve --addr :8891
./dialtone.sh earth src_v1 go-build
./dialtone.sh earth src_v1 download-dev --repo https://github.com/timcash/dialtone.git --branch main --dest ./.dialtone/plugins/earth-dev --port 5181
./dialtone.sh earth src_v1 test
./dialtone.sh earth src_v1 test --attach chroma
```

## Notes

- UI composition follows `src/plugins/ui/src_v1` (`setupApp`, `SectionManager`, `Menu`).
- Hero section keeps the `www` earth component look/behavior (cloud shaders, atmosphere, rotating hex layer).
- Go server serves `ui/dist` with SPA fallback.
- `install` installs deps and then starts Vite dev with HMR by default.
- `install --no-dev` keeps install-only behavior.
- `open-dev` points browser hosts to each host's local Vite URL (`http://127.0.0.1:<port>` by default).
- `dev` auto-starts a headed remote browser on `chroma` when running in WSL.
- `dev --browser-node <mesh-node>` overrides the remote browser node.
- `dev` publishes lifecycle logs to NATS subject `logs.dev.earth-src-v1`.
- Default `test` uses isolated `role=earth-test` headless browser and only cleans up that role.
- `test --attach <mesh-node>` attaches to a headed `role=earth-dev` browser on the remote node and leaves dev browser sessions running.
