import { JSONCodec, connect, type NatsConnection, type Subscription } from "nats.ws";

export type RoverState = {
  connected: boolean;
  mode: string;
  batteryV: string;
  speedMS: string;
  altitudeM: string;
  headingDeg: string;
  latitude: string;
  longitude: string;
  satellites: string;
  link: string;
  fps: string;
  bitrate: string;
  feed: string;
  latencyMS: string;
  logs: string[];
  steeringProfile: Array<[string, string, string]>;
  keyParams: Array<[string, string, string]>;
};

type InitResponse = {
  ws_path?: string;
  ws_port?: number;
  internal_ws_port?: number;
};

type StateListener = (state: RoverState) => void;

const jc = JSONCodec();
const listeners = new Set<StateListener>();

let nc: NatsConnection | null = null;
let reconnectTimer = 0;

const state: RoverState = {
  connected: false,
  mode: "BOOT",
  batteryV: "0.0",
  speedMS: "0.0",
  altitudeM: "0.0",
  headingDeg: "0",
  latitude: "0.0000",
  longitude: "0.0000",
  satellites: "0",
  link: "offline",
  fps: "0",
  bitrate: "0.0 Mbps",
  feed: "mock-a",
  latencyMS: "0",
  logs: ["[mock] waiting for rover stream"],
  steeringProfile: [
    ["STEER_TRIM", "1500", "mock"],
    ["TURN_RATE_MAX", "28", "mock"],
    ["THR_EXPO", "0.42", "mock"],
    ["BRAKE_FORCE", "0.18", "mock"],
  ],
  keyParams: [
    ["CRUISE_SPEED", "2.8", "mock"],
    ["RTL_SPEED", "2.1", "mock"],
    ["NAVL1_PERIOD", "12", "mock"],
    ["WPNAV_RADIUS", "1.4", "mock"],
  ],
};

function emit(): void {
  for (const listener of listeners) {
    listener({ ...state, logs: [...state.logs], steeringProfile: [...state.steeringProfile], keyParams: [...state.keyParams] });
  }
}

function appendLog(line: string): void {
  state.logs = [...state.logs.slice(-79), line];
}

function scheduleReconnect(delayMS: number): void {
  if (reconnectTimer) {
    window.clearTimeout(reconnectTimer);
  }
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = 0;
    void startMockConnection();
  }, delayMS);
}

async function consume(sub: Subscription): Promise<void> {
  for await (const msg of sub) {
    try {
      const payload = jc.decode(msg.data) as Record<string, unknown>;
      applyMessage(msg.subject, payload);
    } catch {
      appendLog(`[decode] failed subject=${msg.subject}`);
      emit();
    }
  }
}

function applyMessage(subject: string, payload: Record<string, unknown>): void {
  switch (subject) {
    case "mavlink.heartbeat":
      state.connected = true;
      state.link = "online";
      state.mode = String(payload.mode ?? "GUIDED");
      break;
    case "mavlink.vfr_hud":
      state.speedMS = Number(payload.groundspeed ?? payload.airspeed ?? 0).toFixed(1);
      state.altitudeM = Number(payload.alt ?? 0).toFixed(1);
      state.headingDeg = String(Math.round(Number(payload.heading ?? 0)));
      break;
    case "mavlink.sys_status":
      state.batteryV = (Number(payload.voltage_battery ?? 0) / 1000).toFixed(1);
      break;
    case "mavlink.gps_raw_int":
      state.satellites = String(payload.satellites_visible ?? 0);
      break;
    case "mavlink.global_position_int":
      state.latitude = Number(payload.lat ?? 0).toFixed(4);
      state.longitude = Number(payload.lon ?? 0).toFixed(4);
      if (payload.relative_alt !== undefined) {
        state.altitudeM = Number(payload.relative_alt).toFixed(1);
      }
      if (payload.hdg !== undefined) {
        state.headingDeg = String(Math.round(Number(payload.hdg) / 100));
      }
      break;
    case "rover.status":
      state.mode = String(payload.mode ?? state.mode);
      state.feed = String(payload.feed ?? state.feed);
      state.fps = String(payload.fps ?? state.fps);
      state.latencyMS = String(payload.latency_ms ?? state.latencyMS);
      state.bitrate = `${Number(payload.bitrate_mbps ?? 0).toFixed(1)} Mbps`;
      break;
    case "rover.steering":
      state.steeringProfile = [
        ["STEER_TRIM", String(payload.trim ?? 1500), "live"],
        ["TURN_RATE_MAX", String(payload.turn_rate_max ?? 28), "live"],
        ["THR_EXPO", String(payload.throttle_expo ?? 0.42), "live"],
        ["BRAKE_FORCE", String(payload.brake_force ?? 0.18), "live"],
      ];
      break;
    case "rover.params":
      state.keyParams = [
        ["CRUISE_SPEED", String(payload.cruise_speed ?? 2.8), "live"],
        ["RTL_SPEED", String(payload.rtl_speed ?? 2.1), "live"],
        ["NAVL1_PERIOD", String(payload.nav_l1_period ?? 12), "live"],
        ["WPNAV_RADIUS", String(payload.wpnav_radius ?? 1.4), "live"],
      ];
      break;
    case "rover.log":
      appendLog(String(payload.line ?? "[mock] log"));
      break;
    case "rover.command_ack":
      appendLog(`[ack] ${String(payload.cmd ?? "command")} => ${String(payload.status ?? "ok")}`);
      break;
    default:
      if (subject.startsWith("mavlink.")) {
        appendLog(`[mavlink] ${subject}`);
      }
      break;
  }
  emit();
}

export async function startMockConnection(): Promise<void> {
  try {
    const initRes = await fetch("/api/init", { cache: "no-store" });
    if (!initRes.ok) {
      throw new Error(`init failed with ${initRes.status}`);
    }
    const initData = (await initRes.json()) as InitResponse;
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsPath = initData.ws_path ?? "/natsws";
    const wsPort = initData.internal_ws_port ?? initData.ws_port ?? 4223;
    const server = wsPath
      ? `${protocol}//${window.location.host}${wsPath}`
      : `${protocol}//${window.location.hostname}:${wsPort}`;

    nc?.close();
    nc = await connect({ servers: [server] });
    state.connected = true;
    state.link = "online";
    appendLog(`[mock] connected ${server}`);
    emit();

    void consume(nc.subscribe("mavlink.>"));
    void consume(nc.subscribe("rover.>"));

    nc.closed().then(() => {
      state.connected = false;
      state.link = "reconnecting";
      appendLog("[mock] connection closed");
      emit();
      scheduleReconnect(1500);
    });
  } catch (error) {
    state.connected = false;
    state.link = "offline";
    appendLog(`[mock] connect failed: ${error instanceof Error ? error.message : String(error)}`);
    emit();
    scheduleReconnect(2000);
  }
}

export function subscribeRoverState(listener: StateListener): () => void {
  listeners.add(listener);
  listener({ ...state, logs: [...state.logs], steeringProfile: [...state.steeringProfile], keyParams: [...state.keyParams] });
  return () => {
    listeners.delete(listener);
  };
}

export function sendRoverCommand(cmd: string, extra: Record<string, unknown> = {}): void {
  if (!nc) {
    appendLog(`[mock] command dropped: ${cmd}`);
    emit();
    return;
  }
  nc.publish("rover.command", jc.encode({ cmd, ...extra }));
  appendLog(`[cmd] ${cmd}`);
  emit();
}
