# LLM Prompt: Finish chrome src_v3

You are working in `/home/user/dialtone`.

The target is `src/plugins/chrome/src_v3`.

Goal:

- build a very small Chrome service
- manage one Chrome browser instance
- manage one fixed profile
- keep one browser-level `chromedp` allocator connection open
- track one explicit `managedTargetID`
- expose only NATS request/reply commands
- stop all hidden compatibility behavior

Do not preserve old `src_v1` behavior.

## Architecture Required

The final service must do only this:

1. start daemon
2. launch or attach one Chrome browser
3. keep one browser allocator connection open for the daemon lifetime
4. choose one explicit page target as `managedTargetID`
5. commands operate via NATS request/reply only
6. each command may create a short-lived child context bound to `managedTargetID`
7. if the allocator connection drops unexpectedly, mark the daemon unhealthy
8. do not silently relaunch new browsers if a matching one already exists but is unhealthy

## Current Files

- `src/plugins/chrome/src_v3/main.go`
- `src/plugins/chrome/src_v3/README.md`
- `src/plugins/chrome/scaffold/main.go`

## Current Verified Commands

These are currently the only commands considered verified:

```bash
./dialtone.sh chrome src_v3 build
./dialtone.sh chrome src_v3 deploy --host legion --service --role dev
./dialtone.sh chrome src_v3 status --host legion
./dialtone.sh chrome src_v3 doctor --host legion
./dialtone.sh chrome src_v3 logs --host legion --lines 80
./dialtone.sh chrome src_v3 reset --host legion
```

## Broken / Incomplete

These are not yet trusted:

```bash
./dialtone.sh chrome src_v3 open --host legion --url https://example.com
./dialtone.sh chrome src_v3 goto --host legion --url https://dialtone.earth
./dialtone.sh chrome src_v3 get-url --host legion
./dialtone.sh chrome src_v3 tabs --host legion
./dialtone.sh chrome src_v3 tab-open --host legion --url https://example.com/?tab=2
./dialtone.sh chrome src_v3 tab-close --host legion --index 1
./dialtone.sh chrome src_v3 close --host legion
```

## Known Issue

On Windows, the short-lived Chrome launcher PID is not the real browser owner.

The service must track:

- the actual browser process behind the remote debug port
- not the transient launcher process

The service should be considered healthy if:

- the daemon is alive
- the allocator connection is alive
- the debug port is reachable
- the managed target exists

## Required Cleanup

Remove code paths that do any of the following:

- launch extra browsers because reuse was ambiguous
- rediscover a “first page tab” on every action
- hide failures by auto-retrying into a new browser
- depend on old scheduled tasks
- depend on old HTTP control APIs for action commands

The only control plane should be NATS request/reply.

## Required Tests

Use `./dialtone.sh` for all build/test commands.

Minimum required verification:

```bash
./dialtone.sh go src_v1 exec test -vet=off ./plugins/chrome/src_v3 ./plugins/chrome/scaffold
./dialtone.sh chrome src_v3 reset --host legion
./dialtone.sh chrome src_v3 deploy --host legion --service --role dev
./dialtone.sh chrome src_v3 status --host legion
./dialtone.sh chrome src_v3 doctor --host legion
./dialtone.sh chrome src_v3 open --host legion --url https://example.com/?dialtone=open
./dialtone.sh chrome src_v3 get-url --host legion
./dialtone.sh chrome src_v3 goto --host legion --url https://example.com/?dialtone=goto
./dialtone.sh chrome src_v3 tab-open --host legion --url https://example.com/?dialtone=tab-open
./dialtone.sh chrome src_v3 tabs --host legion
./dialtone.sh chrome src_v3 tab-close --host legion
./dialtone.sh chrome src_v3 close --host legion
```

Expected end state:

- one `dialtone_chrome_v3.exe` daemon
- one Chrome browser instance for role `dev`
- one fixed profile
- no extra scheduled task
- no repeated browser reopen loop
- all action commands return promptly

## Deliverable

When done:

1. update `src/plugins/chrome/src_v3/README.md`
2. show exact commands run
3. report whether `open/goto/get-url/tab-open/tab-close/close` were validated on `legion`
