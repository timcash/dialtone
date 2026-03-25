# Windows Dev Notes

This repo may be worked from a Windows checkout while the real runtime and tests execute inside WSL.

## Preferred Layout

- Windows repo: `C:\Users\timca\dialtone`
- WSL repo: `/home/user/dialtone`
- WSL tmux session for visible command execution: `windows`

Keep the Windows repo for editing, review, and native Windows Git operations.
Keep the WSL repo for Linux runtime checks, REPL/plugin tests, SSH, tmux, and dependency installs.

## Command Routing

Use the `wsl-tmux` wrapper from Windows so WSL commands run inside the visible tmux session:

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 test"
wsl-tmux read
wsl-tmux interrupt
```

The tmux session name is expected to be `windows`.
If the pane gets noisy or wedged, it is fine to recreate the session:

```powershell
wsl.exe bash -lc "tmux kill-session -t windows 2>/dev/null || true; tmux new-session -d -s windows -c /home/user/dialtone"
```

## Git Rules

- Trust native Windows Git for the Windows checkout.
- Trust WSL Git for the WSL checkout.
- Do not judge the Windows repo state from `/mnt/c/...` inside WSL. Line endings and file mode handling can make that view misleading.
- If pushing from WSL hangs on auth, use a temporary `GIT_ASKPASS` helper that shells out to Windows `gh.exe`.

## Editing Flow

1. Edit in `C:\Users\timca\dialtone`.
2. Sync the changed files into `/home/user/dialtone` when the WSL copy needs the same patch.
3. Normalize line endings in WSL after copying if needed:

```bash
perl -0pi -e 's/\r\n/\n/g' path/to/file
```

Large unexpected WSL diffs usually mean CRLF was copied into Linux files.

## Testing in WSL

Prefer running plugin commands through the REPL path, not by bypassing it.

Typical commands:

```powershell
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 process-clean"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh repl src_v3 test"
wsl-tmux "cd /home/user/dialtone && ./dialtone.sh ssh src_v1 probe --host grey --timeout 5s"
```

For the REPL suite:

- `process-clean` before reruns is often necessary.
- A fresh tmux session can be cleaner than trying to recover a stuck pane.
- The generated reports live at:
  - `/home/user/dialtone/src/plugins/repl/src_v3/TEST.md`
  - `/home/user/dialtone/src/plugins/repl/src_v3/TEST_RAW.md`
  - `/home/user/dialtone/src/plugins/repl/src_v3/ERRORS.md`

## SSH / Mesh Notes

- Mesh SSH was configured to work without passing usernames/passwords on the command line when `env/dialtone.json` has the right node config.
- The REPL SSH test path may need a reachable default node. In this environment, preferring `grey` worked better than defaulting to `wsl`.
- If a WSL-local SSH test needs loopback, use `127.0.0.1` explicitly instead of assuming a tailnet hostname will resolve and accept auth from inside WSL.

## Config Notes

Use `env/dialtone.json` as the main config source.
Do not create accidental config copies under `src/env/`.

If bootstrap tests need temp config propagation, make sure the bootstrap path copies through required fields like:

- `TS_AUTHKEY`
- `TS_API_KEY`
- `TS_TAILNET`
- `CLOUDFLARE_API_TOKEN`
- `CLOUDFLARE_ACCOUNT_ID`
- `DIALTONE_DOMAIN`

## Safe Sync Pattern

If both repos changed:

1. Commit and push the Windows repo with native Windows Git.
2. Rebase the WSL repo onto the new `origin/main`.
3. Re-run WSL tests.
4. Commit and push the WSL repo.
5. Fast-forward the Windows repo again if needed.

This avoids mixing Windows line-ending churn with real Linux/runtime changes.
