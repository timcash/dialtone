# Install Commands

## Command Line Help
```shell
./dialtone.sh install --help
```

## List Dependencies
Prints every dependency the installer knows about for the current platform, the download URL, cached download location, size, and install status.
```shell
./dialtone.sh install list
./dialtone.sh --env test.env install list
```

## Install One Dependency
Installs a single dependency by name (as shown in `install list`).
```shell
./dialtone.sh install dependency zig
./dialtone.sh --env test.env install dependency go
```

## Full Install
```shell
./dialtone.sh install
```

## Clean Install
Removes the existing dependency directory before reinstalling.
```shell
./dialtone.sh install --clean
./dialtone.sh --env test.env install --clean
```

## Clean Cache
Removes cached downloads before running install.
```shell
./dialtone.sh install --clean-cache
./dialtone.sh --env test.env install --clean-cache
```

## Cache Location
The installer uses `DIALTONE_CACHE` to store downloaded archives and binaries.
```shell
export DIALTONE_CACHE=./dialtone_cache
```