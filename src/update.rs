//! Self-update: a node runs the newest signed release, installed A/B, and a build that does not
//! start healthy is rolled back.
//!
//! - **Detect.** Every beat carries the version its node runs. A node checks GitHub for the
//!   latest release now and then, and as soon as a peer's beat shows a newer version.
//! - **Trust.** CI signs `dialtone-release.json` (every binary's SHA-256, BLAKE3 hash and size)
//!   with the release key (`cargo xtask release-manifest`); its public key is below. A binary is
//!   installed only if the manifest verifies and the binary matches it, so a binary from a peer
//!   is as trusted as one from GitHub.
//! - **Fetch.** From peers that run the release on the same OS and architecture (iroh-blobs, by
//!   BLAKE3 hash), else from GitHub.
//! - **A/B.** The running binary moves aside as `dialtone.prev`, the new one takes its path, and
//!   the node exits with `EXIT_RESTART` so its service (systemd, launchd, the Windows start
//!   loop) starts the new one.
//! - **Trial.** The new binary must become healthy within `TRIAL_DEADLINE`: gossip joined, and a
//!   neighbor if the old one had any, after running `TRIAL_MIN_UP`. If it does not, or it fails
//!   to start `MAX_TRIAL_STARTS` times, `dialtone.prev` takes the path back, the version is
//!   marked bad (never tried again), and the node exits so the old binary runs.
//!
//! Only a binary in `<state dir>/bin` updates itself (the one a service runs), never a
//! development build run from elsewhere; `DIALTONE_AUTO_UPDATE=0` turns it off.

use std::io::{Read, Seek, SeekFrom};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::OnceLock;
use std::time::{Duration, Instant};

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// The public key releases are signed with (`cargo xtask release-pubkey`).
const RELEASE_PUBLIC_KEY: &str = "56a08fa0ae8a8a9601e2725b891af2d9dbba7598e80121b7074613b2a6c5e473";
/// Where releases are published; `DIALTONE_RELEASE_URL` points elsewhere (a mirror, a test).
pub const DEFAULT_RELEASES_URL: &str = "https://github.com/timcash/dialtone/releases";
pub const MANIFEST_NAME: &str = "dialtone-release.json";
pub const MANIFEST_SCHEMA: &str = "dialtone.release.v1";
/// The exit code a node stops with to have its service start the binary now at its path.
pub const EXIT_RESTART: i32 = 75;
/// A trial binary that has started this many times without becoming healthy is rolled back.
pub const MAX_TRIAL_STARTS: u32 = 3;
/// A trial binary must have run this long, healthy, to be kept.
const TRIAL_MIN_UP: Duration = Duration::from_secs(60);
/// ... and must get there within this long of starting.
const TRIAL_DEADLINE: Duration = Duration::from_secs(180);

/// Gossip is joined (set by the Mesh actor).
pub static READY: AtomicBool = AtomicBool::new(false);
/// Current gossip neighbors (set by the Mesh actor).
pub static NEIGHBORS: AtomicU64 = AtomicU64::new(0);
static RUNNING_VERSION: OnceLock<String> = OnceLock::new();
static RESTART: OnceLock<tokio_util::sync::CancellationToken> = OnceLock::new();
static RESTART_REQUESTED: AtomicBool = AtomicBool::new(false);

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Manifest {
    pub schema: String,
    pub version: String,
    pub assets: Vec<Asset>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Asset {
    pub name: String,
    pub sha256: String,
    pub blake3: String,
    pub size: u64,
}

impl Manifest {
    pub fn asset(&self, name: &str) -> Option<&Asset> {
        self.assets.iter().find(|a| a.name == name)
    }
}

/// A signed manifest as it travels: the exact bytes that were signed, and the signature (hex).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct SignedManifest {
    pub manifest: String,
    pub sig: String,
}

impl SignedManifest {
    /// The manifest, if the release key signed it.
    pub fn verify(&self) -> Result<Manifest, String> {
        verify_manifest(self.manifest.as_bytes(), &self.sig, &release_public_key()?)
    }
}

fn release_public_key() -> Result<iroh::PublicKey, String> {
    // Tests sign releases with their own key; a published build ignores the override.
    let hex_key = match std::env::var("DIALTONE_RELEASE_PUBKEY") {
        Ok(k) if cfg!(debug_assertions) => k,
        _ => RELEASE_PUBLIC_KEY.to_string(),
    };
    let bytes: [u8; 32] = hex::decode(hex_key.trim())
        .ok()
        .and_then(|b| b.try_into().ok())
        .ok_or("the release public key is not 32 bytes of hex")?;
    iroh::PublicKey::from_bytes(&bytes).map_err(|e| format!("release public key: {e}"))
}

/// Check `bytes` against `sig_hex` with `key` and parse them.
pub fn verify_manifest(
    bytes: &[u8],
    sig_hex: &str,
    key: &iroh::PublicKey,
) -> Result<Manifest, String> {
    let sig: [u8; 64] = hex::decode(sig_hex.trim())
        .ok()
        .and_then(|b| b.try_into().ok())
        .ok_or("the manifest signature is not 64 bytes of hex")?;
    key.verify(bytes, &iroh::Signature::from_bytes(&sig))
        .map_err(|_| "the manifest is not signed by the release key".to_string())?;
    let manifest: Manifest =
        serde_json::from_slice(bytes).map_err(|e| format!("bad manifest: {e}"))?;
    if manifest.schema != MANIFEST_SCHEMA {
        return Err(format!("unknown manifest schema {}", manifest.schema));
    }
    parse_version(&manifest.version)
        .ok_or_else(|| format!("bad manifest version {}", manifest.version))?;
    Ok(manifest)
}

/// The release binary for an OS and architecture (`std::env::consts` names).
pub fn asset_name(os: &str, arch: &str) -> Option<&'static str> {
    match (os, arch) {
        ("windows", "x86_64") => Some("dialtone-windows-x86_64.exe"),
        ("macos", "aarch64") => Some("dialtone-macos-arm64"),
        ("linux", "x86_64") => Some("dialtone-linux-x86_64"),
        ("linux", "aarch64") => Some("dialtone-linux-arm64"),
        _ => None,
    }
}

pub fn own_asset() -> Option<&'static str> {
    asset_name(std::env::consts::OS, std::env::consts::ARCH)
}

/// `"0.5.0"` or `"v0.5.0"` -> (0, 5, 0); a pre-release or build suffix is ignored.
pub fn parse_version(v: &str) -> Option<(u64, u64, u64)> {
    let core = v.trim().trim_start_matches('v');
    let core = core.split(['-', '+']).next()?;
    let mut parts = core.split('.').map(|p| p.parse::<u64>().ok());
    let v = (parts.next()??, parts.next()??, parts.next()??);
    parts.next().is_none().then_some(v)
}

/// Whether `candidate` is a later version than `current`.
pub fn newer(candidate: &str, current: &str) -> bool {
    match (parse_version(candidate), parse_version(current)) {
        (Some(c), Some(r)) => c > r,
        _ => false,
    }
}

/// The version this node runs: the release it installed, while its binary is still that
/// one, else the version it was built as.
pub fn running_version() -> &'static str {
    RUNNING_VERSION
        .get()
        .map(String::as_str)
        .unwrap_or(env!("CARGO_PKG_VERSION"))
}

/// What a node remembers about its updates, in `<state dir>/update.json`.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct UpdateState {
    /// The binary now at the path is on trial.
    #[serde(default)]
    pub trial: Option<Trial>,
    /// The release this node installed last and kept.
    #[serde(default)]
    pub committed: Option<Installed>,
    /// Versions that were rolled back; never tried again.
    #[serde(default)]
    pub bad: Vec<String>,
    /// The digest of the release binary this node keeps in its store for peers.
    #[serde(default)]
    pub shared: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Trial {
    pub version: String,
    pub sha256: String,
    /// How many times the trial binary has started.
    pub starts: u32,
    /// Whether the old binary had gossip neighbors (the new one must get one).
    pub had_neighbors: bool,
    pub installed_unix: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Installed {
    pub version: String,
    pub sha256: String,
}

/// The files a node's updater keeps, all in its state directory.
#[derive(Clone, Debug)]
pub struct Paths {
    pub state_dir: PathBuf,
    /// The running binary (the path its service starts).
    pub exe: PathBuf,
}

impl Paths {
    pub fn new(state_dir: &Path, exe: &Path) -> Self {
        Paths {
            state_dir: state_dir.to_path_buf(),
            exe: exe.to_path_buf(),
        }
    }

    /// This process's paths: the state dir in use and the binary it runs.
    pub fn current() -> Result<Self, String> {
        let exe = std::env::current_exe().map_err(|e| format!("current binary: {e}"))?;
        Ok(Paths::new(&crate::mesh::default_state_dir(), &exe))
    }

    fn sibling(&self, stem: &str) -> PathBuf {
        let ext = self
            .exe
            .extension()
            .map(|e| format!(".{}", e.to_string_lossy()))
            .unwrap_or_default();
        self.exe.with_file_name(format!("{stem}{ext}"))
    }

    /// The binary that ran before the one on trial (the B slot).
    pub fn prev(&self) -> PathBuf {
        self.sibling("dialtone.prev")
    }
    fn new_binary(&self) -> PathBuf {
        self.sibling("dialtone.new")
    }
    fn bad(&self) -> PathBuf {
        self.sibling("dialtone.bad")
    }
    fn state_file(&self) -> PathBuf {
        self.state_dir.join("update.json")
    }
    /// The signed manifest of the release this node runs, served to peers.
    pub fn manifest(&self) -> PathBuf {
        self.state_dir.join("update").join(MANIFEST_NAME)
    }
    fn staged_manifest(&self) -> PathBuf {
        self.state_dir
            .join("update")
            .join(format!("{MANIFEST_NAME}.trial"))
    }

    /// Only the binary a service runs from `<state dir>/bin` updates itself.
    pub fn managed(&self) -> bool {
        let canon = |p: &Path| p.canonicalize().unwrap_or_else(|_| p.to_path_buf());
        self.exe.parent().map(canon) == Some(canon(&self.state_dir.join("bin")))
    }

    pub fn load(&self) -> UpdateState {
        std::fs::read(self.state_file())
            .ok()
            .and_then(|b| serde_json::from_slice(&b).ok())
            .unwrap_or_default()
    }

    pub fn save(&self, state: &UpdateState) -> Result<(), String> {
        let path = self.state_file();
        let tmp = path.with_extension("json.tmp");
        let body = serde_json::to_vec_pretty(state).map_err(|e| e.to_string())?;
        std::fs::write(&tmp, body).map_err(|e| format!("{}: {e}", tmp.display()))?;
        std::fs::rename(&tmp, &path).map_err(|e| format!("{}: {e}", path.display()))
    }

    /// The signed manifest of the release this node runs, if it runs one.
    pub fn running_manifest(&self) -> Option<SignedManifest> {
        read_signed(&self.manifest())
    }
}

fn read_signed(path: &Path) -> Option<SignedManifest> {
    let manifest = std::fs::read_to_string(path).ok()?;
    let sig = std::fs::read_to_string(path.with_extension(sig_ext(path))).ok()?;
    Some(SignedManifest { manifest, sig })
}

fn sig_ext(path: &Path) -> String {
    format!(
        "{}.sig",
        path.extension()
            .map(|e| e.to_string_lossy())
            .unwrap_or_default()
    )
}

fn write_signed(path: &Path, signed: &SignedManifest) -> Result<(), String> {
    if let Some(dir) = path.parent() {
        std::fs::create_dir_all(dir).map_err(|e| format!("{}: {e}", dir.display()))?;
    }
    std::fs::write(path, &signed.manifest).map_err(|e| format!("{}: {e}", path.display()))?;
    std::fs::write(path.with_extension(sig_ext(path)), &signed.sig)
        .map_err(|e| format!("{}: {e}", path.display()))
}

pub fn sha256_hex(bytes: &[u8]) -> String {
    hex::encode(Sha256::digest(bytes))
}

fn sha256_file(path: &Path) -> Result<String, String> {
    let mut file = std::fs::File::open(path).map_err(|e| format!("{}: {e}", path.display()))?;
    let mut hasher = Sha256::new();
    let mut buf = vec![0u8; 1 << 20];
    loop {
        let n = file.read(&mut buf).map_err(|e| e.to_string())?;
        if n == 0 {
            break;
        }
        hasher.update(&buf[..n]);
    }
    Ok(hex::encode(hasher.finalize()))
}

/// Whether auto-update is on for this process (see the module docs).
pub fn enabled(paths: &Paths) -> bool {
    std::env::var("DIALTONE_AUTO_UPDATE").map_or(true, |v| v.trim() != "0") && paths.managed()
}

/// Called first when a node that owns its state dir starts. Sets the running version, and
/// for a binary on trial counts the start, rolls back after too many, or starts the watchdog
/// that keeps or rolls back the binary.
pub fn on_start(paths: &Paths) {
    let mut state = paths.load();
    let Ok(sha) = sha256_file(&paths.exe) else {
        return;
    };
    let mut version = None;
    if let Some(c) = &state.committed {
        if c.sha256 == sha {
            version = Some(c.version.clone());
        }
    }
    if let Some(mut trial) = state.trial.clone() {
        if trial.sha256 != sha {
            // Rolled back, or replaced by hand: nothing is on trial any more.
            state.trial = None;
            let _ = paths.save(&state);
        } else {
            trial.starts += 1;
            state.trial = Some(trial.clone());
            let _ = paths.save(&state);
            if trial.starts > MAX_TRIAL_STARTS {
                rollback(
                    paths,
                    &format!(
                        "v{} started {} times without becoming healthy",
                        trial.version, MAX_TRIAL_STARTS
                    ),
                );
            }
            println!(
                "[update] v{} is on trial (start {} of {MAX_TRIAL_STARTS}); it is kept once it is healthy",
                trial.version, trial.starts
            );
            if test_marker(&paths.exe) == Some(TestMarker::Panic) {
                panic!("test build that panics when it starts");
            }
            version = Some(trial.version.clone());
            let watched = paths.clone();
            std::thread::spawn(move || watch_trial(&watched, &trial));
        }
    }
    if let Some(v) = version {
        let _ = RUNNING_VERSION.set(v);
    }
}

/// Keep the binary on trial once it is healthy, or roll it back at the deadline.
fn watch_trial(paths: &Paths, trial: &Trial) {
    let started = Instant::now();
    let min_up = env_secs("DIALTONE_TRIAL_MIN_SECS", TRIAL_MIN_UP);
    let deadline = env_secs("DIALTONE_TRIAL_DEADLINE_SECS", TRIAL_DEADLINE);
    let hangs = test_marker(&paths.exe) == Some(TestMarker::Hang);
    loop {
        std::thread::sleep(Duration::from_millis(500));
        let healthy = !hangs
            && READY.load(Ordering::Relaxed)
            && (!trial.had_neighbors || NEIGHBORS.load(Ordering::Relaxed) > 0);
        if healthy && started.elapsed() >= min_up {
            match commit(paths, trial) {
                Ok(()) => println!("[update] kept v{}: it started healthy", trial.version),
                Err(e) => eprintln!("[update] could not record v{} as kept: {e}", trial.version),
            }
            return;
        }
        if started.elapsed() >= deadline {
            let why = if !READY.load(Ordering::Relaxed) || hangs {
                "did not join gossip"
            } else {
                "found no gossip neighbor"
            };
            rollback(
                paths,
                &format!("v{} {why} within {}s", trial.version, deadline.as_secs()),
            );
        }
    }
}

fn env_secs(name: &str, default: Duration) -> Duration {
    std::env::var(name)
        .ok()
        .and_then(|v| v.trim().parse().ok())
        .map(Duration::from_secs)
        .unwrap_or(default)
}

fn commit(paths: &Paths, trial: &Trial) -> Result<(), String> {
    let mut state = paths.load();
    state.trial = None;
    state.bad.retain(|v| v != &trial.version);
    state.committed = Some(Installed {
        version: trial.version.clone(),
        sha256: trial.sha256.clone(),
    });
    // State first: once the manifest is in place, the update actor shares the binary and
    // saves the state too.
    paths.save(&state)?;
    match read_signed(&paths.staged_manifest()) {
        Some(signed) => write_signed(&paths.manifest(), &signed),
        None => Ok(()),
    }
}

/// Put the previous binary back at the path, mark the trial version bad, and exit so the
/// service starts the previous binary. Returns only when there is nothing to roll back to.
fn rollback(paths: &Paths, why: &str) {
    let mut state = paths.load();
    let version = state.trial.as_ref().map(|t| t.version.clone());
    let prev = paths.prev();
    if !prev.exists() {
        eprintln!("[update] {why}, but there is no previous binary to go back to; keeping it");
        state.trial = None;
        let _ = paths.save(&state);
        return;
    }
    eprintln!("[update] rolling back: {why}");
    let _ = std::fs::remove_file(paths.bad());
    // A running binary can be renamed (also on Windows), not overwritten.
    if let Err(e) = std::fs::rename(&paths.exe, paths.bad()) {
        eprintln!("[update] rollback failed: {e}");
        return;
    }
    if let Err(e) = std::fs::rename(&prev, &paths.exe) {
        eprintln!("[update] rollback failed: {e}; restoring the trial binary");
        let _ = std::fs::rename(paths.bad(), &paths.exe);
        return;
    }
    if let Some(v) = version {
        if !state.bad.contains(&v) {
            state.bad.push(v);
        }
    }
    state.trial = None;
    let _ = paths.save(&state);
    eprintln!("[update] the previous binary is back; exiting so the service starts it");
    std::process::exit(EXIT_RESTART);
}

/// Install `bytes` (checked against `manifest`) as the binary at the path, keeping the running
/// one as `dialtone.prev`, and put it on trial. The caller then restarts the node.
pub fn install(
    paths: &Paths,
    signed: &SignedManifest,
    manifest: &Manifest,
    asset: &Asset,
    bytes: &[u8],
) -> Result<(), String> {
    let sha = sha256_hex(bytes);
    if sha != asset.sha256 || bytes.len() as u64 != asset.size {
        return Err(format!("{} does not match the signed manifest", asset.name));
    }
    let new = paths.new_binary();
    std::fs::write(&new, bytes).map_err(|e| format!("{}: {e}", new.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(&new, std::fs::Permissions::from_mode(0o755))
            .map_err(|e| e.to_string())?;
    }
    write_signed(&paths.staged_manifest(), signed)?;
    let prev = paths.prev();
    let _ = std::fs::remove_file(&prev);
    std::fs::rename(&paths.exe, &prev)
        .map_err(|e| format!("move the running binary aside: {e}"))?;
    if let Err(e) = std::fs::rename(&new, &paths.exe) {
        let _ = std::fs::rename(&prev, &paths.exe);
        return Err(format!("put the new binary in place: {e}"));
    }
    let mut state = paths.load();
    state.trial = Some(Trial {
        version: manifest.version.clone(),
        sha256: sha,
        starts: 0,
        had_neighbors: NEIGHBORS.load(Ordering::Relaxed) > 0,
        installed_unix: crate::now_unix(),
    });
    paths.save(&state)
}

/// Ask the node to shut down so its service starts the binary now at its path.
pub fn request_restart() {
    RESTART_REQUESTED.store(true, Ordering::Relaxed);
    restart_token().cancel();
}

pub fn restart_token() -> &'static tokio_util::sync::CancellationToken {
    RESTART.get_or_init(tokio_util::sync::CancellationToken::new)
}

pub fn restart_requested() -> bool {
    RESTART_REQUESTED.load(Ordering::Relaxed)
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
enum TestMarker {
    Panic,
    Hang,
}

/// Tests append a marker to a copy of the binary to make it a release that panics when it
/// starts, or never becomes healthy. Only debug builds look.
fn test_marker(exe: &Path) -> Option<TestMarker> {
    if !cfg!(debug_assertions) {
        return None;
    }
    let mut file = std::fs::File::open(exe).ok()?;
    file.seek(SeekFrom::End(-32)).ok()?;
    let mut tail = Vec::new();
    file.read_to_end(&mut tail).ok()?;
    let tail = String::from_utf8_lossy(&tail);
    if tail.contains("DIALTONE-TEST-PANIC") {
        Some(TestMarker::Panic)
    } else if tail.contains("DIALTONE-TEST-HANG") {
        Some(TestMarker::Hang)
    } else {
        None
    }
}

/// An HTTPS client with the same TLS setup as iroh (webpki roots, ring).
pub fn http_client() -> Result<reqwest::Client, String> {
    let tls = iroh::tls::CaTlsConfig::default()
        .client_config(iroh::tls::default_provider())
        .map_err(|e| format!("tls: {e}"))?;
    reqwest::Client::builder()
        .tls_backend_preconfigured(tls)
        .user_agent(concat!("dialtone/", env!("CARGO_PKG_VERSION")))
        .connect_timeout(Duration::from_secs(15))
        .build()
        .map_err(|e| format!("http client: {e}"))
}

pub fn releases_url() -> String {
    std::env::var("DIALTONE_RELEASE_URL")
        .ok()
        .filter(|u| !u.trim().is_empty())
        .unwrap_or_else(|| DEFAULT_RELEASES_URL.to_string())
        .trim_end_matches('/')
        .to_string()
}

async fn get_bytes(
    client: &reqwest::Client,
    url: &str,
    timeout: Duration,
) -> Result<Vec<u8>, String> {
    let resp = client
        .get(url)
        .timeout(timeout)
        .send()
        .await
        .map_err(|e| format!("GET {url}: {e}"))?;
    if !resp.status().is_success() {
        return Err(format!("GET {url}: {}", resp.status()));
    }
    Ok(resp
        .bytes()
        .await
        .map_err(|e| format!("GET {url}: {e}"))?
        .to_vec())
}

/// The latest release's signed manifest, from GitHub (or `DIALTONE_RELEASE_URL`).
pub async fn latest_manifest(client: &reqwest::Client) -> Result<SignedManifest, String> {
    let base = format!("{}/latest/download/{MANIFEST_NAME}", releases_url());
    let manifest = get_bytes(client, &base, Duration::from_secs(30)).await?;
    let sig = get_bytes(client, &format!("{base}.sig"), Duration::from_secs(30)).await?;
    Ok(SignedManifest {
        manifest: String::from_utf8(manifest).map_err(|_| "manifest is not UTF-8")?,
        sig: String::from_utf8_lossy(&sig).trim().to_string(),
    })
}

/// A release binary from GitHub (or `DIALTONE_RELEASE_URL`), checked against its manifest.
pub async fn download_asset(
    client: &reqwest::Client,
    manifest: &Manifest,
    asset: &Asset,
) -> Result<Vec<u8>, String> {
    let url = format!(
        "{}/download/v{}/{}",
        releases_url(),
        manifest.version,
        asset.name
    );
    let bytes = get_bytes(client, &url, Duration::from_secs(900)).await?;
    if sha256_hex(&bytes) != asset.sha256 {
        return Err(format!("{url} does not match the signed manifest"));
    }
    Ok(bytes)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn signed(key: &iroh::SecretKey, version: &str) -> SignedManifest {
        let manifest = serde_json::to_string(&serde_json::json!({
            "schema": MANIFEST_SCHEMA,
            "version": version,
            "assets": [{ "name": "dialtone-linux-x86_64", "sha256": "ab", "blake3": "cd", "size": 2 }],
        }))
        .unwrap();
        let sig = hex::encode(key.sign(manifest.as_bytes()).to_bytes());
        SignedManifest { manifest, sig }
    }

    /// A manifest verifies only with the key that signed it, and only unaltered.
    #[test]
    fn manifests_verify_only_signed_and_unaltered() {
        let key = iroh::SecretKey::from_bytes(&[7; 32]);
        let other = iroh::SecretKey::from_bytes(&[8; 32]);
        let s = signed(&key, "0.5.0");
        let m = verify_manifest(s.manifest.as_bytes(), &s.sig, &key.public()).unwrap();
        assert_eq!(m.version, "0.5.0");
        assert!(m.asset("dialtone-linux-x86_64").is_some());
        assert!(verify_manifest(s.manifest.as_bytes(), &s.sig, &other.public()).is_err());
        let altered = s.manifest.replace("0.5.0", "9.5.0");
        assert!(verify_manifest(altered.as_bytes(), &s.sig, &key.public()).is_err());
        assert!(verify_manifest(s.manifest.as_bytes(), "zz", &key.public()).is_err());
    }

    #[test]
    fn versions_compare_numerically() {
        assert_eq!(parse_version("v0.10.2"), Some((0, 10, 2)));
        assert_eq!(parse_version("1.2.3-rc1"), Some((1, 2, 3)));
        assert_eq!(parse_version("1.2"), None);
        assert!(newer("0.10.0", "0.9.9"));
        assert!(!newer("0.4.2", "0.4.2"));
        assert!(!newer("junk", "0.1.0"));
    }

    #[test]
    fn every_release_target_has_an_asset() {
        assert_eq!(
            asset_name("windows", "x86_64"),
            Some("dialtone-windows-x86_64.exe")
        );
        assert_eq!(asset_name("macos", "aarch64"), Some("dialtone-macos-arm64"));
        assert_eq!(asset_name("linux", "aarch64"), Some("dialtone-linux-arm64"));
        assert_eq!(asset_name("freebsd", "x86_64"), None);
    }

    /// install moves the running binary to the B slot and puts the new one on trial; the
    /// A/B files and state are what `on_start` and `rollback` work from.
    #[test]
    fn install_keeps_the_previous_binary_and_starts_a_trial() {
        let dir = std::env::temp_dir().join(format!("dialtone-update-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(dir.join("bin")).unwrap();
        let exe = dir.join("bin").join("dialtone");
        std::fs::write(&exe, b"old").unwrap();
        let paths = Paths::new(&dir, &exe);
        assert!(paths.managed());
        let manifest = Manifest {
            schema: MANIFEST_SCHEMA.into(),
            version: "0.9.0".into(),
            assets: vec![Asset {
                name: "x".into(),
                sha256: sha256_hex(b"new"),
                blake3: String::new(),
                size: 3,
            }],
        };
        let signed = SignedManifest {
            manifest: "{}".into(),
            sig: "00".into(),
        };
        let wrong = Asset {
            sha256: sha256_hex(b"other"),
            ..manifest.assets[0].clone()
        };
        assert!(install(&paths, &signed, &manifest, &wrong, b"new").is_err());
        install(&paths, &signed, &manifest, &manifest.assets[0], b"new").unwrap();
        assert_eq!(std::fs::read(&exe).unwrap(), b"new");
        assert_eq!(std::fs::read(paths.prev()).unwrap(), b"old");
        let trial = paths.load().trial.unwrap();
        assert_eq!((trial.version.as_str(), trial.starts), ("0.9.0", 0));
        assert_eq!(trial.sha256, sha256_hex(b"new"));
        commit(&paths, &trial).unwrap();
        let state = paths.load();
        assert!(state.trial.is_none());
        assert_eq!(state.committed.unwrap().version, "0.9.0");
        assert!(paths.running_manifest().is_some());
        let _ = std::fs::remove_dir_all(&dir);
    }
}
