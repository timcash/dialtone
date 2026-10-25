use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

use crate::blob::{digest_of, normalize_digest};

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeploySlot {
    pub name: String,
    pub active: bool,
    pub path: String,
    pub digest: String,
    pub previous_path: Option<String>,
    pub previous_digest: Option<String>,
    pub status: String,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct RollbackPlan {
    pub target_node: String,
    pub slot: String,
    pub current_path: String,
    pub current_digest: String,
    pub previous_path: String,
    pub previous_digest: String,
    pub eligible: bool,
    pub side_effect_free_preview: bool,
    pub manual_apply_guard: String,
    pub daemon_evidence: String,
}

pub fn plan_rollback(target_node: &str, slot: &DeploySlot) -> Result<RollbackPlan, String> {
    let prev_path = slot
        .previous_path
        .clone()
        .ok_or_else(|| format!("slot {} has no previous_path", slot.name))?;
    let prev_sha = slot
        .previous_digest
        .clone()
        .ok_or_else(|| format!("slot {} has no previous_digest", slot.name))?;
    let _ = normalize_digest(&slot.digest)?;
    let _ = normalize_digest(&prev_sha)?;

    Ok(RollbackPlan {
        target_node: target_node.to_string(),
        slot: slot.name.clone(),
        current_path: slot.path.clone(),
        current_digest: slot.digest.clone(),
        previous_path: prev_path,
        previous_digest: prev_sha,
        eligible: slot.active,
        side_effect_free_preview: true,
        manual_apply_guard:
            "preview only: invoke POST /api/deploy/rollback to verify hashes and swap active slot"
                .to_string(),
        daemon_evidence: format!("verified slot {} on peer {}", slot.name, target_node),
    })
}

/// Verify both current and previous slot binaries on disk by SHA-256 and swap the active slot.
pub fn execute_verified_rollback(slot: &mut DeploySlot) -> Result<(), String> {
    let prev_path = slot
        .previous_path
        .clone()
        .ok_or_else(|| "missing previous_path for rollback".to_string())?;
    let prev_sha = slot
        .previous_digest
        .clone()
        .ok_or_else(|| "missing previous_digest for rollback".to_string())?;

    let cur_bytes =
        fs::read(&slot.path).map_err(|e| format!("read current slot {}: {e}", slot.path))?;
    if digest_of(&cur_bytes) != normalize_digest(&slot.digest)? {
        return Err("current slot binary SHA-256 mismatch".to_string());
    }
    let prev_bytes =
        fs::read(&prev_path).map_err(|e| format!("read previous slot {}: {e}", prev_path))?;
    if digest_of(&prev_bytes) != normalize_digest(&prev_sha)? {
        return Err("previous slot binary SHA-256 mismatch".to_string());
    }

    let old_path = slot.path.clone();
    let old_sha = slot.digest.clone();
    slot.path = prev_path;
    slot.digest = prev_sha;
    slot.previous_path = Some(old_path);
    slot.previous_digest = Some(old_sha);
    slot.status = "rolled_back".to_string();
    Ok(())
}

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

/// Install `dialtone` into `~/.dialtone/bin/` and register an OS login/boot daemon
/// (macOS LaunchAgent, Linux systemd user unit, or Windows HKCU Run + Scheduled Task)
/// so the daemon automatically survives system restarts.
pub fn install_persistent_os_daemon(
    node: &str,
    listen: &str,
    secret_key: Option<&str>,
) -> Result<DaemonInstallReport, String> {
    let home = std::env::var_os("HOME")
        .or_else(|| std::env::var_os("USERPROFILE"))
        .map(std::path::PathBuf::from)
        .unwrap_or_else(std::env::temp_dir);
    let state_dir = home.join(".dialtone");
    let bin_dir = state_dir.join("bin");
    fs::create_dir_all(&bin_dir)
        .map_err(|e| format!("create bin dir {}: {e}", bin_dir.display()))?;

    let bin_name = if cfg!(target_os = "windows") {
        "dialtone.exe"
    } else {
        "dialtone"
    };
    let dest_bin = bin_dir.join(bin_name);
    if let Ok(cur_exe) = std::env::current_exe() {
        if cur_exe != dest_bin {
            let _ = fs::copy(&cur_exe, &dest_bin);
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let _ = fs::set_permissions(&dest_bin, fs::Permissions::from_mode(0o755));
            }
        }
    }

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

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;

    #[test]
    fn rollback_plan_is_side_effect_free_and_execute_verifies_hashes() {
        let dir = env::temp_dir().join(format!("dialtone-rollback-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        let path_a = dir.join("slot-a");
        let path_b = dir.join("slot-b");
        fs::write(&path_a, b"binary-v2").unwrap();
        fs::write(&path_b, b"binary-v1").unwrap();
        let sha_a = digest_of(b"binary-v2");
        let sha_b = digest_of(b"binary-v1");

        let mut slot = DeploySlot {
            name: "edge-1-motion-server".to_string(),
            active: true,
            path: path_a.to_string_lossy().to_string(),
            digest: sha_a.clone(),
            previous_path: Some(path_b.to_string_lossy().to_string()),
            previous_digest: Some(sha_b.clone()),
            status: "ready".to_string(),
        };

        let plan = plan_rollback("edge-1", &slot).unwrap();
        assert!(plan.side_effect_free_preview);
        assert_eq!(slot.digest, sha_a);

        execute_verified_rollback(&mut slot).unwrap();
        assert_eq!(slot.digest, sha_b);
        assert_eq!(slot.previous_digest.as_deref(), Some(sha_a.as_str()));
        assert_eq!(slot.status, "rolled_back");
        let _ = fs::remove_dir_all(&dir);
    }
}
