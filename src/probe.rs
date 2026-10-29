//! Probe: the running node testing itself.
//!
//! Every `PROBE_INTERVAL_SECS` the node checks the gossip mesh, which costs no traffic of
//! its own (it reads what gossip already brought):
//!
//! - `MSH`  every allowlisted peer is in the mesh (on an open channel, just the count);
//! - `NBR`  the node has at least two gossip neighbors (one in a two-node mesh);
//! - `BEAT` every peer's beats are fresh, and every peer hears as many nodes as this one.
//!
//! A full probe (`dialtone probe --all`, `POST /api/probe {"all": true}`) also tests the
//! node's other capabilities, which move data:
//!
//! - `STO`  a fresh blob is written to the store and read back intact;
//! - `PUL`  up to `PULL_SAMPLE` peers each write a fresh blob on request (`fetch.rs`), and
//!   it is pulled over QUIC and its SHA-256 verified;
//! - `PRC`  the probe program, staged from the store, is running under the supervisor;
//! - `SEC`  config.json holds no plaintext secret, only keys derived with argon2id-v2, and
//!   the node publishes itself to no public directory (Pkarr DNS, Mainline DHT).
//!
//! A compact report travels in the node's beats, so every HUD shows every node's results.
//! Probe blobs are forgotten once verified (pulled) or a few runs old (own), so the store
//! does not grow.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use crate::api::NodeState;
use crate::blob::{Blobs, Store};
use crate::mesh::{
    now_unix, MeshConfig, ProbeBeat, ProbeCheck, ProbePull, ProbeReport, DEFAULT_PEER_TTL_SECS,
    KDF_VERSION_V2,
};
use crate::service::ProcessSpec;

/// How often a running node probes the gossip mesh.
pub const PROBE_INTERVAL_SECS: u64 = 30;
/// How many peers a full probe pulls a blob from.
pub const PULL_SAMPLE: usize = 3;
/// Own probe blobs kept for peers to pull; peers ask for them at any time.
const OWN_BLOBS_KEPT: usize = 16;
/// The most peers a check's detail names.
const NAMED_MAX: usize = 5;

/// A peer as the probe sees it, taken from the node state.
#[derive(Clone, Debug)]
pub struct PeerView {
    pub node: String,
    pub node_id: String,
    pub age_secs: u64,
    /// The peer's beat is fresh up to this age (three of its beat intervals).
    pub fresh_secs: u64,
    /// How many other nodes the peer hears, from its latest beat.
    pub heard: u32,
    /// For a full probe: the ticket of a blob the peer just wrote, or why there is none.
    pub blob_ticket: Option<Result<String, String>>,
}

/// Everything a probe run needs, gathered under the node-state lock; the run itself does
/// its network and disk work without the lock.
pub struct ProbeInputs {
    /// Digest of the running dialtone binary.
    pub build: String,
    pub node: String,
    pub node_id: String,
    pub cycle: u64,
    /// Also test the store, pulls and programs.
    pub all: bool,
    /// Gossip neighbors on the beats topic.
    pub neighbors: usize,
    pub blobs: Blobs,
    pub store: Option<Store>,
    pub peers: Vec<PeerView>,
    pub allowlist: Vec<String>,
    pub probe_program: Option<ProcessSpec>,
    /// Own probe blobs too old to keep.
    pub expired_blobs: Vec<String>,
    /// How the endpoint looks up and publishes addresses (`IrohTransportDiagnostics`).
    pub lookup_modes: Vec<String>,
    /// The node's config.json, which `SEC` reads.
    pub config_path: std::path::PathBuf,
}

/// A request to the Probe actor, answered on `reply`.
#[derive(Debug)]
pub struct ProbeRequest {
    pub all: bool,
    pub reply: std::sync::mpsc::SyncSender<Result<ProbeReport, String>>,
}

/// Ask the Probe actor for a run and wait for its report (from a blocking thread).
pub fn request_probe(
    tx: &tokio::sync::mpsc::UnboundedSender<ProbeRequest>,
    all: bool,
) -> Result<ProbeReport, String> {
    let (reply, answer) = std::sync::mpsc::sync_channel(1);
    tx.send(ProbeRequest { all, reply })
        .map_err(|_| "the probe actor has stopped".to_string())?;
    answer
        .recv_timeout(Duration::from_secs(110))
        .map_err(|_| "the probe did not finish within 110s".to_string())?
}

/// Run one probe against the live node: gather, test without the lock, record. `tickets`
/// maps a peer's NodeId to the blob it offered for `PUL` (full probes only).
pub fn run_probe_now(
    state: &Arc<Mutex<NodeState>>,
    all: bool,
    tickets: std::collections::HashMap<String, Result<String, String>>,
) -> Result<ProbeReport, String> {
    let mut inputs = state
        .lock()
        .map_err(|_| "state poisoned".to_string())?
        .probe_inputs(all);
    for peer in &mut inputs.peers {
        peer.blob_ticket = tickets.get(&peer.node_id).cloned();
    }
    let report = run(inputs);
    state
        .lock()
        .map_err(|_| "state poisoned".to_string())?
        .record_probe(&report);
    Ok(report)
}

/// The probe itself. Blocking: call from a plain thread or `spawn_blocking`.
pub fn run(inputs: ProbeInputs) -> ProbeReport {
    let mut checks = Vec::new();

    // MSH: every allowlisted peer has a live entry in the mesh.
    let listed: Vec<&String> = inputs
        .allowlist
        .iter()
        .filter(|id| **id != inputs.node_id)
        .collect();
    let missing: Vec<String> = listed
        .iter()
        .filter(|id| {
            !inputs
                .peers
                .iter()
                .any(|p| &p.node_id == **id && p.age_secs <= DEFAULT_PEER_TTL_SECS)
        })
        .map(|id| id.chars().take(10).collect())
        .collect();
    checks.push(ProbeCheck {
        code: "MSH".to_string(),
        ok: missing.is_empty(),
        detail: if listed.is_empty() {
            let live = inputs
                .peers
                .iter()
                .filter(|p| p.node_id != inputs.node_id && p.age_secs <= DEFAULT_PEER_TTL_SECS)
                .count();
            format!("open channel, {live} peers in the mesh")
        } else if missing.is_empty() {
            format!(
                "{}/{} allowlisted peers in the mesh",
                listed.len(),
                listed.len()
            )
        } else {
            format!("missing {}", name_some(&missing))
        },
        ms: None,
    });

    // NBR: enough gossip neighbors that one lost link does not cut the node off.
    let live: Vec<&PeerView> = inputs
        .peers
        .iter()
        .filter(|p| p.age_secs <= p.fresh_secs)
        .collect();
    let needed = live.len().min(2);
    checks.push(ProbeCheck {
        code: "NBR".to_string(),
        ok: inputs.neighbors >= needed,
        detail: format!("{} gossip neighbor(s), {needed} needed", inputs.neighbors),
        ms: None,
    });

    // BEAT: beats arrive from every peer, and every peer hears the whole mesh.
    let mesh_size = live.len() + 1;
    let beats: Vec<ProbeBeat> = inputs
        .peers
        .iter()
        .map(|p| {
            let hears_all = p.heard as usize + 1 >= mesh_size;
            ProbeBeat {
                peer: p.node.clone(),
                age_secs: p.age_secs,
                hears_all,
                ok: p.age_secs <= p.fresh_secs && hears_all,
            }
        })
        .collect();
    let fresh = beats.iter().filter(|b| b.ok).count();
    let bad: Vec<String> = inputs
        .peers
        .iter()
        .zip(&beats)
        .filter(|(_, b)| !b.ok)
        .map(|(p, b)| {
            if p.age_secs > p.fresh_secs {
                format!("{} silent {}s", b.peer, b.age_secs)
            } else {
                format!("{} hears {}/{}", b.peer, p.heard, mesh_size - 1)
            }
        })
        .collect();
    checks.push(ProbeCheck {
        code: "BEAT".to_string(),
        ok: fresh == beats.len(),
        detail: if beats.is_empty() {
            "no peers yet".to_string()
        } else if bad.is_empty() {
            format!("{fresh}/{} peers beating, each hears all", beats.len())
        } else {
            name_some(&bad)
        },
        ms: None,
    });

    let mut report = ProbeReport {
        build: inputs.build.clone(),
        cycle: inputs.cycle,
        updated_at_unix: now_unix(),
        beats,
        ..Default::default()
    };
    if inputs.all {
        full_checks(&inputs, &mut checks, &mut report);
    }
    for digest in &inputs.expired_blobs {
        let _ = inputs.blobs.forget_blocking(digest);
    }
    report.passed_count = checks.iter().filter(|c| c.ok).count() as u8;
    report.total_count = checks.len() as u8;
    report.checks = checks;
    report
}

/// STO, PUL, PRC and SEC: the store, pulls from sampled peers, the probe program and the
/// node's secrets and privacy.
fn full_checks(inputs: &ProbeInputs, checks: &mut Vec<ProbeCheck>, report: &mut ProbeReport) {
    // STO: write a blob nobody has seen before and read it back.
    let payload = format!(
        "dialtone probe blob from {} (cycle {}, {} ms)",
        inputs.node,
        inputs.cycle,
        now_unix_ms()
    );
    let started = Instant::now();
    let stored = inputs
        .blobs
        .put_blocking(payload.as_bytes(), None)
        .and_then(|digest| {
            let back = inputs.blobs.get_blocking(&digest)?;
            if back == payload.as_bytes() {
                Ok(digest)
            } else {
                Err("read back different bytes".to_string())
            }
        });
    let sto_ms = ms_since(started);
    if let Ok(digest) = &stored {
        report.blob_digest = digest.clone();
    }
    checks.push(ProbeCheck {
        code: "STO".to_string(),
        ok: stored.is_ok(),
        detail: match &stored {
            Ok(digest) => format!("wrote and read back {}", short(digest)),
            Err(e) => e.clone(),
        },
        ms: Some(sto_ms),
    });

    // PUL: download each sampled peer's fresh blob over QUIC; the pull checks its SHA-256.
    for peer in &inputs.peers {
        let Some(offered) = &peer.blob_ticket else {
            continue;
        };
        let started = Instant::now();
        let result = match (offered, &inputs.store) {
            (Ok(ticket), Some(store)) => store.pull_blob(ticket, true),
            (Ok(_), None) => Err("no store".to_string()),
            (Err(e), _) => Err(e.clone()),
        };
        let ms = ms_since(started);
        report.pulls.push(match result {
            Ok(pulled) => {
                let _ = inputs.blobs.forget_blocking(&pulled.digest);
                ProbePull {
                    peer: peer.node.clone(),
                    ok: true,
                    ms,
                    bytes: pulled.bytes,
                    error: String::new(),
                }
            }
            Err(e) => ProbePull {
                peer: peer.node.clone(),
                ok: false,
                ms,
                bytes: 0,
                error: e,
            },
        });
    }
    let pulls = &report.pulls;
    let pulled = pulls.iter().filter(|p| p.ok).count();
    let failed: Vec<String> = pulls
        .iter()
        .filter(|p| !p.ok)
        .map(|p| p.peer.clone())
        .collect();
    checks.push(ProbeCheck {
        code: "PUL".to_string(),
        ok: failed.is_empty(),
        detail: if pulls.is_empty() {
            "no peers to pull from".to_string()
        } else if failed.is_empty() {
            format!("pulled {pulled}/{} sampled peer blobs", pulls.len())
        } else {
            format!("pull failed from {}", name_some(&failed))
        },
        ms: (!pulls.is_empty())
            .then(|| pulls.iter().map(|p| p.ms).sum::<f64>() / pulls.len() as f64),
    });

    // PRC: the probe program, staged from the store, runs under the supervisor.
    let program = inputs.probe_program.as_ref();
    let running =
        program.is_some_and(|p| p.status.as_deref() == Some("running") && p.pid.is_some());
    checks.push(ProbeCheck {
        code: "PRC".to_string(),
        ok: running,
        detail: match program {
            Some(p) if running => format!("{} running as pid {}", p.name, p.pid.unwrap_or(0)),
            Some(p) => format!(
                "{} is {}",
                p.name,
                p.status.as_deref().unwrap_or("not started")
            ),
            None => "probe program not configured".to_string(),
        },
        ms: None,
    });
    if let Some(p) = program {
        report.program_name = p.name.clone();
        report.program_digest = p.digest.clone();
        report.pid = p.pid;
        report.staged_path = p.path.clone();
    }

    // SEC: read from disk, where a leaked secret would be.
    let cfg = crate::mesh::load_mesh_config(&inputs.config_path);
    let issues = security_issues(&cfg, &inputs.lookup_modes);
    checks.push(ProbeCheck {
        code: "SEC".to_string(),
        ok: issues.is_empty(),
        detail: if issues.is_empty() {
            "no plaintext secret, argon2id-v2 keys, no public directory".to_string()
        } else {
            issues.join("; ")
        },
        ms: None,
    });
}

/// What is wrong with a node's secrets and privacy, if anything.
fn security_issues(cfg: &MeshConfig, lookup_modes: &[String]) -> Vec<String> {
    let mut issues = Vec::new();
    if cfg.secret_key.is_some() {
        issues.push("plaintext secret in config.json".to_string());
    }
    if let Some(keys) = &cfg.derived_keys {
        if keys.kdf_version != KDF_VERSION_V2 {
            issues.push(format!("keys derived with {:?}", keys.kdf_version));
        }
    }
    for mode in lookup_modes {
        if mode == "dns-pkarr" || mode == "mainline-dht" {
            issues.push(format!("publishes to {mode}"));
        }
    }
    issues
}

/// The first few names, then how many more.
fn name_some(names: &[String]) -> String {
    let mut text = names
        .iter()
        .take(NAMED_MAX)
        .cloned()
        .collect::<Vec<_>>()
        .join(", ");
    if names.len() > NAMED_MAX {
        text.push_str(&format!(" and {} more", names.len() - NAMED_MAX));
    }
    text
}

/// Own probe blobs past `OWN_BLOBS_KEPT`, oldest first, given the newest-last history.
pub fn expired(history: &[String]) -> Vec<String> {
    history[..history.len().saturating_sub(OWN_BLOBS_KEPT)].to_vec()
}

fn now_unix_ms() -> u128 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_millis())
        .unwrap_or(0)
}

fn ms_since(started: Instant) -> f64 {
    (started.elapsed().as_secs_f64() * 10_000.0).round() / 10.0
}

fn short(digest: &str) -> String {
    digest
        .trim_start_matches("sha256:")
        .chars()
        .take(12)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::blob::digest_of;
    use crate::node::{build_endpoint_builder, IrohTransportConfig};
    use iroh::address_lookup::memory::MemoryLookup;
    use iroh::protocol::Router;
    use iroh_blobs::BlobsProtocol;

    fn temp_blobs(name: &str) -> (std::path::PathBuf, Blobs) {
        let dir =
            std::env::temp_dir().join(format!("dialtone-probe-{name}-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        let blobs = Blobs::open_blocking(&dir).unwrap();
        (dir, blobs)
    }

    fn running_probe_program() -> ProcessSpec {
        ProcessSpec {
            name: "probe".to_string(),
            path: "/tmp/probe".to_string(),
            digest: digest_of(b"probe"),
            desired_state: "running".to_string(),
            args: Vec::new(),
            env: Vec::new(),
            cwd: None,
            startup_grace_secs: None,
            status: Some("running".to_string()),
            pid: Some(4242),
            restart_count: 0,
            service_unit: None,
            deploy_ticket: None,
            deployed_from: None,
        }
    }

    fn peer(node: &str, node_id: &str, age_secs: u64, heard: u32) -> PeerView {
        PeerView {
            node: node.to_string(),
            node_id: node_id.to_string(),
            age_secs,
            fresh_secs: 15,
            heard,
            blob_ticket: None,
        }
    }

    fn inputs(blobs: &Blobs, store: Option<Store>, peers: Vec<PeerView>) -> ProbeInputs {
        ProbeInputs {
            build: String::new(),
            node: "node-a".to_string(),
            node_id: "a".to_string(),
            cycle: 7,
            all: false,
            neighbors: 2,
            blobs: blobs.clone(),
            store,
            allowlist: peers.iter().map(|p| p.node_id.clone()).collect(),
            peers,
            probe_program: Some(running_probe_program()),
            expired_blobs: Vec::new(),
            lookup_modes: Vec::new(),
            config_path: std::env::temp_dir().join("dialtone-probe-test-no-config.json"),
        }
    }

    /// The default probe checks gossip only: membership, neighbors and beats both ways.
    #[test]
    fn gossip_probe_checks_membership_neighbors_and_beats() {
        let (dir, blobs) = temp_blobs("gossip");
        let healthy = vec![peer("node-b", "b", 2, 2), peer("node-c", "c", 4, 2)];
        let report = run(inputs(&blobs, None, healthy.clone()));
        assert_eq!(report.passed_count, 3, "{:#?}", report.checks);
        assert_eq!(report.total_count, 3);
        assert!(report.check("STO").is_none());

        // c is silent and b hears nobody; a has no neighbors; d is missing.
        let mut sick = inputs(
            &blobs,
            None,
            vec![peer("node-b", "b", 2, 0), peer("node-c", "c", 60, 2)],
        );
        sick.neighbors = 0;
        sick.allowlist.push("d".repeat(64));
        let report = run(sick);
        for code in ["MSH", "NBR", "BEAT"] {
            assert_eq!(report.check(code).map(|c| c.ok), Some(false), "{code}");
        }
        let beat = report.check("BEAT").unwrap();
        assert!(beat.detail.contains("node-b hears 0/1"), "{}", beat.detail);
        assert!(beat.detail.contains("node-c silent 60s"), "{}", beat.detail);

        blobs.close_blocking();
        let _ = std::fs::remove_dir_all(&dir);
    }

    /// A full probe: node B writes a fresh blob on request, node A pulls it over QUIC.
    #[test]
    fn full_probe_pulls_a_fresh_blob_from_a_peer() {
        let (dir_a, blobs_a) = temp_blobs("a");
        let (dir_b, blobs_b) = temp_blobs("b");
        let rt = tokio::runtime::Runtime::new().unwrap();
        let (store_a, store_b, router_b, id_b) = rt.block_on(async {
            let cfg = IrohTransportConfig::local_offline();
            let ep_a = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let ep_b = build_endpoint_builder(&cfg).unwrap().bind().await.unwrap();
            let router_b = Router::builder(ep_b.clone())
                .accept(iroh_blobs::ALPN, BlobsProtocol::new(blobs_b.api(), None))
                .spawn();
            let lookup = MemoryLookup::new();
            ep_a.address_lookup().unwrap().add(lookup.clone());
            let id_b = ep_b.id().to_string();
            let store_a = Store::new(ep_a, blobs_a.clone(), lookup, cfg.clone());
            let store_b = Store::new(ep_b, blobs_b.clone(), MemoryLookup::new(), cfg);
            (store_a, store_b, router_b, id_b)
        });

        // What `fetch::probe_blob` asks node B to do.
        let offered = store_b.share_blob(b"probe blob from node-b", None).unwrap();
        let mut b = peer("node-b", &id_b, 2, 1);
        b.blob_ticket = Some(Ok(offered.ticket_uri.clone()));
        let mut full = inputs(&blobs_a, Some(store_a.clone()), vec![b.clone()]);
        full.all = true;
        let report = run(full);
        assert_eq!(report.passed_count, 7, "{:#?}", report.checks);
        assert_eq!(report.total_count, 7);
        assert!(report.pulls[0].ok && report.pulls[0].bytes > 0);
        // The pulled blob was verified, then forgotten: probes do not fill the store.
        assert!(!blobs_a.has_blocking(&offered.digest));
        assert!(blobs_a.has_blocking(&report.blob_digest));

        // A peer that could not offer a blob, or a bad ticket, fails PUL; no program fails PRC.
        let mut refused = b.clone();
        refused.blob_ticket = Some(Err("no store".to_string()));
        let mut bad = b;
        bad.node = "node-c".to_string();
        bad.blob_ticket = Some(Ok(offered.ticket_uri.replace("sha256:", "sha256:0")));
        let mut failing = inputs(&blobs_a, Some(store_a), vec![refused, bad]);
        failing.all = true;
        failing.probe_program = None;
        failing.expired_blobs = vec![report.blob_digest.clone()];
        let report_fail = run(failing);
        for code in ["PUL", "PRC"] {
            assert_eq!(report_fail.check(code).map(|c| c.ok), Some(false), "{code}");
        }
        assert_eq!(report_fail.check("STO").map(|c| c.ok), Some(true));
        assert!(!blobs_a.has_blocking(&report.blob_digest));

        rt.block_on(async {
            let _ = router_b.shutdown().await;
        });
        blobs_a.close_blocking();
        blobs_b.close_blocking();
        let _ = std::fs::remove_dir_all(&dir_a);
        let _ = std::fs::remove_dir_all(&dir_b);
    }

    #[test]
    fn security_flags_plaintext_secrets_old_keys_and_public_directories() {
        let mut cfg = MeshConfig::default();
        assert!(security_issues(&cfg, &["relay".to_string()]).is_empty());
        cfg.secret_key = Some("dt-leaked".to_string());
        cfg.derived_keys = Some(crate::mesh::ChannelKeysHex {
            kdf_version: "argon2id-v1".to_string(),
            topic_id_hex: String::new(),
            auth_key_hex: String::new(),
            enc_key_hex: String::new(),
        });
        let modes = ["dns-pkarr".to_string(), "mainline-dht".to_string()];
        assert_eq!(security_issues(&cfg, &modes).len(), 4);
    }

    #[test]
    fn own_probe_blobs_older_than_the_previous_run_expire() {
        let h = |n: usize| (0..n).map(|i| i.to_string()).collect::<Vec<_>>();
        assert!(expired(&h(0)).is_empty());
        assert!(expired(&h(OWN_BLOBS_KEPT)).is_empty());
        assert_eq!(
            expired(&h(OWN_BLOBS_KEPT + 2)),
            vec!["0".to_string(), "1".to_string()]
        );
    }
}
