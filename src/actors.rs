//! The running node as actors: one task per job, joined by channels and stopped by one
//! cancellation token.
//!
//! - **Mesh** owns the gossip subscriptions: beats and summaries on the beats topic,
//!   inventories on the events topic (see `Message`), a few neighbors per topic, and
//!   inventory fetches when a beat announces one this node has not seen.
//! - **Stats** samples the host every second and keeps the rolling summary.
//! - **Supervisor** reconciles processes every second and tells Mesh when they change.
//! - **Devices** lists the cameras, microphones, keyboards and other devices connected to
//!   the node every minute and tells Mesh when they change.
//! - **Links** times the Rtt to each gossip neighbor and observes how it is reached
//!   (direct, relay, tailnet).
//! - **Allowlist** reloads the allowlist when `config.json` changes.
//! - **Deploy** pulls, verifies and starts programs that allowlisted peers run.
//! - **Probe** checks the gossip mesh every 30s, and runs full probes (store, pulls,
//!   programs) on request (see `probe.rs`).
//!
//! The API runs on its own threads and reaches Mesh through `MeshControlCommand`.
//! Actors share the node's state (`NodeState`) for reads and writes the HUD shows.

use std::collections::{BTreeMap, HashMap, HashSet};
use std::path::PathBuf;
use std::str::FromStr;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use dialtone::api::NodeState;
use dialtone::blob::Blobs;
use dialtone::fetch;
use dialtone::mesh::{
    beat_fresh_secs, beat_interval_for, build_mesh_rtt_matrix, cached_local_public_ip_geo,
    default_seeds, events_topic_id, is_valid_node_name, merge_beat, merge_inventory,
    monotonic_now_ms, node_capabilities_for, node_role_for, normalize_channel_name, now_unix,
    observe_peer_remote_addrs, open_message, prune_stale_peers, seal_message, try_load_mesh_config,
    Beat, CatchUpHave, ChannelKeys, FrameCache, Inventory, LiveAllowlist, MeshControlCommand,
    Message, NodeDetails, PeerScores, ReplayGuard, SeedList, BEAT_INTERVAL_SECS,
    DEFAULT_PEER_TTL_SECS, SUMMARY_EVERY_N_BEATS, SUMMARY_INTERVAL_SECS,
};
use dialtone::node::{advertised_endpoint_addr, build_diagnostics, IrohTransportConfig};
use dialtone::probe::{ProbeRequest, PULL_SAMPLE};
use dialtone::service::ProcessSpec;
use dialtone::stats::{sample_stats, StatsSummary, StatsWindow};
use futures_lite::StreamExt;
use iroh::address_lookup::memory::MemoryLookup;
use iroh::{EndpointAddr, EndpointId};
use iroh_gossip::api::{Event as GossipEvent, GossipReceiver, GossipSender};
use iroh_gossip::net::Gossip;
use iroh_gossip::proto::TopicId;
use tokio::sync::{mpsc, watch};
use tokio::task::JoinHandle;
use tokio_util::sync::CancellationToken;

use crate::self_beat;

/// Handles every actor shares. Cheap to clone.
#[derive(Clone)]
pub(crate) struct Ctx {
    pub node_id: String,
    pub endpoint: iroh::Endpoint,
    pub memory_lookup: MemoryLookup,
    pub transport: IrohTransportConfig,
    pub key_path: PathBuf,
    pub config_path: PathBuf,
    pub blobs: Blobs,
    pub allowlist: LiveAllowlist,
    pub state: Option<Arc<Mutex<NodeState>>>,
    pub net: Arc<dialtone::mesh::NetCounters>,
    /// Every node's latest sealed frames, served to neighbors catching up.
    pub frames: Arc<Mutex<FrameCache>>,
    /// Peer scores (GossipSub-style).
    pub scores: Arc<Mutex<PeerScores>>,
    pub cancel: CancellationToken,
}

impl Ctx {
    /// Run `f` on the peer scores.
    fn scores<R>(&self, f: impl FnOnce(&mut PeerScores) -> R) -> Option<R> {
        Some(f(&mut *self.scores.lock().ok()?))
    }

    /// Run `f` on the node state, if this node has one (the `node` action does not).
    fn with_state<R>(&self, f: impl FnOnce(&mut NodeState) -> R) -> Option<R> {
        let mut st = self.state.as_ref()?.lock().ok()?;
        Some(f(&mut st))
    }
}

/// Sent to Mesh when this node's inventory (processes, services) changed.
pub(crate) struct InventoryChanged;

/// The programs a peer says it runs, sent by Mesh to Deploy for every admitted inventory.
pub(crate) struct PeerPrograms {
    node: String,
    node_id: String,
    processes: Vec<ProcessSpec>,
}

/// Everything Mesh needs at start; the rest it builds itself.
pub(crate) struct MeshStart {
    pub node: String,
    pub node_key: iroh::SecretKey,
    pub gossip: Gossip,
    pub sender: GossipSender,
    pub receiver: GossipReceiver,
    pub keys: Option<ChannelKeys>,
    pub channel: String,
    pub topic_id: TopicId,
    pub seed_ids: Vec<EndpointId>,
    pub known_peer_addrs: HashMap<EndpointId, EndpointAddr>,
    pub print_first_beat: bool,
    pub control_rx: mpsc::UnboundedReceiver<MeshControlCommand>,
}

/// Start every actor. The returned handles finish after `ctx.cancel` is cancelled.
pub(crate) fn spawn_all(ctx: Ctx, start: MeshStart) -> Vec<JoinHandle<()>> {
    let (rtt_tx, rtt_rx) = watch::channel(BTreeMap::new());
    let (summary_tx, summary_rx) = watch::channel(None);
    let (inventory_tx, inventory_rx) = mpsc::unbounded_channel();
    let (deploy_tx, deploy_rx) = mpsc::unbounded_channel();
    let (probe_tx, probe_rx) = mpsc::unbounded_channel();
    let (rejoin_tx, rejoin_rx) = mpsc::unbounded_channel();
    ctx.with_state(|st| {
        st.probe_tx = Some(probe_tx);
        st.runtime = Some(tokio::runtime::Handle::current());
    });
    let mesh = Mesh::new(
        ctx.clone(),
        start,
        rtt_rx.clone(),
        summary_rx,
        inventory_rx,
        deploy_tx,
        rejoin_rx,
    );
    vec![
        tokio::spawn(mesh.run()),
        tokio::spawn(stats_actor(ctx.clone(), rtt_rx, summary_tx)),
        tokio::spawn(supervisor_actor(ctx.clone(), inventory_tx.clone())),
        tokio::spawn(devices_actor(ctx.clone())),
        tokio::spawn(links_actor(ctx.clone(), rtt_tx, rejoin_tx)),
        tokio::spawn(allowlist_actor(ctx.clone())),
        tokio::spawn(probe_actor(ctx.clone(), probe_rx)),
        tokio::spawn(deploy_actor(ctx, deploy_rx)),
    ]
}

fn every(period: Duration) -> tokio::time::Interval {
    let mut tick = tokio::time::interval(period);
    tick.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    tick
}

// ---------------------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------------------

/// Sample the host every second, keep the rolling summary and publish it for Mesh.
async fn stats_actor(
    ctx: Ctx,
    rtt_rx: watch::Receiver<BTreeMap<String, f64>>,
    summary_tx: watch::Sender<Option<StatsSummary>>,
) {
    let mut window = StatsWindow::new(SUMMARY_INTERVAL_SECS);
    let mut tick = every(Duration::from_secs(1));
    let mut counter = 0u64;
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        counter += 1;
        let (node, bound_ip) = ctx
            .with_state(|st| {
                (
                    st.node.clone(),
                    st.iroh_runtime.bound_non_tailnet_ipv4.clone(),
                )
            })
            .unwrap_or_default();
        let stats = sample_stats(&node, bound_ip.as_deref(), counter);
        let restarts = ctx
            .with_state(|st| st.processes.iter().map(|p| p.restart_count).sum())
            .unwrap_or(0);
        let rtt = rtt_rx.borrow().clone();
        let now = now_unix();
        window.record_sample(now, &stats, &rtt, restarts);
        let summary = window.summarize(now);
        summary_tx.send_replace(Some(summary.clone()));

        ctx.with_state(|st| {
            st.stats = stats.clone();
            if let Some(me) = st.self_entry_mut() {
                me.stats = stats;
                me.peer_rtt_ms = rtt;
                me.summary_stats = Some(summary);
                if let Some(geo) = cached_local_public_ip_geo() {
                    me.geo_location = Some(geo);
                }
            }
            let matrix = build_mesh_rtt_matrix(&st.nodes);
            st.mesh_rtt_ms = matrix;
            st.refresh_live_alerts();
        });
    }
}

// ---------------------------------------------------------------------------------------
// Supervisor
// ---------------------------------------------------------------------------------------

/// Reconcile processes every second; report spawns, stops and restarts to Mesh so peers
/// see them within about a second.
async fn supervisor_actor(ctx: Ctx, inventory_tx: mpsc::UnboundedSender<InventoryChanged>) {
    let mut tick = every(Duration::from_secs(1));
    let mut last: Option<Vec<ProcessSpec>> = None;
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        let changed = ctx.with_state(|st| {
            st.reconcile_processes();
            if last.as_ref() == Some(&st.processes) {
                return None;
            }
            last = Some(st.processes.clone());
            let (processes, services) = (st.processes.clone(), st.service_endpoints.clone());
            if let Some(me) = st.self_entry_mut() {
                me.processes = processes;
                me.service_endpoints = services;
            }
            Some(())
        });
        if let Some(Some(())) = changed {
            let _ = inventory_tx.send(InventoryChanged);
        }
    }
}

// ---------------------------------------------------------------------------------------
// Devices
// ---------------------------------------------------------------------------------------

/// How often the devices connected to the node are listed again.
const DEVICES_INTERVAL: Duration = Duration::from_secs(60);

/// List this node's devices now and every minute. They stay on the node: peers fetch them
/// with its details when their HUD focuses it (`GET /api/details`).
async fn devices_actor(ctx: Ctx) {
    let mut tick = every(DEVICES_INTERVAL);
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        let listed = tokio::task::spawn_blocking(dialtone::devices::list_devices).await;
        let devices = match listed {
            Ok(Ok(devices)) => devices,
            Ok(Err(err)) => {
                eprintln!("[devices] could not list devices: {err}");
                continue;
            }
            Err(_) => continue,
        };
        ctx.with_state(|st| {
            st.devices = devices.clone();
            if let Some(me) = st.self_entry_mut() {
                me.devices = devices;
            }
        });
    }
}

// ---------------------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------------------

/// How often Links times the Rtt to each gossip neighbor.
const LINKS_INTERVAL: Duration = Duration::from_secs(10);
/// How long Links waits for a neighbor: a relayed link with a 200ms Rtt still stalls for
/// seconds now and then (3s timeouts failed about one request in five across machines).
const LINKS_TIMEOUT: Duration = Duration::from_secs(10);

/// A neighbor that answers this many Rtt requests in a row without counting this node as
/// its neighbor is joined again (see `links_actor`).
const ONE_SIDED_ROUNDS: u32 = 2;

/// Every `LINKS_INTERVAL`, time the Rtt to each gossip neighbor over a fetch connection kept open
/// for it, and record how it is reached (direct IP, relay, tailnet). Only neighbors: the
/// rest of the mesh is reached through them, and each node reports its own neighbors'
/// Rtts in its beats, so the HUD's Rtt matrix still covers the mesh.
///
/// The answer also says whether the neighbor counts this node as its neighbor. iroh-gossip's
/// HyParView can lose the one `Neighbor` reply that completes a link and never send it
/// again, leaving this node with a neighbor that does not know it; a Join sent to that
/// neighbor (`rejoin_tx`, to Mesh) completes the link on both sides.
async fn links_actor(
    ctx: Ctx,
    rtt_tx: watch::Sender<BTreeMap<String, f64>>,
    rejoin_tx: mpsc::UnboundedSender<EndpointId>,
) {
    let mut tick = every(LINKS_INTERVAL);
    let mut conns: HashMap<EndpointId, iroh::endpoint::Connection> = HashMap::new();
    let mut rtt: BTreeMap<String, f64> = BTreeMap::new();
    let mut one_sided: HashMap<EndpointId, u32> = HashMap::new();
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        let neighbors: Vec<(EndpointId, String)> = ctx
            .with_state(|st| {
                st.gossip_neighbors
                    .iter()
                    .filter_map(|id| {
                        let name = st.nodes.iter().find(|n| &n.node_id == id)?.node.clone();
                        Some((id.parse().ok()?, name))
                    })
                    .collect()
            })
            .unwrap_or_default();
        conns.retain(|id, conn| {
            let keep = neighbors.iter().any(|(n, _)| n == id);
            if !keep {
                conn.close(0u32.into(), b"no longer a neighbor");
            }
            keep
        });
        rtt.retain(|name, _| neighbors.iter().any(|(_, n)| n == name));
        one_sided.retain(|id, _| neighbors.iter().any(|(n, _)| n == id));
        for (pid, name) in &neighbors {
            if !conns.contains_key(pid) {
                match fetch::connect(&ctx.endpoint, *pid, LINKS_TIMEOUT).await {
                    Ok(conn) => {
                        conns.insert(*pid, conn);
                    }
                    Err(err) => {
                        ctx.scores(|s| s.fetched(*pid, Some(&err)));
                        continue;
                    }
                }
            }
            match fetch::rtt(&conns[pid], LINKS_TIMEOUT).await {
                Ok((ms, counts_us)) => {
                    let rounds = one_sided.entry(*pid).or_default();
                    *rounds = if counts_us { 0 } else { *rounds + 1 };
                    if *rounds >= ONE_SIDED_ROUNDS {
                        *rounds = 0;
                        println!("[links] neighbor '{name}' does not count this node as its neighbor; joining it again");
                        let _ = rejoin_tx.send(*pid);
                    }
                    let smoothed = match rtt.get(name) {
                        Some(&prev) => ((prev * 0.6 + ms * 0.4) * 10.0).round() / 10.0,
                        None => ms,
                    };
                    rtt.insert(name.clone(), smoothed);
                }
                Err(err) => {
                    ctx.scores(|s| s.fetched(*pid, Some(&err)));
                    conns.remove(pid);
                }
            }
            let observed = tokio::time::timeout(
                Duration::from_millis(350),
                observe_peer_remote_addrs(&ctx.endpoint, &pid.to_string()),
            )
            .await;
            if let Ok((addrs, tailnet)) = observed {
                if !addrs.is_empty() {
                    let id = pid.to_string();
                    ctx.with_state(|st| {
                        if let Some(peer) = st.nodes.iter_mut().find(|n| n.node_id == id) {
                            peer.observed_remote_addrs = addrs;
                            peer.tailnet_used = tailnet;
                        }
                    });
                }
            }
        }
        rtt_tx.send_replace(rtt.clone());
    }
    for conn in conns.values() {
        conn.close(0u32.into(), b"shutting down");
    }
}

// ---------------------------------------------------------------------------------------
// Allowlist
// ---------------------------------------------------------------------------------------

/// Reload the allowlist when `config.json` changes. A bad file is reported and the last
/// good allowlist stays in force.
async fn allowlist_actor(ctx: Ctx) {
    let mtime = || {
        std::fs::metadata(&ctx.config_path)
            .and_then(|m| m.modified())
            .ok()
    };
    let mut seen = mtime();
    let mut tick = every(Duration::from_secs(BEAT_INTERVAL_SECS));
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        let now = mtime();
        if now == seen {
            continue;
        }
        seen = now;
        match try_load_mesh_config(&ctx.config_path) {
            Ok(disk) => {
                ctx.allowlist.replace(&disk.allowlist);
                ctx.with_state(|st| st.mesh_config.allowlist = disk.allowlist);
                println!(
                    "[allowlist] reloaded {} NodeId(s) from {}",
                    ctx.allowlist.len(),
                    ctx.config_path.display()
                );
            }
            Err(err) => eprintln!(
                "[allowlist] NOT reloaded, keeping the current {} NodeId(s): {err}",
                ctx.allowlist.len()
            ),
        }
    }
}

// ---------------------------------------------------------------------------------------
// Probe
// ---------------------------------------------------------------------------------------

/// Wait for the mesh to form, then check it every `PROBE_INTERVAL_SECS`; run a probe
/// whenever one is requested (`POST /api/probe`), full or gossip-only.
async fn probe_actor(ctx: Ctx, mut requests: mpsc::UnboundedReceiver<ProbeRequest>) {
    let Some(state) = ctx.state.clone() else {
        return;
    };
    let period = Duration::from_secs(dialtone::probe::PROBE_INTERVAL_SECS);
    let mut tick = tokio::time::interval_at(
        tokio::time::Instant::now() + Duration::from_secs(10),
        period,
    );
    tick.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    loop {
        let (all, reply) = tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => (false, None),
            Some(req) = requests.recv() => (req.all, Some(req.reply)),
        };
        let tickets = if all {
            sample_probe_blobs(&ctx).await
        } else {
            HashMap::new()
        };
        let state = state.clone();
        let result = tokio::task::spawn_blocking(move || {
            dialtone::probe::run_probe_now(&state, all, tickets)
        })
        .await
        .unwrap_or_else(|_| Err("probe run panicked".to_string()));
        match &result {
            Ok(report) => println!(
                "[probe] #{} {}/{} ok{}",
                report.cycle,
                report.passed_count,
                report.total_count,
                report
                    .checks
                    .iter()
                    .filter(|c| !c.ok)
                    .map(|c| format!("; {} failed: {}", c.code, c.detail))
                    .collect::<String>()
            ),
            Err(err) => eprintln!("[probe] could not run: {err}"),
        }
        if let Some(reply) = reply {
            let _ = reply.send(result);
        }
    }
}

/// For a full probe: ask up to `PULL_SAMPLE` live peers, chosen at random, to write a
/// fresh blob, and collect the tickets (or why there is none) by NodeId.
async fn sample_probe_blobs(ctx: &Ctx) -> HashMap<String, Result<String, String>> {
    let now = now_unix();
    let mut peers: Vec<String> = ctx
        .with_state(|st| {
            st.nodes
                .iter()
                .filter(|n| {
                    n.node_id != ctx.node_id
                        && now.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n)
                })
                .map(|n| n.node_id.clone())
                .collect()
        })
        .unwrap_or_default();
    ctx.scores(|s| {
        peers.retain(|id| {
            id.parse()
                .map(|pid| !s.avoid_fetching(&pid))
                .unwrap_or(false)
        })
    });
    shuffle(&mut peers);
    peers.truncate(PULL_SAMPLE);
    let asks: Vec<_> = peers
        .into_iter()
        .map(|id| {
            let ctx = ctx.clone();
            tokio::spawn(async move {
                let result = match id.parse::<EndpointId>() {
                    Ok(peer) => {
                        let result =
                            fetch::probe_blob(&ctx.endpoint, peer, Duration::from_secs(15)).await;
                        ctx.scores(|s| s.fetched(peer, result.as_ref().err().map(String::as_str)));
                        result
                    }
                    Err(_) => Err("invalid NodeId".to_string()),
                };
                (id, result)
            })
        })
        .collect();
    let mut tickets = HashMap::new();
    for ask in asks {
        if let Ok((id, result)) = ask.await {
            tickets.insert(id, result);
        }
    }
    tickets
}

// ---------------------------------------------------------------------------------------
// Deploy
// ---------------------------------------------------------------------------------------

/// Run the programs allowlisted peers offer: pull each from its ticket (only from an
/// allowlisted provider), verify its SHA-256, stage it and start it. A copy follows its
/// origin: when the origin's inventory stops offering a program, the copy is removed.
async fn deploy_actor(ctx: Ctx, mut offers: mpsc::UnboundedReceiver<PeerPrograms>) {
    let mut attempted: HashSet<(String, String)> = HashSet::new();
    loop {
        let offer = tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            offer = offers.recv() => match offer {
                Some(offer) => offer,
                None => break,
            },
        };
        let offered: HashSet<(String, String)> = offer
            .processes
            .iter()
            .filter(|p| p.desired_state == "running" && p.deploy_ticket.is_some())
            .map(|p| (p.name.clone(), p.digest.clone()))
            .collect();
        let origin = offer.node.clone();
        ctx.with_state(|st| {
            let stale: Vec<String> = st
                .processes
                .iter()
                .filter(|p| p.deployed_from.as_deref() == Some(offer.node_id.as_str()))
                .filter(|p| !offered.contains(&(p.name.clone(), p.digest.clone())))
                .map(|p| p.name.clone())
                .collect();
            for name in stale {
                if st.remove_process(&name) {
                    st.record_log(
                        "blob-supervisor",
                        "INFO",
                        "stdout",
                        format!("removed '{name}': peer '{origin}' no longer offers it"),
                    );
                }
            }
        });
        attempted.retain(|(node_id, digest)| {
            *node_id != offer.node_id || offered.iter().any(|(_, d)| d == digest)
        });
        for program in offer.processes {
            if program.desired_state != "running"
                || dialtone::supervisor::is_probe_program(&program.name)
            {
                continue;
            }
            let Some(ticket) = program.deploy_ticket.clone() else {
                continue;
            };
            let provider_listed = dialtone::blob::parse_blob_ticket_uri(&ticket)
                .map(|(t, _)| ctx.allowlist.contains(&t.addr().id))
                .unwrap_or(false);
            if !provider_listed {
                continue;
            }
            let Some((running, store)) = ctx.with_state(|st| {
                let running = st.processes.iter().any(|p| {
                    p.name == program.name
                        && p.digest == program.digest
                        && p.status.as_deref() == Some("running")
                });
                (running, st.store.clone())
            }) else {
                continue;
            };
            let Some(store) = store else { continue };
            if running || !attempted.insert((offer.node_id.clone(), program.digest.clone())) {
                continue;
            }
            let state = ctx.state.clone();
            let provider = offer.node.clone();
            let origin_id = offer.node_id.clone();
            tokio::task::spawn_blocking(move || {
                let Ok(report) = store.pull_blob(&ticket, false) else {
                    return;
                };
                let Ok((digest, staged)) = dialtone::supervisor::stage_program(
                    &store.blobs,
                    &program.name,
                    &report.digest,
                ) else {
                    return;
                };
                let Some(Ok(mut st)) = state.as_ref().map(|s| s.lock()) else {
                    return;
                };
                if !st
                    .blob_fetches
                    .iter()
                    .any(|f| f.ticket_uri == report.ticket_uri)
                {
                    st.blob_fetches.push(report);
                }
                let up_to_date = st.processes.iter().any(|p| {
                    p.name == program.name
                        && p.digest == digest
                        && p.status.as_deref() == Some("running")
                });
                if up_to_date {
                    return;
                }
                let mut spec = program.clone();
                spec.digest = digest;
                spec.path = staged.to_string_lossy().to_string();
                spec.pid = None;
                spec.status = None;
                spec.deploy_ticket = None;
                spec.deployed_from = Some(origin_id);
                match st.processes.iter_mut().find(|p| p.name == spec.name) {
                    Some(existing) => *existing = spec,
                    None => st.processes.push(spec),
                }
                st.reconcile_processes();
                st.record_log(
                    "blob-supervisor",
                    "INFO",
                    "stdout",
                    format!(
                        "auto-deployed & executed process blob '{}' from peer '{}'",
                        program.name, provider
                    ),
                );
            });
        }
    }
}

// ---------------------------------------------------------------------------------------
// Mesh
// ---------------------------------------------------------------------------------------

/// GossipSub's D_lo: below this many neighbors on a topic, Mesh joins a few more.
const MIN_NEIGHBORS: usize = 2;
/// How many known peers, chosen at random, Mesh joins when it is short of neighbors.
const JOIN_SAMPLE: usize = 3;
/// Inventory fetches in flight at once (a node that just joined fetches every inventory).
const FETCH_SLOTS: usize = 4;
/// Ask a peer for its inventory again no sooner than this.
const FETCH_RETRY: Duration = Duration::from_secs(10);
/// Every this many beats, Mesh catches up from one neighbor (anti-entropy).
const CATCH_UP_EVERY: u64 = 12;

/// Frames fetched from peers, to be admitted like gossip: a peer's own inventory, or what
/// a neighbor sent this node to catch up (beats with their age in seconds there).
struct Fetched {
    catch_up: bool,
    beats: Vec<(u64, Vec<u8>)>,
    inventories: Vec<Vec<u8>>,
}

/// Why `admit` dropped a frame.
enum Rejected {
    /// It did not open with the channel key, verify, pass the allowlist or name check.
    Invalid,
    /// This node's own, a replay, or older than one already admitted.
    Ignored,
}

struct Mesh {
    ctx: Ctx,
    node: String,
    node_key: iroh::SecretKey,
    gossip: Gossip,
    sender: GossipSender,
    receiver: GossipReceiver,
    /// The events topic (`events_topic_id`): inventories, when they change.
    events_sender: Option<GossipSender>,
    events_receiver: Option<GossipReceiver>,
    subscribed: bool,
    resubscribe_backoff: Duration,
    keys: Option<ChannelKeys>,
    channel: String,
    topic_id: TopicId,
    /// The configured seeds: joined at start, and when the node has no neighbors left.
    seed_ids: Vec<EndpointId>,
    /// Every peer address learned from beats; neighbors are sampled from these.
    known_peer_addrs: HashMap<EndpointId, EndpointAddr>,
    counter: u64,
    beat_interval_secs: u64,
    print_first_beat: bool,
    replay_guard: ReplayGuard,
    /// Latest inventory per peer NodeId, applied again when its entry is first created.
    inventories: HashMap<String, Inventory>,
    /// Peer NodeId -> when Mesh last asked it for its inventory (monotonic ms).
    inventory_asked: HashMap<String, u64>,
    fetch_slots: Arc<tokio::sync::Semaphore>,
    fetched_tx: mpsc::UnboundedSender<Fetched>,
    fetched_rx: mpsc::UnboundedReceiver<Fetched>,
    own_inventory_version: u64,
    /// A catch-up is in flight.
    catching_up: bool,
    /// The first catch-up finished (or there was nobody to catch up from); until then,
    /// beats announcing unknown inventories do not each start a fetch.
    caught_up: bool,
    control_rx: mpsc::UnboundedReceiver<MeshControlCommand>,
    rtt_rx: watch::Receiver<BTreeMap<String, f64>>,
    summary_rx: watch::Receiver<Option<StatsSummary>>,
    inventory_rx: mpsc::UnboundedReceiver<InventoryChanged>,
    deploy_tx: mpsc::UnboundedSender<PeerPrograms>,
    /// Neighbors that do not count this node as theirs (from Links), to join again.
    rejoin_rx: mpsc::UnboundedReceiver<EndpointId>,
}

/// The next event on a topic, or never when not subscribed.
async fn next_event(
    rx: &mut Option<GossipReceiver>,
) -> Option<Result<GossipEvent, iroh_gossip::api::ApiError>> {
    match rx {
        Some(rx) => rx.next().await,
        None => std::future::pending().await,
    }
}

/// Shuffle in place (Fisher-Yates), seeded by std's random hasher keys.
fn shuffle<T>(items: &mut [T]) {
    use std::hash::{BuildHasher, Hasher};
    for i in (1..items.len()).rev() {
        let mut h = std::collections::hash_map::RandomState::new().build_hasher();
        h.write_usize(i);
        let j = (h.finish() % (i as u64 + 1)) as usize;
        items.swap(i, j);
    }
}

impl Mesh {
    fn new(
        ctx: Ctx,
        start: MeshStart,
        rtt_rx: watch::Receiver<BTreeMap<String, f64>>,
        summary_rx: watch::Receiver<Option<StatsSummary>>,
        inventory_rx: mpsc::UnboundedReceiver<InventoryChanged>,
        deploy_tx: mpsc::UnboundedSender<PeerPrograms>,
        rejoin_rx: mpsc::UnboundedReceiver<EndpointId>,
    ) -> Self {
        let (fetched_tx, fetched_rx) = mpsc::unbounded_channel();
        Mesh {
            ctx,
            node: start.node,
            node_key: start.node_key,
            gossip: start.gossip,
            sender: start.sender,
            receiver: start.receiver,
            events_sender: None,
            events_receiver: None,
            subscribed: true,
            resubscribe_backoff: Duration::from_secs(1),
            keys: start.keys,
            channel: start.channel,
            topic_id: start.topic_id,
            seed_ids: start.seed_ids,
            known_peer_addrs: start.known_peer_addrs,
            counter: 1,
            beat_interval_secs: BEAT_INTERVAL_SECS,
            print_first_beat: start.print_first_beat,
            replay_guard: ReplayGuard::default(),
            inventories: HashMap::new(),
            inventory_asked: HashMap::new(),
            fetch_slots: Arc::new(tokio::sync::Semaphore::new(FETCH_SLOTS)),
            fetched_tx,
            fetched_rx,
            own_inventory_version: 0,
            catching_up: false,
            caught_up: false,
            control_rx: start.control_rx,
            rtt_rx,
            summary_rx,
            inventory_rx,
            deploy_tx,
            rejoin_rx,
        }
    }

    async fn run(mut self) {
        self.subscribe_events().await;
        self.refresh_own_inventory(false).await;
        let mut beat_tick = every(Duration::from_secs(self.beat_interval_secs));
        // When the gossip subscription ends, stop polling it and re-subscribe with backoff.
        let resubscribe_at = tokio::time::sleep(Duration::ZERO);
        tokio::pin!(resubscribe_at);
        loop {
            tokio::select! {
                _ = self.ctx.cancel.cancelled() => break,
                _ = &mut resubscribe_at, if !self.subscribed => {
                    if let Err(retry_in) = self.resubscribe().await {
                        resubscribe_at.as_mut().reset(tokio::time::Instant::now() + retry_in);
                    }
                }
                Some(cmd) = self.control_rx.recv() => self.on_control(cmd).await,
                Some(InventoryChanged) = self.inventory_rx.recv() => self.refresh_own_inventory(true).await,
                Some(fetched) = self.fetched_rx.recv() => self.on_fetched(fetched),
                Some(peer) = self.rejoin_rx.recv() => {
                    let _ = tokio::time::timeout(
                        Duration::from_millis(600),
                        self.sender.join_peers(vec![peer]),
                    )
                    .await;
                }
                _ = beat_tick.tick() => {
                    self.on_beat_tick().await;
                    // Larger meshes beat less often (`beat_interval_for`).
                    let nodes = self.ctx.with_state(|st| st.nodes.len()).unwrap_or(1);
                    let interval = beat_interval_for(nodes);
                    if interval != self.beat_interval_secs {
                        self.beat_interval_secs = interval;
                        beat_tick = tokio::time::interval_at(
                            tokio::time::Instant::now() + Duration::from_secs(interval),
                            Duration::from_secs(interval),
                        );
                        beat_tick.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
                    }
                }
                event = self.receiver.next(), if self.subscribed => {
                    let ended = match event {
                        Some(Ok(GossipEvent::NeighborUp(peer))) => {
                            self.neighbors_changed();
                            if !self.caught_up {
                                self.catch_up(peer);
                            }
                            None
                        }
                        Some(Ok(GossipEvent::NeighborDown(_))) => {
                            self.neighbors_changed();
                            self.keep_neighbors().await;
                            None
                        }
                        Some(Ok(GossipEvent::Received(msg))) => {
                            self.ctx.net.count_in(msg.content.len());
                            self.on_received(&msg.content, msg.delivered_from);
                            None
                        }
                        Some(Ok(GossipEvent::Lagged)) => {
                            eprintln!("[gossip-lagged] channel '{}': this node fell behind and missed gossip events", self.channel);
                            None
                        }
                        Some(Err(err)) => Some(err.to_string()),
                        None => Some(format!("subscription to '{}' ended", self.channel)),
                    };
                    if let Some(reason) = ended {
                        eprintln!("[gossip-err] {reason}; re-subscribing in {:?}", self.resubscribe_backoff);
                        self.subscribed = false;
                        self.events_sender = None;
                        self.events_receiver = None;
                        resubscribe_at.as_mut().reset(tokio::time::Instant::now() + self.resubscribe_backoff);
                    }
                }
                event = next_event(&mut self.events_receiver) => match event {
                    Some(Ok(GossipEvent::Received(msg))) => {
                        self.ctx.net.count_in(msg.content.len());
                        self.on_received(&msg.content, msg.delivered_from);
                    }
                    Some(Ok(_)) => {}
                    // Joined again on the next beat tick.
                    Some(Err(_)) | None => {
                        self.events_sender = None;
                        self.events_receiver = None;
                    }
                },
            }
        }
    }

    async fn resubscribe(&mut self) -> Result<(), Duration> {
        let first_peers = self.join_candidates(None);
        match self.gossip.subscribe(self.topic_id, first_peers).await {
            Ok(topic) => {
                (self.sender, self.receiver) = topic.split();
                self.subscribed = true;
                self.resubscribe_backoff = Duration::from_secs(1);
                self.subscribe_events().await;
                println!("[gossip] re-subscribed to channel '{}'", self.channel);
                Ok(())
            }
            Err(err) => {
                self.resubscribe_backoff =
                    (self.resubscribe_backoff * 2).min(Duration::from_secs(30));
                eprintln!(
                    "[gossip-err] re-subscribe failed: {err}; retrying in {:?}",
                    self.resubscribe_backoff
                );
                Err(self.resubscribe_backoff)
            }
        }
    }

    /// Subscribe to the events topic, joining the seeds and a few known peers.
    async fn subscribe_events(&mut self) {
        if self.keys.is_none() {
            return;
        }
        let first_peers = self.join_candidates(None);
        match self
            .gossip
            .subscribe(events_topic_id(self.topic_id), first_peers)
            .await
        {
            Ok(topic) => {
                let (sender, receiver) = topic.split();
                self.events_sender = Some(sender);
                self.events_receiver = Some(receiver);
            }
            Err(err) => eprintln!("[gossip-err] could not subscribe to the events topic: {err}"),
        }
    }

    /// The seeds plus up to `JOIN_SAMPLE` other known peers, chosen at random, leaving out
    /// the current neighbors of `topic` and graylisted peers.
    fn join_candidates(&self, neighbors: Option<&GossipReceiver>) -> Vec<EndpointId> {
        let own = self.ctx.endpoint.id();
        let mut current: HashSet<EndpointId> = neighbors
            .map(|rx| rx.neighbors().collect())
            .unwrap_or_default();
        self.ctx.scores(|s| {
            current.extend(self.known_peer_addrs.keys().filter(|id| s.graylisted(id)));
            current.extend(self.seed_ids.iter().filter(|id| s.graylisted(id)));
        });
        let mut known: Vec<EndpointId> = self
            .known_peer_addrs
            .keys()
            .filter(|id| **id != own && !current.contains(*id) && !self.seed_ids.contains(id))
            .copied()
            .collect();
        shuffle(&mut known);
        known.truncate(JOIN_SAMPLE);
        let mut out: Vec<EndpointId> = self
            .seed_ids
            .iter()
            .filter(|id| **id != own && !current.contains(*id))
            .copied()
            .collect();
        shuffle(&mut out);
        out.truncate(JOIN_SAMPLE);
        out.extend(known);
        out
    }

    /// Neighbors on a topic that are not graylisted.
    fn good_neighbors(&self, rx: &GossipReceiver) -> usize {
        self.ctx
            .scores(|s| rx.neighbors().filter(|id| !s.graylisted(id)).count())
            .unwrap_or(0)
    }

    /// Below `MIN_NEIGHBORS` good neighbors on a topic, join a few peers. HyParView keeps
    /// the active view full from there; no node dials every other node.
    async fn keep_neighbors(&mut self) {
        if self.keys.is_none() {
            return;
        }
        if self.subscribed && self.good_neighbors(&self.receiver) < MIN_NEIGHBORS {
            let peers = self.join_candidates(Some(&self.receiver));
            if !peers.is_empty() {
                let _ =
                    tokio::time::timeout(Duration::from_millis(600), self.sender.join_peers(peers))
                        .await;
            }
        }
        if self.events_receiver.is_none() {
            self.subscribe_events().await;
        }
        if let (Some(rx), Some(tx)) = (&self.events_receiver, &self.events_sender) {
            if self.good_neighbors(rx) < MIN_NEIGHBORS {
                let peers = self.join_candidates(Some(rx));
                if !peers.is_empty() {
                    let _ = tokio::time::timeout(Duration::from_millis(600), tx.join_peers(peers))
                        .await;
                }
            }
        }
    }

    /// Publish the beats topic's neighbors for the probe, the Links actor and `/api/net`.
    fn neighbors_changed(&mut self) {
        let neighbors: Vec<String> = self.receiver.neighbors().map(|id| id.to_string()).collect();
        self.ctx
            .net
            .neighbors
            .store(neighbors.len() as u64, std::sync::atomic::Ordering::Relaxed);
        self.ctx.with_state(|st| st.gossip_neighbors = neighbors);
    }

    /// Remember a peer's address for fetches and joins.
    fn learn_peer(&mut self, addr: &EndpointAddr) {
        let clean = advertised_endpoint_addr(addr, &self.ctx.transport);
        if clean.id == self.ctx.endpoint.id()
            || self.known_peer_addrs.get(&clean.id) == Some(&clean)
        {
            return;
        }
        self.known_peer_addrs.insert(clean.id, clean.clone());
        self.ctx.memory_lookup.add_endpoint_info(clean);
    }

    async fn on_control(&mut self, cmd: MeshControlCommand) {
        match cmd {
            MeshControlCommand::JoinMesh {
                channel,
                keys,
                invites,
                node_name,
            } => {
                if let Some(name) = node_name {
                    self.node = name;
                }
                self.channel = normalize_channel_name(Some(&channel));
                self.topic_id = keys.topic_id;
                let fingerprint = keys.fingerprint();
                self.keys = Some(keys);

                let mut boot_ids = Vec::new();
                let mut addrs: Vec<EndpointAddr> = default_seeds();
                for raw in &invites {
                    if let Ok(list) = SeedList::from_str(raw) {
                        addrs.extend(list.peers);
                    }
                }
                for addr in addrs {
                    self.learn_peer(&addr);
                    if addr.id != self.ctx.endpoint.id() && !boot_ids.contains(&addr.id) {
                        boot_ids.push(addr.id);
                    }
                }

                if let Ok(topic) = self.gossip.subscribe(self.topic_id, boot_ids.clone()).await {
                    (self.sender, self.receiver) = topic.split();
                    self.subscribed = true;
                    self.seed_ids = boot_ids.clone();
                    self.subscribe_events().await;
                    println!(
                        "[GOSSIP-JOIN] Unlocked '{}' gossip channel ({}) with {} peers",
                        self.channel,
                        fingerprint,
                        boot_ids.len()
                    );
                }
            }
        }
    }

    /// A beat about this node, with its stats, summary and neighbor Rtts.
    fn fresh_beat(&self) -> Beat {
        let adv = advertised_endpoint_addr(&self.ctx.endpoint.addr(), &self.ctx.transport);
        let diag = build_diagnostics(
            &self.ctx.transport,
            &adv,
            Some(&self.ctx.key_path),
            &self.ctx.node_id,
        );
        // This node's own seed list, for `dialtone status`; beats leave it out on the wire.
        let seed_list = SeedList {
            topic: self.topic_id,
            peers: vec![adv],
        }
        .to_text()
        .unwrap_or_default();
        let mut beat = self_beat(
            &self.ctx.blobs,
            &self.node,
            &self.ctx.node_id,
            seed_list,
            self.counter,
            diag,
        );
        beat.beat_interval_secs = self.beat_interval_secs;
        beat.inventory_version = self.own_inventory_version;
        beat.peer_rtt_ms = self.rtt_rx.borrow().clone();
        beat.summary_stats = self.summary_rx.borrow().clone();
        beat
    }

    /// This node's inventory as the node state has it now, with its details.
    fn own_inventory(&self) -> Inventory {
        let (processes, service_endpoints) = self
            .ctx
            .with_state(|st| (st.processes.clone(), st.service_endpoints.clone()))
            .unwrap_or_default();
        let adv = advertised_endpoint_addr(&self.ctx.endpoint.addr(), &self.ctx.transport);
        Inventory {
            node: self.node.clone(),
            node_id: self.ctx.node_id.clone(),
            version: 0,
            details: Some(NodeDetails {
                role: node_role_for(&self.node),
                capabilities: node_capabilities_for(&self.node),
                geo_location: cached_local_public_ip_geo(),
                iroh_transport: build_diagnostics(
                    &self.ctx.transport,
                    &adv,
                    Some(&self.ctx.key_path),
                    &self.ctx.node_id,
                ),
            }),
            processes,
            service_endpoints,
            sent_unix_ms: 0,
        }
    }

    /// Rebuild this node's inventory; when it changed, seal it (the frame peers fetch and
    /// neighbors relay), broadcast it on the events topic (if `broadcast`) and announce the
    /// new version in the next beat.
    async fn refresh_own_inventory(&mut self, broadcast: bool) {
        let inventory = self.own_inventory().for_wire();
        if inventory.version == self.own_inventory_version {
            return;
        }
        let Some((wire, _)) = self.seal(&Message::Inventory(inventory.clone())) else {
            return;
        };
        self.own_inventory_version = inventory.version;
        if let Ok(mut frames) = self.ctx.frames.lock() {
            frames.put_inventory(&self.ctx.node_id, wire.clone(), inventory.version);
        }
        if broadcast {
            if let Some(tx) = &self.events_sender {
                self.ctx.net.count_out(wire.len());
                let _ = tokio::time::timeout(Duration::from_millis(600), tx.broadcast(wire.into()))
                    .await;
            }
        }
    }

    /// Seal a message; returns the frame and the `sent_unix_ms` stamped in it.
    fn seal(&self, msg: &Message) -> Option<(Vec<u8>, u64)> {
        let keys = self.keys.as_ref()?;
        let wire = seal_message(msg, keys, &self.node_key).ok()?;
        let sent_ms = open_message(&wire, keys).ok()?.sent_unix_ms();
        Some((wire, sent_ms))
    }

    /// Broadcast a beat on the beats topic and keep its frame for neighbors catching up.
    async fn broadcast(&mut self, msg: &Message) {
        if let Some((wire, sent_ms)) = self.seal(msg) {
            if let Ok(mut frames) = self.ctx.frames.lock() {
                frames.put_beat(&self.ctx.node_id, wire.clone(), sent_ms, now_unix());
            }
            self.ctx.net.count_out(wire.len());
            let _ = tokio::time::timeout(
                Duration::from_millis(600),
                self.sender.broadcast(wire.into()),
            )
            .await;
        }
    }

    /// Ask `neighbor` for every beat and inventory this node lacks (Scuttlebutt-style
    /// anti-entropy). The digest names, for each node this node knows, its latest beat (or
    /// that the beat is fresh) and its inventory version; the neighbor answers with the
    /// frames that are newer or different. Run on the first link to the mesh, then every
    /// `CATCH_UP_EVERY` beats, it repairs what gossip lost, and a node that joins gets the
    /// whole mesh from one neighbor instead of fetching every inventory from its author.
    fn catch_up(&mut self, neighbor: EndpointId) {
        if self.catching_up || self.keys.is_none() {
            return;
        }
        let now = now_unix();
        let own = self.ctx.node_id.clone();
        let mut have: CatchUpHave = self
            .ctx
            .with_state(|st| {
                st.nodes
                    .iter()
                    .filter(|n| n.node_id != own)
                    .map(|n| {
                        let fresh = now.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n);
                        let beat = if fresh { u64::MAX } else { n.sent_unix_ms };
                        (n.node_id.clone(), [beat, 0])
                    })
                    .collect()
            })
            .unwrap_or_default();
        for (id, inventory) in &self.inventories {
            have.entry(id.clone()).or_insert([0, 0])[1] = inventory.version;
        }
        self.catching_up = true;
        let (ctx, fetched) = (self.ctx.clone(), self.fetched_tx.clone());
        tokio::spawn(async move {
            let result =
                fetch::catch_up(&ctx.endpoint, neighbor, have, Duration::from_secs(20)).await;
            ctx.scores(|s| s.fetched(neighbor, result.as_ref().err().map(String::as_str)));
            let caught = result.unwrap_or_else(|err| {
                eprintln!("[catch-up] {err}");
                Default::default()
            });
            let _ = fetched.send(Fetched {
                catch_up: true,
                beats: caught.beats,
                inventories: caught.inventories,
            });
        });
    }

    /// Admit fetched frames like gossip: beats first (so inventories find their node), then
    /// inventories.
    fn on_fetched(&mut self, fetched: Fetched) {
        if fetched.catch_up {
            self.catching_up = false;
            self.caught_up = true;
            if !fetched.beats.is_empty() || !fetched.inventories.is_empty() {
                println!(
                    "[catch-up] {} beat(s) and {} inventory frame(s) from a neighbor",
                    fetched.beats.len(),
                    fetched.inventories.len()
                );
            }
        }
        let now = now_unix();
        for (age, frame) in fetched.beats {
            if let Ok(msg) = self.admit(&frame, true) {
                let received = now.saturating_sub(age);
                self.keep_frame(&msg, frame, received);
                if let Some(mut beat) = msg.into_beat() {
                    self.learn_addresses(&beat);
                    beat.updated_at_unix = received;
                    self.merge_peer_beat(beat);
                }
            }
        }
        for frame in fetched.inventories {
            if let Ok(msg) = self.admit(&frame, true) {
                self.keep_frame(&msg, frame, now);
                if let Message::Inventory(inventory) = msg {
                    self.on_inventory(inventory);
                }
            }
        }
    }

    /// Keep an admitted frame for neighbors catching up.
    fn keep_frame(&self, msg: &Message, frame: Vec<u8>, received_unix: u64) {
        let Ok(mut frames) = self.ctx.frames.lock() else {
            return;
        };
        match msg {
            Message::Inventory(i) => frames.put_inventory(&i.node_id, frame, i.version),
            _ => frames.put_beat(msg.node_id(), frame, msg.sent_unix_ms(), received_unix),
        }
    }

    async fn on_beat_tick(&mut self) {
        let now = now_unix();
        let ttl = DEFAULT_PEER_TTL_SECS.max(5 * self.beat_interval_secs);
        let pruned = self.ctx.with_state(|st| {
            if let Some(me) = st.self_entry_mut() {
                me.updated_at_unix = now;
            }
            let pruned = prune_stale_peers(&mut st.nodes, now, ttl);
            let live_ids: HashSet<String> = st.nodes.iter().map(|n| n.node_id.clone()).collect();
            (pruned, live_ids)
        });
        if let Some((pruned, live_ids)) = pruned {
            if pruned > 0 {
                self.inventories.retain(|id, _| live_ids.contains(id));
                self.inventory_asked.retain(|id, _| live_ids.contains(id));
                if let Ok(mut frames) = self.ctx.frames.lock() {
                    frames.retain(&live_ids);
                }
                println!("[mesh] pruned {pruned} peer(s) silent for over {ttl}s");
            }
        }
        self.ctx.scores(PeerScores::decay);
        self.keep_neighbors().await;
        // Catch up from a random good neighbor now and then; with no neighbor after a few
        // beats, stop waiting for the first catch-up.
        let mut neighbors: Vec<EndpointId> = self
            .ctx
            .scores(|s| {
                self.receiver
                    .neighbors()
                    .filter(|id| !s.avoid_fetching(id))
                    .collect()
            })
            .unwrap_or_default();
        shuffle(&mut neighbors);
        match neighbors.first() {
            Some(&n) if !self.caught_up || self.counter.is_multiple_of(CATCH_UP_EVERY) => {
                self.catch_up(n)
            }
            None if self.counter >= 3 => self.caught_up = true,
            _ => {}
        }

        let counter = self.counter;
        let is_summary = counter <= 2 || counter.is_multiple_of(SUMMARY_EVERY_N_BEATS);
        if is_summary {
            // Details drift slowly (the public-IP lookup, addresses): catch them here.
            self.refresh_own_inventory(true).await;
        }
        let mut beat = self.fresh_beat();
        let local_summary = beat.summary_stats.clone();
        beat.kind = if is_summary { "summary" } else { "regular" }.to_string();
        if !is_summary {
            beat.summary_stats = None;
        }
        let node_id = self.ctx.node_id.clone();
        self.ctx.with_state(|st| {
            st.reconcile_processes();
            beat.heard = st
                .nodes
                .iter()
                .filter(|n| {
                    n.node_id != node_id && now.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n)
                })
                .count() as u32;
            beat.processes = st.processes.clone();
            beat.service_endpoints = st.service_endpoints.clone();
            beat.devices = st.devices.clone();
            beat.probe = st.probe.clone();
            st.iroh_runtime = beat.iroh_transport.clone();
            st.stats = beat.stats.clone();
            let mut local_copy = beat.clone();
            local_copy.summary_stats = local_summary.clone();
            merge_beat(&mut st.nodes, local_copy);
            if let (true, Some(s)) = (is_summary, &local_summary) {
                st.record_log(
                    "gossip-beat",
                    "INFO",
                    "stdout",
                    format!(
                        "published 30s summary beat #{} (cpu {:.1}% ±{:.1}%, mem {:.1}% ±{:.1}%, thrashing={}, heard={})",
                        counter,
                        s.cpu_pct.mean,
                        s.cpu_pct.stddev,
                        s.mem_pct.mean,
                        s.mem_pct.stddev,
                        s.thrashing_detected,
                        beat.heard
                    ),
                );
            }
        });
        if counter == 1 && self.print_first_beat {
            println!(
                "{}",
                serde_json::to_string_pretty(&beat).unwrap_or_default()
            );
        }
        let msg = if is_summary {
            Message::Summary(beat)
        } else {
            Message::Beat(beat)
        };
        self.broadcast(&msg).await;
        self.counter += 1;
    }

    /// Admit a message only if it opens with the channel key, its signature matches a
    /// listed node, its name is valid and it is not a replay. A `fetched` frame skips the
    /// replay window (a relayed inventory can be hours old) but must still be newer than
    /// anything admitted from its author.
    fn admit(&mut self, content: &[u8], fetched: bool) -> Result<Message, Rejected> {
        let keys = self.keys.as_ref().ok_or(Rejected::Ignored)?;
        let msg = match open_message(content, keys) {
            Ok(msg) => msg,
            Err(reason) => {
                eprintln!(
                    "[gossip-auth-reject] dropped gossip payload on channel '{}': {}",
                    self.channel, reason
                );
                return Err(Rejected::Invalid);
            }
        };
        // open_message verified the signature, so msg.node_id() is the real sender.
        let node_id = msg.node_id().to_string();
        if node_id == self.ctx.node_id {
            return Err(Rejected::Ignored);
        }
        if !self.ctx.allowlist.contains_str(&node_id) {
            eprintln!(
                "[gossip-allowlist-reject] dropped message from unlisted node_id={node_id} on channel '{}' (run 'dialtone allowlist add {node_id}' to authorize)",
                self.channel
            );
            return Err(Rejected::Invalid);
        }
        if !is_valid_node_name(msg.node()) {
            eprintln!(
                "[gossip-auth-reject] dropped message from node_id={node_id} with an invalid node name"
            );
            return Err(Rejected::Invalid);
        }
        let now_ms = if fetched {
            msg.sent_unix_ms()
        } else {
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|d| d.as_millis() as u64)
                .unwrap_or(0)
        };
        if let Err(reason) = self
            .replay_guard
            .admit(&msg.replay_key(), msg.sent_unix_ms(), now_ms)
        {
            // A fetched frame this node already has is expected; a gossip replay is not.
            if !fetched {
                eprintln!(
                    "[gossip-replay-reject] dropped message from node={} node_id={node_id}: {reason}",
                    msg.node()
                );
            }
            return Err(Rejected::Ignored);
        }
        Ok(msg)
    }

    /// Learn the addresses a beat advertises, for fetches and joins.
    fn learn_addresses(&mut self, beat: &Beat) {
        let Ok(pid) = beat.node_id.parse::<EndpointId>() else {
            return;
        };
        let mut addr = EndpointAddr::new(pid);
        for r in &beat.iroh_transport.advertised_relay_urls {
            if let Ok(url) = r.parse::<iroh::RelayUrl>() {
                addr = addr.with_relay_url(url);
            }
        }
        for ip in &beat.iroh_transport.advertised_ip_addrs {
            if let Ok(sock) = ip.parse::<std::net::SocketAddr>() {
                addr = addr.with_ip_addr(sock);
            }
        }
        self.learn_peer(&addr);
    }

    /// Fetch a peer's inventory when its beat announces a version this node does not have
    /// (it missed the broadcast, or joined after it).
    fn want_inventory(&mut self, beat: &Beat) {
        if !self.caught_up
            || beat.inventory_version == 0
            || self.inventories.get(&beat.node_id).map(|i| i.version)
                == Some(beat.inventory_version)
        {
            return;
        }
        let now = monotonic_now_ms();
        if let Some(&at) = self.inventory_asked.get(&beat.node_id) {
            if now.saturating_sub(at) < FETCH_RETRY.as_millis() as u64 {
                return;
            }
        }
        let Ok(peer) = beat.node_id.parse::<EndpointId>() else {
            return;
        };
        if self
            .ctx
            .scores(|s| s.avoid_fetching(&peer))
            .unwrap_or(false)
        {
            return;
        }
        self.inventory_asked.insert(beat.node_id.clone(), now);
        let (ctx, slots, fetched) = (
            self.ctx.clone(),
            self.fetch_slots.clone(),
            self.fetched_tx.clone(),
        );
        tokio::spawn(async move {
            let Ok(_slot) = slots.acquire_owned().await else {
                return;
            };
            let result = fetch::inventory(&ctx.endpoint, peer, Duration::from_secs(10)).await;
            ctx.scores(|s| s.fetched(peer, result.as_ref().err().map(String::as_str)));
            match result {
                Ok(frame) => {
                    let _ = fetched.send(Fetched {
                        catch_up: false,
                        beats: Vec::new(),
                        inventories: vec![frame],
                    });
                }
                Err(err) => eprintln!("[fetch] inventory: {err}"),
            }
        });
    }

    /// A gossip message `delivered_from` a neighbor: dropped when that neighbor is
    /// graylisted (GossipSub's graylist), otherwise admitted, scored and kept.
    fn on_received(&mut self, content: &[u8], delivered_from: EndpointId) {
        if self
            .ctx
            .scores(|s| s.graylisted(&delivered_from))
            .unwrap_or(false)
        {
            return;
        }
        let admitted = self.admit(content, false);
        match &admitted {
            Ok(_) => self.ctx.scores(|s| s.delivered(delivered_from, true)),
            Err(Rejected::Invalid) => self.ctx.scores(|s| s.delivered(delivered_from, false)),
            Err(Rejected::Ignored) => None,
        };
        let Ok(msg) = admitted else {
            return;
        };
        self.keep_frame(&msg, content.to_vec(), now_unix());
        match msg {
            Message::Inventory(inventory) => self.on_inventory(inventory),
            beat => {
                if let Some(beat) = beat.into_beat() {
                    self.on_beat(beat);
                }
            }
        }
    }

    fn on_inventory(&mut self, inventory: Inventory) {
        if !self.ctx.allowlist.contains_str(&inventory.node_id) {
            return;
        }
        let _ = self.deploy_tx.send(PeerPrograms {
            node: inventory.node.clone(),
            node_id: inventory.node_id.clone(),
            processes: inventory.processes.clone(),
        });
        self.ctx
            .with_state(|st| merge_inventory(&mut st.nodes, &inventory));
        self.inventories
            .insert(inventory.node_id.clone(), inventory);
    }

    fn on_beat(&mut self, mut beat: Beat) {
        self.learn_addresses(&beat);
        self.want_inventory(&beat);
        // Stamp with our clock once admitted, so clock skew between hosts cannot cause
        // early TTL pruning.
        beat.updated_at_unix = now_unix();
        self.merge_peer_beat(beat);
    }

    /// Merge an admitted peer beat into the node state (with its inventory, the first time).
    fn merge_peer_beat(&mut self, beat: Beat) {
        let peer = beat.node.clone();
        let inventory = self.inventories.get(&beat.node_id).cloned();
        self.ctx.with_state(|st| {
            let first_seen = !st.nodes.iter().any(|n| n.node_id == beat.node_id);
            merge_beat(&mut st.nodes, beat);
            if let (true, Some(inv)) = (first_seen, &inventory) {
                merge_inventory(&mut st.nodes, inv);
            }
            if first_seen {
                println!("[mesh] peer '{peer}' joined");
                st.record_event(
                    "mesh.peer_joined",
                    "info",
                    format!("Discovered mesh peer '{peer}'"),
                );
                st.record_log(
                    "gossip-mesh",
                    "INFO",
                    "stdout",
                    format!("peer '{peer}' joined the gossip mesh"),
                );
            }
        });
    }
}
