//! Blob-backed plugin registry and installer (`dialtone install <plugin>`).
//!
//! Every installed plugin stores its canonical bundle/descriptor inside Dialtone's
//! content-addressed `iroh-blobs` `Blobs` store (tagged by SHA-256 and verified via
//! BLAKE3 streaming on peer pulls) and records its manifest at
//! `<state_dir>/plugins/<name>/manifest.json`.

use std::fs;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::blob::{digest_of, normalize_digest, Blobs, Store};
use crate::mesh::default_state_dir;
use crate::now_unix;

pub const PLUGIN_MANIFEST_SCHEMA: &str = "dialtone.plugin.v1";
pub const SHELL_PLUGIN_NAME: &str = "shell";
pub const MAX_PLUGIN_BYTES: usize = 32 * 1024 * 1024;

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
}
