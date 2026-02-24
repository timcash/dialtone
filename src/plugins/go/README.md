# Go Plugin

The Go plugin manages the managed Go toolchain and Go-related workflows through `./dialtone.sh go src_v1 ...`.

## Commands

```bash
./dialtone.sh go src_v1 install
./dialtone.sh go src_v1 lint
./dialtone.sh go src_v1 exec <go-args...>
./dialtone.sh go src_v1 run <go-args...>      # alias for exec
./dialtone.sh go src_v1 pb-dump <file.pb>
./dialtone.sh go src_v1 test
```

## Usage

### Install managed Go toolchain

```bash
./dialtone.sh go src_v1 install
```

Installs Go into `DIALTONE_ENV/go`.

### Run lint

```bash
./dialtone.sh go src_v1 lint
```

Runs `go vet ./...` using the managed toolchain when available.

### Run arbitrary Go commands

```bash
./dialtone.sh go src_v1 exec run ./src/cmd/dev/main.go
./dialtone.sh go src_v1 exec build ./src/...
./dialtone.sh go src_v1 exec build ./plugins/repl/src_v1/cmd/repld
```

`go run` is an alias:

```bash
./dialtone.sh go src_v1 run ./src/cmd/dev/main.go
```

`go build` auto-output behavior:
- If you run `./dialtone.sh go src_v1 exec build <single-target>` without `-o`,
  Dialtone writes binaries under gitignored `.dialtone/bin/`:
  - plugin targets: `.dialtone/bin/plugins/<plugin>/<src_vN>/<binary>`
  - `dev.go`: `.dialtone/bin/dev/dev`
  - other single targets: `.dialtone/bin/misc/<binary>`
- If `-o` is provided, your explicit output path is preserved.

### Inspect protobuf binaries

```bash
./dialtone.sh go src_v1 pb-dump path/to/file.pb
```

## Testing

Run Go plugin integration tests:

```bash
./dialtone.sh go src_v1 test
```

Current test coverage verifies:
- stdout from child commands is visible through `./dialtone.sh`
- stderr output and non-zero failure details propagate through `./dialtone.sh`

## Notes

- The plugin relies on `DIALTONE_ENV` to locate managed toolchains.
- If Go is missing, run `./dialtone.sh install` or `./dialtone.sh go install`.
