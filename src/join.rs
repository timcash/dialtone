//! Join: hand the channel keys to a node that presents a valid open invite (ALPN
//! `dialtone/join/1`).
//!
//! A member mints an invite (`dialtone invite`): the channel, an expiry, a nonce and the
//! member itself as the node to present it to, under a MAC with the channel's `auth_key`.
//! The invite carries no keys, so it is worthless once it expires. A new node opens one
//! stream to the member, writes the invite, and reads back the channel keys and seeds. On an
//! open channel (empty allowlist) that is all joining takes: nobody lists anybody's NodeId.

use std::path::PathBuf;
use std::time::Duration;

use iroh::endpoint::Connection;
use iroh::protocol::{AcceptError, ProtocolHandler};
use iroh::Endpoint;
use serde::{Deserialize, Serialize};

use crate::mesh::{load_mesh_config, ChannelKeys, ChannelKeysHex, OpenInvite, Seed};
use crate::node::{advertised_endpoint_addr, IrohTransportConfig};

pub const ALPN: &[u8] = b"dialtone/join/1";
const MAX_REQUEST_BYTES: usize = 16 << 10;
const MAX_ANSWER_BYTES: usize = 64 << 10;
/// Members an invite and a join answer name: this node, then its seeds (the peers it hears),
/// so a new node gets in, and back in after a restart, while any one of them is up.
const MAX_INVITE_SEEDS: usize = 4;

/// What a member answers a valid invite with.
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct JoinAnswer {
    pub channel: String,
    pub derived_keys: ChannelKeysHex,
    pub seeds: Vec<Seed>,
}

/// Mints invites and answers them. Keys and allowlist are read from `config.json` on each
/// request, so a channel switch or allowlist edit applies at once.
#[derive(Clone, Debug)]
pub struct Joins {
    endpoint: Endpoint,
    transport: IrohTransportConfig,
    config_path: PathBuf,
}

impl Joins {
    pub fn new(endpoint: Endpoint, transport: IrohTransportConfig, config_path: PathBuf) -> Self {
        Joins {
            endpoint,
            transport,
            config_path,
        }
    }

    /// This node as a seed: its NodeId, home relay and a few direct addresses.
    pub fn self_seed(&self) -> Seed {
        Seed::from_addr(&advertised_endpoint_addr(
            &self.endpoint.addr(),
            &self.transport,
        ))
    }

    /// The members to name in an invite or a join answer: this node, then its seeds.
    fn seeds(&self) -> Vec<Seed> {
        let me = self.self_seed();
        let mut seeds = vec![me.clone()];
        seeds.extend(
            load_mesh_config(&self.config_path)
                .seeds
                .into_iter()
                .filter(|s| s.node_id != me.node_id),
        );
        seeds.truncate(MAX_INVITE_SEEDS);
        seeds
    }

    fn open_channel_keys(&self) -> Result<ChannelKeys, String> {
        let cfg = load_mesh_config(&self.config_path);
        if !cfg.allowlist.is_empty() {
            return Err(
                "this channel uses an allowlist; ask its owner to 'dialtone allowlist add' your NodeId"
                    .to_string(),
            );
        }
        cfg.active_channel_keys()
            .ok_or_else(|| "this node has no channel keys".to_string())
    }

    /// A new open invite to this node's channel, valid for `ttl_secs`.
    pub fn mint(&self, ttl_secs: u64) -> Result<OpenInvite, String> {
        let keys = self.open_channel_keys()?;
        OpenInvite::new(&keys, self.seeds(), crate::now_unix(), ttl_secs)
    }

    fn answer(&self, request: &[u8]) -> Result<JoinAnswer, String> {
        let text = std::str::from_utf8(request).map_err(|_| "invite is not text".to_string())?;
        let invite: OpenInvite = text.parse()?;
        let keys = self.open_channel_keys()?;
        invite.check(&keys, crate::now_unix())?;
        Ok(JoinAnswer {
            channel: keys.channel.clone(),
            derived_keys: keys.to_hex_bundle(),
            seeds: self.seeds(),
        })
    }
}

impl ProtocolHandler for Joins {
    async fn accept(&self, connection: Connection) -> Result<(), AcceptError> {
        let remote = connection.remote_id();
        let (mut send, mut recv) = connection.accept_bi().await?;
        let request = recv
            .read_to_end(MAX_REQUEST_BYTES)
            .await
            .map_err(AcceptError::from_err)?;
        let body = match self.answer(&request) {
            Ok(answer) => {
                println!("[join] gave the channel keys to {}", remote.fmt_short());
                serde_json::to_vec(&answer).unwrap_or_default()
            }
            Err(err) => {
                eprintln!("[join] refused {}: {err}", remote.fmt_short());
                serde_json::to_vec(&serde_json::json!({ "error": err })).unwrap_or_default()
            }
        };
        send.write_all(&body).await.map_err(AcceptError::from_err)?;
        send.finish().map_err(AcceptError::from_err)?;
        connection.closed().await;
        Ok(())
    }
}

/// Present `invite` to the members it names, in order, and return the first answer.
pub async fn redeem(
    endpoint: &Endpoint,
    invite: &OpenInvite,
    timeout: Duration,
) -> Result<JoinAnswer, String> {
    let mut errors = Vec::new();
    let text = invite.to_text()?;
    for seed in &invite.seeds {
        let Some(addr) = seed.to_addr() else {
            errors.push(format!("invalid NodeId '{}'", seed.node_id));
            continue;
        };
        let short = addr.id.fmt_short().to_string();
        let attempt = async {
            let conn = endpoint
                .connect(addr, ALPN)
                .await
                .map_err(|e| format!("connect: {e}"))?;
            let (mut send, mut recv) = conn.open_bi().await.map_err(|e| e.to_string())?;
            send.write_all(text.as_bytes())
                .await
                .map_err(|e| e.to_string())?;
            send.finish().map_err(|e| e.to_string())?;
            let body = recv
                .read_to_end(MAX_ANSWER_BYTES)
                .await
                .map_err(|e| e.to_string())?;
            conn.close(0u32.into(), b"done");
            let value: serde_json::Value =
                serde_json::from_slice(&body).map_err(|e| format!("bad answer: {e}"))?;
            if let Some(err) = value.get("error").and_then(|e| e.as_str()) {
                return Err(err.to_string());
            }
            serde_json::from_value::<JoinAnswer>(value).map_err(|e| format!("bad answer: {e}"))
        };
        match tokio::time::timeout(timeout, attempt).await {
            Ok(Ok(answer)) => return Ok(answer),
            Ok(Err(err)) => errors.push(format!("{short}: {err}")),
            Err(_) => errors.push(format!("{short}: timed out")),
        }
    }
    Err(format!(
        "no member accepted the invite ({})",
        errors.join("; ")
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::mesh::{derive_channel_keys_v2, save_mesh_config, LiveAllowlist, MeshConfig};
    use crate::node::{build_endpoint_builder, AllowlistHook};
    use iroh::protocol::Router;

    const SECRET: &str = "dt-Open_Invite_Test_Secret_2026!";

    /// A member on an open channel (empty allowlist unless `allowlist` names NodeIds) that
    /// answers joins, and its seed with direct addresses only.
    async fn member(
        name: &str,
        allowlist: &[String],
    ) -> Result<(Router, Joins, Seed, PathBuf), String> {
        let dir = std::env::temp_dir().join(format!("dialtone-join-{name}-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
        let config_path = dir.join("config.json");
        let mut cfg = MeshConfig::default();
        cfg.apply_secret("metal", SECRET);
        cfg.allowlist = allowlist.to_vec();
        save_mesh_config(&config_path, &cfg)?;

        let config = IrohTransportConfig::local_offline();
        let endpoint = build_endpoint_builder(&config)?
            .hooks(AllowlistHook::new(LiveAllowlist::new(allowlist)))
            .bind()
            .await
            .map_err(|e| format!("bind member: {e}"))?;
        let joins = Joins::new(endpoint.clone(), config, config_path.clone());
        let seed = Seed {
            node_id: endpoint.id().to_string(),
            relay_url: Some("none".to_string()),
            ip_addrs: endpoint.addr().ip_addrs().map(|a| a.to_string()).collect(),
        };
        let router = Router::builder(endpoint)
            .accept(ALPN, joins.clone())
            .accept(crate::fetch::ALPN, crate::fetch::FetchHandler::new(None))
            .spawn();
        Ok((router, joins, seed, dir))
    }

    async fn joiner() -> Result<Endpoint, String> {
        build_endpoint_builder(&IrohTransportConfig::local_offline())?
            .bind()
            .await
            .map_err(|e| format!("bind joiner: {e}"))
    }

    #[tokio::test]
    async fn open_invite_hands_over_the_channel_keys_until_it_expires() -> Result<(), String> {
        let (router, _joins, seed, dir) = member("open", &[]).await?;
        let keys = derive_channel_keys_v2("metal", SECRET);
        let new_node = joiner().await?;
        let wait = Duration::from_secs(10);

        // A valid invite: the joiner gets the member's channel keys and the member as a seed.
        let invite = OpenInvite::new(&keys, vec![seed.clone()], crate::now_unix(), 60)?;
        let text = invite.to_text()?;
        let answer = redeem(&new_node, &text.parse()?, wait).await?;
        assert_eq!(answer.channel, "metal");
        assert_eq!(answer.derived_keys, keys.to_hex_bundle());
        assert_eq!(answer.seeds[0].node_id, seed.node_id);

        // Any number of nodes can use it while it is valid.
        let another = joiner().await?;
        assert!(redeem(&another, &invite, wait).await.is_ok());

        // Expired, altered, or from another channel: refused.
        let expired = OpenInvite::new(&keys, vec![seed.clone()], crate::now_unix() - 120, 60)?;
        let err = redeem(&new_node, &expired, wait).await.unwrap_err();
        assert!(err.contains("expired"), "{err}");
        let mut altered = invite.clone();
        altered.expires_at_unix += 3600;
        let err = redeem(&new_node, &altered, wait).await.unwrap_err();
        assert!(err.contains("altered"), "{err}");
        let other = derive_channel_keys_v2("metal", "dt-Some_Other_Channel_Secret_2026!");
        let forged = OpenInvite::new(&other, vec![seed], crate::now_unix(), 60)?;
        assert!(redeem(&new_node, &forged, wait).await.is_err());

        // On an open channel only gossip and join are open to strangers.
        let fetch = tokio::time::timeout(
            wait,
            new_node.connect(invite.seeds[0].to_addr().unwrap(), crate::fetch::ALPN),
        )
        .await
        .map_err(|_| "fetch connect timed out".to_string())?;
        if let Ok(conn) = fetch {
            let answer = crate::fetch::request(&conn, crate::fetch::Request::Details, wait).await;
            assert!(
                answer.is_err(),
                "a stranger fetched from an open channel: {answer:?}"
            );
        }

        router.shutdown().await.map_err(|e| e.to_string())?;
        let _ = std::fs::remove_dir_all(dir);
        Ok(())
    }

    #[tokio::test]
    async fn a_channel_with_an_allowlist_gives_no_keys_for_invites() -> Result<(), String> {
        let someone = iroh::SecretKey::generate().public().to_string();
        let (router, joins, seed, dir) = member("closed", &[someone]).await?;
        assert!(joins.mint(60).unwrap_err().contains("allowlist"));
        let keys = derive_channel_keys_v2("metal", SECRET);
        let invite = OpenInvite::new(&keys, vec![seed], crate::now_unix(), 60)?;
        assert!(redeem(&joiner().await?, &invite, Duration::from_secs(10))
            .await
            .is_err());
        router.shutdown().await.map_err(|e| e.to_string())?;
        let _ = std::fs::remove_dir_all(dir);
        Ok(())
    }
}
