import { JSONCodec, connect, type NatsConnection } from 'nats.ws';
import { logError, logInfo, logWarn, setNATSPublisher } from './logging';

let nc: NatsConnection | null = null;
const jc = JSONCodec();
const listeners: ((data: any) => void)[] = [];

export async function initConnection() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const hostname = window.location.hostname;

  try {
    const initRes = await fetch('/api/init');
    const initData = await initRes.json();
    const wsPath = initData.ws_path || '';
    const wsPort = initData.internal_ws_port || initData.ws_port || 4223;
    const server = wsPath
      ? `${protocol}//${window.location.host}${wsPath}`
      : `${protocol}//${hostname}:${wsPort}`;
    logInfo('ui/connection', `[NATS] Connecting to ${server}...`);
    nc = await connect({ servers: [server] });
    setNATSPublisher((subject, payload) => {
      if (nc) nc.publish(subject, payload);
    });
    logInfo('ui/connection', `[NATS] Connected.`);

    // Subscribe to Mavlink
    const sub = nc.subscribe('mavlink.>');
    (async () => {
      for await (const m of sub) {
        try {
          const payload = jc.decode(m.data);
          emit(payload);
        } catch (err) {
          logError('ui/connection', '[NATS] Decode error', err);
        }
      }
    })();

    nc.closed().then(() => {
      setNATSPublisher(null);
      logWarn('ui/connection', '[NATS] Connection closed, retrying...');
      setTimeout(initConnection, 2000);
    });
  } catch (err) {
    setNATSPublisher(null);
    logError('ui/connection', '[NATS] Connection failed', err);
    setTimeout(initConnection, 5000);
  }
}

function emit(data: any) {
  listeners.forEach(l => l(data));
}

export function sendCommand(cmd: string, mode?: string, extra?: Record<string, unknown>) {
  if (!nc) {
    logWarn('ui/connection', `[NATS] Not connected, cannot send command: ${cmd}`);
    return;
  }
  const payload: any = { cmd };
  if (mode) payload.mode = mode;
  if (extra && typeof extra === 'object') Object.assign(payload, extra);
  logInfo('ui/connection', `[NATS] Publishing rover.command cmd=${cmd}${mode ? ` mode=${mode}` : ''}`);
  nc.publish('rover.command', jc.encode(payload));
}

export function addMavlinkListener(cb: (data: any) => void) {
  listeners.push(cb);
  return () => {
    const idx = listeners.indexOf(cb);
    if (idx >= 0) listeners.splice(idx, 1);
  };
}
