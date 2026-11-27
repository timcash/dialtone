//! End-to-end and in-process integration tests for P3 signed channel manifests
//! (`dialtone.channel.manifest.v1`), fail-closed enforcement, gossip + JoinAnswer propagation,
//! daemon-managed plugins, and P5c `GET /api/channels` per API.md.
mod common;

use std::fs;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use common::{api_token, get_json, http, post_json, run_full, run_ok, until_ok, Node, TempDir};
use dialtone::api::{serve_http_blocking, NodeState};
use dialtone::blob::{digest_of, Blobs};
use dialtone::channel_log::{channel_log_path, ChannelLogQuery, ChannelLogStore};
use dialtone::channel_manifest::{
    apply_incoming_manifest, load_channel_manifest, save_channel_manifest, ChannelManifest,
    ChannelPolicyGuard, ManifestEnvelope, CHANNEL_MANIFEST_SCHEMA,
};
use dialtone::join::{redeem, Joins};
use dialtone::mesh::{
    channel_plugin_data_dir, derive_channel_keys_v2, events_topic_id, open_message,
    save_mesh_config, seal_message, LiveAllowlist, MeshConfig, Message, OpenInvite, Seed,
};
use dialtone::node::{
    build_endpoint_builder, load_or_create_secret_key_at, AllowlistHook, IrohTransportConfig,
    IrohTransportDiagnostics,
};
use dialtone::plugin::{
    dispatch_plugin, install_builtin_plugin, ManagedPluginManager, PluginDispatchContext,
    PluginDispatchError, PluginManifest, PLUGIN_MANIFEST_SCHEMA,
};
use dialtone::shell::{remote_shell, ShellHandler, ShellManager, ShellRequest, ShellResponse};
use futures_lite::StreamExt;
use iroh::address_lookup::memory::MemoryLookup;
use iroh::protocol::Router;
use iroh_gossip::{Event as GossipEvent, Gossip, ALPN as GOSSIP_ALPN};
use serde_json::Value;

const SECRET_A: &str = "dt-ChannelManifest_Alpha_Secret_2026!";
const SECRET_B: &str = "dt-ChannelManifest_Beta_Secret_2026!";

fn write_mock_plugin(
    plugins_dir: &Path,
    name: &str,
    unix_script: &str,
    win_script: &str,
) -> PathBuf {
    let dir = plugins_dir.join(name);
    fs::create_dir_all(&dir).unwrap();
    let (filename, body, entrypoint) = if cfg!(windows) {
        let f = format!("dialtone-{name}.cmd");
        (f.clone(), win_script.as_bytes().to_vec(), Some(f))
    } else {
        (
            format!("dialtone-{name}"),
            unix_script.as_bytes().to_vec(),
            None,
        )
    };
    let bin_path = dir.join(&filename);
    fs::write(&bin_path, &body).unwrap();
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&bin_path, fs::Permissions::from_mode(0o755)).unwrap();
    }
    let manifest = PluginManifest {
        schema: PLUGIN_MANIFEST_SCHEMA.to_string(),
        name: name.to_string(),
        version: "0.5.0".to_string(),
        kind: "blob-binary".to_string(),
        description: format!("mock plugin {name}"),
        protocols: Vec::new(),
        blob_digest: digest_of(&body),
        blake3_hash: "00".repeat(32),
        bytes: body.len() as u64,
        entrypoint,
        ticket_uri: None,
        source_repo: None,
        installed_at_unix: 1_795_000_000,
    };
    fs::write(
        dir.join("manifest.json"),
        serde_json::to_vec_pretty(&manifest).unwrap(),
    )
    .unwrap();
    bin_path
}

#[test]
fn cli_channel_create_and_manifest_allow_deny_round_trip() {
    let dir = TempDir::new("manifest-cli-rt");
    let d = Some(dir.path());

    // `dialtone channel create` creates a channel and signs its seq=1 manifest with creator admin.
    let created: Value = serde_json::from_str(&run_ok(
        d,
        &[
            "channel",
            "create",
            "ops",
            "--allow",
            "task,shell",
            "--json",
        ],
    ))
    .unwrap();
    assert_eq!(created["status"].as_str(), Some("created"));
    assert_eq!(created["channel"].as_str(), Some("ops"));
    let cid = created["channel_id"].as_str().unwrap().to_string();
    assert!(cid.starts_with("ch-"), "unexpected channel_id={cid}");
    assert_eq!(created["seq"].as_u64(), Some(1));
    assert_eq!(
        created["allowed_plugins"],
        serde_json::json!(["shell", "task"])
    );

    // `dialtone channel manifest show --json` reports valid signature and is_admin=true.
    let shown: Value =
        serde_json::from_str(&run_ok(d, &["channel", "manifest", "show", "--json"])).unwrap();
    assert_eq!(shown["channel_id"].as_str(), Some(cid.as_str()));
    assert_eq!(shown["is_admin"].as_bool(), Some(true));
    assert_eq!(shown["signature_valid"].as_bool(), Some(true));
    assert_eq!(shown["manifest"]["seq"].as_u64(), Some(1));

    // `allow agent` bumps seq to 2 and adds `agent`.
    let allowed: Value = serde_json::from_str(&run_ok(
        d,
        &["channel", "manifest", "allow", "agent", "--json"],
    ))
    .unwrap();
    assert_eq!(allowed["seq"].as_u64(), Some(2));
    assert_eq!(
        allowed["allowed_plugins"],
        serde_json::json!(["agent", "shell", "task"])
    );

    // `deny shell` bumps seq to 3 and removes `shell`.
    let denied: Value = serde_json::from_str(&run_ok(
        d,
        &["channel", "manifest", "deny", "shell", "--json"],
    ))
    .unwrap();
    assert_eq!(denied["seq"].as_u64(), Some(3));
    assert_eq!(
        denied["allowed_plugins"],
        serde_json::json!(["agent", "task"])
    );

    // `sign` bumps seq to 4 and re-signs cleanly.
    let resigned: Value =
        serde_json::from_str(&run_ok(d, &["channel", "manifest", "sign", "--json"])).unwrap();
    assert_eq!(resigned["seq"].as_u64(), Some(4));

    let loaded = load_channel_manifest(dir.path(), &cid).unwrap().unwrap();
    assert_eq!(loaded.seq, 4);
    assert!(loaded.verify_for_channel(&cid).is_ok());
}

#[test]
fn manifest_enforces_install_dispatch_and_shell_fail_closed_and_legacy_no_manifest() {
    let dir = TempDir::new("manifest-enforce-e2e");
    let d = Some(dir.path());
    let api_port: u16 = 18961;
    let listen = format!("127.0.0.1:{api_port}");

    // Start node joined to `metal` with NO manifest yet (legacy channel mode).
    let _node = Node::start(
        dir.path(),
        &dir.join("node.log"),
        api_port,
        &[
            "up",
            "--node",
            "enforce-node",
            "--listen",
            &listen,
            "--private-mesh",
            "--channel",
            "metal",
            "--secret",
            SECRET_A,
        ],
    );
    until_ok(30, "node API up", || {
        get_json(api_port, "/api/channels").is_some()
    });

    // Legacy channel (no manifest): GET /api/channels reports manifest=null.
    let ch_before = get_json(api_port, "/api/channels").unwrap();
    assert_eq!(ch_before["schema"].as_str(), Some("dialtone.channels.v1"));
    assert!(ch_before["channels"][0]["manifest"].is_null());
    assert_eq!(ch_before["channels"][0]["is_admin"].as_bool(), Some(false));
    let cid = ch_before["channels"][0]["channel_id"]
        .as_str()
        .unwrap()
        .to_string();

    // Legacy channel allows installing `shell` and calling POST /api/shell, and logs a warning to channel log.
    let token = api_token(dir.path());
    let inst_ok = post_json(api_port, "/api/plugins/install", &token, r#"{"name":"shell"}"#)
        .expect("legacy channel should allow install");
    assert_eq!(inst_ok["name"].as_str(), Some("shell"));

    let shell_ok = post_json(
        api_port,
        "/api/shell",
        &token,
        r#"{"request":{"action":"list"}}"#,
    )
    .expect("legacy channel should allow shell");
    assert_eq!(shell_ok["kind"].as_str(), Some("listed"));

    let log_json = get_json(api_port, &format!("/api/channel/log?channel={cid}")).unwrap();
    let entries = log_json["entries"].as_array().unwrap();
    assert!(
        entries
            .iter()
            .any(|e| e["kind"].as_str() == Some("manifest.legacy_warning")),
        "expected legacy no-manifest warning in channel log: {entries:?}"
    );

    // Now initialize signed manifest allowing ONLY `task` (denying `shell` and `agent`).
    run_ok(
        d,
        &[
            "channel",
            "manifest",
            "init",
            "--allow",
            "task",
            "--listen",
            &listen,
        ],
    );

    // POST /api/plugins/install for `agent` (not permitted) -> HTTP 403!
    let (code_inst, body_inst) = http(
        api_port,
        "POST",
        "/api/plugins/install",
        Some(&token),
        Some(r#"{"name":"agent"}"#),
    )
    .unwrap();
    assert_eq!(code_inst, 403, "expected 403 for disallowed install: {body_inst}");
    assert!(body_inst.contains("not allowed"), "{body_inst}");

    // CLI `dialtone install agent` must also refuse (never bypass daemon 403 to offline install).
    let (cli_ok, cli_out, cli_err) =
        run_full(d, &["install", "agent", "--listen", &listen]);
    assert!(!cli_ok, "CLI install of disallowed plugin must fail: {cli_out}{cli_err}");

    // POST /api/shell must be refused with HTTP 403 while `shell` is not in allowed_plugins!
    let (code_sh, body_sh) = http(
        api_port,
        "POST",
        "/api/shell",
        Some(&token),
        Some(r#"{"request":{"action":"list"}}"#),
    )
    .unwrap();
    assert_eq!(code_sh, 403, "expected 403 for disallowed shell: {body_sh}");
    assert!(body_sh.contains("not allowed"), "{body_sh}");

    // Plugin dispatch (`dispatch_plugin`): permitted `task` runs, disallowed `agent` is refused!
    let plugins_dir = dir.path().join("plugins");
    write_mock_plugin(
        &plugins_dir,
        "task",
        "#!/bin/sh\nexit 0\n",
        "@echo off\r\nexit /b 0\r\n",
    );
    write_mock_plugin(
        &plugins_dir,
        "agent",
        "#!/bin/sh\nexit 0\n",
        "@echo off\r\nexit /b 0\r\n",
    );
    let keys = derive_channel_keys_v2("metal", SECRET_A);
    let ctx = PluginDispatchContext {
        state_dir: dir.path().to_path_buf(),
        plugins_dir,
        api_url: format!("http://{listen}"),
        node_name: "enforce-node".to_string(),
        channel_keys: Some(keys),
    };
    assert_eq!(dispatch_plugin(&ctx, "task", &[]).unwrap(), 0);
    let agent_err = dispatch_plugin(&ctx, "agent", &[]).unwrap_err();
    match agent_err {
        PluginDispatchError::Refused(msg) => assert!(msg.contains("not allowed"), "{msg}"),
        other => panic!("expected Refused for disallowed plugin dispatch, got {other:?}"),
    }

    // Allow `shell` via manifest update -> POST /api/shell succeeds again!
    run_ok(
        d,
        &["channel", "manifest", "allow", "shell", "--listen", &listen],
    );
    let shell_after = post_json(
        api_port,
        "/api/shell",
        &token,
        r#"{"request":{"action":"list"}}"#,
    )
    .expect("shell should succeed once allowed by manifest");
    assert_eq!(shell_after["kind"].as_str(), Some("listed"));
}

#[tokio::test]
async fn shell_quic_handler_enforces_channel_manifest_fail_closed() {
    let dir = TempDir::new("manifest-shell-quic");
    let blobs = Blobs::open(&dir.join("blobs")).await.unwrap();
    let plugins_dir = dir.join("plugins");
    install_builtin_plugin(&blobs, &plugins_dir, "shell").unwrap();

    let cid = "ch-deadbeef1234";
    let admin_key = iroh::SecretKey::generate();
    // Manifest allows only `task`, NOT `shell`.
    let m1 = ChannelManifest::new_signed(cid, "metal", &["task".into()], &admin_key).unwrap();
    save_channel_manifest(dir.path(), &m1).unwrap();

    let guard = ChannelPolicyGuard::new(
        dir.path().to_path_buf(),
        Some(cid.to_string()),
        "server-node",
        Some(admin_key.clone()),
    );

    let cfg = IrohTransportConfig::local_offline();
    let server_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
    let handler = ShellHandler::new(
        Arc::new(ShellManager::new("server-node")),
        blobs.clone(),
        plugins_dir,
    )
    .with_policy(guard);
    let router = Router::builder(server_ep.clone())
        .accept(dialtone::shell::ALPN, handler)
        .spawn();

    let lookup = MemoryLookup::new();
    lookup.add_endpoint_info(server_ep.addr());
    let client_ep = build_endpoint_builder(&cfg)
        .unwrap()
        .address_lookup(lookup)
        .bind()
        .await
        .unwrap();

    // Refused while `shell` is not allowed.
    let resp1 = remote_shell(&client_ep, server_ep.id(), &ShellRequest::List)
        .await
        .unwrap();
    match resp1 {
        ShellResponse::Error { error } => assert!(error.contains("not allowed"), "{error}"),
        other => panic!("expected Error when shell denied by manifest, got {other:?}"),
    }

    // Update manifest to allow `shell` -> immediately succeeds over QUIC!
    let mut m2 = m1.clone();
    m2.allow_plugin("shell", None, &admin_key).unwrap();
    save_channel_manifest(dir.path(), &m2).unwrap();

    let resp2 = remote_shell(&client_ep, server_ep.id(), &ShellRequest::List)
        .await
        .unwrap();
    assert!(
        matches!(resp2, ShellResponse::Listed { .. }),
        "expected Listed after shell allowed, got {resp2:?}"
    );

    client_ep.close().await;
    let _ = router.shutdown().await;
    blobs.close().await;
}

#[tokio::test]
async fn manifest_propagates_between_two_nodes_on_one_channel_and_invisible_to_other_channel() {
    let root = TempDir::new("manifest-gossip-iso");
    let dir_a1 = root.join("a1");
    let dir_a2 = root.join("a2");
    let dir_b = root.join("b");
    fs::create_dir_all(&dir_a1).unwrap();
    fs::create_dir_all(&dir_a2).unwrap();
    fs::create_dir_all(&dir_b).unwrap();

    let keys_a = derive_channel_keys_v2("alpha", SECRET_A);
    let keys_b = derive_channel_keys_v2("beta", SECRET_B);
    let cid_a = keys_a.fingerprint();
    let cid_b = keys_b.fingerprint();

    let cfg = IrohTransportConfig::local_offline();
    let key_a1 = iroh::SecretKey::generate();
    let key_a2 = iroh::SecretKey::generate();
    let key_b = iroh::SecretKey::generate();

    let lookup_a1 = MemoryLookup::new();
    let lookup_a2 = MemoryLookup::new();
    let lookup_b = MemoryLookup::new();

    let ep_a1 = build_endpoint_builder(&cfg)
        .unwrap()
        .secret_key(key_a1.clone())
        .address_lookup(lookup_a1.clone())
        .bind()
        .await
        .unwrap();
    let ep_a2 = build_endpoint_builder(&cfg)
        .unwrap()
        .secret_key(key_a2.clone())
        .address_lookup(lookup_a2.clone())
        .bind()
        .await
        .unwrap();
    let ep_b = build_endpoint_builder(&cfg)
        .unwrap()
        .secret_key(key_b.clone())
        .address_lookup(lookup_b.clone())
        .bind()
        .await
        .unwrap();

    lookup_a1.add_endpoint_info(ep_a2.addr());
    lookup_a1.add_endpoint_info(ep_b.addr());
    lookup_a2.add_endpoint_info(ep_a1.addr());
    lookup_a2.add_endpoint_info(ep_b.addr());
    lookup_b.add_endpoint_info(ep_a1.addr());
    lookup_b.add_endpoint_info(ep_a2.addr());

    let g_a1 = Gossip::builder().spawn(ep_a1.clone());
    let g_a2 = Gossip::builder().spawn(ep_a2.clone());
    let g_b = Gossip::builder().spawn(ep_b.clone());

    let r_a1 = Router::builder(ep_a1.clone())
        .accept(GOSSIP_ALPN, g_a1.clone())
        .spawn();
    let r_a2 = Router::builder(ep_a2.clone())
        .accept(GOSSIP_ALPN, g_a2.clone())
        .spawn();
    let r_b = Router::builder(ep_b.clone())
        .accept(GOSSIP_ALPN, g_b.clone())
        .spawn();

    let topic_a = events_topic_id(keys_a.topic_id);
    let topic_b = events_topic_id(keys_b.topic_id);

    let (tx_a1, mut rx_a1) = g_a1
        .subscribe(topic_a, vec![ep_a2.id()])
        .await
        .unwrap()
        .split();
    let (_tx_a2, mut rx_a2) = g_a2
        .subscribe(topic_a, vec![ep_a1.id()])
        .await
        .unwrap()
        .split();
    let (_tx_b, mut rx_b) = g_b
        .subscribe(topic_b, vec![ep_a1.id(), ep_a2.id()])
        .await
        .unwrap()
        .split();

    // Wait for both A1 and A2 to connect on Channel A's events topic.
    let _ = tokio::time::timeout(Duration::from_secs(10), rx_a1.joined()).await;
    let _ = tokio::time::timeout(Duration::from_secs(10), rx_a2.joined()).await;

    // Admin A1 creates & seals manifest seq=1 for Channel A and broadcasts on Channel A events topic.
    let m1 = ChannelManifest::new_signed(
        &cid_a,
        "alpha",
        &["task".into(), "shell".into()],
        &key_a1,
    )
    .unwrap();
    save_channel_manifest(&dir_a1, &m1).unwrap();
    let env1 = ManifestEnvelope {
        node: "a1".to_string(),
        node_id: ep_a1.id().to_string(),
        manifest: m1.clone(),
        sent_unix_ms: 0,
    };
    let wire1 = seal_message(&Message::Manifest(env1), &keys_a, &key_a1).unwrap();

    // Node A2 receives, opens, verifies, and applies manifest seq=1.
    let mut received_m1 = None;
    for _ in 0..10 {
        let _ = tx_a1.broadcast(wire1.clone().into()).await;
        if let Ok(Some(Ok(GossipEvent::Received(msg)))) =
            tokio::time::timeout(Duration::from_millis(400), rx_a2.next()).await
        {
            if let Ok(Message::Manifest(env)) = open_message(&msg.content, &keys_a) {
                assert!(apply_incoming_manifest(&dir_a2, &cid_a, &env.manifest).unwrap());
                received_m1 = Some(env.manifest);
                break;
            }
        }
    }
    let received_m1 = received_m1.expect("A2 did not receive manifest seq=1");
    assert_eq!(received_m1.seq, 1);
    assert_eq!(received_m1.allowed_plugins, vec!["shell", "task"]);

    // Channel B node cannot open Channel A's wire frame and receives nothing on Channel B's events topic.
    assert!(
        open_message(&wire1, &keys_b).is_err(),
        "Channel B decrypted Channel A manifest frame!"
    );
    let b_got = tokio::time::timeout(Duration::from_millis(400), rx_b.next()).await;
    assert!(
        b_got.is_err()
            || !matches!(
                b_got,
                Ok(Some(Ok(GossipEvent::Received(_))))
            ),
        "Channel B unexpectedly received a Channel A gossip frame"
    );
    assert!(load_channel_manifest(&dir_b, &cid_a).unwrap().is_none());
    assert!(load_channel_manifest(&dir_b, &cid_b).unwrap().is_none());

    // Admin A1 updates to seq=2 (allow agent) and broadcasts -> A2 applies seq=2.
    let mut m2 = m1.clone();
    m2.allow_plugin("agent", None, &key_a1).unwrap();
    let env2 = ManifestEnvelope {
        node: "a1".to_string(),
        node_id: ep_a1.id().to_string(),
        manifest: m2.clone(),
        sent_unix_ms: 0,
    };
    let wire2 = seal_message(&Message::Manifest(env2), &keys_a, &key_a1).unwrap();

    let mut received_m2 = None;
    for _ in 0..10 {
        let _ = tx_a1.broadcast(wire2.clone().into()).await;
        if let Ok(Some(Ok(GossipEvent::Received(msg)))) =
            tokio::time::timeout(Duration::from_millis(400), rx_a2.next()).await
        {
            if let Ok(Message::Manifest(env)) = open_message(&msg.content, &keys_a) {
                if env.manifest.seq == 2 {
                    assert!(apply_incoming_manifest(&dir_a2, &cid_a, &env.manifest).unwrap());
                    received_m2 = Some(env.manifest);
                    break;
                }
            }
        }
    }
    let received_m2 = received_m2.expect("A2 did not receive manifest seq=2");
    assert_eq!(received_m2.seq, 2);
    assert!(received_m2.allows_plugin("agent"));

    // Stale seq=1 or wrong-admin seq=3 are rejected on A2.
    assert!(apply_incoming_manifest(&dir_a2, &cid_a, &m1).is_err());
    let mut forged = m2.clone();
    forged.admin = key_a2.public().to_string();
    forged.seq = 3;
    forged.sign_in_place(&key_a2).unwrap();
    assert!(apply_incoming_manifest(&dir_a2, &cid_a, &forged).is_err());

    let _ = r_a1.shutdown().await;
    let _ = r_a2.shutdown().await;
    let _ = r_b.shutdown().await;
}

#[tokio::test]
async fn join_answer_carries_signed_channel_manifest() {
    let dir = TempDir::new("manifest-join-answer");
    let config_path = dir.join("config.json");
    let mut cfg = MeshConfig::default();
    let keys = cfg.apply_secret("metal", SECRET_A);
    let cid = keys.fingerprint();
    save_mesh_config(&config_path, &cfg).unwrap();

    let admin_key = iroh::SecretKey::generate();
    let mut manifest = ChannelManifest::new_signed(
        &cid,
        "metal",
        &["task".into(), "shell".into(), "agent".into()],
        &admin_key,
    )
    .unwrap();
    manifest.bump_and_sign(&admin_key).unwrap();
    save_channel_manifest(dir.path(), &manifest).unwrap();

    let tcfg = IrohTransportConfig::local_offline();
    let member_ep = build_endpoint_builder(&tcfg)
        .unwrap()
        .hooks(AllowlistHook::new(LiveAllowlist::default()))
        .bind()
        .await
        .unwrap();
    let joins = Joins::new(member_ep.clone(), tcfg.clone(), config_path);
    let seed = Seed {
        node_id: member_ep.id().to_string(),
        relay_url: Some("none".to_string()),
        ip_addrs: member_ep.addr().ip_addrs().map(|a| a.to_string()).collect(),
    };
    let router = Router::builder(member_ep)
        .accept(dialtone::join::ALPN, joins)
        .spawn();

    let joiner_ep = build_endpoint_builder(&tcfg).unwrap().bind().await.unwrap();
    let invite = OpenInvite::new(&keys, vec![seed], dialtone::now_unix(), 120).unwrap();
    let answer = redeem(&joiner_ep, &invite, Duration::from_secs(10))
        .await
        .unwrap();
    assert_eq!(answer.channel, "metal");
    let got_manifest = answer
        .manifest
        .expect("JoinAnswer should carry the signed channel manifest");
    assert_eq!(got_manifest, manifest);
    assert!(got_manifest.verify_for_channel(&cid).is_ok());

    joiner_ep.close().await;
    let _ = router.shutdown().await;
}

#[test]
fn get_api_channels_matches_api_md_contract_and_since_hlc_log_query() {
    let dir = TempDir::new("api-channels-shape");
    let blobs = Blobs::open_blocking(&dir.join("blobs")).unwrap();
    let node_key = load_or_create_secret_key_at(&dir.join("iroh-secret-key")).unwrap();
    let node_id = node_key.public().to_string();

    let mut diag = IrohTransportDiagnostics::default();
    diag.node_id = node_id.clone();

    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let port = listener.local_addr().unwrap().port();
    let listen = format!("127.0.0.1:{port}");

    let mut state = NodeState::new_live("rosie", "up", &listen, blobs.clone(), diag);
    let keys = state.mesh_config.apply_secret("metal", SECRET_A);
    let cid = keys.fingerprint();
    state.mesh_config.updated_at_unix = 1_795_000_000;
    state.node_key = Some(node_key.clone());

    // Create a signed seq=3 manifest for `metal` with this node as creator admin.
    let mut manifest = ChannelManifest::new_signed(
        &cid,
        "metal",
        &["task".into(), "shell".into(), "agent".into()],
        &node_key,
    )
    .unwrap();
    manifest.bump_and_sign(&node_key).unwrap();
    manifest.bump_and_sign(&node_key).unwrap();
    assert_eq!(manifest.seq, 3);
    save_channel_manifest(dir.path(), &manifest).unwrap();

    // Install mock `task` and `agent` plugins and write a channel task config with rpc_addr.
    let plugins_dir = dir.join("plugins");
    write_mock_plugin(&plugins_dir, "task", "#!/bin/sh\nexit 0\n", "@echo off\r\n");
    write_mock_plugin(&plugins_dir, "agent", "#!/bin/sh\nexit 0\n", "@echo off\r\n");
    let task_data_dir = channel_plugin_data_dir(dir.path(), &cid, "task");
    fs::create_dir_all(&task_data_dir).unwrap();
    fs::write(
        task_data_dir.join("config.json"),
        serde_json::to_vec_pretty(&serde_json::json!({ "rpc_addr": "127.0.0.1:8794" })).unwrap(),
    )
    .unwrap();

    // Seed two channel log entries to verify `GET /api/channel/log?channel=...&since_hlc=...`.
    let mut log_store = ChannelLogStore::open(&channel_log_path(dir.path(), &cid), cid.clone())
        .unwrap();
    let e1 = log_store
        .append_local(&node_key, "rosie", "daemon", "test.one", None, "info", "first")
        .unwrap();
    std::thread::sleep(Duration::from_millis(5));
    let _e2 = log_store
        .append_local(&node_key, "rosie", "daemon", "test.two", None, "info", "second")
        .unwrap();
    state.channel_log = Some(Arc::new(Mutex::new(log_store)));

    let shared = Arc::new(Mutex::new(state));
    let server_state = Arc::clone(&shared);
    let server = std::thread::spawn(move || {
        let _ = serve_http_blocking(listener, server_state, Some(2));
    });

    let body = get_json(port, "/api/channels").expect("GET /api/channels");
    assert_eq!(body["schema"].as_str(), Some("dialtone.channels.v1"));
    assert_eq!(body["node"].as_str(), Some("rosie"));
    let channels = body["channels"].as_array().unwrap();
    assert_eq!(channels.len(), 1);
    let ch = &channels[0];
    assert_eq!(ch["channel_id"].as_str(), Some(cid.as_str()));
    assert_eq!(ch["name"].as_str(), Some("metal"));
    assert_eq!(ch["active"].as_bool(), Some(true));
    assert_eq!(ch["joined_at_unix"].as_u64(), Some(1_795_000_000));
    assert_eq!(ch["peers"].as_u64(), Some(0));
    assert_eq!(ch["is_admin"].as_bool(), Some(true));
    assert_eq!(
        ch["manifest"]["schema"].as_str(),
        Some(CHANNEL_MANIFEST_SCHEMA)
    );
    assert_eq!(ch["manifest"]["seq"].as_u64(), Some(3));
    assert_eq!(ch["manifest"]["admin"].as_str(), Some(node_id.as_str()));
    assert_eq!(
        ch["manifest"]["allowed_plugins"],
        serde_json::json!(["agent", "shell", "task"])
    );
    assert_eq!(ch["manifest"]["signature_valid"].as_bool(), Some(true));
    assert_eq!(
        ch["installed_plugins"],
        serde_json::json!(["agent", "task"])
    );
    assert_eq!(
        ch["plugins"]["task"]["rpc_addr"].as_str(),
        Some("127.0.0.1:8794")
    );
    assert_eq!(
        ch["plugins"]["task"]["data_dir"].as_str(),
        Some(task_data_dir.to_string_lossy().as_ref())
    );
    // Verify no secret or enc_key leaks anywhere in the JSON response.
    let serialized = serde_json::to_string(&body).unwrap();
    assert!(!serialized.contains(SECRET_A));
    assert!(!serialized.contains(&dialtone::hex_encode(&keys.enc_key)));

    // Verify `GET /api/channel/log?channel=<cid>&limit=10&since_hlc=<hlc_ms>` works cleanly.
    let log_resp = get_json(
        port,
        &format!("/api/channel/log?channel={cid}&limit=10&since_hlc={}:0", e1.hlc_ms),
    )
    .expect("GET /api/channel/log with since_hlc");
    assert_eq!(log_resp["channel_id"].as_str(), Some(cid.as_str()));
    assert_eq!(log_resp["entries"].as_array().unwrap().len(), 1);
    assert_eq!(
        log_resp["entries"][0]["message"].as_str(),
        Some("second")
    );

    let _ = server.join();
    blobs.close_blocking();
}

#[test]
fn managed_plugins_start_with_channel_env_only_when_manifest_permits() {
    let dir = TempDir::new("managed-plugins-policy");
    let plugins_dir = dir.join("plugins");
    let keys = derive_channel_keys_v2("metal", SECRET_A);
    let cid = keys.fingerprint();
    let admin_key = iroh::SecretKey::generate();

    // Mock plugin that writes its received channel env vars to a marker file inside DIALTONE_CHANNEL_DATA_DIR and stays alive briefly.
    let unix_script = "#!/bin/sh\nmkdir -p \"$DIALTONE_CHANNEL_DATA_DIR\"\nprintf 'CID=%s\\nPKEY=%s\\nDDIR=%s\\n' \"$DIALTONE_CHANNEL_ID\" \"$DIALTONE_CHANNEL_PLUGIN_KEY\" \"$DIALTONE_CHANNEL_DATA_DIR\" > \"$DIALTONE_CHANNEL_DATA_DIR/env.txt\"\nsleep 10\n";
    let win_script = "@echo off\r\nif not exist \"%DIALTONE_CHANNEL_DATA_DIR%\" mkdir \"%DIALTONE_CHANNEL_DATA_DIR%\"\r\n(\r\necho CID=%DIALTONE_CHANNEL_ID%\r\necho PKEY=%DIALTONE_CHANNEL_PLUGIN_KEY%\r\necho DDIR=%DIALTONE_CHANNEL_DATA_DIR%\r\n) > \"%DIALTONE_CHANNEL_DATA_DIR%\\env.txt\"\r\nping -n 11 127.0.0.1 >NUL\r\n";
    write_mock_plugin(&plugins_dir, "task", unix_script, win_script);
    write_mock_plugin(&plugins_dir, "agent", unix_script, win_script);

    // Manifest permits `task` only, NOT `agent`.
    let m1 = ChannelManifest::new_signed(&cid, "metal", &["task".into()], &admin_key).unwrap();
    save_channel_manifest(dir.path(), &m1).unwrap();

    let ctx = PluginDispatchContext {
        state_dir: dir.path().to_path_buf(),
        plugins_dir,
        api_url: "http://127.0.0.1:8788".to_string(),
        node_name: "legion".to_string(),
        channel_keys: Some(keys.clone()),
    };

    let mut mgr = ManagedPluginManager::new();
    let statuses = mgr.reconcile(&ctx, &["task".into(), "agent".into()], &[], None);
    let task_st = statuses.iter().find(|s| s.name == "task").unwrap();
    let agent_st = statuses.iter().find(|s| s.name == "agent").unwrap();
    assert!(task_st.running, "permitted plugin task should start: {task_st:?}");
    assert!(!agent_st.running, "disallowed plugin agent must not start: {agent_st:?}");
    assert!(
        agent_st
            .error
            .as_deref()
            .unwrap_or("")
            .contains("not allowed"),
        "{agent_st:?}"
    );

    let marker = channel_plugin_data_dir(dir.path(), &cid, "task").join("env.txt");
    until_ok(10, "managed plugin wrote channel env marker", || {
        marker.is_file() && fs::read_to_string(&marker).map(|s| s.contains("PKEY=")).unwrap_or(false)
    });
    let env_txt = fs::read_to_string(&marker).unwrap();
    assert!(env_txt.contains(&format!("CID={cid}")), "{env_txt}");
    let expected_key = dialtone::mesh::derive_channel_plugin_key_hex(&keys, "task");
    assert!(env_txt.contains(&format!("PKEY={expected_key}")), "{env_txt}");

    // Now deny `task` in manifest -> reconcile must stop it!
    let mut m2 = m1.clone();
    m2.deny_plugin("task", &admin_key).unwrap();
    save_channel_manifest(dir.path(), &m2).unwrap();
    let statuses2 = mgr.reconcile(&ctx, &["task".into()], &[], None);
    assert!(!statuses2[0].running, "denied plugin must be stopped: {statuses2:?}");

    // Ensure channel log warning helper did not fire when manifest was present.
    let log_store = ChannelLogStore::open(&channel_log_path(dir.path(), &cid), cid).unwrap();
    assert!(log_store.query(&ChannelLogQuery::default()).is_empty());

    mgr.stop_all();
}
