//! Small helpers shared by the tasks: running commands, the node's HTTP API, hashing.

use serde_json::Value;
use sha2::{Digest, Sha256};
use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::time::Duration;

pub type R<T> = Result<T, String>;

pub fn say(s: &str) {
    println!("==> {s}");
}

/// The repository root (xtask/ is one level down).
pub fn root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("xtask lives in the repository")
        .to_path_buf()
}

pub fn home() -> PathBuf {
    std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from)
        .unwrap_or_default()
}

pub const EXE: &str = std::env::consts::EXE_SUFFIX;

/// The release binary this checkout builds.
pub fn release_bin() -> PathBuf {
    root().join(format!("target/release/dialtone{EXE}"))
}

pub fn cargo() -> Command {
    Command::new(std::env::var("CARGO").unwrap_or_else(|_| "cargo".into()))
}

/// Run a command with inherited output; fail unless it exits 0.
pub fn run(cmd: &mut Command) -> R<()> {
    let st = cmd.status().map_err(|e| format!("{cmd:?}: {e}"))?;
    if st.success() {
        Ok(())
    } else {
        Err(format!("{} failed ({st})", show(cmd)))
    }
}

/// Run a command and return its stdout (stderr passes through); fail unless it exits 0.
pub fn output(cmd: &mut Command) -> R<String> {
    let out = cmd
        .stderr(Stdio::inherit())
        .output()
        .map_err(|e| format!("{}: {e}", show(cmd)))?;
    if !out.status.success() {
        return Err(format!("{} failed ({})", show(cmd), out.status));
    }
    Ok(String::from_utf8_lossy(&out.stdout).replace('\r', ""))
}

/// Whether a command exits 0, with its output discarded.
pub fn quietly(cmd: &mut Command) -> bool {
    cmd.stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .is_ok_and(|s| s.success())
}

fn show(cmd: &Command) -> String {
    let mut s = cmd.get_program().to_string_lossy().into_owned();
    if let Some(a) = cmd.get_args().next() {
        s.push(' ');
        s.push_str(&a.to_string_lossy());
    }
    s
}

pub fn sha256_file(path: &Path) -> R<String> {
    let bytes = std::fs::read(path).map_err(|e| format!("read {}: {e}", path.display()))?;
    Ok(hex(&Sha256::digest(&bytes)))
}

pub fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

/// GET or POST on a node's local HTTP API; returns the status code and the parsed body.
pub fn http(
    port: u16,
    method: &str,
    path: &str,
    token: Option<&str>,
    body: Option<&Value>,
    timeout: Duration,
) -> R<(u16, Value)> {
    let addr = std::net::SocketAddr::from(([127, 0, 0, 1], port));
    let mut s = TcpStream::connect_timeout(&addr, Duration::from_secs(2))
        .map_err(|e| format!("connect 127.0.0.1:{port}: {e}"))?;
    let _ = s.set_read_timeout(Some(timeout));
    let payload = body.map(|b| b.to_string()).unwrap_or_default();
    let auth = token
        .map(|t| format!("Authorization: Bearer {t}\r\n"))
        .unwrap_or_default();
    let req = format!(
        "{method} {path} HTTP/1.1\r\nHost: 127.0.0.1\r\n{auth}Content-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{payload}",
        payload.len()
    );
    s.write_all(req.as_bytes())
        .map_err(|e| format!("write to 127.0.0.1:{port}: {e}"))?;
    let mut raw = Vec::new();
    s.read_to_end(&mut raw)
        .map_err(|e| format!("read from 127.0.0.1:{port}: {e}"))?;
    let text = String::from_utf8_lossy(&raw);
    let (head, body) = text
        .split_once("\r\n\r\n")
        .ok_or_else(|| format!("malformed HTTP response from 127.0.0.1:{port}"))?;
    let code = head
        .split_whitespace()
        .nth(1)
        .and_then(|c| c.parse().ok())
        .unwrap_or(0);
    Ok((code, serde_json::from_str(body).unwrap_or(Value::Null)))
}

/// GET a JSON document from a node's API, failing on anything but 200.
pub fn get(port: u16, path: &str) -> R<Value> {
    match http(port, "GET", path, None, None, Duration::from_secs(5))? {
        (200, v) => Ok(v),
        (code, v) => Err(format!("GET {path} on {port}: {code} {v}")),
    }
}

pub fn sleep(secs: f64) {
    std::thread::sleep(Duration::from_secs_f64(secs));
}

/// A small deterministic random sequence (xorshift64), seeded so runs can be compared.
pub struct Rng(u64);

impl Rng {
    pub fn new(seed: u64) -> Self {
        Rng(seed.wrapping_mul(0x9E37_79B9_7F4A_7C15) | 1)
    }
    fn next(&mut self) -> u64 {
        self.0 ^= self.0 << 13;
        self.0 ^= self.0 >> 7;
        self.0 ^= self.0 << 17;
        self.0
    }
    /// `k` distinct items of `items`.
    pub fn sample<T: Clone>(&mut self, items: &[T], k: usize) -> Vec<T> {
        let mut v = items.to_vec();
        let k = k.min(v.len());
        for i in 0..k {
            let j = i + (self.next() % (v.len() - i) as u64) as usize;
            v.swap(i, j);
        }
        v.truncate(k);
        v
    }
}
