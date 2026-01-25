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

term.writeln('\x1b[1;36m>>> DIALTONE INTERFACE INITIALIZED\x1b[0m');
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
  hud: {
    alt: document.getElementById('hud-alt'),
    spd: document.getElementById('hud-spd'),
    mode: document.getElementById('hud-mode')
  },
  connStatus: document.getElementById('conn-status')
};

// Start Connection
async function connectNATS() {
  const wsPort = 4223; // Standard NATS WS port
  // Use hostname from window location (handles remote/local)
  const server = `${PROTOCOL}//${HOSTNAME}:${wsPort}`;

  term.writeln(`\x1b[90m>>> Connecting to NATS at ${server}...\x1b[0m`);

  try {
    nc = await connect({ servers: [server] });
    updateStatus(true);

    // Subscribe to ALL mavlink messages for debugging/telemetry
    const sub = nc.subscribe("mavlink.>");
    (async () => {
      let msgCount = 0;
      for await (const m of sub) {
        msgCount++;
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

  // HUD (VFR_HUD)
  if (data.airspeed !== undefined || data.vfr_hud) {
    const src = data.vfr_hud || data;
    if (els.hud.spd) els.hud.spd.innerText = parseFloat(src.airspeed).toFixed(1);
    if (els.hud.alt) els.hud.alt.innerText = parseFloat(src.alt).toFixed(1);
  }

  // Attitude
  if (data.roll !== undefined || data.attitude) {
    const att = data.attitude || data;
    // Three.js rotation (order might need tuning)
    robotGroup.rotation.z = -att.roll;  // Roll
    robotGroup.rotation.x = att.pitch; // Pitch
    robotGroup.rotation.y = -att.yaw;   // Yaw
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
