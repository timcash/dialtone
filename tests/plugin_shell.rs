//! End-to-end integration tests for Dialtone's blob-backed plugin system (`dialtone install <plugin>`)
//! and cross-OS persistent terminal session manager (`dialtone install shell` over QUIC `dialtone/shell/1`).
mod common;

use common::*;
use serde_json::Value;

const SECRET: &str = "dt-PluginShell_E2E_Secret_2026!";

/// Test 16: HTTP API requires valid Bearer token for plugin installation and shell session routes.
#[test]
fn http_api_requires_bearer_token_for_plugin_install_and_shell_routes() {
    let dir = TempDir::new("plugin-auth");
    let api_port: u16 = 18941;
    let listen = format!("127.0.0.1:{api_port}");
    let _node = Node::start(
        dir.path(),
        &dir.join("node.log"),
        api_port,
        &[
            "up",
            "--node",
            "auth-check-node",
            "--listen",
            &listen,
            "--private-mesh",
        ],
    );
    until_ok(30, "node API up", || {
        get_json(api_port, "/api/processes").is_some()
    });

    // Unauthenticated POSTs must be rejected with 401.
    let (code1, _) = http(
        api_port,
        "POST",
        "/api/plugins/install",
        None,
        Some(r#"{"name":"shell"}"#),
    )
    .unwrap();
    assert_eq!(code1, 401, "unauthenticated /api/plugins/install accepted");

    let (code2, _) = http(
        api_port,
        "POST",
        "/api/shell",
        None,
        Some(r#"{"request":{"action":"list"}}"#),
    )
    .unwrap();
    assert_eq!(code2, 401, "unauthenticated /api/shell accepted");

    // Authenticated POST with ~/.dialtone/api-token succeeds.
    let token = api_token(dir.path());
    let installed = post_json(
        api_port,
        "/api/plugins/install",
        &token,
        r#"{"name":"shell"}"#,
    )
    .expect("authenticated install failed");
    assert_eq!(installed["name"].as_str(), Some("shell"));

    let listed = post_json(
        api_port,
        "/api/shell",
        &token,
        r#"{"request":{"action":"list"}}"#,
    )
    .expect("authenticated shell list failed");
    assert_eq!(listed["kind"].as_str(), Some("listed"));
}

/// Test 17: Real CLI workflow — `dialtone install shell`, `plugin list`, `shell new/send/capture/ls/kill`.
#[test]
fn cli_dialtone_install_shell_and_local_session_workflow() {
    let dir = TempDir::new("plugin-cli");
    let d = Some(dir.path());
    let api_port: u16 = 18942;
    let listen = format!("127.0.0.1:{api_port}");

    let _node = Node::start(
        dir.path(),
        &dir.join("node.log"),
        api_port,
        &[
            "up",
            "--node",
            "local-shell-node",
            "--listen",
            &listen,
            "--private-mesh",
        ],
    );
    until_ok(30, "node API up", || {
        get_json(api_port, "/api/processes").is_some()
    });

    // Before `dialtone install shell`, shell commands are gated and fail cleanly.
    let (ok_before, out_before, err_before) = run_full(d, &["shell", "ls", "--listen", &listen]);
    let combined_before = format!("{out_before}{err_before}");
    assert!(
        !ok_before,
        "shell should fail before install: {combined_before}"
    );
    assert!(
        combined_before.contains("dialtone install shell"),
        "expected install hint, got: {combined_before}"
    );

    // Install `shell` plugin via blob store.
    let inst_json: Value = serde_json::from_str(&run_ok(
        d,
        &["install", "shell", "--listen", &listen, "--json"],
    ))
    .unwrap();
    assert_eq!(inst_json["name"].as_str(), Some("shell"));
    assert!(inst_json["artifact_digest"]
        .as_str()
        .unwrap_or("")
        .starts_with("sha256:"));

    // Verify `dialtone plugin list --json` reports `shell` as verified.
    let list_json: Value = serde_json::from_str(&run_ok(
        d,
        &["plugin", "list", "--listen", &listen, "--json"],
    ))
    .unwrap();
    let plugins = list_json["plugins"].as_array().unwrap();
    assert_eq!(plugins.len(), 1);
    assert_eq!(plugins[0]["manifest"]["name"].as_str(), Some("shell"));
    assert_eq!(plugins[0]["verified"].as_bool(), Some(true));

    // Create a persistent session, set an env var in command 1, read it back in command 2.
    let created: Value = serde_json::from_str(&run_ok(
        d,
        &["shell", "new", "work", "--listen", &listen, "--json"],
    ))
    .unwrap();
    assert_eq!(created["session"]["name"].as_str(), Some("work"));

    let (cmd1, cmd2) = if cfg!(windows) {
        (
            "$env:DT_CLI_STATE = 'persisted_99'",
            "Write-Output $env:DT_CLI_STATE",
        )
    } else {
        ("export DT_CLI_STATE=persisted_99", "echo $DT_CLI_STATE")
    };
    run_ok(
        d,
        &["shell", "send", "work", "--listen", &listen, "--", cmd1],
    );
    let out2 = run_ok(
        d,
        &["shell", "send", "work", "--listen", &listen, "--", cmd2],
    );
    assert!(
        out2.contains("persisted_99"),
        "state did not persist across CLI sends: {out2}"
    );

    // Capture scrollback and list sessions.
    let cap = run_ok(
        d,
        &[
            "shell", "capture", "work", "--lines", "20", "--listen", &listen,
        ],
    );
    assert!(
        cap.contains("persisted_99"),
        "capture missing scrollback output: {cap}"
    );

    let ls_json: Value =
        serde_json::from_str(&run_ok(d, &["shell", "ls", "--listen", &listen, "--json"])).unwrap();
    assert_eq!(ls_json["sessions"].as_array().unwrap().len(), 1);

    // Kill the session and verify list is empty.
    run_ok(d, &["shell", "kill", "work", "--listen", &listen]);
    let ls_after: Value =
        serde_json::from_str(&run_ok(d, &["shell", "ls", "--listen", &listen, "--json"])).unwrap();
    assert_eq!(ls_after["sessions"].as_array().unwrap().len(), 0);
}

/// Test 18: Two real Dialtone nodes on a shared secret channel — remote persistent shell over QUIC
/// replacing both SSH and tmux.
#[test]
fn cli_two_node_channel_remote_shell_replaces_ssh_and_tmux() {
    let root = TempDir::new("plugin-2node");
    let ctrl_dir = root.join("controller");
    let work_dir = root.join("worker");
    std::fs::create_dir_all(&ctrl_dir).unwrap();
    std::fs::create_dir_all(&work_dir).unwrap();

    let ctrl_api: u16 = 18943;
    let work_api: u16 = 18944;
    let ctrl_quic: u16 = 19543;
    let work_quic: u16 = 19544;
    let ctrl_listen = format!("127.0.0.1:{ctrl_api}");
    let work_listen = format!("127.0.0.1:{work_api}");

    // Bootstrap node identities and mutual allowlists at fixed private-mesh QUIC ports.
    let ctrl_status: Value = serde_json::from_str(&run_ok(
        Some(&ctrl_dir),
        &["status", "--node", "controller", "--private-mesh"],
    ))
    .unwrap();
    let work_status: Value = serde_json::from_str(&run_ok(
        Some(&work_dir),
        &["status", "--node", "worker", "--private-mesh"],
    ))
    .unwrap();
    let ctrl_id = ctrl_status["diagnostics"]["node_id"]
        .as_str()
        .unwrap()
        .to_string();
    let work_id = work_status["diagnostics"]["node_id"]
        .as_str()
        .unwrap()
        .to_string();
    let ip = ctrl_status["diagnostics"]["bound_non_tailnet_ipv4"]
        .as_str()
        .unwrap_or("127.0.0.1")
        .to_string();

    run_ok(
        Some(&ctrl_dir),
        &[
            "allowlist",
            "add",
            &work_id,
            "--addr",
            &format!("{ip}:{work_quic}"),
        ],
    );
    run_ok(
        Some(&work_dir),
        &[
            "allowlist",
            "add",
            &ctrl_id,
            "--addr",
            &format!("{ip}:{ctrl_quic}"),
        ],
    );

    let _ctrl = Node::start(
        &ctrl_dir,
        &root.join("controller.log"),
        ctrl_api,
        &[
            "up",
            "--node",
            "controller",
            "--listen",
            &ctrl_listen,
            "--private-mesh",
            "--quic-port",
            &ctrl_quic.to_string(),
            "--secret",
            SECRET,
        ],
    );
    let _work = Node::start(
        &work_dir,
        &root.join("worker.log"),
        work_api,
        &[
            "up",
            "--node",
            "worker",
            "--listen",
            &work_listen,
            "--private-mesh",
            "--quic-port",
            &work_quic.to_string(),
            "--secret",
            SECRET,
        ],
    );

    // Wait until controller sees worker in the gossip mesh.
    until_ok(45, "controller sees worker in mesh", || {
        let Some(mesh) = get_json(ctrl_api, "/api/mesh") else {
            return false;
        };
        mesh["nodes"]
            .as_array()
            .map(|arr| arr.iter().any(|n| n["node"].as_str() == Some("worker")))
            .unwrap_or(false)
    });

    // Install `shell` plugin on `worker`.
    run_ok(
        Some(&work_dir),
        &["install", "shell", "--listen", &work_listen],
    );

    // From `controller`, create a persistent session on `worker`, run stateful commands,
    // capture scrollback, list sessions, and kill it — zero SSH, zero tmux!
    run_ok(
        Some(&ctrl_dir),
        &[
            "shell",
            "new",
            "ops",
            "--node",
            "worker",
            "--listen",
            &ctrl_listen,
        ],
    );

    let (set_cmd, get_cmd) = if cfg!(windows) {
        (
            "$env:DT_REMOTE_PROOF = 'dialtone_replaces_ssh_tmux'",
            "Write-Output $env:DT_REMOTE_PROOF",
        )
    } else {
        (
            "export DT_REMOTE_PROOF=dialtone_replaces_ssh_tmux",
            "echo $DT_REMOTE_PROOF",
        )
    };
    run_ok(
        Some(&ctrl_dir),
        &[
            "shell",
            "send",
            "ops",
            "--node",
            "worker",
            "--listen",
            &ctrl_listen,
            "--",
            set_cmd,
        ],
    );
    let out = run_ok(
        Some(&ctrl_dir),
        &[
            "shell",
            "send",
            "ops",
            "--node",
            "worker",
            "--listen",
            &ctrl_listen,
            "--",
            get_cmd,
        ],
    );
    assert!(
        out.contains("dialtone_replaces_ssh_tmux"),
        "remote stateful output missing: {out}"
    );

    let cap = run_ok(
        Some(&ctrl_dir),
        &[
            "shell",
            "capture",
            "ops",
            "--node",
            "worker",
            "--lines",
            "25",
            "--listen",
            &ctrl_listen,
        ],
    );
    assert!(
        cap.contains("dialtone_replaces_ssh_tmux"),
        "remote capture missing output: {cap}"
    );

    let ls_json: Value = serde_json::from_str(&run_ok(
        Some(&ctrl_dir),
        &[
            "shell",
            "ls",
            "--node",
            "worker",
            "--listen",
            &ctrl_listen,
            "--json",
        ],
    ))
    .unwrap();
    assert_eq!(ls_json["node"].as_str(), Some("worker"));
    assert_eq!(ls_json["sessions"].as_array().unwrap().len(), 1);

    run_ok(
        Some(&ctrl_dir),
        &[
            "shell",
            "kill",
            "ops",
            "--node",
            "worker",
            "--listen",
            &ctrl_listen,
        ],
    );
}
