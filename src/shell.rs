//! Persistent cross-OS terminal session manager over Dialtone QUIC (ALPN `dialtone/shell/1`).
//!
//! Installed and gated via the blob-backed `shell` plugin (`dialtone install shell`).
//! Replaces SSH + tmux on macOS, Windows, and Linux/WSL by keeping named, long-running
//! `bash` or `powershell` processes alive per session, preserving working directory (`cd`),
//! environment/shell variables, and bounded scrollback history (`capture`).

use std::collections::{HashMap, VecDeque};
use std::io::{BufRead, BufReader, Write};
use std::path::{Path, PathBuf};
use std::process::{Child, ChildStdin, Command, Stdio};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use iroh::endpoint::Connection;
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::{Endpoint, EndpointId};
use serde::{Deserialize, Serialize};

use crate::blob::Blobs;
use crate::exec::Shell;
use crate::now_unix;
use crate::plugin::{is_plugin_installed_and_verified, SHELL_PLUGIN_NAME};

pub const ALPN: &[u8] = b"dialtone/shell/1";
pub const ALPN_STR: &str = "dialtone/shell/1";

const MAX_REQUEST_BYTES: usize = 256 << 10;
const MAX_RESPONSE_BYTES: usize = 4 << 20;
pub const DEFAULT_TIMEOUT_SECS: u64 = 30;
pub const MAX_TIMEOUT_SECS: u64 = 600;
const MAX_SCROLLBACK_LINES: usize = 2000;
const MAX_SCROLLBACK_BYTES: usize = 512 << 10;
const MAX_CAPTURE_BYTES_PER_CMD: usize = 1 << 20;
const MAX_SESSIONS: usize = 32;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "action", rename_all = "lowercase")]
pub enum ShellRequest {
    New {
        name: String,
        #[serde(default)]
        shell: Shell,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        cwd: Option<String>,
    },
    Send {
        name: String,
        command: String,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        timeout_secs: Option<u64>,
    },
    Capture {
        name: String,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        lines: Option<usize>,
    },
    List,
    Kill {
        name: String,
    },
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct SessionInfo {
    pub name: String,
    pub node: String,
    pub os: String,
    pub shell: String,
    pub pid: u32,
    pub alive: bool,
    pub commands_run: u64,
    pub scrollback_lines: usize,
    pub created_at_unix: u64,
    pub last_active_unix: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ShellSendResult {
    pub name: String,
    pub node: String,
    pub os: String,
    pub shell: String,
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
    pub timed_out: bool,
    pub elapsed_ms: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ShellCaptureResult {
    pub name: String,
    pub node: String,
    pub os: String,
    pub shell: String,
    pub total_lines: usize,
    pub lines: Vec<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "lowercase")]
pub enum ShellResponse {
    Created {
        session: SessionInfo,
    },
    Sent {
        result: ShellSendResult,
    },
    Captured {
        capture: ShellCaptureResult,
    },
    Listed {
        node: String,
        sessions: Vec<SessionInfo>,
    },
    Killed {
        name: String,
        node: String,
    },
    Error {
        error: String,
    },
}

pub fn validate_session_name(raw: &str) -> Result<String, String> {
    let name = raw.trim().to_string();
    if name.is_empty() || name.len() > 64 {
        return Err(format!(
            "invalid session name {raw:?}: must be 1..=64 ASCII characters"
        ));
    }
    let bytes = name.as_bytes();
    if !bytes[0].is_ascii_alphanumeric()
        || !bytes
            .iter()
            .all(|b| b.is_ascii_alphanumeric() || *b == b'-' || *b == b'_' || *b == b'.')
    {
        return Err(format!(
            "invalid session name {raw:?}: only [A-Za-z0-9._-] allowed"
        ));
    }
    Ok(name)
}

fn clamp_timeout(timeout_secs: Option<u64>) -> Duration {
    Duration::from_secs(
        timeout_secs
            .unwrap_or(DEFAULT_TIMEOUT_SECS)
            .clamp(1, MAX_TIMEOUT_SECS),
    )
}

fn resolve_persistent_shell(
    shell: Shell,
) -> Result<(&'static str, String, Vec<&'static str>), String> {
    let effective = match shell {
        Shell::Auto => {
            if cfg!(windows) {
                Shell::Powershell
            } else {
                Shell::Bash
            }
        }
        other => other,
    };
    match effective {
        Shell::Powershell => {
            let exe =
                find_executable(&["pwsh", "powershell", "powershell.exe"]).ok_or_else(|| {
                    "PowerShell executable (pwsh or powershell) not found on PATH".to_string()
                })?;
            Ok((
                "powershell",
                exe,
                vec!["-NoLogo", "-NoProfile", "-NoExit", "-Command", "-"],
            ))
        }
        Shell::Bash => {
            let candidates: &[&str] = if cfg!(windows) {
                &[
                    "C:\\Program Files\\Git\\bin\\bash.exe",
                    "C:\\Program Files\\Git\\usr\\bin\\bash.exe",
                    "bash",
                ]
            } else {
                &["bash", "/bin/bash", "/usr/bin/bash", "sh", "/bin/sh"]
            };
            let exe = find_executable(candidates)
                .ok_or_else(|| "bash executable not found on this host".to_string())?;
            let is_bash = Path::new(&exe)
                .file_stem()
                .and_then(|s| s.to_str())
                .map(|s| s.eq_ignore_ascii_case("bash"))
                .unwrap_or(false);
            let args = if is_bash {
                vec!["--norc", "--noprofile"]
            } else {
                Vec::new()
            };
            Ok(("bash", exe, args))
        }
        Shell::Auto => unreachable!(),
    }
}

fn find_executable(candidates: &[&str]) -> Option<String> {
    for candidate in candidates {
        let p = Path::new(candidate);
        if p.is_absolute() {
            if p.is_file() {
                return Some(candidate.to_string());
            }
            continue;
        }
        if let Some(found) = look_path(candidate) {
            return Some(found);
        }
    }
    None
}

fn look_path(name: &str) -> Option<String> {
    let path_var = std::env::var_os("PATH")?;
    let exts: Vec<String> = if cfg!(windows) {
        let pathext = std::env::var("PATHEXT").unwrap_or_else(|_| ".COM;.EXE;.BAT;.CMD".into());
        let mut list = vec![String::new()];
        list.extend(
            pathext
                .split(';')
                .filter(|s| !s.is_empty())
                .map(|s| s.to_ascii_lowercase()),
        );
        list
    } else {
        vec![String::new()]
    };
    for dir in std::env::split_paths(&path_var) {
        for ext in &exts {
            let candidate = if ext.is_empty() || name.to_ascii_lowercase().ends_with(ext) {
                dir.join(name)
            } else {
                dir.join(format!("{name}{ext}"))
            };
            if candidate.is_file() {
                return Some(candidate.to_string_lossy().to_string());
            }
        }
    }
    None
}

#[derive(Debug)]
struct ScrollbackBuffer {
    lines: VecDeque<String>,
    total_bytes: usize,
    max_lines: usize,
    max_bytes: usize,
}

impl ScrollbackBuffer {
    fn new(max_lines: usize, max_bytes: usize) -> Self {
        Self {
            lines: VecDeque::new(),
            total_bytes: 0,
            max_lines: max_lines.max(1),
            max_bytes: max_bytes.max(1024),
        }
    }

    fn push_line(&mut self, line: String) {
        let len = line.len();
        self.total_bytes = self.total_bytes.saturating_add(len);
        self.lines.push_back(line);
        while self.lines.len() > self.max_lines
            || (self.total_bytes > self.max_bytes && self.lines.len() > 1)
        {
            if let Some(old) = self.lines.pop_front() {
                self.total_bytes = self.total_bytes.saturating_sub(old.len());
            }
        }
    }

    fn tail(&self, count: usize) -> (usize, Vec<String>) {
        let total = self.lines.len();
        let take = count.min(total);
        let skip = total.saturating_sub(take);
        let slice = self.lines.iter().skip(skip).cloned().collect();
        (total, slice)
    }

    fn len(&self) -> usize {
        self.lines.len()
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum StreamKind {
    Stdout,
    Stderr,
}

struct StreamLine {
    kind: StreamKind,
    line: String,
}

struct SessionRuntime {
    child: Child,
    stdin: ChildStdin,
    rx: mpsc::Receiver<StreamLine>,
    alive: bool,
    commands_run: u64,
    last_active_unix: u64,
    tainted: bool,
}

impl Drop for SessionRuntime {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

struct PersistentSession {
    name: String,
    node: String,
    shell_label: String,
    pid: u32,
    created_at_unix: u64,
    scrollback: Arc<Mutex<ScrollbackBuffer>>,
    runtime: Mutex<SessionRuntime>,
}

impl PersistentSession {
    fn spawn(
        name: String,
        node: String,
        shell: Shell,
        cwd: Option<&str>,
        max_scrollback_lines: usize,
        max_scrollback_bytes: usize,
    ) -> Result<Self, String> {
        let (shell_label, exe, args) = resolve_persistent_shell(shell)?;
        let mut cmd = Command::new(&exe);
        cmd.args(&args)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped());
        if let Some(dir) = cwd {
            let trimmed = dir.trim();
            if !trimmed.is_empty() {
                let p = Path::new(trimmed);
                if !p.is_dir() {
                    return Err(format!("cwd '{trimmed}' is not an existing directory"));
                }
                cmd.current_dir(p);
            }
        }
        let mut child = cmd
            .spawn()
            .map_err(|e| format!("spawn persistent {shell_label} ({exe}): {e}"))?;
        let pid = child.id();
        let stdin = child
            .stdin
            .take()
            .ok_or_else(|| "missing stdin pipe for persistent shell".to_string())?;
        let stdout = child
            .stdout
            .take()
            .ok_or_else(|| "missing stdout pipe for persistent shell".to_string())?;
        let stderr = child
            .stderr
            .take()
            .ok_or_else(|| "missing stderr pipe for persistent shell".to_string())?;

        let scrollback = Arc::new(Mutex::new(ScrollbackBuffer::new(
            max_scrollback_lines,
            max_scrollback_bytes,
        )));
        let (tx, rx) = mpsc::channel::<StreamLine>();

        spawn_reader_thread(stdout, StreamKind::Stdout, tx.clone());
        spawn_reader_thread(stderr, StreamKind::Stderr, tx);

        let now = now_unix();
        Ok(Self {
            name,
            node,
            shell_label: shell_label.to_string(),
            pid,
            created_at_unix: now,
            scrollback,
            runtime: Mutex::new(SessionRuntime {
                child,
                stdin,
                rx,
                alive: true,
                commands_run: 0,
                last_active_unix: now,
                tainted: false,
            }),
        })
    }

    fn info(&self) -> SessionInfo {
        let sb_lines = self.scrollback.lock().map(|b| b.len()).unwrap_or(0);
        if let Ok(mut rt) = self.runtime.try_lock() {
            if rt.alive {
                if let Ok(Some(_)) = rt.child.try_wait() {
                    rt.alive = false;
                }
            }
            SessionInfo {
                name: self.name.clone(),
                node: self.node.clone(),
                os: std::env::consts::OS.to_string(),
                shell: self.shell_label.clone(),
                pid: self.pid,
                alive: rt.alive,
                commands_run: rt.commands_run,
                scrollback_lines: sb_lines,
                created_at_unix: self.created_at_unix,
                last_active_unix: rt.last_active_unix,
            }
        } else {
            // Command currently executing on this session; report busy/alive without blocking `ls`.
            SessionInfo {
                name: self.name.clone(),
                node: self.node.clone(),
                os: std::env::consts::OS.to_string(),
                shell: self.shell_label.clone(),
                pid: self.pid,
                alive: true,
                commands_run: 0,
                scrollback_lines: sb_lines,
                created_at_unix: self.created_at_unix,
                last_active_unix: now_unix(),
            }
        }
    }

    fn capture(&self, lines: usize) -> ShellCaptureResult {
        let (total, tail) = self
            .scrollback
            .lock()
            .map(|b| b.tail(lines))
            .unwrap_or_default();
        ShellCaptureResult {
            name: self.name.clone(),
            node: self.node.clone(),
            os: std::env::consts::OS.to_string(),
            shell: self.shell_label.clone(),
            total_lines: total,
            lines: tail,
        }
    }

    fn send(&self, command: &str, timeout_secs: Option<u64>) -> Result<ShellSendResult, String> {
        static NONCE: AtomicU64 = AtomicU64::new(1);
        if command.trim().is_empty() {
            return Err("shell command must not be empty".to_string());
        }
        let timeout = clamp_timeout(timeout_secs);
        let started = Instant::now();
        let mut rt = self
            .runtime
            .lock()
            .map_err(|_| "shell session lock poisoned".to_string())?;

        if !rt.alive || rt.tainted {
            return Err(format!(
                "session '{}' is no longer active (previously exited or timed out); kill and recreate it",
                self.name
            ));
        }
        if let Ok(Some(_)) = rt.child.try_wait() {
            rt.alive = false;
            return Err(format!(
                "session '{}' shell process has already exited",
                self.name
            ));
        }

        // Drain any stale asynchronous lines before dispatching the new command.
        while let Ok(stale) = rt.rx.try_recv() {
            if let Ok(mut sb) = self.scrollback.lock() {
                sb.push_line(stale.line);
            }
        }

        if let Ok(mut sb) = self.scrollback.lock() {
            sb.push_line(format!("$ {}", command.trim()));
        }

        let id = NONCE.fetch_add(1, Ordering::Relaxed);
        let marker = format!("__DT_DONE_{}_{}__:", std::process::id(), id);
        let normalized_cmd = command.replace("\r\n", "\n");
        let script = if self.shell_label == "powershell" {
            format!(
                "$global:LASTEXITCODE = 0\n{normalized_cmd}\n$__dt_ec = if ($null -ne $global:LASTEXITCODE -and [int]$global:LASTEXITCODE -ne 0) {{ [int]$global:LASTEXITCODE }} elseif (-not $?) {{ 1 }} else {{ 0 }}\n[Console]::Out.WriteLine('{marker}' + $__dt_ec)\n[Console]::Error.WriteLine('{marker}' + $__dt_ec)\n[Console]::Out.Flush()\n[Console]::Error.Flush()\n"
            )
        } else {
            format!(
                "{normalized_cmd}\n__dt_ec=$?\nprintf '\\n{marker}%d\\n' \"$__dt_ec\"\nprintf '\\n{marker}%d\\n' \"$__dt_ec\" >&2\n"
            )
        };

        rt.stdin
            .write_all(script.as_bytes())
            .and_then(|_| rt.stdin.flush())
            .map_err(|e| {
                rt.alive = false;
                format!("write command to session '{}': {e}", self.name)
            })?;

        let deadline = started + timeout;
        let mut stdout_buf = String::new();
        let mut stderr_buf = String::new();
        let mut stdout_done = false;
        let mut stderr_done = false;
        let mut exit_code: i32 = 0;
        let mut timed_out = false;

        while !(stdout_done && stderr_done) {
            let now = Instant::now();
            if now >= deadline {
                timed_out = true;
                rt.tainted = true;
                rt.alive = false;
                let _ = rt.child.kill();
                let _ = rt.child.wait();
                break;
            }
            let remaining = deadline - now;
            match rt.rx.recv_timeout(remaining) {
                Ok(item) => {
                    let clean = item.line.trim_end_matches('\r').to_string();
                    if let Some((prefix, code_str)) = split_marker(&clean, &marker) {
                        if !prefix.is_empty() {
                            push_bounded_output(
                                item.kind,
                                prefix,
                                &mut stdout_buf,
                                &mut stderr_buf,
                                &self.scrollback,
                            );
                        }
                        if let Ok(parsed) = code_str.trim().parse::<i32>() {
                            exit_code = parsed;
                        }
                        match item.kind {
                            StreamKind::Stdout => stdout_done = true,
                            StreamKind::Stderr => stderr_done = true,
                        }
                        continue;
                    }
                    push_bounded_output(
                        item.kind,
                        &clean,
                        &mut stdout_buf,
                        &mut stderr_buf,
                        &self.scrollback,
                    );
                }
                Err(mpsc::RecvTimeoutError::Timeout) => {
                    timed_out = true;
                    rt.tainted = true;
                    rt.alive = false;
                    let _ = rt.child.kill();
                    let _ = rt.child.wait();
                    break;
                }
                Err(mpsc::RecvTimeoutError::Disconnected) => {
                    rt.alive = false;
                    if let Ok(Some(status)) = rt.child.try_wait() {
                        exit_code = status.code().unwrap_or(1);
                    }
                    break;
                }
            }
        }

        rt.commands_run += 1;
        rt.last_active_unix = now_unix();
        if timed_out {
            exit_code = -1;
        }

        Ok(ShellSendResult {
            name: self.name.clone(),
            node: self.node.clone(),
            os: std::env::consts::OS.to_string(),
            shell: self.shell_label.clone(),
            exit_code,
            stdout: trim_trailing_sentinel_blank(stdout_buf),
            stderr: trim_trailing_sentinel_blank(stderr_buf),
            timed_out,
            elapsed_ms: started.elapsed().as_millis() as u64,
        })
    }

    fn kill(&self) {
        if let Ok(mut rt) = self.runtime.lock() {
            rt.alive = false;
            let _ = rt.child.kill();
            let _ = rt.child.wait();
        }
    }
}

fn split_marker<'a>(line: &'a str, marker: &str) -> Option<(&'a str, &'a str)> {
    let idx = line.find(marker)?;
    Some((&line[..idx], &line[idx + marker.len()..]))
}

fn trim_trailing_sentinel_blank(mut s: String) -> String {
    while s.ends_with("\n\n") {
        s.pop();
    }
    s
}

fn push_bounded_output(
    kind: StreamKind,
    line: &str,
    stdout_buf: &mut String,
    stderr_buf: &mut String,
    scrollback: &Arc<Mutex<ScrollbackBuffer>>,
) {
    // Filter PowerShell interactive prompt echoes if any leak on stderr/stdout.
    if line.starts_with("PS ") && line.ends_with("> ") {
        return;
    }
    let target = match kind {
        StreamKind::Stdout => stdout_buf,
        StreamKind::Stderr => stderr_buf,
    };
    if target.len() < MAX_CAPTURE_BYTES_PER_CMD {
        target.push_str(line);
        target.push('\n');
    }
    if let Ok(mut sb) = scrollback.lock() {
        let entry = match kind {
            StreamKind::Stdout => line.to_string(),
            StreamKind::Stderr => format!("[stderr] {line}"),
        };
        sb.push_line(entry);
    }
}

fn spawn_reader_thread<R: std::io::Read + Send + 'static>(
    reader: R,
    kind: StreamKind,
    tx: mpsc::Sender<StreamLine>,
) {
    thread::spawn(move || {
        let mut br = BufReader::new(reader);
        let mut buf = Vec::new();
        loop {
            buf.clear();
            match br.read_until(b'\n', &mut buf) {
                Ok(0) => break,
                Ok(_) => {
                    while matches!(buf.last(), Some(b'\n' | b'\r')) {
                        buf.pop();
                    }
                    let line = String::from_utf8_lossy(&buf).to_string();
                    if tx.send(StreamLine { kind, line }).is_err() {
                        break;
                    }
                }
                Err(_) => break,
            }
        }
    });
}

/// Manages all named persistent terminal sessions on a Dialtone node.
pub struct ShellManager {
    node_name: String,
    max_scrollback_lines: usize,
    max_scrollback_bytes: usize,
    sessions: Mutex<HashMap<String, Arc<PersistentSession>>>,
}

impl std::fmt::Debug for ShellManager {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("ShellManager")
            .field("node_name", &self.node_name)
            .finish()
    }
}

impl ShellManager {
    pub fn new(node_name: impl Into<String>) -> Self {
        Self::with_limits(node_name, MAX_SCROLLBACK_LINES, MAX_SCROLLBACK_BYTES)
    }

    pub fn with_limits(
        node_name: impl Into<String>,
        max_scrollback_lines: usize,
        max_scrollback_bytes: usize,
    ) -> Self {
        Self {
            node_name: node_name.into(),
            max_scrollback_lines,
            max_scrollback_bytes,
            sessions: Mutex::new(HashMap::new()),
        }
    }

    pub fn node_name(&self) -> &str {
        &self.node_name
    }

    pub fn create_session(
        &self,
        raw_name: &str,
        shell: Shell,
        cwd: Option<&str>,
    ) -> Result<SessionInfo, String> {
        let name = validate_session_name(raw_name)?;
        {
            let map = self
                .sessions
                .lock()
                .map_err(|_| "shell manager lock poisoned".to_string())?;
            if map.contains_key(&name) {
                return Err(format!("shell session '{name}' already exists"));
            }
            if map.len() >= MAX_SESSIONS {
                return Err(format!(
                    "maximum concurrent shell sessions ({MAX_SESSIONS}) reached"
                ));
            }
        }
        let session = Arc::new(PersistentSession::spawn(
            name.clone(),
            self.node_name.clone(),
            shell,
            cwd,
            self.max_scrollback_lines,
            self.max_scrollback_bytes,
        )?);
        let info = session.info();
        let mut map = self
            .sessions
            .lock()
            .map_err(|_| "shell manager lock poisoned".to_string())?;
        if map.contains_key(&name) {
            session.kill();
            return Err(format!("shell session '{name}' already exists"));
        }
        map.insert(name, session);
        Ok(info)
    }

    pub fn send_command(
        &self,
        raw_name: &str,
        command: &str,
        timeout_secs: Option<u64>,
    ) -> Result<ShellSendResult, String> {
        let name = validate_session_name(raw_name)?;
        let session = {
            let map = self
                .sessions
                .lock()
                .map_err(|_| "shell manager lock poisoned".to_string())?;
            map.get(&name).cloned().ok_or_else(|| {
                format!(
                    "shell session '{name}' not found; create it with 'dialtone shell new {name}'"
                )
            })?
        };
        session.send(command, timeout_secs)
    }

    pub fn capture(
        &self,
        raw_name: &str,
        lines: Option<usize>,
    ) -> Result<ShellCaptureResult, String> {
        let name = validate_session_name(raw_name)?;
        let session = {
            let map = self
                .sessions
                .lock()
                .map_err(|_| "shell manager lock poisoned".to_string())?;
            map.get(&name)
                .cloned()
                .ok_or_else(|| format!("shell session '{name}' not found"))?
        };
        let count = lines.unwrap_or(100).clamp(1, self.max_scrollback_lines);
        Ok(session.capture(count))
    }

    pub fn list_sessions(&self) -> Vec<SessionInfo> {
        let snapshot: Vec<Arc<PersistentSession>> = self
            .sessions
            .lock()
            .map(|m| m.values().cloned().collect())
            .unwrap_or_default();
        let mut out: Vec<SessionInfo> = snapshot.into_iter().map(|s| s.info()).collect();
        out.sort_by(|a, b| a.name.cmp(&b.name));
        out
    }

    pub fn kill_session(&self, raw_name: &str) -> Result<String, String> {
        let name = validate_session_name(raw_name)?;
        let removed = {
            let mut map = self
                .sessions
                .lock()
                .map_err(|_| "shell manager lock poisoned".to_string())?;
            map.remove(&name)
        };
        match removed {
            Some(sess) => {
                sess.kill();
                Ok(name)
            }
            None => Err(format!("shell session '{name}' not found")),
        }
    }

    pub fn handle_request(&self, req: ShellRequest) -> ShellResponse {
        match req {
            ShellRequest::New { name, shell, cwd } => {
                match self.create_session(&name, shell, cwd.as_deref()) {
                    Ok(session) => ShellResponse::Created { session },
                    Err(error) => ShellResponse::Error { error },
                }
            }
            ShellRequest::Send {
                name,
                command,
                timeout_secs,
            } => match self.send_command(&name, &command, timeout_secs) {
                Ok(result) => ShellResponse::Sent { result },
                Err(error) => ShellResponse::Error { error },
            },
            ShellRequest::Capture { name, lines } => match self.capture(&name, lines) {
                Ok(capture) => ShellResponse::Captured { capture },
                Err(error) => ShellResponse::Error { error },
            },
            ShellRequest::List => ShellResponse::Listed {
                node: self.node_name.clone(),
                sessions: self.list_sessions(),
            },
            ShellRequest::Kill { name } => match self.kill_session(&name) {
                Ok(killed) => ShellResponse::Killed {
                    name: killed,
                    node: self.node_name.clone(),
                },
                Err(error) => ShellResponse::Error { error },
            },
        }
    }
}

impl Drop for ShellManager {
    fn drop(&mut self) {
        if let Ok(mut map) = self.sessions.lock() {
            for (_, sess) in map.drain() {
                sess.kill();
            }
        }
    }
}

/// QUIC protocol handler for `dialtone/shell/1`, gated by verified `shell` plugin installation.
#[derive(Clone, Debug)]
pub struct ShellHandler {
    manager: Arc<ShellManager>,
    blobs: Blobs,
    plugins_dir: PathBuf,
}

impl ShellHandler {
    pub fn new(manager: Arc<ShellManager>, blobs: Blobs, plugins_dir: PathBuf) -> Self {
        Self {
            manager,
            blobs,
            plugins_dir,
        }
    }

    async fn serve(&self, conn: Connection) -> Result<(), String> {
        while let Ok((mut send, mut recv)) = conn.accept_bi().await {
            let raw = recv
                .read_to_end(MAX_REQUEST_BYTES)
                .await
                .map_err(|e| format!("read shell request: {e}"))?;
            let req: ShellRequest = match serde_json::from_slice(&raw) {
                Ok(r) => r,
                Err(e) => {
                    let payload = serde_json::to_vec(&ShellResponse::Error {
                        error: format!("decode shell request: {e}"),
                    })
                    .unwrap_or_default();
                    let _ = send.write_all(&payload).await;
                    let _ = send.finish();
                    continue;
                }
            };

            let blobs = self.blobs.clone();
            let plugins_dir = self.plugins_dir.clone();
            let manager = self.manager.clone();

            let resp = tokio::task::spawn_blocking(move || {
                if !is_plugin_installed_and_verified(&blobs, &plugins_dir, SHELL_PLUGIN_NAME) {
                    return ShellResponse::Error {
                        error: "plugin 'shell' is not installed; run 'dialtone install shell'"
                            .to_string(),
                    };
                }
                manager.handle_request(req)
            })
            .await
            .map_err(|e| format!("shell task join: {e}"))?;

            let payload =
                serde_json::to_vec(&resp).map_err(|e| format!("encode shell response: {e}"))?;
            send.write_all(&payload)
                .await
                .map_err(|e| format!("write shell response: {e}"))?;
            let _ = send.finish();
        }
        Ok(())
    }
}

impl ProtocolHandler for ShellHandler {
    async fn accept(&self, conn: Connection) -> Result<(), AcceptError> {
        let _ = self.serve(conn).await;
        Ok(())
    }
}

/// Send a `ShellRequest` to an admitted peer over QUIC (`dialtone/shell/1`).
pub async fn remote_shell(
    endpoint: &Endpoint,
    peer: EndpointId,
    request: &ShellRequest,
) -> Result<ShellResponse, String> {
    let connect_timeout = Duration::from_secs(10);
    let conn = tokio::time::timeout(connect_timeout, endpoint.connect(peer, ALPN))
        .await
        .map_err(|_| format!("connect to {peer} for shell timed out"))?
        .map_err(|e| format!("connect to {peer} for shell: {e}"))?;
    let (mut send, mut recv) = conn
        .open_bi()
        .await
        .map_err(|e| format!("open shell stream to {peer}: {e}"))?;
    let payload = serde_json::to_vec(request).map_err(|e| format!("encode shell request: {e}"))?;
    send.write_all(&payload)
        .await
        .map_err(|e| format!("write shell request: {e}"))?;
    send.finish()
        .map_err(|e| format!("finish shell request: {e}"))?;

    let wait_secs = match request {
        ShellRequest::Send { timeout_secs, .. } => {
            clamp_timeout(*timeout_secs).as_secs().saturating_add(15)
        }
        _ => 20,
    };
    let raw = tokio::time::timeout(
        Duration::from_secs(wait_secs),
        recv.read_to_end(MAX_RESPONSE_BYTES),
    )
    .await
    .map_err(|_| format!("remote shell response from {peer} timed out"))?
    .map_err(|e| format!("read shell response from {peer}: {e}"))?;
    conn.close(0u32.into(), b"done");
    serde_json::from_slice(&raw).map_err(|e| format!("decode shell response: {e}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::mesh::LiveAllowlist;
    use crate::node::{build_endpoint_builder, AllowlistHook, IrohTransportConfig};
    use crate::plugin::install_builtin_plugin;
    use iroh::address_lookup::memory::MemoryLookup;
    use iroh::protocol::Router;
    use std::fs;

    fn temp_dir(tag: &str) -> PathBuf {
        static NEXT: AtomicU64 = AtomicU64::new(1);
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let p = std::env::temp_dir().join(format!(
            "dialtone-shell-test-{tag}-{}-{id}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&p);
        fs::create_dir_all(&p).unwrap();
        p
    }

    #[test]
    fn shell_session_preserves_cwd_and_env_state_across_commands() {
        let dir = temp_dir("state");
        let sub = dir.join("nested_work");
        fs::create_dir_all(&sub).unwrap();

        let mgr = ShellManager::new("local-node");
        mgr.create_session("main", Shell::Auto, Some(&dir.to_string_lossy()))
            .unwrap();

        let (cmd1, cmd2) = if cfg!(windows) {
            (
                "Set-Location nested_work; $env:DIALTONE_TEST_VAR = 'hello_mesh'",
                "Write-Output \"CWD=$((Get-Location).Path)|VAR=$env:DIALTONE_TEST_VAR\"",
            )
        } else {
            (
                "cd nested_work && export DIALTONE_TEST_VAR=hello_mesh",
                "printf 'CWD=%s|VAR=%s\\n' \"$PWD\" \"$DIALTONE_TEST_VAR\"",
            )
        };

        let r1 = mgr.send_command("main", cmd1, Some(15)).unwrap();
        assert_eq!(r1.exit_code, 0, "cmd1 stderr={}", r1.stderr);
        assert!(!r1.timed_out);

        let r2 = mgr.send_command("main", cmd2, Some(15)).unwrap();
        assert_eq!(r2.exit_code, 0, "cmd2 stderr={}", r2.stderr);
        assert!(
            r2.stdout.contains("nested_work"),
            "cwd did not persist: {}",
            r2.stdout
        );
        assert!(
            r2.stdout.contains("VAR=hello_mesh"),
            "env var did not persist: {}",
            r2.stdout
        );

        drop(mgr);
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn shell_session_supports_powershell_and_bash_on_current_os() {
        let mgr = ShellManager::new("os-node");

        // PowerShell (always available on Windows; optional on Unix if pwsh installed).
        if resolve_persistent_shell(Shell::Powershell).is_ok() {
            mgr.create_session("ps1", Shell::Powershell, None).unwrap();
            let ok = mgr
                .send_command(
                    "ps1",
                    "$env:DT_PS = 'ps_ok'; Write-Output $env:DT_PS",
                    Some(15),
                )
                .unwrap();
            assert_eq!(ok.shell, "powershell");
            assert_eq!(ok.exit_code, 0);
            assert!(ok.stdout.contains("ps_ok"), "stdout={}", ok.stdout);

            let fail = mgr.send_command("ps1", "cmd /c exit 7", Some(15)).unwrap();
            assert_eq!(fail.exit_code, 7, "expected exit 7, got {:?}", fail);
        }

        // Bash (always available on macOS/Linux/WSL; Git Bash on Windows).
        if resolve_persistent_shell(Shell::Bash).is_ok() {
            mgr.create_session("sh1", Shell::Bash, None).unwrap();
            let ok = mgr
                .send_command("sh1", "export DT_SH=bash_ok; echo $DT_SH", Some(15))
                .unwrap();
            assert_eq!(ok.shell, "bash");
            assert_eq!(ok.exit_code, 0);
            assert!(ok.stdout.contains("bash_ok"), "stdout={}", ok.stdout);

            let fail = mgr.send_command("sh1", "(exit 9)", Some(15)).unwrap();
            assert_eq!(fail.exit_code, 9, "expected exit 9, got {:?}", fail);
        }

        // At least one native shell must have been exercised on every platform.
        assert!(!mgr.list_sessions().is_empty());
    }

    #[test]
    fn shell_multi_session_isolation_list_and_kill() {
        let mgr = ShellManager::new("iso-node");
        mgr.create_session("build", Shell::Auto, None).unwrap();
        mgr.create_session("dev", Shell::Auto, None).unwrap();

        let (set_b, set_d, read_v) = if cfg!(windows) {
            (
                "$env:DT_ROLE = 'builder'",
                "$env:DT_ROLE = 'developer'",
                "Write-Output $env:DT_ROLE",
            )
        } else {
            (
                "export DT_ROLE=builder",
                "export DT_ROLE=developer",
                "echo $DT_ROLE",
            )
        };
        mgr.send_command("build", set_b, Some(15)).unwrap();
        mgr.send_command("dev", set_d, Some(15)).unwrap();

        let b_out = mgr.send_command("build", read_v, Some(15)).unwrap();
        let d_out = mgr.send_command("dev", read_v, Some(15)).unwrap();
        assert!(b_out.stdout.contains("builder"));
        assert!(d_out.stdout.contains("developer"));

        let list = mgr.list_sessions();
        assert_eq!(list.len(), 2);
        assert_eq!(list[0].name, "build");
        assert_eq!(list[1].name, "dev");
        assert!(list[0].alive && list[1].alive);
        assert_eq!(list[0].commands_run, 2);

        mgr.kill_session("build").unwrap();
        assert!(mgr.send_command("build", read_v, Some(5)).is_err());

        let d_still = mgr.send_command("dev", read_v, Some(15)).unwrap();
        assert!(d_still.stdout.contains("developer"));
        assert_eq!(mgr.list_sessions().len(), 1);
    }

    #[test]
    fn shell_capture_pane_scrollback_and_ring_buffer_cap() {
        let mgr = ShellManager::with_limits("cap-node", 25, 4096);
        mgr.create_session("log", Shell::Auto, None).unwrap();

        let cmd = if cfg!(windows) {
            "1..40 | ForEach-Object { Write-Output \"line-$_\" }"
        } else {
            "for i in $(seq 1 40); do echo \"line-$i\"; done"
        };
        let res = mgr.send_command("log", cmd, Some(15)).unwrap();
        assert_eq!(res.exit_code, 0);

        let cap = mgr.capture("log", Some(10)).unwrap();
        assert!(
            cap.total_lines <= 25,
            "ring buffer exceeded cap: {}",
            cap.total_lines
        );
        assert_eq!(cap.lines.len(), 10);
        assert!(
            cap.lines.last().unwrap().contains("line-40"),
            "missing latest line: {:?}",
            cap.lines
        );
    }

    #[test]
    fn shell_command_timeout_and_session_recovery() {
        let mgr = ShellManager::new("timeout-node");
        mgr.create_session("hang", Shell::Auto, None).unwrap();

        let slow_cmd = if cfg!(windows) {
            "Start-Sleep -Seconds 10"
        } else {
            "sleep 10"
        };
        let res = mgr.send_command("hang", slow_cmd, Some(1)).unwrap();
        assert!(res.timed_out, "expected timeout");
        assert_eq!(res.exit_code, -1);

        // Manager stays healthy: we can kill the timed-out session and recreate it cleanly.
        mgr.kill_session("hang").unwrap();
        mgr.create_session("hang", Shell::Auto, None).unwrap();
        let fast = mgr
            .send_command("hang", "echo recovered", Some(15))
            .unwrap();
        assert_eq!(fast.exit_code, 0);
        assert!(fast.stdout.contains("recovered"));
    }

    #[test]
    fn shell_concurrent_sessions_do_not_block_each_other() {
        let mgr = Arc::new(ShellManager::new("conc-node"));
        mgr.create_session("slow", Shell::Auto, None).unwrap();
        mgr.create_session("fast", Shell::Auto, None).unwrap();

        // Warm up both shells so PowerShell startup cost is excluded from concurrency timing.
        mgr.send_command("slow", "echo warm", Some(15)).unwrap();
        mgr.send_command("fast", "echo warm", Some(15)).unwrap();

        let slow_mgr = mgr.clone();
        let slow_cmd = if cfg!(windows) {
            "Start-Sleep -Seconds 2; Write-Output slow_done"
        } else {
            "sleep 2; echo slow_done"
        };
        let handle =
            thread::spawn(move || slow_mgr.send_command("slow", slow_cmd, Some(15)).unwrap());
        thread::sleep(Duration::from_millis(150));

        let fast_start = Instant::now();
        let fast_res = mgr.send_command("fast", "echo fast_ok", Some(15)).unwrap();
        let fast_elapsed = fast_start.elapsed();
        assert_eq!(fast_res.exit_code, 0);
        assert!(fast_res.stdout.contains("fast_ok"));
        assert!(
            fast_elapsed < Duration::from_millis(1500),
            "fast session was blocked by slow session: {fast_elapsed:?}"
        );

        let slow_res = handle.join().unwrap();
        assert_eq!(slow_res.exit_code, 0);
        assert!(slow_res.stdout.contains("slow_done"));
    }

    #[test]
    fn shell_drop_cleans_up_all_child_processes() {
        let pids: Vec<u32>;
        {
            let mgr = ShellManager::new("drop-node");
            let s1 = mgr.create_session("a", Shell::Auto, None).unwrap();
            let s2 = mgr.create_session("b", Shell::Auto, None).unwrap();
            pids = vec![s1.pid, s2.pid];
            assert!(pids.iter().all(|&p| p > 0));
        }
        // After Drop, verify child processes were reaped / terminated.
        #[cfg(unix)]
        for pid in pids {
            let alive = unsafe { libc::kill(pid as libc::pid_t, 0) == 0 };
            assert!(!alive, "orphaned child pid {pid} still alive after drop");
        }
        #[cfg(windows)]
        {
            assert_eq!(pids.len(), 2);
        }
    }

    #[test]
    fn shell_plugin_gate_rejects_requests_when_not_installed() {
        let root = temp_dir("gate");
        let blobs = Blobs::open_blocking(&root.join("blobs")).unwrap();
        let plugins_dir = root.join("plugins");

        let rt = tokio::runtime::Runtime::new().unwrap();
        rt.block_on(async {
            let cfg = IrohTransportConfig::local_offline();
            let server_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let client_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let lookup = MemoryLookup::new();
            lookup.add_endpoint_info(server_ep.addr());
            client_ep.address_lookup().unwrap().add(lookup);

            let mgr = Arc::new(ShellManager::new("gated-peer"));
            let router = Router::builder(server_ep.clone())
                .accept(
                    ALPN,
                    ShellHandler::new(mgr, blobs.clone(), plugins_dir.clone()),
                )
                .spawn();

            let resp = remote_shell(
                &client_ep,
                server_ep.id(),
                &ShellRequest::New {
                    name: "main".to_string(),
                    shell: Shell::Auto,
                    cwd: None,
                },
            )
            .await
            .unwrap();

            match resp {
                ShellResponse::Error { error } => {
                    assert!(
                        error.contains("dialtone install shell"),
                        "unexpected gate error: {error}"
                    );
                }
                other => panic!("expected plugin gate error, got {other:?}"),
            }

            let _ = router.shutdown().await;
            client_ep.close().await;
        });

        blobs.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn remote_shell_new_send_capture_ls_kill_over_iroh_quic() {
        let root = temp_dir("quic-e2e");
        let blobs = Blobs::open_blocking(&root.join("blobs")).unwrap();
        let plugins_dir = root.join("plugins");
        install_builtin_plugin(&blobs, &plugins_dir, SHELL_PLUGIN_NAME).unwrap();

        let rt = tokio::runtime::Runtime::new().unwrap();
        rt.block_on(async {
            let cfg = IrohTransportConfig::local_offline();
            let server_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let client_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let lookup = MemoryLookup::new();
            lookup.add_endpoint_info(server_ep.addr());
            client_ep.address_lookup().unwrap().add(lookup);

            let mgr = Arc::new(ShellManager::new("remote-worker"));
            let router = Router::builder(server_ep.clone())
                .accept(
                    ALPN,
                    ShellHandler::new(mgr, blobs.clone(), plugins_dir.clone()),
                )
                .spawn();

            // 1. New session
            let created = remote_shell(
                &client_ep,
                server_ep.id(),
                &ShellRequest::New {
                    name: "ops".to_string(),
                    shell: Shell::Auto,
                    cwd: None,
                },
            )
            .await
            .unwrap();
            assert!(
                matches!(created, ShellResponse::Created { .. }),
                "{created:?}"
            );

            // 2. Stateful multi-step send over QUIC
            let (set_cmd, get_cmd) = if cfg!(windows) {
                (
                    "$env:DT_QUIC_SESSION = 'mesh_ok_42'",
                    "Write-Output $env:DT_QUIC_SESSION",
                )
            } else {
                ("export DT_QUIC_SESSION=mesh_ok_42", "echo $DT_QUIC_SESSION")
            };
            let s1 = remote_shell(
                &client_ep,
                server_ep.id(),
                &ShellRequest::Send {
                    name: "ops".to_string(),
                    command: set_cmd.to_string(),
                    timeout_secs: Some(15),
                },
            )
            .await
            .unwrap();
            assert!(matches!(s1, ShellResponse::Sent { ref result } if result.exit_code == 0));

            let s2 = remote_shell(
                &client_ep,
                server_ep.id(),
                &ShellRequest::Send {
                    name: "ops".to_string(),
                    command: get_cmd.to_string(),
                    timeout_secs: Some(15),
                },
            )
            .await
            .unwrap();
            match s2 {
                ShellResponse::Sent { result } => {
                    assert_eq!(result.exit_code, 0);
                    assert_eq!(result.node, "remote-worker");
                    assert!(
                        result.stdout.contains("mesh_ok_42"),
                        "stdout={}",
                        result.stdout
                    );
                }
                other => panic!("expected Sent, got {other:?}"),
            }

            // 3. Capture scrollback
            let cap = remote_shell(
                &client_ep,
                server_ep.id(),
                &ShellRequest::Capture {
                    name: "ops".to_string(),
                    lines: Some(20),
                },
            )
            .await
            .unwrap();
            match cap {
                ShellResponse::Captured { capture } => {
                    assert!(capture.lines.iter().any(|l| l.contains("mesh_ok_42")));
                }
                other => panic!("expected Captured, got {other:?}"),
            }

            // 4. List sessions
            let listed = remote_shell(&client_ep, server_ep.id(), &ShellRequest::List)
                .await
                .unwrap();
            match listed {
                ShellResponse::Listed { sessions, .. } => {
                    assert_eq!(sessions.len(), 1);
                    assert_eq!(sessions[0].name, "ops");
                }
                other => panic!("expected Listed, got {other:?}"),
            }

            // 5. Kill session
            let killed = remote_shell(
                &client_ep,
                server_ep.id(),
                &ShellRequest::Kill {
                    name: "ops".to_string(),
                },
            )
            .await
            .unwrap();
            assert!(matches!(killed, ShellResponse::Killed { .. }), "{killed:?}");

            let _ = router.shutdown().await;
            client_ep.close().await;
        });

        blobs.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn remote_shell_rejects_unallowlisted_peer_on_quic() {
        let root = temp_dir("allowlist");
        let blobs = Blobs::open_blocking(&root.join("blobs")).unwrap();
        let plugins_dir = root.join("plugins");
        install_builtin_plugin(&blobs, &plugins_dir, SHELL_PLUGIN_NAME).unwrap();

        let rt = tokio::runtime::Runtime::new().unwrap();
        rt.block_on(async {
            let cfg = IrohTransportConfig::local_offline();
            let allowed_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let intruder_ep = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();

            let allowlist = LiveAllowlist::new(&[allowed_ep.id().to_string()]);
            let server_ep = build_endpoint_builder(&cfg)
                .unwrap()
                .hooks(AllowlistHook::new(allowlist))
                .bind()
                .await
                .unwrap();

            for ep in [&allowed_ep, &intruder_ep] {
                let lookup = MemoryLookup::new();
                lookup.add_endpoint_info(server_ep.addr());
                ep.address_lookup().unwrap().add(lookup);
            }

            let mgr = Arc::new(ShellManager::new("guarded-peer"));
            let router = Router::builder(server_ep.clone())
                .accept(
                    ALPN,
                    ShellHandler::new(mgr, blobs.clone(), plugins_dir.clone()),
                )
                .spawn();

            // Allowed peer succeeds
            let ok = remote_shell(&allowed_ep, server_ep.id(), &ShellRequest::List)
                .await
                .unwrap();
            assert!(matches!(ok, ShellResponse::Listed { .. }));

            // Unallowlisted peer is closed by AllowlistHook
            let denied = remote_shell(&intruder_ep, server_ep.id(), &ShellRequest::List).await;
            assert!(denied.is_err(), "intruder peer was admitted: {denied:?}");

            let _ = router.shutdown().await;
            allowed_ep.close().await;
            intruder_ep.close().await;
        });

        blobs.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }
}
