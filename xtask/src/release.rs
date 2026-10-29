//! The signed release manifest: each published binary's SHA-256, BLAKE3 hash and size,
//! signed with the release key. A node verifies a binary against it whether the binary came
//! from GitHub or from a peer (`src/update.rs` holds the public key).

use crate::util::{cargo, hex, output, quietly, run, say, R};
use ed25519_dalek::{Signer, SigningKey};
use serde_json::json;
use std::path::Path;
use std::process::Command;

pub const MANIFEST: &str = "dialtone-release.json";

/// Where `release build` stages the binaries and their `.sha256` files.
pub const ASSETS: &str = "target/release-assets";

/// `./dialtone.sh release build [TARGET] | manifest [DIR] | pubkey`.
pub fn release(args: &[String]) -> R<()> {
    match args.first().map(String::as_str) {
        Some("build") => build(args.get(1).map(String::as_str)),
        Some("manifest") => manifest(args.get(1).map(String::as_str).unwrap_or(ASSETS)),
        Some("pubkey") => pubkey(),
        _ => Err("usage: ./dialtone.sh release build [TARGET] | manifest [DIR] | pubkey".into()),
    }
}

/// The published name of the binary for a target: the names nodes look for (`asset_name` in
/// src/update.rs).
fn asset_for(target: &str) -> Option<&'static str> {
    let os = if target.contains("windows") {
        "windows"
    } else if target.contains("apple-darwin") {
        "macos"
    } else if target.contains("linux") {
        "linux"
    } else {
        return None;
    };
    match (os, target.split('-').next()?) {
        ("windows", "x86_64") => Some("dialtone-windows-x86_64.exe"),
        ("macos", "aarch64") => Some("dialtone-macos-arm64"),
        ("linux", "x86_64") => Some("dialtone-linux-x86_64"),
        ("linux", "aarch64") => Some("dialtone-linux-arm64"),
        _ => None,
    }
}

/// This machine's target triple, from `rustc -vV`.
fn host_target() -> R<String> {
    output(Command::new("rustc").arg("-vV"))?
        .lines()
        .find_map(|l| l.strip_prefix("host: "))
        .map(|h| h.trim().to_string())
        .ok_or_else(|| "rustc -vV printed no host".into())
}

/// `release build [TARGET]`: the size-optimized `dist` build for TARGET (default: this machine),
/// staged as `target/release-assets/<asset>` with its `.sha256` (`sha256sum` format).
fn build(target: Option<&str>) -> R<()> {
    let target = match target {
        Some(t) => t.to_string(),
        None => host_target()?,
    };
    let asset =
        asset_for(&target).ok_or_else(|| format!("no release binary is published for {target}"))?;
    // C code (ring, blake3) builds for musl with musl-gcc.
    if target.ends_with("-musl") && !quietly(Command::new("musl-gcc").arg("--version")) {
        if std::env::var_os("CI").is_none() {
            return Err("building for musl needs musl-gcc: install musl-tools".into());
        }
        say("Install musl-tools");
        run(Command::new("sudo").args(["apt-get", "update", "-qq"]))?;
        run(Command::new("sudo").args(["apt-get", "install", "-y", "-qq", "musl-tools"]))?;
    }
    say(&format!("Release build for {target} (dist profile)"));
    run(cargo().args([
        "build",
        "-p",
        "dialtone",
        "--profile",
        "dist",
        "--target",
        &target,
    ]))?;
    let exe = if target.contains("windows") {
        ".exe"
    } else {
        ""
    };
    let built = format!("target/{target}/dist/dialtone{exe}");
    std::fs::create_dir_all(ASSETS).map_err(|e| format!("{ASSETS}: {e}"))?;
    let staged = Path::new(ASSETS).join(asset);
    std::fs::copy(&built, &staged).map_err(|e| format!("copy {built}: {e}"))?;
    let bytes = std::fs::read(&staged).map_err(|e| e.to_string())?;
    let sha256 = hex(&<sha2::Sha256 as sha2::Digest>::digest(&bytes));
    std::fs::write(
        Path::new(ASSETS).join(format!("{asset}.sha256")),
        format!(
            "{sha256}  {asset}
"
        ),
    )
    .map_err(|e| e.to_string())?;
    println!(
        "    {}  {sha256}  ({} bytes)",
        staged.display(),
        bytes.len()
    );
    Ok(())
}

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

/// `release pubkey`: the public key nodes verify releases with.
fn pubkey() -> R<()> {
    println!("{}", hex(key()?.verifying_key().as_bytes()));
    Ok(())
}

/// `release manifest [DIR]`: write `dialtone-release.json` and its `.sig` into DIR (default
/// target/release-assets), covering every `dialtone-*` binary there, as the version in
/// Cargo.toml. On a tag in CI the tag must be that version.
fn manifest(dir: &str) -> R<()> {
    let version = package_version()?;
    let version = version.as_str();
    if let Ok(tag) = std::env::var("GITHUB_REF_NAME") {
        if std::env::var("GITHUB_REF_TYPE").as_deref() == Ok("tag")
            && tag.trim_start_matches('v') != version
        {
            return Err(format!(
                "tag {tag} is not the version in Cargo.toml ({version})"
            ));
        }
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
