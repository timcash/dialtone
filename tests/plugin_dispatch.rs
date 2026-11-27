//! End-to-end CLI tests for call-time plugin subcommand dispatch (`dialtone <plugin> ...`).
mod common;

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};

use common::{TempDir, BIN};
use dialtone::blob::digest_of;
use dialtone::plugin::{PluginManifest, PLUGIN_MANIFEST_SCHEMA};

fn install_mock_plugin(
    state_dir: &Path,
    name: &str,
    unix_script: &str,
    windows_script: &str,
) -> PathBuf {
    let dir = state_dir.join("plugins").join(name);
    fs::create_dir_all(&dir).unwrap();
    let (filename, body, entrypoint) = if cfg!(windows) {
        let f = format!("dialtone-{name}.cmd");
        (f.clone(), windows_script.as_bytes().to_vec(), Some(f))
    } else {
        (
            format!("dialtone-{name}"),
            unix_script.as_bytes().to_vec(),
            None,
        )
    };
    let bin_path = dir.join(&filename);
    fs::write(&bin_path, &body).unwrap();
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&bin_path, fs::Permissions::from_mode(0o755)).unwrap();
    }
    let manifest = PluginManifest {
        schema: PLUGIN_MANIFEST_SCHEMA.to_string(),
        name: name.to_string(),
        version: "0.1.0".to_string(),
        kind: "blob-binary".to_string(),
        description: format!("Mock CLI plugin {name}"),
        protocols: Vec::new(),
        blob_digest: digest_of(&body),
        blake3_hash: "00".repeat(32),
        bytes: body.len() as u64,
        entrypoint,
        ticket_uri: None,
        source_repo: None,
        installed_at_unix: 1_700_000_000,
    };
    fs::write(
        dir.join("manifest.json"),
        serde_json::to_vec_pretty(&manifest).unwrap(),
    )
    .unwrap();
    bin_path
}

fn run_cli(state_dir: &Path, args: &[&str], extra_env: &[(&str, &str)]) -> (i32, String, String) {
    let mut cmd = Command::new(BIN);
    cmd.args(args)
        .stdin(Stdio::null())
        .env("DIALTONE_STATE_DIR", state_dir);
    for (k, v) in extra_env {
        cmd.env(k, v);
    }
    let out = cmd.output().expect("execute dialtone CLI");
    (
        out.status.code().unwrap_or(-1),
        String::from_utf8_lossy(&out.stdout).into_owned(),
        String::from_utf8_lossy(&out.stderr).into_owned(),
    )
}

#[test]
fn cli_dispatches_verified_plugin_with_args_env_and_exit_code() {
    let dir = TempDir::new("dispatch-cli-ok");
    install_mock_plugin(
        dir.path(),
        "demo",
        "#!/bin/sh\necho \"HOME=$DIALTONE_HOME\"\necho \"API=$DIALTONE_API\"\necho \"NODE=$DIALTONE_NODE\"\necho \"SECRET=$DIALTONE_SECRET\"\necho \"ARGS=$*\"\nexit 37\n",
        "@echo off\r\necho HOME=%DIALTONE_HOME%\r\necho API=%DIALTONE_API%\r\necho NODE=%DIALTONE_NODE%\r\necho SECRET=%DIALTONE_SECRET%\r\necho ARGS=%*\r\nexit /b 37\r\n",
    );

    let (code, stdout, stderr) = run_cli(
        dir.path(),
        &[
            "--node",
            "alpha",
            "--listen",
            "127.0.0.1:9876",
            "demo",
            "subcmd",
            "--help",
            "--flag",
            "val",
        ],
        &[("DIALTONE_SECRET", "dt-ShouldNotLeakToPlugin12345")],
    );

    assert_eq!(code, 37, "expected exit code 37, stderr={stderr}");
    assert!(
        stdout.contains(&format!("HOME={}", dir.path().display())),
        "{stdout}"
    );
    assert!(stdout.contains("API=http://127.0.0.1:9876"), "{stdout}");
    assert!(stdout.contains("NODE=alpha"), "{stdout}");
    assert!(
        !stdout.contains("ShouldNotLeakToPlugin"),
        "secret leaked to plugin env: {stdout}"
    );
    assert!(stdout.contains("ARGS=subcmd --help --flag val"), "{stdout}");
}

#[test]
fn cli_unknown_subcommand_lists_installed_plugins() {
    let dir = TempDir::new("dispatch-cli-missing");
    install_mock_plugin(
        dir.path(),
        "task",
        "#!/bin/sh\nexit 0\n",
        "@echo off\r\nexit /b 0\r\n",
    );
    install_mock_plugin(
        dir.path(),
        "alpha",
        "#!/bin/sh\nexit 0\n",
        "@echo off\r\nexit /b 0\r\n",
    );

    let (code, _stdout, stderr) = run_cli(dir.path(), &["nope", "list"], &[]);
    assert_eq!(code, 2, "expected exit code 2, stderr={stderr}");
    assert!(stderr.contains("unknown command 'nope'"), "{stderr}");
    assert!(stderr.contains("alpha, task"), "{stderr}");
}

#[test]
fn cli_refuses_tampered_plugin_binary() {
    let dir = TempDir::new("dispatch-cli-tamper");
    let bin = install_mock_plugin(
        dir.path(),
        "task",
        "#!/bin/sh\necho ORIGINAL\n",
        "@echo off\r\necho ORIGINAL\r\n",
    );
    fs::write(&bin, b"#!/bin/sh\necho TAMPERED\n").unwrap();

    let (code, stdout, stderr) = run_cli(dir.path(), &["task", "list"], &[]);
    assert_ne!(code, 0, "tampered plugin must be refused");
    assert!(
        !stdout.contains("TAMPERED"),
        "tampered plugin must not execute: {stdout}"
    );
    assert!(stderr.contains("digest mismatch"), "{stderr}");
}

#[test]
fn cli_builtin_command_always_wins_over_same_named_plugin() {
    let dir = TempDir::new("dispatch-cli-builtin");
    install_mock_plugin(
        dir.path(),
        "secret",
        "#!/bin/sh\necho HIJACKED_PLUGIN\n",
        "@echo off\r\necho HIJACKED_PLUGIN\r\n",
    );

    let (code, stdout, stderr) = run_cli(dir.path(), &["secret"], &[]);
    assert_eq!(code, 0, "builtin secret failed: {stderr}");
    assert!(
        stdout.trim().starts_with("dt-"),
        "expected builtin dt- secret, got: {stdout}"
    );
    assert!(
        !stdout.contains("HIJACKED_PLUGIN"),
        "plugin shadowed builtin command: {stdout}"
    );
}

#[test]
fn cli_dispatches_plugin_with_channel_derived_key_and_hides_raw_secret() {
    let dir = TempDir::new("dispatch-cli-channel-key");
    let raw_secret = "dt-Cli_Channel_Plugin_Key_Test_2026!";
    let mut cfg = dialtone::mesh::MeshConfig::default();
    let keys = cfg.apply_secret("metal", raw_secret);
    dialtone::mesh::save_mesh_config(&dir.path().join("config.json"), &cfg).unwrap();

    install_mock_plugin(
        dir.path(),
        "task",
        "#!/bin/sh\necho \"CID=$DIALTONE_CHANNEL_ID\"\necho \"PKEY=$DIALTONE_CHANNEL_PLUGIN_KEY\"\necho \"DDIR=$DIALTONE_CHANNEL_DATA_DIR\"\necho \"SEC=$DIALTONE_SECRET\"\n",
        "@echo off\r\necho CID=%DIALTONE_CHANNEL_ID%\r\necho PKEY=%DIALTONE_CHANNEL_PLUGIN_KEY%\r\necho DDIR=%DIALTONE_CHANNEL_DATA_DIR%\r\necho SEC=%DIALTONE_SECRET%\r\n",
    );

    let (code, stdout, stderr) = run_cli(
        dir.path(),
        &["task", "status"],
        &[("DIALTONE_SECRET", raw_secret)],
    );
    assert_eq!(code, 0, "stderr={stderr}");
    let expected_cid = keys.fingerprint();
    let expected_key = dialtone::mesh::derive_channel_plugin_key_hex(&keys, "task");
    let expected_ddir = dialtone::mesh::channel_plugin_data_dir(dir.path(), &expected_cid, "task");
    assert!(stdout.contains(&format!("CID={expected_cid}")), "{stdout}");
    assert!(stdout.contains(&format!("PKEY={expected_key}")), "{stdout}");
    assert!(
        stdout.contains(&format!("DDIR={}", expected_ddir.display())),
        "{stdout}"
    );
    assert!(
        !stdout.contains(raw_secret),
        "raw channel secret leaked to plugin env: {stdout}"
    );
}
