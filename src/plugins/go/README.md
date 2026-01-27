# Go Plugin

The Go plugin manages the Go toolchain and provides Go-specific utilities.

## Features
- **Toolchain Management**: Install a specific Go version into the `DIALTONE_ENV` directory.
- **Linting**: Run `go vet` using the isolated Go toolchain.

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

## Configuration
The plugin uses `DIALTONE_ENV` from your `.env` file to determine where to install Go.
