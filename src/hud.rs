//! The HUD: the node's local web UI, served by the API at `/`.

use std::borrow::Cow;
use std::path::Path;

use serde_json::json;

use crate::api::*;
use crate::h3_land::LAND_H3_BITSET_B64;
use crate::mesh::{BEAT_INTERVAL_SECS, SUMMARY_INTERVAL_SECS};

/// Render the HUD page. `csp_nonce` must match the `Content-Security-Policy` header sent
/// with it; only the two `<script>` tags carrying it may run.
pub(crate) fn render_hud(
    state: &NodeState,
    selected_peer: Option<&str>,
    csp_nonce: &str,
) -> String {
    let peer_focus = selected_peer.unwrap_or(&state.node);
    let summary = state.blobs.summary();
    let initial_gossip = json!({
        "schema": "dialtone.gossip.v1",
        "mesh": "dialtone-iroh-gossip",
        "self_node": state.node,
        "mode": state.mode,
        "selected_peer": peer_focus,
        "beat_interval_secs": BEAT_INTERVAL_SECS,
        "summary_interval_secs": SUMMARY_INTERVAL_SECS,
        "self_iroh_runtime": state.iroh_runtime,
        "boot_stages": state.boot_stages,
        "mesh_config": masked_mesh_config(&state.mesh_config),
        "blob_stats": {
            "blob_keys": summary.blob_keys,
            "blob_bytes": summary.blob_bytes,
            "blob_fetches": state.blob_fetches.len(),
            "installed_programs": state.installed_programs.len()
        },
        "nodes": state.nodes,
        "mesh_rtt_ms": state.mesh_rtt_ms,
        "service_endpoints": state.service_endpoints
    });
    let initial_json = serde_json::to_string(&initial_gossip).unwrap_or_else(|_| "{}".to_string());

    let template = read_from_hud_dir("index.html")
        .and_then(|b| String::from_utf8(b).ok())
        .map(Cow::Owned)
        .unwrap_or(Cow::Borrowed(include_str!("hud/index.html")));

    template
        .replace("__SELF_NODE__", &html_escape(&state.node))
        .replace("__CSP_NONCE__", csp_nonce)
        .replace("__INITIAL_JSON__", &script_safe_json(&initial_json))
        .replace("__LAND_H3_BITSET_B64__", LAND_H3_BITSET_B64)
}

/// Cache policy for vendored libraries: their paths carry the version, so they never change.
const IMMUTABLE: &str = "public, max-age=31536000, immutable";

/// Set to the `src/hud` directory to serve the HUD from disk while developing it: an edit
/// to a HUD file then needs only a browser reload, no rebuild or restart.
pub const HUD_DIR_ENV: &str = "DIALTONE_HUD_DIR";

/// Read `rel` (a fixed name from this module, never a request path) from `DIALTONE_HUD_DIR`.
fn read_from_hud_dir(rel: &str) -> Option<Vec<u8>> {
    let dir = std::env::var_os(HUD_DIR_ENV)?;
    std::fs::read(Path::new(&dir).join(rel)).ok()
}

/// A HUD static file: `(content type, bytes, Cache-Control)`. Served from the binary, or
/// from `DIALTONE_HUD_DIR` when it is set (uncached, so every reload shows the edit).
pub(crate) fn hud_asset(path: &str) -> Option<(&'static str, Cow<'static, [u8]>, &'static str)> {
    hud_asset_in(
        path,
        std::env::var_os(HUD_DIR_ENV).as_deref().map(Path::new),
    )
}

fn hud_asset_in(
    path: &str,
    dir: Option<&Path>,
) -> Option<(&'static str, Cow<'static, [u8]>, &'static str)> {
    // Only names the binary itself serves are looked up on disk, so `..` cannot escape.
    let (content_type, embedded, cache) = embedded_hud_asset(path)?;
    let rel = path.trim_start_matches("/hud/");
    let on_disk = dir.and_then(|d| std::fs::read(d.join(rel)).ok());
    Some(match on_disk {
        Some(bytes) => (content_type, Cow::Owned(bytes), "no-store"),
        None => (content_type, Cow::Borrowed(embedded), cache),
    })
}

/// The HUD's static files compiled into the binary. three.js and h3-js are vendored (pinned
/// versions, npm integrity-checked) so the HUD needs no CDN and works with no Internet.
fn embedded_hud_asset(path: &str) -> Option<(&'static str, &'static [u8], &'static str)> {
    const JS: &str = "text/javascript; charset=utf-8";
    Some(match path {
        "/hud/style.css" => (
            "text/css; charset=utf-8",
            include_bytes!("hud/style.css"),
            "no-cache",
        ),
        "/hud/app.js" => (JS, include_bytes!("hud/app.js"), "no-cache"),
        "/hud/vendor/three@0.179.1/three.module.min.js" => (
            JS,
            include_bytes!("hud/vendor/three@0.179.1/three.module.min.js"),
            IMMUTABLE,
        ),
        "/hud/vendor/three@0.179.1/three.core.min.js" => (
            JS,
            include_bytes!("hud/vendor/three@0.179.1/three.core.min.js"),
            IMMUTABLE,
        ),
        "/hud/vendor/h3-js@4.4.0/h3-js.umd.js" => (
            JS,
            include_bytes!("hud/vendor/h3-js@4.4.0/h3-js.umd.js"),
            IMMUTABLE,
        ),
        _ => return None,
    })
}

/// Escape text for an HTML text node or attribute value.
fn html_escape(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&#39;")
}

/// JSON that is safe to embed inside `<script>`: no `</script>`, `<!--` or line separators.
fn script_safe_json(json: &str) -> String {
    json.replace('<', "\\u003c")
        .replace('>', "\\u003e")
        .replace('&', "\\u0026")
        .replace('\u{2028}', "\\u2028")
        .replace('\u{2029}', "\\u2029")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hud_dir_serves_edited_files_uncached_and_only_known_names() {
        let dir = std::env::temp_dir().join(format!("dialtone-hud-dir-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        std::fs::write(dir.join("app.js"), b"// edited").unwrap();
        std::fs::write(dir.join("secret.txt"), b"nope").unwrap();

        let (_, body, cache) = hud_asset_in("/hud/app.js", Some(&dir)).unwrap();
        assert_eq!((&*body, cache), (&b"// edited"[..], "no-store"));
        // A file missing from the directory falls back to the embedded copy.
        let (_, css, cache) = hud_asset_in("/hud/style.css", Some(&dir)).unwrap();
        assert_eq!(
            (&*css, cache),
            (&include_bytes!("hud/style.css")[..], "no-cache")
        );
        assert!(hud_asset_in("/hud/secret.txt", Some(&dir)).is_none());
        assert!(hud_asset_in("/hud/../Cargo.toml", Some(&dir)).is_none());
        let (_, embedded, _) = hud_asset_in("/hud/app.js", None).unwrap();
        assert_eq!(&*embedded, &include_bytes!("hud/app.js")[..]);
        let _ = std::fs::remove_dir_all(&dir);
    }
}
