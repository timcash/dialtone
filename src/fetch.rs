//! Fetch: ask one node for one thing, over QUIC (ALPN `dialtone/fetch/1`).
//!
//! Gossip carries everything every node needs, and nothing else (see `wire.rs`). A node
//! fetches from a peer only when someone asks:
//!
//! - `details`    what beats leave out, for the HUD's focused node: its devices, full probe,
//!   processes with their status, services, rolling summary, store, stats and transport;
//! - `probe_blob` a fresh probe blob's ticket, for a full probe to pull (`probe.rs`).
//! - `release`    the signed manifest of the release the node runs, for a node that sees a
//!   newer version in its beats (`update.rs`).
//!
//! One request per stream. The allowlist hook admits the connection and the QUIC handshake
//! proves which node answers.

use std::sync::{Arc, Mutex};
use std::time::Duration;

use iroh::endpoint::Connection;
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::{Endpoint, EndpointId};
use serde::{Deserialize, Serialize};

use crate::api::NodeState;

pub const ALPN: &[u8] = b"dialtone/fetch/1";
const MAX_REQUEST_BYTES: usize = 64 << 10;
const MAX_ANSWER_BYTES: usize = 16 << 20;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "get", rename_all = "snake_case")]
pub enum Request {
    Details,
    ProbeBlob,
    Release,
}

impl Request {
    fn name(&self) -> &'static str {
        match self {
            Request::Details => "details",
            Request::ProbeBlob => "probe_blob",
            Request::Release => "release",
        }
    }
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

    async fn answer(&self, request: &[u8]) -> Result<Vec<u8>, String> {
        let request: Request =
            serde_json::from_slice(request).map_err(|e| format!("bad request: {e}"))?;
        let state = self
            .state
            .clone()
            .ok_or("this node has no state to share")?;
        let json = match request {
            Request::Details => lock(&state)?.details_json(),
            Request::Release => {
                let signed = crate::update::Paths::current()?
                    .running_manifest()
                    .ok_or("this node runs no signed release")?;
                serde_json::to_value(signed).map_err(|e| e.to_string())?
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
        while let Ok((mut send, mut recv)) = connection.accept_bi().await {
            let request = recv
                .read_to_end(MAX_REQUEST_BYTES)
                .await
                .map_err(AcceptError::from_err)?;
            let body = match self.answer(&request).await {
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

/// Fetch `peer`'s details (see `NodeState::details_json`); refuses an answer naming another
/// node.
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

/// Ask `peer` for the signed manifest of the release it runs (verify it before use).
pub async fn release(
    endpoint: &Endpoint,
    peer: EndpointId,
    timeout: Duration,
) -> Result<crate::update::SignedManifest, String> {
    let value = fetch_once(endpoint, peer, Request::Release, timeout).await?;
    serde_json::from_value(value).map_err(|e| format!("bad release answer: {e}"))
}
