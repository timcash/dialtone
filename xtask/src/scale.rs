//! Scale tests: many nodes on this machine, or on every VM of the fleet as one mesh, gossip
//! only; convergence time and traffic per node.

use crate::fleet::Fleet;
use crate::util::{self, output, release_bin, say, Rng, R};
use serde_json::{json, Value};
use std::collections::BTreeMap;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::time::Instant;

/// Nodes started here: stopped and their state removed when the test ends (or fails).
struct Nodes {
    root: PathBuf,
    children: Vec<Child>,
    secret: &'static str,
}

impl Nodes {
    fn new(prefix: &str, secret: &'static str) -> R<Nodes> {
        stop_leftovers(secret);
        let root = std::env::temp_dir().join(format!("{prefix}-{}", std::process::id()));
        std::fs::create_dir_all(&root).map_err(|e| format!("{}: {e}", root.display()))?;
        Ok(Nodes {
            root,
            children: Vec::new(),
            secret,
        })
    }

    fn dir(&self, k: usize) -> PathBuf {
        self.root.join(format!("s{k}"))
    }

    fn start(&mut self, k: usize, node: &str, extra: &[&str]) -> R<()> {
        let log = std::fs::File::create(self.root.join(format!("s{k}.log")))
            .map_err(|e| e.to_string())?;
        let child = Command::new(release_bin())
            .env("DIALTONE_STATE_DIR", self.dir(k))
            .args([
                "up",
                "--node",
                node,
                "--listen",
                &format!("127.0.0.1:{}", 21000 + k),
            ])
            .args([
                "--quic-port",
                &(20000 + k).to_string(),
                "--secret",
                self.secret,
            ])
            .args(extra)
            .stdout(log.try_clone().map_err(|e| e.to_string())?)
            .stderr(log)
            .spawn()
            .map_err(|e| format!("start {node}: {e}"))?;
        self.children.push(child);
        Ok(())
    }
}

impl Drop for Nodes {
    fn drop(&mut self) {
        for c in &mut self.children {
            let _ = c.kill();
        }
        for c in &mut self.children {
            let _ = c.wait();
        }
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

/// Nodes left by a run that was killed would hold the ports.
fn stop_leftovers(secret: &str) {
    if cfg!(windows) {
        let ps = format!(
            "Get-CimInstance Win32_Process | Where-Object {{ $_.Name -eq 'dialtone.exe' -and $_.CommandLine -like '*{secret}*' }} | ForEach-Object {{ Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }}"
        );
        util::quietly(Command::new("powershell.exe").args([
            "-NoProfile",
            "-NonInteractive",
            "-Command",
            &ps,
        ]));
    } else if util::quietly(Command::new("pkill").args([
        "-9",
        "-f",
        "--",
        &format!("--secret {secret}"),
    ])) {
        util::sleep(1.0);
    }
}

/// `status --node <node>` in a node's state directory: its NodeId and LAN address.
fn identity(dir: &PathBuf, node: &str, private: bool) -> R<(String, String)> {
    std::fs::create_dir_all(dir).map_err(|e| e.to_string())?;
    let mut cmd = Command::new(release_bin());
    cmd.env("DIALTONE_STATE_DIR", dir)
        .args(["status", "--node", node]);
    if private {
        cmd.arg("--private-mesh");
    }
    let v: Value = serde_json::from_str(&output(cmd.stderr(Stdio::null()))?)
        .map_err(|e| format!("status of {node}: {e}"))?;
    let d = &v["diagnostics"];
    let id = d["node_id"]
        .as_str()
        .ok_or("status has no node_id")?
        .to_string();
    let ip = d["bound_non_tailnet_ipv4"]
        .as_str()
        .unwrap_or("127.0.0.1")
        .to_string();
    Ok((id, ip))
}

pub fn local(n: usize, secs: f64) -> R<()> {
    let budget: f64 = std::env::var("SCALE_BUDGET_KBPS")
        .ok()
        .and_then(|b| b.parse().ok())
        .unwrap_or(0.0);
    say(&format!("Scale test: {n} nodes, measuring for {secs}s"));
    crate::build()?;
    let mut nodes = Nodes::new("dialtone-scale", "dt-Scale_Local_Mesh_Secret_2026!")?;

    // Identities (in parallel), then each node allowlists everyone but dials only 3 seeds.
    let ids: Vec<(String, String)> = std::thread::scope(|s| {
        let jobs: Vec<_> = (1..=n)
            .map(|k| {
                let dir = nodes.dir(k);
                s.spawn(move || identity(&dir, &format!("s{k}"), true))
            })
            .collect();
        jobs.into_iter()
            .map(|j| j.join().expect("identity thread"))
            .collect::<R<_>>()
    })?;
    let ip = ids.last().map(|x| x.1.clone()).unwrap_or_default();
    let mut rng = Rng::new(n as u64);
    for i in 1..=n {
        let others: Vec<usize> = (1..=n).filter(|&j| j != i).collect();
        let seeds = rng.sample(&others, 3);
        let cfg = json!({
            "allowlist": others.iter().map(|&j| &ids[j - 1].0).collect::<Vec<_>>(),
            "seeds": seeds.iter().map(|&j| json!({"node_id": ids[j - 1].0, "relay_url": "none", "ip_addrs": [format!("{ip}:{}", 20000 + j)]})).collect::<Vec<_>>(),
        });
        std::fs::write(nodes.dir(i).join("config.json"), cfg.to_string())
            .map_err(|e| e.to_string())?;
    }
    for k in 1..=n {
        nodes.start(k, &format!("s{k}"), &["--private-mesh"])?;
    }

    // Convergence: every node lists every node.
    let started = Instant::now();
    let net = |k: usize| util::get((21000 + k) as u16, "/api/net").unwrap_or(Value::Null);
    let converged = || (1..=n).all(|k| net(k)["nodes_known"].as_u64().unwrap_or(0) >= n as u64);
    let mut ok = false;
    for _ in 0..180 {
        if converged() {
            ok = true;
            break;
        }
        util::sleep(1.0);
    }
    if !ok {
        for k in 1..=n {
            let d = net(k);
            println!(
                "    s{k:<4} knows {} neighbors {}",
                d["nodes_known"], d["gossip"]["neighbors"]
            );
        }
        println!("    last lines of s1's log:");
        let log = std::fs::read_to_string(nodes.root.join("s1.log")).unwrap_or_default();
        let lines: Vec<&str> = log.lines().collect();
        lines[lines.len().saturating_sub(15)..]
            .iter()
            .for_each(|l| println!("      {l}"));
        return Err(format!("all {n} nodes see each other (gave up after 180s)"));
    }
    println!(
        "    converged: every node sees all {n} in {}s",
        started.elapsed().as_secs()
    );

    let start: Vec<Value> = (1..=n).map(net).collect();
    util::sleep(secs);
    let end: Vec<Value> = (1..=n).map(net).collect();
    let rows: Vec<Row> = start
        .iter()
        .zip(&end)
        .filter_map(|(a, b)| Row::new(a, b, secs))
        .collect();
    if rows.is_empty() {
        return Err("no node answered /api/net".into());
    }
    let c = |f: fn(&Row) -> f64| rows.iter().map(f).collect::<Vec<f64>>();
    println!(
        "    per node (avg / max): in {:.1} / {:.1} kbit/s, out {:.1} / {:.1} kbit/s",
        avg(&c(|r| r.kin)),
        max(&c(|r| r.kin)),
        avg(&c(|r| r.kout)),
        max(&c(|r| r.kout))
    );
    println!(
        "    gossip: {:.1} msgs/s received per node, {:.0} bytes per message sent, {:.1} neighbors, {:.1} connections opened since boot",
        avg(&c(|r| r.msgs_in)),
        avg(&c(|r| r.bytes_per_msg_out)),
        avg(&c(|r| r.neighbors)),
        avg(&c(|r| r.conns))
    );
    println!(
        "    gossip wire: payload {:.1} kbit/s; {:.2} data frames per message delivered, data {:.1} kbit/s, control {:.1} kbit/s",
        avg(&c(|r| r.payload)),
        avg(&c(|r| r.frames_per_msg)),
        avg(&c(|r| r.data)),
        avg(&c(|r| r.ctrl))
    );
    let worst = max(&c(|r| r.kin)).max(max(&c(|r| r.kout)));
    if budget > 0.0 && worst > budget {
        return Err(format!(
            "{worst:.1} kbit/s is over the budget of {budget:.0} kbit/s"
        ));
    }
    Ok(())
}

/// One node's traffic between two /api/net snapshots.
struct Row {
    kin: f64,
    kout: f64,
    msgs_in: f64,
    bytes_per_msg_out: f64,
    neighbors: f64,
    conns: f64,
    graylisted: f64,
    payload: f64,
    frames_per_msg: f64,
    data: f64,
    ctrl: f64,
}

impl Row {
    fn new(a: &Value, b: &Value, secs: f64) -> Option<Row> {
        if !a["socket"].is_object() || !b["socket"].is_object() {
            return None;
        }
        let d = |sec: &str, k: &str| {
            b[sec][k].as_f64().unwrap_or(0.0) - a[sec][k].as_f64().unwrap_or(0.0)
        };
        let kbps = |bytes: f64| bytes * 8.0 / secs / 1000.0;
        let msgs = d("gossip", "gossip_msgs_in");
        let out_n = d("gossip", "gossip_msgs_out");
        Some(Row {
            kin: kbps(d("socket", "bytes_in")),
            kout: kbps(d("socket", "bytes_out")),
            msgs_in: msgs / secs,
            bytes_per_msg_out: if out_n > 0.0 {
                d("gossip", "gossip_bytes_out") / out_n
            } else {
                0.0
            },
            neighbors: b["gossip"]["neighbors"].as_f64().unwrap_or(0.0),
            conns: b["socket"]["conns_opened"].as_f64().unwrap_or(0.0),
            graylisted: b["scores"].as_array().map_or(0, |s| {
                s.iter()
                    .filter(|x| x["graylisted"].as_bool() == Some(true))
                    .count()
            }) as f64,
            payload: kbps(d("gossip", "gossip_bytes_in")),
            frames_per_msg: d("gossip_wire", "data_recv") / msgs.max(1.0),
            data: kbps(d("gossip_wire", "data_recv_bytes")),
            ctrl: kbps(d("gossip_wire", "ctrl_recv_bytes")),
        })
    }
}

fn avg(xs: &[f64]) -> f64 {
    xs.iter().sum::<f64>() / xs.len().max(1) as f64
}

fn max(xs: &[f64]) -> f64 {
    xs.iter().cloned().fold(0.0, f64::max)
}

// Shell on a VM. __PREFIX__, __N__, __URL__ and __SECRET__ are filled in per VM. Scale nodes
// carry the scale secret on their command line (their state directory is only in their
// environment); the VM's own daemon never does.
const SCALE_STOP: &str = r#"
pkill -f Scale_Fleet_Mesh_Secret || true
sleep 1
rm -rf /tmp/dialtone-scale
"#;
const SCALE_IDS: &str = r#"
set -e
root=/tmp/dialtone-scale; exe="$HOME/.dialtone/bin/dialtone"
for k in $(seq 1 __N__); do
  mkdir -p "$root/s$k"
  echo "NODE $k $(DIALTONE_STATE_DIR="$root/s$k" "$exe" status --node "__PREFIX__-s$k" 2>/dev/null | tr -d '\n')"
done
"#;
// After the config files are written; setsid and nohup so the nodes outlive the SSH session.
const SCALE_START: &str = r#"
root=/tmp/dialtone-scale; exe="$HOME/.dialtone/bin/dialtone"
for k in $(seq 1 __N__); do
  DIALTONE_STATE_DIR="$root/s$k" setsid nohup "$exe" up --node "__PREFIX__-s$k" --listen "127.0.0.1:$((21000 + k))" --quic-port "$((20000 + k))" --relay-url "__URL__" --secret "__SECRET__" > "$root/s$k/node.log" 2>&1 < /dev/null &
done
"#;
const SCALE_NET: &str = r#"
for k in $(seq 1 __N__); do
  curl -s -m 5 "http://127.0.0.1:$((21000 + k))/api/net" | tr -d '\n'
  echo
done
"#;

/// A scale node on one of the VMs.
#[derive(Clone)]
struct ScaleNode {
    id: String,
    ip: String,
    index: usize,
    machine: String,
}

/// Stops every scale node on every VM when the test ends (or fails).
struct ScaleStop<'a>(&'a Fleet);

impl Drop for ScaleStop<'_> {
    fn drop(&mut self) {
        let _ = self.0.each(|node| self.0.sh(node, SCALE_STOP));
    }
}

/// N throwaway nodes on every VM (each runs the VM's installed binary), one mesh across them:
/// convergence, then traffic per node per VM.
pub fn on_fleet(fleet: &Fleet, n: usize, secs: f64) -> R<()> {
    let url = std::env::var("SCALE_RELAY_URL")
        .unwrap_or_else(|_| "https://usw1-1.relay.n0.iroh.link./".into());
    let secret = "dt-Scale_Fleet_Mesh_Secret_2026!";
    say(&format!(
        "Scale test: {n} nodes on each of: {} (relay {url}), measuring for {secs}s",
        fleet.nodes.join(" ")
    ));
    let _stop = ScaleStop(fleet);
    for (_, r) in fleet.each(|node| fleet.sh(node, SCALE_STOP)) {
        r?;
    }

    // Identities on every VM.
    let mut all: Vec<ScaleNode> = Vec::new();
    for (machine, out) in fleet.each(|node| {
        fleet.sh(
            node,
            &SCALE_IDS
                .replace("__PREFIX__", node)
                .replace("__N__", &n.to_string()),
        )
    }) {
        for line in out?.lines() {
            let mut w = line.splitn(3, ' ');
            let (Some("NODE"), Some(idx), Some(status)) = (w.next(), w.next(), w.next()) else {
                continue;
            };
            let v: Value = serde_json::from_str(status)
                .map_err(|e| format!("status of a scale node on {machine}: {e}"))?;
            let d = &v["diagnostics"];
            all.push(ScaleNode {
                id: d["node_id"].as_str().ok_or("status has no node_id")?.into(),
                ip: d["bound_non_tailnet_ipv4"]
                    .as_str()
                    .unwrap_or("127.0.0.1")
                    .into(),
                index: idx.parse().map_err(|_| "bad index")?,
                machine: machine.clone(),
            });
        }
    }
    let total = all.len();
    if total != n * fleet.nodes.len() {
        return Err(format!(
            "expected {} identities, got {total}",
            n * fleet.nodes.len()
        ));
    }

    // Configs: everyone allowlists everyone; 3 random seeds on any VM, reached directly (the
    // VM's own address) or through the relay.
    let mut rng = Rng::new(total as u64);
    let mut configs: BTreeMap<String, String> = BTreeMap::new();
    for me in &all {
        let others: Vec<ScaleNode> = all.iter().filter(|o| o.id != me.id).cloned().collect();
        let seeds = rng.sample(&others, 3);
        let cfg = json!({
            "allowlist": others.iter().map(|o| &o.id).collect::<Vec<_>>(),
            "seeds": seeds.iter().map(|o| json!({"node_id": o.id, "relay_url": url, "ip_addrs": [format!("{}:{}", o.ip, 20000 + o.index)]})).collect::<Vec<_>>(),
        });
        configs
            .entry(me.machine.clone())
            .or_default()
            .push_str(&format!(
                "cat > \"/tmp/dialtone-scale/s{}/config.json\" <<'CONFIG'\n{cfg}\nCONFIG\n",
                me.index
            ));
    }

    // Start every node.
    for (_, r) in fleet.each(|node| {
        let start = SCALE_START
            .replace("__PREFIX__", node)
            .replace("__N__", &n.to_string())
            .replace("__URL__", &url)
            .replace("__SECRET__", secret);
        let configs = configs.get(node).map(String::as_str).unwrap_or("");
        fleet.sh(node, &format!("{configs}{start}"))
    }) {
        r?;
    }
    println!("    started {total} nodes");

    // Every node's /api/net, by node name, with its VM.
    let net = SCALE_NET.replace("__N__", &n.to_string());
    let snapshot = || -> BTreeMap<String, (String, Value)> {
        let mut out = BTreeMap::new();
        for (machine, text) in fleet.each(|node| fleet.sh(node, &net)) {
            for line in text.unwrap_or_default().lines() {
                let v: Value = serde_json::from_str(line).unwrap_or(Value::Null);
                if let Some(name) = v["node"].as_str() {
                    out.insert(name.to_string(), (machine.clone(), v.clone()));
                }
            }
        }
        out
    };

    // Convergence: every node lists every node.
    let started = Instant::now();
    let mut status = String::new();
    let mut ok = false;
    for _ in 0..30 {
        let snap = snapshot();
        let known: Vec<u64> = snap
            .values()
            .map(|(_, v)| v["nodes_known"].as_u64().unwrap_or(0))
            .collect();
        let lo = known.iter().copied().min().unwrap_or(0);
        status = format!(
            "    {}/{total} nodes answer, nodes known: min {lo}, max {}",
            known.len(),
            known.iter().copied().max().unwrap_or(0)
        );
        if known.len() == total && lo >= total as u64 {
            ok = true;
            break;
        }
        util::sleep(5.0);
    }
    println!("{status}");
    if !ok {
        return Err(format!(
            "the scale mesh of {total} nodes did not converge within about 4 minutes"
        ));
    }
    println!(
        "    converged: every node sees all {total} in {}s",
        started.elapsed().as_secs()
    );
    // Joining costs extra (shorter beat intervals while the mesh looks small): measure the
    // steady state after it.
    let warmup: f64 = std::env::var("SCALE_WARMUP")
        .ok()
        .and_then(|w| w.parse().ok())
        .unwrap_or(60.0);
    println!("    warming up {warmup}s before measuring");
    util::sleep(warmup);

    let a = snapshot();
    util::sleep(secs);
    let b = snapshot();
    let mut per: BTreeMap<String, Vec<Row>> = BTreeMap::new();
    for (name, (machine, y)) in &b {
        if let Some(row) = a.get(name).and_then(|(_, x)| Row::new(x, y, secs)) {
            per.entry(machine.clone()).or_default().push(row);
        }
    }
    let mut groups: Vec<(String, Vec<&Row>)> = per
        .iter()
        .map(|(m, r)| (m.clone(), r.iter().collect()))
        .collect();
    groups.push(("all".into(), per.values().flatten().collect()));
    for (machine, rows) in groups {
        let c = |f: fn(&Row) -> f64| rows.iter().map(|r| f(r)).collect::<Vec<f64>>();
        println!(
            "    {machine:<8} {:>3} nodes: in {:.1} / {:.1} kbit/s (avg / max; gossip payload {:.1}), out {:.1} / {:.1}, {:.1} msgs/s, {:.1} neighbors, {} graylisted",
            rows.len(),
            avg(&c(|r| r.kin)),
            max(&c(|r| r.kin)),
            avg(&c(|r| r.payload)),
            avg(&c(|r| r.kout)),
            max(&c(|r| r.kout)),
            avg(&c(|r| r.msgs_in)),
            avg(&c(|r| r.neighbors)),
            c(|r| r.graylisted).iter().sum::<f64>()
        );
        println!(
            "    {:<8}            gossip wire: {:.2} data frames per message delivered, data {:.1} kbit/s, control {:.1} kbit/s",
            "",
            avg(&c(|r| r.frames_per_msg)),
            avg(&c(|r| r.data)),
            avg(&c(|r| r.ctrl))
        );
    }
    Ok(())
}
