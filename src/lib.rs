pub mod api;
pub mod blob;
pub mod deploy;
pub mod devices;
pub mod fetch;
pub mod h3_land;
pub mod hud;
pub mod join;
pub mod link;
pub mod mesh;
pub mod node;
pub mod probe;
pub mod record;
pub mod service;
pub mod stats;
pub mod supervisor;
pub mod wire;

use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

pub fn now_unix() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

pub fn run_cmd(prog: &str, args: &[&str]) -> Option<String> {
    let out = Command::new(prog).args(args).output().ok()?;
    if !out.status.success() {
        return None;
    }
    Some(String::from_utf8_lossy(&out.stdout).trim().to_string())
}

pub fn hex_encode(bytes: &[u8]) -> String {
    hex::encode(bytes)
}

pub fn hex_decode(raw: &str) -> Result<Vec<u8>, String> {
    hex::decode(raw.trim()).map_err(|e| format!("invalid hex: {e}"))
}

/// Standard alphabet, any padding, no trailing-bit check: decodes whatever peers and older
/// builds produced (standard or URL-safe after normalisation, padded or not).
const BASE64_LENIENT: base64::engine::GeneralPurpose = base64::engine::GeneralPurpose::new(
    &base64::alphabet::STANDARD,
    base64::engine::GeneralPurposeConfig::new()
        .with_encode_padding(true)
        .with_decode_padding_mode(base64::engine::DecodePaddingMode::Indifferent)
        .with_decode_allow_trailing_bits(true),
);

pub fn base64_encode(bytes: &[u8]) -> String {
    base64::Engine::encode(&base64::engine::general_purpose::STANDARD, bytes)
}

pub fn base64url_encode(bytes: &[u8]) -> String {
    base64::Engine::encode(&base64::engine::general_purpose::URL_SAFE_NO_PAD, bytes)
}

/// Decode standard or URL-safe base64, with or without padding; whitespace is ignored.
pub fn base64_decode(raw: &str) -> Result<Vec<u8>, String> {
    let normalised: String = raw
        .chars()
        .filter(|c| !c.is_ascii_whitespace() && *c != '=')
        .map(|c| match c {
            '-' => '+',
            '_' => '/',
            other => other,
        })
        .collect();
    base64::Engine::decode(&BASE64_LENIENT, normalised).map_err(|e| format!("invalid base64: {e}"))
}

use crate::node::{
    advertised_endpoint_addr, build_diagnostics, build_endpoint_builder, default_secret_key_path,
    load_or_create_secret_key_at, IrohTransportConfig, IrohTransportDiagnostics,
};

pub async fn collect_node_diagnostics(
    config: &IrohTransportConfig,
) -> Result<IrohTransportDiagnostics, String> {
    let key_path = default_secret_key_path();
    let secret_key = load_or_create_secret_key_at(&key_path)?;
    let endpoint = build_endpoint_builder(config)?
        .secret_key(secret_key)
        .bind()
        .await
        .map_err(|e| format!("bind iroh endpoint: {e}"))?;
    let node_id = endpoint.id().to_string();
    let advertised = advertised_endpoint_addr(&endpoint.addr(), config);
    let diagnostics = build_diagnostics(config, &advertised, Some(&key_path), &node_id);
    endpoint.close().await;
    Ok(diagnostics)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The encoders replaced hand-written code; these vectors are what that code produced and
    /// accepted (beat envelopes and API payloads from older builds must still decode).
    #[test]
    fn hex_and_base64_match_the_previous_hand_written_codecs() {
        assert_eq!(hex_encode(&[0x0a, 0xff, 0x10]), "0aff10");
        assert_eq!(hex_decode(" 0AfF10 ").unwrap(), vec![0x0a, 0xff, 0x10]);
        assert!(hex_decode("abc").is_err());

        assert_eq!(base64_encode(b"dialtone"), "ZGlhbHRvbmU=");
        assert_eq!(base64url_encode(b"dialtone"), "ZGlhbHRvbmU");
        assert_eq!(base64_encode(&[0xfb, 0xff]), "+/8=");
        assert_eq!(base64url_encode(&[0xfb, 0xff]), "-_8");
        for form in ["ZGlhbHRvbmU=", "ZGlhbHRvbmU", "ZGlh bHRv\nbmU="] {
            assert_eq!(base64_decode(form).unwrap(), b"dialtone", "{form}");
        }
        for form in ["+/8=", "+/8", "-_8", "-_8="] {
            assert_eq!(base64_decode(form).unwrap(), vec![0xfb, 0xff], "{form}");
        }
        assert!(base64_decode("@@@@").is_err());
    }
}
