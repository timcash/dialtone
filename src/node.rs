use std::env;
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::str::FromStr;

use std::net::{IpAddr, Ipv4Addr, SocketAddr, UdpSocket};

use iroh::{
    endpoint::{presets, AfterHandshakeOutcome, BindOpts, Connection, EndpointHooks, VarInt},
    Endpoint, EndpointAddr, RelayMode, SecretKey,
};
use iroh_mainline_address_lookup::DhtAddressLookup;
use iroh_mdns_address_lookup::MdnsAddressLookup;
use serde::{Deserialize, Serialize};

pub fn is_tailnet_ip(ip: IpAddr) -> bool {
    match ip {
        IpAddr::V4(v4) => {
            let o = v4.octets();
            // Tailscale CGNAT range 100.64.0.0/10 (100.64.0.0 - 100.127.255.255)
            o[0] == 100 && (o[1] & 0b1100_0000) == 64
        }
        IpAddr::V6(v6) => {
            let s = v6.segments();
            // Tailscale ULA range fd7a:115c:a1e0::/48
            s[0] == 0xfd7a && s[1] == 0x115c && s[2] == 0xa1e0
        }
    }
}

pub fn detect_non_tailnet_ipv4() -> Option<Ipv4Addr> {
    let sock = UdpSocket::bind("0.0.0.0:0").ok()?;
    sock.connect("1.1.1.1:80").ok()?;
    match sock.local_addr().ok()?.ip() {
        IpAddr::V4(v4)
            if !is_tailnet_ip(IpAddr::V4(v4)) && !v4.is_loopback() && !v4.is_unspecified() =>
        {
            Some(v4)
        }
        _ => None,
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum IrohRelayPolicy {
    Default,
    Disabled,
    RelayOnly,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct IrohTransportConfig {
    pub profile: String,
    pub dns_lookup: bool,
    pub local_discovery: bool,
    pub dht_lookup: bool,
    pub relay_mode: IrohRelayPolicy,
    pub relay_urls: Vec<String>,
    #[serde(default = "default_true")]
    pub exclude_tailnet: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub quic_port: Option<u16>,
}

fn default_true() -> bool {
    true
}

impl Default for IrohTransportConfig {
    fn default() -> Self {
        Self {
            profile: "wan_with_relay".to_string(),
            dns_lookup: false,
            local_discovery: false,
            dht_lookup: false,
            relay_mode: IrohRelayPolicy::Default,
            relay_urls: Vec::new(),
            exclude_tailnet: true,
            quic_port: None,
        }
    }
}

impl IrohTransportConfig {
    pub fn local_offline() -> Self {
        Self {
            profile: "local_offline".to_string(),
            dns_lookup: false,
            local_discovery: false,
            dht_lookup: false,
            relay_mode: IrohRelayPolicy::Disabled,
            relay_urls: Vec::new(),
            exclude_tailnet: false,
            quic_port: None,
        }
    }

    pub fn private_mesh(quic_port: Option<u16>) -> Self {
        Self {
            profile: "private_mesh".to_string(),
            dns_lookup: false,
            local_discovery: false,
            dht_lookup: false,
            relay_mode: IrohRelayPolicy::Disabled,
            relay_urls: Vec::new(),
            exclude_tailnet: true,
            quic_port,
        }
    }

    pub fn preset_name(&self) -> &'static str {
        match (self.dns_lookup, self.relay_mode) {
            (true, IrohRelayPolicy::Disabled) => "n0-disable-relay",
            (true, _) => "n0",
            (false, IrohRelayPolicy::Disabled) => "minimal-no-relay",
            (false, _) => "minimal-with-relay",
        }
    }

    pub fn relay_config_name(&self) -> &'static str {
        if self.relay_mode == IrohRelayPolicy::Disabled {
            "disabled"
        } else if self.relay_urls.is_empty() {
            "n0-public"
        } else {
            "custom"
        }
    }

    pub fn active_lookup_modes(&self) -> Vec<String> {
        let mut modes = vec!["ticket-memory".to_string(), "endpoint-id-hint".to_string()];
        if self.dns_lookup {
            modes.push("dns-pkarr".to_string());
        }
        if self.local_discovery {
            modes.push("mdns-local".to_string());
        }
        if self.dht_lookup {
            modes.push("mainline-dht".to_string());
        }
        modes
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct IrohTransportDiagnostics {
    pub profile: String,
    pub endpoint_preset: String,
    pub relay_mode: String,
    pub relay_config: String,
    pub relay_urls: Vec<String>,
    pub persistent_identity: bool,
    pub identity_key_path: Option<String>,
    #[serde(alias = "endpoint_id")] // compat
    pub node_id: String,
    pub address_lookup_modes: Vec<String>,
    pub ticket_addr_modes: Vec<String>,
    #[serde(default)]
    pub exclude_tailnet: bool,
    #[serde(default)]
    pub bound_non_tailnet_ipv4: Option<String>,
    #[serde(default)]
    pub advertised_relay_urls: Vec<String>,
    #[serde(default)]
    pub advertised_ip_addrs: Vec<String>,
}

pub fn default_secret_key_path() -> PathBuf {
    if let Ok(path) = env::var("DIALTONE_IROH_SECRET_KEY_FILE") {
        let trimmed = path.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    let home = env::var("DIALTONE_STATE_DIR")
        .or_else(|_| env::var("DIALTONE_HOME"))
        .ok()
        .filter(|s| !s.trim().is_empty())
        .map(PathBuf::from)
        .or_else(|| {
            env::var("HOME")
                .or_else(|_| env::var("USERPROFILE"))
                .ok()
                .map(|h| PathBuf::from(h).join(".dialtone"))
        })
        .unwrap_or_else(|| PathBuf::from(".dialtone"));
    home.join("iroh-secret-key")
}

pub fn load_or_create_secret_key_at(path: &Path) -> Result<SecretKey, String> {
    match fs::read_to_string(path) {
        Ok(raw) => {
            let text = raw.trim();
            if text.is_empty() {
                return Err(format!("iroh secret key {} is empty", path.display()));
            }
            SecretKey::from_str(text)
                .map_err(|err| format!("parse iroh secret key {}: {err}", path.display()))
        }
        Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
            let secret_key = SecretKey::generate();
            write_secret_key(path, &secret_key)?;
            Ok(secret_key)
        }
        Err(err) => Err(format!("read iroh secret key {}: {err}", path.display())),
    }
}

fn write_secret_key(path: &Path, secret_key: &SecretKey) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|err| format!("create iroh secret key dir {}: {err}", parent.display()))?;
    }
    let encoded = crate::hex_encode(&secret_key.to_bytes());
    let mut options = fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(path)
        .map_err(|err| format!("create iroh secret key {}: {err}", path.display()))?;
    file.write_all(encoded.as_bytes())
        .and_then(|_| file.write_all(b"\n"))
        .map_err(|err| format!("write iroh secret key {}: {err}", path.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut permissions = file
            .metadata()
            .map_err(|err| format!("metadata iroh secret key {}: {err}", path.display()))?
            .permissions();
        permissions.set_mode(0o600);
        fs::set_permissions(path, permissions)
            .map_err(|err| format!("chmod iroh secret key {}: {err}", path.display()))?;
    }
    Ok(())
}

/// QUIC close code sent to a node that is not on the allowlist.
pub const NOT_ALLOWLISTED_CLOSE_CODE: u32 = 403;

/// Endpoint hook that closes every connection (gossip and blobs, in and out) right after
/// the TLS handshake unless the remote NodeId is on the live allowlist.
#[derive(Clone, Debug)]
pub struct AllowlistHook {
    allowlist: crate::mesh::LiveAllowlist,
}

impl AllowlistHook {
    pub fn new(allowlist: crate::mesh::LiveAllowlist) -> Self {
        Self { allowlist }
    }
}

impl EndpointHooks for AllowlistHook {
    async fn after_handshake<'a>(&'a self, conn: &'a Connection) -> AfterHandshakeOutcome {
        let remote = conn.remote_id();
        if self.allowlist.contains(&remote) {
            return AfterHandshakeOutcome::accept();
        }
        eprintln!(
            "[allowlist-reject] closed QUIC connection from node_id={remote} (run 'dialtone allowlist add {remote}' to authorize)"
        );
        AfterHandshakeOutcome::Reject {
            error_code: VarInt::from_u32(NOT_ALLOWLISTED_CLOSE_CODE),
            reason: b"not on allowlist".to_vec(),
        }
    }
}

pub fn build_endpoint_builder(
    config: &IrohTransportConfig,
) -> Result<iroh::endpoint::Builder, String> {
    let mut builder = match (config.dns_lookup, config.relay_mode) {
        (true, IrohRelayPolicy::Disabled) => Endpoint::builder(presets::N0DisableRelay),
        (true, _) => Endpoint::builder(presets::N0),
        (false, IrohRelayPolicy::Disabled) => Endpoint::builder(presets::Minimal),
        (false, _) => Endpoint::builder(presets::Minimal).relay_mode(RelayMode::Default),
    };

    let bind_port = config.quic_port.unwrap_or(0);
    if config.exclude_tailnet {
        if let Some(v4) = detect_non_tailnet_ipv4() {
            builder = builder
                .clear_ip_transports()
                .bind_addr_with_opts(
                    SocketAddr::new(IpAddr::V4(v4), bind_port),
                    BindOpts::default()
                        .set_prefix_len(24)
                        .set_is_default_route(true),
                )
                .map_err(|err| format!("bind non-tailnet ipv4 {v4}:{bind_port}: {err}"))?;
        }
    } else if let Some(port) = config.quic_port {
        builder = builder
            .clear_ip_transports()
            .bind_addr_with_opts(
                SocketAddr::new(IpAddr::V4(Ipv4Addr::UNSPECIFIED), port),
                BindOpts::default().set_is_default_route(true),
            )
            .map_err(|err| format!("bind ipv4 0.0.0.0:{port}: {err}"))?;
    }

    if !config.relay_urls.is_empty() {
        if config.relay_mode == IrohRelayPolicy::Disabled {
            return Err(
                "custom Iroh relay URLs cannot be used when relays are disabled".to_string(),
            );
        }
        let parsed: Vec<iroh::RelayUrl> = config
            .relay_urls
            .iter()
            .map(|u| {
                u.parse::<iroh::RelayUrl>()
                    .map_err(|err| format!("invalid iroh relay URL {u:?}: {err}"))
            })
            .collect::<Result<Vec<_>, _>>()?;
        builder = builder.relay_mode(RelayMode::custom(parsed));
    }
    if config.local_discovery {
        builder = builder.address_lookup(MdnsAddressLookup::builder());
    }
    if config.dht_lookup {
        builder = builder.address_lookup(DhtAddressLookup::builder());
    }
    Ok(builder)
}

pub fn advertised_endpoint_addr(addr: &EndpointAddr, config: &IrohTransportConfig) -> EndpointAddr {
    let mut filtered = EndpointAddr::new(addr.id);
    for relay_url in addr.relay_urls() {
        filtered = filtered.with_relay_url(relay_url.clone());
    }
    if config.relay_mode != IrohRelayPolicy::RelayOnly {
        for sock_addr in addr.ip_addrs() {
            if config.exclude_tailnet && is_tailnet_ip(sock_addr.ip()) {
                continue;
            }
            filtered = filtered.with_ip_addr(*sock_addr);
        }
    }
    filtered
}

pub fn build_diagnostics(
    config: &IrohTransportConfig,
    advertised_addr: &EndpointAddr,
    key_path: Option<&Path>,
    node_id: &str,
) -> IrohTransportDiagnostics {
    let mut ticket_addr_modes = Vec::new();
    let advertised_relay_urls: Vec<String> = advertised_addr
        .relay_urls()
        .map(|u| u.to_string())
        .collect();
    let advertised_ip_addrs: Vec<String> =
        advertised_addr.ip_addrs().map(|a| a.to_string()).collect();

    if !advertised_relay_urls.is_empty() {
        ticket_addr_modes.push("relay".to_string());
    }
    if !advertised_ip_addrs.is_empty() {
        ticket_addr_modes.push("direct-ip".to_string());
    }
    if ticket_addr_modes.is_empty() {
        ticket_addr_modes.push("endpoint-id-only".to_string());
    }

    let relay_mode = match config.relay_mode {
        IrohRelayPolicy::Default => "default",
        IrohRelayPolicy::Disabled => "disabled",
        IrohRelayPolicy::RelayOnly => "relay-only",
    };

    let bound_non_tailnet_ipv4 = if config.exclude_tailnet {
        detect_non_tailnet_ipv4().map(|ip| ip.to_string())
    } else {
        None
    };

    IrohTransportDiagnostics {
        profile: config.profile.clone(),
        endpoint_preset: config.preset_name().to_string(),
        relay_mode: relay_mode.to_string(),
        relay_config: config.relay_config_name().to_string(),
        relay_urls: config.relay_urls.clone(),
        persistent_identity: key_path.is_some(),
        identity_key_path: key_path.map(|p| p.to_string_lossy().to_string()),
        node_id: node_id.to_string(),
        address_lookup_modes: config.active_lookup_modes(),
        ticket_addr_modes,
        exclude_tailnet: config.exclude_tailnet,
        bound_non_tailnet_ipv4,
        advertised_relay_urls,
        advertised_ip_addrs,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn secret_key_persists_with_0600_and_produces_stable_node_id() {
        let path = env::temp_dir().join(format!("dialtone-secret-key-test-{}", std::process::id()));
        let _ = fs::remove_file(&path);

        let key1 = load_or_create_secret_key_at(&path).unwrap();
        let key2 = load_or_create_secret_key_at(&path).unwrap();
        assert_eq!(key1.public(), key2.public());

        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = fs::metadata(&path).unwrap().permissions().mode() & 0o777;
            assert_eq!(mode, 0o600);
        }

        let cfg = IrohTransportConfig::local_offline();
        let rt = tokio::runtime::Runtime::new().unwrap();
        let (id1, id2) = rt.block_on(async {
            let ep1 = build_endpoint_builder(&cfg)
                .unwrap()
                .secret_key(key1)
                .bind()
                .await
                .unwrap();
            let id1 = ep1.id().to_string();
            ep1.close().await;

            let ep2 = build_endpoint_builder(&cfg)
                .unwrap()
                .secret_key(key2)
                .bind()
                .await
                .unwrap();
            let id2 = ep2.id().to_string();
            ep2.close().await;
            (id1, id2)
        });

        assert_eq!(id1, id2);
        let _ = fs::remove_file(&path);
    }

    #[test]
    fn tailnet_ip_filter_strips_cgnat_and_ula_while_preserving_physical_addrs() {
        assert!(is_tailnet_ip("100.87.0.37".parse().unwrap()));
        assert!(is_tailnet_ip("100.67.101.57".parse().unwrap()));
        assert!(is_tailnet_ip("fd7a:115c:a1e0::3833:26".parse().unwrap()));
        assert!(!is_tailnet_ip("192.168.1.212".parse().unwrap()));
        assert!(!is_tailnet_ip("72.34.124.189".parse().unwrap()));

        let id = SecretKey::generate().public();
        let addr = EndpointAddr::new(id)
            .with_ip_addr("100.87.0.37:41641".parse().unwrap())
            .with_ip_addr("192.168.1.212:57987".parse().unwrap());
        let filtered = advertised_endpoint_addr(&addr, &IrohTransportConfig::default());
        let ips: Vec<String> = filtered.ip_addrs().map(|a| a.to_string()).collect();
        assert_eq!(ips, vec!["192.168.1.212:57987".to_string()]);
    }
}
