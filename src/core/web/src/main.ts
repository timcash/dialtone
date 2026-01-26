import './style.css';
import * as THREE from 'three';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { connect, JSONCodec, type NatsConnection } from 'nats.ws';

// --- State & Constants ---
const HOSTNAME = window.location.hostname;
const PROTOCOL = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

// --- 1. Terminal Setup (Left Column) ---
const term = new Terminal({
  theme: {
    background: '#0a0a0c',
    foreground: '#c5c6c7',
    cursor: '#66fcf1',
    selectionBackground: 'rgba(102, 252, 241, 0.3)'
  },
  fontFamily: '"Orbitron", monospace',
  fontSize: 12,
  cursorBlink: true,
  convertEol: true
});
const fitAddon = new FitAddon();
term.loadAddon(fitAddon);

const termContainer = document.getElementById('terminal-container');
if (termContainer) {
  term.open(termContainer);
  fitAddon.fit();
}

term.writeln('\x1b[1;36m>>> DIALTONE INTERFACE INITIALIZED (v1.0.1)\x1b[0m');
term.writeln('\x1b[90m>>> Waiting for connection...\x1b[0m');

// Resize Observer for Terminal
new ResizeObserver(() => {
  fitAddon.fit();
}).observe(termContainer!);


// --- 2. 3D Visualization Setup (Center Column) ---
const threeContainer = document.getElementById('three-container');
const scene = new THREE.Scene();
scene.background = new THREE.Color(0x000000);

// Camera
const fov = 75;
const aspect = threeContainer ? threeContainer.clientWidth / threeContainer.clientHeight : window.innerWidth / window.innerHeight;
const near = 0.1;
const far = 1000;
const camera = new THREE.PerspectiveCamera(fov, aspect, near, far);
camera.position.set(0, -10, 10);
camera.lookAt(0, 0, 0);

// Renderer
const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
if (threeContainer) {
  renderer.setSize(threeContainer.clientWidth, threeContainer.clientHeight);
  threeContainer.appendChild(renderer.domElement);
}

// Lighting
const ambientLight = new THREE.AmbientLight(0xffffff, 0.5);
scene.add(ambientLight);
const dirLight = new THREE.DirectionalLight(0xffffff, 1);
dirLight.position.set(10, 10, 10);
scene.add(dirLight);

// Grid (The "2d plane" reference)
const gridHelper = new THREE.GridHelper(50, 50, 0x333333, 0x111111);
gridHelper.rotation.x = Math.PI / 2;
scene.add(gridHelper);

// Robot Representation (Triangle/Cone with Rings)
const robotGroup = new THREE.Group();
scene.add(robotGroup);

// Main Body (Triangle/Cone)
const geometry = new THREE.ConeGeometry(1, 3, 4); // 4 sides = pyramid/arrow-like
const material = new THREE.MeshPhongMaterial({ color: 0x66fcf1, wireframe: false });
const robotMesh = new THREE.Mesh(geometry, material);
robotMesh.rotation.x = Math.PI / 2; // Point forward along Y? standard definition varies
robotGroup.add(robotMesh);

// Euler Rings
const ringGeo = new THREE.TorusGeometry(2, 0.05, 16, 100);
const ringMatX = new THREE.MeshBasicMaterial({ color: 0xff4d4d }); // Pitch
const ringMatY = new THREE.MeshBasicMaterial({ color: 0x00ff00 }); // Roll
const ringMatZ = new THREE.MeshBasicMaterial({ color: 0x0000ff }); // Yaw
const ringX = new THREE.Mesh(ringGeo, ringMatX);
const ringY = new THREE.Mesh(ringGeo, ringMatY);
const ringZ = new THREE.Mesh(ringGeo, ringMatZ);

ringY.rotation.x = Math.PI / 2;

robotGroup.add(ringX); // Visualization only
robotGroup.add(ringY);
robotGroup.add(ringZ);


// Helper for resizing
window.addEventListener('resize', () => {
  if (threeContainer) {
    const width = threeContainer.clientWidth;
    const height = threeContainer.clientHeight;
    renderer.setSize(width, height);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  }
});

function animate() {
  requestAnimationFrame(animate);
  renderer.render(scene, camera);
}
animate();

// --- 3. Telemetry & Commands (Connection Logic) ---

const jc = JSONCodec();
let nc: NatsConnection | null = null;
let isConnected = false;

// UI Elements
const els = {
  heartbeat: document.getElementById('val-heartbeat'),
  batt: document.getElementById('val-batt'),
  sats: document.getElementById('val-sats'),
  nats: document.getElementById('val-nats'),
  lat: document.getElementById('val-lat'),
  lon: document.getElementById('val-lon'),
  rp: document.getElementById('val-rp'),
  yaw: document.getElementById('val-yaw'),
  hud: {
    alt: document.getElementById('hud-alt'),
    spd: document.getElementById('hud-spd'),
    mode: document.getElementById('hud-mode')
  },
  alertFeed: document.getElementById('alert-feed'),
  btnManual: document.getElementById('btn-manual'),
  btnGuided: document.getElementById('btn-guided'),
  connStatus: document.getElementById('conn-status'),
  version: document.getElementById('ui-version')
};



// Fetch Config/Init
fetch('/api/init')
  .then(res => res.json())
  .then(data => {
    if (els.version && data.version) {
      els.version.innerText = data.version;
    }
  })
  .catch(err => console.error("Failed to fetch init:", err));

// Start Connection
async function connectNATS() {
  const wsPort = 4223; // Standard NATS WS port
  // Use 127.0.0.1 for local/headless consistency, otherwise use location hostname
  const host = (HOSTNAME === 'localhost' || HOSTNAME === '127.0.0.1') ? '127.0.0.1' : HOSTNAME;
  const server = `${PROTOCOL}//${host}:${wsPort}`;

  console.log(`NATS Connection Info: PROTOCOL=${PROTOCOL}, HOSTNAME=${HOSTNAME}, host=${host}, server=${server}`);
  term.writeln(`\x1b[90m>>> Connecting to NATS at ${server}...\x1b[0m`);

  try {
    console.log(`Attempting to connect to NATS at ${server}...`);
    nc = await connect({ servers: [server] });
    console.log("NATS Connected!");
    updateStatus(true);

    // Subscribe to ALL mavlink messages for debugging/telemetry
    const sub = nc.subscribe("mavlink.>");
    console.log("Subscribed to mavlink.>");
    (async () => {
      let msgCount = 0;
      for await (const m of sub) {
        msgCount++;
        console.log(`Received message ${msgCount} on ${m.subject}`);
        if (els.nats) els.nats.innerText = msgCount.toString();

        try {
          const data = jc.decode(m.data) as any;
          handleMessage(data, m.subject);
        } catch (e) {
          // Ignore decode errors
        }
      }
    })();

    // Subscribe to specific topics if needed separately
    // But wildcard covers it.

    nc.closed().then(() => {
      updateStatus(false);
      setTimeout(connectNATS, 2000);
    });

  } catch (err) {
    term.writeln(`\x1b[1;31m>>> NATS CONNECTION FAILED: ${err}\x1b[0m`);
    setTimeout(connectNATS, 5000);
  }
}

function handleMessage(data: any, subject: string) {
  // Heartbeat
  if (subject.includes("heartbeat") || data.type === 'HEARTBEAT') {
    if (els.heartbeat) {
      els.heartbeat.innerText = "ACTIVE";
      els.heartbeat.style.color = "#00ff00";
      setTimeout(() => els.heartbeat ? els.heartbeat.style.color = "" : null, 500);
    }
    if (els.hud.mode && data.custom_mode !== undefined) {
      els.hud.mode.innerText = `MODE ${data.custom_mode}`;
    }
  }

  // System Status / Battery
  if (data.voltage_battery) {
    if (els.batt) els.batt.innerText = (data.voltage_battery / 1000).toFixed(1) + " V";
  }
  // If it comes as sys_status object
  if (data.sys_status) {
    if (els.batt) els.batt.innerText = (data.sys_status.voltage_battery / 1000).toFixed(1) + " V";
  }

  // GPS
  if (data.satellites_visible !== undefined || (data.gps_raw_int && data.gps_raw_int.satellites_visible)) {
    const count = data.satellites_visible ?? data.gps_raw_int.satellites_visible;
    if (els.sats) els.sats.innerText = count;
  }

  // Global Position (GPS Coord)
  if (subject.includes("global_position_int") || data.lat !== undefined) {
    if (els.lat) els.lat.innerText = data.lat.toFixed(6);
    if (els.lon) els.lon.innerText = data.lon.toFixed(6);
    if (els.hud.alt && data.relative_alt !== undefined) {
      els.hud.alt.innerText = data.relative_alt.toFixed(1);
    }
  }

  // HUD (VFR_HUD)
  if (data.airspeed !== undefined || data.vfr_hud) {
    const src = data.vfr_hud || data;
    if (els.hud.spd) els.hud.spd.innerText = parseFloat(src.airspeed).toFixed(1);
    if (els.hud.alt && src.alt !== undefined) els.hud.alt.innerText = parseFloat(src.alt).toFixed(1);
  }

  // Attitude
  if (data.roll !== undefined || data.attitude || subject.includes("attitude")) {
    const att = data.attitude || data;
    // Three.js rotation (order might need tuning)
    robotGroup.rotation.z = -att.roll;  // Roll
    robotGroup.rotation.x = att.pitch; // Pitch
    robotGroup.rotation.y = -att.yaw;   // Yaw

    // Text Display
    if (els.rp) {
      const r = (att.roll * 180 / Math.PI).toFixed(1);
      const p = (att.pitch * 180 / Math.PI).toFixed(1);
      els.rp.innerText = `${r}° / ${p}°`;
    }
    if (els.yaw) {
      let y = (att.yaw * 180 / Math.PI);
      if (y < 0) y += 360;
      els.yaw.innerText = y.toFixed(1) + "°";
    }
  }

  // Status Text / Alerts
  if (subject.includes("statustext") || data.severity !== undefined) {
    addAlert(data.text || data, data.severity || 6);
  }

  // Command ACK
  if (subject.includes("ack") || data.command !== undefined) {
    const result = data.result || 0;
    addAlert(`CMD ${data.command} ACK: ${result === 0 ? 'OK' : 'FAIL (' + result + ')'}`, result === 0 ? 6 : 3);
  }
}

function addAlert(text: string, severity: number) {
  if (!els.alertFeed) return;

  const li = document.createElement('li');
  li.innerText = `${new Date().toLocaleTimeString([], { hour12: false })} ${text}`;

  // Severity coloring (0-7, where 0-2 are critical/error, 3-4 warning, 5-7 info)
  if (severity <= 2) li.classList.add('alert-danger');
  else if (severity <= 4) li.classList.add('alert-warning');
  else li.classList.add('alert-info');

  els.alertFeed.prepend(li);

  // Keep only last 15
  while (els.alertFeed.children.length > 15) {
    els.alertFeed.removeChild(els.alertFeed.lastChild!);
  }
}

function updateStatus(online: boolean) {
  if (els.connStatus) {
    els.connStatus.textContent = online ? 'ONLINE' : 'OFFLINE';
    if (online) els.connStatus.classList.add('online');
    else els.connStatus.classList.remove('online');
  }

  if (online && !isConnected) {
    term.writeln('\x1b[1;32m>>> CONNECTED TO BACKEND\x1b[0m');
  } else if (!online && isConnected) {
    term.writeln('\x1b[1;31m>>> CONNECTION LOST\x1b[0m');
  }
  isConnected = online;
}

connectNATS();

// Command Buttons
document.getElementById('btn-arm')?.addEventListener('click', () => {
  term.writeln('\x1b[33m[CMD] ARM SYSTEM REQUESTED\x1b[0m');
  if (nc) {
    nc.publish("rover.command", jc.encode({ cmd: "arm" }));
  }
});

document.getElementById('btn-disarm')?.addEventListener('click', () => {
  term.writeln('\x1b[33m[CMD] DISARM SYSTEM REQUESTED\x1b[0m');
  if (nc) {
    nc.publish("rover.command", jc.encode({ cmd: "disarm" }));
  }
});

document.getElementById('btn-manual')?.addEventListener('click', () => {
  term.writeln('\x1b[33m[CMD] SET MODE: MANUAL REQUESTED\x1b[0m');
  if (nc) {
    nc.publish("rover.command", jc.encode({ cmd: "mode", mode: "manual" }));
  }
});

document.getElementById('btn-guided')?.addEventListener('click', () => {
  term.writeln('\x1b[33m[CMD] SET MODE: GUIDED REQUESTED\x1b[0m');
  if (nc) {
    nc.publish("rover.command", jc.encode({ cmd: "mode", mode: "guided" }));
  }
});

// Input handling
const cmdInput = document.getElementById('cmd-input') as HTMLInputElement;
cmdInput?.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    const val = cmdInput.value;
    if (val) {
      term.writeln(`\r\n$ ${val}`);
      // Parse command? Or just generic publish?
      // For now, publish as command log
      term.writeln('\x1b[90mCommand sent...\x1b[0m');
      cmdInput.value = '';
    }
  }
});
