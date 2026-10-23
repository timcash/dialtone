use std::env;
use std::path::Path;

use serde::{Deserialize, Serialize};
use serde_json::Value;

/// Canonical peer-first service/process/blob URI:
/// `dialtone://<peer-name>/<role>/<name>` (also accepts `daemon-v1://`).
#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct PeerRef {
    pub scheme: String,
    pub peer: String,
    pub role: String,
    pub name: String,
}

impl PeerRef {
    pub fn parse(raw: &str) -> Result<Self, String> {
        let trimmed = raw.trim();
        let (scheme, rest) = trimmed
            .split_once("://")
            .ok_or_else(|| format!("missing URI scheme in {trimmed:?}"))?;
        if scheme != "dialtone" && scheme != "daemon-v1" {
            return Err(format!(
                "unsupported URI scheme {scheme:?}; expected dialtone:// or daemon-v1://"
            ));
        }
        let segments: Vec<&str> = rest.split('/').filter(|s| !s.is_empty()).collect();
        if segments.len() != 3 {
            return Err(format!(
                "peer ref {trimmed:?} must have 3 segments (<peer>/<role>/<name>); role-only refs are rejected"
            ));
        }
        let [peer, role, name] = [segments[0].trim(), segments[1].trim(), segments[2].trim()];
        if peer.is_empty() || role.is_empty() || name.is_empty() {
            return Err(format!("peer ref {trimmed:?} contains empty segment"));
        }
        if peer.contains('{') || peer.contains('}') {
            return Err(format!(
                "peer ref {trimmed:?} contains unexpanded template placeholder"
            ));
        }
        Ok(Self {
            scheme: scheme.to_string(),
            peer: peer.to_string(),
            role: role.to_string(),
            name: name.to_string(),
        })
    }

    pub fn expand_template(template: &str, node: &str) -> Result<Self, String> {
        let clean_node = node.trim();
        if clean_node.is_empty() {
            return Err("cannot expand {node} template with empty peer name".to_string());
        }
        Self::parse(&template.replace("{node}", clean_node))
    }

    pub fn to_uri(&self) -> String {
        format!(
            "{}://{}/{}/{}",
            self.scheme, self.peer, self.role, self.name
        )
    }
}

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

pub fn resolve_service<'a>(
    reference: &PeerRef,
    services: &'a [Service],
) -> Result<&'a Service, String> {
    let candidates: Vec<&Service> = services
        .iter()
        .filter(|svc| {
            svc.advertised_by.eq_ignore_ascii_case(&reference.peer)
                && svc.role.eq_ignore_ascii_case(&reference.role)
                && (svc.name.eq_ignore_ascii_case(&reference.name)
                    || svc.url.eq_ignore_ascii_case(&reference.to_uri()))
        })
        .collect();

    match candidates.as_slice() {
        [] => Err(format!(
            "no matching service found for {}",
            reference.to_uri()
        )),
        [single] => {
            if single.status != "running"
                && single.status != "ready"
                && single.status != "advertised"
            {
                return Err(format!(
                    "service {} on peer {} has non-ready status {:?}",
                    reference.to_uri(),
                    reference.peer,
                    single.status
                ));
            }
            Ok(*single)
        }
        _ => Err(format!(
            "ambiguous service resolution for {}: {} candidates matched",
            reference.to_uri(),
            candidates.len()
        )),
    }
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
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct KeyLeaseDefinition {
    pub name: String,
    pub kind: String,
    #[serde(default)]
    pub env: Vec<String>,
    #[serde(default)]
    pub path: Option<String>,
    #[serde(default)]
    pub expires_at_unix: Option<u64>,
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct KeyLeaseStatus {
    pub name: String,
    pub holder: String,
    pub kind: String,
    pub available: bool,
    pub status: String,
    pub source: String,
    pub expires_at_unix: Option<u64>,
    pub warning: Option<String>,
}

impl KeyLeaseDefinition {
    /// Evaluate lease availability and expiry without ever exposing secret values.
    pub fn evaluate(&self, holder: &str, now_unix: u64) -> KeyLeaseStatus {
        let mut available = false;
        let mut source = String::from("unconfigured");

        for env_key in &self.env {
            if let Ok(val) = env::var(env_key) {
                if !val.trim().is_empty() {
                    available = true;
                    source = format!("env:{env_key}");
                    break;
                }
            }
        }
        if !available {
            if let Some(path) = &self.path {
                source = format!("file:{path}");
                if Path::new(path).is_file() {
                    available = true;
                }
            } else if let Some(first_env) = self.env.first() {
                source = format!("env:{first_env}");
            }
        }

        let expired = self
            .expires_at_unix
            .map(|exp| exp <= now_unix)
            .unwrap_or(false);

        let (status, warning) = if !available {
            (
                "missing".to_string(),
                Some(format!("warning missing required key lease {}", self.name)),
            )
        } else if expired {
            (
                "expired".to_string(),
                Some(format!("warning expired key lease {}", self.name)),
            )
        } else {
            ("healthy".to_string(), None)
        };

        KeyLeaseStatus {
            name: self.name.clone(),
            holder: holder.to_string(),
            kind: self.kind.clone(),
            available: available && !expired,
            status,
            source,
            expires_at_unix: self.expires_at_unix,
            warning,
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeployPolicyConfig {
    pub manifest_schemas: Vec<String>,
    pub source_bundle_schemas: Vec<String>,
    pub build_input_roots: Vec<String>,
    pub forbidden_build_input_segments: Vec<String>,
}

impl Default for DeployPolicyConfig {
    fn default() -> Self {
        Self {
            manifest_schemas: vec![
                "dialtone.blob_manifest.v1".to_string(),
                "daemon.v1.artifact_manifest.v1".to_string(), // compat
            ],
            source_bundle_schemas: vec![
                "dialtone.component_source_bundle.v1".to_string(),
                "daemon.v1.component_source_bundle.v1".to_string(),
            ],
            build_input_roots: vec!["src/".to_string(), "daemon/v1/".to_string()],
            forbidden_build_input_segments: vec![
                ".git".to_string(),
                ".run".to_string(),
                "blobs".to_string(),
                "bin".to_string(),
                "dist".to_string(),
                "node_modules".to_string(),
                "target".to_string(),
            ],
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct CatalogPolicy {
    pub name: String,
    pub schema: String,
    pub owner: String,
    pub id_field: String,
    pub url_path: String,
    #[serde(default)]
    pub entries: Vec<Value>,
}

impl Default for CatalogPolicy {
    fn default() -> Self {
        Self {
            name: "programs".to_string(),
            schema: "dialtone.program_catalog.v1".to_string(),
            owner: "dialtone".to_string(),
            id_field: "id".to_string(),
            url_path: "/api/program-catalog".to_string(),
            entries: Vec::new(),
        }
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub struct DialtoneMetadata {
    #[serde(default = "default_metadata_schema")]
    pub schema: String,
    #[serde(default)]
    pub deploy_policy: DeployPolicyConfig,
    #[serde(default)]
    pub catalogs: Vec<CatalogPolicy>,
}

fn default_metadata_schema() -> String {
    "dialtone.metadata.v1".to_string()
}

impl Default for DialtoneMetadata {
    fn default() -> Self {
        Self {
            schema: default_metadata_schema(),
            deploy_policy: DeployPolicyConfig::default(),
            catalogs: vec![CatalogPolicy::default()],
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn key_lease_evaluates_without_exposing_secret_and_detects_missing_or_expired() {
        let missing_lease = KeyLeaseDefinition {
            name: "cloudflare-api".to_string(),
            kind: "api-token".to_string(),
            env: vec!["DIALTONE_NONEXISTENT_SECRET_TOKEN".to_string()],
            path: None,
            expires_at_unix: None,
        };
        let status = missing_lease.evaluate("node-alpha", 1000);
        assert!(!status.available);
        assert_eq!(status.status, "missing");
        assert!(status.warning.unwrap().contains("warning missing"));

        env::set_var(
            "DIALTONE_TEST_LEASE_SECRET",
            "super-secret-value-do-not-leak",
        );
        let active_lease = KeyLeaseDefinition {
            name: "cursor-agent".to_string(),
            kind: "api-token".to_string(),
            env: vec!["DIALTONE_TEST_LEASE_SECRET".to_string()],
            path: None,
            expires_at_unix: Some(2000),
        };
        let healthy = active_lease.evaluate("node-alpha", 1000);
        assert!(healthy.available);
        assert_eq!(healthy.status, "healthy");
        let serialized = serde_json::to_string(&healthy).unwrap();
        assert!(!serialized.contains("super-secret-value-do-not-leak"));

        let expired = active_lease.evaluate("node-alpha", 2500);
        assert!(!expired.available);
        assert_eq!(expired.status, "expired");
        env::remove_var("DIALTONE_TEST_LEASE_SECRET");
    }

    #[test]
    fn peer_ref_parses_concrete_peer_role_and_service() {
        let parsed = PeerRef::parse("dialtone://cell-a-controller/controller/nats").unwrap();
        assert_eq!(parsed.peer, "cell-a-controller");
        assert_eq!(parsed.role, "controller");
        assert_eq!(parsed.name, "nats");
        assert_eq!(
            parsed.to_uri(),
            "dialtone://cell-a-controller/controller/nats"
        );
    }

    #[test]
    fn peer_ref_rejects_ambiguous_role_only_uri() {
        let err = PeerRef::parse("dialtone://controller/nats").unwrap_err();
        assert!(err.contains("role-only refs are rejected"));
    }

    #[test]
    fn peer_ref_expands_node_template_and_resolves_service() {
        let parsed =
            PeerRef::expand_template("dialtone://{node}/controller/nats", "node-alpha").unwrap();
        let services = vec![Service {
            name: "nats".to_string(),
            kind: "nats-jetstream".to_string(),
            protocol: "nats".to_string(),
            url: "dialtone://node-alpha/controller/nats".to_string(),
            listen: "127.0.0.1:4222".to_string(),
            status: "running".to_string(),
            advertised_by: "node-alpha".to_string(),
            role: "controller".to_string(),
            mesh: "dialtone-iroh-gossip".to_string(),
            subjects: vec!["cell/events.>".to_string()],
        }];
        let resolved = resolve_service(&parsed, &services).unwrap();
        assert_eq!(resolved.listen, "127.0.0.1:4222");
    }
}
