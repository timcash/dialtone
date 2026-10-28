//! The fleet: hosts from fleet.local.env, PowerShell over SSH, deploy and the probe report.

use crate::util::{self, home, output, quietly, release_bin, run, say, R};
use base64::Engine;
use serde_json::Value;
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::process::Command;

const LIVE_API: u16 = 8788;

pub struct Fleet {
    /// Every node; deploy waits for all of their probes.
    pub nodes: Vec<String>,
    /// This machine's node.
    pub local: String,
    launchd: String,
    /// Windows nodes reached over SSH (from macOS the first compiles the Windows binary).
    pub windows: Vec<String>,
    ssh: HashMap<String, String>,
    routes: HashMap<String, Vec<String>>,
}

impl Fleet {
    /// Read fleet.local.env: `KEY="value"` lines, `$HOME` expanded, `#` comments.
    pub fn load() -> R<Fleet> {
        let file = std::env::var("FLEET_FILE").unwrap_or_else(|_| "fleet.local.env".into());
        let text = std::fs::read_to_string(&file).map_err(|_| {
            format!("{file} not found: it lists this fleet's hosts (see the deploy section of README.md)")
        })?;
        let home = home().to_string_lossy().replace('\\', "/");
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
        let words = |k: &str| -> Vec<String> {
            vars.get(k)
                .map(|v| v.split_whitespace().map(String::from).collect())
                .unwrap_or_default()
        };
        let local = vars
            .get("LOCAL_NODE")
            .cloned()
            .ok_or("LOCAL_NODE is not set")?;
        let windows = words("WINDOWS_NODES");
        let ssh = windows
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
            nodes: words("FLEET_NODES"),
            local,
            launchd: vars.get("LOCAL_LAUNCHD").cloned().unwrap_or_default(),
            windows,
            ssh,
            routes: HashMap::new(),
        })
    }

    /// For each Windows node, the first SSH alternative in SSH_<node> (`a|b|...`, SSH options
    /// followed by the destination) that answers, tried in parallel per node.
    pub fn pick_routes(&mut self) -> R<()> {
        let found: Vec<(String, Option<Vec<String>>)> = std::thread::scope(|s| {
            let jobs: Vec<_> = self
                .windows
                .iter()
                .map(|node| {
                    let alts = self.ssh[node].clone();
                    s.spawn(move || {
                        let alts: Vec<Vec<String>> = alts
                            .split('|')
                            .map(|a| a.split_whitespace().map(String::from).collect())
                            .collect();
                        // Two rounds: a jump host that is slow to answer once should not fail a deploy.
                        let hit = alts.iter().chain(alts.iter()).find(|a| {
                            quietly(
                                Command::new("ssh")
                                    .args(["-o", "ConnectTimeout=5", "-o", "BatchMode=yes"])
                                    .args(*a)
                                    .arg("exit 0"),
                            )
                        });
                        (node.clone(), hit.cloned())
                    })
                })
                .collect();
            jobs.into_iter()
                .map(|j| j.join().expect("route thread"))
                .collect()
        });
        for (node, route) in found {
            let route = route.ok_or_else(|| {
                format!("cannot reach {node} over SSH (tried: {})", self.ssh[&node])
            })?;
            self.routes.insert(node, route);
        }
        Ok(())
    }

    fn is_local(&self, node: &str) -> bool {
        cfg!(windows) && node == self.local
    }

    /// Run PowerShell on a Windows node (this machine, when it is the local node); returns stdout.
    pub fn win(&self, node: &str, script: &str) -> R<String> {
        let script = format!("$ProgressPreference = 'SilentlyContinue'; {script}");
        let utf16: Vec<u8> = script.encode_utf16().flat_map(u16::to_le_bytes).collect();
        let b64 = base64::engine::general_purpose::STANDARD.encode(utf16);
        let ps = ["-NoProfile", "-NonInteractive", "-EncodedCommand", &b64];
        if self.is_local(node) {
            return output(Command::new("powershell.exe").args(ps));
        }
        let route = self
            .routes
            .get(node)
            .ok_or_else(|| format!("no SSH route to {node}"))?;
        output(
            Command::new("ssh")
                .args(route)
                .arg(format!("powershell {}", ps.join(" "))),
        )
    }

    /// scp's options and destination for a node.
    fn scp_route(&self, node: &str) -> R<(&[String], &str)> {
        let route = self
            .routes
            .get(node)
            .ok_or_else(|| format!("no SSH route to {node}"))?;
        let (dest, opts) = route.split_last().ok_or("empty SSH route")?;
        Ok((opts, dest))
    }

    /// Copy a local file to a path under the node's home directory.
    pub fn copy_to(&self, node: &str, local: &Path, remote: &str) -> R<()> {
        if self.is_local(node) {
            return std::fs::copy(local, home().join(remote))
                .map(|_| ())
                .map_err(|e| e.to_string());
        }
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

    /// Run `pre` (it puts the new binary at .dialtone\deploy-new\dialtone.exe), then install it
    /// as the node's daemon; returns the installed binary's SHA-256.
    fn win_install(&self, node: &str, pre: &str) -> R<String> {
        let script = format!(
            "$ErrorActionPreference = 'Stop'; New-Item -ItemType Directory -Force -Path \"$env:USERPROFILE\\.dialtone\\deploy-new\" | Out-Null; {pre} {}",
            WIN_INSTALL.replace("NODE", node)
        );
        let out = self.win(node, &script)?;
        out.lines()
            .find_map(|l| l.strip_prefix("SHA256="))
            .map(|s| s.trim().to_string())
            .filter(|s| s.len() == 64)
            .ok_or_else(|| format!("install on {node} did not report its binary"))
    }
}

/// Stop the node, install .dialtone\deploy-new\dialtone.exe, restart it at logon, print its SHA-256.
const WIN_INSTALL: &str = r#"
  $d = "$env:USERPROFILE\.dialtone"
  cmd.exe /c "schtasks /End /TN DialtoneDaemon >nul 2>&1"
  # Only this node: other dialtone.exe on the machine (another state directory) keep running.
  Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq "$d\bin\dialtone.exe" } | ForEach-Object { cmd.exe /c "taskkill /PID $($_.ProcessId) /T /F >nul 2>&1" }
  Get-CimInstance Win32_Process | Where-Object { $_.CommandLine -like "*start-dialtone.cmd*" -or $_.CommandLine -like "*\.dialtone\run\*" } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
  Start-Sleep -Milliseconds 800
  New-Item -ItemType Directory -Force -Path "$d\bin" | Out-Null
  if (Test-Path "$d\bin\dialtone.exe") { Move-Item "$d\bin\dialtone.exe" "$d\bin\dialtone.exe.bak" -Force }
  Move-Item "$d\deploy-new\dialtone.exe" "$d\bin\dialtone.exe" -Force
  $cmd = "$d\start-dialtone.cmd"
  Set-Content -Path $cmd -Encoding Ascii -Value ("@echo off`r`n:loop`r`n`"%USERPROFILE%\.dialtone\bin\dialtone.exe`" up --node NODE --listen 127.0.0.1:8788 >> `"%USERPROFILE%\.dialtone\daemon.stdout.log`" 2>> `"%USERPROFILE%\.dialtone\daemon.stderr.log`"`r`ntimeout /t 2 /nobreak >nul`r`ngoto loop`r`n")
  schtasks /Create /F /SC ONLOGON /TN DialtoneDaemon /TR "conhost.exe --headless cmd.exe /c `"$cmd`"" | Out-Null
  schtasks /Run /TN DialtoneDaemon | Out-Null
  Write-Output ("SHA256=" + (Get-FileHash "$d\bin\dialtone.exe" -Algorithm SHA256).Hash.ToLower())
"#;

/// On a Windows node: download the published .exe and refuse it unless its SHA-256 matches.
const WIN_DOWNLOAD: &str = r#"
$n = "$env:USERPROFILE\.dialtone\deploy-new\dialtone.exe"
curl.exe -fsSL "URL" -o $n
if ($LASTEXITCODE -ne 0) { throw "download failed" }
$want = ((curl.exe -fsSL "URL.sha256") -split '\s+')[0].ToLower()
if ($want -ne (Get-FileHash $n -Algorithm SHA256).Hash.ToLower()) { throw "the download does not match its published SHA-256" }
"#;

pub fn deploy(args: &[String]) -> R<()> {
    let (mut skip_check, mut tag) = (false, None::<String>);
    let mut it = args.iter();
    while let Some(a) = it.next() {
        match a.as_str() {
            "--skip-check" => skip_check = true,
            "--release" => tag = Some("latest".into()),
            "--tag" => tag = Some(it.next().ok_or("--tag needs a version")?.clone()),
            other => return Err(format!("unknown deploy option {other}")),
        }
    }
    let mut fleet = Fleet::load()?;
    let releases = match tag.as_deref() {
        None | Some("latest") => {
            "https://github.com/timcash/dialtone/releases/latest/download".to_string()
        }
        Some(t) => format!("https://github.com/timcash/dialtone/releases/download/{t}"),
    };
    let asset = if cfg!(windows) {
        "dialtone-windows-x86_64.exe"
    } else if cfg!(target_os = "macos") && cfg!(target_arch = "aarch64") {
        "dialtone-macos-arm64"
    } else {
        return Err("deploy runs from macOS (Apple silicon) or Windows".into());
    };

    // Find SSH routes while the local build or download runs (an offline route costs 5s).
    let local_node = fleet.local.clone();
    let (local, routes) = std::thread::scope(|s| -> R<(PathBuf, Fleet)> {
        let routes = s.spawn(move || fleet.pick_routes().map(|_| fleet));
        let binary = if let Some(tag) = &tag {
            say(&format!("Download release {tag} for {local_node}"));
            download(&releases, asset)?
        } else {
            if skip_check {
                say("Release build");
                crate::build()?;
            } else {
                crate::check()?;
            }
            release_bin()
        };
        let fleet = routes.join().expect("route thread")?;
        Ok((binary, fleet))
    })?;
    fleet = routes;

    // node => SHA-256 of the binary installed on it; the gate waits for probes from exactly these.
    let mut expect: HashMap<String, String> = HashMap::new();
    say(&format!("Install on {}", fleet.local));
    let sha = if cfg!(windows) {
        fleet.win_install(
            &fleet.local,
            &format!(
                "Copy-Item '{}' \"$env:USERPROFILE\\.dialtone\\deploy-new\\dialtone.exe\";",
                local.display()
            ),
        )?
    } else {
        let bin = home().join(".dialtone/bin");
        std::fs::create_dir_all(&bin).map_err(|e| e.to_string())?;
        let installed = bin.join("dialtone");
        if installed.exists() {
            std::fs::copy(&installed, bin.join("dialtone.bak")).map_err(|e| e.to_string())?;
        }
        std::fs::copy(&local, bin.join("dialtone.new")).map_err(|e| e.to_string())?;
        std::fs::rename(bin.join("dialtone.new"), &installed).map_err(|e| e.to_string())?;
        let uid = output(Command::new("id").arg("-u"))?;
        run(Command::new("launchctl").args([
            "kickstart",
            "-k",
            &format!("gui/{}/{}", uid.trim(), fleet.launchd),
        ]))?;
        util::sha256_file(&installed)?
    };
    println!("    {} {}", fleet.local, &sha[..16]);
    expect.insert(fleet.local.clone(), sha);

    let mkdir = "New-Item -ItemType Directory -Force -Path \"$env:USERPROFILE\\.dialtone\\deploy-new\" | Out-Null";
    if let Some(tag) = &tag {
        say(&format!(
            "Download release {tag} on {}",
            fleet.windows.join(" ")
        ));
        let download =
            WIN_DOWNLOAD.replace("URL", &format!("{releases}/dialtone-windows-x86_64.exe"));
        for node in &fleet.windows {
            let sha = fleet.win_install(node, &download)?;
            println!("    {node} {}  (SHA-256 matches the release)", &sha[..16]);
            expect.insert(node.clone(), sha);
        }
    } else if cfg!(windows) {
        // This machine built the Windows binary already.
        say(&format!("Install on {}", fleet.windows.join(" ")));
        for node in &fleet.windows {
            fleet.win(node, mkdir)?;
            fleet.copy_to(node, &local, ".dialtone/deploy-new/dialtone.exe")?;
            let sha = fleet.win_install(node, "")?;
            println!("    {node} {}", &sha[..16]);
            expect.insert(node.clone(), sha);
        }
    } else if let Some(builder) = fleet.windows.first() {
        say(&format!(
            "Build on {builder}, install on {}",
            fleet.windows.join(" ")
        ));
        let tmp = std::env::temp_dir();
        let src = tmp.join("dialtone-src.tgz");
        run(Command::new("tar")
            .env("COPYFILE_DISABLE", "1")
            .arg("czf")
            .arg(&src)
            .args(["Cargo.toml", "Cargo.lock", ".cargo", "src", "xtask"]))?;
        fleet.copy_to(builder, &src, ".dialtone/dialtone-src.tgz")?;
        fleet.win(
            builder,
            r#"
      $ErrorActionPreference = "Stop"
      $b = "$env:USERPROFILE\.dialtone\fleet-build"
      New-Item -ItemType Directory -Force -Path $b | Out-Null
      tar.exe -xzf "$env:USERPROFILE\.dialtone\dialtone-src.tgz" -C $b
      Set-Location $b
      & "$env:USERPROFILE\.cargo\bin\cargo.exe" build --release -p dialtone
      if ($LASTEXITCODE -ne 0) { throw "cargo build failed" }"#,
        )?;
        let exe = tmp.join("dialtone-windows.exe");
        fleet.copy_from(
            builder,
            ".dialtone/fleet-build/target/release/dialtone.exe",
            &exe,
        )?;
        for node in &fleet.windows {
            fleet.win(node, mkdir)?;
            fleet.copy_to(node, &exe, ".dialtone/deploy-new/dialtone.exe")?;
            let sha = fleet.win_install(node, "")?;
            println!("    {node} {}", &sha[..16]);
            expect.insert(node.clone(), sha);
        }
        let _ = std::fs::remove_file(src);
        let _ = std::fs::remove_file(exe);
    }

    say("Waiting for every node's gossip probe to pass on the new build (up to 150s)");
    let mut last = String::new();
    let mut passed = false;
    for _ in 0..30 {
        let (ok, text) = report(&fleet.nodes, &expect);
        last = text;
        if ok {
            passed = true;
            break;
        }
        util::sleep(5.0);
    }
    print!("{last}");
    if !passed {
        return Err("not every node passed its probe within 150s (FAIL lines above)".into());
    }

    // The store, pulls (a fresh blob from each peer) and programs, from this node.
    // Peers that just restarted can miss a pull or two, so the full probe gets three tries.
    say(&format!("Full probe on {}", fleet.local));
    let bin = home().join(format!(".dialtone/bin/dialtone{}", util::EXE));
    for attempt in 1..=3 {
        let out = output(Command::new(&bin).args(["probe", "--all"])).unwrap_or_default();
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
            say("DEPLOYED: every node passed its probe");
            return Ok(());
        }
        if attempt < 3 {
            util::sleep(15.0);
        }
    }
    Err(format!(
        "the full probe on {} failed three times",
        fleet.local
    ))
}

/// Download a release asset and check it against its published SHA-256.
fn download(releases: &str, asset: &str) -> R<PathBuf> {
    let path = std::env::temp_dir().join(asset);
    run(Command::new("curl")
        .args(["-fsSL", &format!("{releases}/{asset}"), "-o"])
        .arg(&path))?;
    let want = output(Command::new("curl").args(["-fsSL", &format!("{releases}/{asset}.sha256")]))?;
    if want.split_whitespace().next() != Some(util::sha256_file(&path)?.as_str()) {
        return Err(format!(
            "the downloaded {asset} does not match its published SHA-256"
        ));
    }
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755))
            .map_err(|e| e.to_string())?;
    }
    let help = output(Command::new(&path).arg("--help")).unwrap_or_default();
    println!(
        "    {}  (SHA-256 matches the release)",
        help.lines().next().unwrap_or("")
    );
    Ok(path)
}

pub fn report_live() -> R<()> {
    let fleet = Fleet::load()?;
    print!("{}", report(&fleet.nodes, &HashMap::new()).1);
    Ok(())
}

/// Every node's latest probe, from this machine's node. Passes when every node's probe passed
/// and, for each node in `builds`, came from the binary with that SHA-256.
fn report(names: &[String], builds: &HashMap<String, String>) -> (bool, String) {
    let mesh = util::get(LIVE_API, "/api/mesh").unwrap_or(Value::Null);
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
            "    {:<8} build {:<12} probe #{:<4} {}/{}  {}{}{}\n",
            name,
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
