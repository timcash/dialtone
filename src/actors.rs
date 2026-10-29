//! The running node as actors: one task per job, joined by channels and stopped by one
//! cancellation token.
//!
//! - **Mesh** owns the gossip subscriptions: beats on the beats topic, inventories on the
//!   events topic (see `Message`), and a few neighbors per topic. It sends nothing outside
//!   gossip: a node learns the mesh from beats, and fetches only what someone asks for.
//! - **Stats** samples the host every second and keeps the rolling summary.
//! - **Supervisor** reconciles processes every second and tells Mesh when they change.
//! - **Devices** lists the cameras, microphones, keyboards and other devices connected to
//!   the node every minute and tells Mesh when they change.
//! - **Paths** reads QUIC's Rtt estimate and selected path (direct or relay) from each
//!   gossip neighbor's open connection, without sending anything.
//! - **Allowlist** reloads the allowlist when `config.json` changes.
//! - **Deploy** pulls, verifies and starts the programs allowlisted peers announce to this
//!   node (`Deploy` messages), and removes copies their origin no longer announces.
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
    default_seeds, events_topic_id, is_valid_node_name, merge_beat, merge_inventory, mesh_offer,
    monotonic_now_ms, node_capabilities_for, node_role_for, normalize_channel_name, now_unix,
    observe_peer_remote_addrs, open_message, prune_stale_peers, refresh_seeds, save_mesh_config,
    seal_message, try_load_mesh_config, Beat, ChannelKeys, Deploy, Inventory, LiveAllowlist,
    MeshControlCommand, Message, NeighborLink, NodeDetails, PeerScores, ReplayGuard, Seed,
    SeedList, BEAT_INTERVAL_SECS, DEFAULT_PEER_TTL_SECS, SUMMARY_INTERVAL_SECS,
};
use dialtone::node::{
    advertised_endpoint_addr, build_diagnostics, GossipConnections, IrohTransportConfig,
};
use dialtone::probe::{ProbeRequest, PULL_SAMPLE};
use dialtone::service::ProcessSpec;
use dialtone::stats::{sample_stats, StatsSummary, StatsWindow};
use dialtone::update;
use dialtone::wire::{neighbor_prefix, MAX_NEIGHBORS};
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
    /// NodeIds this node has had a valid message from: on an open channel (empty allowlist)
    /// the handshake hook admits only them to fetch, blobs and link.
    pub members: LiveAllowlist,
    pub state: Option<Arc<Mutex<NodeState>>>,
    pub net: Arc<dialtone::mesh::NetCounters>,
    /// The open gossip connections, read by the Paths actor.
    pub gossip_conns: GossipConnections,
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
    pub control_rx: mpsc::UnboundedReceiver<MeshControlCommand>,
    /// Where this node updates itself, when it does (`dialtone::update`).
    pub update: Option<update::Paths>,
}

/// Start every actor. The returned handles finish after `ctx.cancel` is cancelled.
pub(crate) fn spawn_all(ctx: Ctx, mut start: MeshStart) -> Vec<JoinHandle<()>> {
    let (update_tx, update_rx) = mpsc::unbounded_channel();
    let update_paths = start.update.take();
    let (rtt_tx, rtt_rx) = watch::channel(BTreeMap::new());
    let (summary_tx, summary_rx) = watch::channel(None);
    let (inventory_tx, inventory_rx) = mpsc::unbounded_channel();
    let (deploy_tx, deploy_rx) = mpsc::unbounded_channel();
    let (probe_tx, probe_rx) = mpsc::unbounded_channel();
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
        update_tx,
    );
    let mut actors = vec![
        tokio::spawn(mesh.run()),
        tokio::spawn(stats_actor(ctx.clone(), rtt_rx, summary_tx)),
        tokio::spawn(supervisor_actor(ctx.clone(), inventory_tx.clone())),
        tokio::spawn(devices_actor(ctx.clone())),
        tokio::spawn(paths_actor(ctx.clone(), rtt_tx)),
        tokio::spawn(allowlist_actor(ctx.clone())),
        tokio::spawn(probe_actor(ctx.clone(), probe_rx)),
        tokio::spawn(deploy_actor(ctx.clone(), deploy_rx)),
    ];
    if let Some(paths) = update_paths {
        actors.push(tokio::spawn(update_actor(ctx, paths, update_rx)));
    }
    actors
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
            let by_name: BTreeMap<String, f64> = rtt
                .iter()
                .filter_map(|(id, ms)| {
                    let peer = st.nodes.iter().find(|n| &n.node_id == id)?;
                    Some((peer.node.clone(), *ms))
                })
                .collect();
            if let Some(me) = st.self_entry_mut() {
                me.stats = stats;
                me.peer_rtt_ms = by_name;
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
// Paths
// ---------------------------------------------------------------------------------------

/// How often Paths reads the gossip neighbors' connections.
const PATHS_INTERVAL: Duration = Duration::from_secs(5);

/// Every `PATHS_INTERVAL`, read QUIC's Rtt estimate and selected path to each gossip
/// neighbor from the connection iroh-gossip already has open (`GossipConnections`), and how
/// iroh reaches it (direct IP, relay, tailnet). Nothing is sent: QUIC measures the Rtt from
/// the acknowledgements of the gossip itself. The Rtts, by NodeId, go into this node's beats.
async fn paths_actor(ctx: Ctx, rtt_tx: watch::Sender<BTreeMap<String, f64>>) {
    let mut tick = every(PATHS_INTERVAL);
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        ctx.gossip_conns.prune();
        let neighbors: Vec<EndpointId> = ctx
            .with_state(|st| {
                st.gossip_neighbors
                    .iter()
                    .filter_map(|id| id.parse().ok())
                    .collect()
            })
            .unwrap_or_default();
        let mut rtt = BTreeMap::new();
        for pid in &neighbors {
            if let Some(path) = ctx.gossip_conns.path(pid) {
                if path.rtt_ms > 0.0 {
                    rtt.insert(pid.to_string(), path.rtt_ms);
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
        rtt_tx.send_replace(rtt);
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

/// An origin re-announces each program it deploys this often, for nodes that missed it.
const REANNOUNCE_EVERY: Duration = Duration::from_secs(600);
/// A copy whose origin is in the mesh but has not announced it for this long is removed.
const DEPLOY_EXPIRY_SECS: u64 = 3 * REANNOUNCE_EVERY.as_secs();
/// A program whose pull failed is tried again no sooner than this.
const DEPLOY_RETRY_SECS: u64 = 120;
/// Peers that already run a program, asked for its blob before its origin.
const DEPLOY_PEER_PROVIDERS: usize = 3;

/// Run what allowlisted peers announce to this node: pull each program (from a peer that
/// already runs it when there is one, else from its origin), verify its SHA-256, stage it and
/// start it, after a random delay of up to one beat interval so a large mesh does not pull
/// at once. A copy follows its origin: it is removed when the origin announces it removed or
/// no longer targets this node, or stops announcing it while still in the mesh.
async fn deploy_actor(ctx: Ctx, mut deploys: mpsc::UnboundedReceiver<Deploy>) {
    // (origin NodeId, program name) -> when the origin last announced it to this node.
    let mut heard: HashMap<(String, String), u64> = HashMap::new();
    // (origin NodeId, program name, digest) -> when this node last tried to pull it.
    let mut attempted: HashMap<(String, String, String), u64> = HashMap::new();
    let mut tick = every(Duration::from_secs(60));
    loop {
        let deploy = tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {
                expire_copies(&ctx, &mut heard);
                continue;
            }
            deploy = deploys.recv() => match deploy {
                Some(deploy) => deploy,
                None => break,
            },
        };
        let (origin, name) = (deploy.node_id.clone(), deploy.process.name.clone());
        if !deploy.targets(&ctx.node_id) {
            heard.remove(&(origin.clone(), name.clone()));
            remove_copy(&ctx, &origin, &name, &deploy.node);
            continue;
        }
        let now = now_unix();
        heard.insert((origin.clone(), name.clone()), now);
        let Some((running, store, providers, delay)) = ctx.with_state(|st| {
            let running = st.processes.iter().any(|p| {
                p.name == name
                    && p.digest == deploy.process.digest
                    && p.deployed_from.as_deref() == Some(origin.as_str())
            });
            (
                running,
                st.store.clone(),
                peer_providers(st, &ctx.node_id, &origin, &deploy.process.digest),
                st.nodes
                    .iter()
                    .find(|n| n.node_id == ctx.node_id)
                    .map(|me| me.beat_interval_secs)
                    .unwrap_or(BEAT_INTERVAL_SECS),
            )
        }) else {
            continue;
        };
        let Some(store) = store else { continue };
        let key = (origin.clone(), name.clone(), deploy.process.digest.clone());
        if running
            || attempted
                .get(&key)
                .is_some_and(|at| now.saturating_sub(*at) < DEPLOY_RETRY_SECS)
        {
            continue;
        }
        attempted.insert(key, now);
        let state = ctx.state.clone();
        let delay = Duration::from_millis(random_below(delay * 1000));
        tokio::spawn(async move {
            tokio::time::sleep(delay).await;
            let _ = tokio::task::spawn_blocking(move || {
                install_copy(state, store, deploy, providers);
            })
            .await;
        });
    }
}

/// Up to `DEPLOY_PEER_PROVIDERS` peers, other than the origin, whose inventory says they run
/// the program with `digest`: gossip neighbors first, the rest at random.
fn peer_providers(st: &NodeState, own: &str, origin: &str, digest: &str) -> Vec<EndpointId> {
    let mut holders: Vec<&Beat> = st
        .nodes
        .iter()
        .filter(|n| n.node_id != own && n.node_id != origin)
        .filter(|n| n.processes.iter().any(|p| p.digest == digest))
        .collect();
    shuffle(&mut holders);
    holders.sort_by_key(|n| !st.gossip_neighbors.contains(&n.node_id));
    holders
        .iter()
        .filter_map(|n| n.node_id.parse().ok())
        .take(DEPLOY_PEER_PROVIDERS)
        .collect()
}

/// Pull, verify, stage and start one announced program (blocking).
fn install_copy(
    state: Option<Arc<Mutex<NodeState>>>,
    store: dialtone::blob::Store,
    deploy: Deploy,
    providers: Vec<EndpointId>,
) {
    let program = deploy.process;
    let report = match store.pull_blob_from(&deploy.ticket, false, providers) {
        Ok(report) => report,
        Err(err) => {
            eprintln!(
                "[deploy] could not pull '{}' from '{}': {err}",
                program.name, deploy.node
            );
            return;
        }
    };
    let Ok((digest, staged)) =
        dialtone::supervisor::stage_program(&store.blobs, &program.name, &report.digest)
    else {
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
    let mut spec = program.clone();
    spec.digest = digest;
    spec.path = staged.to_string_lossy().to_string();
    spec.pid = None;
    spec.status = None;
    spec.deploy_ticket = None;
    spec.deployed_from = Some(deploy.node_id.clone());
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
            "deployed '{}' from peer '{}' and started it",
            program.name, deploy.node
        ),
    );
}

/// Remove the copy of `name` deployed from `origin`, if this node runs one.
fn remove_copy(ctx: &Ctx, origin: &str, name: &str, origin_name: &str) {
    ctx.with_state(|st| {
        let is_copy = st
            .processes
            .iter()
            .any(|p| p.name == name && p.deployed_from.as_deref() == Some(origin));
        if is_copy && st.remove_process(name) {
            st.record_log(
                "blob-supervisor",
                "INFO",
                "stdout",
                format!("removed '{name}': peer '{origin_name}' no longer deploys it here"),
            );
        }
    });
}

/// Remove copies whose origin is in the mesh but has not announced them for
/// `DEPLOY_EXPIRY_SECS`. A copy this node has not heard about yet (it was installed before
/// a restart) gets the full time from now.
fn expire_copies(ctx: &Ctx, heard: &mut HashMap<(String, String), u64>) {
    let now = now_unix();
    let stale: Vec<(String, String, String)> = ctx
        .with_state(|st| {
            st.processes
                .iter()
                .filter_map(|p| {
                    let origin = p.deployed_from.clone()?;
                    let at = *heard.entry((origin.clone(), p.name.clone())).or_insert(now);
                    let origin_beat = st.nodes.iter().find(|n| n.node_id == origin)?;
                    let origin_live = now.saturating_sub(origin_beat.updated_at_unix)
                        <= beat_fresh_secs(origin_beat);
                    (origin_live && now.saturating_sub(at) > DEPLOY_EXPIRY_SECS)
                        .then(|| (origin, p.name.clone(), origin_beat.node.clone()))
                })
                .collect()
        })
        .unwrap_or_default();
    for (origin, name, origin_name) in stale {
        heard.remove(&(origin.clone(), name.clone()));
        remove_copy(ctx, &origin, &name, &origin_name);
    }
}

/// A random number of milliseconds below `max` (0 when `max` is 0).
fn random_below(max: u64) -> u64 {
    use std::hash::{BuildHasher, Hasher};
    let mut h = std::collections::hash_map::RandomState::new().build_hasher();
    h.write_u64(max);
    h.finish() % max.max(1)
}

// ---------------------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------------------

/// A peer whose beat shows a newer release than this node runs.
pub(crate) struct UpdateHint {
    node_id: String,
    version: String,
}

/// How often a node asks GitHub for the latest release (`DIALTONE_UPDATE_CHECK_SECS`).
const UPDATE_CHECK_EVERY: Duration = Duration::from_secs(3600);
/// The first check comes this long after start (or sooner when checks are more frequent).
const UPDATE_FIRST_CHECK: Duration = Duration::from_secs(60);
/// A peer's beats showing a newer release are followed at most this often (a peer serves its
/// release only once it has kept it, which takes a minute after it restarts into it).
const UPDATE_HINT_GAP: Duration = Duration::from_secs(15);
/// How often the actor looks whether a release it just kept is in the store for peers yet.
const UPDATE_SHARE_EVERY: Duration = Duration::from_secs(10);
/// How long fetching the release binary from peers may take.
const UPDATE_PEER_FETCH: Duration = Duration::from_secs(600);

/// Self-update (`dialtone::update`): asks GitHub for the latest release now and then, and
/// follows peers whose beats show a newer one; installs it A/B and restarts into it.
async fn update_actor(
    ctx: Ctx,
    paths: update::Paths,
    mut hints: mpsc::UnboundedReceiver<UpdateHint>,
) {
    let client = match update::http_client() {
        Ok(c) => Some(c),
        Err(e) => {
            eprintln!("[update] {e}; this node updates from peers only");
            None
        }
    };
    let mut share_tick = every(UPDATE_SHARE_EVERY);
    let every = std::env::var("DIALTONE_UPDATE_CHECK_SECS")
        .ok()
        .and_then(|v| v.trim().parse().ok())
        .map(Duration::from_secs)
        .unwrap_or(UPDATE_CHECK_EVERY);
    // Spread out, so a mesh that restarted together does not ask at once.
    let spread = |d: Duration| {
        let ms = d.as_millis() as u64;
        Duration::from_millis(ms * 4 / 5 + random_below(ms * 2 / 5))
    };
    let mut next_check = tokio::time::Instant::now() + spread(every.min(UPDATE_FIRST_CHECK));
    let mut last_hint: Option<tokio::time::Instant> = None;
    loop {
        let (signed, from) = tokio::select! {
            _ = ctx.cancel.cancelled() => return,
            _ = share_tick.tick() => {
                share_own_binary(&ctx, &paths).await;
                continue;
            }
            _ = tokio::time::sleep_until(next_check) => {
                next_check = tokio::time::Instant::now() + spread(every);
                let Some(client) = &client else { continue };
                if paths.load().trial.is_some() {
                    continue;
                }
                match update::latest_manifest(client).await {
                    Ok(signed) => (signed, "GitHub".to_string()),
                    Err(e) => {
                        eprintln!("[update] release check: {e}");
                        continue;
                    }
                }
            }
            Some(hint) = hints.recv() => {
                if last_hint.is_some_and(|t| t.elapsed() < UPDATE_HINT_GAP)
                    || paths.load().trial.is_some()
                {
                    continue;
                }
                last_hint = Some(tokio::time::Instant::now());
                let Ok(peer) = hint.node_id.parse::<EndpointId>() else { continue };
                match fetch::release(&ctx.endpoint, peer, Duration::from_secs(10)).await {
                    Ok(signed) => (signed, format!("peer {}", peer.fmt_short())),
                    Err(e) => {
                        eprintln!("[update] {} runs v{}, but its release: {e}", peer.fmt_short(), hint.version);
                        continue;
                    }
                }
            }
        };
        if let Err(e) = consider_release(&ctx, &paths, client.as_ref(), &signed, &from).await {
            eprintln!("[update] {e}");
        }
    }
}

/// Install `signed`'s release if it is newer than this node runs and was never rolled back
/// here, then restart into it.
async fn consider_release(
    ctx: &Ctx,
    paths: &update::Paths,
    client: Option<&reqwest::Client>,
    signed: &update::SignedManifest,
    from: &str,
) -> Result<(), String> {
    let manifest = signed
        .verify()
        .map_err(|e| format!("release from {from}: {e}"))?;
    let running = update::running_version();
    if !update::newer(&manifest.version, running) || paths.load().bad.contains(&manifest.version) {
        return Ok(());
    }
    let name = update::own_asset().ok_or("no release binary for this OS and architecture")?;
    let asset = manifest
        .asset(name)
        .ok_or_else(|| format!("v{} has no {name}", manifest.version))?
        .clone();
    println!(
        "[update] v{} is out (this node runs v{running}, heard from {from}); fetching {name}",
        manifest.version
    );
    // Peers that run it on this OS and architecture hold the binary in their stores.
    let own = ctx.node_id.clone();
    let providers: Vec<EndpointId> = ctx
        .with_state(|st| {
            st.nodes
                .iter()
                .filter(|n| {
                    n.node_id != own
                        && n.version == manifest.version
                        && n.os == std::env::consts::OS
                        && n.arch == std::env::consts::ARCH
                })
                .filter_map(|n| n.node_id.parse().ok())
                .collect()
        })
        .unwrap_or_default();
    let bytes = match binary_from_peers(ctx, &asset, providers).await {
        Ok(bytes) => {
            println!("[update] got {name} from peers");
            bytes
        }
        Err(peer_err) => match client {
            Some(client) => update::download_asset(client, &manifest, &asset).await?,
            None => return Err(format!("{name} from peers: {peer_err}")),
        },
    };
    update::install(paths, signed, &manifest, &asset, &bytes)?;
    println!(
        "[update] installed v{} (v{running} kept as {}); restarting into it",
        manifest.version,
        paths.prev().display()
    );
    update::request_restart();
    Ok(())
}

/// The release binary from peers' stores, by its BLAKE3 hash, checked against its SHA-256.
async fn binary_from_peers(
    ctx: &Ctx,
    asset: &update::Asset,
    providers: Vec<EndpointId>,
) -> Result<Vec<u8>, String> {
    if providers.is_empty() {
        return Err("no peer runs it on this OS and architecture".into());
    }
    let hash: iroh_blobs::Hash = asset
        .blake3
        .parse()
        .map_err(|e| format!("bad BLAKE3 hash in the manifest: {e}"))?;
    let store = ctx
        .with_state(|st| st.store.clone())
        .flatten()
        .ok_or("no store")?;
    tokio::time::timeout(
        UPDATE_PEER_FETCH,
        store.downloader.download(hash, providers),
    )
    .await
    .map_err(|_| "timed out".to_string())?
    .map_err(|e| e.to_string())?;
    store
        .blobs
        .adopt(hash, &format!("sha256:{}", asset.sha256))
        .await
}

/// Keep the release binary this node runs in its store, so peers on the same OS and
/// architecture can update from it; the one it ran before is let go.
/// Does nothing once it is there.
async fn share_own_binary(ctx: &Ctx, paths: &update::Paths) {
    let Some(signed) = paths.running_manifest() else {
        return;
    };
    let Some(asset) = signed
        .verify()
        .ok()
        .and_then(|m| update::own_asset().and_then(|n| m.asset(n).cloned()))
    else {
        return;
    };
    let digest = format!("sha256:{}", asset.sha256);
    let mut state = paths.load();
    if state.shared.as_deref() == Some(digest.as_str()) {
        return;
    }
    let exe = paths.exe.clone();
    let Ok(Ok(bytes)) = tokio::task::spawn_blocking(move || std::fs::read(exe)).await else {
        return;
    };
    if update::sha256_hex(&bytes) != asset.sha256 {
        return;
    }
    if let Err(e) = ctx.blobs.put(&bytes, Some(&digest)).await {
        eprintln!("[update] could not share this binary with peers: {e}");
        return;
    }
    if let Some(old) = state.shared.replace(digest.clone()) {
        if old != digest {
            let _ = ctx.blobs.forget(&old).await;
        }
    }
    let _ = paths.save(&state);
    println!("[update] this node now serves its release binary to peers");
}

// ---------------------------------------------------------------------------------------
// Mesh
// ---------------------------------------------------------------------------------------

/// GossipSub's D_lo: below this many neighbors on a topic, Mesh joins a few more.
const MIN_NEIGHBORS: usize = 2;
/// How many known peers, chosen at random, Mesh joins when it is short of neighbors.
const JOIN_SAMPLE: usize = 3;
/// A peer Mesh dials itself for gossip (`dial_for_gossip`) is dialed at most this often.
const GOSSIP_DIAL_EVERY: Duration = Duration::from_secs(30);
/// A neighbor whose beats this many times in a row do not list this node is joined again.
const ONE_SIDED_BEATS: u32 = 2;
/// Every this many beats, Mesh rebuilds its inventory to catch details that drift slowly
/// (the public-IP lookup, addresses); it is broadcast only when it changed.
const INVENTORY_REFRESH_EVERY: u64 = 6;
/// Every this many beats, Mesh saves the peers it hears as its seeds (`refresh_seeds`).
const SEED_REFRESH_EVERY: u64 = 12;
/// When new nodes join, an origin re-announces its deploys at most this often.
const NEW_NODE_ANNOUNCE_GAP: Duration = Duration::from_secs(60);
/// How long an origin keeps announcing a program it removed (for nodes that missed it).
const REMOVED_ANNOUNCE_FOR: Duration = Duration::from_secs(3600);

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
    replay_guard: ReplayGuard,
    /// Latest inventory per peer NodeId, applied again when its entry is first created.
    inventories: HashMap<String, Inventory>,
    own_inventory_version: u64,
    /// Neighbors by how many beats in a row did not list this node (see `on_beat`).
    one_sided: HashMap<EndpointId, u32>,
    /// When each current neighbor became one, and when a silent one was last joined again
    /// (monotonic ms; see `rejoin_silent_neighbors`).
    neighbor_since: HashMap<EndpointId, u64>,
    silent_rejoined_at: HashMap<EndpointId, u64>,
    /// When `dial_for_gossip` last dialed each peer (monotonic ms).
    gossip_dialed_at: HashMap<EndpointId, u64>,
    /// What this node last announced for each program it deploys, by name, with when it was
    /// removed (monotonic ms) for programs it no longer deploys.
    announced: HashMap<String, (Deploy, Option<u64>)>,
    /// When every deploy was last announced again (monotonic ms).
    announced_all_at: u64,
    /// A node joined since then: announce again soon, so it hears what runs on the mesh.
    new_node_seen: bool,
    control_rx: mpsc::UnboundedReceiver<MeshControlCommand>,
    /// QUIC's Rtt to each gossip neighbor, by NodeId (from Paths).
    rtt_rx: watch::Receiver<BTreeMap<String, f64>>,
    summary_rx: watch::Receiver<Option<StatsSummary>>,
    inventory_rx: mpsc::UnboundedReceiver<InventoryChanged>,
    deploy_tx: mpsc::UnboundedSender<Deploy>,
    /// Peers whose beats show a newer release than this node runs (`update_actor`).
    update_tx: mpsc::UnboundedSender<UpdateHint>,
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
        deploy_tx: mpsc::UnboundedSender<Deploy>,
        update_tx: mpsc::UnboundedSender<UpdateHint>,
    ) -> Self {
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
            replay_guard: ReplayGuard::default(),
            inventories: HashMap::new(),
            own_inventory_version: 0,
            one_sided: HashMap::new(),
            neighbor_since: HashMap::new(),
            silent_rejoined_at: HashMap::new(),
            gossip_dialed_at: HashMap::new(),
            announced: HashMap::new(),
            announced_all_at: 0,
            new_node_seen: false,
            control_rx: start.control_rx,
            rtt_rx,
            summary_rx,
            inventory_rx,
            deploy_tx,
            update_tx,
        }
    }

    async fn run(mut self) {
        self.subscribe_events().await;
        update::READY.store(true, std::sync::atomic::Ordering::Relaxed);
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
                Some(InventoryChanged) = self.inventory_rx.recv() => {
                    self.refresh_own_inventory(true).await;
                    self.announce_deploys().await;
                }
                _ = beat_tick.tick() => {
                    self.on_beat_tick().await;
                    // Larger meshes beat less often (`beat_interval_for`).
                    let nodes = self.ctx.with_state(|st| st.nodes.len()).unwrap_or(1);
                    let interval = beat_interval_for(nodes);
                    if interval != self.beat_interval_secs {
                        self.beat_interval_secs = interval;
                        update::BEAT_INTERVAL_SECS.store(interval, std::sync::atomic::Ordering::Relaxed);
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
                            self.neighbor_since.insert(peer, monotonic_now_ms());
                            self.neighbors_changed();
                            None
                        }
                        Some(Ok(GossipEvent::NeighborDown(peer))) => {
                            self.neighbor_since.remove(&peer);
                            self.neighbors_changed();
                            self.keep_neighbors().await;
                            None
                        }
                        Some(Ok(GossipEvent::Received(msg))) => {
                            self.ctx.net.count_in(msg.content.len());
                            self.on_received(&msg.content, msg.delivered_from).await;
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
                        self.on_received(&msg.content, msg.delivered_from).await;
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
                self.dial_for_gossip(&peers);
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

    /// Dial peers for gossip and hand the connections to it. iroh-gossip (0.101) dials a peer
    /// only when nothing is queued for it, and keeps what it queued when that dial fails, so a
    /// seed that was down for a while when this node started is never dialed again and the
    /// node stays alone after the seed is back. A connection made here sends what it queued.
    fn dial_for_gossip(&mut self, peers: &[EndpointId]) -> Vec<EndpointId> {
        let now = monotonic_now_ms();
        let every = GOSSIP_DIAL_EVERY.as_millis() as u64;
        let mut dialed = Vec::new();
        for &peer in peers {
            if self
                .gossip_dialed_at
                .get(&peer)
                .is_some_and(|at| now.saturating_sub(*at) < every)
            {
                continue;
            }
            self.gossip_dialed_at.insert(peer, now);
            dialed.push(peer);
            let endpoint = self.ctx.endpoint.clone();
            let gossip = self.gossip.clone();
            tokio::spawn(async move {
                let dial = endpoint.connect(peer, iroh_gossip::net::GOSSIP_ALPN);
                if let Ok(Ok(conn)) = tokio::time::timeout(Duration::from_secs(20), dial).await {
                    let _ = gossip.handle_connection(conn).await;
                }
            });
        }
        dialed
    }

    /// Join the seeds this node has heard no fresh beat from. A group of nodes that are each
    /// other's neighbors but split from the rest (they all started while the seed was down)
    /// has neighbors enough, so `keep_neighbors` would never bring it back. A seed that
    /// restarted counts once its beats go stale, not only once it is pruned (minutes later).
    async fn join_unheard_seeds(&mut self) {
        if !self.subscribed || self.keys.is_none() {
            return;
        }
        let own = self.ctx.endpoint.id();
        let now = now_unix();
        let heard: HashSet<String> = self
            .ctx
            .with_state(|st| {
                st.nodes
                    .iter()
                    .filter(|n| now.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n))
                    .map(|n| n.node_id.clone())
                    .collect()
            })
            .unwrap_or_default();
        let unheard: Vec<EndpointId> = self
            .seed_ids
            .iter()
            .filter(|id| **id != own && !heard.contains(&id.to_string()))
            .copied()
            .collect();
        let dialed = self.dial_for_gossip(&unheard);
        if !dialed.is_empty() {
            let _ =
                tokio::time::timeout(Duration::from_millis(600), self.sender.join_peers(dialed))
                    .await;
        }
    }

    /// Publish the beats topic's neighbors for the probe, the Paths actor and `/api/net`.
    fn neighbors_changed(&mut self) {
        let neighbors: Vec<String> = self.receiver.neighbors().map(|id| id.to_string()).collect();
        update::NEIGHBORS.store(neighbors.len() as u64, std::sync::atomic::Ordering::Relaxed);
        self.ctx
            .net
            .neighbors
            .store(neighbors.len() as u64, std::sync::atomic::Ordering::Relaxed);
        self.ctx.with_state(|st| st.gossip_neighbors = neighbors);
    }

    /// Keep the saved seeds to peers this node hears, so after a restart it still reaches the
    /// mesh when the member it joined through is gone.
    fn refresh_saved_seeds(&mut self) {
        let now = now_unix();
        let own = self.ctx.endpoint.id().to_string();
        let live: Vec<Seed> = self
            .ctx
            .with_state(|st| {
                st.nodes
                    .iter()
                    .filter(|n| {
                        n.node_id != own
                            && now.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n)
                    })
                    .filter_map(|n| n.node_id.parse::<EndpointId>().ok())
                    .filter_map(|id| self.known_peer_addrs.get(&id).map(Seed::from_addr))
                    .collect()
            })
            .unwrap_or_default();
        // Change only the seeds on disk: the CLI may have just edited the rest of the config.
        let config_path = self.ctx.config_path.clone();
        let saved = self
            .ctx
            .with_state(|st| {
                let mut disk = try_load_mesh_config(&config_path).ok()?;
                let next = refresh_seeds(&disk.seeds, &live)?;
                disk.seeds = next.clone();
                save_mesh_config(&config_path, &disk).ok()?;
                st.mesh_config.seeds = next.clone();
                Some(next)
            })
            .flatten();
        if let Some(seeds) = saved {
            self.seed_ids = seeds
                .iter()
                .filter_map(|s| s.node_id.parse().ok())
                .collect();
            println!(
                "[mesh] saved {} seed(s): the peers this node hears",
                seeds.len()
            );
        }
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

    /// A beat about this node, with its stats, summary and gossip neighbors.
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
        let rtt = self.rtt_rx.borrow().clone();
        beat.neighbors = self
            .receiver
            .neighbors()
            .take(MAX_NEIGHBORS)
            .map(|id| {
                let id = id.to_string();
                NeighborLink {
                    node_id_prefix: neighbor_prefix(&id),
                    rtt_ms: rtt.get(&id).copied().unwrap_or(0.0),
                }
            })
            .collect();
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

    /// Rebuild this node's inventory; when it changed, broadcast it on the events topic (if
    /// `broadcast`) and announce the new version in the next beat.
    async fn refresh_own_inventory(&mut self, broadcast: bool) {
        let inventory = self.own_inventory().for_wire();
        if inventory.version == self.own_inventory_version {
            return;
        }
        let Some((wire, _)) = self.seal(&Message::Inventory(inventory.clone())) else {
            return;
        };
        self.own_inventory_version = inventory.version;
        if broadcast {
            if let Some(tx) = &self.events_sender {
                self.ctx.net.count_out(wire.len());
                let _ = tokio::time::timeout(Duration::from_millis(600), tx.broadcast(wire.into()))
                    .await;
            }
        }
    }

    /// The programs this node deploys to the mesh, as announcements. Targets named by node
    /// name are announced by NodeId.
    fn own_deploys(&self) -> HashMap<String, Deploy> {
        self.ctx
            .with_state(|st| {
                st.processes
                    .iter()
                    .filter(|p| p.desired_state == "running")
                    .filter_map(|p| {
                        let (ticket, targets) = mesh_offer(p)?;
                        let targets = targets
                            .into_iter()
                            .map(|t| match st.nodes.iter().find(|n| n.node == t) {
                                Some(n) => n.node_id.clone(),
                                None => t,
                            })
                            .collect();
                        let deploy = Deploy {
                            node: self.node.clone(),
                            node_id: self.ctx.node_id.clone(),
                            process: p.clone(),
                            ticket,
                            targets,
                            sent_unix_ms: 0,
                        };
                        Some((p.name.clone(), deploy))
                    })
                    .collect()
            })
            .unwrap_or_default()
    }

    /// Announce what changed in this node's deploys: new or changed programs, and removed
    /// ones. Every `REANNOUNCE_EVERY`, and soon after a node joins, announce them all again
    /// (removed ones for `REMOVED_ANNOUNCE_FOR`), so nodes that missed one catch up through
    /// gossip alone.
    async fn announce_deploys(&mut self) {
        if self.events_sender.is_none() {
            return;
        }
        let now = monotonic_now_ms();
        let current = self.own_deploys();
        let mut send = Vec::new();
        for (name, deploy) in &current {
            let changed = match self.announced.get(name) {
                Some((last, removed)) => {
                    removed.is_some()
                        || last.process.digest != deploy.process.digest
                        || last.targets != deploy.targets
                        || last.ticket != deploy.ticket
                }
                None => true,
            };
            if changed {
                send.push(deploy.clone());
                self.announced.insert(name.clone(), (deploy.clone(), None));
            }
        }
        for (name, (last, removed)) in self.announced.iter_mut() {
            if removed.is_none() && !current.contains_key(name) {
                last.process.desired_state = "removed".to_string();
                *removed = Some(now);
                send.push(last.clone());
            }
        }
        self.announced.retain(|_, (_, removed)| {
            removed
                .is_none_or(|at| now.saturating_sub(at) < REMOVED_ANNOUNCE_FOR.as_millis() as u64)
        });
        let since_all = now.saturating_sub(self.announced_all_at);
        let again = since_all >= REANNOUNCE_EVERY.as_millis() as u64
            || (self.new_node_seen && since_all >= NEW_NODE_ANNOUNCE_GAP.as_millis() as u64);
        if again && !self.announced.is_empty() {
            send = self.announced.values().map(|(d, _)| d.clone()).collect();
        }
        if again {
            self.announced_all_at = now;
            self.new_node_seen = false;
        }
        for deploy in send {
            println!(
                "[deploy] announcing '{}' ({}) to {}",
                deploy.process.name,
                deploy.process.desired_state,
                if deploy.targets.is_empty() {
                    "every node".to_string()
                } else {
                    format!("{} node(s)", deploy.targets.len())
                }
            );
            if let (Some((wire, _)), Some(tx)) =
                (self.seal(&Message::Deploy(deploy)), &self.events_sender)
            {
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

    /// Broadcast a beat on the beats topic.
    async fn broadcast(&mut self, msg: &Message) {
        if let Some((wire, _)) = self.seal(msg) {
            self.ctx.net.count_out(wire.len());
            let _ = tokio::time::timeout(
                Duration::from_millis(600),
                self.sender.broadcast(wire.into()),
            )
            .await;
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
                // A peer that went silent is no longer a join candidate (seeds stay).
                let seeds = self.seed_ids.clone();
                self.known_peer_addrs
                    .retain(|id, _| seeds.contains(id) || live_ids.contains(&id.to_string()));
                println!("[mesh] pruned {pruned} peer(s) silent for over {ttl}s");
            }
        }
        self.ctx.scores(PeerScores::decay);
        self.keep_neighbors().await;
        self.join_unheard_seeds().await;
        self.rejoin_silent_neighbors().await;

        let counter = self.counter;
        if counter.is_multiple_of(SEED_REFRESH_EVERY) {
            self.refresh_saved_seeds();
        }
        if counter <= 2 || counter.is_multiple_of(INVENTORY_REFRESH_EVERY) {
            self.refresh_own_inventory(true).await;
        }
        let mut beat = self.fresh_beat();
        let node_id = self.ctx.node_id.clone();
        self.ctx.with_state(|st| {
            st.reconcile_processes();
            beat.heard = st
                .nodes
                .iter()
                .filter(|n| {
                    n.node_id != node_id
                        && now.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n)
                })
                .count() as u32;
            beat.processes = st.processes.clone();
            beat.service_endpoints = st.service_endpoints.clone();
            beat.devices = st.devices.clone();
            beat.probe = st.probe.clone();
            beat.peer_rtt_ms = st
                .nodes
                .iter()
                .find(|n| n.node_id == node_id)
                .map(|me| me.peer_rtt_ms.clone())
                .unwrap_or_default();
            st.iroh_runtime = beat.iroh_transport.clone();
            st.stats = beat.stats.clone();
            merge_beat(&mut st.nodes, beat.clone());
        });
        self.broadcast(&Message::Beat(beat)).await;
        self.counter += 1;
        self.announce_deploys().await;
    }

    /// Admit a message only if it opens with the channel key, its signature matches a
    /// listed node, its name is valid and it is not a replay.
    fn admit(&mut self, content: &[u8]) -> Result<Message, Rejected> {
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
        if !self.ctx.allowlist.admits_str(&node_id) {
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
        let now_ms = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_millis() as u64)
            .unwrap_or(0);
        if let Err(reason) = self
            .replay_guard
            .admit(&msg.replay_key(), msg.sent_unix_ms(), now_ms)
        {
            eprintln!(
                "[gossip-replay-reject] dropped message from node={} node_id={node_id}: {reason}",
                msg.node()
            );
            return Err(Rejected::Ignored);
        }
        if let Ok(id) = node_id.parse() {
            self.ctx.members.insert(id);
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

    /// A gossip message `delivered_from` a neighbor: dropped when that neighbor is
    /// graylisted (GossipSub's graylist), otherwise admitted, scored and kept.
    async fn on_received(&mut self, content: &[u8], delivered_from: EndpointId) {
        if self
            .ctx
            .scores(|s| s.graylisted(&delivered_from))
            .unwrap_or(false)
        {
            return;
        }
        let admitted = self.admit(content);
        match &admitted {
            Ok(_) => self.ctx.scores(|s| s.delivered(delivered_from, true)),
            Err(Rejected::Invalid) => self.ctx.scores(|s| s.delivered(delivered_from, false)),
            Err(Rejected::Ignored) => None,
        };
        let Ok(msg) = admitted else {
            return;
        };
        match msg {
            Message::Inventory(inventory) => self.on_inventory(inventory),
            Message::Beat(beat) => self.on_beat(beat).await,
            Message::Deploy(deploy) => {
                let _ = self.deploy_tx.send(deploy);
            }
        }
    }

    fn on_inventory(&mut self, inventory: Inventory) {
        if !self.ctx.allowlist.admits_str(&inventory.node_id) {
            return;
        }
        self.ctx
            .with_state(|st| merge_inventory(&mut st.nodes, &inventory));
        self.inventories
            .insert(inventory.node_id.clone(), inventory);
    }

    async fn on_beat(&mut self, mut beat: Beat) {
        self.learn_addresses(&beat);
        if update::newer(&beat.version, update::running_version()) {
            let _ = self.update_tx.send(UpdateHint {
                node_id: beat.node_id.clone(),
                version: beat.version.clone(),
            });
        }
        self.rejoin_if_one_sided(&beat).await;
        // Stamp with our clock once admitted, so clock skew between hosts cannot cause
        // early TTL pruning.
        beat.updated_at_unix = now_unix();
        self.merge_peer_beat(beat);
    }

    /// A neighbor that does not count this node as its neighbor sends it nothing, so its
    /// beats never arrive (when it has no other neighbors, they go nowhere at all). A
    /// neighbor this node has not heard a fresh beat from for three beat intervals is joined
    /// again, which completes the link on both sides; at most once per three intervals.
    async fn rejoin_silent_neighbors(&mut self) {
        let now = monotonic_now_ms();
        let grace = 3 * self.beat_interval_secs * 1000;
        let neighbors: Vec<EndpointId> = self.receiver.neighbors().collect();
        self.neighbor_since.retain(|id, _| neighbors.contains(id));
        self.silent_rejoined_at
            .retain(|id, _| neighbors.contains(id));
        let unix = now_unix();
        let silent: Vec<(EndpointId, String)> = self
            .ctx
            .with_state(|st| {
                neighbors
                    .iter()
                    .filter(|id| {
                        let id = id.to_string();
                        !st.nodes.iter().any(|n| {
                            n.node_id == id
                                && unix.saturating_sub(n.updated_at_unix) <= beat_fresh_secs(n)
                        })
                    })
                    .map(|id| {
                        let name = st
                            .nodes
                            .iter()
                            .find(|n| n.node_id == id.to_string())
                            .map(|n| n.node.clone())
                            .unwrap_or_else(|| id.fmt_short().to_string());
                        (*id, name)
                    })
                    .collect()
            })
            .unwrap_or_default();
        for (id, name) in silent {
            let since = *self.neighbor_since.entry(id).or_insert(now);
            let last = self.silent_rejoined_at.get(&id).copied().unwrap_or(0);
            if now.saturating_sub(since) < grace || now.saturating_sub(last) < grace {
                continue;
            }
            self.silent_rejoined_at.insert(id, now);
            println!(
                "[mesh] neighbor '{name}' has sent no beat for {}s; joining it again",
                grace / 1000
            );
            let _ =
                tokio::time::timeout(Duration::from_millis(600), self.sender.join_peers(vec![id]))
                    .await;
        }
    }

    /// iroh-gossip's HyParView can lose the one `Neighbor` reply that completes a link and
    /// never send it again, leaving this node with a neighbor that does not know it. Each
    /// beat lists its sender's neighbors: when a neighbor's beats leave this node out
    /// `ONE_SIDED_BEATS` times in a row, a Join sent to it completes the link on both sides.
    async fn rejoin_if_one_sided(&mut self, beat: &Beat) {
        let Ok(peer) = beat.node_id.parse::<EndpointId>() else {
            return;
        };
        if !self.receiver.neighbors().any(|n| n == peer) {
            self.one_sided.remove(&peer);
            return;
        }
        let own = neighbor_prefix(&self.ctx.node_id);
        let rounds = self.one_sided.entry(peer).or_default();
        *rounds = if beat.neighbors.iter().any(|n| n.node_id_prefix == own) {
            0
        } else {
            *rounds + 1
        };
        if *rounds >= ONE_SIDED_BEATS {
            *rounds = 0;
            println!(
                "[mesh] neighbor '{}' does not list this node as its neighbor; joining it again",
                beat.node
            );
            let _ = tokio::time::timeout(
                Duration::from_millis(600),
                self.sender.join_peers(vec![peer]),
            )
            .await;
        }
    }

    /// Merge an admitted peer beat into the node state (with its inventory, the first time).
    fn merge_peer_beat(&mut self, mut beat: Beat) {
        let peer = beat.node.clone();
        let inventory = self.inventories.get(&beat.node_id).cloned();
        let joined = self.ctx.with_state(|st| {
            // The beat names its neighbors by NodeId prefix; the HUD wants names.
            beat.peer_rtt_ms = beat
                .neighbors
                .iter()
                .filter(|l| l.rtt_ms > 0.0)
                .filter_map(|l| {
                    let n = st
                        .nodes
                        .iter()
                        .find(|n| n.node_id.starts_with(&l.node_id_prefix))?;
                    Some((n.node.clone(), l.rtt_ms))
                })
                .collect();
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
            first_seen
        });
        if joined == Some(true) {
            self.new_node_seen = true;
        }
    }
}
