//! The signed release manifest: each published binary's SHA-256, BLAKE3 hash and size,
//! signed with the release key. A node verifies a binary against it whether the binary came
//! from GitHub or from a peer (`src/update.rs` holds the public key).

use crate::util::{hex, output, say, R};
use ed25519_dalek::{Signer, SigningKey};
use serde_json::json;
use std::path::Path;

pub const MANIFEST: &str = "dialtone-release.json";

/// The release key from DIALTONE_RELEASE_KEY: 32 bytes as 64 hex characters.
fn key() -> R<SigningKey> {
    let raw = std::env::var("DIALTONE_RELEASE_KEY")
        .map_err(|_| "DIALTONE_RELEASE_KEY is not set (64 hex characters)")?;
    let raw = raw.trim();
    let bytes: Vec<u8> = (0..raw.len())
        .step_by(2)
        .map(|i| {
            raw.get(i..i + 2)
                .and_then(|b| u8::from_str_radix(b, 16).ok())
        })
        .collect::<Option<_>>()
        .ok_or("DIALTONE_RELEASE_KEY is not hex")?;
    let seed: [u8; 32] = bytes
        .try_into()
        .map_err(|_| "DIALTONE_RELEASE_KEY is not 32 bytes")?;
    Ok(SigningKey::from_bytes(&seed))
}

/// `release-pubkey`: the public key nodes verify releases with.
pub fn pubkey() -> R<()> {
    println!("{}", hex(key()?.verifying_key().as_bytes()));
    Ok(())
}

/// `release-manifest <version> <dir>`: write `dialtone-release.json` and its `.sig` into dir,
/// covering every `dialtone-*` binary there.
pub fn manifest(args: &[String]) -> R<()> {
    let [version, dir] = args else {
        return Err("usage: release-manifest <version> <dir with the release binaries>".into());
    };
    let version = version.trim_start_matches('v');
    let cargo_version = package_version()?;
    if version != cargo_version {
        return Err(format!(
            "version {version} is not the version in Cargo.toml ({cargo_version})"
        ));
    }
    let key = key()?;
    // Nodes verify with the public key in src/update.rs: a release signed with another key
    // would be refused by every node.
    let public = hex(key.verifying_key().as_bytes());
    let embedded = std::fs::read_to_string("src/update.rs").map_err(|e| e.to_string())?;
    if !embedded.contains(&format!("RELEASE_PUBLIC_KEY: &str = \"{public}\"")) {
        return Err(format!(
            "DIALTONE_RELEASE_KEY (public key {public}) is not the key in src/update.rs"
        ));
    }
    let dir = Path::new(dir);
    let mut assets = Vec::new();
    let mut names: Vec<String> = std::fs::read_dir(dir)
        .map_err(|e| format!("{}: {e}", dir.display()))?
        .flatten()
        .map(|e| e.file_name().to_string_lossy().into_owned())
        .filter(|n| {
            n.starts_with("dialtone-") && !n.ends_with(".sha256") && !n.starts_with(MANIFEST)
        })
        .collect();
    names.sort();
    for name in &names {
        let bytes = std::fs::read(dir.join(name)).map_err(|e| format!("{name}: {e}"))?;
        let sha256 = hex(&<sha2::Sha256 as sha2::Digest>::digest(&bytes));
        assets.push(json!({
            "name": name,
            "sha256": sha256,
            "blake3": blake3::hash(&bytes).to_hex().to_string(),
            "size": bytes.len(),
        }));
        println!("    {name} {} ({} bytes)", &sha256[..16], bytes.len());
    }
    if assets.is_empty() {
        return Err(format!("no dialtone-* binaries in {}", dir.display()));
    }
    let body = serde_json::to_vec_pretty(&json!({
        "schema": "dialtone.release.v1",
        "version": version,
        "assets": assets,
    }))
    .map_err(|e| e.to_string())?;
    let sig = key.sign(&body);
    std::fs::write(dir.join(MANIFEST), &body).map_err(|e| e.to_string())?;
    std::fs::write(dir.join(format!("{MANIFEST}.sig")), hex(&sig.to_bytes()))
        .map_err(|e| e.to_string())?;
    say(&format!(
        "Signed {MANIFEST} for v{version} ({} binaries) with key {}",
        names.len(),
        &hex(key.verifying_key().as_bytes())[..16]
    ));
    Ok(())
}

/// The root package's version, from `cargo metadata`.
fn package_version() -> R<String> {
    let meta =
        output(crate::util::cargo().args(["metadata", "--no-deps", "--format-version", "1"]))?;
    let meta: serde_json::Value = serde_json::from_str(&meta).map_err(|e| e.to_string())?;
    meta["packages"]
        .as_array()
        .into_iter()
        .flatten()
        .find(|p| p["name"] == "dialtone")
        .and_then(|p| p["version"].as_str())
        .map(str::to_string)
        .ok_or_else(|| "no dialtone package in cargo metadata".into())
}
