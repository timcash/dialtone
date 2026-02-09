const log = (msg: string) => {
  const time = new Date().toLocaleTimeString();
  console.log('[NIX-UI] ' + time + ': ' + msg);
  const logs = document.getElementById('logs')!;
  const entry = document.createElement('div');
  entry.textContent = '[' + time + '] ' + msg;
  logs.insertBefore(entry, logs.firstChild);
};

document.querySelector<HTMLDivElement>('#app')!.innerHTML = '<div>' +
    '<h1>Nix Node Manager</h1>' +
    '<div class="card">' +
      '<p>Host Status: <span id="status">Connecting...</span></p>' +
      '<div class="controls">' +
        '<button id="start-proc">Start Nix Sub-Process</button>' +
        '<button id="error-ping" style="background: #ff4444; margin-left: 10px;">Trigger Error Ping</button>' +
      '</div>' +
      '<div id="process-list" style="margin-top: 20px; text-align: left;">' +
        '<h3>Active Sub-Processes:</h3>' +
        '<div id="procs"></div>' +
      '</div>' +
      '<div id="logs-container" style="margin-top: 20px; border-top: 1px solid #444; padding-top: 10px; text-align: left; max-height: 200px; overflow-y: auto;">' +
        '<h3>UI Activity Logs:</h3>' +
        '<div id="logs" style="font-family: monospace; font-size: 12px;"></div>' +
      '</div>' +
    '</div>' +
  '</div>';

async function updateStatus() {
  try {
    const res = await fetch('/api/status')
    const data = await res.json()
    document.getElementById('status')!.textContent = data.status
  } catch {
    document.getElementById('status')!.textContent = 'Offline'
  }
}

async function listProcesses() {
  try {
    const res = await fetch('/api/processes')
    const procs = await res.json()
    const list = document.getElementById('procs')!
    list.innerHTML = ''
    procs.forEach((p: any) => {
      const container = document.createElement('div')
      container.className = 'proc-container'
      container.style.border = '1px solid #333'
      container.style.margin = '10px 0'
      container.style.padding = '10px'
      container.style.borderRadius = '4px'

      const header = document.createElement('div')
      header.className = 'proc-header'
      header.innerHTML = '<strong>' + p.id + '</strong> (' + p.status + ') ' +
        '<button style="margin-left: 10px;" class="proc-item" id="stop-' + p.id + '">Stop</button>';
      
      const procLogs = document.createElement('div')
      procLogs.style.fontSize = '11px'
      procLogs.style.color = '#00ff00'
      procLogs.style.background = '#000'
      procLogs.style.padding = '5px'
      procLogs.style.marginTop = '5px'
      procLogs.style.maxHeight = '60px'
      procLogs.style.overflowY = 'auto'
      procLogs.innerHTML = (p.logs || []).join('<br/>')

      container.appendChild(header)
      container.appendChild(procLogs)
      list.appendChild(container)

      const stopBtn = document.getElementById('stop-' + p.id);
      if (stopBtn) {
        stopBtn.onclick = () => (window as any).stopProcess(p.id);
        // Add ID to button for chromedp
        stopBtn.parentElement!.id = p.id;
      }
    })
  } catch {}
}

(window as any).stopProcess = async (id: string) => {
  log('Stopping process ' + id + '...');
  await fetch('/api/stop?id=' + id)
  listProcesses()
};

document.getElementById('start-proc')!.onclick = async () => {
  log('Starting new Nix sub-process...');
  const res = await fetch('/api/processes', { method: 'POST' })
  const data = await res.json()
  listProcesses()
  log('Process ' + data.id + ' started.');
};

document.getElementById('error-ping')!.onclick = () => {
  console.error('[ERROR-PING] Manual error trigger');
  log('Error ping triggered.');
};

setInterval(() => {
  updateStatus()
  listProcesses()
}, 2000)

updateStatus()
listProcesses()
log('Nix Node Manager Initialized.');
