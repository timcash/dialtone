use std::collections::HashMap;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

use crate::blobs::{
    evaluate_tiered_blob_fetch, has_cached_blob, read_cached_blob, store_verified_blob,
    BlobTransportReport, TieredBlobFetchRequest,
};
use crate::deploy::{
    evaluate_deploy_build_decision, generate_deploy_plan, install_persistent_os_daemon,
    plan_rollback, probe_build_cache, render_install_v1_sh, validate_manifest_contract,
    BuildCacheRecord, BuildProbeRequest, DeployManifest, DeploySlot,
};
use crate::evidence::{
    paginate_kv_records, plan_alert_ack, sample_live_host_resources, AlertRecord, EventRecord,
    HostResourceTelemetry, KeyValueRecord, KvPageMeta, ServiceLogRecord,
};
use crate::gossip::{
    default_arch, default_config_path, default_os, default_state_dir, node_capabilities_for,
    node_role_for, normalize_channel_name, now_unix, prune_stale_peers, save_mesh_config,
    secret_channel_fingerprint, validate_channel_password, BootStageStatus, DialtoneHeartbeat,
    MeshControlCommand, PersistentMeshConfig, DEFAULT_CHANNEL_NAME, DEFAULT_PEER_TTL_SECS,
};
use crate::identity::IrohTransportDiagnostics;
use crate::journal::{upsert_command_intent, CursorJobRecord, RobotCommandIntentRecord};
use crate::metadata::{
    CatalogPolicy, DialtoneMetadata, InstalledBinary, KeyLeaseStatus, ManagedProcessSpec,
};
use crate::refs::ServiceAdvertisement;
use crate::supervisor::{plan_process_intent, stage_executable_blob, ManagedProcessSupervisor};

pub const MAX_HTTP_BODY_BYTES: usize = 2 * 1024 * 1024;
pub const MAX_CONCURRENT_HTTP_CONNECTIONS: usize = 32;
pub const HTTP_IO_TIMEOUT: Duration = Duration::from_secs(2);

pub fn default_api_token_path() -> PathBuf {
    default_state_dir().join("api-token")
}

/// Load or generate a persistent 32-byte random API token stored in `~/.dialtone/api-token` (`0600`) (Issue #7).
pub fn load_or_create_api_token() -> Result<String, String> {
    if let Ok(env_tok) = std::env::var("DIALTONE_API_TOKEN") {
        let trimmed = env_tok.trim();
        if !trimmed.is_empty() {
            return Ok(trimmed.to_string());
        }
    }
    let path = default_api_token_path();
    if let Ok(raw) = fs::read_to_string(&path) {
        let trimmed = raw.trim();
        if trimmed.len() >= 16 {
            return Ok(trimmed.to_string());
        }
    }
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create api-token dir {}: {e}", parent.display()))?;
    }
    let rand_bytes = iroh::SecretKey::generate().to_bytes();
    let token = format!("dtk-{}", crate::base64url_encode(&rand_bytes));
    fs::write(&path, format!("{token}\n"))
        .map_err(|e| format!("write api-token {}: {e}", path.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let _ = fs::set_permissions(&path, fs::Permissions::from_mode(0o600));
    }
    Ok(token)
}

/// Refuse non-loopback `--listen` addresses unless an explicit API token has been configured (Issue #7).
pub fn validate_listen_addr_security(
    listen: &str,
    explicit_token_configured: bool,
) -> Result<(), String> {
    let trimmed = listen.trim();
    let host_part = if let Some(rest) = trimmed.strip_prefix('[') {
        rest.split(']').next().unwrap_or("")
    } else {
        trimmed.rsplit_once(':').map(|(h, _)| h).unwrap_or(trimmed)
    };
    let is_loopback = matches!(
        host_part.trim().to_ascii_lowercase().as_str(),
        "127.0.0.1" | "localhost" | "::1"
    );
    if !is_loopback && !explicit_token_configured {
        return Err(format!(
            "refusing non-loopback --listen address '{listen}' without an explicit API token (pass --api-token <token> or set DIALTONE_API_TOKEN)"
        ));
    }
    Ok(())
}

pub fn default_boot_stages(listen: &str, blob_dir: &std::path::Path) -> Vec<BootStageStatus> {
    let now = now_unix();
    vec![
        BootStageStatus {
            step: 1,
            id: "local_runtime".to_string(),
            title: "Local Runtime & Stdout Logger".to_string(),
            status: "ready".to_string(),
            detail: "Structured stdout logging and local state initialized".to_string(),
            updated_at_unix: now,
        },
        BootStageStatus {
            step: 2,
            id: "local_web_server".to_string(),
            title: "Local Web HUD & Config Server".to_string(),
            status: "ready".to_string(),
            detail: format!("Serving 3D HUD and config API on http://{listen}"),
            updated_at_unix: now,
        },
        BootStageStatus {
            step: 3,
            id: "blob_store".to_string(),
            title: "Content-Addressed Blob Store (iroh-blobs)".to_string(),
            status: "ready".to_string(),
            detail: format!("SHA-256 CAS ready at {}", blob_dir.display()),
            updated_at_unix: now,
        },
        BootStageStatus {
            step: 4,
            id: "iroh_endpoint".to_string(),
            title: "Iroh 1.3 QUIC Endpoint (No-Tailnet NIC)".to_string(),
            status: "ready".to_string(),
            detail: "Persistent Ed25519 identity bound to physical IPv4".to_string(),
            updated_at_unix: now,
        },
        BootStageStatus {
            step: 5,
            id: "gossip_mesh".to_string(),
            title: "Gossip Mesh & Channel Password Gate".to_string(),
            status: "ready".to_string(),
            detail:
                "Protected 'dialtone' channel (enter password in Web UI or run 'dialtone join <PASSWORD>')"
                    .to_string(),
            updated_at_unix: now,
        },
    ]
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct DaemonRuntimeState {
    pub node: String,
    pub mode: String,
    pub listen: String,
    pub blob_cache_dir: PathBuf,
    pub allowed_deploy_roots: Vec<String>,
    #[serde(skip)]
    pub api_token: String,
    #[serde(default)]
    pub boot_stages: Vec<BootStageStatus>,
    #[serde(default)]
    pub mesh_config: PersistentMeshConfig,
    #[serde(skip)]
    pub control_tx: Option<tokio::sync::mpsc::UnboundedSender<MeshControlCommand>>,
    #[serde(skip)]
    pub supervisor: Arc<Mutex<ManagedProcessSupervisor>>,
    pub metadata: DialtoneMetadata,
    pub iroh_runtime: IrohTransportDiagnostics,
    pub nodes: Vec<DialtoneHeartbeat>,
    pub installed_binaries: Vec<InstalledBinary>,
    pub managed_processes: Vec<ManagedProcessSpec>,
    pub deploy_slots: Vec<DeploySlot>,
    pub deploy_manifests: Vec<DeployManifest>,
    pub deploy_build_caches: Vec<BuildCacheRecord>,
    pub blob_fetches: Vec<BlobTransportReport>,
    pub service_endpoints: Vec<ServiceAdvertisement>,
    pub key_leases: Vec<KeyLeaseStatus>,
    pub host_resources: HostResourceTelemetry,
    pub events: Vec<EventRecord>,
    pub logs: Vec<ServiceLogRecord>,
    pub alerts: Vec<AlertRecord>,
    pub robot_command_intents: Vec<RobotCommandIntentRecord>,
    pub cursor_jobs: Vec<CursorJobRecord>,
    pub kv: Vec<KeyValueRecord>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kv_page: Option<KvPageMeta>,
}

impl DaemonRuntimeState {
    pub fn reconcile_live_processes(&mut self) {
        let sup_arc = self.supervisor.clone();
        if let Ok(mut sup) = sup_arc.lock() {
            for spec in self.managed_processes.iter_mut() {
                if std::path::Path::new(&spec.path).exists() {
                    let prev_pid = spec.pid;
                    let _ = sup.reconcile(std::slice::from_mut(spec));
                    if spec.desired_state == "running" && spec.pid.is_some() && spec.pid != prev_pid
                    {
                        println!(
                            "[SUPERVISOR] Supervised blob process '{}' active (sha256={}, pid={}, path={})",
                            spec.name,
                            spec.sha256,
                            spec.pid.unwrap_or(0),
                            spec.path
                        );
                    }
                }
            }
        }
        let self_node = self.node.clone();
        let procs_snapshot = self.managed_processes.clone();
        if let Some(self_hb) = self.nodes.iter_mut().find(|n| n.node == self_node) {
            self_hb.managed_processes = procs_snapshot;
        }
    }

    pub fn new_seeded(
        node: &str,
        mode: &str,
        listen: &str,
        blob_cache_dir: PathBuf,
        iroh_runtime: IrohTransportDiagnostics,
    ) -> Self {
        let now = now_unix();
        let sample_sha =
            "sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d".to_string();
        let prev_sha =
            "sha256:a1892e88f06224d9bd720a5f5a2c9fd70184c4eaf887f96e7df89f229484ffb1".to_string();
        let service_endpoints = vec![ServiceAdvertisement {
            name: "robot-v4-nats".to_string(),
            kind: "nats-jetstream".to_string(),
            protocol: "nats".to_string(),
            url: format!("dialtone://{node}/controller/robot-v4-nats"),
            listen: "127.0.0.1:4222".to_string(),
            status: "running".to_string(),
            advertised_by: node.to_string(),
            role: "controller".to_string(),
            mesh: "dialtone-iroh-gossip".to_string(),
            subjects: vec![
                "dialtone/events.>".to_string(),
                "dialtone/test.>".to_string(),
            ],
            status_probe: None,
        }];

        let mut peer_heartbeats = Vec::new();
        for peer_name in ["node-alpha", "node-beta", "edge-1", "pendant-1"] {
            peer_heartbeats.push(DialtoneHeartbeat {
                schema: "dialtone.iroh.heartbeat.v1".to_string(),
                node: peer_name.to_string(),
                endpoint_id: iroh_runtime.endpoint_id.clone(),
                ticket: format!("iroh-gossip-v1:{peer_name}"),
                counter: 1,
                updated_at_unix: now,
                channel_mac: None,
                os: default_os(),
                arch: default_arch(),
                role: node_role_for(peer_name),
                capabilities: node_capabilities_for(peer_name),
                host_resources: sample_live_host_resources(
                    peer_name,
                    iroh_runtime.bound_non_tailnet_ipv4.as_deref(),
                    1,
                ),
                service_endpoints: if peer_name == node {
                    service_endpoints.clone()
                } else {
                    Vec::new()
                },
                managed_processes: Vec::new(),
                iroh_transport: iroh_runtime.clone(),
                observed_remote_addrs: Vec::new(),
                tailnet_used: false,
            });
        }

        let mut metadata = DialtoneMetadata::default();
        metadata.catalogs = vec![
            CatalogPolicy {
                name: "programs".to_string(),
                schema: "dialtone.program_catalog.v1".to_string(),
                owner: "motion-server".to_string(),
                id_field: "program_id".to_string(),
                url_path: "/api/program-catalog".to_string(),
                entries: vec![json!({
                    "program_id": "row-05-calibration",
                    "program_name": "Row 05 Calibration Loop",
                    "row_count": 12
                })],
            },
            CatalogPolicy {
                name: "jobs".to_string(),
                schema: "dialtone.jobs_catalog.v1".to_string(),
                owner: "scheduler".to_string(),
                id_field: "job_id".to_string(),
                url_path: "/api/catalogs/jobs".to_string(),
                entries: vec![json!({
                    "job_id": "job-01",
                    "title": "Mesh Health Audit"
                })],
            },
        ];

        let boot_stages = default_boot_stages(listen, &blob_cache_dir);
        let mesh_config = PersistentMeshConfig {
            node_name: Some(node.to_string()),
            listen_addr: Some(listen.to_string()),
            channel: DEFAULT_CHANNEL_NAME.to_string(),
            configured: true,
            secret_key: None,
            derived_keys: None,
            channel_fingerprint: secret_channel_fingerprint("dialtone:seeded"),
            bootstrap_tickets: Vec::new(),
            seed_peers: Vec::new(),
            allowed_endpoint_ids: Vec::new(),
            used_invite_nonces: Vec::new(),
            daemon_installed: false,
            daemon_service_path: None,
            managed_processes: Vec::new(),
            updated_at_unix: now,
        };
        let api_token =
            load_or_create_api_token().unwrap_or_else(|_| "dtk-seeded-token".to_string());

        Self {
            node: node.to_string(),
            mode: mode.to_string(),
            listen: listen.to_string(),
            api_token,
            blob_cache_dir,
            allowed_deploy_roots: vec!["/opt/dialtone/".to_string(), "/tmp/".to_string()],
            boot_stages,
            mesh_config,
            control_tx: None,
            supervisor: Arc::new(Mutex::new(ManagedProcessSupervisor::new())),
            metadata,
            iroh_runtime,
            nodes: peer_heartbeats,
            installed_binaries: vec![InstalledBinary {
                name: "robot-v4-cartesiand".to_string(),
                path: "/opt/dialtone/bin/robot-v4-cartesiand".to_string(),
                sha256: sample_sha.clone(),
            }],
            managed_processes: vec![ManagedProcessSpec {
                name: "robot-edge-motion-server".to_string(),
                path: "/opt/dialtone/bin/robot-v4-cartesiand".to_string(),
                sha256: sample_sha.clone(),
                desired_state: "running".to_string(),
                args: vec![
                    "--nats".to_string(),
                    format!("dialtone://{node}/controller/robot-v4-nats"),
                ],
                env: Vec::new(),
                cwd: None,
                startup_grace_secs: Some(5),
                status: Some("running".to_string()),
                pid: Some(4242),
                service_unit: Some("robot-edge-motion-server.service".to_string()),
            }],
            deploy_slots: vec![DeploySlot {
                name: "edge-1-motion-server".to_string(),
                active: true,
                path: "/opt/dialtone/slots/a/robot-v4-cartesiand".to_string(),
                sha256: sample_sha,
                previous_path: Some("/opt/dialtone/slots/b/robot-v4-cartesiand".to_string()),
                previous_sha256: Some(prev_sha),
                status: "ready".to_string(),
            }],
            deploy_manifests: Vec::new(),
            deploy_build_caches: Vec::new(),
            blob_fetches: Vec::new(),
            service_endpoints,
            key_leases: vec![KeyLeaseStatus {
                name: "pendant-devkit".to_string(),
                holder: node.to_string(),
                kind: "ssh-private-key".to_string(),
                available: true,
                status: "healthy".to_string(),
                source: "metadata".to_string(),
                expires_at_unix: None,
                warning: None,
            }],
            host_resources: HostResourceTelemetry::default(),
            events: vec![EventRecord {
                id: "evt-1".to_string(),
                kind: "mesh.boot".to_string(),
                severity: "info".to_string(),
                source: node.to_string(),
                message: "dialtone iroh 1.3 mesh runtime initialized".to_string(),
                timestamp_unix: now,
                test_session_id: "session-default".to_string(),
                daemon_owned: true,
            }],
            logs: vec![ServiceLogRecord {
                service: "robot-edge-motion-server".to_string(),
                level: "INFO".to_string(),
                stream: "stdout".to_string(),
                cursor: 1,
                message: "connected to dialtone peer-qualified NATS endpoint".to_string(),
                timestamp_unix: now,
            }],
            alerts: vec![AlertRecord {
                id: "alert-edge-1-nominal".to_string(),
                target_node: "edge-1".to_string(),
                status: "active".to_string(),
                severity: "info".to_string(),
                source: "telemetry".to_string(),
                summary: "Edge-1 telemetry nominal".to_string(),
                telemetry_ref: "dialtone://edge-1/edge/telemetry".to_string(),
                last_seen_unix: now,
                acknowledged_by: None,
            }],
            robot_command_intents: Vec::new(),
            cursor_jobs: vec![CursorJobRecord {
                id: "cursor-job-1".to_string(),
                host: "node-beta".to_string(),
                remote_user: Some("operator".to_string()),
                prompt: "audit mesh service endpoints".to_string(),
                status: "completed".to_string(),
                logs: vec![
                    "audit started".to_string(),
                    "0 forbidden dependencies".to_string(),
                ],
                updated_at_unix: now,
            }],
            kv: vec![
                KeyValueRecord {
                    key: "mesh/leader".to_string(),
                    value: json!(node),
                    updated_at_unix: now,
                },
                KeyValueRecord {
                    key: "mesh/protocol".to_string(),
                    value: json!("iroh-1.3.0"),
                    updated_at_unix: now,
                },
            ],
            kv_page: None,
        }
    }
}

fn masked_mesh_config(cfg: &PersistentMeshConfig) -> PersistentMeshConfig {
    let mut c = cfg.clone();
    c.secret_key = None;
    c.derived_keys = None;
    c
}

pub fn serve_http_blocking(
    listener: TcpListener,
    state: Arc<Mutex<DaemonRuntimeState>>,
    max_requests: Option<usize>,
) -> Result<(), String> {
    let active_workers = Arc::new(AtomicUsize::new(0));
    let mut handled = 0usize;
    let mut join_handles = Vec::new();

    for stream in listener.incoming() {
        let mut stream = stream.map_err(|e| format!("accept connection: {e}"))?;
        if active_workers.load(Ordering::SeqCst) >= MAX_CONCURRENT_HTTP_CONNECTIONS {
            let _ = write_json(
                &mut stream,
                503,
                &json!({ "error": "too many concurrent HTTP connections" }),
            );
            continue;
        }
        active_workers.fetch_add(1, Ordering::SeqCst);
        let state_clone = state.clone();
        let active_clone = active_workers.clone();
        let worker = thread::spawn(move || {
            let _ = handle_connection(stream, &state_clone);
            active_clone.fetch_sub(1, Ordering::SeqCst);
        });
        if max_requests.is_some() {
            join_handles.push(worker);
        }
        handled += 1;
        if let Some(limit) = max_requests {
            if handled >= limit {
                break;
            }
        }
    }

    for h in join_handles {
        let _ = h.join();
    }
    Ok(())
}

fn read_http_request(stream: &mut TcpStream) -> Result<Option<(String, String)>, (u16, String)> {
    let mut buf = Vec::with_capacity(8192);
    let mut chunk = [0u8; 8192];
    let header_end = loop {
        if let Some(pos) = buf.windows(4).position(|w| w == b"\r\n\r\n") {
            break pos;
        }
        if buf.len() >= 65536 {
            return Err((400, "HTTP header exceeds 64 KiB limit".to_string()));
        }
        let n = stream.read(&mut chunk).map_err(|e| (400, e.to_string()))?;
        if n == 0 {
            if buf.is_empty() {
                return Ok(None);
            }
            break buf.len();
        }
        buf.extend_from_slice(&chunk[..n]);
    };

    let head = String::from_utf8_lossy(&buf[..header_end]).to_string();
    let body_start = (header_end + 4).min(buf.len());
    let mut body_bytes = buf[body_start..].to_vec();

    let mut content_length = 0usize;
    for line in head.lines().skip(1) {
        if let Some((k, v)) = line.split_once(':') {
            if k.trim().eq_ignore_ascii_case("content-length") {
                content_length = v.trim().parse::<usize>().unwrap_or(0);
            }
        }
    }

    if content_length > MAX_HTTP_BODY_BYTES {
        return Err((
            413,
            format!("HTTP request body exceeds maximum size of {MAX_HTTP_BODY_BYTES} bytes"),
        ));
    }

    while body_bytes.len() < content_length {
        let remaining = content_length - body_bytes.len();
        let to_read = remaining.min(chunk.len());
        let n = stream
            .read(&mut chunk[..to_read])
            .map_err(|e| (400, e.to_string()))?;
        if n == 0 {
            break;
        }
        body_bytes.extend_from_slice(&chunk[..n]);
    }

    let body = String::from_utf8_lossy(&body_bytes).to_string();
    Ok(Some((head, body)))
}

fn handle_connection(
    mut stream: TcpStream,
    state: &Arc<Mutex<DaemonRuntimeState>>,
) -> Result<(), String> {
    let _ = stream.set_read_timeout(Some(HTTP_IO_TIMEOUT));
    let _ = stream.set_write_timeout(Some(HTTP_IO_TIMEOUT));

    let (head, body_owned) = match read_http_request(&mut stream) {
        Ok(Some(v)) => v,
        Ok(None) => return Ok(()),
        Err((status, msg)) => return write_json(&mut stream, status, &json!({ "error": msg })),
    };
    let body_part = body_owned.as_str();
    let mut lines = head.lines();
    let request_line = lines.next().unwrap_or_default();
    let mut parts = request_line.split_whitespace();
    let method = parts.next().unwrap_or("GET");
    let full_target = parts.next().unwrap_or("/");
    let (path, query_str) = full_target.split_once('?').unwrap_or((full_target, ""));
    let query = parse_query(query_str);

    let mut headers = HashMap::new();
    for line in lines {
        if let Some((k, v)) = line.split_once(':') {
            headers.insert(k.trim().to_ascii_lowercase(), v.trim().to_string());
        }
    }

    if let Err(err) = validate_local_http_request(method, &headers) {
        return write_json(&mut stream, 403, &json!({ "error": err }));
    }

    if path == "/ws" {
        if let Some(ws_key) = headers.get("sec-websocket-key") {
            return handle_websocket_upgrade(stream, ws_key, state);
        }
    }

    let mut st = state.lock().map_err(|_| "state poisoned".to_string())?;
    if method.eq_ignore_ascii_case("POST") {
        if let Err(err) = validate_bearer_token(&headers, &st.api_token) {
            return write_json(&mut stream, 401, &json!({ "error": err }));
        }
    }
    let now = now_unix();
    let self_name = st.node.clone();
    if let Some(self_hb) = st.nodes.iter_mut().find(|n| n.node == self_name) {
        self_hb.updated_at_unix = now;
    }
    prune_stale_peers(&mut st.nodes, now, DEFAULT_PEER_TTL_SECS);

    match (method, path) {
        ("GET", "/") | ("GET", "/nodes") | ("GET", "/peer") | ("GET", "/hud") => {
            let html = render_dashboard_html(&st, query.get("peer-id").map(String::as_str));
            write_response(
                &mut stream,
                200,
                "text/html; charset=utf-8",
                html.as_bytes(),
                false,
            )
        }
        ("GET", "/health") => {
            let body = json!({
                "status": "ok",
                "node": st.node,
                "mode": st.mode,
                "listen": st.listen,
                "local_url": format!("http://{}", st.listen),
                "peers": st.nodes.len(),
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config)
            });
            write_json(&mut stream, 200, &body)
        }
        ("GET", "/api/boot-status") | ("GET", "/api/config") => {
            let body = json!({
                "schema": "dialtone.boot_status.v1",
                "node": st.node,
                "mode": st.mode,
                "listen": st.listen,
                "local_url": format!("http://{}", st.listen),
                "blob_cache_dir": st.blob_cache_dir,
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config),
                "iroh_runtime": st.iroh_runtime
            });
            write_json(&mut stream, 200, &body)
        }
        ("POST", "/api/mesh/join") | ("POST", "/api/config/join") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let channel = normalize_channel_name(parsed.get("channel").and_then(Value::as_str));
            let raw_password = parsed
                .get("password")
                .or_else(|| parsed.get("channel_password"))
                .or_else(|| parsed.get("secret_key"))
                .or_else(|| parsed.get("secret"))
                .or_else(|| parsed.get("key"))
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim()
                .to_string();
            let raw_ticket = parsed
                .get("bootstrap_ticket")
                .or_else(|| parsed.get("ticket"))
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim()
                .to_string();
            let new_node_name = parsed
                .get("node")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(String::from);

            let validated_password = match validate_channel_password(&raw_password) {
                Ok(p) => p,
                Err(err) => {
                    return write_json(&mut stream, 400, &json!({ "error": err }));
                }
            };

            if let Some(ref nn) = new_node_name {
                st.node = nn.clone();
            }

            let keys = st
                .mesh_config
                .apply_channel_password(&channel, &validated_password);
            let fp = keys.fingerprint();
            let topic_id = keys.topic_id;

            if raw_ticket.starts_with(crate::gossip::INVITE_TICKET_PREFIX) {
                let parsed_invite = match raw_ticket.parse::<crate::gossip::InviteTicket>() {
                    Ok(inv) => inv,
                    Err(err) => return write_json(&mut stream, 400, &json!({ "error": err })),
                };
                let self_ep = st.iroh_runtime.endpoint_id.clone();
                if let Err(err) = crate::gossip::redeem_invite_ticket(
                    &parsed_invite,
                    &keys,
                    &self_ep,
                    now,
                    &mut st.mesh_config,
                ) {
                    return write_json(&mut stream, 400, &json!({ "error": err }));
                }
            } else if !raw_ticket.is_empty()
                && !st.mesh_config.bootstrap_tickets.contains(&raw_ticket)
            {
                st.mesh_config.bootstrap_tickets.push(raw_ticket.clone());
            }

            let tickets = st.mesh_config.bootstrap_tickets.clone();
            st.mesh_config.node_name = Some(st.node.clone());
            st.mesh_config.listen_addr = Some(st.listen.clone());
            st.mesh_config.updated_at_unix = now;
            let _ = save_mesh_config(&default_config_path(), &st.mesh_config);

            if let Some(stage5) = st.boot_stages.iter_mut().find(|s| s.step == 5) {
                stage5.status = "ready".to_string();
                stage5.detail = format!("Joined password-protected channel '{}' ({fp})", channel);
                stage5.updated_at_unix = now;
            }

            let node_src = st.node.clone();
            st.events.push(EventRecord {
                id: format!("evt-join-{now}"),
                kind: "mesh.join".to_string(),
                severity: "info".to_string(),
                source: node_src,
                message: format!(
                    "Joined password-protected gossip channel '{}' ({fp})",
                    channel
                ),
                timestamp_unix: now,
                test_session_id: "session-live".to_string(),
                daemon_owned: true,
            });

            if let Some(tx) = &st.control_tx {
                let _ = tx.send(MeshControlCommand::JoinMesh {
                    channel: channel.clone(),
                    keys,
                    bootstrap_tickets: tickets,
                    node_name: new_node_name,
                });
            }

            println!(
                "[MESH-CONFIG] Node '{}' joined channel '{}' ({}) (topic={})",
                st.node, channel, fp, topic_id
            );

            let resp = json!({
                "status": "joined",
                "node": st.node,
                "channel": channel,
                "channel_fingerprint": fp,
                "topic_id": topic_id.to_string(),
                "configured": true,
                "local_url": format!("http://{}", st.listen),
                "boot_stages": st.boot_stages
            });
            write_json(&mut stream, 200, &resp)
        }
        ("POST", "/api/daemon/install") => {
            match install_persistent_os_daemon(
                &st.node,
                &st.listen,
                st.mesh_config.secret_key.as_deref(),
            ) {
                Ok(report) => {
                    st.mesh_config.daemon_installed = true;
                    st.mesh_config.daemon_service_path = Some(report.service_unit_path.clone());
                    st.mesh_config.updated_at_unix = now;
                    let _ = save_mesh_config(&default_config_path(), &st.mesh_config);
                    println!(
                        "[DAEMON-INSTALL] Installed persistent OS daemon for '{}': {}",
                        st.node, report.instructions
                    );
                    write_json(&mut stream, 200, &report)
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
            }
        }
        ("GET", "/install/v1/install.sh") => {
            let script = render_install_v1_sh(&format!("http://{}", st.listen));
            write_response(
                &mut stream,
                200,
                "text/x-shellscript",
                script.as_bytes(),
                false,
            )
        }
        ("GET", "/install/v1/manifest.json") => {
            let manifest = json!({
                "schema": "dialtone.install.v1",
                "server": format!("http://{}", st.listen),
                "node": st.node
            });
            write_json(&mut stream, 200, &manifest)
        }
        ("GET", "/api/state") => {
            st.reconcile_live_processes();
            let include_kv = query
                .get("include_kv")
                .map(|v| v != "false" && v != "0")
                .unwrap_or(true);
            let offset = query.get("kv_offset").and_then(|v| v.parse::<usize>().ok());
            let limit = query.get("kv_limit").and_then(|v| v.parse::<usize>().ok());
            match paginate_kv_records(&st.kv, include_kv, offset, limit) {
                Ok((kv_slice, page_meta)) => {
                    let mut projected = st.clone();
                    projected.mesh_config = masked_mesh_config(&st.mesh_config);
                    projected.kv = kv_slice;
                    projected.kv_page = page_meta;
                    write_json(&mut stream, 200, &projected)
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
            }
        }
        ("GET", "/api/mesh") | ("GET", "/api/gossip") => {
            st.reconcile_live_processes();
            let body = json!({
                "schema": "dialtone.gossip.v1",
                "mesh": "dialtone-iroh-gossip",
                "self_node": st.node,
                "self_iroh_runtime": st.iroh_runtime,
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config),
                "nodes": st.nodes,
                "managed_processes": st.managed_processes,
                "service_endpoints": st.service_endpoints
            });
            write_json(&mut stream, 200, &body)
        }
        ("GET", "/api/iroh-runtime") => {
            let body = json!({
                "schema": "dialtone.iroh_runtime.v1",
                "owner": "dialtone-rust-daemon",
                "steady_state_data_plane": "iroh-quic",
                "http_plane_role": "local-bootstrap-control-only",
                "node": st.node,
                "diagnostics": st.iroh_runtime
            });
            write_json(&mut stream, 200, &body)
        }
        ("HEAD", "/api/artifacts/blob") => {
            let sha = query.get("sha256").cloned().unwrap_or_default();
            if has_cached_blob(&st.blob_cache_dir, &sha) {
                write_response(&mut stream, 200, "application/octet-stream", b"", true)
            } else {
                write_response(&mut stream, 404, "application/octet-stream", b"", true)
            }
        }
        ("GET", "/api/artifacts/blob") => {
            let sha = query.get("sha256").cloned().unwrap_or_default();
            match read_cached_blob(&st.blob_cache_dir, &sha) {
                Ok(bytes) => {
                    write_response(&mut stream, 200, "application/octet-stream", &bytes, false)
                }
                Err(err) => write_json(&mut stream, 404, &json!({ "error": err })),
            }
        }
        ("POST", "/api/artifacts/blob") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let b64 = parsed
                .get("bytes_base64")
                .and_then(Value::as_str)
                .unwrap_or_default();
            let expected_sha = parsed.get("sha256").and_then(Value::as_str);
            match crate::base64_decode(b64) {
                Ok(raw_bytes) => {
                    match store_verified_blob(&st.blob_cache_dir, &raw_bytes, expected_sha) {
                        Ok((sha, path)) => write_json(
                            &mut stream,
                            200,
                            &json!({ "sha256": sha, "path": path, "bytes": raw_bytes.len() }),
                        ),
                        Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                    }
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err.to_string() })),
            }
        }
        ("POST", "/api/artifacts/fetch") | ("GET", "/api/blob-fetch-report") => {
            let req = if method == "POST" {
                serde_json::from_str::<TieredBlobFetchRequest>(body_part).map_err(|e| e.to_string())
            } else {
                Ok(TieredBlobFetchRequest {
                    sha256: query.get("sha256").cloned().unwrap_or_default(),
                    uri: query.get("uri").cloned().unwrap_or_default(),
                    provider_node: query
                        .get("provider")
                        .cloned()
                        .unwrap_or_else(|| "node-beta".to_string()),
                    expected_bytes: 1024,
                    http_fallback_enabled: query
                        .get("http_fallback")
                        .map(|v| v == "1" || v == "true")
                        .unwrap_or(false),
                })
            };
            match req.and_then(|r| evaluate_tiered_blob_fetch(&st.blob_cache_dir, &r)) {
                Ok(plan) => write_json(&mut stream, 200, &plan),
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
            }
        }
        ("GET", "/api/deploy-plan") | ("POST", "/api/deploy-plan") => {
            let target = query
                .get("target")
                .cloned()
                .unwrap_or_else(|| "robot-v4-default".to_string());
            match generate_deploy_plan(&target) {
                Ok(plan) => write_json(&mut stream, 200, &plan),
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
            }
        }
        ("POST", "/api/deploy/gossip-manifest") | ("POST", "/api/deploy/apply-manifest") => {
            match serde_json::from_str::<DeployManifest>(body_part) {
                Ok(manifest) => {
                    match validate_manifest_contract(
                        &manifest,
                        &st.metadata.deploy_policy,
                        &st.allowed_deploy_roots,
                    ) {
                        Ok(()) => {
                            st.deploy_manifests.push(manifest.clone());
                            write_json(
                                &mut stream,
                                200,
                                &json!({ "status": "accepted", "manifest_id": manifest.id }),
                            )
                        }
                        Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                    }
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err.to_string() })),
            }
        }
        ("POST", "/api/deploy/build-request") => {
            match serde_json::from_str::<BuildProbeRequest>(body_part) {
                Ok(req) => {
                    let resp = probe_build_cache(&st.deploy_build_caches, &req);
                    write_json(&mut stream, 200, &resp)
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err.to_string() })),
            }
        }
        ("GET", "/api/build-decision") => {
            let case = query
                .get("case")
                .map(String::as_str)
                .unwrap_or("cache-reused");
            match evaluate_deploy_build_decision(case) {
                Ok(decision) => write_json(&mut stream, 200, &decision),
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
            }
        }
        ("GET", "/api/rollback-plan") | ("POST", "/api/rollback-plan") => {
            let target_node = query
                .get("node")
                .cloned()
                .unwrap_or_else(|| "edge-1".to_string());
            if let Some(slot) = st.deploy_slots.first() {
                match plan_rollback(&target_node, slot) {
                    Ok(plan) => write_json(&mut stream, 200, &plan),
                    Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                }
            } else {
                write_json(&mut stream, 404, &json!({ "error": "no deploy slots" }))
            }
        }
        ("GET", "/api/processes") => {
            st.reconcile_live_processes();
            write_json(
                &mut stream,
                200,
                &json!({
                    "schema": "dialtone.managed_processes.v1",
                    "node": st.node,
                    "managed_processes": st.managed_processes
                }),
            )
        }
        ("POST", "/api/processes/blob") | ("POST", "/api/processes") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let name = parsed
                .get("name")
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim()
                .to_string();
            if name.is_empty() {
                return write_json(&mut stream, 400, &json!({ "error": "name is required" }));
            }
            let desired_state = parsed
                .get("desired_state")
                .and_then(Value::as_str)
                .unwrap_or("running")
                .trim()
                .to_string();
            let args: Vec<String> = parsed
                .get("args")
                .and_then(Value::as_array)
                .map(|arr| {
                    arr.iter()
                        .filter_map(Value::as_str)
                        .map(String::from)
                        .collect()
                })
                .unwrap_or_default();
            let env_vars: Vec<String> = parsed
                .get("env")
                .and_then(Value::as_array)
                .map(|arr| {
                    arr.iter()
                        .filter_map(Value::as_str)
                        .map(String::from)
                        .collect()
                })
                .unwrap_or_default();
            let cwd = parsed
                .get("cwd")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(String::from);

            let mut sha256 = parsed
                .get("sha256")
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim()
                .to_string();

            if let Some(b64) = parsed
                .get("bytes_base64")
                .and_then(Value::as_str)
                .filter(|s| !s.trim().is_empty())
            {
                let raw = match crate::base64_decode(b64) {
                    Ok(b) => b,
                    Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                };
                let exp = if sha256.is_empty() {
                    None
                } else {
                    Some(sha256.as_str())
                };
                match store_verified_blob(&st.blob_cache_dir, &raw, exp) {
                    Ok((verified_sha, _)) => sha256 = verified_sha,
                    Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                }
            }

            if sha256.is_empty() {
                if let Some(existing) = st.managed_processes.iter().find(|p| p.name == name) {
                    sha256 = existing.sha256.clone();
                }
            }
            if sha256.is_empty() {
                return write_json(
                    &mut stream,
                    400,
                    &json!({ "error": "sha256 or bytes_base64 is required to configure a blob subprocess" }),
                );
            }

            let (verified_sha, staged_path) =
                match stage_executable_blob(&st.blob_cache_dir, &name, &sha256) {
                    Ok(v) => v,
                    Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                };

            let mut spec = ManagedProcessSpec {
                name: name.clone(),
                path: staged_path.to_string_lossy().to_string(),
                sha256: verified_sha.clone(),
                desired_state: desired_state.clone(),
                args,
                env: env_vars,
                cwd,
                startup_grace_secs: Some(2),
                status: None,
                pid: None,
                service_unit: None,
            };

            let sup_res = {
                let sup_arc = st.supervisor.clone();
                let mut sup = sup_arc.lock().unwrap();
                sup.reconcile(std::slice::from_mut(&mut spec))
            };
            if let Err(err) = sup_res {
                return write_json(&mut stream, 400, &json!({ "error": err }));
            }

            if let Some(existing) = st.managed_processes.iter_mut().find(|p| p.name == name) {
                *existing = spec.clone();
            } else {
                st.managed_processes.push(spec.clone());
            }

            if let Some(existing_cfg) = st
                .mesh_config
                .managed_processes
                .iter_mut()
                .find(|p| p.name == name)
            {
                *existing_cfg = spec.clone();
            } else {
                st.mesh_config.managed_processes.push(spec.clone());
            }
            st.mesh_config.updated_at_unix = now_unix();
            let _ = save_mesh_config(&default_config_path(), &st.mesh_config);

            let self_node = st.node.clone();
            let procs_snapshot = st.managed_processes.clone();
            if let Some(self_hb) = st.nodes.iter_mut().find(|n| n.node == self_node) {
                self_hb.managed_processes = procs_snapshot;
            }

            println!(
                "[SUPERVISOR] Configured blob subprocess '{}' (sha256={}, desired_state={}, status={:?}, pid={:?})",
                spec.name, spec.sha256, spec.desired_state, spec.status, spec.pid
            );

            write_json(
                &mut stream,
                200,
                &json!({
                    "status": "configured",
                    "node": st.node,
                    "process": spec
                }),
            )
        }
        ("GET", "/api/process-intent") | ("POST", "/api/process-intent") => {
            let target_node = query
                .get("node")
                .cloned()
                .unwrap_or_else(|| "edge-1".to_string());
            let action = query
                .get("action")
                .cloned()
                .unwrap_or_else(|| "restart".to_string());
            if let Some(proc) = st.managed_processes.first() {
                match plan_process_intent(&st.node, &target_node, proc, &action) {
                    Ok(plan) => write_json(&mut stream, 200, &plan),
                    Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                }
            } else {
                write_json(&mut stream, 404, &json!({ "error": "no managed process" }))
            }
        }
        ("GET", "/api/alert-ack") | ("POST", "/api/alert-ack") => {
            let actor = query
                .get("actor")
                .cloned()
                .unwrap_or_else(|| "operator-a".to_string());
            let note = query
                .get("note")
                .cloned()
                .unwrap_or_else(|| "verified nominal".to_string());
            if let Some(alert) = st.alerts.first() {
                match plan_alert_ack(alert, &actor, &note) {
                    Ok(plan) => write_json(&mut stream, 200, &plan),
                    Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                }
            } else {
                write_json(&mut stream, 404, &json!({ "error": "no alerts" }))
            }
        }
        ("GET", "/api/program-catalog") => {
            let cat = st.metadata.catalogs.first().cloned().unwrap_or_default();
            write_json(&mut stream, 200, &cat)
        }
        ("GET", "/api/catalogs") => write_json(&mut stream, 200, &st.metadata.catalogs),
        ("POST", "/api/commands/intent") => {
            match serde_json::from_str::<RobotCommandIntentRecord>(body_part) {
                Ok(intent) => {
                    let merged = upsert_command_intent(&mut st.robot_command_intents, intent);
                    write_json(&mut stream, 200, &json!({ "merged": merged }))
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err.to_string() })),
            }
        }
        ("GET", "/api/cursor/jobs") => write_json(&mut stream, 200, &st.cursor_jobs),
        _ if path.starts_with("/api/catalogs/") => {
            let name = path.trim_start_matches("/api/catalogs/");
            if let Some(cat) = st.metadata.catalogs.iter().find(|c| c.name == name) {
                write_json(&mut stream, 200, cat)
            } else {
                write_json(&mut stream, 404, &json!({ "error": "catalog not found" }))
            }
        }
        _ => write_json(
            &mut stream,
            404,
            &json!({ "error": "not found", "path": path }),
        ),
    }
}

fn handle_websocket_upgrade(
    mut stream: TcpStream,
    ws_key: &str,
    state: &Arc<Mutex<DaemonRuntimeState>>,
) -> Result<(), String> {
    let mut input = ws_key.as_bytes().to_vec();
    input.extend_from_slice(b"258EAFA5-E914-47DA-95CA-C5AB0DC85B11");
    let accept = crate::base64_encode(&crate::sha1_digest(&input));

    let handshake = format!(
        "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: {accept}\r\n\r\n"
    );
    stream
        .write_all(handshake.as_bytes())
        .map_err(|e| e.to_string())?;

    let payload = {
        let st = state.lock().map_err(|_| "state poisoned".to_string())?;
        serde_json::to_vec(&json!({
            "event": "state_snapshot",
            "node": st.node,
            "peers": st.nodes.len()
        }))
        .map_err(|e| e.to_string())?
    };

    let mut frame = vec![0x81];
    if payload.len() < 126 {
        frame.push(payload.len() as u8);
    } else {
        frame.push(126);
        frame.extend_from_slice(&(payload.len() as u16).to_be_bytes());
    }
    frame.extend_from_slice(&payload);
    stream.write_all(&frame).map_err(|e| e.to_string())
}

fn render_dashboard_html(state: &DaemonRuntimeState, selected_peer: Option<&str>) -> String {
    let peer_focus = selected_peer.unwrap_or(&state.node);
    let initial_gossip = json!({
        "schema": "dialtone.gossip.v1",
        "mesh": "dialtone-iroh-gossip",
        "self_node": state.node,
        "mode": state.mode,
        "selected_peer": peer_focus,
        "self_iroh_runtime": state.iroh_runtime,
        "boot_stages": state.boot_stages,
        "mesh_config": masked_mesh_config(&state.mesh_config),
        "nodes": state.nodes,
        "service_endpoints": state.service_endpoints
    });
    let initial_json = serde_json::to_string(&initial_gossip).unwrap_or_else(|_| "{}".to_string());

    let template = r##"<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>dialtone // __SELF_NODE__ — Dialtone Iroh 1.3 Mesh Control Plane</title>
<style>
  :root {
    --bg: #060607;
    --fg-bright: #f4f4f6;
    --fg: #c4c4cc;
    --fg-muted: #71717a;
    --fg-dim: #3f3f46;
    --red: #ff2e3b;
    --red-soft: rgba(255, 46, 59, 0.16);
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  html, body {
    width: 100vw; height: 100vh; overflow: hidden;
    background: var(--bg);
    color: var(--fg);
    font-family: "JetBrains Mono", "SF Mono", "Menlo", "Consolas", monospace;
    font-size: 12px;
    line-height: 1.5;
    font-variant-numeric: tabular-nums;
    -webkit-font-smoothing: antialiased;
    user-select: none;
  }
  #hud-3d-canvas {
    position: fixed; inset: 0; width: 100vw; height: 100vh;
    z-index: 1; cursor: grab;
  }
  #hud-3d-canvas:active { cursor: grabbing; }

  /* Subtle monochrome CRT scanline + vignette */
  .crt-layer {
    position: fixed; inset: 0; z-index: 2; pointer-events: none;
    background:
      linear-gradient(rgba(255, 255, 255, 0) 50%, rgba(0, 0, 0, 0.28) 50%),
      radial-gradient(circle at 50% 55%, transparent 55%, rgba(0, 0, 0, 0.78) 100%);
    background-size: 100% 3px, 100% 100%;
  }

  /* Borderless Hacker Terminal 'top' Overlay */
  .term-overlay {
    position: fixed; inset: 0; z-index: 3; pointer-events: none;
    display: flex; flex-direction: column; justify-content: space-between;
    padding: 24px 32px;
  }

  .term-top {
    pointer-events: auto;
    max-width: 1180px;
    background: linear-gradient(180deg, rgba(6, 6, 7, 0.88) 0%, rgba(6, 6, 7, 0.62) 78%, rgba(6, 6, 7, 0) 100%);
    padding-bottom: 18px;
  }

  .term-header-line {
    display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between;
    gap: 16px;
    margin-bottom: 4px;
  }
  .term-title {
    color: var(--fg-bright);
    font-weight: 700;
    letter-spacing: 0.08em;
    text-transform: lowercase;
  }
  .term-title span.sub {
    color: var(--fg-muted);
    font-weight: 400;
    letter-spacing: 0.02em;
    margin-left: 10px;
  }
  .term-kv-group {
    display: flex; flex-wrap: wrap; gap: 20px;
    color: var(--fg-muted);
    font-size: 11px;
  }
  .term-kv strong {
    color: var(--fg-bright);
    font-weight: 600;
  }
  .term-kv.alert strong, .alert-text {
    color: var(--red) !important;
    font-weight: 700;
  }

  /* Minimalist typographic command links (no button boxes) */
  .term-actions {
    display: flex; flex-wrap: wrap; gap: 16px;
    margin-top: 8px;
    margin-bottom: 14px;
    font-size: 11px;
  }
  .term-cmd {
    background: none; border: none; outline: none;
    color: var(--fg-muted);
    font-family: inherit; font-size: 11px;
    cursor: pointer;
    padding: 0;
    letter-spacing: 0.04em;
    transition: color 0.12s ease;
  }
  .term-cmd:hover, .term-cmd.active {
    color: var(--fg-bright);
  }
  .term-cmd.alert {
    color: var(--red);
    font-weight: 700;
  }

  /* Boot pipeline status strip */
  .boot-strip {
    display: flex; flex-wrap: wrap; gap: 18px;
    font-size: 10px;
    color: var(--fg-muted);
    margin-bottom: 14px;
    letter-spacing: 0.04em;
  }
  .boot-item.warn {
    color: var(--red);
    font-weight: 700;
  }

  /* Inline Terminal Prompt Drawer for Secret Key / Daemon / Blob */
  .term-drawer {
    display: none;
    margin: 10px 0 18px 0;
    padding: 12px 0;
    user-select: text;
  }
  .term-drawer.open { display: block; }
  .prompt-row {
    display: flex; flex-wrap: wrap; align-items: center; gap: 12px;
    margin-bottom: 8px;
  }
  .prompt-label {
    color: var(--fg-bright);
    font-weight: 700;
  }
  .term-input {
    background: rgba(255, 255, 255, 0.04);
    border: none;
    border-bottom: 1px solid var(--fg-dim);
    color: var(--fg-bright);
    font-family: inherit;
    font-size: 12px;
    padding: 4px 8px;
    outline: none;
    min-width: 280px;
    user-select: text;
  }
  .term-input:focus {
    border-bottom-color: var(--fg-bright);
    background: rgba(255, 255, 255, 0.07);
  }
  .term-input.short { min-width: 130px; width: 150px; }
  .term-feedback {
    font-size: 11px;
    color: var(--fg-muted);
    margin-top: 6px;
  }

  /* 'top' / 'htop' Borderless Process & Node Table */
  .top-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 11px;
    text-align: left;
  }
  .top-table th {
    color: var(--fg-muted);
    font-weight: 600;
    letter-spacing: 0.08em;
    padding: 3px 16px 6px 0;
    text-transform: uppercase;
    font-size: 10px;
  }
  .top-table td {
    padding: 4px 16px 4px 0;
    color: var(--fg);
    white-space: nowrap;
    cursor: pointer;
    transition: color 0.1s ease, background 0.1s ease;
  }
  .top-table tr.node-row:hover td {
    color: var(--fg-bright);
    background: rgba(255, 255, 255, 0.03);
  }
  .top-table tr.node-row.selected td {
    color: var(--fg-bright);
    font-weight: 600;
  }
  .top-table tr.node-row.has-warning td.node-col {
    color: var(--red);
    font-weight: 700;
  }
  .ascii-bar {
    color: var(--fg-dim);
    margin-right: 6px;
    letter-spacing: -0.05em;
  }
  .ascii-bar .fill { color: var(--fg); }
  .ascii-bar .fill.red { color: var(--red); }

  /* Minimalist Bottom Target Telemetry & Log Stream */
  .term-bottom {
    pointer-events: auto;
    display: flex; flex-direction: column; gap: 6px;
    max-width: 1180px;
    background: linear-gradient(0deg, rgba(6, 6, 7, 0.85) 0%, rgba(6, 6, 7, 0) 100%);
    padding-top: 16px;
    user-select: text;
  }
  .focus-line {
    display: flex; flex-wrap: wrap; gap: 18px; align-items: baseline;
    font-size: 11px;
    color: var(--fg-muted);
  }
  .focus-line strong { color: var(--fg-bright); }
  .caps-line {
    font-size: 10px;
    color: var(--fg-muted);
    letter-spacing: 0.03em;
    word-spacing: 8px;
  }
  .log-line {
    font-size: 11px;
    color: var(--fg);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
</style>
</head>
<body>
<canvas id="hud-3d-canvas"></canvas>
<div class="crt-layer"></div>

<div class="term-overlay">
  <div class="term-top">
    <!-- Line 1: Classic unix 'top' header -->
    <div class="term-header-line">
      <div class="term-title">
        dialtone // <span id="m-self-node">__SELF_NODE__</span>
        <span class="sub">Dialtone Iroh 1.3 Mesh Control Plane</span>
      </div>
      <div class="term-kv-group">
        <span class="term-kv">nodes <strong id="m-peer-count">1</strong></span>
        <span class="term-kv">channel <strong id="m-channel">ch-6469616c746f</strong></span>
        <span class="term-kv">bind <strong id="m-bound-ip">--</strong></span>
        <span class="term-kv">cluster <strong id="m-cluster-res">--</strong></span>
        <span class="term-kv" id="m-warn-kv">warnings <strong id="m-warn-count">0</strong></span>
        <span class="term-kv">sync <strong id="sync-lamp">live</strong></span>
      </div>
    </div>

    <!-- Line 2: Dependency boot stages (compact one-line) -->
    <div id="boot-stage-list" class="boot-strip"></div>

    <!-- Line 3: Minimalist typographic command bar -->
    <div class="term-actions">
      <button class="term-cmd" id="btn-setup" onclick="toggleSetupModal()">[unlock / join channel]</button>
      <button class="term-cmd" id="btn-daemon" onclick="installSystemDaemon()">[install daemon: <span id="daemon-status-badge">checking</span>]</button>
      <button class="term-cmd" onclick="stageSampleBlob()">[verify blob cas]</button>
      <button class="term-cmd active" id="btn-orbit" onclick="toggleOrbit()">[orbit: on]</button>
      <button class="term-cmd" onclick="resetCamera()">[reset cam]</button>
      <button class="term-cmd" onclick="cycleTarget()">[next node]</button>
      <button class="term-cmd" onclick="triggerPulseWave()">[ping swarm]</button>
    </div>

    <!-- Collapsible inline terminal prompt for joining password-protected channel -->
    <div id="setup-modal-backdrop" class="term-drawer">
      <div class="prompt-row">
        <span class="prompt-label">channel&gt;</span>
        <input id="input-channel-name" class="term-input short" type="text" value="dialtone" />
        <span class="prompt-label">password&gt;</span>
        <input id="input-secret-key" class="term-input" type="password" placeholder="enter password for 'dialtone' channel" />
        <span class="prompt-label">node&gt;</span>
        <input id="input-node-name" class="term-input short" type="text" value="__SELF_NODE__" />
        <input id="input-bootstrap-ticket" class="term-input" type="text" placeholder="optional bootstrap peer/ticket" style="min-width:190px;" />
      </div>
      <div class="prompt-row" style="gap:18px;">
        <button class="term-cmd active" onclick="submitMeshJoin()">[unlock &amp; join channel]</button>
        <button class="term-cmd" onclick="copyShareableTicket()">[copy local ticket]</button>
        <button class="term-cmd" onclick="toggleSetupModal(false)">[close]</button>
        <span style="color:var(--fg-muted);font-size:10px;">cli: <code>dialtone join &lt;password&gt;</code></span>
      </div>
      <div id="setup-feedback" class="term-feedback">ready. enter channel password above to unlock and join the 'dialtone' mesh.</div>
    </div>

    <!-- Borderless 'top'-style process/node table -->
    <table class="top-table">
      <thead>
        <tr>
          <th>NODE</th>
          <th>OS/ARCH</th>
          <th>BIND IP</th>
          <th>HB</th>
          <th>CORES</th>
          <th>CPU%</th>
          <th>MEM%</th>
          <th>DISK%</th>
          <th>NET RX/TX</th>
          <th>STATUS / WARNINGS</th>
        </tr>
      </thead>
      <tbody id="peer-rows"></tbody>
    </table>
  </div>

  <!-- Minimalist typographic footer for selected node details + live log -->
  <div class="term-bottom">
    <div class="focus-line">
      <span>focus <strong id="t-name">__PEER_FOCUS__</strong> (<span id="t-os-arch">--</span>)</span>
      <span>role <strong id="t-role">--</strong></span>
      <span>tailnet <strong id="t-tailnet-flag">off</strong></span>
      <span id="t-epid" style="color:var(--fg-dim);">id --</span>
    </div>
    <div id="t-paths" class="caps-line"></div>
    <div id="t-caps" class="caps-line"></div>
    <div id="hud-ticker" class="log-line">&gt; [gossip] initialized 3d topology projection for __SELF_NODE__</div>
  </div>
</div>

<script>
const INITIAL_STATE = __INITIAL_JSON__;
const API_TOKEN = "__API_TOKEN__";
let meshState = INITIAL_STATE;
let selectedNode = INITIAL_STATE.selected_peer || INITIAL_STATE.self_node || "node-alpha";
let autoOrbit = true;
let camYaw = 0.55;
let camPitch = 0.38;
let camDist = 520;
let dragging = false;
let lastMouse = { x: 0, y: 0 };
let screenNodes = [];
let stars = [];
let pulseWaves = [];

const WARN_CPU_PCT = 80.0;
const WARN_MEM_PCT = 80.0;
const WARN_DISK_PCT = 85.0;

const canvas = document.getElementById("hud-3d-canvas");
const ctx = canvas.getContext("2d");

function resizeCanvas() {
  canvas.width = window.innerWidth * window.devicePixelRatio;
  canvas.height = window.innerHeight * window.devicePixelRatio;
  ctx.setTransform(window.devicePixelRatio, 0, 0, window.devicePixelRatio, 0, 0);
}
window.addEventListener("resize", resizeCanvas);
resizeCanvas();

for (let i = 0; i < 120; i++) {
  stars.push({
    x: (Math.random() - 0.5) * 1400,
    y: (Math.random() - 0.5) * 900,
    z: (Math.random() - 0.5) * 1400,
    s: Math.random() * 1.4 + 0.4
  });
}

canvas.addEventListener("mousedown", (e) => {
  const rect = canvas.getBoundingClientRect();
  const mx = e.clientX - rect.left;
  const my = e.clientY - rect.top;
  for (const sn of screenNodes) {
    const dx = mx - sn.sx;
    const dy = my - sn.sy;
    if (Math.hypot(dx, dy) <= sn.hitRadius) {
      selectNode(sn.node);
      return;
    }
  }
  dragging = true;
  lastMouse = { x: e.clientX, y: e.clientY };
});
window.addEventListener("mousemove", (e) => {
  if (!dragging) return;
  const dx = e.clientX - lastMouse.x;
  const dy = e.clientY - lastMouse.y;
  lastMouse = { x: e.clientX, y: e.clientY };
  camYaw += dx * 0.0065;
  camPitch = Math.max(-1.1, Math.min(1.25, camPitch + dy * 0.0065));
});
window.addEventListener("mouseup", () => { dragging = false; });
canvas.addEventListener("wheel", (e) => {
  e.preventDefault();
  camDist = Math.max(240, Math.min(980, camDist + e.deltaY * 0.45));
}, { passive: false });

function nodeWarnings(n) {
  const hr = n.host_resources || {};
  const warns = [];
  if ((hr.cpu_percent || 0) >= WARN_CPU_PCT) warns.push("HIGH CPU");
  if ((hr.mem_percent || 0) >= WARN_MEM_PCT) warns.push("LOW MEM");
  if ((hr.disk_percent || 0) >= WARN_DISK_PCT) warns.push("LOW DISK");
  if (n.tailnet_used) warns.push("VPN PATH");
  return warns;
}

function asciiBar(pct, isWarn) {
  const slots = 8;
  const filled = Math.max(0, Math.min(slots, Math.round((pct / 100) * slots)));
  const empty = slots - filled;
  const fillCls = isWarn ? "fill red" : "fill";
  return `<span class="ascii-bar">[<span class="${fillCls}">${"|".repeat(filled)}</span>${".".repeat(empty)}]</span>`;
}

function toggleOrbit() {
  autoOrbit = !autoOrbit;
  const btn = document.getElementById("btn-orbit");
  btn.textContent = `[orbit: ${autoOrbit ? "on" : "off"}]`;
  btn.classList.toggle("active", autoOrbit);
}
function resetCamera() {
  camYaw = 0.55; camPitch = 0.38; camDist = 520;
}
function cycleTarget() {
  const nodes = meshState.nodes || [];
  if (!nodes.length) return;
  const idx = nodes.findIndex(n => n.node === selectedNode);
  const next = nodes[(idx + 1) % nodes.length];
  selectNode(next.node);
}
function triggerPulseWave() {
  pulseWaves.push({ r: 10, alpha: 0.85, red: false });
  document.getElementById("hud-ticker").textContent =
    `> [ping] broadcasted 3d topology probe from ${meshState.self_node} across ${meshState.nodes.length} nodes`;
}
function selectNode(name) {
  selectedNode = name;
  pulseWaves.push({ r: 5, alpha: 0.75, red: false });
  renderHudPanels();
}

function toggleSetupModal(forceOpen) {
  const el = document.getElementById("setup-modal-backdrop");
  if (typeof forceOpen === "boolean") {
    el.classList.toggle("open", forceOpen);
  } else {
    el.classList.toggle("open");
  }
}

async function copyShareableTicket() {
  const nodes = meshState.nodes || [];
  const selfEntry = nodes.find(n => n.node === meshState.self_node) || nodes[0];
  const ticket = (selfEntry && selfEntry.ticket) || "";
  if (navigator.clipboard && ticket) {
    await navigator.clipboard.writeText(ticket);
  }
  document.getElementById("setup-feedback").innerHTML =
    `copied ticket: <span style="word-break:break-all;color:var(--fg-bright);">${ticket}</span>`;
}

async function submitMeshJoin() {
  const channelName = (document.getElementById("input-channel-name").value || "dialtone").trim() || "dialtone";
  const password = document.getElementById("input-secret-key").value.trim();
  const bootstrapTicket = document.getElementById("input-bootstrap-ticket").value.trim();
  const nodeName = document.getElementById("input-node-name").value.trim();
  const fb = document.getElementById("setup-feedback");
  if (!password) {
    fb.innerHTML = `<span class="alert-text">error: password is required for channel '${channelName}'</span>`;
    return;
  }
  fb.textContent = `authenticating channel '${channelName}' and dialing seed peers...`;
  try {
    const resp = await fetch("/api/mesh/join", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + API_TOKEN
      },
      body: JSON.stringify({
        channel: channelName,
        password: password,
        bootstrap_ticket: bootstrapTicket,
        node: nodeName
      })
    });
    const data = await resp.json();
    if (!resp.ok) {
      fb.innerHTML = `<span class="alert-text">error: ${data.error || "join failed"}</span>`;
      return;
    }
    triggerPulseWave();
    fb.innerHTML = `unlocked channel <strong style="color:var(--fg-bright)">${data.channel} (${data.channel_fingerprint})</strong> as <strong>${data.node}</strong>`;
    await pollGossipState();
  } catch (err) {
    fb.innerHTML = `<span class="alert-text">error: ${err}</span>`;
  }
}

async function installSystemDaemon() {
  const fb = document.getElementById("setup-feedback");
  toggleSetupModal(true);
  fb.textContent = "installing persistent os service...";
  try {
    const resp = await fetch("/api/daemon/install", {
      method: "POST",
      headers: { "Authorization": "Bearer " + API_TOKEN }
    });
    const data = await resp.json();
    if (!resp.ok) {
      fb.innerHTML = `<span class="alert-text">error: ${data.error || "daemon install failed"}</span>`;
      return;
    }
    fb.innerHTML = `daemon installed: <strong style="color:var(--fg-bright)">${data.instructions}</strong>`;
    await pollGossipState();
  } catch (err) {
    fb.innerHTML = `<span class="alert-text">error: ${err}</span>`;
  }
}

async function stageSampleBlob() {
  const fb = document.getElementById("setup-feedback");
  toggleSetupModal(true);
  const payload = btoa(`dialtone-verified-blob-${Date.now()}-${meshState.self_node}`);
  try {
    const resp = await fetch("/api/artifacts/blob", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer " + API_TOKEN
      },
      body: JSON.stringify({ bytes_base64: payload })
    });
    const data = await resp.json();
    if (!resp.ok) {
      fb.innerHTML = `<span class="alert-text">blob error: ${data.error || "failed"}</span>`;
      return;
    }
    fb.innerHTML = `staged cas blob <strong style="color:var(--fg-bright)">sha256:${data.sha256.slice(0, 16)}..</strong> (${data.bytes}B)`;
  } catch (err) {
    fb.innerHTML = `<span class="alert-text">blob error: ${err}</span>`;
  }
}

function fmtGiB(bytes) {
  return (bytes / (1024 * 1024 * 1024)).toFixed(1);
}

function renderHudPanels() {
  const nodes = meshState.nodes || [];
  const selfRuntime = meshState.self_iroh_runtime || {};
  const cfg = meshState.mesh_config || {};
  const stages = meshState.boot_stages || [];

  document.getElementById("m-self-node").textContent = (meshState.self_node || "node-alpha").toLowerCase();
  document.getElementById("m-bound-ip").textContent = selfRuntime.bound_non_tailnet_ipv4 || "lan";
  document.getElementById("m-channel").textContent = cfg.channel_fingerprint || "unconfigured";
  document.getElementById("m-peer-count").textContent = `${nodes.length} online`;

  if (stages.length) {
    document.getElementById("boot-stage-list").innerHTML = stages.map(s => {
      const isOk = s.status === "ready";
      return `<span class="boot-item ${isOk ? "" : "warn"}">[${s.step}/5 ${s.id}:${s.status}]</span>`;
    }).join("");
  }

  const daemonBadge = document.getElementById("daemon-status-badge");
  daemonBadge.textContent = cfg.daemon_installed ? "active" : "off";

  const setupBtn = document.getElementById("btn-setup");
  setupBtn.classList.toggle("alert", cfg.configured === false);

  let totalCores = 0;
  let totalMemGiB = 0;
  let totalWarnings = 0;

  nodes.forEach(n => {
    const hr = n.host_resources || {};
    totalCores += (hr.cpu_cores || 8);
    totalMemGiB += (hr.mem_total_bytes || 0) / (1024 * 1024 * 1024);
    totalWarnings += nodeWarnings(n).length;
  });

  document.getElementById("m-cluster-res").textContent = `${totalCores}c / ${Math.round(totalMemGiB)}G`;
  document.getElementById("m-warn-count").textContent = String(totalWarnings);
  document.getElementById("m-warn-kv").classList.toggle("alert", totalWarnings > 0);

  const tbody = document.getElementById("peer-rows");
  tbody.innerHTML = nodes.map(n => {
    const hr = n.host_resources || {};
    const cpuVal = hr.cpu_percent || 0;
    const memVal = hr.mem_percent || 0;
    const diskVal = hr.disk_percent || 0;
    const cpuWarn = cpuVal >= WARN_CPU_PCT;
    const memWarn = memVal >= WARN_MEM_PCT;
    const diskWarn = diskVal >= WARN_DISK_PCT;
    const warns = nodeWarnings(n);
    const hasWarn = warns.length > 0;
    const isSel = n.node === selectedNode;
    const ip = (n.iroh_transport && n.iroh_transport.bound_non_tailnet_ipv4) || "wan";
    const nic = (hr.interfaces && hr.interfaces[0]) || { rx_bytes_per_sec: 0, tx_bytes_per_sec: 0 };
    const rxKb = (nic.rx_bytes_per_sec / 1024).toFixed(0);
    const txKb = (nic.tx_bytes_per_sec / 1024).toFixed(0);
    const prefix = isSel ? "&gt; " : "  ";
    const statusHtml = hasWarn
      ? `<span class="alert-text">${warns.join(" / ")}</span>`
      : `ok (${n.role || "peer"})`;

    return `<tr class="node-row ${isSel ? "selected" : ""} ${hasWarn ? "has-warning" : ""}" onclick="selectNode('${n.node}')">
      <td class="node-col">${prefix}${n.node}</td>
      <td>${n.os || "os"}/${n.arch || "arch"}</td>
      <td>${ip}</td>
      <td>#${n.counter}</td>
      <td>${hr.cpu_cores || 8}</td>
      <td class="${cpuWarn ? "alert-text" : ""}">${asciiBar(cpuVal, cpuWarn)}${cpuVal.toFixed(1)}%</td>
      <td class="${memWarn ? "alert-text" : ""}">${asciiBar(memVal, memWarn)}${memVal.toFixed(1)}% (${fmtGiB(hr.mem_used_bytes || 0)}/${fmtGiB(hr.mem_total_bytes || 1)}G)</td>
      <td class="${diskWarn ? "alert-text" : ""}">${diskVal.toFixed(0)}%</td>
      <td>${rxKb}K / ${txKb}K</td>
      <td>${statusHtml}</td>
    </tr>`;
  }).join("");

  const target = nodes.find(n => n.node === selectedNode) || nodes[0];
  if (!target) return;
  document.getElementById("t-name").textContent = target.node;
  document.getElementById("t-role").textContent = target.role || "peer";
  document.getElementById("t-os-arch").textContent = `${target.os || "os"}/${target.arch || "arch"}`;
  document.getElementById("t-epid").textContent = `id ${(target.endpoint_id || "").slice(0, 24)}..`;
  const tFlag = document.getElementById("t-tailnet-flag");
  tFlag.textContent = target.tailnet_used ? "WARN:TAILNET" : "off (physical)";
  tFlag.classList.toggle("alert-text", Boolean(target.tailnet_used));

  const caps = target.capabilities || [];
  document.getElementById("t-caps").textContent = `caps // ${caps.join("   ")}`;

  const obs = target.observed_remote_addrs || [];
  const advIps = (target.iroh_transport && target.iroh_transport.advertised_ip_addrs) || [];
  const paths = [...obs, ...advIps.map(ip => `ip:${ip}`)];
  document.getElementById("t-paths").textContent = `paths // ${paths.join("   ")}`;
}

function project3D(x, y, z, w, h) {
  const cosY = Math.cos(camYaw), sinY = Math.sin(camYaw);
  const cosP = Math.cos(camPitch), sinP = Math.sin(camPitch);
  const x1 = x * cosY - z * sinY;
  const z1 = x * sinY + z * cosY;
  const y2 = y * cosP - z1 * sinP;
  const z2 = y * sinP + z1 * cosP + camDist;
  if (z2 < 40) return null;
  const fov = 560;
  const scale = fov / z2;
  return { sx: w / 2 + x1 * scale, sy: h * 0.57 - y2 * scale, scale, depth: z2 };
}

function drawLine3D(a, b, w, h, color, width, dash = []) {
  const pa = project3D(a.x, a.y, a.z, w, h);
  const pb = project3D(b.x, b.y, b.z, w, h);
  if (!pa || !pb) return;
  ctx.save();
  ctx.strokeStyle = color;
  ctx.lineWidth = width;
  if (dash.length) ctx.setLineDash(dash);
  ctx.beginPath();
  ctx.moveTo(pa.sx, pa.sy);
  ctx.lineTo(pb.sx, pb.sy);
  ctx.stroke();
  ctx.restore();
}

function computeNodePositions3D(nodes, t) {
  const count = nodes.length;
  const radius = count <= 1 ? 0 : 195;
  return nodes.map((n, i) => {
    const angle = (i / Math.max(1, count)) * Math.PI * 2 + 0.3;
    const bob = Math.sin(t * 0.002 + i * 1.7) * 8;
    return {
      node: n.node,
      data: n,
      x: Math.cos(angle) * radius,
      y: bob,
      z: Math.sin(angle) * radius
    };
  });
}

function renderFrame(t) {
  const w = window.innerWidth;
  const h = window.innerHeight;
  ctx.clearRect(0, 0, w, h);

  if (autoOrbit && !dragging) {
    camYaw += 0.0018;
  }

  // 1. Monochrome starfield dust
  ctx.fillStyle = "rgba(255, 255, 255, 0.22)";
  for (const st of stars) {
    const p = project3D(st.x, st.y, st.z, w, h);
    if (p) {
      ctx.fillRect(p.sx, p.sy, st.s * p.scale, st.s * p.scale);
    }
  }

  // 2. Minimalist wireframe floor grid (y = -75)
  const floorY = -75;
  for (let g = -320; g <= 320; g += 64) {
    drawLine3D({ x: g, y: floorY, z: -320 }, { x: g, y: floorY, z: 320 }, w, h, "rgba(255, 255, 255, 0.04)", 1);
    drawLine3D({ x: -320, y: floorY, z: g }, { x: 320, y: floorY, z: g }, w, h, "rgba(255, 255, 255, 0.04)", 1);
  }

  [120, 195, 285].forEach((r, idx) => {
    ctx.save();
    ctx.strokeStyle = idx === 1 ? "rgba(255, 255, 255, 0.16)" : "rgba(255, 255, 255, 0.06)";
    ctx.lineWidth = 1;
    ctx.beginPath();
    let started = false;
    for (let a = 0; a <= 64; a++) {
      const th = (a / 64) * Math.PI * 2;
      const p = project3D(Math.cos(th) * r, floorY, Math.sin(th) * r, w, h);
      if (p) {
        if (!started) { ctx.moveTo(p.sx, p.sy); started = true; }
        else ctx.lineTo(p.sx, p.sy);
      }
    }
    ctx.stroke();
    ctx.restore();
  });

  pulseWaves = pulseWaves.filter(pw => pw.alpha > 0.02);
  for (const pw of pulseWaves) {
    pw.r += 4.2;
    pw.alpha *= 0.96;
    ctx.save();
    ctx.strokeStyle = pw.red
      ? `rgba(255, 46, 59, ${pw.alpha})`
      : `rgba(244, 244, 246, ${pw.alpha * 0.6})`;
    ctx.lineWidth = 1.4;
    ctx.beginPath();
    let started = false;
    for (let a = 0; a <= 48; a++) {
      const th = (a / 48) * Math.PI * 2;
      const p = project3D(Math.cos(th) * pw.r, floorY, Math.sin(th) * pw.r, w, h);
      if (p) {
        if (!started) { ctx.moveTo(p.sx, p.sy); started = true; }
        else ctx.lineTo(p.sx, p.sy);
      }
    }
    ctx.stroke();
    ctx.restore();
  }

  const nodes = meshState.nodes || [];
  const pos3d = computeNodePositions3D(nodes, t);
  const relayPos = { x: 0, y: 145 + Math.sin(t * 0.0025) * 5, z: 0 };

  // 3. Monochrome QUIC mesh edges & packet pulses
  for (let i = 0; i < pos3d.length; i++) {
    const a = pos3d[i];
    drawLine3D(a, relayPos, w, h, "rgba(255, 255, 255, 0.14)", 1, [3, 5]);
    drawLine3D(a, { x: a.x, y: floorY, z: a.z }, w, h, "rgba(255, 255, 255, 0.12)", 1, [2, 4]);

    for (let j = i + 1; j < pos3d.length; j++) {
      const b = pos3d[j];
      const edgeWarn = nodeWarnings(a.data).length > 0 || nodeWarnings(b.data).length > 0;
      drawLine3D(
        a,
        b,
        w,
        h,
        edgeWarn ? "rgba(255, 46, 59, 0.45)" : "rgba(255, 255, 255, 0.28)",
        1.4
      );

      for (let k = 0; k < 2; k++) {
        const frac = ((t * 0.0005 + k * 0.5 + (i + j) * 0.17) % 1.0);
        const dir = k === 0 ? frac : 1.0 - frac;
        const px = a.x + (b.x - a.x) * dir;
        const py = a.y + (b.y - a.y) * dir;
        const pz = a.z + (b.z - a.z) * dir;
        const pp = project3D(px, py, pz, w, h);
        if (pp) {
          ctx.save();
          ctx.fillStyle = edgeWarn ? "#ff2e3b" : "#f4f4f6";
          ctx.beginPath();
          ctx.arc(pp.sx, pp.sy, Math.max(1.8, 3.0 * pp.scale), 0, Math.PI * 2);
          ctx.fill();
          ctx.restore();
        }
      }
    }
  }

  // 4. Orbital Iroh 1.3 Relay wireframe node
  const rp = project3D(relayPos.x, relayPos.y, relayPos.z, w, h);
  if (rp) {
    const rSize = 13 * rp.scale;
    ctx.save();
    ctx.strokeStyle = "rgba(244, 244, 246, 0.55)";
    ctx.lineWidth = 1.2;
    ctx.translate(rp.sx, rp.sy);
    ctx.rotate(t * 0.0012);
    ctx.strokeRect(-rSize, -rSize, rSize * 2, rSize * 2);
    ctx.restore();

    ctx.fillStyle = "rgba(161, 161, 170, 0.85)";
    ctx.font = "10px monospace";
    ctx.textAlign = "center";
    ctx.fillText("relay // n0.iroh", rp.sx, rp.sy - rSize - 8);
  }

  // 5. 3D Gossip Peer Nodes (greyscale + red warning highlights)
  screenNodes = [];
  const projectedNodes = pos3d.map(item => {
    const p = project3D(item.x, item.y, item.z, w, h);
    return p ? { ...item, ...p } : null;
  }).filter(Boolean).sort((a, b) => b.depth - a.depth);

  for (const pn of projectedNodes) {
    const isSel = pn.node === selectedNode;
    const warns = nodeWarnings(pn.data);
    const hasWarn = warns.length > 0;
    const radius = Math.max(14, 24 * pn.scale);
    const strokeCol = hasWarn
      ? "#ff2e3b"
      : (isSel ? "#ffffff" : "rgba(196, 196, 204, 0.75)");
    const hr = pn.data.host_resources || {};
    const cpuPct = Math.min(100, hr.cpu_percent || 10) / 100;
    const memPct = Math.min(100, hr.mem_percent || 20) / 100;

    screenNodes.push({ node: pn.node, sx: pn.sx, sy: pn.sy, hitRadius: radius * 1.5 });

    ctx.save();
    ctx.translate(pn.sx, pn.sy);

    // Outer target ring
    ctx.strokeStyle = strokeCol;
    ctx.lineWidth = isSel || hasWarn ? 2.0 : 1.1;
    ctx.beginPath();
    ctx.arc(0, 0, radius * 1.32, t * 0.0018, t * 0.0018 + Math.PI * 1.4);
    ctx.stroke();

    // CPU arc (red if warning, light grey otherwise)
    ctx.strokeStyle = (hr.cpu_percent || 0) >= WARN_CPU_PCT ? "#ff2e3b" : "rgba(244, 244, 246, 0.85)";
    ctx.lineWidth = 2.4;
    ctx.beginPath();
    ctx.arc(0, 0, radius * 1.08, -Math.PI / 2, -Math.PI / 2 + cpuPct * Math.PI * 2);
    ctx.stroke();

    // Memory arc (red if warning, mid grey otherwise)
    ctx.strokeStyle = (hr.mem_percent || 0) >= WARN_MEM_PCT ? "#ff2e3b" : "rgba(113, 113, 122, 0.85)";
    ctx.lineWidth = 2.0;
    ctx.beginPath();
    ctx.arc(0, 0, radius * 0.88, -Math.PI / 2, -Math.PI / 2 + memPct * Math.PI * 2);
    ctx.stroke();

    // Core sphere
    ctx.fillStyle = hasWarn ? "rgba(255, 46, 59, 0.85)" : (isSel ? "#f4f4f6" : "rgba(161, 161, 170, 0.7)");
    ctx.beginPath();
    ctx.arc(0, 0, radius * 0.42, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();

    // Typography labels
    ctx.save();
    ctx.textAlign = "center";
    ctx.font = isSel ? "bold 11px monospace" : "11px monospace";
    ctx.fillStyle = hasWarn ? "#ff2e3b" : (isSel ? "#ffffff" : "#d4d4d8");
    ctx.fillText(pn.node, pn.sx, pn.sy - radius * 1.55);

    ctx.font = "10px monospace";
    ctx.fillStyle = hasWarn ? "#ff2e3b" : "#71717a";
    const subLabel = hasWarn
      ? warns.join(" / ")
      : `cpu ${(hr.cpu_percent || 0).toFixed(0)}%  mem ${(hr.mem_percent || 0).toFixed(0)}%`;
    ctx.fillText(subLabel, pn.sx, pn.sy + radius * 1.65);
    ctx.restore();
  }

  requestAnimationFrame(renderFrame);
}

async function pollGossipState() {
  try {
    const resp = await fetch("/api/gossip", { cache: "no-store" });
    if (!resp.ok) {
      document.getElementById("sync-lamp").textContent = "err";
      document.getElementById("sync-lamp").className = "alert-text";
      return;
    }
    const data = await resp.json();
    meshState = data;
    document.getElementById("sync-lamp").textContent = "live";
    document.getElementById("sync-lamp").className = "";
    if (!meshState.nodes.some(n => n.node === selectedNode) && meshState.nodes.length > 0) {
      selectedNode = meshState.self_node || meshState.nodes[0].node;
    }
    renderHudPanels();
    const peerSummary = (meshState.nodes || []).map(n => `${n.node}#${n.counter}`).join("  ");
    document.getElementById("hud-ticker").textContent =
      `> [gossip-sync] ${meshState.nodes.length} nodes active (tailnet=off) :: ${peerSummary}`;
  } catch (e) {
    document.getElementById("sync-lamp").textContent = "offline";
    document.getElementById("sync-lamp").className = "alert-text";
  }
}

renderHudPanels();
if (INITIAL_STATE.mesh_config && INITIAL_STATE.mesh_config.configured === false) {
  toggleSetupModal(true);
}
requestAnimationFrame(renderFrame);
setInterval(pollGossipState, 1500);
</script>
</body>
</html>"##;

    template
        .replace("__SELF_NODE__", &state.node)
        .replace("__PEER_FOCUS__", peer_focus)
        .replace("__INITIAL_JSON__", &initial_json)
        .replace("__API_TOKEN__", &state.api_token)
}

fn constant_time_eq(a: &[u8], b: &[u8]) -> bool {
    if a.len() != b.len() {
        return false;
    }
    let mut diff = 0u8;
    for (&x, &y) in a.iter().zip(b.iter()) {
        diff |= x ^ y;
    }
    diff == 0
}

fn validate_bearer_token(
    headers: &HashMap<String, String>,
    expected_token: &str,
) -> Result<(), String> {
    if expected_token.trim().is_empty() {
        return Err("API token is not configured on this daemon".to_string());
    }
    let provided = headers
        .get("authorization")
        .and_then(|v| {
            let v = v.trim();
            v.strip_prefix("Bearer ")
                .or_else(|| v.strip_prefix("bearer "))
                .map(str::trim)
        })
        .or_else(|| headers.get("x-dialtone-token").map(|s| s.trim()));

    match provided {
        Some(tok) if constant_time_eq(tok.as_bytes(), expected_token.trim().as_bytes()) => Ok(()),
        Some(_) => Err("invalid API bearer token".to_string()),
        None => Err(
            "missing API authentication: pass 'Authorization: Bearer <token>' (see ~/.dialtone/api-token)"
                .to_string(),
        ),
    }
}

fn validate_local_http_request(
    method: &str,
    headers: &HashMap<String, String>,
) -> Result<(), String> {
    if let Some(host_val) = headers.get("host") {
        let host_only = host_val
            .trim()
            .trim_start_matches('[')
            .split(']')
            .next()
            .unwrap_or("")
            .split(':')
            .next()
            .unwrap_or("")
            .to_ascii_lowercase();
        let allowed_host = matches!(
            host_only.as_str(),
            "" | "localhost" | "127.0.0.1" | "::1" | "0.0.0.0"
        );
        if !allowed_host {
            return Err(format!(
                "forbidden Host header '{host_val}': local HTTP control plane only accepts localhost/127.0.0.1"
            ));
        }
    }

    if method.eq_ignore_ascii_case("POST") {
        if let Some(origin_val) = headers.get("origin") {
            let orig = origin_val.trim().to_ascii_lowercase();
            let allowed_origin = orig == "null"
                || orig.starts_with("http://localhost")
                || orig.starts_with("http://127.0.0.1")
                || orig.starts_with("http://[::1]")
                || orig.starts_with("https://localhost")
                || orig.starts_with("https://127.0.0.1")
                || orig.starts_with("https://[::1]");
            if !allowed_origin {
                return Err(format!(
                    "forbidden Origin header '{origin_val}': cross-origin POST requests are blocked"
                ));
            }
        }
    }

    Ok(())
}

fn parse_query(query: &str) -> HashMap<String, String> {
    let mut map = HashMap::new();
    for pair in query.split('&') {
        if pair.is_empty() {
            continue;
        }
        let (k, v) = pair.split_once('=').unwrap_or((pair, ""));
        map.insert(k.to_string(), v.to_string());
    }
    map
}

fn write_json<T: Serialize>(stream: &mut TcpStream, status: u16, value: &T) -> Result<(), String> {
    let body = serde_json::to_vec(value).map_err(|e| e.to_string())?;
    write_response(stream, status, "application/json", &body, false)
}

fn write_response(
    stream: &mut TcpStream,
    status: u16,
    content_type: &str,
    body: &[u8],
    head_only: bool,
) -> Result<(), String> {
    let reason = match status {
        200 => "OK",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        413 => "Payload Too Large",
        503 => "Service Unavailable",
        _ => "Response",
    };
    let header = format!(
        "HTTP/1.1 {status} {reason}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    );
    stream
        .write_all(header.as_bytes())
        .map_err(|e| e.to_string())?;
    if !head_only {
        stream.write_all(body).map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::identity::{build_diagnostics, IrohTransportConfig};
    use iroh::{EndpointAddr, SecretKey};
    use std::env;
    use std::thread;

    #[test]
    fn http_and_websocket_server_serves_ui_state_blobs_catalogs_and_ws_upgrade() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-http-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();

        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-123");

        let state = Arc::new(Mutex::new(DaemonRuntimeState::new_seeded(
            "node-alpha",
            "hub",
            &addr.to_string(),
            tmp_dir.clone(),
            diag,
        )));

        let state_bg = state.clone();
        let handle = thread::spawn(move || {
            serve_http_blocking(listener, state_bg, Some(6)).unwrap();
        });

        // 1. GET / dashboard UI
        let resp_ui = raw_http_request(addr, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n");
        assert!(resp_ui.contains("200 OK"));
        assert!(resp_ui.contains("Dialtone Iroh 1.3 Mesh Control Plane"));

        // 2. GET /api/state?kv_offset=0&kv_limit=1
        let resp_state = raw_http_request(
            addr,
            "GET /api/state?kv_offset=0&kv_limit=1 HTTP/1.1\r\nHost: localhost\r\n\r\n",
        );
        assert!(resp_state.contains("200 OK"));
        assert!(resp_state.contains("\"kv_page\""));
        assert!(resp_state.contains("\"has_more\":true"));

        // 3. GET /api/catalogs/jobs
        let resp_cat = raw_http_request(
            addr,
            "GET /api/catalogs/jobs HTTP/1.1\r\nHost: localhost\r\n\r\n",
        );
        assert!(resp_cat.contains("200 OK"));
        assert!(resp_cat.contains("Mesh Health Audit"));

        // 4. GET /api/rollback-plan?node=edge-1
        let resp_rb = raw_http_request(
            addr,
            "GET /api/rollback-plan?node=edge-1 HTTP/1.1\r\nHost: localhost\r\n\r\n",
        );
        assert!(resp_rb.contains("200 OK"));
        assert!(resp_rb.contains("\"side_effect_free_preview\":true"));

        // 5. Reject external Host / cross-origin POST (CSRF / DNS rebinding defense)
        let resp_csrf = raw_http_request(
            addr,
            "POST /api/daemon/install HTTP/1.1\r\nHost: localhost\r\nOrigin: https://evil.example.com\r\nContent-Length: 2\r\n\r\n{}",
        );
        assert!(resp_csrf.contains("403 Forbidden"));

        // 6. GET /ws WebSocket upgrade
        let resp_ws = raw_http_request(
            addr,
            "GET /ws HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n",
        );
        assert!(resp_ws.contains("101 Switching Protocols"));
        assert!(resp_ws.contains("s3pPLMBiTxaQ9kYGzzhZRbK+xOo="));

        handle.join().unwrap();
        let _ = std::fs::remove_dir_all(&tmp_dir);
    }

    #[test]
    fn issue_7_post_routes_require_bearer_token_and_non_loopback_listen_is_guarded() {
        assert!(validate_listen_addr_security("127.0.0.1:8787", false).is_ok());
        assert!(validate_listen_addr_security("localhost:8787", false).is_ok());
        assert!(validate_listen_addr_security("[::1]:8787", false).is_ok());
        assert!(validate_listen_addr_security("0.0.0.0:8787", false).is_err());
        assert!(validate_listen_addr_security("0.0.0.0:8787", true).is_ok());

        let tmp_dir = env::temp_dir().join(format!("dialtone-auth-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();

        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-auth");

        let mut seeded = DaemonRuntimeState::new_seeded(
            "node-alpha",
            "hub",
            &addr.to_string(),
            tmp_dir.clone(),
            diag,
        );
        seeded.api_token = "dtk-test-token-1234567890abcdef".to_string();
        let state = Arc::new(Mutex::new(seeded));

        let state_bg = state.clone();
        let handle = thread::spawn(move || {
            serve_http_blocking(listener, state_bg, Some(3)).unwrap();
        });

        // 1. POST without token -> 401 Unauthorized
        let resp_unauth = raw_http_request(
            addr,
            "POST /api/artifacts/blob HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 27\r\n\r\n{\"bytes_base64\":\"ZGlhbHRvbmU=\"}",
        );
        assert!(resp_unauth.contains("401 Unauthorized"));

        // 2. POST with wrong token -> 401 Unauthorized
        let resp_wrong = raw_http_request(
            addr,
            "POST /api/artifacts/blob HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer wrong-token\r\nContent-Type: application/json\r\nContent-Length: 27\r\n\r\n{\"bytes_base64\":\"ZGlhbHRvbmU=\"}",
        );
        assert!(resp_wrong.contains("401 Unauthorized"));

        // 3. POST with valid Bearer token -> 200 OK
        let body = "{\"bytes_base64\":\"ZGlhbHRvbmU=\"}";
        let req_ok = format!(
            "POST /api/artifacts/blob HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer dtk-test-token-1234567890abcdef\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
            body.len(),
            body
        );
        let resp_ok = raw_http_request(addr, &req_ok);
        assert!(resp_ok.contains("200 OK"));
        assert!(resp_ok.contains("\"sha256\""));

        handle.join().unwrap();
        let _ = std::fs::remove_dir_all(&tmp_dir);
    }

    #[test]
    fn issue_8_concurrent_requests_succeed_while_stalled_connection_is_open() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-conc-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();

        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-conc");

        let state = Arc::new(Mutex::new(DaemonRuntimeState::new_seeded(
            "node-alpha",
            "hub",
            &addr.to_string(),
            tmp_dir.clone(),
            diag,
        )));

        let state_bg = state.clone();
        let handle = thread::spawn(move || {
            serve_http_blocking(listener, state_bg, Some(3)).unwrap();
        });

        // Client 1 opens a TCP connection and stalls (never sends headers)
        let _stalled_stream = TcpStream::connect(addr).unwrap();
        thread::sleep(Duration::from_millis(40));

        // Clients 2 and 3 must still be served immediately in < 500ms without head-of-line blocking
        let start = std::time::Instant::now();
        let resp_2 = raw_http_request(addr, "GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n");
        let resp_3 = raw_http_request(addr, "GET /api/gossip HTTP/1.1\r\nHost: localhost\r\n\r\n");
        let elapsed = start.elapsed();

        assert!(resp_2.contains("200 OK"));
        assert!(resp_3.contains("200 OK"));
        assert!(
            elapsed < Duration::from_millis(800),
            "concurrent requests took {:?}, expected < 800ms despite stalled connection",
            elapsed
        );

        drop(_stalled_stream);
        handle.join().unwrap();
        let _ = std::fs::remove_dir_all(&tmp_dir);
    }

    fn raw_http_request(addr: std::net::SocketAddr, req: &str) -> String {
        let mut stream = TcpStream::connect(addr).unwrap();
        stream.write_all(req.as_bytes()).unwrap();
        let mut out = Vec::new();
        stream.read_to_end(&mut out).unwrap();
        String::from_utf8_lossy(&out).to_string()
    }
}
