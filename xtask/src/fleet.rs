//! The fleet: Linux VMs from fleet.local.env, reached over SSH. One VM (the build VM) builds
//! dialtone and runs the test suite; `test` and `deploy` install that build on every VM and wait
//! until every probe passes on it. Nothing runs on the machine that drives them.

use crate::util::{quietly, run, say, R};
use serde_json::Value;
use std::collections::HashMap;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

const LIVE_API: u16 = 8788;

pub struct Fleet {
    /// Every VM; deploy waits for all of their probes.
    pub nodes: Vec<String>,
    /// The VM that builds dialtone and runs the test suite (BUILD_NODE, default the first).
    pub builder: String,
    ssh: HashMap<String, String>,
    routes: HashMap<String, Vec<String>>,
}

impl Fleet {
    /// Read fleet.local.env: `KEY="value"` lines, `$HOME` expanded, `#` comments.
    pub fn load() -> R<Fleet> {
        let file = std::env::var("FLEET_FILE").unwrap_or_else(|_| "fleet.local.env".into());
        let text = std::fs::read_to_string(&file).map_err(|_| {
            format!("{file} not found: it lists this fleet's VMs (see the dev tasks section of README.md)")
        })?;
        let home = crate::util::home().to_string_lossy().replace('\\', "/");
        let mut vars: HashMap<String, String> = HashMap::new();
        for line in text.lines() {
            let line = line.trim();
            let Some((k, v)) = line.split_once('=') else {
                continue;
            };
            if line.starts_with('#') {
                continue;
            }
            let v = v.trim();
            let v = match v.strip_prefix('"') {
                Some(q) => q.split('"').next().unwrap_or(""),
                None => v.split(" #").next().unwrap_or("").trim(),
            };
            let v = v.replace("${HOME}", &home).replace("$HOME", &home);
            vars.insert(k.trim().trim_start_matches("export ").to_string(), v);
        }
        if let Some(p) = vars.get("FLEET_PATH").filter(|p| !p.is_empty()) {
            let sep = if cfg!(windows) { ";" } else { ":" };
            let path = std::env::var("PATH").unwrap_or_default();
            std::env::set_var("PATH", format!("{p}{sep}{path}"));
        }
        let nodes: Vec<String> = vars
            .get("FLEET_NODES")
            .map(|v| v.split_whitespace().map(String::from).collect())
            .unwrap_or_default();
        let first = nodes.first().ok_or("FLEET_NODES is not set")?.clone();
        let builder = vars.get("BUILD_NODE").cloned().unwrap_or(first);
        if !nodes.contains(&builder) {
            return Err(format!("BUILD_NODE {builder} is not in FLEET_NODES"));
        }
        let ssh = nodes
            .iter()
            .map(|n| {
                let key = format!("SSH_{}", n.replace('-', "_"));
                (
                    n.clone(),
                    vars.get(&key).cloned().unwrap_or_else(|| n.clone()),
                )
            })
            .collect();
        Ok(Fleet {
            nodes,
            builder,
            ssh,
            routes: HashMap::new(),
        })
    }

    /// For each VM, the first SSH alternative in SSH_<node> (`a|b|...`, SSH options followed by
    /// the destination) that answers, tried in parallel.
    pub fn pick_routes(&mut self) -> R<()> {
        let found = self.each(|node| {
            let alts: Vec<Vec<String>> = self.ssh[node]
                .split('|')
                .map(|a| a.split_whitespace().map(String::from).collect())
                .collect();
            // Two rounds: a VM that is slow to answer once should not fail a deploy.
            alts.iter()
                .chain(alts.iter())
                .find(|a| {
                    quietly(
                        Command::new("ssh")
                            .args(["-o", "ConnectTimeout=8", "-o", "BatchMode=yes"])
                            .args(*a)
                            .arg("exit 0"),
                    )
                })
                .cloned()
                .ok_or_else(|| format!("cannot reach {node} over SSH (tried: {})", self.ssh[node]))
        });
        for (node, route) in found {
            self.routes.insert(node, route?);
        }
        Ok(())
    }

    /// Run `f` for every VM at once; results in FLEET_NODES order.
    pub fn each<T: Send>(&self, f: impl Fn(&str) -> R<T> + Sync) -> Vec<(String, R<T>)> {
        std::thread::scope(|s| {
            let jobs: Vec<_> = self
                .nodes
                .iter()
                .map(|node| {
                    let f = &f;
                    (node.clone(), s.spawn(move || f(node)))
                })
                .collect();
            jobs.into_iter()
                .map(|(node, j)| (node, j.join().expect("fleet thread")))
                .collect()
        })
    }

    fn route(&self, node: &str) -> R<&Vec<String>> {
        self.routes
            .get(node)
            .ok_or_else(|| format!("no SSH route to {node}"))
    }

    /// Run a shell script on a VM (the script goes on stdin); returns stdout.
    pub fn sh(&self, node: &str, script: &str) -> R<String> {
        self.remote(node, script, Stdio::piped())
    }

    /// Run a shell script on a VM with its output shown here as it runs.
    pub fn sh_live(&self, node: &str, script: &str) -> R<()> {
        self.remote(node, script, Stdio::inherit()).map(|_| ())
    }

    fn remote(&self, node: &str, script: &str, stdout: Stdio) -> R<String> {
        let mut child = Command::new("ssh")
            .args(["-o", "ServerAliveInterval=30"])
            .args(self.route(node)?)
            .arg("/bin/sh -s")
            .stdin(Stdio::piped())
            .stdout(stdout)
            .stderr(Stdio::inherit())
            .spawn()
            .map_err(|e| format!("ssh {node}: {e}"))?;
        child
            .stdin
            .take()
            .expect("piped stdin")
            .write_all(script.as_bytes())
            .map_err(|e| format!("ssh {node}: {e}"))?;
        let out = child
            .wait_with_output()
            .map_err(|e| format!("ssh {node}: {e}"))?;
        if !out.status.success() {
            return Err(format!("script on {node} failed ({})", out.status));
        }
        Ok(String::from_utf8_lossy(&out.stdout).into_owned())
    }

    /// scp's options and destination for a VM.
    fn scp_route(&self, node: &str) -> R<(&[String], &str)> {
        let (dest, opts) = self.route(node)?.split_last().ok_or("empty SSH route")?;
        Ok((opts, dest))
    }

    /// Copy a local file (a relative path: scp reads `C:\...` as a host) to a path under the
    /// VM's home directory.
    pub fn copy_to(&self, node: &str, local: &Path, remote: &str) -> R<()> {
        let (opts, dest) = self.scp_route(node)?;
        run(Command::new("scp")
            .arg("-q")
            .args(opts)
            .arg(local)
            .arg(format!("{dest}:{remote}")))
    }

    pub fn copy_from(&self, node: &str, remote: &str, local: &Path) -> R<()> {
        let (opts, dest) = self.scp_route(node)?;
        run(Command::new("scp")
            .arg("-q")
            .args(opts)
            .arg(format!("{dest}:{remote}"))
            .arg(local))
    }

    /// Run `pre` (it puts the new binary at ~/.dialtone/deploy-new/dialtone), then install it
    /// and restart the VM's service; returns the installed binary's SHA-256.
    fn install(&self, node: &str, pre: &str) -> R<String> {
        let script = format!("set -e\nmkdir -p \"$HOME/.dialtone/deploy-new\"\n{pre}\n{INSTALL}");
        self.sh(node, &script)?
            .lines()
            .find_map(|l| l.strip_prefix("SHA256="))
            .map(|s| s.trim().to_string())
            .filter(|s| s.len() == 64)
            .ok_or_else(|| format!("install on {node} did not report its binary"))
    }

    /// The live mesh as the build VM's node sees it.
    fn mesh(&self) -> Value {
        let get = format!("curl -s -m 10 http://127.0.0.1:{LIVE_API}/api/mesh");
        self.sh(&self.builder, &get)
            .ok()
            .and_then(|s| serde_json::from_str(&s).ok())
            .unwrap_or(Value::Null)
    }
}

/// Everything a node needs to build and test dialtone: a C toolchain, pkg-config, git and curl
/// (from apt), and Rust through rustup with rustfmt and clippy. Each step is skipped when its
/// tool is already there.
const DEV_SETUP: &str = r#"
set -e
missing=""
for tool in cc pkg-config git curl; do
  command -v "$tool" >/dev/null || missing="$missing $tool"
done
if [ -n "$missing" ]; then
  if ! command -v apt-get >/dev/null; then
    echo "missing:$missing (install them, then run dev again; only apt is automated)" >&2
    exit 1
  fi
  apt="-o Acquire::Retries=5 -o Acquire::http::Timeout=30"
  sudo apt-get $apt update -qq
  sudo DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get $apt install -y -qq build-essential pkg-config git curl > /tmp/dialtone-dev-apt.log 2>&1 || { tail -n 20 /tmp/dialtone-dev-apt.log >&2; exit 1; }
fi
if [ ! -x "$HOME/.cargo/bin/rustup" ]; then
  curl -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal >/dev/null 2>&1
fi
"$HOME/.cargo/bin/rustup" component add rustfmt clippy >/dev/null 2>&1
echo "READY $("$HOME/.cargo/bin/rustc" --version), $(cc --version | head -n 1), $(git --version)"
"#;

/// `./dialtone.sh dev [node | user@host ...]`: set up every VM (or the ones named, from
/// FLEET_NODES or any SSH destination) for development, all at once.
pub fn dev(args: &[String]) -> R<()> {
    let mut fleet = match Fleet::load() {
        Ok(f) => f,
        Err(_) if !args.is_empty() => Fleet {
            nodes: Vec::new(),
            builder: String::new(),
            ssh: HashMap::new(),
            routes: HashMap::new(),
        },
        Err(e) => return Err(e),
    };
    if !args.is_empty() {
        for a in args {
            fleet.ssh.entry(a.clone()).or_insert_with(|| a.clone());
        }
        fleet.nodes = args.to_vec();
    }
    say(&format!(
        "Development toolchains on {}",
        fleet.nodes.join(" ")
    ));
    fleet.pick_routes()?;
    let mut failed = Vec::new();
    for (node, out) in fleet.each(|node| fleet.sh(node, DEV_SETUP)) {
        match out {
            Ok(out) => {
                let ready = out.lines().find_map(|l| l.strip_prefix("READY "));
                println!("    {node:<12} {}", ready.unwrap_or("?"));
            }
            Err(e) => {
                println!("    {node:<12} FAIL {e}");
                failed.push(node);
            }
        }
    }
    if failed.is_empty() {
        Ok(())
    } else {
        Err(format!("not set up: {}", failed.join(" ")))
    }
}

/// On the build VM: fresh sources from ~/dialtone-src.tgz, then __STEPS__ (the test suite, the
/// release build or both). The target directory stays between runs, so only what changed is
/// rebuilt.
const BUILD: &str = r#"
set -e
export PATH="$HOME/.cargo/bin:$PATH" CARGO_TARGET_DIR="$HOME/dialtone-target"
rm -rf "$HOME/dialtone-src"
mkdir -p "$HOME/dialtone-src"
tar --warning=no-unknown-keyword -xzf "$HOME/dialtone-src.tgz" -C "$HOME/dialtone-src"
cd "$HOME/dialtone-src"
__STEPS__
"#;

/// Install ~/.dialtone/deploy-new/dialtone, restart the node's service (`dialtone daemon
/// install` makes it), print its SHA-256.
const INSTALL: &str = r#"
d="$HOME/.dialtone"
mkdir -p "$d/bin"
if [ -f "$d/bin/dialtone" ]; then cp "$d/bin/dialtone" "$d/bin/dialtone.bak"; fi
chmod 755 "$d/deploy-new/dialtone"
mv "$d/deploy-new/dialtone" "$d/bin/dialtone"
systemctl --user restart dialtone
echo "SHA256=$(sha256sum "$d/bin/dialtone" | cut -d' ' -f1)"
"#;

/// On a VM: download the published binary for its architecture and refuse it unless its
/// SHA-256 matches.
const DOWNLOAD: &str = r#"
case "$(uname -m)" in aarch64|arm64) a=dialtone-linux-arm64 ;; *) a=dialtone-linux-x86_64 ;; esac
n="$HOME/.dialtone/deploy-new/dialtone"
curl -fsSL "RELEASES/$a" -o "$n"
want=$(curl -fsSL "RELEASES/$a.sha256" | cut -d' ' -f1)
if [ "$want" != "$(sha256sum "$n" | cut -d' ' -f1)" ]; then echo "the download does not match its published SHA-256" >&2; exit 1; fi
"#;

/// The sources the build VM needs, as a .tgz under target/. The path is relative: GNU tar
/// (Git Bash) reads `C:\...` as a remote host.
fn source_tgz() -> R<PathBuf> {
    std::fs::create_dir_all("target").map_err(|e| e.to_string())?;
    let src = PathBuf::from("target/dialtone-src.tgz");
    run(Command::new("tar")
        .env("COPYFILE_DISABLE", "1")
        .arg("czf")
        .arg(&src)
        .args([
            "Cargo.toml",
            "Cargo.lock",
            ".cargo",
            "src",
            "tests",
            "xtask",
        ]))?;
    Ok(src)
}

/// `./dialtone.sh deploy`: build on the build VM (or download release `--tag`), install on every
/// VM and wait for every probe. It runs no tests: `./dialtone.sh test` deploys what it tested.
pub fn deploy(args: &[String]) -> R<()> {
    let mut tag = None::<String>;
    let mut it = args.iter();
    while let Some(a) = it.next() {
        match a.as_str() {
            "--release" => tag = Some("latest".into()),
            "--tag" => tag = Some(it.next().ok_or("--tag needs a version")?.clone()),
            other => return Err(format!("unknown deploy option {other}")),
        }
    }
    let mut fleet = Fleet::load()?;
    fleet.pick_routes()?;
    if tag.is_none() {
        on_builder(&fleet, None, true)?;
    }
    roll_out(&fleet, tag.as_deref())
}

/// On the build VM, from the sources of this checkout: `cargo test --workspace <suite>` when
/// `suite` is given, then the release build when `release`.
pub fn on_builder(fleet: &Fleet, suite: Option<&[String]>, release: bool) -> R<()> {
    let b = &fleet.builder;
    say(&format!("Development toolchains on {b}"));
    fleet.sh(b, DEV_SETUP)?;
    let src = source_tgz()?;
    fleet.copy_to(b, &src, "dialtone-src.tgz")?;
    let _ = std::fs::remove_file(&src);
    let mut steps = Vec::new();
    if let Some(args) = suite {
        // Each argument single-quoted for the remote shell.
        let quoted: Vec<String> = args
            .iter()
            .map(|a| format!("'{}'", a.replace('\'', r"'\''")))
            .collect();
        steps.push(format!("cargo test --workspace {}", quoted.join(" ")));
    }
    if release {
        steps.push("cargo build --release -p dialtone".into());
    }
    let tests = match suite {
        Some([]) => Some("Test suite (unit, one node, three nodes, self-update)".to_string()),
        Some(args) => Some(format!("cargo test --workspace {}", args.join(" "))),
        None => None,
    };
    say(&match (tests, release) {
        (Some(t), true) => format!("{t} on {b}, then the release build"),
        (Some(t), false) => format!("{t} on {b}"),
        (None, _) => format!("Release build on {b}"),
    });
    fleet.sh_live(b, &BUILD.replace("__STEPS__", &steps.join("\n")))
}

/// Install the build VM's release build (or download release `tag`) on every VM; wait until
/// every probe passes on exactly that binary, then a full probe on each VM.
pub fn roll_out(fleet: &Fleet, tag: Option<&str>) -> R<()> {
    let installed = match tag {
        Some(tag) => {
            let releases = match tag {
                "latest" => "https://github.com/timcash/dialtone/releases/latest/download".into(),
                t => format!("https://github.com/timcash/dialtone/releases/download/{t}"),
            };
            say(&format!(
                "Download release {tag} on {}",
                fleet.nodes.join(" ")
            ));
            let pre = DOWNLOAD.replace("RELEASES", &releases);
            fleet.each(|node| fleet.install(node, &pre))
        }
        None => {
            let b = &fleet.builder;
            let built = PathBuf::from("target/dialtone-linux");
            if fleet.nodes.len() > 1 {
                fleet.copy_from(b, "dialtone-target/release/dialtone", &built)?;
            }
            say(&format!("Install on {}", fleet.nodes.join(" ")));
            let installed = fleet.each(|node| {
                if node == b {
                    let cp = "cp \"$HOME/dialtone-target/release/dialtone\" \"$HOME/.dialtone/deploy-new/dialtone\"";
                    fleet.install(node, cp)
                } else {
                    fleet.sh(node, "mkdir -p \"$HOME/.dialtone/deploy-new\"")?;
                    fleet.copy_to(node, &built, ".dialtone/deploy-new/dialtone")?;
                    fleet.install(node, "")
                }
            });
            let _ = std::fs::remove_file(&built);
            installed
        }
    };
    // node => SHA-256 of the binary installed on it; the gate waits for probes from exactly these.
    let mut expect: HashMap<String, String> = HashMap::new();
    for (node, sha) in installed {
        let sha = sha?;
        println!("    {node:<12} {}", &sha[..16]);
        expect.insert(node, sha);
    }

    say("Waiting for every VM's gossip probe to pass on the new build (up to 150s)");
    let mut last = String::new();
    let mut passed = false;
    for _ in 0..30 {
        let (ok, text) = report(&fleet.nodes, &expect, &fleet.mesh());
        last = text;
        if ok {
            passed = true;
            break;
        }
        crate::util::sleep(5.0);
    }
    print!("{last}");
    if !passed {
        return Err("not every VM passed its probe within 150s (FAIL lines above)".into());
    }

    // The store, pulls (a fresh blob from each peer) and programs, on every VM. Peers that just
    // restarted can miss a pull or two, so each full probe gets three tries.
    for node in &fleet.nodes {
        say(&format!("Full probe on {node}"));
        full_probe(fleet, node)?;
    }
    say("DEPLOYED: every VM passed its probe");
    Ok(())
}

fn full_probe(fleet: &Fleet, node: &str) -> R<()> {
    for attempt in 1..=3 {
        let out = fleet
            .sh(node, "\"$HOME/.dialtone/bin/dialtone\" probe --all")
            .unwrap_or_default();
        let p = serde_json::from_str::<Value>(&out).unwrap_or(Value::Null)["probe"].clone();
        for c in p["checks"].as_array().into_iter().flatten() {
            let ok = c["ok"].as_bool() == Some(true);
            println!(
                "    {:<4} {} {}",
                c["code"].as_str().unwrap_or("?"),
                if ok { "ok  " } else { "FAIL" },
                c["detail"].as_str().unwrap_or("")
            );
        }
        if p["total_count"].is_u64() && p["passed_count"] == p["total_count"] {
            return Ok(());
        }
        if attempt < 3 {
            crate::util::sleep(15.0);
        }
    }
    Err(format!("the full probe on {node} failed three times"))
}

/// Every VM's latest probe in the live mesh; fails unless all of them passed.
pub fn probes(fleet: &Fleet) -> R<()> {
    say("Latest probe of every VM in the live mesh");
    let (ok, text) = report(&fleet.nodes, &HashMap::new(), &fleet.mesh());
    print!("{text}");
    if ok {
        Ok(())
    } else {
        Err("not every VM's latest probe passed (FAIL lines above)".into())
    }
}

/// Every VM's latest probe, from the live mesh. Passes when every VM's probe passed and, for
/// each VM in `builds`, came from the binary with that SHA-256.
fn report(names: &[String], builds: &HashMap<String, String>, mesh: &Value) -> (bool, String) {
    let nodes: HashMap<&str, &Value> = mesh["nodes"]
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(|n| Some((n["node"].as_str()?, n)))
        .collect();
    let (mut all_ok, mut text) = (true, String::new());
    for name in names {
        let node = nodes.get(name.as_str());
        let p = node.map(|n| &n["probe"]).unwrap_or(&Value::Null);
        let build = p["build"]
            .as_str()
            .unwrap_or("")
            .trim_start_matches("sha256:");
        let fails: Vec<String> = p["checks"]
            .as_array()
            .into_iter()
            .flatten()
            .filter(|c| c["ok"].as_bool() != Some(true))
            .map(|c| {
                format!(
                    "{}: {}",
                    c["code"].as_str().unwrap_or("?"),
                    c["detail"].as_str().unwrap_or("")
                )
            })
            .collect();
        // Beats carry the first bytes of the build digest.
        let on_build = builds
            .get(name)
            .is_none_or(|sha| !build.is_empty() && sha.starts_with(build));
        let total = p["total_count"].as_u64().unwrap_or(0);
        let passed =
            node.is_some() && on_build && total > 0 && p["passed_count"].as_u64() == Some(total);
        all_ok &= passed;
        let pulls: Vec<String> = p["pulls"]
            .as_array()
            .into_iter()
            .flatten()
            .filter(|x| x["ok"].as_bool() == Some(true))
            .map(|x| {
                format!(
                    "{} {:.0}ms",
                    x["peer"].as_str().unwrap_or("?"),
                    x["ms"].as_f64().unwrap_or(0.0)
                )
            })
            .collect();
        let note = if node.is_none() {
            "  missing from the mesh"
        } else if !on_build {
            "  no probe from the new build yet"
        } else {
            ""
        };
        text.push_str(&format!(
            "    {:<12} {:<8} build {:<12} probe #{:<4} {}/{}  {}{}{}\n",
            name,
            node.and_then(|n| n["version"].as_str())
                .filter(|v| !v.is_empty())
                .map(|v| format!("v{v}"))
                .unwrap_or_else(|| "v?".into()),
            if build.is_empty() {
                "?"
            } else {
                &build[..build.len().min(12)]
            },
            p["cycle"],
            p["passed_count"],
            p["total_count"],
            if pulls.is_empty() {
                String::new()
            } else {
                format!("pulls: {}", pulls.join(" "))
            },
            note,
            if fails.is_empty() {
                String::new()
            } else {
                format!("  FAIL {}", fails.join("; "))
            },
        ));
    }
    (all_ok, text)
}
