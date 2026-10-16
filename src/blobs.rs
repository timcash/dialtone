use std::fs;
use std::path::{Path, PathBuf};
use std::time::Duration;

use iroh::address_lookup::memory::MemoryLookup;
use iroh::protocol::Router;
use iroh_blobs::{store::mem::MemStore, ticket::BlobTicket, BlobsProtocol};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::identity::{build_endpoint_builder, IrohTransportConfig};

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct BlobTransportReport {
    pub sha256: String,
    pub blake3_hash: String,
    pub provider_endpoint_id: String,
    pub ticket_uri: String,
    pub bytes: u64,
    pub transferred_bytes: u64,
    pub method: String,
    pub fallback_used: bool,
    pub active_address_modes: Vec<String>,
    pub known_address_modes: Vec<String>,
    pub remote_info_status: String,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct TieredBlobFetchRequest {
    pub sha256: String,
    pub uri: String,
    pub provider_node: String,
    pub expected_bytes: u64,
    pub http_fallback_enabled: bool,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct TieredBlobFetchPlan {
    pub sha256: String,
    pub uri_scheme: String,
    pub provider_node: String,
    pub method: String,
    pub status: String,
    pub transferred_bytes: u64,
    pub native_ticket_attempted: bool,
    pub fallback_used: bool,
    pub message: String,
}

pub fn compute_sha256(bytes: &[u8]) -> String {
    let digest = Sha256::digest(bytes);
    format!("sha256:{}", crate::hex_encode(&digest))
}

pub fn normalize_sha256(raw: &str) -> Result<String, String> {
    let trimmed = raw.trim().to_ascii_lowercase();
    let hex = trimmed.strip_prefix("sha256:").unwrap_or(&trimmed);
    if hex.len() != 64 || !hex.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err(format!("invalid sha256 digest: {raw:?}"));
    }
    Ok(format!("sha256:{hex}"))
}

pub fn blob_cache_path(cache_root: &Path, sha256: &str) -> Result<PathBuf, String> {
    let normalized = normalize_sha256(sha256)?;
    let hex = normalized.strip_prefix("sha256:").unwrap_or(&normalized);
    Ok(cache_root.join(&hex[..2]).join(hex))
}

pub fn has_cached_blob(cache_root: &Path, sha256: &str) -> bool {
    let Ok(normalized) = normalize_sha256(sha256) else {
        return false;
    };
    let Ok(path) = blob_cache_path(cache_root, &normalized) else {
        return false;
    };
    let Ok(bytes) = fs::read(&path) else {
        return false;
    };
    compute_sha256(&bytes) == normalized
}

pub fn read_cached_blob(cache_root: &Path, sha256: &str) -> Result<Vec<u8>, String> {
    let normalized = normalize_sha256(sha256)?;
    let path = blob_cache_path(cache_root, &normalized)?;
    let bytes = fs::read(&path).map_err(|e| format!("read cached blob {}: {e}", path.display()))?;
    let observed = compute_sha256(&bytes);
    if observed != normalized {
        return Err(format!(
            "cached blob sha256 mismatch: expected {normalized}, observed {observed}"
        ));
    }
    Ok(bytes)
}

pub fn store_verified_blob(
    cache_root: &Path,
    bytes: &[u8],
    expected_sha256: Option<&str>,
) -> Result<(String, PathBuf), String> {
    let observed = compute_sha256(bytes);
    if let Some(expected) = expected_sha256 {
        let expected = normalize_sha256(expected)?;
        if observed != expected {
            return Err(format!(
                "blob sha256 mismatch: expected {expected}, observed {observed}"
            ));
        }
    }
    let blob_path = blob_cache_path(cache_root, &observed)?;
    if let Some(parent) = blob_path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create blob dir {}: {e}", parent.display()))?;
    }
    fs::write(&blob_path, bytes).map_err(|e| format!("write blob {}: {e}", blob_path.display()))?;
    Ok((observed, blob_path))
}

/// Evaluate tiered blob hydration order:
/// 1. Local content-addressed SHA-256 cache hit (`transferred_bytes = 0`).
/// 2. Native `iroh-blobs://ticket/...` fetch over QUIC.
/// 3. Bounded HTTP peer-bridge fallback only when explicitly enabled.
pub fn evaluate_tiered_blob_fetch(
    cache_root: &Path,
    req: &TieredBlobFetchRequest,
) -> Result<TieredBlobFetchPlan, String> {
    let sha256 = normalize_sha256(&req.sha256)?;
    let scheme = req
        .uri
        .split_once("://")
        .map(|(s, _)| s.to_string())
        .unwrap_or_else(|| "unknown".to_string());

    if has_cached_blob(cache_root, &sha256) {
        return Ok(TieredBlobFetchPlan {
            sha256,
            uri_scheme: scheme,
            provider_node: req.provider_node.clone(),
            method: "local-cache".to_string(),
            status: "cache_hit".to_string(),
            transferred_bytes: 0,
            native_ticket_attempted: false,
            fallback_used: false,
            message: "verified payload already present in local content-addressed store"
                .to_string(),
        });
    }

    if req.uri.starts_with("iroh-blobs://ticket/") {
        return Ok(TieredBlobFetchPlan {
            sha256,
            uri_scheme: "iroh-blobs".to_string(),
            provider_node: req.provider_node.clone(),
            method: "iroh-blobs-ticket".to_string(),
            status: "planned_native_iroh".to_string(),
            transferred_bytes: req.expected_bytes,
            native_ticket_attempted: true,
            fallback_used: false,
            message: format!(
                "fetching via native iroh-blobs QUIC ticket from producer peer {}",
                req.provider_node
            ),
        });
    }

    if (req.uri.starts_with("http://") || req.uri.starts_with("https://"))
        && req.http_fallback_enabled
    {
        return Ok(TieredBlobFetchPlan {
            sha256,
            uri_scheme: scheme,
            provider_node: req.provider_node.clone(),
            method: "http-peer-fallback".to_string(),
            status: "planned_http_fallback".to_string(),
            transferred_bytes: req.expected_bytes,
            native_ticket_attempted: false,
            fallback_used: true,
            message: "explicit HTTP bootstrap fallback enabled".to_string(),
        });
    }

    Err(format!(
        "steady-state blob fetch rejected URI {:?}: native iroh-blobs://ticket/... required when HTTP fallback is disabled",
        req.uri
    ))
}

/// Format a native `iroh-blobs` ticket URI with its canonical SHA-256 verification anchor.
pub fn format_blob_ticket_uri(ticket: &BlobTicket, sha256: &str) -> String {
    format!("iroh-blobs://ticket/{}#{}", ticket, sha256)
}

pub fn parse_blob_ticket_uri(uri: &str) -> Result<(BlobTicket, String), String> {
    let rest = uri
        .trim()
        .strip_prefix("iroh-blobs://ticket/")
        .ok_or_else(|| format!("URI {uri:?} does not start with iroh-blobs://ticket/"))?;
    let (ticket_str, sha256_str) = rest
        .split_once('#')
        .ok_or_else(|| format!("URI {uri:?} missing #sha256:<digest> fragment"))?;
    let ticket: BlobTicket = ticket_str
        .parse()
        .map_err(|e| format!("parse iroh-blobs ticket: {e}"))?;
    let sha256 = normalize_sha256(sha256_str)?;
    Ok((ticket, sha256))
}

/// Live end-to-end native `iroh-blobs` transfer between a provider endpoint and
/// a fetcher endpoint over QUIC, verifying SHA-256 before storing into the target cache.
pub async fn transfer_blob_via_iroh_ticket(
    payload: &[u8],
    work_dir: &Path,
    config: &IrohTransportConfig,
) -> Result<BlobTransportReport, String> {
    fs::create_dir_all(work_dir)
        .map_err(|e| format!("create work dir {}: {e}", work_dir.display()))?;
    let provider_cache = work_dir.join("provider-cache");
    let fetcher_cache = work_dir.join("fetcher-cache");

    let (sha256, cached_path) = store_verified_blob(&provider_cache, payload, None)?;
    let cached_bytes = fs::read(&cached_path)
        .map_err(|e| format!("read cached blob {}: {e}", cached_path.display()))?;

    let provider_endpoint = build_endpoint_builder(config)?
        .bind()
        .await
        .map_err(|e| format!("bind provider endpoint: {e}"))?;
    let provider_id = provider_endpoint.id();
    let store = MemStore::new();
    let tag = store
        .blobs()
        .add_slice(&cached_bytes)
        .await
        .map_err(|e| format!("import blob into MemStore: {e}"))?;
    let ticket = BlobTicket::new(provider_endpoint.addr(), tag.hash, tag.format);
    let ticket_uri = format_blob_ticket_uri(&ticket, &sha256);
    let blake3_hash = tag.hash.to_string();

    let blobs_proto = BlobsProtocol::new(&store, None);
    let router = Router::builder(provider_endpoint.clone())
        .accept(iroh_blobs::ALPN, blobs_proto)
        .spawn();

    let (parsed_ticket, expected_sha256) = parse_blob_ticket_uri(&ticket_uri)?;
    let fetcher_endpoint = build_endpoint_builder(config)?
        .bind()
        .await
        .map_err(|e| format!("bind fetcher endpoint: {e}"))?;
    let memory_lookup = MemoryLookup::new();
    memory_lookup.add_endpoint_info(parsed_ticket.addr().clone());
    fetcher_endpoint
        .address_lookup()
        .map_err(|e| format!("address lookup handle: {e}"))?
        .add(memory_lookup);

    let mem_store = MemStore::new();
    let downloader = mem_store.downloader(&fetcher_endpoint);
    tokio::time::timeout(
        Duration::from_secs(10),
        downloader.download(parsed_ticket.hash(), Some(parsed_ticket.addr().id)),
    )
    .await
    .map_err(|_| "iroh-blobs download timed out".to_string())?
    .map_err(|e| format!("iroh-blobs download failed: {e}"))?;

    let (active_address_modes, known_address_modes, remote_info_status) =
        observe_remote_info(&fetcher_endpoint, provider_id).await;

    let exported_tmp = work_dir.join("exported.bin");
    mem_store
        .blobs()
        .export(parsed_ticket.hash(), &exported_tmp)
        .await
        .map_err(|e| format!("export downloaded blob: {e}"))?;
    let downloaded_bytes =
        fs::read(&exported_tmp).map_err(|e| format!("read exported blob: {e}"))?;
    let (verified_sha256, _target_path) =
        store_verified_blob(&fetcher_cache, &downloaded_bytes, Some(&expected_sha256))?;

    fetcher_endpoint.close().await;
    let _ = router.shutdown().await;
    let _ = fs::remove_dir_all(work_dir);

    Ok(BlobTransportReport {
        sha256: verified_sha256,
        blake3_hash,
        provider_endpoint_id: provider_id.to_string(),
        ticket_uri,
        bytes: downloaded_bytes.len() as u64,
        transferred_bytes: downloaded_bytes.len() as u64,
        method: "iroh-blobs-ticket".to_string(),
        fallback_used: false,
        active_address_modes,
        known_address_modes,
        remote_info_status,
    })
}

async fn observe_remote_info(
    endpoint: &iroh::Endpoint,
    provider: iroh::EndpointId,
) -> (Vec<String>, Vec<String>, String) {
    let Some(remote_info) = endpoint.remote_info(provider).await else {
        return (Vec::new(), Vec::new(), "unavailable".to_string());
    };
    let mut active_modes = Vec::new();
    let mut known_modes = Vec::new();
    for addr in remote_info.addrs() {
        let mode = if addr.addr().is_ip() {
            "direct-ip"
        } else if addr.addr().is_relay() {
            "relay"
        } else if addr.addr().is_custom() {
            "custom"
        } else {
            "unknown"
        };
        if !known_modes.iter().any(|m| m == mode) {
            known_modes.push(mode.to_string());
        }
        if matches!(addr.usage(), iroh::endpoint::TransportAddrUsage::Active)
            && !active_modes.iter().any(|m| m == mode)
        {
            active_modes.push(mode.to_string());
        }
    }
    let status = if known_modes.is_empty() {
        "observed-empty"
    } else if active_modes.is_empty() {
        "observed-known-only"
    } else {
        "observed-active"
    };
    (active_modes, known_modes, status.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn sha256_verification_rejects_mismatched_digest() {
        let dir = env::temp_dir().join(format!("dialtone-blob-sha-test-{}", std::process::id()));
        let err = store_verified_blob(
            &dir,
            b"dialtone-artifact",
            Some("sha256:0000000000000000000000000000000000000000000000000000000000000000"),
        )
        .unwrap_err();
        assert!(err.contains("blob sha256 mismatch"));
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn tiered_blob_fetch_prefers_local_cache_then_native_ticket_and_guards_http_fallback() {
        let dir = env::temp_dir().join(format!("dialtone-tiered-blob-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let payload = b"cached-payload-bytes";
        let (sha, _) = store_verified_blob(&dir, payload, None).unwrap();

        let cache_hit = evaluate_tiered_blob_fetch(
            &dir,
            &TieredBlobFetchRequest {
                sha256: sha.clone(),
                uri: format!("iroh-blobs://ticket/fake#{sha}"),
                provider_node: "node-beta".to_string(),
                expected_bytes: 20,
                http_fallback_enabled: false,
            },
        )
        .unwrap();
        assert_eq!(cache_hit.method, "local-cache");
        assert_eq!(cache_hit.transferred_bytes, 0);

        let uncached_sha =
            "sha256:1111111111111111111111111111111111111111111111111111111111111111";
        let native_plan = evaluate_tiered_blob_fetch(
            &dir,
            &TieredBlobFetchRequest {
                sha256: uncached_sha.to_string(),
                uri: format!("iroh-blobs://ticket/blob123#{uncached_sha}"),
                provider_node: "node-beta".to_string(),
                expected_bytes: 4096,
                http_fallback_enabled: false,
            },
        )
        .unwrap();
        assert_eq!(native_plan.method, "iroh-blobs-ticket");
        assert!(native_plan.native_ticket_attempted);

        let rejected = evaluate_tiered_blob_fetch(
            &dir,
            &TieredBlobFetchRequest {
                sha256: uncached_sha.to_string(),
                uri: "http://node-beta:8787/api/artifacts/blob".to_string(),
                provider_node: "node-beta".to_string(),
                expected_bytes: 4096,
                http_fallback_enabled: false,
            },
        )
        .unwrap_err();
        assert!(rejected.contains("native iroh-blobs://ticket/... required"));

        let fallback_plan = evaluate_tiered_blob_fetch(
            &dir,
            &TieredBlobFetchRequest {
                sha256: uncached_sha.to_string(),
                uri: "http://node-beta:8787/api/artifacts/blob".to_string(),
                provider_node: "node-beta".to_string(),
                expected_bytes: 4096,
                http_fallback_enabled: true,
            },
        )
        .unwrap();
        assert_eq!(fallback_plan.method, "http-peer-fallback");
        assert!(fallback_plan.fallback_used);
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn live_iroh_blobs_ticket_transfer_verifies_sha256_and_records_path_evidence() {
        let dir = env::temp_dir().join(format!(
            "dialtone-live-blob-transfer-{}",
            std::process::id()
        ));
        let payload = b"dialtone content-addressed artifact payload over iroh-blobs QUIC";
        let expected_sha = compute_sha256(payload);

        let rt = tokio::runtime::Runtime::new().unwrap();
        let report = rt
            .block_on(transfer_blob_via_iroh_ticket(
                payload,
                &dir,
                &IrohTransportConfig::local_offline(),
            ))
            .unwrap();

        assert_eq!(report.sha256, expected_sha);
        assert_eq!(report.bytes, payload.len() as u64);
        assert!(report.ticket_uri.starts_with("iroh-blobs://ticket/"));
        assert!(report
            .known_address_modes
            .contains(&"direct-ip".to_string()));
    }
}
