use serde::{Deserialize, Serialize};

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
