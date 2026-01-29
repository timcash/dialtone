# Go Plugin

The Go plugin manages the Go toolchain and provides Go-specific utilities.

## Features
- **Toolchain Management**: Install a specific Go version into the `DIALTONE_ENV` directory.
- **Linting**: Run `go vet` using the isolated Go toolchain.
- **Command Execution**: Run arbitrary `go` commands with the isolated toolchain.
- **Protobuf Inspection**: Inspect protobuf files via the bundled pb-dump tool.

## Usage

### Install Go
Installs the Go toolchain (default: 1.25.5) into your configured dependency directory.
```bash
./dialtone.sh go install
```

### Lint Code
Runs `go vet ./...` using the Go binary in `DIALTONE_ENV`.
```bash
./dialtone.sh go lint
```

### Run Go Commands
Runs arbitrary Go commands using the toolchain in `DIALTONE_ENV`.
```bash
./dialtone.sh go exec run ./path/to/main.go
```

### Run Go (Alias)
Alias for `exec` that uses the same isolated toolchain.
```bash
./dialtone.sh go run ./path/to/main.go
```

### Inspect Protobuf Files
Uses the bundled pb-dump tool to print protobuf structure/strings.
```bash
./dialtone.sh go pb-dump path/to/file.pb
```

## Configuration
The plugin uses `DIALTONE_ENV` from your `.env` file to determine where to install Go.
