//! Built-in channel-scoped, encrypted, signed, gossip-synced append-only log (`dialtone log`).
//!
//! Every channel gets its own CRDT log keyed by `(hlc_ms, hlc_seq, author_node_id)`:
//! - Encryption key derived via RFC 5869 HKDF-SHA256 from `ChannelKeys` with label
//!   `"dialtone/channel-log/v1"` (never the raw channel secret).
//! - Dedicated gossip topic `SHA256("dialtone.channel.log.v1:" || beats_topic_id)`.
//! - Every entry is Ed25519-signed by the writing node (`author_node_id`), AEAD-sealed
//!   with XChaCha20-Poly1305 bound to `channel_id` via AAD, and secret-redacted before
//!   signing or persistence.
//! - Bounded retention (`MAX_CHANNEL_LOG_ENTRIES` / `MAX_CHANNEL_LOG_FILE_BYTES`).

use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use chacha20poly1305::{
    aead::{Aead, KeyInit, Payload},
    XChaCha20Poly1305, XNonce,
};
use iroh_gossip::TopicId;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::mesh::{channel_dir, ChannelKeys};

pub const CHANNEL_LOG_KEY_LABEL: &str = "dialtone/channel-log/v1";
pub const CHANNEL_LOG_MAGIC: &[u8; 4] = b"dtl1";
pub const MAX_CHANNEL_LOG_MESSAGE_BYTES: usize = 4096;
pub const MAX_CHANNEL_LOG_FRAME_BYTES: usize = 16 * 1024;
pub const MAX_CHANNEL_LOG_ENTRIES: usize = 10_000;
pub const MAX_CHANNEL_LOG_FILE_BYTES: u64 = 16 * 1024 * 1024;

fn now_unix_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis() as u64)
        .unwrap_or(0)
}

fn hmac_sha256(key: &[u8], data: &[u8]) -> [u8; 32] {
    const BLOCK_LEN: usize = 64;
    let mut k_block = [0u8; BLOCK_LEN];
    if key.len() > BLOCK_LEN {
        let hashed: [u8; 32] = Sha256::digest(key).into();
        k_block[..32].copy_from_slice(&hashed);
    } else {
        k_block[..key.len()].copy_from_slice(key);
    }
    let mut i_key_pad = [0x36u8; BLOCK_LEN];
    let mut o_key_pad = [0x5cu8; BLOCK_LEN];
    for i in 0..BLOCK_LEN {
        i_key_pad[i] ^= k_block[i];
        o_key_pad[i] ^= k_block[i];
    }
    let mut inner = Sha256::new();
    inner.update(i_key_pad);
    inner.update(data);
    let inner_hash: [u8; 32] = inner.finalize().into();

    let mut outer = Sha256::new();
    outer.update(o_key_pad);
    outer.update(inner_hash);
    outer.finalize().into()
}

/// Derive the 32-byte channel log encryption key via RFC 5869 HKDF-SHA256 from `ChannelKeys`
/// with label `"dialtone/channel-log/v1"` (never from the raw channel secret).
pub fn derive_channel_log_key(keys: &ChannelKeys) -> [u8; 32] {
    let mut ikm = [0u8; 64];
    ikm[..32].copy_from_slice(&keys.enc_key);
    ikm[32..].copy_from_slice(&keys.auth_key);
    let prk = hmac_sha256(keys.topic_id.as_bytes(), &ikm);

    let mut msg = Vec::with_capacity(CHANNEL_LOG_KEY_LABEL.len() + 1);
    msg.extend_from_slice(CHANNEL_LOG_KEY_LABEL.as_bytes());
    msg.push(0x01);
    hmac_sha256(&prk, &msg)
}

/// Dedicated gossip topic for channel-scoped logs, isolated from beats and events.
pub fn channel_log_topic_id(beats_topic: TopicId) -> TopicId {
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.log.v1:");
    hasher.update(beats_topic.as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    TopicId::from_bytes(digest)
}

/// On-disk path for a channel's durable append-only CRDT log:
/// `<state_dir>/channels/<channel_id>/channel-log.jsonl`.
pub fn channel_log_path(state_dir: &Path, channel_id: &str) -> PathBuf {
    channel_dir(state_dir, channel_id).join("channel-log.jsonl")
}

/// Redact secrets, tokens, keys, tickets, and invites from a channel log message,
/// strip em-dash/en-dash characters, and clamp to `MAX_CHANNEL_LOG_MESSAGE_BYTES`.
pub fn redact_channel_log_message(input: &str) -> String {
    let cleaned = input.replace('\u{2014}', " - ").replace('\u{2013}', "-");
    let mut out = String::with_capacity(cleaned.len().min(MAX_CHANNEL_LOG_MESSAGE_BYTES));

    for (line_idx, line) in cleaned.split('\n').enumerate() {
        if line_idx > 0 {
            out.push('\n');
        }
        let mut prev_was_bearer = false;
        for (tok_idx, token) in line.split(' ').enumerate() {
            if tok_idx > 0 {
                out.push(' ');
            }
            if prev_was_bearer && token.len() >= 12 {
                out.push_str("[REDACTED]");
                prev_was_bearer = false;
                continue;
            }
            prev_was_bearer = token.eq_ignore_ascii_case("bearer");
            out.push_str(&redact_token(token));
        }
    }

    if out.len() <= MAX_CHANNEL_LOG_MESSAGE_BYTES {
        out
    } else {
        let mut end = MAX_CHANNEL_LOG_MESSAGE_BYTES.saturating_sub(3);
        while end > 0 && !out.is_char_boundary(end) {
            end -= 1;
        }
        format!("{}...", &out[..end])
    }
}

fn redact_token(token: &str) -> String {
    let trimmed = token.trim_matches(|c: char| matches!(c, '"' | '\'' | ',' | ';' | '(' | ')' | '[' | ']' | '{' | '}'));
    if trimmed.is_empty() {
        return token.to_string();
    }
    let lower = trimmed.to_ascii_lowercase();

    // Exact secret/ticket/invite/API key prefixes.
    let direct_prefixes = [
        ("iroh-blobs://ticket/", "[REDACTED_TICKET]"),
        ("dialtone-invite-v1:", "[REDACTED_INVITE]"),
        ("dialtone-invite-v2:", "[REDACTED_INVITE]"),
        ("iroh-gossip-v1:", "[REDACTED_TICKET]"),
        ("dialtone-gossip-v1:", "[REDACTED_TICKET]"),
        ("dtk-", "[REDACTED_TOKEN]"),
        ("dt-", "[REDACTED_SECRET]"),
        ("ghp_", "[REDACTED_GH_TOKEN]"),
        ("gho_", "[REDACTED_GH_TOKEN]"),
        ("ghu_", "[REDACTED_GH_TOKEN]"),
        ("ghs_", "[REDACTED_GH_TOKEN]"),
        ("ghr_", "[REDACTED_GH_TOKEN]"),
        ("sk-", "[REDACTED_API_KEY]"),
        ("xai-", "[REDACTED_API_KEY]"),
        ("aiza", "[REDACTED_API_KEY]"),
    ];
    for (prefix, repl) in direct_prefixes {
        if lower.starts_with(prefix) && trimmed.len() >= prefix.len() + 12 {
            return token.replace(trimmed, repl);
        }
    }

    // Key-value assignments like token=..., secret=..., api_key=..., password=..., mesh.key=...
    for sep in ['=', ':'] {
        if let Some((k, v)) = trimmed.split_once(sep) {
            let k_lower = k.to_ascii_lowercase();
            let v_clean = v.trim_matches(|c: char| matches!(c, '"' | '\''));
            let sensitive_key = k_lower.contains("token")
                || k_lower.contains("secret")
                || k_lower.contains("api_key")
                || k_lower.contains("apikey")
                || k_lower.contains("api-key")
                || k_lower.contains("password")
                || k_lower.contains("mesh.key")
                || k_lower.contains("plugin_key");
            if sensitive_key && v_clean.len() >= 12 {
                let replacement = format!("{k}{sep}[REDACTED]");
                return token.replace(trimmed, &replacement);
            }
        }
    }

    token.to_string()
}

/// One append-only, signed channel log record ordered by `(hlc_ms, hlc_seq, author_node_id)`.
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ChannelLogEntry {
    pub id: String,
    pub channel_id: String,
    pub hlc_ms: u64,
    pub hlc_seq: u32,
    pub author_node_id: String,
    pub author_node: String,
    pub source: String,
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub task_id: Option<String>,
    pub level: String,
    pub message: String,
    pub sig_hex: String,
}

impl ChannelLogEntry {
    pub fn crdt_key(&self) -> (u64, u32, String) {
        (self.hlc_ms, self.hlc_seq, self.author_node_id.clone())
    }

    /// Canonical byte payload signed by `author_node_id`'s Ed25519 key.
    pub fn canonical_signing_bytes(&self) -> Vec<u8> {
        let mut buf = Vec::with_capacity(128 + self.message.len());
        buf.extend_from_slice(b"dialtone.channel.log.entry.v1\0");
        for part in [
            self.channel_id.as_str(),
            &self.hlc_ms.to_string(),
            &self.hlc_seq.to_string(),
            self.author_node_id.as_str(),
            self.author_node.as_str(),
            self.source.as_str(),
            self.kind.as_str(),
            self.task_id.as_deref().unwrap_or(""),
            self.level.as_str(),
            self.message.as_str(),
        ] {
            buf.extend_from_slice(&(part.len() as u32).to_be_bytes());
            buf.extend_from_slice(part.as_bytes());
        }
        buf
    }

    pub fn sign(&mut self, key: &iroh::SecretKey) {
        let payload = self.canonical_signing_bytes();
        let sig = key.sign(&payload);
        self.sig_hex = crate::hex_encode(&sig.to_bytes());
    }

    pub fn verify_signature(&self) -> Result<(), String> {
        let pubkey: iroh::PublicKey = self
            .author_node_id
            .trim()
            .parse()
            .map_err(|_| format!("invalid author_node_id '{}'", self.author_node_id))?;
        let sig_vec = crate::hex_decode(&self.sig_hex)?;
        let sig_bytes: [u8; 64] = sig_vec
            .try_into()
            .map_err(|_| "channel log signature must be 64 bytes".to_string())?;
        let payload = self.canonical_signing_bytes();
        pubkey
            .verify(&payload, &iroh::Signature::from_bytes(&sig_bytes))
            .map_err(|_| {
                format!(
                    "channel log signature verification failed for author {}",
                    self.author_node_id
                )
            })
    }
}

fn log_aad(channel_id: &str) -> Vec<u8> {
    format!("dialtone.channel.log.v1:{channel_id}").into_bytes()
}

/// Seal a signed `ChannelLogEntry` with XChaCha20-Poly1305 under `derive_channel_log_key(keys)`.
pub fn seal_log_entry(entry: &ChannelLogEntry, keys: &ChannelKeys) -> Result<Vec<u8>, String> {
    let expected_cid = keys.fingerprint();
    if entry.channel_id != expected_cid {
        return Err(format!(
            "log entry channel_id '{}' does not match active channel '{expected_cid}'",
            entry.channel_id
        ));
    }
    entry.verify_signature()?;
    let plaintext = serde_json::to_vec(entry).map_err(|e| format!("encode log entry: {e}"))?;
    let log_key = derive_channel_log_key(keys);
    let rand_bytes = iroh::SecretKey::generate().to_bytes();
    let nonce_bytes = &rand_bytes[..24];
    let nonce = XNonce::from_slice(nonce_bytes);
    let cipher = XChaCha20Poly1305::new((&log_key).into());
    let aad = log_aad(&expected_cid);
    let ciphertext = cipher
        .encrypt(
            nonce,
            Payload {
                msg: &plaintext,
                aad: &aad,
            },
        )
        .map_err(|_| "channel log encryption failed".to_string())?;
    let mut frame = Vec::with_capacity(CHANNEL_LOG_MAGIC.len() + 24 + ciphertext.len());
    frame.extend_from_slice(CHANNEL_LOG_MAGIC);
    frame.extend_from_slice(nonce_bytes);
    frame.extend_from_slice(&ciphertext);
    if frame.len() > MAX_CHANNEL_LOG_FRAME_BYTES {
        return Err(format!(
            "channel log frame exceeds max size ({} > {MAX_CHANNEL_LOG_FRAME_BYTES})",
            frame.len()
        ));
    }
    Ok(frame)
}

/// Decrypt and authenticate a channel log gossip frame using `derive_channel_log_key(keys)`,
/// verifying `channel_id` and the author's Ed25519 signature.
pub fn open_log_entry(frame: &[u8], keys: &ChannelKeys) -> Result<ChannelLogEntry, String> {
    if frame.len() > MAX_CHANNEL_LOG_FRAME_BYTES {
        return Err("channel log frame too large".to_string());
    }
    let Some(rest) = frame.strip_prefix(CHANNEL_LOG_MAGIC.as_slice()) else {
        return Err("channel log frame has invalid magic".to_string());
    };
    if rest.len() < 24 + 16 {
        return Err("channel log frame too short".to_string());
    }
    let (nonce_bytes, ciphertext) = rest.split_at(24);
    let expected_cid = keys.fingerprint();
    let log_key = derive_channel_log_key(keys);
    let cipher = XChaCha20Poly1305::new((&log_key).into());
    let aad = log_aad(&expected_cid);
    let plaintext = cipher
        .decrypt(
            XNonce::from_slice(nonce_bytes),
            Payload {
                msg: ciphertext,
                aad: &aad,
            },
        )
        .map_err(|_| {
            "channel log decryption failed (wrong channel key or tampered frame)".to_string()
        })?;
    let entry: ChannelLogEntry =
        serde_json::from_slice(&plaintext).map_err(|e| format!("decode log entry: {e}"))?;
    if entry.channel_id != expected_cid {
        return Err(format!(
            "channel log channel_id mismatch: expected '{expected_cid}', got '{}'",
            entry.channel_id
        ));
    }
    entry.verify_signature()?;
    Ok(entry)
}

#[derive(Clone, Debug, Default)]
pub struct ChannelLogQuery {
    pub source: Option<String>,
    pub task_id: Option<String>,
    pub since_ms: Option<u64>,
    pub limit: Option<usize>,
}

/// Durable append-only CRDT channel log store keyed by `(hlc_ms, hlc_seq, author_node_id)`.
#[derive(Debug)]
pub struct ChannelLogStore {
    path: PathBuf,
    channel_id: String,
    entries: BTreeMap<(u64, u32, String), ChannelLogEntry>,
    last_hlc_ms: u64,
    last_hlc_seq: u32,
    max_entries: usize,
}

impl ChannelLogStore {
    pub fn open(path: &Path, channel_id: impl Into<String>) -> Result<Self, String> {
        Self::open_with_cap(path, channel_id, MAX_CHANNEL_LOG_ENTRIES)
    }

    pub fn open_with_cap(
        path: &Path,
        channel_id: impl Into<String>,
        max_entries: usize,
    ) -> Result<Self, String> {
        let channel_id = channel_id.into();
        let mut store = Self {
            path: path.to_path_buf(),
            channel_id,
            entries: BTreeMap::new(),
            last_hlc_ms: 0,
            last_hlc_seq: 0,
            max_entries: max_entries.max(1),
        };
        if path.is_file() {
            let raw = fs::read_to_string(path)
                .map_err(|e| format!("read channel log {}: {e}", path.display()))?;
            for line in raw.lines() {
                let trimmed = line.trim();
                if trimmed.is_empty() {
                    continue;
                }
                if let Ok(entry) = serde_json::from_str::<ChannelLogEntry>(trimmed) {
                    if entry.channel_id == store.channel_id && entry.verify_signature().is_ok() {
                        if entry.hlc_ms > store.last_hlc_ms {
                            store.last_hlc_ms = entry.hlc_ms;
                            store.last_hlc_seq = entry.hlc_seq;
                        } else if entry.hlc_ms == store.last_hlc_ms {
                            store.last_hlc_seq = store.last_hlc_seq.max(entry.hlc_seq);
                        }
                        store.entries.insert(entry.crdt_key(), entry);
                    }
                }
            }
            if store.enforce_retention() {
                store.compact_disk()?;
            }
        }
        Ok(store)
    }

    pub fn channel_id(&self) -> &str {
        &self.channel_id
    }

    pub fn path(&self) -> &Path {
        &self.path
    }

    pub fn len(&self) -> usize {
        self.entries.len()
    }

    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    fn next_hlc(&mut self) -> (u64, u32) {
        let now = now_unix_ms();
        if now > self.last_hlc_ms {
            self.last_hlc_ms = now;
            self.last_hlc_seq = 0;
        } else {
            self.last_hlc_seq = self.last_hlc_seq.saturating_add(1);
        }
        (self.last_hlc_ms, self.last_hlc_seq)
    }

    /// Create, redact, sign, persist, and return a new local channel log entry.
    #[allow(clippy::too_many_arguments)]
    pub fn append_local(
        &mut self,
        node_key: &iroh::SecretKey,
        author_node: &str,
        source: &str,
        kind: &str,
        task_id: Option<String>,
        level: &str,
        message: &str,
    ) -> Result<ChannelLogEntry, String> {
        let (hlc_ms, hlc_seq) = self.next_hlc();
        let author_node_id = node_key.public().to_string();
        let prefix = author_node_id.get(..8).unwrap_or(&author_node_id);
        let mut entry = ChannelLogEntry {
            id: format!("{hlc_ms}-{hlc_seq}-{prefix}"),
            channel_id: self.channel_id.clone(),
            hlc_ms,
            hlc_seq,
            author_node_id,
            author_node: author_node.trim().to_string(),
            source: source.trim().to_ascii_lowercase(),
            kind: kind.trim().to_ascii_lowercase(),
            task_id: task_id.map(|s| s.trim().to_string()).filter(|s| !s.is_empty()),
            level: level.trim().to_ascii_lowercase(),
            message: redact_channel_log_message(message),
            sig_hex: String::new(),
        };
        entry.sign(node_key);
        self.insert_and_persist(entry.clone())?;
        Ok(entry)
    }

    /// Merge a verified remote entry into this channel's CRDT log (idempotent).
    /// Returns `Ok(true)` if the entry was newly inserted, `Ok(false)` if already present.
    pub fn merge_verified(&mut self, entry: ChannelLogEntry) -> Result<bool, String> {
        if entry.channel_id != self.channel_id {
            return Err(format!(
                "refusing cross-channel log entry '{}' for channel '{}'",
                entry.channel_id, self.channel_id
            ));
        }
        entry.verify_signature()?;
        let key = entry.crdt_key();
        if self.entries.contains_key(&key) {
            return Ok(false);
        }
        if entry.hlc_ms > self.last_hlc_ms {
            self.last_hlc_ms = entry.hlc_ms;
            self.last_hlc_seq = entry.hlc_seq;
        } else if entry.hlc_ms == self.last_hlc_ms {
            self.last_hlc_seq = self.last_hlc_seq.max(entry.hlc_seq);
        }
        self.insert_and_persist(entry)?;
        Ok(true)
    }

    fn insert_and_persist(&mut self, entry: ChannelLogEntry) -> Result<(), String> {
        self.entries.insert(entry.crdt_key(), entry.clone());
        let needs_compact = self.enforce_retention()
            || fs::metadata(&self.path)
                .map(|m| m.len() > MAX_CHANNEL_LOG_FILE_BYTES)
                .unwrap_or(false);
        if needs_compact {
            self.compact_disk()
        } else {
            self.append_disk(&entry)
        }
    }

    fn enforce_retention(&mut self) -> bool {
        let mut evicted = false;
        while self.entries.len() > self.max_entries {
            if let Some(first_key) = self.entries.keys().next().cloned() {
                self.entries.remove(&first_key);
                evicted = true;
            } else {
                break;
            }
        }
        evicted
    }

    fn append_disk(&self, entry: &ChannelLogEntry) -> Result<(), String> {
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent)
                .map_err(|e| format!("create channel log dir {}: {e}", parent.display()))?;
        }
        let line = serde_json::to_string(entry).map_err(|e| format!("encode log line: {e}"))?;
        let mut f = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.path)
            .map_err(|e| format!("open channel log {}: {e}", self.path.display()))?;
        writeln!(f, "{line}")
            .map_err(|e| format!("append channel log {}: {e}", self.path.display()))?;
        Ok(())
    }

    fn compact_disk(&self) -> Result<(), String> {
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent)
                .map_err(|e| format!("create channel log dir {}: {e}", parent.display()))?;
        }
        let tmp = self
            .path
            .with_extension(format!("jsonl.tmp.{}", std::process::id()));
        let mut buf = String::new();
        for entry in self.entries.values() {
            if let Ok(line) = serde_json::to_string(entry) {
                buf.push_str(&line);
                buf.push('\n');
            }
        }
        fs::write(&tmp, buf.as_bytes())
            .map_err(|e| format!("write temp channel log {}: {e}", tmp.display()))?;
        fs::rename(&tmp, &self.path).map_err(|e| {
            let _ = fs::remove_file(&tmp);
            format!("replace channel log {}: {e}", self.path.display())
        })
    }

    /// Query entries in causal CRDT order with optional filters.
    pub fn query(&self, filter: &ChannelLogQuery) -> Vec<ChannelLogEntry> {
        let mut matched: Vec<ChannelLogEntry> = self
            .entries
            .values()
            .filter(|e| {
                filter
                    .source
                    .as_deref()
                    .map(str::trim)
                    .filter(|s| !s.is_empty())
                    .is_none_or(|src| {
                        e.source.eq_ignore_ascii_case(src)
                            || (src.eq_ignore_ascii_case("task") && e.kind.starts_with("task."))
                    })
            })
            .filter(|e| {
                filter
                    .task_id
                    .as_deref()
                    .map(str::trim)
                    .filter(|s| !s.is_empty())
                    .is_none_or(|tid| e.task_id.as_deref() == Some(tid))
            })
            .filter(|e| filter.since_ms.is_none_or(|ms| e.hlc_ms > ms))
            .cloned()
            .collect();
        if let Some(limit) = filter.limit {
            if matched.len() > limit {
                let start = matched.len() - limit;
                matched = matched.split_off(start);
            }
        }
        matched
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::mesh::derive_channel_keys_v2;
    use crate::node::{build_endpoint_builder, IrohTransportConfig};
    use futures_lite::StreamExt;
    use iroh::address_lookup::memory::MemoryLookup;
    use iroh::protocol::Router;
    use iroh_gossip::{Event as GossipEvent, Gossip, ALPN as GOSSIP_ALPN};
    use std::time::Duration;

    #[test]
    fn channel_log_redacts_secrets_and_enforces_retention_cap() {
        let dir = std::env::temp_dir().join(format!(
            "dialtone-chlog-redact-{}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&dir);
        let log_file = dir.join("channel-log.jsonl");

        let mut store = ChannelLogStore::open_with_cap(&log_file, "ch-aabbccddeeff", 3).unwrap();
        let sk = iroh::SecretKey::generate();

        let e1 = store
            .append_local(
                &sk,
                "legion",
                "task",
                "task.event",
                Some("t-1".into()),
                "info",
                "joined with dt-SuperSecretChannelKey1234567890 and Bearer dtk-myLocalApiToken1234567890 and token=abcdef1234567890 \u{2014} done",
            )
            .unwrap();
        assert!(!e1.message.contains("SuperSecretChannelKey"));
        assert!(!e1.message.contains("myLocalApiToken"));
        assert!(!e1.message.contains("abcdef1234567890"));
        assert!(!e1.message.contains('\u{2014}'));
        assert!(e1.verify_signature().is_ok());

        for i in 2..=5 {
            store
                .append_local(
                    &sk,
                    "legion",
                    "agent",
                    "agent.run",
                    Some(format!("t-{i}")),
                    "info",
                    &format!("run #{i}"),
                )
                .unwrap();
        }
        assert_eq!(store.len(), 3, "retention cap must evict oldest entries");

        // Reopen from disk and confirm persistence + signature verification.
        let reopened = ChannelLogStore::open_with_cap(&log_file, "ch-aabbccddeeff", 3).unwrap();
        assert_eq!(reopened.len(), 3);
        let entries = reopened.query(&ChannelLogQuery::default());
        assert_eq!(entries[0].message, "run #3");
        assert_eq!(entries[2].message, "run #5");

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn channel_b_cannot_decrypt_or_verify_channel_a_log_entries() {
        let keys_a = derive_channel_keys_v2("alpha", "dt-Log_Alpha_Secret_2026_Key!");
        let keys_b = derive_channel_keys_v2("beta", "dt-Log_Beta_Secret_2026_Key!");
        assert_ne!(
            derive_channel_log_key(&keys_a),
            derive_channel_log_key(&keys_b)
        );
        assert_ne!(
            channel_log_topic_id(keys_a.topic_id),
            channel_log_topic_id(keys_b.topic_id)
        );

        let dir = std::env::temp_dir().join(format!(
            "dialtone-chlog-crypto-{}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&dir);
        let sk_a = iroh::SecretKey::generate();
        let mut store_a = ChannelLogStore::open(&dir.join("a.jsonl"), keys_a.fingerprint()).unwrap();
        let entry = store_a
            .append_local(
                &sk_a,
                "alpha-1",
                "task",
                "task.event",
                Some("t-alpha".into()),
                "info",
                "confidential channel A task update",
            )
            .unwrap();

        let sealed = seal_log_entry(&entry, &keys_a).unwrap();
        let opened = open_log_entry(&sealed, &keys_a).unwrap();
        assert_eq!(opened, entry);

        // Channel B cannot decrypt Channel A's frame.
        let err_b = open_log_entry(&sealed, &keys_b).unwrap_err();
        assert!(err_b.contains("decryption failed"), "got: {err_b}");

        // Channel B store refuses to merge a Channel A entry even if presented in plaintext.
        let mut store_b = ChannelLogStore::open(&dir.join("b.jsonl"), keys_b.fingerprint()).unwrap();
        assert!(store_b.merge_verified(entry).is_err());
        assert!(store_b.is_empty());

        let _ = fs::remove_dir_all(&dir);
    }

    /// End-to-end gossip test: entries written on Channel A replicate to another Channel A member
    /// over encrypted gossip, while a Channel B node receives nothing and cannot decrypt Channel A frames.
    #[test]
    fn channel_log_gossips_within_channel_a_and_stays_unreadable_to_channel_b() {
        let dir = std::env::temp_dir().join(format!(
            "dialtone-chlog-gossip-{}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&dir);

        let keys_a = derive_channel_keys_v2("alpha", "dt-LogGossip_Alpha_Secret_2026!");
        let keys_b = derive_channel_keys_v2("beta", "dt-LogGossip_Beta_Secret_2026!");
        let topic_a = channel_log_topic_id(keys_a.topic_id);
        let topic_b = channel_log_topic_id(keys_b.topic_id);

        let sk_a1 = iroh::SecretKey::generate();
        let sk_a2 = iroh::SecretKey::generate();
        let sk_b1 = iroh::SecretKey::generate();

        let mut store_a1 =
            ChannelLogStore::open(&channel_log_path(&dir.join("a1"), &keys_a.fingerprint()), keys_a.fingerprint()).unwrap();
        let mut store_a2 =
            ChannelLogStore::open(&channel_log_path(&dir.join("a2"), &keys_a.fingerprint()), keys_a.fingerprint()).unwrap();
        let store_b1 =
            ChannelLogStore::open(&channel_log_path(&dir.join("b1"), &keys_b.fingerprint()), keys_b.fingerprint()).unwrap();

        let rt = tokio::runtime::Runtime::new().unwrap();
        rt.block_on(async {
            let cfg = IrohTransportConfig::local_offline();
            let make_node = |sk: iroh::SecretKey| {
                let cfg = cfg.clone();
                async move {
                    let lookup = MemoryLookup::new();
                    let ep = build_endpoint_builder(&cfg)
                        .unwrap()
                        .address_lookup(lookup.clone())
                        .secret_key(sk)
                        .bind()
                        .await
                        .unwrap();
                    let gossip = Gossip::builder().spawn(ep.clone());
                    let router = Router::builder(ep.clone())
                        .accept(GOSSIP_ALPN, gossip.clone())
                        .spawn();
                    (ep, lookup, gossip, router)
                }
            };

            let (ep_a1, l_a1, g_a1, r_a1) = make_node(sk_a1.clone()).await;
            let (ep_a2, l_a2, g_a2, r_a2) = make_node(sk_a2.clone()).await;
            let (ep_b1, l_b1, g_b1, r_b1) = make_node(sk_b1.clone()).await;

            // Cross-register addresses so all nodes could connect if topics/keys matched.
            l_a1.add_endpoint_info(ep_a2.addr());
            l_a1.add_endpoint_info(ep_b1.addr());
            l_a2.add_endpoint_info(ep_a1.addr());
            l_b1.add_endpoint_info(ep_a1.addr());

            let (tx_a1, _rx_a1) = g_a1.subscribe(topic_a, vec![ep_a2.id()]).await.unwrap().split();
            let (_tx_a2, mut rx_a2) = g_a2.subscribe(topic_a, vec![ep_a1.id()]).await.unwrap().split();
            let (_tx_b1, mut rx_b1) = g_b1.subscribe(topic_b, vec![ep_a1.id()]).await.unwrap().split();

            // Wait briefly for A1 <-> A2 neighbor formation on topic_a.
            let _ = tokio::time::timeout(Duration::from_secs(5), rx_a2.joined()).await;

            let entry_a = store_a1
                .append_local(
                    &sk_a1,
                    "a1",
                    "task",
                    "task.event",
                    Some("t-channel-a-1".into()),
                    "info",
                    "task created on channel A",
                )
                .unwrap();
            let sealed_frame = seal_log_entry(&entry_a, &keys_a).unwrap();

            // Broadcast on Channel A log topic (retry briefly if gossip mesh is still warming up).
            let mut received_on_a2 = None;
            for _ in 0..8 {
                let _ = tx_a1.broadcast(sealed_frame.clone().into()).await;
                if let Ok(Some(Ok(GossipEvent::Received(msg)))) =
                    tokio::time::timeout(Duration::from_millis(400), rx_a2.next()).await
                {
                    let opened = open_log_entry(&msg.content, &keys_a).unwrap();
                    store_a2.merge_verified(opened.clone()).unwrap();
                    received_on_a2 = Some(opened);
                    break;
                }
            }
            let got = received_on_a2.expect("Channel A member A2 must receive A1's log entry");
            assert_eq!(got.id, entry_a.id);
            assert_eq!(got.message, "task created on channel A");
            assert_eq!(store_a2.len(), 1);

            // Channel B must receive nothing on its topic, and cannot decrypt A1's wire frame.
            let b_event = tokio::time::timeout(Duration::from_millis(300), rx_b1.next()).await;
            assert!(
                !matches!(b_event, Ok(Some(Ok(GossipEvent::Received(_))))),
                "Channel B must not receive Channel A log frames"
            );
            assert!(open_log_entry(&sealed_frame, &keys_b).is_err());
            assert!(store_b1.is_empty());

            let _ = r_a1.shutdown().await;
            let _ = r_a2.shutdown().await;
            let _ = r_b1.shutdown().await;
        });

        let _ = fs::remove_dir_all(&dir);
    }
}
