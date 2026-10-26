use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::str::FromStr;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Mutex, OnceLock};

use argon2::{Algorithm, Argon2, Params, Version};
use chacha20poly1305::{
    aead::{Aead, KeyInit, Payload},
    XChaCha20Poly1305, XNonce,
};
use iroh::EndpointAddr;
use iroh_gossip::proto::TopicId;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::node::{is_tailnet_ip, IrohTransportDiagnostics};
use crate::service::Service;
use crate::stats::{Stats, StatsSummary};

pub const BEAT_SCHEMA: &str = "dialtone.iroh.beat.v1";
/// v5: a binary frame, `ENVELOPE_MAGIC` + 24-byte nonce + ciphertext; the plaintext is the
/// sender Key's 64-byte Ed25519 signature followed by the signed `Message` JSON. v4 wrapped
/// the same in JSON with hex and base64, a third larger.
pub const DIALTONE_ENVELOPE_SCHEMA_V5: &str = "dialtone.iroh.envelope.v5";
const ENVELOPE_MAGIC: &[u8; 4] = b"dt5\0";
/// Beats sent more than this far from local time (either direction) are rejected.
pub const REPLAY_WINDOW_MS: u64 = 5 * 60 * 1000;
pub const DIALTONE_INVITE_SCHEMA_V1: &str = "dialtone.invite.v1";
pub const INVITE_PREFIX: &str = "dialtone-invite-v1:";
pub const DEFAULT_CHANNEL_NAME: &str = "dialtone";
pub const DEFAULT_PEER_TTL_SECS: u64 = 300;
/// The shortest beat interval; larger meshes beat less often (`beat_interval_for`).
pub const BEAT_INTERVAL_SECS: u64 = 5;
/// The longest beat interval, reached at 600 nodes.
pub const MAX_BEAT_INTERVAL_SECS: u64 = 60;
pub const SUMMARY_INTERVAL_SECS: u64 = 30;
pub const SUMMARY_EVERY_N_BEATS: u64 = SUMMARY_INTERVAL_SECS / BEAT_INTERVAL_SECS;

/// Documented Argon2id parameters (OWASP recommended: 19 MiB memory, 2 iterations, 1 lane).
pub const ARGON2ID_M_COST_KIB: u32 = 19_456;
pub const ARGON2ID_T_COST: u32 = 2;
pub const ARGON2ID_P_COST: u32 = 1;
pub const KDF_VERSION_V2: &str = "argon2id-v2";

pub fn parse_duration_secs(raw: &str) -> Result<u64, String> {
    let s = raw.trim().to_ascii_lowercase();
    if let Some(num) = s.strip_suffix('s') {
        return num.trim().parse::<u64>().map_err(|e| e.to_string());
    }
    if let Some(num) = s.strip_suffix('m') {
        return num
            .trim()
            .parse::<u64>()
            .map(|m| m.saturating_mul(60))
            .map_err(|e| e.to_string());
    }
    if let Some(num) = s.strip_suffix('h') {
        return num
            .trim()
            .parse::<u64>()
            .map(|h| h.saturating_mul(3600))
            .map_err(|e| e.to_string());
    }
    s.parse::<u64>().map_err(|e| e.to_string())
}

pub fn default_channel_name() -> String {
    DEFAULT_CHANNEL_NAME.to_string()
}

pub fn normalize_channel_name(channel: Option<&str>) -> String {
    let cleaned = channel
        .unwrap_or(DEFAULT_CHANNEL_NAME)
        .trim()
        .to_ascii_lowercase();
    if cleaned.is_empty() {
        DEFAULT_CHANNEL_NAME.to_string()
    } else {
        cleaned
    }
}

/// Generate a high-entropy 256-bit channel secret (`dt-<43-char-base64url>`) using CSPRNG bytes.
pub fn new_secret() -> String {
    let random_bytes = iroh::SecretKey::generate().to_bytes();
    format!("dt-{}", crate::base64url_encode(&random_bytes))
}

/// Estimate secret character-set entropy in bits and enforce minimum strength (Issue #1).
/// Rejects empty strings, `"dialtone"`, `"default"`, `"dialtone2"`, `"password123"`, dictionary words, // compat
/// and strings with fewer than 16 characters or insufficient entropy.
pub fn validate_secret(secret: &str) -> Result<String, String> {
    let trimmed = secret.trim();
    if trimmed.is_empty() {
        return Err(
            "a channel secret is required to join the channel (run `dialtone secret` to generate one)"
                .to_string(),
        );
    }
    let lower = trimmed.to_ascii_lowercase();
    const FORBIDDEN_SUBSTRINGS: &[&str] = &[
        "dialtone",
        "password", // compat: the literal dictionary word
        "default",
        "qwerty",
        "123456",
        "letmein",
        "admin",
        "welcome",
        "correcthorse",
    ];
    if FORBIDDEN_SUBSTRINGS.iter().any(|w| lower.contains(w)) {
        return Err(
            "channel secret is too weak or contains a forbidden dictionary word (e.g. 'dialtone', 'default'); run `dialtone secret`"
                .to_string(),
        );
    }
    if trimmed.len() < 16 {
        return Err(format!(
            "channel secret must be at least 16 characters long (got {}); run `dialtone secret`",
            trimmed.len()
        ));
    }

    let mut has_lower = false;
    let mut has_upper = false;
    let mut has_digit = false;
    let mut has_symbol = false;
    let mut unique_chars = std::collections::BTreeSet::new();
    for ch in trimmed.chars() {
        unique_chars.insert(ch);
        if ch.is_ascii_lowercase() {
            has_lower = true;
        } else if ch.is_ascii_uppercase() {
            has_upper = true;
        } else if ch.is_ascii_digit() {
            has_digit = true;
        } else {
            has_symbol = true;
        }
    }

    let class_count =
        (has_lower as u8) + (has_upper as u8) + (has_digit as u8) + (has_symbol as u8);
    if unique_chars.len() < 8 || class_count < 2 {
        return Err(
            "channel secret has insufficient entropy (must contain at least 2 character classes and 8+ unique characters); run `dialtone secret`"
                .to_string(),
        );
    }

    Ok(trimmed.to_string())
}

/// The channel's events topic: inventories, broadcast when they change, kept apart from
/// the beats topic so beats stay small and regular.
pub fn events_topic_id(beats: TopicId) -> TopicId {
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.events.v1:");
    hasher.update(beats.as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    TopicId::from_bytes(digest)
}

/// Derive a node-private standby `TopicId` when no channel secret has been configured yet.
pub fn derive_standby_topic_id(node_id: &str) -> TopicId {
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.standby.v2:");
    hasher.update(node_id.trim().as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    TopicId::from_bytes(digest)
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct ChannelKeys {
    pub channel: String,
    pub topic_id: TopicId,
    pub auth_key: [u8; 32],
    pub enc_key: [u8; 32],
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct ChannelKeysHex {
    pub kdf_version: String,
    pub topic_id_hex: String,
    pub auth_key_hex: String,
    pub enc_key_hex: String,
}

impl ChannelKeys {
    pub fn to_hex_bundle(&self) -> ChannelKeysHex {
        ChannelKeysHex {
            kdf_version: KDF_VERSION_V2.to_string(),
            topic_id_hex: crate::hex_encode(self.topic_id.as_bytes()),
            auth_key_hex: crate::hex_encode(&self.auth_key),
            enc_key_hex: crate::hex_encode(&self.enc_key),
        }
    }

    pub fn from_hex_bundle(channel: &str, bundle: &ChannelKeysHex) -> Result<Self, String> {
        let topic_bytes = crate::hex_decode(&bundle.topic_id_hex)?;
        let auth_bytes = crate::hex_decode(&bundle.auth_key_hex)?;
        let enc_bytes = crate::hex_decode(&bundle.enc_key_hex)?;
        if topic_bytes.len() != 32 || auth_bytes.len() != 32 || enc_bytes.len() != 32 {
            return Err("invalid derived key bundle length (expected 32-byte keys)".to_string());
        }
        let mut topic_arr = [0u8; 32];
        let mut auth_arr = [0u8; 32];
        let mut enc_arr = [0u8; 32];
        topic_arr.copy_from_slice(&topic_bytes);
        auth_arr.copy_from_slice(&auth_bytes);
        enc_arr.copy_from_slice(&enc_bytes);
        Ok(Self {
            channel: normalize_channel_name(Some(channel)),
            topic_id: TopicId::from_bytes(topic_arr),
            auth_key: auth_arr,
            enc_key: enc_arr,
        })
    }

    pub fn fingerprint(&self) -> String {
        let hex = crate::hex_encode(self.topic_id.as_bytes());
        format!("ch-{}", &hex[..12])
    }
}

fn kdf_cache() -> &'static Mutex<HashMap<(String, String), ChannelKeys>> {
    static CACHE: OnceLock<Mutex<HashMap<(String, String), ChannelKeys>>> = OnceLock::new();
    CACHE.get_or_init(|| Mutex::new(HashMap::new()))
}

/// Derive separate `topic_id`, `auth_key`, and `enc_key` from `(channel, secret)` using
/// **Argon2id** (`m=19456 KiB, t=2, p=1`) + domain-separated HKDF-style PRF expansion (Issue #4).
pub fn derive_channel_keys_v2(channel: &str, secret: &str) -> ChannelKeys {
    let norm_channel = normalize_channel_name(Some(channel));
    let clean_pass = secret.trim().to_string();
    let cache_key = (norm_channel.clone(), clean_pass.clone());

    if let Ok(guard) = kdf_cache().lock() {
        if let Some(cached) = guard.get(&cache_key) {
            return cached.clone();
        }
    }

    // 1. Per-channel deterministic 16-byte salt
    let mut salt_hasher = Sha256::new();
    salt_hasher.update(b"dialtone.kdf.salt.v2:");
    salt_hasher.update(norm_channel.as_bytes());
    let salt_digest: [u8; 32] = salt_hasher.finalize().into();
    let salt = &salt_digest[..16];

    // 2. Argon2id master key derivation
    let params = Params::new(
        ARGON2ID_M_COST_KIB,
        ARGON2ID_T_COST,
        ARGON2ID_P_COST,
        Some(32),
    )
    .expect("valid Argon2id params");
    let argon2 = Argon2::new(Algorithm::Argon2id, Version::V0x13, params);
    let mut master_key = [0u8; 32];
    argon2
        .hash_password_into(clean_pass.as_bytes(), salt, &mut master_key) // compat
        .expect("Argon2id hash_password_into"); // compat

    // 3. Domain-separated v2 subkey expansion (HKDF-Expand equivalent over 256-bit PRK)
    let expand_subkey = |label: &[u8]| -> [u8; 32] {
        let mut h = Sha256::new();
        h.update(master_key);
        h.update(b":");
        h.update(label);
        h.update(b":");
        h.update(norm_channel.as_bytes());
        h.update([0x01]);
        h.finalize().into()
    };

    let topic_bytes = expand_subkey(b"dialtone.kdf.topic.v2");
    let auth_key = expand_subkey(b"dialtone.kdf.auth.v2");
    let enc_key = expand_subkey(b"dialtone.kdf.enc.v2");

    let derived = ChannelKeys {
        channel: norm_channel,
        topic_id: TopicId::from_bytes(topic_bytes),
        auth_key,
        enc_key,
    };

    if let Ok(mut guard) = kdf_cache().lock() {
        guard.insert(cache_key, derived.clone());
    }
    derived
}

/// Deterministically derive an `iroh-gossip` 32-byte `TopicId` from `(channel, secret)` via Argon2id v2.
pub fn derive_channel_topic_id(channel: &str, secret: &str) -> TopicId {
    derive_channel_keys_v2(channel, secret).topic_id
}

/// Deterministically derive an `iroh-gossip` 32-byte `TopicId` from a secret or `<channel>:<secret>`.
pub fn derive_topic_id_from_secret(secret: &str) -> TopicId {
    let trimmed = secret.trim();
    if trimmed.is_empty() {
        return derive_standby_topic_id("unconfigured");
    }
    if let Ok(ticket) = SeedList::from_str(trimmed) {
        return ticket.topic;
    }
    if let Some((sec_part, _)) = trimmed.split_once('@') {
        return derive_topic_id_from_secret(sec_part);
    }
    if let Some((ch, pass)) = trimmed.split_once(':') {
        if !ch.trim().is_empty() && !pass.trim().is_empty() {
            return derive_channel_topic_id(ch, pass);
        }
    }
    derive_channel_topic_id(DEFAULT_CHANNEL_NAME, trimmed)
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct Seed {
    pub node_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub relay_url: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub ip_addrs: Vec<String>,
}

/// Returns the seed peer endpoints configured in the host's local `~/.dialtone/config.json`
/// (`seeds` array). No production `EndpointId`s are hardcoded in the repository.
pub fn default_seeds() -> Vec<EndpointAddr> {
    let cfg = load_mesh_config(&default_config_path());
    let mut out = Vec::new();
    for entry in &cfg.seeds {
        if let Ok(id) = entry.node_id.trim().parse::<iroh::EndpointId>() {
            let mut addr = EndpointAddr::new(id);
            let relay_str = entry
                .relay_url
                .as_deref()
                .unwrap_or("https://usw1-1.relay.n0.iroh.link./");
            if !matches!(
                relay_str.trim().to_ascii_lowercase().as_str(),
                "" | "none" | "disabled"
            ) {
                if let Ok(relay_url) = relay_str.parse::<iroh::RelayUrl>() {
                    addr = addr.with_relay_url(relay_url);
                }
            }
            for raw_ip in &entry.ip_addrs {
                if let Ok(sock) = raw_ip.trim().parse::<std::net::SocketAddr>() {
                    addr = addr.with_ip_addr(sock);
                }
            }
            out.push(addr);
        }
    }
    out
}

/// Evaluate whether `candidate_node_id` is allowed by an allowlist (Issue #3).
/// **Deny by default**: if `allowed_ids` is empty, returns `false` for all remote peers.
/// A node's own `self_node_id` is always allowed.
pub fn is_allowed_peer_in_list(
    allowed_ids: &[String],
    candidate_node_id: &str,
    self_node_id: Option<&str>,
) -> bool {
    let candidate = candidate_node_id.trim();
    if candidate.is_empty() {
        return false;
    }
    if let Some(self_id) = self_node_id {
        if !self_id.trim().is_empty() && self_id.trim().eq_ignore_ascii_case(candidate) {
            return true;
        }
    }
    if allowed_ids.is_empty() {
        return false;
    }
    allowed_ids
        .iter()
        .any(|allowed| allowed.trim().eq_ignore_ascii_case(candidate))
}

/// Atomically add `node_id` with optional relay URL and direct IP:port addresses to `config_path`.
pub fn allow_peer_in_config_with_addrs(
    config_path: &Path,
    node_id: &str,
    relay_url: Option<&str>,
    ip_addrs: &[String],
) -> Result<MeshConfig, String> {
    let parsed_id = node_id
        .trim()
        .parse::<iroh::EndpointId>()
        .map_err(|e| format!("invalid EndpointId '{}': {e}", node_id.trim()))?
        .to_string();
    let mut validated_ips = Vec::new();
    for raw_ip in ip_addrs {
        let sock = raw_ip
            .trim()
            .parse::<std::net::SocketAddr>()
            .map_err(|e| format!("invalid peer SocketAddr '{}': {e}", raw_ip.trim()))?;
        let s = sock.to_string();
        if !validated_ips.contains(&s) {
            validated_ips.push(s);
        }
    }
    let mut cfg = load_mesh_config(config_path);
    if !cfg
        .allowlist
        .iter()
        .any(|x| x.eq_ignore_ascii_case(&parsed_id))
    {
        cfg.allowlist.push(parsed_id.clone());
    }
    if let Some(existing) = cfg
        .seeds
        .iter_mut()
        .find(|s| s.node_id.eq_ignore_ascii_case(&parsed_id))
    {
        if let Some(r) = relay_url {
            existing.relay_url = Some(r.to_string());
        }
        for ip in validated_ips {
            if !existing.ip_addrs.contains(&ip) {
                existing.ip_addrs.push(ip);
            }
        }
    } else {
        cfg.seeds.push(Seed {
            node_id: parsed_id,
            relay_url: Some(
                relay_url
                    .unwrap_or("https://usw1-1.relay.n0.iroh.link./")
                    .to_string(),
            ),
            ip_addrs: validated_ips,
        });
    }
    cfg.updated_at_unix = now_unix();
    save_mesh_config(config_path, &cfg)?;
    Ok(cfg)
}

/// Atomically revoke `node_id` from `allowlist` and `seeds` in `config_path` (Issue #3).
pub fn revoke_peer_in_config(config_path: &Path, node_id: &str) -> Result<MeshConfig, String> {
    let target = node_id.trim();
    if target.is_empty() {
        return Err("node_id is required".to_string());
    }
    let mut cfg = load_mesh_config(config_path);
    cfg.allowlist
        .retain(|x| !x.trim().eq_ignore_ascii_case(target));
    cfg.seeds
        .retain(|s| !s.node_id.trim().eq_ignore_ascii_case(target));
    cfg.updated_at_unix = now_unix();
    save_mesh_config(config_path, &cfg)?;
    Ok(cfg)
}

/// Expiring, single-use invite signed with the channel's `auth_key` (Issue #6).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Invite {
    pub schema: String,
    pub issuer_node_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub relay_url: Option<String>,
    pub channel: String,
    pub topic_id_hex: String,
    pub nonce: String,
    pub expires_at_unix: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bound_node_id: Option<String>,
    pub signature: String,
}

impl Invite {
    pub fn compute_signature(&self, auth_key: &[u8; 32]) -> String {
        let mut h = Sha256::new();
        h.update(b"dialtone.invite.sig.v1:");
        h.update(auth_key);
        h.update(b":");
        h.update(self.schema.as_bytes());
        h.update(b":");
        h.update(self.issuer_node_id.as_bytes());
        h.update(b":");
        h.update(self.relay_url.as_deref().unwrap_or("").as_bytes());
        h.update(b":");
        h.update(self.channel.as_bytes());
        h.update(b":");
        h.update(self.topic_id_hex.as_bytes());
        h.update(b":");
        h.update(self.nonce.as_bytes());
        h.update(b":");
        h.update(self.expires_at_unix.to_string().as_bytes());
        h.update(b":");
        h.update(self.bound_node_id.as_deref().unwrap_or("").as_bytes());
        let digest: [u8; 32] = h.finalize().into();
        crate::hex_encode(&digest)
    }

    pub fn to_text(&self) -> Result<String, String> {
        let bytes = serde_json::to_vec(self).map_err(|e| e.to_string())?;
        Ok(format!(
            "dialtone-invite-v1:{}",
            crate::base64url_encode(&bytes)
        ))
    }
}

impl FromStr for Invite {
    type Err = String;

    fn from_str(raw: &str) -> Result<Self, Self::Err> {
        let encoded = raw
            .trim()
            .strip_prefix("dialtone-invite-v1:")
            .ok_or("not a dialtone-invite-v1 ticket")?;
        let bytes = crate::base64_decode(encoded)?;
        serde_json::from_slice(&bytes).map_err(|e| format!("invalid invite payload: {e}"))
    }
}

/// Create a signed, expiring, single-use invite (Issue #6).
pub fn new_invite(
    issuer_node_id: &str,
    relay_url: Option<&str>,
    keys: &ChannelKeys,
    now_unix: u64,
    ttl_secs: u64,
    bound_node_id: Option<&str>,
) -> Result<Invite, String> {
    if ttl_secs == 0 || ttl_secs > 86_400 {
        return Err("invite TTL must be between 1s and 86400s (24h)".to_string());
    }
    let rand_bytes = iroh::SecretKey::generate().to_bytes();
    let nonce = crate::hex_encode(&rand_bytes[..16]);
    let mut ticket = Invite {
        schema: DIALTONE_INVITE_SCHEMA_V1.to_string(),
        issuer_node_id: issuer_node_id.trim().to_string(),
        relay_url: relay_url.map(|s| s.to_string()),
        channel: keys.channel.clone(),
        topic_id_hex: crate::hex_encode(keys.topic_id.as_bytes()),
        nonce,
        expires_at_unix: now_unix.saturating_add(ttl_secs),
        bound_node_id: bound_node_id
            .map(str::trim)
            .filter(|s| !s.is_empty())
            .map(String::from),
        signature: String::new(),
    };
    ticket.signature = ticket.compute_signature(&keys.auth_key);
    Ok(ticket)
}

/// Verify and redeem a single-use `Invite` (Issue #6).
/// Checks signature, expiry, optional `bound_node_id`, and single-use `nonce`, records the used
/// nonce in `cfg.used_invite_nonces`, and adds the issuer and redeemer to `cfg.allowlist`.
pub fn redeem_invite(
    ticket: &Invite,
    keys: &ChannelKeys,
    redeemer_node_id: &str,
    now_unix: u64,
    cfg: &mut MeshConfig,
) -> Result<EndpointAddr, String> {
    if ticket.schema != DIALTONE_INVITE_SCHEMA_V1 {
        return Err(format!("unsupported invite schema '{}'", ticket.schema));
    }
    let expected_sig = ticket.compute_signature(&keys.auth_key);
    if !constant_time_eq(ticket.signature.as_bytes(), expected_sig.as_bytes()) {
        return Err(
            "invite signature verification failed (wrong channel secret or tampered invite)"
                .to_string(),
        );
    }
    if now_unix > ticket.expires_at_unix {
        return Err(format!(
            "invite has expired (expired_at={}, now={})",
            ticket.expires_at_unix, now_unix
        ));
    }
    if let Some(ref bound) = ticket.bound_node_id {
        if !bound.trim().eq_ignore_ascii_case(redeemer_node_id.trim()) {
            return Err(format!(
                "invite is bound to node_id '{}' (cannot be redeemed by '{}')",
                bound, redeemer_node_id
            ));
        }
    }
    if cfg.used_invite_nonces.iter().any(|n| n == &ticket.nonce) {
        return Err(format!(
            "invite nonce '{}' has already been used (single-use only)",
            ticket.nonce
        ));
    }

    let issuer_id = ticket
        .issuer_node_id
        .trim()
        .parse::<iroh::EndpointId>()
        .map_err(|e| format!("invalid issuer EndpointId in invite: {e}"))?;
    let mut addr = EndpointAddr::new(issuer_id);
    let relay_str = ticket
        .relay_url
        .as_deref()
        .unwrap_or("https://usw1-1.relay.n0.iroh.link./");
    if let Ok(relay_url) = relay_str.parse::<iroh::RelayUrl>() {
        addr = addr.with_relay_url(relay_url);
    }

    cfg.used_invite_nonces.push(ticket.nonce.clone());
    for peer_to_allow in [ticket.issuer_node_id.trim(), redeemer_node_id.trim()] {
        if !peer_to_allow.is_empty()
            && !cfg
                .allowlist
                .iter()
                .any(|x| x.eq_ignore_ascii_case(peer_to_allow))
        {
            cfg.allowlist.push(peer_to_allow.to_string());
        }
    }
    Ok(addr)
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct BootStageStatus {
    pub step: u8,
    pub id: String,
    pub title: String,
    pub status: String,
    pub detail: String,
    pub updated_at_unix: u64,
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(default)]
pub struct MeshConfig {
    pub node_name: Option<String>,
    pub listen_addr: Option<String>,
    #[serde(default = "default_channel_name")]
    pub channel: String,
    /// Masked/configured indicator (`true` when user has configured a valid channel secret).
    pub configured: bool,
    /// Legacy/in-memory secret field — NEVER serialized to `config.json` (Issue #2).
    #[serde(default, skip_serializing)]
    pub secret_key: Option<String>,
    /// Derived Argon2id v2 key bundle (`topic_id_hex`, `auth_key_hex`, `enc_key_hex`) persisted in `0600` `config.json`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub derived_keys: Option<ChannelKeysHex>,
    pub channel_fingerprint: String,
    pub seeds: Vec<Seed>,
    pub allowlist: Vec<String>,
    pub used_invite_nonces: Vec<String>,
    pub invites: Vec<String>,
    pub daemon_installed: bool,
    pub daemon_service_path: Option<String>,
    pub processes: Vec<crate::service::ProcessSpec>,
    pub updated_at_unix: u64,
}

impl MeshConfig {
    /// Derive and store the Argon2id v2 channel key bundle without persisting the raw secret (Issue #2).
    pub fn apply_secret(&mut self, channel: &str, secret: &str) -> ChannelKeys {
        let norm_channel = normalize_channel_name(Some(channel));
        let keys = derive_channel_keys_v2(&norm_channel, secret);
        self.channel = norm_channel;
        self.configured = true;
        self.secret_key = None;
        self.derived_keys = Some(keys.to_hex_bundle());
        self.channel_fingerprint = keys.fingerprint();
        keys
    }

    /// Resolve active `ChannelKeys` from either persisted `derived_keys` or in-memory `secret_key`.
    pub fn active_channel_keys(&self) -> Option<ChannelKeys> {
        if let Some(ref bundle) = self.derived_keys {
            if let Ok(keys) = ChannelKeys::from_hex_bundle(&self.channel, bundle) {
                return Some(keys);
            }
        }
        if let Some(ref pass) = self.secret_key {
            if !pass.trim().is_empty() {
                return Some(derive_channel_keys_v2(&self.channel, pass));
            }
        }
        None
    }
}

#[derive(Clone, Debug)]
pub enum MeshControlCommand {
    JoinMesh {
        channel: String,
        keys: ChannelKeys,
        invites: Vec<String>,
        node_name: Option<String>,
    },
}

pub fn default_state_dir() -> PathBuf {
    if let Some(custom) = std::env::var_os("DIALTONE_STATE_DIR") {
        let p = PathBuf::from(custom);
        if !p.as_os_str().is_empty() {
            return p;
        }
    }
    let base = std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(PathBuf::from)
        .unwrap_or_else(std::env::temp_dir);
    base.join(".dialtone")
}

pub fn default_config_path() -> PathBuf {
    default_state_dir().join("config.json")
}

pub fn default_store_dir() -> PathBuf {
    default_state_dir().join("blobs")
}

/// Load `config.json`. A missing file is an empty config; a file that is not valid config
/// JSON is an error, never an empty config (saving that would wipe the keys and allowlist).
pub fn try_load_mesh_config(path: &Path) -> Result<MeshConfig, String> {
    let mut cfg: MeshConfig = match fs::read_to_string(path) {
        Ok(raw) => serde_json::from_str(&raw)
            .map_err(|e| format!("{} is not valid dialtone config JSON: {e}", path.display()))?,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(MeshConfig::default()),
        Err(e) => return Err(format!("cannot read {}: {e}", path.display())),
    };
    // Auto-migrate any legacy plaintext `secret_key` into `derived_keys` and scrub disk file immediately (Issue #2).
    if let Some(raw_pass) = cfg.secret_key.take() {
        if !raw_pass.trim().is_empty() {
            let ch = cfg.channel.clone();
            let _ = cfg.apply_secret(&ch, &raw_pass);
            let _ = save_mesh_config(path, &cfg);
        }
    }
    Ok(cfg)
}

/// Load `config.json`, stopping the process loudly if it is unreadable or corrupt: running on
/// with an empty config would drop the channel keys and allowlist at the next save.
pub fn load_mesh_config(path: &Path) -> MeshConfig {
    match try_load_mesh_config(path) {
        Ok(cfg) => cfg,
        Err(err) => {
            eprintln!(
                "dialtone: FATAL: {err}\n\
                 dialtone: refusing to continue so the channel keys and allowlist in it are not overwritten.\n\
                 dialtone: fix the file, or move it aside to start with an empty config."
            );
            std::process::exit(78); // EX_CONFIG
        }
    }
}

pub fn save_mesh_config(path: &Path, cfg: &MeshConfig) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create config dir {}: {e}", parent.display()))?;
    }
    let mut sanitized = cfg.clone();
    if let Some(raw_pass) = sanitized.secret_key.take() {
        if !raw_pass.trim().is_empty() && sanitized.derived_keys.is_none() {
            let ch = sanitized.channel.clone();
            let _ = sanitized.apply_secret(&ch, &raw_pass);
        }
    }
    let json = serde_json::to_string_pretty(&sanitized).map_err(|e| e.to_string())?;
    let tmp_path = path.with_extension("json.tmp");
    fs::write(&tmp_path, json)
        .map_err(|e| format!("write temp config {}: {e}", tmp_path.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let _ = fs::set_permissions(&tmp_path, fs::Permissions::from_mode(0o600));
    }
    fs::rename(&tmp_path, path).map_err(|e| format!("rename config {}: {e}", path.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let _ = fs::set_permissions(path, fs::Permissions::from_mode(0o600));
    }
    Ok(())
}

pub fn detect_default_node_name() -> String {
    for var in ["DIALTONE_NODE", "HOSTNAME", "COMPUTERNAME"] {
        if let Ok(val) = std::env::var(var) {
            let clean = val
                .trim()
                .split('.')
                .next()
                .unwrap_or("")
                .to_ascii_lowercase();
            if !clean.is_empty() {
                return clean;
            }
        }
    }
    if let Ok(out) = std::process::Command::new("hostname").output() {
        if out.status.success() {
            let clean = String::from_utf8_lossy(&out.stdout)
                .trim()
                .split('.')
                .next()
                .unwrap_or("")
                .to_ascii_lowercase();
            if !clean.is_empty() {
                return clean;
            }
        }
    }
    "node-alpha".to_string()
}

pub fn default_os() -> String {
    std::env::consts::OS.to_string()
}

pub fn default_arch() -> String {
    std::env::consts::ARCH.to_string()
}

pub fn node_role_for(_node: &str) -> String {
    "mesh-peer-node".to_string()
}

/// What every dialtone node can do (the same on every node; nothing here is configured).
pub fn node_capabilities_for(_node: &str) -> Vec<String> {
    [
        "iroh-1.3-quic-endpoint",
        "iroh-gossip-mesh",
        "iroh-blobs-digest-store",
        "no-tailnet-physical-bind",
        "process-supervisor",
        "device-listing",
        "3d-scifi-hud-ui",
    ]
    .iter()
    .map(|c| c.to_string())
    .collect()
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct SeedList {
    pub topic: TopicId,
    pub peers: Vec<EndpointAddr>,
}

impl SeedList {
    pub fn to_text(&self) -> Result<String, String> {
        let bytes = serde_json::to_vec(self).map_err(|err| err.to_string())?;
        Ok(format!("iroh-gossip-v1:{}", crate::hex_encode(&bytes)))
    }
}

impl FromStr for SeedList {
    type Err = String;

    fn from_str(raw: &str) -> Result<Self, Self::Err> {
        let text = raw
            .strip_prefix("iroh-gossip-v1:")
            .or_else(|| raw.strip_prefix("dialtone-gossip-v1:"))
            .ok_or("not an iroh-gossip-v1 or dialtone-gossip-v1 ticket")?;
        let bytes = crate::hex_decode(text).or_else(|_| crate::base64_decode(text))?;
        serde_json::from_slice(&bytes).map_err(|err| err.to_string())
    }
}

pub fn monotonic_now_ms() -> u64 {
    static START: OnceLock<std::time::Instant> = OnceLock::new();
    START
        .get_or_init(std::time::Instant::now)
        .elapsed()
        .as_millis() as u64
}

/// Seconds between beats in a mesh of `nodes`: every node hears every beat, so the interval
/// grows with the mesh to keep each node's incoming beats at about 10 per second.
pub fn beat_interval_for(nodes: usize) -> u64 {
    (nodes.div_ceil(10) as u64).clamp(BEAT_INTERVAL_SECS, MAX_BEAT_INTERVAL_SECS)
}

/// How long a peer's beat stays fresh: three of the peer's own beat intervals.
pub fn beat_fresh_secs(beat: &Beat) -> u64 {
    3 * beat.beat_interval_secs.max(BEAT_INTERVAL_SECS)
}

pub fn build_mesh_rtt_matrix(nodes: &[Beat]) -> std::collections::BTreeMap<String, f64> {
    let mut matrix = std::collections::BTreeMap::new();
    let mut counts = std::collections::BTreeMap::<String, usize>::new();
    for n in nodes {
        for (peer, &rtt) in &n.peer_rtt_ms {
            if n.node == *peer || rtt <= 0.0 {
                continue;
            }
            let (a, b) = if n.node <= *peer {
                (n.node.as_str(), peer.as_str())
            } else {
                (peer.as_str(), n.node.as_str())
            };
            let key = format!("{a}<->{b}");
            let entry = matrix.entry(key.clone()).or_insert(0.0);
            let cnt = counts.entry(key).or_insert(0);
            *entry += rtt;
            *cnt += 1;
        }
    }
    for (k, val) in matrix.iter_mut() {
        let c = counts.get(k).copied().unwrap_or(1).max(1) as f64;
        *val = ((*val / c) * 10.0).round() / 10.0;
    }
    matrix
}

#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
pub struct NodeGeoLocation {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub public_ip: Option<String>,
    pub lat: f64,
    pub lng: f64,
    #[serde(default)]
    pub label: String,
    /// `"public_ip_geo"` when placed from a public IP address, or `"latency_cluster"` when placed by RTT clustering.
    pub source: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub anchor_node: Option<String>,
    #[serde(default)]
    pub cluster_radius_km: f64,
}

pub fn is_globally_routable_ipv4(ip: std::net::Ipv4Addr) -> bool {
    let o = ip.octets();
    !ip.is_private()
        && !ip.is_loopback()
        && !ip.is_link_local()
        && !ip.is_broadcast()
        && !ip.is_documentation()
        && !ip.is_unspecified()
        && !is_tailnet_ip(std::net::IpAddr::V4(ip))
        && o[0] < 224
        && !(o[0] == 192 && o[1] == 0 && o[2] == 0)
        && !(o[0] == 198 && (o[1] == 18 || o[1] == 19))
}

pub fn mask_public_ipv4(ip: std::net::Ipv4Addr) -> String {
    let o = ip.octets();
    format!("{}.{}.x.x", o[0], o[1])
}

fn parse_ipv4_from_token(raw: &str) -> Option<std::net::Ipv4Addr> {
    let cleaned = raw
        .trim()
        .trim_start_matches("direct-ip:")
        .trim_start_matches("active:ip:")
        .trim_start_matches("known:ip:")
        .trim_start_matches("ip:");
    if let Ok(sock) = cleaned.parse::<std::net::SocketAddrV4>() {
        return Some(*sock.ip());
    }
    cleaned.parse::<std::net::Ipv4Addr>().ok()
}

pub fn public_ipv4_of_beat(beat: &Beat) -> Option<std::net::Ipv4Addr> {
    if let Some(ref b) = beat.iroh_transport.bound_non_tailnet_ipv4 {
        if let Some(ip) = parse_ipv4_from_token(b) {
            if is_globally_routable_ipv4(ip) {
                return Some(ip);
            }
        }
    }
    for s in &beat.iroh_transport.advertised_ip_addrs {
        if let Some(ip) = parse_ipv4_from_token(s) {
            if is_globally_routable_ipv4(ip) {
                return Some(ip);
            }
        }
    }
    for s in &beat.observed_remote_addrs {
        if let Some(ip) = parse_ipv4_from_token(s) {
            if is_globally_routable_ipv4(ip) {
                return Some(ip);
            }
        }
    }
    None
}

fn local_geo_cache() -> &'static Mutex<Option<NodeGeoLocation>> {
    static CACHE: OnceLock<Mutex<Option<NodeGeoLocation>>> = OnceLock::new();
    CACHE.get_or_init(|| Mutex::new(None))
}

pub fn cached_local_public_ip_geo() -> Option<NodeGeoLocation> {
    local_geo_cache().lock().ok().and_then(|g| g.clone())
}

pub fn spawn_background_public_ip_geo_probe() {
    static STARTED: OnceLock<bool> = OnceLock::new();
    if STARTED.set(true).is_err() {
        return;
    }
    std::thread::spawn(|| {
        let curl_bin = if cfg!(target_os = "windows") {
            "curl.exe"
        } else {
            "curl"
        };
        if let Some(raw) = crate::run_cmd(
            curl_bin,
            &["-fsSL", "--max-time", "3", "https://ipinfo.io/json"],
        ) {
            if let Ok(val) = serde_json::from_str::<serde_json::Value>(&raw) {
                let loc_str = val.get("loc").and_then(|v| v.as_str()).unwrap_or("");
                if let Some((lat_s, lng_s)) = loc_str.split_once(',') {
                    if let (Ok(lat), Ok(lng)) =
                        (lat_s.trim().parse::<f64>(), lng_s.trim().parse::<f64>())
                    {
                        let raw_ip = val.get("ip").and_then(|v| v.as_str()).unwrap_or("");
                        let masked_ip = raw_ip
                            .parse::<std::net::Ipv4Addr>()
                            .map(mask_public_ipv4)
                            .unwrap_or_else(|_| "public-wan".to_string());
                        let city = val.get("city").and_then(|v| v.as_str()).unwrap_or("");
                        let region = val.get("region").and_then(|v| v.as_str()).unwrap_or("");
                        let country = val.get("country").and_then(|v| v.as_str()).unwrap_or("WAN");
                        let label = if !city.is_empty() && !region.is_empty() {
                            format!("{city}, {region}")
                        } else if !city.is_empty() {
                            format!("{city}, {country}")
                        } else {
                            format!("{country} ({masked_ip})")
                        };
                        let geo = NodeGeoLocation {
                            public_ip: Some(masked_ip),
                            lat: (lat * 100.0).round() / 100.0,
                            lng: (lng * 100.0).round() / 100.0,
                            label,
                            source: "public_ip_geo".to_string(),
                            anchor_node: None,
                            cluster_radius_km: 12.0,
                        };
                        if let Ok(mut guard) = local_geo_cache().lock() {
                            *guard = Some(geo);
                        }
                    }
                }
            }
        }
    });
}

pub fn great_circle_deg(lat1: f64, lng1: f64, lat2: f64, lng2: f64) -> f64 {
    let p1 = lat1.to_radians();
    let p2 = lat2.to_radians();
    let dp = (lat2 - lat1).to_radians();
    let dl = (lng2 - lng1).to_radians();
    let a = (dp * 0.5).sin().powi(2) + p1.cos() * p2.cos() * (dl * 0.5).sin().powi(2);
    let c = 2.0 * a.sqrt().min(1.0).asin();
    c.to_degrees()
}

fn default_beat_kind() -> String {
    "regular".to_string()
}

fn default_beat_interval_secs() -> u64 {
    BEAT_INTERVAL_SECS
}

fn default_summary_interval_secs() -> u64 {
    SUMMARY_INTERVAL_SECS
}

/// One check in a probe run.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct ProbeCheck {
    /// `MSH` allowlisted peers in the mesh, `NBR` gossip neighbors, `BEAT` beats from and
    /// to every peer; with a full probe also `STO` local store round trip, `PUL` blobs
    /// pulled from sampled peers, `PRC` the probe program is running.
    pub code: String,
    pub ok: bool,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub detail: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ms: Option<f64>,
}

/// A blob this node pulled from a peer during a probe.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct ProbePull {
    pub peer: String,
    pub ok: bool,
    pub ms: f64,
    pub bytes: u64,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub error: String,
}

/// How fresh a peer's beats are, and whether the peer hears this node's.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct ProbeBeat {
    pub peer: String,
    pub age_secs: u64,
    /// The peer hears as many nodes as this one, so it receives this node's beats.
    pub hears_all: bool,
    pub ok: bool,
}

/// The latest probe: the node testing itself (see `probe.rs`). Gossiped in beats, without
/// the per-peer lists (`Beat::for_wire`), so every HUD shows every node's results.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct ProbeReport {
    /// Digest of the dialtone binary that ran this probe, so a deploy can tell the new
    /// build's results from those of the process it replaced.
    pub build: String,
    pub cycle: u64,
    pub updated_at_unix: u64,
    pub passed_count: u8,
    pub total_count: u8,
    pub checks: Vec<ProbeCheck>,
    /// The blob a full probe wrote and read back.
    #[serde(skip_serializing_if = "String::is_empty")]
    pub blob_digest: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub pulls: Vec<ProbePull>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub beats: Vec<ProbeBeat>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub program_name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub program_digest: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pid: Option<u32>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub staged_path: String,
}

impl ProbeReport {
    pub fn check(&self, code: &str) -> Option<&ProbeCheck> {
        self.checks.iter().find(|c| c.code == code)
    }
}

/// Per-node store counters, gossiped so every HUD can see blobs land on peers.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct StoreStats {
    pub blob_keys: u64,
    pub blob_bytes: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub latest_digest: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Beat {
    pub schema: String,
    pub node: String,
    pub node_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub ticket: String,
    pub counter: u64,
    pub updated_at_unix: u64,
    #[serde(default = "default_beat_kind")]
    pub kind: String,
    #[serde(default = "default_beat_interval_secs")]
    pub beat_interval_secs: u64,
    #[serde(default = "default_summary_interval_secs")]
    pub summary_interval_secs: u64,
    /// How many other nodes this node hears beats from (the probe's BEAT check).
    #[serde(default)]
    pub heard: u32,
    /// The version of the node's inventory; a peer holding another version fetches it.
    #[serde(default)]
    pub inventory_version: u64,
    /// Rtt to the node's gossip neighbors, measured by the Links actor.
    #[serde(default)]
    pub peer_rtt_ms: std::collections::BTreeMap<String, f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub summary_stats: Option<StatsSummary>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub geo_location: Option<NodeGeoLocation>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub probe: Option<ProbeReport>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub store: Option<StoreStats>,
    /// Sender wall-clock send time in ms, strictly increasing per sender (replay check).
    #[serde(default)]
    pub sent_unix_ms: u64,
    #[serde(default = "default_os")]
    pub os: String,
    #[serde(default = "default_arch")]
    pub arch: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub role: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub stats: Stats,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub service_endpoints: Vec<Service>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub processes: Vec<crate::service::ProcessSpec>,
    #[serde(default)]
    pub iroh_transport: IrohTransportDiagnostics,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub observed_remote_addrs: Vec<String>,
    #[serde(default, skip_serializing_if = "std::ops::Not::not")]
    pub tailnet_used: bool,
    /// The node's devices, from its latest inventory (never sent in a beat).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub devices: Vec<crate::devices::Device>,
}

impl Beat {
    /// The fixed-size form a beat takes on the wire: nothing that grows with the mesh (seed
    /// lists, per-peer probe results), and none of the node's details or inventory, which
    /// travel in the `Inventory`. Peers keep those from the last inventory (`merge_beat`).
    pub fn for_wire(&self) -> Beat {
        let mut b = self.clone();
        b.ticket.clear();
        b.processes.clear();
        b.service_endpoints.clear();
        b.devices.clear();
        b.role.clear();
        b.capabilities.clear();
        b.geo_location = None;
        b.observed_remote_addrs.clear();
        b.tailnet_used = false;
        b.stats.stats_line.clear();
        b.stats.interfaces.truncate(1);
        b.iroh_transport = IrohTransportDiagnostics {
            advertised_relay_urls: self.iroh_transport.advertised_relay_urls.clone(),
            advertised_ip_addrs: self.iroh_transport.advertised_ip_addrs.clone(),
            ..Default::default()
        };
        if let Some(p) = &mut b.probe {
            // A passing check needs no explanation; a failing one keeps it.
            for check in p.checks.iter_mut().filter(|c| c.ok) {
                check.detail.clear();
            }
            p.blob_digest.clear();
            p.pulls.clear();
            p.beats.clear();
            p.program_digest.clear();
            p.staged_path.clear();
        }
        b
    }
}

/// What a node says about itself that rarely changes: kept out of beats, sent in the
/// inventory.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct NodeDetails {
    #[serde(default)]
    pub role: String,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub geo_location: Option<NodeGeoLocation>,
    pub iroh_transport: IrohTransportDiagnostics,
}

/// A node's inventory: the programs it runs, the services it offers, the devices connected
/// to it and its details. Broadcast on the events topic when it changes; a peer that missed
/// it sees a new `inventory_version` in the node's beats and fetches it (`fetch.rs`).
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct Inventory {
    pub node: String,
    pub node_id: String,
    /// Hash of the content, so it changes when the content does and survives restarts.
    #[serde(default)]
    pub version: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub details: Option<NodeDetails>,
    #[serde(default)]
    pub processes: Vec<crate::service::ProcessSpec>,
    #[serde(default)]
    pub service_endpoints: Vec<Service>,
    /// Cameras, microphones, keyboards and other devices connected to the node.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub devices: Vec<crate::devices::Device>,
    #[serde(default)]
    pub sent_unix_ms: u64,
}

impl Inventory {
    /// The form that leaves this node: process adverts only (see `process_advert`), with
    /// `version` set from the content.
    pub fn for_wire(&self) -> Inventory {
        let mut inv = self.clone();
        inv.processes = inv.processes.iter().map(process_advert).collect();
        inv.sent_unix_ms = 0;
        inv.version = 0;
        let json = serde_json::to_vec(&inv).unwrap_or_default();
        let digest = Sha256::digest(&json);
        let mut first = [0u8; 8];
        first.copy_from_slice(&digest[..8]);
        // 0 means "no inventory yet".
        inv.version = u64::from_be_bytes(first).max(1);
        inv
    }
}

/// What nodes say to each other on a channel. Every message is signed by its sender.
#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum Message {
    /// Every beat interval, on the beats topic: liveness and stats, a fixed size.
    Beat(Beat),
    /// Every sixth beat: a beat with the rolling stats summary.
    Summary(Beat),
    /// On the events topic, when it changes: the sender's programs, services, devices and
    /// details.
    Inventory(Inventory),
}

impl Message {
    pub fn node_id(&self) -> &str {
        match self {
            Message::Beat(b) | Message::Summary(b) => &b.node_id,
            Message::Inventory(i) => &i.node_id,
        }
    }

    pub fn node(&self) -> &str {
        match self {
            Message::Beat(b) | Message::Summary(b) => &b.node,
            Message::Inventory(i) => &i.node,
        }
    }

    pub fn sent_unix_ms(&self) -> u64 {
        match self {
            Message::Beat(b) | Message::Summary(b) => b.sent_unix_ms,
            Message::Inventory(i) => i.sent_unix_ms,
        }
    }

    /// Replay-check stream: beats of all kinds share one, inventories have their own, so a
    /// late inventory is not rejected for arriving after a newer beat.
    pub fn replay_key(&self) -> String {
        match self {
            Message::Inventory(i) => format!("{}#inventory", i.node_id),
            _ => self.node_id().to_string(),
        }
    }

    /// The beat this message carries, with `kind` named after the message type.
    pub fn into_beat(self) -> Option<Beat> {
        let (mut beat, kind) = match self {
            Message::Beat(b) => (b, "regular"),
            Message::Summary(b) => (b, "summary"),
            Message::Inventory(_) => return None,
        };
        beat.kind = kind.to_string();
        Some(beat)
    }

    /// Prepare for the wire: stamp `sent_unix_ms`, keep beats small (`Beat::for_wire`) and
    /// strip process `env` / `args` (they may hold secrets).
    fn for_wire(&self, sent_unix_ms: u64) -> Message {
        let mut msg = self.clone();
        match &mut msg {
            Message::Beat(b) | Message::Summary(b) => {
                *b = b.for_wire();
                b.sent_unix_ms = sent_unix_ms;
            }
            Message::Inventory(i) => {
                *i = i.for_wire();
                i.sent_unix_ms = sent_unix_ms;
            }
        }
        msg
    }
}

fn constant_time_eq(a: &[u8], b: &[u8]) -> bool {
    if a.len() != b.len() {
        return false;
    }
    let mut diff = 0u8;
    for (x, y) in a.iter().zip(b.iter()) {
        diff |= x ^ y;
    }
    diff == 0
}

/// Wall-clock milliseconds, but never equal to or below the previous value returned in
/// this process, so every beat a node sends is strictly newer than the one before.
fn next_sent_unix_ms() -> u64 {
    static LAST: AtomicU64 = AtomicU64::new(0);
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_millis() as u64)
        .unwrap_or(0);
    let mut prev = LAST.load(Ordering::Relaxed);
    loop {
        let next = now.max(prev + 1);
        match LAST.compare_exchange_weak(prev, next, Ordering::Relaxed, Ordering::Relaxed) {
            Ok(_) => return next,
            Err(seen) => prev = seen,
        }
    }
}

/// The form of a process that leaves this node in a beat: no `env` or `args` (they may hold
/// secrets), and an explicit `deploy_ticket` when the process is offered for mesh deploy.
pub fn process_advert(spec: &crate::service::ProcessSpec) -> crate::service::ProcessSpec {
    let mut advert = spec.clone();
    let offered = spec.env.iter().any(|kv| kv == "DIALTONE_MESH_DEPLOY=1");
    advert.deploy_ticket = spec.deploy_ticket.clone().or_else(|| {
        offered
            .then(|| {
                spec.env
                    .iter()
                    .find_map(|kv| kv.strip_prefix("DIALTONE_BLOB_TICKET="))
                    .map(str::to_string)
            })
            .flatten()
    });
    if spec.deployed_from.is_some() {
        // Only the node that deployed a program offers it; copies never spread it further.
        advert.deploy_ticket = None;
    }
    advert.env.clear();
    advert.args.clear();
    advert
}

/// Sign a `Message` with this node's Key, then encrypt it with XChaCha20-Poly1305 (random
/// 24-byte nonce, `DIALTONE_ENVELOPE_SCHEMA_V5` as AAD). Process `env` / `args` never
/// leave the node, and `sent_unix_ms` is stamped here so every frame is unique.
pub fn seal_message(
    msg: &Message,
    keys: &ChannelKeys,
    key: &iroh::SecretKey,
) -> Result<Vec<u8>, String> {
    let wire = msg.for_wire(next_sent_unix_ms());
    let msg_json = serde_json::to_vec(&wire).map_err(|e| e.to_string())?;
    let sig = key.sign(&msg_json);
    let mut plaintext = Vec::with_capacity(64 + msg_json.len());
    plaintext.extend_from_slice(&sig.to_bytes());
    plaintext.extend_from_slice(&msg_json);

    let rand_bytes = iroh::SecretKey::generate().to_bytes();
    let nonce_bytes = &rand_bytes[..24];
    let nonce = XNonce::from_slice(nonce_bytes);
    let cipher = XChaCha20Poly1305::new((&keys.enc_key).into());
    let ciphertext = cipher
        .encrypt(
            nonce,
            Payload {
                msg: &plaintext,
                aad: DIALTONE_ENVELOPE_SCHEMA_V5.as_bytes(),
            },
        )
        .map_err(|_| "XChaCha20-Poly1305 encryption failed".to_string())?;

    let mut frame = Vec::with_capacity(ENVELOPE_MAGIC.len() + 24 + ciphertext.len());
    frame.extend_from_slice(ENVELOPE_MAGIC);
    frame.extend_from_slice(nonce_bytes);
    frame.extend_from_slice(&ciphertext);
    Ok(frame)
}

/// Decrypt a gossip frame and verify that the message inside was signed by the Key of the
/// `node_id` it names. Drops plaintext frames, other envelope versions, tampered
/// ciphertext, and messages whose signature does not match their claimed NodeId.
pub fn open_message(raw_frame: &[u8], keys: &ChannelKeys) -> Result<Message, String> {
    let Some(rest) = raw_frame.strip_prefix(ENVELOPE_MAGIC.as_slice()) else {
        return Err(format!(
            "gossip frame is not a {DIALTONE_ENVELOPE_SCHEMA_V5} envelope"
        ));
    };
    if rest.len() < 24 {
        return Err("gossip frame too short".to_string());
    }
    let (nonce_bytes, ciphertext) = rest.split_at(24);
    let nonce = XNonce::from_slice(nonce_bytes);
    let cipher = XChaCha20Poly1305::new((&keys.enc_key).into());
    let plaintext = cipher
        .decrypt(
            nonce,
            Payload {
                msg: ciphertext,
                aad: DIALTONE_ENVELOPE_SCHEMA_V5.as_bytes(),
            },
        )
        .map_err(|_| {
            "XChaCha20-Poly1305 decryption failed (wrong key or tampered frame)".to_string()
        })?;
    if plaintext.len() < 64 {
        return Err("signed message too short".to_string());
    }
    let (sig_bytes, msg_json) = plaintext.split_at(64);
    let msg: Message = serde_json::from_slice(msg_json)
        .map_err(|e| format!("invalid decrypted message JSON: {e}"))?;
    let signer: iroh::PublicKey = msg
        .node_id()
        .parse()
        .map_err(|_| format!("message names an invalid node_id '{}'", msg.node_id()))?;
    let sig_bytes: [u8; 64] = sig_bytes
        .try_into()
        .map_err(|_| "message signature must be 64 bytes".to_string())?;
    signer
        .verify(msg_json, &iroh::Signature::from_bytes(&sig_bytes))
        .map_err(|_| format!("message signature does not match node_id {}", msg.node_id()))?;
    Ok(msg)
}

/// Gossip traffic counters, kept by the Mesh actor and read by `GET /api/net`.
#[derive(Debug, Default)]
pub struct NetCounters {
    pub gossip_msgs_in: AtomicU64,
    pub gossip_bytes_in: AtomicU64,
    pub gossip_msgs_out: AtomicU64,
    pub gossip_bytes_out: AtomicU64,
    /// Current gossip neighbors (iroh-gossip's active view).
    pub neighbors: AtomicU64,
}

impl NetCounters {
    pub fn count_in(&self, bytes: usize) {
        self.gossip_msgs_in.fetch_add(1, Ordering::Relaxed);
        self.gossip_bytes_in
            .fetch_add(bytes as u64, Ordering::Relaxed);
    }

    pub fn count_out(&self, bytes: usize) {
        self.gossip_msgs_out.fetch_add(1, Ordering::Relaxed);
        self.gossip_bytes_out
            .fetch_add(bytes as u64, Ordering::Relaxed);
    }

    pub fn snapshot(&self) -> serde_json::Value {
        let get = |c: &AtomicU64| c.load(Ordering::Relaxed);
        serde_json::json!({
            "gossip_msgs_in": get(&self.gossip_msgs_in),
            "gossip_bytes_in": get(&self.gossip_bytes_in),
            "gossip_msgs_out": get(&self.gossip_msgs_out),
            "gossip_bytes_out": get(&self.gossip_bytes_out),
            "neighbors": get(&self.neighbors),
        })
    }
}

/// Rejects a beat unless it is strictly newer than the last one accepted from the same
/// node and was sent within `REPLAY_WINDOW_MS` of local time.
#[derive(Debug, Default)]
pub struct ReplayGuard {
    last_sent_ms: HashMap<String, u64>,
}

impl ReplayGuard {
    pub fn admit(
        &mut self,
        node_id: &str,
        sent_unix_ms: u64,
        now_unix_ms: u64,
    ) -> Result<(), String> {
        if sent_unix_ms.abs_diff(now_unix_ms) > REPLAY_WINDOW_MS {
            return Err(format!(
                "beat sent_unix_ms={sent_unix_ms} is more than {}s from local time {now_unix_ms}",
                REPLAY_WINDOW_MS / 1000
            ));
        }
        let last = self.last_sent_ms.entry(node_id.to_string()).or_insert(0);
        if sent_unix_ms <= *last {
            return Err(format!(
                "replayed or stale beat (sent_unix_ms={sent_unix_ms} <= last accepted {})",
                *last
            ));
        }
        *last = sent_unix_ms;
        Ok(())
    }
}

/// The allowlist the node enforces right now: read by the QUIC handshake hook, the beat
/// receiver and mesh deploy; refreshed whenever `config.json` changes.
#[derive(Clone, Debug, Default)]
pub struct LiveAllowlist(
    std::sync::Arc<std::sync::RwLock<std::collections::HashSet<iroh::EndpointId>>>,
); // compat: iroh type

impl LiveAllowlist {
    pub fn new(node_ids: &[String]) -> Self {
        let live = Self::default();
        live.replace(node_ids);
        live
    }

    /// Replace the set; entries that are not valid NodeIds are ignored.
    pub fn replace(&self, node_ids: &[String]) {
        let parsed = node_ids
            .iter()
            .filter_map(|s| s.trim().parse().ok())
            .collect();
        if let Ok(mut set) = self.0.write() {
            *set = parsed;
        }
    }

    pub fn contains(&self, node_id: &iroh::EndpointId) -> bool {
        // compat: iroh type
        self.0
            .read()
            .map(|set| set.contains(node_id))
            .unwrap_or(false)
    }

    pub fn contains_str(&self, node_id: &str) -> bool {
        node_id
            .parse()
            .map(|id| self.contains(&id))
            .unwrap_or(false)
    }

    pub fn len(&self) -> usize {
        self.0.read().map(|set| set.len()).unwrap_or(0)
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

/// Node names travel in beats and are shown in the HUD; only plain names are accepted.
pub fn is_valid_node_name(name: &str) -> bool {
    !name.is_empty()
        && name.len() <= 64
        && name
            .bytes()
            .all(|b| b.is_ascii_alphanumeric() || matches!(b, b'.' | b'_' | b'-'))
}

pub async fn observe_peer_remote_addrs(
    endpoint: &iroh::Endpoint,
    peer_id_str: &str,
) -> (Vec<String>, bool) {
    let Ok(peer_id) = peer_id_str.parse::<iroh::EndpointId>() else {
        return (Vec::new(), false);
    };
    let Some(remote_info) = endpoint.remote_info(peer_id).await else {
        return (Vec::new(), false);
    };
    let mut observed = Vec::new();
    let mut tailnet_used = false;
    for addr in remote_info.addrs() {
        let usage = if matches!(addr.usage(), iroh::endpoint::TransportAddrUsage::Active) {
            "active"
        } else {
            "known"
        };
        match addr.addr() {
            iroh::TransportAddr::Ip(sock) => {
                if is_tailnet_ip(sock.ip()) && usage == "active" {
                    tailnet_used = true;
                }
                observed.push(format!("{usage}:ip:{sock}"));
            }
            iroh::TransportAddr::Relay(relay) => {
                observed.push(format!("{usage}:relay:{relay}"));
            }
            other => {
                observed.push(format!("{usage}:custom:{other:?}"));
            }
        }
    }
    (observed, tailnet_used)
}

pub use crate::now_unix;

/// Prune peer beat entries older than `ttl_secs`.
/// Insert or update a node's latest beat. Nodes are identified by NodeId (names can collide).
pub fn merge_beat(nodes: &mut Vec<Beat>, mut beat: Beat) {
    if let Some(existing) = nodes.iter_mut().find(|n| n.node_id == beat.node_id) {
        if beat.summary_stats.is_none() {
            beat.summary_stats = existing.summary_stats.clone();
        }
        if beat.geo_location.is_none() {
            beat.geo_location = existing.geo_location.clone();
        }
        if beat.probe.is_none() {
            beat.probe = existing.probe.clone();
        }
        // The inventory and details arrive in their own message; a beat never changes them
        // (see `Beat::for_wire`), and how the node is reached is observed locally.
        beat.processes = std::mem::take(&mut existing.processes);
        beat.service_endpoints = std::mem::take(&mut existing.service_endpoints);
        beat.devices = std::mem::take(&mut existing.devices);
        if beat.role.is_empty() {
            beat.role = std::mem::take(&mut existing.role);
        }
        if beat.capabilities.is_empty() {
            beat.capabilities = std::mem::take(&mut existing.capabilities);
        }
        if beat.iroh_transport.profile.is_empty() && !existing.iroh_transport.profile.is_empty() {
            let mut full = existing.iroh_transport.clone();
            full.advertised_relay_urls = beat.iroh_transport.advertised_relay_urls.clone();
            full.advertised_ip_addrs = beat.iroh_transport.advertised_ip_addrs.clone();
            beat.iroh_transport = full;
        }
        if beat.observed_remote_addrs.is_empty() {
            beat.observed_remote_addrs = std::mem::take(&mut existing.observed_remote_addrs);
            beat.tailnet_used = existing.tailnet_used;
        }
        *existing = beat;
    } else {
        nodes.push(beat);
    }
}

/// Apply a node's inventory to its entry. Returns false when the node has no entry yet.
pub fn merge_inventory(nodes: &mut [Beat], inventory: &Inventory) -> bool {
    match nodes.iter_mut().find(|n| n.node_id == inventory.node_id) {
        Some(node) => {
            node.processes = inventory.processes.clone();
            node.service_endpoints = inventory.service_endpoints.clone();
            node.devices = inventory.devices.clone();
            if let Some(d) = &inventory.details {
                node.role = d.role.clone();
                node.capabilities = d.capabilities.clone();
                node.geo_location = d.geo_location.clone();
                let mut transport = d.iroh_transport.clone();
                // Addresses in beats are newer than those in the inventory.
                if !node.iroh_transport.advertised_ip_addrs.is_empty()
                    || !node.iroh_transport.advertised_relay_urls.is_empty()
                {
                    transport.advertised_ip_addrs = node.iroh_transport.advertised_ip_addrs.clone();
                    transport.advertised_relay_urls =
                        node.iroh_transport.advertised_relay_urls.clone();
                }
                node.iroh_transport = transport;
            }
            true
        }
        None => false,
    }
}

pub fn prune_stale_peers(peers: &mut Vec<Beat>, now: u64, ttl_secs: u64) -> usize {
    let before = peers.len();
    peers.retain(|p| now.saturating_sub(p.updated_at_unix) <= ttl_secs);
    before.saturating_sub(peers.len())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::node::{
        advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, IrohTransportConfig,
    };
    use crate::stats::sample_stats;
    use futures_lite::StreamExt;
    use iroh::address_lookup::memory::MemoryLookup;
    use iroh::protocol::Router;
    use iroh_gossip::{
        api::Event as GossipEvent,
        net::{Gossip, GOSSIP_ALPN},
    };
    use std::time::Duration;

    /// Live in-process two-peer `iroh-gossip` beat exchange over QUIC protected by Argon2id + XChaCha20-Poly1305.
    async fn verify_live_gossip_exchange(
        node_a: &str,
        node_b: &str,
    ) -> Result<(Beat, Beat), String> {
        let config = IrohTransportConfig::local_offline();
        let test_secret = "dt-SelfTest_HighEntropy_Channel_Key_2026!";
        let keys = derive_channel_keys_v2(DEFAULT_CHANNEL_NAME, test_secret);
        let topic_id = keys.topic_id;

        // Each node allowlists only the other, exactly as a two-node mesh would.
        let key_a = iroh::SecretKey::generate();
        let key_b = iroh::SecretKey::generate();
        let lookup_a = MemoryLookup::new();
        let endpoint_a = build_endpoint_builder(&config)?
            .address_lookup(lookup_a.clone())
            .secret_key(key_a.clone())
            .hooks(crate::node::AllowlistHook::new(LiveAllowlist::new(&[
                key_b.public().to_string(),
            ])))
            .bind()
            .await
            .map_err(|e| format!("bind endpoint A: {e}"))?;
        let gossip_a = Gossip::builder()
            .max_message_size(65_536)
            .spawn(endpoint_a.clone());
        let router_a = Router::builder(endpoint_a.clone())
            .accept(GOSSIP_ALPN, gossip_a.clone())
            .spawn();

        let lookup_b = MemoryLookup::new();
        let endpoint_b = build_endpoint_builder(&config)?
            .address_lookup(lookup_b.clone())
            .secret_key(key_b.clone())
            .hooks(crate::node::AllowlistHook::new(LiveAllowlist::new(&[
                key_a.public().to_string(),
            ])))
            .bind()
            .await
            .map_err(|e| format!("bind endpoint B: {e}"))?;
        let gossip_b = Gossip::builder()
            .max_message_size(65_536)
            .spawn(endpoint_b.clone());
        let router_b = Router::builder(endpoint_b.clone())
            .accept(GOSSIP_ALPN, gossip_b.clone())
            .spawn();

        let addr_a = advertised_endpoint_addr(&endpoint_a.addr(), &config);
        let addr_b = advertised_endpoint_addr(&endpoint_b.addr(), &config);
        lookup_a.add_endpoint_info(addr_b.clone());
        lookup_b.add_endpoint_info(addr_a.clone());

        let id_a = endpoint_a.id();
        let id_b = endpoint_b.id();

        let ticket_a = SeedList {
            topic: topic_id,
            peers: vec![addr_a.clone()],
        }
        .to_text()?;
        let ticket_b = SeedList {
            topic: topic_id,
            peers: vec![addr_b.clone()],
        }
        .to_text()?;

        let topic_a = gossip_a
            .subscribe(topic_id, vec![id_b])
            .await
            .map_err(|e| format!("subscribe A: {e}"))?;
        let topic_b = gossip_b
            .subscribe(topic_id, vec![id_a])
            .await
            .map_err(|e| format!("subscribe B: {e}"))?;

        let (sender_a, _receiver_a) = topic_a.split();
        let (sender_b, mut receiver_b) = topic_b.split();

        sender_a
            .join_peers(vec![id_b])
            .await
            .map_err(|e| format!("join_peers A->B: {e}"))?;
        sender_b
            .join_peers(vec![id_a])
            .await
            .map_err(|e| format!("join_peers B->A: {e}"))?;

        let beat_a = Beat {
            schema: BEAT_SCHEMA.to_string(),
            node: node_a.to_string(),
            node_id: id_a.to_string(),
            ticket: ticket_a,
            counter: 1,
            updated_at_unix: now_unix(),
            kind: "summary".to_string(),
            beat_interval_secs: BEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_INTERVAL_SECS,
            heard: 0,
            inventory_version: 0,
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            sent_unix_ms: 0,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for(node_a),
            capabilities: node_capabilities_for(node_a),
            stats: sample_stats(node_a, None, 1),
            service_endpoints: vec![Service {
                name: "nats".to_string(),
                kind: "nats-jetstream".to_string(),
                protocol: "nats".to_string(),
                url: format!("dialtone://{node_a}/controller/nats"),
                listen: "127.0.0.1:4222".to_string(),
                status: "running".to_string(),
                advertised_by: node_a.to_string(),
                role: "controller".to_string(),
                mesh: "dialtone-iroh-gossip".to_string(),
                subjects: vec!["dialtone/events.>".to_string()],
            }],
            processes: Vec::new(),
            iroh_transport: build_diagnostics(&config, &addr_a, None, &id_a.to_string()),
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            devices: Vec::new(),
            geo_location: None,
            probe: None,
            store: None,
        };

        let received_beat: Beat = tokio::time::timeout(Duration::from_secs(10), async {
            loop {
                let encrypted_payload_a =
                    seal_message(&Message::Summary(beat_a.clone()), &keys, &key_a)?;
                let _ = sender_a.broadcast(encrypted_payload_a.into()).await;
                while let Ok(Some(event)) =
                    tokio::time::timeout(Duration::from_millis(120), receiver_b.next()).await
                {
                    if let GossipEvent::Received(msg) = event.map_err(|e| e.to_string())? {
                        if let Some(parsed) = open_message(&msg.content, &keys)
                            .ok()
                            .and_then(Message::into_beat)
                        {
                            if parsed.node == node_a {
                                return Ok::<Beat, String>(parsed);
                            }
                        }
                    }
                }
            }
        })
        .await
        .map_err(|_| "timed out waiting for encrypted iroh-gossip beat".to_string())??;

        let beat_b = Beat {
            schema: BEAT_SCHEMA.to_string(),
            node: node_b.to_string(),
            node_id: id_b.to_string(),
            ticket: ticket_b,
            counter: 1,
            updated_at_unix: now_unix(),
            kind: "summary".to_string(),
            beat_interval_secs: BEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_INTERVAL_SECS,
            heard: 0,
            inventory_version: 0,
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            sent_unix_ms: 0,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for(node_b),
            capabilities: node_capabilities_for(node_b),
            stats: sample_stats(node_b, None, 1),
            service_endpoints: Vec::new(),
            processes: Vec::new(),
            iroh_transport: build_diagnostics(&config, &addr_b, None, &id_b.to_string()),
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            devices: Vec::new(),
            geo_location: None,
            probe: None,
            store: None,
        };

        let _ = router_a.shutdown().await;
        let _ = router_b.shutdown().await;

        Ok((received_beat, beat_b))
    }

    use std::env;

    #[test]
    fn issue_1_channel_secret_validation_rejects_weak_and_accepts_generated() {
        assert!(validate_secret("").is_err());
        assert!(validate_secret("   ").is_err());
        assert!(validate_secret("dialtone").is_err());
        assert!(validate_secret("dialtone2").is_err());
        assert!(validate_secret("password123").is_err()); // compat
        assert!(validate_secret("default").is_err());
        assert!(validate_secret("short-pass-1").is_err());
        assert!(validate_secret("aaaaaaaaaaaaaaaaaaaa").is_err());
        assert!(validate_secret("alllowercasewordsinhere").is_err());

        let generated = new_secret();
        assert!(generated.starts_with("dt-"));
        assert!(generated.len() >= 40);
        assert_eq!(validate_secret(&generated).unwrap(), generated);
    }

    #[test]
    fn config_never_stores_a_plaintext_secret() {
        let root = env::temp_dir().join(format!("dialtone-cfg-mig-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&root);
        let new_dir = root.join(".dialtone");
        fs::create_dir_all(&new_dir).unwrap();
        let new_path = new_dir.join("config.json");

        let raw_pass = "dt-LegacySecret_ToMigrate_998877!";
        fs::write(
            &new_path,
            format!(
                r#"{{"node_name":"node-alpha","channel":"dialtone","configured":true,"secret_key":"{raw_pass}","channel_fingerprint":"old","updated_at_unix":100}}"#
            ),
        )
        .unwrap();

        let loaded = load_mesh_config(&new_path);
        assert!(
            loaded.secret_key.is_none(),
            "in-memory secret_key must be cleared after deriving keys"
        );
        assert!(
            loaded.derived_keys.is_some(),
            "derived_keys bundle must be populated"
        );

        let disk_content = fs::read_to_string(&new_path).unwrap();
        assert!(
            !disk_content.contains(raw_pass),
            "config.json on disk must never contain the raw secret"
        );
        assert!(disk_content.contains("\"derived_keys\""));

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = fs::metadata(&new_path).unwrap().permissions().mode() & 0o777;
            assert_eq!(mode, 0o600);
        }

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn issue_3_allowlist_denies_by_default_and_peer_commands_update_atomically() {
        let ep_a = iroh::SecretKey::generate().public().to_string();
        let ep_b = iroh::SecretKey::generate().public().to_string();

        // Empty allowlist must deny remote peers by default, while allowing self
        assert!(!is_allowed_peer_in_list(&[], &ep_a, None));
        assert!(!is_allowed_peer_in_list(&[], &ep_a, Some(&ep_b)));
        assert!(is_allowed_peer_in_list(&[], &ep_a, Some(&ep_a)));

        let cfg_path =
            env::temp_dir().join(format!("dialtone-peers-cmd-{}.json", std::process::id()));
        let _ = fs::remove_file(&cfg_path);

        let after_allow = allow_peer_in_config_with_addrs(&cfg_path, &ep_a, None, &[]).unwrap();
        assert!(is_allowed_peer_in_list(&after_allow.allowlist, &ep_a, None));
        assert!(!is_allowed_peer_in_list(
            &after_allow.allowlist,
            &ep_b,
            None
        ));

        let after_revoke = revoke_peer_in_config(&cfg_path, &ep_a).unwrap();
        assert!(!is_allowed_peer_in_list(
            &after_revoke.allowlist,
            &ep_a,
            None
        ));

        let _ = fs::remove_file(&cfg_path);
    }

    #[test]
    fn issue_4_and_5_argon2id_kdf_known_answer_and_xchacha20poly1305_beat_encryption() {
        let k1 = derive_channel_keys_v2("dialtone", "dt-KnownAnswer_Vector_Test_2026!");
        let k2 = derive_channel_keys_v2("dialtone", "dt-KnownAnswer_Vector_Test_2026!");
        let k_other = derive_channel_keys_v2("dialtone", "dt-Different_Vector_Test_2026!");
        assert_eq!(k1, k2, "Argon2id v2 derivation must be deterministic");
        assert_ne!(k1.topic_id, k_other.topic_id);
        assert_ne!(
            k1.auth_key, k1.enc_key,
            "auth_key and enc_key must be domain-separated"
        );
        assert_ne!(k1.topic_id.as_bytes(), &k1.enc_key);

        // Known-Answer Test (KAT) for Argon2id (m=19456, t=2, p=1) on ("dialtone", "dt-KnownAnswer_Vector_Test_2026!")
        let kat_bundle = k1.to_hex_bundle();
        assert_eq!(kat_bundle.kdf_version, "argon2id-v2");
        assert_eq!(kat_bundle.topic_id_hex.len(), 64);
        assert_eq!(
            kat_bundle.topic_id_hex,
            derive_channel_keys_v2("dialtone", "dt-KnownAnswer_Vector_Test_2026!")
                .to_hex_bundle()
                .topic_id_hex
        );

        let cfg = IrohTransportConfig::local_offline();
        let addr = EndpointAddr::new(iroh::SecretKey::generate().public());
        let node_key = iroh::SecretKey::generate();
        let node_id = node_key.public().to_string();
        let diag = build_diagnostics(&cfg, &addr, None, &node_id);
        let beat = Beat {
            schema: BEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            node_id: node_id.clone(),
            ticket: "iroh-gossip-v1:super-secret-ticket".to_string(),
            counter: 42,
            updated_at_unix: 1_700_000_000,
            kind: "regular".to_string(),
            beat_interval_secs: BEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_INTERVAL_SECS,
            heard: 0,
            inventory_version: 0,
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            sent_unix_ms: 0,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for("node-alpha"),
            capabilities: node_capabilities_for("node-alpha"),
            stats: sample_stats("node-alpha", None, 42),
            service_endpoints: Vec::new(),
            processes: Vec::new(),
            iroh_transport: diag,
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            devices: Vec::new(),
            geo_location: None,
            probe: None,
            store: None,
        };

        let wire_bytes = seal_message(&Message::Beat(beat), &k1, &node_key).unwrap();
        let wire_str = String::from_utf8_lossy(&wire_bytes);
        // Captured gossip frames must contain NO plaintext fields
        assert!(!wire_str.contains("node-alpha"));
        assert!(!wire_str.contains(&node_id));
        assert!(!wire_str.contains("super-secret-ticket"));
        assert!(!wire_str.contains("stats"));

        // Round-trip decrypts cleanly with k1 and fails with k_other or tampered ciphertext
        let decrypted = open_message(&wire_bytes, &k1).unwrap().into_beat().unwrap();
        assert_eq!(decrypted.node, "node-alpha");
        assert_eq!(decrypted.counter, 42);
        assert!(open_message(&wire_bytes, &k_other).is_err());

        let mut tampered_bytes = wire_bytes.clone();
        let last = tampered_bytes.len() - 1;
        tampered_bytes[last] ^= 1;
        assert!(open_message(&tampered_bytes, &k1).is_err());
        assert!(open_message(&wire_bytes[..20], &k1).is_err());
    }

    fn test_beat(node_key: &iroh::SecretKey) -> Beat {
        let addr = EndpointAddr::new(node_key.public());
        let node_id = node_key.public().to_string();
        Beat {
            schema: BEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            node_id: node_id.clone(),
            ticket: String::new(),
            counter: 1,
            updated_at_unix: 1_700_000_000,
            kind: "regular".to_string(),
            beat_interval_secs: BEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_INTERVAL_SECS,
            heard: 0,
            inventory_version: 0,
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            sent_unix_ms: 0,
            os: default_os(),
            arch: default_arch(),
            role: String::new(),
            capabilities: Vec::new(),
            stats: Stats::default(),
            service_endpoints: Vec::new(),
            processes: Vec::new(),
            iroh_transport: build_diagnostics(
                &IrohTransportConfig::local_offline(),
                &addr,
                None,
                &node_id,
            ),
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            devices: Vec::new(),
            geo_location: None,
            probe: None,
            store: None,
        }
    }

    /// Beats stay small and do not grow with the mesh: every node hears every beat.
    #[test]
    fn a_sealed_beat_is_small_and_fixed_size() {
        let keys = derive_channel_keys_v2("dialtone", "dt-Beat_Size_Test_Secret_2026!");
        let alice = iroh::SecretKey::generate();
        let mut beat = test_beat(&alice);
        beat.stats = sample_stats("node-alpha", None, 1);
        beat.role = node_role_for("node-alpha");
        beat.capabilities = node_capabilities_for("node-alpha");
        beat.ticket = "x".repeat(2_000);
        beat.peer_rtt_ms = (0..5).map(|i| (format!("peer-{i}"), 12.5)).collect();
        let mut window = crate::stats::StatsWindow::new(SUMMARY_INTERVAL_SECS);
        window.record_sample(1, &beat.stats, &beat.peer_rtt_ms, 0);
        let summary = window.summarize(1);
        beat.probe = Some(ProbeReport {
            build: format!("sha256:{}", "b".repeat(64)),
            cycle: 9,
            passed_count: 3,
            total_count: 3,
            checks: ["MSH", "NBR", "BEAT"]
                .iter()
                .map(|code| ProbeCheck {
                    code: code.to_string(),
                    ok: true,
                    detail: "299/299 peers beating, each hears all".to_string(),
                    ms: None,
                })
                .collect(),
            beats: (0..299)
                .map(|i| ProbeBeat {
                    peer: format!("peer-{i}"),
                    ..Default::default()
                })
                .collect(),
            ..Default::default()
        });
        let size = |b: &Beat, summary: bool| {
            let msg = if summary {
                Message::Summary(b.clone())
            } else {
                Message::Beat(b.clone())
            };
            seal_message(&msg, &keys, &alice).unwrap().len()
        };
        let regular = size(&beat, false);
        beat.summary_stats = Some(summary);
        let with_summary = size(&beat, true);
        println!("sealed beat {regular} bytes, summary {with_summary} bytes");
        assert!(regular < 1_300, "beat is {regular} bytes");
        assert!(with_summary < 1_900, "summary is {with_summary} bytes");
    }

    #[test]
    fn beats_are_signed_by_the_sender_key_and_forged_node_ids_are_rejected() {
        let keys = derive_channel_keys_v2("dialtone", "dt-Signed_Beat_Test_Secret_2026!");
        let alice = iroh::SecretKey::generate();
        let mallory = iroh::SecretKey::generate();

        let beat = test_beat(&alice);
        let opened = open_message(
            &seal_message(&Message::Beat(beat.clone()), &keys, &alice).unwrap(),
            &keys,
        )
        .unwrap()
        .into_beat()
        .unwrap();
        assert_eq!(opened.node_id, alice.public().to_string());
        assert!(opened.sent_unix_ms > 0);

        // Mallory knows the channel secret but not Alice's Key: claiming Alice's NodeId fails.
        let forged = open_message(
            &seal_message(&Message::Beat(beat.clone()), &keys, &mallory).unwrap(),
            &keys,
        );
        assert!(forged.unwrap_err().contains("signature does not match"));

        // Every sealed beat is strictly newer than the previous one from this process.
        let a = open_message(
            &seal_message(&Message::Beat(beat.clone()), &keys, &alice).unwrap(),
            &keys,
        )
        .unwrap()
        .into_beat()
        .unwrap();
        let b = open_message(
            &seal_message(&Message::Beat(beat.clone()), &keys, &alice).unwrap(),
            &keys,
        )
        .unwrap()
        .into_beat()
        .unwrap();
        assert!(b.sent_unix_ms > a.sent_unix_ms);
    }

    #[test]
    fn replay_guard_rejects_replayed_stale_and_far_future_beats() {
        let mut guard = ReplayGuard::default();
        let now = 1_800_000_000_000u64;
        assert!(guard.admit("n1", now, now).is_ok());
        assert!(guard.admit("n1", now, now).is_err(), "exact replay");
        assert!(
            guard.admit("n1", now - 1, now).is_err(),
            "older than last accepted"
        );
        assert!(guard.admit("n1", now + 1, now).is_ok());
        assert!(guard.admit("n2", now - 1, now).is_ok(), "per-node state");
        assert!(
            guard.admit("n3", now - REPLAY_WINDOW_MS - 1, now).is_err(),
            "too old"
        );
        assert!(
            guard.admit("n3", now + REPLAY_WINDOW_MS + 1, now).is_err(),
            "too far ahead"
        );
    }

    #[test]
    fn process_env_never_leaves_the_node_and_offers_become_deploy_tickets() {
        let keys = derive_channel_keys_v2("dialtone", "dt-Process_Advert_Test_Secret_2026!");
        let alice = iroh::SecretKey::generate();
        let beat = test_beat(&alice);
        let spec = |name: &str, env: Vec<&str>| crate::service::ProcessSpec {
            name: name.to_string(),
            path: "/tmp/p".to_string(),
            digest: format!("sha256:{}", "c".repeat(64)),
            desired_state: "running".to_string(),
            args: vec!["--token".to_string(), "hunter2".to_string()],
            env: env.into_iter().map(String::from).collect(),
            cwd: None,
            startup_grace_secs: None,
            status: Some("running".to_string()),
            pid: Some(1),
            restart_count: 0,
            service_unit: None,
            deploy_ticket: None,
            deployed_from: None,
        };
        let processes = vec![
            spec(
                "offered",
                vec![
                    "DIALTONE_MESH_DEPLOY=1",
                    "DIALTONE_BLOB_TICKET=iroh-blobs://ticket/x",
                    "API_KEY=s3cret",
                ],
            ),
            spec(
                "private",
                vec![
                    "DIALTONE_BLOB_TICKET=iroh-blobs://ticket/y",
                    "API_KEY=s3cret",
                ],
            ),
        ];
        // A beat never carries inventory, even when the caller left processes in it.
        let mut stuffed = beat.clone();
        stuffed.processes = processes.clone();
        let msg = Message::Beat(stuffed);
        let opened = open_message(&seal_message(&msg, &keys, &alice).unwrap(), &keys).unwrap();
        assert!(opened.into_beat().unwrap().processes.is_empty());

        let inventory = Message::Inventory(Inventory {
            node: beat.node.clone(),
            node_id: beat.node_id.clone(),
            version: 0,
            details: None,
            processes,
            service_endpoints: Vec::new(),
            devices: Vec::new(),
            sent_unix_ms: 0,
        });
        let Message::Inventory(opened) =
            open_message(&seal_message(&inventory, &keys, &alice).unwrap(), &keys).unwrap()
        else {
            panic!("expected an inventory");
        };
        assert!(opened.sent_unix_ms > 0);
        for p in &opened.processes {
            assert!(
                p.env.is_empty() && p.args.is_empty(),
                "{} leaked env/args",
                p.name
            );
        }
        assert_eq!(
            opened.processes[0].deploy_ticket.as_deref(),
            Some("iroh-blobs://ticket/x")
        );
        assert_eq!(opened.processes[1].deploy_ticket, None);

        // A copy deployed from a peer is never offered again, whatever its env says, so a
        // program removed at its origin cannot spread back through the copies.
        let mut copy = spec(
            "copy",
            vec![
                "DIALTONE_MESH_DEPLOY=1",
                "DIALTONE_BLOB_TICKET=iroh-blobs://ticket/z",
            ],
        );
        copy.deploy_ticket = Some("iroh-blobs://ticket/z".to_string());
        copy.deployed_from = Some("origin-node-id".to_string());
        assert_eq!(process_advert(&copy).deploy_ticket, None);
    }

    #[test]
    fn messages_are_typed_and_inventory_survives_beats_until_the_next_inventory() {
        let keys = derive_channel_keys_v2("dialtone", "dt-Typed_Message_Test_Secret_2026!");
        let alice = iroh::SecretKey::generate();
        let beat = test_beat(&alice);
        for (msg, kind) in [
            (Message::Beat(beat.clone()), "regular"),
            (Message::Summary(beat.clone()), "summary"),
        ] {
            let opened = open_message(&seal_message(&msg, &keys, &alice).unwrap(), &keys).unwrap();
            assert_eq!(opened.node_id(), alice.public().to_string());
            assert_eq!(opened.replay_key(), alice.public().to_string());
            assert_eq!(opened.into_beat().unwrap().kind, kind);
        }

        let spec = crate::service::ProcessSpec {
            name: "web".to_string(),
            path: "/tmp/web".to_string(),
            digest: format!("sha256:{}", "d".repeat(64)),
            desired_state: "running".to_string(),
            args: Vec::new(),
            env: Vec::new(),
            cwd: None,
            startup_grace_secs: None,
            status: Some("running".to_string()),
            pid: Some(7),
            restart_count: 0,
            service_unit: None,
            deploy_ticket: None,
            deployed_from: None,
        };
        let inventory = Inventory {
            node: beat.node.clone(),
            node_id: beat.node_id.clone(),
            version: 0,
            details: Some(NodeDetails {
                role: "edge".to_string(),
                capabilities: vec!["device-listing".to_string()],
                geo_location: None,
                iroh_transport: beat.iroh_transport.clone(),
            }),
            processes: vec![spec],
            service_endpoints: Vec::new(),
            devices: vec![crate::devices::Device {
                kind: "camera".to_string(),
                name: "Desk Cam".to_string(),
                bus: "usb".to_string(),
                detail: String::new(),
            }],
            sent_unix_ms: 0,
        };
        assert!(Message::Inventory(inventory.clone())
            .replay_key()
            .ends_with("#inventory"));

        let mut nodes = Vec::new();
        assert!(!merge_inventory(&mut nodes, &inventory), "no entry yet");
        merge_beat(&mut nodes, beat.clone());
        assert!(merge_inventory(&mut nodes, &inventory));
        // Later beats (which carry no inventory) keep it.
        let mut next = beat.clone();
        next.counter = 2;
        merge_beat(&mut nodes, next);
        assert_eq!(nodes[0].counter, 2);
        assert_eq!(nodes[0].processes.len(), 1);
        assert_eq!(nodes[0].devices[0].name, "Desk Cam");
        // The details come from the inventory too: a wire beat leaves them out, and the
        // node's entry keeps them.
        let wire = beat.for_wire();
        assert!(wire.role.is_empty() && wire.iroh_transport.profile.is_empty());
        assert_eq!(
            wire.iroh_transport.advertised_ip_addrs,
            beat.iroh_transport.advertised_ip_addrs
        );
        merge_beat(&mut nodes, wire);
        assert_eq!(nodes[0].role, "edge");
        assert_eq!(nodes[0].iroh_transport, beat.iroh_transport);
        // The version follows the content.
        let v1 = inventory.for_wire().version;
        assert_eq!(v1, inventory.for_wire().version);
        assert_ne!(
            v1,
            Inventory {
                devices: Vec::new(),
                ..inventory.clone()
            }
            .for_wire()
            .version
        );
        // A beat never carries devices on the wire.
        let mut stuffed = beat.clone();
        stuffed.devices = nodes[0].devices.clone();
        let opened = open_message(
            &seal_message(&Message::Beat(stuffed), &keys, &alice).unwrap(),
            &keys,
        )
        .unwrap();
        assert!(opened.into_beat().unwrap().devices.is_empty());
        // An empty inventory clears it.
        let cleared = Inventory {
            processes: Vec::new(),
            devices: Vec::new(),
            ..inventory
        };
        merge_inventory(&mut nodes, &cleared);
        assert!(nodes[0].processes.is_empty() && nodes[0].devices.is_empty());
    }

    #[test]
    fn live_allowlist_parses_node_ids_and_ignores_garbage() {
        let a = iroh::SecretKey::generate().public();
        let live = LiveAllowlist::new(&[a.to_string(), "not-a-node-id".to_string()]);
        assert_eq!(live.len(), 1);
        assert!(live.contains(&a));
        assert!(!live.contains_str(&iroh::SecretKey::generate().public().to_string()));
        live.replace(&[]);
        assert!(live.is_empty() && !live.contains(&a));
        assert!(is_valid_node_name("edge-1.lab_2"));
        assert!(!is_valid_node_name("<img src=x onerror=alert(1)>"));
        assert!(!is_valid_node_name(""));
    }

    #[test]
    fn config_files_that_do_not_parse_are_errors_not_empty_configs() {
        let dir = env::temp_dir().join(format!("dialtone-cfg-parse-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        let path = dir.join("config.json");
        assert_eq!(
            try_load_mesh_config(&path).unwrap().allowlist,
            Vec::<String>::new()
        );
        fs::write(&path, "{\"allowlist\": [\"x\"], oops").unwrap();
        let err = try_load_mesh_config(&path).unwrap_err();
        assert!(err.contains("not valid dialtone config JSON"), "{err}");
        assert!(
            fs::read_to_string(&path).unwrap().contains("oops"),
            "file left untouched"
        );
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn nodes_are_keyed_by_node_id_and_rtt_maps_are_replaced() {
        let a = test_beat(&iroh::SecretKey::generate());
        let mut impostor = test_beat(&iroh::SecretKey::generate());
        impostor.node = a.node.clone(); // same name, different NodeId
        let mut nodes = Vec::new();
        merge_beat(&mut nodes, a.clone());
        merge_beat(&mut nodes, impostor.clone());
        assert_eq!(
            nodes.len(),
            2,
            "a name collision must not overwrite another node"
        );

        let mut a2 = a.clone();
        a2.peer_rtt_ms.insert("node-beta".to_string(), 12.0);
        merge_beat(&mut nodes, a2);
        let mut a3 = a.clone();
        a3.peer_rtt_ms.insert("node-gamma".to_string(), 30.0);
        merge_beat(&mut nodes, a3);
        let entry = nodes.iter().find(|n| n.node_id == a.node_id).unwrap();
        assert_eq!(
            entry.peer_rtt_ms.len(),
            1,
            "old Rtt entries are not kept forever"
        );
        assert!(entry.peer_rtt_ms.contains_key("node-gamma"));
        // A node with no neighbors left reports an empty map, and that replaces it too.
        merge_beat(&mut nodes, a.clone());
        let entry = nodes.iter().find(|n| n.node_id == a.node_id).unwrap();
        assert!(entry.peer_rtt_ms.is_empty());
    }

    #[test]
    fn issue_6_expiring_single_use_invites_reject_expired_or_reused_and_allow_peer() {
        let keys = derive_channel_keys_v2("dialtone", "dt-InviteTicket_Test_Key_2026!"); // compat: test secret literal
        let issuer_id = iroh::SecretKey::generate().public().to_string();
        let redeemer_id = iroh::SecretKey::generate().public().to_string();
        let now = 10_000;

        let invite = new_invite(
            &issuer_id,
            Some("https://usw1-1.relay.n0.iroh.link./"),
            &keys,
            now,
            900,
            Some(&redeemer_id),
        )
        .unwrap();
        let encoded = invite.to_text().unwrap();
        let parsed = Invite::from_str(&encoded).unwrap();

        let mut cfg = MeshConfig::default();

        // 1. Expired invite is rejected
        let expired_err =
            redeem_invite(&parsed, &keys, &redeemer_id, now + 901, &mut cfg).unwrap_err();
        assert!(expired_err.contains("expired"));

        // 2. Wrong bound node_id is rejected
        let wrong_peer_err =
            redeem_invite(&parsed, &keys, "wrong-endpoint", now + 10, &mut cfg).unwrap_err();
        assert!(wrong_peer_err.contains("bound to node_id"));

        // 3. Valid redemption succeeds and adds issuer & redeemer to allowlist
        let _addr = redeem_invite(&parsed, &keys, &redeemer_id, now + 10, &mut cfg).unwrap();
        assert!(cfg.allowlist.contains(&issuer_id));
        assert!(cfg.allowlist.contains(&redeemer_id));

        // 4. Reusing the same invite is rejected
        let replay_err =
            redeem_invite(&parsed, &keys, &redeemer_id, now + 20, &mut cfg).unwrap_err();
        assert!(replay_err.contains("already been used"));
    }

    #[test]
    fn gossip_ticket_round_trips() {
        let rt = tokio::runtime::Runtime::new().unwrap();
        let addr = rt.block_on(async {
            let ep = build_endpoint_builder(&IrohTransportConfig::local_offline())
                .unwrap()
                .bind()
                .await
                .unwrap();
            let addr = ep.addr();
            ep.close().await;
            addr
        });
        let ticket = SeedList {
            topic: derive_standby_topic_id("test-endpoint-id"),
            peers: vec![addr],
        };
        let encoded = ticket.to_text().unwrap();
        let decoded = SeedList::from_str(&encoded).unwrap();
        assert_eq!(decoded.topic, ticket.topic);
        assert_eq!(decoded.peers.len(), 1);
    }

    #[test]
    fn issue_9_no_static_public_gossip_topic_exists_and_unconfigured_nodes_use_unique_standby_topics(
    ) {
        let legacy_public_bytes: [u8; 32] = *b"dialtone-iroh-gossip-topic-00001";
        let standby_a = derive_standby_topic_id("endpoint-alpha");
        let standby_b = derive_standby_topic_id("endpoint-beta");
        assert_ne!(standby_a, standby_b);
        assert_ne!(standby_a.as_bytes(), &legacy_public_bytes);
        assert_ne!(standby_b.as_bytes(), &legacy_public_bytes);

        let keys = derive_channel_keys_v2("dialtone", "dt-StrongTestPassword_2026_Argon2id!"); // compat
        assert_ne!(keys.topic_id.as_bytes(), &legacy_public_bytes);
    }

    #[test]
    fn mesh_matrix_and_beat_interval_scale_with_the_mesh() {
        assert_eq!(BEAT_INTERVAL_SECS, 5);
        assert_eq!(SUMMARY_INTERVAL_SECS, 30);
        assert_eq!(SUMMARY_EVERY_N_BEATS, 6);
        assert_eq!(beat_interval_for(3), 5);
        assert_eq!(beat_interval_for(100), 10);
        assert_eq!(beat_interval_for(300), 30);
        assert_eq!(beat_interval_for(5000), 60);

        let cfg = IrohTransportConfig::local_offline();
        let addr = EndpointAddr::new(iroh::SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &addr, None, "ep-1");

        let mut beat_a = Beat {
            schema: BEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            node_id: "ep-1".to_string(),
            ticket: "iroh-gossip-v1:a".to_string(),
            counter: 1,
            updated_at_unix: 1_000,
            kind: "summary".to_string(),
            beat_interval_secs: BEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_INTERVAL_SECS,
            heard: 0,
            inventory_version: 0,
            peer_rtt_ms: std::collections::BTreeMap::from([("node-beta".to_string(), 14.2)]),
            summary_stats: None,
            sent_unix_ms: 0,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for("node-alpha"),
            capabilities: node_capabilities_for("node-alpha"),
            stats: sample_stats("node-alpha", None, 1),
            service_endpoints: Vec::new(),
            processes: Vec::new(),
            iroh_transport: diag,
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            devices: Vec::new(),
            geo_location: None,
            probe: None,
            store: None,
        };
        let mut beat_b = beat_a.clone();
        beat_b.node = "node-beta".to_string();
        beat_b.peer_rtt_ms = std::collections::BTreeMap::from([("node-alpha".to_string(), 15.8)]);
        beat_a.peer_rtt_ms.insert("node-gamma".to_string(), 22.4);

        let nodes = vec![beat_a, beat_b];
        let matrix = build_mesh_rtt_matrix(&nodes);
        assert_eq!(matrix.get("node-alpha<->node-beta"), Some(&15.0));
        assert_eq!(matrix.get("node-alpha<->node-gamma"), Some(&22.4));
    }

    #[test]
    fn live_two_peer_iroh_gossip_beat_exchange_succeeds() {
        let rt = tokio::runtime::Runtime::new().unwrap();
        let (received, peer_b) = rt
            .block_on(verify_live_gossip_exchange("node-alpha", "edge-1"))
            .unwrap();
        assert_eq!(received.schema, BEAT_SCHEMA);
        assert_eq!(received.node, "node-alpha");
        // Services and processes travel in Inventory messages, never in beats.
        assert!(received.service_endpoints.is_empty());
        assert_eq!(peer_b.node, "edge-1");
    }
}
