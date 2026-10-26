//! Fetch: ask one node for one thing, over QUIC (ALPN `dialtone/fetch/1`).
//!
//! Gossip carries only small, fixed-size announcements that every node needs. Anything
//! larger, or wanted by only some nodes, is fetched from the node itself:
//!
//! - `inventory`  its programs, services, devices and details, when one of its beats
//!   announces an `inventory_version` the asker does not have;
//! - `probe_blob` a fresh probe blob's ticket, for a full probe to pull (`probe.rs`);
//! - `probe`      its latest probe report, with the per-peer results beats leave out;
//! - `rtt`        whether the asker is the answering node's gossip neighbor, timed for the
//!   round trip (see the Links actor).
//!
//! One request per stream; a connection may carry many (the Links actor keeps one open per
//! neighbor to time Rtts). The allowlist hook admits the connection, and the QUIC handshake
//! proves which node answers, so answers need no signature.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use iroh::endpoint::Connection;
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::{Endpoint, EndpointId};
use serde::{Deserialize, Serialize};

use crate::api::NodeState;
use crate::mesh::{Inventory, ProbeReport};

pub const ALPN: &[u8] = b"dialtone/fetch/1";
const MAX_REQUEST_BYTES: usize = 1024;
const MAX_ANSWER_BYTES: usize = 4 << 20;

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "get", rename_all = "snake_case")]
pub enum Request {
    Rtt,
    Inventory,
    ProbeBlob,
    Probe,
}

/// Answers fetches from the node state.
#[derive(Clone, Debug)]
pub struct FetchHandler {
    state: Option<Arc<Mutex<NodeState>>>,
}

impl FetchHandler {
    pub fn new(state: Option<Arc<Mutex<NodeState>>>) -> Self {
        FetchHandler { state }
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
                let inventory = lock(&state)?.inventory.clone();
                serde_json::to_value(inventory.ok_or("no inventory yet")?)
                    .map_err(|e| e.to_string())?
            }
            Request::Probe => {
                let probe = lock(&state)?.probe.clone();
                serde_json::to_value(probe.ok_or("no probe yet")?).map_err(|e| e.to_string())?
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
    tokio::time::timeout(timeout, exchange)
        .await
        .map_err(|_| format!("{request:?} timed out"))?
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

/// Fetch `peer`'s inventory; refuses one that names another node.
pub async fn inventory(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<Inventory, String> {
    let value = fetch_once(endpoint, peer, Request::Inventory, timeout).await?;
    let inventory: Inventory = serde_json::from_value(value).map_err(|e| e.to_string())?;
    if inventory.node_id != peer.to_string() {
        return Err(format!(
            "{} answered with the inventory of {}",
            peer.fmt_short(),
            inventory.node_id
        ));
    }
    Ok(inventory)
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

/// Fetch `peer`'s latest probe report.
pub async fn probe(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<ProbeReport, String> {
    let value = fetch_once(endpoint, peer, Request::Probe, timeout).await?;
    serde_json::from_value(value).map_err(|e| e.to_string())
}
