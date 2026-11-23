const POLL_MS = 3000;

function esc(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[ch]));
}

function fmtPct(v) {
  return typeof v === "number" && Number.isFinite(v) ? `${v.toFixed(1)}%` : "—";
}

function fmtBytes(n) {
  if (typeof n !== "number" || !Number.isFinite(n)) return "0 B";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

function shortId(s, len = 14) {
  const str = String(s ?? "");
  return str.length > len ? `${str.slice(0, len)}…` : str || "—";
}

function relayLabel(node) {
  const urls = node?.iroh_transport?.advertised_relay_urls;
  if (Array.isArray(urls) && urls.length > 0) {
    try {
      return new URL(String(urls[0])).hostname.split(".")[0];
    } catch {
      return String(urls[0]);
    }
  }
  return node?.geo_location?.label || "—";
}

function avgRtt(node, selfNode) {
  if (node?.node === selfNode) return "self";
  const map = node?.peer_rtt_ms;
  if (!map || typeof map !== "object") return "—";
  const vals = Object.values(map).filter((v) => typeof v === "number" && Number.isFinite(v));
  if (!vals.length) return "—";
  const mean = vals.reduce((a, b) => a + b, 0) / vals.length;
  return `${mean.toFixed(1)}ms`;
}

function probeBadge(probe) {
  if (!probe || typeof probe !== "object") return "—";
  const pass = probe.passed_count;
  const total = probe.total_count;
  if (typeof pass === "number" && typeof total === "number") {
    const cls = pass === total ? "ok" : "err";
    return `<span class="${cls}">${pass}/${total}</span>`;
  }
  return "—";
}

async function fetchJson(path) {
  const res = await fetch(path, { cache: "no-store" });
  if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
  return res.json();
}

function renderMesh(mesh) {
  const selfNode = mesh.self_node || mesh.mesh_config?.node_name || "dialtone";
  const channel = mesh.mesh_config?.channel || "dialtone";
  const nodeId = mesh.self_iroh_runtime?.node_id || "";
  const nodes = Array.isArray(mesh.nodes) ? mesh.nodes : [];
  const blobs = mesh.blob_stats || {};

  document.getElementById("node-heading").textContent = selfNode;
  document.getElementById("node-meta").textContent =
    `channel: ${channel} · mode: ${mesh.mode || "live"} · node_id: ${shortId(nodeId, 20)}`;

  const onlineCount = nodes.filter((n) => n.online !== false).length;
  document.getElementById("summary-cards").innerHTML = [
    ["CHANNEL", channel],
    ["PEERS ONLINE", `${onlineCount} / ${nodes.length}`],
    ["STORE BLOBS", `${blobs.blob_keys ?? 0} (${fmtBytes(blobs.blob_bytes ?? 0)})`],
    ["PROGRAMS", `${blobs.installed_programs ?? 0}`],
    ["BEAT INTERVAL", `${mesh.beat_interval_secs ?? 5}s`],
  ]
    .map(([k, v]) => `<div class="card"><div class="card-k">${esc(k)}</div><div class="card-v">${esc(v)}</div></div>`)
    .join("");

  document.getElementById("peers-count").textContent = `${nodes.length} node(s)`;
  document.getElementById("peers-body").innerHTML = nodes.length
    ? nodes
        .map((n) => {
          const st = n.stats || {};
          const online = n.online !== false;
          const isSelf = n.node === selfNode;
          return `<tr>
            <td class="${isSelf ? "accent" : ""}">${esc(n.node || n.name || "—")}${isSelf ? " (self)" : ""}</td>
            <td class="${online ? "ok" : "err"}">${online ? "ONLINE" : "OFFLINE"}</td>
            <td>${esc(`${n.os || "?"}/${n.arch || "?"}`)}</td>
            <td>${esc(fmtPct(st.cpu_percent))}</td>
            <td>${esc(fmtPct(st.mem_percent))}</td>
            <td>${esc(fmtPct(st.disk_percent))}</td>
            <td>${esc(avgRtt(n, selfNode))}</td>
            <td>${probeBadge(n.probe)}</td>
            <td class="muted">${esc(relayLabel(n))}</td>
          </tr>`;
        })
        .join("")
    : `<tr><td colspan="9" class="muted">No peers heard yet.</td></tr>`;

  const stages = Array.isArray(mesh.boot_stages)
    ? mesh.boot_stages.map((s) => `${s.name}:${s.status}`).join(" · ")
    : "";
  document.getElementById("boot-stages").textContent = stages;
}

function renderProbe(probe) {
  const checks = Array.isArray(probe?.checks) ? probe.checks : [];
  const passed = checks.filter((c) => c.passed).length;
  document.getElementById("probe-summary").textContent = checks.length
    ? `${passed}/${checks.length} passed`
    : "no probe report yet";
  document.getElementById("probe-checks").innerHTML = checks.length
    ? checks
        .map((c) => {
          const cls = c.passed ? "ok" : "err";
          const code = c.code || c.name || "CHK";
          return `<span class="chip ${cls}" title="${esc(c.detail || "")}">${esc(code)}: ${c.passed ? "PASS" : "FAIL"}</span>`;
        })
        .join("")
    : `<span class="muted">Probes run automatically every 30s (or run: dialtone probe --all).</span>`;
}

function renderPlugins(data) {
  const list = Array.isArray(data?.plugins) ? data.plugins : [];
  document.getElementById("plugins-count").textContent = `${list.length} installed`;
  document.getElementById("plugins-body").innerHTML = list.length
    ? list
        .map((item) => {
          const m = item.manifest || item;
          return `<tr>
            <td>${esc(m.name || "—")}</td>
            <td>${esc(m.version || "—")}</td>
            <td class="muted">${esc(m.kind || "blob-binary")}</td>
            <td class="muted">${esc(m.source_repo || "—")}</td>
            <td class="muted">${esc(shortId(m.blob_digest, 16))}</td>
          </tr>`;
        })
        .join("")
    : `<tr><td colspan="5" class="muted">No plugins installed (dialtone install shell).</td></tr>`;
}

function renderSessions(data) {
  const list = Array.isArray(data?.sessions) ? data.sessions : [];
  document.getElementById("sessions-count").textContent = `${list.length} active`;
  document.getElementById("sessions-body").innerHTML = list.length
    ? list
        .map((s) => `<tr>
          <td class="accent">${esc(s.name || "—")}</td>
          <td>${esc(s.shell || "auto")}</td>
          <td class="muted">${esc(shortId(s.cwd || "~", 20))}</td>
          <td class="${s.status === "idle" ? "muted" : "ok"}">${esc(s.status || "active")}</td>
          <td class="muted">${esc(shortId(s.last_command || "—", 26))}</td>
        </tr>`)
        .join("")
    : `<tr><td colspan="5" class="muted">No active shell sessions.</td></tr>`;
}

function renderProcesses(data) {
  const list = Array.isArray(data?.processes) ? data.processes : [];
  document.getElementById("processes-count").textContent = `${list.length} running`;
  document.getElementById("processes-body").innerHTML = list.length
    ? list
        .map((p) => `<tr>
          <td>${esc(p.name || "—")}</td>
          <td>${esc(p.pid ?? "—")}</td>
          <td class="${(p.restart_count || 0) > 0 ? "warn" : "muted"}">${esc(p.restart_count ?? 0)}</td>
          <td class="muted">${esc(shortId(p.digest, 18))}</td>
        </tr>`)
        .join("")
    : `<tr><td colspan="4" class="muted">No processes running.</td></tr>`;
}

async function refreshAll() {
  const pill = document.getElementById("sync-status");
  try {
    const [mesh, probe, plugins, sessions, processes] = await Promise.all([
      fetchJson("/api/mesh"),
      fetchJson("/api/probe").catch(() => null),
      fetchJson("/api/plugins").catch(() => null),
      fetchJson("/api/shell/sessions").catch(() => null),
      fetchJson("/api/processes").catch(() => null),
    ]);
    renderMesh(mesh);
    renderProbe(probe);
    renderPlugins(plugins);
    renderSessions(sessions);
    renderProcesses(processes);
    pill.textContent = "LIVE";
    pill.className = "pill";
  } catch (err) {
    pill.textContent = "OFFLINE";
    pill.className = "pill err";
  }
}

try {
  const initialEl = document.getElementById("initial-data");
  if (initialEl && initialEl.textContent) {
    renderMesh(JSON.parse(initialEl.textContent));
  }
} catch {
  // Initial hydrate will be replaced by first poll.
}

document.getElementById("btn-refresh")?.addEventListener("click", () => {
  void refreshAll();
});

const themeBtn = document.getElementById("btn-theme");
themeBtn?.addEventListener("click", () => {
  const next = document.documentElement.dataset.theme === "light" ? "dark" : "light";
  document.documentElement.dataset.theme = next;
  themeBtn.textContent = next === "light" ? "dark" : "light";
});

void refreshAll();
setInterval(() => {
  if (!document.hidden) void refreshAll();
}, POLL_MS);
