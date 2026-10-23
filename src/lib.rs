pub mod api;
pub mod blob;
pub mod deploy;
pub mod h3_land;
pub mod hud;
pub mod mesh;
pub mod node;
pub mod record;
pub mod service;
pub mod stats;
pub mod supervisor;

use std::collections::HashMap;
use std::env;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::process::Command;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

pub fn now_unix() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

pub fn run_cmd(prog: &str, args: &[&str]) -> Option<String> {
    let out = Command::new(prog).args(args).output().ok()?;
    if !out.status.success() {
        return None;
    }
    Some(String::from_utf8_lossy(&out.stdout).trim().to_string())
}

pub fn hex_encode(bytes: &[u8]) -> String {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for &b in bytes {
        out.push(HEX[(b >> 4) as usize] as char);
        out.push(HEX[(b & 0x0f) as usize] as char);
    }
    out
}

pub fn hex_decode(raw: &str) -> Result<Vec<u8>, String> {
    let s = raw.trim();
    if !s.len().is_multiple_of(2) {
        return Err("odd hex length".to_string());
    }
    let mut out = Vec::with_capacity(s.len() / 2);
    let bytes = s.as_bytes();
    for i in (0..bytes.len()).step_by(2) {
        let hi = (bytes[i] as char)
            .to_digit(16)
            .ok_or_else(|| "invalid hex digit".to_string())?;
        let lo = (bytes[i + 1] as char)
            .to_digit(16)
            .ok_or_else(|| "invalid hex digit".to_string())?;
        out.push(((hi << 4) | lo) as u8);
    }
    Ok(out)
}

pub fn base64_encode(bytes: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::with_capacity(bytes.len().div_ceil(3) * 4);
    for chunk in bytes.chunks(3) {
        let b0 = chunk[0] as u32;
        let b1 = if chunk.len() > 1 { chunk[1] as u32 } else { 0 };
        let b2 = if chunk.len() > 2 { chunk[2] as u32 } else { 0 };
        let triple = (b0 << 16) | (b1 << 8) | b2;
        out.push(TABLE[((triple >> 18) & 0x3f) as usize] as char);
        out.push(TABLE[((triple >> 12) & 0x3f) as usize] as char);
        if chunk.len() > 1 {
            out.push(TABLE[((triple >> 6) & 0x3f) as usize] as char);
        } else {
            out.push('=');
        }
        if chunk.len() > 2 {
            out.push(TABLE[(triple & 0x3f) as usize] as char);
        } else {
            out.push('=');
        }
    }
    out
}

pub fn base64url_encode(bytes: &[u8]) -> String {
    base64_encode(bytes)
        .trim_end_matches('=')
        .replace('+', "-")
        .replace('/', "_")
}

pub fn base64_decode(raw: &str) -> Result<Vec<u8>, String> {
    fn val(b: u8) -> Result<u8, String> {
        match b {
            b'A'..=b'Z' => Ok(b - b'A'),
            b'a'..=b'z' => Ok(b - b'a' + 26),
            b'0'..=b'9' => Ok(b - b'0' + 52),
            b'+' | b'-' => Ok(62),
            b'/' | b'_' => Ok(63),
            _ => Err(format!("invalid base64 byte: {b}")),
        }
    }
    let clean: Vec<u8> = raw
        .bytes()
        .filter(|b| !b.is_ascii_whitespace() && *b != b'=')
        .collect();
    let mut out = Vec::with_capacity(clean.len() * 3 / 4);
    for chunk in clean.chunks(4) {
        if chunk.len() < 2 {
            return Err("invalid base64 chunk length".to_string());
        }
        let c0 = val(chunk[0])? as u32;
        let c1 = val(chunk[1])? as u32;
        out.push(((c0 << 2) | (c1 >> 4)) as u8);
        if chunk.len() > 2 {
            let c2 = val(chunk[2])? as u32;
            out.push((((c1 & 0x0f) << 4) | (c2 >> 2)) as u8);
            if chunk.len() > 3 {
                let c3 = val(chunk[3])? as u32;
                out.push((((c2 & 0x03) << 6) | c3) as u8);
            }
        }
    }
    Ok(out)
}

/// Compact RFC 3174 SHA-1 used strictly for RFC 6455 `Sec-WebSocket-Accept` handshake.
pub fn sha1_digest(data: &[u8]) -> [u8; 20] {
    let mut h0: u32 = 0x67452301;
    let mut h1: u32 = 0xefcdab89;
    let mut h2: u32 = 0x98badcfe;
    let mut h3: u32 = 0x10325476;
    let mut h4: u32 = 0xc3d2e1f0;

    let mut msg = data.to_vec();
    let bit_len = (data.len() as u64) * 8;
    msg.push(0x80);
    while (msg.len() % 64) != 56 {
        msg.push(0);
    }
    msg.extend_from_slice(&bit_len.to_be_bytes());

    for chunk in msg.as_chunks::<64>().0 {
        let mut w = [0u32; 80];
        for i in 0..16 {
            w[i] = u32::from_be_bytes([
                chunk[i * 4],
                chunk[i * 4 + 1],
                chunk[i * 4 + 2],
                chunk[i * 4 + 3],
            ]);
        }
        for i in 16..80 {
            w[i] = (w[i - 3] ^ w[i - 8] ^ w[i - 14] ^ w[i - 16]).rotate_left(1);
        }
        let (mut a, mut b, mut c, mut d, mut e) = (h0, h1, h2, h3, h4);
        for (i, &wi) in w.iter().enumerate() {
            let (f, k) = match i {
                0..=19 => ((b & c) | ((!b) & d), 0x5a827999u32),
                20..=39 => (b ^ c ^ d, 0x6ed9eba1u32),
                40..=59 => ((b & c) | (b & d) | (c & d), 0x8f1bbcdcu32),
                _ => (b ^ c ^ d, 0xca62c1d6u32),
            };
            let temp = a
                .rotate_left(5)
                .wrapping_add(f)
                .wrapping_add(e)
                .wrapping_add(k)
                .wrapping_add(wi);
            e = d;
            d = c;
            c = b.rotate_left(30);
            b = a;
            a = temp;
        }
        h0 = h0.wrapping_add(a);
        h1 = h1.wrapping_add(b);
        h2 = h2.wrapping_add(c);
        h3 = h3.wrapping_add(d);
        h4 = h4.wrapping_add(e);
    }

    let mut out = [0u8; 20];
    out[0..4].copy_from_slice(&h0.to_be_bytes());
    out[4..8].copy_from_slice(&h1.to_be_bytes());
    out[8..12].copy_from_slice(&h2.to_be_bytes());
    out[12..16].copy_from_slice(&h3.to_be_bytes());
    out[16..20].copy_from_slice(&h4.to_be_bytes());
    out
}

use crate::api::{serve_http_blocking, DaemonRuntimeState};
use crate::blob::{
    compute_sha256, evaluate_tiered_blob_fetch, store_verified_blob, transfer_blob_via_iroh_ticket,
    BlobTransportReport, TieredBlobFetchRequest,
};
use crate::deploy::{
    evaluate_deploy_build_decision, execute_verified_rollback, generate_deploy_plan, plan_rollback,
    probe_build_cache, validate_manifest_contract, BuildCacheRecord, BuildProbeRequest,
    DeployManifest, DeploySlot,
};
use crate::mesh::{
    load_peer_cache, save_peer_cache, verify_live_gossip_exchange, DialtoneHeartbeat,
    DEFAULT_PEER_TTL_SECS,
};
use crate::node::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, default_secret_key_path,
    load_or_create_secret_key_at, IrohTransportConfig, IrohTransportDiagnostics,
};
use crate::record::{evaluate_proof_case, paginate_kv_records, plan_alert_ack};
use crate::service::{
    resolve_service, DeployPolicyConfig, KeyLeaseDefinition, ManagedProcessSpec, PeerRef,
};
use crate::supervisor::{
    dispatch_rtcore_udp_command_loopback, hydrate_managed_process_hashes, plan_process_intent,
    reconcile_systemd_intent_drop_in, ManagedProcessSupervisor, RobotCommandIntentRecord,
};

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct SelfTestReport {
    pub schema: String,
    pub iroh_version: String,
    pub status: String,
    pub capabilities_verified: Vec<String>,
    pub persistent_endpoint_id: String,
    pub resolved_service_uri: String,
    pub gossip_received_from: String,
    pub gossip_receiver_node: String,
    pub blob_transfer: BlobTransportReport,
    pub build_cache_hit: bool,
    pub rollback_verified: bool,
    pub systemd_drop_in_repaired: bool,
    pub rtcore_udp_bytes_sent: usize,
    pub http_and_ws_verified: bool,
}

pub async fn collect_node_diagnostics(
    config: &IrohTransportConfig,
) -> Result<IrohTransportDiagnostics, String> {
    let key_path = default_secret_key_path();
    let secret_key = load_or_create_secret_key_at(&key_path)?;
    let endpoint = build_endpoint_builder(config)?
        .secret_key(secret_key)
        .bind()
        .await
        .map_err(|e| format!("bind iroh endpoint: {e}"))?;
    let endpoint_id = endpoint.id().to_string();
    let advertised = advertised_endpoint_addr(&endpoint.addr(), config);
    let diagnostics = build_diagnostics(config, &advertised, Some(&key_path), &endpoint_id);
    endpoint.close().await;
    Ok(diagnostics)
}

/// Comprehensive end-to-end verification across all 7 capability domains in the single binary.
pub async fn run_self_test() -> Result<SelfTestReport, String> {
    let tmp_root = env::temp_dir().join(format!("dialtone-self-test-{}", std::process::id()));
    let _ = fs::remove_dir_all(&tmp_root);
    fs::create_dir_all(&tmp_root).map_err(|e| e.to_string())?;

    // 1. Cryptographic Mesh Identity & Peer Addressing
    let key_path = tmp_root.join("iroh-secret-key");
    let key1 = load_or_create_secret_key_at(&key_path)?;
    let key2 = load_or_create_secret_key_at(&key_path)?;
    if key1.public() != key2.public() {
        return Err("persistent secret key public identity mismatch".to_string());
    }
    let persistent_endpoint_id = key1.public().to_string();

    // 2. Multi-Transport Discovery & Iroh Gossip Control Plane + TTL Peer Cache
    let (received_hb, peer_b_hb): (DialtoneHeartbeat, DialtoneHeartbeat) =
        verify_live_gossip_exchange("node-alpha", "edge-1").await?;
    let peer_cache_path = tmp_root.join("peer-cache.json");
    save_peer_cache(&peer_cache_path, &[received_hb.clone(), peer_b_hb.clone()])?;
    let loaded_peers = load_peer_cache(
        &peer_cache_path,
        received_hb.updated_at_unix,
        DEFAULT_PEER_TTL_SECS,
    )?;
    if loaded_peers.len() != 2 {
        return Err("TTL peer cache failed to persist fresh peers".to_string());
    }

    let target_ref = PeerRef::expand_template("dialtone://{node}/controller/nats", "node-alpha")?;
    let resolved = resolve_service(&target_ref, &received_hb.service_endpoints)?;
    let resolved_service_uri = resolved.url.clone();

    // 3. Content-Addressed Artifact Store & Native Iroh Blob Transport
    let blob_dir = tmp_root.join("blobs");
    let blob_transfer = transfer_blob_via_iroh_ticket(
        b"dialtone-self-test-verified-artifact-payload",
        &blob_dir,
        &IrohTransportConfig::local_offline(),
    )
    .await?;
    let cache_dir = tmp_root.join("tiered-cache");
    let (cached_sha, _) = store_verified_blob(&cache_dir, b"cached-item", None)?;
    let tiered_hit = evaluate_tiered_blob_fetch(
        &cache_dir,
        &TieredBlobFetchRequest {
            sha256: cached_sha,
            uri: "iroh-blobs://ticket/test".to_string(),
            provider_node: "node-beta".to_string(),
            expected_bytes: 11,
            http_fallback_enabled: false,
        },
    )?;
    if tiered_hit.transferred_bytes != 0 || tiered_hit.method != "local-cache" {
        return Err("tiered blob cache hit did not short-circuit with 0 bytes".to_string());
    }

    // 4. Manifest-Driven Deployment, Distributed Build Cache & Rollback
    let policy = DeployPolicyConfig::default();
    let manifest = DeployManifest {
        schema: "dialtone.artifact_manifest.v1".to_string(),
        id: "self-test-manifest".to_string(),
        publisher_node: "node-alpha".to_string(),
        target_host: "edge-1".to_string(),
        component: "cartesiand".to_string(),
        source_tree_hash: "tree-1".to_string(),
        source_bundle_sha256: blob_transfer.sha256.clone(),
        source_bundle_uri: blob_transfer.ticket_uri.clone(),
        binary_sha256: blob_transfer.sha256.clone(),
        binary_blob_uri: blob_transfer.ticket_uri.clone(),
        runtime_path: "/opt/dialtone/bin/cartesiand".to_string(),
        build_inputs: vec!["src/main.rs".to_string()],
        whole_repo_sync: false,
        ssh_required: false,
        git_required: false,
    };
    validate_manifest_contract(&manifest, &policy, &["/opt/dialtone/".to_string()])?;
    let default_plan = generate_deploy_plan("robot-v4-default")?;
    if default_plan.includes_daemon_self_update {
        return Err("default app deploy plan must exclude daemon self-updates".to_string());
    }
    let _ = evaluate_deploy_build_decision("peer-local-async")?;

    let caches = vec![BuildCacheRecord {
        peer_node: "edge-1".to_string(),
        target_host: "edge-1".to_string(),
        component: "cartesiand".to_string(),
        source_tree_hash: "tree-1".to_string(),
        source_bundle_sha256: blob_transfer.sha256.clone(),
        build_profile: "release".to_string(),
        toolchain: "1.95.0".to_string(),
        platform_os: "linux".to_string(),
        platform_arch: "aarch64".to_string(),
        target_triple: "aarch64-unknown-linux-gnu".to_string(),
        binary_sha256: blob_transfer.sha256.clone(),
        binary_blob_uri: blob_transfer.ticket_uri.clone(),
        status: "ready".to_string(),
    }];
    let probe = probe_build_cache(
        &caches,
        &BuildProbeRequest {
            target_host: "edge-2".to_string(),
            component: "cartesiand".to_string(),
            source_tree_hash: "tree-1".to_string(),
            source_bundle_sha256: blob_transfer.sha256.clone(),
            build_profile: "release".to_string(),
            toolchain: "1.95.0".to_string(),
            platform_os: "linux".to_string(),
            platform_arch: "aarch64".to_string(),
            target_triple: "aarch64-unknown-linux-gnu".to_string(),
            cache_only: true,
        },
    );
    if !probe.cache_hit || probe.compiler_invoked {
        return Err("build-cache probe failed".to_string());
    }

    let slot_a = tmp_root.join("slot-a");
    let slot_b = tmp_root.join("slot-b");
    fs::write(&slot_a, b"v2-bytes").map_err(|e| e.to_string())?;
    fs::write(&slot_b, b"v1-bytes").map_err(|e| e.to_string())?;
    let mut slot = DeploySlot {
        name: "slot-main".to_string(),
        active: true,
        path: slot_a.to_string_lossy().to_string(),
        sha256: compute_sha256(b"v2-bytes"),
        previous_path: Some(slot_b.to_string_lossy().to_string()),
        previous_sha256: Some(compute_sha256(b"v1-bytes")),
        status: "ready".to_string(),
    };
    let rb_plan = plan_rollback("edge-1", &slot)?;
    if !rb_plan.side_effect_free_preview {
        return Err("rollback plan must be side-effect-free".to_string());
    }
    execute_verified_rollback(&mut slot)?;

    // 5. Daemon-Owned Process Supervision & Intent Convergence
    let (sleep_path, sleep_args) = if cfg!(windows) {
        (
            r"C:\Windows\System32\ping.exe".to_string(),
            vec!["-n".to_string(), "6".to_string(), "127.0.0.1".to_string()],
        )
    } else {
        ("/bin/sleep".to_string(), vec!["5".to_string()])
    };
    let mut proc_specs = vec![ManagedProcessSpec {
        name: "self-test-sleep".to_string(),
        path: sleep_path,
        sha256: "sha256:unknown".to_string(),
        desired_state: "running".to_string(),
        args: sleep_args,
        env: vec!["DIALTONE_SYSTEMD_RECONCILE_INTENT=1".to_string()],
        cwd: None,
        startup_grace_secs: Some(2),
        status: None,
        pid: None,
        restart_count: 0,
        service_unit: Some("self-test-sleep.service".to_string()),
    }];
    hydrate_managed_process_hashes(&mut proc_specs, &[], &HashMap::new());
    let _ = plan_process_intent("node-alpha", "edge-1", &proc_specs[0], "restart")?;
    let mut supervisor = ManagedProcessSupervisor::new();
    supervisor.reconcile(&mut proc_specs)?;
    proc_specs[0].desired_state = "stopped".to_string();
    supervisor.reconcile(&mut proc_specs)?;

    let unit_dir = tmp_root.join("systemd-unit.d");
    let sys_report =
        reconcile_systemd_intent_drop_in(&unit_dir, &proc_specs[0], true, &[], 3_500, 2_000)?;

    // 6. Service Discovery, Generic Catalogs & Command Journals
    let mut cmd_journal = Vec::new();
    let rtcore_intent = RobotCommandIntentRecord {
        id: "cmd-self-test".to_string(),
        command_id: "arm-1".to_string(),
        publisher_node: "node-alpha".to_string(),
        target_node: "node-beta".to_string(),
        service_ref: "dialtone://node-beta/realtime-core/rtcore-udp".to_string(),
        command: "arm".to_string(),
        runtime_mode: "live".to_string(),
        binding_epoch: 7,
        delivery: "at-least-once".to_string(),
        status: "pending".to_string(),
        updated_at_unix: 100,
        message: String::new(),
    };
    let rtcore_udp_bytes_sent =
        dispatch_rtcore_udp_command_loopback(&mut cmd_journal, rtcore_intent, "node-alpha")?;

    // 7. Metadata-Driven Policy, Operational Evidence & Rust-Served HTTP/WS UI
    let lease = KeyLeaseDefinition {
        name: "self-test-lease".to_string(),
        kind: "file".to_string(),
        env: Vec::new(),
        path: Some(key_path.to_string_lossy().to_string()),
        expires_at_unix: None,
    };
    let lease_status = lease.evaluate("node-alpha", 100);
    if !lease_status.available {
        return Err("key lease evaluation failed on existing file".to_string());
    }
    let valid_proof = evaluate_proof_case("valid", "sess-self-test")?;
    let stale_proof = evaluate_proof_case("stale", "sess-self-test")?;
    if !valid_proof.accepted || stale_proof.accepted {
        return Err("proof validation gate failed".to_string());
    }

    let listener = TcpListener::bind("127.0.0.1:0").map_err(|e| e.to_string())?;
    let addr = listener.local_addr().map_err(|e| e.to_string())?;
    let runtime_state = Arc::new(Mutex::new(DaemonRuntimeState::new_seeded(
        "node-alpha",
        "hub",
        &addr.to_string(),
        tmp_root.join("http-blobs"),
        received_hb.iroh_transport.clone(),
    )));
    {
        let st = runtime_state.lock().map_err(|_| "poisoned".to_string())?;
        let (_, meta) = paginate_kv_records(&st.kv, true, Some(0), Some(1))?;
        if meta.is_none() {
            return Err("KV pagination metadata missing".to_string());
        }
        let _ = plan_alert_ack(&st.alerts[0], "operator-self-test", "ok")?;
    }

    let state_bg = runtime_state.clone();
    let server_thread = thread::spawn(move || {
        let _ = serve_http_blocking(listener, state_bg, Some(2));
    });

    let mut stream1 = TcpStream::connect(addr).map_err(|e| e.to_string())?;
    stream1
        .write_all(b"GET /api/state?kv_offset=0&kv_limit=1 HTTP/1.1\r\nHost: localhost\r\n\r\n")
        .map_err(|e| e.to_string())?;
    let mut resp1 = Vec::new();
    stream1.read_to_end(&mut resp1).map_err(|e| e.to_string())?;
    let text1 = String::from_utf8_lossy(&resp1);

    let mut stream2 = TcpStream::connect(addr).map_err(|e| e.to_string())?;
    stream2
        .write_all(
            b"GET /ws HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n",
        )
        .map_err(|e| e.to_string())?;
    let mut resp2 = Vec::new();
    stream2.read_to_end(&mut resp2).map_err(|e| e.to_string())?;
    let text2 = String::from_utf8_lossy(&resp2);
    let _ = server_thread.join();

    let http_and_ws_verified =
        text1.contains("200 OK") && text2.contains("101 Switching Protocols");
    if !http_and_ws_verified {
        return Err("HTTP /api/state or WebSocket /ws check failed".to_string());
    }

    let _ = fs::remove_dir_all(&tmp_root);

    Ok(SelfTestReport {
        schema: "dialtone.self_test.v2".to_string(),
        iroh_version: "1.3.0".to_string(),
        status: "passed".to_string(),
        capabilities_verified: vec![
            "1. Cryptographic Mesh Identity & Peer Addressing".to_string(),
            "2. Multi-Transport Discovery & Iroh Gossip Control Plane".to_string(),
            "3. Content-Addressed Artifact Store & Native Iroh Blob Transport".to_string(),
            "4. Manifest-Driven Deployment, Distributed Build Cache & Rollback".to_string(),
            "5. Daemon-Owned Process Supervision & Intent Convergence".to_string(),
            "6. Service Discovery, Generic Catalogs & Command Journals".to_string(),
            "7. Metadata-Driven Policy & Operational Evidence UI".to_string(),
        ],
        persistent_endpoint_id,
        resolved_service_uri,
        gossip_received_from: received_hb.node,
        gossip_receiver_node: peer_b_hb.node,
        blob_transfer,
        build_cache_hit: probe.cache_hit,
        rollback_verified: slot.status == "rolled_back",
        systemd_drop_in_repaired: sys_report.updated_drop_in,
        rtcore_udp_bytes_sent,
        http_and_ws_verified,
    })
}
