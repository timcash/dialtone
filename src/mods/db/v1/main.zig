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

    var i: usize = 1;
    while (i < args.len) : (i += 1) {
        if (std.mem.eql(u8, args[i], "-c")) {
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
