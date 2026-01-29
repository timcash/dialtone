# Compilation Issues Summary

## Goal

We want `./dialtone.sh install` to automatically download and install a minimal C toolchain into the `DIALTONE_ENV` directory (e.g., `./dialtone_dependencies/`) **without requiring sudo**.

This would make the project fully self-contained and portable.

---

## Open Question

**Is there a smaller, self-contained C/C++ toolchain we can bundle for DuckDB's CGO requirements?**

Potential options to investigate:
- **musl-cross-make** - Minimal GCC cross-compiler toolchains
- **cosmopolitan libc** - Single-file C toolchain
- **tinycc (TCC)** - Tiny C Compiler (~100KB)
- **Prebuilt GCC sysroot** - Minimal GCC with just the needed libs
- **DuckDB with different build flags** - Maybe a musl-compatible build?

If anyone knows of a ~50-100MB self-contained GCC/Clang bundle that includes `libstdc++` and works on Linux x86_64, please share!

---

## The Problem

The `dialtone` project uses **DuckDB** (via `github.com/marcboeker/go-duckdb`) for the ticket plugin's database storage. DuckDB's Go bindings require **CGO**, which means a C compiler is needed to build the project.

### Error Without C Compiler

```
# github.com/marcboeker/go-duckdb
transaction.go:6:5: undefined: Conn
```

This error occurs because Go can't compile the CGO bridge code without a C compiler.

## Why This Is Difficult

We explored several approaches to avoid requiring `sudo apt-get install`:

### 1. Zig as C Compiler ❌
- Zig is already installed by the install plugin
- **Problem**: DuckDB's prebuilt `libduckdb.a` was compiled with GCC and links against `libstdc++`
- Zig's `lld` linker cannot properly resolve GCC's C++ standard library symbols
- **Error**: `undefined symbol: std::basic_streambuf<char, std::char_traits<char>>::...`

### 2. Prebuilt LLVM/Clang ❌
- Downloaded LLVM 18.x and 19.x releases
- **Problem 1**: Older LLVM needs `libtinfo.so.5` (system has `libtinfo.so.6`)
- **Problem 2**: Newer LLVM needs `stdlib.h` headers (requires `libc-dev`)
- Prebuilt LLVM has system library dependencies that require sudo to install

### 3. Symlink Workarounds ❌
- Tried symlinking `libtinfo.so.6` → `libtinfo.so.5`
- **Problem**: ABI incompatibility - different symbol versions

## The Solution

**DuckDB requires a proper GCC/Clang installation with C++ standard library support.**

### One-Time Setup (Requires sudo)

```bash
sudo apt-get update && sudo apt-get install -y build-essential
```

This installs:
- `gcc` - GNU C Compiler
- `g++` - GNU C++ Compiler  
- `libc-dev` - C library headers
- `libstdc++` - C++ standard library

After this, `./dialtone.sh install` works correctly.

## Alternative: Remove DuckDB Dependency

If sudo access is truly not available, the ticket plugin could be modified to use:

1. **`modernc.org/sqlite`** - Pure Go SQLite implementation (no CGO)
2. **File-based storage** - JSON/YAML files instead of a database
3. **System DuckDB CLI** - Use DuckDB as external process instead of embedding

## Files Involved

- `src/plugins/ticket/cli/storage.go` - Uses `go-duckdb`
- `src/plugins/ticket/test/integration.go` - Uses `go-duckdb`
- `go.mod` - Declares `github.com/marcboeker/go-duckdb` dependency

## Current Status

The `dialtone.sh` script:
1. Warns if no C compiler is found
2. Provides installation instructions
3. Sets `CGO_ENABLED=0` if no compiler (but DuckDB still fails to compile)

The project **requires** `build-essential` to be installed for full functionality.
