# Plugin Guide

This directory contains Dialtone plugins.

The goal is not just "a command that works." A good plugin should match the same shape as the core plugins, compose the shared infrastructure, and feel natural from both `dialtone.sh` and the REPL.

## The Core Reference Set

These plugins define the patterns new plugins should copy:

| Plugin | What it teaches |
| --- | --- |
| [`repl src_v3`](repl/src_v3/README.md) | Task-first control plane, services, task logs, NATS-backed state |
| [`logs src_v1`](logs/src_v1/README.md) | Structured logging over NATS |
| [`test src_v1`](test/src_v1/README.md) | Shared suite runner, StepContext, `TEST.md` generation |
| [`chrome src_v3`](chrome/src_v3/README.md) | Service-backed browser automation and remote Windows browser control |
| [`ui src_v1`](ui/src_v1/README.md) | Shared UI shell, templates, and fixture browser tests |
| [`ssh src_v1`](ssh/src_v1/README.md) | Mesh host resolution, remote execution, and code sync |
| [`cad src_v1`](cad/src_v1/README.md) | Small full-stack reference plugin |
| [`robot src_v2`](robot/src_v2/README.md) | Large integrated reference plugin |

Use `cad src_v1` when you want the smallest realistic end-to-end example.

Use `robot src_v2` when you want the biggest example of how the same patterns scale into a full system.

## What An Effective Plugin Looks Like

An effective Dialtone plugin has these traits:

1. One public command surface: `./dialtone.sh <plugin> <src_vN> <command>`.
2. The same command can also be used inside the REPL as `/plugin src_vN <command>`.
3. `scaffold/main.go` stays thin and delegates to real code in `src_vN`.
4. Runtime, paths, and env come from `config src_v1` and `env/dialtone.json`.
5. Logs go through `logs src_v1`, not ad hoc `fmt.Print*`.
6. Tests go through `test src_v1`, not custom report generators and custom mini frameworks.
7. Long-lived processes are treated as services through `repl src_v3`.
8. Browser work reuses `chrome src_v3`.
9. Remote host work reuses `ssh src_v1`.
10. UI work starts with `ui src_v1`.
11. Docs stay aligned with the actual command surface.

## Standard Layout

```text
src/plugins/<plugin>/
  README.md
  scaffold/
    main.go
  src_v1/
    README.md
    go/
    cmd/
    ui/
    test/
      cmd/
        main.go
      01_.../
      02_.../
```

Guidelines:

- Keep `scaffold/main.go` as a router, not a second implementation.
- Put the real implementation in `src_vN`.
- If a plugin has a UI, keep it under `src_vN/ui`.
- If a plugin has tests, keep one `src_vN/test/cmd/main.go` orchestrator and suite steps under `src_vN/test/<step>/`.
- Version new behavior with `src_vN`; do not silently mutate old versions into something incompatible.

## Command Contract

The normal public shape is:

```bash
./dialtone.sh <plugin> <src_vN> <command> [args] [--flags]
```

Most effective plugins expose these commands when they make sense:

```bash
./dialtone.sh <plugin> <src_vN> help
./dialtone.sh <plugin> <src_vN> install
./dialtone.sh <plugin> <src_vN> format
./dialtone.sh <plugin> <src_vN> lint
./dialtone.sh <plugin> <src_vN> build
./dialtone.sh <plugin> <src_vN> test
./dialtone.sh <plugin> <src_vN> test --filter <expr>
```

Command rules:

- Use one command per invocation.
- Use `--flags` for optional behavior.
- Do not hide core behavior behind ambient shell variables.
- If the plugin exposes `test --filter`, make sure the scaffold forwards extra CLI args to the real `src_vN/test/cmd/main.go` runner.

## Runtime And Config Contract

Use the config plugin instead of hardcoding paths or relying on the current working directory.

Standard Go pattern:

```go
rt, err := configv1.ResolveRuntime("")
if err != nil {
    return err
}
if err := configv1.LoadEnvFile(rt); err != nil {
    return err
}
if err := configv1.ApplyRuntimeEnv(rt); err != nil {
    return err
}

preset := configv1.NewPluginPreset(rt, "my-plugin", "src_v1")
```

Rules:

- `env/dialtone.json` is the shared runtime config source.
- Do not hardcode `src/plugins/...` path joins everywhere.
- Do not add plugin-specific `.env` files for normal runtime config.
- Resolve runtime once, then derive paths from a preset or local path helper.

## Logging, Tasks, And Services

Match the core control-plane model:

- Normal commands are submitted through `dialtone.sh` and become REPL-managed tasks.
- Long-lived processes should be modeled as services.
- `dialtone>` output stays short and lifecycle-oriented.
- Detailed logs belong in task logs, daemon logs, and NATS log streams.

Rules:

- Use `logs src_v1` for operational output.
- Use `repl src_v3` task and service semantics for long-lived work.
- Use foreground output only for explicit query or operator commands.
- If the plugin owns a service, it should expose a clear `service start|stop|status` style lifecycle.

## Browser, UI, And Remote Host Composition

Do not rebuild the same infrastructure in every plugin.

If your plugin needs:

- a browser session: use `chrome src_v3`
- a UI shell or shared frontend patterns: use `ui src_v1`
- remote mesh execution or sync: use `ssh src_v1`
- path and env resolution: use `config src_v1`

Good examples:

- `cad src_v1` uses a small Go service, a UI, and a browser smoke through the shared stack.
- `robot src_v2` composes browser, UI, remote host execution, artifacts, and REPL tasks at larger scale.

## Test Contract

Use `test src_v1` as the shared harness.

That means:

- one suite orchestrator in `src_vN/test/cmd/main.go`
- `testv1.NewRegistry()` plus registered steps
- `StepContext` for logging, waits, browser actions, screenshots, and reports
- `TEST.md` and `TEST_RAW.md` generated by the shared test layer

Prefer this pattern:

```go
reg := testv1.NewRegistry()
mysetup.Register(reg)
mybrowser.Register(reg)
return reg.Run(testv1.SuiteOptions{
    Version:       "my-plugin-src-v1",
    ReportPath:    "plugins/my-plugin/src_v1/TEST.md",
    RawReportPath: "plugins/my-plugin/src_v1/TEST_RAW.md",
    ReportFormat:  "template",
    ReportRunner:  "test/src_v1",
    NATSURL:       "nats://127.0.0.1:4222",
    NATSSubject:   "logs.test.my-plugin-src-v1",
    AutoStartNATS: true,
})
```

## Anti-Patterns

Avoid these:

- using raw `go`, `bun`, `vite`, `ssh`, or browser launch commands as the public plugin workflow
- launching independent browsers directly from every plugin instead of reusing `chrome src_v3`
- reimplementing SSH or mesh logic instead of using `ssh src_v1`
- custom path walkers and `os.Getwd()` repo discovery in many files
- direct `fmt.Print*` logging for operational output
- plugin-specific hidden env variables for normal options that should be flags
- test scaffolds that swallow extra args and break `--filter`
- giant scaffolds that duplicate the real implementation

## Minimal Checklist For A New Plugin

- Add `scaffold/main.go`.
- Add `src_vN`.
- Implement `help`, `install`, `format`, `lint`, `build`, and `test` where meaningful.
- Resolve runtime through `config src_v1`.
- Log through `logs src_v1`.
- Use `test src_v1` for the suite.
- Reuse `chrome src_v3`, `ui src_v1`, and `ssh src_v1` instead of cloning their jobs.
- Write a version-specific README that matches the real commands and behavior.
