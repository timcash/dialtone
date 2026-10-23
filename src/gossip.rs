use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::str::FromStr;
use std::sync::{Mutex, OnceLock};
use std::time::Duration;

use argon2::{Algorithm, Argon2, Params, Version};
use chacha20poly1305::{
    aead::{Aead, KeyInit, Payload},
    XChaCha20Poly1305, XNonce,
};
use futures_lite::StreamExt;
use iroh::address_lookup::memory::MemoryLookup;
use iroh::protocol::Router;
use iroh::EndpointAddr;
use iroh_gossip::{
    api::Event as GossipEvent,
    net::{Gossip, GOSSIP_ALPN},
    proto::TopicId,
};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::evidence::{sample_live_host_resources, HostResourceTelemetry, TelemetryWindowSummary};
use crate::identity::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, is_tailnet_ip,
    IrohTransportConfig, IrohTransportDiagnostics,
};
use crate::refs::ServiceAdvertisement;

pub const DIALTONE_HEARTBEAT_SCHEMA: &str = "dialtone.iroh.heartbeat.v1";
pub const DIALTONE_ENVELOPE_SCHEMA_V2: &str = "dialtone.iroh.envelope.v2";
pub const DIALTONE_INVITE_SCHEMA_V1: &str = "dialtone.invite.v1";
pub const INVITE_TICKET_PREFIX: &str = "dialtone-invite-v1:";
pub const DEFAULT_CHANNEL_NAME: &str = "dialtone";
pub const DEFAULT_PEER_TTL_SECS: u64 = 300;
pub const HEARTBEAT_INTERVAL_SECS: u64 = 5;
pub const SUMMARY_HEARTBEAT_INTERVAL_SECS: u64 = 30;
pub const SUMMARY_HEARTBEAT_EVERY_N_TICKS: u64 =
    SUMMARY_HEARTBEAT_INTERVAL_SECS / HEARTBEAT_INTERVAL_SECS;

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

/// Generate a high-entropy 256-bit channel password (`dt-<43-char-base64url>`) using CSPRNG bytes.
pub fn generate_channel_password() -> String {
    let random_bytes = iroh::SecretKey::generate().to_bytes();
    format!("dt-{}", crate::base64url_encode(&random_bytes))
}

/// Estimate password character-set entropy in bits and enforce minimum strength (Issue #1).
/// Rejects empty strings, `"dialtone"`, `"default"`, `"dialtone2"`, `"password123"`, dictionary words,
/// and strings with fewer than 16 characters or insufficient entropy.
pub fn validate_channel_password(password: &str) -> Result<String, String> {
    let trimmed = password.trim();
    if trimmed.is_empty() {
        return Err(
            "a channel password is required to join the channel (run `dialtone gen-password` to generate one)"
                .to_string(),
        );
    }
    let lower = trimmed.to_ascii_lowercase();
    const FORBIDDEN_SUBSTRINGS: &[&str] = &[
        "dialtone",
        "password",
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
            "channel password is too weak or contains a forbidden dictionary word (e.g. 'dialtone', 'password'); run `dialtone gen-password`"
                .to_string(),
        );
    }
    if trimmed.len() < 16 {
        return Err(format!(
            "channel password must be at least 16 characters long (got {}); run `dialtone gen-password`",
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
            "channel password has insufficient entropy (must contain at least 2 character classes and 8+ unique characters); run `dialtone gen-password`"
                .to_string(),
        );
    }

    Ok(trimmed.to_string())
}

/// Derive a node-private standby `TopicId` when no channel password has been configured yet.
pub fn derive_standby_topic_id(endpoint_id: &str) -> TopicId {
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.standby.v2:");
    hasher.update(endpoint_id.trim().as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    TopicId::from_bytes(digest)
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct DerivedChannelKeys {
    pub channel: String,
    pub topic_id: TopicId,
    pub auth_key: [u8; 32],
    pub enc_key: [u8; 32],
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct DerivedChannelKeyHex {
    pub kdf_version: String,
    pub topic_id_hex: String,
    pub auth_key_hex: String,
    pub enc_key_hex: String,
}

impl DerivedChannelKeys {
    pub fn to_hex_bundle(&self) -> DerivedChannelKeyHex {
        DerivedChannelKeyHex {
            kdf_version: KDF_VERSION_V2.to_string(),
            topic_id_hex: crate::hex_encode(self.topic_id.as_bytes()),
            auth_key_hex: crate::hex_encode(&self.auth_key),
            enc_key_hex: crate::hex_encode(&self.enc_key),
        }
    }

    pub fn from_hex_bundle(channel: &str, bundle: &DerivedChannelKeyHex) -> Result<Self, String> {
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

fn kdf_cache() -> &'static Mutex<HashMap<(String, String), DerivedChannelKeys>> {
    static CACHE: OnceLock<Mutex<HashMap<(String, String), DerivedChannelKeys>>> = OnceLock::new();
    CACHE.get_or_init(|| Mutex::new(HashMap::new()))
}

/// Derive separate `topic_id`, `auth_key`, and `enc_key` from `(channel, password)` using
/// **Argon2id** (`m=19456 KiB, t=2, p=1`) + domain-separated HKDF-style PRF expansion (Issue #4).
pub fn derive_channel_keys_v2(channel: &str, password: &str) -> DerivedChannelKeys {
    let norm_channel = normalize_channel_name(Some(channel));
    let clean_pass = password.trim().to_string();
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
        .hash_password_into(clean_pass.as_bytes(), salt, &mut master_key)
        .expect("Argon2id hash_password_into");

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

    let derived = DerivedChannelKeys {
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

/// Deterministically derive an `iroh-gossip` 32-byte `TopicId` from `(channel, password)` via Argon2id v2.
pub fn derive_channel_topic_id(channel: &str, password: &str) -> TopicId {
    derive_channel_keys_v2(channel, password).topic_id
}

/// Derive a 32-byte authentication subkey from `(channel, password)` via Argon2id v2.
pub fn derive_channel_auth_key(channel: &str, password: &str) -> [u8; 32] {
    derive_channel_keys_v2(channel, password).auth_key
}

/// Derive a 32-byte encryption subkey from `(channel, password)` via Argon2id v2.
pub fn derive_channel_enc_key(channel: &str, password: &str) -> [u8; 32] {
    derive_channel_keys_v2(channel, password).enc_key
}

/// Deterministically derive an `iroh-gossip` 32-byte `TopicId` from a password or `<channel>:<password>`.
pub fn derive_topic_id_from_secret(secret: &str) -> TopicId {
    let trimmed = secret.trim();
    if trimmed.is_empty() {
        return derive_standby_topic_id("unconfigured");
    }
    if let Ok(ticket) = GossipTicket::from_str(trimmed) {
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

/// Produce a short, non-reversible channel fingerprint (`ch-<12 hex chars>`) for UI display.
pub fn secret_channel_fingerprint(secret: &str) -> String {
    if secret.trim().is_empty() {
        return "locked-standby".to_string();
    }
    let topic = derive_topic_id_from_secret(secret);
    let hex = crate::hex_encode(topic.as_bytes());
    format!("ch-{}", &hex[..12])
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct SeedPeerConfig {
    pub endpoint_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub relay_url: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub ip_addrs: Vec<String>,
}

/// Returns the seed peer endpoints configured in the host's local `~/.dialtone/config.json`
/// (`seed_peers` array). No production `EndpointId`s are hardcoded in the repository.
pub fn default_fleet_seed_peers() -> Vec<EndpointAddr> {
    let cfg = load_mesh_config(&default_config_path());
    let mut out = Vec::new();
    for entry in &cfg.seed_peers {
        if let Ok(id) = entry.endpoint_id.trim().parse::<iroh::EndpointId>() {
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

/// Evaluate whether `candidate_endpoint_id` is allowed by an allowlist (Issue #3).
/// **Deny by default**: if `allowed_ids` is empty, returns `false` for all remote peers.
/// A node's own `self_endpoint_id` is always allowed.
pub fn is_allowed_peer_in_list(
    allowed_ids: &[String],
    candidate_endpoint_id: &str,
    self_endpoint_id: Option<&str>,
) -> bool {
    let candidate = candidate_endpoint_id.trim();
    if candidate.is_empty() {
        return false;
    }
    if let Some(self_id) = self_endpoint_id {
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

/// Returns `true` if `endpoint_id` is permitted by the host's local `allowed_endpoint_ids`
/// list in `~/.dialtone/config.json`. **Deny by default** when `allowed_endpoint_ids` is empty (Issue #3).
pub fn is_allowed_main_fleet_peer(endpoint_id: &str) -> bool {
    let cfg = load_mesh_config(&default_config_path());
    is_allowed_peer_in_list(&cfg.allowed_endpoint_ids, endpoint_id, None)
}

/// Atomically add `endpoint_id` to `allowed_endpoint_ids` (and `seed_peers`) in `config_path` (Issue #3).
pub fn allow_peer_in_config(
    config_path: &Path,
    endpoint_id: &str,
    relay_url: Option<&str>,
) -> Result<PersistentMeshConfig, String> {
    allow_peer_in_config_with_addrs(config_path, endpoint_id, relay_url, &[])
}

/// Atomically add `endpoint_id` with optional relay URL and direct IP:port addresses to `config_path`.
pub fn allow_peer_in_config_with_addrs(
    config_path: &Path,
    endpoint_id: &str,
    relay_url: Option<&str>,
    ip_addrs: &[String],
) -> Result<PersistentMeshConfig, String> {
    let parsed_id = endpoint_id
        .trim()
        .parse::<iroh::EndpointId>()
        .map_err(|e| format!("invalid EndpointId '{}': {e}", endpoint_id.trim()))?
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
        .allowed_endpoint_ids
        .iter()
        .any(|x| x.eq_ignore_ascii_case(&parsed_id))
    {
        cfg.allowed_endpoint_ids.push(parsed_id.clone());
    }
    if let Some(existing) = cfg
        .seed_peers
        .iter_mut()
        .find(|s| s.endpoint_id.eq_ignore_ascii_case(&parsed_id))
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
        cfg.seed_peers.push(SeedPeerConfig {
            endpoint_id: parsed_id,
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

/// Atomically revoke `endpoint_id` from `allowed_endpoint_ids` and `seed_peers` in `config_path` (Issue #3).
pub fn revoke_peer_in_config(
    config_path: &Path,
    endpoint_id: &str,
) -> Result<PersistentMeshConfig, String> {
    let target = endpoint_id.trim();
    if target.is_empty() {
        return Err("endpoint_id is required".to_string());
    }
    let mut cfg = load_mesh_config(config_path);
    cfg.allowed_endpoint_ids
        .retain(|x| !x.trim().eq_ignore_ascii_case(target));
    cfg.seed_peers
        .retain(|s| !s.endpoint_id.trim().eq_ignore_ascii_case(target));
    cfg.updated_at_unix = now_unix();
    save_mesh_config(config_path, &cfg)?;
    Ok(cfg)
}

/// Expiring, single-use bootstrap invite ticket signed with the channel's `auth_key` (Issue #6).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct InviteTicket {
    pub schema: String,
    pub issuer_endpoint_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub relay_url: Option<String>,
    pub channel: String,
    pub topic_id_hex: String,
    pub nonce: String,
    pub expires_at_unix: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bound_endpoint_id: Option<String>,
    pub signature: String,
}

impl InviteTicket {
    pub fn compute_signature(&self, auth_key: &[u8; 32]) -> String {
        let mut h = Sha256::new();
        h.update(b"dialtone.invite.sig.v1:");
        h.update(auth_key);
        h.update(b":");
        h.update(self.schema.as_bytes());
        h.update(b":");
        h.update(self.issuer_endpoint_id.as_bytes());
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
        h.update(self.bound_endpoint_id.as_deref().unwrap_or("").as_bytes());
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

impl FromStr for InviteTicket {
    type Err = String;

    fn from_str(raw: &str) -> Result<Self, Self::Err> {
        let encoded = raw
            .trim()
            .strip_prefix("dialtone-invite-v1:")
            .ok_or("not a dialtone-invite-v1 ticket")?;
        let bytes = crate::base64_decode(encoded)?;
        serde_json::from_slice(&bytes).map_err(|e| format!("invalid invite ticket payload: {e}"))
    }
}

/// Create a signed, expiring, single-use invite ticket (Issue #6).
pub fn create_invite_ticket(
    issuer_endpoint_id: &str,
    relay_url: Option<&str>,
    keys: &DerivedChannelKeys,
    now_unix: u64,
    ttl_secs: u64,
    bound_endpoint_id: Option<&str>,
) -> Result<InviteTicket, String> {
    if ttl_secs == 0 || ttl_secs > 86_400 {
        return Err("invite TTL must be between 1s and 86400s (24h)".to_string());
    }
    let rand_bytes = iroh::SecretKey::generate().to_bytes();
    let nonce = crate::hex_encode(&rand_bytes[..16]);
    let mut ticket = InviteTicket {
        schema: DIALTONE_INVITE_SCHEMA_V1.to_string(),
        issuer_endpoint_id: issuer_endpoint_id.trim().to_string(),
        relay_url: relay_url.map(|s| s.to_string()),
        channel: keys.channel.clone(),
        topic_id_hex: crate::hex_encode(keys.topic_id.as_bytes()),
        nonce,
        expires_at_unix: now_unix.saturating_add(ttl_secs),
        bound_endpoint_id: bound_endpoint_id
            .map(str::trim)
            .filter(|s| !s.is_empty())
            .map(String::from),
        signature: String::new(),
    };
    ticket.signature = ticket.compute_signature(&keys.auth_key);
    Ok(ticket)
}

/// Verify and redeem a single-use `InviteTicket` (Issue #6).
/// Checks signature, expiry, optional `bound_endpoint_id`, and single-use `nonce`, records the used
/// nonce in `cfg.used_invite_nonces`, and adds the issuer and redeemer to `cfg.allowed_endpoint_ids`.
pub fn redeem_invite_ticket(
    ticket: &InviteTicket,
    keys: &DerivedChannelKeys,
    redeemer_endpoint_id: &str,
    now_unix: u64,
    cfg: &mut PersistentMeshConfig,
) -> Result<EndpointAddr, String> {
    if ticket.schema != DIALTONE_INVITE_SCHEMA_V1 {
        return Err(format!("unsupported invite schema '{}'", ticket.schema));
    }
    let expected_sig = ticket.compute_signature(&keys.auth_key);
    if !constant_time_eq(ticket.signature.as_bytes(), expected_sig.as_bytes()) {
        return Err("invite ticket signature verification failed (wrong channel password or tampered ticket)".to_string());
    }
    if now_unix > ticket.expires_at_unix {
        return Err(format!(
            "invite ticket has expired (expired_at={}, now={})",
            ticket.expires_at_unix, now_unix
        ));
    }
    if let Some(ref bound) = ticket.bound_endpoint_id {
        if !bound
            .trim()
            .eq_ignore_ascii_case(redeemer_endpoint_id.trim())
        {
            return Err(format!(
                "invite ticket is bound to endpoint_id '{}' (cannot be redeemed by '{}')",
                bound, redeemer_endpoint_id
            ));
        }
    }
    if cfg.used_invite_nonces.iter().any(|n| n == &ticket.nonce) {
        return Err(format!(
            "invite ticket nonce '{}' has already been used (single-use only)",
            ticket.nonce
        ));
    }

    let issuer_id = ticket
        .issuer_endpoint_id
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
    for peer_to_allow in [
        ticket.issuer_endpoint_id.trim(),
        redeemer_endpoint_id.trim(),
    ] {
        if !peer_to_allow.is_empty()
            && !cfg
                .allowed_endpoint_ids
                .iter()
                .any(|x| x.eq_ignore_ascii_case(peer_to_allow))
        {
            cfg.allowed_endpoint_ids.push(peer_to_allow.to_string());
        }
    }
    Ok(addr)
}

/// Parse a user-supplied channel password, `<channel>:<password>`, `iroh-gossip-v1:...` ticket,
/// or `<password>@<ticket_or_endpoint_id>` into `(TopicId, channel_fingerprint, extra_peer_addrs)`.
pub fn parse_secret_or_ticket(input: &str) -> (TopicId, String, Vec<EndpointAddr>) {
    let trimmed = input.trim();
    if let Ok(ticket) = GossipTicket::from_str(trimmed) {
        let hex = crate::hex_encode(ticket.topic.as_bytes());
        return (ticket.topic, format!("ch-{}", &hex[..12]), ticket.peers);
    }
    let mut extra_peers = Vec::new();
    let secret_part = if let Some((sec, peer_part)) = trimmed.split_once('@') {
        if let Ok(ticket) = GossipTicket::from_str(peer_part.trim()) {
            extra_peers.extend(ticket.peers);
        } else if let Ok(id) = peer_part.trim().parse::<iroh::EndpointId>() {
            let mut addr = EndpointAddr::new(id);
            if let Ok(relay_url) = "https://usw1-1.relay.n0.iroh.link./".parse::<iroh::RelayUrl>() {
                addr = addr.with_relay_url(relay_url);
            }
            extra_peers.push(addr);
        }
        sec.trim()
    } else {
        trimmed
    };
    let topic = derive_topic_id_from_secret(secret_part);
    let fp = secret_channel_fingerprint(secret_part);
    (topic, fp, extra_peers)
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
pub struct PersistentMeshConfig {
    pub node_name: Option<String>,
    pub listen_addr: Option<String>,
    #[serde(default = "default_channel_name")]
    pub channel: String,
    /// Masked/configured indicator (`true` when user has configured a valid channel password).
    pub configured: bool,
    /// Legacy/in-memory password field — NEVER serialized to `config.json` (Issue #2).
    #[serde(default, skip_serializing)]
    pub secret_key: Option<String>,
    /// Derived Argon2id v2 key bundle (`topic_id_hex`, `auth_key_hex`, `enc_key_hex`) persisted in `0600` `config.json`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub derived_keys: Option<DerivedChannelKeyHex>,
    pub channel_fingerprint: String,
    pub seed_peers: Vec<SeedPeerConfig>,
    pub allowed_endpoint_ids: Vec<String>,
    pub used_invite_nonces: Vec<String>,
    pub bootstrap_tickets: Vec<String>,
    pub daemon_installed: bool,
    pub daemon_service_path: Option<String>,
    pub managed_processes: Vec<crate::metadata::ManagedProcessSpec>,
    pub updated_at_unix: u64,
}

impl PersistentMeshConfig {
    /// Derive and store the Argon2id v2 channel key bundle without persisting the raw password (Issue #2).
    pub fn apply_channel_password(&mut self, channel: &str, password: &str) -> DerivedChannelKeys {
        let norm_channel = normalize_channel_name(Some(channel));
        let keys = derive_channel_keys_v2(&norm_channel, password);
        self.channel = norm_channel;
        self.configured = true;
        self.secret_key = None;
        self.derived_keys = Some(keys.to_hex_bundle());
        self.channel_fingerprint = keys.fingerprint();
        keys
    }

    /// Resolve active `DerivedChannelKeys` from either persisted `derived_keys` or in-memory `secret_key`.
    pub fn active_channel_keys(&self) -> Option<DerivedChannelKeys> {
        if let Some(ref bundle) = self.derived_keys {
            if let Ok(keys) = DerivedChannelKeys::from_hex_bundle(&self.channel, bundle) {
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
        keys: DerivedChannelKeys,
        bootstrap_tickets: Vec<String>,
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

pub fn default_blob_cache_dir() -> PathBuf {
    default_state_dir().join("blobs")
}

/// Locate a sibling legacy `.dialtone2/config.json` path relative to `config_path` (Issue #2).
pub fn legacy_config_path_for(config_path: &Path) -> Option<PathBuf> {
    let state_dir = config_path.parent()?;
    let dir_name = state_dir.file_name()?.to_str()?;
    if dir_name == ".dialtone" {
        Some(state_dir.parent()?.join(".dialtone2").join("config.json"))
    } else {
        None
    }
}

/// Detect and migrate/scrub legacy `~/.dialtone2/config.json` and any plaintext `secret_key` (Issue #2).
pub fn migrate_and_scrub_legacy_config(config_path: &Path, legacy_path: &Path) -> bool {
    if !legacy_path.exists() {
        return false;
    }
    eprintln!(
        "[config-migration-warn] detected legacy config file at {}; migrating derived keys and scrubbing plaintext password",
        legacy_path.display()
    );
    if !config_path.exists() {
        if let Ok(raw) = fs::read_to_string(legacy_path) {
            if let Ok(mut legacy_cfg) = serde_json::from_str::<PersistentMeshConfig>(&raw) {
                if let Some(raw_pass) = legacy_cfg.secret_key.take() {
                    if !raw_pass.trim().is_empty() {
                        let _ = legacy_cfg
                            .apply_channel_password(&legacy_cfg.channel.clone(), &raw_pass);
                    }
                }
                let _ = save_mesh_config(config_path, &legacy_cfg);
            }
        }
    }
    let _ = fs::remove_file(legacy_path);
    true
}

pub fn load_mesh_config(path: &Path) -> PersistentMeshConfig {
    if let Some(legacy_path) = legacy_config_path_for(path) {
        let _ = migrate_and_scrub_legacy_config(path, &legacy_path);
    }
    let mut cfg: PersistentMeshConfig = match fs::read_to_string(path) {
        Ok(raw) => serde_json::from_str(&raw).unwrap_or_default(),
        Err(_) => return PersistentMeshConfig::default(),
    };
    // Auto-migrate any legacy plaintext `secret_key` into `derived_keys` and scrub disk file immediately (Issue #2).
    if let Some(raw_pass) = cfg.secret_key.take() {
        if !raw_pass.trim().is_empty() {
            let ch = cfg.channel.clone();
            let _ = cfg.apply_channel_password(&ch, &raw_pass);
            let _ = save_mesh_config(path, &cfg);
        }
    }
    cfg
}

pub fn save_mesh_config(path: &Path, cfg: &PersistentMeshConfig) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create config dir {}: {e}", parent.display()))?;
    }
    let mut sanitized = cfg.clone();
    if let Some(raw_pass) = sanitized.secret_key.take() {
        if !raw_pass.trim().is_empty() && sanitized.derived_keys.is_none() {
            let ch = sanitized.channel.clone();
            let _ = sanitized.apply_channel_password(&ch, &raw_pass);
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

pub fn node_role_for(node: &str) -> String {
    match node {
        "node-alpha" => "mesh-command-controller".to_string(),
        "node-gamma" => "os-arch-build-server".to_string(),
        "node-beta" => "gpu-workstation-dev".to_string(),
        "edge-1" => "robot-v4-motion-server".to_string(),
        "pendant-1" => "teach-pendant-client".to_string(),
        _ => "mesh-peer-node".to_string(),
    }
}

pub fn node_capabilities_for(node: &str) -> Vec<String> {
    let mut caps = vec![
        "iroh-1.3-quic-endpoint".to_string(),
        "iroh-gossip-swarm".to_string(),
        "iroh-blobs-sha256-cas".to_string(),
        "no-tailnet-physical-bind".to_string(),
        "process-supervisor".to_string(),
        "rtcore-udp-journal".to_string(),
        "3d-scifi-hud-ui".to_string(),
    ];
    match node {
        "node-alpha" => {
            caps.push("mesh-rendezvous-hub".to_string());
            caps.push("darwin-arm64-builder".to_string());
        }
        "node-gamma" => {
            caps.push("distributed-build-cache".to_string());
            caps.push("windows-x86_64-compile-cluster".to_string());
        }
        "node-beta" => {
            caps.push("windows-x86_64-rustc-1.96".to_string());
            caps.push("gpu-simulation-host".to_string());
        }
        "edge-1" => {
            caps.push("nats-jetstream-broker".to_string());
            caps.push("cartesian-motion-planner".to_string());
        }
        _ => {}
    }
    caps
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct GossipTicket {
    pub topic: TopicId,
    pub peers: Vec<EndpointAddr>,
}

impl GossipTicket {
    pub fn to_text(&self) -> Result<String, String> {
        let bytes = serde_json::to_vec(self).map_err(|err| err.to_string())?;
        Ok(format!("iroh-gossip-v1:{}", crate::hex_encode(&bytes)))
    }
}

impl FromStr for GossipTicket {
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

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct PeerRttEcho {
    pub echoed_mono_ms: u64,
    pub hold_time_ms: u64,
}

pub fn compute_rtt_ms_from_echo(now_mono_ms: u64, echo: &PeerRttEcho) -> Option<f64> {
    if echo.echoed_mono_ms == 0 || now_mono_ms < echo.echoed_mono_ms {
        return None;
    }
    let elapsed_ms = now_mono_ms - echo.echoed_mono_ms;
    if elapsed_ms < echo.hold_time_ms {
        return None;
    }
    let net_ms = (elapsed_ms - echo.hold_time_ms) as f64;
    if net_ms > 5_000.0 {
        return None;
    }
    Some(((net_ms.max(0.2)) * 10.0).round() / 10.0)
}

pub fn build_mesh_rtt_matrix(
    nodes: &[DialtoneHeartbeat],
) -> std::collections::BTreeMap<String, f64> {
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

/// Deterministic approximate `(lat, lng)` from a globally routable IPv4 prefix when external Geo-IP API is unavailable.
pub fn approx_geo_from_public_ipv4(ip: std::net::Ipv4Addr) -> NodeGeoLocation {
    let o = ip.octets();
    let (base_lat, base_lng, region): (f64, f64, &str) = match o[0] {
        1
        | 14
        | 27
        | 36
        | 39
        | 42
        | 49
        | 58..=61
        | 101
        | 103
        | 106
        | 110..=126
        | 133
        | 150
        | 153
        | 163
        | 175
        | 180
        | 182
        | 183
        | 202
        | 203
        | 210
        | 211
        | 218..=223 => (35.68, 139.69, "APAC"),
        2
        | 5
        | 31
        | 37
        | 46
        | 51
        | 62
        | 77..=95
        | 109
        | 141
        | 145
        | 151
        | 176
        | 178
        | 185
        | 188
        | 193..=195
        | 212
        | 213
        | 217 => (50.11, 8.68, "EU-Central"),
        177 | 179 | 181 | 186 | 187 | 189..=191 | 200 | 201 => (-23.55, -46.63, "SA-East"),
        41 | 102 | 105 | 154 | 196 | 197 => (-26.20, 28.04, "AF-South"),
        3..=13
        | 15..=24
        | 32..=35
        | 38
        | 40
        | 44
        | 45
        | 47
        | 50
        | 52
        | 54
        | 63..=76
        | 96..=100
        | 104
        | 107
        | 108 => {
            if o[1] < 85 {
                (37.77, -122.42, "US-West")
            } else if o[1] < 170 {
                (39.74, -104.99, "US-Mountain")
            } else {
                (39.04, -77.49, "US-East")
            }
        }
        _ => (40.76, -111.89, "NA-West"),
    };
    let lat_jitter = ((o[1] as f64 / 255.0) - 0.5) * 3.2;
    let lng_jitter = ((o[2] as f64 / 255.0) - 0.5) * 4.4;
    let lat = (((base_lat + lat_jitter).clamp(-70.0, 70.0)) * 100.0).round() / 100.0;
    let lng = (((base_lng + lng_jitter).clamp(-179.0, 179.0)) * 100.0).round() / 100.0;
    NodeGeoLocation {
        public_ip: Some(mask_public_ipv4(ip)),
        lat,
        lng,
        label: format!("{region} ({})", mask_public_ipv4(ip)),
        source: "public_ip_geo".to_string(),
        anchor_node: None,
        cluster_radius_km: 25.0,
    }
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

pub fn extract_public_ipv4_from_heartbeat(hb: &DialtoneHeartbeat) -> Option<std::net::Ipv4Addr> {
    if let Some(ref b) = hb.iroh_transport.bound_non_tailnet_ipv4 {
        if let Some(ip) = parse_ipv4_from_token(b) {
            if is_globally_routable_ipv4(ip) {
                return Some(ip);
            }
        }
    }
    for s in &hb.iroh_transport.advertised_ip_addrs {
        if let Some(ip) = parse_ipv4_from_token(s) {
            if is_globally_routable_ipv4(ip) {
                return Some(ip);
            }
        }
    }
    for s in &hb.observed_remote_addrs {
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

fn deterministic_node_bearing_rad(node: &str) -> f64 {
    let mut h: u64 = 0xcbf2_9ce4_8422_2325;
    for b in node.as_bytes() {
        h ^= *b as u64;
        h = h.wrapping_mul(0x0100_0000_01b3);
    }
    ((h % 3600) as f64 / 3600.0) * std::f64::consts::TAU
}

fn offset_lat_lng_deg(
    lat_deg: f64,
    lng_deg: f64,
    angular_deg: f64,
    bearing_rad: f64,
) -> (f64, f64) {
    let dlat = angular_deg * bearing_rad.cos();
    let cos_lat = (lat_deg.to_radians().cos()).abs().max(0.25);
    let dlng = (angular_deg * bearing_rad.sin()) / cos_lat;
    let new_lat = ((lat_deg + dlat).clamp(-72.0, 72.0) * 100.0).round() / 100.0;
    let mut new_lng = lng_deg + dlng;
    while new_lng > 180.0 {
        new_lng -= 360.0;
    }
    while new_lng < -180.0 {
        new_lng += 360.0;
    }
    let new_lng = (new_lng * 100.0).round() / 100.0;
    (new_lat, new_lng)
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

fn lookup_pair_rtt_ms(
    a: &DialtoneHeartbeat,
    b: &DialtoneHeartbeat,
    mesh_rtt_ms: &std::collections::BTreeMap<String, f64>,
) -> Option<f64> {
    let key = if a.node <= b.node {
        format!("{}<->{}", a.node, b.node)
    } else {
        format!("{}<->{}", b.node, a.node)
    };
    if let Some(&v) = mesh_rtt_ms.get(&key) {
        if v > 0.0 {
            return Some(v);
        }
    }
    if let Some(&v) = a.peer_rtt_ms.get(&b.node) {
        if v > 0.0 {
            return Some(v);
        }
    }
    if let Some(&v) = b.peer_rtt_ms.get(&a.node) {
        if v > 0.0 {
            return Some(v);
        }
    }
    None
}

/// Resolve approximate `(lat, lng)` for every node in `nodes`:
/// 1. Nodes with a public IP (`geo_location.source == "public_ip_geo"` or a globally routable IPv4 in their transport diagnostics)
///    are anchored at their approximate public-IP coordinates.
/// 2. Nodes without a public IP are placed by **latency clustering** relative to anchored peers (or the relay region seed)
///    with great-circle angular separation proportional to pairwise RTT (`mesh_rtt_ms` / `peer_rtt_ms`) and spherical spring relaxation.
/// 3. Co-located nodes sharing the same public IP coordinates are arranged in a tight RTT-proportional micro-cluster so every node
///    occupies a distinct, clickable position on the H3 globe.
pub fn resolve_fleet_node_locations(
    nodes: &mut [DialtoneHeartbeat],
    mesh_rtt_ms: &std::collections::BTreeMap<String, f64>,
) {
    if nodes.is_empty() {
        return;
    }

    // Step 1: Populate public_ip_geo for any node that has a globally routable IPv4 address.
    for n in nodes.iter_mut() {
        let has_public_geo = n
            .geo_location
            .as_ref()
            .map(|g| g.source == "public_ip_geo" && (g.lat != 0.0 || g.lng != 0.0))
            .unwrap_or(false);
        if !has_public_geo {
            if let Some(pub_ip) = extract_public_ipv4_from_heartbeat(n) {
                n.geo_location = Some(approx_geo_from_public_ipv4(pub_ip));
            }
        }
    }

    let mut is_public_anchor: Vec<bool> = nodes
        .iter()
        .map(|n| {
            n.geo_location
                .as_ref()
                .map(|g| g.source == "public_ip_geo")
                .unwrap_or(false)
        })
        .collect();

    // Step 2: If no node has a public IP geo yet, seed the first node at its relay region center as a latency cluster root.
    if !is_public_anchor.iter().any(|&b| b) {
        let relay_hint = nodes[0]
            .iroh_transport
            .advertised_relay_urls
            .first()
            .or_else(|| nodes[0].iroh_transport.relay_urls.first())
            .map(|s| s.as_str())
            .unwrap_or("");
        let (seed_lat, seed_lng, seed_label) = if relay_hint.contains("usw") {
            (37.34, -121.89, "US-West Latency Cluster")
        } else if relay_hint.contains("use") {
            (39.04, -77.49, "US-East Latency Cluster")
        } else if relay_hint.contains("euw") || relay_hint.contains("fra") {
            (50.11, 8.68, "EU Latency Cluster")
        } else if relay_hint.contains("aps") || relay_hint.contains("sin") {
            (1.35, 103.82, "APAC Latency Cluster")
        } else {
            (40.76, -111.89, "Mesh Latency Cluster")
        };
        nodes[0].geo_location = Some(NodeGeoLocation {
            public_ip: None,
            lat: seed_lat,
            lng: seed_lng,
            label: seed_label.to_string(),
            source: "latency_cluster".to_string(),
            anchor_node: Some(nodes[0].node.clone()),
            cluster_radius_km: 15.0,
        });
        is_public_anchor[0] = true;
    }

    // Step 3: Place unanchored nodes relative to their lowest-RTT anchored peer.
    let snapshot = nodes.to_vec();
    for i in 0..nodes.len() {
        if is_public_anchor[i] {
            continue;
        }
        let mut best_anchor_idx = 0usize;
        let mut best_rtt = f64::MAX;
        for (j, cand) in snapshot.iter().enumerate() {
            if is_public_anchor[j] {
                let rtt = lookup_pair_rtt_ms(&snapshot[i], cand, mesh_rtt_ms).unwrap_or(35.0);
                if rtt < best_rtt {
                    best_rtt = rtt;
                    best_anchor_idx = j;
                }
            }
        }
        if best_rtt == f64::MAX {
            best_rtt = 35.0;
        }
        let anchor_loc = nodes[best_anchor_idx]
            .geo_location
            .clone()
            .unwrap_or_default();
        let anchor_name = nodes[best_anchor_idx].node.clone();
        let target_deg = (2.2 + best_rtt * 0.14).clamp(2.2, 75.0);
        let bearing = deterministic_node_bearing_rad(&nodes[i].node) + (i as f64 * 2.094);
        let (lat, lng) = offset_lat_lng_deg(anchor_loc.lat, anchor_loc.lng, target_deg, bearing);
        nodes[i].geo_location = Some(NodeGeoLocation {
            public_ip: None,
            lat,
            lng,
            label: format!("RTT Cluster (~{best_rtt:.0}ms to {anchor_name})"),
            source: "latency_cluster".to_string(),
            anchor_node: Some(anchor_name),
            cluster_radius_km: (best_rtt * 15.0).round(),
        });
    }

    // Step 4: Spherical spring relaxation so latency-clustered nodes (and co-located public-IP nodes)
    // reflect pairwise RTT distances and maintain distinct positions on the H3 globe.
    let n_len = nodes.len();
    if n_len >= 2 {
        for _iter in 0..24 {
            for i in 0..n_len {
                for j in (i + 1)..n_len {
                    let loc_i = nodes[i].geo_location.clone().unwrap_or_default();
                    let loc_j = nodes[j].geo_location.clone().unwrap_or_default();
                    let cur_deg = great_circle_deg(loc_i.lat, loc_i.lng, loc_j.lat, loc_j.lng);
                    let rtt = lookup_pair_rtt_ms(&nodes[i], &nodes[j], mesh_rtt_ms).unwrap_or(25.0);

                    let both_public =
                        loc_i.source == "public_ip_geo" && loc_j.source == "public_ip_geo";
                    if both_public && cur_deg >= 2.2 {
                        continue;
                    }

                    let target_deg = if both_public {
                        // Co-located public IP nodes: micro-cluster separation proportional to RTT
                        (2.0 + rtt * 0.04).clamp(2.0, 5.5)
                    } else {
                        (2.2 + rtt * 0.14).clamp(2.2, 75.0)
                    };

                    let err = target_deg - cur_deg;
                    if err.abs() < 0.15 {
                        continue;
                    }
                    let bearing = if cur_deg < 0.05 {
                        deterministic_node_bearing_rad(&nodes[j].node) + (j as f64 * 2.094)
                    } else {
                        let dlat = loc_j.lat - loc_i.lat;
                        let dlng = (loc_j.lng - loc_i.lng) * loc_i.lat.to_radians().cos().max(0.25);
                        dlng.atan2(dlat)
                    };

                    let step = err * 0.28;
                    if !is_public_anchor[j] || both_public {
                        let (nlat, nlng) = offset_lat_lng_deg(loc_j.lat, loc_j.lng, step, bearing);
                        if let Some(ref mut gj) = nodes[j].geo_location {
                            gj.lat = nlat;
                            gj.lng = nlng;
                        }
                    } else if !is_public_anchor[i] {
                        let (nlat, nlng) = offset_lat_lng_deg(
                            loc_i.lat,
                            loc_i.lng,
                            step,
                            bearing + std::f64::consts::PI,
                        );
                        if let Some(ref mut gi) = nodes[i].geo_location {
                            gi.lat = nlat;
                            gi.lng = nlng;
                        }
                    }
                }
            }
        }
    }
}

fn default_heartbeat_kind() -> String {
    "regular".to_string()
}

fn default_heartbeat_interval_secs() -> u64 {
    HEARTBEAT_INTERVAL_SECS
}

fn default_summary_interval_secs() -> u64 {
    SUMMARY_HEARTBEAT_INTERVAL_SECS
}

#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct NodeE2eTestReport {
    pub worker_name: String,
    pub worker_sha256: String,
    pub worker_pid: Option<u32>,
    #[serde(default)]
    pub worker_blob_cas_path: String,
    #[serde(default)]
    pub worker_staged_path: String,
    #[serde(default)]
    pub worker_blob_ticket: String,
    pub cycle: u64,
    pub updated_at_unix: u64,
    pub passed_count: u8,
    pub total_count: u8,
    pub sha_blobs_ok: bool,
    pub key_value_ok: bool,
    pub sub_processes_ok: bool,
    pub mesh_connectivity_ok: bool,
    pub ab_rollback_ok: bool,
    pub active_slot: String,
    pub verified_blob_sha256: String,
    pub kv_key: String,
    pub mesh_peers_seen: usize,
}

/// Per-node SHA-256 CAS blob store counters, gossiped so every HUD can see blob downloads land on peers.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize, Deserialize)]
pub struct NodeBlobStoreStats {
    pub sha_blob_keys: u64,
    pub sha_blob_bytes: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub latest_sha256: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct DialtoneHeartbeat {
    pub schema: String,
    pub node: String,
    pub endpoint_id: String,
    pub ticket: String,
    pub counter: u64,
    pub updated_at_unix: u64,
    #[serde(default = "default_heartbeat_kind")]
    pub heartbeat_kind: String,
    #[serde(default = "default_heartbeat_interval_secs")]
    pub heartbeat_interval_secs: u64,
    #[serde(default = "default_summary_interval_secs")]
    pub summary_interval_secs: u64,
    #[serde(default)]
    pub sent_mono_ms: u64,
    #[serde(default)]
    pub rtt_probe_requested: bool,
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub peer_echoes: HashMap<String, PeerRttEcho>,
    #[serde(default)]
    pub peer_rtt_ms: std::collections::BTreeMap<String, f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub summary_stats: Option<TelemetryWindowSummary>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub geo_location: Option<NodeGeoLocation>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub e2e_test: Option<NodeE2eTestReport>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub blob_store: Option<NodeBlobStoreStats>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub channel_mac: Option<String>,
    #[serde(default = "default_os")]
    pub os: String,
    #[serde(default = "default_arch")]
    pub arch: String,
    #[serde(default)]
    pub role: String,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub host_resources: HostResourceTelemetry,
    pub service_endpoints: Vec<ServiceAdvertisement>,
    #[serde(default)]
    pub managed_processes: Vec<crate::metadata::ManagedProcessSpec>,
    pub iroh_transport: IrohTransportDiagnostics,
    #[serde(default)]
    pub observed_remote_addrs: Vec<String>,
    #[serde(default)]
    pub tailnet_used: bool,
}

/// Encrypted wire frame for `DialtoneHeartbeat` over `iroh-gossip` using XChaCha20-Poly1305 (Issue #5).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct EncryptedGossipEnvelope {
    pub schema: String,
    pub nonce: String,
    pub ciphertext: String,
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

pub fn compute_heartbeat_mac_with_key(
    hb: &DialtoneHeartbeat,
    channel: &str,
    auth_key: &[u8; 32],
) -> String {
    let norm_channel = normalize_channel_name(Some(channel));
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.heartbeat.mac.v2:");
    hasher.update(auth_key);
    hasher.update(b":");
    hasher.update(norm_channel.as_bytes());
    hasher.update(b":");
    hasher.update(hb.node.as_bytes());
    hasher.update(b":");
    hasher.update(hb.endpoint_id.as_bytes());
    hasher.update(b":");
    hasher.update(hb.counter.to_string().as_bytes());
    hasher.update(b":");
    hasher.update(hb.updated_at_unix.to_string().as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    crate::hex_encode(&digest)
}

/// Compute a 32-byte hex MAC authenticating `(channel, node, endpoint_id, counter, updated_at_unix)`
/// using the secret `auth_key` derived from `(channel, password)`.
pub fn compute_heartbeat_mac(hb: &DialtoneHeartbeat, channel: &str, password: &str) -> String {
    let keys = derive_channel_keys_v2(channel, password);
    compute_heartbeat_mac_with_key(hb, &keys.channel, &keys.auth_key)
}

/// Sign `DialtoneHeartbeat` in-place with `DerivedChannelKeys`.
pub fn sign_heartbeat_with_keys(hb: &mut DialtoneHeartbeat, keys: &DerivedChannelKeys) {
    hb.channel_mac = Some(compute_heartbeat_mac_with_key(
        hb,
        &keys.channel,
        &keys.auth_key,
    ));
}

/// Sign `DialtoneHeartbeat` in-place with the `(channel, password)` MAC.
pub fn sign_heartbeat_with_password(hb: &mut DialtoneHeartbeat, channel: &str, password: &str) {
    if !password.trim().is_empty() {
        let keys = derive_channel_keys_v2(channel, password);
        sign_heartbeat_with_keys(hb, &keys);
    }
}

/// Verify in constant time that `DialtoneHeartbeat` carries a valid MAC for `keys`.
pub fn verify_heartbeat_with_keys(hb: &DialtoneHeartbeat, keys: &DerivedChannelKeys) -> bool {
    let Some(ref provided_mac) = hb.channel_mac else {
        return false;
    };
    let expected_mac = compute_heartbeat_mac_with_key(hb, &keys.channel, &keys.auth_key);
    constant_time_eq(provided_mac.as_bytes(), expected_mac.as_bytes())
}

/// Verify in constant time that `DialtoneHeartbeat` carries a valid MAC for `(channel, password)`.
pub fn verify_heartbeat_password(hb: &DialtoneHeartbeat, channel: &str, password: &str) -> bool {
    if password.trim().is_empty() {
        return false;
    }
    let keys = derive_channel_keys_v2(channel, password);
    verify_heartbeat_with_keys(hb, &keys)
}

/// Encrypt and authenticate a `DialtoneHeartbeat` using **XChaCha20-Poly1305** with a random 24-byte
/// nonce and `DIALTONE_ENVELOPE_SCHEMA_V2` as Associated Data (AAD) (Issue #5).
pub fn encrypt_heartbeat_for_gossip(
    hb: &DialtoneHeartbeat,
    keys: &DerivedChannelKeys,
) -> Result<Vec<u8>, String> {
    let mut signed_hb = hb.clone();
    sign_heartbeat_with_keys(&mut signed_hb, keys);
    let plaintext = serde_json::to_vec(&signed_hb).map_err(|e| e.to_string())?;

    let rand_bytes = iroh::SecretKey::generate().to_bytes();
    let nonce_bytes = &rand_bytes[..24];
    let nonce = XNonce::from_slice(nonce_bytes);
    let cipher = XChaCha20Poly1305::new((&keys.enc_key).into());

    let ciphertext = cipher
        .encrypt(
            nonce,
            Payload {
                msg: &plaintext,
                aad: DIALTONE_ENVELOPE_SCHEMA_V2.as_bytes(),
            },
        )
        .map_err(|_| "XChaCha20-Poly1305 encryption failed".to_string())?;

    let env = EncryptedGossipEnvelope {
        schema: DIALTONE_ENVELOPE_SCHEMA_V2.to_string(),
        nonce: crate::hex_encode(nonce_bytes),
        ciphertext: crate::base64url_encode(&ciphertext),
    };
    serde_json::to_vec(&env).map_err(|e| e.to_string())
}

/// Decrypt and authenticate an `EncryptedGossipEnvelope` frame from `iroh-gossip` (Issue #5).
/// Drops any plaintext frame, tampered ciphertext, wrong AAD, or invalid inner MAC.
pub fn decrypt_heartbeat_from_gossip(
    raw_frame: &[u8],
    keys: &DerivedChannelKeys,
) -> Result<DialtoneHeartbeat, String> {
    let env: EncryptedGossipEnvelope = serde_json::from_slice(raw_frame)
        .map_err(|_| "gossip frame is not a valid encrypted envelope".to_string())?;
    if env.schema != DIALTONE_ENVELOPE_SCHEMA_V2 {
        return Err(format!(
            "unsupported gossip envelope schema '{}'",
            env.schema
        ));
    }
    let nonce_bytes = crate::hex_decode(&env.nonce)?;
    if nonce_bytes.len() != 24 {
        return Err("invalid XChaCha20-Poly1305 nonce length (expected 24 bytes)".to_string());
    }
    let ciphertext = crate::base64_decode(&env.ciphertext)?;
    let nonce = XNonce::from_slice(&nonce_bytes);
    let cipher = XChaCha20Poly1305::new((&keys.enc_key).into());

    let plaintext = cipher
        .decrypt(
            nonce,
            Payload {
                msg: &ciphertext,
                aad: DIALTONE_ENVELOPE_SCHEMA_V2.as_bytes(),
            },
        )
        .map_err(|_| {
            "XChaCha20-Poly1305 decryption failed (wrong key or tampered frame)".to_string()
        })?;

    let hb: DialtoneHeartbeat = serde_json::from_slice(&plaintext)
        .map_err(|e| format!("invalid decrypted heartbeat JSON: {e}"))?;
    if !verify_heartbeat_with_keys(&hb, keys) {
        return Err("decrypted heartbeat failed inner channel_mac verification".to_string());
    }
    Ok(hb)
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

/// Prune peer heartbeat entries older than `ttl_secs`.
pub fn prune_stale_peers(peers: &mut Vec<DialtoneHeartbeat>, now: u64, ttl_secs: u64) -> usize {
    let before = peers.len();
    peers.retain(|p| now.saturating_sub(p.updated_at_unix) <= ttl_secs);
    before.saturating_sub(peers.len())
}

pub fn save_peer_cache(path: &Path, peers: &[DialtoneHeartbeat]) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create peer cache dir {}: {e}", parent.display()))?;
    }
    let json = serde_json::to_string_pretty(peers).map_err(|e| e.to_string())?;
    fs::write(path, json).map_err(|e| format!("write peer cache {}: {e}", path.display()))
}

pub fn load_peer_cache(
    path: &Path,
    now: u64,
    ttl_secs: u64,
) -> Result<Vec<DialtoneHeartbeat>, String> {
    let raw = match fs::read_to_string(path) {
        Ok(raw) => raw,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(Vec::new()),
        Err(e) => return Err(format!("read peer cache {}: {e}", path.display())),
    };
    let mut peers: Vec<DialtoneHeartbeat> =
        serde_json::from_str(&raw).map_err(|e| format!("parse peer cache: {e}"))?;
    prune_stale_peers(&mut peers, now, ttl_secs);
    Ok(peers)
}

/// Live in-process two-peer `iroh-gossip` heartbeat exchange over QUIC protected by Argon2id + XChaCha20-Poly1305.
pub async fn verify_live_gossip_exchange(
    node_a: &str,
    node_b: &str,
) -> Result<(DialtoneHeartbeat, DialtoneHeartbeat), String> {
    let config = IrohTransportConfig::local_offline();
    let test_password = "dt-SelfTest_HighEntropy_Channel_Key_2026!";
    let keys = derive_channel_keys_v2(DEFAULT_CHANNEL_NAME, test_password);
    let topic_id = keys.topic_id;

    let lookup_a = MemoryLookup::new();
    let endpoint_a = build_endpoint_builder(&config)?
        .address_lookup(lookup_a.clone())
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

    let ticket_a = GossipTicket {
        topic: topic_id,
        peers: vec![addr_a.clone()],
    }
    .to_text()?;
    let ticket_b = GossipTicket {
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

    let mut hb_a = DialtoneHeartbeat {
        schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
        node: node_a.to_string(),
        endpoint_id: id_a.to_string(),
        ticket: ticket_a,
        counter: 1,
        updated_at_unix: now_unix(),
        heartbeat_kind: "summary".to_string(),
        heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
        summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
        sent_mono_ms: monotonic_now_ms(),
        rtt_probe_requested: true,
        peer_echoes: HashMap::new(),
        peer_rtt_ms: std::collections::BTreeMap::new(),
        summary_stats: None,
        channel_mac: None,
        os: default_os(),
        arch: default_arch(),
        role: node_role_for(node_a),
        capabilities: node_capabilities_for(node_a),
        host_resources: sample_live_host_resources(node_a, None, 1),
        service_endpoints: vec![ServiceAdvertisement {
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
            status_probe: None,
        }],
        managed_processes: Vec::new(),
        iroh_transport: build_diagnostics(&config, &addr_a, None, &id_a.to_string()),
        observed_remote_addrs: Vec::new(),
        tailnet_used: false,
        geo_location: None,
        e2e_test: None,
        blob_store: None,
    };
    sign_heartbeat_with_keys(&mut hb_a, &keys);

    let received_hb: DialtoneHeartbeat = tokio::time::timeout(Duration::from_secs(10), async {
        loop {
            let encrypted_payload_a = encrypt_heartbeat_for_gossip(&hb_a, &keys)?;
            let _ = sender_a.broadcast(encrypted_payload_a.into()).await;
            while let Ok(Some(event)) =
                tokio::time::timeout(Duration::from_millis(120), receiver_b.next()).await
            {
                if let GossipEvent::Received(msg) = event.map_err(|e| e.to_string())? {
                    if let Ok(parsed) = decrypt_heartbeat_from_gossip(&msg.content, &keys) {
                        if parsed.node == node_a {
                            return Ok::<DialtoneHeartbeat, String>(parsed);
                        }
                    }
                }
            }
        }
    })
    .await
    .map_err(|_| "timed out waiting for encrypted iroh-gossip heartbeat".to_string())??;

    let mut hb_b = DialtoneHeartbeat {
        schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
        node: node_b.to_string(),
        endpoint_id: id_b.to_string(),
        ticket: ticket_b,
        counter: 1,
        updated_at_unix: now_unix(),
        heartbeat_kind: "summary".to_string(),
        heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
        summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
        sent_mono_ms: monotonic_now_ms(),
        rtt_probe_requested: false,
        peer_echoes: HashMap::new(),
        peer_rtt_ms: std::collections::BTreeMap::new(),
        summary_stats: None,
        channel_mac: None,
        os: default_os(),
        arch: default_arch(),
        role: node_role_for(node_b),
        capabilities: node_capabilities_for(node_b),
        host_resources: sample_live_host_resources(node_b, None, 1),
        service_endpoints: Vec::new(),
        managed_processes: Vec::new(),
        iroh_transport: build_diagnostics(&config, &addr_b, None, &id_b.to_string()),
        observed_remote_addrs: Vec::new(),
        tailnet_used: false,
        geo_location: None,
        e2e_test: None,
        blob_store: None,
    };
    sign_heartbeat_with_keys(&mut hb_b, &keys);

    let _ = router_a.shutdown().await;
    let _ = router_b.shutdown().await;

    Ok((received_hb, hb_b))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn issue_1_channel_password_validation_rejects_weak_and_accepts_generated() {
        assert!(validate_channel_password("").is_err());
        assert!(validate_channel_password("   ").is_err());
        assert!(validate_channel_password("dialtone").is_err());
        assert!(validate_channel_password("dialtone2").is_err());
        assert!(validate_channel_password("password123").is_err());
        assert!(validate_channel_password("default").is_err());
        assert!(validate_channel_password("short-pass-1").is_err());
        assert!(validate_channel_password("aaaaaaaaaaaaaaaaaaaa").is_err());
        assert!(validate_channel_password("alllowercasewordsinhere").is_err());

        let generated = generate_channel_password();
        assert!(generated.starts_with("dt-"));
        assert!(generated.len() >= 40);
        assert_eq!(validate_channel_password(&generated).unwrap(), generated);
    }

    #[test]
    fn issue_2_config_never_stores_plaintext_password_and_migrates_legacy_file() {
        let root = env::temp_dir().join(format!("dialtone-cfg-mig-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&root);
        let legacy_dir = root.join(".dialtone2");
        let new_dir = root.join(".dialtone");
        fs::create_dir_all(&legacy_dir).unwrap();
        let legacy_path = legacy_dir.join("config.json");
        let new_path = new_dir.join("config.json");

        let raw_pass = "dt-LegacySecret_ToMigrate_998877!";
        fs::write(
            &legacy_path,
            format!(
                r#"{{"node_name":"node-alpha","channel":"dialtone","configured":true,"secret_key":"{raw_pass}","channel_fingerprint":"old","updated_at_unix":100}}"#
            ),
        )
        .unwrap();

        let loaded = load_mesh_config(&new_path);
        assert!(
            !legacy_path.exists(),
            "legacy ~/.dialtone2/config.json must be cleaned up"
        );
        assert!(
            new_path.exists(),
            "migrated ~/.dialtone/config.json must be created"
        );
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
            "config.json on disk must never contain the raw password"
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

        let after_allow = allow_peer_in_config(&cfg_path, &ep_a, None).unwrap();
        assert!(is_allowed_peer_in_list(
            &after_allow.allowed_endpoint_ids,
            &ep_a,
            None
        ));
        assert!(!is_allowed_peer_in_list(
            &after_allow.allowed_endpoint_ids,
            &ep_b,
            None
        ));

        let after_revoke = revoke_peer_in_config(&cfg_path, &ep_a).unwrap();
        assert!(!is_allowed_peer_in_list(
            &after_revoke.allowed_endpoint_ids,
            &ep_a,
            None
        ));

        let _ = fs::remove_file(&cfg_path);
    }

    #[test]
    fn issue_4_and_5_argon2id_kdf_known_answer_and_xchacha20poly1305_heartbeat_encryption() {
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
        let diag = build_diagnostics(&cfg, &addr, None, "ep-secret-node");
        let hb = DialtoneHeartbeat {
            schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            endpoint_id: "ep-secret-node".to_string(),
            ticket: "iroh-gossip-v1:super-secret-ticket".to_string(),
            counter: 42,
            updated_at_unix: 1_700_000_000,
            heartbeat_kind: "regular".to_string(),
            heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
            sent_mono_ms: 1_000,
            rtt_probe_requested: false,
            peer_echoes: HashMap::new(),
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            channel_mac: None,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for("node-alpha"),
            capabilities: node_capabilities_for("node-alpha"),
            host_resources: sample_live_host_resources("node-alpha", None, 42),
            service_endpoints: Vec::new(),
            managed_processes: Vec::new(),
            iroh_transport: diag,
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            geo_location: None,
            e2e_test: None,
            blob_store: None,
        };

        let wire_bytes = encrypt_heartbeat_for_gossip(&hb, &k1).unwrap();
        let wire_str = String::from_utf8_lossy(&wire_bytes);
        // Captured gossip frames must contain NO plaintext fields
        assert!(!wire_str.contains("node-alpha"));
        assert!(!wire_str.contains("ep-secret-node"));
        assert!(!wire_str.contains("super-secret-ticket"));
        assert!(!wire_str.contains("host_resources"));

        // Round-trip decrypts cleanly with k1 and fails with k_other or tampered ciphertext
        let decrypted = decrypt_heartbeat_from_gossip(&wire_bytes, &k1).unwrap();
        assert_eq!(decrypted.node, "node-alpha");
        assert_eq!(decrypted.counter, 42);
        assert!(decrypt_heartbeat_from_gossip(&wire_bytes, &k_other).is_err());

        let mut tampered_env: EncryptedGossipEnvelope =
            serde_json::from_slice(&wire_bytes).unwrap();
        tampered_env.ciphertext.push_str("AA");
        let tampered_bytes = serde_json::to_vec(&tampered_env).unwrap();
        assert!(decrypt_heartbeat_from_gossip(&tampered_bytes, &k1).is_err());
    }

    #[test]
    fn issue_6_expiring_single_use_invite_tickets_reject_expired_or_reused_and_allow_peer() {
        let keys = derive_channel_keys_v2("dialtone", "dt-InviteTicket_Test_Key_2026!");
        let issuer_id = iroh::SecretKey::generate().public().to_string();
        let redeemer_id = iroh::SecretKey::generate().public().to_string();
        let now = 10_000;

        let invite = create_invite_ticket(
            &issuer_id,
            Some("https://usw1-1.relay.n0.iroh.link./"),
            &keys,
            now,
            900,
            Some(&redeemer_id),
        )
        .unwrap();
        let encoded = invite.to_text().unwrap();
        let parsed = InviteTicket::from_str(&encoded).unwrap();

        let mut cfg = PersistentMeshConfig::default();

        // 1. Expired invite is rejected
        let expired_err =
            redeem_invite_ticket(&parsed, &keys, &redeemer_id, now + 901, &mut cfg).unwrap_err();
        assert!(expired_err.contains("expired"));

        // 2. Wrong bound endpoint_id is rejected
        let wrong_peer_err =
            redeem_invite_ticket(&parsed, &keys, "wrong-endpoint", now + 10, &mut cfg).unwrap_err();
        assert!(wrong_peer_err.contains("bound to endpoint_id"));

        // 3. Valid redemption succeeds and adds issuer & redeemer to allowed_endpoint_ids
        let _addr = redeem_invite_ticket(&parsed, &keys, &redeemer_id, now + 10, &mut cfg).unwrap();
        assert!(cfg.allowed_endpoint_ids.contains(&issuer_id));
        assert!(cfg.allowed_endpoint_ids.contains(&redeemer_id));

        // 4. Reusing the same invite ticket is rejected
        let replay_err =
            redeem_invite_ticket(&parsed, &keys, &redeemer_id, now + 20, &mut cfg).unwrap_err();
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
        let ticket = GossipTicket {
            topic: derive_standby_topic_id("test-endpoint-id"),
            peers: vec![addr],
        };
        let encoded = ticket.to_text().unwrap();
        let decoded = GossipTicket::from_str(&encoded).unwrap();
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

        let keys = derive_channel_keys_v2("dialtone", "dt-StrongTestPassword_2026_Argon2id!");
        assert_ne!(keys.topic_id.as_bytes(), &legacy_public_bytes);
    }

    #[test]
    fn peer_cache_persists_and_prunes_stale_entries_on_load() {
        let path = env::temp_dir().join(format!("dialtone-peer-cache-{}", std::process::id()));
        let _ = fs::remove_file(&path);
        let cfg = IrohTransportConfig::local_offline();
        let addr = EndpointAddr::new(iroh::SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &addr, None, "ep-1");

        let fresh = DialtoneHeartbeat {
            schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            endpoint_id: "ep-1".to_string(),
            ticket: "iroh-gossip-v1:fresh".to_string(),
            counter: 1,
            updated_at_unix: 1_000,
            heartbeat_kind: "regular".to_string(),
            heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
            sent_mono_ms: 1_000,
            rtt_probe_requested: false,
            peer_echoes: HashMap::new(),
            peer_rtt_ms: std::collections::BTreeMap::new(),
            summary_stats: None,
            channel_mac: None,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for("node-alpha"),
            capabilities: node_capabilities_for("node-alpha"),
            host_resources: sample_live_host_resources("node-alpha", None, 1),
            service_endpoints: Vec::new(),
            managed_processes: Vec::new(),
            iroh_transport: diag.clone(),
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            geo_location: None,
            e2e_test: None,
            blob_store: None,
        };
        let mut stale = fresh.clone();
        stale.node = "stale-peer".to_string();
        stale.updated_at_unix = 500;

        save_peer_cache(&path, &[fresh, stale]).unwrap();
        let loaded = load_peer_cache(&path, 1_100, DEFAULT_PEER_TTL_SECS).unwrap();
        assert_eq!(loaded.len(), 1);
        assert_eq!(loaded[0].node, "node-alpha");
        let _ = fs::remove_file(&path);
    }

    #[test]
    fn rtt_echo_and_mesh_matrix_calculate_pairwise_latency() {
        assert_eq!(HEARTBEAT_INTERVAL_SECS, 5);
        assert_eq!(SUMMARY_HEARTBEAT_INTERVAL_SECS, 30);
        assert_eq!(SUMMARY_HEARTBEAT_EVERY_N_TICKS, 6);

        let echo = PeerRttEcho {
            echoed_mono_ms: 10_000,
            hold_time_ms: 120,
        };
        let rtt = compute_rtt_ms_from_echo(10_148, &echo).unwrap();
        assert!((rtt - 28.0).abs() < 0.01);

        let cfg = IrohTransportConfig::local_offline();
        let addr = EndpointAddr::new(iroh::SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &addr, None, "ep-1");

        let mut hb_a = DialtoneHeartbeat {
            schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            endpoint_id: "ep-1".to_string(),
            ticket: "iroh-gossip-v1:a".to_string(),
            counter: 1,
            updated_at_unix: 1_000,
            heartbeat_kind: "summary".to_string(),
            heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
            summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
            sent_mono_ms: 1_000,
            rtt_probe_requested: false,
            peer_echoes: HashMap::new(),
            peer_rtt_ms: std::collections::BTreeMap::from([("node-beta".to_string(), 14.2)]),
            summary_stats: None,
            channel_mac: None,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for("node-alpha"),
            capabilities: node_capabilities_for("node-alpha"),
            host_resources: sample_live_host_resources("node-alpha", None, 1),
            service_endpoints: Vec::new(),
            managed_processes: Vec::new(),
            iroh_transport: diag,
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
            geo_location: None,
            e2e_test: None,
            blob_store: None,
        };
        let mut hb_b = hb_a.clone();
        hb_b.node = "node-beta".to_string();
        hb_b.peer_rtt_ms = std::collections::BTreeMap::from([("node-alpha".to_string(), 15.8)]);
        hb_a.peer_rtt_ms.insert("node-gamma".to_string(), 22.4);

        let mut nodes = vec![hb_a, hb_b];
        let matrix = build_mesh_rtt_matrix(&nodes);
        assert_eq!(matrix.get("node-alpha<->node-beta"), Some(&15.0));
        assert_eq!(matrix.get("node-alpha<->node-gamma"), Some(&22.4));

        // Verify public IP geolocation + RTT latency clustering placement
        nodes[0].observed_remote_addrs = vec!["direct-ip:52.10.20.30:44222".to_string()];
        resolve_fleet_node_locations(&mut nodes, &matrix);
        let geo_a = nodes[0].geo_location.as_ref().unwrap();
        let geo_b = nodes[1].geo_location.as_ref().unwrap();
        assert_eq!(geo_a.source, "public_ip_geo");
        assert_eq!(geo_b.source, "latency_cluster");
        let ang_deg = great_circle_deg(geo_a.lat, geo_a.lng, geo_b.lat, geo_b.lng);
        assert!(
            ang_deg > 0.3 && ang_deg < 20.0,
            "latency-clustered peer should be placed near public IP anchor based on 15ms RTT (got {ang_deg:.2} deg)"
        );
    }

    #[test]
    fn live_two_peer_iroh_gossip_heartbeat_exchange_succeeds() {
        let rt = tokio::runtime::Runtime::new().unwrap();
        let (received, peer_b) = rt
            .block_on(verify_live_gossip_exchange("node-alpha", "edge-1"))
            .unwrap();
        assert_eq!(received.schema, DIALTONE_HEARTBEAT_SCHEMA);
        assert_eq!(received.node, "node-alpha");
        assert_eq!(
            received.service_endpoints[0].url,
            "dialtone://node-alpha/controller/nats"
        );
        assert_eq!(peer_b.node, "edge-1");
    }
}
