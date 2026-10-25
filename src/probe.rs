//! Probe: the running node testing itself, every `PROBE_INTERVAL_SECS`.
//!
//! Each run does real work against the live mesh and reports five checks:
//!
//! - `MSH`  every allowlisted peer is in the mesh;
//! - `STO`  a fresh blob is written to the store and read back intact;
//! - `PUL`  every peer's latest probe blob is pulled over QUIC and its SHA-256 verified;
//! - `BEAT` every peer's beats are fresh, and every peer hears this node's beats;
//! - `PRC`  the probe program, staged from the store, is running under the supervisor.
//!
//! The report travels in the node's beats, so every HUD shows every node's results and peers
//! learn the ticket of the blob to pull next. Probe blobs are forgotten once they are two runs
//! old (own) or verified (pulled), so the store does not grow.

use std::sync::{Arc, Mutex};
use std::time::Instant;

use crate::api::NodeState;
use crate::blob::{Blobs, Store};
use crate::mesh::{
    now_unix, ProbeBeat, ProbeCheck, ProbePull, ProbeReport, BEAT_INTERVAL_SECS,
    DEFAULT_PEER_TTL_SECS,
};
use crate::service::ProcessSpec;

/// How often a running node probes itself.
pub const PROBE_INTERVAL_SECS: u64 = 30;
/// A peer whose latest beat is older than this fails `BEAT`.
pub const BEAT_FRESH_SECS: u64 = 3 * BEAT_INTERVAL_SECS;
/// Own probe blobs kept for peers to pull (this run's and the previous one).
const OWN_BLOBS_KEPT: usize = 2;

/// A peer as the probe sees it, taken from the node state.
#[derive(Clone, Debug)]
pub struct PeerView {
    pub node: String,
    pub node_id: String,
    pub age_secs: u64,
    pub hears_us: bool,
    /// The peer's latest probe blob, if it has published one.
    pub blob_ticket: Option<String>,
}

/// Everything a probe run needs, gathered under the node-state lock; the run itself does
/// its network and disk work without the lock.
pub struct ProbeInputs {
    pub node: String,
    pub node_id: String,
    pub cycle: u64,
    pub blobs: Blobs,
    pub store: Option<Store>,
    pub peers: Vec<PeerView>,
    pub allowlist: Vec<String>,
    pub probe_program: Option<ProcessSpec>,
    /// Own probe blobs too old to keep.
    pub expired_blobs: Vec<String>,
}

/// Run one probe against the live node: gather, test without the lock, record.
pub fn run_probe_now(state: &Arc<Mutex<NodeState>>) -> Result<ProbeReport, String> {
    let inputs = state
        .lock()
        .map_err(|_| "state poisoned".to_string())?
        .probe_inputs();
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
            "no peers allowlisted".to_string()
        } else if missing.is_empty() {
            format!(
                "{}/{} allowlisted peers in the mesh",
                listed.len(),
                listed.len()
            )
        } else {
            format!("missing {}", missing.join(", "))
        },
        ms: None,
    });

    // STO: write a blob nobody has seen before, read it back, offer it to peers.
    let payload = format!(
        "dialtone probe blob from {} (cycle {}, {} ms)",
        inputs.node,
        inputs.cycle,
        now_unix_ms()
    );
    let started = Instant::now();
    let stored = match &inputs.store {
        Some(store) => store
            .share_blob(payload.as_bytes(), None)
            .map(|r| (r.digest, r.ticket_uri)),
        None => inputs
            .blobs
            .put_blocking(payload.as_bytes(), None)
            .map(|d| (d, String::new())),
    }
    .and_then(|(digest, ticket)| {
        let back = inputs.blobs.get_blocking(&digest)?;
        if back == payload.as_bytes() {
            Ok((digest, ticket))
        } else {
            Err("read back different bytes".to_string())
        }
    });
    let sto_ms = ms_since(started);
    let (blob_digest, blob_ticket) = match &stored {
        Ok((digest, ticket)) => (digest.clone(), ticket.clone()),
        Err(_) => (String::new(), String::new()),
    };
    checks.push(ProbeCheck {
        code: "STO".to_string(),
        ok: stored.is_ok(),
        detail: match &stored {
            Ok((digest, _)) => format!("wrote and read back {}", short(digest)),
            Err(e) => e.clone(),
        },
        ms: Some(sto_ms),
    });

    // PUL: download every peer's latest probe blob over QUIC; the pull checks its SHA-256.
    let mut pulls = Vec::new();
    let mut already_pulled = 0;
    for peer in &inputs.peers {
        let (Some(ticket), Some(store)) = (&peer.blob_ticket, &inputs.store) else {
            continue;
        };
        // A ticket already pulled in an earlier run would be served from local data, so it
        // proves nothing about the network; wait for the peer's next blob instead.
        let already_here = crate::blob::parse_blob_ticket_uri(ticket)
            .map(|(t, _)| inputs.blobs.has_data_blocking(t.hash()))
            .unwrap_or(false);
        if already_here {
            already_pulled += 1;
            continue;
        }
        let started = Instant::now();
        let result = store.pull_blob(ticket, true);
        let ms = ms_since(started);
        pulls.push(match result {
            Ok(report) => {
                let _ = inputs.blobs.forget_blocking(&report.digest);
                ProbePull {
                    peer: peer.node.clone(),
                    ok: true,
                    ms,
                    bytes: report.bytes,
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
    let pulled = pulls.iter().filter(|p| p.ok).count();
    checks.push(ProbeCheck {
        code: "PUL".to_string(),
        ok: pulled == pulls.len(),
        detail: if pulls.is_empty() && already_pulled > 0 {
            format!("no new blob from {already_pulled} peer(s) since the last run")
        } else if pulls.is_empty() {
            "no peer blobs to pull yet".to_string()
        } else {
            let failed: Vec<&str> = pulls
                .iter()
                .filter(|p| !p.ok)
                .map(|p| p.peer.as_str())
                .collect();
            if failed.is_empty() {
                format!("pulled {pulled}/{} peer blobs", pulls.len())
            } else {
                format!("pull failed from {}", failed.join(", "))
            }
        },
        ms: (!pulls.is_empty())
            .then(|| pulls.iter().map(|p| p.ms).sum::<f64>() / pulls.len() as f64),
    });

    // BEAT: beats arrive from every peer, and every peer hears ours.
    let beats: Vec<ProbeBeat> = inputs
        .peers
        .iter()
        .map(|p| ProbeBeat {
            peer: p.node.clone(),
            age_secs: p.age_secs,
            hears_us: p.hears_us,
            ok: p.age_secs <= BEAT_FRESH_SECS && p.hears_us,
        })
        .collect();
    let fresh = beats.iter().filter(|b| b.ok).count();
    checks.push(ProbeCheck {
        code: "BEAT".to_string(),
        ok: fresh == beats.len(),
        detail: if beats.is_empty() {
            "no peers yet".to_string()
        } else {
            let bad: Vec<String> = beats
                .iter()
                .filter(|b| !b.ok)
                .map(|b| {
                    if b.age_secs > BEAT_FRESH_SECS {
                        format!("{} silent {}s", b.peer, b.age_secs)
                    } else {
                        format!("{} does not hear us", b.peer)
                    }
                })
                .collect();
            if bad.is_empty() {
                format!("{fresh}/{} peers beating both ways", beats.len())
            } else {
                bad.join(", ")
            }
        },
        ms: None,
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

    for digest in &inputs.expired_blobs {
        let _ = inputs.blobs.forget_blocking(digest);
    }

    let passed_count = checks.iter().filter(|c| c.ok).count() as u8;
    ProbeReport {
        cycle: inputs.cycle,
        updated_at_unix: now_unix(),
        passed_count,
        total_count: checks.len() as u8,
        checks,
        blob_digest,
        blob_ticket,
        pulls,
        beats,
        program_name: program.map(|p| p.name.clone()).unwrap_or_default(),
        program_digest: program.map(|p| p.digest.clone()).unwrap_or_default(),
        pid: program.and_then(|p| p.pid),
        staged_path: program.map(|p| p.path.clone()).unwrap_or_default(),
    }
}

/// Own probe blobs past `OWN_BLOBS_KEPT`, oldest first, given the newest-last history.
pub fn expired(history: &[String]) -> Vec<String> {
    history[..history.len().saturating_sub(OWN_BLOBS_KEPT - 1)].to_vec()
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
        }
    }

    /// Two live nodes: each writes its probe blob, the other pulls it over QUIC.
    #[test]
    fn probe_pulls_a_peers_blob_and_checks_beats_both_ways() {
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

        // Node B's probe publishes a blob ticket.
        let report_b = run(ProbeInputs {
            node: "node-b".to_string(),
            node_id: id_b.clone(),
            cycle: 1,
            blobs: blobs_b.clone(),
            store: Some(store_b),
            peers: Vec::new(),
            allowlist: Vec::new(),
            probe_program: Some(running_probe_program()),
            expired_blobs: Vec::new(),
        });
        assert_eq!(report_b.check("STO").map(|c| c.ok), Some(true));
        assert!(report_b.blob_ticket.starts_with("iroh-blobs://ticket/"));

        // Node A pulls it, sees B's beats, and B hears A.
        let peer_b = PeerView {
            node: "node-b".to_string(),
            node_id: id_b.clone(),
            age_secs: 2,
            hears_us: true,
            blob_ticket: Some(report_b.blob_ticket.clone()),
        };
        let report_a = run(ProbeInputs {
            node: "node-a".to_string(),
            node_id: "a".to_string(),
            cycle: 7,
            blobs: blobs_a.clone(),
            store: Some(store_a.clone()),
            peers: vec![peer_b.clone()],
            allowlist: vec![id_b.clone()],
            probe_program: Some(running_probe_program()),
            expired_blobs: Vec::new(),
        });
        assert_eq!(report_a.passed_count, 5, "{:#?}", report_a.checks);
        assert_eq!(report_a.total_count, 5);
        assert!(report_a.pulls[0].ok && report_a.pulls[0].bytes > 0);
        // The pulled blob was verified, then forgotten: probes do not fill the store.
        assert!(!blobs_a.has_blocking(&report_b.blob_digest));
        assert!(blobs_a.has_blocking(&report_a.blob_digest));

        // A silent peer that does not hear us, a bad ticket and a missing allowlisted node
        // fail BEAT, PUL and MSH; an expired own blob is forgotten.
        let silent = PeerView {
            age_secs: 120,
            hears_us: false,
            blob_ticket: Some(report_b.blob_ticket.replace("sha256:", "sha256:0")),
            ..peer_b
        };
        let report_fail = run(ProbeInputs {
            node: "node-a".to_string(),
            node_id: "a".to_string(),
            cycle: 8,
            blobs: blobs_a.clone(),
            store: Some(store_a),
            peers: vec![silent],
            allowlist: vec![id_b, "c".repeat(64)],
            probe_program: None,
            expired_blobs: vec![report_a.blob_digest.clone()],
        });
        for code in ["MSH", "PUL", "BEAT", "PRC"] {
            assert_eq!(report_fail.check(code).map(|c| c.ok), Some(false), "{code}");
        }
        assert_eq!(report_fail.check("STO").map(|c| c.ok), Some(true));
        assert!(!blobs_a.has_blocking(&report_a.blob_digest));

        rt.block_on(async {
            let _ = router_b.shutdown().await;
        });
        blobs_a.close_blocking();
        blobs_b.close_blocking();
        let _ = std::fs::remove_dir_all(&dir_a);
        let _ = std::fs::remove_dir_all(&dir_b);
    }

    #[test]
    fn own_probe_blobs_older_than_the_previous_run_expire() {
        let h = |n: usize| (0..n).map(|i| i.to_string()).collect::<Vec<_>>();
        assert!(expired(&h(0)).is_empty());
        assert!(expired(&h(1)).is_empty());
        assert_eq!(expired(&h(3)), vec!["0".to_string(), "1".to_string()]);
    }
}
