use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

pub fn render_os_service_unit(os_kind: &str, program_path: &Path, node: &str) -> String {
    let bin = program_path.display();
    if os_kind == "macos" {
        format!(
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\">\n<dict>\n  <key>Label</key>\n  <string>com.timcash.dialtone</string>\n  <key>ProgramArguments</key>\n  <array>\n    <string>{bin}</string>\n    <string>up</string>\n    <string>--node</string>\n    <string>{node}</string>\n  </array>\n  <key>RunAtLoad</key>\n  <true/>\n  <key>KeepAlive</key>\n  <true/>\n</dict>\n</plist>\n"
        )
    } else if os_kind == "windows" {
        format!("@echo off\r\n\"{bin}\" up --node {node}\r\n")
    } else {
        format!(
            "[Unit]\nDescription=Dialtone Iroh Mesh Daemon ({node})\nAfter=network-online.target\n\n[Service]\nEnvironmentFile=-/etc/dialtone/daemon.env\nExecStart={bin} up --node {node}\nRestart=always\nRestartSec=2\n\n[Install]\nWantedBy=default.target\n"
        )
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DaemonInstallReport {
    pub status: String,
    pub os: String,
    pub node: String,
    pub listen: String,
    pub program_path: String,
    pub service_unit_path: String,
    pub survives_restarts: bool,
    pub instructions: String,
}

const LAUNCHD_LABEL: &str = "com.timcash.dialtone";

/// A state directory other than the default one (`DIALTONE_STATE_DIR`: a second node on one
/// machine, or a trial). Such a node is never registered to start at login.
pub fn custom_state_dir() -> bool {
    std::env::var_os("DIALTONE_STATE_DIR").is_some()
}

/// Put this binary at `<state>/bin/dialtone`, unless it already runs from there. A binary that
/// is there already (perhaps running) is renamed aside first, which every OS allows, so a
/// running node keeps going until it is restarted.
pub fn place_binary(state_dir: &Path) -> Result<std::path::PathBuf, String> {
    let bin_dir = state_dir.join("bin");
    fs::create_dir_all(&bin_dir)
        .map_err(|e| format!("create bin dir {}: {e}", bin_dir.display()))?;
    let dest = bin_dir.join(format!("dialtone{}", std::env::consts::EXE_SUFFIX));
    let cur = std::env::current_exe().map_err(|e| format!("this binary's path: {e}"))?;
    let same = |a: &Path, b: &Path| match (fs::canonicalize(a), fs::canonicalize(b)) {
        (Ok(a), Ok(b)) => a == b,
        _ => false,
    };
    if same(&cur, &dest) {
        return Ok(dest);
    }
    let staged = dest.with_extension("new");
    fs::copy(&cur, &staged).map_err(|e| format!("copy to {}: {e}", staged.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&staged, fs::Permissions::from_mode(0o755))
            .map_err(|e| format!("chmod {}: {e}", staged.display()))?;
    }
    if dest.exists() {
        let aside = dest.with_extension("bak");
        let _ = fs::remove_file(&aside);
        fs::rename(&dest, &aside).map_err(|e| format!("move {} aside: {e}", dest.display()))?;
    }
    fs::rename(&staged, &dest).map_err(|e| format!("install {}: {e}", dest.display()))?;
    Ok(dest)
}

/// Start the installed login service now (it restarts a node it already runs).
pub fn start_service() -> Result<(), String> {
    let ok = |cmd: &mut std::process::Command| cmd.output().is_ok_and(|o| o.status.success());
    match std::env::consts::OS {
        "macos" => {
            let uid = std::process::Command::new("id")
                .arg("-u")
                .output()
                .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
                .map_err(|e| format!("id -u: {e}"))?;
            let plist = home_dir()
                .join("Library/LaunchAgents")
                .join(format!("{LAUNCHD_LABEL}.plist"));
            let domain = format!("gui/{uid}");
            // Unload first so a changed plist is read again.
            let _ = std::process::Command::new("launchctl")
                .args(["bootout", &format!("{domain}/{LAUNCHD_LABEL}")])
                .output();
            if ok(std::process::Command::new("launchctl").args([
                "bootstrap", // compat: launchctl's own subcommand
                &domain,
                &plist.to_string_lossy(),
            ])) {
                Ok(())
            } else {
                Err("launchctl could not load the login service".into())
            }
        }
        "windows" => {
            if ok(std::process::Command::new("schtasks").args(["/Run", "/TN", "DialtoneDaemon"])) {
                Ok(())
            } else {
                Err("schtasks /Run DialtoneDaemon failed".into())
            }
        }
        _ => {
            let _ = std::process::Command::new("systemctl")
                .args(["--user", "daemon-reload"])
                .output();
            if ok(std::process::Command::new("systemctl").args([
                "--user",
                "restart",
                "dialtone.service",
            ])) {
                Ok(())
            } else {
                Err("systemctl --user restart dialtone failed (no systemd user session?)".into())
            }
        }
    }
}

/// Start `bin up` in the background, detached from this terminal, logging to `log`. For nodes
/// that are not installed at login.
pub fn start_detached(bin: &Path, node: &str, listen: &str, log: &Path) -> Result<u32, String> {
    let out = fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(log)
        .map_err(|e| format!("open {}: {e}", log.display()))?;
    let err = out.try_clone().map_err(|e| e.to_string())?;
    let mut cmd = std::process::Command::new(bin);
    cmd.args(["up", "--node", node, "--listen", listen])
        .stdin(std::process::Stdio::null())
        .stdout(out)
        .stderr(err);
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        cmd.process_group(0);
    }
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        cmd.creation_flags(CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW);
    }
    cmd.spawn()
        .map(|c| c.id())
        .map_err(|e| format!("start {}: {e}", bin.display()))
}

fn home_dir() -> std::path::PathBuf {
    std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(std::path::PathBuf::from)
        .unwrap_or_else(std::env::temp_dir)
}

/// Install `dialtone` into `<state>/bin/` and register an OS login/boot daemon (macOS
/// LaunchAgent, Linux systemd user unit, or Windows HKCU Run + Scheduled Task) so the node
/// starts at login. Refused for a `DIALTONE_STATE_DIR` other than the default: a login service
/// is one per user.
pub fn install_persistent_os_daemon(
    node: &str,
    listen: &str,
    secret_key: Option<&str>,
) -> Result<DaemonInstallReport, String> {
    if custom_state_dir() {
        return Err(
            "DIALTONE_STATE_DIR is set: a node with its own state directory is not installed at login"
                .into(),
        );
    }
    let home = home_dir();
    let state_dir = crate::mesh::default_state_dir();
    let dest_bin = place_binary(&state_dir)?;

    let log_path = state_dir.join("daemon.log");
    let os_kind = std::env::consts::OS;

    if let Some(sec) = secret_key.filter(|s| !s.trim().is_empty()) {
        let cfg_path = state_dir.join("config.json");
        let mut cfg = crate::mesh::load_mesh_config(&cfg_path);
        cfg.node_name = Some(node.to_string());
        cfg.listen_addr = Some(listen.to_string());
        let ch = cfg.channel.clone();
        cfg.apply_secret(&ch, sec.trim());
        let _ = crate::mesh::save_mesh_config(&cfg_path, &cfg);
    }

    if os_kind == "macos" {
        let agents_dir = home.join("Library").join("LaunchAgents");
        fs::create_dir_all(&agents_dir)
            .map_err(|e| format!("create LaunchAgents dir {}: {e}", agents_dir.display()))?;
        let plist_path = agents_dir.join("com.timcash.dialtone.plist");
        let args_xml = format!(
            "    <string>{}</string>\n    <string>up</string>\n    <string>--node</string>\n    <string>{}</string>\n    <string>--listen</string>\n    <string>{}</string>",
            dest_bin.display(),
            node,
            listen
        );
        let plist = format!(
            "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\">\n<dict>\n  <key>Label</key>\n  <string>com.timcash.dialtone</string>\n  <key>ProgramArguments</key>\n  <array>\n{args_xml}\n  </array>\n  <key>RunAtLoad</key>\n  <true/>\n  <key>KeepAlive</key>\n  <true/>\n  <key>StandardOutPath</key>\n  <string>{}</string>\n  <key>StandardErrorPath</key>\n  <string>{}</string>\n</dict>\n</plist>\n",
            log_path.display(),
            log_path.display()
        );
        fs::write(&plist_path, plist)
            .map_err(|e| format!("write LaunchAgent {}: {e}", plist_path.display()))?;
        if std::env::var_os("DIALTONE_STATE_DIR").is_none() {
            let _ = std::process::Command::new("launchctl")
                .args(["load", "-w", &plist_path.to_string_lossy()])
                .output();
        }
        Ok(DaemonInstallReport {
            status: "installed".to_string(),
            os: os_kind.to_string(),
            node: node.to_string(),
            listen: listen.to_string(),
            program_path: dest_bin.to_string_lossy().to_string(),
            service_unit_path: plist_path.to_string_lossy().to_string(),
            survives_restarts: true,
            instructions: format!(
                "macOS LaunchAgent installed at {} (KeepAlive=true, RunAtLoad=true)",
                plist_path.display()
            ),
        })
    } else if os_kind == "windows" {
        let cmd_path = state_dir.join("start-dialtone.cmd");
        let cmd_line = format!(
            "\"{}\" up --node {} --listen {}",
            dest_bin.display(),
            node,
            listen
        );
        let script = format!(
            "@echo off\r\n:loop\r\n{cmd_line} >> \"{}\" 2>&1\r\ntimeout /t 2 /nobreak >nul\r\ngoto loop\r\n",
            log_path.display()
        );
        fs::write(&cmd_path, script)
            .map_err(|e| format!("write startup script {}: {e}", cmd_path.display()))?;
        let headless_tr = format!(
            "conhost.exe --headless cmd.exe /c \"{}\"",
            cmd_path.display()
        );
        let _ = std::process::Command::new("reg")
            .args([
                "add",
                r"HKCU\Software\Microsoft\Windows\CurrentVersion\Run",
                "/v",
                "DialtoneDaemon",
                "/t",
                "REG_SZ",
                "/d",
                &headless_tr,
                "/f",
            ])
            .output();
        let _ = std::process::Command::new("schtasks")
            .args([
                "/Create",
                "/F",
                "/SC",
                "ONLOGON",
                "/TN",
                "DialtoneDaemon",
                "/TR",
                &headless_tr,
            ])
            .output();
        Ok(DaemonInstallReport {
            status: "installed".to_string(),
            os: os_kind.to_string(),
            node: node.to_string(),
            listen: listen.to_string(),
            program_path: dest_bin.to_string_lossy().to_string(),
            service_unit_path: cmd_path.to_string_lossy().to_string(),
            survives_restarts: true,
            instructions: format!(
                "Windows Startup Registry (HKCU\\...\\Run\\DialtoneDaemon) & Scheduled Task (headless auto-restart loop) registered via {}",
                cmd_path.display()
            ),
        })
    } else {
        let systemd_dir = home.join(".config").join("systemd").join("user");
        fs::create_dir_all(&systemd_dir)
            .map_err(|e| format!("create systemd user dir {}: {e}", systemd_dir.display()))?;
        let unit_path = systemd_dir.join("dialtone.service");
        let exec_start = format!(
            "{} up --node {} --listen {}",
            dest_bin.display(),
            node,
            listen
        );
        let unit = format!(
            "[Unit]\nDescription=Dialtone Iroh Mesh Daemon ({node})\nAfter=network-online.target\n\n[Service]\nEnvironmentFile=-/etc/dialtone/daemon.env\nExecStart={exec_start}\nRestart=always\nRestartSec=2\n\n[Install]\nWantedBy=default.target\n"
        );
        fs::write(&unit_path, unit)
            .map_err(|e| format!("write systemd unit {}: {e}", unit_path.display()))?;
        let _ = std::process::Command::new("systemctl")
            .args(["--user", "daemon-reload"])
            .output();
        let _ = std::process::Command::new("systemctl")
            .args(["--user", "enable", "dialtone.service"])
            .output();
        Ok(DaemonInstallReport {
            status: "installed".to_string(),
            os: os_kind.to_string(),
            node: node.to_string(),
            listen: listen.to_string(),
            program_path: dest_bin.to_string_lossy().to_string(),
            service_unit_path: unit_path.to_string_lossy().to_string(),
            survives_restarts: true,
            instructions: format!(
                "Linux systemd user service installed at {} (Restart=always)",
                unit_path.display()
            ),
        })
    }
}
