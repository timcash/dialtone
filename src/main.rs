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
use dialtone::deploy::install_persistent_os_daemon;
use dialtone::mesh::{
    allow_peer_in_config_with_addrs, cached_local_public_ip_geo, default_arch, default_config_path,
    default_os, default_seeds, default_store_dir, derive_standby_topic_id,
    detect_default_node_name, load_mesh_config, new_secret, node_capabilities_for, node_role_for,
    normalize_channel_name, now_unix, parse_duration_secs, redeem_invite, revoke_peer_in_config,
    save_mesh_config, spawn_background_public_ip_geo_probe, validate_secret, Beat, ChannelKeys,
    LiveAllowlist, MeshControlCommand, SeedList, StoreStats, BEAT_INTERVAL_SECS, BEAT_SCHEMA,
    DEFAULT_CHANNEL_NAME, INVITE_PREFIX, SUMMARY_INTERVAL_SECS,
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
         \x20 join <invite>            Join a channel with an invite from any member (dialtone-invite-v2:...),\n\
         \x20                          install this binary and start the node at login (--once: only join)\n\
         \x20 join <secret>            Unlock (or start) a channel with its secret\n\
         \x20 secret                   Generate a new channel secret (dt-...)\n\
         \x20 allowlist [list|add|remove] [<node_id>] Manage the allowlist in ~/.dialtone/config.json\n\
         \x20 blob [share|pull] <arg>  Share a file or text as a blob, or pull a blob by ticket\n\
         \x20 link export <name> --udp|--tcp <ip:port>    Offer a local source to peers over a direct link\n\
         \x20 link import <node>/<name> --udp|--tcp <ip:port>  Open a peer's source and deliver it here\n\
         \x20 link [list|close <id>]   Show links (path direct or relay, Rtt, packets), or close one\n\
         \x20 invite [--ttl 24h]       Make an invite any node can join with until it expires (max 30d)\n\
         \x20 daemon [install|status]  install: put this binary in ~/.dialtone/bin, register the login\n\
         \x20                          service and start the node (a running one restarts into it)\n\
         \x20 probe [--all]            Run one probe now: the gossip checks (MSH, NBR, BEAT; nodes run them\n\
         \x20                          every 30s), and with --all the store, pulls, programs and security\n\
         \x20                          (STO, PUL, PRC, SEC)\n\
         \x20 devices [--json]         List the devices connected to this machine (cameras, mics, keyboards, ...)\n\
         \x20 status                   Print this node's NodeId and transport report\n\
         \x20 help                     Show this help\n\n\
         flags:\n\
         \x20 --node <name>            Node name (default: saved config or hostname)\n\
         \x20 --listen <addr>          HUD and API listen address (default: 127.0.0.1:8788)\n\
         \x20 --api-token <token>      Explicit API bearer token for POST routes (default: ~/.dialtone/api-token)\n\
         \x20 --channel <name>         Channel name (default: dialtone)\n\
         \x20 --secret <secret>        Channel secret (or set DIALTONE_SECRET)\n\
         \x20 --ttl <duration>         TTL for 'dialtone invite' (e.g. 15m, 1h, 7d; default: 24h)\n\
         \x20 --addr <ip:port>         Direct address for 'dialtone allowlist add' (repeatable)\n\
         \x20 --force                  Pull over QUIC even if the blob is already in the store\n\
         \x20 --udp <ip:port>          'dialtone link': where the source sends (export) or packets go (import)\n\
         \x20 --tcp <ip:port>          'dialtone link': the service to connect to (export) or listen on (import)\n\
         \x20 --json                   Print JSON ('devices', 'invite', 'link list')\n\
         \x20 --once                   'up': boot, beat once and exit; 'join': only join, start nothing\n\
         \x20 --all                    'dialtone probe': also test the store, pulls, programs and security\n\n\
         transport (the default: relays on, no public DNS or DHT, Tailscale addresses left out):\n\
         \x20 --private-mesh           No relays: dial peers only at their direct addresses\n\
         \x20 --relay-only             Advertise only the relay, no direct addresses\n\
         \x20 --relay-url <url>        Use this Iroh relay instead of the public ones (repeatable)\n\
         \x20 --quic-port <port>       Bind the QUIC endpoint to this UDP port\n\
         \x20 --lan                    Also find peers on the local network (mDNS)\n\
         \x20 --allow-tailnet          Also use Tailscale addresses"
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
    daemon_subcmd: String,
    allowlist_subcmd: String,
    peer_target: Option<String>,
    peer_addrs: Vec<String>,
    blob_subcmd: String,
    blob_target: Option<String>,
    link_subcmd: String,
    link_target: Option<String>,
    link_udp: Option<String>,
    link_tcp: Option<String>,
    force: bool,
    json: bool,
    invite_ttl_secs: u64,
    once: bool,
    all: bool,
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
        daemon_subcmd: "install".to_string(),
        allowlist_subcmd: "list".to_string(),
        peer_target: None,
        peer_addrs: Vec::new(),
        blob_subcmd: "share".to_string(),
        blob_target: None,
        link_subcmd: "list".to_string(),
        link_target: None,
        link_udp: None,
        link_tcp: None,
        force: false,
        json: false,
        invite_ttl_secs: 86_400,
        once: false,
        all: false,
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
            "link" if !action_set => {
                opts.action = "link".to_string();
                action_set = true;
                if let Some(sub) = args.get(idx + 1).filter(|s| !s.starts_with('-')) {
                    idx += 1;
                    opts.link_subcmd = sub.clone();
                    if let Some(target) = args.get(idx + 1).filter(|s| !s.starts_with('-')) {
                        idx += 1;
                        opts.link_target = Some(target.clone());
                    }
                }
            }
            "--udp" => {
                idx += 1;
                opts.link_udp = Some(args.get(idx).ok_or("missing --udp value")?.clone());
            }
            "--tcp" => {
                idx += 1;
                opts.link_tcp = Some(args.get(idx).ok_or("missing --tcp value")?.clone());
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
            "devices" if !action_set => {
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
                                && !next_arg.starts_with(dialtone::mesh::OPEN_INVITE_PREFIX)
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
            cmd @ ("status" | "probe") if !action_set => {
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
            "--force" => {
                opts.force = true;
            }
            "--json" => {
                opts.json = true;
            }
            "--addr" => {
                idx += 1;
                let addr = args.get(idx).ok_or("missing --addr value")?.clone();
                opts.peer_addrs.push(addr);
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
            "--once" => {
                opts.once = true;
            }
            "--all" => {
                opts.all = true;
            }
            "--lan" => {
                opts.transport.local_discovery = true;
                opts.transport.profile = "lan_only".to_string();
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
        version: dialtone::update::running_version().to_string(),
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

/// Print `GET /api/links` as a table. A link over the relay is flagged: expect more latency.
fn print_links(resp: &serde_json::Value) {
    let text = |v: &serde_json::Value, k: &str| {
        v.get(k).and_then(|x| x.as_str()).unwrap_or("").to_string()
    };
    let rtt = |v: &serde_json::Value| {
        v.get("rtt_ms")
            .and_then(|r| r.as_f64())
            .map(|r| format!("{r:.1}ms"))
            .unwrap_or_else(|| "-".to_string())
    };
    let counters = |v: &serde_json::Value| {
        let c = &v["counters"];
        format!(
            "in {} pkts / out {} pkts, {} dropped",
            c["packets_in"], c["packets_out"], c["drops"]
        )
    };
    let path = |p: String| match p.as_str() {
        "relay" => "RELAY (slower: no direct path)".to_string(),
        "direct" => "DIRECT".to_string(),
        _ => "-".to_string(),
    };
    let empty = Vec::new();
    for e in resp["exports"].as_array().unwrap_or(&empty) {
        println!(
            "export {:<16} {} {:<22} {}",
            text(e, "name"),
            text(e, "kind"),
            text(e, "addr"),
            counters(e)
        );
        for v in e["viewers"].as_array().unwrap_or(&empty) {
            let id = text(v, "node_id");
            println!(
                "    viewer {}  {}  {}",
                id.get(..10).unwrap_or(&id),
                path(text(v, "path")),
                rtt(v)
            );
        }
    }
    for i in resp["imports"].as_array().unwrap_or(&empty) {
        let err = text(i, "error");
        println!(
            "import {:<24} {} -> {:<22} {:<10} {}  {}  {}{}",
            text(i, "id"),
            text(i, "kind"),
            text(i, "addr"),
            text(i, "state"),
            path(text(i, "path")),
            rtt(i),
            counters(i),
            if err.is_empty() {
                String::new()
            } else {
                format!("  ({err})")
            }
        );
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

/// Present an open invite to the members it names, then save the channel keys and seeds they
/// hand back. Joining by invite makes this node part of an open channel, so any allowlist from
/// a previous channel is cleared.
async fn join_with_invite(opts: &CliOptions, raw: &str) -> Result<serde_json::Value, String> {
    let invite: dialtone::mesh::OpenInvite = raw.parse()?;
    if invite.expires_at_unix < now_unix() {
        return Err(
            "this invite has expired; ask a member for a new one ('dialtone invite')".to_string(),
        );
    }
    let secret_key = load_or_create_secret_key_at(&default_secret_key_path())?;
    let endpoint = build_endpoint_builder(&opts.transport)?
        .secret_key(secret_key)
        .bind()
        .await
        .map_err(|e| format!("bind iroh endpoint: {e}"))?;
    let node_id = endpoint.id().to_string();
    let answer = dialtone::join::redeem(&endpoint, &invite, Duration::from_secs(20)).await;
    endpoint.close().await;
    let answer = answer?;

    let path = default_config_path();
    let mut cfg = load_mesh_config(&path);
    cfg.node_name = Some(opts.node.clone());
    cfg.listen_addr = Some(opts.listen.clone());
    let keys = cfg.apply_keys(&answer.channel, answer.derived_keys)?;
    let cleared = std::mem::take(&mut cfg.allowlist).len();
    cfg.seeds = answer.seeds;
    cfg.updated_at_unix = now_unix();
    save_mesh_config(&path, &cfg)?;
    Ok(json!({
        "status": "joined",
        "node": opts.node,
        "node_id": node_id,
        "channel": keys.channel,
        "channel_fingerprint": keys.fingerprint(),
        "seeds": cfg.seeds.len(),
        "allowlist_cleared": cleared,
        "config_path": path
    }))
}

/// Ask the node answering on `listen` to restart, and wait until it has stopped and answers
/// again; false if it did not come back.
fn restart_running_node(listen: &str, token: &str) -> bool {
    let up = || local_daemon_http_json(listen, "GET", "/api/net", None, token, 2).is_ok();
    let within = |secs: u64, f: &dyn Fn() -> bool| {
        let start = std::time::Instant::now();
        while start.elapsed() < std::time::Duration::from_secs(secs) {
            if f() {
                return true;
            }
            std::thread::sleep(std::time::Duration::from_millis(500));
        }
        false
    };
    local_daemon_http_json(listen, "POST", "/api/restart", None, token, 5).is_ok()
        && within(20, &|| !up())
        && within(30, &up)
}

/// `dialtone daemon install`, and `dialtone join <invite>` after joining: put this binary in the
/// state directory and start the node, so it keeps running after this command and, in the
/// default state directory, starts at login. A node already running there restarts into the
/// new binary. Where no service can be installed (a container without systemd, say) the node
/// keeps itself running, and restarts after updates and crashes, but does not start at boot.
fn install_and_start(opts: &CliOptions) -> Result<(), String> {
    use dialtone::deploy::{custom_state_dir, place_binary, start_detached, start_service};
    let state = dialtone::mesh::default_state_dir();
    let log = state.join("daemon.log");
    let token = load_or_create_api_token().unwrap_or_default();
    let running = local_daemon_http_json(&opts.listen, "GET", "/api/net", None, &token, 2).is_ok();
    // A running node that is not restarted would keep the store, and a second one could not
    // start. One that does not come back (an older release with nothing to start it again) is
    // started here.
    let start_without_service = |bin: &std::path::Path| -> Result<String, String> {
        if running && restart_running_node(&opts.listen, &token) {
            return Ok("the running node restarted into it".into());
        }
        let pid = start_detached(bin, &opts.node, &opts.listen, &log)?;
        Ok(format!("started it (pid {pid})"))
    };
    let bin = if custom_state_dir() {
        let bin = place_binary(&state)?;
        let how = start_without_service(&bin)?;
        println!(
            "Installed {}; {how}. DIALTONE_STATE_DIR is set, so it does not start at login.",
            bin.display()
        );
        bin
    } else {
        let rep =
            install_persistent_os_daemon(&opts.node, &opts.listen, opts.secret_key.as_deref())?;
        let bin = std::path::PathBuf::from(&rep.program_path);
        let service = if running && cfg!(windows) {
            // The Windows login loop restarts the node it runs; starting the task again would
            // run a second one.
            local_daemon_http_json(&opts.listen, "POST", "/api/restart", None, &token, 5)?;
            println!(
                "Installed {}; the running node restarts into it.",
                bin.display()
            );
            true
        } else if let Err(e) = start_service() {
            let how = start_without_service(&bin)?;
            println!(
                "Installed {}; {how}. It restarts itself after updates and crashes, but {e}, \
                 so it does not start at boot.",
                bin.display()
            );
            false
        } else {
            println!(
                "Installed {}; it runs now and starts at login ({}).",
                bin.display(),
                rep.service_unit_path
            );
            true
        };
        let mut cfg = load_mesh_config(&default_config_path());
        cfg.daemon_installed = service;
        cfg.daemon_service_path = service.then(|| rep.service_unit_path.clone());
        cfg.updated_at_unix = now_unix();
        save_mesh_config(&default_config_path(), &cfg)?;
        bin
    };
    println!("  HUD:    http://{}", opts.listen);
    println!("  Status: {} status", bin.display());
    Ok(())
}

async fn run_node(mut opts: CliOptions) -> Result<(), String> {
    // Only the node that owns the state dir cleans its run dir and updates itself: one-shot
    // runs may share it with a running node.
    let owns_run_dir = !opts.once;
    let update_paths = dialtone::update::Paths::current().ok();
    if let Some(p) = update_paths
        .as_ref()
        .filter(|p| owns_run_dir && p.managed())
    {
        let var = |name: &str| env::var(name).ok();
        if !dialtone::update::has_service(&p.state_dir, std::process::id(), var) {
            let args: Vec<std::ffi::OsString> = env::args_os().skip(1).collect();
            return dialtone::supervisor::keep_running(&p.exe, &args, shutdown_signal()).await;
        }
    }
    let auto_update = owns_run_dir && update_paths.as_ref().is_some_and(dialtone::update::enabled);
    if auto_update {
        if let Some(p) = &update_paths {
            dialtone::update::on_start(p);
        }
    }
    let config_path = default_config_path();
    let blob_dir = default_store_dir();
    // One-shot runs may share the state dir with a running node, which owns the store; they
    // get a private one instead.
    let blobs = if opts.once {
        Blobs::open_or_private(&blob_dir).await?
    } else {
        Blobs::open(&blob_dir).await?
    };
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

    validate_listen_addr_security(&opts.listen, opts.api_token.is_some())?;

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

    println!(
        "[BOOT 1/5] Local Runtime & Stdout Logger ................. [READY] (node={}, os={}/{})",
        opts.node,
        default_os(),
        default_arch()
    );

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

    let shared_state: Option<Arc<Mutex<NodeState>>> = {
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
        // A one-shot run may share the state dir with a running node: it leaves the config alone.
        if !opts.once {
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
    println!(
        "[BOOT 3/5] Content-Addressed Blob Store (iroh-blobs) ..... [READY] {} (stage blobs via Web UI or POST /api/blob)",
        blobs.dir().display()
    );

    // =========================================================================
    // [BOOT 4/5] Iroh 1.3 QUIC Endpoint & Non-Tailnet Physical NIC
    // =========================================================================
    let key_path = default_secret_key_path();
    let secret_key = load_or_create_secret_key_at(&key_path)?;
    // Beats are signed with this Key; peers verify them against our NodeId.
    let node_key = secret_key.clone();
    // Enforced at the QUIC handshake for every protocol, and for beats and deploys below.
    let live_allowlist = LiveAllowlist::new(&saved_cfg.allowlist);
    // On an open channel: NodeIds that sent a valid message, admitted to fetch, blobs, link.
    let members = LiveAllowlist::default();
    let memory_lookup = MemoryLookup::new();
    // Weak handles to the open gossip connections: their QUIC Rtt and path, read for free.
    let gossip_conns = dialtone::node::GossipConnections::default();
    let endpoint = build_endpoint_builder(&opts.transport)?
        .address_lookup(memory_lookup.clone())
        .secret_key(secret_key)
        .hooks(
            AllowlistHook::new(live_allowlist.clone())
                .with_members(members.clone())
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
    let links = dialtone::link::Links::new(endpoint.clone());
    if let Some(st) = shared_state.as_ref().and_then(|s| s.lock().ok()).as_mut() {
        st.links = Some(links.clone());
    }
    let joins = dialtone::join::Joins::new(
        endpoint.clone(),
        opts.transport.clone(),
        config_path.clone(),
    );
    if let Some(st) = shared_state.as_ref().and_then(|s| s.lock().ok()).as_mut() {
        st.joins = Some(joins.clone());
    }
    let router = Router::builder(endpoint.clone())
        .accept(GOSSIP_ALPN, gossip.clone())
        .accept(iroh_blobs::ALPN, blobs_proto)
        .accept(
            dialtone::fetch::ALPN,
            dialtone::fetch::FetchHandler::new(shared_state.clone()),
        )
        .accept(dialtone::link::ALPN, links)
        .accept(dialtone::join::ALPN, joins)
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
    println!(
        "[BOOT 4/5] Iroh 1.3 QUIC Endpoint (No-Tailnet NIC) ....... [READY] id={}.. ip={}",
        &node_id[..16],
        diagnostics
            .bound_non_tailnet_ipv4
            .as_deref()
            .unwrap_or("LAN/WAN")
    );

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
        println!(
            "[BOOT 5/5] Gossip Mesh & Channel Secret ................ [READY] channel={} fp={} peers={}",
            active_channel,
            channel_fp,
            seed_ids.len()
        );
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
        println!("[BOOT 5/5] Gossip Mesh & Channel Secret ................ [SECRET REQUIRED]");
        println!(
            "           -> Enter the '{}' channel secret in the Web UI at {}",
            active_channel, local_web_url
        );
        println!("           -> Or run in terminal: dialtone join <SECRET>");
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
        members,
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
            update: update_paths.clone().filter(|_| auto_update),
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
            _ = dialtone::update::restart_token().cancelled() => "update",
        }
    }
    #[cfg(not(unix))]
    {
        tokio::select! {
            _ = tokio::signal::ctrl_c() => "ctrl-c",
            _ = dialtone::update::restart_token().cancelled() => "update",
        }
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
        "link" => (|| -> Result<(), String> {
            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();
            let call = |path: &str, body: serde_json::Value| {
                local_daemon_http_json(&opts.listen, "POST", path, Some(&body), &api_token, 20)
            };
            let resp = match opts.link_subcmd.as_str() {
                "export" => {
                    let name = opts.link_target.as_deref().ok_or(
                        "usage: dialtone link export <name> --udp <ip:port> | --tcp <ip:port>",
                    )?;
                    call(
                        "/api/links/export",
                        json!({ "name": name, "udp": opts.link_udp, "tcp": opts.link_tcp }),
                    )?
                }
                "import" => {
                    let (node, name) = opts
                        .link_target
                        .as_deref()
                        .and_then(|t| t.split_once('/'))
                        .ok_or("usage: dialtone link import <node>/<name> --udp <ip:port> | --tcp <ip:port>")?;
                    call(
                        "/api/links/import",
                        json!({ "node": node, "name": name, "udp": opts.link_udp, "tcp": opts.link_tcp }),
                    )?
                }
                "close" => {
                    let id = opts
                        .link_target
                        .as_deref()
                        .ok_or("usage: dialtone link close <name | node/name>")?;
                    call("/api/links/close", json!({ "id": id }))?
                }
                "list" | "status" => {
                    let resp =
                        local_daemon_http_json(&opts.listen, "GET", "/api/links", None, &api_token, 5)?;
                    if opts.json {
                        resp
                    } else {
                        print_links(&resp);
                        return Ok(());
                    }
                }
                other => {
                    return Err(format!(
                        "unknown link subcommand {other:?}; use: dialtone link [export|import|list|close]"
                    ))
                }
            };
            println!(
                "{}",
                serde_json::to_string_pretty(&resp).unwrap_or_default()
            );
            Ok(())
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
        "invite" => (|| -> Result<(), String> {
            // The running node makes the invite: it is the member a new node presents it to.
            let cfg = load_mesh_config(&default_config_path());
            let listen = match (&cfg.listen_addr, opts.listen_explicit) {
                (Some(saved), false) => saved.clone(),
                _ => opts.listen.clone(),
            };
            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();
            // A node that has just started serves its API before its endpoint is up, and
            // answers "node not running" until then.
            let started = std::time::Instant::now();
            let resp = loop {
                match local_daemon_http_json(
                    &listen,
                    "POST",
                    "/api/invites",
                    Some(&json!({ "ttl_secs": opts.invite_ttl_secs })),
                    &api_token,
                    10,
                ) {
                    Err(e)
                        if e.contains("node not running")
                            && started.elapsed() < Duration::from_secs(30) =>
                    {
                        std::thread::sleep(Duration::from_millis(500))
                    }
                    r => break r,
                }
            }
            .map_err(|e| format!("{e} (the node must be running: 'dialtone up')"))?;
            if opts.json {
                println!(
                    "{}",
                    serde_json::to_string_pretty(&resp).unwrap_or_default()
                );
            } else {
                println!(
                    "{}",
                    resp.get("invite")
                        .and_then(|v| v.as_str())
                        .unwrap_or_default()
                );
                eprintln!(
                    "Invite to channel '{}', valid until {} (unix). Any node joins with:\n  dialtone join <invite>",
                    resp.get("channel").and_then(|v| v.as_str()).unwrap_or_default(),
                    resp.get("expires_at_unix").and_then(|v| v.as_u64()).unwrap_or_default()
                );
            }
            Ok(())
        })(),
        "join"
            if opts
                .secret_key
                .as_deref()
                .is_some_and(|s| s.trim().starts_with(dialtone::mesh::OPEN_INVITE_PREFIX)) =>
        {
            let raw = opts.secret_key.clone().unwrap_or_default();
            match join_with_invite(&opts, &raw).await {
                Err(err) => {
                    eprintln!("dialtone: {err}");
                    std::process::exit(2);
                }
                Ok(report) => {
                    if opts.once {
                        println!("{}", serde_json::to_string_pretty(&report).unwrap());
                        Ok(())
                    } else {
                        println!(
                            "Joined channel '{}' as '{}'.",
                            report["channel"].as_str().unwrap_or_default(),
                            opts.node
                        );
                        install_and_start(&opts)
                    }
                }
            }
        }
        "join" => {
            let Some(raw_secret) = opts
                .secret_key
                .clone()
                .or_else(|| opts.invites.first().cloned())
            else {
                eprintln!(
                    "dialtone: usage: dialtone join <SECRET> [--channel dialtone] [--listen 127.0.0.1:8788]"
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
                run_node(opts).await
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
                install_and_start(&opts)
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
        "up" => run_node(opts).await,
        other => Err(format!("unsupported action {other:?}")),
    };

    if let Err(err) = result {
        eprintln!("dialtone: {err}");
        std::process::exit(1);
    }
    if dialtone::update::restart_requested() {
        println!("[update] exiting so the service starts the new binary");
        std::process::exit(dialtone::update::EXIT_RESTART);
    }
}
