use std::collections::HashMap;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use serde::Serialize;
use serde_json::{json, Value};

use crate::blob::{BlobTransportReport, Blobs, Store};
use crate::deploy::{
    execute_verified_rollback, install_persistent_os_daemon, plan_rollback, DeploySlot,
};
use crate::hud::render_hud;
use crate::mesh::{
    build_mesh_rtt_matrix, cached_local_public_ip_geo, default_arch, default_config_path,
    default_os, default_state_dir, monotonic_now_ms, node_capabilities_for, node_role_for,
    normalize_channel_name, now_unix, place_nodes, prune_stale_peers, save_mesh_config,
    validate_secret, Beat, BootStageStatus, MeshConfig, MeshControlCommand, ProbeReport,
    BEAT_INTERVAL_SECS, DEFAULT_CHANNEL_NAME, DEFAULT_PEER_TTL_SECS, SUMMARY_INTERVAL_SECS,
};
use crate::node::IrohTransportDiagnostics;
use crate::record::{
    paginate_kv_records, AlertRecord, EventRecord, KeyValueRecord, KvPageMeta, ServiceLogRecord,
};
use crate::service::{InstalledProgram, ProcessSpec, Service};
use crate::stats::{pct_or_unknown, sample_stats, Stats};
use crate::supervisor::{
    ensure_probe_program, probe_program_bytes, stage_program, Supervisor, PROBE_PROGRAM_NAME,
};

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
            id: "store".to_string(),
            title: "Content-Addressed Blob Store (iroh-blobs)".to_string(),
            status: "ready".to_string(),
            detail: format!("Store ready at {}", blob_dir.display()),
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
            title: "Gossip Mesh & Channel Secret Gate".to_string(),
            status: "ready".to_string(),
            detail:
                "Protected 'dialtone' channel (enter secret in Web UI or run 'dialtone join <SECRET>')"
                    .to_string(),
            updated_at_unix: now,
        },
    ]
}

#[derive(Clone, Debug, Serialize)]
pub struct NodeState {
    pub node: String,
    pub mode: String,
    pub listen: String,
    pub store_dir: PathBuf,
    #[serde(skip)]
    pub blobs: Blobs,
    #[serde(skip)]
    pub api_token: String,
    #[serde(default)]
    pub boot_stages: Vec<BootStageStatus>,
    #[serde(default)]
    pub mesh_config: MeshConfig,
    #[serde(skip)]
    pub control_tx: Option<tokio::sync::mpsc::UnboundedSender<MeshControlCommand>>,
    #[serde(skip)]
    pub store: Option<Store>,
    #[serde(skip)]
    pub supervisor: Arc<Mutex<Supervisor>>,
    pub iroh_runtime: IrohTransportDiagnostics,
    pub nodes: Vec<Beat>,
    #[serde(default)]
    pub mesh_rtt_ms: std::collections::BTreeMap<String, f64>,
    pub installed_programs: Vec<InstalledProgram>,
    pub processes: Vec<ProcessSpec>,
    pub deploy_slots: Vec<DeploySlot>,
    /// Devices connected to this node, refreshed by the Devices actor.
    pub devices: Vec<crate::devices::Device>,
    pub blob_fetches: Vec<BlobTransportReport>,
    pub service_endpoints: Vec<Service>,
    pub stats: Stats,
    pub events: Vec<EventRecord>,
    pub logs: Vec<ServiceLogRecord>,
    pub alerts: Vec<AlertRecord>,
    pub kv: Vec<KeyValueRecord>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kv_page: Option<KvPageMeta>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub probe: Option<ProbeReport>,
}

impl NodeState {
    /// This node's own entry in `nodes`, found by NodeId (a peer may reuse our name).
    pub fn self_entry_mut(&mut self) -> Option<&mut Beat> {
        let id = self.iroh_runtime.node_id.clone();
        let name = self.node.clone();
        self.nodes.iter_mut().find(|n| {
            if id.is_empty() {
                n.node == name
            } else {
                n.node_id == id
            }
        })
    }

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
                        source: "summary_beat".to_string(),
                        summary: format!(
                            "Node {} thrashing detected (score {:.1}): {}",
                            peer.node,
                            summary.thrashing_score,
                            summary.thrashing_reasons.join(", ")
                        ),
                        stats_ref: format!("dialtone://{}/stats/summary", peer.node),
                        last_seen_unix: now,
                        acknowledged_by: None,
                    });
                }
            }
            if let Some(pct) = peer.stats.disk_percent.filter(|p| *p >= 85.0) {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-disk", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "stats".to_string(),
                    summary: format!("Node {} disk usage high ({:.1}%)", peer.node, pct),
                    stats_ref: format!("dialtone://{}/stats/disk", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
            if let Some(pct) = peer.stats.cpu_percent.filter(|p| *p >= 85.0) {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-cpu", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "stats".to_string(),
                    summary: format!("Node {} CPU usage high ({:.1}%)", peer.node, pct),
                    stats_ref: format!("dialtone://{}/stats/cpu", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
            if let Some(pct) = peer.stats.mem_percent.filter(|p| *p >= 85.0) {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-mem", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "stats".to_string(),
                    summary: format!("Node {} memory usage high ({:.1}%)", peer.node, pct),
                    stats_ref: format!("dialtone://{}/stats/mem", peer.node),
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
                source: "stats".to_string(),
                summary: format!(
                    "Node {} stats nominal (CPU {}, MEM {}, DISK {})",
                    self.node,
                    pct_or_unknown(self.stats.cpu_percent),
                    pct_or_unknown(self.stats.mem_percent),
                    pct_or_unknown(self.stats.disk_percent)
                ),
                stats_ref: format!("dialtone://{}/stats", self.node),
                last_seen_unix: now,
                acknowledged_by: None,
            });
        }
        self.alerts = live_alerts;
    }

    pub fn reconcile_processes(&mut self) {
        let mut started_logs = Vec::new();
        let listen_kv = format!("APP_LISTEN={}", self.listen);
        let sup_arc = self.supervisor.clone();
        if let Ok(mut sup) = sup_arc.lock() {
            for spec in self.processes.iter_mut() {
                if spec.name == PROBE_PROGRAM_NAME
                    && !spec.env.iter().any(|kv| kv.starts_with("APP_LISTEN="))
                {
                    spec.env.push(listen_kv.clone());
                }
                if std::path::Path::new(&spec.path).exists() {
                    let prev_pid = spec.pid;
                    let _ = sup.reconcile(std::slice::from_mut(spec));
                    if spec.desired_state == "running" && spec.pid.is_some() && spec.pid != prev_pid
                    {
                        let msg = format!(
                            "Supervised blob process '{}' active (digest={}, pid={}, path={})",
                            spec.name,
                            spec.digest,
                            spec.pid.unwrap_or(0),
                            spec.path
                        );
                        println!("[SUPERVISOR] {msg}");
                        started_logs.push((spec.name.clone(), msg));
                    }
                } else {
                    spec.pid = None;
                    spec.status = Some("missing_program".to_string());
                }
            }
        }
        for (svc, msg) in started_logs {
            self.record_log(&svc, "INFO", "supervisor", msg);
        }
        let procs_snapshot = self.processes.clone();
        let probe_snapshot = self.probe.clone();
        if let Some(self_beat) = self.self_entry_mut() {
            self_beat.processes = procs_snapshot;
            if probe_snapshot.is_some() {
                self_beat.probe = probe_snapshot;
            }
            if let Some(local_geo) = cached_local_public_ip_geo() {
                self_beat.geo_location = Some(local_geo);
            }
        }
        self.mesh_rtt_ms = build_mesh_rtt_matrix(&self.nodes);
        place_nodes(&mut self.nodes, &self.mesh_rtt_ms);
    }

    /// Execute one end-to-end 5-capability verification cycle driven by the supervised process blob:
    /// 1. SHA-256 content-addressed blob storage & verification (`blobs_ok`)
    /// 2. Key-Value store write, read & pagination (`key_value_ok`)
    /// 3. Process supervision & process-intent planning (`processes_ok`)
    /// 4. Encrypted gossip mesh connectivity & QUIC transport audit (`mesh_connectivity_ok`)
    /// 5. A-to-B deployment slot rollback preview & verified slot flip (`ab_rollback_ok`)
    pub fn run_probe(&mut self) -> ProbeReport {
        let now = now_unix();
        let next_cycle = self
            .probe
            .as_ref()
            .map(|r| r.cycle.saturating_add(1))
            .unwrap_or(1);

        // 1. BLB: put and verify the probe program blob and this cycle's probe record blob in the store
        let exec_payload = probe_program_bytes();
        let (program_digest, store_path, exec_blob_ok) =
            match self.blobs.put_blocking(exec_payload, None) {
                Ok(sha) => {
                    let read_ok = self
                        .blobs
                        .get_blocking(&sha)
                        .map(|b| b == exec_payload)
                        .unwrap_or(false);
                    (sha, self.store_dir.to_string_lossy().to_string(), read_ok)
                }
                Err(_) => (String::new(), String::new(), false),
            };

        let blob_payload = format!("dialtone-probe-sha-blob-node={}", self.node);
        let (verified_digest, cycle_blob_ok) =
            match self.blobs.put_blocking(blob_payload.as_bytes(), None) {
                Ok(sha) => {
                    let read_ok = self
                        .blobs
                        .get_blocking(&sha)
                        .map(|b| b == blob_payload.as_bytes())
                        .unwrap_or(false);
                    (sha, read_ok)
                }
                Err(_) => (String::new(), false),
            };
        let blobs_ok = exec_blob_ok && cycle_blob_ok;

        // 2. Sub-Processes: load executable from SHA-256 blob store -> stage to run dir -> supervise & verify PID
        let staged_from_store_ok = if !program_digest.is_empty() {
            stage_program(&self.blobs, PROBE_PROGRAM_NAME, &program_digest).is_ok()
        } else {
            false
        };
        let _ = ensure_probe_program(&self.blobs, &mut self.processes, None);
        self.reconcile_processes();
        let probe_proc = self
            .processes
            .iter()
            .find(|p| p.name == PROBE_PROGRAM_NAME)
            .or_else(|| {
                self.processes
                    .iter()
                    .find(|p| p.status.as_deref() == Some("running"))
            })
            .cloned();
        let (program_name, program_digest, pid, staged_path, ticket, processes_ok) =
            if let Some(ref proc) = probe_proc {
                let running = proc.status.as_deref() == Some("running") && proc.pid.is_some();
                let ticket = proc
                    .env
                    .iter()
                    .find_map(|kv| kv.strip_prefix("DIALTONE_BLOB_TICKET="))
                    .unwrap_or("")
                    .to_string();
                (
                    proc.name.clone(),
                    proc.digest.clone(),
                    proc.pid,
                    proc.path.clone(),
                    ticket,
                    staged_from_store_ok && running,
                )
            } else {
                (
                    PROBE_PROGRAM_NAME.to_string(),
                    String::new(),
                    None,
                    String::new(),
                    String::new(),
                    false,
                )
            };

        // 3. Mesh Connectivity: verify bound QUIC endpoint & active gossip peer table
        let mesh_peers_seen = self.nodes.len();
        let mesh_connectivity_ok = !self.iroh_runtime.node_id.is_empty() && mesh_peers_seen >= 1;

        // 4. A-to-B Rollback: stage verified slot A & slot B blobs, plan rollback, and flip A <-> B
        let slot_a_bytes = format!("dialtone-slot-A-binary-{}", self.node);
        let slot_b_bytes = format!("dialtone-slot-B-binary-{}", self.node);
        let mut ab_rollback_ok = false;
        let mut active_slot = "A".to_string();
        let stage_slot = |bytes: &[u8], slot: &str| {
            let sha = self.blobs.put_blocking(bytes, None)?;
            stage_program(&self.blobs, slot, &sha)
        };
        if let (Ok((sha_a, path_a)), Ok((sha_b, path_b))) = (
            stage_slot(slot_a_bytes.as_bytes(), "probe-slot-a"),
            stage_slot(slot_b_bytes.as_bytes(), "probe-slot-b"),
        ) {
            let path_a_str = path_a.to_string_lossy().to_string();
            let path_b_str = path_b.to_string_lossy().to_string();
            if !self
                .deploy_slots
                .iter()
                .any(|s| s.name == "dialtone-probe-slot")
            {
                self.deploy_slots.push(DeploySlot {
                    name: "dialtone-probe-slot".to_string(),
                    active: true,
                    path: path_a_str.clone(),
                    digest: sha_a.clone(),
                    previous_path: Some(path_b_str.clone()),
                    previous_digest: Some(sha_b.clone()),
                    status: "active-A".to_string(),
                });
            }
            if let Some(slot) = self
                .deploy_slots
                .iter_mut()
                .find(|s| s.name == "dialtone-probe-slot")
            {
                if let Ok(plan) = plan_rollback(&self.node, slot) {
                    if plan.eligible
                        && plan.side_effect_free_preview
                        && execute_verified_rollback(slot).is_ok()
                    {
                        active_slot = if slot.digest == sha_a {
                            "A".to_string()
                        } else {
                            "B".to_string()
                        };
                        slot.status = format!("active-{active_slot}");
                        ab_rollback_ok = true;
                    }
                }
            }
        }

        // 5. Key-Value Store: write live probe status record & verify paginated read-back
        let kv_key = format!("probe/{}/status", self.node);
        let kv_val = json!({
            "node": self.node,
            "cycle": next_cycle,
            "sha_blobs": blobs_ok,
            "processes": processes_ok,
            "mesh_connectivity": mesh_connectivity_ok,
            "ab_rollback": ab_rollback_ok,
            "active_slot": active_slot,
            "pid": pid,
            "updated_at_unix": now
        });
        if let Some(existing_kv) = self.kv.iter_mut().find(|r| r.key == kv_key) {
            existing_kv.value = kv_val;
            existing_kv.updated_at_unix = now;
        } else {
            self.kv.push(KeyValueRecord {
                key: kv_key.clone(),
                value: kv_val,
                updated_at_unix: now,
            });
        }
        let key_value_ok = paginate_kv_records(&self.kv, true, Some(0), Some(64))
            .map(|(records, _)| records.iter().any(|r| r.key == kv_key))
            .unwrap_or(false);

        let passed_count = (blobs_ok as u8)
            + (key_value_ok as u8)
            + (processes_ok as u8)
            + (mesh_connectivity_ok as u8)
            + (ab_rollback_ok as u8);

        let report = ProbeReport {
            program_name,
            program_digest,
            pid,
            store_path,
            staged_path,
            ticket,
            cycle: next_cycle,
            updated_at_unix: now,
            passed_count,
            total_count: 5,
            blobs_ok,
            key_value_ok,
            processes_ok,
            mesh_connectivity_ok,
            ab_rollback_ok,
            active_slot,
            verified_digest,
            kv_key,
            mesh_peers_seen,
        };
        self.probe = Some(report.clone());
        if let Some(self_beat) = self.self_entry_mut() {
            self_beat.probe = Some(report.clone());
        }
        report
    }

    pub fn new_live(
        node: &str,
        mode: &str,
        listen: &str,
        blobs: Blobs,
        iroh_runtime: IrohTransportDiagnostics,
    ) -> Self {
        let store_dir = blobs.dir().to_path_buf();
        let now = now_unix();
        let stats = sample_stats(node, iroh_runtime.bound_non_tailnet_ipv4.as_deref(), 1);
        let service_endpoints = vec![Service {
            name: "dialtone-control".to_string(),
            kind: "http-control-plane".to_string(),
            protocol: "http".to_string(),
            url: format!("http://{listen}"),
            listen: listen.to_string(),
            status: "running".to_string(),
            advertised_by: node.to_string(),
            role: mode.to_string(),
            mesh: "dialtone-iroh-gossip".to_string(),
            subjects: vec!["dialtone.iroh.beat.v1".to_string()],
        }];

        let self_beat = Beat {
            schema: "dialtone.iroh.beat.v1".to_string(),
            node: node.to_string(),
            node_id: iroh_runtime.node_id.clone(),
            ticket: format!("iroh-gossip-v1:{node}"),
            counter: 1,
            updated_at_unix: now,
            kind: "summary".to_string(),
            beat_interval_secs: BEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_INTERVAL_SECS,
            sent_mono_ms: monotonic_now_ms(),
            rtt_probe_requested: true,
            peer_echoes: HashMap::new(),
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            sent_unix_ms: 0,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for(node),
            capabilities: node_capabilities_for(node),
            stats: stats.clone(),
            service_endpoints: service_endpoints.clone(),
            processes: Vec::new(),
            iroh_transport: iroh_runtime.clone(),
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            devices: Vec::new(),
            geo_location: cached_local_public_ip_geo(),
            probe: None,
            store: None,
        };

        let mut installed_programs = Vec::new();
        if let Ok(exe_path) = std::env::current_exe() {
            if let Ok(bytes) = fs::read(&exe_path) {
                let digest = crate::blob::digest_of(&bytes);
                installed_programs.push(InstalledProgram {
                    name: "dialtone".to_string(),
                    path: exe_path.to_string_lossy().to_string(),
                    digest,
                });
            }
        }

        let boot_stages = default_boot_stages(listen, &store_dir);
        let mesh_config = MeshConfig {
            node_name: Some(node.to_string()),
            listen_addr: Some(listen.to_string()),
            channel: DEFAULT_CHANNEL_NAME.to_string(),
            configured: false,
            secret_key: None,
            derived_keys: None,
            channel_fingerprint: "unconfigured".to_string(),
            invites: Vec::new(),
            seeds: Vec::new(),
            allowlist: Vec::new(),
            used_invite_nonces: Vec::new(),
            daemon_installed: false,
            daemon_service_path: None,
            processes: Vec::new(),
            updated_at_unix: now,
        };
        let api_token = load_or_create_api_token().unwrap_or_else(|_| "dtk-live-token".to_string());

        let mut st = Self {
            node: node.to_string(),
            mode: mode.to_string(),
            listen: listen.to_string(),
            api_token,
            store_dir,
            blobs,
            boot_stages,
            mesh_config,
            control_tx: None,
            store: None,
            supervisor: Arc::new(Mutex::new(Supervisor::new())),
            iroh_runtime,
            nodes: vec![self_beat],
            mesh_rtt_ms: std::collections::BTreeMap::new(),
            installed_programs,
            processes: Vec::new(),
            deploy_slots: Vec::new(),
            devices: Vec::new(),
            blob_fetches: Vec::new(),
            service_endpoints,
            stats: stats.clone(),
            events: Vec::new(),
            logs: Vec::new(),
            alerts: Vec::new(),
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
                    key: "mesh/beat_interval_secs".to_string(),
                    value: json!(BEAT_INTERVAL_SECS),
                    updated_at_unix: now,
                },
                KeyValueRecord {
                    key: "mesh/summary_interval_secs".to_string(),
                    value: json!(SUMMARY_INTERVAL_SECS),
                    updated_at_unix: now,
                },
            ],
            kv_page: None,
            probe: None,
        };
        st.record_event(
            "mesh.boot",
            "info",
            format!(
                "dialtone live mesh daemon initialized on {} ({}/{}, {} cores, {} mem, {} disk)",
                node,
                default_os(),
                default_arch(),
                stats
                    .cpu_cores
                    .map(|c| c.to_string())
                    .unwrap_or_else(|| "?".to_string()),
                pct_or_unknown(stats.mem_percent),
                pct_or_unknown(stats.disk_percent)
            ),
        );
        st.record_log(
            "dialtone-daemon",
            "INFO",
            "stdout",
            format!(
                "started live stats & gossip runtime on http://{} (beat={}s, summary={}s, cpu={}, mem={}, disk={})",
                listen,
                BEAT_INTERVAL_SECS,
                SUMMARY_INTERVAL_SECS,
                pct_or_unknown(stats.cpu_percent),
                pct_or_unknown(stats.mem_percent),
                pct_or_unknown(stats.disk_percent)
            ),
        );
        place_nodes(&mut st.nodes, &st.mesh_rtt_ms);
        st.refresh_live_alerts();
        st
    }
}

pub(crate) fn masked_mesh_config(cfg: &MeshConfig) -> MeshConfig {
    let mut c = cfg.clone();
    c.secret_key = None;
    c.derived_keys = None;
    c
}

pub fn serve_http_blocking(
    listener: TcpListener,
    state: Arc<Mutex<NodeState>>,
    max_requests: Option<usize>,
) -> Result<(), String> {
    let active_processes = Arc::new(AtomicUsize::new(0));
    let mut handled = 0usize;
    let mut join_handles = Vec::new();

    for stream in listener.incoming() {
        let mut stream = stream.map_err(|e| format!("accept connection: {e}"))?;
        if active_processes.load(Ordering::SeqCst) >= MAX_CONCURRENT_HTTP_CONNECTIONS {
            let _ = write_json(
                &mut stream,
                503,
                &json!({ "error": "too many concurrent HTTP connections" }),
            );
            continue;
        }
        active_processes.fetch_add(1, Ordering::SeqCst);
        let state_clone = state.clone();
        let active_clone = active_processes.clone();
        let process = thread::spawn(move || {
            let _ = handle_connection(stream, &state_clone);
            active_clone.fetch_sub(1, Ordering::SeqCst);
        });
        if max_requests.is_some() {
            join_handles.push(process);
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

/// Largest request head (request line + headers) the API accepts.
pub const MAX_HTTP_HEAD_BYTES: usize = 64 * 1024;
/// Most headers a request may carry.
pub const MAX_HTTP_HEADERS: usize = 64;

/// One parsed API request. Header names are lower-case; the query is percent-decoded.
struct Request {
    method: String,
    path: String,
    query: HashMap<String, String>,
    headers: HashMap<String, String>,
    body: String,
}

type HttpError = (u16, String);

fn bad_request(msg: impl Into<String>) -> HttpError {
    (400, msg.into())
}

/// Read and parse one request with httparse. Rejects what the API never needs instead of
/// guessing: chunked or any other `Transfer-Encoding`, duplicate or conflicting
/// `Content-Length`, bodies cut short, non-UTF-8 bodies and oversize heads or bodies.
fn read_request<R: Read>(stream: &mut R) -> Result<Option<Request>, HttpError> {
    let mut buf = Vec::with_capacity(8192);
    let mut chunk = [0u8; 8192];
    loop {
        let mut header_slots = [httparse::EMPTY_HEADER; MAX_HTTP_HEADERS];
        let mut parsed = httparse::Request::new(&mut header_slots);
        let status = match parsed.parse(&buf) {
            Ok(status) => status,
            Err(httparse::Error::TooManyHeaders) => {
                return Err((431, format!("more than {MAX_HTTP_HEADERS} headers")))
            }
            Err(e) => return Err(bad_request(format!("malformed request: {e}"))),
        };
        if let httparse::Status::Complete(head_len) = status {
            if head_len > MAX_HTTP_HEAD_BYTES {
                return Err((431, "HTTP header exceeds 64 KiB limit".to_string()));
            }
            let method = parsed.method.unwrap_or("GET").to_string();
            let target = parsed.path.unwrap_or("/");
            let (path, query_str) = target.split_once('?').unwrap_or((target, ""));
            let path = path.to_string();
            let query = parse_query(query_str);

            let mut headers = HashMap::new();
            let mut content_length: Option<usize> = None;
            for h in parsed.headers.iter() {
                let name = h.name.to_ascii_lowercase();
                let value = std::str::from_utf8(h.value)
                    .map_err(|_| bad_request(format!("header {name} is not UTF-8")))?
                    .trim()
                    .to_string();
                match name.as_str() {
                    "transfer-encoding" => {
                        return Err((501, "Transfer-Encoding is not supported".to_string()))
                    }
                    "content-length" => {
                        let n = value
                            .parse::<usize>()
                            .map_err(|_| bad_request("invalid Content-Length"))?;
                        if content_length.is_some_and(|prev| prev != n) {
                            return Err(bad_request("conflicting Content-Length headers"));
                        }
                        content_length = Some(n);
                    }
                    _ => {}
                }
                headers.insert(name, value);
            }

            let content_length = content_length.unwrap_or(0);
            if content_length > MAX_HTTP_BODY_BYTES {
                return Err((
                    413,
                    format!(
                        "HTTP request body exceeds maximum size of {MAX_HTTP_BODY_BYTES} bytes"
                    ),
                ));
            }
            let mut body = buf.split_off(head_len);
            while body.len() < content_length {
                let to_read = (content_length - body.len()).min(chunk.len());
                let n = stream
                    .read(&mut chunk[..to_read])
                    .map_err(|e| bad_request(e.to_string()))?;
                if n == 0 {
                    return Err(bad_request("request body shorter than Content-Length"));
                }
                body.extend_from_slice(&chunk[..n]);
            }
            body.truncate(content_length);
            let body = String::from_utf8(body).map_err(|_| bad_request("body is not UTF-8"))?;
            return Ok(Some(Request {
                method,
                path,
                query,
                headers,
                body,
            }));
        }

        if buf.len() >= MAX_HTTP_HEAD_BYTES {
            return Err((431, "HTTP header exceeds 64 KiB limit".to_string()));
        }
        let n = stream
            .read(&mut chunk)
            .map_err(|e| bad_request(e.to_string()))?;
        if n == 0 {
            if buf.is_empty() {
                return Ok(None);
            }
            return Err(bad_request("connection closed mid-request"));
        }
        buf.extend_from_slice(&chunk[..n]);
    }
}

fn handle_connection(mut stream: TcpStream, state: &Arc<Mutex<NodeState>>) -> Result<(), String> {
    let _ = stream.set_read_timeout(Some(HTTP_IO_TIMEOUT));
    let _ = stream.set_write_timeout(Some(HTTP_IO_TIMEOUT));

    let req = match read_request(&mut stream) {
        Ok(Some(req)) => req,
        Ok(None) => return Ok(()),
        Err((status, msg)) => return write_json(&mut stream, status, &json!({ "error": msg })),
    };
    let (method, path) = (req.method.as_str(), req.path.as_str());

    if let Err(err) = validate_local_http_request(method, &req.headers) {
        return write_json(&mut stream, 403, &json!({ "error": err }));
    }

    // The HUD's static files need no node state.
    if method.eq_ignore_ascii_case("GET") {
        if let Some((content_type, body, cache)) = crate::hud::hud_asset(path) {
            let headers = format!("Cache-Control: {cache}\r\nX-Content-Type-Options: nosniff\r\n");
            return write_response_with_headers(
                &mut stream,
                200,
                content_type,
                &headers,
                &body,
                false,
            );
        }
    }

    // Routes write their response into a buffer while they hold the node-state lock; the
    // socket write happens after the lock is released, so a slow client never stalls the mesh.
    let mut out = Vec::new();
    let routed = route_request(
        &mut out,
        method,
        path,
        &req.query,
        &req.headers,
        &req.body,
        state,
    );
    stream.write_all(&out).map_err(|e| e.to_string())?;
    routed
}

#[allow(clippy::too_many_arguments)]
fn route_request(
    mut stream: &mut Vec<u8>,
    method: &str,
    path: &str,
    query: &HashMap<String, String>,
    headers: &HashMap<String, String>,
    body_part: &str,
    state: &Arc<Mutex<NodeState>>,
) -> Result<(), String> {
    let mut st = state.lock().map_err(|_| "state poisoned".to_string())?;
    if method.eq_ignore_ascii_case("POST") {
        if let Err(err) = validate_bearer_token(headers, &st.api_token) {
            return write_json(&mut stream, 401, &json!({ "error": err }));
        }
    }
    let now = now_unix();
    if let Some(self_beat) = st.self_entry_mut() {
        self_beat.updated_at_unix = now;
    }
    prune_stale_peers(&mut st.nodes, now, DEFAULT_PEER_TTL_SECS);

    match (method, path) {
        ("GET", "/") => {
            st.reconcile_processes();
            let nonce = crate::hex_encode(&iroh::SecretKey::generate().to_bytes()[..16]);
            let html = render_hud(&st, query.get("peer-id").map(String::as_str), &nonce);
            write_response_with_headers(
                &mut stream,
                200,
                "text/html; charset=utf-8",
                &hud_security_headers(&nonce),
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
                "store_dir": st.store_dir,
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config),
                "iroh_runtime": st.iroh_runtime
            });
            write_json(&mut stream, 200, &body)
        }
        ("POST", "/api/mesh/join") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let channel = normalize_channel_name(parsed.get("channel").and_then(Value::as_str));
            let raw_secret = parsed
                .get("secret")
                .or_else(|| parsed.get("channel_secret"))
                .or_else(|| parsed.get("secret_key"))
                .or_else(|| parsed.get("secret"))
                .or_else(|| parsed.get("key"))
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim()
                .to_string();
            let raw_ticket = parsed
                .get("invite")
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

            let validated_secret = match validate_secret(&raw_secret) {
                Ok(p) => p,
                Err(err) => {
                    return write_json(&mut stream, 400, &json!({ "error": err }));
                }
            };

            if let Some(ref nn) = new_node_name {
                st.node = nn.clone();
            }

            let keys = st.mesh_config.apply_secret(&channel, &validated_secret);
            let fp = keys.fingerprint();
            let topic_id = keys.topic_id;

            if raw_ticket.starts_with(crate::mesh::INVITE_PREFIX) {
                let parsed_invite = match raw_ticket.parse::<crate::mesh::Invite>() {
                    Ok(inv) => inv,
                    Err(err) => return write_json(&mut stream, 400, &json!({ "error": err })),
                };
                let self_ep = st.iroh_runtime.node_id.clone();
                if let Err(err) = crate::mesh::redeem_invite(
                    &parsed_invite,
                    &keys,
                    &self_ep,
                    now,
                    &mut st.mesh_config,
                ) {
                    return write_json(&mut stream, 400, &json!({ "error": err }));
                }
            } else if !raw_ticket.is_empty() && !st.mesh_config.invites.contains(&raw_ticket) {
                st.mesh_config.invites.push(raw_ticket.clone());
            }

            let tickets = st.mesh_config.invites.clone();
            st.mesh_config.node_name = Some(st.node.clone());
            st.mesh_config.listen_addr = Some(st.listen.clone());
            st.mesh_config.updated_at_unix = now;
            let _ = save_mesh_config(&default_config_path(), &st.mesh_config);

            if let Some(stage5) = st.boot_stages.iter_mut().find(|s| s.step == 5) {
                stage5.status = "ready".to_string();
                stage5.detail = format!("Joined secret-protected channel '{}' ({fp})", channel);
                stage5.updated_at_unix = now;
            }

            let node_src = st.node.clone();
            st.events.push(EventRecord {
                id: format!("evt-join-{now}"),
                kind: "mesh.join".to_string(),
                severity: "info".to_string(),
                source: node_src,
                message: format!(
                    "Joined secret-protected gossip channel '{}' ({fp})",
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
                    invites: tickets,
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
        ("GET", "/api/state") => {
            st.reconcile_processes();
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
        ("GET", "/api/devices") => {
            let body = json!({ "node": st.node, "devices": st.devices });
            write_json(&mut stream, 200, &body)
        }
        ("GET", "/api/mesh") => {
            st.reconcile_processes();
            let summary = st.blobs.summary();
            let body = json!({
                "schema": "dialtone.gossip.v1",
                "mesh": "dialtone-iroh-gossip",
                "self_node": st.node,
                "beat_interval_secs": BEAT_INTERVAL_SECS,
                "summary_interval_secs": SUMMARY_INTERVAL_SECS,
                "self_iroh_runtime": st.iroh_runtime,
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config),
                "blob_stats": {
                    "blob_keys": summary.blob_keys,
                    "blob_bytes": summary.blob_bytes,
                    "blob_fetches": st.blob_fetches.len(),
                    "kv_keys": st.kv.len(),
                    "installed_programs": st.installed_programs.len()
                },
                "nodes": st.nodes,
                "mesh_rtt_ms": st.mesh_rtt_ms,
                "processes": st.processes,
                "service_endpoints": st.service_endpoints
            });
            write_json(&mut stream, 200, &body)
        }
        ("HEAD", "/api/blob") => {
            let sha = query.get("digest").cloned().unwrap_or_default();
            let blobs = st.blobs.clone();
            drop(st);
            if blobs.has_blocking(&sha) {
                write_response(&mut stream, 200, "application/octet-stream", b"", true)
            } else {
                write_response(&mut stream, 404, "application/octet-stream", b"", true)
            }
        }
        ("GET", "/api/blob") => {
            let sha = query.get("digest").cloned().unwrap_or_default();
            let blobs = st.blobs.clone();
            drop(st);
            match blobs.get_blocking(&sha) {
                Ok(bytes) => {
                    write_response(&mut stream, 200, "application/octet-stream", &bytes, false)
                }
                Err(err) => write_json(&mut stream, 404, &json!({ "error": err })),
            }
        }
        ("POST", "/api/blob") | ("POST", "/api/blob/share") => {
            let parsed: Value = serde_json::from_str(body_part).unwrap_or_else(|_| json!({}));
            let b64 = parsed
                .get("bytes_base64")
                .and_then(Value::as_str)
                .unwrap_or_default();
            let expected_sha = parsed
                .get("digest")
                .and_then(Value::as_str)
                .map(String::from);
            match crate::base64_decode(b64) {
                Ok(raw_bytes) => {
                    let maybe_bridge = st.store.clone();
                    let blobs = st.blobs.clone();
                    let node_name = st.node.clone();
                    drop(st);
                    if let Some(bridge) = maybe_bridge {
                        match bridge.share_blob(&raw_bytes, expected_sha.as_deref()) {
                            Ok(rep) => write_json(
                                &mut stream,
                                200,
                                &json!({
                                    "status": rep.status,
                                    "node": node_name,
                                    "digest": rep.digest,
                                    "blake3_hash": rep.blake3_hash,
                                    "bytes": rep.bytes,
                                    "provider_node_id": rep.provider_node_id,
                                    "ticket_uri": rep.ticket_uri
                                }),
                            ),
                            Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                        }
                    } else {
                        match blobs.put_blocking(&raw_bytes, expected_sha.as_deref()) {
                            Ok(sha) => write_json(
                                &mut stream,
                                200,
                                &json!({ "digest": sha, "bytes": raw_bytes.len() }),
                            ),
                            Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                        }
                    }
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err.to_string() })),
            }
        }
        ("POST", "/api/blob/pull") => {
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
            let Some(bridge) = st.store.clone() else {
                return write_json(
                    &mut stream,
                    503,
                    &json!({ "error": "live iroh-blobs bridge is not initialized on this node" }),
                );
            };
            drop(st);
            match bridge.pull_blob(&ticket_uri, force) {
                Ok(report) => {
                    if let Ok(mut st_after) = state.lock() {
                        st_after.blob_fetches.push(report.clone());
                    }
                    write_json(&mut stream, 200, &report)
                }
                Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
            }
        }
        ("GET", "/api/processes") => {
            st.reconcile_processes();
            write_json(
                &mut stream,
                200,
                &json!({
                    "schema": "dialtone.processes.v1",
                    "node": st.node,
                    "processes": st.processes
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
            if desired_state == "removed" {
                return remove_process(&mut stream, &mut st, &name);
            }
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
            let mut env_vars: Vec<String> = parsed
                .get("env")
                .and_then(Value::as_array)
                .map(|arr| {
                    arr.iter()
                        .filter_map(Value::as_str)
                        .map(String::from)
                        .collect()
                })
                .unwrap_or_default();
            let deploy_to_mesh = parsed
                .get("deploy_to_mesh")
                .and_then(Value::as_bool)
                .unwrap_or_else(|| {
                    parsed.get("scope").and_then(Value::as_str) == Some("mesh")
                        || env_vars.iter().any(|kv| kv == "DIALTONE_MESH_DEPLOY=1")
                });
            let cwd = parsed
                .get("cwd")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(String::from);

            // Blob work (disk and network) runs without the node-state lock held.
            let bridge = st.store.clone();
            let blobs = st.blobs.clone();
            let known_digest = st
                .processes
                .iter()
                .find(|p| p.name == name)
                .map(|p| p.digest.clone());
            drop(st);
            let mut pulled = None;

            let mut digest = parsed
                .get("digest")
                .and_then(Value::as_str)
                .unwrap_or("")
                .trim()
                .to_string();
            let mut resolved_ticket_uri = parsed
                .get("ticket_uri")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|s| !s.is_empty())
                .map(String::from);

            if let Some(b64) = parsed
                .get("bytes_base64")
                .and_then(Value::as_str)
                .filter(|s| !s.trim().is_empty())
            {
                let raw = match crate::base64_decode(b64) {
                    Ok(b) => b,
                    Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                };
                let exp = if digest.is_empty() {
                    None
                } else {
                    Some(digest.as_str())
                };
                if let Some(ref bridge) = bridge {
                    match bridge.share_blob(&raw, exp) {
                        Ok(rep) => {
                            digest = rep.digest;
                            resolved_ticket_uri = Some(rep.ticket_uri);
                        }
                        Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                    }
                } else {
                    match blobs.put_blocking(&raw, exp) {
                        Ok(verified_sha) => digest = verified_sha,
                        Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                    }
                }
            }

            if let Some(ref t_uri) = resolved_ticket_uri {
                if digest.is_empty() || !blobs.has_blocking(&digest) {
                    if let Some(ref bridge) = bridge {
                        match bridge.pull_blob(t_uri, false) {
                            Ok(rep) => {
                                digest = rep.digest.clone();
                                pulled = Some(rep);
                            }
                            Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                        }
                    }
                }
            }

            if digest.is_empty() {
                if let Some(known) = known_digest {
                    digest = known;
                }
            }
            if digest.is_empty() {
                return write_json(
                    &mut stream,
                    400,
                    &json!({ "error": "digest, bytes_base64, or ticket_uri is required to configure a blob process" }),
                );
            }

            if deploy_to_mesh && resolved_ticket_uri.is_none() {
                if let (Some(ref bridge), Ok(cached_bytes)) = (&bridge, blobs.get_blocking(&digest))
                {
                    if let Ok(rep) = bridge.share_blob(&cached_bytes, Some(&digest)) {
                        resolved_ticket_uri = Some(rep.ticket_uri);
                    }
                }
            }
            if deploy_to_mesh && !env_vars.iter().any(|kv| kv == "DIALTONE_MESH_DEPLOY=1") {
                env_vars.push("DIALTONE_MESH_DEPLOY=1".to_string());
            }
            if let Some(ref t_uri) = resolved_ticket_uri {
                env_vars.retain(|kv| !kv.starts_with("DIALTONE_BLOB_TICKET="));
                env_vars.push(format!("DIALTONE_BLOB_TICKET={t_uri}"));
            }

            let mut st = state.lock().map_err(|_| "state poisoned".to_string())?;
            if let Some(rep) = pulled {
                st.blob_fetches.push(rep);
            }
            let (verified_sha, staged_path) = match stage_program(&blobs, &name, &digest) {
                Ok(v) => v,
                Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
            };

            let prev_restart_count = st
                .processes
                .iter()
                .find(|p| p.name == name && p.digest == verified_sha)
                .map(|p| p.restart_count)
                .unwrap_or(0);

            let mut spec = ProcessSpec {
                name: name.clone(),
                path: staged_path.to_string_lossy().to_string(),
                digest: verified_sha.clone(),
                desired_state: desired_state.clone(),
                args,
                env: env_vars,
                cwd,
                startup_grace_secs: Some(2),
                status: None,
                pid: None,
                restart_count: prev_restart_count,
                service_unit: None,
                deploy_ticket: None,
            };

            let sup_res = {
                let sup_arc = st.supervisor.clone();
                let mut sup = sup_arc.lock().unwrap();
                sup.reconcile(std::slice::from_mut(&mut spec))
            };
            if let Err(err) = sup_res {
                return write_json(&mut stream, 400, &json!({ "error": err }));
            }

            if let Some(existing) = st.processes.iter_mut().find(|p| p.name == name) {
                *existing = spec.clone();
            } else {
                st.processes.push(spec.clone());
            }

            if let Some(existing_cfg) = st.mesh_config.processes.iter_mut().find(|p| p.name == name)
            {
                *existing_cfg = spec.clone();
            } else {
                st.mesh_config.processes.push(spec.clone());
            }
            st.mesh_config.updated_at_unix = now_unix();
            let _ = save_mesh_config(&default_config_path(), &st.mesh_config);

            let procs_snapshot = st.processes.clone();
            if let Some(self_beat) = st.self_entry_mut() {
                self_beat.processes = procs_snapshot;
            }

            println!(
                "[SUPERVISOR] Configured blob process '{}' (digest={}, desired_state={}, status={:?}, pid={:?})",
                spec.name, spec.digest, spec.desired_state, spec.status, spec.pid
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
        ("POST", "/api/probe") | ("GET", "/api/probe") => {
            let report = if method == "POST" || st.probe.is_none() {
                st.run_probe()
            } else {
                st.probe.clone().unwrap_or_default()
            };
            write_json(
                &mut stream,
                200,
                &json!({
                    "status": "ok",
                    "node": st.node,
                    "probe": report,
                    "deploy_slots": st.deploy_slots
                }),
            )
        }
        _ => write_json(
            &mut stream,
            404,
            &json!({ "error": "not found", "path": path }),
        ),
    }
}

/// Stop a process and forget it: drop its spec from the node state and `config.json`, and
/// delete its staged program file when no other spec runs it.
fn remove_process<W: Write>(stream: &mut W, st: &mut NodeState, name: &str) -> Result<(), String> {
    let in_config = st.mesh_config.processes.iter().any(|p| p.name == name);
    let Some(mut spec) = st
        .processes
        .iter()
        .position(|p| p.name == name)
        .map(|i| st.processes.remove(i))
        .or_else(|| {
            st.mesh_config
                .processes
                .iter()
                .find(|p| p.name == name)
                .cloned()
        })
    else {
        return write_json(
            stream,
            404,
            &json!({ "error": format!("no process named {name:?}") }),
        );
    };
    spec.desired_state = "removed".to_string();
    if let Ok(mut sup) = st.supervisor.lock() {
        let _ = sup.reconcile(std::slice::from_mut(&mut spec));
    }
    if in_config {
        st.mesh_config.processes.retain(|p| p.name != name);
        st.mesh_config.updated_at_unix = now_unix();
        let _ = save_mesh_config(&default_config_path(), &st.mesh_config);
    }
    let run_dir = st.store_dir.parent().unwrap_or(&st.store_dir).join("run");
    let staged = std::path::Path::new(&spec.path);
    if staged.starts_with(&run_dir) && !st.processes.iter().any(|p| p.path == spec.path) {
        let _ = fs::remove_file(staged);
    }
    let procs_snapshot = st.processes.clone();
    if let Some(me) = st.self_entry_mut() {
        me.processes = procs_snapshot;
    }
    st.record_log(
        "supervisor",
        "INFO",
        "stdout",
        format!("removed process '{name}'"),
    );
    write_json(
        stream,
        200,
        &json!({ "status": "removed", "node": st.node, "process": name }),
    )
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
        return ip.is_loopback() || crate::node::is_tailnet_ip(ip);
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

/// Percent-decode a query string (`+` is a space). A repeated key keeps its last value.
fn parse_query(query: &str) -> HashMap<String, String> {
    form_urlencoded::parse(query.as_bytes())
        .map(|(k, v)| (k.into_owned(), v.into_owned()))
        .collect()
}

fn write_json<W: Write, T: Serialize>(
    stream: &mut W,
    status: u16,
    value: &T,
) -> Result<(), String> {
    let body = serde_json::to_vec(value).map_err(|e| e.to_string())?;
    write_response(stream, status, "application/json", &body, false)
}

/// Headers for the HUD page. The CSP lets only the page's own nonce-tagged scripts and the
/// HUD's own files (served by this node, three.js / h3-js vendored) run, so markup a peer manages to get into the
/// page can never execute; the page may only talk back to this API.
fn hud_security_headers(nonce: &str) -> String {
    format!(
        "Content-Security-Policy: default-src 'none'; script-src 'nonce-{nonce}' 'self'; \
         style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; \
         base-uri 'none'; form-action 'none'; frame-ancestors 'none'\r\n\
         X-Content-Type-Options: nosniff\r\n\
         Referrer-Policy: no-referrer\r\n\
         X-Frame-Options: DENY\r\n"
    )
}

fn write_response<W: Write>(
    stream: &mut W,
    status: u16,
    content_type: &str,
    body: &[u8],
    head_only: bool,
) -> Result<(), String> {
    write_response_with_headers(stream, status, content_type, "", body, head_only)
}

/// `extra_headers` is a block of complete `Name: value\r\n` lines.
fn write_response_with_headers<W: Write>(
    stream: &mut W,
    status: u16,
    content_type: &str,
    extra_headers: &str,
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
        "HTTP/1.1 {status} {reason}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\n{extra_headers}Connection: close\r\n\r\n",
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
    use crate::node::{build_diagnostics, IrohTransportConfig};
    use iroh::{EndpointAddr, SecretKey};
    use std::env;
    use std::thread;

    #[test]
    fn hud_page_runs_only_nonce_scripts_and_never_embeds_the_api_token() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-hud-csp-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&IrohTransportConfig::local_offline(), &ep_addr, None, "n1");
        let mut node = NodeState::new_live(
            "node-alpha",
            "hub",
            &addr.to_string(),
            Blobs::open_blocking(&tmp_dir).unwrap(),
            diag,
        );
        node.api_token = "dtk-must-never-appear-in-html-0123456789".to_string();
        let state = Arc::new(Mutex::new(node));
        let state_bg = state.clone();
        let handle = thread::spawn(move || {
            serve_http_blocking(listener, state_bg, Some(1)).unwrap();
        });

        let resp = raw_http_request(
            addr,
            "GET /?peer-id=%3C/script%3E%3Cscript%3Ealert(1)%3C/script%3E HTTP/1.1\r\nHost: localhost\r\n\r\n",
        );
        handle.join().unwrap();
        let (head, body) = resp.split_once("\r\n\r\n").unwrap();
        let csp = head
            .lines()
            .find(|l| l.starts_with("Content-Security-Policy:"))
            .expect("HUD must send a CSP");
        let nonce = csp
            .split("'nonce-")
            .nth(1)
            .unwrap()
            .split('\'')
            .next()
            .unwrap();
        assert!(nonce.len() >= 16);
        assert!(csp.contains("default-src 'none'") && !csp.contains("unsafe-eval"));
        assert!(!csp.contains("jsdelivr"), "the HUD loads no CDN code");
        // importmap, h3-js and app.js carry the nonce; the rest are inert JSON data blocks.
        assert_eq!(body.matches(&format!("nonce=\"{nonce}\"")).count(), 3);
        for tag in body.split("<script").skip(1) {
            let open = tag.split('>').next().unwrap();
            assert!(
                open.contains(&format!("nonce=\"{nonce}\""))
                    || open.contains("type=\"application/json\""),
                "script without nonce: <script{open}>"
            );
        }
        assert!(!body.contains("cdn.jsdelivr"));
        assert!(!body.contains("dtk-must-never-appear-in-html"));
        assert!(!body.contains("__API_TOKEN__") && !body.contains("__CSP_NONCE__"));
        assert!(head.contains("X-Content-Type-Options: nosniff"));
        let _ = std::fs::remove_dir_all(&tmp_dir);
    }

    fn parse(raw: &str) -> Result<Option<Request>, HttpError> {
        read_request(&mut std::io::Cursor::new(raw.as_bytes().to_vec()))
    }

    #[test]
    fn request_parser_decodes_queries_and_reads_exact_bodies() {
        let req = parse(
            "POST /api/x?peer-id=%3Cnode%20a%3E&q=a+b HTTP/1.1\r\nHost: h\r\nContent-Length: 4\r\n\r\nbodyEXTRA",
        )
        .unwrap()
        .unwrap();
        assert_eq!((req.method.as_str(), req.path.as_str()), ("POST", "/api/x"));
        assert_eq!(req.query["peer-id"], "<node a>");
        assert_eq!(req.query["q"], "a b");
        assert_eq!(req.headers["host"], "h");
        assert_eq!(req.body, "body");
        assert!(
            parse("").unwrap().is_none(),
            "an idle connection is not an error"
        );
    }

    #[test]
    fn request_parser_rejects_ambiguous_or_oversize_requests() {
        let status = |raw: &str| parse(raw).err().map(|(code, _)| code);
        let chunked = "POST / HTTP/1.1\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n";
        assert_eq!(status(chunked), Some(501));
        let conflicting = "POST / HTTP/1.1\r\nContent-Length: 1\r\nContent-Length: 2\r\n\r\nab";
        assert_eq!(status(conflicting), Some(400));
        assert_eq!(
            status("POST / HTTP/1.1\r\nContent-Length: x\r\n\r\n"),
            Some(400)
        );
        assert_eq!(
            status("POST / HTTP/1.1\r\nContent-Length: 9\r\n\r\nshort"),
            Some(400)
        );
        assert_eq!(
            status("GET / HTTP/1.1\r\nHost: h\r\n"),
            Some(400),
            "head cut short"
        );
        assert_eq!(status("GET /\x01 HTTP/1.1\r\n\r\n"), Some(400));
        let huge = format!(
            "POST / HTTP/1.1\r\nContent-Length: {}\r\n\r\n",
            MAX_HTTP_BODY_BYTES + 1
        );
        assert_eq!(status(&huge), Some(413));
        let many: String = (0..=MAX_HTTP_HEADERS)
            .map(|i| format!("X-{i}: v\r\n"))
            .collect();
        assert_eq!(status(&format!("GET / HTTP/1.1\r\n{many}\r\n")), Some(431));
        let long = format!(
            "GET / HTTP/1.1\r\nX: {}\r\n\r\n",
            "a".repeat(MAX_HTTP_HEAD_BYTES)
        );
        assert_eq!(status(&long), Some(431));
    }

    #[test]
    fn hud_static_files_are_served_with_their_types_and_cache_policy() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-hud-assets-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&IrohTransportConfig::local_offline(), &ep_addr, None, "n1");
        let node = NodeState::new_live(
            "node-alpha",
            "hub",
            &addr.to_string(),
            Blobs::open_blocking(&tmp_dir).unwrap(),
            diag,
        );
        let state = Arc::new(Mutex::new(node));
        let handle = thread::spawn(move || {
            serve_http_blocking(listener, state, Some(4)).unwrap();
        });
        let get = |path: &str| {
            raw_http_request(
                addr,
                &format!("GET {path} HTTP/1.1\r\nHost: localhost\r\n\r\n"),
            )
        };
        let app = get("/hud/app.js");
        assert!(app.starts_with("HTTP/1.1 200"));
        assert!(app.contains("Content-Type: text/javascript") && app.contains("no-cache"));
        let three = get("/hud/vendor/three@0.179.1/three.module.min.js");
        assert!(three.contains("immutable") && three.contains("nosniff"));
        let css = get("/hud/style.css");
        assert!(css.contains("Content-Type: text/css"));
        assert!(get("/hud/../Cargo.toml").starts_with("HTTP/1.1 404"));
        handle.join().unwrap();
        let _ = std::fs::remove_dir_all(&tmp_dir);
    }

    #[test]
    fn http_server_serves_hud_and_state_and_rejects_cross_origin_posts() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-http-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();

        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-123");

        let state = Arc::new(Mutex::new(NodeState::new_live(
            "node-alpha",
            "hub",
            &addr.to_string(),
            Blobs::open_blocking(&tmp_dir).unwrap(),
            diag,
        )));

        let state_bg = state.clone();
        let handle = thread::spawn(move || {
            serve_http_blocking(listener, state_bg, Some(3)).unwrap();
        });

        // 1. GET / hud UI
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

        // 5. Reject external Host / cross-origin POST (CSRF / DNS rebinding defense)
        let resp_csrf = raw_http_request(
            addr,
            "POST /api/daemon/install HTTP/1.1\r\nHost: localhost\r\nOrigin: https://evil.example.com\r\nContent-Length: 2\r\n\r\n{}",
        );
        assert!(resp_csrf.contains("403 Forbidden"));

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

        let mut seeded = NodeState::new_live(
            "node-alpha",
            "hub",
            &addr.to_string(),
            Blobs::open_blocking(&tmp_dir).unwrap(),
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
            "POST /api/blob HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 27\r\n\r\n{\"bytes_base64\":\"ZGlhbHRvbmU=\"}",
        );
        assert!(resp_unauth.contains("401 Unauthorized"));

        // 2. POST with wrong token -> 401 Unauthorized
        let resp_wrong = raw_http_request(
            addr,
            "POST /api/blob HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer wrong-token\r\nContent-Type: application/json\r\nContent-Length: 27\r\n\r\n{\"bytes_base64\":\"ZGlhbHRvbmU=\"}",
        );
        assert!(resp_wrong.contains("401 Unauthorized"));

        // 3. POST with valid Bearer token -> 200 OK
        let body = "{\"bytes_base64\":\"ZGlhbHRvbmU=\"}";
        let req_ok = format!(
            "POST /api/blob HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer dtk-test-token-1234567890abcdef\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{}",
            body.len(),
            body
        );
        let resp_ok = raw_http_request(addr, &req_ok);
        assert!(resp_ok.contains("200 OK"));
        assert!(resp_ok.contains("\"digest\""));

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

        let state = Arc::new(Mutex::new(NodeState::new_live(
            "node-alpha",
            "hub",
            &addr.to_string(),
            Blobs::open_blocking(&tmp_dir).unwrap(),
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
        let resp_3 = raw_http_request(addr, "GET /api/mesh HTTP/1.1\r\nHost: localhost\r\n\r\n");
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

    #[test]
    fn probe_runs_all_five_checks_and_flips_ab_slot() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-probe-tick-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-probe");

        let mut st = NodeState::new_live(
            "node-alpha",
            "hub",
            "127.0.0.1:0",
            Blobs::open_blocking(&tmp_dir).unwrap(),
            diag,
        );

        let report1 = st.run_probe();
        assert_eq!(report1.cycle, 1);
        assert!(report1.blobs_ok);
        assert!(report1.key_value_ok);
        assert!(report1.processes_ok);
        assert!(report1.mesh_connectivity_ok);
        assert!(report1.ab_rollback_ok);
        assert_eq!(report1.passed_count, 5);
        assert_eq!(report1.total_count, 5);
        assert_eq!(report1.active_slot, "B");
        assert!(!report1.store_path.is_empty());
        assert!(!report1.staged_path.is_empty());
        assert!(std::path::Path::new(&report1.store_path).exists());
        assert!(std::path::Path::new(&report1.staged_path).exists());

        let report2 = st.run_probe();
        assert_eq!(report2.cycle, 2);
        assert_eq!(report2.passed_count, 5);
        assert_eq!(report2.active_slot, "A");

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
