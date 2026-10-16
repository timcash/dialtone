use std::fs;
use std::path::Path;
use std::str::FromStr;
use std::time::Duration;

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

use crate::evidence::{sample_live_host_resources, HostResourceTelemetry};
use crate::identity::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, is_tailnet_ip,
    IrohTransportConfig, IrohTransportDiagnostics,
};
use crate::refs::ServiceAdvertisement;

use sha2::{Digest, Sha256};

pub const DIALTONE_GOSSIP_TOPIC_BYTES: &[u8; 32] = b"dialtone-iroh-gossip-topic-00001";
pub const DIALTONE_HEARTBEAT_SCHEMA: &str = "dialtone.iroh.heartbeat.v1";
pub const DEFAULT_CHANNEL_NAME: &str = "dialtone";
pub const DEFAULT_PEER_TTL_SECS: u64 = 300;

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

/// Validate a password for the `"dialtone"` channel (or any named channel).
/// Rejects empty strings as well as the public default strings `"dialtone"` and `"default"`
/// so unauthorized peers cannot enter the main channel without knowing the password.
pub fn validate_channel_password(password: &str) -> Result<String, String> {
    let trimmed = password.trim();
    if trimmed.is_empty()
        || trimmed.eq_ignore_ascii_case("dialtone")
        || trimmed.eq_ignore_ascii_case("default")
        || trimmed == "dialtone-iroh-gossip-topic-00001"
    {
        return Err(
            "a channel password is required to join the 'dialtone' channel (cannot be empty, 'dialtone', or 'default')"
                .to_string(),
        );
    }
    Ok(trimmed.to_string())
}

/// Derive a node-private standby `TopicId` when no channel password has been configured yet.
pub fn derive_standby_topic_id(endpoint_id: &str) -> TopicId {
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.standby.v1:");
    hasher.update(endpoint_id.trim().as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    TopicId::from_bytes(digest)
}

/// Deterministically derive an `iroh-gossip` 32-byte `TopicId` from `(channel, password)`.
pub fn derive_channel_topic_id(channel: &str, password: &str) -> TopicId {
    let norm_channel = normalize_channel_name(Some(channel));
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.topic.v1:");
    hasher.update(norm_channel.as_bytes());
    hasher.update(b":");
    hasher.update(password.trim().as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    TopicId::from_bytes(digest)
}

/// Derive a separate 32-byte authentication subkey from `(channel, password)` for signing and
/// verifying `DialtoneHeartbeat` frames over `iroh-gossip`. This subkey is never placed in
/// tickets or exposed over HTTP.
pub fn derive_channel_auth_key(channel: &str, password: &str) -> [u8; 32] {
    let norm_channel = normalize_channel_name(Some(channel));
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.channel.auth.v1:");
    hasher.update(norm_channel.as_bytes());
    hasher.update(b":");
    hasher.update(password.trim().as_bytes());
    hasher.finalize().into()
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

/// Returns `true` if `endpoint_id` is permitted by the host's local `allowed_endpoint_ids`
/// list in `~/.dialtone/config.json` (or `true` when no allowlist restriction is configured).
pub fn is_allowed_main_fleet_peer(endpoint_id: &str) -> bool {
    let cfg = load_mesh_config(&default_config_path());
    if cfg.allowed_endpoint_ids.is_empty() {
        return true;
    }
    let candidate = endpoint_id.trim();
    cfg.allowed_endpoint_ids
        .iter()
        .any(|allowed| allowed.trim().eq_ignore_ascii_case(candidate))
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
pub struct PersistentMeshConfig {
    pub node_name: Option<String>,
    pub listen_addr: Option<String>,
    #[serde(default = "default_channel_name")]
    pub channel: String,
    /// Masked/configured indicator (`true` when user has configured a valid channel password).
    pub configured: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub secret_key: Option<String>,
    pub channel_fingerprint: String,
    #[serde(default)]
    pub seed_peers: Vec<SeedPeerConfig>,
    #[serde(default)]
    pub allowed_endpoint_ids: Vec<String>,
    pub bootstrap_tickets: Vec<String>,
    pub daemon_installed: bool,
    pub daemon_service_path: Option<String>,
    #[serde(default)]
    pub managed_processes: Vec<crate::metadata::ManagedProcessSpec>,
    pub updated_at_unix: u64,
}

#[derive(Clone, Debug)]
pub enum MeshControlCommand {
    JoinMesh {
        channel: String,
        secret_key: String,
        bootstrap_tickets: Vec<String>,
        node_name: Option<String>,
    },
}

pub fn default_state_dir() -> std::path::PathBuf {
    if let Some(custom) = std::env::var_os("DIALTONE_STATE_DIR") {
        let p = std::path::PathBuf::from(custom);
        if !p.as_os_str().is_empty() {
            return p;
        }
    }
    let base = std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(std::path::PathBuf::from)
        .unwrap_or_else(std::env::temp_dir);
    base.join(".dialtone")
}

pub fn default_config_path() -> std::path::PathBuf {
    default_state_dir().join("config.json")
}

pub fn default_blob_cache_dir() -> std::path::PathBuf {
    default_state_dir().join("blobs")
}

pub fn load_mesh_config(path: &Path) -> PersistentMeshConfig {
    match fs::read_to_string(path) {
        Ok(raw) => serde_json::from_str(&raw).unwrap_or_default(),
        Err(_) => PersistentMeshConfig::default(),
    }
}

pub fn save_mesh_config(path: &Path, cfg: &PersistentMeshConfig) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|e| format!("create config dir {}: {e}", parent.display()))?;
    }
    let json = serde_json::to_string_pretty(cfg).map_err(|e| e.to_string())?;
    fs::write(path, json).map_err(|e| format!("write config {}: {e}", path.display()))?;
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

/// Compute a 32-byte hex MAC authenticating `(channel, node, endpoint_id, counter, updated_at_unix)`
/// using the secret `auth_key` derived from `(channel, password)`.
pub fn compute_heartbeat_mac(hb: &DialtoneHeartbeat, channel: &str, password: &str) -> String {
    let norm_channel = normalize_channel_name(Some(channel));
    let auth_key = derive_channel_auth_key(&norm_channel, password);
    let mut hasher = Sha256::new();
    hasher.update(b"dialtone.heartbeat.mac.v1:");
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

/// Sign `DialtoneHeartbeat` in-place with the `(channel, password)` MAC.
pub fn sign_heartbeat_with_password(hb: &mut DialtoneHeartbeat, channel: &str, password: &str) {
    if !password.trim().is_empty() {
        hb.channel_mac = Some(compute_heartbeat_mac(hb, channel, password));
    }
}

/// Verify in constant time that `DialtoneHeartbeat` carries a valid MAC for `(channel, password)`.
pub fn verify_heartbeat_password(hb: &DialtoneHeartbeat, channel: &str, password: &str) -> bool {
    if password.trim().is_empty() {
        return false;
    }
    let Some(ref provided_mac) = hb.channel_mac else {
        return false;
    };
    let expected_mac = compute_heartbeat_mac(hb, channel, password);
    if provided_mac.len() != expected_mac.len() {
        return false;
    }
    let mut diff = 0u8;
    for (a, b) in provided_mac.as_bytes().iter().zip(expected_mac.as_bytes()) {
        diff |= a ^ b;
    }
    diff == 0
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

/// Live in-process two-peer `iroh-gossip` heartbeat exchange over QUIC protected by a channel password.
pub async fn verify_live_gossip_exchange(
    node_a: &str,
    node_b: &str,
) -> Result<(DialtoneHeartbeat, DialtoneHeartbeat), String> {
    let config = IrohTransportConfig::local_offline();
    let test_password = "self-test-channel-password";
    let topic_id = derive_channel_topic_id(DEFAULT_CHANNEL_NAME, test_password);

    let lookup_a = MemoryLookup::new();
    let endpoint_a = build_endpoint_builder(&config)?
        .address_lookup(lookup_a.clone())
        .bind()
        .await
        .map_err(|e| format!("bind endpoint A: {e}"))?;
    let gossip_a = Gossip::builder().spawn(endpoint_a.clone());
    let router_a = Router::builder(endpoint_a.clone())
        .accept(GOSSIP_ALPN, gossip_a.clone())
        .spawn();

    let lookup_b = MemoryLookup::new();
    let endpoint_b = build_endpoint_builder(&config)?
        .address_lookup(lookup_b.clone())
        .bind()
        .await
        .map_err(|e| format!("bind endpoint B: {e}"))?;
    let gossip_b = Gossip::builder().spawn(endpoint_b.clone());
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
    sign_heartbeat_with_password(&mut hb_a, DEFAULT_CHANNEL_NAME, test_password);
    let payload_a = serde_json::to_vec(&hb_a).map_err(|e| e.to_string())?;

    let received_hb: DialtoneHeartbeat = tokio::time::timeout(Duration::from_secs(10), async {
        loop {
            let _ = sender_a.broadcast(payload_a.clone().into()).await;
            while let Ok(Some(event)) =
                tokio::time::timeout(Duration::from_millis(120), receiver_b.next()).await
            {
                if let GossipEvent::Received(msg) = event.map_err(|e| e.to_string())? {
                    if let Ok(parsed) = serde_json::from_slice::<DialtoneHeartbeat>(&msg.content) {
                        if parsed.node == node_a
                            && verify_heartbeat_password(
                                &parsed,
                                DEFAULT_CHANNEL_NAME,
                                test_password,
                            )
                        {
                            return Ok::<DialtoneHeartbeat, String>(parsed);
                        }
                    }
                }
            }
        }
    })
    .await
    .map_err(|_| "timed out waiting for iroh-gossip heartbeat".to_string())??;

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
    sign_heartbeat_with_password(&mut hb_b, DEFAULT_CHANNEL_NAME, test_password);

    let _ = router_a.shutdown().await;
    let _ = router_b.shutdown().await;

    Ok((received_hb, hb_b))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn channel_password_validation_rejects_default_and_authenticates_heartbeats() {
        assert!(validate_channel_password("").is_err());
        assert!(validate_channel_password("   ").is_err());
        assert!(validate_channel_password("dialtone").is_err());
        assert!(validate_channel_password("DIALTONE").is_err());
        assert!(validate_channel_password("default").is_err());
        assert_eq!(
            validate_channel_password("rover-fleet-2026").unwrap(),
            "rover-fleet-2026"
        );

        let topic_1 = derive_channel_topic_id("dialtone", "pass-alpha");
        let topic_2 = derive_channel_topic_id("dialtone", "pass-beta");
        assert_ne!(topic_1, topic_2);

        let cfg = IrohTransportConfig::local_offline();
        let addr = EndpointAddr::new(iroh::SecretKey::generate().public());
        let diag = build_diagnostics(&cfg, &addr, None, "ep-1");
        let mut hb = DialtoneHeartbeat {
            schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
            node: "node-alpha".to_string(),
            endpoint_id: "ep-1".to_string(),
            ticket: "iroh-gossip-v1:test".to_string(),
            counter: 7,
            updated_at_unix: 1_700_000_000,
            channel_mac: None,
            os: default_os(),
            arch: default_arch(),
            role: node_role_for("node-alpha"),
            capabilities: node_capabilities_for("node-alpha"),
            host_resources: sample_live_host_resources("node-alpha", None, 7),
            service_endpoints: Vec::new(),
            managed_processes: Vec::new(),
            iroh_transport: diag,
            observed_remote_addrs: Vec::new(),
            tailnet_used: false,
        };

        // Unsigned heartbeat must be rejected
        assert!(!verify_heartbeat_password(&hb, "dialtone", "pass-alpha"));

        // Signed with pass-alpha must pass for pass-alpha and fail for pass-beta
        sign_heartbeat_with_password(&mut hb, "dialtone", "pass-alpha");
        assert!(verify_heartbeat_password(&hb, "dialtone", "pass-alpha"));
        assert!(!verify_heartbeat_password(&hb, "dialtone", "pass-beta"));

        // Tampering with node name or endpoint_id invalidates MAC
        let mut spoofed = hb.clone();
        spoofed.node = "node-beta".to_string();
        assert!(!verify_heartbeat_password(
            &spoofed,
            "dialtone",
            "pass-alpha"
        ));
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
