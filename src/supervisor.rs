use std::collections::HashMap;
use std::fs;
use std::net::UdpSocket;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::time::Duration;

use serde::{Deserialize, Serialize};

use crate::blob::{digest_of, normalize_digest, read_blob};
use crate::service::{InstalledProgram, ProcessSpec};

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
pub fn audit_process_dependencies(spec: &ProcessSpec) -> Vec<String> {
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
    spec: &ProcessSpec,
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
pub fn hydrate_process_digests(
    processes: &mut [ProcessSpec],
    installed: &[InstalledProgram],
    previous_paths: &HashMap<String, String>,
) {
    for proc in processes.iter_mut() {
        if let Some(prev_path) = previous_paths.get(&proc.name) {
            if prev_path != &proc.path {
                proc.digest = "sha256:unknown".to_string();
            }
        }
        if proc.digest.trim().is_empty() || proc.digest == "sha256:unknown" {
            if let Some(bin) = installed.iter().find(|b| b.path == proc.path) {
                if !bin.digest.is_empty() && bin.digest != "sha256:unknown" {
                    proc.digest = bin.digest.clone();
                    continue;
                }
            }
            if let Ok(bytes) = fs::read(&proc.path) {
                proc.digest = digest_of(&bytes);
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
    spec: &ProcessSpec,
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

/// Materialize a verified blob (`sha256:<hex>`) from `store_dir` into an executable
/// binary/script under `<store_dir>/../run/<name>-<short_hex>`, setting `0o755` permissions
/// on Unix and using atomic temp-file rename so running processes are never corrupted.
pub fn stage_program(
    store_dir: &Path,
    name: &str,
    digest: &str,
) -> Result<(String, PathBuf), String> {
    let normalized_sha = normalize_digest(digest)?;
    let bytes = read_blob(store_dir, &normalized_sha)?;
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

    let run_dir = store_dir.parent().unwrap_or(store_dir).join("run");
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

pub const PROBE_PROGRAM_NAME: &str = "probe";
/// Name the probe program had before the domain-language rename; such specs are stopped.
pub const OLD_PROBE_PROGRAM_NAME: &str = "mesh-blob-worker"; // compat

/// Every node runs its own probe program, so a peer's probe is never deployed here.
pub fn is_probe_program(name: &str) -> bool {
    name == PROBE_PROGRAM_NAME || name == OLD_PROBE_PROGRAM_NAME
}

/// Canonical cross-platform polyglot probe test process blob (`sh` on macOS/Linux, `cmd.exe` on Windows)
/// sharing an identical SHA-256 digest across all OS/arch nodes in the mesh.
/// Continuously exercises SHA-256 blobs, Key-Value store, Process supervision, Mesh connectivity, and A/B rollback.
pub fn probe_program_bytes() -> &'static [u8] {
    b":; while true; do if [ -n \"$APP_LISTEN\" ]; then \"$APP_BIN\" probe --listen \"$APP_LISTEN\"; fi; sleep 5; done; exit 0;\r\n@echo off\r\n:dialtone_probe_loop\r\nif defined APP_LISTEN \"%APP_BIN%\" probe --listen \"%APP_LISTEN%\"\r\nping -n 6 127.0.0.1 >NUL\r\ngoto dialtone_probe_loop\r\n"
}

/// Automatically store, verify, and stage the default cross-platform process blob in `store_dir`
/// and ensure its `ProcessSpec` is registered in `specs`.
pub fn ensure_probe_program(
    store_dir: &Path,
    specs: &mut Vec<ProcessSpec>,
    ticket_uri: Option<&str>,
) -> Result<ProcessSpec, String> {
    specs.retain(|p| p.name != OLD_PROBE_PROGRAM_NAME);
    let payload = probe_program_bytes();
    let (digest, _) = crate::blob::put_blob(store_dir, payload, None)?;
    let (verified_sha, staged_path) = stage_program(store_dir, PROBE_PROGRAM_NAME, &digest)?;

    let mut env_vars = vec![
        "DIALTONE_MESH_DEPLOY=1".to_string(),
        format!("PROCESS_ROLE={PROBE_PROGRAM_NAME}"),
    ];
    if let Some(t_uri) = ticket_uri.map(str::trim).filter(|s| !s.is_empty()) {
        env_vars.push(format!("DIALTONE_BLOB_TICKET={t_uri}"));
    } else if let Some(existing) = specs.iter().find(|p| p.name == PROBE_PROGRAM_NAME) {
        for kv in &existing.env {
            if kv.starts_with("DIALTONE_BLOB_TICKET=") {
                env_vars.push(kv.clone());
            }
        }
    }

    if let Some(existing) = specs.iter_mut().find(|p| p.name == PROBE_PROGRAM_NAME) {
        existing.path = staged_path.to_string_lossy().to_string();
        existing.digest = verified_sha;
        if existing.desired_state.trim().is_empty() {
            existing.desired_state = "running".to_string();
        }
        existing.env = env_vars;
        return Ok(existing.clone());
    }

    let spec = ProcessSpec {
        name: PROBE_PROGRAM_NAME.to_string(),
        path: staged_path.to_string_lossy().to_string(),
        digest: verified_sha,
        desired_state: "running".to_string(),
        args: Vec::new(),
        env: env_vars,
        cwd: None,
        startup_grace_secs: Some(2),
        status: None,
        pid: None,
        restart_count: 0,
        service_unit: None,
        deploy_ticket: None,
    };
    specs.push(spec.clone());
    Ok(spec)
}

/// A running process and, on Windows, the Job Object that owns its whole process tree.
#[derive(Debug)]
struct SupervisedChild {
    child: Child,
    path: String,
    digest: String,
    #[cfg(windows)]
    job: Option<tree::Job>,
}

/// OS process-tree control. Every process runs as the root of its own tree so the supervisor
/// can stop it *with* everything it started:
/// - Unix: the process leads a new process group; stopping signals the whole group
///   (SIGTERM, then SIGKILL after a grace period).
/// - Windows: the process is put in its own Job Object with kill-on-close; stopping
///   terminates the job, and if the node dies the OS closes the handle and kills the tree.
mod tree {
    use std::process::{Child, Command};
    use std::time::{Duration, Instant};

    pub const STOP_GRACE: Duration = Duration::from_millis(1500);

    #[cfg(unix)]
    pub fn isolate(cmd: &mut Command) {
        use std::os::unix::process::CommandExt;
        cmd.process_group(0);
    }

    #[cfg(not(unix))]
    pub fn isolate(_cmd: &mut Command) {}

    #[cfg(unix)]
    pub fn signal_group(pgid: u32, sig: libc::c_int) {
        // SAFETY: killpg only sends a signal; an already-gone group returns ESRCH.
        unsafe {
            libc::killpg(pgid as libc::pid_t, sig);
        }
    }

    fn wait_up_to(child: &mut Child, limit: Duration) -> bool {
        let started = Instant::now();
        while started.elapsed() < limit {
            if matches!(child.try_wait(), Ok(Some(_))) {
                return true;
            }
            std::thread::sleep(Duration::from_millis(25));
        }
        false
    }

    /// Stop a process and every process it started, then reap it.
    #[cfg(unix)]
    pub fn stop(child: &mut Child) {
        let pgid = child.id();
        signal_group(pgid, libc::SIGTERM);
        wait_up_to(child, STOP_GRACE);
        // Anything still in the group (including grandchildren) is killed outright.
        signal_group(pgid, libc::SIGKILL);
        let _ = child.wait();
    }

    #[cfg(windows)]
    pub fn stop(child: &mut Child, job: Option<&Job>) {
        match job {
            Some(job) => job.terminate(),
            None => {
                let _ = child.kill();
            }
        }
        wait_up_to(child, STOP_GRACE);
        let _ = child.kill();
        let _ = child.wait();
    }

    #[cfg(windows)]
    #[derive(Debug)]
    pub struct Job(windows_sys::Win32::Foundation::HANDLE);

    // SAFETY: a Job Object handle may be used and closed from any thread.
    #[cfg(windows)]
    unsafe impl Send for Job {}

    #[cfg(windows)]
    impl Job {
        /// Create a kill-on-close job and put `child` in it.
        pub fn adopt(child: &Child) -> Option<Job> {
            use std::os::windows::io::AsRawHandle;
            use windows_sys::Win32::System::JobObjects::{
                AssignProcessToJobObject, CreateJobObjectW, JobObjectExtendedLimitInformation,
                SetInformationJobObject, JOBOBJECT_EXTENDED_LIMIT_INFORMATION,
                JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
            };
            // SAFETY: plain Win32 calls with valid arguments; the handle is owned by `Job`.
            unsafe {
                let handle = CreateJobObjectW(std::ptr::null(), std::ptr::null());
                if handle.is_null() {
                    return None;
                }
                let job = Job(handle);
                let mut info: JOBOBJECT_EXTENDED_LIMIT_INFORMATION = std::mem::zeroed();
                info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
                let ok = SetInformationJobObject(
                    job.0,
                    JobObjectExtendedLimitInformation,
                    &info as *const _ as *const core::ffi::c_void,
                    std::mem::size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
                );
                if ok == 0 || AssignProcessToJobObject(job.0, child.as_raw_handle() as _) == 0 {
                    return None;
                }
                Some(job)
            }
        }

        pub fn terminate(&self) {
            // SAFETY: the handle is a live job handle owned by self.
            unsafe {
                windows_sys::Win32::System::JobObjects::TerminateJobObject(self.0, 1);
            }
        }
    }

    #[cfg(windows)]
    impl Drop for Job {
        fn drop(&mut self) {
            // SAFETY: closing the handle we own; kill-on-close then ends any remaining tree.
            unsafe {
                windows_sys::Win32::Foundation::CloseHandle(self.0);
            }
        }
    }
}

/// File in a node's `run/` directory listing the process groups it started, so a node that
/// was killed without a chance to stop them can clean them up at its next boot (Unix).
const CHILDREN_FILE: &str = "children.json";

#[derive(Debug, Default, Serialize, Deserialize)]
struct ChildRecord {
    name: String,
    pid: u32,
    path: String,
}

fn run_dir_of(spec_path: &str) -> Option<PathBuf> {
    Path::new(spec_path).parent().map(Path::to_path_buf)
}

/// Kill process trees left behind by a previous run of this node (it was SIGKILLed or crashed
/// before it could stop them). Only processes whose command still runs a program from
/// `run_dir` are touched, so a recycled PID is never signalled. Returns how many were stopped.
#[cfg(unix)]
pub fn reap_stale_processes(run_dir: &Path) -> usize {
    let file = run_dir.join(CHILDREN_FILE);
    let Ok(raw) = fs::read_to_string(&file) else {
        return 0;
    };
    let records: Vec<ChildRecord> = serde_json::from_str(&raw).unwrap_or_default();
    let run_dir_str = run_dir.to_string_lossy().to_string();
    let mut stopped = 0;
    for rec in records {
        let cmd = Command::new("ps")
            .args(["-o", "command=", "-p", &rec.pid.to_string()])
            .output()
            .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
            .unwrap_or_default();
        if !cmd.trim().is_empty() && cmd.contains(&run_dir_str) {
            tree::signal_group(rec.pid, libc::SIGKILL);
            stopped += 1;
        }
    }
    let _ = fs::remove_file(&file);
    stopped
}

/// Windows needs no reaping: each process tree is in a kill-on-close Job Object.
#[cfg(not(unix))]
pub fn reap_stale_processes(_run_dir: &Path) -> usize {
    0
}

#[derive(Debug)]
pub struct Supervisor {
    children: HashMap<String, SupervisedChild>,
    /// Every `run/` directory a process was started from (where `children.json` lives).
    run_dirs: std::collections::HashSet<PathBuf>,
}

impl Default for Supervisor {
    fn default() -> Self {
        Self::new()
    }
}

impl Supervisor {
    pub fn new() -> Self {
        Self {
            children: HashMap::new(),
            run_dirs: std::collections::HashSet::new(),
        }
    }

    fn stop_entry(entry: &mut SupervisedChild) {
        #[cfg(unix)]
        tree::stop(&mut entry.child);
        #[cfg(windows)]
        tree::stop(&mut entry.child, entry.job.as_ref());
    }

    /// Record the running process trees in each `run/` directory (see `reap_stale_processes`).
    fn write_children_files(&self) {
        let mut by_dir: HashMap<PathBuf, Vec<ChildRecord>> = self
            .run_dirs
            .iter()
            .map(|d| (d.clone(), Vec::new()))
            .collect();
        for (name, entry) in &self.children {
            if let Some(dir) = run_dir_of(&entry.path) {
                by_dir.entry(dir).or_default().push(ChildRecord {
                    name: name.clone(),
                    pid: entry.child.id(),
                    path: entry.path.clone(),
                });
            }
        }
        for (dir, records) in by_dir {
            let file = dir.join(CHILDREN_FILE);
            if records.is_empty() {
                let _ = fs::remove_file(file);
            } else if let Ok(json) = serde_json::to_vec_pretty(&records) {
                let _ = fs::write(file, json);
            }
        }
    }

    /// Bring every process to its desired state. A process that fails to start is marked
    /// `failed: <reason>` and the others are still reconciled.
    pub fn reconcile(&mut self, specs: &mut [ProcessSpec]) -> Result<(), String> {
        let before: Vec<u32> = self.children.values().map(|c| c.child.id()).collect();
        let mut failures = Vec::new();
        for spec in specs.iter_mut() {
            if spec.desired_state == "running" {
                let mut needs_spawn = true;
                if let Some(entry) = self.children.get_mut(&spec.name) {
                    if entry.path != spec.path || entry.digest != spec.digest {
                        Self::stop_entry(entry);
                        self.children.remove(&spec.name);
                    } else if matches!(entry.child.try_wait(), Ok(None)) {
                        spec.pid = Some(entry.child.id());
                        spec.status = Some("running".to_string());
                        needs_spawn = false;
                    } else {
                        spec.restart_count = spec.restart_count.saturating_add(1);
                        if let Some(mut gone) = self.children.remove(&spec.name) {
                            // The process exited; take down anything it left in its tree.
                            Self::stop_entry(&mut gone);
                        }
                    }
                }
                if needs_spawn {
                    match Self::spawn(spec) {
                        Ok(entry) => {
                            if let Some(dir) = run_dir_of(&entry.path) {
                                self.run_dirs.insert(dir);
                            }
                            spec.pid = Some(entry.child.id());
                            spec.status = Some("running".to_string());
                            self.children.insert(spec.name.clone(), entry);
                        }
                        Err(err) => {
                            spec.pid = None;
                            spec.status = Some(format!("failed: {err}"));
                            failures.push(err);
                        }
                    }
                }
            } else if let Some(mut entry) = self.children.remove(&spec.name) {
                Self::stop_entry(&mut entry);
                spec.pid = None;
                spec.status = Some("stopped".to_string());
            } else {
                spec.pid = None;
                spec.status = Some("stopped".to_string());
            }
        }
        let after: Vec<u32> = self.children.values().map(|c| c.child.id()).collect();
        if before != after {
            self.write_children_files();
        }
        if failures.is_empty() {
            Ok(())
        } else {
            Err(failures.join("; "))
        }
    }

    fn spawn(spec: &ProcessSpec) -> Result<SupervisedChild, String> {
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
        tree::isolate(&mut cmd);
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
            .map_err(|e| format!("spawn process {}: {e}", spec.name))?;
        #[cfg(windows)]
        let job = tree::Job::adopt(&child);
        Ok(SupervisedChild {
            child,
            path: spec.path.clone(),
            digest: spec.digest.clone(),
            #[cfg(windows)]
            job,
        })
    }

    pub fn kill_child(&mut self, name: &str) -> bool {
        if let Some(entry) = self.children.get_mut(name) {
            Self::stop_entry(entry);
            true
        } else {
            false
        }
    }

    /// Stop every process tree (node shutdown).
    pub fn shutdown_all(&mut self) {
        for (_, mut entry) in self.children.drain() {
            Self::stop_entry(&mut entry);
        }
        self.write_children_files();
    }
}

impl Drop for Supervisor {
    fn drop(&mut self) {
        self.shutdown_all();
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::blob::put_blob;
    use std::env;

    /// Writes a shell program into `dir/run/` that starts a background grandchild, records its
    /// pid in `dir/grandchild.pid`, and waits.
    #[cfg(unix)]
    fn tree_spec(dir: &Path, name: &str) -> ProcessSpec {
        let run = dir.join("run");
        fs::create_dir_all(&run).unwrap();
        let path = run.join(format!("{name}.sh"));
        let pid_file = dir.join("grandchild.pid");
        fs::write(
            &path,
            format!(":; sleep 60 & echo $! > {}; wait\n", pid_file.display()),
        )
        .unwrap();
        ProcessSpec {
            name: name.to_string(),
            path: path.to_string_lossy().to_string(),
            digest: "sha256:test".to_string(),
            desired_state: "running".to_string(),
            args: Vec::new(),
            env: Vec::new(),
            cwd: None,
            startup_grace_secs: None,
            status: None,
            pid: None,
            restart_count: 0,
            service_unit: None,
            deploy_ticket: None,
        }
    }

    #[cfg(unix)]
    fn alive(pid: i32) -> bool {
        // SAFETY: signal 0 only checks that the process exists.
        unsafe { libc::kill(pid, 0) == 0 }
    }

    #[cfg(unix)]
    fn wait_for_grandchild(dir: &Path) -> i32 {
        for _ in 0..100 {
            if let Ok(raw) = fs::read_to_string(dir.join("grandchild.pid")) {
                if let Ok(pid) = raw.trim().parse() {
                    return pid;
                }
            }
            std::thread::sleep(Duration::from_millis(20));
        }
        panic!("grandchild never started");
    }

    #[cfg(unix)]
    #[test]
    fn stopping_a_process_stops_everything_it_started() {
        let dir = env::temp_dir().join(format!("dialtone-tree-stop-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let mut spec = tree_spec(&dir, "tree");
        let mut sup = Supervisor::new();
        sup.reconcile(std::slice::from_mut(&mut spec)).unwrap();
        let grandchild = wait_for_grandchild(&dir);
        assert!(alive(grandchild));

        spec.desired_state = "stopped".to_string();
        sup.reconcile(std::slice::from_mut(&mut spec)).unwrap();
        std::thread::sleep(Duration::from_millis(100));
        assert!(!alive(grandchild), "grandchild survived its parent's stop");
        assert!(!dir.join("run").join(CHILDREN_FILE).exists());
        let _ = fs::remove_dir_all(&dir);
    }

    #[cfg(unix)]
    #[test]
    fn trees_left_by_a_killed_node_are_reaped_at_the_next_boot() {
        let dir = env::temp_dir().join(format!("dialtone-tree-reap-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let mut spec = tree_spec(&dir, "orphan");
        let mut sup = Supervisor::new();
        sup.reconcile(std::slice::from_mut(&mut spec)).unwrap();
        let grandchild = wait_for_grandchild(&dir);
        let child = spec.pid.unwrap() as i32;
        // Simulate SIGKILL of the node: the supervisor never gets to stop its processes.
        std::mem::forget(sup);
        assert!(alive(child) && alive(grandchild));

        assert_eq!(reap_stale_processes(&dir.join("run")), 1);
        std::thread::sleep(Duration::from_millis(100));
        // The child is a zombie of this test process until reaped; its group is gone.
        unsafe {
            libc::waitpid(child, std::ptr::null_mut(), 0);
        }
        assert!(!alive(child) && !alive(grandchild));
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn systemd_drop_in_repairs_drift_removes_retired_overrides_and_bounds_stop_timeout() {
        let dir = env::temp_dir().join(format!("dialtone-systemd-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let retired = dir.join("retired-a.conf");
        fs::write(&retired, "[Service]\nEnvironment=\"OLD=1\"\n").unwrap();
        let canonical_file = dir.join("25-daemon-intent.conf");
        fs::write(&canonical_file, "drifted-content").unwrap();

        let spec = ProcessSpec {
            name: "rtcore".to_string(),
            path: "/usr/local/bin/rtcore".to_string(),
            digest: "sha256:unknown".to_string(),
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
            deploy_ticket: None,
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
        let mut specs = vec![ProcessSpec {
            name: "sleep-process".to_string(),
            path: sleep_path,
            digest: "sha256:unknown".to_string(),
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
        hydrate_process_digests(&mut specs, &[], &HashMap::new());
        assert!(specs[0].digest.starts_with("sha256:") && specs[0].digest != "sha256:unknown");

        let mut sup = Supervisor::new();
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
        let (digest, _) = put_blob(&blob_dir, script_bytes, None).unwrap();
        let (verified_sha, staged_path) =
            stage_program(&blob_dir, "stats-blob-process", &digest).unwrap();
        assert_eq!(verified_sha, digest);
        assert!(staged_path.exists());

        let mut specs = vec![ProcessSpec {
            name: "stats-blob-process".to_string(),
            path: staged_path.to_string_lossy().to_string(),
            digest: verified_sha,
            desired_state: "running".to_string(),
            args: Vec::new(),
            env: vec!["PROCESS_ROLE=stats".to_string()],
            cwd: None,
            startup_grace_secs: Some(2),
            status: None,
            pid: None,
            restart_count: 0,
            service_unit: None,
            deploy_ticket: None,
        }];

        let mut sup = Supervisor::new();
        sup.reconcile(&mut specs).unwrap();
        let pid1 = specs[0].pid.expect("blob process should have pid");
        assert_eq!(specs[0].status.as_deref(), Some("running"));
        assert_eq!(specs[0].restart_count, 0);

        // Simulate crash/kill of the blob process and verify reconcile automatically respawns it
        assert!(sup.kill_child("stats-blob-process"));
        sup.reconcile(&mut specs).unwrap();
        let pid2 = specs[0]
            .pid
            .expect("blob process should respawn with a pid");
        assert_eq!(specs[0].status.as_deref(), Some("running"));
        assert_ne!(pid1, pid2, "respawned blob process receives a new pid");
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
