//! Dev tasks for dialtone. `./dialtone.sh <task>` and `cargo xtask <task>` both run this.

mod fleet;
mod rejoin;
mod release;
mod scale;
mod test;
mod util;

use util::{cargo, root, run, say, R};

const HELP: &str = "\
dialtone dev tasks (./dialtone.sh <task>, or cargo xtask <task>)

  test [opts]            every test, from one command. With no options, end to end on the VMs:
                         lint here; the build VM runs the whole cargo test suite and the release
                         build; the rejoin test; every VM installs that build and must pass its
                         gossip probe on it, then a full probe; then scale nodes on every VM as
                         one mesh
                           --here         run on this machine instead: lint, cargo test, release
                                          build (what GitHub CI runs)
                           --only lint    just the leak gate, domain language, fmt and clippy
                           --only suite   just cargo test (on the build VM, or --here)
                           --only rejoin  a node on the previous signed release joins the live
                                          channel by invite, goes away, rejoins and must update
                                          itself from what its peers' beats say
                           --only scale   just the scale test: convergence and traffic per node (on
                                          the VMs' installed build, or --here with this machine's
                                          release build, failing above SCALE_BUDGET_KBPS kbit/s)
                           --only probes  every VM's latest probe in the live mesh
                           --scale N      scale nodes per VM (default 10; 0 skips) or --here
                                          (default 20 with --only scale; none otherwise)
                           --secs S       how long the scale test measures traffic (default 30)
                           -- ARGS        passed to cargo test, e.g. -- --test e2e
  dev [node ...]         set up nodes for development (C toolchain, pkg-config, git, curl, Rust
                         with rustfmt and clippy): every VM, or the ones named (a VM's name or any
                         SSH destination)
  deploy [opts]          build on the build VM, install on every VM, wait for every probe to pass;
                         runs no tests (`test` deploys the build it tested)
                           --release      every VM downloads the latest GitHub release (SHA-256 checked)
                           --tag vX.Y.Z   ... or this release
  release-manifest V DIR sign DIR's dialtone-* binaries as release V (DIALTONE_RELEASE_KEY):
                         dialtone-release.json and .sig, which nodes verify updates with
  release-pubkey         the public key of DIALTONE_RELEASE_KEY

`test` (without --here), `dev` and `deploy` read the VMs from fleet.local.env (never committed).";

fn main() {
    std::env::set_current_dir(root()).expect("cd to the repository");
    let args: Vec<String> = std::env::args().skip(1).collect();
    let rest = args.get(1..).unwrap_or_default();
    let result = match args.first().map(String::as_str) {
        Some("test") => test::test(rest),
        Some("dev") => fleet::dev(rest),
        Some("deploy") => fleet::deploy(rest),
        Some("release-manifest") => release::manifest(rest),
        Some("release-pubkey") => release::pubkey(),
        Some("help" | "-h" | "--help") | None => {
            println!("{HELP}");
            Ok(())
        }
        Some(other) => Err(format!("unknown task {other} (see ./dialtone.sh help)")),
    };
    if let Err(e) = result {
        eprintln!("FAIL: {e}");
        std::process::exit(1);
    }
}

/// The checks that run no node: the leak gate, domain language, formatting and clippy.
pub fn check_static() -> R<()> {
    lint()?;
    say("Format");
    run(cargo().args(["fmt", "--all", "--check"]))?;
    say("Clippy");
    run(cargo().args([
        "clippy",
        "--workspace",
        "--all-targets",
        "--",
        "-D",
        "warnings",
    ]))
}

pub fn build() -> R<()> {
    run(cargo().args(["build", "--release", "-p", "dialtone"]))
}

fn lint() -> R<()> {
    use base64::Engine;
    say("Leak gate: no host names or NodeIds in tracked files or history");
    // The names themselves must not be in the repository either, so they are encoded.
    let forbidden = String::from_utf8(
        base64::engine::general_purpose::STANDARD
            .decode("cm9zaWV8bGVnaW9ufGJ1aWxkLTF8amV0c29ufHN0ZWFtZGVja3w2NDQ2MzczOTc4NzJ8MDAxMmQ5NjdkMTY1fGZiZDViM2I3ZTRlMg==")
            .expect("valid base64"),
    )
    .expect("ascii");
    let hits = util::output(
        std::process::Command::new("git").args(["grep", "-n", "-i", "-E", &forbidden]),
    )
    .unwrap_or_default();
    if !hits.is_empty() {
        hits.lines().take(5).for_each(|l| eprintln!("{l}"));
        return Err("a host name or NodeId is in a tracked file".into());
    }
    // The history that is or will be published: this branch and the remotes (a clone's own
    // old local branches and tags are not pushed).
    for search in ["-G", "--grep="] {
        let log = util::output(std::process::Command::new("git").args([
            "log",
            "HEAD",
            "--remotes",
            "-i",
            "--oneline",
            &format!("{search}{forbidden}"),
        ]))?;
        if !log.trim().is_empty() {
            return Err("a host name or NodeId is in git history".into());
        }
    }

    say("Domain language: one word per concept (goal.md), no banned synonyms in src/");
    // banned pattern (extended regex, case-insensitive) => word to use instead.
    // A line that must keep an old name (or names an iroh type) ends with `// compat`.
    let rules = [
        (
            r"heartbeat|(^|[^a-z0-9])hb([^a-z0-9]|$)|_hb([^a-z0-9]|$)|(^|[^a-z0-9])hb_",
            "beat",
        ),
        (r"telemetry|host_?resources?", "stats"),
        (r"fleet|swarm", "mesh"),
        (r"artifact", "blob"),
        (r"(^|[^a-z0-9])cas([^a-z0-9]|$)", "store"),
        (r"blob_?cache|cache_dir|cache_root", "store"),
        (
            r"sub_?process|managed_?process|supervised_?process|(^|[^a-z])worker",
            "process",
        ),
        (
            r"(^|[^0-9a-f])e2e([^0-9a-f]|$)|test[-_]tick|capability[-_ ]tick",
            "probe",
        ),
        (r"dashboard", "hud"),
        (r"endpoint_ids?", "node_id"),
        (r"seed_peers?|bootstrap", "seed / invite"),
        (r"password", "secret"),
        (r"invite[_ ]?tickets?", "invite"),
    ];
    let mut files: Vec<String> = Vec::new();
    for (dir, exts) in [
        ("src", &["rs"][..]),
        ("src/hud", &["html", "css", "js"][..]),
    ] {
        for e in std::fs::read_dir(dir)
            .map_err(|e| format!("{dir}: {e}"))?
            .flatten()
        {
            let p = e.path();
            let ok = p.extension().is_some_and(|x| exts.iter().any(|w| x == *w));
            if ok && !p.ends_with("h3_land.rs") {
                files.push(p.to_string_lossy().replace('\\', "/"));
            }
        }
    }
    files.sort();
    let mut clean = true;
    for (pattern, word) in rules {
        let out = std::process::Command::new("git")
            .args(["grep", "--no-index", "-n", "-i", "-E", pattern, "--"])
            .args(&files)
            .output()
            .map_err(|e| format!("git grep: {e}"))?;
        let text = String::from_utf8_lossy(&out.stdout);
        let hits: Vec<&str> = text.lines().filter(|l| !l.contains("// compat")).collect();
        if !hits.is_empty() {
            println!("    say \"{word}\" (pattern: {pattern}):");
            for h in hits.iter().take(5) {
                println!("      {}", h.chars().take(160).collect::<String>());
            }
            clean = false;
        }
    }
    if clean {
        Ok(())
    } else {
        Err("banned words in src/".into())
    }
}
