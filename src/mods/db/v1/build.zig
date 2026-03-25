const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const exe = b.addExecutable(.{
        .name = "dialtone_db",
        .root_module = b.createModule(.{
            .root_source_file = b.path("main.zig"),
            .target = target,
            .optimize = optimize,
        }),
    });

    exe.linkLibC();
    // Link math library
    exe.linkSystemLibrary("m");

    const c_flags = &[_][]const u8{
        "-Os",
        "-DSQLITE_CORE",
        "-DSQLITE_ENABLE_LOAD_EXTENSION",
        "-w", // Ignore warnings to avoid noise
    };

    // Add include paths
    exe.addIncludePath(b.path("."));
    exe.addIncludePath(b.path("sqlite-graph/include"));
    exe.addIncludePath(b.path("sqlite-graph/src"));
    exe.addIncludePath(b.path("sqlite-graph/src/cypher"));

    // Add main files
    exe.addCSourceFile(.{ .file = b.path("sqlite3.c"), .flags = c_flags });

    // Add all graph source files
    const graph_sources = &[_][]const u8{
        "sqlite-graph/src/graph-traverse.c",
        "sqlite-graph/src/graph-bulk.c",
        "sqlite-graph/src/graph-benchmark.c",
        "sqlite-graph/src/graph-enhanced.c",
        "sqlite-graph/src/graph-parallel.c",
        "sqlite-graph/src/graph-cache.c",
        "sqlite-graph/src/graph-algo.c",
        "sqlite-graph/src/graph-memory.c",
        "sqlite-graph/src/graph-compress.c",
        "sqlite-graph/src/graph-json.c",
        "sqlite-graph/src/graph-util.c",
        "sqlite-graph/src/graph-schema.c",
        "sqlite-graph/src/graph-destructors.c",
        "sqlite-graph/src/graph-tvf.c",
        "sqlite-graph/src/graph-vtab.c",
        "sqlite-graph/src/graph.c",
        "sqlite-graph/src/graph-advanced.c",
        "sqlite-graph/src/graph-performance.c",
        "sqlite-graph/src/cypher/cypher-executor-sql.c",
        "sqlite-graph/src/cypher/cypher-json.c",
        "sqlite-graph/src/cypher/cypher-lexer.c",
        "sqlite-graph/src/cypher/cypher-physical-plan.c",
        "sqlite-graph/src/cypher/cypher-planner.c",
        "sqlite-graph/src/cypher/cypher-expressions.c",
        "sqlite-graph/src/cypher/cypher-sql.c",
        "sqlite-graph/src/cypher/cypher-planner-sql.c",
        "sqlite-graph/src/cypher/cypher-logical-plan.c",
        "sqlite-graph/src/cypher/cypher-executor.c",
        "sqlite-graph/src/cypher/cypher-write.c",
        "sqlite-graph/src/cypher/cypher-parser.c",
        "sqlite-graph/src/cypher/cypher-write-sql.c",
        "sqlite-graph/src/cypher/cypher-ast.c",
        "sqlite-graph/src/cypher/cypher-execution-context.c",
        "sqlite-graph/src/cypher/cypher-storage.c",
        "sqlite-graph/src/cypher/cypher-iterators.c",
    };

    exe.addCSourceFiles(.{
        .files = graph_sources,
        .flags = c_flags,
    });

    const install_step = b.addInstallArtifact(exe, .{
        .dest_dir = .{ .override = .{ .custom = "../../../../../bin/mods/db/v1" } },
    });
    b.getInstallStep().dependOn(&install_step.step);

    const run_cmd = b.addRunArtifact(exe);
    run_cmd.step.dependOn(b.getInstallStep());

    if (b.args) |args| {
        run_cmd.addArgs(args);
    }

    const run_step = b.step("run", "Run the app");
    run_step.dependOn(&run_cmd.step);
}
