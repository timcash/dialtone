import * as THREE from 'three';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';

// --- Terminal Setup (Commands) ---
const term = new Terminal({
    theme: { background: 'transparent' },
    fontFamily: 'Consolas, "Courier New", monospace',
    fontSize: 12,
    cursorBlink: true
});
const fitAddon = new FitAddon();
term.loadAddon(fitAddon);

const terminalEl = document.getElementById('terminal');
if (terminalEl) {
    term.open(terminalEl);
    fitAddon.fit();
}

// --- MAVLink Log Setup ---
const mavlinkTerm = new Terminal({
    theme: { background: 'transparent' },
    fontFamily: 'Consolas, "Courier New", monospace',
    fontSize: 12,
    cursorBlink: false
});
const mavlinkFitAddon = new FitAddon();
mavlinkTerm.loadAddon(mavlinkFitAddon);

const mavlinkEl = document.getElementById('mavlink-log');
if (mavlinkEl) {
    mavlinkTerm.open(mavlinkEl);
    mavlinkFitAddon.fit();
}

// Subscribe to SSE streams
const terminalEventSource = new EventSource('/terminal-stream');
terminalEventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    term.write(data.message);
};

const mavlinkEventSource = new EventSource('/mavlink-stream');
mavlinkEventSource.onmessage = (event) => {
    const data = JSON.parse(event.data);
    mavlinkTerm.write(data.message);
};

// Subscribe to WebSocket for telemetry
const telemetryDot = document.getElementById('telemetry-dot');
const telemetryStatus = document.getElementById('telemetry-status');
const hudMode = document.getElementById('hud-mode');

let lastTelemetryTime = 0;
let telemetryWS: WebSocket | null = null;
let parameters: Record<string, any> = {};

function connectTelemetry() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const backendHost = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
        ? 'localhost:3002'
        : `${window.location.host}`;
    const wsUrl = `${protocol}//${backendHost}/telemetry-ws`;

    console.log(`Connecting to Telemetry WebSocket at: ${wsUrl}`);
    telemetryWS = new WebSocket(wsUrl);
    (window as any).telemetryWS = telemetryWS; // Expose for testing

    telemetryWS.onopen = () => {
        console.log('Telemetry WebSocket connected');
    };

    telemetryWS.onerror = (error) => {
        console.error('Telemetry WebSocket error:', error);
        (window as any).lastWSError = error;
    };

    telemetryWS.onmessage = (event) => {
        const data = JSON.parse(event.data);
        (window as any).telemetryData = data; // For testing/debugging
        lastTelemetryTime = Date.now();

        if (data.status && data.status.running !== undefined) {
            updateTelemetryUI(data.status.running);
            return;
        }

        if (telemetryDot) {
            telemetryDot.classList.add('online');
            telemetryDot.classList.remove('offline');
        }

        // Update orientation from telemetry
        if (data.attitude) {
            arrowGroup.rotation.x = data.attitude.pitch;
            arrowGroup.rotation.z = data.attitude.roll;
            arrowGroup.rotation.y = -data.attitude.yaw;
        }

        // Update HUD / Status
        if (data.mode) {
            if (telemetryStatus) {
                const armText = data.armed ? '<span style="color:#00ff00">ARMED</span>' : '<span style="color:#ff0000">DISARMED</span>';
                telemetryStatus.innerHTML = `MAVLink: ${data.mode} (${armText})`;
            }
            if (hudMode) {
                hudMode.innerText = data.mode;
                hudMode.style.color = data.armed ? '#00ffcc' : '#ff4d4d';
            }

            if (data.battery_voltage !== undefined) {
                const batteryEl = document.getElementById('hud-battery');
                if (batteryEl) batteryEl.innerText = `${data.battery_voltage.toFixed(2)}V (${data.battery_remaining}%)`;
            }
        }

        if (data.speed !== undefined) {
            const speedEl = document.getElementById('hud-speed');
            if (speedEl) speedEl.innerText = `${data.speed.toFixed(1)} m/s`;
        }

        if (data.gps && data.gps.altitude !== undefined) {
            const altEl = document.getElementById('hud-alt');
            if (altEl) altEl.innerText = `${data.gps.altitude.toFixed(1)} m`;
        }

        if (data.gps) {
            const gpsEl = document.getElementById('hud-gps');
            if (gpsEl) gpsEl.innerText = `GPS: ${data.gps.fix_type > 2 ? 'FIX' : 'NO FIX'} (${data.gps.satellites} sats)`;

            const gpsCoordsEl = document.getElementById('hud-gps-coords');
            if (gpsCoordsEl && data.gps) {
                const lat = typeof data.gps.latitude === 'number' ? data.gps.latitude.toFixed(6) : '--';
                const lon = typeof data.gps.longitude === 'number' ? data.gps.longitude.toFixed(6) : '--';
                gpsCoordsEl.innerText = `LAT: ${lat} | LON: ${lon}`;
                console.log(`Updated GPS: ${lat}, ${lon} (fix: ${data.gps.fix_type})`);
            }
        }

        // If parameters are sent
        if (data.parameters) {
            parameters = data.parameters;
            updateParameterTable();
        }
    };

    telemetryWS.onclose = () => {
        console.log('Telemetry WebSocket closed');
        updateTelemetryUI(false);
        setTimeout(connectTelemetry, 2000);
    };

    telemetryWS.onerror = (err) => {
        console.error('Telemetry WebSocket error:', err);
    };
}

function updateTelemetryUI(running: boolean) {
    if (telemetryDot) {
        if (running) {
            telemetryDot.classList.add('online');
            telemetryDot.classList.remove('offline');
        } else {
            telemetryDot.classList.add('offline');
            telemetryDot.classList.remove('online');
        }
    }
    if (telemetryStatus) {
        telemetryStatus.innerText = running ? 'MAVLink Connected' : 'MAVLink Disconnected';
    }
}

function updateParameterTable() {
    const tbody = document.getElementById('parameter_body');
    if (!tbody) return;

    tbody.innerHTML = '';
    const sortedKeys = Object.keys(parameters).sort();

    if (sortedKeys.length === 0) {
        tbody.innerHTML = '<tr><td colspan="2" style="text-align:center; color: #444;">No parameters received</td></tr>';
        return;
    }

    sortedKeys.forEach(key => {
        const row = document.createElement('tr');
        const nameCell = document.createElement('td');
        const valCell = document.createElement('td');

        nameCell.innerText = key;
        valCell.innerText = typeof parameters[key] === 'object' ? parameters[key].value : parameters[key];

        row.appendChild(nameCell);
        row.appendChild(valCell);
        tbody.appendChild(row);
    });
}

connectTelemetry();

// Update connection status dot if no data received
setInterval(() => {
    if (Date.now() - lastTelemetryTime > 3000) {
        if (telemetryDot) {
            telemetryDot.classList.add('offline');
            telemetryDot.classList.remove('online');
        }
    }
}, 1000);

// --- Three.js Setup ---
const scene = new THREE.Scene();
scene.background = new THREE.Color(0x050505);

const canvas = document.getElementById('threejs') as HTMLCanvasElement;
const mainView = document.getElementById('main-view');

const camera = new THREE.PerspectiveCamera(75, (mainView?.clientWidth || window.innerWidth) / (mainView?.clientHeight || window.innerHeight), 0.1, 1000);
const renderer = new THREE.WebGLRenderer({ canvas: canvas, antialias: true });

if (mainView) {
    renderer.setSize(mainView.clientWidth, mainView.clientHeight);
} else {
    renderer.setSize(window.innerWidth, window.innerHeight);
}

renderer.setPixelRatio(window.devicePixelRatio);

// Orientation Arrow (Representing the Rover)
const arrowGroup = new THREE.Group();
scene.add(arrowGroup);

const bodyGeom = new THREE.ConeGeometry(0.5, 2, 32);
const bodyMat = new THREE.MeshPhongMaterial({ color: 0x00ffcc });
const body = new THREE.Mesh(bodyGeom, bodyMat);
body.rotation.x = Math.PI / 2;
arrowGroup.add(body);

const wingGeom = new THREE.BoxGeometry(2, 0.1, 0.5);
const wingMat = new THREE.MeshPhongMaterial({ color: 0xff4d4d });
const wings = new THREE.Mesh(wingGeom, wingMat);
arrowGroup.add(wings);

// Ground Grid
const grid = new THREE.GridHelper(20, 20, 0x333333, 0x111111);
grid.rotation.x = Math.PI / 2;
scene.add(grid);

// Lighting
const ambientLight = new THREE.AmbientLight(0xffffff, 0.4);
scene.add(ambientLight);

const directionalLight = new THREE.DirectionalLight(0xffffff, 1);
directionalLight.position.set(5, 5, 5);
scene.add(directionalLight);

camera.position.set(0, -5, 5);
camera.lookAt(0, 0, 0);

// --- Button List Event Listeners ---
const commandMap: Record<string, string> = {
    'btn-connect': 'connect',
    'btn-arm': '--rover-arm',
    'btn-disarm': '--rover-disarm',
    'btn-cam-start': '--start-camera',
    'btn-cam-stop': '--stop-camera',
    'btn-fwd': 'move forward',
    'btn-bwd': 'move backward',
    'btn-left-half': 'turn left half',
    'btn-right-half': 'turn right half',
    'btn-left-full': 'turn left full',
    'btn-right-full': 'turn right full'
};

Object.entries(commandMap).forEach(([id, cmd]) => {
    const btn = document.getElementById(id);
    if (btn) {
        btn.onclick = () => {
            if (id === 'btn-connect') {
                if (telemetryWS && telemetryWS.readyState === WebSocket.OPEN) {
                    telemetryWS.send(JSON.stringify({ type: 'connect' }));
                }
            } else if (id === 'btn-cam-start') {
                runCommand(cmd);
                setTimeout(() => {
                    const feed = document.getElementById('camera-feed') as HTMLImageElement;
                    const container = document.getElementById('camera-container');
                    if (feed && container) {
                        feed.src = `http://${window.location.hostname}:8080/stream`;
                        container.style.display = 'block';
                    }
                }, 3000);
            } else if (id === 'btn-cam-stop') {
                runCommand(cmd);
                const container = document.getElementById('camera-container');
                if (container) container.style.display = 'none';
            } else {
                runCommand(cmd);
            }
        };
    }
});

const toggleMockBtn = document.getElementById('btn-toggle-mock');
if (toggleMockBtn) {
    toggleMockBtn.onclick = () => {
        if (telemetryWS && telemetryWS.readyState === WebSocket.OPEN) {
            telemetryWS.send(JSON.stringify({ type: 'toggle_mock' }));
        }
    };
}

// --- Parameter Form ---
const paramForm = document.getElementById('parameter_form') as HTMLFormElement;
if (paramForm) {
    paramForm.onsubmit = (e) => {
        e.preventDefault();
        const nameInput = document.getElementById('param-name') as HTMLInputElement;
        const valInput = document.getElementById('param-value') as HTMLInputElement;

        if (nameInput.value && valInput.value) {
            const cmd = `set ${nameInput.value} ${valInput.value}`;
            runCommand(cmd);
            nameInput.value = '';
            valInput.value = '';
        }
    };
}

async function runCommand(cmd: string) {
    term.write(`\r\n\x1b[33m--- Executing ${cmd} ---\x1b[0m\r\n`);
    if (telemetryWS && telemetryWS.readyState === WebSocket.OPEN) {
        telemetryWS.send(JSON.stringify({ type: 'command', command: cmd }));
    } else {
        term.write(`\r\n\x1b[31mError: WebSocket not connected\x1b[0m\r\n`);
    }
}

// --- Animation ---
function animate() {
    requestAnimationFrame(animate);
    renderer.render(scene, camera);
}

window.addEventListener('resize', () => {
    if (mainView) {
        camera.aspect = mainView.clientWidth / mainView.clientHeight;
        camera.updateProjectionMatrix();
        renderer.setSize(mainView.clientWidth, mainView.clientHeight);
    }
    fitAddon.fit();
    mavlinkFitAddon.fit();
});

animate();
term.write('\x1b[1;32mRover UI Initialized. Ready for commands.\x1b[0m\r\n');
