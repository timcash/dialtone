# Embedded SQLite + GraphDB

This directory outlines how to build a 100% standalone, statically-compiled C binary that contains SQLite and the Cypher graph extension.

## Directory Structure Needed

To compile `main.c`, you will need to fetch the dependencies into this folder so they can be compiled together:

1.  **SQLite Amalgamation** (`sqlite3.c` and `sqlite3.h`)
2.  **sqlite-graph source code** (`agentflare-ai/sqlite-graph`)

You can fetch them via shell commands:

```sh
# 1. Fetch SQLite Amalgamation (Version 3.45.1 used as an example)
wget https://www.sqlite.org/2024/sqlite-amalgamation-3450100.zip
unzip sqlite-amalgamation-3450100.zip
mv sqlite-amalgamation-3450100/sqlite3.c .
mv sqlite-amalgamation-3450100/sqlite3.h .
rm -rf sqlite-amalgamation-3450100*

# 2. Fetch sqlite-graph source
git clone https://github.com/agentflare-ai/sqlite-graph.git
```

## Compilation

You can compile this statically so it requires no external `.so` libraries. It embeds everything directly into `dialtone_db`.

```sh
# Compile everything together statically
gcc -static -Os \
    -I. -I./sqlite-graph/include \
    main.c \
    sqlite3.c \
    ./sqlite-graph/src/*.c \
    -o dialtone_db \
    -lpthread -lm -DSQLITE_OMIT_LOAD_EXTENSION
```

> *Note: If you run into issues with `glibc` static linking on Linux, try using `musl-gcc` or building inside an Alpine Linux container.*

## Running

```sh
./dialtone_db
```
