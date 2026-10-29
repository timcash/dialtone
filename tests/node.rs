//! One node: CLI, secrets, allowlist, API auth, supervisor, blobs, probe, one-shot runs, routes.
mod common;

use std::fs;

use base64::Engine;
use common::*;
use serde_json::Value;

const API: u16 = 18794;
const LISTEN: &str = "127.0.0.1:18794";

#[test]
fn one_node() {
    let dir = TempDir::new("node");
    let d = Some(dir.path());

    run_ok(d, &["devices", "--json"]);
    assert!(run_ok(d, &["status", "--node", "node-alpha"])
        .contains(r#""steady_state_data_plane": "iroh-quic""#));
    let boot = run_ok(
        d,
        &[
            "--node",
            "node-alpha",
            "--listen",
            "127.0.0.1:0",
            "--private-mesh",
            "--once",
        ],
    );
    for stage in 1..=5 {
        assert!(
            boot.contains(&format!("[BOOT {stage}/5]")),
            "boot stage {stage} missing"
        );
    }
    assert!(boot.contains(r#""schema": "dialtone.iroh.beat.v1""#));
    println!("    CLI: devices, status, five boot stages and a beat");

    let join = |secret: &str| {
        run(
            d,
            &["join", secret, "--listen", "127.0.0.1:19999", "--once"],
        )
    };
    assert!(
        !join("dialtone").0,
        "the channel name was accepted as a secret"
    );
    assert!(!join("shortpass1").0, "a weak secret was accepted");
    let secret = run_ok(None, &["secret"]).trim().to_string();
    let (ok, out) = join(&secret);
    assert!(
        ok && out.contains(r#""status": "saved_mesh_config""#),
        "{out}"
    );
    let config = fs::read_to_string(dir.join("config.json")).unwrap();
    assert!(
        !config.contains(&secret),
        "the plaintext secret was written to config.json"
    );
    assert!(config.contains(r#""derived_keys""#));
    println!("    Secrets: weak ones rejected, only derived keys stored");

    let peer = "1".repeat(64);
    let add = ["allowlist", "add", &peer, "--addr", "127.0.0.1:19440"];
    assert!(run_ok(d, &add).contains(r#""status": "allowed""#));
    assert!(run_ok(d, &["allowlist", "list"]).contains(&peer));
    assert!(run_ok(d, &["allowlist", "remove", &peer]).contains(r#""status": "revoked""#));
    run_ok(d, &add);
    assert!(
        !run(d, &["invite", "--listen", "127.0.0.1:19999"]).0,
        "an invite was made with no node running"
    );
    println!("    Allowlist add / list / remove; invites need a running node");

    let mut node = Node::start(
        dir.path(),
        &dir.join("node.log"),
        API,
        &[
            "up",
            "--node",
            "check-node",
            "--listen",
            LISTEN,
            "--private-mesh",
        ],
    );
    until_ok(30, "node API up", || {
        get_json(API, "/api/processes").is_some()
    });
    let token = api_token(dir.path());
    let post = |path: &str, body: &str| {
        post_json(API, path, &token, body).unwrap_or_else(|| panic!("POST {path} {body} failed"))
    };
    let (code, _) = http(API, "POST", "/api/blob", None, Some("{}")).unwrap();
    assert_eq!(code, 401, "unauthenticated POST was accepted");

    // The program records its environment: the node's DIALTONE_* settings must not reach it.
    let unix_dir = dir.path().display().to_string();
    let program = polyglot(
        &format!(r#"env > "{unix_dir}/process.env"; sleep 30; exit 0"#),
        &[
            "@echo off",
            &format!(r#"set > "{}""#, dir.join("process.env").display()),
            "ping -n 31 127.0.0.1 >NUL",
        ],
    );
    let b64 = base64::engine::general_purpose::STANDARD.encode(program);
    let digest = post("/api/blob", &format!(r#"{{"bytes_base64":"{b64}"}}"#))["digest"]
        .as_str()
        .unwrap()
        .to_string();
    let started = post(
        "/api/processes/blob",
        &format!(r#"{{"name":"check-process","digest":"{digest}","desired_state":"running"}}"#),
    );
    assert_eq!(started["process"]["status"], "running", "{started}");
    let pid1 = started["process"]["pid"].as_u64().unwrap();
    kill_pid(pid1);
    let check_process = || -> Option<Value> {
        get_json(API, "/api/processes")?["processes"]
            .as_array()?
            .iter()
            .find(|p| p["name"] == "check-process")
            .cloned()
    };
    let mut pid2 = 0;
    until_ok(
        15,
        "the supervisor restarted the killed process",
        || match check_process() {
            Some(p) if p["status"] == "running" && p["restart_count"].as_u64() >= Some(1) => {
                pid2 = p["pid"].as_u64().unwrap_or(0);
                true
            }
            _ => false,
        },
    );
    assert_ne!(
        pid1, pid2,
        "the supervisor did not restart the crashed process"
    );
    let env_file = dir.join("process.env");
    until_ok(10, "the process recorded its environment", || {
        fs::read_to_string(&env_file)
            .map(|e| e.lines().any(|l| l.starts_with("APP_BIN=")))
            .unwrap_or(false)
    });
    let env = fs::read_to_string(&env_file).unwrap();
    assert!(
        !env.lines().any(|l| l.starts_with("DIALTONE_")),
        "the node's DIALTONE_* environment reached a process"
    );
    let (code, body) = http(
        API,
        "POST",
        "/api/processes",
        Some(&token),
        Some(r#"{"name":"check-process","desired_state":"removed"}"#),
    )
    .unwrap();
    assert!(
        code == 200 && body.contains(r#""status":"removed""#),
        "{body}"
    );
    let staged = fs::read_dir(dir.join("run"))
        .map(|r| {
            r.flatten().any(|e| {
                e.file_name()
                    .to_string_lossy()
                    .starts_with("check-process-")
            })
        })
        .unwrap_or(false);
    assert!(!staged, "a removed process left its staged program");
    println!("    Supervisor: started from a blob without the node's env, restarted after kill -9, removed with its file");

    let share: Value = serde_json::from_str(&run_ok(
        d,
        &["blob", "share", "check-blob", "--listen", LISTEN],
    ))
    .unwrap();
    let ticket = share["ticket_uri"].as_str().unwrap();
    assert!(
        run_ok(d, &["blob", "pull", ticket, "--force", "--listen", LISTEN])
            .contains(r#""method": "local-self""#)
    );
    let gossip = post("/api/probe", "{}");
    assert_eq!(
        check_codes(&gossip["probe"]),
        ["BEAT", "MSH", "NBR"],
        "{gossip}"
    );
    let full = post("/api/probe", r#"{"all":true}"#);
    let p = &full["probe"];
    assert_eq!(
        check_codes(p),
        ["BEAT", "MSH", "NBR", "PRC", "PUL", "SEC", "STO"],
        "{full}"
    );
    let ok = |code: &str| {
        p["checks"]
            .as_array()
            .unwrap()
            .iter()
            .any(|c| c["code"] == code && c["ok"] == true)
    };
    assert!(ok("STO") && ok("PRC") && ok("SEC"), "{full}");
    println!("    Blobs share / pull, probe (gossip only, then full)");

    let before = fs::read(dir.join("config.json")).unwrap();
    run_ok(
        d,
        &[
            "--node",
            "check-once",
            "--listen",
            "127.0.0.1:0",
            "--private-mesh",
            "--once",
        ],
    );
    assert_eq!(
        before,
        fs::read(dir.join("config.json")).unwrap(),
        "a one-shot run rewrote the running node's config"
    );
    for route in [
        "/api/deploy-plan",
        "/ws",
        "/api/gossip",
        "/api/artifacts/blob",
        "/api/e2e/status",
    ] {
        let (code, _) = http(API, "GET", route, None, None).unwrap();
        assert_eq!(code, 404, "removed route {route} answered");
    }
    println!("    A one-shot run leaves the node's config alone; removed routes return 404");
    node.stop();
}
