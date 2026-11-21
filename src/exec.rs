//! Exec: run a bash or PowerShell command on this node or an admitted mesh peer over QUIC
//! (ALPN `dialtone/exec/1`).
//!
//! One request per bidirectional stream. The allowlist hook (`AllowlistHook` in `node.rs`)
//! admits only allowlisted peers (or channel members on an open channel), and the QUIC
//! handshake cryptographically authenticates the remote NodeId.

use std::io::Read;
use std::path::Path;
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant};

use iroh::endpoint::Connection;
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::{Endpoint, EndpointId};
use serde::{Deserialize, Serialize};

pub const ALPN: &[u8] = b"dialtone/exec/1";
const MAX_REQUEST_BYTES: usize = 64 << 10;
const MAX_RESPONSE_BYTES: usize = 4 << 20;
const MAX_STREAM_CAPTURE_BYTES: usize = 1 << 20;
pub const DEFAULT_TIMEOUT_SECS: u64 = 30;
pub const MAX_TIMEOUT_SECS: u64 = 600;

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Shell {
    #[default]
    Auto,
    Bash,
    Powershell,
}

impl Shell {
    pub fn parse(raw: &str) -> Result<Self, String> {
        match raw.trim().to_ascii_lowercase().as_str() {
            "" | "auto" => Ok(Shell::Auto),
            "bash" | "sh" => Ok(Shell::Bash),
            "powershell" | "pwsh" | "ps" => Ok(Shell::Powershell),
            other => Err(format!(
                "unsupported shell '{other}' (expected auto, bash, or powershell)"
            )),
        }
    }

    pub fn as_str(&self) -> &'static str {
        match self {
            Shell::Auto => "auto",
            Shell::Bash => "bash",
            Shell::Powershell => "powershell",
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ExecRequest {
    #[serde(default)]
    pub shell: Shell,
    pub command: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cwd: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_secs: Option<u64>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ExecResponse {
    pub node: String,
    pub os: String,
    pub shell: String,
    pub exit_code: i32,
    pub stdout: String,
    pub stderr: String,
    pub timed_out: bool,
    pub elapsed_ms: u64,
}

fn clamp_timeout(timeout_secs: Option<u64>) -> Duration {
    let secs = timeout_secs
        .unwrap_or(DEFAULT_TIMEOUT_SECS)
        .clamp(1, MAX_TIMEOUT_SECS);
    Duration::from_secs(secs)
}

fn resolve_shell(shell: Shell) -> Result<(&'static str, String, Vec<&'static str>), String> {
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
                vec!["-NoProfile", "-NonInteractive", "-Command"],
            ))
        }
        Shell::Bash => {
            let candidates: &[&str] = if cfg!(windows) {
                &[
                    "bash",
                    "C:\\Program Files\\Git\\bin\\bash.exe",
                    "C:\\Program Files\\Git\\usr\\bin\\bash.exe",
                ]
            } else {
                &["bash", "/bin/bash", "/usr/bin/bash", "sh", "/bin/sh"]
            };
            let exe = find_executable(candidates)
                .ok_or_else(|| "bash executable not found on this host".to_string())?;
            Ok(("bash", exe, vec!["-c"]))
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

fn read_bounded_sync<R: Read>(mut reader: R, limit: usize) -> String {
    let mut buf = Vec::new();
    let mut chunk = [0u8; 8192];
    let mut truncated = false;
    loop {
        match reader.read(&mut chunk) {
            Ok(0) | Err(_) => break,
            Ok(n) => {
                let remaining = limit.saturating_sub(buf.len());
                if remaining > 0 {
                    buf.extend_from_slice(&chunk[..n.min(remaining)]);
                }
                if n > remaining {
                    truncated = true;
                }
            }
        }
    }
    let mut text = String::from_utf8_lossy(&buf).to_string();
    if truncated {
        text.push_str("\n[output truncated]");
    }
    text
}

/// Run a bash or PowerShell command on this host with bounded timeout and output capture.
pub fn run_local_sync(node_name: &str, req: &ExecRequest) -> Result<ExecResponse, String> {
    if req.command.trim().is_empty() {
        return Err("command must not be empty".to_string());
    }
    if req.command.len() > MAX_REQUEST_BYTES {
        return Err("command exceeds maximum allowed size".to_string());
    }
    if let Some(cwd) = &req.cwd {
        let p = Path::new(cwd);
        if !p.is_dir() {
            return Err(format!("working directory does not exist: {cwd}"));
        }
    }

    let (shell_label, exe, prefix_args) = resolve_shell(req.shell)?;
    let timeout = clamp_timeout(req.timeout_secs);
    let started = Instant::now();

    let mut cmd = Command::new(&exe);
    cmd.args(&prefix_args)
        .arg(&req.command)
        .stdin(Stdio::null())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    if let Some(cwd) = &req.cwd {
        cmd.current_dir(cwd);
    }

    let mut child = cmd
        .spawn()
        .map_err(|e| format!("spawn {shell_label} ({exe}): {e}"))?;
    let stdout_pipe = child
        .stdout
        .take()
        .ok_or_else(|| "failed to capture child stdout".to_string())?;
    let stderr_pipe = child
        .stderr
        .take()
        .ok_or_else(|| "failed to capture child stderr".to_string())?;

    let out_thread =
        thread::spawn(move || read_bounded_sync(stdout_pipe, MAX_STREAM_CAPTURE_BYTES));
    let err_thread =
        thread::spawn(move || read_bounded_sync(stderr_pipe, MAX_STREAM_CAPTURE_BYTES));

    let (exit_code, timed_out) = loop {
        match child.try_wait() {
            Ok(Some(status)) => break (status.code().unwrap_or(-1), false),
            Ok(None) => {
                if started.elapsed() >= timeout {
                    let _ = child.kill();
                    let _ = child.wait();
                    break (-1, true);
                }
                thread::sleep(Duration::from_millis(20));
            }
            Err(e) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(format!("wait for {shell_label}: {e}"));
            }
        }
    };

    let stdout = out_thread.join().unwrap_or_default();
    let stderr = err_thread.join().unwrap_or_default();

    Ok(ExecResponse {
        node: node_name.to_string(),
        os: crate::mesh::default_os(),
        shell: shell_label.to_string(),
        exit_code,
        stdout,
        stderr,
        timed_out,
        elapsed_ms: started.elapsed().as_millis() as u64,
    })
}

/// Async wrapper that keeps subprocess execution off tokio worker threads.
pub async fn run_local(node_name: &str, req: &ExecRequest) -> Result<ExecResponse, String> {
    let node = node_name.to_string();
    let req = req.clone();
    tokio::task::spawn_blocking(move || run_local_sync(&node, &req))
        .await
        .map_err(|e| format!("exec task failed: {e}"))?
}

/// Answers `dialtone/exec/1` QUIC requests from admitted mesh peers.
#[derive(Clone, Debug)]
pub struct ExecHandler {
    node: String,
}

impl ExecHandler {
    pub fn new(node: impl Into<String>) -> Self {
        Self { node: node.into() }
    }

    async fn handle(&self, raw: &[u8]) -> Vec<u8> {
        let req: ExecRequest = match serde_json::from_slice(raw) {
            Ok(req) => req,
            Err(e) => {
                return serde_json::to_vec(&serde_json::json!({
                    "error": format!("invalid exec request: {e}")
                }))
                .unwrap_or_default();
            }
        };
        match run_local(&self.node, &req).await {
            Ok(resp) => serde_json::to_vec(&resp).unwrap_or_default(),
            Err(err) => {
                serde_json::to_vec(&serde_json::json!({ "error": err })).unwrap_or_default()
            }
        }
    }
}

impl ProtocolHandler for ExecHandler {
    async fn accept(&self, connection: Connection) -> Result<(), AcceptError> {
        while let Ok((mut send, mut recv)) = connection.accept_bi().await {
            let raw = recv
                .read_to_end(MAX_REQUEST_BYTES)
                .await
                .map_err(AcceptError::from_err)?;
            let body = self.handle(&raw).await;
            send.write_all(&body).await.map_err(AcceptError::from_err)?;
            send.finish().map_err(AcceptError::from_err)?;
        }
        Ok(())
    }
}

/// Execute a bash or PowerShell command on an admitted remote peer over QUIC (`dialtone/exec/1`).
pub async fn remote(
    endpoint: &Endpoint,
    peer: EndpointId,
    req: &ExecRequest,
) -> Result<ExecResponse, String> {
    let cmd_timeout = clamp_timeout(req.timeout_secs);
    let rpc_timeout = cmd_timeout + Duration::from_secs(10);
    let connect_timeout = Duration::from_secs(10);

    let conn = tokio::time::timeout(connect_timeout, endpoint.connect(peer, ALPN))
        .await
        .map_err(|_| format!("connect to {} timed out", peer.fmt_short()))?
        .map_err(|e| format!("connect to {}: {e}", peer.fmt_short()))?;

    let exchange = async {
        let (mut send, mut recv) = conn.open_bi().await.map_err(|e| e.to_string())?;
        let payload = serde_json::to_vec(req).map_err(|e| e.to_string())?;
        send.write_all(&payload).await.map_err(|e| e.to_string())?;
        send.finish().map_err(|e| e.to_string())?;
        let answer = recv
            .read_to_end(MAX_RESPONSE_BYTES)
            .await
            .map_err(|e| e.to_string())?;
        let value: serde_json::Value =
            serde_json::from_slice(&answer).map_err(|e| format!("bad exec answer: {e}"))?;
        if let Some(err) = value.get("error").and_then(|e| e.as_str()) {
            return Err(err.to_string());
        }
        serde_json::from_value(value).map_err(|e| format!("decode exec answer: {e}"))
    };

    let result = tokio::time::timeout(rpc_timeout, exchange)
        .await
        .map_err(|_| "remote exec timed out".to_string())?;
    conn.close(0u32.into(), b"done");
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::node::{build_endpoint_builder, IrohTransportConfig};
    use iroh::address_lookup::memory::MemoryLookup;
    use iroh::protocol::Router;
    use iroh::SecretKey;

    #[test]
    fn shell_parser_accepts_expected_aliases() {
        assert_eq!(Shell::parse("").unwrap(), Shell::Auto);
        assert_eq!(Shell::parse("auto").unwrap(), Shell::Auto);
        assert_eq!(Shell::parse("bash").unwrap(), Shell::Bash);
        assert_eq!(Shell::parse("powershell").unwrap(), Shell::Powershell);
        assert_eq!(Shell::parse("pwsh").unwrap(), Shell::Powershell);
        assert_eq!(Shell::parse("ps").unwrap(), Shell::Powershell);
        assert!(Shell::parse("cmd-invalid").is_err());
    }

    #[tokio::test]
    async fn local_exec_runs_auto_shell_and_captures_exit_code() {
        let req = ExecRequest {
            shell: Shell::Auto,
            command: "echo hello-dialtone".to_string(),
            cwd: None,
            timeout_secs: Some(10),
        };
        let resp = run_local("self-node", &req).await.unwrap();
        assert_eq!(resp.node, "self-node");
        assert_eq!(resp.exit_code, 0);
        assert!(!resp.timed_out);
        assert!(resp.stdout.contains("hello-dialtone"));

        let fail_req = ExecRequest {
            shell: Shell::Auto,
            command: "exit 7".to_string(),
            cwd: None,
            timeout_secs: Some(10),
        };
        let fail = run_local("self-node", &fail_req).await.unwrap();
        assert_eq!(fail.exit_code, 7);
    }

    #[tokio::test]
    async fn remote_exec_runs_over_iroh_quic_between_two_nodes() {
        let cfg = IrohTransportConfig::local_offline();
        let server_ep = build_endpoint_builder(&cfg)
            .unwrap()
            .secret_key(SecretKey::generate())
            .bind()
            .await
            .unwrap();
        let router = Router::builder(server_ep.clone())
            .accept(ALPN, ExecHandler::new("remote-peer"))
            .spawn();

        let lookup = MemoryLookup::new();
        lookup.add_endpoint_info(server_ep.addr());
        let client_ep = build_endpoint_builder(&cfg)
            .unwrap()
            .secret_key(SecretKey::generate())
            .address_lookup(lookup)
            .bind()
            .await
            .unwrap();

        let req = ExecRequest {
            shell: Shell::Auto,
            command: "echo quic-exec-ok".to_string(),
            cwd: None,
            timeout_secs: Some(15),
        };
        let resp = remote(&client_ep, server_ep.id(), &req).await.unwrap();
        assert_eq!(resp.node, "remote-peer");
        assert_eq!(resp.exit_code, 0);
        assert!(!resp.timed_out);
        assert!(resp.stdout.contains("quic-exec-ok"));

        client_ep.close().await;
        let _ = router.shutdown().await;
    }
}
