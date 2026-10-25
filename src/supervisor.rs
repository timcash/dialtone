use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};

use serde::{Deserialize, Serialize};

use crate::blob::{normalize_digest, Blobs};
use crate::service::ProcessSpec;

/// Materialize a verified blob (`sha256:<hex>`) from the store into an executable
/// binary/script under `<store dir>/../run/<name>-<short_hex>`, setting `0o755` permissions
/// on Unix and using atomic temp-file rename so running processes are never corrupted.
pub fn stage_program(blobs: &Blobs, name: &str, digest: &str) -> Result<(String, PathBuf), String> {
    let normalized_sha = normalize_digest(digest)?;
    let bytes = blobs.get_blocking(&normalized_sha)?;
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

    let store_dir = blobs.dir();
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
    // The file name carries the digest, so an identical file is already the right program.
    // Rewriting it would also fail on Windows while that program is running.
    if fs::read(&target_path).is_ok_and(|existing| existing == bytes) {
        return Ok((normalized_sha, target_path));
    }
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
/// Every node runs its own probe program, so a peer's probe is never deployed here.
pub fn is_probe_program(name: &str) -> bool {
    name == PROBE_PROGRAM_NAME
}

/// The probe program: a polyglot script (`sh` on macOS/Linux, `cmd.exe` on Windows, same
/// digest everywhere) that just stays alive. The probe checks that the supervisor staged it
/// from the store and keeps it running; the probe itself runs in the node (`probe.rs`).
pub fn probe_program_bytes() -> &'static [u8] {
    b":; while true; do sleep 5; done; exit 0;\r\n@echo off\r\n:dialtone_probe_loop\r\nping -n 6 127.0.0.1 >NUL\r\ngoto dialtone_probe_loop\r\n"
}

/// Automatically store, verify, and stage the default cross-platform process blob
/// and ensure its `ProcessSpec` is registered in `specs`.
pub fn ensure_probe_program(
    blobs: &Blobs,
    specs: &mut Vec<ProcessSpec>,
    ticket_uri: Option<&str>,
) -> Result<ProcessSpec, String> {
    let payload = probe_program_bytes();
    let digest = blobs.put_blocking(payload, None)?;
    let (verified_sha, staged_path) = stage_program(blobs, PROBE_PROGRAM_NAME, &digest)?;

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
        deployed_from: None,
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

/// Delete staged programs in `run_dir` that no process named in `keep` uses. Copies deployed
/// from peers live only in memory, so a restart leaves their files behind; ones still offered
/// are staged again under the same name. Call at boot, after reaping, before staging.
/// Returns how many files were deleted.
pub fn sweep_run_dir(run_dir: &Path, keep: &[String]) -> usize {
    // A staged file is `<name>-<12 hex>`, plus `.cmd` or `.exe` on Windows.
    let staged_for = |file: &str, name: &str| {
        let Some(rest) = file.strip_prefix(name).and_then(|r| r.strip_prefix('-')) else {
            return false;
        };
        let hex = rest.trim_end_matches(".cmd").trim_end_matches(".exe");
        hex.len() == 12 && hex.bytes().all(|b| b.is_ascii_hexdigit())
    };
    let mut deleted = 0;
    for entry in fs::read_dir(run_dir).into_iter().flatten().flatten() {
        let file = entry.file_name().to_string_lossy().to_string();
        if file == CHILDREN_FILE || !entry.path().is_file() {
            continue;
        }
        if keep.iter().any(|name| staged_for(&file, name)) {
            continue;
        }
        if fs::remove_file(entry.path()).is_ok() {
            deleted += 1;
        }
    }
    deleted
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
    use std::env;
    #[cfg(unix)]
    use std::time::Duration;

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
            deployed_from: None,
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
    fn boot_sweep_keeps_configured_programs_and_deletes_the_rest() {
        let dir = env::temp_dir().join(format!("dialtone-sweep-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        for f in [
            "probe-b4b0317fed65",
            "probe-b4b0317fed65.cmd",
            "web-0123456789ab.exe",
            "probe-slot-a-6d4605c87fb6",
            "deploy-check-3eb500702822.cmd",
            ".web-0123456789ab.tmp.42",
            CHILDREN_FILE,
        ] {
            fs::write(dir.join(f), b"x").unwrap();
        }
        let keep = vec![PROBE_PROGRAM_NAME.to_string(), "web".to_string()];
        assert_eq!(sweep_run_dir(&dir, &keep), 3);
        let mut left: Vec<String> = fs::read_dir(&dir)
            .unwrap()
            .flatten()
            .map(|e| e.file_name().to_string_lossy().to_string())
            .collect();
        left.sort();
        assert_eq!(
            left,
            vec![
                CHILDREN_FILE,
                "probe-b4b0317fed65",
                "probe-b4b0317fed65.cmd",
                "web-0123456789ab.exe"
            ]
        );
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn supervisor_spawns_and_stops_a_real_process() {
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
            deployed_from: None,
        }];
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
        let blobs = Blobs::open_blocking(&dir.join("blobs")).unwrap();

        let script_bytes: &[u8] = if cfg!(windows) {
            b"@echo off\r\nping -n 30 127.0.0.1 >NUL\r\n"
        } else {
            b"#!/bin/sh\nsleep 30\n"
        };
        let digest = blobs.put_blocking(script_bytes, None).unwrap();
        let (verified_sha, staged_path) =
            stage_program(&blobs, "stats-blob-process", &digest).unwrap();
        // Staging the same program again keeps the file it already staged.
        let first_staged = fs::metadata(&staged_path).unwrap().modified().unwrap();
        let (_, again) = stage_program(&blobs, "stats-blob-process", &digest).unwrap();
        assert_eq!(again, staged_path);
        assert_eq!(
            fs::metadata(&again).unwrap().modified().unwrap(),
            first_staged
        );
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
            deployed_from: None,
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
}
