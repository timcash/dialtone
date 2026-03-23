const std = @import("std");
const c = @cImport({
    @cInclude("sqlite3.h");
});

extern fn sqlite3_graph_init(
    db: ?*c.sqlite3,
    pzErrMsg: [*c][*c]u8,
    pApi: ?*const c.sqlite3_api_routines,
) callconv(.c) c_int;

fn printCallback(NotUsed: ?*anyopaque, argc: c_int, argv: [*c][*c]u8, azColName: [*c][*c]u8) callconv(.c) c_int {
    _ = NotUsed;
    var stdout_buffer: [4096]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
    const stdout = &stdout_writer.interface;

    var i: usize = 0;
    while (i < @as(usize, @intCast(argc))) : (i += 1) {
        const col_name = if (azColName[i] != null) std.mem.span(@as([*:0]const u8, @ptrCast(azColName[i]))) else "NULL";
        const val = if (argv[i] != null) std.mem.span(@as([*:0]const u8, @ptrCast(argv[i]))) else "NULL";
        stdout.print("{s} = {s}\n", .{ col_name, val }) catch {};
    }
    stdout.flush() catch {};
    return 0;
}

pub fn main() !void {
    var arena_state = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena_state.deinit();
    const allocator = arena_state.allocator();

    const args = try std.process.argsAlloc(allocator);

    var db: ?*c.sqlite3 = null;
    var err_msg: [*c]u8 = null;
    var rc: c_int = 0;

    rc = c.sqlite3_auto_extension(@ptrCast(&sqlite3_graph_init));
    if (rc != c.SQLITE_OK) {
        std.debug.print("Failed to register static graph extension.\n", .{});
        std.process.exit(1);
    }

    var db_path: [*c]const u8 = ":memory:";
    var command_to_run: ?[:0]const u8 = null;
    var run_benchmark: bool = false;

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        if (std.mem.eql(u8, args[i], "-h") or std.mem.eql(u8, args[i], "--help")) {
            std.debug.print(
                \\Dialtone DB (SQLite + Graph)
                \\
                \\Usage:
                \\  dialtone_db [database_file] [-c "query"]
                \\  dialtone_db [-h | --help]
                \\  dialtone_db [--benchmark]
                \\
                \\Options:
                \\  [database_file]   Path to SQLite database file. Defaults to ':memory:'
                \\  -c "query"        Run a single query and exit.
                \\  --benchmark       Run the built-in performance benchmark.
                \\  -h, --help        Show this help message.
                \\
                \\Examples:
                \\  dialtone_db                             # Start interactive REPL in memory
                \\  dialtone_db my_graph.db                 # Start interactive REPL with file
                \\  dialtone_db -c "SELECT 1;"              # Run query against ephemeral memory DB
                \\  dialtone_db my_graph.db -c "SELECT 1;"  # Run query against file DB
                \\  dialtone_db --benchmark                 # Run performance benchmark
                \\
            , .{});
            std.process.exit(0);
        } else if (std.mem.eql(u8, args[i], "--benchmark")) {
            run_benchmark = true;
        } else if (std.mem.eql(u8, args[i], "-c")) {
            if (i + 1 < args.len) {
                command_to_run = try allocator.dupeZ(u8, args[i + 1]);
                i += 1;
            }
        } else {
            db_path = try allocator.dupeZ(u8, args[i]);
        }
    }

    rc = c.sqlite3_open(db_path, &db);
    if (rc != c.SQLITE_OK) {
        std.debug.print("Cannot open database\n", .{});
        std.process.exit(1);
    }
    defer _ = c.sqlite3_close(db);

    rc = c.sqlite3_exec(db, "CREATE VIRTUAL TABLE IF NOT EXISTS mygraph USING graph()", null, null, &err_msg);
    if (rc != c.SQLITE_OK) {
        std.debug.print("Failed to create graph table: {s}\n", .{err_msg});
        c.sqlite3_free(err_msg);
        std.process.exit(1);
    }

    if (run_benchmark) {
        var timer = try std.time.Timer.start();
        
        var stdout_buffer: [4096]u8 = undefined;
        var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
        const stdout = &stdout_writer.interface;

        try stdout.print("Generating realistic network topology (10,000 nodes)...\n", .{});
        try stdout.flush();

        rc = c.sqlite3_exec(db, "BEGIN TRANSACTION;", null, null, &err_msg);
        if (rc != c.SQLITE_OK) { std.debug.print("Error: {s}\n", .{err_msg}); std.process.exit(1); }
        
        var sql_buf: [1024]u8 = undefined;
        
        // 1. Generate 100 Routers
        var idx: usize = 1;
        while (idx <= 100) : (idx += 1) {
            const status = if (idx % 10 == 0) "offline" else "online";
            const sql = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_node_add({d}, '{{\"type\": \"Router\", \"name\": \"Router-{d}\", \"status\": \"{s}\"}}');", .{ idx, idx, status });
            rc = c.sqlite3_exec(db, sql.ptr, null, null, &err_msg);
        }

        // 2. Generate 1,000 Hosts connected to Routers
        idx = 101;
        while (idx <= 1100) : (idx += 1) {
            const status = if (idx % 15 == 0) "maintenance" else "active";
            const sql = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_node_add({d}, '{{\"type\": \"Host\", \"name\": \"Host-{d}\", \"status\": \"{s}\"}}');", .{ idx, idx, status });
            rc = c.sqlite3_exec(db, sql.ptr, null, null, &err_msg);
        }

        // 3. Generate 8,900 Processes running on Hosts
        idx = 1101;
        while (idx <= 10000) : (idx += 1) {
            const status = if (idx % 25 == 0) "failed" else "running";
            const sql = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_node_add({d}, '{{\"type\": \"Process\", \"name\": \"Task-{d}\", \"status\": \"{s}\"}}');", .{ idx, idx, status });
            rc = c.sqlite3_exec(db, sql.ptr, null, null, &err_msg);
        }

        // 4. Edges: Router <-> Router (Latency/Weight)
        idx = 1;
        while (idx < 100) : (idx += 1) {
            const sql1 = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_edge_add({d}, {d}, 'LINK', '{{\"latency\": 1}}');", .{ idx, idx + 1 });
            rc = c.sqlite3_exec(db, sql1.ptr, null, null, &err_msg);
            if (idx + 5 <= 100) {
                const sql2 = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_edge_add({d}, {d}, 'LINK', '{{\"latency\": 5}}');", .{ idx, idx + 5 });
                rc = c.sqlite3_exec(db, sql2.ptr, null, null, &err_msg);
            }
        }

        // 5. Edges: Host -> Router
        idx = 101;
        while (idx <= 1100) : (idx += 1) {
            const router_id = 1 + (idx % 100); // Distribute across routers
            const sql = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_edge_add({d}, {d}, 'CONNECTED_TO', '{{\"latency\": 2}}');", .{ idx, router_id });
            rc = c.sqlite3_exec(db, sql.ptr, null, null, &err_msg);
        }

        // 6. Edges: Process -> Host
        idx = 1101;
        while (idx <= 10000) : (idx += 1) {
            const host_id = 101 + (idx % 1000); // Distribute processes across hosts
            const sql = try std.fmt.bufPrintZ(&sql_buf, "SELECT graph_edge_add({d}, {d}, 'RUNS_ON', '{{\"cpu_usage\": {d}, \"memory_usage\": {d}}}');", .{ idx, host_id, (idx % 100), (idx % 500) });
            rc = c.sqlite3_exec(db, sql.ptr, null, null, &err_msg);
        }

        rc = c.sqlite3_exec(db, "COMMIT;", null, null, &err_msg);
        if (rc != c.SQLITE_OK) { std.debug.print("Error: {s}\n", .{err_msg}); std.process.exit(1); }

        const insert_time = @as(f64, @floatFromInt(timer.read())) / @as(f64, @floatFromInt(std.time.ns_per_s));
        try stdout.print("1. Bulk Insert Heterogeneous Nodes/Edges: {d:.4} seconds\n", .{insert_time});
        try stdout.flush();

        timer.reset();
        rc = c.sqlite3_exec(db, "SELECT graph_shortest_path_weighted(101, 1100);", null, null, &err_msg);
        if (rc != c.SQLITE_OK) { std.debug.print("SP Error: {s}\n", .{err_msg}); c.sqlite3_free(err_msg); }
        const sp_time = @as(f64, @floatFromInt(timer.read())) / @as(f64, @floatFromInt(std.time.ns_per_s));
        try stdout.print("2. Shortest Path (Weighted) across network (Host -> Host): {d:.4} seconds\n", .{sp_time});
        try stdout.flush();
        
        timer.reset();
        rc = c.sqlite3_exec(db, "SELECT count(*) FROM mygraph_nodes WHERE json_extract(properties, '$.type') = 'Process' AND json_extract(properties, '$.status') = 'failed';", null, null, &err_msg);
        if (rc != c.SQLITE_OK) { std.debug.print("SQL JSON Error: {s}\n", .{err_msg}); c.sqlite3_free(err_msg); }
        const json_time = @as(f64, @floatFromInt(timer.read())) / @as(f64, @floatFromInt(std.time.ns_per_s));
        try stdout.print("3. SQL Native JSON Extract (Count failed processes): {d:.4} seconds\n", .{json_time});
        try stdout.flush();

        timer.reset();
        rc = c.sqlite3_exec(db, "SELECT cypher_execute('MATCH (p)-[:RUNS_ON]->(h) RETURN p, h LIMIT 1000');", null, null, &err_msg);
        if (rc != c.SQLITE_OK) { std.debug.print("Cypher Error: {s}\n", .{err_msg}); c.sqlite3_free(err_msg); }
        const cypher_time = @as(f64, @floatFromInt(timer.read())) / @as(f64, @floatFromInt(std.time.ns_per_s));
        try stdout.print("4. Cypher MATCH (Tasks to Hosts - Limit 1000): {d:.4} seconds\n", .{cypher_time});
        try stdout.flush();
        
        timer.reset();
        rc = c.sqlite3_exec(db, "SELECT graph_betweenness_centrality();", null, null, &err_msg);
        if (rc != c.SQLITE_OK) { std.debug.print("Centrality Error: {s}\n", .{err_msg}); c.sqlite3_free(err_msg); }
        const bc_time = @as(f64, @floatFromInt(timer.read())) / @as(f64, @floatFromInt(std.time.ns_per_s));
        try stdout.print("5. Betweenness Centrality Algorithm: {d:.4} seconds\n", .{bc_time});
        try stdout.flush();

        std.process.exit(0);
    }

    if (command_to_run == null) {
        // Create Default Network Topology for interactive sessions
        const setup_sql = 
            "SELECT graph_node_add(1, '{\"name\": \"Router-A\", \"ip\": \"10.0.0.1\"}');" ++
            "SELECT graph_node_add(2, '{\"name\": \"Router-B\", \"ip\": \"10.0.0.2\"}');" ++
            "SELECT graph_node_add(3, '{\"name\": \"Router-C\", \"ip\": \"10.0.0.3\"}');" ++
            "SELECT graph_node_add(4, '{\"name\": \"Router-D\", \"ip\": \"10.0.0.4\"}');" ++
            "SELECT graph_edge_add(1, 2, 'LINK', '{\"latency\": 10}');" ++
            "SELECT graph_edge_add(2, 3, 'LINK', '{\"latency\": 5}');" ++
            "SELECT graph_edge_add(1, 4, 'LINK', '{\"latency\": 2}');" ++
            "SELECT graph_edge_add(4, 3, 'LINK', '{\"latency\": 2}');";
        
        rc = c.sqlite3_exec(db, setup_sql, null, null, &err_msg);
        if (rc != c.SQLITE_OK) {
            std.debug.print("Setup Error: {s}\n", .{err_msg});
            c.sqlite3_free(err_msg);
        }
    }

    if (command_to_run) |cmd| {
        rc = c.sqlite3_exec(db, cmd.ptr, printCallback, null, &err_msg);
        if (rc != c.SQLITE_OK) {
            std.debug.print("Error: {s}\n", .{err_msg});
            c.sqlite3_free(err_msg);
            std.process.exit(1);
        }
    } else {
        var stdout_buffer: [4096]u8 = undefined;
        var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
        const stdout = &stdout_writer.interface;

        try stdout.print("Dialtone DB (SQLite + Graph).\n", .{});
        try stdout.print("Network nodes (1=A, 2=B, 3=C, 4=D) and edges with 'latency' loaded.\n", .{});
        try stdout.print("Shortest path from A(1) to C(3) should prefer the low-latency 4ms path over the 15ms path.\n", .{});
        try stdout.print("Use .exit or .quit to exit.\n", .{});
        try stdout.flush();
        
        // Print shortest path demonstration
        const query_shortest = "SELECT graph_shortest_path_weighted(1, 3)";
        rc = c.sqlite3_exec(db, query_shortest, printCallback, null, &err_msg);
        if (rc != c.SQLITE_OK) {
            std.debug.print("Shortest Path Error: {s}\n", .{err_msg});
            c.sqlite3_free(err_msg);
        }

        var buffer: [4096]u8 = undefined;
        while (true) {
            try stdout.print("dialtone> ", .{});
            try stdout.flush();
            
            var eof = false;
            var len: usize = 0;
            while (true) {
                var byte: [1]u8 = undefined;
                const n = std.posix.read(std.posix.STDIN_FILENO, &byte) catch 0;
                if (n == 0) { eof = true; break; }
                if (byte[0] == '\n') break;
                if (len < buffer.len) {
                    buffer[len] = byte[0];
                    len += 1;
                }
            }
            if (eof and len == 0) break;
            if (len == 0) continue;
            
            // To handle EOF gracefully
            const line = buffer[0..len];

            const trimmed = std.mem.trim(u8, line, " \r\n\t");
            if (trimmed.len == 0) continue;
            
            if (std.mem.eql(u8, trimmed, ".exit") or std.mem.eql(u8, trimmed, ".quit")) {
                break;
            }

            const null_terminated = try allocator.dupeZ(u8, trimmed);
            defer allocator.free(null_terminated);

            rc = c.sqlite3_exec(db, null_terminated.ptr, printCallback, null, &err_msg);
            if (rc != c.SQLITE_OK) {
                std.debug.print("Error: {s}\n", .{err_msg});
                c.sqlite3_free(err_msg);
            }
        }
    }
}
