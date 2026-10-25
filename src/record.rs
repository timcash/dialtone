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
    pub stats_ref: String,
    pub last_seen_unix: u64,
    pub acknowledged_by: Option<String>,
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
}
