//! `./dialtone.sh test`: every test, one command. With no options everything runs end to end on
//! the fleet's VMs; `--here` runs on this machine instead (what GitHub CI does); `--only` picks
//! one part.

use crate::fleet::{self, Fleet};
use crate::util::{cargo, run, say, R};

/// What `--only` can pick.
const PARTS: [&str; 5] = ["lint", "suite", "rejoin", "scale", "probes"];

struct Opts {
    here: bool,
    only: Option<String>,
    /// Scale nodes per VM, or on this machine with `--here`.
    scale: Option<usize>,
    secs: f64,
    /// After `--`: passed to `cargo test`.
    cargo: Vec<String>,
}

fn parse(args: &[String]) -> R<Opts> {
    let mut o = Opts {
        here: false,
        only: None,
        scale: None,
        secs: 30.0,
        cargo: Vec::new(),
    };
    let mut it = args.iter();
    while let Some(a) = it.next() {
        let mut value = |name: &str| it.next().ok_or(format!("{name} needs a value")).cloned();
        match a.as_str() {
            "--here" => o.here = true,
            "--only" => {
                let part = value("--only")?;
                if !PARTS.contains(&part.as_str()) {
                    return Err(format!("unknown part {part}: --only {}", PARTS.join(", ")));
                }
                o.only = Some(part);
            }
            "--scale" => {
                let n = value("--scale")?;
                o.scale = Some(n.parse().map_err(|_| format!("not a number: {n}"))?);
            }
            "--secs" => {
                let s = value("--secs")?;
                o.secs = s.parse().map_err(|_| format!("not a number: {s}"))?;
            }
            "--" => {
                o.cargo = it.cloned().collect();
                break;
            }
            other => {
                return Err(format!(
                    "unknown test option {other} (see ./dialtone.sh help)"
                ))
            }
        }
    }
    Ok(o)
}

pub fn test(args: &[String]) -> R<()> {
    let o = parse(args)?;
    match (o.only.as_deref(), o.here) {
        (Some("lint"), _) => crate::check_static(),
        (None, true) => {
            crate::check_static()?;
            suite_here(&o.cargo)?;
            say("Release build");
            crate::build()?;
            if let Some(n) = o.scale.filter(|&n| n > 0) {
                crate::scale::local(n, o.secs)?;
            }
            say("ALL TESTS PASSED on this machine");
            Ok(())
        }
        (Some("suite"), true) => suite_here(&o.cargo),
        (Some("scale"), true) => crate::scale::local(o.scale.unwrap_or(20), o.secs),
        (Some(part @ ("probes" | "rejoin")), true) => Err(format!(
            "--only {part} uses the VMs' live mesh: drop --here"
        )),
        (None, false) => {
            let mut fleet = Fleet::load()?;
            crate::check_static()?;
            fleet.pick_routes()?;
            fleet::on_builder(&fleet, Some(&o.cargo), true)?;
            // Before the install, which restarts every VM's node: that also makes them forget
            // the departed test node.
            crate::rejoin::run(&fleet)?;
            fleet::roll_out(&fleet, None)?;
            let n = o.scale.unwrap_or(10);
            if n > 0 {
                crate::scale::on_fleet(&fleet, n, o.secs)?;
            }
            say(&format!("ALL TESTS PASSED on {}", fleet.nodes.join(" ")));
            Ok(())
        }
        (Some(part), false) => {
            let mut fleet = Fleet::load()?;
            fleet.pick_routes()?;
            match part {
                "suite" => fleet::on_builder(&fleet, Some(&o.cargo), false),
                "scale" => crate::scale::on_fleet(&fleet, o.scale.unwrap_or(10), o.secs),
                "probes" => fleet::probes(&fleet),
                "rejoin" => {
                    crate::rejoin::run(&fleet)?;
                    fleet::restart_nodes(&fleet)
                }
                _ => unreachable!("parts are checked when parsed"),
            }
        }
        (Some(_), true) => unreachable!("parts are checked when parsed"),
    }
}

/// `cargo test --workspace` on this machine: unit tests, one node, three nodes, self-update.
fn suite_here(extra: &[String]) -> R<()> {
    say("Tests (unit, one node, three nodes, self-update)");
    run(cargo().args(["test", "--workspace"]).args(extra))
}
