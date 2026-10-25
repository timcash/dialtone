pub mod api;
pub mod blob;
pub mod deploy;
pub mod devices;
pub mod h3_land;
pub mod hud;
pub mod mesh;
pub mod node;
pub mod record;
pub mod service;
pub mod stats;
pub mod supervisor;

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
    hex::encode(bytes)
}

pub fn hex_decode(raw: &str) -> Result<Vec<u8>, String> {
    hex::decode(raw.trim()).map_err(|e| format!("invalid hex: {e}"))
}

/// Standard alphabet, any padding, no trailing-bit check: decodes whatever peers and older
/// builds produced (standard or URL-safe after normalisation, padded or not).
const BASE64_LENIENT: base64::engine::GeneralPurpose = base64::engine::GeneralPurpose::new(
    &base64::alphabet::STANDARD,
    base64::engine::GeneralPurposeConfig::new()
        .with_encode_padding(true)
        .with_decode_padding_mode(base64::engine::DecodePaddingMode::Indifferent)
        .with_decode_allow_trailing_bits(true),
);

pub fn base64_encode(bytes: &[u8]) -> String {
    base64::Engine::encode(&base64::engine::general_purpose::STANDARD, bytes)
}

pub fn base64url_encode(bytes: &[u8]) -> String {
    base64::Engine::encode(&base64::engine::general_purpose::URL_SAFE_NO_PAD, bytes)
}

/// Decode standard or URL-safe base64, with or without padding; whitespace is ignored.
pub fn base64_decode(raw: &str) -> Result<Vec<u8>, String> {
    let normalised: String = raw
        .chars()
        .filter(|c| !c.is_ascii_whitespace() && *c != '=')
        .map(|c| match c {
            '-' => '+',
            '_' => '/',
            other => other,
        })
        .collect();
    base64::Engine::decode(&BASE64_LENIENT, normalised).map_err(|e| format!("invalid base64: {e}"))
}

use crate::api::{serve_http_blocking, NodeState};
use crate::blob::{digest_of, transfer_blob_via_iroh_ticket, BlobTransportReport};
use crate::deploy::{execute_verified_rollback, plan_rollback, DeploySlot};
use crate::mesh::{verify_live_gossip_exchange, Beat};
use crate::node::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, default_secret_key_path,
    load_or_create_secret_key_at, IrohTransportConfig, IrohTransportDiagnostics,
};
use crate::record::{paginate_kv_records, KeyValueRecord};
use crate::service::ProcessSpec;
use crate::supervisor::Supervisor;

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct SelfTestReport {
    pub schema: String,
    pub iroh_version: String,
    pub status: String,
    pub capabilities_verified: Vec<String>,
    #[serde(alias = "persistent_endpoint_id")] // compat
    pub persistent_node_id: String,
    pub gossip_received_from: String,
    pub gossip_receiver_node: String,
    pub blob_transfer: BlobTransportReport,
    pub rollback_verified: bool,
    pub process_started_and_stopped: bool,
    pub api_verified: bool,
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
    let node_id = endpoint.id().to_string();
    let advertised = advertised_endpoint_addr(&endpoint.addr(), config);
    let diagnostics = build_diagnostics(config, &advertised, Some(&key_path), &node_id);
    endpoint.close().await;
    Ok(diagnostics)
}

/// End-to-end check of the node's real capabilities, each exercised for real (no fixtures):
/// Key persistence, a live two-node gossip exchange (signed beats, allowlist hooks), a live
/// iroh-blobs transfer, A/B slot rollback, process supervision, KV paging, and the API.
pub async fn run_self_test() -> Result<SelfTestReport, String> {
    let tmp_root = env::temp_dir().join(format!("dialtone-self-test-{}", std::process::id()));
    let _ = fs::remove_dir_all(&tmp_root);
    fs::create_dir_all(&tmp_root).map_err(|e| e.to_string())?;

    // 1. Key: the node's identity persists across loads.
    let key_path = tmp_root.join("iroh-secret-key");
    let key1 = load_or_create_secret_key_at(&key_path)?;
    let key2 = load_or_create_secret_key_at(&key_path)?;
    if key1.public() != key2.public() {
        return Err("persistent Key public identity mismatch".to_string());
    }
    let persistent_node_id = key1.public().to_string();

    // 2. Mesh: two nodes that allowlist only each other exchange a signed, encrypted beat.
    let (received_beat, peer_b_beat): (Beat, Beat) =
        verify_live_gossip_exchange("node-alpha", "edge-1").await?;

    // 3. Blob: a blob crosses iroh-blobs QUIC and its digest is re-verified.
    let blob_transfer = transfer_blob_via_iroh_ticket(
        b"dialtone-self-test-verified-blob",
        &tmp_root.join("blobs"),
        &IrohTransportConfig::local_offline(),
    )
    .await?;

    // 4. Slot: an A/B rollback is planned without side effects, then executed with digest checks.
    let slot_a = tmp_root.join("slot-a");
    let slot_b = tmp_root.join("slot-b");
    fs::write(&slot_a, b"v2-bytes").map_err(|e| e.to_string())?;
    fs::write(&slot_b, b"v1-bytes").map_err(|e| e.to_string())?;
    let mut slot = DeploySlot {
        name: "slot-main".to_string(),
        active: true,
        path: slot_a.to_string_lossy().to_string(),
        digest: digest_of(b"v2-bytes"),
        previous_path: Some(slot_b.to_string_lossy().to_string()),
        previous_digest: Some(digest_of(b"v1-bytes")),
        status: "ready".to_string(),
    };
    if !plan_rollback("edge-1", &slot)?.side_effect_free_preview {
        return Err("rollback plan must be side-effect-free".to_string());
    }
    execute_verified_rollback(&mut slot)?;

    // 5. Supervisor: a real process is started, then stopped.
    let (sleep_path, sleep_args) = if cfg!(windows) {
        (
            r"C:\Windows\System32\ping.exe".to_string(),
            vec!["-n".to_string(), "6".to_string(), "127.0.0.1".to_string()],
        )
    } else {
        ("/bin/sleep".to_string(), vec!["5".to_string()])
    };
    let mut specs = vec![ProcessSpec {
        name: "self-test-sleep".to_string(),
        path: sleep_path,
        digest: "sha256:self-test".to_string(),
        desired_state: "running".to_string(),
        args: sleep_args,
        env: Vec::new(),
        cwd: None,
        startup_grace_secs: None,
        status: None,
        pid: None,
        restart_count: 0,
        service_unit: None,
        deploy_ticket: None,
    }];
    let mut supervisor = Supervisor::new();
    supervisor.reconcile(&mut specs)?;
    let started = specs[0].status.as_deref() == Some("running") && specs[0].pid.is_some();
    specs[0].desired_state = "stopped".to_string();
    supervisor.reconcile(&mut specs)?;
    let process_started_and_stopped = started && specs[0].pid.is_none();
    if !process_started_and_stopped {
        return Err("supervisor did not start and stop the process".to_string());
    }

    // 6. Record: KV records page with bounds.
    let kv: Vec<KeyValueRecord> = (0..3)
        .map(|i| KeyValueRecord {
            key: format!("self-test/{i}"),
            value: serde_json::json!(i),
            updated_at_unix: now_unix(),
        })
        .collect();
    let (page, meta) = paginate_kv_records(&kv, true, Some(1), Some(1))?;
    if page.len() != 1 || meta.is_none() {
        return Err("KV pagination returned the wrong page".to_string());
    }

    // 7. API: a node serves its live state.
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|e| e.to_string())?;
    let addr = listener.local_addr().map_err(|e| e.to_string())?;
    let state = Arc::new(Mutex::new(NodeState::new_live(
        "node-alpha",
        "up",
        &addr.to_string(),
        crate::blob::Blobs::open_blocking(&tmp_root.join("api-blobs"))?,
        received_beat.iroh_transport.clone(),
    )));
    let state_bg = state.clone();
    let server_thread = thread::spawn(move || {
        let _ = serve_http_blocking(listener, state_bg, Some(1));
    });
    let mut stream = TcpStream::connect(addr).map_err(|e| e.to_string())?;
    stream
        .write_all(b"GET /api/state HTTP/1.1\r\nHost: localhost\r\n\r\n")
        .map_err(|e| e.to_string())?;
    let mut resp = Vec::new();
    stream.read_to_end(&mut resp).map_err(|e| e.to_string())?;
    let _ = server_thread.join();
    let api_verified = String::from_utf8_lossy(&resp).contains("200 OK");
    if !api_verified {
        return Err("API GET /api/state check failed".to_string());
    }

    let _ = fs::remove_dir_all(&tmp_root);

    Ok(SelfTestReport {
        schema: "dialtone.self_test.v3".to_string(),
        iroh_version: "1.3.0".to_string(),
        status: "passed".to_string(),
        capabilities_verified: vec![
            "1. Key: persistent node identity".to_string(),
            "2. Mesh: signed, encrypted beat between allowlisted nodes over iroh-gossip"
                .to_string(),
            "3. Blob: iroh-blobs transfer with digest re-verification".to_string(),
            "4. Slot: side-effect-free rollback plan and verified A/B flip".to_string(),
            "5. Supervisor: real process start and stop".to_string(),
            "6. Record: bounded KV paging".to_string(),
            "7. API: live node state served over HTTP".to_string(),
        ],
        persistent_node_id,
        gossip_received_from: received_beat.node,
        gossip_receiver_node: peer_b_beat.node,
        blob_transfer,
        rollback_verified: slot.status == "rolled_back",
        process_started_and_stopped,
        api_verified,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The encoders replaced hand-written code; these vectors are what that code produced and
    /// accepted (beat envelopes and API payloads from older builds must still decode).
    #[test]
    fn hex_and_base64_match_the_previous_hand_written_codecs() {
        assert_eq!(hex_encode(&[0x0a, 0xff, 0x10]), "0aff10");
        assert_eq!(hex_decode(" 0AfF10 ").unwrap(), vec![0x0a, 0xff, 0x10]);
        assert!(hex_decode("abc").is_err());

        assert_eq!(base64_encode(b"dialtone"), "ZGlhbHRvbmU=");
        assert_eq!(base64url_encode(b"dialtone"), "ZGlhbHRvbmU");
        assert_eq!(base64_encode(&[0xfb, 0xff]), "+/8=");
        assert_eq!(base64url_encode(&[0xfb, 0xff]), "-_8");
        for form in ["ZGlhbHRvbmU=", "ZGlhbHRvbmU", "ZGlh bHRv\nbmU="] {
            assert_eq!(base64_decode(form).unwrap(), b"dialtone", "{form}");
        }
        for form in ["+/8=", "+/8", "-_8", "-_8="] {
            assert_eq!(base64_decode(form).unwrap(), vec![0xfb, 0xff], "{form}");
        }
        assert!(base64_decode("@@@@").is_err());
    }
}
