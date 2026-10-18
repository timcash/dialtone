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
    BlobTransportReport, LiveBlobBridge, TieredBlobFetchRequest,
};
use crate::deploy::{
    evaluate_deploy_build_decision, execute_verified_rollback, generate_deploy_plan,
    install_persistent_os_daemon, plan_rollback, probe_build_cache, render_install_v1_sh,
    validate_manifest_contract, BuildCacheRecord, BuildProbeRequest, DeployManifest, DeploySlot,
};
use crate::evidence::{
    paginate_kv_records, plan_alert_ack, sample_live_host_resources, AlertRecord, EventRecord,
    HostResourceTelemetry, KeyValueRecord, KvPageMeta, ServiceLogRecord,
};
use crate::gossip::{
    build_mesh_rtt_matrix, cached_local_public_ip_geo, default_arch, default_config_path,
    default_os, default_state_dir, monotonic_now_ms, node_capabilities_for, node_role_for,
    normalize_channel_name, now_unix, prune_stale_peers, resolve_fleet_node_locations,
    save_mesh_config, secret_channel_fingerprint, validate_channel_password, BootStageStatus,
    DialtoneHeartbeat, MeshControlCommand, PersistentMeshConfig, DEFAULT_CHANNEL_NAME,
    DEFAULT_PEER_TTL_SECS, HEARTBEAT_INTERVAL_SECS, SUMMARY_HEARTBEAT_INTERVAL_SECS,
};
use crate::h3_land::LAND_H3_BITSET_B64;
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
    pub blob_bridge: Option<LiveBlobBridge>,
    #[serde(skip)]
    pub supervisor: Arc<Mutex<ManagedProcessSupervisor>>,
    pub metadata: DialtoneMetadata,
    pub iroh_runtime: IrohTransportDiagnostics,
    pub nodes: Vec<DialtoneHeartbeat>,
    #[serde(default)]
    pub mesh_rtt_ms: std::collections::BTreeMap<String, f64>,
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
    pub fn record_log(
        &mut self,
        service: &str,
        level: &str,
        stream: &str,
        message: impl Into<String>,
    ) {
        let next_cursor = self.logs.last().map(|l| l.cursor + 1).unwrap_or(1);
        self.logs.push(ServiceLogRecord {
            service: service.to_string(),
            level: level.to_string(),
            stream: stream.to_string(),
            cursor: next_cursor,
            message: message.into(),
            timestamp_unix: now_unix(),
        });
        if self.logs.len() > 256 {
            let drain = self.logs.len() - 256;
            self.logs.drain(0..drain);
        }
    }

    pub fn record_event(&mut self, kind: &str, severity: &str, message: impl Into<String>) {
        let now = now_unix();
        let next_idx = self.events.len() + 1;
        self.events.push(EventRecord {
            id: format!("evt-{now}-{next_idx}"),
            kind: kind.to_string(),
            severity: severity.to_string(),
            source: self.node.clone(),
            message: message.into(),
            timestamp_unix: now,
            test_session_id: "session-live".to_string(),
            daemon_owned: true,
        });
        if self.events.len() > 256 {
            let drain = self.events.len() - 256;
            self.events.drain(0..drain);
        }
    }

    pub fn refresh_live_alerts(&mut self) {
        let now = now_unix();
        let mut live_alerts = Vec::new();
        for peer in &self.nodes {
            if let Some(ref summary) = peer.summary_stats {
                if summary.thrashing_detected {
                    live_alerts.push(AlertRecord {
                        id: format!("alert-{}-thrashing", peer.node),
                        target_node: peer.node.clone(),
                        status: "active".to_string(),
                        severity: "warning".to_string(),
                        source: "summary_heartbeat".to_string(),
                        summary: format!(
                            "Node {} thrashing detected (score {:.1}): {}",
                            peer.node,
                            summary.thrashing_score,
                            summary.thrashing_reasons.join(", ")
                        ),
                        telemetry_ref: format!("dialtone://{}/telemetry/summary", peer.node),
                        last_seen_unix: now,
                        acknowledged_by: None,
                    });
                }
            }
            if peer.host_resources.disk_percent >= 85.0 {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-disk", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "telemetry".to_string(),
                    summary: format!(
                        "Node {} disk usage high ({:.1}%)",
                        peer.node, peer.host_resources.disk_percent
                    ),
                    telemetry_ref: format!("dialtone://{}/telemetry/disk", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
            if peer.host_resources.cpu_percent >= 85.0 {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-cpu", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "telemetry".to_string(),
                    summary: format!(
                        "Node {} CPU usage high ({:.1}%)",
                        peer.node, peer.host_resources.cpu_percent
                    ),
                    telemetry_ref: format!("dialtone://{}/telemetry/cpu", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
            if peer.host_resources.mem_percent >= 85.0 {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-mem", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "telemetry".to_string(),
                    summary: format!(
                        "Node {} memory usage high ({:.1}%)",
                        peer.node, peer.host_resources.mem_percent
                    ),
                    telemetry_ref: format!("dialtone://{}/telemetry/mem", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
        }
        if live_alerts.is_empty() {
            live_alerts.push(AlertRecord {
                id: format!("alert-{}-nominal", self.node),
                target_node: self.node.clone(),
                status: "active".to_string(),
                severity: "info".to_string(),
                source: "telemetry".to_string(),
                summary: format!(
                    "Node {} telemetry nominal (CPU {:.1}%, MEM {:.1}%, DISK {:.1}%)",
                    self.node,
                    self.host_resources.cpu_percent,
                    self.host_resources.mem_percent,
                    self.host_resources.disk_percent
                ),
                telemetry_ref: format!("dialtone://{}/telemetry", self.node),
                last_seen_unix: now,
                acknowledged_by: None,
            });
        }
        self.alerts = live_alerts;
    }

    pub fn reconcile_live_processes(&mut self) {
        let mut started_logs = Vec::new();
        let sup_arc = self.supervisor.clone();
        if let Ok(mut sup) = sup_arc.lock() {
            for spec in self.managed_processes.iter_mut() {
                if std::path::Path::new(&spec.path).exists() {
                    let prev_pid = spec.pid;
                    let _ = sup.reconcile(std::slice::from_mut(spec));
                    if spec.desired_state == "running" && spec.pid.is_some() && spec.pid != prev_pid
                    {
                        let msg = format!(
                            "Supervised blob process '{}' active (sha256={}, pid={}, path={})",
                            spec.name,
                            spec.sha256,
                            spec.pid.unwrap_or(0),
                            spec.path
                        );
                        println!("[SUPERVISOR] {msg}");
                        started_logs.push((spec.name.clone(), msg));
                    }
                } else {
                    spec.pid = None;
                    spec.status = Some("missing_binary".to_string());
                }
            }
        }
        for (svc, msg) in started_logs {
            self.record_log(&svc, "INFO", "supervisor", msg);
        }
        let self_node = self.node.clone();
        let procs_snapshot = self.managed_processes.clone();
        if let Some(self_hb) = self.nodes.iter_mut().find(|n| n.node == self_node) {
            self_hb.managed_processes = procs_snapshot;
            if let Some(local_geo) = cached_local_public_ip_geo() {
                self_hb.geo_location = Some(local_geo);
            }
        }
        self.mesh_rtt_ms = build_mesh_rtt_matrix(&self.nodes);
        resolve_fleet_node_locations(&mut self.nodes, &self.mesh_rtt_ms);
    }

    pub fn new_live(
        node: &str,
        mode: &str,
        listen: &str,
        blob_cache_dir: PathBuf,
        iroh_runtime: IrohTransportDiagnostics,
    ) -> Self {
        let now = now_unix();
        let host_resources =
            sample_live_host_resources(node, iroh_runtime.bound_non_tailnet_ipv4.as_deref(), 1);
        let service_endpoints = vec![ServiceAdvertisement {
            name: "dialtone-control".to_string(),
            kind: "http-control-plane".to_string(),
            protocol: "http".to_string(),
            url: format!("http://{listen}"),
            listen: listen.to_string(),
            status: "running".to_string(),
            advertised_by: node.to_string(),
            role: mode.to_string(),
            mesh: "dialtone-iroh-gossip".to_string(),
            subjects: vec!["dialtone.iroh.heartbeat.v1".to_string()],
            status_probe: None,
        }];

        let self_hb = DialtoneHeartbeat {
            schema: "dialtone.iroh.heartbeat.v1".to_string(),
            node: node.to_string(),
            endpoint_id: iroh_runtime.endpoint_id.clone(),
            ticket: format!("iroh-gossip-v1:{node}"),
            counter: 1,
            updated_at_unix: now,
            heartbeat_kind: "summary".to_string(),
            heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
            sent_mono_ms: monotonic_now_ms(),
            rtt_probe_requested: true,
            peer_echoes: HashMap::new(),
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            channel_mac: None,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for(node),
            capabilities: node_capabilities_for(node),
            host_resources: host_resources.clone(),
            service_endpoints: service_endpoints.clone(),
            managed_processes: Vec::new(),
            iroh_transport: iroh_runtime.clone(),
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            geo_location: cached_local_public_ip_geo(),
        };

        let mut installed_binaries = Vec::new();
        if let Ok(exe_path) = std::env::current_exe() {
            if let Ok(bytes) = fs::read(&exe_path) {
                let sha256 = crate::blobs::compute_sha256(&bytes);
                installed_binaries.push(InstalledBinary {
                    name: "dialtone".to_string(),
                    path: exe_path.to_string_lossy().to_string(),
                    sha256,
                });
            }
        }

        let boot_stages = default_boot_stages(listen, &blob_cache_dir);
        let mesh_config = PersistentMeshConfig {
            node_name: Some(node.to_string()),
            listen_addr: Some(listen.to_string()),
            channel: DEFAULT_CHANNEL_NAME.to_string(),
            configured: false,
            secret_key: None,
            derived_keys: None,
            channel_fingerprint: "unconfigured".to_string(),
            bootstrap_tickets: Vec::new(),
            seed_peers: Vec::new(),
            allowed_endpoint_ids: Vec::new(),
            used_invite_nonces: Vec::new(),
            daemon_installed: false,
            daemon_service_path: None,
            managed_processes: Vec::new(),
            updated_at_unix: now,
        };
        let api_token = load_or_create_api_token().unwrap_or_else(|_| "dtk-live-token".to_string());

        let mut st = Self {
            node: node.to_string(),
            mode: mode.to_string(),
            listen: listen.to_string(),
            api_token,
            blob_cache_dir,
            allowed_deploy_roots: vec!["/opt/dialtone/".to_string(), "/tmp/".to_string()],
            boot_stages,
            mesh_config,
            control_tx: None,
            blob_bridge: None,
            supervisor: Arc::new(Mutex::new(ManagedProcessSupervisor::new())),
            metadata: DialtoneMetadata::default(),
            iroh_runtime,
            nodes: vec![self_hb],
            mesh_rtt_ms: std::collections::BTreeMap::new(),
            installed_binaries,
            managed_processes: Vec::new(),
            deploy_slots: Vec::new(),
            deploy_manifests: Vec::new(),
            deploy_build_caches: Vec::new(),
            blob_fetches: Vec::new(),
            service_endpoints,
            key_leases: Vec::new(),
            host_resources: host_resources.clone(),
            events: Vec::new(),
            logs: Vec::new(),
            alerts: Vec::new(),
            robot_command_intents: Vec::new(),
            cursor_jobs: Vec::new(),
            kv: vec![
                KeyValueRecord {
                    key: "mesh/self_node".to_string(),
                    value: json!(node),
                    updated_at_unix: now,
                },
                KeyValueRecord {
                    key: "mesh/protocol".to_string(),
                    value: json!("iroh-1.3.0"),
                    updated_at_unix: now,
                },
                KeyValueRecord {
                    key: "mesh/heartbeat_interval_secs".to_string(),
                    value: json!(HEARTBEAT_INTERVAL_SECS),
                    updated_at_unix: now,
                },
                KeyValueRecord {
                    key: "mesh/summary_heartbeat_interval_secs".to_string(),
                    value: json!(SUMMARY_HEARTBEAT_INTERVAL_SECS),
                    updated_at_unix: now,
                },
            ],
            kv_page: None,
        };
        st.record_event(
            "mesh.boot",
            "info",
            format!(
                "dialtone live mesh daemon initialized on {} ({}/{}, {} cores, {:.1}% mem, {:.1}% disk)",
                node,
                default_os(),
                default_arch(),
                host_resources.cpu_cores,
                host_resources.mem_percent,
                host_resources.disk_percent
            ),
        );
        st.record_log(
            "dialtone-daemon",
            "INFO",
            "stdout",
            format!(
                "started live telemetry & gossip runtime on http://{} (hb={}s, summary={}s, cpu={:.1}%, mem={:.1}%, disk={:.1}%)",
                listen,
                HEARTBEAT_INTERVAL_SECS,
                SUMMARY_HEARTBEAT_INTERVAL_SECS,
                host_resources.cpu_percent,
                host_resources.mem_percent,
                host_resources.disk_percent
            ),
        );
        resolve_fleet_node_locations(&mut st.nodes, &st.mesh_rtt_ms);
        st.refresh_live_alerts();
        st
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
                heartbeat_kind: "summary".to_string(),
                heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
                summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
                sent_mono_ms: monotonic_now_ms(),
                rtt_probe_requested: false,
                peer_echoes: HashMap::new(),
                peer_rtt_ms: std::collections::BTreeMap::new(),
                summary_stats: None,
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
                geo_location: None,
            });
        }
        let seeded_rtt = build_mesh_rtt_matrix(&peer_heartbeats);
        resolve_fleet_node_locations(&mut peer_heartbeats, &seeded_rtt);

        let metadata = DialtoneMetadata {
            catalogs: vec![
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
            ],
            ..Default::default()
        };

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
            blob_bridge: None,
            supervisor: Arc::new(Mutex::new(ManagedProcessSupervisor::new())),
            metadata,
            iroh_runtime,
            nodes: peer_heartbeats,
            mesh_rtt_ms: seeded_rtt,
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
                restart_count: 0,
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
            st.reconcile_live_processes();
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
                "heartbeat_interval_secs": HEARTBEAT_INTERVAL_SECS,
                "summary_interval_secs": SUMMARY_HEARTBEAT_INTERVAL_SECS,
                "self_iroh_runtime": st.iroh_runtime,
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config),
                "nodes": st.nodes,
                "mesh_rtt_ms": st.mesh_rtt_ms,
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
        ("POST", "/api/artifacts/blob") | ("POST", "/api/blobs/share") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let b64 = parsed
                .get("bytes_base64")
                .and_then(Value::as_str)
                .unwrap_or_default();
            let expected_sha = parsed
                .get("sha256")
                .and_then(Value::as_str)
                .map(String::from);
            match crate::base64_decode(b64) {
                Ok(raw_bytes) => {
                    let maybe_bridge = st.blob_bridge.clone();
                    let cache_dir = st.blob_cache_dir.clone();
                    let node_name = st.node.clone();
                    drop(st);
                    if let Some(bridge) = maybe_bridge {
                        match bridge.share_verified_blob(
                            &cache_dir,
                            &raw_bytes,
                            expected_sha.as_deref(),
                        ) {
                            Ok(rep) => write_json(
                                &mut stream,
                                200,
                                &json!({
                                    "status": rep.status,
                                    "node": node_name,
                                    "sha256": rep.sha256,
                                    "blake3_hash": rep.blake3_hash,
                                    "path": rep.path,
                                    "bytes": rep.bytes,
                                    "provider_endpoint_id": rep.provider_endpoint_id,
                                    "ticket_uri": rep.ticket_uri
                                }),
                            ),
                            Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                        }
                    } else {
                        match store_verified_blob(&cache_dir, &raw_bytes, expected_sha.as_deref()) {
                            Ok((sha, path)) => write_json(
                                &mut stream,
                                200,
                                &json!({ "sha256": sha, "path": path, "bytes": raw_bytes.len() }),
                            ),
                            Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                        }
                    }
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err.to_string() })),
            }
        }
        ("POST", "/api/blobs/pull") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let ticket_uri = parsed
                .get("ticket_uri")
                .or_else(|| parsed.get("uri"))
                .or_else(|| parsed.get("ticket"))
                .and_then(Value::as_str)
                .unwrap_or_default()
                .trim()
                .to_string();
            let force = parsed
                .get("force")
                .and_then(Value::as_bool)
                .unwrap_or(false);
            if ticket_uri.is_empty() {
                return write_json(
                    &mut stream,
                    400,
                    &json!({ "error": "ticket_uri is required (iroh-blobs://ticket/...#sha256:...)" }),
                );
            }
            let Some(bridge) = st.blob_bridge.clone() else {
                return write_json(
                    &mut stream,
                    503,
                    &json!({ "error": "live iroh-blobs bridge is not initialized on this node" }),
                );
            };
            let cache_dir = st.blob_cache_dir.clone();
            drop(st);
            match bridge.pull_verified_blob_ticket(&cache_dir, &ticket_uri, force) {
                Ok(report) => {
                    if let Ok(mut st_after) = state.lock() {
                        st_after.blob_fetches.push(report.clone());
                    }
                    write_json(&mut stream, 200, &report)
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
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
        ("POST", "/api/deploy/rollback") => {
            if let Some(slot) = st.deploy_slots.first_mut() {
                match execute_verified_rollback(slot) {
                    Ok(()) => {
                        let updated = slot.clone();
                        write_json(
                            &mut stream,
                            200,
                            &json!({ "status": "rolled_back", "slot": updated }),
                        )
                    }
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

            if let Some(t_uri) = parsed
                .get("ticket_uri")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|s| !s.is_empty())
            {
                if let Some(ref bridge) = st.blob_bridge {
                    match bridge.pull_verified_blob_ticket(&st.blob_cache_dir, t_uri, false) {
                        Ok(rep) => {
                            sha256 = rep.sha256.clone();
                            st.blob_fetches.push(rep);
                        }
                        Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                    }
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
                    &json!({ "error": "sha256, bytes_base64, or ticket_uri is required to configure a blob subprocess" }),
                );
            }

            let (verified_sha, staged_path) =
                match stage_executable_blob(&st.blob_cache_dir, &name, &sha256) {
                    Ok(v) => v,
                    Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                };

            let prev_restart_count = st
                .managed_processes
                .iter()
                .find(|p| p.name == name && p.sha256 == verified_sha)
                .map(|p| p.restart_count)
                .unwrap_or(0);

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
                restart_count: prev_restart_count,
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
        ("GET", "/api/cursor/jobs") => {
            let offset = query
                .get("offset")
                .and_then(|v| v.parse::<usize>().ok())
                .unwrap_or(0);
            let limit = query
                .get("limit")
                .and_then(|v| v.parse::<usize>().ok())
                .unwrap_or(50)
                .clamp(1, 200);
            let slice: Vec<CursorJobRecord> = st
                .cursor_jobs
                .iter()
                .skip(offset)
                .take(limit)
                .cloned()
                .collect();
            write_json(&mut stream, 200, &slice)
        }
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
        "heartbeat_interval_secs": HEARTBEAT_INTERVAL_SECS,
        "summary_interval_secs": SUMMARY_HEARTBEAT_INTERVAL_SECS,
        "self_iroh_runtime": state.iroh_runtime,
        "boot_stages": state.boot_stages,
        "mesh_config": masked_mesh_config(&state.mesh_config),
        "nodes": state.nodes,
        "mesh_rtt_ms": state.mesh_rtt_ms,
        "service_endpoints": state.service_endpoints
    });
    let initial_json = serde_json::to_string(&initial_gossip).unwrap_or_else(|_| "{}".to_string());

    let template = r##"<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>dialtone // __SELF_NODE__ — Dialtone Iroh 1.3 Mesh Control Plane</title>
<script type="importmap">
{
  "imports": {
    "three": "https://cdn.jsdelivr.net/npm/three@0.179.1/build/three.module.js",
    "h3-js": "https://cdn.jsdelivr.net/npm/h3-js@4.4.0/+esm"
  }
}
</script>
<style>
  /* deployrobotic greyscale palette + semantic colors:
     - Greyscale tokens (--ground, --ink, --ink-2, --line, --line-2, --dot)
     - Blue (--accent) for focus edge highlights
     - Yellow (--warn) for warnings
     - Red (--red) for errors */
  :root {
    --ground: #ebebeb;
    --ice: #dedede;
    --paper: #f5f5f5;
    --ink: #1a1a1a;
    --ink-2: #5c5c5c;
    --line: #a6a6a6;
    --line-2: #cccccc;
    --dot: #d9d9d9;
    --accent: #1f5fd6;
    --warn: #ca8a04;
    --red: #d42a20;
    --panel-bg: rgba(245, 245, 245, 0.90);
    --menu-bg: rgba(235, 235, 235, 0.94);
    --sans: "Saira Variable", "Titillium Web", "Segoe UI", Arial, sans-serif;
    --mono: "B612 Mono", "JetBrains Mono", ui-monospace, "SF Mono", Menlo, Consolas, monospace;
    --safe-top: env(safe-area-inset-top, 0px);
    --safe-bottom: env(safe-area-inset-bottom, 0px);
    --safe-left: env(safe-area-inset-left, 0px);
    --safe-right: env(safe-area-inset-right, 0px);
    color-scheme: light;
  }
  :root[data-theme="dark"] {
    color-scheme: dark;
    --ground: #111111;
    --ice: #161616;
    --paper: #1c1c1c;
    --ink: #ececec;
    --ink-2: #9c9c9c;
    --line: #474747;
    --line-2: #2e2e2e;
    --dot: #1f1f1f;
    --accent: #5b93ff;
    --warn: #eab308;
    --red: #ef4136;
    --panel-bg: rgba(17, 17, 17, 0.88);
    --menu-bg: rgba(17, 17, 17, 0.94);
  }

  * { box-sizing: border-box; margin: 0; padding: 0; }
  [hidden] { display: none !important; }

  html, body {
    width: 100%; height: 100%; overflow: hidden;
    overscroll-behavior: none;
    background-color: var(--ground);
    background-image: radial-gradient(var(--dot) 1px, transparent 1.2px);
    background-size: 12px 12px;
    color: var(--ink);
    font-family: var(--mono);
    font-size: 12px;
    line-height: 1.5;
    font-variant-numeric: tabular-nums;
    -webkit-font-smoothing: antialiased;
    user-select: none;
  }

  .app-shell {
    position: fixed; inset: 0;
    width: 100%; height: 100dvh;
    background-color: var(--ground);
    background-image: radial-gradient(var(--dot) 1px, transparent 1.2px);
    background-size: 12px 12px;
  }

  #hud-3d-canvas {
    position: absolute; inset: 0;
    width: 100%; height: 100%;
    display: block;
    z-index: 1;
    cursor: grab;
    touch-action: none;
  }
  #hud-3d-canvas:active { cursor: grabbing; }

  .scene-shade {
    position: absolute; inset: 0;
    z-index: 2;
    pointer-events: none;
    background: radial-gradient(ellipse at 50% 44%, transparent 56%, color-mix(in srgb, var(--ground) 55%, transparent) 100%);
  }

  /* Bottom terminal overlay: greyscale linework with Blue focus edge highlight */
  .terminal-overlay {
    position: fixed;
    left: max(14px, calc(var(--safe-left) + 14px));
    right: max(78px, calc(var(--safe-right) + 78px));
    bottom: max(14px, calc(var(--safe-bottom) + 14px));
    max-width: 920px;
    margin: 0 auto;
    z-index: 10;
    background: var(--panel-bg);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    border: 1px solid var(--line);
    border-left: 3px solid var(--accent);
    border-radius: 3px;
    padding: 10px 14px 11px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
    transition: border-color 0.2s ease, opacity 0.2s ease;
  }
  .terminal-overlay[data-severity="warn"] {
    border-left-color: var(--warn);
  }
  .terminal-overlay[data-severity="error"] {
    border-left-color: var(--red);
  }

  .term-header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px 12px;
    padding-bottom: 6px;
    margin-bottom: 6px;
    border-bottom: 1px solid var(--line-2);
    font-size: 10.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-2);
  }
  .layer-pill {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 1px 7px;
    border: 1px solid var(--accent);
    border-radius: 2px;
    color: var(--ink);
    font-weight: 700;
    background: color-mix(in srgb, var(--accent) 12%, transparent);
  }
  .layer-pill .layer-num {
    color: var(--accent);
    font-weight: 700;
  }
  .tour-badge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--ink-2);
  }
  .tour-badge[data-playing="true"] .dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent);
  }
  .tour-badge[data-playing="false"] .dot {
    width: 6px; height: 6px; border-radius: 1px;
    background: var(--ink-2);
  }
  .status-pill {
    margin-left: auto;
    padding: 1px 7px;
    border: 1px solid var(--line);
    border-radius: 2px;
    font-weight: 700;
    color: var(--ink);
  }
  .status-pill[data-severity="warn"] {
    border-color: var(--warn);
    color: var(--warn);
  }
  .status-pill[data-severity="error"] {
    border-color: var(--red);
    color: var(--red);
  }

  .top-shell-pre {
    font-family: var(--mono);
    font-size: 11.5px;
    line-height: 1.5;
    color: var(--ink);
    white-space: pre-wrap;
    word-break: break-word;
    user-select: text;
  }
  .top-shell-pre .hi { color: var(--ink); font-weight: 700; }
  .top-shell-pre .focus { color: var(--accent); font-weight: 700; }
  .top-shell-pre .dim { color: var(--ink-2); }
  .top-shell-pre .warn { color: var(--warn); font-weight: 700; }
  .top-shell-pre .err { color: var(--red); font-weight: 700; }
  .ascii-bar { color: var(--line); letter-spacing: -0.03em; }
  .ascii-bar .fill { color: var(--ink); }
  .ascii-bar .fill.warn { color: var(--warn); }
  .ascii-bar .fill.err { color: var(--red); }

  /* Single "thumb" button on the bottom right — edge highlighted in Blue */
  .thumb-btn {
    position: fixed;
    right: max(14px, calc(var(--safe-right) + 14px));
    bottom: max(14px, calc(var(--safe-bottom) + 14px));
    z-index: 40;
    width: 52px;
    height: 52px;
    display: grid;
    place-items: center;
    background: var(--panel-bg);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    color: var(--ink);
    border: 1.5px solid var(--accent);
    border-radius: 4px;
    cursor: pointer;
    box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 28%, transparent), 0 8px 24px rgba(0, 0, 0, 0.45);
    transition: transform 0.14s ease, border-color 0.18s ease, background 0.18s ease;
    -webkit-tap-highlight-color: transparent;
  }
  .thumb-btn:hover {
    background: color-mix(in srgb, var(--accent) 16%, var(--panel-bg));
  }
  .thumb-btn:active {
    transform: scale(0.95);
  }
  .thumb-btn:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 3px;
  }
  .thumb-btn svg {
    width: 22px; height: 22px;
    stroke: currentColor;
    stroke-width: 1.7;
    fill: none;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .thumb-btn[aria-expanded="true"] .icon-sliders { display: none; }
  .thumb-btn[aria-expanded="false"] .icon-close { display: none; }

  /* Full-screen menu opened by the bottom-right thumb button */
  .fullscreen-menu {
    position: fixed;
    inset: 0;
    z-index: 30;
    background-color: var(--menu-bg);
    background-image: radial-gradient(var(--dot) 1px, transparent 1.2px);
    background-size: 12px 12px;
    backdrop-filter: blur(10px);
    -webkit-backdrop-filter: blur(10px);
    overflow-y: auto;
    padding: max(20px, calc(var(--safe-top) + 16px)) max(20px, calc(var(--safe-right) + 20px)) max(88px, calc(var(--safe-bottom) + 84px)) max(20px, calc(var(--safe-left) + 20px));
  }
  .menu-container {
    max-width: 860px;
    margin: 0 auto;
    display: grid;
    gap: 18px;
  }
  .menu-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 8px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--line);
  }
  .menu-title {
    font-family: var(--sans);
    font-size: 18px;
    font-weight: 600;
    letter-spacing: -0.02em;
    color: var(--ink);
  }
  .menu-subtitle {
    font-size: 11px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--ink-2);
  }

  .menu-section {
    border: 1px solid var(--line);
    border-radius: 3px;
    background: color-mix(in srgb, var(--ground) 84%, transparent);
    padding: 12px 14px;
    display: grid;
    gap: 11px;
  }
  .menu-section.is-focus-section {
    border-color: var(--accent);
    box-shadow: inset 3px 0 0 var(--accent);
  }
  .menu-section-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 10.5px;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-2);
  }
  .menu-section-title span.hi {
    color: var(--ink);
  }

  .btn-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .ctrl-btn {
    min-height: 40px;
    padding: 6px 12px;
    border: 1px solid var(--line);
    border-radius: 3px;
    background: color-mix(in srgb, var(--ground) 88%, transparent);
    color: var(--ink);
    font-family: var(--mono);
    font-size: 11.5px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    cursor: pointer;
    display: inline-flex;
    align-items: center;
    gap: 7px;
    transition: border-color 0.15s ease, background 0.15s ease;
  }
  .ctrl-btn:hover {
    border-color: var(--ink);
  }
  /* Edge highlight in blue for the item the user should focus on / active state */
  .ctrl-btn[aria-pressed="true"],
  .ctrl-btn.is-focus {
    border-color: var(--accent);
    box-shadow: inset 0 0 0 1px var(--accent);
    background: color-mix(in srgb, var(--accent) 13%, transparent);
    color: var(--ink);
  }
  .ctrl-btn[data-severity="warn"] {
    border-color: var(--warn);
  }
  .ctrl-btn[data-severity="error"] {
    border-color: var(--red);
  }
  .node-dot {
    width: 7px; height: 7px; border-radius: 50%;
    background: var(--ink-2);
  }
  .node-dot.focus { background: var(--accent); }
  .node-dot.warn { background: var(--warn); }
  .node-dot.error { background: var(--red); }

  .slider-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 12px 18px;
  }
  .slider-row {
    display: grid;
    gap: 5px;
  }
  .slider-label {
    display: flex;
    justify-content: space-between;
    font-size: 11px;
    color: var(--ink-2);
    letter-spacing: 0.04em;
  }
  .slider-label strong {
    color: var(--ink);
    font-weight: 700;
  }
  input[type="range"] {
    -webkit-appearance: none;
    appearance: none;
    width: 100%;
    height: 6px;
    background: var(--line-2);
    border: 1px solid var(--line);
    border-radius: 2px;
    outline: none;
    cursor: pointer;
  }
  input[type="range"]::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 2px;
    background: var(--ink);
    border: 2px solid var(--accent);
    cursor: pointer;
  }
  input[type="range"]:focus-visible {
    border-color: var(--accent);
  }

  .mesh-form-row {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
  }
  .mesh-input {
    flex: 1 1 240px;
    min-height: 40px;
    padding: 6px 10px;
    border: 1px solid var(--line);
    border-radius: 3px;
    background: var(--ground);
    color: var(--ink);
    font-family: var(--mono);
    font-size: 11.5px;
  }
  .mesh-input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .mesh-output {
    padding: 8px 10px;
    border: 1px solid var(--line);
    border-left: 2px solid var(--accent);
    background: var(--ground);
    color: var(--ink);
    font-family: var(--mono);
    font-size: 11px;
    white-space: pre-wrap;
    word-break: break-all;
    user-select: all;
  }

  @media (max-width: 768px) {
    .terminal-overlay {
      left: 10px;
      right: 72px;
      bottom: max(10px, var(--safe-bottom));
      padding: 8px 10px;
    }
    .top-shell-pre {
      font-size: 10px;
      line-height: 1.42;
    }
    .term-header {
      font-size: 9.5px;
      gap: 6px 8px;
    }
  }
</style>
</head>
<body>
<div class="app-shell">
  <canvas id="hud-3d-canvas"></canvas>
  <div class="scene-shade"></div>

  <!-- Bottom terminal overlay showing live 'top' telemetry + mesh connection instructions -->
  <section id="terminal-overlay" class="terminal-overlay" data-severity="nominal" aria-live="polite">
    <div class="term-header">
      <span class="layer-pill" id="hud-layer-pill" title="Current deployrobotic zoom layer">
        <span>LAYER</span>
        <span class="layer-num" id="hud-layer-num">3</span>
        <span id="hud-layer-name">SITE</span>
      </span>
      <span class="tour-badge" id="hud-tour-badge" data-playing="true">
        <i class="dot"></i>
        <span id="hud-tour-text">TOUR LOOP</span>
      </span>
      <span id="hud-stop-label" style="color:var(--ink);font-weight:700">--</span>
      <span class="status-pill" id="hud-status-pill" data-severity="nominal">NOMINAL</span>
    </div>
    <pre id="node-top-hud" class="top-shell-pre">initializing h3 globe &amp; micro-factory telemetry...</pre>
  </section>

  <!-- Single bottom-right thumb button opening the full-screen menu -->
  <button id="thumb-menu-btn" class="thumb-btn" type="button" aria-expanded="false" aria-controls="fullscreen-menu" aria-label="Open 3D Globe &amp; Mesh Controls Menu" title="Open 3D Globe &amp; Mesh Controls">
    <svg class="icon-sliders" viewBox="0 0 24 24" aria-hidden="true">
      <line x1="4" y1="6" x2="20" y2="6"></line>
      <line x1="4" y1="12" x2="20" y2="12"></line>
      <line x1="4" y1="18" x2="20" y2="18"></line>
      <circle cx="9" cy="6" r="2.2" fill="var(--ground)"></circle>
      <circle cx="15" cy="12" r="2.2" fill="var(--ground)"></circle>
      <circle cx="11" cy="18" r="2.2" fill="var(--ground)"></circle>
    </svg>
    <svg class="icon-close" viewBox="0 0 24 24" aria-hidden="true">
      <line x1="6" y1="6" x2="18" y2="18"></line>
      <line x1="18" y1="6" x2="6" y2="18"></line>
    </svg>
  </button>

  <!-- Full-screen menu of buttons and sliders for controlling items in the 3D globe display -->
  <aside id="fullscreen-menu" class="fullscreen-menu" hidden>
    <div class="menu-container">
      <header class="menu-header">
        <div>
          <h1 class="menu-title">DIALTONE 3D GLOBE &amp; LAYER CONTROLS</h1>
          <p class="menu-subtitle">GREYSCALE LINE-ART // BLUE FOCUS // YELLOW WARN // RED ERROR</p>
        </div>
        <button id="menu-theme-btn" class="ctrl-btn" type="button">THEME: DARK</button>
      </header>

      <!-- 1. Tour Playback & Endless Loop Controls -->
      <section class="menu-section is-focus-section">
        <div class="menu-section-title">
          <span>01 // ENDLESS TOUR &amp; CAMERA PLAYBACK</span>
          <span class="hi" id="menu-tour-status">PLAYING LOOP</span>
        </div>
        <div class="btn-row">
          <button id="tour-playback" class="ctrl-btn is-focus" type="button" aria-pressed="true">▶ TOUR LOOP: ON</button>
          <button id="btn-prev-stop" class="ctrl-btn" type="button">◀ PREV STOP</button>
          <button id="btn-next-stop" class="ctrl-btn" type="button">NEXT STOP ▶</button>
          <button id="btn-reset-cam" class="ctrl-btn" type="button">RESET ORBIT</button>
        </div>
      </section>

      <!-- 2. deployrobotic Zoom Layer Selector (Layers 1..5) -->
      <section class="menu-section">
        <div class="menu-section-title">
          <span>02 // DEPLOYROBOTIC ZOOM LAYER STACK (1–5)</span>
          <span class="hi" id="menu-layer-readout">AUTO (TOUR)</span>
        </div>
        <div class="btn-row" id="layer-btn-row">
          <button class="ctrl-btn" type="button" data-layer-mode="auto" aria-pressed="true">AUTO CYCLE</button>
          <button class="ctrl-btn" type="button" data-layer-mode="1" aria-pressed="false">1 GLOBE</button>
          <button class="ctrl-btn" type="button" data-layer-mode="2" aria-pressed="false">2 REGION</button>
          <button class="ctrl-btn" type="button" data-layer-mode="3" aria-pressed="false">3 SITE FACTORY</button>
          <button class="ctrl-btn" type="button" data-layer-mode="4" aria-pressed="false">4 CONTAINERS</button>
          <button class="ctrl-btn" type="button" data-layer-mode="5" aria-pressed="false">5 PROCESSES</button>
        </div>
      </section>

      <!-- 3. Fleet Node Focus Selector -->
      <section class="menu-section">
        <div class="menu-section-title">
          <span>03 // FLEET NODES (CLICK TO FOCUS MICRO-FACTORY)</span>
          <span class="hi" id="menu-node-count">0 NODES</span>
        </div>
        <div class="btn-row" id="menu-node-buttons"></div>
      </section>

      <!-- 4. Sliders for Controlling 3D Globe & Factory Elements -->
      <section class="menu-section">
        <div class="menu-section-title">
          <span>04 // 3D GLOBE &amp; FACTORY PARAMETERS</span>
          <span class="hi">LIVE SLIDERS</span>
        </div>
        <div class="slider-grid">
          <label class="slider-row">
            <span class="slider-label"><span>CAMERA ZOOM LAYER</span><strong id="val-zoom-layer">3.0</strong></span>
            <input id="slider-zoom-layer" type="range" min="1.0" max="5.0" step="0.1" value="3.0">
          </label>
          <label class="slider-row">
            <span class="slider-label"><span>TOUR STEP DURATION</span><strong id="val-tour-dwell">4.5s</strong></span>
            <input id="slider-tour-dwell" type="range" min="2.0" max="12.0" step="0.5" value="4.5">
          </label>
          <label class="slider-row">
            <span class="slider-label"><span>3D FACTORY ICON SCALE</span><strong id="val-factory-scale">1.0x</strong></span>
            <input id="slider-factory-scale" type="range" min="0.4" max="2.4" step="0.1" value="1.0">
          </label>
          <label class="slider-row">
            <span class="slider-label"><span>H3 LAND CELL INK</span><strong id="val-land-ink">100%</strong></span>
            <input id="slider-land-ink" type="range" min="0" max="100" step="5" value="100">
          </label>
          <label class="slider-row">
            <span class="slider-label"><span>CLOUD &amp; ATMOSPHERE</span><strong id="val-cloud-opacity">50%</strong></span>
            <input id="slider-cloud-opacity" type="range" min="0" max="100" step="5" value="50">
          </label>
          <label class="slider-row">
            <span class="slider-label"><span>QUIC PACKET SPEED</span><strong id="val-packet-speed">1.0x</strong></span>
            <input id="slider-packet-speed" type="range" min="0.0" max="2.5" step="0.1" value="1.0">
          </label>
          <label class="slider-row">
            <span class="slider-label"><span>TERMINAL OVERLAY OPACITY</span><strong id="val-term-opacity">92%</strong></span>
            <input id="slider-term-opacity" type="range" min="15" max="100" step="5" value="92">
          </label>
        </div>
        <div class="btn-row" style="margin-top:4px">
          <button id="toggle-h3-land" class="ctrl-btn" type="button" aria-pressed="true">H3 LAND: ON</button>
          <button id="toggle-quic-arcs" class="ctrl-btn" type="button" aria-pressed="true">QUIC ARCS: ON</button>
          <button id="toggle-clouds" class="ctrl-btn" type="button" aria-pressed="true">CLOUDS: ON</button>
          <button id="toggle-labels" class="ctrl-btn" type="button" aria-pressed="true">FOCUS LABEL: ON</button>
          <button id="toggle-drones" class="ctrl-btn" type="button" aria-pressed="true">FACTORY ACTIVITY: ON</button>
        </div>
      </section>

      <!-- 5. Mesh Connection & Onboarding (Primary Entry Point) -->
      <section class="menu-section" id="menu-mesh-section">
        <div class="menu-section-title">
          <span>05 // MESH CONNECTION &amp; ONBOARDING</span>
          <span class="hi" id="menu-mesh-state-label">CHECKING...</span>
        </div>
        <div class="btn-row">
          <button id="btn-init-mesh" class="ctrl-btn is-focus" type="button">INIT NEW PRIVATE MESH</button>
          <button id="btn-invite-mesh" class="ctrl-btn" type="button">GENERATE INVITE TICKET</button>
        </div>
        <div class="mesh-form-row">
          <input id="input-join-ticket" class="mesh-input" type="text" placeholder="Paste iroh-gossip://invite/... ticket URI to join mesh">
          <button id="btn-join-mesh" class="ctrl-btn is-focus" type="button">JOIN MESH</button>
        </div>
        <pre id="menu-mesh-output" class="mesh-output">CLI Quickstart:
  1. Create mesh:  dialtone channel init
  2. Invite peer:  dialtone channel invite --ttl-secs 900
  3. Join mesh:    dialtone channel join "&lt;ticket&gt;"</pre>
      </section>
    </div>
  </aside>
</div>

<script type="module">
import * as THREE from "three";
import { cellToBoundary, latLngToCell, gridDisk } from "h3-js";

const INITIAL_STATE = __INITIAL_JSON__;
const API_TOKEN = "__API_TOKEN__";
const LAND_H3_BITSET_B64 = "__LAND_H3_BITSET_B64__";

// ---------------------------------------------------------------------------
// 0. deployrobotic Zoom Layer Stack (Layers 1..5) & State
// ---------------------------------------------------------------------------
const ZOOM_LAYERS = [
  { layer: 1, id: "globe",     name: "GLOBE",     camDist: 8.6, desc: "Planetary H3 mesh & great-circle QUIC topology" },
  { layer: 2, id: "region",    name: "REGION",    camDist: 6.1, desc: "Regional H3 resolution-3 cells & RTT latency cluster" },
  { layer: 3, id: "site",      name: "SITE",      camDist: 4.5, desc: "Node micro-factory site & public-IP / RTT geolocation" },
  { layer: 4, id: "container", name: "CONTAINER", camDist: 3.65, desc: "Compute, memory & disk container bays + 30s variability" },
  { layer: 5, id: "process",   name: "PROCESS",   camDist: 3.25, desc: "Supervised jobs, heartbeat spire & QUIC transport sockets" }
];
const PREVIOUS_LAYER_OPACITY = 0.25;

function clamp01(v) {
  return Math.min(1, Math.max(0, v));
}

function zoomLayerOpacity(layer, cameraLayer, part = "outline") {
  const above = cameraLayer - layer;
  if (above < 0) return clamp01(1 + above);
  if (part === "detail") return clamp01(1 - above * 0.45);
  if (above <= 1) return 1 + (PREVIOUS_LAYER_OPACITY - 1) * above;
  return PREVIOUS_LAYER_OPACITY * clamp01(2.6 - above * 0.5);
}

let meshState = INITIAL_STATE;
let selectedNode = INITIAL_STATE.selected_peer || INITIAL_STATE.self_node || "node-alpha";
let userDragOffset = new THREE.Quaternion();
let dragging = false;
let pointerDownPos = { x: 0, y: 0 };
let lastMouse = { x: 0, y: 0 };

// Tour & Layer state
let tourPlaying = true;
let layerMode = "auto"; // "auto" or 1..5
let currentZoomLayer = 3;
let continuousCameraLayer = 3.0;
let targetCameraDist = 4.5;
let currentCameraDist = 6.8;
let tourStopIndex = 0;
let tourStops = [];

// Configurable slider parameters
let cfgTourDwellMs = 4500;
let cfgTourFlightMs = 2000;
let cfgFactoryScale = 1.0;
let cfgLandInk = 1.0;
let cfgCloudOpacity = 0.5;
let cfgPacketSpeed = 1.0;
let cfgShowLand = true;
let cfgShowArcs = true;
let cfgShowClouds = true;
let cfgShowLabels = true;
let cfgShowActivity = true;

const WARN_CPU_PCT = 75.0;
const ERROR_CPU_PCT = 92.0;
const WARN_MEM_PCT = 78.0;
const ERROR_MEM_PCT = 92.0;
const WARN_DISK_PCT = 82.0;
const ERROR_DISK_PCT = 94.0;
const GLOBE_RADIUS = 2.8;

// ---------------------------------------------------------------------------
// Theme Tokens (deployrobotic greyscale + blue focus + yellow warn + red error)
// ---------------------------------------------------------------------------
function readThemePalette() {
  const styles = getComputedStyle(document.documentElement);
  const tok = (name, fb) => styles.getPropertyValue(name).trim() || fb;
  return {
    groundHex: tok("--ground", "#111111"),
    inkHex: tok("--ink", "#ececec"),
    ink2Hex: tok("--ink-2", "#9c9c9c"),
    lineHex: tok("--line", "#474747"),
    line2Hex: tok("--line-2", "#2e2e2e"),
    accentHex: tok("--accent", "#5b93ff"),
    warnHex: tok("--warn", "#eab308"),
    redHex: tok("--red", "#ef4136"),
    ground: new THREE.Color(tok("--ground", "#111111")),
    ink: new THREE.Color(tok("--ink", "#ececec")),
    ink2: new THREE.Color(tok("--ink-2", "#9c9c9c")),
    line: new THREE.Color(tok("--line", "#474747")),
    line2: new THREE.Color(tok("--line-2", "#2e2e2e")),
    accent: new THREE.Color(tok("--accent", "#5b93ff")),
    warn: new THREE.Color(tok("--warn", "#eab308")),
    red: new THREE.Color(tok("--red", "#ef4136"))
  };
}
let palette = readThemePalette();

// ---------------------------------------------------------------------------
// 1. Decode embedded 11,216 H3 resolution-3 land cells (from deployrobotic)
// ---------------------------------------------------------------------------
function decodeLandH3Cells(b64) {
  const bin = atob(b64);
  const cells = [];
  for (let byteIdx = 0; byteIdx < bin.length; byteIdx++) {
    const byte = bin.charCodeAt(byteIdx);
    if (byte === 0) continue;
    for (let bit = 0; bit < 8; bit++) {
      if ((byte & (1 << bit)) !== 0) {
        const idx = (byteIdx << 3) | bit;
        cells.push("83" + idx.toString(16).padStart(4, "0") + "fffffffff");
      }
    }
  }
  return cells;
}

const LAND_H3_CELLS = decodeLandH3Cells(LAND_H3_BITSET_B64);

// ---------------------------------------------------------------------------
// 2. Public IP Geolocation + RTT Latency Clustering (Server + Client Fallback)
// ---------------------------------------------------------------------------
function isGloballyRoutableIpv4(ipStr) {
  const parts = (ipStr || "").split(".").map(Number);
  if (parts.length !== 4 || parts.some(x => Number.isNaN(x) || x < 0 || x > 255)) return false;
  const [a, b, c] = parts;
  if (a === 0 || a === 10 || a === 127) return false;
  if (a === 169 && b === 254) return false;
  if (a === 172 && b >= 16 && b <= 31) return false;
  if (a === 192 && b === 168) return false;
  if (a === 100 && b >= 64 && b <= 127) return false;
  if (a === 198 && (b === 18 || b === 19)) return false;
  if (a === 192 && b === 0 && (c === 0 || c === 2)) return false;
  if (a === 198 && b === 51 && c === 100) return false;
  if (a === 203 && b === 0 && c === 113) return false;
  if (a >= 224) return false;
  return true;
}

function approxGeoFromPublicIpv4(ipStr) {
  const parts = ipStr.split(".").map(Number);
  const [a, b, c] = parts;
  let baseLat = 37.75, baseLng = -122.42, region = "US-West";
  if ([3, 4, 8, 12, 13, 15, 16, 17, 18, 20, 23, 24, 32, 34, 35, 38, 40, 44, 45, 47, 50, 52, 54].includes(a)) {
    baseLat = 37.62; baseLng = -122.38; region = "US-West";
  } else if ((a >= 63 && a <= 76) || (a >= 96 && a <= 100) || (a >= 104 && a <= 108) || (a >= 128 && a <= 174) || (a >= 184 && a <= 192) || (a >= 198 && a <= 209) || a === 216) {
    if (b < 128) { baseLat = 37.38; baseLng = -122.05; region = "US-Pacific"; }
    else { baseLat = 39.04; baseLng = -77.49; region = "US-East"; }
  } else if (a === 2 || a === 5 || a === 31 || a === 37 || a === 46 || (a >= 77 && a <= 95) || (a >= 109 && a <= 109) || (a >= 176 && a <= 185) || (a >= 193 && a <= 195) || (a >= 212 && a <= 213) || a === 217) {
    baseLat = 50.11; baseLng = 8.68; region = "EU-Central";
  } else if (a === 1 || a === 14 || a === 27 || a === 36 || a === 39 || a === 42 || a === 43 || a === 49 || (a >= 58 && a <= 61) || (a >= 101 && a <= 103) || (a >= 110 && a <= 126) || (a >= 175 && a <= 175) || (a >= 180 && a <= 183) || (a >= 202 && a <= 203) || (a >= 210 && a <= 211) || (a >= 218 && a <= 223)) {
    baseLat = 35.68; baseLng = 139.76; region = "APAC-East";
  }
  const latOff = ((b / 255) - 0.5) * 4.2;
  const lngOff = ((c / 255) - 0.5) * 6.4;
  return {
    lat: Math.max(-70, Math.min(72, baseLat + latOff)),
    lng: ((((baseLng + lngOff) + 180) % 360 + 360) % 360) - 180,
    label: `${region} (${a}.${b}.x.x)`,
    source: "public_ip_geo",
    public_ip: `${a}.${b}.x.x`,
    uncertainty_km: 50
  };
}

function extractPublicIpv4FromNode(n) {
  const candidates = [
    ...(n.observed_remote_addrs || []),
    ...((n.iroh_transport && n.iroh_transport.advertised_ip_addrs) || []),
    (n.iroh_transport && n.iroh_transport.bound_non_tailnet_ipv4) || ""
  ];
  for (const raw of candidates) {
    const m = String(raw).match(/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/);
    if (m && isGloballyRoutableIpv4(m[1])) {
      return m[1];
    }
  }
  return null;
}

function relayAnchorGeo(n) {
  const urls = [
    ...((n.iroh_transport && n.iroh_transport.advertised_relay_urls) || []),
    ...((n.iroh_transport && n.iroh_transport.relay_urls) || [])
  ].map(u => String(u).toLowerCase());
  for (const r of urls) {
    if (r.includes("usw") || r.includes("west") || r.includes("sjc") || r.includes("lax")) {
      return { lat: 37.62, lng: -122.38, region: "US-West (usw1)" };
    }
    if (r.includes("use") || r.includes("east") || r.includes("iad") || r.includes("nyc")) {
      return { lat: 39.04, lng: -77.49, region: "US-East (use1)" };
    }
    if (r.includes("euw") || r.includes("euc") || r.includes("fra") || r.includes("ams")) {
      return { lat: 50.11, lng: 8.68, region: "EU-Central" };
    }
    if (r.includes("aps") || r.includes("apn") || r.includes("tok") || r.includes("sin")) {
      return { lat: 35.68, lng: 139.76, region: "APAC" };
    }
  }
  return { lat: 37.62, lng: -122.38, region: "US-West (usw1)" };
}

function fnvBearingRad(a, b) {
  let h = 0x811c9dc5;
  const s = `${a}->${b}`;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return ((h & 0xffff) / 65535.0) * Math.PI * 2;
}

function lookupPairRtt(a, b, nodes, rttMatrix) {
  const k1 = `${a}<->${b}`, k2 = `${b}<->${a}`;
  if (typeof rttMatrix[k1] === "number" && rttMatrix[k1] > 0) return rttMatrix[k1];
  if (typeof rttMatrix[k2] === "number" && rttMatrix[k2] > 0) return rttMatrix[k2];
  const na = nodes.find(x => x.node === a);
  if (na && na.peer_rtt_ms && typeof na.peer_rtt_ms[b] === "number" && na.peer_rtt_ms[b] > 0) return na.peer_rtt_ms[b];
  const nb = nodes.find(x => x.node === b);
  if (nb && nb.peer_rtt_ms && typeof nb.peer_rtt_ms[a] === "number" && nb.peer_rtt_ms[a] > 0) return nb.peer_rtt_ms[a];
  return null;
}

function resolveNodeLocationsJS(nodes, rttMatrix = {}) {
  const out = new Map();
  for (const n of nodes) {
    if (n.geo_location && n.geo_location.source === "public_ip_geo" && Number.isFinite(n.geo_location.lat) && Number.isFinite(n.geo_location.lng)) {
      out.set(n.node, n.geo_location);
      continue;
    }
    const pubIp = extractPublicIpv4FromNode(n);
    if (pubIp) {
      out.set(n.node, approxGeoFromPublicIpv4(pubIp));
    }
  }

  const anchors = nodes.filter(n => out.has(n.node));
  const unplaced = nodes.filter(n => !out.has(n.node));

  if (anchors.length > 0) {
    for (const n of unplaced) {
      let bestAnchor = anchors[0];
      let bestRtt = 18.0;
      let wLat = 0, wLng = 0, wSum = 0;
      for (const anc of anchors) {
        const g = out.get(anc.node);
        const rtt = lookupPairRtt(n.node, anc.node, nodes, rttMatrix) ?? 20.0;
        const w = 1.0 / Math.max(1.0, rtt);
        wLat += g.lat * w;
        wLng += g.lng * w;
        wSum += w;
        if (rtt < bestRtt || bestAnchor.node === anchors[0].node) {
          bestRtt = rtt;
          bestAnchor = anc;
        }
      }
      const baseLat = wSum > 0 ? wLat / wSum : out.get(bestAnchor.node).lat;
      const baseLng = wSum > 0 ? wLng / wSum : out.get(bestAnchor.node).lng;
      const distDeg = Math.max(3.6, Math.min(28.0, 3.4 + bestRtt * 0.22));
      const bearing = fnvBearingRad(bestAnchor.node, n.node);
      const latRad = (baseLat * Math.PI) / 180;
      const lat = Math.max(-72, Math.min(72, baseLat + distDeg * Math.cos(bearing)));
      const lng = ((((baseLng + (distDeg * Math.sin(bearing)) / Math.max(0.25, Math.cos(latRad))) + 180) % 360 + 360) % 360) - 180;
      out.set(n.node, {
        lat,
        lng,
        label: `RTT Cluster ~${bestRtt.toFixed(1)}ms from ${bestAnchor.node}`,
        source: "latency_cluster",
        anchor_node: bestAnchor.node,
        anchor_rtt_ms: Number(bestRtt.toFixed(1)),
        uncertainty_km: Math.round(Math.max(25, Math.min(1800, bestRtt * 65)))
      });
    }
  } else {
    const sorted = [...unplaced].sort((a, b) => a.node.localeCompare(b.node));
    const base = sorted[0] ? relayAnchorGeo(sorted[0]) : { lat: 37.62, lng: -122.38, region: "US-West (usw1)" };
    sorted.forEach((n, idx) => {
      if (idx === 0) {
        out.set(n.node, {
          lat: base.lat,
          lng: base.lng,
          label: `Latency Cluster Seed (${base.region})`,
          source: "latency_cluster",
          anchor_node: n.node,
          anchor_rtt_ms: 0.0,
          uncertainty_km: 80
        });
      } else {
        const seedName = sorted[0].node;
        const rtt = lookupPairRtt(n.node, seedName, nodes, rttMatrix) ?? 16.0;
        const distDeg = Math.max(4.2, Math.min(26.0, 3.8 + rtt * 0.24));
        const bearing = fnvBearingRad(seedName, n.node) + idx * 2.15;
        const latRad = (base.lat * Math.PI) / 180;
        const lat = Math.max(-72, Math.min(72, base.lat + distDeg * Math.cos(bearing)));
        const lng = ((((base.lng + (distDeg * Math.sin(bearing)) / Math.max(0.25, Math.cos(latRad))) + 180) % 360 + 360) % 360) - 180;
        out.set(n.node, {
          lat,
          lng,
          label: `RTT Cluster ~${rtt.toFixed(1)}ms (${base.region})`,
          source: "latency_cluster",
          anchor_node: seedName,
          anchor_rtt_ms: Number(rtt.toFixed(1)),
          uncertainty_km: Math.round(Math.max(30, Math.min(1800, rtt * 65)))
        });
      }
    });
  }

  // Spread co-located nodes slightly so neighboring micro-factories don't overlap on the globe
  const entries = [...out.entries()];
  for (let i = 0; i < entries.length; i++) {
    for (let j = i + 1; j < entries.length; j++) {
      const [na, ga] = entries[i];
      const [nb, gb] = entries[j];
      const dLat = gb.lat - ga.lat;
      const dLng = gb.lng - ga.lng;
      const dist = Math.hypot(dLat, dLng);
      if (dist < 2.2) {
        const angle = fnvBearingRad(na, nb);
        const push = (2.2 - dist) * 0.55;
        gb.lat = Math.max(-72, Math.min(72, gb.lat + Math.cos(angle) * push));
        gb.lng = ((((gb.lng + Math.sin(angle) * push) + 180) % 360 + 360) % 360) - 180;
      }
    }
  }

  return out;
}

// ---------------------------------------------------------------------------
// 3. Node Severity Classification (Nominal = Greyscale, Warn = Yellow, Error = Red)
// ---------------------------------------------------------------------------
function classifyNodeHealth(n) {
  const hr = n.host_resources || {};
  const ss = n.summary_stats || null;
  const sup = n.supervised_processes || [];
  const errors = [];
  const warnings = [];

  if (ss && ss.thrashing_detected) errors.push("THRASHING");
  if ((hr.cpu_percent || 0) >= ERROR_CPU_PCT) errors.push("CPU CRIT");
  else if ((hr.cpu_percent || 0) >= WARN_CPU_PCT) warnings.push("HIGH CPU");

  if ((hr.mem_percent || 0) >= ERROR_MEM_PCT) errors.push("MEM CRIT");
  else if ((hr.mem_percent || 0) >= WARN_MEM_PCT) warnings.push("HIGH MEM");

  if ((hr.disk_percent || 0) >= ERROR_DISK_PCT) errors.push("DISK CRIT");
  else if ((hr.disk_percent || 0) >= WARN_DISK_PCT) warnings.push("HIGH DISK");

  for (const job of sup) {
    if (String(job.status || "").toLowerCase().includes("fail") || String(job.status || "").toLowerCase().includes("crash")) {
      errors.push(`JOB ${job.name} ERR`);
    } else if ((job.restart_count || 0) > 0) {
      warnings.push(`JOB ${job.name} RESTART(${job.restart_count})`);
    }
  }
  if (n.tailnet_used) warnings.push("VPN PATH");

  const severity = errors.length > 0 ? "error" : (warnings.length > 0 ? "warn" : "nominal");
  return { severity, errors, warnings };
}

// ---------------------------------------------------------------------------
// 4. H3 Three.js Globe & deployrobotic Hidden-Line Materials
// ---------------------------------------------------------------------------
function latLngToSurfaceVector(lat, lng, radius = GLOBE_RADIUS) {
  const phi = (lat * Math.PI) / 180;
  const theta = (-lng * Math.PI) / 180;
  return new THREE.Vector3(
    Math.cos(phi) * Math.cos(theta),
    Math.sin(phi),
    Math.cos(phi) * Math.sin(theta)
  ).multiplyScalar(radius);
}

function createOutlineMaterial(color, outlinePx = 1.6) {
  return new THREE.ShaderMaterial({
    side: THREE.BackSide,
    transparent: true,
    uniforms: {
      uColor: { value: new THREE.Color(color) },
      uOutlinePx: { value: outlinePx },
      uOpacity: { value: 1.0 },
      uResolution: { value: new THREE.Vector2(window.innerWidth, window.innerHeight) }
    },
    vertexShader: `
      uniform float uOutlinePx;
      uniform vec2 uResolution;
      void main() {
        vec4 clipPosition = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        vec3 viewNormal = normalize(normalMatrix * normal);
        vec2 clipNormal = normalize((projectionMatrix * vec4(viewNormal, 0.0)).xy);
        vec2 offset = (clipNormal * uOutlinePx * 2.0) / max(uResolution, vec2(1.0));
        clipPosition.xy += offset * clipPosition.w;
        gl_Position = clipPosition;
      }
    `,
    fragmentShader: `
      uniform vec3 uColor;
      uniform float uOpacity;
      void main() {
        gl_FragColor = vec4(uColor, uOpacity);
      }
    `
  });
}

// Depth-only occluder so the page's dotted grid ground shows through the globe
function createOccluderMaterial() {
  return new THREE.MeshBasicMaterial({
    colorWrite: false,
    polygonOffset: true,
    polygonOffsetFactor: 1,
    polygonOffsetUnits: 1
  });
}

const cloudVertexShader = `
  varying vec3 vObjectNormal;
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  void main() {
    vObjectNormal = normalize(position);
    vec4 worldPosition = modelMatrix * vec4(position, 1.0);
    vWorldPosition = worldPosition.xyz;
    vWorldNormal = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * viewMatrix * worldPosition;
  }
`;

const cloudFragmentShader = `
  uniform float uTime;
  uniform float uOpacity;
  varying vec3 vObjectNormal;
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  float hash(vec3 p) {
    p = fract(p * 0.3183099 + vec3(0.1, 0.2, 0.3));
    p *= 17.0;
    return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
  }
  float noise(vec3 x) {
    vec3 i = floor(x);
    vec3 f = fract(x);
    f = f * f * (3.0 - 2.0 * f);
    return mix(
      mix(mix(hash(i + vec3(0,0,0)), hash(i + vec3(1,0,0)), f.x),
          mix(hash(i + vec3(0,1,0)), hash(i + vec3(1,1,0)), f.x), f.y),
      mix(mix(hash(i + vec3(0,0,1)), hash(i + vec3(1,0,1)), f.x),
          mix(hash(i + vec3(0,1,1)), hash(i + vec3(1,1,1)), f.x), f.y),
      f.z
    );
  }
  float fbm(vec3 p) {
    float value = 0.0;
    float amp = 0.5;
    for (int i = 0; i < 4; i++) {
      value += amp * noise(p);
      p = p * 2.03 + vec3(0.17, -0.11, 0.23);
      amp *= 0.5;
    }
    return value;
  }
  void main() {
    vec3 p = normalize(vObjectNormal);
    vec3 drift = vec3(uTime * 0.014, -uTime * 0.005, uTime * 0.010);
    float base = fbm(p * 3.4 + drift);
    float detail = fbm(p * 6.8 - drift * 1.4);
    float bands = sin((p.y * 8.5) + base * 2.6 + uTime * 0.04) * 0.5 + 0.5;
    float cloud = smoothstep(0.56, 0.79, base * 0.65 + detail * 0.35) * smoothstep(0.24, 0.86, bands);
    vec3 viewDir = normalize(cameraPosition - vWorldPosition);
    float limb = 1.0 - max(dot(normalize(vWorldNormal), viewDir), 0.0);
    float rimBoost = mix(0.85, 1.25, pow(limb, 1.5));
    float alpha = cloud * uOpacity * rimBoost;
    if (alpha < 0.015) discard;
    gl_FragColor = vec4(vec3(0.68), alpha);
  }
`;

const atmosphereVertexShader = `
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  void main() {
    vec4 worldPosition = modelMatrix * vec4(position, 1.0);
    vWorldPosition = worldPosition.xyz;
    vWorldNormal = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * viewMatrix * worldPosition;
  }
`;

const atmosphereFragmentShader = `
  uniform vec3 uColor;
  uniform float uStrength;
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  void main() {
    vec3 viewDir = normalize(cameraPosition - vWorldPosition);
    float rim = 1.0 - abs(dot(normalize(vWorldNormal), viewDir));
    float glow = pow(clamp(rim, 0.0, 1.0), 2.9) * uStrength;
    if (glow < 0.01) discard;
    gl_FragColor = vec4(uColor, glow);
  }
`;

function createGraticuleGeometry(radius) {
  const positions = [];
  for (let lat = -75; lat <= 75; lat += 15) {
    for (let lng = -180; lng < 180; lng += 4) {
      const a = latLngToSurfaceVector(lat, lng, radius);
      const b = latLngToSurfaceVector(lat, lng + 4, radius);
      positions.push(a.x, a.y, a.z, b.x, b.y, b.z);
    }
  }
  for (let lng = -180; lng < 180; lng += 15) {
    for (let lat = -75; lat < 75; lat += 4) {
      const a = latLngToSurfaceVector(lat, lng, radius);
      const b = latLngToSurfaceVector(Math.min(75, lat + 4), lng, radius);
      positions.push(a.x, a.y, a.z, b.x, b.y, b.z);
    }
  }
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
  return geometry;
}

function createLandGeometry(radius, cells) {
  const surfacePositions = [];
  const outlinePositions = [];
  const surfaceRadius = radius * 1.002;
  const outlineRadius = radius * 1.003;

  for (const cell of cells) {
    const ring = cellToBoundary(cell, true);
    if (ring.length < 3) continue;
    const surfaceRing = [];
    const outlineRing = [];
    const center = new THREE.Vector3();

    for (const [lng, lat] of ring) {
      const surfacePoint = latLngToSurfaceVector(lat, lng, surfaceRadius);
      const outlinePoint = latLngToSurfaceVector(lat, lng, outlineRadius);
      surfaceRing.push(surfacePoint);
      outlineRing.push(outlinePoint);
      center.add(surfacePoint);
    }
    center.divideScalar(surfaceRing.length).normalize().multiplyScalar(surfaceRadius);

    for (let i = 0; i < surfaceRing.length; i++) {
      const curr = surfaceRing[i];
      const next = surfaceRing[(i + 1) % surfaceRing.length];
      surfacePositions.push(center.x, center.y, center.z, curr.x, curr.y, curr.z, next.x, next.y, next.z);
      const currOut = outlineRing[i];
      const nextOut = outlineRing[(i + 1) % outlineRing.length];
      outlinePositions.push(currOut.x, currOut.y, currOut.z, nextOut.x, nextOut.y, nextOut.z);
    }
  }

  const surface = new THREE.BufferGeometry();
  surface.setAttribute("position", new THREE.Float32BufferAttribute(surfacePositions, 3));
  surface.computeVertexNormals();

  const outlines = new THREE.BufferGeometry();
  outlines.setAttribute("position", new THREE.Float32BufferAttribute(outlinePositions, 3));
  return { surface, outlines };
}

function createLabelMaterial(title, subtitle, borderHex) {
  const c = document.createElement("canvas");
  c.width = 340;
  c.height = 84;
  const g = c.getContext("2d");
  g.fillStyle = palette.groundHex === "#111111" ? "rgba(17, 17, 17, 0.92)" : "rgba(245, 245, 245, 0.94)";
  g.fillRect(6, 8, 328, 68);
  g.strokeStyle = palette.lineHex;
  g.lineWidth = 1.2;
  g.strokeRect(6, 8, 328, 68);
  // Left focus bar in Blue (or Yellow/Red for warn/error)
  g.fillStyle = borderHex;
  g.fillRect(6, 8, 5, 68);

  g.font = "700 20px 'B612 Mono', 'JetBrains Mono', monospace";
  g.fillStyle = palette.inkHex;
  g.textBaseline = "middle";
  g.fillText(title, 22, 31);
  g.font = "500 13.5px 'B612 Mono', 'JetBrains Mono', monospace";
  g.fillStyle = palette.ink2Hex;
  g.fillText(subtitle, 22, 55);

  const texture = new THREE.CanvasTexture(c);
  texture.needsUpdate = true;
  return new THREE.SpriteMaterial({
    map: texture,
    transparent: true,
    depthTest: false,
    depthWrite: false
  });
}

// ---------------------------------------------------------------------------
// 5. Miniature 3D Micro-Factory Node Icon Builder (deployrobotic Line-Art Yard)
// ---------------------------------------------------------------------------
function addLineArtBox(parent, w, h, d, x, y, z, edgeColor, occluderMat, fillColor = null, fillOpacity = 1.0) {
  const geom = new THREE.BoxGeometry(w, h, d);
  const meshMat = fillColor
    ? new THREE.MeshBasicMaterial({
        color: fillColor,
        transparent: fillOpacity < 1.0,
        opacity: fillOpacity,
        polygonOffset: true,
        polygonOffsetFactor: 1,
        polygonOffsetUnits: 1
      })
    : occluderMat;
  const mesh = new THREE.Mesh(geom, meshMat);
  mesh.position.set(x, y, z);
  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geom),
    new THREE.LineBasicMaterial({ color: edgeColor, transparent: true, opacity: 0.95 })
  );
  mesh.add(edges);
  parent.add(mesh);
  return { mesh, edges };
}

function createMicroFactoryIcon(nodeData, geo, isFocused, health) {
  const hr = nodeData.host_resources || {};
  const cpuPct = Math.max(4, Math.min(100, hr.cpu_percent || 8));
  const memPct = Math.max(6, Math.min(100, hr.mem_percent || 25));
  const dskPct = Math.max(6, Math.min(100, hr.disk_percent || 25));
  const cores = Math.max(4, hr.cpu_cores || 8);
  const supCount = (nodeData.supervised_processes || []).length;

  // Determine edge colors:
  // - Red for errors
  // - Yellow for warnings
  // - Blue (--accent) edge highlight for the focused node!
  // - Greyscale (--ink / --ink-2) for nominal unfocused elements
  const primaryEdgeColor = health.severity === "error"
    ? palette.red
    : (health.severity === "warn"
      ? palette.warn
      : (isFocused ? palette.accent : palette.ink2));

  const structureEdgeColor = isFocused ? palette.ink : palette.ink2;
  const focusEdgeColor = health.severity === "error"
    ? palette.red
    : (health.severity === "warn" ? palette.warn : palette.accent);

  const root = new THREE.Group();
  const surfacePoint = latLngToSurfaceVector(geo.lat, geo.lng, GLOBE_RADIUS * 1.004);
  const normal = surfacePoint.clone().normalize();
  let east = new THREE.Vector3(0, 1, 0).cross(normal);
  if (east.lengthSq() < 1e-4) east = new THREE.Vector3(1, 0, 0);
  east.normalize();
  const north = normal.clone().cross(east).normalize();

  // Basis matrix so local +X = east, +Y = surface normal (up), +Z = north
  const basis = new THREE.Matrix4().makeBasis(east, normal, north);
  root.position.copy(surfacePoint);
  root.quaternion.setFromRotationMatrix(basis);

  const baseScale = 0.046 * cfgFactoryScale * (isFocused ? 1.18 : 0.88);
  root.scale.setScalar(baseScale);

  const occluder = createOccluderMaterial();

  // 1. Factory Foundation Pad + Blue Focus Edge Frame
  const padW = 2.5, padH = 0.08, padD = 1.95;
  addLineArtBox(root, padW, padH, padD, 0, padH * 0.5, 0, primaryEdgeColor, occluder, palette.ground, 0.92);

  // Outer focus bracket around the factory when focused or alerting
  let focusBracket = null;
  if (isFocused || health.severity !== "nominal") {
    const bw = padW * 1.16, bd = padD * 1.16, by = 0.04;
    const pts = [
      new THREE.Vector3(-bw / 2, by, -bd / 2), new THREE.Vector3(bw / 2, by, -bd / 2),
      new THREE.Vector3(bw / 2, by, bd / 2), new THREE.Vector3(-bw / 2, by, bd / 2),
      new THREE.Vector3(-bw / 2, by, -bd / 2)
    ];
    focusBracket = new THREE.Line(
      new THREE.BufferGeometry().setFromPoints(pts),
      new THREE.LineBasicMaterial({ color: focusEdgeColor, transparent: true, opacity: 0.95 })
    );
    root.add(focusBracket);
  }

  // 2. Left Container Bay: Compute & Local AI Rack (with blinking status LEDs)
  const bayH = 0.52 + Math.min(0.35, (cores / 32) * 0.35);
  const computeBay = addLineArtBox(
    root, 0.95, bayH, 0.62,
    -0.62, padH + bayH * 0.5, 0.38,
    isFocused ? palette.accent : structureEdgeColor,
    occluder
  );
  // Corrugation lines on container front
  const ridgePts = [];
  for (let rx = -0.36; rx <= 0.36; rx += 0.18) {
    ridgePts.push(new THREE.Vector3(rx, -bayH * 0.38, 0.315), new THREE.Vector3(rx, bayH * 0.38, 0.315));
  }
  const ridgeLines = new THREE.LineSegments(
    new THREE.BufferGeometry().setFromPoints(ridgePts),
    new THREE.LineBasicMaterial({ color: palette.line, transparent: true, opacity: 0.65 })
  );
  computeBay.mesh.add(ridgeLines);

  // Staggered Compute Rack Activity LEDs (like deployrobotic createRackActivity)
  const leds = [];
  const ledGeom = new THREE.BoxGeometry(0.09, 0.045, 0.03);
  for (let i = 0; i < 4; i++) {
    const ledColor = health.severity === "error"
      ? palette.red
      : (health.severity === "warn" && i === 0 ? palette.warn : (isFocused ? palette.accent : palette.ink));
    const ledMat = new THREE.MeshBasicMaterial({ color: ledColor, transparent: true, opacity: 0.85 });
    const ledMesh = new THREE.Mesh(ledGeom, ledMat);
    ledMesh.position.set(-0.26 + i * 0.17, -bayH * 0.15 + (i % 2) * 0.14, 0.325);
    computeBay.mesh.add(ledMesh);
    leds.push(ledMat);
  }

  // 3. Rear Main Fabrication Hall & Rooftop Cooling Fans (spin speed = CPU load)
  const hallH = 0.64;
  const hall = addLineArtBox(
    root, 1.35, hallH, 0.68,
    -0.42, padH + hallH * 0.5, -0.42,
    structureEdgeColor,
    occluder
  );

  // Twin rooftop cooling fan rotors
  const rotors = [];
  for (const fx of [-0.34, 0.34]) {
    const rotorGroup = new THREE.Group();
    rotorGroup.position.set(fx, hallH * 0.5 + 0.03, 0);
    const bladePts = [
      new THREE.Vector3(-0.18, 0, 0), new THREE.Vector3(0.18, 0, 0),
      new THREE.Vector3(0, 0, -0.18), new THREE.Vector3(0, 0, 0.18),
      new THREE.Vector3(-0.12, 0, -0.12), new THREE.Vector3(0.12, 0, 0.12),
      new THREE.Vector3(-0.12, 0, 0.12), new THREE.Vector3(0.12, 0, -0.12)
    ];
    const blades = new THREE.LineSegments(
      new THREE.BufferGeometry().setFromPoints(bladePts),
      new THREE.LineBasicMaterial({ color: isFocused ? palette.accent : palette.ink })
    );
    rotorGroup.add(blades);
    hall.mesh.add(rotorGroup);
    rotors.push(rotorGroup);
  }

  // 4. Right Telemetry Silos: 3 Live Vertical Gauge Towers (CPU / MEM / DISK)
  const gauges = [
    { pct: cpuPct, x: 0.42, z: -0.35, warnAt: WARN_CPU_PCT, errAt: ERROR_CPU_PCT },
    { pct: memPct, x: 0.72, z: -0.35, warnAt: WARN_MEM_PCT, errAt: ERROR_MEM_PCT },
    { pct: dskPct, x: 1.02, z: -0.35, warnAt: WARN_DISK_PCT, errAt: ERROR_DISK_PCT }
  ];
  const gaugeColumns = [];
  for (const g of gauges) {
    const maxH = 0.95;
    // Outer wireframe cage showing 100% capacity
    addLineArtBox(root, 0.20, maxH, 0.20, g.x, padH + maxH * 0.5, g.z, palette.line, occluder);
    // Inner live fill column scaled by real resource percentage
    const fillH = Math.max(0.08, (g.pct / 100) * (maxH - 0.04));
    const barColor = g.pct >= g.errAt
      ? palette.red
      : (g.pct >= g.warnAt ? palette.warn : (isFocused ? palette.accent : palette.ink2));
    const fillBar = addLineArtBox(
      root, 0.14, fillH, 0.14,
      g.x, padH + fillH * 0.5 + 0.01, g.z,
      barColor, occluder, barColor, 0.55
    );
    gaugeColumns.push(fillBar.mesh);
  }

  // 5. Comms & Heartbeat Spire (Front-Right Corner)
  const spireX = 0.78, spireZ = 0.46, spireH = 1.65;
  const spirePts = [
    new THREE.Vector3(spireX - 0.12, padH, spireZ - 0.12), new THREE.Vector3(spireX, padH + spireH, spireZ),
    new THREE.Vector3(spireX + 0.12, padH, spireZ - 0.12), new THREE.Vector3(spireX, padH + spireH, spireZ),
    new THREE.Vector3(spireX + 0.12, padH, spireZ + 0.12), new THREE.Vector3(spireX, padH + spireH, spireZ),
    new THREE.Vector3(spireX - 0.12, padH, spireZ + 0.12), new THREE.Vector3(spireX, padH + spireH, spireZ)
  ];
  const spireLines = new THREE.LineSegments(
    new THREE.BufferGeometry().setFromPoints(spirePts),
    new THREE.LineBasicMaterial({ color: primaryEdgeColor })
  );
  root.add(spireLines);

  // Rising telemetry pulse ring on the spire
  const ringPts = [];
  const ringSegs = 16;
  for (let i = 0; i <= ringSegs; i++) {
    const a = (i / ringSegs) * Math.PI * 2;
    ringPts.push(new THREE.Vector3(Math.cos(a) * 0.16, 0, Math.sin(a) * 0.16));
  }
  const pulseRing = new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(ringPts),
    new THREE.LineBasicMaterial({ color: focusEdgeColor, transparent: true, opacity: 0.85 })
  );
  pulseRing.position.set(spireX, padH + 0.3, spireZ);
  root.add(pulseRing);

  // 6. Orbiting Process / Thread Drones above the factory (Layer 5 detail)
  const droneOrbitGroup = new THREE.Group();
  droneOrbitGroup.position.set(0, padH + 1.15, 0);
  const droneCount = Math.max(1, Math.min(4, 1 + supCount + (cores >= 16 ? 1 : 0)));
  const droneGeom = new THREE.OctahedronGeometry(0.085, 0);
  for (let d = 0; d < droneCount; d++) {
    const angle = (d / droneCount) * Math.PI * 2;
    const rad = 0.62;
    const dEdges = new THREE.LineSegments(
      new THREE.EdgesGeometry(droneGeom),
      new THREE.LineBasicMaterial({ color: isFocused ? palette.accent : palette.ink2 })
    );
    dEdges.position.set(Math.cos(angle) * rad, (d % 2) * 0.12, Math.sin(angle) * rad);
    droneOrbitGroup.add(dEdges);
  }
  root.add(droneOrbitGroup);

  // 7. Invisible hit sphere so clicking/tapping the small factory on desktop or mobile is effortless
  const hitTarget = new THREE.Mesh(
    new THREE.SphereGeometry(2.1, 8, 8),
    new THREE.MeshBasicMaterial({ transparent: true, opacity: 0.0, depthWrite: false })
  );
  hitTarget.position.set(0, 0.7, 0);
  hitTarget.userData = { node: nodeData.node };
  root.add(hitTarget);

  // 8. Focused-Only Callout Label (only when focused/selected and labels enabled)
  let labelSprite = null;
  let connectorLine = null;
  if (isFocused && cfgShowLabels) {
    const anchorLocal = new THREE.Vector3(0.95, spireH + 0.95, 0.45);
    const spireTipLocal = new THREE.Vector3(spireX, padH + spireH, spireZ);
    connectorLine = new THREE.Line(
      new THREE.BufferGeometry().setFromPoints([spireTipLocal, anchorLocal]),
      new THREE.LineBasicMaterial({ color: focusEdgeColor })
    );
    root.add(connectorLine);

    const statusTag = health.severity === "error"
      ? `ERR: ${health.errors[0]}`
      : (health.severity === "warn" ? `WARN: ${health.warnings[0]}` : `CPU ${cpuPct.toFixed(0)}% | MEM ${memPct.toFixed(0)}%`);
    const borderHex = health.severity === "error"
      ? palette.redHex
      : (health.severity === "warn" ? palette.warnHex : palette.accentHex);

    labelSprite = new THREE.Sprite(
      createLabelMaterial(nodeData.node.toUpperCase(), `${geo.label || "mesh"} · ${statusTag}`, borderHex)
    );
    labelSprite.position.copy(anchorLocal).add(new THREE.Vector3(0, 0.35, 0));
    labelSprite.scale.set(6.2, 1.55, 1);
    labelSprite.renderOrder = 20;
    labelSprite.userData = { node: nodeData.node };
    root.add(labelSprite);
  }

  // World-space spire tip on the globe surface for connecting great-circle QUIC arcs
  const spireWorldTip = surfacePoint.clone().addScaledVector(normal, spireH * baseScale);

  return {
    node: nodeData.node,
    geo,
    root,
    hitTarget,
    labelSprite,
    rotors,
    leds,
    pulseRing,
    droneOrbitGroup,
    focusBracket,
    spireH,
    padH,
    cpuPct,
    isFocused,
    health,
    spireWorldTip
  };
}

// ---------------------------------------------------------------------------
// 6. Initialize Three.js Scene, Camera & H3 Globe Hierarchy
// ---------------------------------------------------------------------------
const canvas = document.getElementById("hud-3d-canvas");
const renderer = new THREE.WebGLRenderer({
  canvas,
  antialias: true,
  alpha: true,
  powerPreference: "high-performance"
});
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.setSize(window.innerWidth, window.innerHeight, false);
renderer.setClearColor(0x000000, 0);

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(34, window.innerWidth / window.innerHeight, 0.1, 100);
camera.position.set(0, 0, currentCameraDist);
camera.lookAt(0, 0, 0);

const globeRoot = new THREE.Group();
globeRoot.position.set(0, 0.22, 0);
scene.add(globeRoot);

const globeSurface = new THREE.Group();
globeRoot.add(globeSurface);

const sphereGeom = new THREE.SphereGeometry(GLOBE_RADIUS, 48, 36);
const oceanMesh = new THREE.Mesh(sphereGeom, createOccluderMaterial());
globeSurface.add(oceanMesh);

const limbMat = createOutlineMaterial(palette.ink2Hex, 1.6);
const limbMesh = new THREE.Mesh(sphereGeom, limbMat);
limbMesh.scale.setScalar(1.002);
globeSurface.add(limbMesh);

const graticuleMat = new THREE.LineBasicMaterial({
  color: palette.line,
  transparent: true,
  opacity: 0.42
});
const graticuleLines = new THREE.LineSegments(
  createGraticuleGeometry(GLOBE_RADIUS * 1.002),
  graticuleMat
);
globeSurface.add(graticuleLines);

const landGeom = createLandGeometry(GLOBE_RADIUS, LAND_H3_CELLS);
const continentsMat = new THREE.MeshBasicMaterial({
  color: palette.ink,
  transparent: true,
  opacity: 0.08,
  side: THREE.DoubleSide,
  depthWrite: false,
  polygonOffset: true,
  polygonOffsetFactor: 1,
  polygonOffsetUnits: 1
});
const continentsMesh = new THREE.Mesh(landGeom.surface, continentsMat);
globeSurface.add(continentsMesh);

const landOutlinesMat = new THREE.LineBasicMaterial({
  color: palette.ink2,
  transparent: true,
  opacity: 0.46
});
const landOutlines = new THREE.LineSegments(landGeom.outlines, landOutlinesMat);
globeSurface.add(landOutlines);

const cloudUniforms = {
  uTime: { value: 0 },
  uOpacity: { value: 0.12 }
};
const cloudsMesh = new THREE.Mesh(
  new THREE.SphereGeometry(GLOBE_RADIUS * 1.028, 40, 28),
  new THREE.ShaderMaterial({
    transparent: true,
    depthWrite: false,
    side: THREE.FrontSide,
    uniforms: cloudUniforms,
    vertexShader: cloudVertexShader,
    fragmentShader: cloudFragmentShader
  })
);
globeSurface.add(cloudsMesh);

const atmosphereMat = new THREE.ShaderMaterial({
  transparent: true,
  depthWrite: false,
  side: THREE.BackSide,
  blending: THREE.AdditiveBlending,
  uniforms: {
    uColor: { value: new THREE.Color(palette.ink2Hex) },
    uStrength: { value: 0.16 }
  },
  vertexShader: atmosphereVertexShader,
  fragmentShader: atmosphereFragmentShader
});
const atmosphereMesh = new THREE.Mesh(
  new THREE.SphereGeometry(GLOBE_RADIUS * 1.055, 40, 28),
  atmosphereMat
);
globeSurface.add(atmosphereMesh);

// Dynamic groups for H3 active cells, 3D micro-factories, and great-circle QUIC arcs
const activeH3Group = new THREE.Group();
const siteGroup = new THREE.Group();
const arcGroup = new THREE.Group();
globeSurface.add(activeH3Group);
globeSurface.add(siteGroup);
globeSurface.add(arcGroup);

let siteVisuals = [];
let arcVisuals = [];
let lastVisualSignature = "";

function clearGroup(group) {
  while (group.children.length > 0) {
    const child = group.children.pop();
    child.traverse(obj => {
      if (obj.geometry) obj.geometry.dispose();
    });
  }
}

function createGreatCircleArcPoints(vecA, vecB, segments = 44) {
  const va = vecA.clone().normalize();
  const vb = vecB.clone().normalize();
  const angle = va.angleTo(vb);
  const rA = vecA.length();
  const rB = vecB.length();
  const points = [];
  for (let i = 0; i <= segments; i++) {
    const t = i / segments;
    let dir;
    if (angle < 0.001) {
      dir = va.clone();
    } else {
      const sinTotal = Math.sin(angle);
      const wA = Math.sin((1 - t) * angle) / sinTotal;
      const wB = Math.sin(t * angle) / sinTotal;
      dir = va.clone().multiplyScalar(wA).add(vb.clone().multiplyScalar(wB)).normalize();
    }
    const baseR = rA * (1 - t) + rB * t;
    const archLift = Math.sin(t * Math.PI) * Math.min(0.45, 0.09 + angle * 0.24);
    points.push(dir.multiplyScalar(baseR + archLift));
  }
  return points;
}

function rebuildGlobeNodesAndArcs(force = false) {
  const nodes = meshState.nodes || [];
  const rttMatrix = meshState.mesh_rtt_ms || {};
  const locMap = resolveNodeLocationsJS(nodes, rttMatrix);

  const sig = JSON.stringify({
    theme: document.documentElement.dataset.theme,
    sel: selectedNode,
    scale: cfgFactoryScale,
    labels: cfgShowLabels,
    arcs: cfgShowArcs,
    nodes: nodes.map(n => {
      const g = locMap.get(n.node) || {};
      const hr = n.host_resources || {};
      const h = classifyNodeHealth(n);
      return `${n.node}:${(g.lat || 0).toFixed(2)}:${(g.lng || 0).toFixed(2)}:${h.severity}:${Math.round((hr.cpu_percent || 0) / 3)}:${Math.round((hr.mem_percent || 0) / 4)}`;
    })
  });
  if (!force && sig === lastVisualSignature) return locMap;
  lastVisualSignature = sig;

  clearGroup(activeH3Group);
  clearGroup(siteGroup);
  clearGroup(arcGroup);
  siteVisuals = [];
  arcVisuals = [];

  // Highlight active H3 cell for each node (Blue for focused, Yellow for warn, Red for error, faint greyscale for peers)
  for (const n of nodes) {
    const geo = locMap.get(n.node);
    if (!geo) continue;
    try {
      const centerCell = latLngToCell(geo.lat, geo.lng, 3);
      const isFocused = n.node === selectedNode;
      const health = classifyNodeHealth(n);
      const cellOutlineColor = health.severity === "error"
        ? palette.red
        : (health.severity === "warn"
          ? palette.warn
          : (isFocused ? palette.accent : palette.line));
      const hlGeom = createLandGeometry(GLOBE_RADIUS * 1.003, [centerCell]);
      if (isFocused || health.severity !== "nominal") {
        activeH3Group.add(
          new THREE.Mesh(
            hlGeom.surface,
            new THREE.MeshBasicMaterial({
              color: cellOutlineColor,
              transparent: true,
              opacity: isFocused ? 0.18 : 0.14,
              side: THREE.DoubleSide,
              depthWrite: false
            })
          )
        );
      }
      activeH3Group.add(
        new THREE.LineSegments(
          hlGeom.outlines,
          new THREE.LineBasicMaterial({
            color: cellOutlineColor,
            transparent: true,
            opacity: isFocused ? 0.95 : 0.45
          })
        )
      );
    } catch (_) {}
  }

  // Build 3D Micro-Factory Icons for each node
  for (const n of nodes) {
    const geo = locMap.get(n.node);
    if (!geo) continue;
    const isFocused = n.node === selectedNode;
    const health = classifyNodeHealth(n);
    const factoryVis = createMicroFactoryIcon(n, geo, isFocused, health);
    siteGroup.add(factoryVis.root);
    siteVisuals.push(factoryVis);
  }

  // Build Great-Circle QUIC Latency Arcs between factory comms spires
  if (cfgShowArcs && siteVisuals.length > 1) {
    const pktGeom = new THREE.BoxGeometry(0.028, 0.028, 0.055);
    for (let i = 0; i < siteVisuals.length; i++) {
      for (let j = i + 1; j < siteVisuals.length; j++) {
        const svA = siteVisuals[i];
        const svB = siteVisuals[j];
        const touchesFocused = svA.isFocused || svB.isFocused;
        const hasErr = svA.health.severity === "error" || svB.health.severity === "error";
        const hasWarn = svA.health.severity === "warn" || svB.health.severity === "warn";

        const arcColor = hasErr
          ? palette.red
          : (hasWarn ? palette.warn : (touchesFocused ? palette.accent : palette.line));

        const pts = createGreatCircleArcPoints(svA.spireWorldTip, svB.spireWorldTip, 48);
        const line = new THREE.Line(
          new THREE.BufferGeometry().setFromPoints(pts),
          new THREE.LineBasicMaterial({
            color: arcColor,
            transparent: true,
            opacity: touchesFocused ? 0.82 : 0.36
          })
        );

        const pktFwd = new THREE.Mesh(
          pktGeom,
          new THREE.MeshBasicMaterial({ color: touchesFocused ? palette.accent : palette.ink })
        );
        const pktRev = new THREE.Mesh(
          pktGeom,
          new THREE.MeshBasicMaterial({ color: hasErr ? palette.red : (hasWarn ? palette.warn : palette.ink2) })
        );
        arcGroup.add(line, pktFwd, pktRev);
        const rtt = lookupPairRtt(svA.node, svB.node, nodes, rttMatrix) ?? 20.0;
        arcVisuals.push({ pts, line, pktFwd, pktRev, rtt, phase: (i + j) * 0.31 });
      }
    }
  }

  return locMap;
}

// ---------------------------------------------------------------------------
// 7. Endless Tour Loop & deployrobotic Layer Camera Choreography
// ---------------------------------------------------------------------------
const raycaster = new THREE.Raycaster();
const pointerNDC = new THREE.Vector2();
const viewDir = new THREE.Vector3(0.0, 0.06, 1.0).normalize();

let tourFromQuat = new THREE.Quaternion();
let tourToQuat = new THREE.Quaternion();
let tourFlightStartMs = performance.now();
let lastTourSwitchMs = performance.now();
let tourInitialized = false;

function easeInOutCubic(t) {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

function targetQuatForGeo(geo) {
  const siteNormal = latLngToSurfaceVector(geo.lat, geo.lng, 1).normalize();
  return new THREE.Quaternion().setFromUnitVectors(siteNormal, viewDir);
}

function buildTourSequence() {
  const nodes = meshState.nodes || [];
  if (nodes.length === 0) return [];
  const stops = [];
  if (layerMode === "auto") {
    // Endless loop showing all elements & layers of the system continuously:
    // Layer 1 (Globe overview) -> Layer 2 (Regional H3 cluster) -> then for each node: Layer 3 (Site Factory), Layer 4 (Containers), Layer 5 (Processes)
    const anchorNode = nodes[0].node;
    stops.push({ node: anchorNode, layer: 1, label: "PLANETARY H3 MESH" });
    if (nodes.length > 1) {
      stops.push({ node: anchorNode, layer: 2, label: "REGIONAL RTT CLUSTER" });
    }
    for (const n of nodes) {
      stops.push({ node: n.node, layer: 3, label: `${n.node} // SITE FACTORY` });
      stops.push({ node: n.node, layer: 4, label: `${n.node} // CONTAINERS & VARIABILITY` });
      stops.push({ node: n.node, layer: 5, label: `${n.node} // PROCESSES & QUIC` });
    }
  } else {
    const fixedLayer = Number(layerMode) || 3;
    const layerInfo = ZOOM_LAYERS.find(l => l.layer === fixedLayer) || ZOOM_LAYERS[2];
    for (const n of nodes) {
      stops.push({ node: n.node, layer: fixedLayer, label: `${n.node} // ${layerInfo.name}` });
    }
  }
  return stops;
}

function applyStop(stop, nowMs = performance.now()) {
  if (!stop) return;
  const prevNode = selectedNode;
  selectedNode = stop.node;
  currentZoomLayer = stop.layer;
  const layerSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[2];
  targetCameraDist = layerSpec.camDist;

  // Sync slider in menu
  const zSlider = document.getElementById("slider-zoom-layer");
  const zVal = document.getElementById("val-zoom-layer");
  if (zSlider && zVal) {
    zSlider.value = String(currentZoomLayer);
    zVal.textContent = `${currentZoomLayer}.0 (${layerSpec.name})`;
  }

  userDragOffset.identity();
  rebuildGlobeNodesAndArcs(true);
  renderTopShellTelemetry();
  syncMenuControls();

  const targetVisual = siteVisuals.find(sv => sv.node === selectedNode) || siteVisuals[0];
  if (targetVisual) {
    const destQuat = targetQuatForGeo(targetVisual.geo);
    if (!tourInitialized) {
      globeSurface.quaternion.copy(destQuat);
      tourFromQuat.copy(destQuat);
      tourToQuat.copy(destQuat);
      currentCameraDist = targetCameraDist;
      tourInitialized = true;
    } else if (prevNode !== selectedNode) {
      tourFromQuat.copy(globeSurface.quaternion);
      tourToQuat.copy(destQuat);
      tourFlightStartMs = nowMs;
    }
  }
  lastTourSwitchMs = nowMs;
}

function stepTour(deltaSteps, nowMs = performance.now()) {
  tourStops = buildTourSequence();
  if (tourStops.length === 0) return;
  tourStopIndex = (tourStopIndex + deltaSteps + tourStops.length) % tourStops.length;
  applyStop(tourStops[tourStopIndex], nowMs);
}

function focusNodeManual(nodeName, layerOverride = null) {
  const nowMs = performance.now();
  const targetLayer = layerOverride || (layerMode === "auto" ? 3 : Number(layerMode));
  applyStop({ node: nodeName, layer: targetLayer, label: `${nodeName} // MANUAL FOCUS` }, nowMs);
}

// ---------------------------------------------------------------------------
// 8. Linux 'top' Terminal Overlay + Mesh Connection Instructions
// ---------------------------------------------------------------------------
function asciiBar(pct, warnAt, errAt) {
  const slots = 12;
  const filled = Math.max(0, Math.min(slots, Math.round((pct / 100) * slots)));
  const empty = slots - filled;
  const cls = pct >= errAt ? "fill err" : (pct >= warnAt ? "fill warn" : "fill");
  return `<span class="ascii-bar">[<span class="${cls}">${"|".repeat(filled)}</span>${".".repeat(empty)}]</span>`;
}

function fmtGiB(bytes) {
  return (bytes / (1024 * 1024 * 1024)).toFixed(1);
}

function renderTopShellTelemetry() {
  const nodes = meshState.nodes || [];
  const locMap = rebuildGlobeNodesAndArcs(false);
  if (!nodes.length) return;

  const idx = Math.max(0, nodes.findIndex(n => n.node === selectedNode));
  const target = nodes[idx] || nodes[0];
  const hr = target.host_resources || {};
  const ss = target.summary_stats || null;
  const health = classifyNodeHealth(target);
  const geo = locMap.get(target.node) || target.geo_location || {
    lat: 37.62,
    lng: -122.38,
    source: "latency_cluster",
    label: "RTT Cluster"
  };

  let h3CellId = "--";
  try {
    h3CellId = latLngToCell(geo.lat, geo.lng, 3);
  } catch (_) {}

  const layerSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[2];
  const meshConnected = Boolean(meshState.channel_configured) && nodes.length > 1;
  const overlaySeverity = !meshState.channel_configured
    ? "warn"
    : health.severity;

  // Update Terminal Header Badges
  const termEl = document.getElementById("terminal-overlay");
  if (termEl) termEl.dataset.severity = overlaySeverity;
  document.getElementById("hud-layer-num").textContent = String(layerSpec.layer);
  document.getElementById("hud-layer-name").textContent = layerSpec.name;

  const tourBadge = document.getElementById("hud-tour-badge");
  const tourText = document.getElementById("hud-tour-text");
  if (tourBadge && tourText) {
    tourBadge.dataset.playing = String(tourPlaying);
    tourText.textContent = tourPlaying ? "TOUR LOOP" : "PAUSED";
  }

  document.getElementById("hud-stop-label").textContent =
    `NODE ${idx + 1}/${nodes.length}: ${target.node.toUpperCase()} (${geo.label || "mesh"})`;

  const statusPill = document.getElementById("hud-status-pill");
  if (statusPill) {
    statusPill.dataset.severity = overlaySeverity;
    if (health.severity === "error") {
      statusPill.textContent = `ERROR: ${health.errors.join(", ")}`;
    } else if (!meshState.channel_configured) {
      statusPill.textContent = "WARN: MESH UNCONFIGURED";
    } else if (health.severity === "warn") {
      statusPill.textContent = `WARN: ${health.warnings.join(", ")}`;
    } else {
      statusPill.textContent = `NOMINAL (${nodes.length} PEERS)`;
    }
  }

  const cpuVal = hr.cpu_percent || 0;
  const memVal = hr.mem_percent || 0;
  const diskVal = hr.disk_percent || 0;

  const cpuCls = cpuVal >= ERROR_CPU_PCT ? "err" : (cpuVal >= WARN_CPU_PCT ? "warn" : "hi");
  const memCls = memVal >= ERROR_MEM_PCT ? "err" : (memVal >= WARN_MEM_PCT ? "warn" : "hi");
  const dskCls = diskVal >= ERROR_DISK_PCT ? "err" : (diskVal >= WARN_DISK_PCT ? "warn" : "hi");

  const cpuStd = (ss && ss.cpu_pct && typeof ss.cpu_pct.stddev === "number") ? ss.cpu_pct.stddev.toFixed(1) : "0.0";
  const cpuAvg = (ss && ss.cpu_pct && typeof ss.cpu_pct.mean === "number") ? ss.cpu_pct.mean.toFixed(1) : cpuVal.toFixed(1);
  const cpuMax = (ss && ss.cpu_pct && typeof ss.cpu_pct.max === "number") ? ss.cpu_pct.max.toFixed(1) : cpuVal.toFixed(1);
  const memStd = (ss && ss.mem_pct && typeof ss.mem_pct.stddev === "number") ? ss.mem_pct.stddev.toFixed(1) : "0.0";
  const thrashHtml = (ss && ss.thrashing_detected)
    ? `<span class="err">THRASHING (score ${ss.thrashing_score.toFixed(0)})</span>`
    : `<span class="hi">nominal</span>`;

  const nic = (hr.interfaces && hr.interfaces[0]) || { rx_bytes_per_sec: 0, tx_bytes_per_sec: 0 };
  const rxKb = (nic.rx_bytes_per_sec / 1024).toFixed(0);
  const txKb = (nic.tx_bytes_per_sec / 1024).toFixed(0);
  const bindIp = (target.iroh_transport && target.iroh_transport.bound_non_tailnet_ipv4) || "wan";
  const supJobs = (target.supervised_processes || []).length;

  const peerRttEntries = Object.entries(target.peer_rtt_ms || {})
    .map(([peer, ms]) => {
      const num = Number(ms);
      const rCls = num >= 220 ? "warn" : "hi";
      return `${peer}:<span class="${rCls}">${num.toFixed(1)}ms</span>`;
    })
    .join("  ");

  const nowStr = new Date().toTimeString().slice(0, 8);
  const hbKind = target.heartbeat_kind || "regular";

  const lines = [
    `<span class="focus">top</span> - ${nowStr}  node <span class="focus">${idx + 1}/${nodes.length}</span>: <span class="hi">${target.node}</span> (${target.os || "os"}/${target.arch || "arch"})  h3:<span class="hi">${h3CellId}</span>  loc:<span class="hi">${geo.label}</span>`,
    `<span class="dim">Tasks:</span>  <span class="hi">${hr.cpu_cores || 8}</span> cores, <span class="hi">${supJobs}</span> sup_jobs, hb <span class="hi">#${target.counter}</span> (${hbKind} 5s/30s)  <span class="dim">|</span>  <span class="focus">L${layerSpec.layer} ${layerSpec.name}:</span> ${layerSpec.desc}`,
    `<span class="dim">%Cpu(s):</span> ${asciiBar(cpuVal, WARN_CPU_PCT, ERROR_CPU_PCT)} <span class="${cpuCls}">${cpuVal.toFixed(1).padStart(5, " ")}%</span> us, 30s avg ${cpuAvg}% ±${cpuStd}% (pk ${cpuMax}%), state: ${thrashHtml}`,
    `<span class="dim">GiB Mem:</span> ${asciiBar(memVal, WARN_MEM_PCT, ERROR_MEM_PCT)} <span class="${memCls}">${memVal.toFixed(1).padStart(5, " ")}%</span> used (${fmtGiB(hr.mem_used_bytes || 0)}/${fmtGiB(hr.mem_total_bytes || 1)} GiB, ±${memStd}%)   <span class="dim">GiB Dsk:</span> ${asciiBar(diskVal, WARN_DISK_PCT, ERROR_DISK_PCT)} <span class="${dskCls}">${diskVal.toFixed(1).padStart(5, " ")}%</span> (${fmtGiB(hr.disk_used_bytes || 0)}/${fmtGiB(hr.disk_total_bytes || 1)} GiB)`
  ];

  if (!meshConnected) {
    lines.push(
      `<span class="warn">WARN: MESH NOT CONNECTED (${meshState.channel_configured ? "waiting for peers" : "standby mode"})</span> — <span class="focus">HOW TO CONNECT MESH:</span>`,
      `  <span class="focus">1.</span> Init channel: <span class="hi">dialtone channel init</span>   <span class="focus">2.</span> Invite peer: <span class="hi">dialtone channel invite</span>   <span class="focus">3.</span> Join: <span class="hi">dialtone channel join &lt;ticket&gt;</span>  (or tap <span class="focus">[≡]</span> bottom-right)`
    );
  } else {
    lines.push(
      `<span class="dim">Net/RTT:</span> rx <span class="hi">${rxKb}</span> KB/s  tx <span class="hi">${txKb}</span> KB/s  <span class="dim">|</span>  peers: ${peerRttEntries || "probing..."}  <span class="dim">|</span>  bind <span class="hi">${bindIp}</span>`
    );
  }

  document.getElementById("node-top-hud").innerHTML = lines.join("\n");
}

// ---------------------------------------------------------------------------
// 9. Thumb Button & Full-Screen Menu Interactions
// ---------------------------------------------------------------------------
const thumbBtn = document.getElementById("thumb-menu-btn");
const fullscreenMenu = document.getElementById("fullscreen-menu");

function toggleFullscreenMenu(forceOpen = null) {
  const isOpen = forceOpen !== null ? forceOpen : fullscreenMenu.hasAttribute("hidden");
  if (isOpen) {
    fullscreenMenu.removeAttribute("hidden");
    thumbBtn.setAttribute("aria-expanded", "true");
    syncMenuControls();
  } else {
    fullscreenMenu.setAttribute("hidden", "");
    thumbBtn.setAttribute("aria-expanded", "false");
  }
}

thumbBtn.addEventListener("click", () => toggleFullscreenMenu());

function syncMenuControls() {
  const nodes = meshState.nodes || [];
  const playBtn = document.getElementById("tour-playback");
  const tourStatus = document.getElementById("menu-tour-status");
  if (playBtn && tourStatus) {
    playBtn.setAttribute("aria-pressed", String(tourPlaying));
    playBtn.classList.toggle("is-focus", tourPlaying);
    playBtn.textContent = tourPlaying ? "▶ TOUR LOOP: ON (PAUSE)" : "■ TOUR PAUSED (PLAY LOOP)";
    tourStatus.textContent = tourPlaying ? "PLAYING ENDLESS LOOP" : "PAUSED (MANUAL)";
  }

  // Layer mode buttons
  document.querySelectorAll("#layer-btn-row button").forEach(btn => {
    const active = btn.dataset.layerMode === String(layerMode);
    btn.setAttribute("aria-pressed", String(active));
    btn.classList.toggle("is-focus", active);
  });
  const layerReadout = document.getElementById("menu-layer-readout");
  if (layerReadout) {
    const lSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[2];
    layerReadout.textContent = layerMode === "auto"
      ? `AUTO (CURRENT: L${lSpec.layer} ${lSpec.name})`
      : `LOCKED L${lSpec.layer} ${lSpec.name}`;
  }

  // Node selector buttons
  const nodeCountEl = document.getElementById("menu-node-count");
  if (nodeCountEl) nodeCountEl.textContent = `${nodes.length} ACTIVE`;
  const nodeWrap = document.getElementById("menu-node-buttons");
  if (nodeWrap) {
    nodeWrap.innerHTML = "";
    for (const n of nodes) {
      const h = classifyNodeHealth(n);
      const isSel = n.node === selectedNode;
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = `ctrl-btn${isSel ? " is-focus" : ""}`;
      btn.setAttribute("aria-pressed", String(isSel));
      btn.dataset.severity = h.severity;
      const dotCls = h.severity === "error" ? "error" : (h.severity === "warn" ? "warn" : (isSel ? "focus" : ""));
      const hr = n.host_resources || {};
      btn.innerHTML = `<span class="node-dot ${dotCls}"></span><span>${n.node}</span><span style="color:var(--ink-2)">${(hr.cpu_percent || 0).toFixed(0)}%</span>`;
      btn.addEventListener("click", () => {
        tourPlaying = false;
        focusNodeManual(n.node);
      });
      nodeWrap.appendChild(btn);
    }
  }

  // Mesh state label
  const meshLabel = document.getElementById("menu-mesh-state-label");
  if (meshLabel) {
    meshLabel.textContent = meshState.channel_configured
      ? `PRIVATE MESH (${meshState.channel_id || "active"})`
      : "STANDBY (UNCONFIGURED)";
  }
}

// Menu button handlers
document.getElementById("tour-playback").addEventListener("click", () => {
  tourPlaying = !tourPlaying;
  lastTourSwitchMs = performance.now();
  renderTopShellTelemetry();
  syncMenuControls();
});

document.getElementById("btn-prev-stop").addEventListener("click", () => {
  stepTour(-1, performance.now());
});

document.getElementById("btn-next-stop").addEventListener("click", () => {
  stepTour(1, performance.now());
});

document.getElementById("btn-reset-cam").addEventListener("click", () => {
  userDragOffset.identity();
  focusNodeManual(selectedNode);
});

document.querySelectorAll("#layer-btn-row button").forEach(btn => {
  btn.addEventListener("click", () => {
    const m = btn.dataset.layerMode;
    layerMode = m === "auto" ? "auto" : Number(m);
    if (layerMode !== "auto") {
      focusNodeManual(selectedNode, layerMode);
    } else {
      stepTour(0, performance.now());
    }
    syncMenuControls();
  });
});

// Theme toggle (deployrobotic dark / light greyscale)
document.getElementById("menu-theme-btn").addEventListener("click", (e) => {
  const root = document.documentElement;
  const next = root.dataset.theme === "dark" ? "light" : "dark";
  root.dataset.theme = next;
  e.currentTarget.textContent = `THEME: ${next.toUpperCase()}`;
  palette = readThemePalette();
  limbMat.uniforms.uColor.value.set(palette.ink2Hex);
  graticuleMat.color.copy(palette.line);
  continentsMat.color.copy(palette.ink);
  landOutlinesMat.color.copy(palette.ink2);
  atmosphereMat.uniforms.uColor.value.set(palette.ink2Hex);
  rebuildGlobeNodesAndArcs(true);
});

// Sliders
document.getElementById("slider-zoom-layer").addEventListener("input", (e) => {
  const val = parseFloat(e.target.value);
  continuousCameraLayer = val;
  const nearest = Math.round(val);
  currentZoomLayer = Math.max(1, Math.min(5, nearest));
  layerMode = currentZoomLayer;
  const lSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[2];
  // Interpolate camera distance smoothly between layers
  const lowIdx = Math.max(0, Math.min(4, Math.floor(val) - 1));
  const highIdx = Math.max(0, Math.min(4, Math.ceil(val) - 1));
  const frac = val - Math.floor(val);
  targetCameraDist = THREE.MathUtils.lerp(ZOOM_LAYERS[lowIdx].camDist, ZOOM_LAYERS[highIdx].camDist, frac);
  document.getElementById("val-zoom-layer").textContent = `${val.toFixed(1)} (${lSpec.name})`;
  renderTopShellTelemetry();
  syncMenuControls();
});

document.getElementById("slider-tour-dwell").addEventListener("input", (e) => {
  const sec = parseFloat(e.target.value);
  cfgTourDwellMs = sec * 1000;
  document.getElementById("val-tour-dwell").textContent = `${sec.toFixed(1)}s`;
});

document.getElementById("slider-factory-scale").addEventListener("input", (e) => {
  cfgFactoryScale = parseFloat(e.target.value);
  document.getElementById("val-factory-scale").textContent = `${cfgFactoryScale.toFixed(1)}x`;
  rebuildGlobeNodesAndArcs(true);
});

document.getElementById("slider-land-ink").addEventListener("input", (e) => {
  cfgLandInk = parseFloat(e.target.value) / 100;
  document.getElementById("val-land-ink").textContent = `${Math.round(cfgLandInk * 100)}%`;
});

document.getElementById("slider-cloud-opacity").addEventListener("input", (e) => {
  cfgCloudOpacity = parseFloat(e.target.value) / 100;
  document.getElementById("val-cloud-opacity").textContent = `${Math.round(cfgCloudOpacity * 100)}%`;
});

document.getElementById("slider-packet-speed").addEventListener("input", (e) => {
  cfgPacketSpeed = parseFloat(e.target.value);
  document.getElementById("val-packet-speed").textContent = `${cfgPacketSpeed.toFixed(1)}x`;
});

document.getElementById("slider-term-opacity").addEventListener("input", (e) => {
  const pct = parseFloat(e.target.value);
  document.getElementById("val-term-opacity").textContent = `${Math.round(pct)}%`;
  document.getElementById("terminal-overlay").style.opacity = String(pct / 100);
});

// Toggle buttons
function bindToggle(id, labelPrefix, getter, setter) {
  const btn = document.getElementById(id);
  btn.addEventListener("click", () => {
    const next = !getter();
    setter(next);
    btn.setAttribute("aria-pressed", String(next));
    btn.textContent = `${labelPrefix}: ${next ? "ON" : "OFF"}`;
    rebuildGlobeNodesAndArcs(true);
  });
}
bindToggle("toggle-h3-land", "H3 LAND", () => cfgShowLand, v => { cfgShowLand = v; });
bindToggle("toggle-quic-arcs", "QUIC ARCS", () => cfgShowArcs, v => { cfgShowArcs = v; });
bindToggle("toggle-clouds", "CLOUDS", () => cfgShowClouds, v => { cfgShowClouds = v; });
bindToggle("toggle-labels", "FOCUS LABEL", () => cfgShowLabels, v => { cfgShowLabels = v; });
bindToggle("toggle-drones", "FACTORY ACTIVITY", () => cfgShowActivity, v => { cfgShowActivity = v; });

// Mesh API actions (Init / Invite / Join)
async function callMeshApi(path, bodyObj) {
  const outEl = document.getElementById("menu-mesh-output");
  outEl.textContent = `Calling ${path}...`;
  try {
    const resp = await fetch(path, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${API_TOKEN}`
      },
      body: JSON.stringify(bodyObj || {})
    });
    const data = await resp.json();
    outEl.textContent = JSON.stringify(data, null, 2);
    await pollGossipState();
  } catch (err) {
    outEl.textContent = `ERROR: ${err}`;
  }
}

document.getElementById("btn-init-mesh").addEventListener("click", () => {
  callMeshApi("/api/channel/init", {});
});
document.getElementById("btn-invite-mesh").addEventListener("click", () => {
  callMeshApi("/api/channel/invite", { ttl_secs: 900 });
});
document.getElementById("btn-join-mesh").addEventListener("click", () => {
  const ticket = document.getElementById("input-join-ticket").value.trim();
  if (!ticket) {
    document.getElementById("menu-mesh-output").textContent = "Paste an iroh-gossip://invite/... ticket first.";
    return;
  }
  callMeshApi("/api/channel/join", { ticket });
});

// ---------------------------------------------------------------------------
// 10. Pointer Orbit, Raycast Picking & Frame Loop
// ---------------------------------------------------------------------------
window.addEventListener("resize", () => {
  const w = window.innerWidth;
  const h = window.innerHeight;
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(w, h, false);
  camera.aspect = w / Math.max(h, 1);
  camera.updateProjectionMatrix();
  limbMat.uniforms.uResolution.value.set(w, h);
});

canvas.addEventListener("pointerdown", (e) => {
  pointerDownPos = { x: e.clientX, y: e.clientY };
  lastMouse = { x: e.clientX, y: e.clientY };
  dragging = true;
});

window.addEventListener("pointermove", (e) => {
  if (!dragging) return;
  const dx = e.clientX - lastMouse.x;
  const dy = e.clientY - lastMouse.y;
  lastMouse = { x: e.clientX, y: e.clientY };
  const qYaw = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), dx * 0.0055);
  const qPitch = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), dy * 0.0055);
  userDragOffset.premultiply(qYaw).premultiply(qPitch);
  lastTourSwitchMs = performance.now();
});

window.addEventListener("pointerup", (e) => {
  if (!dragging) return;
  dragging = false;
  if (Math.hypot(e.clientX - pointerDownPos.x, e.clientY - pointerDownPos.y) < 6) {
    const rect = canvas.getBoundingClientRect();
    pointerNDC.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    pointerNDC.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
    raycaster.setFromCamera(pointerNDC, camera);
    const pickables = siteVisuals.flatMap(sv =>
      sv.labelSprite ? [sv.hitTarget, sv.labelSprite] : [sv.hitTarget]
    );
    const hits = raycaster.intersectObjects(pickables, false);
    if (hits.length > 0 && hits[0].object.userData && hits[0].object.userData.node) {
      focusNodeManual(hits[0].object.userData.node);
    }
  }
});

canvas.addEventListener("wheel", (e) => {
  e.preventDefault();
  targetCameraDist = Math.max(3.0, Math.min(11.5, targetCameraDist + e.deltaY * 0.004));
}, { passive: false });

window.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !fullscreenMenu.hasAttribute("hidden")) {
    toggleFullscreenMenu(false);
  }
});

let prevTimeMs = performance.now();

function renderFrame(nowMs) {
  const delta = Math.min((nowMs - prevTimeMs) / 1000, 0.05);
  prevTimeMs = nowMs;
  const elapsed = nowMs / 1000;

  // Advance endless tour loop when playing
  const cycleMs = cfgTourFlightMs + cfgTourDwellMs;
  if (tourPlaying && !dragging && (nowMs - lastTourSwitchMs) >= cycleMs) {
    stepTour(1, nowMs);
  }

  // Smoothly interpolate continuous zoom layer opacity (deployrobotic zoom-layers.ts)
  continuousCameraLayer = THREE.MathUtils.lerp(continuousCameraLayer, currentZoomLayer, 1 - Math.exp(-delta * 4.5));
  const globeOutlineWeight = zoomLayerOpacity(1, continuousCameraLayer, "outline");
  const globeDetailWeight = zoomLayerOpacity(1, continuousCameraLayer, "detail");

  continentsMesh.visible = cfgShowLand;
  landOutlines.visible = cfgShowLand;
  continentsMat.opacity = 0.08 * cfgLandInk * globeDetailWeight;
  landOutlinesMat.opacity = 0.48 * cfgLandInk * globeOutlineWeight;
  graticuleLines.visible = globeDetailWeight > 0.02;
  graticuleMat.opacity = 0.36 * globeDetailWeight;

  cloudsMesh.visible = cfgShowClouds;
  cloudUniforms.uTime.value = elapsed;
  cloudUniforms.uOpacity.value = 0.22 * cfgCloudOpacity * globeDetailWeight;
  atmosphereMesh.visible = cfgShowClouds;
  atmosphereMat.uniforms.uStrength.value = 0.24 * cfgCloudOpacity * globeOutlineWeight;

  // Camera & Globe Quaternion slerp
  const targetVisual = siteVisuals.find(sv => sv.node === selectedNode) || siteVisuals[0];
  if (targetVisual && !dragging) {
    const flightRaw = Math.min(1, Math.max(0, (nowMs - tourFlightStartMs) / cfgTourFlightMs));
    const flightEased = easeInOutCubic(flightRaw);

    const destQuat = targetQuatForGeo(targetVisual.geo);
    const dwellOrbitAngle = Math.sin(elapsed * 0.48) * (currentZoomLayer >= 3 ? 0.045 : 0.11) * flightEased;
    const orbitQuat = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), dwellOrbitAngle);
    destQuat.premultiply(orbitQuat);
    destQuat.premultiply(userDragOffset);

    if (flightRaw < 1.0) {
      globeSurface.quaternion.copy(tourFromQuat).slerp(destQuat, flightEased);
    } else {
      globeSurface.quaternion.slerp(destQuat, 1 - Math.exp(-delta * 5.0));
    }

    const flightArcLift = Math.sin(flightRaw * Math.PI) * 0.95;
    currentCameraDist = THREE.MathUtils.lerp(currentCameraDist, targetCameraDist, 1 - Math.exp(-delta * 4.2));
    camera.position.set(0, 0, currentCameraDist + flightArcLift);
  } else if (dragging && targetVisual) {
    const destQuat = targetQuatForGeo(targetVisual.geo);
    destQuat.premultiply(userDragOffset);
    globeSurface.quaternion.slerp(destQuat, 0.35);
    camera.position.set(0, 0, currentCameraDist);
  }

  globeRoot.position.y = 0.22 + Math.sin(elapsed * 0.6) * 0.018;

  // Animate each 3D Micro-Factory ("alive" with CPU fans, rack LEDs, spire heartbeat ring, and process drones)
  siteVisuals.forEach((sv, idx) => {
    const fanSpeed = cfgShowActivity ? (1.8 + sv.cpuPct * 0.14) : 0;
    for (const rotor of sv.rotors) {
      rotor.rotation.y = elapsed * fanSpeed;
    }

    sv.leds.forEach((ledMat, ledIdx) => {
      if (!cfgShowActivity) {
        ledMat.opacity = 0.7;
        return;
      }
      const wave = 0.5 + 0.5 * Math.sin(elapsed * (3.2 + ledIdx * 0.7 + sv.cpuPct * 0.05) + idx * 1.9);
      ledMat.opacity = 0.25 + 0.75 * Math.pow(wave, 3);
    });

    if (sv.pulseRing) {
      const p = (elapsed * 0.65 + idx * 0.33) % 1.0;
      sv.pulseRing.position.y = sv.padH + p * sv.spireH;
      sv.pulseRing.scale.setScalar(1.0 - p * 0.55);
      sv.pulseRing.material.opacity = (1.0 - p) * (sv.isFocused ? 0.95 : 0.55);
    }

    if (sv.droneOrbitGroup) {
      sv.droneOrbitGroup.visible = cfgShowActivity;
      sv.droneOrbitGroup.rotation.y = elapsed * (sv.isFocused ? 1.35 : 0.75) + idx;
    }

    if (sv.focusBracket) {
      sv.focusBracket.material.opacity = 0.65 + 0.35 * Math.sin(elapsed * 3.8);
    }
  });

  // Animate QUIC packets along great-circle arcs
  for (const av of arcVisuals) {
    const baseSpeed = Math.max(0.22, Math.min(1.1, 14.0 / Math.max(4.0, av.rtt))) * cfgPacketSpeed;
    const tFwd = (elapsed * baseSpeed + av.phase) % 1.0;
    const tRev = (1.0 - tFwd + 1.0) % 1.0;
    const idxFwd = Math.min(av.pts.length - 1, Math.floor(tFwd * av.pts.length));
    const idxRev = Math.min(av.pts.length - 1, Math.floor(tRev * av.pts.length));
    av.pktFwd.position.copy(av.pts[idxFwd]);
    av.pktRev.position.copy(av.pts[idxRev]);
  }

  renderer.render(scene, camera);
  requestAnimationFrame(renderFrame);
}

async function pollGossipState() {
  try {
    const resp = await fetch("/api/gossip", { cache: "no-store" });
    if (!resp.ok) return;
    const data = await resp.json();
    meshState = data;
    if (!meshState.nodes.some(n => n.node === selectedNode) && meshState.nodes.length > 0) {
      selectedNode = meshState.self_node || meshState.nodes[0].node;
      stepTour(0, performance.now());
    } else {
      renderTopShellTelemetry();
      if (!fullscreenMenu.hasAttribute("hidden")) {
        syncMenuControls();
      }
    }
  } catch (_) {}
}

tourStops = buildTourSequence();
if (tourStops.length > 0) {
  // Start on Layer 3 (Site Factory) of the initial node so the micro-factory is immediately visible, then loop endlessly
  const startIdx = Math.max(0, tourStops.findIndex(s => s.node === selectedNode && s.layer === 3));
  tourStopIndex = startIdx;
  applyStop(tourStops[tourStopIndex], performance.now());
} else {
  renderTopShellTelemetry();
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
        .replace("__LAND_H3_BITSET_B64__", LAND_H3_BITSET_B64)
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

fn extract_host_only(raw: &str) -> String {
    let trimmed = raw.trim();
    let without_scheme = trimmed
        .strip_prefix("http://")
        .or_else(|| trimmed.strip_prefix("https://"))
        .or_else(|| trimmed.strip_prefix("HTTP://"))
        .or_else(|| trimmed.strip_prefix("HTTPS://"))
        .unwrap_or(trimmed);
    let authority = without_scheme.split('/').next().unwrap_or("");
    if let Some(rest) = authority.strip_prefix('[') {
        rest.split(']').next().unwrap_or("").to_ascii_lowercase()
    } else {
        authority
            .split(':')
            .next()
            .unwrap_or("")
            .to_ascii_lowercase()
    }
}

fn is_allowed_local_or_tailnet_host(host_only: &str) -> bool {
    if matches!(
        host_only,
        "" | "localhost" | "127.0.0.1" | "::1" | "0.0.0.0"
    ) {
        return true;
    }
    if let Ok(ip) = host_only.parse::<std::net::IpAddr>() {
        return ip.is_loopback() || crate::identity::is_tailnet_ip(ip);
    }
    host_only.ends_with(".ts.net") || !host_only.contains('.')
}

fn validate_local_http_request(
    method: &str,
    headers: &HashMap<String, String>,
) -> Result<(), String> {
    if let Some(host_val) = headers.get("host") {
        let host_only = extract_host_only(host_val);
        if !is_allowed_local_or_tailnet_host(&host_only) {
            return Err(format!(
                "forbidden Host header '{host_val}': HTTP control plane only accepts localhost, 127.0.0.1, or tailnet (.ts.net / 100.64.0.0/10) hosts"
            ));
        }
    }

    if method.eq_ignore_ascii_case("POST") {
        if let Some(origin_val) = headers.get("origin") {
            let orig = origin_val.trim().to_ascii_lowercase();
            if orig != "null" {
                let origin_host = extract_host_only(&orig);
                if !is_allowed_local_or_tailnet_host(&origin_host) {
                    return Err(format!(
                        "forbidden Origin header '{origin_val}': cross-origin POST requests are blocked"
                    ));
                }
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
