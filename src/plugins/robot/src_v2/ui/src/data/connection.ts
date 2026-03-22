import { JSONCodec, connect, type NatsConnection } from 'nats.ws';
import { logError, logInfo, logWarn, setNATSPublisher } from './logging';

let nc: NatsConnection | null = null;
const jc = JSONCodec();

const MAX_EVENT_HISTORY = 500;

type RobotEventCategory =
  | 'mavlink'
  | 'camera'
  | 'command'
  | 'service'
  | 'ui'
  | 'system'
  | 'unknown';

export type RobotEvent = {
  subject: string;
  category: RobotEventCategory;
  payload: any;
  timestamp: string;
  logLine: string;
  level: 'INFO' | 'WARN' | 'ERROR';
  noisy: boolean;
};

type RobotEventListener = (event: RobotEvent) => void;

const eventListeners: RobotEventListener[] = [];
const eventHistory: RobotEvent[] = [];

const subscriptions = [
  'mavlink.>',
  'camera.>',
  'rover.command',
  'robot.>',
  'logs.ui.robot',
] as const;

function setConnectionState(connected: boolean) {
  const value = connected ? 'true' : 'false';
  document.body.setAttribute('data-nats-connected', value);
  const header = document.querySelector("[aria-label='App Header']") as HTMLElement | null;
  if (header) {
    header.setAttribute('data-nats-connected', value);
  }
}

function pushRobotEvent(event: RobotEvent) {
  eventHistory.push(event);
  if (eventHistory.length > MAX_EVENT_HISTORY) {
    eventHistory.splice(0, eventHistory.length - MAX_EVENT_HISTORY);
  }
  eventListeners.forEach((listener) => listener(event));
  (window as any).__robotEventStore = {
    size: eventHistory.length,
    latest: event,
    history: eventHistory.slice(-120),
  };
}

function normalizeTimestamp(payload: any): string {
  if (typeof payload?.timestamp === 'string' && payload.timestamp.trim() !== '') {
    return payload.timestamp;
  }
  if (typeof payload?.timestamp === 'number' && Number.isFinite(payload.timestamp)) {
    return new Date(payload.timestamp).toISOString();
  }
  return new Date().toISOString();
}

function stringifyValue(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function summarizeExtraPayload(payload: Record<string, unknown>): string {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(payload)) {
    if (key === 'cmd' || key === 'mode') continue;
    parts.push(`${key}=${stringifyValue(value)}`);
  }
  return parts.join(' ');
}

function summarizePayload(payload: any): string {
  if (payload == null) return '';
  if (typeof payload.message === 'string' && payload.message.trim() !== '') return payload.message.trim();
  if (typeof payload.text === 'string' && payload.text.trim() !== '') return payload.text.trim();
  if (typeof payload.type === 'string' && payload.type.trim() !== '') {
    switch (payload.type) {
      case 'COMMAND_ACK':
        return `cmd=${stringifyValue(payload.command)} result=${stringifyValue(payload.result)}`;
      case 'HEARTBEAT':
        return `mode=${stringifyValue(payload.custom_mode)} mav_type=${stringifyValue(payload.mav_type)}`;
      case 'GLOBAL_POSITION_INT':
        return `lat=${stringifyValue(payload.lat)} lon=${stringifyValue(payload.lon)} alt=${stringifyValue(payload.alt)}`;
      case 'ATTITUDE':
        return `roll=${stringifyValue(payload.roll)} pitch=${stringifyValue(payload.pitch)} yaw=${stringifyValue(payload.yaw)}`;
      case 'AUTOSWAP_SUPERVISOR':
        return `status=${stringifyValue(payload.status)} worker=${stringifyValue(payload.worker_version)} pid=${stringifyValue(payload.worker_pid)} release=${stringifyValue(payload.last_release_tag)}`;
      case 'AUTOSWAP_RUNTIME':
        return `listen=${stringifyValue(payload.listen)} running=${stringifyValue(payload.running_count)}/${stringifyValue(payload.process_count)} procs=${stringifyValue(payload.process_names)}`;
      default:
        return payload.type;
    }
  }
  return stringifyValue(payload);
}

function normalizeRobotEvent(subject: string, payload: any): RobotEvent {
  const ts = normalizeTimestamp(payload);
  const timeText = new Date(ts).toLocaleTimeString();
  let category: RobotEventCategory = 'unknown';
  let level: 'INFO' | 'WARN' | 'ERROR' = 'INFO';
  let prefix = '[EVENT]';
  let noisy = false;

  if (subject.startsWith('mavlink.')) {
    category = 'mavlink';
    prefix = '[MAVLINK]';
    if (subject === 'mavlink.statustext') {
      prefix = '[MAVLINK][STATUSTEXT]';
      const sev = stringifyValue(payload?.severity).toUpperCase();
      if (sev.includes('CRITICAL') || sev.includes('EMERGENCY') || sev.includes('ALERT') || sev.includes('ERROR')) {
        level = 'ERROR';
      } else if (sev.includes('WARNING')) {
        level = 'WARN';
      }
    } else if (subject === 'mavlink.command_ack') {
      prefix = '[MAVLINK][COMMAND_ACK]';
      if (stringifyValue(payload?.result).toUpperCase().includes('FAILED')) {
        level = 'ERROR';
      }
    } else if (subject === 'mavlink.stats') {
      if (Array.isArray(payload?.errors) && payload.errors.length > 0) {
        prefix = '[MAVLINK][STATS]';
        level = 'ERROR';
      } else {
        noisy = true;
      }
    } else if (['mavlink.heartbeat', 'mavlink.attitude', 'mavlink.global_position_int', 'mavlink.sys_status', 'mavlink.rc_channels', 'mavlink.servo_output_raw', 'mavlink.control_feedback'].includes(subject)) {
      noisy = true;
    }
  } else if (subject.startsWith('camera.')) {
    category = 'camera';
    prefix = '[CAMERA]';
    if (subject === 'camera.heartbeat') noisy = true;
  } else if (subject === 'rover.command') {
    category = 'command';
    prefix = '[COMMAND]';
  } else if (subject.startsWith('robot.')) {
    category = 'service';
    prefix = '[ROBOT]';
    if (Array.isArray(payload?.errors) && payload.errors.length > 0) {
      level = 'ERROR';
    } else if (['robot.service', 'robot.autoswap.supervisor', 'robot.autoswap.runtime'].includes(subject)) {
      noisy = true;
    }
  } else if (subject === 'logs.ui.robot') {
    category = 'ui';
    prefix = `[UI][${stringifyValue(payload?.source) || 'log'}]`;
    const rawLevel = stringifyValue(payload?.level).toUpperCase();
    if (rawLevel === 'ERROR') level = 'ERROR';
    if (rawLevel === 'WARN') level = 'WARN';
  } else {
    category = 'system';
  }

  const summary = summarizePayload(payload);
  const logLine = `[${timeText}] ${prefix} ${summary}`.trim();
  return { subject, category, payload, timestamp: ts, logLine, level, noisy };
}

async function bindSubscriptions(next: NatsConnection) {
  for (const subject of subscriptions) {
    const sub = next.subscribe(subject);
    void (async () => {
      for await (const msg of sub) {
        try {
          const payload = jc.decode(msg.data);
          pushRobotEvent(normalizeRobotEvent(msg.subject, payload));
        } catch (err) {
          logError('ui/connection', `[NATS] Decode error subject=${msg.subject}`, err);
        }
      }
    })();
  }
}

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
    setConnectionState(false);
    nc = await connect({ servers: [server] });
    setNATSPublisher((subject, payload) => {
      if (nc) nc.publish(subject, payload);
    });
    setConnectionState(true);
    logInfo('ui/connection', `[NATS] Connected.`);
    await bindSubscriptions(nc);

    nc.closed().then(() => {
      setNATSPublisher(null);
      setConnectionState(false);
      logWarn('ui/connection', '[NATS] Connection closed, retrying...');
      setTimeout(initConnection, 2000);
    });
  } catch (err) {
    setNATSPublisher(null);
    setConnectionState(false);
    logError('ui/connection', '[NATS] Connection failed', err);
    setTimeout(initConnection, 5000);
  }
}

export function sendCommand(cmd: string, mode?: string, extra?: Record<string, unknown>) {
  if (!nc) {
    logWarn('ui/connection', `[NATS] Not connected, cannot send command: ${cmd}`);
    return;
  }
  const payload: any = { cmd };
  if (mode) payload.mode = mode;
  if (extra && typeof extra === 'object') Object.assign(payload, extra);
  const extraSummary = summarizeExtraPayload(payload);
  const summary = `[NATS] Publishing rover.command cmd=${cmd}${mode ? ` mode=${mode}` : ''}${extraSummary ? ` ${extraSummary}` : ''}`;
  const header = document.querySelector("[aria-label='App Header']") as HTMLElement | null;
  if (header) {
    header.setAttribute('data-last-rover-command', cmd);
    header.setAttribute('data-last-rover-command-mode', mode || '');
    header.setAttribute('data-last-rover-command-extra', extraSummary);
  }
  logInfo('ui/connection', summary);
  nc.publish('rover.command', jc.encode(payload));
}

export function addRobotEventListener(cb: RobotEventListener, opts?: { replay?: boolean }) {
  if (opts?.replay !== false) {
    eventHistory.forEach((event) => cb(event));
  }
  eventListeners.push(cb);
  return () => {
    const idx = eventListeners.indexOf(cb);
    if (idx >= 0) eventListeners.splice(idx, 1);
  };
}

export function addMavlinkListener(cb: (data: any) => void) {
  return addRobotEventListener((event) => {
    if (event.subject.startsWith('mavlink.')) {
      cb(event.payload);
    }
  });
}

export function getRobotEventHistory(filter?: { category?: RobotEventCategory | 'all' }) {
  const wanted = filter?.category || 'all';
  if (wanted === 'all') return [...eventHistory];
  return eventHistory.filter((event) => event.category === wanted);
}
