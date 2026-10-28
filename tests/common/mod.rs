//! Shared helpers for the tests that start real nodes from the built binary.
#![allow(dead_code)]

use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread::sleep;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde_json::Value;

pub const BIN: &str = env!("CARGO_BIN_EXE_dialtone");

/// A fresh directory under the system temp dir, removed on drop.
pub struct TempDir(pub PathBuf);

impl TempDir {
    pub fn new(label: &str) -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .subsec_nanos();
        let dir = std::env::temp_dir().join(format!(
            "dialtone-test-{label}-{}-{nanos}",
            std::process::id()
        ));
        fs::create_dir_all(&dir).unwrap();
        TempDir(dir)
    }

    pub fn path(&self) -> &Path {
        &self.0
    }

    pub fn join(&self, p: &str) -> PathBuf {
        self.0.join(p)
    }
}

impl Drop for TempDir {
    fn drop(&mut self) {
        // Files of processes that just stopped can stay locked for a moment on Windows.
        for _ in 0..10 {
            if fs::remove_dir_all(&self.0).is_ok() || !self.0.exists() {
                return;
            }
            sleep(Duration::from_millis(300));
        }
    }
}

/// `dialtone <args>` with this state directory; (succeeded, stdout).
pub fn run(state_dir: Option<&Path>, args: &[&str]) -> (bool, String) {
    let (ok, out, _) = run_full(state_dir, args);
    (ok, out)
}

/// `run` with stderr too: (succeeded, stdout, stderr).
pub fn run_full(state_dir: Option<&Path>, args: &[&str]) -> (bool, String, String) {
    let mut cmd = Command::new(BIN);
    cmd.args(args).stdin(Stdio::null());
    if let Some(dir) = state_dir {
        cmd.env("DIALTONE_STATE_DIR", dir);
    }
    let out = cmd.output().expect("run dialtone");
    (
        out.status.success(),
        String::from_utf8_lossy(&out.stdout).into_owned(),
        String::from_utf8_lossy(&out.stderr).into_owned(),
    )
}

/// `run` that must succeed; returns stdout.
pub fn run_ok(state_dir: Option<&Path>, args: &[&str]) -> String {
    let (ok, out, err) = run_full(state_dir, args);
    assert!(ok, "dialtone {} failed: {out}{err}", args.join(" "));
    out
}

/// A running `dialtone` (usually `up`) with its output in `log`. Dropping it stops the
/// processes its supervisor started, then the node.
pub struct Node {
    child: Option<Child>,
    pub api_port: u16,
}

impl Node {
    pub fn start(state_dir: &Path, log: &Path, api_port: u16, args: &[&str]) -> Node {
        let log = fs::File::create(log).unwrap();
        let child = Command::new(BIN)
            .args(args)
            .env("DIALTONE_STATE_DIR", state_dir)
            .stdin(Stdio::null())
            .stdout(log.try_clone().unwrap())
            .stderr(log)
            .spawn()
            .expect("start dialtone");
        Node {
            child: Some(child),
            api_port,
        }
    }

    pub fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            if let Some(list) = get_json(self.api_port, "/api/processes") {
                for p in list["processes"].as_array().into_iter().flatten() {
                    if let Some(pid) = p["pid"].as_u64() {
                        kill_pid(pid);
                    }
                }
            }
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

impl Drop for Node {
    fn drop(&mut self) {
        self.stop();
    }
}

/// Kill a process at once (`kill -9`; `taskkill /F` on Windows).
pub fn kill_pid(pid: u64) {
    #[cfg(unix)]
    let mut cmd = {
        let mut c = Command::new("kill");
        c.args(["-9", &pid.to_string()]);
        c
    };
    #[cfg(windows)]
    let mut cmd = {
        let mut c = Command::new("taskkill");
        c.args(["/F", "/PID", &pid.to_string()]);
        c
    };
    let _ = cmd
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();
}

/// One HTTP/1.1 request to 127.0.0.1:port; (status code, body).
pub fn http(
    port: u16,
    method: &str,
    path: &str,
    token: Option<&str>,
    body: Option<&str>,
) -> Result<(u16, String), String> {
    let addr: SocketAddr = ([127, 0, 0, 1], port).into();
    let mut stream =
        TcpStream::connect_timeout(&addr, Duration::from_secs(2)).map_err(|e| e.to_string())?;
    let _ = stream.set_read_timeout(Some(Duration::from_secs(20)));
    let _ = stream.set_write_timeout(Some(Duration::from_secs(20)));
    let mut req = format!("{method} {path} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n");
    if let Some(t) = token {
        req.push_str(&format!("Authorization: Bearer {t}\r\n"));
    }
    if let Some(b) = body {
        req.push_str(&format!(
            "Content-Type: application/json\r\nContent-Length: {}\r\n\r\n{b}",
            b.len()
        ));
    } else {
        req.push_str("\r\n");
    }
    stream
        .write_all(req.as_bytes())
        .map_err(|e| e.to_string())?;
    let mut raw = Vec::new();
    stream.read_to_end(&mut raw).map_err(|e| e.to_string())?;
    let text = String::from_utf8_lossy(&raw);
    let (head, body) = text.split_once("\r\n\r\n").ok_or("malformed response")?;
    let code = head
        .split_whitespace()
        .nth(1)
        .and_then(|c| c.parse().ok())
        .ok_or("no status code")?;
    Ok((code, body.to_string()))
}

/// GET that succeeded (2xx) with a JSON body.
pub fn get_json(port: u16, path: &str) -> Option<Value> {
    match http(port, "GET", path, None, None) {
        Ok((code, body)) if (200..300).contains(&code) => serde_json::from_str(&body).ok(),
        _ => None,
    }
}

/// POST that succeeded (2xx) with a JSON body.
pub fn post_json(port: u16, path: &str, token: &str, body: &str) -> Option<Value> {
    match http(port, "POST", path, Some(token), Some(body)) {
        Ok((code, body)) if (200..300).contains(&code) => serde_json::from_str(&body).ok(),
        _ => None,
    }
}

/// The node's API token, from its state directory.
pub fn api_token(state_dir: &Path) -> String {
    fs::read_to_string(state_dir.join("api-token"))
        .unwrap_or_default()
        .trim()
        .to_string()
}

/// Retry every second until `f` holds; panics after `secs`.
pub fn until_ok(secs: u64, what: &str, mut f: impl FnMut() -> bool) {
    let deadline = Instant::now() + Duration::from_secs(secs);
    loop {
        if f() {
            return;
        }
        if Instant::now() >= deadline {
            panic!("{what} (gave up after {secs}s)");
        }
        sleep(Duration::from_secs(1));
    }
}

/// The codes of a probe's checks, sorted.
pub fn check_codes(probe: &Value) -> Vec<String> {
    let mut codes: Vec<String> = probe["checks"]
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(|c| c["code"].as_str().map(str::to_string))
        .collect();
    codes.sort();
    codes
}

/// A program the supervisor runs on any OS: `:;` lines run under /bin/sh, the rest as a .cmd.
pub fn polyglot(sh: &str, cmd: &[&str]) -> Vec<u8> {
    let mut s = format!(":; {sh}\r\n");
    for line in cmd {
        s.push_str(line);
        s.push_str("\r\n");
    }
    s.into_bytes()
}
