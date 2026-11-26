//! Blob-backed plugin registry and installer (`dialtone install <plugin>`).
//!
//! Every installed plugin stores its canonical bundle/descriptor inside Dialtone's
//! content-addressed `iroh-blobs` `Blobs` store (tagged by SHA-256 and verified via
//! BLAKE3 streaming on peer pulls) and records its manifest at
//! `<state_dir>/plugins/<name>/manifest.json`.

use std::fs;
use std::io::Read;
use std::path::{Component, Path, PathBuf};
use std::process::{Command, Stdio};

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::blob::{digest_of, normalize_digest, Blobs, Store};
use crate::mesh::default_state_dir;
use crate::now_unix;

pub const PLUGIN_MANIFEST_SCHEMA: &str = "dialtone.plugin.v1";
pub const SHELL_PLUGIN_NAME: &str = "shell";
pub const MAX_PLUGIN_BYTES: usize = 32 * 1024 * 1024;

/// Built-in Dialtone CLI subcommands that always take precedence over installed plugins.
pub const BUILTIN_COMMANDS: &[&str] = &[
    "up",
    "join",
    "secret",
    "allowlist",
    "blob",
    "link",
    "invite",
    "daemon",
    "probe",
    "exec",
    "install",
    "plugin",
    "shell",
    "devices",
    "status",
    "help",
];

pub fn is_builtin_command(cmd: &str) -> bool {
    let lower = cmd.trim().to_ascii_lowercase();
    BUILTIN_COMMANDS.contains(&lower.as_str())
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginManifest {
    pub schema: String,
    pub name: String,
    pub version: String,
    pub kind: String,
    pub description: String,
    #[serde(default)]
    pub protocols: Vec<String>,
    pub blob_digest: String,
    pub blake3_hash: String,
    pub bytes: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub entrypoint: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ticket_uri: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub source_repo: Option<String>,
    pub installed_at_unix: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginStatus {
    pub manifest: PluginManifest,
    pub verified: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginShareReport {
    pub name: String,
    pub version: String,
    pub blob_digest: String,
    pub blake3_hash: String,
    pub bytes: u64,
    pub ticket_uri: String,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
struct BuiltinPluginBundle {
    schema: String,
    name: String,
    version: String,
    kind: String,
    description: String,
    protocols: Vec<String>,
    capabilities: Vec<String>,
}

pub fn default_plugins_dir() -> PathBuf {
    default_state_dir().join("plugins")
}

pub fn plugins_dir_for_store(store_dir: &Path) -> PathBuf {
    store_dir
        .parent()
        .map(|p| p.join("plugins"))
        .unwrap_or_else(|| store_dir.join("plugins"))
}

pub fn validate_plugin_name(raw: &str) -> Result<String, String> {
    let name = raw.trim().to_ascii_lowercase();
    if name.is_empty() || name.len() > 64 {
        return Err(format!(
            "invalid plugin name {raw:?}: must be 1..=64 ASCII characters"
        ));
    }
    let bytes = name.as_bytes();
    if !bytes[0].is_ascii_alphanumeric() {
        return Err(format!(
            "invalid plugin name {raw:?}: must start with an ASCII letter or digit"
        ));
    }
    if !bytes
        .iter()
        .all(|b| b.is_ascii_lowercase() || b.is_ascii_digit() || *b == b'-' || *b == b'_')
    {
        return Err(format!(
            "invalid plugin name {raw:?}: only [a-z0-9_-] allowed"
        ));
    }
    Ok(name)
}

fn manifest_path(plugins_dir: &Path, name: &str) -> PathBuf {
    plugins_dir.join(name).join("manifest.json")
}

pub fn builtin_plugin_bundle(name: &str) -> Result<(String, String, Vec<String>, Vec<u8>), String> {
    let canonical = validate_plugin_name(name)?;
    match canonical.as_str() {
        SHELL_PLUGIN_NAME => {
            let description =
                "Cross-OS persistent terminal session manager over Dialtone QUIC (replaces SSH + tmux)"
                    .to_string();
            let protocols = vec![crate::shell::ALPN_STR.to_string()];
            let bundle = BuiltinPluginBundle {
                schema: PLUGIN_MANIFEST_SCHEMA.to_string(),
                name: canonical,
                version: env!("CARGO_PKG_VERSION").to_string(),
                kind: "builtin-protocol".to_string(),
                description: description.clone(),
                protocols: protocols.clone(),
                capabilities: vec![
                    "session.new".to_string(),
                    "session.send".to_string(),
                    "session.capture".to_string(),
                    "session.ls".to_string(),
                    "session.kill".to_string(),
                    "shell.bash".to_string(),
                    "shell.powershell".to_string(),
                ],
            };
            let bytes = serde_json::to_vec_pretty(&bundle)
                .map_err(|e| format!("serialize builtin shell plugin bundle: {e}"))?;
            Ok(("builtin-protocol".to_string(), description, protocols, bytes))
        }
        other => Err(format!(
            "unknown built-in plugin '{other}'; pass --ticket <blob-ticket> or --file <path> to install a custom plugin (available built-ins: shell)"
        )),
    }
}

pub fn install_builtin_plugin(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
) -> Result<PluginManifest, String> {
    let canonical = validate_plugin_name(name)?;
    let (kind, description, protocols, bytes) = builtin_plugin_bundle(&canonical)?;
    install_plugin_from_bytes(
        blobs,
        plugins_dir,
        &canonical,
        &kind,
        &description,
        protocols,
        &bytes,
        None,
    )
}

#[allow(clippy::too_many_arguments)]
pub fn install_plugin_from_bytes(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
    kind: &str,
    description: &str,
    protocols: Vec<String>,
    bytes: &[u8],
    ticket_uri: Option<String>,
) -> Result<PluginManifest, String> {
    let canonical = validate_plugin_name(name)?;
    if bytes.is_empty() || bytes.len() > MAX_PLUGIN_BYTES {
        return Err(format!(
            "plugin '{canonical}' blob size {} bytes is out of bounds (1..={MAX_PLUGIN_BYTES})",
            bytes.len()
        ));
    }
    let (digest, hash) = futures_lite::future::block_on(blobs.put(bytes, None))?;
    let manifest = PluginManifest {
        schema: PLUGIN_MANIFEST_SCHEMA.to_string(),
        name: canonical.clone(),
        version: env!("CARGO_PKG_VERSION").to_string(),
        kind: kind.to_string(),
        description: description.to_string(),
        protocols,
        blob_digest: digest,
        blake3_hash: hash.to_string(),
        bytes: bytes.len() as u64,
        entrypoint: None,
        ticket_uri,
        source_repo: None,
        installed_at_unix: now_unix(),
    };
    write_manifest(plugins_dir, &manifest)?;
    Ok(manifest)
}

pub fn install_plugin_from_file(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
    file_path: &Path,
) -> Result<PluginManifest, String> {
    install_plugin_from_file_with_repo(blobs, plugins_dir, name, file_path, None)
}

pub fn install_plugin_from_file_with_repo(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
    file_path: &Path,
    source_repo: Option<&str>,
) -> Result<PluginManifest, String> {
    let canonical = validate_plugin_name(name)?;
    let validated_repo = match source_repo.map(str::trim).filter(|s| !s.is_empty()) {
        Some(r) => Some(validate_plugin_repo(r)?),
        None => None,
    };
    let bytes = fs::read(file_path)
        .map_err(|e| format!("read plugin file {}: {e}", file_path.display()))?;
    let (kind, description, protocols) = inspect_bundle_metadata(&canonical, &bytes);
    let mut manifest = install_plugin_from_bytes(
        blobs,
        plugins_dir,
        &canonical,
        &kind,
        &description,
        protocols,
        &bytes,
        None,
    )?;
    if validated_repo.is_some() {
        manifest.source_repo = validated_repo;
        write_manifest(plugins_dir, &manifest)?;
    }
    Ok(manifest)
}

pub fn validate_plugin_repo(raw: &str) -> Result<String, String> {
    let trimmed = raw.trim().trim_end_matches(".git");
    let Some((owner, repo)) = trimmed.split_once('/') else {
        return Err(format!(
            "invalid plugin repo {raw:?}: expected 'owner/repo' (e.g. timcash/dialtone-shell)"
        ));
    };
    let valid_part = |s: &str| {
        !s.is_empty()
            && s.len() <= 100
            && s.bytes()
                .all(|b| b.is_ascii_alphanumeric() || b == b'-' || b == b'_' || b == b'.')
    };
    if !valid_part(owner) || !valid_part(repo) || repo.contains('/') {
        return Err(format!(
            "invalid plugin repo {raw:?}: expected 'owner/repo'"
        ));
    }
    Ok(format!("{owner}/{repo}"))
}

pub fn plugin_asset_name(name: &str) -> Result<String, String> {
    let canonical = validate_plugin_name(name)?;
    let suffix = match (std::env::consts::OS, std::env::consts::ARCH) {
        ("windows", "x86_64") => "windows-x86_64.exe",
        ("macos", "aarch64") => "macos-arm64",
        ("linux", "x86_64") => "linux-x86_64",
        ("linux", "aarch64") => "linux-arm64",
        (os, arch) => {
            return Err(format!(
                "unsupported platform {os}/{arch} for plugin '{canonical}'"
            ))
        }
    };
    Ok(format!("dialtone-{canonical}-{suffix}"))
}

/// Download a standalone plugin binary from its own GitHub repository (`owner/repo`),
/// store it in Dialtone's blob store, and record its verified manifest.
/// Supports private repos when `gh` is authenticated locally or `GH_TOKEN`/`GITHUB_TOKEN` is set.
pub fn install_plugin_from_repo(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
    repo: &str,
) -> Result<PluginManifest, String> {
    let canonical = validate_plugin_name(name)?;
    let validated_repo = validate_plugin_repo(repo)?;
    let asset = plugin_asset_name(&canonical)?;
    let tmp_dir = std::env::temp_dir().join(format!(
        "dialtone-plugin-dl-{}-{}",
        std::process::id(),
        now_unix()
    ));
    let _ = fs::remove_dir_all(&tmp_dir);
    fs::create_dir_all(&tmp_dir)
        .map_err(|e| format!("create temp dir {}: {e}", tmp_dir.display()))?;

    let downloaded = tmp_dir.join(&asset);
    let gh_status = std::process::Command::new("gh")
        .args([
            "release",
            "download",
            "--repo",
            &validated_repo,
            "--pattern",
            &asset,
            "--dir",
            &tmp_dir.to_string_lossy(),
            "--clobber",
        ])
        .output();

    let got_via_gh = matches!(&gh_status, Ok(o) if o.status.success() && downloaded.is_file());
    if !got_via_gh {
        // Fallback: direct HTTPS download from latest release (with optional GH_TOKEN/GITHUB_TOKEN).
        let url = format!("https://github.com/{validated_repo}/releases/latest/download/{asset}");
        let token = std::env::var("GH_TOKEN")
            .or_else(|_| std::env::var("GITHUB_TOKEN"))
            .ok()
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty());
        let dl_url = url.clone();
        let body = std::thread::spawn(move || -> Result<Vec<u8>, String> {
            let rt = tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
                .map_err(|e| format!("create http runtime: {e}"))?;
            rt.block_on(async move {
                let client = reqwest::Client::builder()
                    .timeout(std::time::Duration::from_secs(60))
                    .build()
                    .map_err(|e| format!("build http client: {e}"))?;
                let mut req = client.get(&dl_url);
                if let Some(tok) = token {
                    req = req.bearer_auth(tok);
                }
                let resp = req
                    .send()
                    .await
                    .map_err(|e| format!("download plugin from {dl_url}: {e}"))?;
                if !resp.status().is_success() {
                    return Err(format!("HTTP {}", resp.status()));
                }
                resp.bytes()
                    .await
                    .map(|b| b.to_vec())
                    .map_err(|e| format!("read plugin response body: {e}"))
            })
        })
        .join()
        .map_err(|_| "plugin download thread panicked".to_string())?;
        let body = match body {
            Ok(b) => b,
            Err(http_err) => {
                let gh_hint = match gh_status {
                    Ok(o) => String::from_utf8_lossy(&o.stderr).trim().to_string(),
                    Err(e) => format!("gh unavailable: {e}"),
                };
                let _ = fs::remove_dir_all(&tmp_dir);
                return Err(format!(
                    "download {asset} from {validated_repo} failed ({http_err}); gh: {gh_hint}"
                ));
            }
        };
        fs::write(&downloaded, &body)
            .map_err(|e| format!("write {}: {e}", downloaded.display()))?;
    }

    let res = install_plugin_from_file_with_repo(
        blobs,
        plugins_dir,
        &canonical,
        &downloaded,
        Some(&validated_repo),
    );
    let _ = fs::remove_dir_all(&tmp_dir);
    res
}

/// Verify an installed `blob-binary` plugin against the `iroh-blobs` store and stage its
/// executable file under `<plugins_dir>/<name>/bin/`. Returns `Ok(None)` for non-binary plugins.
pub fn stage_plugin_binary(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
) -> Result<Option<(String, PathBuf)>, String> {
    let manifest = verify_installed_plugin(blobs, plugins_dir, name)?;
    if manifest.kind != "blob-binary" {
        return Ok(None);
    }
    let bytes = blobs.get_blocking(&manifest.blob_digest)?;
    let short_hex = manifest
        .blob_digest
        .strip_prefix("sha256:")
        .unwrap_or(&manifest.blob_digest);
    let short = &short_hex[..short_hex.len().min(12)];
    let ext = if cfg!(windows) { ".exe" } else { "" };
    let bin_dir = plugins_dir.join(&manifest.name).join("bin");
    fs::create_dir_all(&bin_dir)
        .map_err(|e| format!("create plugin bin dir {}: {e}", bin_dir.display()))?;
    let bin_path = bin_dir.join(format!("dialtone-{}-{short}{ext}", manifest.name));
    if !bin_path.is_file()
        || fs::read(&bin_path)
            .map(|b| digest_of(&b) != manifest.blob_digest)
            .unwrap_or(true)
    {
        let tmp = bin_dir.join(format!(".stage-{}-{}.tmp", std::process::id(), now_unix()));
        fs::write(&tmp, &bytes)
            .map_err(|e| format!("stage plugin binary {}: {e}", tmp.display()))?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&tmp, fs::Permissions::from_mode(0o755))
                .map_err(|e| format!("chmod staged plugin {}: {e}", tmp.display()))?;
        }
        fs::rename(&tmp, &bin_path).map_err(|e| {
            let _ = fs::remove_file(&tmp);
            format!("install staged plugin {}: {e}", bin_path.display())
        })?;
    }
    Ok(Some((manifest.blob_digest, bin_path)))
}

pub fn install_plugin_from_ticket(
    store: &Store,
    plugins_dir: &Path,
    name: &str,
    ticket_uri: &str,
    force: bool,
) -> Result<PluginManifest, String> {
    let canonical = validate_plugin_name(name)?;
    let report = store.pull_blob(ticket_uri, force)?;
    let bytes = store.blobs.get_blocking(&report.digest)?;
    let (kind, description, protocols) = inspect_bundle_metadata(&canonical, &bytes);
    let manifest = PluginManifest {
        schema: PLUGIN_MANIFEST_SCHEMA.to_string(),
        name: canonical,
        version: env!("CARGO_PKG_VERSION").to_string(),
        kind,
        description,
        protocols,
        blob_digest: report.digest,
        blake3_hash: report.blake3_hash,
        bytes: report.bytes,
        entrypoint: None,
        ticket_uri: Some(ticket_uri.to_string()),
        source_repo: None,
        installed_at_unix: now_unix(),
    };
    write_manifest(plugins_dir, &manifest)?;
    Ok(manifest)
}

fn looks_like_executable(bytes: &[u8]) -> bool {
    if bytes.len() < 4 {
        return false;
    }
    // ELF, PE/MZ, Mach-O (32/64/universal), or POSIX shebang script
    bytes.starts_with(b"\x7fELF")
        || bytes.starts_with(b"MZ")
        || bytes.starts_with(b"#!")
        || matches!(
            &bytes[..4],
            [0xcf, 0xfa, 0xed, 0xfe]
                | [0xce, 0xfa, 0xed, 0xfe]
                | [0xfe, 0xed, 0xfa, 0xcf]
                | [0xca, 0xfe, 0xba, 0xbe]
        )
}

fn inspect_bundle_metadata(name: &str, bytes: &[u8]) -> (String, String, Vec<String>) {
    if let Ok(bundle) = serde_json::from_slice::<BuiltinPluginBundle>(bytes) {
        if bundle.schema == PLUGIN_MANIFEST_SCHEMA {
            return (bundle.kind, bundle.description, bundle.protocols);
        }
    }
    if looks_like_executable(bytes) {
        let protocols = if name == SHELL_PLUGIN_NAME {
            vec![crate::shell::ALPN_STR.to_string()]
        } else {
            Vec::new()
        };
        return (
            "blob-binary".to_string(),
            format!("Standalone blob-backed Dialtone plugin binary '{name}'"),
            protocols,
        );
    }
    (
        "blob-bundle".to_string(),
        format!("Blob-backed Dialtone plugin '{name}'"),
        Vec::new(),
    )
}

fn write_manifest(plugins_dir: &Path, manifest: &PluginManifest) -> Result<(), String> {
    let dir = plugins_dir.join(&manifest.name);
    fs::create_dir_all(&dir).map_err(|e| format!("create plugin dir {}: {e}", dir.display()))?;
    let body = serde_json::to_vec_pretty(manifest)
        .map_err(|e| format!("serialize plugin manifest: {e}"))?;
    let path = manifest_path(plugins_dir, &manifest.name);
    let tmp = dir.join(format!("manifest.json.tmp.{}", std::process::id()));
    fs::write(&tmp, body).map_err(|e| format!("write {}: {e}", tmp.display()))?;
    fs::rename(&tmp, &path).map_err(|e| {
        let _ = fs::remove_file(&tmp);
        format!("install manifest {}: {e}", path.display())
    })
}

pub fn load_plugin_manifest(plugins_dir: &Path, name: &str) -> Result<PluginManifest, String> {
    let canonical = validate_plugin_name(name)?;
    let path = manifest_path(plugins_dir, &canonical);
    let raw = fs::read_to_string(&path).map_err(|_| {
        format!("plugin '{canonical}' is not installed; run 'dialtone install {canonical}'")
    })?;
    let manifest: PluginManifest = serde_json::from_str(&raw)
        .map_err(|e| format!("corrupt plugin manifest {}: {e}", path.display()))?;
    if manifest.schema != PLUGIN_MANIFEST_SCHEMA || manifest.name != canonical {
        return Err(format!(
            "plugin manifest mismatch for '{canonical}' at {}",
            path.display()
        ));
    }
    Ok(manifest)
}

pub fn verify_installed_plugin(
    blobs: &Blobs,
    plugins_dir: &Path,
    name: &str,
) -> Result<PluginManifest, String> {
    let manifest = load_plugin_manifest(plugins_dir, name)?;
    let expected = normalize_digest(&manifest.blob_digest)?;
    let bytes = blobs.get_blocking(&expected).map_err(|e| {
        format!(
            "plugin '{}' blob {} missing from store: {e}",
            manifest.name, expected
        )
    })?;
    let observed = digest_of(&bytes);
    if observed != expected {
        return Err(format!(
            "plugin '{}' blob digest mismatch: expected {expected}, observed {observed}",
            manifest.name
        ));
    }
    if bytes.len() as u64 != manifest.bytes {
        return Err(format!(
            "plugin '{}' size mismatch: expected {}, observed {}",
            manifest.name,
            manifest.bytes,
            bytes.len()
        ));
    }
    Ok(manifest)
}

pub fn is_plugin_installed_and_verified(blobs: &Blobs, plugins_dir: &Path, name: &str) -> bool {
    verify_installed_plugin(blobs, plugins_dir, name).is_ok()
}

pub fn share_installed_plugin(
    store: &Store,
    plugins_dir: &Path,
    name: &str,
) -> Result<PluginShareReport, String> {
    let manifest = verify_installed_plugin(&store.blobs, plugins_dir, name)?;
    let bytes = store.blobs.get_blocking(&manifest.blob_digest)?;
    let report = store.share_blob(&bytes, Some(&manifest.blob_digest))?;
    let mut updated = manifest.clone();
    updated.ticket_uri = Some(report.ticket_uri.clone());
    let _ = write_manifest(plugins_dir, &updated);
    Ok(PluginShareReport {
        name: manifest.name,
        version: manifest.version,
        blob_digest: report.digest,
        blake3_hash: report.blake3_hash,
        bytes: report.bytes,
        ticket_uri: report.ticket_uri,
    })
}

pub fn list_installed_plugins(
    blobs: &Blobs,
    plugins_dir: &Path,
) -> Result<Vec<PluginStatus>, String> {
    if !plugins_dir.exists() {
        return Ok(Vec::new());
    }
    let mut entries = Vec::new();
    let dir = fs::read_dir(plugins_dir)
        .map_err(|e| format!("read plugins dir {}: {e}", plugins_dir.display()))?;
    for entry in dir.flatten() {
        if !entry.path().is_dir() {
            continue;
        }
        let name = entry.file_name().to_string_lossy().to_string();
        if validate_plugin_name(&name).is_err() {
            continue;
        }
        match load_plugin_manifest(plugins_dir, &name) {
            Ok(manifest) => match verify_installed_plugin(blobs, plugins_dir, &name) {
                Ok(verified_manifest) => entries.push(PluginStatus {
                    manifest: verified_manifest,
                    verified: true,
                    error: None,
                }),
                Err(err) => entries.push(PluginStatus {
                    manifest,
                    verified: false,
                    error: Some(err),
                }),
            },
            Err(_) => continue,
        }
    }
    entries.sort_by(|a, b| a.manifest.name.cmp(&b.manifest.name));
    Ok(entries)
}

pub fn remove_installed_plugin(plugins_dir: &Path, name: &str) -> Result<(), String> {
    let canonical = validate_plugin_name(name)?;
    let dir = plugins_dir.join(&canonical);
    if !dir.exists() {
        return Err(format!("plugin '{canonical}' is not installed"));
    }
    fs::remove_dir_all(&dir)
        .map_err(|e| format!("remove plugin '{canonical}' at {}: {e}", dir.display()))
}

/// Runtime context passed into a dispatched plugin process via environment variables.
/// Secrets are never placed in argv or forwarded via env.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PluginDispatchContext {
    pub state_dir: PathBuf,
    pub plugins_dir: PathBuf,
    pub api_url: String,
    pub node_name: String,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ResolvedPlugin {
    pub manifest: PluginManifest,
    pub binary_path: PathBuf,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum PluginDispatchError {
    NotFound(String),
    Refused(String),
}

impl std::fmt::Display for PluginDispatchError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PluginDispatchError::NotFound(msg) | PluginDispatchError::Refused(msg) => {
                f.write_str(msg)
            }
        }
    }
}

/// List canonical names of plugins with valid manifests under `plugins_dir` without touching
/// the blob store (safe to call while the `dialtone up` daemon holds `store.lock`).
pub fn list_plugin_names(plugins_dir: &Path) -> Vec<String> {
    let Ok(dir) = fs::read_dir(plugins_dir) else {
        return Vec::new();
    };
    let mut names = Vec::new();
    for entry in dir.flatten() {
        if !entry.path().is_dir() {
            continue;
        }
        let name = entry.file_name().to_string_lossy().to_string();
        if load_plugin_manifest(plugins_dir, &name).is_ok() {
            names.push(name);
        }
    }
    names.sort();
    names.dedup();
    names
}

fn unknown_command_message(plugins_dir: &Path, cmd: &str) -> String {
    let installed = list_plugin_names(plugins_dir);
    if installed.is_empty() {
        format!(
            "unknown command '{cmd}' (no plugins installed; install one with 'dialtone install <plugin>')"
        )
    } else {
        format!(
            "unknown command '{cmd}' (installed plugins: {})",
            installed.join(", ")
        )
    }
}

fn safe_relative_entrypoint(raw: &str) -> Result<PathBuf, String> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return Err("empty plugin entrypoint".to_string());
    }
    let rel = PathBuf::from(trimmed);
    if rel.is_absolute() {
        return Err(format!("plugin entrypoint {raw:?} must be relative"));
    }
    for comp in rel.components() {
        match comp {
            Component::Normal(_) | Component::CurDir => {}
            Component::ParentDir | Component::RootDir | Component::Prefix(_) => {
                return Err(format!(
                    "plugin entrypoint {raw:?} cannot escape the plugin directory"
                ));
            }
        }
    }
    Ok(rel)
}

fn sha256_file_with_len(path: &Path) -> Result<(String, u64), String> {
    let mut file = fs::File::open(path)
        .map_err(|e| format!("open plugin binary {}: {e}", path.display()))?;
    let mut hasher = Sha256::new();
    let mut buf = [0u8; 64 * 1024];
    let mut total: u64 = 0;
    loop {
        let n = file
            .read(&mut buf)
            .map_err(|e| format!("read plugin binary {}: {e}", path.display()))?;
        if n == 0 {
            break;
        }
        hasher.update(&buf[..n]);
        total = total.saturating_add(n as u64);
    }
    Ok((format!("sha256:{}", hex::encode(hasher.finalize())), total))
}

fn locate_plugin_binary(
    plugins_dir: &Path,
    manifest: &PluginManifest,
) -> Result<PathBuf, PluginDispatchError> {
    let plugin_dir = plugins_dir.join(&manifest.name);
    let ext = std::env::consts::EXE_SUFFIX;

    if let Some(ref ep) = manifest.entrypoint {
        let rel = safe_relative_entrypoint(ep).map_err(PluginDispatchError::Refused)?;
        let candidate = plugin_dir.join(&rel);
        if candidate.is_file() {
            return Ok(candidate);
        }
        if !ext.is_empty() && !ep.ends_with(ext) {
            let with_ext = plugin_dir.join(format!("{ep}{ext}"));
            if with_ext.is_file() {
                return Ok(with_ext);
            }
        }
        return Err(PluginDispatchError::Refused(format!(
            "plugin '{}' entrypoint {} not found under {}",
            manifest.name,
            rel.display(),
            plugin_dir.display()
        )));
    }

    let short_hex = manifest
        .blob_digest
        .strip_prefix("sha256:")
        .unwrap_or(&manifest.blob_digest);
    let short = &short_hex[..short_hex.len().min(12)];

    let candidates = [
        plugin_dir.join(format!("dialtone-{}{ext}", manifest.name)),
        plugin_dir.join(format!("{}{ext}", manifest.name)),
        plugin_dir
            .join("bin")
            .join(format!("dialtone-{}-{short}{ext}", manifest.name)),
    ];
    for candidate in &candidates {
        if candidate.is_file() {
            return Ok(candidate.clone());
        }
    }

    Err(PluginDispatchError::Refused(format!(
        "plugin '{}' is installed in manifest.json but no executable binary was found in {} (expected dialtone-{}{ext})",
        manifest.name,
        plugin_dir.display(),
        manifest.name
    )))
}

/// Resolve an installed plugin binary from `<plugins_dir>/<name>/` and verify its SHA-256
/// digest and byte size against `manifest.json` before execution.
pub fn resolve_plugin_binary(
    plugins_dir: &Path,
    raw_name: &str,
) -> Result<ResolvedPlugin, PluginDispatchError> {
    if is_builtin_command(raw_name) {
        return Err(PluginDispatchError::Refused(format!(
            "'{raw_name}' is a built-in dialtone command and cannot be shadowed by a plugin"
        )));
    }
    let Ok(canonical) = validate_plugin_name(raw_name) else {
        return Err(PluginDispatchError::NotFound(unknown_command_message(
            plugins_dir,
            raw_name,
        )));
    };
    let manifest_file = manifest_path(plugins_dir, &canonical);
    if !manifest_file.is_file() {
        return Err(PluginDispatchError::NotFound(unknown_command_message(
            plugins_dir,
            raw_name,
        )));
    }
    let manifest = load_plugin_manifest(plugins_dir, &canonical)
        .map_err(PluginDispatchError::Refused)?;
    if manifest.kind == "builtin-protocol" {
        return Err(PluginDispatchError::Refused(format!(
            "plugin '{canonical}' is a built-in protocol plugin, not a standalone executable"
        )));
    }

    let binary_path = locate_plugin_binary(plugins_dir, &manifest)?;
    let expected =
        normalize_digest(&manifest.blob_digest).map_err(PluginDispatchError::Refused)?;
    let (observed, observed_bytes) =
        sha256_file_with_len(&binary_path).map_err(PluginDispatchError::Refused)?;
    if observed != expected {
        return Err(PluginDispatchError::Refused(format!(
            "refusing to execute plugin '{canonical}': digest mismatch for {} (expected {expected}, observed {observed})",
            binary_path.display()
        )));
    }
    if manifest.bytes > 0 && observed_bytes != manifest.bytes {
        return Err(PluginDispatchError::Refused(format!(
            "refusing to execute plugin '{canonical}': size mismatch for {} (expected {} bytes, observed {observed_bytes} bytes)",
            binary_path.display(),
            manifest.bytes
        )));
    }

    Ok(ResolvedPlugin {
        manifest,
        binary_path,
    })
}

/// Construct the `Command` to execute a verified plugin with passthrough args, inherited stdio,
/// safe Dialtone context env vars, and node secrets stripped.
pub fn build_plugin_command(
    ctx: &PluginDispatchContext,
    resolved: &ResolvedPlugin,
    args: &[String],
) -> Command {
    let mut cmd = Command::new(&resolved.binary_path);
    cmd.args(args)
        .stdin(Stdio::inherit())
        .stdout(Stdio::inherit())
        .stderr(Stdio::inherit());
    for secret_var in [
        "DIALTONE_SECRET",
        "DIALTONE_SECRET_KEY",
        "DIALTONE_GATEWAY_TOKEN",
        "MESH_ORIGIN_TOKEN",
    ] {
        cmd.env_remove(secret_var);
    }
    cmd.env("DIALTONE_HOME", &ctx.state_dir)
        .env("DIALTONE_STATE_DIR", &ctx.state_dir)
        .env("DIALTONE_API", &ctx.api_url)
        .env("DIALTONE_NODE", &ctx.node_name)
        .env("DIALTONE_PLUGIN_NAME", &resolved.manifest.name)
        .env(
            "DIALTONE_PLUGIN_DIR",
            ctx.plugins_dir.join(&resolved.manifest.name),
        );
    cmd
}

/// Verify and execute `<plugins_dir>/<name>/...` passing through args, stdio, and exit code.
pub fn dispatch_plugin(
    ctx: &PluginDispatchContext,
    name: &str,
    args: &[String],
) -> Result<i32, PluginDispatchError> {
    let resolved = resolve_plugin_binary(&ctx.plugins_dir, name)?;
    let mut cmd = build_plugin_command(ctx, &resolved, args);
    let status = cmd.status().map_err(|e| {
        PluginDispatchError::Refused(format!(
            "failed to execute plugin '{}' ({}): {e}",
            resolved.manifest.name,
            resolved.binary_path.display()
        ))
    })?;
    if let Some(code) = status.code() {
        return Ok(code);
    }
    #[cfg(unix)]
    {
        use std::os::unix::process::ExitStatusExt;
        if let Some(sig) = status.signal() {
            return Ok(128 + sig);
        }
    }
    Ok(1)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::node::{build_endpoint_builder, IrohTransportConfig};
    use iroh::address_lookup::memory::MemoryLookup;
    use iroh::protocol::Router;
    use iroh_blobs::BlobsProtocol;
    use std::sync::atomic::{AtomicU64, Ordering};

    fn temp_plugin_env(tag: &str) -> (PathBuf, PathBuf, Blobs) {
        static NEXT: AtomicU64 = AtomicU64::new(1);
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let root = std::env::temp_dir().join(format!(
            "dialtone-plugin-test-{tag}-{}-{id}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&root);
        let store_dir = root.join("blobs");
        let plugins_dir = root.join("plugins");
        let blobs = Blobs::open_blocking(&store_dir).expect("open blob store");
        (root, plugins_dir, blobs)
    }

    #[test]
    fn plugin_name_validation_rejects_traversal_and_bad_chars() {
        assert_eq!(validate_plugin_name("shell").unwrap(), "shell");
        assert_eq!(
            validate_plugin_name("  My-Plugin_1 ").unwrap(),
            "my-plugin_1"
        );
        for bad in [
            "",
            "../shell",
            "a/b",
            "a\\b",
            "-leading-dash",
            "has space",
            "semi;colon",
            &"a".repeat(65),
        ] {
            assert!(
                validate_plugin_name(bad).is_err(),
                "expected invalid name {bad:?} to be rejected"
            );
        }
    }

    #[test]
    fn plugin_pack_and_install_stores_verified_blob_in_iroh_store() {
        let (root, plugins_dir, blobs) = temp_plugin_env("pack");
        let manifest = install_builtin_plugin(&blobs, &plugins_dir, "shell").unwrap();
        assert_eq!(manifest.name, "shell");
        assert_eq!(manifest.kind, "builtin-protocol");
        assert!(manifest.protocols.iter().any(|p| p == "dialtone/shell/1"));
        assert!(blobs.has_blocking(&manifest.blob_digest));
        assert!(is_plugin_installed_and_verified(
            &blobs,
            &plugins_dir,
            "shell"
        ));
        blobs.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn plugin_tamper_detection_fails_closed_on_digest_mismatch() {
        let (root, plugins_dir, blobs) = temp_plugin_env("tamper");
        let mut manifest = install_builtin_plugin(&blobs, &plugins_dir, "shell").unwrap();
        // Tamper manifest to reference a different digest not matching the blob.
        manifest.blob_digest = digest_of(b"tampered payload");
        write_manifest(&plugins_dir, &manifest).unwrap();

        assert!(!is_plugin_installed_and_verified(
            &blobs,
            &plugins_dir,
            "shell"
        ));
        let err = verify_installed_plugin(&blobs, &plugins_dir, "shell").unwrap_err();
        assert!(
            err.contains("missing from store") || err.contains("mismatch"),
            "{err}"
        );

        // Also test size tampering while digest is present in store.
        let mut good = install_builtin_plugin(&blobs, &plugins_dir, "shell").unwrap();
        good.bytes += 42;
        write_manifest(&plugins_dir, &good).unwrap();
        let err = verify_installed_plugin(&blobs, &plugins_dir, "shell").unwrap_err();
        assert!(err.contains("size mismatch"), "{err}");

        blobs.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn plugin_list_info_and_remove_lifecycle() {
        let (root, plugins_dir, blobs) = temp_plugin_env("lifecycle");
        install_builtin_plugin(&blobs, &plugins_dir, "shell").unwrap();
        let custom_file = root.join("custom.bin");
        fs::write(&custom_file, b"custom-plugin-bytes-v1").unwrap();
        install_plugin_from_file(&blobs, &plugins_dir, "custom-tool", &custom_file).unwrap();

        let list = list_installed_plugins(&blobs, &plugins_dir).unwrap();
        assert_eq!(list.len(), 2);
        assert_eq!(list[0].manifest.name, "custom-tool");
        assert!(list[0].verified);
        assert_eq!(list[1].manifest.name, "shell");
        assert!(list[1].verified);

        remove_installed_plugin(&plugins_dir, "custom-tool").unwrap();
        let list2 = list_installed_plugins(&blobs, &plugins_dir).unwrap();
        assert_eq!(list2.len(), 1);
        assert_eq!(list2[0].manifest.name, "shell");

        blobs.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn plugin_install_from_remote_peer_over_iroh_blobs_quic() {
        let (root_a, plugins_a, blobs_a) = temp_plugin_env("peer-a");
        let (root_b, plugins_b, blobs_b) = temp_plugin_env("peer-b");

        let rt = tokio::runtime::Runtime::new().unwrap();
        let (store_a, store_b, router_a) = rt.block_on(async {
            let bind = || async {
                build_endpoint_builder(&IrohTransportConfig::local_offline())
                    .unwrap()
                    .bind()
                    .await
                    .unwrap()
            };
            let (ep_a, ep_b) = (bind().await, bind().await);
            let router_a = Router::builder(ep_a.clone())
                .accept(iroh_blobs::ALPN, BlobsProtocol::new(blobs_a.api(), None))
                .spawn();
            let cfg = IrohTransportConfig::local_offline();
            let store_a = Store::new(ep_a, blobs_a.clone(), MemoryLookup::new(), cfg.clone());
            let lookup_b = MemoryLookup::new();
            ep_b.address_lookup().unwrap().add(lookup_b.clone());
            let store_b = Store::new(ep_b, blobs_b.clone(), lookup_b, cfg);
            (store_a, store_b, router_a)
        });

        install_builtin_plugin(&blobs_a, &plugins_a, "shell").unwrap();
        let shared = share_installed_plugin(&store_a, &plugins_a, "shell").unwrap();
        assert!(
            !shared.ticket_uri.is_empty() && shared.ticket_uri.contains("blob"),
            "ticket={}",
            shared.ticket_uri
        );

        // Node B installs `shell` purely from Node A's blob ticket over QUIC!
        let installed_b =
            install_plugin_from_ticket(&store_b, &plugins_b, "shell", &shared.ticket_uri, false)
                .unwrap();
        assert_eq!(installed_b.name, "shell");
        assert_eq!(installed_b.blob_digest, shared.blob_digest);
        assert!(is_plugin_installed_and_verified(
            &blobs_b, &plugins_b, "shell"
        ));

        rt.block_on(async {
            let _ = router_a.shutdown().await;
        });
        drop((store_a, store_b));
        blobs_a.close_blocking();
        blobs_b.close_blocking();
        let _ = fs::remove_dir_all(&root_a);
        let _ = fs::remove_dir_all(&root_b);
    }

    fn write_test_plugin(
        plugins_dir: &Path,
        name: &str,
        script_unix: &str,
        script_windows: &str,
        entrypoint_override: Option<&str>,
    ) -> (PathBuf, PluginManifest) {
        let dir = plugins_dir.join(name);
        fs::create_dir_all(&dir).unwrap();
        let (filename, body, entrypoint) = if cfg!(windows) {
            let fname = format!("dialtone-{name}.cmd");
            let ep = entrypoint_override
                .map(str::to_string)
                .or_else(|| Some(fname.clone()));
            (fname, script_windows.as_bytes().to_vec(), ep)
        } else {
            let fname = format!("dialtone-{name}");
            (
                fname,
                script_unix.as_bytes().to_vec(),
                entrypoint_override.map(str::to_string),
            )
        };
        let bin_path = dir.join(&filename);
        fs::write(&bin_path, &body).unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            fs::set_permissions(&bin_path, fs::Permissions::from_mode(0o755)).unwrap();
        }
        let manifest = PluginManifest {
            schema: PLUGIN_MANIFEST_SCHEMA.to_string(),
            name: name.to_string(),
            version: "0.1.0".to_string(),
            kind: "blob-binary".to_string(),
            description: format!("test plugin {name}"),
            protocols: Vec::new(),
            blob_digest: digest_of(&body),
            blake3_hash: "00".repeat(32),
            bytes: body.len() as u64,
            entrypoint,
            ticket_uri: None,
            source_repo: None,
            installed_at_unix: now_unix(),
        };
        write_manifest(plugins_dir, &manifest).unwrap();
        (bin_path, manifest)
    }

    #[test]
    fn dispatch_found_verifies_digest_and_passes_args_and_env() {
        let (root, plugins_dir, blobs) = temp_plugin_env("dispatch-found");
        blobs.close_blocking();

        write_test_plugin(
            &plugins_dir,
            "task",
            "#!/bin/sh\nprintf 'NODE=%s\\nAPI=%s\\nHOME=%s\\nNAME=%s\\nSECRET=%s\\nARGS=%s\\n' \"$DIALTONE_NODE\" \"$DIALTONE_API\" \"$DIALTONE_HOME\" \"$DIALTONE_PLUGIN_NAME\" \"$DIALTONE_SECRET\" \"$*\"\n",
            "@echo off\r\necho NODE=%DIALTONE_NODE%\r\necho API=%DIALTONE_API%\r\necho HOME=%DIALTONE_HOME%\r\necho NAME=%DIALTONE_PLUGIN_NAME%\r\necho SECRET=%DIALTONE_SECRET%\r\necho ARGS=%*\r\n",
            None,
        );

        let ctx = PluginDispatchContext {
            state_dir: root.clone(),
            plugins_dir: plugins_dir.clone(),
            api_url: "http://127.0.0.1:8788".to_string(),
            node_name: "rosie".to_string(),
        };
        let resolved = resolve_plugin_binary(&plugins_dir, "task").unwrap();
        let args = vec![
            "list".to_string(),
            "--json".to_string(),
            "--help".to_string(),
            "hello world".to_string(),
        ];
        let mut cmd = build_plugin_command(&ctx, &resolved, &args);
        cmd.stdout(Stdio::piped()).stderr(Stdio::piped());
        let out = cmd.output().expect("run dispatched plugin");
        assert!(out.status.success(), "stderr={}", String::from_utf8_lossy(&out.stderr));
        let stdout = String::from_utf8_lossy(&out.stdout);
        assert!(stdout.contains("NODE=rosie"), "{stdout}");
        assert!(stdout.contains("API=http://127.0.0.1:8788"), "{stdout}");
        assert!(
            stdout.contains(&format!("HOME={}", root.display())),
            "{stdout}"
        );
        assert!(stdout.contains("NAME=task"), "{stdout}");
        assert!(stdout.contains("SECRET=\n") || stdout.contains("SECRET=\r\n"), "{stdout}");
        assert!(
            stdout.contains("ARGS=list --json --help hello world"),
            "{stdout}"
        );

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn dispatch_not_found_lists_installed_plugins() {
        let (root, plugins_dir, blobs) = temp_plugin_env("dispatch-not-found");
        blobs.close_blocking();

        let empty_err = resolve_plugin_binary(&plugins_dir, "missing").unwrap_err();
        match empty_err {
            PluginDispatchError::NotFound(msg) => {
                assert!(msg.contains("unknown command 'missing'"), "{msg}");
                assert!(msg.contains("no plugins installed"), "{msg}");
            }
            other => panic!("expected NotFound, got {other:?}"),
        }

        write_test_plugin(&plugins_dir, "task", "#!/bin/sh\nexit 0\n", "@echo off\r\nexit /b 0\r\n", None);
        write_test_plugin(&plugins_dir, "alpha", "#!/bin/sh\nexit 0\n", "@echo off\r\nexit /b 0\r\n", None);

        let err = resolve_plugin_binary(&plugins_dir, "missing").unwrap_err();
        match err {
            PluginDispatchError::NotFound(msg) => {
                assert!(msg.contains("unknown command 'missing'"), "{msg}");
                assert!(msg.contains("alpha, task"), "{msg}");
            }
            other => panic!("expected NotFound, got {other:?}"),
        }

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn dispatch_refuses_digest_mismatch_and_size_mismatch() {
        let (root, plugins_dir, blobs) = temp_plugin_env("dispatch-tamper");
        blobs.close_blocking();

        let (bin_path, mut manifest) = write_test_plugin(
            &plugins_dir,
            "task",
            "#!/bin/sh\necho ok\n",
            "@echo off\r\necho ok\r\n",
            None,
        );

        // Tamper the binary on disk after manifest write.
        fs::write(&bin_path, b"#!/bin/sh\necho tampered\n").unwrap();
        let err = resolve_plugin_binary(&plugins_dir, "task").unwrap_err();
        match err {
            PluginDispatchError::Refused(msg) => {
                assert!(msg.contains("digest mismatch"), "{msg}");
            }
            other => panic!("expected Refused on digest mismatch, got {other:?}"),
        }

        // Restore bytes matching digest, but tamper expected byte size in manifest.
        let bytes = b"#!/bin/sh\necho ok\n";
        fs::write(&bin_path, bytes).unwrap();
        manifest.blob_digest = digest_of(bytes);
        manifest.bytes = (bytes.len() as u64) + 10;
        write_manifest(&plugins_dir, &manifest).unwrap();
        let size_err = resolve_plugin_binary(&plugins_dir, "task").unwrap_err();
        match size_err {
            PluginDispatchError::Refused(msg) => {
                assert!(msg.contains("size mismatch"), "{msg}");
            }
            other => panic!("expected Refused on size mismatch, got {other:?}"),
        }

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn dispatch_builtin_commands_always_win_over_plugin_names() {
        let (root, plugins_dir, blobs) = temp_plugin_env("dispatch-builtin");
        blobs.close_blocking();

        for builtin in ["status", "exec", "plugin", "install", "shell", "up", "help"] {
            assert!(is_builtin_command(builtin), "missing builtin {builtin}");
            assert!(
                is_builtin_command(&builtin.to_ascii_uppercase()),
                "case-insensitive check failed for {builtin}"
            );
        }
        assert!(!is_builtin_command("task"));

        // Even if a valid plugin directory named "status" exists, dispatch refuses it.
        write_test_plugin(
            &plugins_dir,
            "status",
            "#!/bin/sh\necho hijacked\n",
            "@echo off\r\necho hijacked\r\n",
            None,
        );
        let err = resolve_plugin_binary(&plugins_dir, "status").unwrap_err();
        match err {
            PluginDispatchError::Refused(msg) => {
                assert!(msg.contains("built-in"), "{msg}");
            }
            other => panic!("expected Refused for builtin name, got {other:?}"),
        }

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn dispatch_passes_through_exit_code() {
        let (root, plugins_dir, blobs) = temp_plugin_env("dispatch-exit");
        blobs.close_blocking();

        write_test_plugin(
            &plugins_dir,
            "exiter",
            "#!/bin/sh\nexit \"$1\"\n",
            "@echo off\r\nexit /b %1\r\n",
            None,
        );
        let ctx = PluginDispatchContext {
            state_dir: root.clone(),
            plugins_dir: plugins_dir.clone(),
            api_url: "http://127.0.0.1:8788".to_string(),
            node_name: "rosie".to_string(),
        };
        assert_eq!(
            dispatch_plugin(&ctx, "exiter", &["0".to_string()]).unwrap(),
            0
        );
        assert_eq!(
            dispatch_plugin(&ctx, "exiter", &["42".to_string()]).unwrap(),
            42
        );

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn dispatch_rejects_entrypoint_path_traversal() {
        let (root, plugins_dir, blobs) = temp_plugin_env("dispatch-traversal");
        blobs.close_blocking();

        write_test_plugin(
            &plugins_dir,
            "evil",
            "#!/bin/sh\nexit 0\n",
            "@echo off\r\nexit /b 0\r\n",
            Some("../outside"),
        );
        let err = resolve_plugin_binary(&plugins_dir, "evil").unwrap_err();
        match err {
            PluginDispatchError::Refused(msg) => {
                assert!(msg.contains("cannot escape"), "{msg}");
            }
            other => panic!("expected Refused on traversal, got {other:?}"),
        }

        let _ = fs::remove_dir_all(&root);
    }
}
