# Go Plugin

The Go plugin manages the managed Go toolchain and Go-related workflows through `./dialtone.sh go ...`.

## Commands

```bash
./dialtone.sh go install
./dialtone.sh go lint
./dialtone.sh go exec <go-args...>
./dialtone.sh go run <go-args...>      # alias for exec
./dialtone.sh go pb-dump <file.pb>
./dialtone.sh go test
```

## Usage

### Install managed Go toolchain

```bash
./dialtone.sh go install
```

Installs Go into `DIALTONE_ENV/go`.

### Run lint

```bash
./dialtone.sh go lint
```

Runs `go vet ./...` using the managed toolchain when available.

### Run arbitrary Go commands

```bash
./dialtone.sh go exec run ./src/cmd/dev/main.go
./dialtone.sh go exec build ./src/...
```

`go run` is an alias:

```bash
./dialtone.sh go run ./src/cmd/dev/main.go
```

### Inspect protobuf binaries

```bash
./dialtone.sh go pb-dump path/to/file.pb
```

## Testing

Run Go plugin integration tests:

```bash
./dialtone.sh go test
```

Current test coverage verifies:
- stdout from child commands is visible through `./dialtone.sh`
- stderr output and non-zero failure details propagate through `./dialtone.sh`

## Notes

- The plugin relies on `DIALTONE_ENV` to locate managed toolchains.
- If Go is missing, run `./dialtone.sh install` or `./dialtone.sh go install`.
