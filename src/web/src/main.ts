import './style.css'
import { connect, JSONCodec, type NatsConnection } from 'nats.ws'

const jc = JSONCodec();
let nc: NatsConnection | null = null;

// DOM Elements
const uptimeEl = document.getElementById('uptime')!;
const platformEl = document.getElementById('platform')!;
const viewerEl = document.getElementById('viewer')!;
const connectionsEl = document.getElementById('connections')!;
const inMsgsEl = document.getElementById('in_msgs')!;
const outMsgsEl = document.getElementById('out_msgs')!;
const inBytesEl = document.getElementById('in_bytes')!;
const outBytesEl = document.getElementById('out_bytes')!;
const tsIpsEl = document.getElementById('ts-ips')!;
// MAVLink Elements
const mavHeartbeatEl = document.getElementById('mav-heartbeat')!;
const mavModeEl = document.getElementById('mav-mode')!;
const mavStatusEl = document.getElementById('mav-status')!;
const mavTypeEl = document.getElementById('mav-type')!;

const hostnameDisplay = document.getElementById('hostname-display')!;
const statusIndicator = document.getElementById('status-indicator')!;
const statusText = document.getElementById('status-text')!;
const subjectInput = document.getElementById('subject') as HTMLInputElement;
const messageInput = document.getElementById('message') as HTMLTextAreaElement;
const sendBtn = document.getElementById('send-btn') as HTMLButtonElement;
const logList = document.getElementById('log-list')!;

function addLog(msg: string, type: 'info' | 'error' | 'success' = 'info') {
  const item = document.createElement('div');
  item.className = `log-item ${type}`;
  item.textContent = `[${new Date().toLocaleTimeString()}] ${msg}`;
  item.setAttribute('aria-label', `Log Entry ${type}`);
  logList.prepend(item);
}

// Initial Data Fetch
async function initApp() {
  try {
    const resp = await fetch('/api/init');
    const config = await resp.json();

    hostnameDisplay.textContent = `: ${config.hostname}`;
    tsIpsEl.textContent = (config.ips || []).join(', ');

    initStatsWS();
    initNatsWS(config.ws_port);
  } catch (err) {
    addLog(`Initialization failed: ${err}`, 'error');
  }
}

// Stats via WebSocket (Existing Dashboard Logic)
function initStatsWS() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

  ws.onmessage = (event) => {
    const stats = JSON.parse(event.data);
    uptimeEl.textContent = stats.uptime;
    platformEl.textContent = `${stats.os} / ${stats.arch}`;
    viewerEl.textContent = stats.caller;
    connectionsEl.textContent = stats.connections;
    inMsgsEl.textContent = stats.in_msgs;
    outMsgsEl.textContent = stats.out_msgs;
    inBytesEl.textContent = stats.in_bytes;
    outBytesEl.textContent = stats.out_bytes;
  };

  ws.onclose = () => {
    setTimeout(initStatsWS, 2000);
  };
}

// Nats via WebSocket (New Messenger Logic)
async function initNatsWS(port: number) {
  try {
    // Use the current hostname/IP for WebSocket connection
    const wsUrl = `ws://${window.location.hostname}:${port}`;
    addLog(`Connecting to NATS at ${wsUrl}...`);

    nc = await connect({
      servers: [wsUrl],
    });

    statusIndicator.className = 'status-online';
    statusText.textContent = 'CONNECTED';
    sendBtn.disabled = false;
    addLog('NATS Messaging Ready', 'success');

    // Subscribe to MAVLink heartbeats
    const sub = nc.subscribe("mavlink.heartbeat");
    (async () => {
      for await (const m of sub) {
        try {
          const msg = jc.decode(m.data) as any;
          mavHeartbeatEl.textContent = new Date(msg.timestamp * 1000).toLocaleTimeString();
          mavModeEl.textContent = msg.base_mode;
          mavStatusEl.textContent = msg.system_status;
          mavTypeEl.textContent = msg.mav_type;
          
          // Flash effect
          mavHeartbeatEl.style.color = '#00ff00';
          setTimeout(() => mavHeartbeatEl.style.color = '', 500);
        } catch (e) {
          console.error("Failed to decode heartbeat", e);
        }
      }
    })();

    nc.closed().then(() => {
      statusIndicator.className = 'status-offline';
      statusText.textContent = 'DISCONNECTED';
      sendBtn.disabled = true;
      addLog('NATS Connection closed', 'error');
    });

  } catch (err) {
    statusIndicator.className = 'status-offline';
    statusText.textContent = 'NATS ERROR';
    addLog(`NATS Connection failed: ${err}`, 'error');
  }
}

// Event Listeners
sendBtn.addEventListener('click', async () => {
  if (!nc) return;

  const subject = subjectInput.value.trim();
  const payload = messageInput.value.trim();

  if (!subject) {
    addLog('Subject is required', 'error');
    return;
  }

  try {
    let data: Uint8Array;
    try {
      // Try as JSON
      const obj = JSON.parse(payload);
      data = jc.encode(obj);
    } catch {
      // Fallback to text
      data = new TextEncoder().encode(payload);
    }

    nc.publish(subject, data);
    addLog(`Published to ${subject}`, 'success');
  } catch (err) {
    addLog(`Publish failed: ${err}`, 'error');
  }
});

initApp();
