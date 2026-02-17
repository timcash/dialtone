import { JSONCodec, connect, type NatsConnection } from 'nats.ws';

let nc: NatsConnection | null = null;
const jc = JSONCodec();
const listeners: ((data: any) => void)[] = [];
let statusInterval: number | undefined;

export async function initConnection() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const hostname = window.location.hostname;

  try {
    const res = await fetch('/api/init');
    const data = await res.json();
    // Use internal_ws_port if available (dynamic discovery), fallback to ws_port
    const wsPort = data.internal_ws_port || data.ws_port || 4223;
    const wsPath = data.ws_path || '';

    let server = '';
    if (wsPath) {
      server = `${protocol}//${window.location.host}${wsPath}`;
    } else {
      server = `${protocol}//${hostname}:${wsPort}`;
    }

    console.log(`[NATS] Connecting to ${server}...`);
    nc = await connect({ servers: [server] });
    console.log(`[NATS] Connected.`);

    // Subscribe to Mavlink
    const sub = nc.subscribe('mavlink.>');
    (async () => {
      for await (const m of sub) {
        try {
          const payload = jc.decode(m.data);
          emit(payload);
        } catch (err) {
          console.error('[NATS] Decode error:', err);
        }
      }
    })();

    nc.closed().then(() => {
      console.warn('[NATS] Connection closed, retrying...');
      setTimeout(initConnection, 2000);
    });

    // Start polling system status to replace server-side ticker
    startStatusPolling();

  } catch (err) {
    console.error('[NATS] Connection failed:', err);
    setTimeout(initConnection, 5000);
  }
}

function startStatusPolling() {
  if (statusInterval) clearInterval(statusInterval);
  statusInterval = window.setInterval(async () => {
    try {
      const res = await fetch('/api/status');
      if (res.ok) {
        const stats = await res.json();
        // Flatten structure to match UI expectations where possible
        emit({
          uptime: stats.uptime,
          nats_total: stats.nats?.messages_in || 0,
          connections: stats.nats?.connections || 0,
          type: 'SYSTEM_STATUS' // Tag it so listeners can filter if needed
        });
      }
    } catch (e) {
      // Ignore poll errors
    }
  }, 1000);
}

function emit(data: any) {
  listeners.forEach(l => l(data));
}

export function sendCommand(cmd: string, mode?: string) {
  if (!nc) {
    console.warn('[NATS] Not connected, cannot send command:', cmd);
    return;
  }
  const payload: any = { cmd };
  if (mode) payload.mode = mode;
  nc.publish('rover.command', jc.encode(payload));
}

export function addMavlinkListener(cb: (data: any) => void) {
  listeners.push(cb);
  return () => {
    const idx = listeners.indexOf(cb);
    if (idx >= 0) listeners.splice(idx, 1);
  };
}
