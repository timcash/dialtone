use std::env;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::str::FromStr;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use dialtone::api::{
    default_boot_stages, load_or_create_api_token, serve_http_blocking,
    validate_listen_addr_security, NodeState,
};
use dialtone::blob::{Blobs, Store};
use dialtone::collect_node_diagnostics;
use dialtone::deploy::{install_persistent_os_daemon, render_os_service_unit};
use dialtone::mesh::{
    allow_peer_in_config_with_addrs, cached_local_public_ip_geo, default_arch, default_config_path,
    default_os, default_seeds, default_store_dir, derive_standby_topic_id,
    detect_default_node_name, is_allowed_peer_in_list, load_mesh_config, new_invite, new_secret,
    node_capabilities_for, node_role_for, normalize_channel_name, now_unix, parse_duration_secs,
    redeem_invite, revoke_peer_in_config, save_mesh_config, spawn_background_public_ip_geo_probe,
    validate_secret, Beat, ChannelKeys, LiveAllowlist, MeshControlCommand, SeedList, StoreStats,
    BEAT_INTERVAL_SECS, BEAT_SCHEMA, DEFAULT_CHANNEL_NAME, INVITE_PREFIX, SUMMARY_INTERVAL_SECS,
};
use dialtone::node::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, default_secret_key_path,
    load_or_create_secret_key_at, AllowlistHook, IrohRelayPolicy, IrohTransportConfig,
    IrohTransportDiagnostics,
};
use dialtone::stats::{sample_stats, StatsWindow};
use iroh::address_lookup::memory::MemoryLookup;
use iroh::protocol::Router;
use iroh_gossip::net::{Gossip, GOSSIP_ALPN};
use serde_json::json;

mod actors;

fn print_usage() {
    println!(
        "usage: dialtone [action] [flags]\n\n\
         Single-binary Iroh 1.3 + Rust mesh control plane, 3D tactical HUD, blob store & daemon.\n\
         Running `dialtone` with no arguments boots in dependency order and prints the local Web HUD URL.\n\n\
         actions:\n\
         \x20 up                       Start the node: HUD -> store -> Iroh QUIC -> mesh\n\
         \x20 join <secret>            Unlock and join a channel with its secret\n\
         \x20 secret                   Generate a new channel secret (dt-...)\n\
         \x20 allowlist [list|add|remove] [<node_id>] Manage the allowlist in ~/.dialtone/config.json\n\
         \x20 blob [share|pull] <arg>  Share a file or text as a blob, or pull a blob by ticket\n\
         \x20 invite [--ttl 15m]       Mint a signed, expiring, single-use invite\n\
         \x20 daemon [install|status]  Install or inspect the daemon that starts the node at login\n\
         \x20 probe [--all]            Run one probe now: the gossip checks (MSH, NBR, BEAT; nodes run them\n\
         \x20                          every 30s), and with --all the store, pulls and programs (STO, PUL, PRC)\n\
         \x20 devices [--json]         List the devices connected to this machine (cameras, mics, keyboards, ...)\n\
         \x20 mesh-test                Check the running node's security, mesh convergence and blob transfer\n\
         \x20 hud                      Serve the HUD and API only (no mesh)\n\
         \x20 node                     Run the node without the HUD or API\n\
         \x20 status                   Print this node's NodeId and transport report\n\
         \x20 service                  Render OS login service unit (macos | linux | windows)\n\
         \x20 help                     Show this help\n\n\
         flags:\n\
         \x20 --node <name>            Node name (default: saved config or hostname)\n\
         \x20 --listen <addr>          HUD and API listen address (default: 127.0.0.1:8788)\n\
         \x20 --api-token <token>      Explicit API bearer token for POST routes (default: ~/.dialtone/api-token)\n\
         \x20 --channel <name>         Channel name (default: dialtone)\n\
         \x20 --secret <secret>        Channel secret (or set DIALTONE_SECRET)\n\
         \x20 --invite <invite>        Invite to redeem when joining (repeatable)\n\
         \x20 --ttl <duration>         TTL for 'dialtone invite' (e.g. 15m, 1h, 300s; default: 15m)\n\
         \x20 --peer <node_id>         Target NodeId for 'dialtone invite' or 'dialtone allowlist'\n\
         \x20 --addr <ip:port>         Direct address for 'dialtone allowlist add' (repeatable)\n\
         \x20 --expect-peers <count>   Expected peer count for 'dialtone mesh-test'\n\
         \x20 --timeout <duration>     Timeout for 'dialtone mesh-test' (default: 15s)\n\
         \x20 --force                  Pull over QUIC even if the blob is already in the store\n\
         \x20 --install-daemon         Install the daemon so the node starts at login\n\
         \x20 --no-tailnet             Exclude Tailscale (100.64.0.0/10 & fd7a:115c:a1e0::/48) and bind physical IPv4\n\
         \x20 --allow-tailnet          Allow Tailscale interface addresses\n\
         \x20 --private-mesh           Disable relays, Pkarr DNS, and Mainline DHT (direct static peer QUIC only)\n\
         \x20 --quic-port <port>       Explicit UDP port to bind for the Iroh QUIC endpoint\n\
         \x20 --lan                    Enable mDNS local discovery (lan_only profile)\n\
         \x20 --dns                    Enable public Pkarr DNS lookup/publish (disabled by default)\n\
         \x20 --dht                    Enable Mainline DHT address lookup (disabled by default)\n\
         \x20 --no-dns                 Disable DNS/Pkarr lookup\n\
         \x20 --no-relay               Disable Iroh relay servers\n\
         \x20 --relay-only             Advertise relay-only gossip tickets\n\
         \x20 --relay-url <url>        Custom Iroh relay URL (repeatable)\n\
         \x20 --os <macos|linux|windows> Target OS for service unit generation\n\
         \x20 --once                   Run one cycle and exit\n\
         \x20 --all                    'dialtone probe': also test the store, pulls and programs"
    );
}

struct CliOptions {
    action: String,
    node: String,
    node_explicit: bool,
    listen: String,
    listen_explicit: bool,
    api_token: Option<String>,
    channel: String,
    secret_key: Option<String>,
    install_daemon: bool,
    daemon_subcmd: String,
    allowlist_subcmd: String,
    peer_target: Option<String>,
    peer_addrs: Vec<String>,
    blob_subcmd: String,
    blob_target: Option<String>,
    force: bool,
    json: bool,
    expect_peers: Option<usize>,
    timeout_secs: u64,
    invite_ttl_secs: u64,
    once: bool,
    all: bool,
    os_kind: String,
    invites: Vec<String>,
    transport: IrohTransportConfig,
}

fn parse_cli(args: Vec<String>) -> Result<CliOptions, String> {
    let saved = load_mesh_config(&default_config_path());
    let default_node = saved
        .node_name
        .clone()
        .filter(|s| !s.trim().is_empty())
        .unwrap_or_else(detect_default_node_name);
    let default_listen = saved
        .listen_addr
        .clone()
        .filter(|s| !s.trim().is_empty())
        .unwrap_or_else(|| "127.0.0.1:8788".to_string());
    let default_channel = env::var("DIALTONE_CHANNEL")
        .ok()
        .filter(|s| !s.trim().is_empty())
        .unwrap_or_else(|| normalize_channel_name(Some(&saved.channel)));
    let env_secret = env::var("DIALTONE_SECRET")
        .or_else(|_| env::var("DIALTONE_SECRET_KEY"))
        .ok()
        .filter(|s| !s.trim().is_empty());

    let mut opts = CliOptions {
        action: "up".to_string(),
        node: default_node,
        node_explicit: false,
        listen: default_listen,
        listen_explicit: false,
        api_token: env::var("DIALTONE_API_TOKEN")
            .ok()
            .filter(|s| !s.trim().is_empty()),
        channel: default_channel,
        secret_key: env_secret.or(saved.secret_key),
        install_daemon: false,
        daemon_subcmd: "install".to_string(),
        allowlist_subcmd: "list".to_string(),
        peer_target: None,
        peer_addrs: Vec::new(),
        blob_subcmd: "share".to_string(),
        blob_target: None,
        force: false,
        json: false,
        expect_peers: None,
        timeout_secs: 15,
        invite_ttl_secs: 900,
        once: false,
        all: false,
        os_kind: if cfg!(target_os = "macos") {
            "macos".to_string()
        } else if cfg!(target_os = "windows") {
            "windows".to_string()
        } else {
            "linux".to_string()
        },
        invites: saved.invites,
        transport: IrohTransportConfig::default(),
    };

    let mut action_set = false;
    let mut idx = 0;
    while idx < args.len() {
        match args[idx].as_str() {
            "help" | "-h" | "--help" => {
                opts.action = "help".to_string();
                return Ok(opts);
            }
            "mesh-test" if !action_set => {
                opts.action = "mesh-test".to_string();
                action_set = true;
            }
            "secret" if !action_set => {
                opts.action = "secret".to_string();
                action_set = true;
            }
            "blob" if !action_set => {
                opts.action = "blob".to_string();
                action_set = true;
                if let Some(sub) = args.get(idx + 1) {
                    if !sub.starts_with('-') {
                        idx += 1;
                        opts.blob_subcmd = sub.clone();
                        if let Some(target) = args.get(idx + 1) {
                            if !target.starts_with('-') {
                                idx += 1;
                                opts.blob_target = Some(target.clone());
                            }
                        }
                    }
                }
            }
            "allowlist" if !action_set => {
                opts.action = "allowlist".to_string();
                action_set = true;
                if let Some(sub) = args.get(idx + 1) {
                    if !sub.starts_with('-') {
                        idx += 1;
                        opts.allowlist_subcmd = sub.clone();
                        if matches!(opts.allowlist_subcmd.as_str(), "add" | "remove") {
                            if let Some(target) = args.get(idx + 1) {
                                if !target.starts_with('-') {
                                    idx += 1;
                                    opts.peer_target = Some(target.clone());
                                }
                            }
                        }
                    }
                }
            }
            "invite" if !action_set => {
                opts.action = "invite".to_string();
                action_set = true;
            }
            "devices" | "device" if !action_set => {
                opts.action = "devices".to_string();
                action_set = true;
            }

            "up" if !action_set => {
                opts.action = "up".to_string();
                action_set = true;
            }
            "join" if !action_set => {
                opts.action = "join".to_string();
                action_set = true;
                if let Some(next_arg) = args.get(idx + 1) {
                    if !next_arg.starts_with('-') {
                        idx += 1;
                        if let Some((ch, pass)) = next_arg.split_once(':') {
                            if !ch.trim().is_empty()
                                && !pass.trim().is_empty()
                                && !next_arg.starts_with("iroh-gossip-v1:")
                                && !next_arg.starts_with("dialtone-gossip-v1:")
                                && !next_arg.starts_with(INVITE_PREFIX)
                            {
                                opts.channel = normalize_channel_name(Some(ch));
                                opts.secret_key = Some(pass.trim().to_string());
                            } else {
                                opts.secret_key = Some(next_arg.clone());
                            }
                        } else {
                            opts.secret_key = Some(next_arg.clone());
                        }
                    }
                }
            }
            "daemon" if !action_set => {
                opts.action = "daemon".to_string();
                action_set = true;
                if let Some(next_arg) = args.get(idx + 1) {
                    if !next_arg.starts_with('-') {
                        idx += 1;
                        opts.daemon_subcmd = next_arg.clone();
                    }
                }
            }
            cmd @ ("status" | "node" | "hud" | "probe" | "service") if !action_set => {
                opts.action = cmd.to_string();
                action_set = true;
            }
            "--node" => {
                idx += 1;
                opts.node = args.get(idx).ok_or("missing --node value")?.clone();
                opts.node_explicit = true;
            }
            "--listen" => {
                idx += 1;
                opts.listen = args.get(idx).ok_or("missing --listen value")?.clone();
                opts.listen_explicit = true;
            }
            "--api-token" => {
                idx += 1;
                opts.api_token = Some(args.get(idx).ok_or("missing --api-token value")?.clone());
            }
            "--channel" => {
                idx += 1;
                let ch = args.get(idx).ok_or("missing --channel value")?;
                opts.channel = normalize_channel_name(Some(ch));
            }
            "--secret" => {
                idx += 1;
                opts.secret_key = Some(args.get(idx).ok_or("missing --secret value")?.clone());
            }
            "--ttl" => {
                idx += 1;
                let raw_ttl = args.get(idx).ok_or("missing --ttl value")?;
                opts.invite_ttl_secs = parse_duration_secs(raw_ttl)?;
            }
            "--timeout" => {
                idx += 1;
                let raw_to = args.get(idx).ok_or("missing --timeout value")?;
                opts.timeout_secs = parse_duration_secs(raw_to)?;
            }
            "--expect-peers" => {
                idx += 1;
                let raw_ep = args.get(idx).ok_or("missing --expect-peers value")?;
                opts.expect_peers = Some(
                    raw_ep
                        .parse::<usize>()
                        .map_err(|e| format!("invalid --expect-peers value: {e}"))?,
                );
            }
            "--force" => {
                opts.force = true;
            }
            "--json" => {
                opts.json = true;
            }
            "--peer" | "--endpoint-id" => {
                idx += 1;
                opts.peer_target = Some(args.get(idx).ok_or("missing --peer value")?.clone());
            }
            "--addr" | "--peer-addr" => {
                idx += 1;
                let addr = args.get(idx).ok_or("missing --addr value")?.clone();
                opts.peer_addrs.push(addr);
            }
            "--install-daemon" | "--daemon" => {
                opts.install_daemon = true;
            }
            "--invite" => {
                idx += 1;
                let t = args.get(idx).ok_or("missing --invite value")?.clone();
                if !opts.invites.contains(&t) {
                    opts.invites.push(t);
                }
            }
            "--no-tailnet" => {
                opts.transport.exclude_tailnet = true;
            }
            "--allow-tailnet" => {
                opts.transport.exclude_tailnet = false;
            }
            "--private-mesh" => {
                opts.transport = IrohTransportConfig::private_mesh(opts.transport.quic_port);
            }
            "--quic-port" => {
                idx += 1;
                let raw_port = args.get(idx).ok_or("missing --quic-port value")?;
                let port = raw_port
                    .parse::<u16>()
                    .map_err(|e| format!("invalid --quic-port value: {e}"))?;
                opts.transport.quic_port = Some(port);
            }
            "--os" => {
                idx += 1;
                opts.os_kind = args.get(idx).ok_or("missing --os value")?.clone();
            }
            "--once" => {
                opts.once = true;
            }
            "--all" => {
                opts.all = true;
            }
            "--lan" | "--local-discovery" => {
                opts.transport.local_discovery = true;
                opts.transport.profile = "lan_only".to_string();
            }
            "--dns" => {
                opts.transport.dns_lookup = true;
            }
            "--dht" => {
                opts.transport.dht_lookup = true;
            }
            "--no-dns" => {
                opts.transport.dns_lookup = false;
            }
            "--no-relay" => {
                opts.transport.relay_mode = IrohRelayPolicy::Disabled;
            }
            "--relay-only" => {
                opts.transport.relay_mode = IrohRelayPolicy::RelayOnly;
                opts.transport.profile = "relay_only".to_string();
            }
            "--relay-url" => {
                idx += 1;
                let url = args.get(idx).ok_or("missing --relay-url value")?.clone();
                opts.transport.relay_urls.push(url);
            }
            other if other.starts_with("-psn_") => {
                // Ignore macOS Finder LaunchServices ProcessSerialNumber argument on double-click
            }
            other => return Err(format!("unknown argument {other:?}")),
        }
        idx += 1;
    }

    Ok(opts)
}

fn self_beat(
    blobs: &Blobs,
    node: &str,
    node_id: &str,
    ticket: String,
    counter: u64,
    diag: IrohTransportDiagnostics,
) -> Beat {
    let resources = sample_stats(node, diag.bound_non_tailnet_ipv4.as_deref(), counter);
    let summary = blobs.summary();
    Beat {
        schema: BEAT_SCHEMA.to_string(),
        node: node.to_string(),
        node_id: node_id.to_string(),
        ticket,
        counter,
        updated_at_unix: now_unix(),
        kind: "regular".to_string(),
        beat_interval_secs: BEAT_INTERVAL_SECS,
        summary_interval_secs: SUMMARY_INTERVAL_SECS,
        heard: 0,
        inventory_version: 0,
        peer_rtt_ms: std::collections::BTreeMap::new(),
        neighbors: Vec::new(),
        summary_stats: None,
        os: default_os(),
        arch: default_arch(),
        role: node_role_for(node),
        capabilities: node_capabilities_for(node),
        stats: resources,
        service_endpoints: Vec::new(),
        processes: Vec::new(),
        iroh_transport: diag,
        observed_remote_addrs: vec!["local:self".to_string()],
        tailnet_used: false,
        devices: Vec::new(),
        geo_location: cached_local_public_ip_geo(),
        probe: None,
        store: Some(StoreStats {
            blob_keys: summary.blob_keys as u64,
            blob_bytes: summary.blob_bytes,
            latest_digest: summary.latest_digest,
        }),
        sent_unix_ms: 0,
    }
}

fn bind_local_web_listener(requested: &str, explicit: bool) -> Result<TcpListener, String> {
    if let Ok(l) = TcpListener::bind(requested) {
        return Ok(l);
    }
    if explicit {
        return TcpListener::bind(requested).map_err(|e| format!("bind {requested}: {e}"));
    }
    for port in 8788..=8799 {
        let candidate = format!("127.0.0.1:{port}");
        if let Ok(l) = TcpListener::bind(&candidate) {
            return Ok(l);
        }
    }
    TcpListener::bind("127.0.0.1:0").map_err(|e| format!("bind local web port: {e}"))
}

fn update_boot_stage(
    state: &Option<Arc<Mutex<NodeState>>>,
    step: u8,
    status: &str,
    detail: String,
) {
    if let Some(st_arc) = state {
        if let Ok(mut st) = st_arc.lock() {
            if let Some(stage) = st.boot_stages.iter_mut().find(|s| s.step == step) {
                stage.status = status.to_string();
                stage.detail = detail;
                stage.updated_at_unix = now_unix();
            }
        }
    }
}

/// Send an HTTP GET/POST JSON request to an already-running local `dialtone` daemon.
fn local_daemon_http_json(
    listen_addr: &str,
    method: &str,
    path: &str,
    body_json: Option<&serde_json::Value>,
    api_token: &str,
    timeout_secs: u64,
) -> Result<serde_json::Value, String> {
    let sock_addr = listen_addr
        .parse::<std::net::SocketAddr>()
        .map_err(|e| format!("invalid listen address '{listen_addr}': {e}"))?;
    let mut stream = TcpStream::connect_timeout(&sock_addr, Duration::from_millis(800))
        .map_err(|e| format!("connect local daemon at http://{listen_addr}: {e}"))?;
    let _ = stream.set_read_timeout(Some(Duration::from_secs(timeout_secs)));
    let _ = stream.set_write_timeout(Some(Duration::from_secs(timeout_secs)));
    let payload = body_json
        .map(|v| serde_json::to_string(v).unwrap_or_default())
        .unwrap_or_default();
    let req = if method.eq_ignore_ascii_case("POST") {
        format!(
            "POST {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {api_token}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            payload.len(),
            payload
        )
    } else {
        format!("GET {path} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
    };
    stream
        .write_all(req.as_bytes())
        .map_err(|e| format!("write HTTP request to {listen_addr}: {e}"))?;
    let mut resp_bytes = Vec::new();
    stream
        .read_to_end(&mut resp_bytes)
        .map_err(|e| format!("read HTTP response from {listen_addr}: {e}"))?;
    let resp_str = String::from_utf8_lossy(&resp_bytes);
    let (headers, body) = resp_str
        .split_once("\r\n\r\n")
        .ok_or_else(|| format!("malformed HTTP response from {listen_addr}"))?;
    let parsed: serde_json::Value = serde_json::from_str(body)
        .map_err(|e| format!("parse JSON response from {listen_addr}: {e} (body={body})"))?;
    if !headers.contains("200 OK") {
        let err_msg = parsed.get("error").and_then(|v| v.as_str()).unwrap_or(body);
        return Err(format!("daemon returned error ({headers}): {err_msg}"));
    }
    Ok(parsed)
}

/// Send `POST /api/mesh/join` to an already-running local `dialtone` HTTP server if reachable.
fn try_join_running_local_daemon(
    listen_addr: &str,
    channel: &str,
    secret: &str,
    invites: &[String],
    node: Option<&str>,
    api_token: &str,
) -> Option<serde_json::Value> {
    local_daemon_http_json(
        listen_addr,
        "POST",
        "/api/mesh/join",
        Some(&json!({
            "channel": channel,
            "secret": secret,
            "invite": invites.first().cloned().unwrap_or_default(),
            "node": node
        })),
        api_token,
        3,
    )
    .ok()
}

async fn run_node(mut opts: CliOptions, serve_http: bool) -> Result<(), String> {
    let verbose_boot = serve_http || !opts.once;
    let config_path = default_config_path();
    let blob_dir = default_store_dir();
    // `index` previews and one-shot runs may share the state dir with a running daemon,
    // which owns the store; they get a private one instead.
    let blobs = if opts.action == "hud" || opts.once {
        Blobs::open_or_private(&blob_dir).await?
    } else {
        Blobs::open(&blob_dir).await?
    };
    // Only the node that owns the state dir cleans its run dir: `hud` and one-shot runs may
    // share it with a running node whose processes and programs must not be touched.
    let owns_run_dir = serve_http && opts.action != "hud" && !opts.once;
    let run_dir = blob_dir.parent().unwrap_or(&blob_dir).join("run");
    if owns_run_dir {
        let reaped = dialtone::supervisor::reap_stale_processes(&run_dir);
        if reaped > 0 {
            println!("[supervisor] stopped {reaped} process tree(s) left by a previous run");
        }
    }

    if !opts.once {
        spawn_background_public_ip_geo_probe();
    }

    if serve_http {
        validate_listen_addr_security(&opts.listen, opts.api_token.is_some())?;
    }

    // =========================================================================
    // [BOOT 1/5] Local Runtime, Config & Stdout Logger
    // =========================================================================
    let mut saved_cfg = load_mesh_config(&config_path);
    if owns_run_dir {
        let mut keep: Vec<(String, String)> = saved_cfg
            .processes
            .iter()
            .map(|p| (p.name.clone(), p.digest.clone()))
            .collect();
        keep.push((
            dialtone::supervisor::PROBE_PROGRAM_NAME.to_string(),
            dialtone::blob::digest_of(dialtone::supervisor::probe_program_bytes()),
        ));
        let swept = dialtone::supervisor::sweep_run_dir(&run_dir, &keep);
        if swept > 0 {
            println!("[supervisor] deleted {swept} staged program(s) no process uses");
        }
    }
    if opts.channel == DEFAULT_CHANNEL_NAME && !saved_cfg.channel.trim().is_empty() {
        opts.channel = normalize_channel_name(Some(&saved_cfg.channel));
    }
    if opts.secret_key.is_none() && saved_cfg.secret_key.is_some() {
        opts.secret_key = saved_cfg.secret_key.clone();
    }
    for t in &saved_cfg.invites {
        if !opts.invites.contains(t) {
            opts.invites.push(t.clone());
        }
    }
    let active_channel = normalize_channel_name(Some(&opts.channel));
    let validated_secret = opts
        .secret_key
        .as_deref()
        .and_then(|raw| validate_secret(raw).ok());
    let active_keys: Option<ChannelKeys> = if let Some(ref pass) = validated_secret {
        Some(saved_cfg.apply_secret(&active_channel, pass))
    } else if saved_cfg.channel == active_channel {
        saved_cfg.active_channel_keys()
    } else {
        None
    };
    let has_mesh_credentials = active_keys.is_some();

    if opts.install_daemon {
        if let Ok(rep) =
            install_persistent_os_daemon(&opts.node, &opts.listen, validated_secret.as_deref())
        {
            saved_cfg.daemon_installed = true;
            saved_cfg.daemon_service_path = Some(rep.service_unit_path.clone());
            if verbose_boot {
                println!(
                    "[DAEMON] Persistent OS service installed: {}",
                    rep.instructions
                );
            }
        }
    }

    if verbose_boot {
        println!(
            "[BOOT 1/5] Local Runtime & Stdout Logger ................. [READY] (node={}, os={}/{})",
            opts.node,
            default_os(),
            default_arch()
        );
    }

    // =========================================================================
    // [BOOT 2/5] Local Web HUD & Configuration Console (starts FIRST!)
    // =========================================================================
    let (control_tx, control_rx) = tokio::sync::mpsc::unbounded_channel::<MeshControlCommand>();
    let mut local_web_url = format!("http://{}", opts.listen);

    let placeholder_diag = IrohTransportDiagnostics {
        profile: opts.transport.profile.clone(),
        endpoint_preset: opts.transport.preset_name().to_string(),
        relay_mode: if opts.transport.relay_mode == IrohRelayPolicy::Disabled {
            "disabled".to_string()
        } else {
            "default".to_string()
        },
        relay_config: opts.transport.relay_config_name().to_string(),
        relay_urls: opts.transport.relay_urls.clone(),
        persistent_identity: true,
        identity_key_path: Some(default_secret_key_path().to_string_lossy().to_string()),
        node_id: "booting-iroh-endpoint".to_string(),
        address_lookup_modes: opts.transport.active_lookup_modes(),
        ticket_addr_modes: vec!["relay".to_string(), "direct-ip".to_string()],
        exclude_tailnet: opts.transport.exclude_tailnet,
        bound_non_tailnet_ipv4: None,
        advertised_relay_urls: Vec::new(),
        advertised_ip_addrs: Vec::new(),
    };

    let shared_state: Option<Arc<Mutex<NodeState>>> = if serve_http {
        // Persist the requested address, not an auto-fallback port that was only free this boot.
        let requested_listen = opts.listen.clone();
        let listener = bind_local_web_listener(&opts.listen, opts.listen_explicit)?;
        let bound_addr = listener.local_addr().map_err(|e| e.to_string())?;
        opts.listen = bound_addr.to_string();
        local_web_url = format!("http://{bound_addr}");

        let mut runtime_state = NodeState::new_live(
            &opts.node,
            &opts.action,
            &opts.listen,
            blobs.clone(),
            placeholder_diag.clone(),
        );
        if let Some(ref explicit_tok) = opts.api_token {
            runtime_state.api_token = explicit_tok.clone();
        } else if let Ok(tok) = load_or_create_api_token() {
            runtime_state.api_token = tok;
        }
        runtime_state.control_tx = Some(control_tx.clone());
        runtime_state.boot_stages = default_boot_stages(&opts.listen, &blob_dir);
        for stg in &mut runtime_state.boot_stages {
            if stg.step >= 3 {
                stg.status = "booting".to_string();
                stg.detail = "Initializing subsystem in dependency order...".to_string();
            }
        }
        let fp = if let Some(ref keys) = active_keys {
            keys.fingerprint()
        } else {
            format!("{active_channel}:locked-standby")
        };
        saved_cfg.node_name = Some(opts.node.clone());
        saved_cfg.listen_addr = Some(requested_listen);
        saved_cfg.channel = active_channel.clone();
        saved_cfg.configured = has_mesh_credentials;
        saved_cfg.channel_fingerprint = fp;
        saved_cfg.invites = opts.invites.clone();
        saved_cfg.updated_at_unix = now_unix();
        // `index` is a read-only hud preview: never overwrite the daemon's saved config.
        if opts.action != "hud" {
            let _ = save_mesh_config(&config_path, &saved_cfg);
        }
        runtime_state.mesh_config = saved_cfg.clone();

        for mut saved_proc in saved_cfg.processes.clone() {
            if let Ok((verified_sha, staged_path)) =
                dialtone::supervisor::stage_program(&blobs, &saved_proc.name, &saved_proc.digest)
            {
                saved_proc.digest = verified_sha;
                saved_proc.path = staged_path.to_string_lossy().to_string();
            }
            if let Some(existing) = runtime_state
                .processes
                .iter_mut()
                .find(|p| p.name == saved_proc.name)
            {
                *existing = saved_proc;
            } else {
                runtime_state.processes.push(saved_proc);
            }
        }
        if !opts.once {
            let _ = dialtone::supervisor::ensure_probe_program(
                &blobs,
                &mut runtime_state.processes,
                None,
            );
        }
        runtime_state.reconcile_processes();

        if !opts.once {
            let mut boot_beat = self_beat(
                &blobs,
                &opts.node,
                "initializing-endpoint",
                "iroh-gossip-v1:booting".to_string(),
                0,
                placeholder_diag,
            );
            boot_beat.processes = runtime_state.processes.clone();
            runtime_state.nodes = vec![boot_beat];
        }

        let state = Arc::new(Mutex::new(runtime_state));
        let max_reqs = if opts.once { Some(1) } else { None };
        if opts.action == "hud" {
            println!("index listening {local_web_url}");
            return serve_http_blocking(listener, state, max_reqs);
        }
        let state_for_http = Arc::clone(&state);
        std::thread::spawn(move || {
            let _ = serve_http_blocking(listener, state_for_http, None);
        });

        println!(
            "[BOOT 2/5] Local Web HUD & Config Server ................. [READY] {local_web_url}"
        );
        println!(
            "================================================================================"
        );
        println!("  DIALTONE // 3D SCI-FI TACTICAL HUD & CONFIGURATION WEBAPP ONLINE");
        println!("  -> Open Local Web UI : {local_web_url}");
        println!("  -> Join Channel CLI  : dialtone join <SECRET>");
        println!("  -> Survive Restarts  : dialtone daemon install");
        println!(
            "================================================================================"
        );
        Some(state)
    } else {
        None
    };

    // =========================================================================
    // [BOOT 3/5] Content-Addressed Blob Store (iroh-blobs)
    // =========================================================================
    let probe_bytes = format!("dialtone-store-ready-{}", opts.node);
    let (probe_sha, _) = blobs.put(probe_bytes.as_bytes(), None).await?;
    update_boot_stage(
        &shared_state,
        3,
        "ready",
        format!(
            "Store active at {} (self-check {})",
            blobs.dir().display(),
            &probe_sha[..18]
        ),
    );
    if verbose_boot {
        println!(
            "[BOOT 3/5] Content-Addressed Blob Store (iroh-blobs) ..... [READY] {} (stage blobs via Web UI or POST /api/blob)",
            blobs.dir().display()
        );
    }

    // =========================================================================
    // [BOOT 4/5] Iroh 1.3 QUIC Endpoint & Non-Tailnet Physical NIC
    // =========================================================================
    let key_path = default_secret_key_path();
    let secret_key = load_or_create_secret_key_at(&key_path)?;
    // Beats are signed with this Key; peers verify them against our NodeId.
    let node_key = secret_key.clone();
    // Enforced at the QUIC handshake for every protocol, and for beats and deploys below.
    let live_allowlist = LiveAllowlist::new(&saved_cfg.allowlist);
    let memory_lookup = MemoryLookup::new();
    // Weak handles to the open gossip connections: their QUIC Rtt and path, read for free.
    let gossip_conns = dialtone::node::GossipConnections::default();
    let endpoint = build_endpoint_builder(&opts.transport)?
        .address_lookup(memory_lookup.clone())
        .secret_key(secret_key)
        .hooks(
            AllowlistHook::new(live_allowlist.clone())
                .with_gossip_connections(gossip_conns.clone()),
        )
        .bind()
        .await
        .map_err(|e| format!("bind iroh endpoint: {e}"))?;
    let node_id = endpoint.id().to_string();

    if opts.transport.relay_mode != IrohRelayPolicy::Disabled && !opts.once {
        let _ = tokio::time::timeout(Duration::from_secs(4), endpoint.online()).await;
    }

    // Remember message ids for three of the longest beat intervals: a beat still travelling
    // after iroh-gossip forgot its id would be delivered again (the default is 90s).
    let broadcast_config = iroh_gossip::proto::PlumtreeConfig {
        message_id_retention: Duration::from_secs(3 * dialtone::mesh::MAX_BEAT_INTERVAL_SECS),
        ..Default::default()
    };
    let gossip = Gossip::builder()
        .max_message_size(65_536)
        .broadcast_config(broadcast_config)
        .spawn(endpoint.clone());
    if let Some(st) = shared_state.as_ref().and_then(|s| s.lock().ok()).as_mut() {
        st.gossip_metrics = Some(gossip.metrics().clone());
    }
    let blobs_proto = iroh_blobs::BlobsProtocol::new(blobs.api(), None);
    let router = Router::builder(endpoint.clone())
        .accept(GOSSIP_ALPN, gossip.clone())
        .accept(iroh_blobs::ALPN, blobs_proto)
        .accept(
            dialtone::fetch::ALPN,
            dialtone::fetch::FetchHandler::new(shared_state.clone()),
        )
        .spawn();

    if let Some(st_arc) = &shared_state {
        let bridge = Store::new(
            endpoint.clone(),
            blobs.clone(),
            memory_lookup.clone(),
            opts.transport.clone(),
        );
        let default_ticket = if !opts.once {
            bridge
                .share_blob(dialtone::supervisor::probe_program_bytes(), None)
                .ok()
                .map(|r| r.ticket_uri)
        } else {
            None
        };
        if let Ok(mut st) = st_arc.lock() {
            st.store = Some(bridge);
            if !opts.once {
                let _ = dialtone::supervisor::ensure_probe_program(
                    &blobs,
                    &mut st.processes,
                    default_ticket.as_deref(),
                );
                st.reconcile_processes();
            }
        }
    }

    let advertised = advertised_endpoint_addr(&endpoint.addr(), &opts.transport);
    let diagnostics = build_diagnostics(&opts.transport, &advertised, Some(&key_path), &node_id);

    update_boot_stage(
        &shared_state,
        4,
        "ready",
        format!(
            "Bound endpoint {}.. on physical IP {}",
            &node_id[..12],
            diagnostics
                .bound_non_tailnet_ipv4
                .as_deref()
                .unwrap_or("LAN/WAN")
        ),
    );
    if verbose_boot {
        println!(
            "[BOOT 4/5] Iroh 1.3 QUIC Endpoint (No-Tailnet NIC) ....... [READY] id={}.. ip={}",
            &node_id[..16],
            diagnostics
                .bound_non_tailnet_ipv4
                .as_deref()
                .unwrap_or("LAN/WAN")
        );
    }

    // =========================================================================
    // [BOOT 5/5] Gossip Mesh & Channel Secret (iroh-gossip)
    // =========================================================================
    let mut invite_peer_addrs = Vec::new();
    if let Some(ref keys) = active_keys {
        let now = now_unix();
        let raw_tickets_snapshot = opts.invites.clone();
        let mut redeemed_any = false;
        for raw_t in &raw_tickets_snapshot {
            if raw_t.starts_with(INVITE_PREFIX) {
                if let Ok(parsed_inv) = raw_t.parse::<dialtone::mesh::Invite>() {
                    if let Ok(issuer_addr) =
                        redeem_invite(&parsed_inv, keys, &node_id, now, &mut saved_cfg)
                    {
                        invite_peer_addrs.push(issuer_addr);
                        redeemed_any = true;
                    }
                }
            }
        }
        if redeemed_any {
            let _ = save_mesh_config(&config_path, &saved_cfg);
        }
    }

    let (topic_id, channel_fp) = if let Some(ref keys) = active_keys {
        (keys.topic_id, keys.fingerprint())
    } else {
        (
            derive_standby_topic_id(&node_id),
            format!("{active_channel}:locked-standby"),
        )
    };

    let mut known_peer_addrs: std::collections::HashMap<iroh::EndpointId, iroh::EndpointAddr> =
        std::collections::HashMap::new();
    let mut joined_peer_ids: std::collections::HashSet<iroh::EndpointId> =
        std::collections::HashSet::new();
    let mut seed_ids = Vec::new();

    // Dial mesh seed peers once valid channel keys are active (either via relay or static direct IP).
    if has_mesh_credentials && !opts.once {
        for seed_addr in default_seeds() {
            let clean_seed = advertised_endpoint_addr(&seed_addr, &opts.transport);
            if clean_seed.id != endpoint.id()
                && (opts.transport.relay_mode != IrohRelayPolicy::Disabled
                    || clean_seed.ip_addrs().next().is_some())
            {
                if joined_peer_ids.insert(clean_seed.id) {
                    seed_ids.push(clean_seed.id);
                }
                known_peer_addrs.insert(clean_seed.id, clean_seed.clone());
                memory_lookup.add_endpoint_info(clean_seed);
            }
        }
    }

    if has_mesh_credentials {
        for peer_addr in invite_peer_addrs {
            let clean_addr = advertised_endpoint_addr(&peer_addr, &opts.transport);
            if clean_addr.id != endpoint.id() {
                if joined_peer_ids.insert(clean_addr.id) {
                    seed_ids.push(clean_addr.id);
                }
                known_peer_addrs.insert(clean_addr.id, clean_addr.clone());
            }
            memory_lookup.add_endpoint_info(clean_addr);
        }

        for raw_ticket in &opts.invites {
            if let Ok(parsed) = SeedList::from_str(raw_ticket) {
                for peer_addr in parsed.peers {
                    let clean_addr = advertised_endpoint_addr(&peer_addr, &opts.transport);
                    if clean_addr.id != endpoint.id() {
                        if joined_peer_ids.insert(clean_addr.id) {
                            seed_ids.push(clean_addr.id);
                        }
                        known_peer_addrs.insert(clean_addr.id, clean_addr.clone());
                    }
                    memory_lookup.add_endpoint_info(clean_addr);
                }
            }
        }
    }

    let initial_ticket = SeedList {
        topic: topic_id,
        peers: vec![advertised.clone()],
    }
    .to_text()?;

    let mut rolling_window = StatsWindow::new(SUMMARY_INTERVAL_SECS);
    let measured_peer_rtt_ms = std::collections::BTreeMap::new();

    let mut initial_self_beat = self_beat(
        &blobs,
        &opts.node,
        &node_id,
        initial_ticket,
        1,
        diagnostics.clone(),
    );
    initial_self_beat.kind = "summary".to_string();

    if let Some(st_arc) = &shared_state {
        if let Ok(mut st) = st_arc.lock() {
            st.reconcile_processes();
            initial_self_beat.processes = st.processes.clone();
            initial_self_beat.service_endpoints = st.service_endpoints.clone();
            initial_self_beat.probe = st.probe.clone();
        }
    }

    let init_restarts: u32 = initial_self_beat
        .processes
        .iter()
        .map(|p| p.restart_count)
        .sum();
    let now_init = now_unix();
    rolling_window.record_sample(
        now_init,
        &initial_self_beat.stats,
        &measured_peer_rtt_ms,
        init_restarts,
    );
    initial_self_beat.summary_stats = Some(rolling_window.summarize(now_init));

    if let Some(st_arc) = &shared_state {
        if let Ok(mut st) = st_arc.lock() {
            st.iroh_runtime = diagnostics.clone();
            st.stats = initial_self_beat.stats.clone();
            st.nodes = vec![initial_self_beat.clone()];
            st.refresh_live_alerts();
        }
    }

    let topic = gossip
        .subscribe(topic_id, seed_ids.clone())
        .await
        .map_err(|e| format!("subscribe gossip topic: {e}"))?;
    let (sender, receiver) = topic.split();
    if !seed_ids.is_empty() {
        let _ = tokio::time::timeout(
            Duration::from_millis(800),
            sender.join_peers(seed_ids.clone()),
        )
        .await;
    }

    if has_mesh_credentials {
        update_boot_stage(
            &shared_state,
            5,
            "ready",
            format!(
                "Unlocked '{}' channel ({}) with {} seeds",
                active_channel,
                channel_fp,
                seed_ids.len()
            ),
        );
        if verbose_boot {
            println!(
                "[BOOT 5/5] Gossip Mesh & Channel Secret ................ [READY] channel={} fp={} peers={}",
                active_channel,
                channel_fp,
                seed_ids.len()
            );
        }
    } else {
        update_boot_stage(
            &shared_state,
            5,
            "setup_needed",
            format!(
                "Channel '{}' is secret-protected — enter secret at {} or run 'dialtone join <SECRET>'",
                active_channel, local_web_url
            ),
        );
        if verbose_boot {
            println!("[BOOT 5/5] Gossip Mesh & Channel Secret ................ [SECRET REQUIRED]");
            println!(
                "           -> Enter the '{}' channel secret in the Web UI at {}",
                active_channel, local_web_url
            );
            println!("           -> Or run in terminal: dialtone join <SECRET>");
        }
    }

    if opts.once {
        let json = serde_json::to_string_pretty(&initial_self_beat).map_err(|e| e.to_string())?;
        println!("{json}");
        if let Some(ref keys) = active_keys {
            let msg = dialtone::mesh::Message::Beat(initial_self_beat.clone());
            if let Ok(enc) = dialtone::mesh::seal_message(&msg, keys, &node_key) {
                let _ = sender.broadcast(enc.into()).await;
            }
        }
        let _ = router.shutdown().await;
        blobs.close().await;
        return Ok(());
    }

    live_allowlist.replace(&saved_cfg.allowlist);
    let cancel = tokio_util::sync::CancellationToken::new();
    let ctx = actors::Ctx {
        node_id: node_id.clone(),
        endpoint: endpoint.clone(),
        memory_lookup: memory_lookup.clone(),
        transport: opts.transport.clone(),
        key_path: key_path.clone(),
        config_path: config_path.clone(),
        blobs: blobs.clone(),
        allowlist: live_allowlist.clone(),
        state: shared_state.clone(),
        net: shared_state
            .as_ref()
            .and_then(|s| s.lock().ok().map(|st| st.net.clone()))
            .unwrap_or_default(),
        gossip_conns,
        scores: shared_state
            .as_ref()
            .and_then(|s| s.lock().ok().map(|st| st.scores.clone()))
            .unwrap_or_default(),
        cancel: cancel.clone(),
    };
    let actors = actors::spawn_all(
        ctx,
        actors::MeshStart {
            node: opts.node.clone(),
            node_key,
            gossip: gossip.clone(),
            sender,
            receiver,
            keys: active_keys,
            channel: active_channel,
            topic_id,
            seed_ids,
            known_peer_addrs,
            print_first_beat: !serve_http,
            control_rx,
        },
    );

    let signal_name = shutdown_signal().await;
    println!("[shutdown] {signal_name} received; stopping processes and closing the endpoint");
    cancel.cancel();
    for actor in actors {
        let _ = tokio::time::timeout(Duration::from_secs(5), actor).await;
    }

    // Graceful shutdown: stop every process tree, then close the endpoint.
    if let Some(st_arc) = &shared_state {
        if let Ok(st) = st_arc.lock() {
            if let Ok(mut sup) = st.supervisor.lock() {
                sup.shutdown_all();
            }
        }
    }
    let _ = router.shutdown().await;
    blobs.close().await;
    Ok(())
}

#[cfg(unix)]
async fn recv_or_pending(signal: &mut Option<tokio::signal::unix::Signal>) {
    match signal {
        Some(s) => {
            s.recv().await;
        }
        None => std::future::pending::<()>().await,
    }
}

/// Resolves with the name of the first shutdown signal: ctrl-c, or SIGTERM / SIGHUP on Unix.
async fn shutdown_signal() -> &'static str {
    #[cfg(unix)]
    {
        use tokio::signal::unix::{signal, SignalKind};
        let mut term = signal(SignalKind::terminate()).ok();
        let mut hup = signal(SignalKind::hangup()).ok();
        tokio::select! {
            _ = tokio::signal::ctrl_c() => "SIGINT",
            _ = recv_or_pending(&mut term) => "SIGTERM",
            _ = recv_or_pending(&mut hup) => "SIGHUP",
        }
    }
    #[cfg(not(unix))]
    {
        let _ = tokio::signal::ctrl_c().await;
        "ctrl-c"
    }
}

#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    let opts = match parse_cli(args) {
        Ok(opts) => opts,
        Err(err) => {
            eprintln!("dialtone: {err}");
            print_usage();
            std::process::exit(2);
        }
    };

    let result: Result<(), String> = match opts.action.as_str() {
        "help" => {
            print_usage();
            Ok(())
        }
        "secret" => {
            let pw = new_secret();
            println!("{pw}");
            Ok(())
        }
        "allowlist" => (|| -> Result<(), String> {
            let cfg_path = default_config_path();
            match opts.allowlist_subcmd.as_str() {
                "list" | "ls" => {
                    let cfg = load_mesh_config(&cfg_path);
                    println!(
                        "{}",
                        serde_json::to_string_pretty(&json!({
                            "schema": "dialtone.peers_allowlist.v1",
                            "config_path": cfg_path,
                            "default_policy": "deny_unless_listed",
                            "allowlist": cfg.allowlist,
                            "seeds": cfg.seeds
                        }))
                        .unwrap()
                    );
                    Ok(())
                }
                                "add" => {
                    let target = opts
                        .peer_target
                        .as_deref()
                        .ok_or_else(|| "usage: dialtone allowlist add <node_id> [--addr <ip:port>]".to_string())?;
                    let relay_override = opts.transport.relay_urls.first().map(String::as_str);
                    let updated = allow_peer_in_config_with_addrs(
                        &cfg_path,
                        target,
                        relay_override,
                        &opts.peer_addrs,
                    )?;
                    println!(
                        "{}",
                        serde_json::to_string_pretty(&json!({
                            "status": "allowed",
                            "node_id": target.trim().to_ascii_lowercase(),
                            "allowlist": updated.allowlist,
                            "seeds": updated.seeds,
                            "config_path": cfg_path
                        }))
                        .unwrap()
                    );
                    Ok(())
                }
                                "remove" => {
                    let target = opts
                        .peer_target
                        .as_deref()
                        .ok_or_else(|| "usage: dialtone allowlist remove <node_id>".to_string())?;
                    let updated = revoke_peer_in_config(&cfg_path, target)?;
                    println!(
                        "{}",
                        serde_json::to_string_pretty(&json!({
                            "status": "revoked",
                            "node_id": target.trim().to_ascii_lowercase(),
                            "allowlist": updated.allowlist,
                            "config_path": cfg_path
                        }))
                        .unwrap()
                    );
                    Ok(())
                }
                other => Err(format!(
                    "unknown allowlist subcommand {other:?}; use: dialtone allowlist [list|add|remove] [<node_id>]"
                )),
            }
        })(),
        "blob" => (|| -> Result<(), String> {
            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();
            match opts.blob_subcmd.as_str() {
                "share" | "push" | "stage" => {
                    let arg = opts.blob_target.as_deref().ok_or_else(|| {
                        "usage: dialtone blob share <file-path-or-payload>".to_string()
                    })?;
                    let path = PathBuf::from(arg);
                    let raw_bytes = if path.exists() && path.is_file() {
                        std::fs::read(&path)
                            .map_err(|e| format!("read file {}: {e}", path.display()))?
                    } else {
                        arg.as_bytes().to_vec()
                    };
                    let b64 = dialtone::base64_encode(&raw_bytes);
                    let resp = local_daemon_http_json(
                        &opts.listen,
                        "POST",
                        "/api/blob/share",
                        Some(&json!({ "bytes_base64": b64 })),
                        &api_token,
                        15,
                    )?;
                    println!("{}", serde_json::to_string_pretty(&resp).unwrap());
                    Ok(())
                }
                "pull" | "fetch" | "get" => {
                    let ticket_uri = opts.blob_target.as_deref().ok_or_else(|| {
                        "usage: dialtone blob pull <iroh-blobs://ticket/...#sha256:...> [--force]"
                            .to_string()
                    })?;
                    let resp = local_daemon_http_json(
                        &opts.listen,
                        "POST",
                        "/api/blob/pull",
                        Some(&json!({
                            "ticket_uri": ticket_uri,
                            "force": opts.force
                        })),
                        &api_token,
                        30,
                    )?;
                    println!("{}", serde_json::to_string_pretty(&resp).unwrap());
                    Ok(())
                }
                other => Err(format!(
                    "unknown blob subcommand {other:?}; use: dialtone blob [share|pull] <arg>"
                )),
            }
        })(),
        "devices" => dialtone::devices::list_devices().map(|devices| {
            if opts.json {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&devices).unwrap_or_default()
                );
                return;
            }
            println!("{:<11} {:<12} {:<44} DETAIL", "KIND", "BUS", "NAME");
            for d in &devices {
                println!("{:<11} {:<12} {:<44} {}", d.kind, d.bus, d.name, d.detail);
            }
            println!("{} device(s)", devices.len());
        }),
        "probe" => (|| -> Result<(), String> {
            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();
            let resp = local_daemon_http_json(
                &opts.listen,
                "POST",
                "/api/probe",
                Some(&json!({ "all": opts.all })),
                &api_token,
                if opts.all { 120 } else { 5 },
            )?;
            println!("{}", serde_json::to_string_pretty(&resp).unwrap());
            Ok(())
        })(),
        "mesh-test" => (|| -> Result<(), String> {
            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();
            let cfg_disk = load_mesh_config(&default_config_path());
            let boot_deadline =
                std::time::Instant::now() + Duration::from_secs(opts.timeout_secs.max(1));
            let config_resp = loop {
                let resp = local_daemon_http_json(
                    &opts.listen,
                    "GET",
                    "/api/config",
                    None,
                    &api_token,
                    5,
                )?;
                let stages = resp
                    .get("boot_stages")
                    .and_then(|v| v.as_array())
                    .ok_or("missing boot_stages in /api/config")?;
                if stages.len() >= 5
                    && stages
                        .iter()
                        .all(|s| s.get("status").and_then(|v| v.as_str()) == Some("ready"))
                {
                    break resp;
                }
                if std::time::Instant::now() >= boot_deadline {
                    return Err(format!("not all 5 boot stages are ready: {stages:?}"));
                }
                std::thread::sleep(Duration::from_millis(400));
            };

            if cfg_disk.secret_key.is_some() {
                return Err(
                    "security check failed: plaintext secret_key present in config.json"
                        .to_string(),
                );
            }
            let kdf_ver = cfg_disk
                .derived_keys
                .as_ref()
                .map(|k| k.kdf_version.as_str())
                .unwrap_or("");
            if kdf_ver != "argon2id-v2" {
                return Err(format!(
                    "security check failed: expected derived_keys.kdf_version == 'argon2id-v2', got {kdf_ver:?}"
                ));
            }
            if cfg_disk.allowlist.is_empty() {
                return Err("security check failed: allowlist is empty in config.json".to_string());
            }

            let lookup_modes: Vec<String> = config_resp
                .get("iroh_runtime")
                .and_then(|r| r.get("address_lookup_modes"))
                .and_then(|m| m.as_array())
                .map(|arr| {
                    arr.iter()
                        .filter_map(|v| v.as_str().map(String::from))
                        .collect()
                })
                .unwrap_or_default();
            let public_directory_exposed = lookup_modes
                .iter()
                .any(|m| m == "dns-pkarr" || m == "mainline-dht");
            if public_directory_exposed {
                return Err(format!(
                    "privacy check failed: public directory lookup/publishing active ({lookup_modes:?})"
                ));
            }

            let own_ep_id = config_resp
                .get("iroh_runtime")
                .and_then(|r| r.get("node_id"))
                .and_then(|v| v.as_str());
            let expected_peers = opts
                .expect_peers
                .unwrap_or_else(|| cfg_disk.allowlist.len().max(1));
            let deadline =
                std::time::Instant::now() + Duration::from_secs(opts.timeout_secs.max(1));
            let (converged_nodes, mesh_rtt_ms) = loop {
                let state_resp =
                    local_daemon_http_json(&opts.listen, "GET", "/api/state", None, &api_token, 5)?;
                let nodes = state_resp
                    .get("nodes")
                    .and_then(|v| v.as_array())
                    .cloned()
                    .unwrap_or_default();
                let rtt_matrix = state_resp
                    .get("mesh_rtt_ms")
                    .and_then(|v| v.as_object())
                    .cloned()
                    .unwrap_or_default();
                let mut valid_peers = Vec::new();
                let mut all_stats_real = true;
                for n in &nodes {
                    let ep_id = n.get("node_id").and_then(|v| v.as_str()).unwrap_or("");
                    let tailnet_used = n
                        .get("tailnet_used")
                        .and_then(|v| v.as_bool())
                        .unwrap_or(false);
                    // Peers only reach node state after their beat's signature verified.
                    if is_allowed_peer_in_list(&cfg_disk.allowlist, ep_id, own_ep_id)
                        && (!opts.transport.exclude_tailnet || !tailnet_used)
                    {
                        let hr = n.get("stats").cloned().unwrap_or(json!({}));
                        let disk_used = hr
                            .get("disk_used_bytes")
                            .and_then(|v| v.as_u64())
                            .unwrap_or(0);
                        let disk_total = hr
                            .get("disk_total_bytes")
                            .and_then(|v| v.as_u64())
                            .unwrap_or(0);
                        let mem_total = hr
                            .get("mem_total_bytes")
                            .and_then(|v| v.as_u64())
                            .unwrap_or(0);
                        let is_fake_disk = disk_used == 120 * 1024 * 1024 * 1024
                            && disk_total == 512 * 1024 * 1024 * 1024;
                        if disk_total == 0 || mem_total == 0 || is_fake_disk {
                            all_stats_real = false;
                        }
                        if n.get("summary_stats").is_none()
                            || n.get("summary_stats").map(|v| v.is_null()).unwrap_or(true)
                        {
                            all_stats_real = false;
                        }
                        valid_peers.push(json!({
                            "node": n.get("node"),
                            "os": n.get("os"),
                            "arch": n.get("arch"),
                            "node_id": ep_id,
                            "counter": n.get("counter"),
                            "kind": n.get("kind"),
                            "beat_interval_secs": n.get("beat_interval_secs"),
                            "summary_interval_secs": n.get("summary_interval_secs"),
                            "updated_at_unix": n.get("updated_at_unix"),
                            "observed_remote_addrs": n.get("observed_remote_addrs"),
                            "tailnet_used": tailnet_used,
                            "peer_rtt_ms": n.get("peer_rtt_ms"),
                            "geo_location": n.get("geo_location"),
                            "processes": n.get("processes"),
                            "probe": n.get("probe"),
                            "stats": {
                                "cpu_cores": hr.get("cpu_cores"),
                                "cpu_percent": hr.get("cpu_percent"),
                                "mem_percent": hr.get("mem_percent"),
                                "mem_used_bytes": hr.get("mem_used_bytes"),
                                "mem_total_bytes": hr.get("mem_total_bytes"),
                                "disk_percent": hr.get("disk_percent"),
                                "disk_used_bytes": hr.get("disk_used_bytes"),
                                "disk_total_bytes": hr.get("disk_total_bytes")
                            },
                            "summary_stats": n.get("summary_stats")
                        }));
                    }
                }
                let rtt_ready =
                    expected_peers <= 1 || rtt_matrix.len() >= expected_peers.saturating_sub(1);
                if valid_peers.len() >= expected_peers && all_stats_real && rtt_ready {
                    break (valid_peers, serde_json::Value::Object(rtt_matrix));
                }
                if std::time::Instant::now() >= deadline {
                    return Err(format!(
                        "mesh convergence timed out after {}s: expected {} allowlisted peers with real stats & RTT, observed {} peers (all_stats_real={}, rtt_pairs={})",
                        opts.timeout_secs,
                        expected_peers,
                        valid_peers.len(),
                        all_stats_real,
                        rtt_matrix.len()
                    ));
                }
                std::thread::sleep(Duration::from_millis(500));
            };

            let probe_payload = format!("dialtone-mesh-test-probe-{}-{}", opts.node, now_unix());
            let share_resp = local_daemon_http_json(
                &opts.listen,
                "POST",
                "/api/blob/share",
                Some(&json!({
                    "bytes_base64": dialtone::base64_encode(probe_payload.as_bytes())
                })),
                &api_token,
                10,
            )?;
            let ticket_uri = share_resp
                .get("ticket_uri")
                .and_then(|v| v.as_str())
                .ok_or("missing ticket_uri from POST /api/blob/share")?;
            let pull_resp = local_daemon_http_json(
                &opts.listen,
                "POST",
                "/api/blob/pull",
                Some(&json!({
                    "ticket_uri": ticket_uri,
                    "force": true
                })),
                &api_token,
                15,
            )?;

            println!(
                "{}",
                serde_json::to_string_pretty(&json!({
                    "schema": "dialtone.mesh_test.v1",
                    "status": "ok",
                    "node": config_resp.get("node"),
                    "channel_fingerprint": cfg_disk.channel_fingerprint,
                    "kdf_version": kdf_ver,
                    "beat_interval_secs": BEAT_INTERVAL_SECS,
                    "summary_interval_secs": SUMMARY_INTERVAL_SECS,
                    "real_stats_verified": true,
                    "summary_stats_verified": true,
                    "public_directory_publishing": false,
                    "address_lookup_modes": lookup_modes,
                    "expected_peers": expected_peers,
                    "converged_peers": converged_nodes.len(),
                    "mesh_rtt_ms": mesh_rtt_ms,
                    "nodes": converged_nodes,
                    "blob_round_trip": {
                        "digest": pull_resp.get("digest"),
                        "method": pull_resp.get("method"),
                        "transferred_bytes": pull_resp.get("transferred_bytes"),
                        "ticket_uri": ticket_uri
                    }
                }))
                .unwrap()
            );
            Ok(())
        })(),
        "invite" => (|| -> Result<(), String> {
            let cfg_path = default_config_path();
            let mut cfg = load_mesh_config(&cfg_path);
            let channel = normalize_channel_name(Some(&opts.channel));
            let keys = if let Some(ref raw_pass) = opts.secret_key {
                let valid = validate_secret(raw_pass)?;
                cfg.apply_secret(&channel, &valid)
            } else if let Some(k) = cfg.active_channel_keys() {
                k
            } else {
                return Err(
                    "cannot mint invite: no channel keys configured (pass --secret <SECRET> or run 'dialtone join <SECRET>' first)"
                        .to_string(),
                );
            };

            let key_path = default_secret_key_path();
            let secret_key = load_or_create_secret_key_at(&key_path)?;
            let node_id = secret_key.public().to_string();

            let now = now_unix();
            let invite = new_invite(
                &node_id,
                Some("https://usw1-1.relay.n0.iroh.link./"),
                &keys,
                now,
                opts.invite_ttl_secs,
                opts.peer_target.as_deref(),
            )?;
            let invite_token = invite.to_text()?;
            println!(
                "{}",
                serde_json::to_string_pretty(&json!({
                    "schema": "dialtone.invite.v1",
                    "channel": keys.channel,
                    "channel_fingerprint": keys.fingerprint(),
                    "inviter_node_id": node_id,
                    "bound_node_id": opts.peer_target,
                    "ttl_secs": opts.invite_ttl_secs,
                    "expires_at_unix": invite.expires_at_unix,
                    "invite": invite_token,

                }))
                .unwrap()
            );
            Ok(())
        })(),
        "join" => {
            let Some(raw_secret) = opts
                .secret_key
                .clone()
                .or_else(|| opts.invites.first().cloned())
            else {
                eprintln!(
                    "dialtone: usage: dialtone join <SECRET> [--channel dialtone] [--invite <INVITE>] [--listen 127.0.0.1:8788]"
                );
                std::process::exit(2);
            };
            let channel = normalize_channel_name(Some(&opts.channel));
            let secret = match validate_secret(&raw_secret) {
                Ok(p) => p,
                Err(err) => {
                    eprintln!("dialtone: {err}");
                    std::process::exit(2);
                }
            };
            let mut cfg = load_mesh_config(&default_config_path());
            cfg.node_name = Some(opts.node.clone());
            cfg.listen_addr = Some(opts.listen.clone());
            let keys = cfg.apply_secret(&channel, &secret);
            let fp = keys.fingerprint();
            let topic_id = keys.topic_id;

            let key_path = default_secret_key_path();
            let self_ep_id = load_or_create_secret_key_at(&key_path)
                .ok()
                .map(|k| k.public().to_string())
                .unwrap_or_default();
            let now = now_unix();

            for t in &opts.invites {
                if t.starts_with(INVITE_PREFIX) {
                    let parsed_inv = match t.parse::<dialtone::mesh::Invite>() {
                        Ok(inv) => inv,
                        Err(err) => {
                            eprintln!("dialtone: {err}");
                            std::process::exit(2);
                        }
                    };
                    if let Err(err) = redeem_invite(&parsed_inv, &keys, &self_ep_id, now, &mut cfg)
                    {
                        eprintln!("dialtone: {err}");
                        std::process::exit(2);
                    }
                } else if !cfg.invites.contains(t) {
                    cfg.invites.push(t.clone());
                }
            }
            cfg.updated_at_unix = now;
            let _ = save_mesh_config(&default_config_path(), &cfg);

            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();

            if let Some(resp) = try_join_running_local_daemon(
                &opts.listen,
                &channel,
                &secret,
                &opts.invites,
                if opts.node_explicit {
                    Some(opts.node.as_str())
                } else {
                    None
                },
                &api_token,
            ) {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&json!({
                        "status": "joined_running_daemon",
                        "channel": channel,
                        "channel_fingerprint": fp,
                        "topic_id": topic_id.to_string(),
                        "daemon_response": resp
                    }))
                    .unwrap()
                );
                Ok(())
            } else if opts.once {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&json!({
                        "status": "saved_mesh_config",
                        "node": opts.node,
                        "channel": channel,
                        "channel_fingerprint": fp,
                        "topic_id": topic_id.to_string(),
                        "config_path": default_config_path()
                    }))
                    .unwrap()
                );
                Ok(())
            } else {
                println!(
                    "[JOIN] Derived & saved v2 channel subkeys for '{}' channel ({}) — booting dialtone daemon & local Web HUD...",
                    channel, fp
                );
                run_node(opts, true).await
            }
        }
        "daemon" => {
            if opts.daemon_subcmd == "status" {
                let cfg = load_mesh_config(&default_config_path());
                println!(
                    "{}",
                    serde_json::to_string_pretty(&json!({
                        "schema": "dialtone.daemon_status.v1",
                        "node": opts.node,
                        "listen": opts.listen,
                        "channel": cfg.channel,
                        "daemon_installed": cfg.daemon_installed,
                        "daemon_service_path": cfg.daemon_service_path,
                        "channel_fingerprint": cfg.channel_fingerprint,
                        "configured": cfg.configured
                    }))
                    .unwrap()
                );
                Ok(())
            } else {
                install_persistent_os_daemon(&opts.node, &opts.listen, opts.secret_key.as_deref())
                    .map(|rep| {
                        let mut cfg = load_mesh_config(&default_config_path());
                        cfg.daemon_installed = true;
                        cfg.daemon_service_path = Some(rep.service_unit_path.clone());
                        cfg.updated_at_unix = now_unix();
                        let _ = save_mesh_config(&default_config_path(), &cfg);
                        println!("{}", serde_json::to_string_pretty(&rep).unwrap());
                    })
            }
        }
        "status" => collect_node_diagnostics(&opts.transport).await.map(|diag| {
            println!(
                "{}",
                serde_json::to_string_pretty(&json!({
                    "schema": "dialtone.iroh_runtime.v1",
                    "owner": "dialtone-rust-daemon",
                    "steady_state_data_plane": "iroh-quic",
                    "node": opts.node,
                    "diagnostics": diag
                }))
                .unwrap()
            );
        }),
        "service" => {
            let exe =
                env::current_exe().unwrap_or_else(|_| PathBuf::from("/usr/local/bin/dialtone"));
            let unit = render_os_service_unit(&opts.os_kind, &exe, &opts.node);
            println!("{unit}");
            Ok(())
        }
        "node" => run_node(opts, false).await,
        "up" | "hud" => run_node(opts, true).await,
        other => Err(format!("unsupported action {other:?}")),
    };

    if let Err(err) = result {
        eprintln!("dialtone: {err}");
        std::process::exit(1);
    }
}
