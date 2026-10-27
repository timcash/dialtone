//! Links: a direct QUIC connection from one node to another for live media, over ALPN
//! `dialtone/link/1`. Gossip stays small; a link carries the heavy, latency-sensitive
//! traffic between exactly two nodes, and only while someone has it open.
//!
//! - **Export** (on the node with the source): `dialtone link export cam --udp 127.0.0.1:5000`
//!   receives UDP packets on that address (point ffmpeg at it), or, with `--tcp`, connects to
//!   a TCP service there for each link. Nothing is sent until a peer imports it.
//! - **Import** (on the viewer): `dialtone link import <node>/cam --udp 127.0.0.1:6000`
//!   delivers the packets to that address (point ffplay at it), or, with `--tcp`, listens
//!   there and carries each TCP connection over its own QUIC stream.
//!
//! UDP packets travel as QUIC datagrams: unreliable and unordered, so a late packet is
//! dropped instead of holding up the next one. Packets larger than the path allows are split
//! into parts (4-byte header: sequence, part, parts); a lost part drops its packet. Replies
//! (RTCP, say) travel back the same way. Links reuse iroh's hole punching: the path is
//! direct when the two nodes can reach each other, and falls back to the relay (flagged
//! `relay` in `GET /api/links`) when they cannot. The allowlist hook admits the connection.

use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use iroh::endpoint::{Connection, RecvStream, SendStream};
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::{Endpoint, EndpointId};
use serde::{Deserialize, Serialize};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::{TcpListener, TcpStream, UdpSocket};
use tokio_util::sync::CancellationToken;

pub const ALPN: &[u8] = b"dialtone/link/1";
/// The largest handshake line either side sends.
const MAX_LINE_BYTES: usize = 4096;
/// Bytes before each datagram's payload: sequence (2), part (1), parts (1).
const PART_HEADER: usize = 4;
/// The largest UDP packet a link carries.
const MAX_PACKET: usize = 65_507;
/// How long an import waits to connect, and between tries after a failure.
const CONNECT_TIMEOUT: Duration = Duration::from_secs(10);
const RETRY_EVERY: Duration = Duration::from_secs(3);
/// How long a socket loop pauses after an error, so a failing socket never spins.
const ERROR_PAUSE: Duration = Duration::from_millis(20);

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum LinkKind {
    Udp,
    Tcp,
}

/// What an importer asks for, sent as one JSON line on the first stream.
#[derive(Debug, Serialize, Deserialize)]
struct Hello {
    name: String,
    kind: LinkKind,
}

/// Packets and bytes through one link, both ways.
#[derive(Debug, Default)]
pub struct Counters {
    packets_in: AtomicU64,
    packets_out: AtomicU64,
    bytes_in: AtomicU64,
    bytes_out: AtomicU64,
    drops: AtomicU64,
}

impl Counters {
    fn inbound(&self, bytes: usize) {
        self.packets_in.fetch_add(1, Ordering::Relaxed);
        self.bytes_in.fetch_add(bytes as u64, Ordering::Relaxed);
    }

    fn outbound(&self, bytes: usize) {
        self.packets_out.fetch_add(1, Ordering::Relaxed);
        self.bytes_out.fetch_add(bytes as u64, Ordering::Relaxed);
    }

    fn drop_one(&self) {
        self.drops.fetch_add(1, Ordering::Relaxed);
    }

    fn json(&self) -> serde_json::Value {
        serde_json::json!({
            "packets_in": self.packets_in.load(Ordering::Relaxed),
            "packets_out": self.packets_out.load(Ordering::Relaxed),
            "bytes_in": self.bytes_in.load(Ordering::Relaxed),
            "bytes_out": self.bytes_out.load(Ordering::Relaxed),
            "drops": self.drops.load(Ordering::Relaxed),
        })
    }
}

/// How a link's connection is reached right now: its selected path and QUIC's Rtt.
fn path_json(conn: &Connection) -> serde_json::Value {
    let paths = conn.paths();
    match paths.iter().find(|p| p.is_selected()) {
        Some(p) => serde_json::json!({
            "path": if p.is_ip() { "direct" } else { "relay" },
            "rtt_ms": (p.rtt().as_secs_f64() * 10_000.0).round() / 10.0,
        }),
        None => serde_json::json!({ "path": "", "rtt_ms": null }),
    }
}

/// A peer connected to an export.
struct Viewer {
    node_id: EndpointId,
    conn: Connection,
}

/// One exported source.
struct Export {
    kind: LinkKind,
    addr: SocketAddr,
    /// UDP: the socket the source sends to, and where its packets come from (replies go
    /// back there).
    udp: Option<Arc<UdpSocket>>,
    source: Arc<Mutex<Option<SocketAddr>>>,
    viewers: Arc<Mutex<Vec<Viewer>>>,
    counters: Arc<Counters>,
    cancel: CancellationToken,
}

/// One import: where it delivers, and how it is doing.
struct Import {
    node: String,
    node_id: EndpointId,
    name: String,
    kind: LinkKind,
    addr: SocketAddr,
    conn: Arc<Mutex<Option<Connection>>>,
    error: Arc<Mutex<String>>,
    counters: Arc<Counters>,
    cancel: CancellationToken,
}

#[derive(Default)]
struct Inner {
    exports: HashMap<String, Export>,
    imports: HashMap<String, Import>,
}

/// This node's exports and imports. Cheap to clone; registered on the Router as the
/// `dialtone/link/1` handler and kept in the node state for the API.
#[derive(Clone)]
pub struct Links {
    endpoint: Endpoint,
    runtime: tokio::runtime::Handle,
    inner: Arc<Mutex<Inner>>,
}

impl std::fmt::Debug for Links {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("Links").finish_non_exhaustive()
    }
}

impl Links {
    /// Must be called inside the node's tokio runtime.
    pub fn new(endpoint: Endpoint) -> Self {
        Links {
            endpoint,
            runtime: tokio::runtime::Handle::current(),
            inner: Arc::default(),
        }
    }

    fn lock(&self) -> std::sync::MutexGuard<'_, Inner> {
        self.inner.lock().unwrap_or_else(|e| e.into_inner())
    }

    /// Offer `name` to allowlisted peers. UDP: bind `addr` and forward what arrives there.
    /// TCP: connect to `addr` for each link.
    pub fn export(&self, name: &str, kind: LinkKind, addr: SocketAddr) -> Result<(), String> {
        if name.is_empty() || name.contains('/') {
            return Err("an export needs a name without '/'".to_string());
        }
        self.close(name);
        let cancel = CancellationToken::new();
        let counters = Arc::new(Counters::default());
        let viewers: Arc<Mutex<Vec<Viewer>>> = Arc::default();
        let source: Arc<Mutex<Option<SocketAddr>>> = Arc::default();
        let udp = match kind {
            LinkKind::Udp => {
                let sock = self
                    .runtime
                    .block_on(UdpSocket::bind(addr))
                    .map_err(|e| format!("bind udp {addr}: {e}"))?;
                let sock = Arc::new(sock);
                self.runtime.spawn(forward_source(
                    sock.clone(),
                    source.clone(),
                    viewers.clone(),
                    counters.clone(),
                    cancel.clone(),
                ));
                Some(sock)
            }
            LinkKind::Tcp => None,
        };
        self.lock().exports.insert(
            name.to_string(),
            Export {
                kind,
                addr,
                udp,
                source,
                viewers,
                counters,
                cancel,
            },
        );
        Ok(())
    }

    /// Open `name` on `node_id` and deliver it to `addr`; returns the import's id
    /// (`<node>/<name>`). The link reconnects until it is closed.
    pub fn import(
        &self,
        node: &str,
        node_id: EndpointId,
        name: &str,
        kind: LinkKind,
        addr: SocketAddr,
    ) -> Result<String, String> {
        let id = format!("{node}/{name}");
        self.close(&id);
        let cancel = CancellationToken::new();
        let import = Import {
            node: node.to_string(),
            node_id,
            name: name.to_string(),
            kind,
            addr,
            conn: Arc::default(),
            error: Arc::default(),
            counters: Arc::new(Counters::default()),
            cancel: cancel.clone(),
        };
        let listener = match kind {
            LinkKind::Tcp => Some(
                self.runtime
                    .block_on(TcpListener::bind(addr))
                    .map_err(|e| format!("listen tcp {addr}: {e}"))?,
            ),
            LinkKind::Udp => None,
        };
        let run = run_import(
            self.endpoint.clone(),
            node_id,
            name.to_string(),
            kind,
            addr,
            listener,
            import.conn.clone(),
            import.error.clone(),
            import.counters.clone(),
            cancel,
        );
        self.lock().imports.insert(id.clone(), import);
        self.runtime.spawn(run);
        Ok(id)
    }

    /// Close an export (by name) or an import (by `<node>/<name>`). Returns whether one was
    /// open.
    pub fn close(&self, id: &str) -> bool {
        let mut inner = self.lock();
        if let Some(export) = inner.exports.remove(id) {
            export.cancel.cancel();
            for v in export
                .viewers
                .lock()
                .unwrap_or_else(|e| e.into_inner())
                .drain(..)
            {
                v.conn.close(0u32.into(), b"export closed");
            }
            return true;
        }
        if let Some(import) = inner.imports.remove(id) {
            import.cancel.cancel();
            if let Some(conn) = import.conn.lock().unwrap_or_else(|e| e.into_inner()).take() {
                conn.close(0u32.into(), b"import closed");
            }
            return true;
        }
        false
    }

    /// Every export and import with its path, Rtt and counters (`GET /api/links`).
    pub fn status(&self) -> serde_json::Value {
        let inner = self.lock();
        let exports: Vec<_> = inner
            .exports
            .iter()
            .map(|(name, e)| {
                let viewers: Vec<_> = e
                    .viewers
                    .lock()
                    .unwrap_or_else(|e| e.into_inner())
                    .iter()
                    .map(|v| {
                        let mut j = path_json(&v.conn);
                        j["node_id"] = v.node_id.to_string().into();
                        j
                    })
                    .collect();
                serde_json::json!({
                    "name": name,
                    "kind": e.kind,
                    "addr": e.addr.to_string(),
                    "viewers": viewers,
                    "counters": e.counters.json(),
                })
            })
            .collect();
        let imports: Vec<_> = inner
            .imports
            .iter()
            .map(|(id, i)| {
                let conn = i.conn.lock().unwrap_or_else(|e| e.into_inner()).clone();
                let mut j = match &conn {
                    Some(c) => path_json(c),
                    None => serde_json::json!({ "path": "", "rtt_ms": null }),
                };
                j["id"] = id.clone().into();
                j["node"] = i.node.clone().into();
                j["node_id"] = i.node_id.to_string().into();
                j["name"] = i.name.clone().into();
                j["kind"] = serde_json::to_value(i.kind).unwrap_or_default();
                j["addr"] = i.addr.to_string().into();
                j["state"] = if conn.is_some() { "up" } else { "connecting" }.into();
                j["error"] = i
                    .error
                    .lock()
                    .unwrap_or_else(|e| e.into_inner())
                    .clone()
                    .into();
                j["counters"] = i.counters.json();
                j
            })
            .collect();
        serde_json::json!({ "exports": exports, "imports": imports })
    }
}

impl ProtocolHandler for Links {
    async fn accept(&self, connection: Connection) -> Result<(), AcceptError> {
        let viewer = connection.remote_id();
        while let Ok((mut send, mut recv)) = connection.accept_bi().await {
            let hello: Result<Hello, String> = read_line(&mut recv)
                .await
                .and_then(|l| serde_json::from_slice(&l).map_err(|e| e.to_string()));
            let found = hello.and_then(|h| {
                let inner = self.lock();
                let e = inner
                    .exports
                    .get(&h.name)
                    .filter(|e| e.kind == h.kind)
                    .ok_or_else(|| format!("no {:?} export named '{}'", h.kind, h.name))?;
                Ok((
                    e.kind,
                    e.addr,
                    e.udp.clone(),
                    e.source.clone(),
                    e.viewers.clone(),
                    e.counters.clone(),
                    e.cancel.clone(),
                ))
            });
            let (kind, addr, udp, source, viewers, counters, cancel) = match found {
                Ok(f) => f,
                Err(err) => {
                    let _ = write_line(&mut send, &serde_json::json!({ "error": err })).await;
                    continue;
                }
            };
            match kind {
                LinkKind::Udp => {
                    let Some(sock) = udp else { continue };
                    if write_line(&mut send, &serde_json::json!({ "ok": true }))
                        .await
                        .is_err()
                    {
                        continue;
                    }
                    viewers
                        .lock()
                        .unwrap_or_else(|e| e.into_inner())
                        .push(Viewer {
                            node_id: viewer,
                            conn: connection.clone(),
                        });
                    // The viewer's replies go back to the source.
                    let conn = connection.clone();
                    tokio::spawn(async move {
                        let mut parts = Reassembly::default();
                        loop {
                            let datagram = tokio::select! {
                                _ = cancel.cancelled() => break,
                                d = conn.read_datagram() => match d {
                                    Ok(d) => d,
                                    Err(_) => break,
                                },
                            };
                            let Some(packet) = parts.add(&datagram, &counters) else {
                                continue;
                            };
                            let to = *source.lock().unwrap_or_else(|e| e.into_inner());
                            if let Some(to) = to {
                                counters.inbound(packet.len());
                                let _ = sock.send_to(&packet, to).await;
                            }
                        }
                        viewers
                            .lock()
                            .unwrap_or_else(|e| e.into_inner())
                            .retain(|v| v.conn.stable_id() != conn.stable_id());
                        drop(send);
                    });
                }
                LinkKind::Tcp => {
                    let counters = counters.clone();
                    tokio::spawn(async move {
                        match TcpStream::connect(addr).await {
                            Ok(tcp) => {
                                if write_line(&mut send, &serde_json::json!({ "ok": true }))
                                    .await
                                    .is_ok()
                                {
                                    splice(tcp, send, recv, &counters, cancel).await;
                                }
                            }
                            Err(e) => {
                                let _ = write_line(
                                    &mut send,
                                    &serde_json::json!({ "error": format!("connect tcp {addr}: {e}") }),
                                )
                                .await;
                            }
                        }
                    });
                }
            }
        }
        Ok(())
    }
}

/// UDP export: forward every packet the source sends to every viewer, as datagrams.
async fn forward_source(
    sock: Arc<UdpSocket>,
    source: Arc<Mutex<Option<SocketAddr>>>,
    viewers: Arc<Mutex<Vec<Viewer>>>,
    counters: Arc<Counters>,
    cancel: CancellationToken,
) {
    let mut buf = vec![0u8; MAX_PACKET];
    let mut seq: u16 = 0;
    loop {
        let (len, from) = tokio::select! {
            _ = cancel.cancelled() => break,
            r = sock.recv_from(&mut buf) => match r {
                Ok(r) => r,
                // Windows reports an ICMP "port unreachable" as a failed receive; never spin.
                Err(_) => {
                    tokio::time::sleep(ERROR_PAUSE).await;
                    continue;
                }
            },
        };
        *source.lock().unwrap_or_else(|e| e.into_inner()) = Some(from);
        let conns: Vec<Connection> = viewers
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .iter()
            .map(|v| v.conn.clone())
            .collect();
        for conn in conns {
            if send_packet(&conn, seq, &buf[..len]) {
                counters.outbound(len);
            } else {
                counters.drop_one();
            }
        }
        seq = seq.wrapping_add(1);
    }
}

/// Send one packet as one or more datagrams. Returns false when it could not be sent
/// (congestion or a closed connection); the packet is then dropped, never queued.
fn send_packet(conn: &Connection, seq: u16, packet: &[u8]) -> bool {
    let Some(max) = conn.max_datagram_size() else {
        return false;
    };
    let room = max.saturating_sub(PART_HEADER).max(1);
    let parts = packet.len().div_ceil(room).max(1);
    if parts > u8::MAX as usize {
        return false;
    }
    for (i, chunk) in packet.chunks(room).enumerate().take(parts) {
        let mut d = Vec::with_capacity(PART_HEADER + chunk.len());
        d.extend_from_slice(&seq.to_be_bytes());
        d.push(i as u8);
        d.push(parts as u8);
        d.extend_from_slice(chunk);
        if conn.send_datagram(d.into()).is_err() {
            return false;
        }
    }
    if packet.is_empty() {
        let mut d = seq.to_be_bytes().to_vec();
        d.extend_from_slice(&[0, 1]);
        return conn.send_datagram(d.into()).is_ok();
    }
    true
}

/// Puts packets back together from their parts. Only the packet in progress is kept: when
/// a part of a newer packet arrives first, the older one is dropped (its part was lost).
#[derive(Default)]
struct Reassembly {
    seq: Option<u16>,
    parts: Vec<Option<Vec<u8>>>,
}

impl Reassembly {
    fn add(&mut self, datagram: &[u8], counters: &Counters) -> Option<Vec<u8>> {
        if datagram.len() < PART_HEADER {
            counters.drop_one();
            return None;
        }
        let seq = u16::from_be_bytes([datagram[0], datagram[1]]);
        let (part, parts) = (datagram[2] as usize, datagram[3] as usize);
        let payload = &datagram[PART_HEADER..];
        if parts <= 1 {
            return Some(payload.to_vec());
        }
        if part >= parts {
            counters.drop_one();
            return None;
        }
        if self.seq != Some(seq) || self.parts.len() != parts {
            if self.seq.is_some() {
                counters.drop_one();
            }
            self.seq = Some(seq);
            self.parts = vec![None; parts];
        }
        self.parts[part] = Some(payload.to_vec());
        if self.parts.iter().all(Option::is_some) {
            self.seq = None;
            let packet = std::mem::take(&mut self.parts)
                .into_iter()
                .flatten()
                .flatten()
                .collect();
            return Some(packet);
        }
        None
    }
}

/// Run an import until it is closed: connect, deliver, and connect again when the link
/// drops.
#[allow(clippy::too_many_arguments)]
async fn run_import(
    endpoint: Endpoint,
    node_id: EndpointId,
    name: String,
    kind: LinkKind,
    addr: SocketAddr,
    listener: Option<TcpListener>,
    slot: Arc<Mutex<Option<Connection>>>,
    error: Arc<Mutex<String>>,
    counters: Arc<Counters>,
    cancel: CancellationToken,
) {
    let set_error = |e: String| *error.lock().unwrap_or_else(|p| p.into_inner()) = e;
    while !cancel.is_cancelled() {
        let conn =
            match tokio::time::timeout(CONNECT_TIMEOUT, endpoint.connect(node_id, ALPN)).await {
                Ok(Ok(conn)) => conn,
                Ok(Err(e)) => {
                    set_error(format!("connect: {e}"));
                    sleep_or_cancel(&cancel, RETRY_EVERY).await;
                    continue;
                }
                Err(_) => {
                    set_error("connect timed out".to_string());
                    sleep_or_cancel(&cancel, RETRY_EVERY).await;
                    continue;
                }
            };
        let result = match kind {
            LinkKind::Udp => import_udp(&conn, &name, addr, &slot, &counters, &cancel).await,
            LinkKind::Tcp => match &listener {
                Some(l) => import_tcp(&conn, &name, l, &slot, &counters, &cancel).await,
                None => Err("no listener".to_string()),
            },
        };
        *slot.lock().unwrap_or_else(|p| p.into_inner()) = None;
        match result {
            Ok(()) => set_error(String::new()),
            Err(e) => set_error(e),
        }
        conn.close(0u32.into(), b"import closed");
        sleep_or_cancel(&cancel, RETRY_EVERY).await;
    }
}

async fn sleep_or_cancel(cancel: &CancellationToken, d: Duration) {
    tokio::select! {
        _ = cancel.cancelled() => {}
        _ = tokio::time::sleep(d) => {}
    }
}

/// Ask the exporter for `name`; returns the stream that keeps the link open.
async fn hello(
    conn: &Connection,
    name: &str,
    kind: LinkKind,
) -> Result<(SendStream, RecvStream), String> {
    let (mut send, mut recv) = conn.open_bi().await.map_err(|e| e.to_string())?;
    write_line(
        &mut send,
        &serde_json::to_value(Hello {
            name: name.to_string(),
            kind,
        })
        .unwrap_or_default(),
    )
    .await?;
    let answer: serde_json::Value =
        serde_json::from_slice(&read_line(&mut recv).await?).map_err(|e| e.to_string())?;
    match answer.get("error").and_then(|e| e.as_str()) {
        Some(err) => Err(err.to_string()),
        None => Ok((send, recv)),
    }
}

/// UDP import: deliver the exporter's datagrams to `addr`; send what comes back from there
/// to the exporter.
async fn import_udp(
    conn: &Connection,
    name: &str,
    addr: SocketAddr,
    slot: &Arc<Mutex<Option<Connection>>>,
    counters: &Arc<Counters>,
    cancel: &CancellationToken,
) -> Result<(), String> {
    let (_send, mut recv) = hello(conn, name, LinkKind::Udp).await?;
    let local: SocketAddr = if addr.is_ipv4() {
        "0.0.0.0:0".parse().unwrap()
    } else {
        "[::]:0".parse().unwrap()
    };
    let sock = UdpSocket::bind(local)
        .await
        .map_err(|e| format!("bind udp: {e}"))?;
    *slot.lock().unwrap_or_else(|p| p.into_inner()) = Some(conn.clone());
    let mut parts = Reassembly::default();
    let mut buf = vec![0u8; MAX_PACKET];
    let mut seq: u16 = 0;
    let mut end = [0u8; 1];
    loop {
        tokio::select! {
            _ = cancel.cancelled() => return Ok(()),
            // The exporter closes the stream when it closes the export.
            _ = recv.read(&mut end) => return Err("the export closed".to_string()),
            d = conn.read_datagram() => {
                let d = d.map_err(|e| format!("link lost: {e}"))?;
                if let Some(packet) = parts.add(&d, counters) {
                    counters.inbound(packet.len());
                    let _ = sock.send_to(&packet, addr).await;
                }
            }
            r = sock.recv_from(&mut buf) => {
                let Ok((len, _)) = r else {
                    // Nothing listens at `addr` (an ICMP error on Windows); never spin.
                    tokio::time::sleep(ERROR_PAUSE).await;
                    continue;
                };
                {
                    if send_packet(conn, seq, &buf[..len]) {
                        counters.outbound(len);
                    } else {
                        counters.drop_one();
                    }
                    seq = seq.wrapping_add(1);
                }
            }
        }
    }
}

/// TCP import: carry each connection accepted on `listener` over its own QUIC stream.
async fn import_tcp(
    conn: &Connection,
    name: &str,
    listener: &TcpListener,
    slot: &Arc<Mutex<Option<Connection>>>,
    counters: &Arc<Counters>,
    cancel: &CancellationToken,
) -> Result<(), String> {
    *slot.lock().unwrap_or_else(|p| p.into_inner()) = Some(conn.clone());
    loop {
        let tcp = tokio::select! {
            _ = cancel.cancelled() => return Ok(()),
            closed = conn.closed() => return Err(format!("link lost: {closed}")),
            a = listener.accept() => match a {
                Ok((tcp, _)) => tcp,
                Err(_) => {
                    tokio::time::sleep(ERROR_PAUSE).await;
                    continue;
                }
            },
        };
        let (conn, name, counters, cancel) = (
            conn.clone(),
            name.to_string(),
            counters.clone(),
            cancel.clone(),
        );
        tokio::spawn(async move {
            if let Ok((send, recv)) = hello(&conn, &name, LinkKind::Tcp).await {
                splice(tcp, send, recv, &counters, cancel).await;
            }
        });
    }
}

/// Copy both ways between a TCP connection and a QUIC stream until either side ends.
async fn splice(
    tcp: TcpStream,
    mut send: SendStream,
    mut recv: RecvStream,
    counters: &Counters,
    cancel: CancellationToken,
) {
    let _ = tcp.set_nodelay(true);
    let (mut tcp_r, mut tcp_w) = tcp.into_split();
    let up = async {
        let mut buf = vec![0u8; 64 << 10];
        while let Ok(n) = tcp_r.read(&mut buf).await {
            if n == 0 || send.write_all(&buf[..n]).await.is_err() {
                break;
            }
            counters.outbound(n);
        }
        let _ = send.finish();
    };
    let down = async {
        let mut buf = vec![0u8; 64 << 10];
        while let Ok(Some(n)) = recv.read(&mut buf).await {
            if tcp_w.write_all(&buf[..n]).await.is_err() {
                break;
            }
            counters.inbound(n);
        }
        let _ = tcp_w.shutdown().await;
    };
    tokio::select! {
        _ = cancel.cancelled() => {}
        _ = async { tokio::join!(up, down) } => {}
    }
}

async fn write_line(send: &mut SendStream, value: &serde_json::Value) -> Result<(), String> {
    let mut line = serde_json::to_vec(value).map_err(|e| e.to_string())?;
    line.push(b'\n');
    send.write_all(&line).await.map_err(|e| e.to_string())
}

async fn read_line(recv: &mut RecvStream) -> Result<Vec<u8>, String> {
    let mut line = Vec::new();
    let mut byte = [0u8; 1];
    loop {
        match recv.read(&mut byte).await.map_err(|e| e.to_string())? {
            Some(1) if byte[0] == b'\n' => return Ok(line),
            Some(1) => line.push(byte[0]),
            Some(_) => {}
            None => return Err("stream ended".to_string()),
        }
        if line.len() > MAX_LINE_BYTES {
            return Err("handshake line too long".to_string());
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn split_packets_come_back_whole_and_a_lost_part_drops_only_its_packet() {
        let counters = Counters::default();
        let mut r = Reassembly::default();
        let packet: Vec<u8> = (0..250u8).collect();
        let room = 100;
        let datagrams = |seq: u16| -> Vec<Vec<u8>> {
            let parts = packet.len().div_ceil(room);
            packet
                .chunks(room)
                .enumerate()
                .map(|(i, c)| {
                    let mut d = seq.to_be_bytes().to_vec();
                    d.extend_from_slice(&[i as u8, parts as u8]);
                    d.extend_from_slice(c);
                    d
                })
                .collect()
        };
        // Whole.
        let mut out = None;
        for d in datagrams(1) {
            out = r.add(&d, &counters);
        }
        assert_eq!(out.as_deref(), Some(packet.as_slice()));
        // Part 1 of packet 2 lost: packet 3 still comes through, packet 2 is dropped.
        let two = datagrams(2);
        assert!(r.add(&two[0], &counters).is_none());
        let mut out = None;
        for d in datagrams(3) {
            out = r.add(&d, &counters);
        }
        assert_eq!(out.as_deref(), Some(packet.as_slice()));
        assert_eq!(counters.drops.load(Ordering::Relaxed), 1);
        // A packet that fits travels in one datagram.
        let mut one = 9u16.to_be_bytes().to_vec();
        one.extend_from_slice(&[0, 1, 42]);
        assert_eq!(r.add(&one, &counters), Some(vec![42]));
    }
}
