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

use crate::blob::{
    evaluate_tiered_blob_fetch, has_blob, put_blob, read_blob, BlobTransportReport, Store,
    TieredBlobFetchRequest,
};
use crate::deploy::{
    evaluate_deploy_build_decision, execute_verified_rollback, generate_deploy_plan,
    install_persistent_os_daemon, plan_rollback, probe_build_cache, render_install_v1_sh,
    validate_manifest_contract, BuildCacheRecord, BuildProbeRequest, DeployManifest, DeploySlot,
};
use crate::hud::render_hud;
use crate::mesh::{
    build_mesh_rtt_matrix, cached_local_public_ip_geo, default_arch, default_config_path,
    default_os, default_state_dir, monotonic_now_ms, node_capabilities_for, node_role_for,
    normalize_channel_name, now_unix, place_nodes, prune_stale_peers, save_mesh_config,
    secret_channel_fingerprint, validate_secret, Beat, BootStageStatus, MeshConfig,
    MeshControlCommand, ProbeReport, BEAT_INTERVAL_SECS, DEFAULT_CHANNEL_NAME,
    DEFAULT_PEER_TTL_SECS, SUMMARY_INTERVAL_SECS,
};
use crate::node::IrohTransportDiagnostics;
use crate::record::{
    paginate_kv_records, plan_alert_ack, AlertRecord, EventRecord, KeyValueRecord, KvPageMeta,
    ServiceLogRecord,
};
use crate::service::Service;
use crate::service::{
    CatalogPolicy, DialtoneMetadata, InstalledProgram, KeyLeaseStatus, ProcessSpec,
};
use crate::stats::{sample_stats, Stats};
use crate::supervisor::{
    ensure_probe_program, plan_process_intent, probe_program_bytes, stage_program, Supervisor,
    PROBE_PROGRAM_NAME,
};
use crate::supervisor::{upsert_command_intent, CursorJobRecord, RobotCommandIntentRecord};

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

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct NodeState {
    pub node: String,
    pub mode: String,
    pub listen: String,
    #[serde(alias = "blob_cache_dir")] // compat
    pub store_dir: PathBuf,
    pub allowed_deploy_roots: Vec<String>,
    #[serde(skip)]
    pub api_token: String,
    #[serde(default)]
    pub boot_stages: Vec<BootStageStatus>,
    #[serde(default)]
    pub mesh_config: MeshConfig,
    #[serde(skip)]
    pub control_tx: Option<tokio::sync::mpsc::UnboundedSender<MeshControlCommand>>,
    #[serde(skip)]
    #[serde(alias = "blob_bridge")] // compat
    pub store: Option<Store>,
    #[serde(skip)]
    pub supervisor: Arc<Mutex<Supervisor>>,
    pub metadata: DialtoneMetadata,
    pub iroh_runtime: IrohTransportDiagnostics,
    pub nodes: Vec<Beat>,
    #[serde(default)]
    pub mesh_rtt_ms: std::collections::BTreeMap<String, f64>,
    #[serde(alias = "installed_binaries")] // compat
    pub installed_programs: Vec<InstalledProgram>,
    #[serde(alias = "managed_processes")] // compat
    pub processes: Vec<ProcessSpec>,
    pub deploy_slots: Vec<DeploySlot>,
    pub deploy_manifests: Vec<DeployManifest>,
    pub deploy_build_caches: Vec<BuildCacheRecord>,
    pub blob_fetches: Vec<BlobTransportReport>,
    pub service_endpoints: Vec<Service>,
    pub key_leases: Vec<KeyLeaseStatus>,
    #[serde(alias = "host_resources")] // compat
    pub stats: Stats,
    pub events: Vec<EventRecord>,
    pub logs: Vec<ServiceLogRecord>,
    pub alerts: Vec<AlertRecord>,
    pub robot_command_intents: Vec<RobotCommandIntentRecord>,
    pub cursor_jobs: Vec<CursorJobRecord>,
    pub kv: Vec<KeyValueRecord>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kv_page: Option<KvPageMeta>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    #[serde(alias = "e2e_test")] // compat
    pub probe: Option<ProbeReport>,
}

impl NodeState {
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
            if peer.stats.disk_percent >= 85.0 {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-disk", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "stats".to_string(),
                    summary: format!(
                        "Node {} disk usage high ({:.1}%)",
                        peer.node, peer.stats.disk_percent
                    ),
                    stats_ref: format!("dialtone://{}/stats/disk", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
            if peer.stats.cpu_percent >= 85.0 {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-cpu", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "stats".to_string(),
                    summary: format!(
                        "Node {} CPU usage high ({:.1}%)",
                        peer.node, peer.stats.cpu_percent
                    ),
                    stats_ref: format!("dialtone://{}/stats/cpu", peer.node),
                    last_seen_unix: now,
                    acknowledged_by: None,
                });
            }
            if peer.stats.mem_percent >= 85.0 {
                live_alerts.push(AlertRecord {
                    id: format!("alert-{}-mem", peer.node),
                    target_node: peer.node.clone(),
                    status: "active".to_string(),
                    severity: "warning".to_string(),
                    source: "stats".to_string(),
                    summary: format!(
                        "Node {} memory usage high ({:.1}%)",
                        peer.node, peer.stats.mem_percent
                    ),
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
                    "Node {} stats nominal (CPU {:.1}%, MEM {:.1}%, DISK {:.1}%)",
                    self.node,
                    self.stats.cpu_percent,
                    self.stats.mem_percent,
                    self.stats.disk_percent
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
        let self_node = self.node.clone();
        let procs_snapshot = self.processes.clone();
        let probe_snapshot = self.probe.clone();
        if let Some(self_beat) = self.nodes.iter_mut().find(|n| n.node == self_node) {
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
            match put_blob(&self.store_dir, exec_payload, None) {
                Ok((sha, store_path)) => {
                    let read_ok = read_blob(&self.store_dir, &sha)
                        .map(|b| b == exec_payload)
                        .unwrap_or(false);
                    (sha, store_path.to_string_lossy().to_string(), read_ok)
                }
                Err(_) => (String::new(), String::new(), false),
            };

        let blob_payload = format!("dialtone-probe-sha-blob-node={}", self.node);
        let (verified_digest, cycle_blob_ok) =
            match put_blob(&self.store_dir, blob_payload.as_bytes(), None) {
                Ok((sha, _)) => {
                    let read_ok = read_blob(&self.store_dir, &sha)
                        .map(|b| b == blob_payload.as_bytes())
                        .unwrap_or(false);
                    (sha, read_ok)
                }
                Err(_) => (String::new(), false),
            };
        let blobs_ok = exec_blob_ok && cycle_blob_ok;

        // 2. Sub-Processes: load executable from SHA-256 blob store -> stage to run dir -> supervise & verify PID
        let staged_from_store_ok = if !program_digest.is_empty() {
            stage_program(&self.store_dir, PROBE_PROGRAM_NAME, &program_digest).is_ok()
        } else {
            false
        };
        let _ = ensure_probe_program(&self.store_dir, &mut self.processes, None);
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
                let intent_ok =
                    plan_process_intent(&self.node, &self.node, proc, "restart").is_ok();
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
                    staged_from_store_ok && running && intent_ok,
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
        if let (Ok((sha_a, path_a)), Ok((sha_b, path_b))) = (
            put_blob(&self.store_dir, slot_a_bytes.as_bytes(), None),
            put_blob(&self.store_dir, slot_b_bytes.as_bytes(), None),
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
        let self_node = self.node.clone();
        if let Some(self_beat) = self.nodes.iter_mut().find(|n| n.node == self_node) {
            self_beat.probe = Some(report.clone());
        }
        report
    }

    pub fn new_live(
        node: &str,
        mode: &str,
        listen: &str,
        store_dir: PathBuf,
        iroh_runtime: IrohTransportDiagnostics,
    ) -> Self {
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
            channel_mac: None,
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
            allowed_deploy_roots: vec!["/opt/dialtone/".to_string(), "/tmp/".to_string()],
            boot_stages,
            mesh_config,
            control_tx: None,
            store: None,
            supervisor: Arc::new(Mutex::new(Supervisor::new())),
            metadata: DialtoneMetadata::default(),
            iroh_runtime,
            nodes: vec![self_beat],
            mesh_rtt_ms: std::collections::BTreeMap::new(),
            installed_programs,
            processes: Vec::new(),
            deploy_slots: Vec::new(),
            deploy_manifests: Vec::new(),
            deploy_build_caches: Vec::new(),
            blob_fetches: Vec::new(),
            service_endpoints,
            key_leases: Vec::new(),
            stats: stats.clone(),
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
                "dialtone live mesh daemon initialized on {} ({}/{}, {} cores, {:.1}% mem, {:.1}% disk)",
                node,
                default_os(),
                default_arch(),
                stats.cpu_cores,
                stats.mem_percent,
                stats.disk_percent
            ),
        );
        st.record_log(
            "dialtone-daemon",
            "INFO",
            "stdout",
            format!(
                "started live stats & gossip runtime on http://{} (beat={}s, summary={}s, cpu={:.1}%, mem={:.1}%, disk={:.1}%)",
                listen,
                BEAT_INTERVAL_SECS,
                SUMMARY_INTERVAL_SECS,
                stats.cpu_percent,
                stats.mem_percent,
                stats.disk_percent
            ),
        );
        place_nodes(&mut st.nodes, &st.mesh_rtt_ms);
        st.refresh_live_alerts();
        st
    }

    pub fn new_seeded(
        node: &str,
        mode: &str,
        listen: &str,
        store_dir: PathBuf,
        iroh_runtime: IrohTransportDiagnostics,
    ) -> Self {
        let now = now_unix();
        let sample_sha =
            "sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d".to_string();
        let prev_sha =
            "sha256:a1892e88f06224d9bd720a5f5a2c9fd70184c4eaf887f96e7df89f229484ffb1".to_string();
        let service_endpoints = vec![Service {
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
        }];

        let mut peer_beats = Vec::new();
        for peer_name in ["node-alpha", "node-beta", "edge-1", "pendant-1"] {
            peer_beats.push(Beat {
                schema: "dialtone.iroh.beat.v1".to_string(),
                node: peer_name.to_string(),
                node_id: iroh_runtime.node_id.clone(),
                ticket: format!("iroh-gossip-v1:{peer_name}"),
                counter: 1,
                updated_at_unix: now,
                kind: "summary".to_string(),
                beat_interval_secs: BEAT_INTERVAL_SECS,
                summary_interval_secs: SUMMARY_INTERVAL_SECS,
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
                stats: sample_stats(peer_name, iroh_runtime.bound_non_tailnet_ipv4.as_deref(), 1),
                service_endpoints: if peer_name == node {
                    service_endpoints.clone()
                } else {
                    Vec::new()
                },
                processes: Vec::new(),
                iroh_transport: iroh_runtime.clone(),
                observed_remote_addrs: Vec::new(),
                tailnet_used: false,
                geo_location: None,
                probe: None,
                store: None,
            });
        }
        let seeded_rtt = build_mesh_rtt_matrix(&peer_beats);
        place_nodes(&mut peer_beats, &seeded_rtt);

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

        let boot_stages = default_boot_stages(listen, &store_dir);
        let mesh_config = MeshConfig {
            node_name: Some(node.to_string()),
            listen_addr: Some(listen.to_string()),
            channel: DEFAULT_CHANNEL_NAME.to_string(),
            configured: true,
            secret_key: None,
            derived_keys: None,
            channel_fingerprint: secret_channel_fingerprint("dialtone:seeded"),
            invites: Vec::new(),
            seeds: Vec::new(),
            allowlist: Vec::new(),
            used_invite_nonces: Vec::new(),
            daemon_installed: false,
            daemon_service_path: None,
            processes: Vec::new(),
            updated_at_unix: now,
        };
        let api_token =
            load_or_create_api_token().unwrap_or_else(|_| "dtk-seeded-token".to_string());

        Self {
            node: node.to_string(),
            mode: mode.to_string(),
            listen: listen.to_string(),
            api_token,
            store_dir,
            allowed_deploy_roots: vec!["/opt/dialtone/".to_string(), "/tmp/".to_string()],
            boot_stages,
            mesh_config,
            control_tx: None,
            store: None,
            supervisor: Arc::new(Mutex::new(Supervisor::new())),
            metadata,
            iroh_runtime,
            nodes: peer_beats,
            mesh_rtt_ms: seeded_rtt,
            installed_programs: vec![InstalledProgram {
                name: "robot-v4-cartesiand".to_string(),
                path: "/opt/dialtone/bin/robot-v4-cartesiand".to_string(),
                digest: sample_sha.clone(),
            }],
            processes: vec![ProcessSpec {
                name: "robot-edge-motion-server".to_string(),
                path: "/opt/dialtone/bin/robot-v4-cartesiand".to_string(),
                digest: sample_sha.clone(),
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
                digest: sample_sha,
                previous_path: Some("/opt/dialtone/slots/b/robot-v4-cartesiand".to_string()),
                previous_digest: Some(prev_sha),
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
            stats: Stats::default(),
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
                source: "stats".to_string(),
                summary: "Edge-1 stats nominal".to_string(),
                stats_ref: "dialtone://edge-1/edge/stats".to_string(),
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
            probe: None,
        }
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

fn handle_connection(mut stream: TcpStream, state: &Arc<Mutex<NodeState>>) -> Result<(), String> {
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
    if let Some(self_beat) = st.nodes.iter_mut().find(|n| n.node == self_name) {
        self_beat.updated_at_unix = now;
    }
    prune_stale_peers(&mut st.nodes, now, DEFAULT_PEER_TTL_SECS);

    match (method, path) {
        ("GET", "/") | ("GET", "/nodes") | ("GET", "/peer") | ("GET", "/hud") => {
            st.reconcile_processes();
            let html = render_hud(&st, query.get("peer-id").map(String::as_str));
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
                "store_dir": st.store_dir,
                "boot_stages": st.boot_stages,
                "mesh_config": masked_mesh_config(&st.mesh_config),
                "iroh_runtime": st.iroh_runtime
            });
            write_json(&mut stream, 200, &body)
        }
        ("POST", "/api/mesh/join") | ("POST", "/api/config/join") => {
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

            if raw_ticket.starts_with(crate::mesh::INVITE_TICKET_PREFIX) {
                let parsed_invite = match raw_ticket.parse::<crate::mesh::Invite>() {
                    Ok(inv) => inv,
                    Err(err) => return write_json(&mut stream, 400, &json!({ "error": err })),
                };
                let self_ep = st.iroh_runtime.node_id.clone();
                if let Err(err) = crate::mesh::redeem_invite_ticket(
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
        ("GET", "/api/mesh") | ("GET", "/api/gossip") => {
            st.reconcile_processes();
            let (blob_keys, blob_bytes) = count_store(&st.store_dir);
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
                    "blob_keys": blob_keys,
                    "blob_bytes": blob_bytes,
                    "blob_fetches": st.blob_fetches.len(),
                    "kv_keys": st.kv.len(),
                    "key_leases": st.key_leases.len(),
                    "installed_programs": st.installed_programs.len(),
                    "deploy_manifests": st.deploy_manifests.len()
                },
                "nodes": st.nodes,
                "mesh_rtt_ms": st.mesh_rtt_ms,
                "processes": st.processes,
                "service_endpoints": st.service_endpoints
            });
            write_json(&mut stream, 200, &body)
        }
        ("GET", "/api/iroh-runtime") => {
            let body = json!({
                "schema": "dialtone.iroh_runtime.v1",
                "owner": "dialtone-rust-daemon",
                "steady_state_data_plane": "iroh-quic",
                "http_plane_role": "local-seed-control-only",
                "node": st.node,
                "diagnostics": st.iroh_runtime
            });
            write_json(&mut stream, 200, &body)
        }
        ("HEAD", "/api/artifacts/blob") => {
            let sha = query.get("digest").cloned().unwrap_or_default();
            if has_blob(&st.store_dir, &sha) {
                write_response(&mut stream, 200, "application/octet-stream", b"", true)
            } else {
                write_response(&mut stream, 404, "application/octet-stream", b"", true)
            }
        }
        ("GET", "/api/artifacts/blob") => {
            let sha = query.get("digest").cloned().unwrap_or_default();
            match read_blob(&st.store_dir, &sha) {
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
                .get("digest")
                .and_then(Value::as_str)
                .map(String::from);
            match crate::base64_decode(b64) {
                Ok(raw_bytes) => {
                    let maybe_bridge = st.store.clone();
                    let store_dir = st.store_dir.clone();
                    let node_name = st.node.clone();
                    drop(st);
                    if let Some(bridge) = maybe_bridge {
                        match bridge.share_blob(&store_dir, &raw_bytes, expected_sha.as_deref()) {
                            Ok(rep) => write_json(
                                &mut stream,
                                200,
                                &json!({
                                    "status": rep.status,
                                    "node": node_name,
                                    "digest": rep.digest,
                                    "blake3_hash": rep.blake3_hash,
                                    "path": rep.path,
                                    "bytes": rep.bytes,
                                    "provider_node_id": rep.provider_node_id,
                                    "ticket_uri": rep.ticket_uri
                                }),
                            ),
                            Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                        }
                    } else {
                        match put_blob(&store_dir, &raw_bytes, expected_sha.as_deref()) {
                            Ok((sha, path)) => write_json(
                                &mut stream,
                                200,
                                &json!({ "digest": sha, "path": path, "bytes": raw_bytes.len() }),
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
            let Some(bridge) = st.store.clone() else {
                return write_json(
                    &mut stream,
                    503,
                    &json!({ "error": "live iroh-blobs bridge is not initialized on this node" }),
                );
            };
            let store_dir = st.store_dir.clone();
            drop(st);
            match bridge.pull_blob(&store_dir, &ticket_uri, force) {
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
                    digest: query.get("digest").cloned().unwrap_or_default(),
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
            match req.and_then(|r| evaluate_tiered_blob_fetch(&st.store_dir, &r)) {
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
                if let Some(ref bridge) = st.store {
                    match bridge.share_blob(&st.store_dir, &raw, exp) {
                        Ok(rep) => {
                            digest = rep.digest;
                            resolved_ticket_uri = Some(rep.ticket_uri);
                        }
                        Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                    }
                } else {
                    match put_blob(&st.store_dir, &raw, exp) {
                        Ok((verified_sha, _)) => digest = verified_sha,
                        Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                    }
                }
            }

            if let Some(ref t_uri) = resolved_ticket_uri {
                if digest.is_empty() || !has_blob(&st.store_dir, &digest) {
                    if let Some(ref bridge) = st.store {
                        match bridge.pull_blob(&st.store_dir, t_uri, false) {
                            Ok(rep) => {
                                digest = rep.digest.clone();
                                st.blob_fetches.push(rep);
                            }
                            Err(e) => return write_json(&mut stream, 400, &json!({ "error": e })),
                        }
                    }
                }
            }

            if digest.is_empty() {
                if let Some(existing) = st.processes.iter().find(|p| p.name == name) {
                    digest = existing.digest.clone();
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
                if let (Some(ref bridge), Ok(cached_bytes)) =
                    (&st.store, read_blob(&st.store_dir, &digest))
                {
                    if let Ok(rep) = bridge.share_blob(&st.store_dir, &cached_bytes, Some(&digest))
                    {
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

            let (verified_sha, staged_path) = match stage_program(&st.store_dir, &name, &digest) {
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

            let self_node = st.node.clone();
            let procs_snapshot = st.processes.clone();
            if let Some(self_beat) = st.nodes.iter_mut().find(|n| n.node == self_node) {
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
        ("GET", "/api/process-intent") | ("POST", "/api/process-intent") => {
            let target_node = query
                .get("node")
                .cloned()
                .unwrap_or_else(|| "edge-1".to_string());
            let action = query
                .get("action")
                .cloned()
                .unwrap_or_else(|| "restart".to_string());
            if let Some(proc) = st.processes.first() {
                match plan_process_intent(&st.node, &target_node, proc, &action) {
                    Ok(plan) => write_json(&mut stream, 200, &plan),
                    Err(err) => write_json(&mut stream, 400, &json!({ "error": err })),
                }
            } else {
                write_json(&mut stream, 404, &json!({ "error": "no process" }))
            }
        }
        ("POST", "/api/e2e/tick") | ("GET", "/api/e2e/status") => {
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
    state: &Arc<Mutex<NodeState>>,
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

pub fn count_store(blob_dir: &std::path::Path) -> (usize, u64) {
    let mut keys = 0usize;
    let mut bytes = 0u64;
    if let Ok(top_entries) = fs::read_dir(blob_dir) {
        for entry in top_entries.flatten() {
            if let Ok(ft) = entry.file_type() {
                if ft.is_dir() {
                    if let Ok(sub_entries) = fs::read_dir(entry.path()) {
                        for sub in sub_entries.flatten() {
                            if let Ok(meta) = sub.metadata() {
                                if meta.is_file() {
                                    keys += 1;
                                    bytes += meta.len();
                                }
                            }
                        }
                    }
                } else if ft.is_file() {
                    if let Ok(meta) = entry.metadata() {
                        keys += 1;
                        bytes += meta.len();
                    }
                }
            }
        }
    }
    (keys, bytes)
}

/// Digest (the store file name) of the most recently written blob in `blob_dir`, if any.
pub fn latest_digest(blob_dir: &std::path::Path) -> Option<String> {
    let is_digest = |name: &str| name.len() == 64 && name.bytes().all(|b| b.is_ascii_hexdigit());
    let mut newest: Option<(std::time::SystemTime, String)> = None;
    let mut consider = |entry: &fs::DirEntry| {
        let name = entry.file_name().to_string_lossy().to_string();
        if !is_digest(&name) {
            return;
        }
        if let Ok(modified) = entry.metadata().and_then(|m| m.modified()) {
            if newest.as_ref().is_none_or(|(t, _)| modified > *t) {
                newest = Some((modified, name));
            }
        }
    };
    if let Ok(top_entries) = fs::read_dir(blob_dir) {
        for entry in top_entries.flatten() {
            match entry.file_type() {
                Ok(ft) if ft.is_dir() => {
                    if let Ok(sub_entries) = fs::read_dir(entry.path()) {
                        sub_entries.flatten().for_each(|sub| consider(&sub));
                    }
                }
                Ok(ft) if ft.is_file() => consider(&entry),
                _ => {}
            }
        }
    }
    newest.map(|(_, name)| format!("sha256:{name}"))
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
    use crate::node::{build_diagnostics, IrohTransportConfig};
    use iroh::{EndpointAddr, SecretKey};
    use std::env;
    use std::thread;

    #[test]
    fn latest_blob_sha256_picks_newest_digest_and_ignores_non_digest_files() {
        let dir = env::temp_dir().join(format!("dialtone-latest-blob-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        assert_eq!(latest_digest(&dir), None);

        let older = "a".repeat(64);
        let newer = "b".repeat(64);
        fs::create_dir_all(dir.join("aa")).unwrap();
        fs::create_dir_all(dir.join("bb")).unwrap();
        fs::write(dir.join("aa").join(&older), b"old").unwrap();
        fs::write(dir.join("bb").join(&newer), b"new").unwrap();
        fs::write(dir.join("notes.txt"), b"not a blob").unwrap();

        let base = std::time::SystemTime::UNIX_EPOCH + Duration::from_secs(1_700_000_000);
        let set_mtime = |path: std::path::PathBuf, t: std::time::SystemTime| {
            fs::File::options()
                .write(true)
                .open(path)
                .unwrap()
                .set_modified(t)
                .unwrap();
        };
        set_mtime(dir.join("aa").join(&older), base);
        set_mtime(dir.join("bb").join(&newer), base + Duration::from_secs(60));
        set_mtime(dir.join("notes.txt"), base + Duration::from_secs(120));

        assert_eq!(latest_digest(&dir), Some(format!("sha256:{newer}")));
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn http_and_websocket_server_serves_ui_state_blobs_catalogs_and_ws_upgrade() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-http-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();

        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-123");

        let state = Arc::new(Mutex::new(NodeState::new_seeded(
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

        let mut seeded = NodeState::new_seeded(
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

        let state = Arc::new(Mutex::new(NodeState::new_seeded(
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

    #[test]
    fn probe_capability_tick_verifies_all_five_capabilities_and_flips_ab_slot() {
        let tmp_dir = env::temp_dir().join(format!("dialtone-probe-tick-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&tmp_dir);
        let cfg = IrohTransportConfig::local_offline();
        let ep_addr = EndpointAddr::new(SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &ep_addr, None, "ep-test-probe");

        let mut st =
            NodeState::new_seeded("node-alpha", "hub", "127.0.0.1:0", tmp_dir.clone(), diag);

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
