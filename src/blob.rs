use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use futures_lite::future::block_on;
use futures_lite::StreamExt;
use iroh::address_lookup::memory::MemoryLookup;
use iroh_blobs::api::blobs::BlobStatus;
use iroh_blobs::api::downloader::Downloader;
use iroh_blobs::store::fs::options::Options as FsOptions;
use iroh_blobs::store::fs::FsStore;
use iroh_blobs::store::GcConfig;
use iroh_blobs::{ticket::BlobTicket, Hash};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

use crate::node::IrohTransportConfig;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct BlobTransportReport {
    pub digest: String,
    pub blake3_hash: String,
    pub provider_node_id: String,
    pub ticket_uri: String,
    pub bytes: u64,
    pub transferred_bytes: u64,
    pub method: String,
    pub fallback_used: bool,
    pub active_address_modes: Vec<String>,
    pub known_address_modes: Vec<String>,
    pub remote_info_status: String,
}

pub fn digest_of(bytes: &[u8]) -> String {
    let digest = Sha256::digest(bytes);
    format!("sha256:{}", crate::hex_encode(&digest))
}

pub fn normalize_digest(raw: &str) -> Result<String, String> {
    let trimmed = raw.trim().to_ascii_lowercase();
    let hex = trimmed.strip_prefix("sha256:").unwrap_or(&trimmed);
    if hex.len() != 64 || !hex.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err(format!("invalid digest digest: {raw:?}"));
    }
    Ok(format!("sha256:{hex}"))
}

/// Tag prefix that names a blob by its SHA-256: `sha256/<hex>` -> BLAKE3 hash.
const SHA256_TAG_PREFIX: &str = "sha256/";
/// Tag pointing at the blob most recently added to the store.
const LATEST_TAG: &str = "latest";
/// How long the node's downloader waits to connect to a provider.
const DOWNLOAD_CONNECT_TIMEOUT: Duration = Duration::from_secs(10);
/// How often unreferenced data (untagged partial downloads) is removed.
const GC_INTERVAL: Duration = Duration::from_secs(600);
/// How long a starting node waits for the store's lock: a node restarted right after SIGTERM
/// waits for the old one to finish shutting down, and a process another thread is spawning
/// can hold a copy of the lock for a moment until it execs.
const LOCK_WAIT: Duration = Duration::from_secs(10);

fn sha256_tag(digest: &str) -> String {
    format!(
        "{SHA256_TAG_PREFIX}{}",
        digest.strip_prefix("sha256:").unwrap_or(digest)
    )
}

/// Take the exclusive lock on `<dir>/store.lock`, retrying for up to `wait` while another
/// process holds it. Runs once, before the store opens, so it sleeps on the calling thread.
fn lock_store(dir: &Path, wait: Duration) -> Result<fs::File, String> {
    let file = fs::File::create(dir.join("store.lock"))
        .map_err(|e| format!("open store lock in {}: {e}", dir.display()))?;
    let start = std::time::Instant::now();
    let deadline = start + wait;
    let mut told = false;
    loop {
        match file.try_lock() {
            Ok(()) => return Ok(file),
            Err(fs::TryLockError::Error(e)) => {
                return Err(format!("lock store {}: {e}", dir.display()))
            }
            Err(fs::TryLockError::WouldBlock) if std::time::Instant::now() < deadline => {
                if !told && start.elapsed() >= Duration::from_secs(1) {
                    eprintln!(
                        "[store] {} is in use; waiting up to {}s for it to be released",
                        dir.display(),
                        wait.as_secs()
                    );
                    told = true;
                }
                std::thread::sleep(Duration::from_millis(50));
            }
            Err(fs::TryLockError::WouldBlock) => {
                return Err(format!(
                    "store {} is in use by another process",
                    dir.display()
                ))
            }
        }
    }
}

/// Blob count, total bytes and the newest digest; kept current on every add.
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct StoreSummary {
    pub blob_keys: usize,
    pub blob_bytes: u64,
    pub latest_digest: Option<String>,
}

/// The node's blob store: iroh-blobs' `FsStore` (BLAKE3, verified streaming), with every
/// blob also named by its SHA-256 as the tag `sha256/<hex>`. Tagged blobs are kept; GC
/// removes anything untagged. One process owns a store directory at a time.
#[derive(Clone)]
pub struct Blobs {
    store: FsStore,
    dir: PathBuf,
    summary: Arc<Mutex<StoreSummary>>,
    /// A throwaway store (see [`Blobs::open_or_private`]), deleted on close.
    private: bool,
    /// Exclusive OS lock on `<dir>/store.lock`, held until `close`. The database itself
    /// waits forever for another process's lock; this one gives up after [`LOCK_WAIT`].
    lock: Arc<Mutex<Option<fs::File>>>,
}

impl std::fmt::Debug for Blobs {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Blobs").field("dir", &self.dir).finish()
    }
}

impl Blobs {
    /// Open (or create) the store in `dir`, importing any blobs left in the old
    /// one-file-per-SHA-256 layout. Waits up to [`LOCK_WAIT`] for another process to let go.
    pub async fn open(dir: &Path) -> Result<Self, String> {
        Self::open_within(dir, LOCK_WAIT).await
    }

    async fn open_within(dir: &Path, wait: Duration) -> Result<Self, String> {
        fs::create_dir_all(dir).map_err(|e| format!("create store {}: {e}", dir.display()))?;
        let lock_file = lock_store(dir, wait)?;
        let mut options = FsOptions::new(dir);
        options.gc = Some(GcConfig {
            interval: GC_INTERVAL,
            add_protected: None,
        });
        let store = FsStore::load_with_opts(dir.join("blobs.db"), options)
            .await
            .map_err(|e| format!("open store {}: {e}", dir.display()))?;
        let blobs = Blobs {
            store,
            dir: dir.to_path_buf(),
            summary: Arc::default(),
            private: false,
            lock: Arc::new(Mutex::new(Some(lock_file))),
        };
        blobs.import_old_layout().await?;
        blobs.refresh_summary().await?;
        Ok(blobs)
    }

    /// [`Blobs::open`] from synchronous code. The store runs on its own runtime, so this is
    /// safe from any thread.
    pub fn open_blocking(dir: &Path) -> Result<Self, String> {
        block_on(Self::open(dir))
    }

    /// Open `dir`, or a private store under the temp dir when another process (the daemon)
    /// already owns it. Used by `index` and one-shot runs that share the daemon's state dir.
    pub async fn open_or_private(dir: &Path) -> Result<Self, String> {
        match Self::open_within(dir, Duration::ZERO).await {
            Ok(blobs) => Ok(blobs),
            Err(err) => {
                let private =
                    std::env::temp_dir().join(format!("dialtone-store-{}", std::process::id()));
                eprintln!(
                    "[store] {err}; using a private store at {}",
                    private.display()
                );
                let _ = fs::remove_dir_all(&private);
                let mut blobs = Self::open(&private).await?;
                blobs.private = true;
                Ok(blobs)
            }
        }
    }

    pub fn dir(&self) -> &Path {
        &self.dir
    }

    /// Flush and close the store, releasing its database for the next process.
    pub async fn close(&self) {
        let _ = self.store.sync_db().await;
        let _ = self.store.shutdown().await;
        if let Ok(mut lock) = self.lock.lock() {
            lock.take();
        }
        if self.private {
            let _ = fs::remove_dir_all(&self.dir);
        }
    }

    pub fn close_blocking(&self) {
        block_on(self.close())
    }

    /// The iroh-blobs store API (serving, downloading).
    pub fn api(&self) -> &iroh_blobs::api::Store {
        &self.store
    }

    pub fn summary(&self) -> StoreSummary {
        self.summary.lock().map(|s| s.clone()).unwrap_or_default()
    }

    /// BLAKE3 hash of a complete blob named by `digest`.
    pub async fn hash_of(&self, digest: &str) -> Option<Hash> {
        let digest = normalize_digest(digest).ok()?;
        let tag = self.store.tags().get(sha256_tag(&digest)).await.ok()??;
        match self.store.blobs().status(tag.hash).await.ok()? {
            BlobStatus::Complete { .. } => Some(tag.hash),
            _ => None,
        }
    }

    pub async fn has(&self, digest: &str) -> bool {
        self.hash_of(digest).await.is_some()
    }

    /// Add `bytes`, checking them against `expected_digest` first. Returns the SHA-256
    /// digest and the BLAKE3 hash. Adding a blob that is already stored changes nothing.
    pub async fn put(
        &self,
        bytes: &[u8],
        expected_digest: Option<&str>,
    ) -> Result<(String, Hash), String> {
        let digest = digest_of(bytes);
        if let Some(expected) = expected_digest {
            let expected = normalize_digest(expected)?;
            if digest != expected {
                return Err(format!(
                    "blob digest mismatch: expected {expected}, observed {digest}"
                ));
            }
        }
        if let Some(hash) = self.hash_of(&digest).await {
            return Ok((digest, hash));
        }
        let added = self
            .store
            .blobs()
            .add_slice(bytes)
            .with_named_tag(sha256_tag(&digest))
            .await
            .map_err(|e| format!("add blob to store: {e}"))?;
        self.note_added(&digest, added.hash, bytes.len() as u64)
            .await?;
        Ok((digest, added.hash))
    }

    /// Name a blob the node's downloader just fetched: verify its SHA-256 and tag it.
    pub async fn adopt(&self, hash: Hash, expected_digest: &str) -> Result<Vec<u8>, String> {
        let expected = normalize_digest(expected_digest)?;
        let bytes = self
            .store
            .blobs()
            .get_bytes(hash)
            .await
            .map_err(|e| format!("read downloaded blob: {e}"))?
            .to_vec();
        let observed = digest_of(&bytes);
        if observed != expected {
            // Left untagged, so GC removes it.
            return Err(format!(
                "blob digest mismatch: expected {expected}, observed {observed}"
            ));
        }
        if self.hash_of(&expected).await.is_none() {
            self.store
                .tags()
                .set(sha256_tag(&expected), hash)
                .await
                .map_err(|e| format!("tag blob: {e}"))?;
            self.note_added(&expected, hash, bytes.len() as u64).await?;
        }
        Ok(bytes)
    }

    /// The bytes of the blob named by `digest`, checked against it.
    pub async fn get(&self, digest: &str) -> Result<Vec<u8>, String> {
        let digest = normalize_digest(digest)?;
        let hash = self
            .hash_of(&digest)
            .await
            .ok_or_else(|| format!("blob {digest} is not in the store"))?;
        let bytes = self
            .store
            .blobs()
            .get_bytes(hash)
            .await
            .map_err(|e| format!("read blob {digest}: {e}"))?
            .to_vec();
        let observed = digest_of(&bytes);
        if observed != digest {
            return Err(format!(
                "stored blob digest mismatch: expected {digest}, observed {observed}"
            ));
        }
        Ok(bytes)
    }

    /// Drop the name `sha256/<hex>`; GC then removes the data unless another tag keeps it.
    pub async fn forget(&self, digest: &str) -> Result<(), String> {
        let digest = normalize_digest(digest)?;
        let Some(hash) = self.hash_of(&digest).await else {
            return Ok(());
        };
        let size = match self.store.blobs().status(hash).await {
            Ok(BlobStatus::Complete { size }) => size,
            _ => 0,
        };
        self.store
            .tags()
            .delete(sha256_tag(&digest))
            .await
            .map_err(|e| format!("forget blob {digest}: {e}"))?;
        if let Ok(mut s) = self.summary.lock() {
            s.blob_keys = s.blob_keys.saturating_sub(1);
            s.blob_bytes = s.blob_bytes.saturating_sub(size);
        }
        Ok(())
    }

    pub fn forget_blocking(&self, digest: &str) -> Result<(), String> {
        block_on(self.forget(digest))
    }

    pub fn put_blocking(
        &self,
        bytes: &[u8],
        expected_digest: Option<&str>,
    ) -> Result<String, String> {
        block_on(self.put(bytes, expected_digest)).map(|(digest, _)| digest)
    }

    pub fn get_blocking(&self, digest: &str) -> Result<Vec<u8>, String> {
        block_on(self.get(digest))
    }

    /// Whether the data for `hash` is complete in the store, named or not (a forgotten blob
    /// stays until GC). A pull of such a blob would not touch the network.
    pub fn has_data_blocking(&self, hash: Hash) -> bool {
        matches!(
            block_on(self.store.blobs().status(hash)),
            Ok(BlobStatus::Complete { .. })
        )
    }

    pub fn has_blocking(&self, digest: &str) -> bool {
        block_on(self.has(digest))
    }

    /// List all complete SHA-256 digests (`sha256:<hex>`) stored in this blob store.
    pub async fn list_digests(&self) -> Result<Vec<String>, String> {
        let mut out = Vec::new();
        let mut tags = self
            .store
            .tags()
            .list_prefix(SHA256_TAG_PREFIX)
            .await
            .map_err(|e| format!("list store tags: {e}"))?;
        while let Some(tag) = tags.next().await {
            let Ok(tag) = tag else { continue };
            if matches!(
                self.store.blobs().status(tag.hash).await,
                Ok(BlobStatus::Complete { .. })
            ) {
                let hex = String::from_utf8_lossy(tag.name.as_ref())
                    .trim_start_matches(SHA256_TAG_PREFIX)
                    .to_string();
                out.push(format!("sha256:{hex}"));
            }
        }
        out.sort();
        out.dedup();
        Ok(out)
    }

    pub fn list_digests_blocking(&self) -> Result<Vec<String>, String> {
        block_on(self.list_digests())
    }

    async fn note_added(&self, digest: &str, hash: Hash, size: u64) -> Result<(), String> {
        self.store
            .tags()
            .set(LATEST_TAG, hash)
            .await
            .map_err(|e| format!("tag latest blob: {e}"))?;
        if let Ok(mut s) = self.summary.lock() {
            s.blob_keys += 1;
            s.blob_bytes += size;
            s.latest_digest = Some(digest.to_string());
        }
        Ok(())
    }

    async fn refresh_summary(&self) -> Result<(), String> {
        let latest = self
            .store
            .tags()
            .get(LATEST_TAG)
            .await
            .map_err(|e| format!("read latest tag: {e}"))?
            .map(|t| t.hash);
        let mut summary = StoreSummary::default();
        let mut tags = self
            .store
            .tags()
            .list_prefix(SHA256_TAG_PREFIX)
            .await
            .map_err(|e| format!("list store tags: {e}"))?;
        while let Some(tag) = tags.next().await {
            let Ok(tag) = tag else { continue };
            let Ok(BlobStatus::Complete { size }) = self.store.blobs().status(tag.hash).await
            else {
                continue;
            };
            summary.blob_keys += 1;
            summary.blob_bytes += size;
            if Some(tag.hash) == latest {
                let hex = String::from_utf8_lossy(tag.name.as_ref())
                    .trim_start_matches(SHA256_TAG_PREFIX)
                    .to_string();
                summary.latest_digest = Some(format!("sha256:{hex}"));
            }
        }
        if let Ok(mut s) = self.summary.lock() {
            *s = summary;
        }
        Ok(())
    }

    /// Import blobs from the old layout (`<dir>/<hex[..2]>/<hex>`, one file per SHA-256),
    /// oldest first so the newest stays `latest`, then remove the old files.
    async fn import_old_layout(&self) -> Result<(), String> {
        let mut old = Vec::new();
        for shard in fs::read_dir(&self.dir).into_iter().flatten().flatten() {
            let name = shard.file_name().to_string_lossy().to_string();
            if name.len() != 2 || !name.bytes().all(|b| b.is_ascii_hexdigit()) {
                continue;
            }
            for file in fs::read_dir(shard.path()).into_iter().flatten().flatten() {
                let hex = file.file_name().to_string_lossy().to_string();
                if normalize_digest(&hex).is_ok() {
                    let mtime = file.metadata().and_then(|m| m.modified()).ok();
                    old.push((mtime, hex, file.path()));
                }
            }
        }
        old.sort();
        for (_, hex, path) in old {
            match fs::read(&path) {
                Ok(bytes) if digest_of(&bytes) == format!("sha256:{hex}") => {
                    self.put(&bytes, Some(&hex)).await?;
                }
                _ => eprintln!("[store] skipped corrupt old blob {}", path.display()),
            }
            let _ = fs::remove_file(&path);
            if let Some(shard) = path.parent() {
                let _ = fs::remove_dir(shard);
            }
        }
        Ok(())
    }
}

/// Format a native `iroh-blobs` ticket URI with its canonical SHA-256 verification anchor.
pub fn format_blob_ticket_uri(ticket: &BlobTicket, digest: &str) -> String {
    format!("iroh-blobs://ticket/{}#{}", ticket, digest)
}

pub fn parse_blob_ticket_uri(uri: &str) -> Result<(BlobTicket, String), String> {
    let rest = uri
        .trim()
        .strip_prefix("iroh-blobs://ticket/")
        .ok_or_else(|| format!("URI {uri:?} does not start with iroh-blobs://ticket/"))?;
    let (ticket_str, digest_str) = rest
        .split_once('#')
        .ok_or_else(|| format!("URI {uri:?} missing #sha256:<digest> fragment"))?;
    let ticket: BlobTicket = ticket_str
        .parse()
        .map_err(|e| format!("parse iroh-blobs ticket: {e}"))?;
    let digest = normalize_digest(digest_str)?;
    Ok((ticket, digest))
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

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct SharedBlobTicketReport {
    pub status: String,
    pub digest: String,
    pub blake3_hash: String,
    pub bytes: u64,
    pub provider_node_id: String,
    pub ticket_uri: String,
}

/// The node's blob service: its store, served on its endpoint, and its one downloader.
#[derive(Clone)]
pub struct Store {
    pub endpoint: iroh::Endpoint,
    pub blobs: Blobs,
    pub downloader: Downloader,
    pub memory_lookup: MemoryLookup,
    pub transport_config: IrohTransportConfig,
    pub runtime_handle: tokio::runtime::Handle,
}

impl std::fmt::Debug for Store {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Store")
            .field("node_id", &self.endpoint.id().to_string())
            .finish()
    }
}

impl Store {
    /// Must be called inside the node's tokio runtime (the downloader spawns its actor).
    pub fn new(
        endpoint: iroh::Endpoint,
        blobs: Blobs,
        memory_lookup: MemoryLookup,
        transport_config: IrohTransportConfig,
    ) -> Self {
        // iroh-blobs gives a connection 1s by default; a relayed path or a high-Rtt link
        // needs longer to handshake, and a pooled connection is reused across pulls.
        let mut pool = iroh_blobs::util::connection_pool::Options::default();
        pool.connect_timeout = DOWNLOAD_CONNECT_TIMEOUT;
        pool.idle_timeout = Duration::from_secs(30);
        let downloader = Downloader::builder(blobs.api(), &endpoint)
            .pool_options(pool)
            .build();
        Store {
            endpoint,
            blobs,
            downloader,
            memory_lookup,
            transport_config,
            runtime_handle: tokio::runtime::Handle::current(),
        }
    }

    /// Run `fut` on the node's runtime from a synchronous (HTTP) thread.
    fn run<T: Send + 'static>(
        &self,
        fut: impl std::future::Future<Output = Result<T, String>> + Send + 'static,
    ) -> Result<T, String> {
        block_on(self.runtime_handle.spawn(fut)).map_err(|_| "blob task panicked".to_string())?
    }

    /// Add `bytes` to the store and return a ticket other nodes can pull it with.
    pub fn share_blob(
        &self,
        bytes: &[u8],
        expected_digest: Option<&str>,
    ) -> Result<SharedBlobTicketReport, String> {
        let (digest, hash) = block_on(self.blobs.put(bytes, expected_digest))?;
        let addr =
            crate::node::advertised_endpoint_addr(&self.endpoint.addr(), &self.transport_config);
        let ticket = BlobTicket::new(addr, hash, iroh_blobs::BlobFormat::Raw);
        Ok(SharedBlobTicketReport {
            status: "shared".to_string(),
            ticket_uri: format_blob_ticket_uri(&ticket, &digest),
            digest,
            blake3_hash: hash.to_string(),
            bytes: bytes.len() as u64,
            provider_node_id: self.endpoint.id().to_string(),
        })
    }

    /// Fetch the blob a ticket names with the node's downloader (unless it is already
    /// stored), verify its SHA-256 and keep it.
    pub fn pull_blob(
        &self,
        ticket_uri: &str,
        force_network: bool,
    ) -> Result<BlobTransportReport, String> {
        self.pull_blob_from(ticket_uri, force_network, Vec::new())
    }

    /// Like `pull_blob`, but ask `others` first, in order, and the ticket's provider last:
    /// a blob is content-addressed, so any node that holds it can serve it.
    pub fn pull_blob_from(
        &self,
        ticket_uri: &str,
        force_network: bool,
        others: Vec<iroh::EndpointId>,
    ) -> Result<BlobTransportReport, String> {
        let (ticket, expected_digest) = parse_blob_ticket_uri(ticket_uri)?;
        let provider_id = ticket.addr().id;
        let report = |bytes: u64, transferred: u64, mode: &str, status: &str| BlobTransportReport {
            digest: expected_digest.clone(),
            blake3_hash: ticket.hash().to_string(),
            provider_node_id: provider_id.to_string(),
            ticket_uri: ticket_uri.to_string(),
            bytes,
            transferred_bytes: transferred,
            method: mode.to_string(),
            fallback_used: false,
            active_address_modes: vec![mode.to_string()],
            known_address_modes: vec![mode.to_string()],
            remote_info_status: status.to_string(),
        };

        // A node never dials itself: a ticket it issued is read from its own store.
        let self_ticket = provider_id == self.endpoint.id();
        if self_ticket || !force_network {
            if let Ok(bytes) = block_on(self.blobs.get(&expected_digest)) {
                return Ok(if self_ticket {
                    report(bytes.len() as u64, 0, "local-self", "self")
                } else {
                    report(bytes.len() as u64, 0, "local-cache", "cache-hit")
                });
            }
            if self_ticket {
                return Err(format!(
                    "blob {expected_digest} is not in this node's store"
                ));
            }
        }

        self.memory_lookup.add_endpoint_info(ticket.addr().clone());
        let blobs = self.blobs.clone();
        let downloader = self.downloader.clone();
        let endpoint = self.endpoint.clone();
        let hash = ticket.hash();
        let digest = expected_digest.clone();
        let own = self.endpoint.id();
        let mut providers: Vec<iroh::EndpointId> = others
            .into_iter()
            .filter(|p| *p != provider_id && *p != own)
            .collect();
        providers.push(provider_id);
        // Each provider that does not answer costs up to the connect timeout.
        let timeout =
            Duration::from_secs(15) + DOWNLOAD_CONNECT_TIMEOUT * (providers.len() as u32 - 1);
        let (len, active, known, status) = self.run(async move {
            // Fetched over this node's own endpoint, so the provider sees our NodeId and its
            // allowlist decides.
            tokio::time::timeout(timeout, downloader.download(hash, providers))
                .await
                .map_err(|_| format!("iroh-blobs download from peer {provider_id} timed out"))?
                .map_err(|e| {
                    format!("iroh-blobs download from peer {provider_id} failed (is this node on its allowlist?): {e}")
                })?;
            let (active, known, status) = observe_remote_info(&endpoint, provider_id).await;
            let bytes = blobs.adopt(hash, &digest).await?;
            Ok((bytes.len() as u64, active, known, status))
        })?;
        let mut rep = report(len, len, "iroh-blobs-ticket", &status);
        rep.active_address_modes = active;
        rep.known_address_modes = known;
        Ok(rep)
    }
}

/// Channel-scoped wrapper around `iroh_blobs::BlobsProtocol` that enforces channel membership
/// on every incoming QUIC connection before delegating to the underlying blob store, and tracks
/// active weak handles so revoked peers are disconnected immediately on channel switch.
#[derive(Clone, Debug)]
pub struct ChannelBlobsProtocol {
    channel_id: String,
    inner: iroh_blobs::BlobsProtocol,
    allowlist: crate::mesh::LiveAllowlist,
    members: crate::mesh::LiveAllowlist,
    active_conns: Arc<Mutex<Vec<(iroh::EndpointId, iroh::endpoint::WeakConnectionHandle)>>>,
}

impl ChannelBlobsProtocol {
    pub fn new(
        channel_id: impl Into<String>,
        inner: iroh_blobs::BlobsProtocol,
        allowlist: crate::mesh::LiveAllowlist,
        members: crate::mesh::LiveAllowlist,
    ) -> Self {
        Self {
            channel_id: channel_id.into(),
            inner,
            allowlist,
            members,
            active_conns: Arc::new(Mutex::new(Vec::new())),
        }
    }

    pub fn channel_id(&self) -> &str {
        &self.channel_id
    }

    pub fn admits(&self, peer: &iroh::EndpointId) -> bool {
        if self.allowlist.is_empty() {
            self.members.contains(peer)
        } else {
            self.allowlist.contains(peer)
        }
    }

    /// Close all active pooled blob connections whose peers are no longer admitted to this channel.
    pub fn disconnect_unauthorized(&self) {
        if let Ok(mut conns) = self.active_conns.lock() {
            conns.retain(|(peer, weak)| {
                let Some(conn) = weak.upgrade() else {
                    return false;
                };
                if !self.admits(peer) {
                    conn.close(
                        crate::node::NOT_ALLOWLISTED_CLOSE_CODE.into(),
                        b"channel blob access revoked",
                    );
                    return false;
                }
                true
            });
        }
    }
}

impl iroh::protocol::ProtocolHandler for ChannelBlobsProtocol {
    async fn accept(
        &self,
        connection: iroh::endpoint::Connection,
    ) -> Result<(), iroh::protocol::AcceptError> {
        let remote = connection.remote_id();
        if !self.admits(&remote) {
            connection.close(
                crate::node::NOT_ALLOWLISTED_CLOSE_CODE.into(),
                b"channel blob access denied",
            );
            return Ok(());
        }
        if let Ok(mut conns) = self.active_conns.lock() {
            conns.retain(|(_, w)| w.upgrade().is_some());
            conns.push((remote, connection.weak_handle()));
        }
        self.inner.accept(connection).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::node::build_endpoint_builder;
    use iroh::protocol::Router;
    use iroh_blobs::store::mem::MemStore;
    use iroh_blobs::BlobsProtocol;
    use std::env;

    fn temp_store(name: &str) -> (PathBuf, Blobs) {
        let dir = env::temp_dir().join(format!("dialtone-{name}-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let blobs = Blobs::open_blocking(&dir).unwrap();
        (dir, blobs)
    }

    #[test]
    fn put_names_blobs_by_sha256_and_rejects_a_mismatched_digest() {
        let (dir, blobs) = temp_store("blob-put");
        let err = blobs
            .put_blocking(
                b"dialtone-blob",
                Some("sha256:0000000000000000000000000000000000000000000000000000000000000000"),
            )
            .unwrap_err();
        assert!(err.contains("blob digest mismatch"));
        assert_eq!(blobs.summary(), StoreSummary::default());

        let digest = blobs.put_blocking(b"dialtone-blob", None).unwrap();
        assert_eq!(digest, digest_of(b"dialtone-blob"));
        assert!(blobs.has_blocking(&digest));
        assert_eq!(blobs.get_blocking(&digest).unwrap(), b"dialtone-blob");
        // Adding it again is a no-op: the count and `latest` stay put.
        blobs.put_blocking(b"other", None).unwrap();
        blobs.put_blocking(b"dialtone-blob", None).unwrap();
        let summary = blobs.summary();
        assert_eq!((summary.blob_keys, summary.blob_bytes), (2, 18));
        assert_eq!(summary.latest_digest, Some(digest_of(b"other")));
        assert!(!blobs.has_blocking(&digest_of(b"never stored")));
        drop(blobs);
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn a_store_in_use_is_waited_for_briefly_never_forever() {
        let (dir, blobs) = temp_store("blob-locked");
        let start = std::time::Instant::now();
        let err = block_on(Blobs::open_within(&dir, Duration::ZERO))
            .expect_err("a second open with no wait must fail");
        assert!(err.contains("in use by another process"), "{err}");
        assert!(start.elapsed() < Duration::from_secs(2));
        let err = block_on(Blobs::open_within(&dir, Duration::from_millis(300)))
            .expect_err("a second open must give up once the wait is over");
        assert!(err.contains("in use by another process"), "{err}");

        // A node restarted right after SIGTERM waits for the old one to close the store.
        let start = std::time::Instant::now();
        let closer = std::thread::spawn(move || {
            std::thread::sleep(Duration::from_millis(400));
            blobs.close_blocking();
        });
        let reopened = Blobs::open_blocking(&dir).expect("the store is free once closed");
        assert!(start.elapsed() >= Duration::from_millis(300));
        closer.join().unwrap();
        reopened.close_blocking();
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn store_survives_a_reopen_and_imports_the_old_file_layout() {
        let dir = env::temp_dir().join(format!("dialtone-blob-reopen-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        // The old layout: one file per blob at <dir>/<hex[..2]>/<hex>, newest by mtime.
        let base = std::time::SystemTime::UNIX_EPOCH + Duration::from_secs(1_700_000_000);
        let write_old = |bytes: &[u8], age: u64| {
            let digest = digest_of(bytes);
            let hex = digest.trim_start_matches("sha256:").to_string();
            fs::create_dir_all(dir.join(&hex[..2])).unwrap();
            let path = dir.join(&hex[..2]).join(&hex);
            fs::write(&path, bytes).unwrap();
            let file = fs::File::options().write(true).open(&path).unwrap();
            file.set_modified(base + Duration::from_secs(age)).unwrap();
            digest
        };
        let older = write_old(b"older program", 0);
        let newer = write_old(b"newer program", 60);
        fs::create_dir_all(dir.join("ab")).unwrap();
        fs::write(dir.join("ab").join("a".repeat(64)), b"corrupt").unwrap();

        let blobs = Blobs::open_blocking(&dir).unwrap();
        assert_eq!(blobs.get_blocking(&older).unwrap(), b"older program");
        assert_eq!(blobs.summary().latest_digest, Some(newer.clone()));
        assert_eq!(blobs.summary().blob_keys, 2, "the corrupt file is skipped");
        let left: Vec<_> = fs::read_dir(&dir)
            .unwrap()
            .flatten()
            .map(|e| e.file_name().to_string_lossy().to_string())
            .filter(|n| n.len() == 2)
            .collect();
        assert!(
            left.is_empty(),
            "old files are removed after import: {left:?}"
        );
        blobs.close_blocking();

        let reopened = Blobs::open_blocking(&dir).unwrap();
        assert_eq!(reopened.get_blocking(&newer).unwrap(), b"newer program");
        assert_eq!(reopened.summary().latest_digest, Some(newer));
        assert_eq!(reopened.summary().blob_keys, 2);
        reopened.close_blocking();
        let _ = fs::remove_dir_all(&dir);
    }

    /// The node's downloader fetches over QUIC, the SHA-256 is checked and the blob is named.
    #[test]
    fn pull_downloads_verifies_and_tags_a_peers_blob() {
        let (provider_dir, provider_blobs) = temp_store("blob-provider");
        let (fetcher_dir, fetcher_blobs) = temp_store("blob-fetcher");
        let rt = tokio::runtime::Runtime::new().unwrap();
        let (provider, fetcher, router) = rt.block_on(async {
            let bind = || async {
                build_endpoint_builder(&IrohTransportConfig::local_offline())
                    .unwrap()
                    .bind()
                    .await
                    .unwrap()
            };
            let (p_ep, f_ep) = (bind().await, bind().await);
            let router = Router::builder(p_ep.clone())
                .accept(
                    iroh_blobs::ALPN,
                    BlobsProtocol::new(provider_blobs.api(), None),
                )
                .spawn();
            let cfg = IrohTransportConfig::local_offline();
            let provider = Store::new(
                p_ep,
                provider_blobs.clone(),
                MemoryLookup::new(),
                cfg.clone(),
            );
            let lookup = MemoryLookup::new();
            f_ep.address_lookup().unwrap().add(lookup.clone());
            let fetcher = Store::new(f_ep, fetcher_blobs.clone(), lookup, cfg);
            (provider, fetcher, router)
        });

        let shared = provider.share_blob(b"program bytes", None).unwrap();
        let pulled = fetcher.pull_blob(&shared.ticket_uri, false).unwrap();
        assert_eq!(pulled.method, "iroh-blobs-ticket");
        assert_eq!(pulled.digest, digest_of(b"program bytes"));
        assert_eq!(
            fetcher_blobs.get_blocking(&pulled.digest).unwrap(),
            b"program bytes"
        );
        assert_eq!(
            fetcher_blobs.summary().latest_digest,
            Some(pulled.digest.clone())
        );
        let again = fetcher.pull_blob(&shared.ticket_uri, false).unwrap();
        assert_eq!(again.method, "local-cache");

        // A ticket whose SHA-256 anchor does not match its content is refused and not kept.
        let (ticket, _) = parse_blob_ticket_uri(&shared.ticket_uri).unwrap();
        let lie = format_blob_ticket_uri(&ticket, &digest_of(b"something else"));
        let err = fetcher.pull_blob(&lie, true).unwrap_err();
        assert!(err.contains("digest mismatch"), "{err}");
        assert!(!fetcher_blobs.has_blocking(&digest_of(b"something else")));

        rt.block_on(async {
            let _ = router.shutdown().await;
        });
        drop((provider, fetcher, provider_blobs, fetcher_blobs));
        let _ = fs::remove_dir_all(&provider_dir);
        let _ = fs::remove_dir_all(&fetcher_dir);
    }

    /// The handshake hook is what makes the allowlist real: a node on the provider's
    /// allowlist can fetch a blob, any other NodeId is closed right after the TLS handshake.
    #[test]
    fn allowlist_hook_admits_listed_nodes_and_closes_everyone_else() {
        use crate::mesh::LiveAllowlist;
        use crate::node::AllowlistHook;

        async fn fetch(
            key: iroh::SecretKey,
            provider: iroh::EndpointAddr,
            hash: iroh_blobs::Hash,
        ) -> Result<Vec<u8>, String> {
            let lookup = MemoryLookup::new();
            lookup.add_endpoint_info(provider.clone());
            let ep = build_endpoint_builder(&IrohTransportConfig::local_offline())?
                .address_lookup(lookup)
                .secret_key(key)
                .bind()
                .await
                .map_err(|e| e.to_string())?;
            let store = MemStore::new();
            let res = tokio::time::timeout(
                Duration::from_secs(5),
                store.downloader(&ep).download(hash, Some(provider.id)),
            )
            .await;
            let out = match res {
                Ok(Ok(_)) => store
                    .blobs()
                    .get_bytes(hash)
                    .await
                    .map(|b| b.to_vec())
                    .map_err(|e| e.to_string()),
                Ok(Err(e)) => Err(e.to_string()),
                Err(_) => Err("timed out".to_string()),
            };
            ep.close().await;
            out
        }

        let rt = tokio::runtime::Runtime::new().unwrap();
        rt.block_on(async {
            let allowed = iroh::SecretKey::generate();
            let stranger = iroh::SecretKey::generate();
            let provider_ep = build_endpoint_builder(&IrohTransportConfig::local_offline())
                .unwrap()
                .hooks(AllowlistHook::new(LiveAllowlist::new(&[allowed
                    .public()
                    .to_string()])))
                .bind()
                .await
                .unwrap();
            let provider_store = MemStore::new();
            let tag = provider_store
                .blobs()
                .add_slice(b"only for allowlisted nodes")
                .await
                .unwrap();
            let router = Router::builder(provider_ep.clone())
                .accept(iroh_blobs::ALPN, BlobsProtocol::new(&provider_store, None))
                .spawn();
            let provider_addr = provider_ep.addr();

            let got = fetch(allowed, provider_addr.clone(), tag.hash)
                .await
                .unwrap();
            assert_eq!(got, b"only for allowlisted nodes");
            assert!(fetch(stranger, provider_addr, tag.hash).await.is_err());
            let _ = router.shutdown().await;
        });
    }

    /// A blob stored on Channel A is visible and fetchable by Channel A members,
    /// and neither listable nor fetchable from a node on Channel B even with the ticket/hash.
    #[test]
    fn channel_scoped_blob_is_not_listable_or_fetchable_from_other_channel() {
        use crate::mesh::{channel_blobs_dir, derive_channel_keys_v2, LiveAllowlist};

        let root = env::temp_dir().join(format!(
            "dialtone-blob-channel-scope-{}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&root);

        let keys_a = derive_channel_keys_v2("alpha", "dt-Blob_Scope_Alpha_Secret_2026!");
        let keys_b = derive_channel_keys_v2("beta", "dt-Blob_Scope_Beta_Secret_2026!");
        let cid_a = keys_a.fingerprint();
        let cid_b = keys_b.fingerprint();

        let blobs_a1 = Blobs::open_blocking(&channel_blobs_dir(&root.join("a1"), &cid_a)).unwrap();
        let blobs_a2 = Blobs::open_blocking(&channel_blobs_dir(&root.join("a2"), &cid_a)).unwrap();
        let blobs_b1 = Blobs::open_blocking(&channel_blobs_dir(&root.join("b1"), &cid_b)).unwrap();
        // Same-host channel B store under a1's state_dir must also be isolated from channel A.
        let blobs_a1_chan_b =
            Blobs::open_blocking(&channel_blobs_dir(&root.join("a1"), &cid_b)).unwrap();

        let members_a = LiveAllowlist::default();
        let empty_allowlist = LiveAllowlist::default();

        let rt = tokio::runtime::Runtime::new().unwrap();
        let (store_a1, store_a2, store_b1, proto_a1, router_a1) = rt.block_on(async {
            let bind = || async {
                build_endpoint_builder(&IrohTransportConfig::local_offline())
                    .unwrap()
                    .bind()
                    .await
                    .unwrap()
            };
            let (ep_a1, ep_a2, ep_b1) = (bind().await, bind().await, bind().await);
            // Only ep_a2 is a verified member of Channel A; ep_b1 is on Channel B.
            members_a.insert(ep_a2.id());

            let proto_a1 = ChannelBlobsProtocol::new(
                cid_a.clone(),
                BlobsProtocol::new(blobs_a1.api(), None),
                empty_allowlist.clone(),
                members_a.clone(),
            );
            let router_a1 = Router::builder(ep_a1.clone())
                .accept(iroh_blobs::ALPN, proto_a1.clone())
                .spawn();

            let cfg = IrohTransportConfig::local_offline();
            let store_a1 = Store::new(ep_a1, blobs_a1.clone(), MemoryLookup::new(), cfg.clone());
            let lookup_a2 = MemoryLookup::new();
            ep_a2.address_lookup().unwrap().add(lookup_a2.clone());
            let store_a2 = Store::new(ep_a2, blobs_a2.clone(), lookup_a2, cfg.clone());

            let lookup_b1 = MemoryLookup::new();
            ep_b1.address_lookup().unwrap().add(lookup_b1.clone());
            let store_b1 = Store::new(ep_b1, blobs_b1.clone(), lookup_b1, cfg);
            (store_a1, store_a2, store_b1, proto_a1, router_a1)
        });

        let payload = b"channel-a-confidential-artifact";
        let shared = store_a1.share_blob(payload, None).unwrap();

        // Channel A store lists the blob; Channel B stores do NOT list it.
        assert!(blobs_a1.list_digests_blocking().unwrap().contains(&shared.digest));
        assert!(blobs_a1_chan_b.list_digests_blocking().unwrap().is_empty());
        assert!(blobs_b1.list_digests_blocking().unwrap().is_empty());

        // Channel A member (A2) CAN fetch the blob.
        let pulled_a2 = store_a2.pull_blob(&shared.ticket_uri, true).unwrap();
        assert_eq!(pulled_a2.digest, shared.digest);
        assert_eq!(blobs_a2.get_blocking(&shared.digest).unwrap(), payload);

        // Channel B node (B1) CANNOT fetch Channel A's blob even with the full ticket/hash.
        let err_b1 = store_b1.pull_blob(&shared.ticket_uri, true).unwrap_err();
        assert!(
            err_b1.contains("failed") || err_b1.contains("timed out"),
            "unexpected error: {err_b1}"
        );
        assert!(!blobs_b1.has_blocking(&shared.digest));
        assert!(blobs_b1.list_digests_blocking().unwrap().is_empty());

        // Clearing channel members and disconnecting unauthorized pooled connections
        // immediately prevents A2 from fetching any new blob on Channel A.
        let payload2 = b"channel-a-post-revocation-artifact";
        let shared2 = store_a1.share_blob(payload2, None).unwrap();
        members_a.clear();
        proto_a1.disconnect_unauthorized();
        let err_a2_revoked = store_a2.pull_blob(&shared2.ticket_uri, true).unwrap_err();
        assert!(
            err_a2_revoked.contains("failed") || err_a2_revoked.contains("timed out"),
            "revoked peer should be denied: {err_a2_revoked}"
        );
        assert!(!blobs_a2.has_blocking(&shared2.digest));

        rt.block_on(async {
            let _ = router_a1.shutdown().await;
        });
        drop((store_a1, store_a2, store_b1));
        blobs_a1.close_blocking();
        blobs_a2.close_blocking();
        blobs_b1.close_blocking();
        blobs_a1_chan_b.close_blocking();
        let _ = fs::remove_dir_all(&root);
    }
}
