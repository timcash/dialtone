use std::collections::HashMap;
use std::fs;
use std::net::UdpSocket;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::Duration;

use serde::{Deserialize, Serialize};

use crate::blobs::{compute_sha256, normalize_sha256, read_cached_blob};
use crate::metadata::{InstalledBinary, ManagedProcessSpec};

pub const RTCORE_ARM_MAGIC: u32 = 0x4152_4d44;
pub const RTCORE_MODE_BOUND_VERSION: u32 = 2;
pub const RTCORE_PACKET_BYTES: usize = 64;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct RobotCommandIntentRecord {
    pub id: String,
    pub command_id: String,
    pub publisher_node: String,
    pub target_node: String,
    pub service_ref: String,
    pub command: String,
    pub runtime_mode: String,
    pub binding_epoch: u64,
    pub delivery: String,
    pub status: String,
    pub updated_at_unix: u64,
    pub message: String,
}

pub fn is_terminal_command_status(status: &str) -> bool {
    matches!(status, "applied" | "failed" | "rejected")
}

pub fn upsert_command_intent(
    journal: &mut Vec<RobotCommandIntentRecord>,
    incoming: RobotCommandIntentRecord,
) -> bool {
    if let Some(existing) = journal.iter_mut().find(|r| r.id == incoming.id) {
        if is_terminal_command_status(&existing.status)
            && !is_terminal_command_status(&incoming.status)
        {
            return false;
        }
        if existing.updated_at_unix > incoming.updated_at_unix || *existing == incoming {
            return false;
        }
        *existing = incoming;
    } else {
        journal.push(incoming);
    }
    if journal.len() > 120 {
        let overflow = journal.len() - 120;
        journal.drain(0..overflow);
    }
    true
}

pub fn encode_rtcore_udp_packet(
    intent: &RobotCommandIntentRecord,
    sender_id: &str,
) -> Result<[u8; RTCORE_PACKET_BYTES], String> {
    let cmd_code: u32 = match intent.command.as_str() {
        "arm" => 1,
        "disarm" => 0,
        other => return Err(format!("unsupported rtcore command {other:?}")),
    };
    let mode_code: u32 = match intent.runtime_mode.as_str() {
        "sim" => 0,
        "live" => 1,
        other => return Err(format!("unsupported rtcore runtime_mode {other:?}")),
    };

    let mut pkt = [0u8; RTCORE_PACKET_BYTES];
    pkt[0..4].copy_from_slice(&RTCORE_ARM_MAGIC.to_le_bytes());
    pkt[4..8].copy_from_slice(&RTCORE_MODE_BOUND_VERSION.to_le_bytes());
    pkt[8..12].copy_from_slice(&cmd_code.to_le_bytes());
    let sender_bytes = sender_id.as_bytes();
    let copy_len = sender_bytes.len().min(32);
    pkt[16..16 + copy_len].copy_from_slice(&sender_bytes[..copy_len]);
    pkt[48..52].copy_from_slice(&mode_code.to_le_bytes());
    pkt[56..64].copy_from_slice(&intent.binding_epoch.to_le_bytes());
    Ok(pkt)
}

pub fn dispatch_rtcore_udp_command_loopback(
    journal: &mut Vec<RobotCommandIntentRecord>,
    mut intent: RobotCommandIntentRecord,
    sender_id: &str,
) -> Result<usize, String> {
    let receiver =
        UdpSocket::bind("127.0.0.1:0").map_err(|e| format!("bind rtcore udp receiver: {e}"))?;
    receiver
        .set_read_timeout(Some(Duration::from_secs(2)))
        .map_err(|e| format!("set read timeout: {e}"))?;
    let target_addr = receiver
        .local_addr()
        .map_err(|e| format!("receiver local_addr: {e}"))?;

    let sender =
        UdpSocket::bind("127.0.0.1:0").map_err(|e| format!("bind rtcore udp sender: {e}"))?;
    let packet = encode_rtcore_udp_packet(&intent, sender_id)?;
    let sent = sender
        .send_to(&packet, target_addr)
        .map_err(|e| format!("send rtcore udp packet: {e}"))?;

    let mut recv_buf = [0u8; 128];
    let (n, _) = receiver
        .recv_from(&mut recv_buf)
        .map_err(|e| format!("recv rtcore udp packet: {e}"))?;
    if n != RTCORE_PACKET_BYTES || &recv_buf[..RTCORE_PACKET_BYTES] != packet.as_slice() {
        return Err("rtcore udp packet verification mismatch".to_string());
    }

    intent.status = "applied".to_string();
    intent.message = format!("applied via rtcore-udp ({sent} bytes)");
    let _ = upsert_command_intent(journal, intent);
    Ok(sent)
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct CursorJobRecord {
    pub id: String,
    pub host: String,
    pub remote_user: Option<String>,
    pub prompt: String,
    pub status: String,
    pub logs: Vec<String>,
    pub updated_at_unix: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct CursorLogPage {
    pub job_id: String,
    pub total_lines: usize,
    pub offset: usize,
    pub limit: usize,
    pub has_more: bool,
    pub lines: Vec<String>,
}

impl CursorJobRecord {
    pub fn log_page(&self, offset: usize, limit: usize) -> CursorLogPage {
        let bounded_limit = limit.clamp(1, 200);
        let total = self.logs.len();
        let lines: Vec<String> = self
            .logs
            .iter()
            .skip(offset)
            .take(bounded_limit)
            .cloned()
            .collect();
        let has_more = offset.saturating_add(lines.len()) < total;
        CursorLogPage {
            job_id: self.id.clone(),
            total_lines: total,
            offset,
            limit: bounded_limit,
            has_more,
            lines,
        }
    }

    pub fn stop(&mut self) {
        self.status = "stopped".to_string();
        self.logs.push("job stopped by operator".to_string());
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProcessIntentPlan {
    pub publisher_node: String,
    pub target_node: String,
    pub process: String,
    pub action: String,
    pub desired_state: String,
    pub restart_requested: bool,
    pub current_status: String,
    pub transition_diff: String,
    pub dependency_warnings: Vec<String>,
}

/// Audit process args and env for forbidden steady-state dependencies (fixed LAN IPs,
/// Tailnet `.ts.net`, SSH/scp, remote git).
pub fn audit_process_dependencies(spec: &ManagedProcessSpec) -> Vec<String> {
    let mut warnings = Vec::new();
    for item in spec.args.iter().chain(spec.env.iter()) {
        let lower = item.to_ascii_lowercase();
        if lower.contains(".ts.net") {
            warnings.push(format!(
                "forbidden Tailnet hostname dependency in {:?}",
                item
            ));
        }
        if lower.contains("192.168.") || lower.contains("100.64.") {
            warnings.push(format!(
                "forbidden fixed LAN/Tailnet IP literal in {:?}",
                item
            ));
        }
        if lower.contains("ssh ") || lower.contains("scp ") || lower.contains("git clone") {
            warnings.push(format!("forbidden SSH/scp/git dependency in {:?}", item));
        }
    }
    warnings
}

pub fn plan_process_intent(
    publisher_node: &str,
    target_node: &str,
    spec: &ManagedProcessSpec,
    action: &str,
) -> Result<ProcessIntentPlan, String> {
    let (desired_state, restart_requested) = match action {
        "start" => ("running".to_string(), false),
        "stop" => ("stopped".to_string(), false),
        "restart" => ("running".to_string(), true),
        other => {
            return Err(format!(
                "unsupported process intent action {other:?}; expected start, stop, or restart"
            ))
        }
    };
    let current_status = spec.status.clone().unwrap_or_else(|| "stopped".to_string());
    let transition_diff = format!(
        "desired_state: {} -> {}, restart: {}",
        spec.desired_state, desired_state, restart_requested
    );
    let dependency_warnings = audit_process_dependencies(spec);

    Ok(ProcessIntentPlan {
        publisher_node: publisher_node.to_string(),
        target_node: target_node.to_string(),
        process: spec.name.clone(),
        action: action.to_string(),
        desired_state,
        restart_requested,
        current_status,
        transition_diff,
        dependency_warnings,
    })
}

/// Hydrate empty or `sha256:unknown` process hashes from matching installed binaries,
/// falling back to hashing the executable path on disk.
pub fn hydrate_managed_process_hashes(
    processes: &mut [ManagedProcessSpec],
    installed: &[InstalledBinary],
    previous_paths: &HashMap<String, String>,
) {
    for proc in processes.iter_mut() {
        if let Some(prev_path) = previous_paths.get(&proc.name) {
            if prev_path != &proc.path {
                proc.sha256 = "sha256:unknown".to_string();
            }
        }
        if proc.sha256.trim().is_empty() || proc.sha256 == "sha256:unknown" {
            if let Some(bin) = installed.iter().find(|b| b.path == proc.path) {
                if !bin.sha256.is_empty() && bin.sha256 != "sha256:unknown" {
                    proc.sha256 = bin.sha256.clone();
                    continue;
                }
            }
            if let Ok(bytes) = fs::read(&proc.path) {
                proc.sha256 = compute_sha256(&bytes);
            }
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct SystemdReconcileReport {
    pub drop_in_path: String,
    pub updated_drop_in: bool,
    pub removed_drop_ins: Vec<String>,
    pub stop_status: String,
}

/// Render canonical `<unit>.d/25-daemon-intent.conf`, repair byte-for-byte drift when enabled,
/// remove retired drop-in `.conf` overrides, and exclude `DIALTONE_*` / `DAEMON_V1_*` supervisor
/// variables from the child environment.
pub fn reconcile_systemd_intent_drop_in(
    unit_dir: &Path,
    spec: &ManagedProcessSpec,
    reconcile_drift: bool,
    remove_drop_ins: &[String],
    simulated_stop_elapsed_ms: u64,
    stop_timeout_ms: u64,
) -> Result<SystemdReconcileReport, String> {
    fs::create_dir_all(unit_dir)
        .map_err(|e| format!("create systemd drop-in dir {}: {e}", unit_dir.display()))?;

    let mut removed = Vec::new();
    for name in remove_drop_ins {
        let trimmed = name.trim();
        if trimmed.is_empty() {
            continue;
        }
        if trimmed.contains('/')
            || trimmed.contains("..")
            || !trimmed.ends_with(".conf")
            || trimmed == "25-daemon-intent.conf"
        {
            return Err(format!("invalid retired systemd drop-in name {trimmed:?}"));
        }
        let target = unit_dir.join(trimmed);
        if target.exists() {
            fs::remove_file(&target)
                .map_err(|e| format!("remove retired drop-in {}: {e}", target.display()))?;
            removed.push(trimmed.to_string());
        }
    }

    let child_env: Vec<&String> = spec
        .env
        .iter()
        .filter(|kv| !kv.starts_with("DIALTONE_") && !kv.starts_with("DAEMON_V1_"))
        .collect();

    let mut canonical = String::from("[Service]\nExecStart=\n");
    if spec.args.is_empty() {
        canonical.push_str(&format!("ExecStart={}\n", spec.path));
    } else {
        canonical.push_str(&format!(
            "ExecStart={} {}\n",
            spec.path,
            spec.args.join(" ")
        ));
    }
    for env_kv in child_env {
        canonical.push_str(&format!("Environment=\"{env_kv}\"\n"));
    }

    let drop_in_path = unit_dir.join("25-daemon-intent.conf");
    let existing = fs::read_to_string(&drop_in_path).ok();
    let updated_drop_in = match existing {
        None => {
            fs::write(&drop_in_path, &canonical)
                .map_err(|e| format!("write {}: {e}", drop_in_path.display()))?;
            true
        }
        Some(cur) if reconcile_drift && cur != canonical => {
            fs::write(&drop_in_path, &canonical)
                .map_err(|e| format!("repair drift {}: {e}", drop_in_path.display()))?;
            true
        }
        Some(_) => false,
    };

    let bounded_timeout = stop_timeout_ms.clamp(100, 30_000);
    let stop_status =
        if spec.desired_state == "stopped" && simulated_stop_elapsed_ms > bounded_timeout {
            "stop_pending".to_string()
        } else if spec.desired_state == "stopped" {
            "stopped".to_string()
        } else {
            "running".to_string()
        };

    Ok(SystemdReconcileReport {
        drop_in_path: drop_in_path.to_string_lossy().to_string(),
        updated_drop_in,
        removed_drop_ins: removed,
        stop_status,
    })
}

/// Materialize a verified blob (`sha256:<hex>`) from `blob_cache_dir` into an executable
/// binary/script under `<blob_cache_dir>/../run/<name>-<short_hex>`, setting `0o755` permissions
/// on Unix and using atomic temp-file rename so running processes are never corrupted.
pub fn stage_executable_blob(
    blob_cache_dir: &Path,
    name: &str,
    sha256: &str,
) -> Result<(String, PathBuf), String> {
    let normalized_sha = normalize_sha256(sha256)?;
    let bytes = read_cached_blob(blob_cache_dir, &normalized_sha)?;
    let short_hex = &normalized_sha
        .strip_prefix("sha256:")
        .unwrap_or(&normalized_sha)[..12];
    let safe_name: String = name
        .chars()
        .filter(|c| c.is_ascii_alphanumeric() || *c == '-' || *c == '_' || *c == '.')
        .collect();
    if safe_name.is_empty() {
        return Err(format!("invalid process name {name:?}"));
    }

    let run_dir = blob_cache_dir
        .parent()
        .unwrap_or(blob_cache_dir)
        .join("run");
    fs::create_dir_all(&run_dir)
        .map_err(|e| format!("create executable blob run dir {}: {e}", run_dir.display()))?;

    let filename = if cfg!(windows)
        && !safe_name.ends_with(".exe")
        && !safe_name.ends_with(".cmd")
        && !safe_name.ends_with(".bat")
    {
        if bytes.starts_with(b"MZ") {
            format!("{safe_name}-{short_hex}.exe")
        } else {
            format!("{safe_name}-{short_hex}.cmd")
        }
    } else {
        format!("{safe_name}-{short_hex}")
    };

    let target_path = run_dir.join(&filename);
    let tmp_path = run_dir.join(format!(".{filename}.tmp.{}", std::process::id()));
    fs::write(&tmp_path, &bytes)
        .map_err(|e| format!("write staged blob {}: {e}", tmp_path.display()))?;

    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let perms = fs::Permissions::from_mode(0o755);
        fs::set_permissions(&tmp_path, perms)
            .map_err(|e| format!("chmod +x staged blob {}: {e}", tmp_path.display()))?;
    }

    fs::rename(&tmp_path, &target_path).map_err(|e| {
        format!(
            "rename staged blob {} -> {}: {e}",
            tmp_path.display(),
            target_path.display()
        )
    })?;

    Ok((normalized_sha, target_path))
}

pub const DEFAULT_SUBPROCESS_BLOB_NAME: &str = "mesh-blob-worker";

/// Canonical cross-platform polyglot E2E test subprocess blob (`sh` on macOS/Linux, `cmd.exe` on Windows)
/// sharing an identical SHA-256 digest across all OS/arch nodes in the fleet.
/// Continuously exercises SHA-256 blobs, Key-Value store, Subprocess supervision, Mesh connectivity, and A/B rollback.
pub fn default_subprocess_blob_bytes() -> &'static [u8] {
    b":; while true; do if [ -n \"$APP_LISTEN\" ]; then \"$APP_BIN\" test-tick --listen \"$APP_LISTEN\"; fi; sleep 5; done; exit 0;\r\n@echo off\r\n:dialtone_blob_loop\r\nif defined APP_LISTEN \"%APP_BIN%\" test-tick --listen \"%APP_LISTEN%\"\r\nping -n 6 127.0.0.1 >NUL\r\ngoto dialtone_blob_loop\r\n"
}

/// Automatically store, verify, and stage the default cross-platform subprocess blob in `blob_cache_dir`
/// and ensure its `ManagedProcessSpec` is registered in `specs`.
pub fn ensure_default_subprocess_blob(
    blob_cache_dir: &Path,
    specs: &mut Vec<ManagedProcessSpec>,
    ticket_uri: Option<&str>,
) -> Result<ManagedProcessSpec, String> {
    let payload = default_subprocess_blob_bytes();
    let (sha256, _) = crate::blobs::store_verified_blob(blob_cache_dir, payload, None)?;
    let (verified_sha, staged_path) =
        stage_executable_blob(blob_cache_dir, DEFAULT_SUBPROCESS_BLOB_NAME, &sha256)?;

    let mut env_vars = vec![
        "DIALTONE_MESH_DEPLOY=1".to_string(),
        "WORKER_ROLE=mesh-blob-worker".to_string(),
    ];
    if let Some(t_uri) = ticket_uri.map(str::trim).filter(|s| !s.is_empty()) {
        env_vars.push(format!("DIALTONE_BLOB_TICKET={t_uri}"));
    } else if let Some(existing) = specs
        .iter()
        .find(|p| p.name == DEFAULT_SUBPROCESS_BLOB_NAME)
    {
        for kv in &existing.env {
            if kv.starts_with("DIALTONE_BLOB_TICKET=") {
                env_vars.push(kv.clone());
            }
        }
    }

    if let Some(existing) = specs
        .iter_mut()
        .find(|p| p.name == DEFAULT_SUBPROCESS_BLOB_NAME)
    {
        existing.path = staged_path.to_string_lossy().to_string();
        existing.sha256 = verified_sha;
        if existing.desired_state.trim().is_empty() {
            existing.desired_state = "running".to_string();
        }
        existing.env = env_vars;
        return Ok(existing.clone());
    }

    let spec = ManagedProcessSpec {
        name: DEFAULT_SUBPROCESS_BLOB_NAME.to_string(),
        path: staged_path.to_string_lossy().to_string(),
        sha256: verified_sha,
        desired_state: "running".to_string(),
        args: Vec::new(),
        env: env_vars,
        cwd: None,
        startup_grace_secs: Some(2),
        status: None,
        pid: None,
        restart_count: 0,
        service_unit: None,
    };
    specs.push(spec.clone());
    Ok(spec)
}

#[derive(Debug)]
struct SupervisedChild {
    child: Child,
    path: String,
    sha256: String,
}

#[derive(Debug)]
pub struct ManagedProcessSupervisor {
    children: HashMap<String, SupervisedChild>,
}

impl Default for ManagedProcessSupervisor {
    fn default() -> Self {
        Self::new()
    }
}

impl ManagedProcessSupervisor {
    pub fn new() -> Self {
        Self {
            children: HashMap::new(),
        }
    }

    pub fn reconcile(&mut self, specs: &mut [ManagedProcessSpec]) -> Result<(), String> {
        for spec in specs.iter_mut() {
            if spec.desired_state == "running" {
                let mut needs_spawn = true;
                if let Some(entry) = self.children.get_mut(&spec.name) {
                    if entry.path != spec.path || entry.sha256 != spec.sha256 {
                        let _ = entry.child.kill();
                        let _ = entry.child.wait();
                        self.children.remove(&spec.name);
                    } else if entry.child.try_wait().map_err(|e| e.to_string())?.is_none() {
                        spec.pid = Some(entry.child.id());
                        spec.status = Some("running".to_string());
                        needs_spawn = false;
                    } else {
                        spec.restart_count = spec.restart_count.saturating_add(1);
                        self.children.remove(&spec.name);
                    }
                }
                if needs_spawn {
                    #[cfg(unix)]
                    let mut cmd = {
                        let header = fs::read(&spec.path).unwrap_or_default();
                        if header.starts_with(b":;") {
                            let mut c = Command::new("/bin/sh");
                            c.arg(&spec.path).args(&spec.args);
                            c
                        } else {
                            let mut c = Command::new(&spec.path);
                            c.args(&spec.args);
                            c
                        }
                    };
                    #[cfg(not(unix))]
                    let mut cmd = {
                        let mut c = Command::new(&spec.path);
                        c.args(&spec.args);
                        c
                    };
                    cmd.stdin(Stdio::null())
                        .stdout(Stdio::null())
                        .stderr(Stdio::null());
                    if let Ok(exe_path) = std::env::current_exe() {
                        cmd.env("APP_BIN", exe_path);
                    }
                    for kv in &spec.env {
                        if kv.starts_with("DIALTONE_") || kv.starts_with("DAEMON_V1_") {
                            continue;
                        }
                        if let Some((k, v)) = kv.split_once('=') {
                            cmd.env(k.trim(), v);
                        }
                    }
                    if let Some(ref cwd) = spec.cwd {
                        if !cwd.trim().is_empty() {
                            cmd.current_dir(cwd);
                        }
                    }
                    let child = cmd
                        .spawn()
                        .map_err(|e| format!("spawn managed process {}: {e}", spec.name))?;
                    spec.pid = Some(child.id());
                    spec.status = Some("running".to_string());
                    self.children.insert(
                        spec.name.clone(),
                        SupervisedChild {
                            child,
                            path: spec.path.clone(),
                            sha256: spec.sha256.clone(),
                        },
                    );
                }
            } else if let Some(mut entry) = self.children.remove(&spec.name) {
                let _ = entry.child.kill();
                let _ = entry.child.wait();
                spec.pid = None;
                spec.status = Some("stopped".to_string());
            } else {
                spec.pid = None;
                spec.status = Some("stopped".to_string());
            }
        }
        Ok(())
    }

    pub fn kill_child(&mut self, name: &str) -> bool {
        if let Some(entry) = self.children.get_mut(name) {
            let _ = entry.child.kill();
            let _ = entry.child.wait();
            true
        } else {
            false
        }
    }

    pub fn shutdown_all(&mut self) {
        for (_, mut entry) in self.children.drain() {
            let _ = entry.child.kill();
            let _ = entry.child.wait();
        }
    }
}

impl Drop for ManagedProcessSupervisor {
    fn drop(&mut self) {
        self.shutdown_all();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::blobs::store_verified_blob;
    use std::env;

    #[test]
    fn systemd_drop_in_repairs_drift_removes_retired_overrides_and_bounds_stop_timeout() {
        let dir = env::temp_dir().join(format!("dialtone-systemd-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let retired = dir.join("retired-a.conf");
        fs::write(&retired, "[Service]\nEnvironment=\"OLD=1\"\n").unwrap();
        let canonical_file = dir.join("25-daemon-intent.conf");
        fs::write(&canonical_file, "drifted-content").unwrap();

        let spec = ManagedProcessSpec {
            name: "rtcore".to_string(),
            path: "/usr/local/bin/rtcore".to_string(),
            sha256: "sha256:unknown".to_string(),
            desired_state: "stopped".to_string(),
            args: vec![
                "--nats".to_string(),
                "dialtone://node-alpha/controller/nats".to_string(),
            ],
            env: vec![
                "DIALTONE_SYSTEMD_RECONCILE_INTENT=1".to_string(),
                "APP_MODE=live".to_string(),
            ],
            cwd: None,
            startup_grace_secs: None,
            status: None,
            pid: None,
            restart_count: 0,
            service_unit: Some("rtcore.service".to_string()),
        };

        let report = reconcile_systemd_intent_drop_in(
            &dir,
            &spec,
            true,
            &["retired-a.conf".to_string()],
            5_000,
            2_000,
        )
        .unwrap();

        assert!(report.updated_drop_in);
        assert_eq!(report.removed_drop_ins, vec!["retired-a.conf".to_string()]);
        assert_eq!(report.stop_status, "stop_pending");
        let content = fs::read_to_string(&canonical_file).unwrap();
        assert!(content.contains("Environment=\"APP_MODE=live\""));
        assert!(!content.contains("DIALTONE_SYSTEMD_RECONCILE_INTENT"));
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn supervisor_spawns_and_stops_real_child_process_and_hydrates_sha() {
        let (sleep_path, sleep_args) = if cfg!(windows) {
            (
                r"C:\Windows\System32\ping.exe".to_string(),
                vec!["-n".to_string(), "6".to_string(), "127.0.0.1".to_string()],
            )
        } else {
            ("/bin/sleep".to_string(), vec!["5".to_string()])
        };
        let mut specs = vec![ManagedProcessSpec {
            name: "sleep-worker".to_string(),
            path: sleep_path,
            sha256: "sha256:unknown".to_string(),
            desired_state: "running".to_string(),
            args: sleep_args,
            env: Vec::new(),
            cwd: None,
            startup_grace_secs: None,
            status: None,
            pid: None,
            restart_count: 0,
            service_unit: None,
        }];
        hydrate_managed_process_hashes(&mut specs, &[], &HashMap::new());
        assert!(specs[0].sha256.starts_with("sha256:") && specs[0].sha256 != "sha256:unknown");

        let mut sup = ManagedProcessSupervisor::new();
        sup.reconcile(&mut specs).unwrap();
        assert_eq!(specs[0].status.as_deref(), Some("running"));
        assert!(specs[0].pid.is_some());

        specs[0].desired_state = "stopped".to_string();
        sup.reconcile(&mut specs).unwrap();
        assert_eq!(specs[0].status.as_deref(), Some("stopped"));
        assert!(specs[0].pid.is_none());
    }

    #[test]
    fn supervisor_stages_verified_blob_and_respawns_on_crash() {
        let dir = env::temp_dir().join(format!("dialtone-blob-proc-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let blob_dir = dir.join("blobs");
        fs::create_dir_all(&blob_dir).unwrap();

        let script_bytes: &[u8] = if cfg!(windows) {
            b"@echo off\r\nping -n 30 127.0.0.1 >NUL\r\n"
        } else {
            b"#!/bin/sh\nsleep 30\n"
        };
        let (sha256, _) = store_verified_blob(&blob_dir, script_bytes, None).unwrap();
        let (verified_sha, staged_path) =
            stage_executable_blob(&blob_dir, "telemetry-blob-worker", &sha256).unwrap();
        assert_eq!(verified_sha, sha256);
        assert!(staged_path.exists());

        let mut specs = vec![ManagedProcessSpec {
            name: "telemetry-blob-worker".to_string(),
            path: staged_path.to_string_lossy().to_string(),
            sha256: verified_sha,
            desired_state: "running".to_string(),
            args: Vec::new(),
            env: vec!["WORKER_ROLE=telemetry".to_string()],
            cwd: None,
            startup_grace_secs: Some(2),
            status: None,
            pid: None,
            restart_count: 0,
            service_unit: None,
        }];

        let mut sup = ManagedProcessSupervisor::new();
        sup.reconcile(&mut specs).unwrap();
        let pid1 = specs[0].pid.expect("blob subprocess should have pid");
        assert_eq!(specs[0].status.as_deref(), Some("running"));
        assert_eq!(specs[0].restart_count, 0);

        // Simulate crash/kill of the blob subprocess and verify reconcile automatically respawns it
        assert!(sup.kill_child("telemetry-blob-worker"));
        sup.reconcile(&mut specs).unwrap();
        let pid2 = specs[0]
            .pid
            .expect("blob subprocess should respawn with a pid");
        assert_eq!(specs[0].status.as_deref(), Some("running"));
        assert_ne!(pid1, pid2, "respawned blob subprocess receives a new pid");
        assert_eq!(specs[0].restart_count, 1);

        // Stop cleanly
        specs[0].desired_state = "stopped".to_string();
        sup.reconcile(&mut specs).unwrap();
        assert_eq!(specs[0].status.as_deref(), Some("stopped"));
        assert!(specs[0].pid.is_none());

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn command_journal_deduplicates_terminal_intents_and_dispatches_rtcore_udp() {
        let mut journal = Vec::new();
        let intent = RobotCommandIntentRecord {
            id: "cmd-1".to_string(),
            command_id: "arm-seq-1".to_string(),
            publisher_node: "node-alpha".to_string(),
            target_node: "node-beta".to_string(),
            service_ref: "dialtone://node-beta/realtime-core/rtcore-udp".to_string(),
            command: "arm".to_string(),
            runtime_mode: "live".to_string(),
            binding_epoch: 42,
            delivery: "at-least-once".to_string(),
            status: "pending".to_string(),
            updated_at_unix: 100,
            message: String::new(),
        };

        let bytes =
            dispatch_rtcore_udp_command_loopback(&mut journal, intent.clone(), "node-alpha")
                .unwrap();
        assert_eq!(bytes, RTCORE_PACKET_BYTES);
        assert_eq!(journal[0].status, "applied");

        let mut replay = intent;
        replay.updated_at_unix = 200;
        assert!(!upsert_command_intent(&mut journal, replay));
        assert_eq!(journal[0].status, "applied");
    }

    #[test]
    fn cursor_job_paginates_logs_and_stops_cleanly() {
        let mut job = CursorJobRecord {
            id: "cursor-1".to_string(),
            host: "node-beta".to_string(),
            remote_user: Some("operator".to_string()),
            prompt: "verify mesh convergence".to_string(),
            status: "running".to_string(),
            logs: vec![
                "line-1".to_string(),
                "line-2".to_string(),
                "line-3".to_string(),
            ],
            updated_at_unix: 100,
        };
        let page = job.log_page(0, 2);
        assert_eq!(page.lines.len(), 2);
        assert!(page.has_more);
        job.stop();
        assert_eq!(job.status, "stopped");
    }
}
