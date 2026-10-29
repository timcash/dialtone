//! `./dialtone.sh test --only rejoin`: a node on an older release joins the live channel by
//! invite (as an outside node would), goes away, rejoins with its updater on, hears from its
//! peers' beats that a newer release is out, updates itself and keeps it after its trial.

use crate::fleet::Fleet;
use crate::util::{say, sleep, R};
use serde_json::Value;
use std::time::Instant;

const NAME: &str = "rejoin-test";
const STATE: &str = "/tmp/dialtone-rejoin";
const API: u16 = 8795;
const QUIC: u16 = 20995;

/// Download release __FROM__'s Linux binary as a managed install, then join the channel with an
/// invite from this VM's own node. Prints the join report.
const SETUP: &str = r#"
set -e
s=__STATE__
pkill -f "$s/" 2>/dev/null || true
rm -rf "$s"
mkdir -p "$s/bin"
case "$(uname -m)" in aarch64|arm64) a=dialtone-linux-arm64 ;; *) a=dialtone-linux-x86_64 ;; esac
r=https://github.com/timcash/dialtone/releases/download/__FROM__
curl -fsSL "$r/$a" -o "$s/bin/dialtone"
want=$(curl -fsSL "$r/$a.sha256" | cut -d' ' -f1)
if [ "$want" != "$(sha256sum "$s/bin/dialtone" | cut -d' ' -f1)" ]; then echo "the download does not match its published SHA-256" >&2; exit 1; fi
chmod 755 "$s/bin/dialtone"
invite=$("$HOME/.dialtone/bin/dialtone" invite --ttl 10m 2>/dev/null)
DIALTONE_STATE_DIR="$s" "$s/bin/dialtone" join "$invite" --once --node __NAME__ --listen 127.0.0.1:__API__ | tr -d '\n'
echo
"#;

/// First visit: the node runs with its updater off.
const VISIT: &str = r#"
s=__STATE__
DIALTONE_AUTO_UPDATE=0 DIALTONE_STATE_DIR="$s" setsid nohup "$s/bin/dialtone" up --node __NAME__ --listen 127.0.0.1:__API__ --quic-port __QUIC__ >> "$s/node.log" 2>&1 < /dev/null &
"#;

/// The node goes away.
const LEAVE: &str = r#"
pkill -f "__STATE__/bin/dialtone" || true
sleep 1
"#;

/// Rejoin: a restart loop, as systemd, launchd or the Windows task run a node, so it can
/// restart into an update (and roll back).
const SERVICE: &str = r#"
s=__STATE__
cat > "$s/service.sh" <<'SERVICE'
s=__STATE__
while [ ! -f "$s/stop" ]; do
  DIALTONE_SUPERVISED=1 DIALTONE_STATE_DIR="$s" "$s/bin/dialtone" up --node __NAME__ --listen 127.0.0.1:__API__ --quic-port __QUIC__ >> "$s/node.log" 2>&1 < /dev/null
  sleep 1
done
SERVICE
setsid nohup sh "$s/service.sh" > /dev/null 2>&1 < /dev/null &
"#;

const STOP: &str = r#"
touch __STATE__/stop 2>/dev/null
pkill -f "__STATE__/" || true
sleep 1
rm -rf __STATE__
"#;

fn fill(script: &str, from: &str) -> String {
    script
        .replace("__STATE__", STATE)
        .replace("__NAME__", NAME)
        .replace("__API__", &API.to_string())
        .replace("__QUIC__", &QUIC.to_string())
        .replace("__FROM__", from)
}

/// Stops the test node and removes its state when the test ends (or fails).
struct Cleanup<'a>(&'a Fleet, &'a str);

impl Drop for Cleanup<'_> {
    fn drop(&mut self) {
        let _ = self.0.sh(self.1, &fill(STOP, ""));
    }
}

/// The two newest signed releases (they carry dialtone-release.json): (older, newer).
fn releases(fleet: &Fleet, vm: &str) -> R<(String, String)> {
    let list = fleet.sh(
        vm,
        "curl -fsSL -H 'Accept: application/vnd.github+json' 'https://api.github.com/repos/timcash/dialtone/releases?per_page=30'",
    )?;
    let list: Value = serde_json::from_str(&list).map_err(|e| format!("GitHub releases: {e}"))?;
    let signed: Vec<String> = list
        .as_array()
        .into_iter()
        .flatten()
        .filter(|r| r["draft"] != true && r["prerelease"] != true)
        .filter(|r| {
            r["assets"].as_array().is_some_and(|a| {
                a.iter()
                    .any(|x| x["name"].as_str() == Some("dialtone-release.json"))
            })
        })
        .filter_map(|r| r["tag_name"].as_str().map(String::from))
        .collect();
    match &signed[..] {
        [newer, older, ..] => Ok((older.clone(), newer.clone())),
        _ => Err("the rejoin test needs two signed releases on GitHub".into()),
    }
}

/// The test node as the build VM's node sees it: (version, seconds since its last beat).
fn seen(fleet: &Fleet) -> Option<(String, u64)> {
    let mesh = fleet.mesh();
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    mesh["nodes"].as_array()?.iter().find_map(|n| {
        (n["node"].as_str() == Some(NAME)).then(|| {
            let version = n["version"].as_str().unwrap_or("").to_string();
            let at = n["updated_at_unix"].as_u64().unwrap_or(0);
            (version, now.saturating_sub(at))
        })
    })
}

/// Wait up to `secs` for `done`, checking every 5s; `false` when time ran out.
fn wait(secs: u64, mut done: impl FnMut() -> bool) -> bool {
    let started = Instant::now();
    while started.elapsed().as_secs() < secs {
        if done() {
            return true;
        }
        sleep(5.0);
    }
    done()
}

pub fn run(fleet: &Fleet) -> R<()> {
    // The last VM hosts the test node; the build VM's node watches the mesh.
    let vm = fleet.nodes.last().ok_or("no VMs")?.clone();
    let (from, to) = releases(fleet, &vm)?;
    let (from_v, to_v) = (from.trim_start_matches('v'), to.trim_start_matches('v'));
    say(&format!(
        "Rejoin test on {vm}: a node on {from} joins by invite, goes away, rejoins and must update itself to {to}"
    ));
    let _cleanup = Cleanup(fleet, &vm);
    let started = Instant::now();
    let t = || started.elapsed().as_secs();

    let report = fleet.sh(&vm, &fill(SETUP, &from))?;
    let report: Value = serde_json::from_str(report.trim())
        .map_err(|e| format!("join with an invite on {vm}: {e}"))?;
    if report["status"] != "joined" {
        return Err(format!("join with an invite on {vm}: {report}"));
    }
    println!(
        "    {:>4}s joined channel '{}' by invite ({} seeds)",
        t(),
        report["channel"].as_str().unwrap_or("?"),
        report["seeds"]
    );

    fleet.sh(&vm, &fill(VISIT, &from))?;
    if !wait(90, || {
        seen(fleet).is_some_and(|(v, age)| v == from_v && age < 15)
    }) {
        return Err(format!(
            "the test node on {from} did not show up in the mesh within 90s"
        ));
    }
    println!("    {:>4}s in the mesh on {from} (updater off)", t());

    fleet.sh(&vm, &fill(LEAVE, &from))?;
    println!("    {:>4}s gone; away for 30s", t());
    sleep(30.0);

    fleet.sh(&vm, &fill(SERVICE, &from))?;
    println!(
        "    {:>4}s back, in a restart loop with its updater on",
        t()
    );
    let committed = || {
        fleet
            .sh(&vm, &format!("cat {STATE}/update.json 2>/dev/null"))
            .ok()
            .and_then(|s| serde_json::from_str::<Value>(&s).ok())
            .and_then(|u| u["committed"]["version"].as_str().map(String::from))
    };
    let updated = wait(420, || {
        seen(fleet).is_some_and(|(v, age)| v == to_v && age < 15)
            && committed().as_deref() == Some(to_v)
    });
    let log = fleet
        .sh(
            &vm,
            &format!("grep -a '\\[update\\]' {STATE}/node.log || true"),
        )
        .unwrap_or_default();
    for line in log.lines() {
        println!("           {line}");
    }
    if !updated {
        return Err(format!(
            "the test node did not update itself to {to} and keep it within 7 minutes of rejoining"
        ));
    }
    if !log.contains("heard from peer") {
        return Err(format!(
            "the test node updated to {to}, but found the release on GitHub instead of in a peer's beat"
        ));
    }
    println!(
        "    {:>4}s on {to}: heard of it from a peer, installed it, restarted and kept it",
        t()
    );
    Ok(())
}
