# Embedded Graph Database (SQLite + Cypher)

This module provides a 100% standalone, statically-compiled binary that embeds a full SQLite database alongside a powerful Graph Database extension. 

There are no external dependencies, no `.so` or `.dylib` files required, and no complex deployment steps. Everything is compiled into a single executable (`dialtone_db`) using Zig.

## What is inside?

1.  **SQLite 3**: The core database engine (compiled from the official C amalgamation).
2.  **sqlite-graph**: A pure C99 extension (`agentflare-ai/sqlite-graph`) that adds native graph database capabilities to SQLite.
3.  **Zig CLI Wrapper**: A minimal, memory-safe CLI (`main.zig`) that statically links the C libraries, initializes the graph extension in memory, and provides an interactive REPL and command execution interface.

### About `sqlite-graph`
This plugin allows us to use SQLite as a proper graph database without the overhead of heavy solutions like Neo4j. It provides:
*   **Cypher Query Support**: Execute standard pattern-matching queries like `MATCH (a)-[:LINK]->(b) RETURN a`.
*   **Property Graphs**: Nodes and edges store properties as JSON strings, allowing schema-less attachments like `{"latency": 10}`.
*   **Virtual Tables**: The graph is exposed as standard SQLite virtual tables (`mygraph_nodes`, `mygraph_edges`), allowing you to mix graph traversals with standard SQL `JOIN`s.

#### Built-in Algorithms
The extension includes high-performance graph algorithms written directly in C:
*   **Dijkstra's Shortest Path**: Finds the lowest cost path based on edge weights. *(Note: We patched `graph-algo.c` slightly so Dijkstra automatically respects `latency` or `weight` keys in JSON edge properties!)*
*   **Breadth-First Search (BFS)**: Fast shortest-path routing for unweighted networks.
*   **PageRank**: Calculates node importance based on relationship connectivity.
*   **Centrality (Degree/Closeness/Betweenness)**: Determines the most critical bottlenecks or highly connected hubs in the graph.

## End-to-End Instructions

### 1. Building the Binary
We use Zig as a C compiler and build system. Zig perfectly handles the C-compilation of SQLite and the graph extension, linking them together with `libc`.

To build the project:
```sh
# Ensure you are in the module directory
cd src/mods/db/v1

# Use the wrapper script to compile the standalone binary
# (This ensures the .zig-cache is placed outside the repo)
./build.sh
```
*(If you do not have Zig installed globally, you can use Nix: `nix-shell -p zig --run "./build.sh"`)*

This compiles the binary and places it in the repository root at: `bin/mods/db/v1/dialtone_db`

### 2. Running the Interactive REPL
If you run the binary with no arguments, it boots an in-memory database, automatically seeds it with a **Computer Networking Topology**, and drops you into a REPL.

```sh
../../../../bin/mods/db/v1/dialtone_db
```

**Example Session:**
```
Dialtone DB (SQLite + Graph).
Network nodes (1=A, 2=B, 3=C, 4=D) and edges with 'latency' loaded.
Shortest path from A(1) to C(3) should prefer the low-latency 4ms path over the 15ms path.
Use .exit or .quit to exit.
graph_shortest_path_weighted(1, 3) = {"path":[1,4,3], "distance":4.000000}
dialtone> SELECT cypher_execute('MATCH (r:Router) RETURN r');
```

### 3. Running Single Commands
If you want to execute a query script automatically (e.g., from another Dialtone mod or shell script), use the `-c` flag.

**Important:** If no database file is provided, each command runs against a fresh, empty in-memory (`:memory:`) database and destroys it upon exit. It **does not** include the default network topology seed data that the REPL uses.

```sh
# Example: Creating a new node in an ephemeral database (data is lost immediately)
../../../../bin/mods/db/v1/dialtone_db -c "SELECT cypher_execute('CREATE (r:Router {name: \"Core-1\", ip: \"10.0.0.1\"})');"

# Example: Running the latency-aware shortest path algorithm (will return empty/failure on a fresh DB)
../../../../bin/mods/db/v1/dialtone_db -c "SELECT graph_shortest_path_weighted(1, 3);"
```

To persist data, pass a file path as the first positional argument before the `-c` flag:

```sh
# Example: Persisting state across commands
../../../../bin/mods/db/v1/dialtone_db my_graph.db -c "SELECT cypher_execute('CREATE (r:Router {name: \"Core-1\", ip: \"10.0.0.1\"})');"
../../../../bin/mods/db/v1/dialtone_db my_graph.db -c "SELECT cypher_execute('MATCH (r:Router) RETURN r');"
```

## How the Code Works

*   **`build.zig`**: The build script. It explicitly lists `sqlite3.c` and the 35+ C files inside `sqlite-graph/src/**/*.c`. It instructs Zig's internal Clang compiler to compile them all together with `-DSQLITE_CORE` and `-DSQLITE_ENABLE_LOAD_EXTENSION`.
*   **`main.zig`**: The entry point.
    1.  Uses `sqlite3_auto_extension()` to statically inject `sqlite3_graph_init` into the SQLite runtime.
    2.  Opens the database (in-memory `":memory:"` by default, but you can pass a file path like `my_graph.db`).
    3.  Runs the `CREATE VIRTUAL TABLE` setup.
    4.  Manages the standard input/output loop for the CLI.
*   **`patch_dijkstra`**: In `sqlite-graph/src/graph-algo.c`, we modified the SQL query driving Dijkstra's algorithm to do a dynamic `COALESCE` on the edge's JSON properties. It looks for `weight`, then `latency`, defaulting to `1.0`. We also exposed this via `graphShortestPathWeightedFunc` in `graph.c`.

## Notes for AI Agents

When working with this module, please adhere to the following constraints and gotchas:

1.  **Statefulness:** Single commands (`-c`) run against an empty `:memory:` database unless a file path is provided as the first positional argument (e.g., `dialtone_db my_graph.db -c "..."`). State is not saved unless a file is specified.
2.  **Seed Data:** The "Computer Networking Topology" seed is ONLY injected when starting the interactive REPL with no arguments. It is not available when using `-c` or when providing a database file.
3.  **Build Constraints:** Never run `zig build` directly. Always use `./build.sh`. The wrapper script explicitly redirects the local and global Zig caches (`ZIG_LOCAL_CACHE_DIR`, `ZIG_GLOBAL_CACHE_DIR`) outside the repository (e.g., to `/tmp/`) to prevent massive binary caches from polluting the workspace and overflowing your context window.
