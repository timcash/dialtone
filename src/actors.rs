//! The running node as actors: one task per job, joined by channels and stopped by one
//! cancellation token.
//!
//! - **Mesh** owns the gossip subscription: beats, RTT echoes, peer discovery, channel joins.
//! - **Stats** samples the host every second and keeps the rolling summary.
//! - **Supervisor** reconciles processes every second and tells Mesh when they change.
//! - **Links** observes how each peer is reached (direct, relay, tailnet).
//! - **Allowlist** reloads the allowlist when `config.json` changes.
//! - **Deploy** pulls, verifies and starts programs that allowlisted peers run.
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
use dialtone::mesh::{
    build_mesh_rtt_matrix, cached_local_public_ip_geo, compute_rtt_ms_from_echo, default_seeds,
    is_valid_node_name, merge_beat, monotonic_now_ms, normalize_channel_name, now_unix,
    observe_peer_remote_addrs, open_beat, place_nodes, prune_stale_peers, seal_beat,
    try_load_mesh_config, Beat, ChannelKeys, LiveAllowlist, MeshControlCommand, ReplayGuard,
    SeedList, BEAT_INTERVAL_SECS, DEFAULT_CHANNEL_NAME, DEFAULT_PEER_TTL_SECS,
    SUMMARY_EVERY_N_BEATS, SUMMARY_INTERVAL_SECS,
};
use dialtone::node::{advertised_endpoint_addr, build_diagnostics, IrohTransportConfig};
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

use crate::{echoes_for_peers, self_beat};

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
    pub cancel: CancellationToken,
}

impl Ctx {
    /// Run `f` on the node state, if this node has one (the `node` action does not).
    fn with_state<R>(&self, f: impl FnOnce(&mut NodeState) -> R) -> Option<R> {
        let mut st = self.state.as_ref()?.lock().ok()?;
        Some(f(&mut st))
    }
}

/// The processes and services this node runs, sent by Supervisor when they change.
pub(crate) type ProcessesChanged = (Vec<ProcessSpec>, Vec<dialtone::service::Service>);

/// The programs a peer says it runs, sent by Mesh to Deploy for every admitted beat.
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
    pub joined_peer_ids: HashSet<EndpointId>,
    pub print_first_beat: bool,
    pub control_rx: mpsc::UnboundedReceiver<MeshControlCommand>,
}

/// Start every actor. The returned handles finish after `ctx.cancel` is cancelled.
pub(crate) fn spawn_all(ctx: Ctx, start: MeshStart) -> Vec<JoinHandle<()>> {
    let (rtt_tx, rtt_rx) = watch::channel(BTreeMap::new());
    let (summary_tx, summary_rx) = watch::channel(None);
    let (procs_tx, procs_rx) = mpsc::unbounded_channel();
    let (deploy_tx, deploy_rx) = mpsc::unbounded_channel();
    let mesh = Mesh::new(ctx.clone(), start, rtt_tx, summary_rx, procs_rx, deploy_tx);
    vec![
        tokio::spawn(mesh.run()),
        tokio::spawn(stats_actor(ctx.clone(), rtt_rx, summary_tx)),
        tokio::spawn(supervisor_actor(ctx.clone(), procs_tx)),
        tokio::spawn(links_actor(ctx.clone())),
        tokio::spawn(allowlist_actor(ctx.clone())),
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
            place_nodes(&mut st.nodes, &matrix);
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
async fn supervisor_actor(ctx: Ctx, procs_tx: mpsc::UnboundedSender<ProcessesChanged>) {
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
            let change = (st.processes.clone(), st.service_endpoints.clone());
            if let Some(me) = st.self_entry_mut() {
                me.processes = change.0.clone();
                me.service_endpoints = change.1.clone();
            }
            Some(change)
        });
        if let Some(Some(change)) = changed {
            let _ = procs_tx.send(change);
        }
    }
}

// ---------------------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------------------

/// Every beat interval, record how each peer is reached (direct IP, relay, tailnet).
async fn links_actor(ctx: Ctx) {
    let mut tick = every(Duration::from_secs(BEAT_INTERVAL_SECS));
    loop {
        tokio::select! {
            _ = ctx.cancel.cancelled() => break,
            _ = tick.tick() => {}
        }
        let peer_ids: Vec<String> = ctx
            .with_state(|st| {
                st.nodes
                    .iter()
                    .filter(|n| n.node_id != ctx.node_id)
                    .map(|n| n.node_id.clone())
                    .collect()
            })
            .unwrap_or_default();
        for pid in peer_ids {
            let observed = tokio::time::timeout(
                Duration::from_millis(350),
                observe_peer_remote_addrs(&ctx.endpoint, &pid),
            )
            .await;
            if let Ok((addrs, tailnet)) = observed {
                if addrs.is_empty() {
                    continue;
                }
                ctx.with_state(|st| {
                    if let Some(peer) = st.nodes.iter_mut().find(|n| n.node_id == pid) {
                        peer.observed_remote_addrs = addrs;
                        peer.tailnet_used = tailnet;
                    }
                });
            }
        }
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
// Deploy
// ---------------------------------------------------------------------------------------

/// Run the programs allowlisted peers run: pull each from its ticket (only from an
/// allowlisted provider), verify its SHA-256, stage it and start it.
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

struct Mesh {
    ctx: Ctx,
    node: String,
    node_key: iroh::SecretKey,
    gossip: Gossip,
    sender: GossipSender,
    receiver: GossipReceiver,
    subscribed: bool,
    resubscribe_backoff: Duration,
    keys: Option<ChannelKeys>,
    channel: String,
    topic_id: TopicId,
    seed_ids: Vec<EndpointId>,
    known_peer_addrs: HashMap<EndpointId, EndpointAddr>,
    joined_peer_ids: HashSet<EndpointId>,
    counter: u64,
    print_first_beat: bool,
    replay_guard: ReplayGuard,
    /// Peer name -> (the peer's send time, our receive time), echoed back for RTT.
    last_rx_from_peer: HashMap<String, (u64, u64)>,
    measured_rtt_ms: BTreeMap<String, f64>,
    last_echo_reply_mono_ms: HashMap<String, u64>,
    control_rx: mpsc::UnboundedReceiver<MeshControlCommand>,
    rtt_tx: watch::Sender<BTreeMap<String, f64>>,
    summary_rx: watch::Receiver<Option<StatsSummary>>,
    procs_rx: mpsc::UnboundedReceiver<ProcessesChanged>,
    deploy_tx: mpsc::UnboundedSender<PeerPrograms>,
}

impl Mesh {
    fn new(
        ctx: Ctx,
        start: MeshStart,
        rtt_tx: watch::Sender<BTreeMap<String, f64>>,
        summary_rx: watch::Receiver<Option<StatsSummary>>,
        procs_rx: mpsc::UnboundedReceiver<ProcessesChanged>,
        deploy_tx: mpsc::UnboundedSender<PeerPrograms>,
    ) -> Self {
        Mesh {
            ctx,
            node: start.node,
            node_key: start.node_key,
            gossip: start.gossip,
            sender: start.sender,
            receiver: start.receiver,
            subscribed: true,
            resubscribe_backoff: Duration::from_secs(1),
            keys: start.keys,
            channel: start.channel,
            topic_id: start.topic_id,
            seed_ids: start.seed_ids,
            known_peer_addrs: start.known_peer_addrs,
            joined_peer_ids: start.joined_peer_ids,
            counter: 1,
            print_first_beat: start.print_first_beat,
            replay_guard: ReplayGuard::default(),
            last_rx_from_peer: HashMap::new(),
            measured_rtt_ms: BTreeMap::new(),
            last_echo_reply_mono_ms: HashMap::new(),
            control_rx: start.control_rx,
            rtt_tx,
            summary_rx,
            procs_rx,
            deploy_tx,
        }
    }

    async fn run(mut self) {
        let mut beat_tick = every(Duration::from_secs(BEAT_INTERVAL_SECS));
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
                Some(change) = self.procs_rx.recv() => self.on_processes_changed(change).await,
                _ = beat_tick.tick() => self.on_beat_tick().await,
                event = self.receiver.next(), if self.subscribed => {
                    let ended = match event {
                        Some(Ok(GossipEvent::NeighborUp(peer))) => {
                            self.on_neighbor_up(peer).await;
                            None
                        }
                        Some(Ok(GossipEvent::NeighborDown(_))) => {
                            self.rejoin_seeds(Duration::from_millis(400)).await;
                            None
                        }
                        Some(Ok(GossipEvent::Received(msg))) => {
                            self.on_received(&msg.content).await;
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
                        resubscribe_at.as_mut().reset(tokio::time::Instant::now() + self.resubscribe_backoff);
                    }
                }
            }
        }
    }

    async fn resubscribe(&mut self) -> Result<(), Duration> {
        match self
            .gossip
            .subscribe(self.topic_id, self.seed_ids.clone())
            .await
        {
            Ok(topic) => {
                (self.sender, self.receiver) = topic.split();
                self.subscribed = true;
                self.resubscribe_backoff = Duration::from_secs(1);
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

    /// Remember a peer's address; returns whether it is new to the mesh.
    fn learn_peer(&mut self, addr: &EndpointAddr) -> bool {
        let clean = advertised_endpoint_addr(addr, &self.ctx.transport);
        let mut new = false;
        if clean.id != self.ctx.endpoint.id() {
            self.known_peer_addrs.insert(clean.id, clean.clone());
            new = self.joined_peer_ids.insert(clean.id);
        }
        self.ctx.memory_lookup.add_endpoint_info(clean);
        new
    }

    async fn rejoin_seeds(&mut self, timeout: Duration) {
        if self.keys.is_some() && !self.seed_ids.is_empty() {
            let _ =
                tokio::time::timeout(timeout, self.sender.join_peers(self.seed_ids.clone())).await;
        }
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
                    self.rejoin_seeds(Duration::from_millis(800)).await;
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

    /// A beat about this node, with the current seed list, stats and summary.
    fn fresh_beat(&self) -> Beat {
        let adv = advertised_endpoint_addr(&self.ctx.endpoint.addr(), &self.ctx.transport);
        let diag = build_diagnostics(
            &self.ctx.transport,
            &adv,
            Some(&self.ctx.key_path),
            &self.ctx.node_id,
        );
        let mut peers = vec![adv];
        peers.extend(self.known_peer_addrs.values().cloned());
        let seed_list = SeedList {
            topic: self.topic_id,
            peers,
        }
        .to_text()
        .unwrap_or_else(|e| {
            eprintln!("[beat] could not encode seed list: {e}");
            String::new()
        });
        let mut beat = self_beat(
            &self.ctx.blobs,
            &self.node,
            &self.ctx.node_id,
            seed_list,
            self.counter,
            diag,
        );
        let now_mono = monotonic_now_ms();
        beat.sent_mono_ms = now_mono;
        beat.peer_echoes = echoes_for_peers(&self.last_rx_from_peer, now_mono);
        beat.peer_rtt_ms = self.measured_rtt_ms.clone();
        beat.summary_stats = self.summary_rx.borrow().clone();
        beat
    }

    /// Copy this node's processes, services and last probe into `beat`.
    fn attach_processes(&self, beat: &mut Beat) {
        self.ctx.with_state(|st| {
            beat.processes = st.processes.clone();
            beat.service_endpoints = st.service_endpoints.clone();
            beat.probe = st.probe.clone();
        });
    }

    async fn broadcast(&mut self, beat: &Beat, timeout: Duration) {
        let Some(keys) = &self.keys else { return };
        if let Ok(wire) = seal_beat(beat, keys, &self.node_key) {
            let _ = tokio::time::timeout(timeout, self.sender.broadcast(wire.into())).await;
        }
    }

    async fn on_processes_changed(&mut self, (processes, services): ProcessesChanged) {
        if self.keys.is_none() {
            return;
        }
        let mut beat = self.fresh_beat();
        beat.processes = processes;
        beat.service_endpoints = services;
        self.broadcast(&beat, Duration::from_millis(500)).await;
    }

    async fn on_beat_tick(&mut self) {
        let now = now_unix();
        let pruned = self.ctx.with_state(|st| {
            if let Some(me) = st.self_entry_mut() {
                me.updated_at_unix = now;
            }
            let pruned = prune_stale_peers(&mut st.nodes, now, DEFAULT_PEER_TTL_SECS);
            let live: HashSet<String> = st.nodes.iter().map(|n| n.node.clone()).collect();
            (pruned, live)
        });
        if let Some((pruned, live)) = pruned {
            if pruned > 0 {
                self.measured_rtt_ms.retain(|name, _| live.contains(name));
                self.last_rx_from_peer.retain(|name, _| live.contains(name));
                self.rtt_tx.send_replace(self.measured_rtt_ms.clone());
                println!("[mesh] pruned {pruned} peer(s) silent for over {DEFAULT_PEER_TTL_SECS}s");
            }
        }

        if self.keys.is_some() && !self.seed_ids.is_empty() {
            for seed in default_seeds() {
                if seed.id != self.ctx.endpoint.id() {
                    self.ctx.memory_lookup.add_endpoint_info(seed);
                }
            }
            for addr in self.known_peer_addrs.values() {
                self.ctx.memory_lookup.add_endpoint_info(addr.clone());
            }
            self.rejoin_seeds(Duration::from_millis(600)).await;
        }

        let counter = self.counter;
        let mut beat = self.fresh_beat();
        let is_summary = counter <= 2 || counter.is_multiple_of(SUMMARY_EVERY_N_BEATS);
        let local_summary = beat.summary_stats.clone();
        beat.kind = if is_summary { "summary" } else { "regular" }.to_string();
        if !is_summary {
            beat.summary_stats = None;
        }
        let node_id = self.ctx.node_id.clone();
        let measured = &self.measured_rtt_ms;
        let in_state = self.ctx.with_state(|st| {
            st.reconcile_processes();
            beat.processes = st.processes.clone();
            beat.service_endpoints = st.service_endpoints.clone();
            beat.probe = st.probe.clone();
            let missing_rtt = st
                .nodes
                .iter()
                .any(|n| n.node_id != node_id && !measured.contains_key(&n.node));
            beat.rtt_probe_requested = missing_rtt || counter <= 2 || counter.is_multiple_of(3);
            st.iroh_runtime = beat.iroh_transport.clone();
            st.stats = beat.stats.clone();
            let mut local_copy = beat.clone();
            local_copy.summary_stats = local_summary.clone();
            merge_beat(&mut st.nodes, local_copy);
            let matrix = build_mesh_rtt_matrix(&st.nodes);
            place_nodes(&mut st.nodes, &matrix);
            st.mesh_rtt_ms = matrix;
            st.refresh_live_alerts();
            if let (true, Some(s)) = (is_summary, &local_summary) {
                st.record_log(
                    "gossip-beat",
                    "INFO",
                    "stdout",
                    format!(
                        "published 30s summary beat #{} (cpu {:.1}% ±{:.1}%, mem {:.1}% ±{:.1}%, thrashing={}, rtt_peers={})",
                        counter,
                        s.cpu_pct.mean,
                        s.cpu_pct.stddev,
                        s.mem_pct.mean,
                        s.mem_pct.stddev,
                        s.thrashing_detected,
                        measured.len()
                    ),
                );
            }
        });
        if in_state.is_none() {
            beat.rtt_probe_requested = counter <= 2 || counter.is_multiple_of(3);
        }
        if counter == 1 && self.print_first_beat {
            println!(
                "{}",
                serde_json::to_string_pretty(&beat).unwrap_or_default()
            );
        }
        self.broadcast(&beat, Duration::from_millis(600)).await;
        self.counter += 1;
    }

    async fn on_neighbor_up(&mut self, peer: EndpointId) {
        if self.keys.is_none() {
            return;
        }
        let listed = peer.to_string() == self.ctx.node_id || self.ctx.allowlist.contains(&peer);
        if self.channel == DEFAULT_CHANNEL_NAME && !listed {
            self.rejoin_seeds(Duration::from_millis(400)).await;
            return;
        }
        let mut beat = self.fresh_beat();
        beat.kind = "summary".to_string();
        beat.rtt_probe_requested = true;
        self.attach_processes(&mut beat);
        self.broadcast(&beat, Duration::from_millis(600)).await;
    }

    /// Admit a beat only if it opens with the channel key, its signature matches a listed
    /// node, its name is valid and it is not a replay.
    fn admit(&mut self, content: &[u8]) -> Option<Beat> {
        let keys = self.keys.as_ref()?;
        let beat = match open_beat(content, keys) {
            Ok(beat) => beat,
            Err(reason) => {
                eprintln!(
                    "[gossip-auth-reject] dropped gossip payload on channel '{}': {}",
                    self.channel, reason
                );
                return None;
            }
        };
        // open_beat verified the signature, so beat.node_id is the real sender.
        if beat.node_id == self.ctx.node_id {
            return None;
        }
        if !self.ctx.allowlist.contains_str(&beat.node_id) {
            eprintln!(
                "[gossip-allowlist-reject] dropped beat from unlisted node_id={} on channel '{}' (run 'dialtone allowlist add {}' to authorize)",
                beat.node_id, self.channel, beat.node_id
            );
            return None;
        }
        if !is_valid_node_name(&beat.node) {
            eprintln!(
                "[gossip-auth-reject] dropped beat from node_id={} with an invalid node name",
                beat.node_id
            );
            return None;
        }
        let now_ms = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_millis() as u64)
            .unwrap_or(0);
        if let Err(reason) = self
            .replay_guard
            .admit(&beat.node_id, beat.sent_unix_ms, now_ms)
        {
            eprintln!(
                "[gossip-replay-reject] dropped beat from node={} node_id={}: {}",
                beat.node, beat.node_id, reason
            );
            return None;
        }
        Some(beat)
    }

    /// Update RTT to the beat's sender from its echo of our last beat.
    fn measure_rtt(&mut self, beat: &mut Beat, rx_mono: u64) {
        if beat.sent_mono_ms > 0 {
            self.last_rx_from_peer
                .insert(beat.node.clone(), (beat.sent_mono_ms, rx_mono));
        }
        if let Some(echo) = beat.peer_echoes.get(&self.node) {
            if echo.hold_time_ms <= 600 || !self.measured_rtt_ms.contains_key(&beat.node) {
                if let Some(rtt_ms) = compute_rtt_ms_from_echo(rx_mono, echo) {
                    let smoothed = match self.measured_rtt_ms.get(&beat.node) {
                        Some(&prev) => (((prev * 0.6) + (rtt_ms * 0.4)) * 10.0).round() / 10.0,
                        None => rtt_ms,
                    };
                    self.measured_rtt_ms.insert(beat.node.clone(), smoothed);
                }
            }
        }
        if let Some(&reported) = beat.peer_rtt_ms.get(&self.node) {
            if reported > 0.0 {
                self.measured_rtt_ms
                    .entry(beat.node.clone())
                    .or_insert(reported);
            }
        }
        if let Some(&ours) = self.measured_rtt_ms.get(&beat.node) {
            beat.peer_rtt_ms.insert(self.node.clone(), ours);
        }
        self.rtt_tx.send_replace(self.measured_rtt_ms.clone());
    }

    /// Learn the addresses a beat advertises and join any peer that is new.
    async fn discover_peers(&mut self, beat: &Beat) {
        let mut addrs = Vec::new();
        if let Ok(list) = SeedList::from_str(&beat.ticket) {
            addrs.extend(list.peers);
        }
        if let Ok(pid) = beat.node_id.parse::<EndpointId>() {
            let mut direct = EndpointAddr::new(pid);
            for r in &beat.iroh_transport.advertised_relay_urls {
                if let Ok(url) = r.parse::<iroh::RelayUrl>() {
                    direct = direct.with_relay_url(url);
                }
            }
            for ip in &beat.iroh_transport.advertised_ip_addrs {
                if let Ok(sock) = ip.parse::<std::net::SocketAddr>() {
                    direct = direct.with_ip_addr(sock);
                }
            }
            addrs.push(direct);
        }
        let mut new_peers = Vec::new();
        for addr in addrs {
            if self.learn_peer(&addr) {
                new_peers.push(addr.id);
            }
        }
        if !new_peers.is_empty() {
            for id in &new_peers {
                if !self.seed_ids.contains(id) {
                    self.seed_ids.push(*id);
                }
            }
            let _ = tokio::time::timeout(
                Duration::from_millis(600),
                self.sender.join_peers(new_peers),
            )
            .await;
        }
    }

    async fn on_received(&mut self, content: &[u8]) {
        let Some(mut beat) = self.admit(content) else {
            return;
        };
        let rx_mono = monotonic_now_ms();
        self.measure_rtt(&mut beat, rx_mono);
        self.discover_peers(&beat).await;
        let (addrs, tailnet) = tokio::time::timeout(
            Duration::from_millis(400),
            observe_peer_remote_addrs(&self.ctx.endpoint, &beat.node_id),
        )
        .await
        .unwrap_or_else(|_| (vec!["iroh-quic:active".to_string()], false));
        beat.observed_remote_addrs = addrs;
        beat.tailnet_used = tailnet;

        let rtt_log = self
            .measured_rtt_ms
            .get(&beat.node)
            .map(|v| format!("{v:.1}ms"))
            .unwrap_or_else(|| "probing".to_string());
        println!(
            "[gossip-rx] node={} kind={} counter={} rtt={} addrs={:?} tailnet_used={}",
            beat.node,
            beat.kind,
            beat.counter,
            rtt_log,
            beat.observed_remote_addrs,
            beat.tailnet_used
        );

        let peer = beat.node.clone();
        let need_echo_reply = beat.rtt_probe_requested || !self.measured_rtt_ms.contains_key(&peer);
        // Stamp with our clock once admitted, so clock skew between hosts cannot cause
        // early TTL pruning.
        beat.updated_at_unix = now_unix();

        let _ = self.deploy_tx.send(PeerPrograms {
            node: beat.node.clone(),
            node_id: beat.node_id.clone(),
            processes: beat.processes.clone(),
        });
        let measured = self.measured_rtt_ms.clone();
        self.ctx.with_state(|st| {
            let first_seen = !st.nodes.iter().any(|n| n.node_id == beat.node_id);
            merge_beat(&mut st.nodes, beat);
            if let Some(me) = st.self_entry_mut() {
                me.peer_rtt_ms = measured;
                if let Some(geo) = cached_local_public_ip_geo() {
                    me.geo_location = Some(geo);
                }
            }
            let matrix = build_mesh_rtt_matrix(&st.nodes);
            place_nodes(&mut st.nodes, &matrix);
            st.mesh_rtt_ms = matrix;
            st.refresh_live_alerts();
            if first_seen {
                st.record_event(
                    "mesh.peer_joined",
                    "info",
                    format!("Discovered mesh peer '{peer}'"),
                );
                st.record_log(
                    "gossip-mesh",
                    "INFO",
                    "stdout",
                    format!("peer '{peer}' joined active gossip mesh (rtt={rtt_log})"),
                );
            }
        });

        let last_reply = self
            .last_echo_reply_mono_ms
            .get(&peer)
            .copied()
            .unwrap_or(0);
        if need_echo_reply && rx_mono.saturating_sub(last_reply) >= 1_500 {
            self.last_echo_reply_mono_ms.insert(peer.clone(), rx_mono);
            let mut echo = self.fresh_beat();
            echo.kind = "rtt_echo".to_string();
            echo.rtt_probe_requested = !self.measured_rtt_ms.contains_key(&peer);
            self.ctx.with_state(|st| {
                echo.processes = st.processes.clone();
                echo.service_endpoints = st.service_endpoints.clone();
            });
            self.broadcast(&echo, Duration::from_millis(400)).await;
        }
    }
}
