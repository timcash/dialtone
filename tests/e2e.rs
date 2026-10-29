//! Three real nodes on this machine, an open channel joined with only an invite, and a
//! node whose seed was down when it started.
// A node is an index into NAMES, QUIC, API and the state directories together.
#![allow(clippy::needless_range_loop)]
mod common;

use std::collections::HashSet;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream, UdpSocket};
use std::thread::{self, sleep};
use std::time::{Duration, Instant};

use base64::Engine;
use common::*;
use serde_json::{json, Value};

const NAMES: [&str; 3] = ["e2e-a", "e2e-b", "e2e-c"];
const QUIC: [u16; 3] = [19511, 19512, 19513];
const API: [u16; 3] = [18911, 18912, 18913];
const SECRET: &str = "dt-E2E_Local_Mesh_Secret_2026!";

#[test]
fn three_nodes() {
    let root = TempDir::new("e2e");
    let dirs: Vec<_> = NAMES.iter().map(|n| root.join(n)).collect();

    // Identities, then everyone allowlists everyone at a fixed QUIC port.
    let mut ids = Vec::new();
    let mut ip = "127.0.0.1".to_string();
    for (i, name) in NAMES.iter().enumerate() {
        std::fs::create_dir_all(&dirs[i]).unwrap();
        let status: Value = serde_json::from_str(&run_ok(
            Some(&dirs[i]),
            &["status", "--node", name, "--private-mesh"],
        ))
        .unwrap();
        ids.push(
            status["diagnostics"]["node_id"]
                .as_str()
                .unwrap()
                .to_string(),
        );
        ip = status["diagnostics"]["bound_non_tailnet_ipv4"]
            .as_str()
            .unwrap_or("127.0.0.1")
            .to_string();
    }
    for i in 0..3 {
        for j in 0..3 {
            if i != j {
                let addr = format!("{ip}:{}", QUIC[j]);
                run_ok(
                    Some(&dirs[i]),
                    &["allowlist", "add", &ids[j], "--addr", &addr],
                );
            }
        }
    }
    let mut nodes: Vec<Node> = (0..3)
        .map(|i| {
            let listen = format!("127.0.0.1:{}", API[i]);
            let quic = QUIC[i].to_string();
            Node::start(
                &dirs[i],
                &root.join(&format!("{}.log", NAMES[i])),
                API[i],
                &[
                    "up",
                    "--node",
                    NAMES[i],
                    "--listen",
                    &listen,
                    "--private-mesh",
                    "--quic-port",
                    &quic,
                    "--secret",
                    SECRET,
                ],
            )
        })
        .collect();
    let get = |i: usize, path: &str| get_json(API[i], path);
    let post =
        |i: usize, path: &str, body: &str| post_json(API[i], path, &api_token(&dirs[i]), body);

    for i in 0..3 {
        until_ok(60, &format!("{} sees 3 nodes", NAMES[i]), || {
            get(i, "/api/mesh").is_some_and(|m| m["nodes"].as_array().map(Vec::len) == Some(3))
        });
    }
    println!("    1. Mesh: 3 nodes see each other");

    // Once the mesh has formed, a node talks to it only through gossip: no new connections.
    let conns = |i: usize| get(i, "/api/net").map(|n| n["socket"]["conns_opened"].clone());
    sleep(Duration::from_secs(10));
    let before: Vec<_> = (0..3).map(conns).collect();
    sleep(Duration::from_secs(20));
    for i in 0..3 {
        let after = conns(i);
        assert_eq!(
            after, before[i],
            "{} opened connection(s) in 20s of steady state",
            NAMES[i]
        );
    }
    println!("    Gossip only: no node opened a connection in 20s of steady state");

    // The gossip probe passes on every node, then a full probe pulls a fresh blob from each peer.
    let passes = |p: &Value, n: u64| p["passed_count"] == n && p["total_count"] == n;
    for i in 0..3 {
        until_ok(60, &format!("{} gossip probe 3/3", NAMES[i]), || {
            post(i, "/api/probe", "{}").is_some_and(|r| passes(&r["probe"], 3))
        });
    }
    for i in 0..3 {
        until_ok(
            60,
            &format!("{} full probe 7/7 with 2 pulls", NAMES[i]),
            || {
                post(i, "/api/probe", r#"{"all":true}"#).is_some_and(|r| {
                    let pulls = r["probe"]["pulls"]
                        .as_array()
                        .map_or(0, |p| p.iter().filter(|x| x["ok"] == true).count());
                    passes(&r["probe"], 7) && pulls == 2
                })
            },
        );
    }
    println!("    2. Probe: gossip checks 3/3 on every node; full probe 7/7, each pulled a fresh blob from both peers");
    let d = get(0, &format!("/api/details?node_id={}", ids[1])).expect("details of b");
    assert!(
        d["node_id"] == ids[1].as_str()
            && d["probe"]["checks"]
                .as_array()
                .is_some_and(|c| !c.is_empty())
            && d["devices"].is_array(),
        "{d}"
    );
    let net = get(0, "/api/net").unwrap();
    let scores = net["scores"].as_array().unwrap();
    assert!(
        !scores.is_empty() && !scores.iter().any(|s| s["graylisted"] == true),
        "{net}"
    );
    println!(
        "    Details: a fetched b's devices and probe from b; a scores its peers, none graylisted"
    );

    let program = base64::engine::general_purpose::STANDARD.encode(polyglot(
        "while true; do sleep 5; done; exit 0;",
        &[
            "@echo off",
            &format!("REM e2e {}", root.path().display()),
            ":loop",
            "ping -n 6 127.0.0.1 >NUL",
            "goto loop",
        ],
    ));
    let runs = |i: usize, name: &str| {
        get(i, "/api/processes").is_some_and(|r| {
            r["processes"].as_array().is_some_and(|ps| {
                ps.iter()
                    .any(|p| p["name"] == name && p["status"] == "running")
            })
        })
    };
    let deploy = json!({"name": "e2e-program", "bytes_base64": program, "desired_state": "running", "deploy_to_mesh": true});
    post(0, "/api/processes/blob", &deploy.to_string()).expect("deploy to the mesh");
    for i in [1, 2] {
        until_ok(
            60,
            &format!("{} runs the deployed program", NAMES[i]),
            || runs(i, "e2e-program"),
        );
    }
    post(
        0,
        "/api/processes",
        r#"{"name":"e2e-program","desired_state":"removed"}"#,
    )
    .unwrap();
    for i in 0..3 {
        until_ok(60, &format!("{} removed the program", NAMES[i]), || {
            !runs(i, "e2e-program")
        });
    }
    println!("    3. Deploy: ran on all 3 nodes, removing it on the origin removed every copy");

    // A deploy announced to one node runs only there.
    let targeted = json!({"name": "e2e-targeted", "bytes_base64": program, "desired_state": "running",
        "deploy_to_mesh": true, "deploy_targets": [NAMES[1]]});
    post(0, "/api/processes/blob", &targeted.to_string()).expect("targeted deploy");
    until_ok(60, "e2e-b runs the targeted program", || {
        runs(1, "e2e-targeted")
    });
    sleep(Duration::from_secs(5));
    assert!(
        !runs(2, "e2e-targeted"),
        "e2e-c ran a program deployed only to e2e-b"
    );
    post(
        0,
        "/api/processes",
        r#"{"name":"e2e-targeted","desired_state":"removed"}"#,
    )
    .unwrap();
    until_ok(60, "e2e-b removed the targeted program", || {
        !runs(1, "e2e-targeted")
    });
    println!("    Targeted deploy: ran on e2e-b only, removed with its origin");

    // A direct link: UDP packets from a source on a reach a listener on b (one of them split
    // into parts and put back together), and a TCP service on a answers through b.
    post(
        0,
        "/api/links/export",
        r#"{"name":"e2e-cam","udp":"127.0.0.1:19621"}"#,
    )
    .unwrap();
    post(
        0,
        "/api/links/export",
        r#"{"name":"e2e-web","tcp":"127.0.0.1:19631"}"#,
    )
    .unwrap();
    post(
        1,
        "/api/links/import",
        r#"{"node":"e2e-a","name":"e2e-cam","udp":"127.0.0.1:19622"}"#,
    )
    .unwrap();
    post(
        1,
        "/api/links/import",
        r#"{"node":"e2e-a","name":"e2e-web","tcp":"127.0.0.1:19632"}"#,
    )
    .unwrap();
    link_carries_udp_and_tcp();
    let links = get(1, "/api/links").unwrap();
    let cam = links["imports"]
        .as_array()
        .and_then(|i| i.iter().find(|x| x["name"] == "e2e-cam"))
        .unwrap_or_else(|| panic!("no e2e-cam import: {links}"));
    assert!(
        cam["state"] == "up"
            && cam["path"] == "direct"
            && !cam["rtt_ms"].is_null()
            && cam["counters"]["packets_in"].as_u64() >= Some(2),
        "{cam}"
    );
    post(1, "/api/links/close", r#"{"id":"e2e-a/e2e-cam"}"#).unwrap();
    println!("    Link: UDP (a split packet too) and TCP crossed a direct link from a to b; it reports DIRECT and an Rtt");

    nodes[2].stop();
    until_ok(45, "a's probe reports e2e-c as silent", || {
        post(0, "/api/probe", "{}").is_some_and(|r| {
            r["probe"]["checks"].as_array().is_some_and(|cs| {
                cs.iter().any(|c| {
                    c["code"] == "BEAT"
                        && c["ok"] == false
                        && c["detail"].as_str().is_some_and(|d| d.contains("e2e-c"))
                })
            })
        })
    });
    println!("    4. Failure: a's probe flagged e2e-c once it stopped");
    drop(nodes);
}

/// UDP from 19621 (exported on a) arrives on 19622 (imported on b), and a TCP service on
/// 19631 answers through 19632.
fn link_carries_udp_and_tcp() {
    let rx = UdpSocket::bind("127.0.0.1:19622").unwrap();
    rx.set_read_timeout(Some(Duration::from_millis(500)))
        .unwrap();
    let tx = UdpSocket::bind("127.0.0.1:0").unwrap();
    let big: Vec<u8> = (0..=255u8).cycle().take(256 * 12).collect();
    let want: HashSet<Vec<u8>> = [b"small".to_vec(), big].into_iter().collect();
    let mut got = HashSet::new();
    let deadline = Instant::now() + Duration::from_secs(30);
    let mut buf = vec![0u8; 65535];
    while got != want && Instant::now() < deadline {
        for p in want.difference(&got) {
            tx.send_to(p, "127.0.0.1:19621").unwrap();
        }
        while let Ok((n, _)) = rx.recv_from(&mut buf) {
            got.insert(buf[..n].to_vec());
        }
    }
    assert!(
        got.is_superset(&want),
        "packets did not cross the link: received {} of {}",
        got.intersection(&want).count(),
        want.len()
    );

    let srv = TcpListener::bind("127.0.0.1:19631").unwrap();
    thread::spawn(move || {
        for mut c in srv.incoming().flatten() {
            let mut b = [0u8; 100];
            if let Ok(n) = c.read(&mut b) {
                let _ = c.write_all(&[&b[..n], b" back"].concat());
            }
        }
    });
    let mut reply = Vec::new();
    for _ in 0..30 {
        if let Ok(mut c) =
            TcpStream::connect_timeout(&"127.0.0.1:19632".parse().unwrap(), Duration::from_secs(5))
        {
            let _ = c.set_read_timeout(Some(Duration::from_secs(5)));
            if c.write_all(b"hello").is_ok() {
                reply.clear();
                let _ = c.read_to_end(&mut reply);
                if !reply.is_empty() {
                    break;
                }
            }
        }
        sleep(Duration::from_secs(1));
    }
    assert_eq!(
        String::from_utf8_lossy(&reply),
        "hello back",
        "packets did not cross the link"
    );
}

/// An open channel: o-b joins with nothing but an invite from o-a. Neither lists the other.
#[test]
fn open_channel_invite() {
    let o = TempDir::new("open");
    let (a, b, c) = (o.join("a"), o.join("b"), o.join("c"));
    std::fs::create_dir_all(&a).unwrap();
    std::fs::create_dir_all(&b).unwrap();
    let _oa = Node::start(
        &a,
        &o.join("a.log"),
        18914,
        &[
            "up",
            "--node",
            "o-a",
            "--listen",
            "127.0.0.1:18914",
            "--private-mesh",
            "--quic-port",
            "19514",
            "--secret",
            "dt-E2E_Open_Channel_Secret_2026!",
        ],
    );
    until_ok(30, "o-a API up", || get_json(18914, "/api/mesh").is_some());
    // `dialtone invite` waits for a node whose endpoint is still starting.
    let invite = run_ok(
        Some(&a),
        &["invite", "--listen", "127.0.0.1:18914", "--ttl", "10m"],
    )
    .trim()
    .to_string();
    assert!(
        invite.starts_with("dialtone-invite-v2:"),
        "no invite from o-a: {invite}"
    );
    let joined = run_ok(
        Some(&b),
        &[
            "join",
            &invite,
            "--node",
            "o-b",
            "--listen",
            "127.0.0.1:18915",
            "--private-mesh",
            "--once",
        ],
    );
    assert!(
        joined.contains(r#""status": "joined""#),
        "o-b could not join with the invite: {joined}"
    );
    let _ob = Node::start(
        &b,
        &o.join("b.log"),
        18915,
        &[
            "up",
            "--node",
            "o-b",
            "--listen",
            "127.0.0.1:18915",
            "--private-mesh",
            "--quic-port",
            "19515",
        ],
    );
    let sees_two = |port: u16| {
        get_json(port, "/api/mesh").is_some_and(|m| m["nodes"].as_array().map(Vec::len) == Some(2))
    };
    until_ok(60, "o-a sees o-b", || sees_two(18914));
    until_ok(60, "o-b sees o-a", || sees_two(18915));
    let (_, probe) = http(
        18915,
        "POST",
        "/api/probe",
        Some(&api_token(&b)),
        Some("{}"),
    )
    .unwrap();
    assert!(
        probe.contains("open channel, 1 peers in the mesh"),
        "o-b's probe does not report the open channel: {probe}"
    );

    let b64 = base64::engine::general_purpose::URL_SAFE_NO_PAD;
    let raw = invite.split_once(':').unwrap().1.trim_end_matches('=');
    let mut inv: Value = serde_json::from_slice(&b64.decode(raw).unwrap()).unwrap();
    inv["expires_at_unix"] = json!(1);
    let late = format!("dialtone-invite-v2:{}", b64.encode(inv.to_string()));
    assert!(
        !run(
            Some(&c),
            &["join", &late, "--node", "o-c", "--private-mesh", "--once"]
        )
        .0,
        "an expired invite was accepted"
    );
    println!("    5. Open channel: o-b joined o-a with only an invite (no allowlists); an expired one is refused");
}

/// A node that starts while its only seed is down joins it once the seed is back, even
/// after its first dial to the seed failed (iroh-gossip never dials such a peer again).
#[test]
fn seed_back() {
    let s = TempDir::new("seedback");
    let (a, b) = (s.join("a"), s.join("b"));
    std::fs::create_dir_all(&a).unwrap();
    std::fs::create_dir_all(&b).unwrap();
    let a_args = [
        "up",
        "--node",
        "s-a",
        "--listen",
        "127.0.0.1:18916",
        "--private-mesh",
        "--quic-port",
        "19516",
        "--secret",
        "dt-E2E_Seed_Back_Secret_2026!",
    ];
    let mut sa = Node::start(&a, &s.join("a.log"), 18916, &a_args);
    until_ok(30, "s-a API up", || get_json(18916, "/api/mesh").is_some());
    let invite = run_ok(
        Some(&a),
        &["invite", "--listen", "127.0.0.1:18916", "--ttl", "10m"],
    )
    .trim()
    .to_string();
    run_ok(
        Some(&b),
        &[
            "join",
            &invite,
            "--node",
            "s-b",
            "--listen",
            "127.0.0.1:18917",
            "--private-mesh",
            "--once",
        ],
    );
    sa.stop();
    let _sb = Node::start(
        &b,
        &s.join("b.log"),
        18917,
        &[
            "up",
            "--node",
            "s-b",
            "--listen",
            "127.0.0.1:18917",
            "--private-mesh",
            "--quic-port",
            "19517",
        ],
    );
    until_ok(30, "s-b API up", || get_json(18917, "/api/mesh").is_some());
    // Long enough for s-b's first dial to its seed to fail.
    sleep(Duration::from_secs(40));
    let _sa = Node::start(&a, &s.join("a2.log"), 18916, &a_args);
    let sees_two = |port: u16| {
        get_json(port, "/api/mesh").is_some_and(|m| m["nodes"].as_array().map(Vec::len) == Some(2))
    };
    until_ok(60, "s-b joins s-a once it is back", || sees_two(18917));
    println!("    6. A node that started while its seed was down joined it once the seed was back");
}

/// A seed that restarts is joined again within seconds, even by nodes that stay each
/// other's neighbors meanwhile (they have neighbors enough, so only the seed's stale
/// beats bring them back to it; it knows no peers of its own).
#[test]
fn seed_restart() {
    let s = TempDir::new("seedrestart");
    const API_R: [u16; 4] = [18918, 18919, 18920, 18921];
    const QUIC_R: [&str; 4] = ["19518", "19519", "19520", "19521"];
    let names = ["r-a", "r-b", "r-c", "r-d"];
    let dirs: Vec<_> = names.iter().map(|n| s.join(n)).collect();
    for d in &dirs {
        std::fs::create_dir_all(d).unwrap();
    }
    let listen = |k: usize| format!("127.0.0.1:{}", API_R[k]);
    let la = listen(0);
    let a_args = [
        "up",
        "--node",
        names[0],
        "--listen",
        &la,
        "--private-mesh",
        "--quic-port",
        QUIC_R[0],
        "--secret",
        "dt-E2E_Seed_Restart_Secret_2026!",
    ];
    let mut sa = Node::start(&dirs[0], &s.join("r-a.log"), API_R[0], &a_args);
    until_ok(30, "r-a API up", || {
        get_json(API_R[0], "/api/mesh").is_some()
    });
    let invite = run_ok(Some(&dirs[0]), &["invite", "--listen", &la, "--ttl", "10m"])
        .trim()
        .to_string();
    let mut others = Vec::new();
    for k in 1..4 {
        let l = listen(k);
        run_ok(
            Some(&dirs[k]),
            &[
                "join",
                &invite,
                "--node",
                names[k],
                "--listen",
                &l,
                "--private-mesh",
                "--once",
            ],
        );
        others.push(Node::start(
            &dirs[k],
            &s.join(&format!("{}.log", names[k])),
            API_R[k],
            &[
                "up",
                "--node",
                names[k],
                "--listen",
                &l,
                "--private-mesh",
                "--quic-port",
                QUIC_R[k],
            ],
        ));
    }
    let sees_four = |port: u16| {
        get_json(port, "/api/mesh").is_some_and(|m| m["nodes"].as_array().map(Vec::len) == Some(4))
    };
    until_ok(90, "r-a sees all four", || sees_four(API_R[0]));
    sa.stop();
    let _sa = Node::start(&dirs[0], &s.join("r-a2.log"), API_R[0], &a_args);
    // Before the fix this took until the old r-a was pruned (5 minutes).
    until_ok(60, "the restarted seed is joined again", || {
        sees_four(API_R[0])
    });
    println!(
        "    7. A seed that restarted was joined again by nodes that kept their own neighbors"
    );
}
