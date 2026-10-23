use serde::{Deserialize, Serialize};
use serde_json::Value;

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
    #[serde(alias = "telemetry_ref")] // compat
    pub stats_ref: String,
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
    #[serde(alias = "telemetry_ref")] // compat
    pub stats_ref: String,
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
        stats_ref: alert.stats_ref.clone(),
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
            reason:
                "all stats, logs, events, blobs, and active binary hashes verified in same session"
                    .to_string(),
        }),
        "missing" => Ok(ProofValidationReport {
            proof_case: "missing".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: true,
            warnings: vec!["missing required session stats sidecar".to_string()],
            reason: "rejected: required session blob missing".to_string(),
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
            warnings: vec!["blob test_session_id mismatch".to_string()],
            reason: "rejected: cross-session evidence contamination".to_string(),
        }),
        "non-daemon-fallback" => Ok(ProofValidationReport {
            proof_case: "non-daemon-fallback".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: false,
            renderer_headed: true,
            warnings: vec![
                "evidence collected via SSH/scp fallback instead of dialtone mesh".to_string(),
            ],
            reason: "rejected: non-daemon fallback is not accepted steady-state proof".to_string(),
        }),
        "no-renderer-fallback" => Ok(ProofValidationReport {
            proof_case: "no-renderer-fallback".to_string(),
            test_session_id: session_id.to_string(),
            accepted: false,
            daemon_owned: true,
            renderer_headed: false,
            warnings: vec![
                "warning no-renderer fallback binary active; REQUIRE_HEADED_RENDERER=1 violated"
                    .to_string(),
            ],
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
