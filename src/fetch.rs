//! Fetch: ask one node for one thing, over QUIC (ALPN `dialtone/fetch/1`).
//!
//! Gossip carries only small, fixed-size announcements that every node needs. Anything
//! larger, or wanted by only some nodes, is fetched:
//!
//! - `inventory`  the node's own inventory, as the signed frame it broadcasts, when one of
//!   its beats announces an `inventory_version` the asker does not have;
//! - `catch_up`   from a neighbor, every beat and inventory frame the asker lacks, given a
//!   digest of what it has (Scuttlebutt-style anti-entropy; see the Mesh actor);
//! - `details`    the node's devices and full probe report, for the HUD's focused node;
//! - `probe_blob` a fresh probe blob's ticket, for a full probe to pull (`probe.rs`);
//! - `rtt`        whether the asker is the answering node's gossip neighbor, timed for the
//!   round trip (see the Links actor).
//!
//! One request per stream; a connection may carry many (the Links actor keeps one open per
//! neighbor to time Rtts). The allowlist hook admits the connection and the QUIC handshake
//! proves which node answers; frames relayed for other nodes stay signed by their authors.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use iroh::endpoint::Connection;
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::{Endpoint, EndpointId};
use serde::{Deserialize, Serialize};

use crate::api::NodeState;
use crate::mesh::{now_unix, CatchUpHave, DEFAULT_PEER_TTL_SECS};

pub const ALPN: &[u8] = b"dialtone/fetch/1";
/// A catch-up digest names every node the asker knows (about 110 bytes each).
const MAX_REQUEST_BYTES: usize = 1 << 20;
const MAX_ANSWER_BYTES: usize = 16 << 20;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "get", rename_all = "snake_case")]
pub enum Request {
    Rtt,
    Inventory,
    CatchUp { have: CatchUpHave },
    Details,
    ProbeBlob,
}

impl Request {
    fn name(&self) -> &'static str {
        match self {
            Request::Rtt => "rtt",
            Request::Inventory => "inventory",
            Request::CatchUp { .. } => "catch_up",
            Request::Details => "details",
            Request::ProbeBlob => "probe_blob",
        }
    }
}

/// What a `catch_up` brings back: sealed frames, each beat with its age in seconds at the
/// answering node.
#[derive(Debug, Default)]
pub struct CatchUp {
    pub beats: Vec<(u64, Vec<u8>)>,
    pub inventories: Vec<Vec<u8>>,
}

/// Answers fetches from the node state.
#[derive(Clone, Debug)]
pub struct FetchHandler {
    state: Option<Arc<Mutex<NodeState>>>,
    node_id: String,
}

impl FetchHandler {
    pub fn new(state: Option<Arc<Mutex<NodeState>>>, node_id: String) -> Self {
        FetchHandler { state, node_id }
    }

    async fn answer(&self, asker: EndpointId, request: &[u8]) -> Result<Vec<u8>, String> {
        let request: Request =
            serde_json::from_slice(request).map_err(|e| format!("bad request: {e}"))?;
        let state = self
            .state
            .clone()
            .ok_or("this node has no state to share")?;
        let json = match request {
            Request::Rtt => {
                let neighbor = lock(&state)?.gossip_neighbors.contains(&asker.to_string());
                serde_json::json!({ "neighbor": neighbor })
            }
            Request::Inventory => {
                let frames = lock(&state)?.frames.clone();
                let frames = frames.lock().map_err(|_| "frames poisoned".to_string())?;
                let (frame, _) = frames
                    .nodes
                    .get(&self.node_id)
                    .and_then(|f| f.inventory.as_ref())
                    .ok_or("no inventory yet")?;
                serde_json::json!({ "frame": crate::base64url_encode(frame) })
            }
            Request::CatchUp { have } => {
                let frames = lock(&state)?.frames.clone();
                let frames = frames.lock().map_err(|_| "frames poisoned".to_string())?;
                let (beats, inventories) =
                    frames.deltas(&asker.to_string(), &have, now_unix(), DEFAULT_PEER_TTL_SECS);
                serde_json::json!({
                    "beats": beats
                        .iter()
                        .map(|(age, f)| serde_json::json!({ "age_secs": age, "frame": crate::base64url_encode(f) }))
                        .collect::<Vec<_>>(),
                    "inventories": inventories
                        .iter()
                        .map(|f| crate::base64url_encode(f))
                        .collect::<Vec<_>>(),
                })
            }
            Request::Details => {
                let st = lock(&state)?;
                serde_json::json!({
                    "node_id": self.node_id,
                    "devices": st.devices,
                    "probe": st.probe,
                })
            }
            Request::ProbeBlob => {
                // Writing to the store blocks; keep it off the async runtime.
                let ticket = tokio::task::spawn_blocking(move || share_probe_blob(&state))
                    .await
                    .map_err(|_| "probe blob task failed".to_string())??;
                serde_json::json!({ "ticket": ticket })
            }
        };
        serde_json::to_vec(&json).map_err(|e| e.to_string())
    }
}

fn lock(state: &Arc<Mutex<NodeState>>) -> Result<std::sync::MutexGuard<'_, NodeState>, String> {
    state.lock().map_err(|_| "state poisoned".to_string())
}

/// Write a fresh probe blob and return its ticket. The node forgets it a few probes later
/// (`probe::expired`), so serving pulls does not grow the store.
fn share_probe_blob(state: &Arc<Mutex<NodeState>>) -> Result<String, String> {
    let (store, node) = {
        let st = state.lock().map_err(|_| "state poisoned".to_string())?;
        (st.store.clone().ok_or("no store")?, st.node.clone())
    };
    let payload = format!(
        "dialtone probe blob from {node} ({} ms)",
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_millis())
            .unwrap_or(0)
    );
    let report = store.share_blob(payload.as_bytes(), None)?;
    if let Ok(mut st) = state.lock() {
        st.probe_blobs.push(report.digest);
    }
    Ok(report.ticket_uri)
}

impl ProtocolHandler for FetchHandler {
    async fn accept(&self, connection: Connection) -> Result<(), AcceptError> {
        let asker = connection.remote_id();
        while let Ok((mut send, mut recv)) = connection.accept_bi().await {
            let request = recv
                .read_to_end(MAX_REQUEST_BYTES)
                .await
                .map_err(AcceptError::from_err)?;
            let body = match self.answer(asker, &request).await {
                Ok(body) => body,
                Err(err) => {
                    serde_json::to_vec(&serde_json::json!({ "error": err })).unwrap_or_default()
                }
            };
            send.write_all(&body).await.map_err(AcceptError::from_err)?;
            send.finish().map_err(AcceptError::from_err)?;
        }
        Ok(())
    }
}

/// Connect to `peer` for fetches; the connection can carry many requests.
pub async fn connect(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<Connection, String> {
    tokio::time::timeout(timeout, endpoint.connect(peer, ALPN))
        .await
        .map_err(|_| format!("connect to {} timed out", peer.fmt_short()))?
        .map_err(|e| format!("connect to {}: {e}", peer.fmt_short()))
}

/// Send one request on an open connection and read the answer.
pub async fn request(
    conn: &Connection,
    request: Request,
    timeout: Duration,
) -> Result<serde_json::Value, String> {
    let exchange = async {
        let (mut send, mut recv) = conn.open_bi().await.map_err(|e| e.to_string())?;
        let body = serde_json::to_vec(&request).map_err(|e| e.to_string())?;
        send.write_all(&body).await.map_err(|e| e.to_string())?;
        send.finish().map_err(|e| e.to_string())?;
        let answer = recv
            .read_to_end(MAX_ANSWER_BYTES)
            .await
            .map_err(|e| e.to_string())?;
        let value: serde_json::Value =
            serde_json::from_slice(&answer).map_err(|e| format!("bad answer: {e}"))?;
        match value.get("error").and_then(|e| e.as_str()) {
            Some(err) => Err(err.to_string()),
            None => Ok(value),
        }
    };
    let name = request.name();
    tokio::time::timeout(timeout, exchange)
        .await
        .map_err(|_| format!("{name} timed out"))?
}

/// Time an `rtt` request on an open connection: the Rtt in milliseconds, and whether the
/// peer counts this node as its gossip neighbor.
pub async fn rtt(conn: &Connection, timeout: Duration) -> Result<(f64, bool), String> {
    let started = Instant::now();
    let answer = request(conn, Request::Rtt, timeout).await?;
    let ms = (started.elapsed().as_secs_f64() * 10_000.0).round() / 10.0;
    let neighbor = answer
        .get("neighbor")
        .and_then(|n| n.as_bool())
        .unwrap_or(true);
    Ok((ms, neighbor))
}

/// Connect, ask once, close.
async fn fetch_once(
    endpoint: &Endpoint,
    peer: EndpointId,
    what: Request,
    timeout: Duration,
) -> Result<serde_json::Value, String> {
    let conn = connect(endpoint, peer, timeout).await?;
    let answer = request(&conn, what, timeout).await;
    conn.close(0u32.into(), b"done");
    answer
}

/// Fetch `peer`'s inventory: the sealed frame it broadcasts, to be opened like a gossip
/// message (which checks its author's signature).
pub async fn inventory(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<Vec<u8>, String> {
    let value = fetch_once(endpoint, peer, Request::Inventory, timeout).await?;
    let frame = value
        .get("frame")
        .and_then(|f| f.as_str())
        .ok_or("answer has no frame")?;
    crate::base64_decode(frame)
}

/// Ask a neighbor for every frame this node lacks, given what it `have`s.
pub async fn catch_up(
    endpoint: &Endpoint,
    peer: EndpointId,
    have: CatchUpHave,
    timeout: Duration,
) -> Result<CatchUp, String> {
    let value = fetch_once(endpoint, peer, Request::CatchUp { have }, timeout).await?;
    let mut out = CatchUp::default();
    for beat in value
        .get("beats")
        .and_then(|b| b.as_array())
        .into_iter()
        .flatten()
    {
        let age = beat.get("age_secs").and_then(|a| a.as_u64()).unwrap_or(0);
        if let Some(frame) = beat.get("frame").and_then(|f| f.as_str()) {
            out.beats.push((age, crate::base64_decode(frame)?));
        }
    }
    for frame in value
        .get("inventories")
        .and_then(|i| i.as_array())
        .into_iter()
        .flatten()
        .filter_map(|f| f.as_str())
    {
        out.inventories.push(crate::base64_decode(frame)?);
    }
    Ok(out)
}

/// Fetch `peer`'s devices and full probe report; refuses an answer naming another node.
pub async fn details(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<serde_json::Value, String> {
    let value = fetch_once(endpoint, peer, Request::Details, timeout).await?;
    if value.get("node_id").and_then(|n| n.as_str()) != Some(peer.to_string().as_str()) {
        return Err(format!("{} answered for another node", peer.fmt_short()));
    }
    Ok(value)
}

/// Ask `peer` for a fresh probe blob; returns its ticket.
pub async fn probe_blob(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<String, String> {
    let value = fetch_once(endpoint, peer, Request::ProbeBlob, timeout).await?;
    value
        .get("ticket")
        .and_then(|t| t.as_str())
        .map(str::to_string)
        .ok_or_else(|| "answer has no ticket".to_string())
}
