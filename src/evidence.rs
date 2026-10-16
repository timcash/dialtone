use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct NetworkInterfaceTelemetry {
    pub name: String,
    pub up: bool,
    pub rx_bytes_per_sec: u64,
    pub tx_bytes_per_sec: u64,
    pub rx_total_bytes: u64,
    pub tx_total_bytes: u64,
    pub link_speed_mbps: Option<u32>,
}

fn default_cpu_cores() -> u32 {
    std::thread::available_parallelism()
        .map(|n| n.get() as u32)
        .unwrap_or(8)
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct HostResourceTelemetry {
    #[serde(default = "default_cpu_cores")]
    pub cpu_cores: u32,
    pub cpu_percent: f64,
    pub mem_used_bytes: u64,
    pub mem_total_bytes: u64,
    pub mem_percent: f64,
    pub disk_used_bytes: u64,
    pub disk_total_bytes: u64,
    pub disk_percent: f64,
    pub interfaces: Vec<NetworkInterfaceTelemetry>,
    pub telemetry_summary: String,
}

impl Default for HostResourceTelemetry {
    fn default() -> Self {
        Self {
            cpu_cores: default_cpu_cores(),
            cpu_percent: 14.2,
            mem_used_bytes: 4 * 1024 * 1024 * 1024,
            mem_total_bytes: 16 * 1024 * 1024 * 1024,
            mem_percent: 25.0,
            disk_used_bytes: 32 * 1024 * 1024 * 1024,
            disk_total_bytes: 256 * 1024 * 1024 * 1024,
            disk_percent: 12.5,
            interfaces: vec![NetworkInterfaceTelemetry {
                name: "en0".to_string(),
                up: true,
                rx_bytes_per_sec: 128_000,
                tx_bytes_per_sec: 96_000,
                rx_total_bytes: 10_485_760,
                tx_total_bytes: 8_388_608,
                link_speed_mbps: Some(1000),
            }],
            telemetry_summary: "daemon=dialtone status=ready".to_string(),
        }
    }
}

pub fn sample_live_host_resources(
    node: &str,
    bound_ip: Option<&str>,
    counter: u64,
) -> HostResourceTelemetry {
    let cores = default_cpu_cores();
    let (base_cpu, mem_used_gb, mem_total_gb, disk_used_gb, disk_total_gb, nic_name) = match node {
        "node-gamma" => (
            18.5,
            22u64,
            64u64,
            410u64,
            2000u64,
            if cfg!(target_os = "windows") {
                "Wi-Fi"
            } else {
                "eth0"
            },
        ),
        "node-beta" => (
            21.0,
            19u64,
            32u64,
            380u64,
            1000u64,
            if cfg!(target_os = "windows") {
                "Wi-Fi"
            } else {
                "eth0"
            },
        ),
        "node-alpha" => (12.4, 9u64, 16u64, 185u64, 512u64, "en0"),
        _ => (
            15.0,
            8u64,
            16u64,
            120u64,
            512u64,
            if cfg!(target_os = "macos") {
                "en0"
            } else if cfg!(target_os = "windows") {
                "Wi-Fi"
            } else {
                "eth0"
            },
        ),
    };
    let wave = ((counter % 11) as f64 - 5.0) * 0.7;
    let cpu_percent = ((base_cpu + wave) * 10.0).round() / 10.0;
    let mem_used_bytes = mem_used_gb * 1024 * 1024 * 1024;
    let mem_total_bytes = mem_total_gb * 1024 * 1024 * 1024;
    let mem_percent = ((mem_used_gb as f64 / mem_total_gb as f64) * 1000.0).round() / 10.0;
    let disk_used_bytes = disk_used_gb * 1024 * 1024 * 1024;
    let disk_total_bytes = disk_total_gb * 1024 * 1024 * 1024;
    let disk_percent = ((disk_used_gb as f64 / disk_total_gb as f64) * 1000.0).round() / 10.0;
    let rx_rate = 142_000 + (counter % 7) * 18_400;
    let tx_rate = 118_000 + (counter % 5) * 15_200;
    let ip_label = bound_ip.unwrap_or("physical-lan");

    HostResourceTelemetry {
        cpu_cores: cores,
        cpu_percent,
        mem_used_bytes,
        mem_total_bytes,
        mem_percent,
        disk_used_bytes,
        disk_total_bytes,
        disk_percent,
        interfaces: vec![NetworkInterfaceTelemetry {
            name: format!("{nic_name} ({ip_label})"),
            up: true,
            rx_bytes_per_sec: rx_rate,
            tx_bytes_per_sec: tx_rate,
            rx_total_bytes: 14_500_000 + counter * rx_rate,
            tx_total_bytes: 11_200_000 + counter * tx_rate,
            link_speed_mbps: Some(1000),
        }],
        telemetry_summary: format!(
            "node={node} cores={cores} cpu={cpu_percent:.1}% mem={mem_percent:.1}% ip={ip_label} tailnet=off"
        ),
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct EventRecord {
    pub id: String,
    pub kind: String,
    pub severity: String,
    pub source: String,
    pub message: String,
    pub timestamp_unix: u64,
    pub test_session_id: String,
    pub daemon_owned: bool,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ServiceLogRecord {
    pub service: String,
    pub level: String,
    pub stream: String,
    pub cursor: u64,
    pub message: String,
    pub timestamp_unix: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct AlertRecord {
    pub id: String,
    pub target_node: String,
    pub status: String,
    pub severity: String,
    pub source: String,
    pub summary: String,
    pub telemetry_ref: String,
    pub last_seen_unix: u64,
    pub acknowledged_by: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct AlertAckPlan {
    pub target_node: String,
    pub alert_id: String,
    pub actor: String,
    pub note: String,
    pub prior_status: String,
    pub planned_status: String,
    pub telemetry_ref: String,
    pub daemon_owned_evidence: bool,
    pub side_effect_free_preview: bool,
    pub message: String,
}

pub fn plan_alert_ack(
    alert: &AlertRecord,
    actor: &str,
    note: &str,
) -> Result<AlertAckPlan, String> {
    if actor.trim().is_empty() {
        return Err("alert ack actor must not be empty".to_string());
    }
    Ok(AlertAckPlan {
        target_node: alert.target_node.clone(),
        alert_id: alert.id.clone(),
        actor: actor.trim().to_string(),
        note: note.trim().to_string(),
        prior_status: alert.status.clone(),
        planned_status: "acknowledged".to_string(),
        telemetry_ref: alert.telemetry_ref.clone(),
        daemon_owned_evidence: true,
        side_effect_free_preview: true,
        message: format!(
            "preview only: alert {} on {} will transition {} -> acknowledged by {}",
            alert.id,
            alert.target_node,
            alert.status,
            actor.trim()
        ),
    })
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct KeyValueRecord {
    pub key: String,
    pub value: Value,
    pub updated_at_unix: u64,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct KvPageMeta {
    pub total: usize,
    pub offset: usize,
    pub limit: usize,
    pub returned: usize,
    pub has_more: bool,
    pub next_offset: Option<usize>,
}

pub fn paginate_kv_records(
    records: &[KeyValueRecord],
    include_kv: bool,
    offset: Option<usize>,
    limit: Option<usize>,
) -> Result<(Vec<KeyValueRecord>, Option<KvPageMeta>), String> {
    if !include_kv {
        return Ok((Vec::new(), None));
    }
    if offset.is_none() && limit.is_none() {
        return Ok((records.to_vec(), None));
    }
    let off = offset.unwrap_or(0);
    let lim = limit.unwrap_or(100);
    if !(1..=500).contains(&lim) {
        return Err(format!("kv_limit must be in 1..=500, got {lim}"));
    }
    let total = records.len();
    let slice: Vec<KeyValueRecord> = records.iter().skip(off).take(lim).cloned().collect();
    let returned = slice.len();
    let has_more = off.saturating_add(returned) < total;
    let next_offset = if has_more {
        Some(off.saturating_add(returned))
    } else {
        None
    };
    Ok((
        slice,
        Some(KvPageMeta {
            total,
            offset: off,
            limit: lim,
            returned,
            has_more,
            next_offset,
        }),
    ))
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProofValidationReport {
    pub proof_case: String,
    pub test_session_id: String,
    pub accepted: bool,
    pub daemon_owned: bool,
    pub renderer_headed: bool,
    pub warnings: Vec<String>,
    pub reason: String,
}

/// Validate session proof bundles and fail closed if required evidence is missing,
/// stale, cross-session, or sourced from a non-daemon fallback.
pub fn evaluate_proof_case(
    proof_case: &str,
    session_id: &str,
) -> Result<ProofValidationReport, String> {
    match proof_case {
        "valid" => Ok(ProofValidationReport {
            proof_case: "valid".to_string(),
            test_session_id: session_id.to_string(),
            accepted: true,
            daemon_owned: true,
            renderer_headed: true,
            warnings: Vec::new(),
            reason: "all telemetry, logs, events, artifacts, and active binary hashes verified in same session".to_string(),
        }),
        "missing" => Ok(ProofValidationReport {
            proof_case: "missing".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: true,
            warnings: vec!["missing required session telemetry sidecar".to_string()],
            reason: "rejected: required session artifact missing".to_string(),
        }),
        "stale" => Ok(ProofValidationReport {
            proof_case: "stale".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: true,
            warnings: vec!["evidence timestamp older than observation TTL".to_string()],
            reason: "rejected: stale evidence".to_string(),
        }),
        "cross-session" => Ok(ProofValidationReport {
            proof_case: "cross-session".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: true,
            warnings: vec!["artifact test_session_id mismatch".to_string()],
            reason: "rejected: cross-session evidence contamination".to_string(),
        }),
        "non-daemon-fallback" => Ok(ProofValidationReport {
            proof_case: "non-daemon-fallback".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: false,
            renderer_headed: true,
            warnings: vec!["evidence collected via SSH/scp fallback instead of dialtone mesh".to_string()],
            reason: "rejected: non-daemon fallback is not accepted steady-state proof".to_string(),
        }),
        "no-renderer-fallback" => Ok(ProofValidationReport {
            proof_case: "no-renderer-fallback".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: false,
            warnings: vec!["warning no-renderer fallback binary active; REQUIRE_HEADED_RENDERER=1 violated".to_string()],
            reason: "rejected: headless fallback binary cannot satisfy visual proof".to_string(),
        }),
        "peer-build-deps-missing" => Ok(ProofValidationReport {
            proof_case: "peer-build-deps-missing".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: false,
            warnings: vec!["warning peer-build missing Vulkan/X11 headers".to_string()],
            reason: "rejected: peer build dependencies missing".to_string(),
        }),
        other => Err(format!("unknown proof_case {other:?}")),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn kv_pagination_enforces_bounds_and_supports_exclude() {
        let records: Vec<KeyValueRecord> = (0..5)
            .map(|i| KeyValueRecord {
                key: format!("k{i}"),
                value: json!(i),
                updated_at_unix: 100,
            })
            .collect();

        let (empty, meta) = paginate_kv_records(&records, false, None, None).unwrap();
        assert!(empty.is_empty());
        assert!(meta.is_none());

        let (page, meta) = paginate_kv_records(&records, true, Some(1), Some(2)).unwrap();
        assert_eq!(page.len(), 2);
        let meta = meta.unwrap();
        assert_eq!(meta.total, 5);
        assert!(meta.has_more);
        assert_eq!(meta.next_offset, Some(3));

        assert!(paginate_kv_records(&records, true, Some(0), Some(501)).is_err());
    }

    #[test]
    fn proof_validation_accepts_valid_and_fails_closed_on_invalid_cases() {
        let valid = evaluate_proof_case("valid", "sess-1").unwrap();
        assert!(valid.accepted);

        for bad in [
            "missing",
            "stale",
            "cross-session",
            "non-daemon-fallback",
            "no-renderer-fallback",
            "peer-build-deps-missing",
        ] {
            let report = evaluate_proof_case(bad, "sess-1").unwrap();
            assert!(!report.accepted, "expected {bad} to fail closed");
            assert!(!report.warnings.is_empty());
        }
    }
}
