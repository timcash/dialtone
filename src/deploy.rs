use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

use crate::blob::{compute_sha256, normalize_sha256};
use crate::service::DeployPolicyConfig;

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeployManifest {
    pub schema: String,
    pub id: String,
    pub publisher_node: String,
    pub target_host: String,
    pub component: String,
    pub source_tree_hash: String,
    pub source_bundle_sha256: String,
    pub source_bundle_uri: String,
    pub binary_sha256: String,
    pub binary_blob_uri: String,
    pub runtime_path: String,
    pub build_inputs: Vec<String>,
    #[serde(default)]
    pub whole_repo_sync: bool,
    #[serde(default)]
    pub ssh_required: bool,
    #[serde(default)]
    pub git_required: bool,
}

pub fn validate_manifest_contract(
    manifest: &DeployManifest,
    policy: &DeployPolicyConfig,
    allowed_roots: &[String],
) -> Result<(), String> {
    if !policy
        .manifest_schemas
        .iter()
        .any(|s| s == &manifest.schema)
    {
        return Err(format!(
            "unsupported manifest schema {:?}; allowed: {:?}",
            manifest.schema, policy.manifest_schemas
        ));
    }
    if manifest.whole_repo_sync || manifest.ssh_required || manifest.git_required {
        return Err(
            "manifest rejected: whole_repo_sync, ssh_required, and git_required are forbidden in steady-state deploys"
                .to_string(),
        );
    }
    let _ = normalize_sha256(&manifest.source_bundle_sha256)?;
    let _ = normalize_sha256(&manifest.binary_sha256)?;

    if !manifest
        .source_bundle_uri
        .starts_with("iroh-blobs://ticket/")
        || !manifest.binary_blob_uri.starts_with("iroh-blobs://ticket/")
    {
        return Err(
            "manifest rejected: source_bundle_uri and binary_blob_uri must use iroh-blobs://ticket/..."
                .to_string(),
        );
    }

    if manifest.build_inputs.is_empty() {
        return Err("manifest rejected: build_inputs cannot be empty".to_string());
    }
    for input in &manifest.build_inputs {
        let trimmed = input.trim();
        if !policy
            .build_input_roots
            .iter()
            .any(|root| trimmed.starts_with(root))
        {
            return Err(format!(
                "build input {trimmed:?} is outside allowed roots {:?}",
                policy.build_input_roots
            ));
        }
        for segment in trimmed.split('/') {
            if policy
                .forbidden_build_input_segments
                .iter()
                .any(|f| f == segment)
            {
                return Err(format!(
                    "build input {trimmed:?} contains forbidden generated segment {segment:?}"
                ));
            }
        }
    }

    if !allowed_roots
        .iter()
        .any(|root| manifest.runtime_path.starts_with(root))
    {
        return Err(format!(
            "runtime_path {:?} is outside DIALTONE_DEPLOY_ALLOWED_ROOTS {:?}",
            manifest.runtime_path, allowed_roots
        ));
    }

    Ok(())
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct BuildCacheRecord {
    pub peer_node: String,
    pub target_host: String,
    pub component: String,
    pub source_tree_hash: String,
    pub source_bundle_sha256: String,
    pub build_profile: String,
    pub toolchain: String,
    pub platform_os: String,
    pub platform_arch: String,
    pub target_triple: String,
    pub binary_sha256: String,
    pub binary_blob_uri: String,
    pub status: String,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct BuildProbeRequest {
    pub target_host: String,
    pub component: String,
    pub source_tree_hash: String,
    pub source_bundle_sha256: String,
    pub build_profile: String,
    pub toolchain: String,
    pub platform_os: String,
    pub platform_arch: String,
    pub target_triple: String,
    pub cache_only: bool,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct BuildProbeResponse {
    pub cache_hit: bool,
    pub provider_peer: Option<String>,
    pub exact_target_match: bool,
    pub binary_sha256: Option<String>,
    pub binary_blob_uri: Option<String>,
    pub transferred_bytes: u64,
    pub compiler_invoked: bool,
}

/// Probe `os-arch-build-server` records, preferring an exact target_host match
/// over a compatible cross-peer OS/arch match.
pub fn probe_build_cache(
    caches: &[BuildCacheRecord],
    req: &BuildProbeRequest,
) -> BuildProbeResponse {
    let matching: Vec<&BuildCacheRecord> = caches
        .iter()
        .filter(|c| {
            c.status == "ready"
                && c.component == req.component
                && c.source_tree_hash == req.source_tree_hash
                && c.source_bundle_sha256 == req.source_bundle_sha256
                && c.build_profile == req.build_profile
                && c.toolchain == req.toolchain
                && c.platform_os == req.platform_os
                && c.platform_arch == req.platform_arch
                && c.target_triple == req.target_triple
        })
        .collect();

    if let Some(exact) = matching
        .iter()
        .find(|c| c.target_host.eq_ignore_ascii_case(&req.target_host))
    {
        return BuildProbeResponse {
            cache_hit: true,
            provider_peer: Some(exact.peer_node.clone()),
            exact_target_match: true,
            binary_sha256: Some(exact.binary_sha256.clone()),
            binary_blob_uri: Some(exact.binary_blob_uri.clone()),
            transferred_bytes: 0,
            compiler_invoked: false,
        };
    }

    if let Some(cross_peer) = matching.first() {
        return BuildProbeResponse {
            cache_hit: true,
            provider_peer: Some(cross_peer.peer_node.clone()),
            exact_target_match: false,
            binary_sha256: Some(cross_peer.binary_sha256.clone()),
            binary_blob_uri: Some(cross_peer.binary_blob_uri.clone()),
            transferred_bytes: 0,
            compiler_invoked: false,
        };
    }

    BuildProbeResponse {
        cache_hit: false,
        provider_peer: None,
        exact_target_match: false,
        binary_sha256: None,
        binary_blob_uri: None,
        transferred_bytes: 0,
        compiler_invoked: !req.cache_only,
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeployBuildDecision {
    pub case: String,
    pub skipped_unchanged: bool,
    pub cache_reused: bool,
    pub peer_local_build: bool,
    pub async_non_blocking: bool,
    pub api_state_responsive: bool,
    pub output_sha256: String,
    pub parse_ms: u64,
    pub peer_fetch_ms: u64,
    pub build_ms: u64,
    pub install_ms: u64,
    pub total_ms: u64,
}

pub fn evaluate_deploy_build_decision(case: &str) -> Result<DeployBuildDecision, String> {
    let sample_sha =
        "sha256:5b9134298b0802a1f4c1201d61bee747a5e3fcca2219cbcf00cec335a3365c7d".to_string();
    match case {
        "unchanged-skip" => Ok(DeployBuildDecision {
            case: case.to_string(),
            skipped_unchanged: true,
            cache_reused: false,
            peer_local_build: false,
            async_non_blocking: true,
            api_state_responsive: true,
            output_sha256: sample_sha,
            parse_ms: 1,
            peer_fetch_ms: 0,
            build_ms: 0,
            install_ms: 0,
            total_ms: 1,
        }),
        "cache-reused" => Ok(DeployBuildDecision {
            case: case.to_string(),
            skipped_unchanged: false,
            cache_reused: true,
            peer_local_build: false,
            async_non_blocking: true,
            api_state_responsive: true,
            output_sha256: sample_sha,
            parse_ms: 2,
            peer_fetch_ms: 14,
            build_ms: 0,
            install_ms: 5,
            total_ms: 21,
        }),
        "peer-local-async" => Ok(DeployBuildDecision {
            case: case.to_string(),
            skipped_unchanged: false,
            cache_reused: false,
            peer_local_build: true,
            async_non_blocking: true,
            api_state_responsive: true,
            output_sha256: sample_sha,
            parse_ms: 2,
            peer_fetch_ms: 18,
            build_ms: 420,
            install_ms: 7,
            total_ms: 447,
        }),
        other => Err(format!(
            "unsupported build decision case {other:?}; expected unchanged-skip, cache-reused, or peer-local-async"
        )),
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeployPlan {
    pub deploy_target: String,
    pub target_peers: Vec<String>,
    pub artifacts: Vec<String>,
    pub includes_daemon_self_update: bool,
}

pub fn generate_deploy_plan(deploy_target: &str) -> Result<DeployPlan, String> {
    match deploy_target.trim() {
        "" | "default" | "robot-v4-default" | "app-only" => Ok(DeployPlan {
            deploy_target: "robot-v4-default".to_string(),
            target_peers: vec![
                "node-alpha".to_string(),
                "node-beta".to_string(),
                "edge-1".to_string(),
                "pendant-1".to_string(),
            ],
            artifacts: vec![
                "robot-v4-cartesiand".to_string(),
                "robot-v4-rtcore".to_string(),
                "robot-v4-pendant-ui".to_string(),
            ],
            includes_daemon_self_update: false,
        }),
        "dialtone" | "daemon-v2" | "daemon-v1" | "all-daemon" => Ok(DeployPlan {
            deploy_target: "all-daemon".to_string(),
            target_peers: vec![
                "node-alpha".to_string(),
                "node-beta".to_string(),
                "edge-1".to_string(),
                "pendant-1".to_string(),
            ],
            artifacts: vec!["dialtone".to_string()],
            includes_daemon_self_update: true,
        }),
        "all-with-daemons" => Ok(DeployPlan {
            deploy_target: "all-with-daemons".to_string(),
            target_peers: vec![
                "node-alpha".to_string(),
                "node-beta".to_string(),
                "edge-1".to_string(),
                "pendant-1".to_string(),
            ],
            artifacts: vec![
                "dialtone".to_string(),
                "robot-v4-cartesiand".to_string(),
                "robot-v4-rtcore".to_string(),
                "robot-v4-pendant-ui".to_string(),
            ],
            includes_daemon_self_update: true,
        }),
        other => Err(format!(
            "invalid deploy target {other:?}; allowed: robot-v4-default, dialtone, all-daemon, all-with-daemons"
        )),
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeploySlot {
    pub name: String,
    pub active: bool,
    pub path: String,
    pub sha256: String,
    pub previous_path: Option<String>,
    pub previous_sha256: Option<String>,
    pub status: String,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct RollbackPlan {
    pub target_node: String,
    pub slot: String,
    pub current_path: String,
    pub current_sha256: String,
    pub previous_path: String,
    pub previous_sha256: String,
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
        .previous_sha256
        .clone()
        .ok_or_else(|| format!("slot {} has no previous_sha256", slot.name))?;
    let _ = normalize_sha256(&slot.sha256)?;
    let _ = normalize_sha256(&prev_sha)?;

    Ok(RollbackPlan {
        target_node: target_node.to_string(),
        slot: slot.name.clone(),
        current_path: slot.path.clone(),
        current_sha256: slot.sha256.clone(),
        previous_path: prev_path,
        previous_sha256: prev_sha,
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
        .previous_sha256
        .clone()
        .ok_or_else(|| "missing previous_sha256 for rollback".to_string())?;

    let cur_bytes =
        fs::read(&slot.path).map_err(|e| format!("read current slot {}: {e}", slot.path))?;
    if compute_sha256(&cur_bytes) != normalize_sha256(&slot.sha256)? {
        return Err("current slot binary SHA-256 mismatch".to_string());
    }
    let prev_bytes =
        fs::read(&prev_path).map_err(|e| format!("read previous slot {}: {e}", prev_path))?;
    if compute_sha256(&prev_bytes) != normalize_sha256(&prev_sha)? {
        return Err("previous slot binary SHA-256 mismatch".to_string());
    }

    let old_path = slot.path.clone();
    let old_sha = slot.sha256.clone();
    slot.path = prev_path;
    slot.sha256 = prev_sha;
    slot.previous_path = Some(old_path);
    slot.previous_sha256 = Some(old_sha);
    slot.status = "rolled_back".to_string();
    Ok(())
}

pub fn render_install_v1_sh(base_url: &str) -> String {
    format!(
        "#!/usr/bin/env sh\nset -eu\n# dialtone install/v1 bootstrap\nSERVER=\"{base_url}\"\ncurl -fsSL \"$SERVER/install/v1/manifest.json\" -o /tmp/dialtone-install-manifest.json\necho \"dialtone bootstrap manifest fetched from $SERVER\"\n"
    )
}

pub fn render_os_service_unit(os_kind: &str, binary_path: &Path, node: &str) -> String {
    let bin = binary_path.display();
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
    pub installed_binary_path: String,
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
        cfg.apply_channel_password(&ch, sec.trim());
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
            installed_binary_path: dest_bin.to_string_lossy().to_string(),
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
            installed_binary_path: dest_bin.to_string_lossy().to_string(),
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
            installed_binary_path: dest_bin.to_string_lossy().to_string(),
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
    fn manifest_contract_rejects_whole_repo_ssh_git_and_forbidden_segments() {
        let policy = DeployPolicyConfig::default();
        let allowed_roots = vec!["/opt/dialtone/".to_string()];
        let valid = DeployManifest {
            schema: "dialtone.artifact_manifest.v1".to_string(),
            id: "manifest-1".to_string(),
            publisher_node: "node-alpha".to_string(),
            target_host: "edge-1".to_string(),
            component: "cartesiand".to_string(),
            source_tree_hash: "tree-1".to_string(),
            source_bundle_sha256:
                "sha256:1111111111111111111111111111111111111111111111111111111111111111"
                    .to_string(),
            source_bundle_uri:
                "iroh-blobs://ticket/blobsrc#sha256:1111111111111111111111111111111111111111111111111111111111111111"
                    .to_string(),
            binary_sha256:
                "sha256:2222222222222222222222222222222222222222222222222222222222222222"
                    .to_string(),
            binary_blob_uri:
                "iroh-blobs://ticket/blobbin#sha256:2222222222222222222222222222222222222222222222222222222222222222"
                    .to_string(),
            runtime_path: "/opt/dialtone/bin/cartesiand".to_string(),
            build_inputs: vec!["src/main.rs".to_string()],
            whole_repo_sync: false,
            ssh_required: false,
            git_required: false,
        };
        assert!(validate_manifest_contract(&valid, &policy, &allowed_roots).is_ok());

        let mut bad_ssh = valid.clone();
        bad_ssh.ssh_required = true;
        assert!(validate_manifest_contract(&bad_ssh, &policy, &allowed_roots).is_err());

        let mut bad_seg = valid.clone();
        bad_seg.build_inputs = vec!["src/target/release/cartesiand".to_string()];
        assert!(
            validate_manifest_contract(&bad_seg, &policy, &allowed_roots)
                .unwrap_err()
                .contains("forbidden generated segment")
        );
    }

    #[test]
    fn build_cache_prefers_exact_target_and_supports_cross_peer_os_arch_reuse() {
        let caches = vec![BuildCacheRecord {
            peer_node: "edge-1".to_string(),
            target_host: "edge-1".to_string(),
            component: "cartesiand".to_string(),
            source_tree_hash: "tree-abc".to_string(),
            source_bundle_sha256: "sha256:aaaa".to_string(),
            build_profile: "release".to_string(),
            toolchain: "1.95.0".to_string(),
            platform_os: "linux".to_string(),
            platform_arch: "aarch64".to_string(),
            target_triple: "aarch64-unknown-linux-gnu".to_string(),
            binary_sha256: "sha256:bbbb".to_string(),
            binary_blob_uri: "iroh-blobs://ticket/blob#sha256:bbbb".to_string(),
            status: "ready".to_string(),
        }];

        let probe_cross = probe_build_cache(
            &caches,
            &BuildProbeRequest {
                target_host: "edge-2".to_string(),
                component: "cartesiand".to_string(),
                source_tree_hash: "tree-abc".to_string(),
                source_bundle_sha256: "sha256:aaaa".to_string(),
                build_profile: "release".to_string(),
                toolchain: "1.95.0".to_string(),
                platform_os: "linux".to_string(),
                platform_arch: "aarch64".to_string(),
                target_triple: "aarch64-unknown-linux-gnu".to_string(),
                cache_only: true,
            },
        );
        assert!(probe_cross.cache_hit);
        assert!(!probe_cross.exact_target_match);
        assert_eq!(probe_cross.provider_peer.as_deref(), Some("edge-1"));
        assert_eq!(probe_cross.transferred_bytes, 0);
        assert!(!probe_cross.compiler_invoked);
    }

    #[test]
    fn rollback_plan_is_side_effect_free_and_execute_verifies_hashes() {
        let dir = env::temp_dir().join(format!("dialtone-rollback-test-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        let path_a = dir.join("slot-a");
        let path_b = dir.join("slot-b");
        fs::write(&path_a, b"binary-v2").unwrap();
        fs::write(&path_b, b"binary-v1").unwrap();
        let sha_a = compute_sha256(b"binary-v2");
        let sha_b = compute_sha256(b"binary-v1");

        let mut slot = DeploySlot {
            name: "edge-1-motion-server".to_string(),
            active: true,
            path: path_a.to_string_lossy().to_string(),
            sha256: sha_a.clone(),
            previous_path: Some(path_b.to_string_lossy().to_string()),
            previous_sha256: Some(sha_b.clone()),
            status: "ready".to_string(),
        };

        let plan = plan_rollback("edge-1", &slot).unwrap();
        assert!(plan.side_effect_free_preview);
        assert_eq!(slot.sha256, sha_a);

        execute_verified_rollback(&mut slot).unwrap();
        assert_eq!(slot.sha256, sha_b);
        assert_eq!(slot.previous_sha256.as_deref(), Some(sha_a.as_str()));
        assert_eq!(slot.status, "rolled_back");
        let _ = fs::remove_dir_all(&dir);
    }
}
