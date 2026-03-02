package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

func handleProcessStats(w http.ResponseWriter, r *http.Request) {
	resp := buildProcessStatsResponse(normalizeProcessLimit(r.URL.Query().Get("limit")))
	writeJSON(w, http.StatusOK, resp)
}

func handleProcessStatsWS(w http.ResponseWriter, r *http.Request) {
	limit := normalizeProcessLimit(r.URL.Query().Get("limit"))
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx := r.Context()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		resp := buildProcessStatsResponse(limit)
		payload, _ := json.Marshal(resp)
		if werr := conn.Write(ctx, websocket.MessageText, payload); werr != nil {
			return
		}
		select {
		case <-ctx.Done():
			_ = conn.Close(websocket.StatusNormalClosure, "")
			return
		case <-ticker.C:
		}
	}
}

func handleProcessUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	hn, _ := osHostname()
	title := html.EscapeString(fmt.Sprintf("Chrome Daemon Process Monitor (%s)", hn))
	_, _ = w.Write([]byte(`<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>` + title + `</title>
  <style>
    :root { --bg:#0b1020; --fg:#e8eefb; --muted:#9bb0d1; --line:#24314f; --accent:#4cc9f0; }
    body { margin:0; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; background:var(--bg); color:var(--fg);}
    .wrap { padding: 14px; }
    h1 { margin:0 0 8px; font-size: 16px; }
    .meta { color: var(--muted); margin-bottom: 10px; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { padding: 7px 8px; border-bottom: 1px solid var(--line); text-align: left; white-space: nowrap; }
    th { color: var(--accent); position: sticky; top: 0; background: #0e1630; }
    td.cmd { max-width: 760px; overflow: hidden; text-overflow: ellipsis; }
  </style>
  <script>
    function render(data) {
      const tbody = document.getElementById('rows');
      tbody.innerHTML = '';
      (data.processes || []).forEach((p) => {
        const tr = document.createElement('tr');
        tr.innerHTML =
          '<td>' + p.pid + '</td>' +
          '<td>' + (p.name || '') + '</td>' +
          '<td>' + Number(p.cpu || 0).toFixed(2) + '</td>' +
          '<td>' + Number(p.mem_mb || 0).toFixed(1) + '</td>' +
          '<td class=\"cmd\" title=\"' + (p.command || '').replace(/"/g,'&quot;') + '\">' + (p.command || '') + '</td>';
        tbody.appendChild(tr);
      });
      document.getElementById('meta').textContent =
        'host=' + (data.host || '?') + ' os=' + (data.os || '?') + ' updated=' + (data.updated_at || '?') + ' count=' + (data.count || 0);
    }
    async function firstLoad() {
      try {
        const res = await fetch('/processes?limit=60', { cache: 'no-store' });
        const data = await res.json();
        render(data);
      } catch (e) {
        document.getElementById('meta').textContent = 'failed to load process data: ' + e;
      }
    }
    function connectWS() {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      const ws = new WebSocket(proto + '://' + location.host + '/ws/processes?limit=60');
      ws.onmessage = (ev) => { try { render(JSON.parse(ev.data)); } catch (_) {} };
      ws.onclose = () => setTimeout(connectWS, 1200);
      ws.onerror = () => { try { ws.close(); } catch (_) {} };
    }
    window.addEventListener('load', async () => { await firstLoad(); connectWS(); });
  </script>
</head>
<body>
  <div class="wrap">
    <h1>` + title + `</h1>
    <div id="meta" class="meta">loading...</div>
    <table>
      <thead><tr><th>PID</th><th>Name</th><th>CPU</th><th>Mem MB</th><th>Command</th></tr></thead>
      <tbody id="rows"></tbody>
    </table>
  </div>
</body>
</html>`))
}
