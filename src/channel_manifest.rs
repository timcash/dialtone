//! Signed per-channel policy manifest (`dialtone.channel.manifest.v1`) and fail-closed enforcement.
//!
//! Each channel can carry a signed manifest at `<state_dir>/channels/<channel_id>/manifest.json`.
//! A single creator admin signs the canonical JSON representation (excluding the `signature` field)
//! using their Ed25519 key (the node's `iroh-secret-key` or a dedicated `0600` `admin.key` under
//! the channel directory). Updates must carry the recorded admin key, a strictly higher `seq`, and
//! a valid Ed25519 signature.
//!
//! Enforcement is fail-closed when a channel has a manifest: any plugin not listed in
//! `allowed_plugins` (or violating optional constraints) is refused at install, CLI dispatch,
//! daemon-managed plugin start, and `dialtone/shell/1` handling. Legacy channels with no manifest
//! preserve current behavior and emit a warning entry to the channel log.

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, RwLock};

use serde::{Deserialize, Serialize};
use tokio::sync::mpsc;

use crate::blob::normalize_digest;
use crate::channel_log::{channel_log_path, ChannelLogEntry, ChannelLogStore};
use crate::mesh::{channel_dir, channel_plugin_data_dir};
use crate::node::load_or_create_secret_key_at;
use crate::now_unix;
use crate::plugin::validate_plugin_name;

pub const CHANNEL_MANIFEST_SCHEMA: &str = "dialtone.channel.manifest.v1";

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct PluginConstraint {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub min_version: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub blob_digest: Option<String>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub blob_digests: BTreeMap<String, String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ChannelManifest {
    pub schema: String,
    pub channel_id: String,
    pub channel_name: String,
    pub admin: String,
    pub seq: u64,
    #[serde(default)]
    pub allowed_plugins: Vec<String>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub plugin_constraints: BTreeMap<String, PluginConstraint>,
    pub issued_at_unix: u64,
    pub signature: String,
}

#[derive(Serialize)]
struct ChannelManifestUnsigned<'a> {
    schema: &'a str,
    channel_id: &'a str,
    channel_name: &'a str,
    admin: &'a str,
    seq: u64,
    allowed_plugins: &'a [String],
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    plugin_constraints: &'a BTreeMap<String, PluginConstraint>,
    issued_at_unix: u64,
}

/// Summary view returned inside `GET /api/channels` per `API.md`.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ChannelManifestSummary {
    pub schema: String,
    pub seq: u64,
    pub admin: String,
    pub allowed_plugins: Vec<String>,
    pub signature_valid: bool,
}

/// Gossip message wrapper for propagating signed channel manifests over `events_topic_id`.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ManifestEnvelope {
    pub node: String,
    pub node_id: String,
    pub manifest: ChannelManifest,
    #[serde(default)]
    pub sent_unix_ms: u64,
}

/// Lightweight per-channel metadata persisted at `<state_dir>/channels/<channel_id>/config.json`.
/// Never contains raw secrets or private keys.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ChannelLocalMeta {
    pub channel_id: String,
    pub name: String,
    #[serde(default)]
    pub joined_at_unix: u64,
}

impl ChannelManifest {
    /// Canonical JSON bytes excluding the `signature` field.
    pub fn canonical_signing_bytes(&self) -> Result<Vec<u8>, String> {
        let unsigned = ChannelManifestUnsigned {
            schema: &self.schema,
            channel_id: &self.channel_id,
            channel_name: &self.channel_name,
            admin: &self.admin,
            seq: self.seq,
            allowed_plugins: &self.allowed_plugins,
            plugin_constraints: &self.plugin_constraints,
            issued_at_unix: self.issued_at_unix,
        };
        serde_json::to_vec(&unsigned)
            .map_err(|e| format!("serialize canonical channel manifest: {e}"))
    }

    /// Create and sign a new `seq = 1` channel manifest with `admin_key` as creator admin.
    pub fn new_signed(
        channel_id: &str,
        channel_name: &str,
        allowed_plugins: &[String],
        admin_key: &iroh::SecretKey,
    ) -> Result<Self, String> {
        let cid = validate_channel_id(channel_id)?;
        let mut clean_plugins = Vec::new();
        for p in allowed_plugins {
            let trimmed = p.trim();
            if trimmed.is_empty() {
                continue;
            }
            let canonical = validate_plugin_name(trimmed)?;
            if !clean_plugins.contains(&canonical) {
                clean_plugins.push(canonical);
            }
        }
        clean_plugins.sort();
        let mut manifest = Self {
            schema: CHANNEL_MANIFEST_SCHEMA.to_string(),
            channel_id: cid,
            channel_name: channel_name.trim().to_string(),
            admin: admin_key.public().to_string(),
            seq: 1,
            allowed_plugins: clean_plugins,
            plugin_constraints: BTreeMap::new(),
            issued_at_unix: now_unix(),
            signature: String::new(),
        };
        manifest.sign_in_place(admin_key)?;
        Ok(manifest)
    }

    /// Sign this manifest in place using `admin_key`. Refuses if `admin_key` does not match `self.admin`.
    pub fn sign_in_place(&mut self, admin_key: &iroh::SecretKey) -> Result<(), String> {
        let key_pub = admin_key.public().to_string();
        if !self.admin.eq_ignore_ascii_case(&key_pub) {
            return Err(format!(
                "refusing to sign manifest for channel '{}': local key {} is not the recorded admin {}",
                self.channel_id, key_pub, self.admin
            ));
        }
        self.admin = key_pub;
        let payload = self.canonical_signing_bytes()?;
        let sig = admin_key.sign(&payload);
        self.signature = crate::hex_encode(&sig.to_bytes());
        Ok(())
    }

    /// Bump `seq`, refresh `issued_at_unix`, and re-sign.
    pub fn bump_and_sign(&mut self, admin_key: &iroh::SecretKey) -> Result<(), String> {
        self.seq = self.seq.saturating_add(1);
        self.issued_at_unix = now_unix();
        self.sign_in_place(admin_key)
    }

    /// Allow `plugin` (with optional constraint), bump `seq`, and re-sign.
    pub fn allow_plugin(
        &mut self,
        plugin: &str,
        constraint: Option<PluginConstraint>,
        admin_key: &iroh::SecretKey,
    ) -> Result<(), String> {
        let canonical = validate_plugin_name(plugin)?;
        if !self.allowed_plugins.iter().any(|p| p == &canonical) {
            self.allowed_plugins.push(canonical.clone());
            self.allowed_plugins.sort();
        }
        if let Some(c) = constraint {
            if c.min_version.is_some() || c.blob_digest.is_some() || !c.blob_digests.is_empty() {
                self.plugin_constraints.insert(canonical, c);
            }
        }
        self.bump_and_sign(admin_key)
    }

    /// Remove `plugin` from `allowed_plugins` (and its constraints), bump `seq`, and re-sign.
    pub fn deny_plugin(
        &mut self,
        plugin: &str,
        admin_key: &iroh::SecretKey,
    ) -> Result<(), String> {
        let canonical = validate_plugin_name(plugin)?;
        self.allowed_plugins.retain(|p| p != &canonical);
        self.plugin_constraints.remove(&canonical);
        self.bump_and_sign(admin_key)
    }

    /// Verify schema, sequence number, admin public key, and Ed25519 signature over canonical JSON.
    pub fn verify(&self) -> Result<(), String> {
        if self.schema != CHANNEL_MANIFEST_SCHEMA {
            return Err(format!(
                "unsupported channel manifest schema '{}' (expected '{CHANNEL_MANIFEST_SCHEMA}')",
                self.schema
            ));
        }
        validate_channel_id(&self.channel_id)?;
        if self.seq == 0 {
            return Err("channel manifest seq must be >= 1".to_string());
        }
        let pubkey: iroh::PublicKey = self
            .admin
            .trim()
            .parse()
            .map_err(|_| format!("invalid admin EndpointId '{}'", self.admin))?;
        let sig_vec = crate::hex_decode(&self.signature)
            .map_err(|e| format!("invalid manifest signature hex: {e}"))?;
        let sig_bytes: [u8; 64] = sig_vec
            .try_into()
            .map_err(|_| "manifest signature must be 64 bytes".to_string())?;
        let payload = self.canonical_signing_bytes()?;
        pubkey
            .verify(&payload, &iroh::Signature::from_bytes(&sig_bytes))
            .map_err(|_| {
                format!(
                    "channel manifest signature verification failed for channel '{}' (admin={})",
                    self.channel_id, self.admin
                )
            })
    }

    /// Verify this manifest belongs to `expected_channel_id` and has a valid Ed25519 signature.
    pub fn verify_for_channel(&self, expected_channel_id: &str) -> Result<(), String> {
        if !self.channel_id.eq_ignore_ascii_case(expected_channel_id) {
            return Err(format!(
                "channel manifest channel_id '{}' does not match channel '{expected_channel_id}'",
                self.channel_id
            ));
        }
        self.verify()
    }

    /// Whether `plugin_name` is permitted by `allowed_plugins`.
    pub fn allows_plugin(&self, plugin_name: &str) -> bool {
        let target = plugin_name.trim().to_ascii_lowercase();
        self.allowed_plugins
            .iter()
            .any(|p| p.trim().eq_ignore_ascii_case(&target))
    }

    /// Check optional per-plugin constraints (`min_version`, `blob_digest` pins).
    pub fn check_plugin_constraints(
        &self,
        canonical_plugin: &str,
        version: Option<&str>,
        blob_digest: Option<&str>,
    ) -> Result<(), String> {
        let Some(constraint) = self.plugin_constraints.get(canonical_plugin) else {
            return Ok(());
        };
        if let (Some(ref min_ver), Some(ver)) = (&constraint.min_version, version) {
            if crate::update::newer(min_ver, ver) {
                return Err(format!(
                    "plugin '{canonical_plugin}' version '{ver}' is below channel manifest min_version '{min_ver}'"
                ));
            }
        }
        if let Some(observed_raw) = blob_digest {
            let observed = normalize_digest(observed_raw)?;
            if let Some(ref pinned) = constraint.blob_digest {
                let expected = normalize_digest(pinned)?;
                if observed != expected {
                    return Err(format!(
                        "plugin '{canonical_plugin}' blob digest {observed} does not match pinned digest {expected}"
                    ));
                }
            }
            let platform_key = format!("{}-{}", std::env::consts::OS, std::env::consts::ARCH);
            if let Some(pinned) = constraint.blob_digests.get(&platform_key) {
                let expected = normalize_digest(pinned)?;
                if observed != expected {
                    return Err(format!(
                        "plugin '{canonical_plugin}' blob digest {observed} does not match pinned {platform_key} digest {expected}"
                    ));
                }
            }
        }
        Ok(())
    }

    pub fn summary(&self) -> ChannelManifestSummary {
        ChannelManifestSummary {
            schema: self.schema.clone(),
            seq: self.seq,
            admin: self.admin.clone(),
            allowed_plugins: self.allowed_plugins.clone(),
            signature_valid: self.verify().is_ok(),
        }
    }
}

/// Validate a channel identifier against path traversal.
pub fn validate_channel_id(raw: &str) -> Result<String, String> {
    let id = raw.trim();
    if id.is_empty() || id.len() > 64 {
        return Err("invalid channel_id: must be 1..=64 ASCII characters".to_string());
    }
    let bytes = id.as_bytes();
    if !bytes[0].is_ascii_alphanumeric() {
        return Err("invalid channel_id: must start with an ASCII letter or digit".to_string());
    }
    if !bytes
        .iter()
        .all(|b| b.is_ascii_alphanumeric() || *b == b'-' || *b == b'_' || *b == b'.')
    {
        return Err("invalid channel_id: only [A-Za-z0-9._-] allowed".to_string());
    }
    if id.contains("..") {
        return Err("invalid channel_id: path traversal not allowed".to_string());
    }
    Ok(id.to_string())
}

/// On-disk path for a channel's signed manifest:
/// `<state_dir>/channels/<channel_id>/manifest.json`.
pub fn channel_manifest_path(state_dir: &Path, channel_id: &str) -> PathBuf {
    channel_dir(state_dir, channel_id).join("manifest.json")
}

/// Optional dedicated admin key file (`0600`) under the channel directory:
/// `<state_dir>/channels/<channel_id>/admin.key`.
pub fn channel_admin_key_path(state_dir: &Path, channel_id: &str) -> PathBuf {
    channel_dir(state_dir, channel_id).join("admin.key")
}

/// Lightweight per-channel metadata file:
/// `<state_dir>/channels/<channel_id>/config.json`.
pub fn channel_meta_path(state_dir: &Path, channel_id: &str) -> PathBuf {
    channel_dir(state_dir, channel_id).join("config.json")
}

/// Load the local admin signing key for `channel_id`: uses `<channel_dir>/admin.key` if present,
/// otherwise falls back to `<state_dir>/iroh-secret-key`. Never logs or prints key material.
pub fn load_channel_admin_key(
    state_dir: &Path,
    channel_id: &str,
) -> Result<iroh::SecretKey, String> {
    let dedicated = channel_admin_key_path(state_dir, channel_id);
    if dedicated.is_file() {
        return load_or_create_secret_key_at(&dedicated);
    }
    load_or_create_secret_key_at(&state_dir.join("iroh-secret-key"))
}

/// Return true if this node holds the Ed25519 private key matching `manifest.admin`.
pub fn is_local_admin(
    state_dir: &Path,
    channel_id: &str,
    node_id: &str,
    manifest: &ChannelManifest,
) -> bool {
    if !node_id.is_empty() && manifest.admin.eq_ignore_ascii_case(node_id) {
        return true;
    }
    let dedicated = channel_admin_key_path(state_dir, channel_id);
    if dedicated.is_file() {
        if let Ok(key) = load_or_create_secret_key_at(&dedicated) {
            if manifest
                .admin
                .eq_ignore_ascii_case(&key.public().to_string())
            {
                return true;
            }
        }
    }
    false
}

/// Load a channel's manifest from disk. Returns `Ok(None)` only when the file does not exist.
pub fn load_channel_manifest(
    state_dir: &Path,
    channel_id: &str,
) -> Result<Option<ChannelManifest>, String> {
    let cid = validate_channel_id(channel_id)?;
    let path = channel_manifest_path(state_dir, &cid);
    let raw = match fs::read_to_string(&path) {
        Ok(s) => s,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(e) => return Err(format!("read channel manifest {}: {e}", path.display())),
    };
    let manifest: ChannelManifest = serde_json::from_str(&raw)
        .map_err(|e| format!("parse channel manifest {}: {e}", path.display()))?;
    Ok(Some(manifest))
}

/// Atomically persist a verified `ChannelManifest` to `<state_dir>/channels/<channel_id>/manifest.json`.
pub fn save_channel_manifest(
    state_dir: &Path,
    manifest: &ChannelManifest,
) -> Result<PathBuf, String> {
    manifest.verify()?;
    let path = channel_manifest_path(state_dir, &manifest.channel_id);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create channel dir {}: {e}", parent.display()))?;
    }
    let bytes = serde_json::to_vec_pretty(manifest)
        .map_err(|e| format!("serialize channel manifest: {e}"))?;
    let tmp = path.with_extension(format!("json.tmp.{}", std::process::id()));
    fs::write(&tmp, &bytes).map_err(|e| format!("write {}: {e}", tmp.display()))?;
    fs::rename(&tmp, &path).map_err(|e| {
        let _ = fs::remove_file(&tmp);
        format!("install channel manifest {}: {e}", path.display())
    })?;
    Ok(path)
}

/// Validate an incoming manifest update against the currently stored manifest (if any).
/// Accepts only valid signatures matching the recorded admin and with strictly greater `seq`.
pub fn validate_manifest_update(
    existing: Option<&ChannelManifest>,
    incoming: &ChannelManifest,
    expected_channel_id: &str,
) -> Result<(), String> {
    incoming.verify_for_channel(expected_channel_id)?;
    if let Some(cur) = existing {
        if !incoming.admin.eq_ignore_ascii_case(&cur.admin) {
            return Err(format!(
                "manifest update for '{}' rejected: admin '{}' does not match recorded admin '{}'",
                expected_channel_id, incoming.admin, cur.admin
            ));
        }
        if incoming.seq <= cur.seq {
            return Err(format!(
                "manifest update for '{}' rejected: stale seq {} <= current seq {}",
                expected_channel_id, incoming.seq, cur.seq
            ));
        }
    }
    Ok(())
}

/// Validate and apply an incoming manifest (from gossip, JoinAnswer, or local API).
/// Returns `Ok(true)` if the new manifest was persisted, `Ok(false)` if it was identical to current.
pub fn apply_incoming_manifest(
    state_dir: &Path,
    expected_channel_id: &str,
    incoming: &ChannelManifest,
) -> Result<bool, String> {
    let existing = load_channel_manifest(state_dir, expected_channel_id)?;
    if let Some(ref cur) = existing {
        if cur == incoming && cur.verify_for_channel(expected_channel_id).is_ok() {
            return Ok(false);
        }
    }
    validate_manifest_update(existing.as_ref(), incoming, expected_channel_id)?;
    save_channel_manifest(state_dir, incoming)?;
    Ok(true)
}

/// Persist lightweight metadata for a joined channel without overwriting an existing `joined_at_unix`.
pub fn ensure_channel_local_meta(
    state_dir: &Path,
    channel_id: &str,
    name: &str,
    joined_at_unix: u64,
) -> Result<ChannelLocalMeta, String> {
    let cid = validate_channel_id(channel_id)?;
    let path = channel_meta_path(state_dir, &cid);
    let mut meta = fs::read_to_string(&path)
        .ok()
        .and_then(|raw| serde_json::from_str::<ChannelLocalMeta>(&raw).ok())
        .unwrap_or(ChannelLocalMeta {
            channel_id: cid.clone(),
            name: name.to_string(),
            joined_at_unix,
        });
    meta.channel_id = cid;
    if !name.trim().is_empty() {
        meta.name = name.trim().to_string();
    }
    if meta.joined_at_unix == 0 {
        meta.joined_at_unix = joined_at_unix.max(now_unix());
    }
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create channel dir {}: {e}", parent.display()))?;
    }
    let bytes =
        serde_json::to_vec_pretty(&meta).map_err(|e| format!("serialize channel meta: {e}"))?;
    let tmp = path.with_extension(format!("json.tmp.{}", std::process::id()));
    fs::write(&tmp, &bytes).map_err(|e| format!("write {}: {e}", tmp.display()))?;
    fs::rename(&tmp, &path).map_err(|e| {
        let _ = fs::remove_file(&tmp);
        format!("save channel meta {}: {e}", path.display())
    })?;
    Ok(meta)
}

/// Read `ChannelLocalMeta` if present.
pub fn load_channel_local_meta(state_dir: &Path, channel_id: &str) -> Option<ChannelLocalMeta> {
    let cid = validate_channel_id(channel_id).ok()?;
    let path = channel_meta_path(state_dir, &cid);
    let raw = fs::read_to_string(path).ok()?;
    serde_json::from_str(&raw).ok()
}

/// Result of checking a plugin operation against the active channel's signed manifest.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum PluginPolicyDecision {
    AllowedByManifest {
        channel_id: String,
        seq: u64,
    },
    LegacyNoManifest {
        channel_id: Option<String>,
        warning: String,
    },
}

fn is_configured_channel_id(raw: Option<&str>) -> Option<String> {
    let cid = raw?.trim();
    if cid.is_empty()
        || cid.eq_ignore_ascii_case("unconfigured")
        || cid.ends_with(":locked-standby")
    {
        return None;
    }
    Some(cid.to_string())
}

/// Fail-closed policy check for plugin install, dispatch, daemon-start, and `dialtone/shell/1`.
///
/// - When the active channel has a manifest file: the manifest must parse, verify its Ed25519
///   signature for `channel_id`, list `plugin_name` in `allowed_plugins`, and satisfy any
///   configured `plugin_constraints`. Any failure returns `Err`.
/// - When the active channel has no manifest file (legacy channel) or no channel is active:
///   returns `Ok(PluginPolicyDecision::LegacyNoManifest { .. })` so legacy channels continue to
///   work while emitting a channel log warning.
pub fn check_plugin_allowed(
    state_dir: &Path,
    channel_id: Option<&str>,
    plugin_name: &str,
    plugin_version: Option<&str>,
    plugin_digest: Option<&str>,
) -> Result<PluginPolicyDecision, String> {
    let canonical = validate_plugin_name(plugin_name)?;
    let Some(cid) = is_configured_channel_id(channel_id) else {
        return Ok(PluginPolicyDecision::LegacyNoManifest {
            channel_id: None,
            warning: format!(
                "no active channel configured; allowing plugin '{canonical}' in legacy mode"
            ),
        });
    };

    let manifest = match load_channel_manifest(state_dir, &cid) {
        Ok(Some(m)) => m,
        Ok(None) => {
            return Ok(PluginPolicyDecision::LegacyNoManifest {
                channel_id: Some(cid.clone()),
                warning: format!(
                    "legacy channel '{cid}' has no signed manifest; allowing plugin '{canonical}' without manifest enforcement (initialize a manifest with 'dialtone channel manifest init')"
                ),
            });
        }
        Err(err) => {
            return Err(format!(
                "refusing plugin '{canonical}': channel '{cid}' manifest is invalid ({err})"
            ));
        }
    };

    if let Err(err) = manifest.verify_for_channel(&cid) {
        return Err(format!(
            "refusing plugin '{canonical}': channel '{cid}' manifest failed verification ({err})"
        ));
    }
    if !manifest.allows_plugin(&canonical) {
        return Err(format!(
            "refusing plugin '{canonical}': not allowed by channel '{cid}' manifest (seq={}, allowed=[{}])",
            manifest.seq,
            manifest.allowed_plugins.join(", ")
        ));
    }
    manifest.check_plugin_constraints(&canonical, plugin_version, plugin_digest)?;
    Ok(PluginPolicyDecision::AllowedByManifest {
        channel_id: cid,
        seq: manifest.seq,
    })
}

/// Append a warning entry to the channel log when a legacy channel without a manifest allows a plugin.
pub fn record_legacy_manifest_warning(
    state_dir: &Path,
    node_name: &str,
    node_key: Option<&iroh::SecretKey>,
    channel_log: Option<&Arc<Mutex<ChannelLogStore>>>,
    channel_log_tx: Option<&mpsc::UnboundedSender<ChannelLogEntry>>,
    decision: &PluginPolicyDecision,
) {
    let PluginPolicyDecision::LegacyNoManifest {
        channel_id: Some(ref cid),
        ref warning,
    } = decision
    else {
        return;
    };

    if let (Some(log_arc), Some(key)) = (channel_log, node_key) {
        if let Ok(mut store) = log_arc.lock() {
            if let Ok(entry) = store.append_local(
                key,
                node_name,
                "daemon",
                "manifest.legacy_warning",
                None,
                "warn",
                warning,
            ) {
                if let Some(tx) = channel_log_tx {
                    let _ = tx.send(entry);
                }
            }
        }
        return;
    }

    // Offline CLI fallback: sign and append directly to the channel log file if identity key exists.
    let key_path = state_dir.join("iroh-secret-key");
    let owned_key = match node_key {
        Some(k) => Some(k.clone()),
        None if key_path.is_file() => load_or_create_secret_key_at(&key_path).ok(),
        None => None,
    };
    if let Some(key) = owned_key {
        let log_path = channel_log_path(state_dir, cid);
        if let Ok(mut store) = ChannelLogStore::open(&log_path, cid.clone()) {
            let _ = store.append_local(
                &key,
                node_name,
                "daemon",
                "manifest.legacy_warning",
                None,
                "warn",
                warning,
            );
        }
    }
}

/// Live-updating policy guard shared with `ShellHandler`, API routes, and managed plugins.
#[derive(Clone, Debug)]
pub struct ChannelPolicyGuard {
    state_dir: PathBuf,
    active_channel_id: Arc<RwLock<Option<String>>>,
    node_name: Arc<RwLock<String>>,
    node_key: Option<iroh::SecretKey>,
    channel_log: Arc<RwLock<Option<Arc<Mutex<ChannelLogStore>>>>>,
    channel_log_tx: Arc<RwLock<Option<mpsc::UnboundedSender<ChannelLogEntry>>>>,
}

impl ChannelPolicyGuard {
    pub fn new(
        state_dir: PathBuf,
        active_channel_id: Option<String>,
        node_name: impl Into<String>,
        node_key: Option<iroh::SecretKey>,
    ) -> Self {
        Self {
            state_dir,
            active_channel_id: Arc::new(RwLock::new(active_channel_id)),
            node_name: Arc::new(RwLock::new(node_name.into())),
            node_key,
            channel_log: Arc::new(RwLock::new(None)),
            channel_log_tx: Arc::new(RwLock::new(None)),
        }
    }

    pub fn set_active_channel(
        &self,
        channel_id: Option<String>,
        channel_log: Option<Arc<Mutex<ChannelLogStore>>>,
    ) {
        if let Ok(mut guard) = self.active_channel_id.write() {
            *guard = channel_id;
        }
        if let Ok(mut log_guard) = self.channel_log.write() {
            *log_guard = channel_log;
        }
    }

    pub fn set_channel_log_tx(&self, tx: Option<mpsc::UnboundedSender<ChannelLogEntry>>) {
        if let Ok(mut guard) = self.channel_log_tx.write() {
            *guard = tx;
        }
    }

    pub fn set_node_name(&self, name: impl Into<String>) {
        if let Ok(mut guard) = self.node_name.write() {
            *guard = name.into();
        }
    }

    pub fn active_channel_id(&self) -> Option<String> {
        self.active_channel_id.read().ok().and_then(|g| g.clone())
    }

    pub fn check_and_log(
        &self,
        plugin_name: &str,
        version: Option<&str>,
        digest: Option<&str>,
    ) -> Result<PluginPolicyDecision, String> {
        let cid = self.active_channel_id();
        let decision = check_plugin_allowed(
            &self.state_dir,
            cid.as_deref(),
            plugin_name,
            version,
            digest,
        )?;
        if matches!(decision, PluginPolicyDecision::LegacyNoManifest { .. }) {
            let node_name = self
                .node_name
                .read()
                .map(|g| g.clone())
                .unwrap_or_else(|_| "node".to_string());
            let log_arc = self.channel_log.read().ok().and_then(|g| g.clone());
            let log_tx = self.channel_log_tx.read().ok().and_then(|g| g.clone());
            record_legacy_manifest_warning(
                &self.state_dir,
                &node_name,
                self.node_key.as_ref(),
                log_arc.as_ref(),
                log_tx.as_ref(),
                &decision,
            );
        }
        Ok(decision)
    }
}

/// Discover per-plugin local RPC address and data directory for `GET /api/channels`.
pub fn discover_plugin_local_info(
    state_dir: &Path,
    plugins_dir: &Path,
    channel_id: &str,
    plugin_name: &str,
    is_active: bool,
) -> serde_json::Value {
    let data_dir = channel_plugin_data_dir(state_dir, channel_id, plugin_name);
    let read_rpc_addr = |dir: &Path| -> Option<String> {
        let raw = fs::read_to_string(dir.join("config.json")).ok()?;
        let val: serde_json::Value = serde_json::from_str(&raw).ok()?;
        val.get("rpc_addr")
            .and_then(|v| v.as_str())
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
    };
    let mut rpc_addr = read_rpc_addr(&data_dir);
    if rpc_addr.is_none() && is_active {
        rpc_addr = read_rpc_addr(&plugins_dir.join(plugin_name));
    }
    if rpc_addr.is_none() && plugin_name == "task" && is_active {
        rpc_addr = Some("127.0.0.1:8794".to_string());
    }
    let mut obj = serde_json::Map::new();
    if let Some(addr) = rpc_addr {
        obj.insert("rpc_addr".to_string(), serde_json::Value::String(addr));
    }
    obj.insert(
        "data_dir".to_string(),
        serde_json::Value::String(data_dir.to_string_lossy().to_string()),
    );
    serde_json::Value::Object(obj)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicU64, Ordering};

    fn temp_state_dir(tag: &str) -> PathBuf {
        static NEXT: AtomicU64 = AtomicU64::new(1);
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let p = std::env::temp_dir().join(format!(
            "dialtone-manifest-test-{tag}-{}-{id}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&p);
        fs::create_dir_all(&p).unwrap();
        p
    }

    #[test]
    fn manifest_sign_verify_and_reject_wrong_admin_tampered_and_stale_seq() {
        let admin_key = iroh::SecretKey::generate();
        let other_key = iroh::SecretKey::generate();
        let cid = "ch-b39d38af727e";

        let m1 = ChannelManifest::new_signed(
            cid,
            "metal",
            &["task".into(), "shell".into(), "agent".into()],
            &admin_key,
        )
        .unwrap();
        assert_eq!(m1.schema, CHANNEL_MANIFEST_SCHEMA);
        assert_eq!(m1.seq, 1);
        assert_eq!(m1.allowed_plugins, vec!["agent", "shell", "task"]);
        assert!(m1.verify_for_channel(cid).is_ok());

        // Tampered field must fail signature verification.
        let mut tampered = m1.clone();
        tampered.allowed_plugins.push("evil".to_string());
        assert!(tampered.verify_for_channel(cid).is_err());

        // Mismatched channel_id must fail.
        assert!(m1.verify_for_channel("ch-000000000000").is_err());

        // Wrong admin signing a seq=2 update must be rejected against recorded admin.
        let mut forged = m1.clone();
        forged.admin = other_key.public().to_string();
        forged.seq = 2;
        forged.sign_in_place(&other_key).unwrap();
        assert!(forged.verify().is_ok());
        let err = validate_manifest_update(Some(&m1), &forged, cid).unwrap_err();
        assert!(err.contains("does not match recorded admin"), "{err}");

        // Stale seq (same or lower) from the real admin must be rejected.
        let mut stale = m1.clone();
        stale.issued_at_unix += 10;
        stale.sign_in_place(&admin_key).unwrap();
        let err = validate_manifest_update(Some(&m1), &stale, cid).unwrap_err();
        assert!(err.contains("stale seq"), "{err}");

        // Legitimate allow/deny bumps seq and passes update validation.
        let mut m2 = m1.clone();
        m2.deny_plugin("agent", &admin_key).unwrap();
        assert_eq!(m2.seq, 2);
        assert!(!m2.allows_plugin("agent"));
        assert!(m2.allows_plugin("task"));
        assert!(validate_manifest_update(Some(&m1), &m2, cid).is_ok());

        let mut m3 = m2.clone();
        m3.allow_plugin("agent", None, &admin_key).unwrap();
        assert_eq!(m3.seq, 3);
        assert!(m3.allows_plugin("agent"));
        assert!(validate_manifest_update(Some(&m2), &m3, cid).is_ok());

        // Trying to sign with non-admin key fails immediately.
        let mut m4 = m3.clone();
        assert!(m4.allow_plugin("extra", None, &other_key).is_err());
    }

    #[test]
    fn fail_closed_enforcement_and_legacy_no_manifest_warning() {
        let dir = temp_state_dir("enforce");
        let cid = "ch-112233445566";
        let admin_key = iroh::SecretKey::generate();

        // Legacy channel (no manifest.json): allowed with warning logged to channel log.
        let decision = check_plugin_allowed(&dir, Some(cid), "task", None, None).unwrap();
        assert!(matches!(
            decision,
            PluginPolicyDecision::LegacyNoManifest { .. }
        ));
        record_legacy_manifest_warning(&dir, "legion", Some(&admin_key), None, None, &decision);
        let log_store =
            ChannelLogStore::open(&channel_log_path(&dir, cid), cid.to_string()).unwrap();
        let entries = log_store.query(&crate::channel_log::ChannelLogQuery::default());
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].kind, "manifest.legacy_warning");
        assert_eq!(entries[0].level, "warn");

        // Once a signed manifest exists permitting only `task` and `shell`:
        let manifest =
            ChannelManifest::new_signed(cid, "metal", &["task".into(), "shell".into()], &admin_key)
                .unwrap();
        save_channel_manifest(&dir, &manifest).unwrap();

        assert!(matches!(
            check_plugin_allowed(&dir, Some(cid), "task", None, None).unwrap(),
            PluginPolicyDecision::AllowedByManifest { seq: 1, .. }
        ));
        assert!(matches!(
            check_plugin_allowed(&dir, Some(cid), "shell", None, None).unwrap(),
            PluginPolicyDecision::AllowedByManifest { seq: 1, .. }
        ));
        let denied = check_plugin_allowed(&dir, Some(cid), "agent", None, None).unwrap_err();
        assert!(denied.contains("not allowed"), "{denied}");

        // Tampered manifest on disk fails closed for every plugin!
        let mpath = channel_manifest_path(&dir, cid);
        let mut raw_val: serde_json::Value =
            serde_json::from_str(&fs::read_to_string(&mpath).unwrap()).unwrap();
        raw_val["allowed_plugins"] = serde_json::json!(["task", "shell", "agent"]);
        fs::write(&mpath, serde_json::to_vec_pretty(&raw_val).unwrap()).unwrap();
        let bad = check_plugin_allowed(&dir, Some(cid), "task", None, None).unwrap_err();
        assert!(bad.contains("failed verification"), "{bad}");

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn plugin_constraints_enforce_min_version_and_digest_pin() {
        let dir = temp_state_dir("constraints");
        let cid = "ch-aabbccddeeff";
        let admin_key = iroh::SecretKey::generate();
        let mut m = ChannelManifest::new_signed(cid, "ops", &["task".into()], &admin_key).unwrap();
        let good_digest = crate::blob::digest_of(b"good-plugin-binary");
        let bad_digest = crate::blob::digest_of(b"bad-plugin-binary");
        m.allow_plugin(
            "task",
            Some(PluginConstraint {
                min_version: Some("0.5.0".to_string()),
                blob_digest: Some(good_digest.clone()),
                blob_digests: BTreeMap::new(),
            }),
            &admin_key,
        )
        .unwrap();
        save_channel_manifest(&dir, &m).unwrap();

        assert!(
            check_plugin_allowed(&dir, Some(cid), "task", Some("0.5.0"), Some(&good_digest))
                .is_ok()
        );
        let ver_err =
            check_plugin_allowed(&dir, Some(cid), "task", Some("0.4.0"), Some(&good_digest))
                .unwrap_err();
        assert!(ver_err.contains("min_version"), "{ver_err}");
        let dig_err =
            check_plugin_allowed(&dir, Some(cid), "task", Some("0.5.0"), Some(&bad_digest))
                .unwrap_err();
        assert!(dig_err.contains("pinned digest"), "{dig_err}");

        let _ = fs::remove_dir_all(&dir);
    }
}
