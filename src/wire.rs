//! The compact form a beat takes on the wire (postcard, inside the sealed envelope): what
//! every node needs to hear from every other node, and nothing else. A sealed beat is about
//! 250 bytes, so a mesh of 1,000 nodes beating every 120s costs each node about 4 KB/s.
//!
//! Everything else a HUD shows about a peer (its summary stats, store, full transport,
//! processes with their status) is fetched for the focused node only (`fetch.rs`).

use std::net::SocketAddr;

use serde::{Deserialize, Serialize};

use crate::mesh::{
    Beat, NeighborLink, NodeGeoLocation, ProbeCheck, ProbeReport, BEAT_SCHEMA,
    SUMMARY_INTERVAL_SECS,
};
use crate::stats::{NetStats, Stats};

/// Direct addresses a beat carries (the first ones the node advertises).
const MAX_IPS: usize = 2;
/// Neighbors a beat lists: iroh-gossip's active view is at most 5.
pub const MAX_NEIGHBORS: usize = 5;
/// Bytes of a NodeId that name a neighbor in a beat.
pub const NEIGHBOR_PREFIX_BYTES: usize = 4;
/// Bytes of the build digest a probe result carries.
const BUILD_PREFIX_BYTES: usize = 4;
/// Characters of a failing check's detail a beat carries.
const DETAIL_CHARS: usize = 40;
/// n0's public relays, named in beats by their region alone.
const N0_RELAY_PREFIX: &str = "https://";
const N0_RELAY_SUFFIX: &str = ".relay.n0.iroh.link./";
/// A load percentage that was not measured.
const UNKNOWN_PCT: u8 = u8::MAX;

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
struct WireBeat {
    node_id: [u8; 32],
    node: String,
    counter: u64,
    sent_unix_ms: u64,
    interval_secs: u8,
    heard: u32,
    /// A 32-bit hash, so fixed width (a varint would take five bytes).
    inventory_version: [u8; 4],
    os: u8,
    /// CPU, memory and disk percent (`UNKNOWN_PCT` when not measured).
    load: [u8; 3],
    /// The first interface's receive and send rates, KiB per second.
    net: Option<(u32, u32)>,
    /// Latitude and longitude in hundredths of a degree, big-endian i16s.
    geo: Option<[u8; 4]>,
    relay: Option<String>,
    ips: Vec<SocketAddr>,
    neighbors: Vec<WireNeighbor>,
    probe: Option<WireProbe>,
}

/// What later builds send after `WireBeat`. postcard ignores bytes after a value, so a node
/// that predates these fields reads the beat without them, and a beat from such a node has none.
#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
struct WireBeatExt {
    /// The dialtone release the node runs (major, minor, patch; all 0 when unknown).
    version: (u16, u16, u16),
    /// An index into `ARCH_NAMES`: with `os`, which release binary the node runs.
    arch: u8,
}

const ARCH_NAMES: [&str; 3] = ["", "x86_64", "aarch64"];

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
struct WireNeighbor {
    id: [u8; NEIGHBOR_PREFIX_BYTES],
    /// QUIC's Rtt estimate in whole milliseconds, at least 1 (0 when unknown).
    rtt: u16,
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
struct WireProbe {
    build: Vec<u8>,
    cycle: u64,
    checks: Vec<(String, bool, String)>,
}

const OS_NAMES: [&str; 4] = ["", "macos", "linux", "windows"];

fn pct(v: Option<f64>) -> u8 {
    v.map(|p| p.round().clamp(0.0, 100.0) as u8)
        .unwrap_or(UNKNOWN_PCT)
}

fn unpct(v: u8) -> Option<f64> {
    (v != UNKNOWN_PCT).then_some(v as f64)
}

/// `https://usw1-1.relay.n0.iroh.link./` -> `usw1-1`; any other relay URL as it is.
fn compact_relay(url: &str) -> String {
    match url
        .strip_prefix(N0_RELAY_PREFIX)
        .and_then(|r| r.strip_suffix(N0_RELAY_SUFFIX))
    {
        Some(region) if !region.contains(['/', ':']) => region.to_string(),
        _ => url.to_string(),
    }
}

fn expand_relay(relay: &str) -> String {
    if relay.contains("://") {
        relay.to_string()
    } else {
        format!("{N0_RELAY_PREFIX}{relay}{N0_RELAY_SUFFIX}")
    }
}

/// The NodeId prefix (lowercase hex) a beat names a neighbor by.
pub fn neighbor_prefix(node_id: &str) -> String {
    node_id
        .get(..NEIGHBOR_PREFIX_BYTES * 2)
        .unwrap_or(node_id)
        .to_string()
}

/// Encode a beat for the wire. Fields that are not in `WireBeat` are left behind.
pub fn encode_beat(beat: &Beat) -> Result<Vec<u8>, String> {
    let node_id = hex::decode(&beat.node_id)
        .ok()
        .and_then(|b| <[u8; 32]>::try_from(b).ok())
        .ok_or_else(|| format!("beat names an invalid node_id '{}'", beat.node_id))?;
    let os = OS_NAMES.iter().position(|o| *o == beat.os).unwrap_or(0) as u8;
    let net = beat.stats.interfaces.first().map(|i| {
        (
            (i.rx_bytes_per_sec / 1024) as u32,
            (i.tx_bytes_per_sec / 1024) as u32,
        )
    });
    let geo = beat.geo_location.as_ref().map(|g| {
        let lat = (g.lat * 100.0).round().clamp(-9000.0, 9000.0) as i16;
        let lng = (g.lng * 100.0).round().clamp(-18000.0, 18000.0) as i16;
        let mut out = [0u8; 4];
        out[..2].copy_from_slice(&lat.to_be_bytes());
        out[2..].copy_from_slice(&lng.to_be_bytes());
        out
    });
    let t = &beat.iroh_transport;
    let neighbors = beat
        .neighbors
        .iter()
        .take(MAX_NEIGHBORS)
        .filter_map(|n| {
            let id = hex::decode(&n.node_id_prefix).ok()?.try_into().ok()?;
            let rtt = match n.rtt_ms {
                ms if ms > 0.0 => ms.round().clamp(1.0, u16::MAX as f64) as u16,
                _ => 0,
            };
            Some(WireNeighbor { id, rtt })
        })
        .collect();
    let probe = beat.probe.as_ref().map(|p| {
        let digest = p.build.strip_prefix("sha256:").unwrap_or(&p.build);
        let build = digest
            .get(..BUILD_PREFIX_BYTES * 2)
            .and_then(|h| hex::decode(h).ok())
            .unwrap_or_default();
        WireProbe {
            build,
            cycle: p.cycle,
            checks: p
                .checks
                .iter()
                .map(|c| {
                    let detail = if c.ok {
                        String::new()
                    } else {
                        c.detail.chars().take(DETAIL_CHARS).collect()
                    };
                    (c.code.clone(), c.ok, detail)
                })
                .collect(),
        }
    });
    let wire = WireBeat {
        node_id,
        node: beat.node.clone(),
        counter: beat.counter,
        sent_unix_ms: beat.sent_unix_ms,
        interval_secs: beat.beat_interval_secs.min(u8::MAX as u64) as u8,
        heard: beat.heard,
        inventory_version: (beat.inventory_version.min(u32::MAX as u64) as u32).to_be_bytes(),
        os,
        load: [
            pct(beat.stats.cpu_percent),
            pct(beat.stats.mem_percent),
            pct(beat.stats.disk_percent),
        ],
        net,
        geo,
        relay: t.advertised_relay_urls.first().map(|r| compact_relay(r)),
        ips: t
            .advertised_ip_addrs
            .iter()
            .filter_map(|a| a.parse().ok())
            .take(MAX_IPS)
            .collect(),
        neighbors,
        probe,
    };
    let mut out = postcard::to_allocvec(&wire).map_err(|e| e.to_string())?;
    let (major, minor, patch) = crate::update::parse_version(&beat.version).unwrap_or_default();
    let ext = WireBeatExt {
        version: (major as u16, minor as u16, patch as u16),
        arch: ARCH_NAMES.iter().position(|a| *a == beat.arch).unwrap_or(0) as u8,
    };
    out.extend(postcard::to_allocvec(&ext).map_err(|e| e.to_string())?);
    Ok(out)
}

/// Decode a beat from the wire.
pub fn decode_beat(bytes: &[u8]) -> Result<Beat, String> {
    let (w, rest): (WireBeat, &[u8]) =
        postcard::take_from_bytes(bytes).map_err(|e| format!("invalid beat: {e}"))?;
    let ext: WireBeatExt = postcard::from_bytes(rest).unwrap_or_default();
    let stats = Stats {
        cpu_percent: unpct(w.load[0]),
        mem_percent: unpct(w.load[1]),
        disk_percent: unpct(w.load[2]),
        interfaces: w
            .net
            .map(|(rx, tx)| {
                vec![NetStats {
                    up: true,
                    rx_bytes_per_sec: rx as u64 * 1024,
                    tx_bytes_per_sec: tx as u64 * 1024,
                    ..Default::default()
                }]
            })
            .unwrap_or_default(),
        ..Default::default()
    };
    let probe = w.probe.map(|p| {
        let checks: Vec<ProbeCheck> = p
            .checks
            .into_iter()
            .map(|(code, ok, detail)| ProbeCheck {
                code,
                ok,
                detail,
                ms: None,
            })
            .collect();
        ProbeReport {
            build: hex::encode(&p.build),
            cycle: p.cycle,
            // The node probes every 30s and beats right after; the beat dates the result.
            updated_at_unix: w.sent_unix_ms / 1000,
            passed_count: checks.iter().filter(|c| c.ok).count() as u8,
            total_count: checks.len() as u8,
            checks,
            ..Default::default()
        }
    });
    let mut beat = Beat {
        schema: BEAT_SCHEMA.to_string(),
        node: w.node,
        node_id: hex::encode(w.node_id),
        ticket: String::new(),
        counter: w.counter,
        updated_at_unix: 0,
        kind: "regular".to_string(),
        beat_interval_secs: w.interval_secs as u64,
        summary_interval_secs: SUMMARY_INTERVAL_SECS,
        heard: w.heard,
        inventory_version: u32::from_be_bytes(w.inventory_version) as u64,
        peer_rtt_ms: Default::default(),
        neighbors: w
            .neighbors
            .into_iter()
            .map(|n| NeighborLink {
                node_id_prefix: hex::encode(n.id),
                rtt_ms: n.rtt as f64,
            })
            .collect(),
        summary_stats: None,
        geo_location: w.geo.map(|g| NodeGeoLocation {
            lat: i16::from_be_bytes([g[0], g[1]]) as f64 / 100.0,
            lng: i16::from_be_bytes([g[2], g[3]]) as f64 / 100.0,
            source: "public_ip_geo".to_string(),
            ..Default::default()
        }),
        probe,
        store: None,
        sent_unix_ms: w.sent_unix_ms,
        os: OS_NAMES
            .get(w.os as usize)
            .copied()
            .unwrap_or_default()
            .to_string(),
        arch: ARCH_NAMES
            .get(ext.arch as usize)
            .copied()
            .unwrap_or_default()
            .to_string(),
        role: String::new(),
        capabilities: Vec::new(),
        stats,
        service_endpoints: Vec::new(),
        processes: Vec::new(),
        iroh_transport: Default::default(),
        observed_remote_addrs: Vec::new(),
        tailnet_used: false,
        devices: Vec::new(),
        version: match ext.version {
            (0, 0, 0) => String::new(),
            (a, b, c) => format!("{a}.{b}.{c}"),
        },
    };
    beat.iroh_transport.advertised_relay_urls = w.relay.iter().map(|r| expand_relay(r)).collect();
    beat.iroh_transport.advertised_ip_addrs = w.ips.iter().map(|a| a.to_string()).collect();
    Ok(beat)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn n0_relays_are_named_by_region_and_others_kept_whole() {
        let n0 = "https://usw1-1.relay.n0.iroh.link./";
        assert_eq!(compact_relay(n0), "usw1-1");
        assert_eq!(expand_relay(&compact_relay(n0)), n0);
        let own = "https://relay.example.com./";
        assert_eq!(expand_relay(&compact_relay(own)), own);
    }

    /// Beats with and without the extension decode on both sides: an old node reads a new
    /// beat (it stops after `WireBeat`), and a new node reads an old one (no version).
    #[test]
    fn the_beat_extension_works_with_older_nodes_both_ways() {
        let wire = WireBeat {
            node_id: [1; 32],
            node: "n".into(),
            counter: 1,
            sent_unix_ms: 2,
            interval_secs: 5,
            heard: 0,
            inventory_version: [0; 4],
            os: 2,
            load: [UNKNOWN_PCT; 3],
            net: None,
            geo: None,
            relay: None,
            ips: Vec::new(),
            neighbors: Vec::new(),
            probe: None,
        };
        let old = postcard::to_allocvec(&wire).unwrap();
        let beat = decode_beat(&old).unwrap();
        assert_eq!((beat.version.as_str(), beat.arch.as_str()), ("", ""));
        let mut new = old.clone();
        let ext = WireBeatExt {
            version: (0, 5, 0),
            arch: 1,
        };
        new.extend(postcard::to_allocvec(&ext).unwrap());
        assert_eq!(postcard::from_bytes::<WireBeat>(&new).unwrap(), wire);
        let beat = decode_beat(&new).unwrap();
        assert_eq!(
            (beat.version.as_str(), beat.arch.as_str()),
            ("0.5.0", "x86_64")
        );
        assert_eq!(
            decode_beat(&encode_beat(&beat).unwrap()).unwrap().version,
            "0.5.0"
        );
    }

    #[test]
    fn load_percentages_round_and_unknowns_survive() {
        assert_eq!(pct(Some(41.6)), 42);
        assert_eq!(pct(Some(130.0)), 100);
        assert_eq!(unpct(pct(None)), None);
    }
}
