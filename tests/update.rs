//! Self-update from signed releases, on real nodes: a node keeps a release that starts
//! healthy, rolls back one that panics when it starts and one that never gets healthy, and a
//! node that cannot reach the release server updates from a peer that runs the release.
//!
//! A local HTTP server stands in for GitHub, the releases are signed with a test key, and a
//! restart loop stands in for the service (as `start-dialtone.cmd` does). A release binary is
//! this build with a marker appended (`update.rs` reads it in debug builds only).
mod common;

use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, sleep};
use std::time::Duration;

use common::*;
use dialtone::update::{own_asset, sha256_hex, MANIFEST_NAME, MANIFEST_SCHEMA};
use serde_json::{json, Value};

const SECRET: &str = "dt-E2E_Update_Secret_2026!";

/// Releases signed with a test key, laid out like GitHub's download URLs.
struct Releases {
    root: PathBuf,
    key: iroh::SecretKey,
}

impl Releases {
    fn new(root: PathBuf) -> Self {
        fs::create_dir_all(root.join("latest/download")).unwrap();
        Releases {
            root,
            key: iroh::SecretKey::from_bytes(&[42; 32]),
        }
    }

    fn public_key(&self) -> String {
        hex::encode(self.key.public().as_bytes())
    }

    /// Publish this build as release `version` (with `marker` at its end) as the latest
    /// release; returns its SHA-256.
    fn publish(&self, version: &str, marker: &str) -> String {
        let asset = own_asset().expect("a release target");
        let mut bytes = fs::read(BIN).unwrap();
        bytes.extend_from_slice(format!("\ndialtone test release v{version} {marker}").as_bytes());
        let sha = sha256_hex(&bytes);
        let dir = self.root.join(format!("download/v{version}"));
        fs::create_dir_all(&dir).unwrap();
        fs::write(dir.join(asset), &bytes).unwrap();
        let manifest = json!({
            "schema": MANIFEST_SCHEMA,
            "version": version,
            "assets": [{
                "name": asset,
                "sha256": sha,
                "blake3": iroh_blobs::Hash::new(&bytes).to_hex(),
                "size": bytes.len(),
            }],
        })
        .to_string();
        let sig = hex::encode(self.key.sign(manifest.as_bytes()).to_bytes());
        for d in [dir, self.root.join("latest/download")] {
            fs::write(d.join(MANIFEST_NAME), &manifest).unwrap();
            fs::write(d.join(format!("{MANIFEST_NAME}.sig")), &sig).unwrap();
        }
        sha
    }
}

/// Serve `root` over HTTP on a free port; returns the base URL.
fn serve(root: PathBuf) -> String {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}", listener.local_addr().unwrap());
    thread::spawn(move || {
        for stream in listener.incoming().flatten() {
            let root = root.clone();
            thread::spawn(move || {
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut line = String::new();
                let _ = reader.read_line(&mut line);
                loop {
                    let mut header = String::new();
                    if reader.read_line(&mut header).unwrap_or(0) <= 2 {
                        break;
                    }
                }
                let path = line.split_whitespace().nth(1).unwrap_or("/");
                let file = root.join(path.trim_start_matches('/'));
                let mut stream = stream;
                match fs::read(&file) {
                    Ok(body) => {
                        let _ = write!(
                            stream,
                            "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                            body.len()
                        );
                        let _ = stream.write_all(&body);
                    }
                    Err(_) => {
                        let _ = stream.write_all(
                            b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
                        );
                    }
                }
            });
        }
    });
    url
}

/// A node run the way its service runs it: started again whenever it exits.
struct Service {
    stop: Arc<AtomicBool>,
    child: Arc<Mutex<Option<Child>>>,
    thread: Option<thread::JoinHandle<()>>,
    api: u16,
}

impl Service {
    fn start(
        exe: &Path,
        state: &Path,
        log: &Path,
        api: u16,
        args: &[&str],
        env: &[(&str, &str)],
    ) -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let child: Arc<Mutex<Option<Child>>> = Arc::new(Mutex::new(None));
        let (exe, state, log) = (exe.to_path_buf(), state.to_path_buf(), log.to_path_buf());
        let args: Vec<String> = args.iter().map(|s| s.to_string()).collect();
        let env: Vec<(String, String)> = env
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect();
        let (stop2, child2) = (stop.clone(), child.clone());
        let thread = thread::spawn(move || {
            while !stop2.load(Ordering::Relaxed) {
                let out = fs::OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(&log)
                    .unwrap();
                let spawned = Command::new(&exe)
                    .args(&args)
                    .envs(env.iter().map(|(k, v)| (k, v)))
                    .env("DIALTONE_STATE_DIR", &state)
                    .stdin(Stdio::null())
                    .stdout(out.try_clone().unwrap())
                    .stderr(out)
                    .spawn();
                match spawned {
                    Ok(c) => *child2.lock().unwrap() = Some(c),
                    Err(_) => {
                        // The binary is being swapped; try again.
                        sleep(Duration::from_millis(300));
                        continue;
                    }
                }
                loop {
                    sleep(Duration::from_millis(200));
                    let mut guard = child2.lock().unwrap();
                    match guard.as_mut().map(|c| c.try_wait()) {
                        Some(Ok(None)) if !stop2.load(Ordering::Relaxed) => {}
                        _ => break,
                    }
                }
                sleep(Duration::from_millis(500));
            }
        });
        Service {
            stop,
            child,
            thread: Some(thread),
            api,
        }
    }

    fn stop(&mut self) {
        self.stop.store(true, Ordering::Relaxed);
        if let Some(list) = get_json(self.api, "/api/processes") {
            for p in list["processes"].as_array().into_iter().flatten() {
                if let Some(pid) = p["pid"].as_u64() {
                    kill_pid(pid);
                }
            }
        }
        if let Some(mut c) = self.child.lock().unwrap().take() {
            let _ = c.kill();
            let _ = c.wait();
        }
        if let Some(t) = self.thread.take() {
            let _ = t.join();
        }
    }
}

impl Drop for Service {
    fn drop(&mut self) {
        self.stop();
    }
}

/// A state dir with this build installed as `<state>/bin/dialtone`, the way a service runs it.
fn install(state: &Path) -> PathBuf {
    fs::create_dir_all(state.join("bin")).unwrap();
    let exe = state
        .join("bin")
        .join(format!("dialtone{}", std::env::consts::EXE_SUFFIX));
    fs::copy(BIN, &exe).unwrap();
    exe
}

fn sha_of(path: &Path) -> String {
    fs::read(path).map(|b| sha256_hex(&b)).unwrap_or_default()
}

fn update_state(state: &Path) -> Value {
    fs::read(state.join("update.json"))
        .ok()
        .and_then(|b| serde_json::from_slice(&b).ok())
        .unwrap_or(Value::Null)
}

fn is_bad(state: &Path, version: &str) -> bool {
    update_state(state)["bad"]
        .as_array()
        .is_some_and(|b| b.iter().any(|v| v == version))
}

fn committed(state: &Path) -> String {
    update_state(state)["committed"]["version"]
        .as_str()
        .unwrap_or("")
        .to_string()
}

/// The version `name` runs, as `port`'s node sees it.
fn version_seen(port: u16, name: &str) -> String {
    get_json(port, "/api/mesh")
        .and_then(|m| {
            m["nodes"]
                .as_array()?
                .iter()
                .find(|n| n["node"] == name)
                .and_then(|n| n["version"].as_str().map(str::to_string))
        })
        .unwrap_or_default()
}

/// `until_ok`, but on timeout the panic shows the end of each node's log.
fn wait(secs: u64, what: &str, logs: &[&Path], mut f: impl FnMut() -> bool) {
    let start = std::time::Instant::now();
    while start.elapsed() < Duration::from_secs(secs) {
        if f() {
            return;
        }
        sleep(Duration::from_millis(500));
    }
    let tails: Vec<String> = logs
        .iter()
        .map(|l| {
            let text = fs::read_to_string(l).unwrap_or_default();
            let lines: Vec<&str> = text.lines().filter(|l| !l.starts_with("[probe]")).collect();
            format!(
                "--- {}
{}",
                l.display(),
                lines[lines.len().saturating_sub(40)..].join(
                    "
"
                )
            )
        })
        .collect();
    panic!(
        "{what} (gave up after {secs}s)
{}",
        tails.join(
            "
"
        )
    );
}

fn node_env<'a>(url: &'a str, key: &'a str) -> Vec<(&'a str, &'a str)> {
    vec![
        ("DIALTONE_RELEASE_URL", url),
        ("DIALTONE_RELEASE_PUBKEY", key),
        ("DIALTONE_UPDATE_CHECK_SECS", "3"),
        ("DIALTONE_TRIAL_MIN_SECS", "3"),
        // Debug builds start slowly on a busy CI runner.
        ("DIALTONE_TRIAL_DEADLINE_SECS", "45"),
    ]
}

// Appending to a signed Mach-O binary breaks its signature, so the test releases do not run
// on macOS.
#[test]
#[cfg_attr(target_os = "macos", ignore = "test releases are unsigned copies")]
fn update_keeps_a_healthy_release_and_rolls_back_bad_ones() {
    let t = TempDir::new("update");
    let releases = Releases::new(t.join("releases"));
    let url = serve(releases.root.clone());
    let key = releases.public_key();
    let state = t.join("u-a");
    let exe = install(&state);
    let original = sha_of(&exe);
    let log = t.join("u-a.log");
    let _a = Service::start(
        &exe,
        &state,
        &log,
        18931,
        &[
            "up",
            "--node",
            "u-a",
            "--listen",
            "127.0.0.1:18931",
            "--private-mesh",
            "--quic-port",
            "19531",
            "--secret",
            SECRET,
        ],
        &node_env(&url, &key),
    );
    until_ok(60, "u-a API up", || get_json(18931, "/api/mesh").is_some());
    let back_on_original = || sha_of(&exe) == original && get_json(18931, "/api/mesh").is_some();

    releases.publish("90.0.0", "DIALTONE-TEST-HANG");
    wait(
        240,
        "a release that never gets healthy is rolled back",
        &[&log],
        || is_bad(&state, "90.0.0"),
    );
    wait(
        120,
        "the previous binary runs again",
        &[&log],
        back_on_original,
    );
    println!("    1. A release that never got healthy was rolled back at its deadline");

    releases.publish("91.0.0", "DIALTONE-TEST-PANIC");
    wait(
        240,
        "a release that panics when it starts is rolled back",
        &[&log],
        || is_bad(&state, "91.0.0"),
    );
    wait(
        120,
        "the previous binary runs again",
        &[&log],
        back_on_original,
    );
    println!("    2. A release that panicked at start was rolled back after 3 starts");

    let good = releases.publish("92.0.0", "");
    wait(240, "a healthy release is kept", &[&log], || {
        committed(&state) == "92.0.0"
    });
    assert_eq!(
        sha_of(&exe),
        good,
        "the kept release is not the binary at the path"
    );
    let prev = exe.with_file_name(format!("dialtone.prev{}", std::env::consts::EXE_SUFFIX));
    assert_eq!(
        sha_of(&prev),
        original,
        "the previous binary is not kept as dialtone.prev"
    );
    wait(60, "u-a reports v92.0.0", &[&log], || {
        version_seen(18931, "u-a") == "92.0.0"
    });
    assert!(is_bad(&state, "90.0.0") && is_bad(&state, "91.0.0"));
    println!("    3. A healthy release was kept; bad ones stay marked and are not retried");
}

#[test]
#[cfg_attr(target_os = "macos", ignore = "test releases are unsigned copies")]
fn a_node_updates_from_a_peer_that_runs_the_release() {
    let t = TempDir::new("update-peer");
    let releases = Releases::new(t.join("releases"));
    let url = serve(releases.root.clone());
    let key = releases.public_key();
    let (sa, sb) = (t.join("p-a"), t.join("p-b"));
    let (exe_a, exe_b) = (install(&sa), install(&sb));
    let (la, lb) = (t.join("p-a.log"), t.join("p-b.log"));
    let _a = Service::start(
        &exe_a,
        &sa,
        &la,
        18932,
        &[
            "up",
            "--node",
            "p-a",
            "--listen",
            "127.0.0.1:18932",
            "--private-mesh",
            "--quic-port",
            "19532",
            "--secret",
            SECRET,
        ],
        &node_env(&url, &key),
    );
    until_ok(60, "p-a API up", || get_json(18932, "/api/mesh").is_some());
    let invite = run_ok(
        Some(&sa),
        &["invite", "--listen", "127.0.0.1:18932", "--ttl", "10m"],
    )
    .trim()
    .to_string();
    run_ok(
        Some(&sb),
        &[
            "join",
            &invite,
            "--node",
            "p-b",
            "--listen",
            "127.0.0.1:18933",
            "--private-mesh",
            "--once",
        ],
    );
    // p-b cannot reach the release server: only p-a can give it the release.
    let _b = Service::start(
        &exe_b,
        &sb,
        &lb,
        18933,
        &[
            "up",
            "--node",
            "p-b",
            "--listen",
            "127.0.0.1:18933",
            "--private-mesh",
            "--quic-port",
            "19533",
        ],
        &node_env("http://127.0.0.1:9", &key),
    );
    wait(120, "p-a and p-b see each other", &[&la, &lb], || {
        !version_seen(18932, "p-b").is_empty() && !version_seen(18933, "p-a").is_empty()
    });

    let good = releases.publish("93.0.0", "");
    wait(
        300,
        "p-a keeps the release from the server",
        &[&la, &lb],
        || committed(&sa) == "93.0.0",
    );
    wait(300, "p-b keeps the release from p-a", &[&la, &lb], || {
        committed(&sb) == "93.0.0"
    });
    assert_eq!(sha_of(&exe_b), good);
    let log = fs::read_to_string(&lb).unwrap_or_default();
    assert!(
        log.contains("from peers"),
        "p-b did not get the binary from p-a:\n{log}"
    );
    println!("    4. A node that cannot reach the release server updated from a peer");
}
