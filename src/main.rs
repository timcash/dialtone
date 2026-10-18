use std::env;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::str::FromStr;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use dialtone::blobs::{
    evaluate_tiered_blob_fetch, store_verified_blob, LiveBlobBridge, TieredBlobFetchRequest,
};
use dialtone::deploy::{
    evaluate_deploy_build_decision, generate_deploy_plan, install_persistent_os_daemon,
    plan_rollback, render_os_service_unit,
};
use dialtone::evidence::{
    evaluate_proof_case, plan_alert_ack, sample_live_host_resources, TelemetryRollingWindow,
};
use dialtone::gossip::{
    allow_peer_in_config_with_addrs, build_mesh_rtt_matrix, cached_local_public_ip_geo,
    compute_rtt_ms_from_echo, create_invite_ticket, decrypt_heartbeat_from_gossip, default_arch,
    default_blob_cache_dir, default_config_path, default_fleet_seed_peers, default_os,
    derive_standby_topic_id, detect_default_node_name, encrypt_heartbeat_for_gossip,
    generate_channel_password, is_allowed_peer_in_list, load_mesh_config, monotonic_now_ms,
    node_capabilities_for, node_role_for, normalize_channel_name, now_unix,
    observe_peer_remote_addrs, parse_duration_secs, redeem_invite_ticket,
    resolve_fleet_node_locations, revoke_peer_in_config, save_mesh_config,
    sign_heartbeat_with_keys, spawn_background_public_ip_geo_probe, validate_channel_password,
    DerivedChannelKeys, DialtoneHeartbeat, GossipTicket, MeshControlCommand, PeerRttEcho,
    DEFAULT_CHANNEL_NAME, DIALTONE_HEARTBEAT_SCHEMA, HEARTBEAT_INTERVAL_SECS, INVITE_TICKET_PREFIX,
    SUMMARY_HEARTBEAT_EVERY_N_TICKS, SUMMARY_HEARTBEAT_INTERVAL_SECS,
};
use dialtone::http::{
    default_boot_stages, load_or_create_api_token, serve_http_blocking,
    validate_listen_addr_security, DaemonRuntimeState,
};
use dialtone::identity::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, default_secret_key_path,
    load_or_create_secret_key_at, IrohRelayPolicy, IrohTransportConfig, IrohTransportDiagnostics,
};
use dialtone::supervisor::plan_process_intent;
use dialtone::{collect_node_diagnostics, run_self_test};
use futures_lite::StreamExt;
use iroh::address_lookup::memory::MemoryLookup;
use iroh::protocol::Router;
use iroh_gossip::{
    api::Event as GossipEvent,
    net::{Gossip, GOSSIP_ALPN},
};
use serde_json::json;

fn print_usage() {
    println!(
        "usage: dialtone [action] [flags]\n\n\
         Single-binary Iroh 1.3 + Rust mesh control plane, 3D tactical HUD, blob store & daemon.\n\
         Running `dialtone` with no arguments boots in dependency order and prints the local Web HUD URL.\n\n\
         actions:\n\
         \x20 up | start | hub         Boot in dependency order: Local Web HUD -> Blob Store -> Iroh QUIC -> Gossip Mesh\n\
         \x20 join <password>          Unlock & join the 'dialtone' gossip channel with a password (live or persistent)\n\
         \x20 gen-password             Generate a CSPRNG high-entropy channel password (dt-...)\n\
         \x20 peers [allow|revoke|list] [<endpoint_id>] Manage the local peer allowlist in ~/.dialtone/config.json\n\
         \x20 blobs [share|pull] <arg> Share a local file/payload or pull an iroh-blobs://ticket/... via the running daemon\n\
         \x20 invite [--ttl 15m]       Mint a signed, expiring, single-use bootstrap invite ticket\n\
         \x20 daemon [install|status]  Install or inspect persistent OS background daemon (survives restarts)\n\
         \x20 fleet-test               Verify live daemon security posture, multi-node convergence & QUIC blob transfer\n\
         \x20 index                    Run HTTP dashboard, registration, and state API server\n\
         \x20 node                     Run headless Iroh 1.3 gossip + blob mesh peer node\n\
         \x20 status                   Bind persistent Iroh 1.3 endpoint and print transport JSON\n\
         \x20 self-test | ci           Run full 7-capability live verification suite\n\
         \x20 state-fixture            Print representative daemon state JSON\n\
         \x20 mesh-fixture             Print global mesh peer index JSON\n\
         \x20 gossip-fixture           Print Iroh gossip & service discovery JSON\n\
         \x20 iroh-runtime-fixture     Print Iroh 1.3 runtime identity & transport diagnostics\n\
         \x20 deploy-plan-fixture      Print deploy target plan or build-decision evidence\n\
         \x20 blob-fetch-report        Print tiered blob hydration plan (cache / iroh-blobs / fallback)\n\
         \x20 process-intent-fixture   Print start/stop/restart process intent plan & diff\n\
         \x20 rollback-plan-fixture    Print side-effect-free guarded rollback plan\n\
         \x20 alert-ack-fixture        Print side-effect-free alert acknowledgement preview\n\
         \x20 proof-validation-fixture Print session proof acceptance/rejection report\n\
         \x20 program-catalog-fixture  Print program catalog & named dataset evidence\n\
         \x20 service                  Render OS login service unit (macos | linux | windows)\n\
         \x20 help                     Show this help\n\n\
         flags:\n\
         \x20 --node <name>            Peer name (default: auto-detected hostname or saved config)\n\
         \x20 --listen <addr>          Local Web HUD listen address (default: 127.0.0.1:8788)\n\
         \x20 --api-token <token>      Explicit API bearer token for POST routes (default: ~/.dialtone/api-token)\n\
         \x20 --channel <name>         Gossip channel name (default: dialtone)\n\
         \x20 --password <pass>        Password required to unlock & authenticate the gossip channel\n\
         \x20 --bootstrap-ticket <t>   Bootstrap Iroh gossip or invite ticket (repeatable)\n\
         \x20 --ttl <duration>         TTL for 'dialtone invite' (e.g. 15m, 1h, 300s; default: 15m)\n\
         \x20 --peer <endpoint_id>     Optional target EndpointId for 'dialtone invite' or 'dialtone peers'\n\
         \x20 --addr <ip:port>         Static direct peer SocketAddr for 'dialtone peers allow' (repeatable)\n\
         \x20 --expect-peers <count>   Expected converged peer count for 'dialtone fleet-test'\n\
         \x20 --timeout <duration>     Timeout for 'dialtone fleet-test' (default: 15s)\n\
         \x20 --force                  Force QUIC network transfer on 'dialtone blobs pull' even if cached\n\
         \x20 --install-daemon         Register persistent OS service so dialtone survives restarts\n\
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
         \x20 --deploy-target <t>      robot-v4-default | dialtone | all-daemon | all-with-daemons\n\
         \x20 --build-case <c>         unchanged-skip | cache-reused | peer-local-async\n\
         \x20 --intent-action <a>      start | stop | restart\n\
         \x20 --proof-case <c>         valid | missing | stale | cross-session | non-daemon-fallback |\n\
         \x20                          no-renderer-fallback | peer-build-deps-missing\n\
         \x20 --sha256 <digest>        SHA-256 digest for blob-fetch-report\n\
         \x20 --uri <uri>              Artifact URI for blob-fetch-report\n\
         \x20 --http-fallback-enabled  Allow HTTP peer-bridge fallback for blob-fetch-report\n\
         \x20 --actor <name>           Actor name for alert-ack-fixture\n\
         \x20 --note <text>            Note for alert-ack-fixture\n\
         \x20 --os <macos|linux|windows> Target OS for service unit generation\n\
         \x20 --once                   Run one cycle and exit"
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
    peers_subcmd: String,
    peer_target: Option<String>,
    peer_addrs: Vec<String>,
    blobs_subcmd: String,
    blobs_target: Option<String>,
    force: bool,
    expect_peers: Option<usize>,
    timeout_secs: u64,
    invite_ttl_secs: u64,
    once: bool,
    deploy_target: String,
    build_case: Option<String>,
    intent_action: String,
    proof_case: String,
    sha256: String,
    uri: String,
    http_fallback_enabled: bool,
    actor: String,
    note: String,
    os_kind: String,
    bootstrap_tickets: Vec<String>,
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
    let env_password = env::var("DIALTONE_PASSWORD")
        .or_else(|_| env::var("DIALTONE_CHANNEL_PASSWORD"))
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
        secret_key: env_password.or(saved.secret_key),
        install_daemon: false,
        daemon_subcmd: "install".to_string(),
        peers_subcmd: "list".to_string(),
        peer_target: None,
        peer_addrs: Vec::new(),
        blobs_subcmd: "share".to_string(),
        blobs_target: None,
        force: false,
        expect_peers: None,
        timeout_secs: 15,
        invite_ttl_secs: 900,
        once: false,
        deploy_target: "robot-v4-default".to_string(),
        build_case: None,
        intent_action: "restart".to_string(),
        proof_case: "valid".to_string(),
        sha256: "sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d"
            .to_string(),
        uri: "iroh-blobs://ticket/blob123#sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d"
            .to_string(),
        http_fallback_enabled: false,
        actor: "operator-a".to_string(),
        note: "verified nominal".to_string(),
        os_kind: if cfg!(target_os = "macos") {
            "macos".to_string()
        } else if cfg!(target_os = "windows") {
            "windows".to_string()
        } else {
            "linux".to_string()
        },
        bootstrap_tickets: saved.bootstrap_tickets,
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
            "self-test" | "--self-test" | "ci" => {
                opts.action = "self-test".to_string();
                action_set = true;
            }
            "fleet-test" | "mesh-test" if !action_set => {
                opts.action = "fleet-test".to_string();
                action_set = true;
            }
            "gen-password" | "generate-password" if !action_set => {
                opts.action = "gen-password".to_string();
                action_set = true;
            }
            "blobs" | "blob" if !action_set => {
                opts.action = "blobs".to_string();
                action_set = true;
                if let Some(sub) = args.get(idx + 1) {
                    if !sub.starts_with('-') {
                        idx += 1;
                        opts.blobs_subcmd = sub.clone();
                        if let Some(target) = args.get(idx + 1) {
                            if !target.starts_with('-') {
                                idx += 1;
                                opts.blobs_target = Some(target.clone());
                            }
                        }
                    }
                }
            }
            "peers" | "peer" if !action_set => {
                opts.action = "peers".to_string();
                action_set = true;
                if let Some(sub) = args.get(idx + 1) {
                    if !sub.starts_with('-') {
                        idx += 1;
                        opts.peers_subcmd = sub.clone();
                        if matches!(
                            opts.peers_subcmd.as_str(),
                            "allow" | "add" | "revoke" | "remove" | "rm"
                        ) {
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
            "up" | "start" if !action_set => {
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
                                && !next_arg.starts_with(INVITE_TICKET_PREFIX)
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
            cmd @ ("status"
            | "node"
            | "index"
            | "hub"
            | "state-fixture"
            | "mesh-fixture"
            | "gossip-fixture"
            | "iroh-runtime-fixture"
            | "deploy-plan-fixture"
            | "blob-fetch-report"
            | "process-intent-fixture"
            | "rollback-plan-fixture"
            | "alert-ack-fixture"
            | "proof-validation-fixture"
            | "program-catalog-fixture"
            | "service")
                if !action_set =>
            {
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
            "--password" | "--channel-password" | "--pass" | "--secret-key" | "--mesh-key"
            | "--key" | "--secret" => {
                idx += 1;
                opts.secret_key = Some(args.get(idx).ok_or("missing --password value")?.clone());
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
            "--bootstrap-ticket" | "--join-ticket" => {
                idx += 1;
                let t = args
                    .get(idx)
                    .ok_or("missing --bootstrap-ticket value")?
                    .clone();
                if !opts.bootstrap_tickets.contains(&t) {
                    opts.bootstrap_tickets.push(t);
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
            "--deploy-target" => {
                idx += 1;
                opts.deploy_target = args
                    .get(idx)
                    .ok_or("missing --deploy-target value")?
                    .clone();
            }
            "--build-case" => {
                idx += 1;
                opts.build_case = Some(args.get(idx).ok_or("missing --build-case value")?.clone());
            }
            "--intent-action" => {
                idx += 1;
                opts.intent_action = args
                    .get(idx)
                    .ok_or("missing --intent-action value")?
                    .clone();
            }
            "--proof-case" => {
                idx += 1;
                opts.proof_case = args.get(idx).ok_or("missing --proof-case value")?.clone();
            }
            "--sha256" => {
                idx += 1;
                opts.sha256 = args.get(idx).ok_or("missing --sha256 value")?.clone();
            }
            "--uri" => {
                idx += 1;
                opts.uri = args.get(idx).ok_or("missing --uri value")?.clone();
            }
            "--http-fallback-enabled" => {
                opts.http_fallback_enabled = true;
            }
            "--actor" => {
                idx += 1;
                opts.actor = args.get(idx).ok_or("missing --actor value")?.clone();
            }
            "--note" => {
                idx += 1;
                opts.note = args.get(idx).ok_or("missing --note value")?.clone();
            }
            "--os" => {
                idx += 1;
                opts.os_kind = args.get(idx).ok_or("missing --os value")?.clone();
            }
            "--once" => {
                opts.once = true;
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

    if !opts.node_explicit && opts.action.ends_with("fixture") {
        opts.node = "node-alpha".to_string();
    }

    Ok(opts)
}

async fn build_seeded_state(opts: &CliOptions) -> Result<DaemonRuntimeState, String> {
    let diag = collect_node_diagnostics(&opts.transport).await?;
    let blob_dir = env::temp_dir().join("dialtone-blobs");
    Ok(DaemonRuntimeState::new_seeded(
        &opts.node,
        &opts.action,
        &opts.listen,
        blob_dir,
        diag,
    ))
}

fn upsert_heartbeat(nodes: &mut Vec<DialtoneHeartbeat>, mut hb: DialtoneHeartbeat) {
    if let Some(existing) = nodes
        .iter_mut()
        .find(|n| n.node == hb.node || n.endpoint_id == hb.endpoint_id)
    {
        if hb.summary_stats.is_none() {
            hb.summary_stats = existing.summary_stats.clone();
        }
        if hb.geo_location.is_none() {
            hb.geo_location = existing.geo_location.clone();
        }
        for (k, v) in &existing.peer_rtt_ms {
            hb.peer_rtt_ms.entry(k.clone()).or_insert(*v);
        }
        if hb.heartbeat_kind == "rtt_echo" {
            hb.heartbeat_kind = existing.heartbeat_kind.clone();
        }
        *existing = hb;
    } else {
        if hb.heartbeat_kind == "rtt_echo" {
            hb.heartbeat_kind = "regular".to_string();
        }
        nodes.push(hb);
    }
}

fn build_self_heartbeat(
    node: &str,
    endpoint_id: &str,
    ticket: String,
    counter: u64,
    diag: IrohTransportDiagnostics,
) -> DialtoneHeartbeat {
    let resources =
        sample_live_host_resources(node, diag.bound_non_tailnet_ipv4.as_deref(), counter);
    DialtoneHeartbeat {
        schema: DIALTONE_HEARTBEAT_SCHEMA.to_string(),
        node: node.to_string(),
        endpoint_id: endpoint_id.to_string(),
        ticket,
        counter,
        updated_at_unix: now_unix(),
        heartbeat_kind: "regular".to_string(),
        heartbeat_interval_secs: HEARTBEAT_INTERVAL_SECS,
        summary_interval_secs: SUMMARY_HEARTBEAT_INTERVAL_SECS,
        sent_mono_ms: monotonic_now_ms(),
        rtt_probe_requested: false,
        peer_echoes: std::collections::HashMap::new(),
        peer_rtt_ms: std::collections::BTreeMap::new(),
        summary_stats: None,
        os: default_os(),
        arch: default_arch(),
        role: node_role_for(node),
        capabilities: node_capabilities_for(node),
        host_resources: resources,
        service_endpoints: Vec::new(),
        managed_processes: Vec::new(),
        iroh_transport: diag,
        observed_remote_addrs: vec!["local:self".to_string()],
        tailnet_used: false,
        geo_location: cached_local_public_ip_geo(),
        channel_mac: None,
    }
}

fn build_peer_echoes(
    last_rx_from_peer: &std::collections::HashMap<String, (u64, u64)>,
    now_mono_ms: u64,
) -> std::collections::HashMap<String, PeerRttEcho> {
    let mut out = std::collections::HashMap::new();
    for (peer, &(peer_sent_mono, local_rx_mono)) in last_rx_from_peer {
        if peer_sent_mono == 0 || now_mono_ms < local_rx_mono {
            continue;
        }
        let hold_ms = now_mono_ms - local_rx_mono;
        if hold_ms <= 60_000 {
            out.insert(
                peer.clone(),
                PeerRttEcho {
                    echoed_mono_ms: peer_sent_mono,
                    hold_time_ms: hold_ms,
                },
            );
        }
    }
    out
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
    state: &Option<Arc<Mutex<DaemonRuntimeState>>>,
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
    password: &str,
    bootstrap_tickets: &[String],
    node: Option<&str>,
    api_token: &str,
) -> Option<serde_json::Value> {
    local_daemon_http_json(
        listen_addr,
        "POST",
        "/api/mesh/join",
        Some(&json!({
            "channel": channel,
            "password": password,
            "bootstrap_ticket": bootstrap_tickets.first().cloned().unwrap_or_default(),
            "node": node
        })),
        api_token,
        3,
    )
    .ok()
}

async fn run_node_or_hub(mut opts: CliOptions, serve_http: bool) -> Result<(), String> {
    let verbose_boot = serve_http || !opts.once;
    let config_path = default_config_path();
    let blob_dir = default_blob_cache_dir();

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
    if opts.channel == DEFAULT_CHANNEL_NAME && !saved_cfg.channel.trim().is_empty() {
        opts.channel = normalize_channel_name(Some(&saved_cfg.channel));
    }
    if opts.secret_key.is_none() && saved_cfg.secret_key.is_some() {
        opts.secret_key = saved_cfg.secret_key.clone();
    }
    for t in &saved_cfg.bootstrap_tickets {
        if !opts.bootstrap_tickets.contains(t) {
            opts.bootstrap_tickets.push(t.clone());
        }
    }
    let mut active_channel = normalize_channel_name(Some(&opts.channel));
    let validated_password = opts
        .secret_key
        .as_deref()
        .and_then(|raw| validate_channel_password(raw).ok());
    let mut active_keys: Option<DerivedChannelKeys> = if let Some(ref pass) = validated_password {
        Some(saved_cfg.apply_channel_password(&active_channel, pass))
    } else if saved_cfg.channel == active_channel {
        saved_cfg.active_channel_keys()
    } else {
        None
    };
    let has_mesh_credentials = active_keys.is_some();

    if opts.install_daemon {
        if let Ok(rep) =
            install_persistent_os_daemon(&opts.node, &opts.listen, validated_password.as_deref())
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
    let (control_tx, mut control_rx) = tokio::sync::mpsc::unbounded_channel::<MeshControlCommand>();
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
        endpoint_id: "booting-iroh-endpoint".to_string(),
        address_lookup_modes: opts.transport.active_lookup_modes(),
        ticket_addr_modes: vec!["relay".to_string(), "direct-ip".to_string()],
        exclude_tailnet: opts.transport.exclude_tailnet,
        bound_non_tailnet_ipv4: None,
        advertised_relay_urls: Vec::new(),
        advertised_ip_addrs: Vec::new(),
    };

    let shared_state: Option<Arc<Mutex<DaemonRuntimeState>>> = if serve_http {
        let listener = bind_local_web_listener(&opts.listen, opts.listen_explicit)?;
        let bound_addr = listener.local_addr().map_err(|e| e.to_string())?;
        opts.listen = bound_addr.to_string();
        local_web_url = format!("http://{bound_addr}");

        let mut runtime_state = if opts.action == "index" {
            DaemonRuntimeState::new_seeded(
                &opts.node,
                &opts.action,
                &opts.listen,
                blob_dir.clone(),
                placeholder_diag.clone(),
            )
        } else {
            DaemonRuntimeState::new_live(
                &opts.node,
                &opts.action,
                &opts.listen,
                blob_dir.clone(),
                placeholder_diag.clone(),
            )
        };
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
        saved_cfg.listen_addr = Some(opts.listen.clone());
        saved_cfg.channel = active_channel.clone();
        saved_cfg.configured = has_mesh_credentials;
        saved_cfg.channel_fingerprint = fp;
        saved_cfg.bootstrap_tickets = opts.bootstrap_tickets.clone();
        saved_cfg.updated_at_unix = now_unix();
        let _ = save_mesh_config(&config_path, &saved_cfg);
        runtime_state.mesh_config = saved_cfg.clone();

        for mut saved_proc in saved_cfg.managed_processes.clone() {
            if let Ok((verified_sha, staged_path)) = dialtone::supervisor::stage_executable_blob(
                &blob_dir,
                &saved_proc.name,
                &saved_proc.sha256,
            ) {
                saved_proc.sha256 = verified_sha;
                saved_proc.path = staged_path.to_string_lossy().to_string();
            }
            if let Some(existing) = runtime_state
                .managed_processes
                .iter_mut()
                .find(|p| p.name == saved_proc.name)
            {
                *existing = saved_proc;
            } else {
                runtime_state.managed_processes.push(saved_proc);
            }
        }
        runtime_state.reconcile_live_processes();

        if !opts.once {
            let mut boot_hb = build_self_heartbeat(
                &opts.node,
                "initializing-endpoint",
                "iroh-gossip-v1:booting".to_string(),
                0,
                placeholder_diag,
            );
            boot_hb.managed_processes = runtime_state.managed_processes.clone();
            if let Some(ref keys) = active_keys {
                sign_heartbeat_with_keys(&mut boot_hb, keys);
            }
            runtime_state.nodes = vec![boot_hb];
        }

        let state = Arc::new(Mutex::new(runtime_state));
        let max_reqs = if opts.once { Some(1) } else { None };
        if opts.action == "index" {
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
        println!("  -> Join Channel CLI  : dialtone join <PASSWORD>");
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
    std::fs::create_dir_all(&blob_dir)
        .map_err(|e| format!("create blob store {}: {e}", blob_dir.display()))?;
    let probe_bytes = format!("dialtone-cas-ready-{}", opts.node);
    let (probe_sha, _) = store_verified_blob(&blob_dir, probe_bytes.as_bytes(), None)?;
    update_boot_stage(
        &shared_state,
        3,
        "ready",
        format!(
            "SHA-256 CAS active at {} (self-check {})",
            blob_dir.display(),
            &probe_sha[..18]
        ),
    );
    if verbose_boot {
        println!(
            "[BOOT 3/5] Content-Addressed Blob Store (iroh-blobs) ..... [READY] {} (stage blobs via Web UI or POST /api/artifacts/blob)",
            blob_dir.display()
        );
    }

    // =========================================================================
    // [BOOT 4/5] Iroh 1.3 QUIC Endpoint & Non-Tailnet Physical NIC
    // =========================================================================
    let key_path = default_secret_key_path();
    let secret_key = load_or_create_secret_key_at(&key_path)?;
    let memory_lookup = MemoryLookup::new();
    let endpoint = build_endpoint_builder(&opts.transport)?
        .address_lookup(memory_lookup.clone())
        .secret_key(secret_key)
        .bind()
        .await
        .map_err(|e| format!("bind iroh endpoint: {e}"))?;
    let endpoint_id = endpoint.id().to_string();

    if opts.transport.relay_mode != IrohRelayPolicy::Disabled && !opts.once {
        let _ = tokio::time::timeout(Duration::from_secs(4), endpoint.online()).await;
    }

    let gossip = Gossip::builder()
        .max_message_size(65_536)
        .spawn(endpoint.clone());
    let blobs_mem_store = iroh_blobs::store::mem::MemStore::new();
    let blobs_proto = iroh_blobs::BlobsProtocol::new(&blobs_mem_store, None);
    let router = Router::builder(endpoint.clone())
        .accept(GOSSIP_ALPN, gossip.clone())
        .accept(iroh_blobs::ALPN, blobs_proto)
        .spawn();

    if let Some(st_arc) = &shared_state {
        if let Ok(mut st) = st_arc.lock() {
            st.blob_bridge = Some(LiveBlobBridge {
                endpoint: endpoint.clone(),
                store: blobs_mem_store.clone(),
                memory_lookup: memory_lookup.clone(),
                transport_config: opts.transport.clone(),
                runtime_handle: tokio::runtime::Handle::current(),
            });
        }
    }

    let advertised = advertised_endpoint_addr(&endpoint.addr(), &opts.transport);
    let diagnostics =
        build_diagnostics(&opts.transport, &advertised, Some(&key_path), &endpoint_id);

    update_boot_stage(
        &shared_state,
        4,
        "ready",
        format!(
            "Bound endpoint {}.. on physical IP {}",
            &endpoint_id[..12],
            diagnostics
                .bound_non_tailnet_ipv4
                .as_deref()
                .unwrap_or("LAN/WAN")
        ),
    );
    if verbose_boot {
        println!(
            "[BOOT 4/5] Iroh 1.3 QUIC Endpoint (No-Tailnet NIC) ....... [READY] id={}.. ip={}",
            &endpoint_id[..16],
            diagnostics
                .bound_non_tailnet_ipv4
                .as_deref()
                .unwrap_or("LAN/WAN")
        );
    }

    // =========================================================================
    // [BOOT 5/5] Gossip Mesh & Channel Password (iroh-gossip)
    // =========================================================================
    let mut invite_peer_addrs = Vec::new();
    if let Some(ref keys) = active_keys {
        let now = now_unix();
        let raw_tickets_snapshot = opts.bootstrap_tickets.clone();
        let mut redeemed_any = false;
        for raw_t in &raw_tickets_snapshot {
            if raw_t.starts_with(INVITE_TICKET_PREFIX) {
                if let Ok(parsed_inv) = raw_t.parse::<dialtone::gossip::InviteTicket>() {
                    if let Ok(issuer_addr) =
                        redeem_invite_ticket(&parsed_inv, keys, &endpoint_id, now, &mut saved_cfg)
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

    let (mut topic_id, mut channel_fp) = if let Some(ref keys) = active_keys {
        (keys.topic_id, keys.fingerprint())
    } else {
        (
            derive_standby_topic_id(&endpoint_id),
            format!("{active_channel}:locked-standby"),
        )
    };

    let mut known_peer_addrs: std::collections::HashMap<iroh::EndpointId, iroh::EndpointAddr> =
        std::collections::HashMap::new();
    let mut joined_peer_ids: std::collections::HashSet<iroh::EndpointId> =
        std::collections::HashSet::new();
    let mut bootstrap_ids = Vec::new();

    // Dial fleet seed peers once valid channel keys are active (either via relay or static direct IP).
    if has_mesh_credentials && !opts.once {
        for seed_addr in default_fleet_seed_peers() {
            let clean_seed = advertised_endpoint_addr(&seed_addr, &opts.transport);
            if clean_seed.id != endpoint.id()
                && (opts.transport.relay_mode != IrohRelayPolicy::Disabled
                    || clean_seed.ip_addrs().next().is_some())
            {
                if joined_peer_ids.insert(clean_seed.id) {
                    bootstrap_ids.push(clean_seed.id);
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
                    bootstrap_ids.push(clean_addr.id);
                }
                known_peer_addrs.insert(clean_addr.id, clean_addr.clone());
            }
            memory_lookup.add_endpoint_info(clean_addr);
        }

        for raw_ticket in &opts.bootstrap_tickets {
            if let Ok(parsed) = GossipTicket::from_str(raw_ticket) {
                for peer_addr in parsed.peers {
                    let clean_addr = advertised_endpoint_addr(&peer_addr, &opts.transport);
                    if clean_addr.id != endpoint.id() {
                        if joined_peer_ids.insert(clean_addr.id) {
                            bootstrap_ids.push(clean_addr.id);
                        }
                        known_peer_addrs.insert(clean_addr.id, clean_addr.clone());
                    }
                    memory_lookup.add_endpoint_info(clean_addr);
                }
            }
        }
    }

    let initial_ticket = GossipTicket {
        topic: topic_id,
        peers: vec![advertised.clone()],
    }
    .to_text()?;

    let mut rolling_window = TelemetryRollingWindow::new(SUMMARY_HEARTBEAT_INTERVAL_SECS);
    let mut last_rx_from_peer: std::collections::HashMap<String, (u64, u64)> =
        std::collections::HashMap::new();
    let mut measured_peer_rtt_ms: std::collections::BTreeMap<String, f64> =
        std::collections::BTreeMap::new();
    let mut last_echo_reply_mono_ms: std::collections::HashMap<String, u64> =
        std::collections::HashMap::new();

    let mut initial_self_hb = build_self_heartbeat(
        &opts.node,
        &endpoint_id,
        initial_ticket,
        1,
        diagnostics.clone(),
    );
    initial_self_hb.heartbeat_kind = "summary".to_string();
    initial_self_hb.rtt_probe_requested = true;

    if let Some(st_arc) = &shared_state {
        if let Ok(mut st) = st_arc.lock() {
            st.reconcile_live_processes();
            initial_self_hb.managed_processes = st.managed_processes.clone();
            initial_self_hb.service_endpoints = st.service_endpoints.clone();
        }
    }

    let init_restarts: u32 = initial_self_hb
        .managed_processes
        .iter()
        .map(|p| p.restart_count)
        .sum();
    let now_init = now_unix();
    rolling_window.record_sample(
        now_init,
        &initial_self_hb.host_resources,
        &measured_peer_rtt_ms,
        init_restarts,
    );
    initial_self_hb.summary_stats = Some(rolling_window.summarize(now_init));

    if let Some(ref keys) = active_keys {
        sign_heartbeat_with_keys(&mut initial_self_hb, keys);
    }

    if let Some(st_arc) = &shared_state {
        if let Ok(mut st) = st_arc.lock() {
            st.iroh_runtime = diagnostics.clone();
            st.host_resources = initial_self_hb.host_resources.clone();
            st.nodes = vec![initial_self_hb.clone()];
            st.refresh_live_alerts();
        }
    }

    let topic = gossip
        .subscribe(topic_id, bootstrap_ids.clone())
        .await
        .map_err(|e| format!("subscribe gossip topic: {e}"))?;
    let (mut sender, mut receiver) = topic.split();
    if !bootstrap_ids.is_empty() {
        let _ = tokio::time::timeout(
            Duration::from_millis(800),
            sender.join_peers(bootstrap_ids.clone()),
        )
        .await;
    }

    if has_mesh_credentials {
        update_boot_stage(
            &shared_state,
            5,
            "ready",
            format!(
                "Unlocked '{}' channel ({}) with {} seed/bootstrap peers",
                active_channel,
                channel_fp,
                bootstrap_ids.len()
            ),
        );
        if verbose_boot {
            println!(
                "[BOOT 5/5] Gossip Mesh & Channel Password ................ [READY] channel={} fp={} peers={}",
                active_channel,
                channel_fp,
                bootstrap_ids.len()
            );
        }
    } else {
        update_boot_stage(
            &shared_state,
            5,
            "setup_needed",
            format!(
                "Channel '{}' is password-protected — enter password at {} or run 'dialtone join <PASSWORD>'",
                active_channel, local_web_url
            ),
        );
        if verbose_boot {
            println!(
                "[BOOT 5/5] Gossip Mesh & Channel Password ................ [PASSWORD REQUIRED]"
            );
            println!(
                "           -> Enter the '{}' channel password in the Web UI at {}",
                active_channel, local_web_url
            );
            println!("           -> Or run in terminal: dialtone join <PASSWORD>");
        }
    }

    let mut counter = 1u64;
    if opts.once {
        let json = serde_json::to_string_pretty(&initial_self_hb).map_err(|e| e.to_string())?;
        println!("{json}");
        if let Some(ref keys) = active_keys {
            if let Ok(enc) = encrypt_heartbeat_for_gossip(&initial_self_hb, keys) {
                let _ = sender.broadcast(enc.into()).await;
            }
        }
        let _ = router.shutdown().await;
        return Ok(());
    }

    let mut last_broadcast_procs = initial_self_hb.managed_processes.clone();
    let mut sample_tick = tokio::time::interval(Duration::from_secs(1));
    let mut hb_tick = tokio::time::interval(Duration::from_secs(HEARTBEAT_INTERVAL_SECS));
    loop {
        tokio::select! {
            Some(cmd) = control_rx.recv() => {
                match cmd {
                    MeshControlCommand::JoinMesh {
                        channel: new_channel,
                        keys: new_keys,
                        bootstrap_tickets: new_tickets,
                        node_name: maybe_node,
                    } => {
                        if let Some(nn) = maybe_node {
                            opts.node = nn;
                        }
                        active_channel = normalize_channel_name(Some(&new_channel));
                        topic_id = new_keys.topic_id;
                        channel_fp = new_keys.fingerprint();
                        active_keys = Some(new_keys);

                        let mut new_boot_ids = Vec::new();
                        for seed_addr in default_fleet_seed_peers() {
                            let clean = advertised_endpoint_addr(&seed_addr, &opts.transport);
                            if clean.id != endpoint.id() {
                                known_peer_addrs.insert(clean.id, clean.clone());
                                joined_peer_ids.insert(clean.id);
                                new_boot_ids.push(clean.id);
                                memory_lookup.add_endpoint_info(clean);
                            }
                        }
                        for raw_t in &new_tickets {
                            if let Ok(parsed) = GossipTicket::from_str(raw_t) {
                                for p_addr in parsed.peers {
                                    let clean = advertised_endpoint_addr(&p_addr, &opts.transport);
                                    if clean.id != endpoint.id() {
                                        known_peer_addrs.insert(clean.id, clean.clone());
                                        joined_peer_ids.insert(clean.id);
                                        if !new_boot_ids.contains(&clean.id) {
                                            new_boot_ids.push(clean.id);
                                        }
                                        memory_lookup.add_endpoint_info(clean);
                                    }
                                }
                            }
                        }

                        if let Ok(new_topic) = gossip.subscribe(topic_id, new_boot_ids.clone()).await {
                            let (new_s, new_r) = new_topic.split();
                            sender = new_s;
                            receiver = new_r;
                            bootstrap_ids = new_boot_ids.clone();
                            if !new_boot_ids.is_empty() {
                                let _ = tokio::time::timeout(
                                    Duration::from_millis(800),
                                    sender.join_peers(new_boot_ids.clone()),
                                )
                                .await;
                            }
                            println!(
                                "[GOSSIP-JOIN] Unlocked '{}' gossip channel ({}) with {} peers",
                                active_channel,
                                channel_fp,
                                new_boot_ids.len()
                            );
                        }
                    }
                }
            }
            _ = sample_tick.tick() => {
                let bound_ip = diagnostics.bound_non_tailnet_ipv4.as_deref();
                let res = sample_live_host_resources(&opts.node, bound_ip, counter);
                let mut total_restarts = 0u32;
                let mut procs_changed = None;
                if let Some(st_arc) = &shared_state {
                    if let Ok(mut st) = st_arc.lock() {
                        st.reconcile_live_processes();
                        total_restarts = st
                            .managed_processes
                            .iter()
                            .map(|p| p.restart_count)
                            .sum();
                        if st.managed_processes != last_broadcast_procs {
                            last_broadcast_procs = st.managed_processes.clone();
                            procs_changed = Some((
                                st.managed_processes.clone(),
                                st.service_endpoints.clone(),
                            ));
                        }
                    }
                }
                let sample_now = now_unix();
                rolling_window.record_sample(
                    sample_now,
                    &res,
                    &measured_peer_rtt_ms,
                    total_restarts,
                );
                let local_summary = rolling_window.summarize(sample_now);

                if let Some(st_arc) = &shared_state {
                    if let Ok(mut st) = st_arc.lock() {
                        st.host_resources = res.clone();
                        let self_name = st.node.clone();
                        if let Some(self_entry) = st.nodes.iter_mut().find(|n| n.node == self_name) {
                            self_entry.host_resources = res.clone();
                            self_entry.peer_rtt_ms = measured_peer_rtt_ms.clone();
                            self_entry.summary_stats = Some(local_summary.clone());
                            if let Some(local_geo) = cached_local_public_ip_geo() {
                                self_entry.geo_location = Some(local_geo);
                            }
                            if let Some((ref mp, ref se)) = procs_changed {
                                self_entry.managed_processes = mp.clone();
                                self_entry.service_endpoints = se.clone();
                            }
                        }
                        let rtt = build_mesh_rtt_matrix(&st.nodes);
                        resolve_fleet_node_locations(&mut st.nodes, &rtt);
                        st.mesh_rtt_ms = rtt;
                        st.refresh_live_alerts();
                    }
                }

                // If supervised processes changed state (spawned, stopped, or crashed/respawned),
                // broadcast an immediate heartbeat so peers observe the transition within ~1s.
                if let (Some((mp, se)), Some(ref keys)) = (procs_changed, &active_keys) {
                    let adv = advertised_endpoint_addr(&endpoint.addr(), &opts.transport);
                    let diag = build_diagnostics(&opts.transport, &adv, Some(&key_path), &endpoint_id);
                    let mut ticket_peers = vec![adv];
                    ticket_peers.extend(known_peer_addrs.values().cloned());
                    if let Ok(current_ticket) = (GossipTicket {
                        topic: topic_id,
                        peers: ticket_peers,
                    })
                    .to_text()
                    {
                        let mut proc_hb = build_self_heartbeat(
                            &opts.node,
                            &endpoint_id,
                            current_ticket,
                            counter,
                            diag,
                        );
                        proc_hb.host_resources = res;
                        proc_hb.managed_processes = mp;
                        proc_hb.service_endpoints = se;
                        proc_hb.peer_rtt_ms = measured_peer_rtt_ms.clone();
                        proc_hb.summary_stats = Some(local_summary);
                        let now_mono = monotonic_now_ms();
                        proc_hb.sent_mono_ms = now_mono;
                        proc_hb.peer_echoes = build_peer_echoes(&last_rx_from_peer, now_mono);
                        sign_heartbeat_with_keys(&mut proc_hb, keys);
                        if let Ok(wire_bytes) = encrypt_heartbeat_for_gossip(&proc_hb, keys) {
                            let _ = tokio::time::timeout(
                                Duration::from_millis(500),
                                sender.broadcast(wire_bytes.into()),
                            )
                            .await;
                        }
                    }
                }
            }
            _ = hb_tick.tick() => {
                if active_keys.is_some() && !bootstrap_ids.is_empty() {
                    for seed_addr in default_fleet_seed_peers() {
                        if seed_addr.id != endpoint.id() {
                            memory_lookup.add_endpoint_info(seed_addr);
                        }
                    }
                    for p_addr in known_peer_addrs.values() {
                        memory_lookup.add_endpoint_info(p_addr.clone());
                    }
                    let _ = tokio::time::timeout(
                        Duration::from_millis(600),
                        sender.join_peers(bootstrap_ids.clone()),
                    )
                    .await;
                }
                if let Some(st_arc) = &shared_state {
                    let peer_ids: Vec<String> = st_arc
                        .lock()
                        .unwrap()
                        .nodes
                        .iter()
                        .filter(|n| n.node != opts.node)
                        .map(|n| n.endpoint_id.clone())
                        .collect();
                    for pid in peer_ids {
                        if let Ok((obs, tailnet)) = tokio::time::timeout(
                            Duration::from_millis(350),
                            observe_peer_remote_addrs(&endpoint, &pid),
                        )
                        .await
                        {
                            if !obs.is_empty() {
                                let mut st = st_arc.lock().unwrap();
                                if let Some(peer_entry) = st.nodes.iter_mut().find(|n| n.endpoint_id == pid) {
                                    peer_entry.observed_remote_addrs = obs;
                                    peer_entry.tailnet_used = tailnet;
                                }
                            }
                        }
                    }
                }

                let adv = advertised_endpoint_addr(&endpoint.addr(), &opts.transport);
                let diag = build_diagnostics(&opts.transport, &adv, Some(&key_path), &endpoint_id);
                let mut ticket_peers = vec![adv];
                ticket_peers.extend(known_peer_addrs.values().cloned());
                let current_ticket = GossipTicket {
                    topic: topic_id,
                    peers: ticket_peers,
                }
                .to_text()?;
                let mut self_hb = build_self_heartbeat(&opts.node, &endpoint_id, current_ticket, counter, diag.clone());
                self_hb.peer_rtt_ms = measured_peer_rtt_ms.clone();

                let is_summary = counter <= 2 || counter.is_multiple_of(SUMMARY_HEARTBEAT_EVERY_N_TICKS);
                let local_summary = rolling_window.summarize(now_unix());
                if is_summary {
                    self_hb.heartbeat_kind = "summary".to_string();
                    self_hb.summary_stats = Some(local_summary.clone());
                } else {
                    self_hb.heartbeat_kind = "regular".to_string();
                    self_hb.summary_stats = None;
                }

                let mut missing_peer_rtt = false;
                if let Some(st_arc) = &shared_state {
                    let mut st = st_arc.lock().unwrap();
                    st.reconcile_live_processes();
                    last_broadcast_procs = st.managed_processes.clone();
                    self_hb.managed_processes = st.managed_processes.clone();
                    self_hb.service_endpoints = st.service_endpoints.clone();
                    for n in &st.nodes {
                        if n.node != opts.node && !measured_peer_rtt_ms.contains_key(&n.node) {
                            missing_peer_rtt = true;
                        }
                    }
                    self_hb.rtt_probe_requested =
                        missing_peer_rtt || counter <= 2 || counter.is_multiple_of(3);
                    let now_mono = monotonic_now_ms();
                    self_hb.sent_mono_ms = now_mono;
                    self_hb.peer_echoes = build_peer_echoes(&last_rx_from_peer, now_mono);
                    if let Some(ref keys) = active_keys {
                        sign_heartbeat_with_keys(&mut self_hb, keys);
                    }
                    st.iroh_runtime = diag;
                    st.host_resources = self_hb.host_resources.clone();
                    let mut local_copy = self_hb.clone();
                    local_copy.summary_stats = Some(local_summary.clone());
                    upsert_heartbeat(&mut st.nodes, local_copy);
                    let rtt = build_mesh_rtt_matrix(&st.nodes);
                    resolve_fleet_node_locations(&mut st.nodes, &rtt);
                    st.mesh_rtt_ms = rtt;
                    st.refresh_live_alerts();
                    if is_summary {
                        st.record_log(
                            "gossip-heartbeat",
                            "INFO",
                            "stdout",
                            format!(
                                "published 30s summary heartbeat #{} (cpu {:.1}% ±{:.1}%, mem {:.1}% ±{:.1}%, thrashing={}, rtt_peers={})",
                                counter,
                                local_summary.cpu_pct.mean,
                                local_summary.cpu_pct.stddev,
                                local_summary.mem_pct.mean,
                                local_summary.mem_pct.stddev,
                                local_summary.thrashing_detected,
                                measured_peer_rtt_ms.len()
                            ),
                        );
                    }
                } else {
                    self_hb.rtt_probe_requested = counter <= 2 || counter.is_multiple_of(3);
                    let now_mono = monotonic_now_ms();
                    self_hb.sent_mono_ms = now_mono;
                    self_hb.peer_echoes = build_peer_echoes(&last_rx_from_peer, now_mono);
                    if let Some(ref keys) = active_keys {
                        sign_heartbeat_with_keys(&mut self_hb, keys);
                    }
                }
                if counter == 1 && !serve_http {
                    println!("{}", serde_json::to_string_pretty(&self_hb).unwrap());
                }
                if let Some(ref keys) = active_keys {
                    if let Ok(wire_bytes) = encrypt_heartbeat_for_gossip(&self_hb, keys) {
                        let _ = tokio::time::timeout(
                            Duration::from_millis(600),
                            sender.broadcast(wire_bytes.into()),
                        )
                        .await;
                    }
                }
                counter += 1;
            }
            maybe_event = receiver.next() => {
                match maybe_event {
                    Some(Ok(GossipEvent::NeighborUp(peer_id))) => {
                        let Some(ref keys) = active_keys else {
                            continue;
                        };
                        if active_channel == DEFAULT_CHANNEL_NAME {
                            let allowed_ids = if let Some(st_arc) = &shared_state {
                                st_arc
                                    .lock()
                                    .ok()
                                    .map(|st| st.mesh_config.allowed_endpoint_ids.clone())
                                    .unwrap_or_else(|| saved_cfg.allowed_endpoint_ids.clone())
                            } else {
                                saved_cfg.allowed_endpoint_ids.clone()
                            };
                            if !is_allowed_peer_in_list(
                                &allowed_ids,
                                &peer_id.to_string(),
                                Some(&endpoint_id),
                            ) {
                                if !bootstrap_ids.is_empty() {
                                    let _ = tokio::time::timeout(
                                        Duration::from_millis(400),
                                        sender.join_peers(bootstrap_ids.clone()),
                                    )
                                    .await;
                                }
                                continue;
                            }
                        }
                        let adv = advertised_endpoint_addr(&endpoint.addr(), &opts.transport);
                        let diag = build_diagnostics(&opts.transport, &adv, Some(&key_path), &endpoint_id);
                        let mut ticket_peers = vec![adv];
                        ticket_peers.extend(known_peer_addrs.values().cloned());
                        let current_ticket = GossipTicket {
                            topic: topic_id,
                            peers: ticket_peers,
                        }
                        .to_text()?;
                        let mut self_hb = build_self_heartbeat(&opts.node, &endpoint_id, current_ticket, counter, diag);
                        let now_mono = monotonic_now_ms();
                        self_hb.heartbeat_kind = "summary".to_string();
                        self_hb.sent_mono_ms = now_mono;
                        self_hb.rtt_probe_requested = true;
                        self_hb.peer_echoes = build_peer_echoes(&last_rx_from_peer, now_mono);
                        self_hb.peer_rtt_ms = measured_peer_rtt_ms.clone();
                        self_hb.summary_stats = Some(rolling_window.summarize(now_unix()));
                        if let Some(st_arc) = &shared_state {
                            if let Ok(mut st) = st_arc.lock() {
                                st.reconcile_live_processes();
                                self_hb.managed_processes = st.managed_processes.clone();
                                self_hb.service_endpoints = st.service_endpoints.clone();
                            }
                        }
                        sign_heartbeat_with_keys(&mut self_hb, keys);
                        if let Ok(wire_bytes) = encrypt_heartbeat_for_gossip(&self_hb, keys) {
                            let _ = tokio::time::timeout(
                                Duration::from_millis(600),
                                sender.broadcast(wire_bytes.into()),
                            )
                            .await;
                        }
                    }
                    Some(Ok(GossipEvent::NeighborDown(_peer_id))) => {
                        if active_keys.is_some() && !bootstrap_ids.is_empty() {
                            let _ = tokio::time::timeout(
                                Duration::from_millis(400),
                                sender.join_peers(bootstrap_ids.clone()),
                            )
                            .await;
                        }
                    }
                    Some(Ok(GossipEvent::Received(msg))) => {
                        let Some(ref keys) = active_keys else {
                            continue;
                        };
                        let mut rx_hb = match decrypt_heartbeat_from_gossip(&msg.content, keys) {
                            Ok(hb) => hb,
                            Err(reason) => {
                                eprintln!(
                                    "[gossip-auth-reject] dropped gossip payload on channel '{}': {}",
                                    active_channel, reason
                                );
                                continue;
                            }
                        };
                        if rx_hb.node != opts.node {
                            if active_channel == DEFAULT_CHANNEL_NAME {
                                let allowed_ids = if let Some(st_arc) = &shared_state {
                                    st_arc
                                        .lock()
                                        .ok()
                                        .map(|st| st.mesh_config.allowed_endpoint_ids.clone())
                                        .unwrap_or_else(|| saved_cfg.allowed_endpoint_ids.clone())
                                } else {
                                    saved_cfg.allowed_endpoint_ids.clone()
                                };
                                if !is_allowed_peer_in_list(
                                    &allowed_ids,
                                    &rx_hb.endpoint_id,
                                    Some(&endpoint_id),
                                ) {
                                    eprintln!(
                                        "[gossip-allowlist-reject] dropped unlisted peer node={} endpoint_id={} on main 'dialtone' channel (run 'dialtone peers allow {}' to authorize)",
                                        rx_hb.node, rx_hb.endpoint_id, rx_hb.endpoint_id
                                    );
                                    continue;
                                }
                            }

                            let rx_mono = monotonic_now_ms();
                            if rx_hb.sent_mono_ms > 0 {
                                last_rx_from_peer
                                    .insert(rx_hb.node.clone(), (rx_hb.sent_mono_ms, rx_mono));
                            }
                            if let Some(echo) = rx_hb.peer_echoes.get(&opts.node) {
                                if echo.hold_time_ms <= 600
                                    || !measured_peer_rtt_ms.contains_key(&rx_hb.node)
                                {
                                    if let Some(rtt_ms) = compute_rtt_ms_from_echo(rx_mono, echo) {
                                        let updated_rtt = if let Some(&prev) =
                                            measured_peer_rtt_ms.get(&rx_hb.node)
                                        {
                                            (((prev * 0.6) + (rtt_ms * 0.4)) * 10.0).round() / 10.0
                                        } else {
                                            rtt_ms
                                        };
                                        measured_peer_rtt_ms.insert(rx_hb.node.clone(), updated_rtt);
                                    }
                                }
                            }
                            if let Some(&peer_reported) = rx_hb.peer_rtt_ms.get(&opts.node) {
                                if peer_reported > 0.0 {
                                    measured_peer_rtt_ms
                                        .entry(rx_hb.node.clone())
                                        .or_insert(peer_reported);
                                }
                            }
                            if let Some(&our_rtt) = measured_peer_rtt_ms.get(&rx_hb.node) {
                                rx_hb.peer_rtt_ms.insert(opts.node.clone(), our_rtt);
                            }

                            let mut discovered_addrs = Vec::new();
                            if let Ok(peer_ticket) = GossipTicket::from_str(&rx_hb.ticket) {
                                discovered_addrs.extend(peer_ticket.peers);
                            }
                            if let Ok(pid) = rx_hb.endpoint_id.parse::<iroh::EndpointId>() {
                                let mut direct_addr = iroh::EndpointAddr::new(pid);
                                for r in &rx_hb.iroh_transport.advertised_relay_urls {
                                    if let Ok(rurl) = r.parse::<iroh::RelayUrl>() {
                                        direct_addr = direct_addr.with_relay_url(rurl);
                                    }
                                }
                                for ip_s in &rx_hb.iroh_transport.advertised_ip_addrs {
                                    if let Ok(sock) = ip_s.parse::<std::net::SocketAddr>() {
                                        direct_addr = direct_addr.with_ip_addr(sock);
                                    }
                                }
                                discovered_addrs.push(direct_addr);
                            }
                            let mut new_peers = Vec::new();
                            for p_addr in discovered_addrs {
                                let clean = advertised_endpoint_addr(&p_addr, &opts.transport);
                                if clean.id != endpoint.id() {
                                    known_peer_addrs.insert(clean.id, clean.clone());
                                    if joined_peer_ids.insert(clean.id) {
                                        new_peers.push(clean.id);
                                    }
                                }
                                memory_lookup.add_endpoint_info(clean);
                            }
                            if !new_peers.is_empty() {
                                for np in &new_peers {
                                    if !bootstrap_ids.contains(np) {
                                        bootstrap_ids.push(*np);
                                    }
                                }
                                let _ = tokio::time::timeout(
                                    Duration::from_millis(600),
                                    sender.join_peers(new_peers),
                                )
                                .await;
                            }
                            let (obs, tailnet) = tokio::time::timeout(
                                Duration::from_millis(400),
                                observe_peer_remote_addrs(&endpoint, &rx_hb.endpoint_id),
                            )
                            .await
                            .unwrap_or_else(|_| (vec!["iroh-quic:active".to_string()], false));
                            rx_hb.observed_remote_addrs = obs;
                            rx_hb.tailnet_used = tailnet;

                            let rtt_log = measured_peer_rtt_ms
                                .get(&rx_hb.node)
                                .map(|v| format!("{v:.1}ms"))
                                .unwrap_or_else(|| "probing".to_string());
                            println!(
                                "[gossip-rx] node={} kind={} counter={} rtt={} addrs={:?} tailnet_used={}",
                                rx_hb.node,
                                rx_hb.heartbeat_kind,
                                rx_hb.counter,
                                rtt_log,
                                rx_hb.observed_remote_addrs,
                                rx_hb.tailnet_used
                            );

                            let peer_node_name = rx_hb.node.clone();
                            let need_echo_reply = rx_hb.rtt_probe_requested
                                || !measured_peer_rtt_ms.contains_key(&peer_node_name);
                            // Stamp with receiver's local clock after MAC verification so NTP skew
                            // between hosts cannot cause premature TTL pruning.
                            rx_hb.updated_at_unix = now_unix();

                            if let Some(st_arc) = &shared_state {
                                let mut st = st_arc.lock().unwrap();
                                let is_first_seen =
                                    !st.nodes.iter().any(|n| n.node == peer_node_name);
                                upsert_heartbeat(&mut st.nodes, rx_hb);
                                let self_name = st.node.clone();
                                if let Some(self_entry) =
                                    st.nodes.iter_mut().find(|n| n.node == self_name)
                                {
                                    self_entry.peer_rtt_ms = measured_peer_rtt_ms.clone();
                                    if let Some(local_geo) = cached_local_public_ip_geo() {
                                        self_entry.geo_location = Some(local_geo);
                                    }
                                }
                                let rtt = build_mesh_rtt_matrix(&st.nodes);
                                resolve_fleet_node_locations(&mut st.nodes, &rtt);
                                st.mesh_rtt_ms = rtt;
                                st.refresh_live_alerts();
                                if is_first_seen {
                                    st.record_event(
                                        "mesh.peer_joined",
                                        "info",
                                        format!("Discovered mesh peer '{}'", peer_node_name),
                                    );
                                    st.record_log(
                                        "gossip-mesh",
                                        "INFO",
                                        "stdout",
                                        format!(
                                            "peer '{}' joined active gossip swarm (rtt={})",
                                            peer_node_name, rtt_log
                                        ),
                                    );
                                }
                            }

                            let last_reply = last_echo_reply_mono_ms
                                .get(&peer_node_name)
                                .copied()
                                .unwrap_or(0);
                            if need_echo_reply && rx_mono.saturating_sub(last_reply) >= 1_500 {
                                last_echo_reply_mono_ms.insert(peer_node_name.clone(), rx_mono);
                                let adv =
                                    advertised_endpoint_addr(&endpoint.addr(), &opts.transport);
                                let diag = build_diagnostics(
                                    &opts.transport,
                                    &adv,
                                    Some(&key_path),
                                    &endpoint_id,
                                );
                                let mut ticket_peers = vec![adv];
                                ticket_peers.extend(known_peer_addrs.values().cloned());
                                if let Ok(current_ticket) = (GossipTicket {
                                    topic: topic_id,
                                    peers: ticket_peers,
                                })
                                .to_text()
                                {
                                    let mut echo_hb = build_self_heartbeat(
                                        &opts.node,
                                        &endpoint_id,
                                        current_ticket,
                                        counter,
                                        diag,
                                    );
                                    let reply_mono = monotonic_now_ms();
                                    echo_hb.heartbeat_kind = "rtt_echo".to_string();
                                    echo_hb.sent_mono_ms = reply_mono;
                                    echo_hb.rtt_probe_requested =
                                        !measured_peer_rtt_ms.contains_key(&peer_node_name);
                                    echo_hb.peer_echoes =
                                        build_peer_echoes(&last_rx_from_peer, reply_mono);
                                    echo_hb.peer_rtt_ms = measured_peer_rtt_ms.clone();
                                    echo_hb.summary_stats =
                                        Some(rolling_window.summarize(now_unix()));
                                    if let Some(st_arc) = &shared_state {
                                        if let Ok(st) = st_arc.lock() {
                                            echo_hb.managed_processes =
                                                st.managed_processes.clone();
                                            echo_hb.service_endpoints =
                                                st.service_endpoints.clone();
                                        }
                                    }
                                    sign_heartbeat_with_keys(&mut echo_hb, keys);
                                    if let Ok(wire_bytes) =
                                        encrypt_heartbeat_for_gossip(&echo_hb, keys)
                                    {
                                        let _ = tokio::time::timeout(
                                            Duration::from_millis(400),
                                            sender.broadcast(wire_bytes.into()),
                                        )
                                        .await;
                                    }
                                }
                            }
                        }
                    }
                    Some(Ok(_)) => {}
                    Some(Err(err)) => {
                        eprintln!("[gossip-err] {err}");
                        if let Ok(new_topic) = gossip.subscribe(topic_id, bootstrap_ids.clone()).await {
                            let (new_s, new_r) = new_topic.split();
                            sender = new_s;
                            receiver = new_r;
                        }
                    }
                    None => {
                        if let Ok(new_topic) = gossip.subscribe(topic_id, bootstrap_ids.clone()).await {
                            let (new_s, new_r) = new_topic.split();
                            sender = new_s;
                            receiver = new_r;
                        }
                    }
                }
            }
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
        "gen-password" => {
            let pw = generate_channel_password();
            println!("{pw}");
            Ok(())
        }
        "peers" => (|| -> Result<(), String> {
            let cfg_path = default_config_path();
            match opts.peers_subcmd.as_str() {
                "list" | "ls" => {
                    let cfg = load_mesh_config(&cfg_path);
                    println!(
                        "{}",
                        serde_json::to_string_pretty(&json!({
                            "schema": "dialtone.peers_allowlist.v1",
                            "config_path": cfg_path,
                            "default_policy": "deny_unless_listed",
                            "allowed_endpoint_ids": cfg.allowed_endpoint_ids,
                            "seed_peers": cfg.seed_peers
                        }))
                        .unwrap()
                    );
                    Ok(())
                }
                "allow" | "add" => {
                    let target = opts
                        .peer_target
                        .as_deref()
                        .ok_or_else(|| "usage: dialtone peers allow <endpoint_id> [--addr <ip:port>]".to_string())?;
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
                            "endpoint_id": target.trim().to_ascii_lowercase(),
                            "allowed_endpoint_ids": updated.allowed_endpoint_ids,
                            "seed_peers": updated.seed_peers,
                            "config_path": cfg_path
                        }))
                        .unwrap()
                    );
                    Ok(())
                }
                "revoke" | "remove" | "rm" => {
                    let target = opts
                        .peer_target
                        .as_deref()
                        .ok_or_else(|| "usage: dialtone peers revoke <endpoint_id>".to_string())?;
                    let updated = revoke_peer_in_config(&cfg_path, target)?;
                    println!(
                        "{}",
                        serde_json::to_string_pretty(&json!({
                            "status": "revoked",
                            "endpoint_id": target.trim().to_ascii_lowercase(),
                            "allowed_endpoint_ids": updated.allowed_endpoint_ids,
                            "config_path": cfg_path
                        }))
                        .unwrap()
                    );
                    Ok(())
                }
                other => Err(format!(
                    "unknown peers subcommand {other:?}; use: dialtone peers [allow|revoke|list] [<endpoint_id>]"
                )),
            }
        })(),
        "blobs" => (|| -> Result<(), String> {
            let api_token = opts
                .api_token
                .clone()
                .or_else(|| load_or_create_api_token().ok())
                .unwrap_or_default();
            match opts.blobs_subcmd.as_str() {
                "share" | "push" | "stage" => {
                    let arg = opts.blobs_target.as_deref().ok_or_else(|| {
                        "usage: dialtone blobs share <file-path-or-payload>".to_string()
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
                        "/api/blobs/share",
                        Some(&json!({ "bytes_base64": b64 })),
                        &api_token,
                        15,
                    )?;
                    println!("{}", serde_json::to_string_pretty(&resp).unwrap());
                    Ok(())
                }
                "pull" | "fetch" | "get" => {
                    let ticket_uri = opts
                        .blobs_target
                        .as_deref()
                        .or_else(|| {
                            if opts.uri.starts_with("iroh-blobs://ticket/")
                                && !opts.uri.contains("blob123")
                            {
                                Some(opts.uri.as_str())
                            } else {
                                None
                            }
                        })
                        .ok_or_else(|| {
                            "usage: dialtone blobs pull <iroh-blobs://ticket/...#sha256:...> [--force]"
                                .to_string()
                        })?;
                    let resp = local_daemon_http_json(
                        &opts.listen,
                        "POST",
                        "/api/blobs/pull",
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
                    "unknown blobs subcommand {other:?}; use: dialtone blobs [share|pull] <arg>"
                )),
            }
        })(),
        "fleet-test" => (|| -> Result<(), String> {
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
            if cfg_disk.allowed_endpoint_ids.is_empty() {
                return Err(
                    "security check failed: allowed_endpoint_ids is empty in config.json"
                        .to_string(),
                );
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
                .and_then(|r| r.get("endpoint_id"))
                .and_then(|v| v.as_str());
            let expected_peers = opts
                .expect_peers
                .unwrap_or_else(|| cfg_disk.allowed_endpoint_ids.len().max(1));
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
                let mut all_telemetry_real = true;
                for n in &nodes {
                    let ep_id = n.get("endpoint_id").and_then(|v| v.as_str()).unwrap_or("");
                    let tailnet_used = n
                        .get("tailnet_used")
                        .and_then(|v| v.as_bool())
                        .unwrap_or(false);
                    let mac_ok = n
                        .get("channel_mac")
                        .and_then(|v| v.as_str())
                        .map(|s| !s.is_empty())
                        .unwrap_or(false);
                    if is_allowed_peer_in_list(&cfg_disk.allowed_endpoint_ids, ep_id, own_ep_id)
                        && mac_ok
                        && (!opts.transport.exclude_tailnet || !tailnet_used)
                    {
                        let hr = n.get("host_resources").cloned().unwrap_or(json!({}));
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
                            all_telemetry_real = false;
                        }
                        if n.get("summary_stats").is_none()
                            || n.get("summary_stats").map(|v| v.is_null()).unwrap_or(true)
                        {
                            all_telemetry_real = false;
                        }
                        valid_peers.push(json!({
                            "node": n.get("node"),
                            "os": n.get("os"),
                            "arch": n.get("arch"),
                            "endpoint_id": ep_id,
                            "counter": n.get("counter"),
                            "heartbeat_kind": n.get("heartbeat_kind"),
                            "heartbeat_interval_secs": n.get("heartbeat_interval_secs"),
                            "summary_interval_secs": n.get("summary_interval_secs"),
                            "updated_at_unix": n.get("updated_at_unix"),
                            "observed_remote_addrs": n.get("observed_remote_addrs"),
                            "tailnet_used": tailnet_used,
                            "peer_rtt_ms": n.get("peer_rtt_ms"),
                            "geo_location": n.get("geo_location"),
                            "host_resources": {
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
                if valid_peers.len() >= expected_peers && all_telemetry_real && rtt_ready {
                    break (valid_peers, serde_json::Value::Object(rtt_matrix));
                }
                if std::time::Instant::now() >= deadline {
                    return Err(format!(
                        "fleet convergence timed out after {}s: expected {} allowlisted peers with real telemetry & RTT, observed {} peers (all_telemetry_real={}, rtt_pairs={})",
                        opts.timeout_secs,
                        expected_peers,
                        valid_peers.len(),
                        all_telemetry_real,
                        rtt_matrix.len()
                    ));
                }
                std::thread::sleep(Duration::from_millis(500));
            };

            let probe_payload = format!("dialtone-fleet-test-probe-{}-{}", opts.node, now_unix());
            let share_resp = local_daemon_http_json(
                &opts.listen,
                "POST",
                "/api/blobs/share",
                Some(&json!({
                    "bytes_base64": dialtone::base64_encode(probe_payload.as_bytes())
                })),
                &api_token,
                10,
            )?;
            let ticket_uri = share_resp
                .get("ticket_uri")
                .and_then(|v| v.as_str())
                .ok_or("missing ticket_uri from POST /api/blobs/share")?;
            let pull_resp = local_daemon_http_json(
                &opts.listen,
                "POST",
                "/api/blobs/pull",
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
                    "schema": "dialtone.fleet_test.v1",
                    "status": "ok",
                    "node": config_resp.get("node"),
                    "channel_fingerprint": cfg_disk.channel_fingerprint,
                    "kdf_version": kdf_ver,
                    "heartbeat_interval_secs": HEARTBEAT_INTERVAL_SECS,
                    "summary_interval_secs": SUMMARY_HEARTBEAT_INTERVAL_SECS,
                    "real_telemetry_verified": true,
                    "summary_stats_verified": true,
                    "public_directory_publishing": false,
                    "address_lookup_modes": lookup_modes,
                    "expected_peers": expected_peers,
                    "converged_peers": converged_nodes.len(),
                    "mesh_rtt_ms": mesh_rtt_ms,
                    "nodes": converged_nodes,
                    "live_blob_quic_transfer": {
                        "sha256": pull_resp.get("sha256"),
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
                let valid = validate_channel_password(raw_pass)?;
                cfg.apply_channel_password(&channel, &valid)
            } else if let Some(k) = cfg.active_channel_keys() {
                k
            } else {
                return Err(
                    "cannot mint invite ticket: no channel keys configured (pass --password <PASSWORD> or run 'dialtone join <PASSWORD>' first)"
                        .to_string(),
                );
            };

            let key_path = default_secret_key_path();
            let secret_key = load_or_create_secret_key_at(&key_path)?;
            let endpoint_id = secret_key.public().to_string();

            let now = now_unix();
            let invite = create_invite_ticket(
                &endpoint_id,
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
                    "inviter_endpoint_id": endpoint_id,
                    "bound_endpoint_id": opts.peer_target,
                    "ttl_secs": opts.invite_ttl_secs,
                    "expires_at_unix": invite.expires_at_unix,
                    "invite_ticket": invite_token
                }))
                .unwrap()
            );
            Ok(())
        })(),
        "join" => {
            let Some(raw_secret) = opts
                .secret_key
                .clone()
                .or_else(|| opts.bootstrap_tickets.first().cloned())
            else {
                eprintln!(
                    "dialtone: usage: dialtone join <PASSWORD> [--channel dialtone] [--bootstrap-ticket <TICKET_OR_INVITE>] [--listen 127.0.0.1:8788]"
                );
                std::process::exit(2);
            };
            let channel = normalize_channel_name(Some(&opts.channel));
            let password = match validate_channel_password(&raw_secret) {
                Ok(p) => p,
                Err(err) => {
                    eprintln!("dialtone: {err}");
                    std::process::exit(2);
                }
            };
            let mut cfg = load_mesh_config(&default_config_path());
            cfg.node_name = Some(opts.node.clone());
            cfg.listen_addr = Some(opts.listen.clone());
            let keys = cfg.apply_channel_password(&channel, &password);
            let fp = keys.fingerprint();
            let topic_id = keys.topic_id;

            let key_path = default_secret_key_path();
            let self_ep_id = load_or_create_secret_key_at(&key_path)
                .ok()
                .map(|k| k.public().to_string())
                .unwrap_or_default();
            let now = now_unix();

            for t in &opts.bootstrap_tickets {
                if t.starts_with(INVITE_TICKET_PREFIX) {
                    let parsed_inv = match t.parse::<dialtone::gossip::InviteTicket>() {
                        Ok(inv) => inv,
                        Err(err) => {
                            eprintln!("dialtone: {err}");
                            std::process::exit(2);
                        }
                    };
                    if let Err(err) =
                        redeem_invite_ticket(&parsed_inv, &keys, &self_ep_id, now, &mut cfg)
                    {
                        eprintln!("dialtone: {err}");
                        std::process::exit(2);
                    }
                } else if !cfg.bootstrap_tickets.contains(t) {
                    cfg.bootstrap_tickets.push(t.clone());
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
                &password,
                &opts.bootstrap_tickets,
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
                run_node_or_hub(opts, true).await
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
        "status" | "iroh-runtime-fixture" => {
            collect_node_diagnostics(&opts.transport).await.map(|diag| {
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
            })
        }
        "self-test" => run_self_test().await.map(|report| {
            println!("{}", serde_json::to_string_pretty(&report).unwrap());
        }),
        "state-fixture" => build_seeded_state(&opts).await.map(|st| {
            println!("{}", serde_json::to_string_pretty(&st).unwrap());
        }),
        "mesh-fixture" | "gossip-fixture" => build_seeded_state(&opts).await.map(|st| {
            println!(
                "{}",
                serde_json::to_string_pretty(&json!({
                    "schema": "dialtone.gossip.v1",
                    "mesh": "dialtone-iroh-gossip",
                    "nodes": st.nodes,
                    "service_endpoints": st.service_endpoints
                }))
                .unwrap()
            );
        }),
        "deploy-plan-fixture" => {
            if let Some(case) = &opts.build_case {
                evaluate_deploy_build_decision(case).map(|d| {
                    println!("{}", serde_json::to_string_pretty(&d).unwrap());
                })
            } else {
                generate_deploy_plan(&opts.deploy_target).map(|plan| {
                    println!("{}", serde_json::to_string_pretty(&plan).unwrap());
                })
            }
        }
        "blob-fetch-report" => {
            let cache_dir = env::temp_dir().join("dialtone-blobs");
            evaluate_tiered_blob_fetch(
                &cache_dir,
                &TieredBlobFetchRequest {
                    sha256: opts.sha256,
                    uri: opts.uri,
                    provider_node: opts.node,
                    expected_bytes: 4096,
                    http_fallback_enabled: opts.http_fallback_enabled,
                },
            )
            .map(|plan| {
                println!("{}", serde_json::to_string_pretty(&plan).unwrap());
            })
        }
        "process-intent-fixture" => build_seeded_state(&opts).await.and_then(|st| {
            let proc = st.managed_processes.first().ok_or("no managed process")?;
            let plan = plan_process_intent("node-alpha", &opts.node, proc, &opts.intent_action)?;
            println!("{}", serde_json::to_string_pretty(&plan).unwrap());
            Ok(())
        }),
        "rollback-plan-fixture" => build_seeded_state(&opts).await.and_then(|st| {
            let slot = st.deploy_slots.first().ok_or("no deploy slots")?;
            let plan = plan_rollback(&opts.node, slot)?;
            println!("{}", serde_json::to_string_pretty(&plan).unwrap());
            Ok(())
        }),
        "alert-ack-fixture" => build_seeded_state(&opts).await.and_then(|st| {
            let alert = st.alerts.first().ok_or("no alerts")?;
            let plan = plan_alert_ack(alert, &opts.actor, &opts.note)?;
            println!("{}", serde_json::to_string_pretty(&plan).unwrap());
            Ok(())
        }),
        "proof-validation-fixture" => {
            evaluate_proof_case(&opts.proof_case, "session-cli").map(|rep| {
                println!("{}", serde_json::to_string_pretty(&rep).unwrap());
            })
        }
        "program-catalog-fixture" => build_seeded_state(&opts).await.map(|st| {
            println!(
                "{}",
                serde_json::to_string_pretty(&st.metadata.catalogs).unwrap()
            );
        }),
        "service" => {
            let exe =
                env::current_exe().unwrap_or_else(|_| PathBuf::from("/usr/local/bin/dialtone"));
            let unit = render_os_service_unit(&opts.os_kind, &exe, &opts.node);
            println!("{unit}");
            Ok(())
        }
        "node" => run_node_or_hub(opts, false).await,
        "up" | "start" | "index" | "hub" => run_node_or_hub(opts, true).await,
        other => Err(format!("unsupported action {other:?}")),
    };

    if let Err(err) = result {
        eprintln!("dialtone: {err}");
        std::process::exit(1);
    }
}
