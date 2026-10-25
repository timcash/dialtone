use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct Service {
    pub name: String,
    pub kind: String,
    pub protocol: String,
    pub url: String,
    pub listen: String,
    pub status: String,
    pub advertised_by: String,
    pub role: String,
    #[serde(default)]
    pub mesh: String,
    #[serde(default)]
    pub subjects: Vec<String>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct InstalledProgram {
    pub name: String,
    pub path: String,
    #[serde(alias = "sha256")] // compat
    pub digest: String,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct ProcessSpec {
    pub name: String,
    pub path: String,
    #[serde(alias = "sha256")] // compat
    pub digest: String,
    pub desired_state: String,
    #[serde(default)]
    pub args: Vec<String>,
    #[serde(default)]
    pub env: Vec<String>,
    #[serde(default)]
    pub cwd: Option<String>,
    #[serde(default)]
    pub startup_grace_secs: Option<u64>,
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default)]
    pub pid: Option<u32>,
    #[serde(default)]
    pub restart_count: u32,
    #[serde(default)]
    pub service_unit: Option<String>,
    /// Ticket for this process's program when it is offered for mesh deploy (set in beats).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub deploy_ticket: Option<String>,
}
