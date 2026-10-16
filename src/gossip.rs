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

use crate::evidence::{sample_live_host_resources, HostResourceTelemetry};
use crate::identity::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, is_tailnet_ip,
    IrohTransportConfig, IrohTransportDiagnostics,
};
use crate::refs::ServiceAdvertisement;

pub const DIALTONE_GOSSIP_TOPIC_BYTES: &[u8; 32] = b"dialtone-iroh-gossip-topic-00001";
pub const DIALTONE_HEARTBEAT_SCHEMA: &str = "dialtone.iroh.heartbeat.v1";
pub const DIALTONE_ENVELOPE_SCHEMA_V2: &str = "dialtone.iroh.envelope.v2";
pub const DIALTONE_INVITE_SCHEMA_V1: &str = "dialtone.invite.v1";
pub const INVITE_TICKET_PREFIX: &str = "dialtone-invite-v1:";
pub const DEFAULT_CHANNEL_NAME: &str = "dialtone";
pub const DEFAULT_PEER_TTL_SECS: u64 = 300;

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

pub fn default_topic_id() -> TopicId {
    TopicId::from_bytes(*DIALTONE_GOSSIP_TOPIC_BYTES)
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
        h.update(&[0x01]);
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
            if let Ok(relay_url) = relay_str.parse::<iroh::RelayUrl>() {
                addr = addr.with_relay_url(relay_url);
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
    let parsed_id = endpoint_id
        .trim()
        .parse::<iroh::EndpointId>()
        .map_err(|e| format!("invalid EndpointId '{}': {e}", endpoint_id.trim()))?
        .to_string();
    let mut cfg = load_mesh_config(config_path);
    if !cfg
        .allowed_endpoint_ids
        .iter()
        .any(|x| x.eq_ignore_ascii_case(&parsed_id))
    {
        cfg.allowed_endpoint_ids.push(parsed_id.clone());
    }
    if !cfg
        .seed_peers
        .iter()
        .any(|s| s.endpoint_id.eq_ignore_ascii_case(&parsed_id))
    {
        cfg.seed_peers.push(SeedPeerConfig {
            endpoint_id: parsed_id,
            relay_url: Some(
                relay_url
                    .unwrap_or("https://usw1-1.relay.n0.iroh.link./")
                    .to_string(),
            ),
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

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct DialtoneHeartbeat {
    pub schema: String,
    pub node: String,
    pub endpoint_id: String,
    pub ticket: String,
    pub counter: u64,
    pub updated_at_unix: u64,
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
            topic: default_topic_id(),
            peers: vec![addr],
        };
        let encoded = ticket.to_text().unwrap();
        let decoded = GossipTicket::from_str(&encoded).unwrap();
        assert_eq!(decoded.topic, ticket.topic);
        assert_eq!(decoded.peers.len(), 1);
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
