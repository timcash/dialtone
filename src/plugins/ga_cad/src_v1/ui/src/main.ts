import './style.css';

declare const APP_VERSION: string;

type Point = {
  x: number;
  y: number;
};

type Motor = {
  angle: number;
  tx: number;
  ty: number;
};

const app = document.getElementById('app');
if (!app) throw new Error('app root not found');

app.innerHTML = `
  <div class="layout">
    <div class="panel">
      <h1>PGA Kinematic Hierarchy</h1>
      <p>This scene-graph demo mirrors the original PGA example, but runs as a self-contained Dialtone plugin.</p>
      <p class="gear-text"><strong>Parent (Gear):</strong><br />Rotates around the global origin.<br /><span class="math">M_gear = Rotor(t)</span></p>
      <p class="arm-text"><strong>Child (Arm Linkage):</strong><br />Attached to a pin on the gear, with its own local oscillation.<br /><span class="math">M_local = Translator(x, y) * Rotor(sin(t))</span></p>
      <p><strong>Combined World Transform:</strong><br /><span class="math">M_world = M_parent * M_local</span></p>
      <p><strong>Application:</strong><br /><span class="math">P_new = M × P × M~</span></p>
      <p class="footer">Drag to pan. Scroll to zoom.</p>
      <div class="version">ga_cad/src_v1 · ui ${APP_VERSION}</div>
    </div>
    <div class="canvas-wrap"><canvas aria-label="GA CAD Canvas"></canvas></div>
  </div>
`;

const rawCanvas = app.querySelector('canvas');
if (!(rawCanvas instanceof HTMLCanvasElement)) throw new Error('canvas not found');
const canvas: HTMLCanvasElement = rawCanvas;
const rawCtx = canvas.getContext('2d');
if (!rawCtx) throw new Error('2d context not available');
const ctx: CanvasRenderingContext2D = rawCtx;

const gearRadius = 2;
const baseGear = makeBaseGear(12, 1.7, gearRadius);
const baseArm = makeBaseArm(4, 0.2);

let viewportScale = 92;
let offsetX = 0;
let offsetY = 0;
let dragging = false;
let lastClientX = 0;
let lastClientY = 0;

function point(x: number, y: number): Point {
  return { x, y };
}

function rotor(angle: number): Motor {
  return { angle, tx: 0, ty: 0 };
}

function translator(x: number, y: number): Motor {
  return { angle: 0, tx: x, ty: y };
}

function multiply(parent: Motor, local: Motor): Motor {
  const cos = Math.cos(parent.angle);
  const sin = Math.sin(parent.angle);
  return {
    angle: parent.angle + local.angle,
    tx: parent.tx + cos * local.tx - sin * local.ty,
    ty: parent.ty + sin * local.tx + cos * local.ty,
  };
}

function applyMotor(motor: Motor, p: Point): Point {
  const cos = Math.cos(motor.angle);
  const sin = Math.sin(motor.angle);
  return point(
    cos * p.x - sin * p.y + motor.tx,
    sin * p.x + cos * p.y + motor.ty,
  );
}

function makeBaseGear(teeth: number, innerRadius: number, outerRadius: number): Point[] {
  const delta = (2 * Math.PI) / teeth;
  const profile = [
    point(innerRadius * Math.cos(-0.5 * delta), innerRadius * Math.sin(-0.5 * delta)),
    point(innerRadius * Math.cos(-0.25 * delta), innerRadius * Math.sin(-0.25 * delta)),
    point(outerRadius * Math.cos(-0.15 * delta), outerRadius * Math.sin(-0.15 * delta)),
    point(outerRadius * Math.cos(0.15 * delta), outerRadius * Math.sin(0.15 * delta)),
    point(innerRadius * Math.cos(0.25 * delta), innerRadius * Math.sin(0.25 * delta)),
    point(innerRadius * Math.cos(0.5 * delta), innerRadius * Math.sin(0.5 * delta)),
  ];
  const points: Point[] = [];
  for (let i = 0; i < teeth; i += 1) {
    const transform = rotor(i * delta);
    profile.forEach((p) => points.push(applyMotor(transform, p)));
  }
  return points;
}

function makeBaseArm(length: number, halfWidth: number): Point[] {
  return [
    point(-0.3, halfWidth),
    point(length, halfWidth),
    point(length, -halfWidth),
    point(-0.3, -halfWidth),
  ];
}

function resize(): void {
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  const width = window.innerWidth;
  const height = window.innerHeight;
  canvas.width = Math.round(width * dpr);
  canvas.height = Math.round(height * dpr);
  canvas.style.width = `${width}px`;
  canvas.style.height = `${height}px`;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
}

function worldToScreen(p: Point): Point {
  return point(
    window.innerWidth * 0.5 + offsetX + p.x * viewportScale,
    window.innerHeight * 0.5 + offsetY - p.y * viewportScale,
  );
}

function screenToWorld(x: number, y: number): Point {
  return point(
    (x - window.innerWidth * 0.5 - offsetX) / viewportScale,
    -(y - window.innerHeight * 0.5 - offsetY) / viewportScale,
  );
}

function drawGrid(): void {
  const width = window.innerWidth;
  const height = window.innerHeight;
  const worldLeft = screenToWorld(0, 0).x;
  const worldRight = screenToWorld(width, 0).x;
  const worldTop = screenToWorld(0, 0).y;
  const worldBottom = screenToWorld(0, height).y;

  for (let i = Math.floor(worldLeft); i <= Math.ceil(worldRight); i += 1) {
    const x = worldToScreen(point(i, 0)).x;
    ctx.strokeStyle = i === 0 ? 'rgba(255,255,255,0.28)' : i % 5 === 0 ? 'rgba(85,126,150,0.32)' : 'rgba(85,126,150,0.14)';
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, height);
    ctx.stroke();
  }
  for (let i = Math.floor(worldBottom); i <= Math.ceil(worldTop); i += 1) {
    const y = worldToScreen(point(0, i)).y;
    ctx.strokeStyle = i === 0 ? 'rgba(255,255,255,0.28)' : i % 5 === 0 ? 'rgba(85,126,150,0.32)' : 'rgba(85,126,150,0.14)';
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(width, y);
    ctx.stroke();
  }
}

function fillPolygon(points: Point[], fill: string, stroke: string): void {
  if (!points.length) return;
  const first = worldToScreen(points[0]);
  ctx.beginPath();
  ctx.moveTo(first.x, first.y);
  for (let i = 1; i < points.length; i += 1) {
    const p = worldToScreen(points[i]);
    ctx.lineTo(p.x, p.y);
  }
  ctx.closePath();
  ctx.fillStyle = fill;
  ctx.fill();
  ctx.strokeStyle = stroke;
  ctx.lineWidth = 2;
  ctx.stroke();
}

function drawPin(p: Point, label: string): void {
  const s = worldToScreen(p);
  ctx.fillStyle = '#ffffff';
  ctx.beginPath();
  ctx.arc(s.x, s.y, 4, 0, Math.PI * 2);
  ctx.fill();
  ctx.font = '12px IBM Plex Mono, monospace';
  ctx.fillStyle = 'rgba(255,255,255,0.88)';
  ctx.fillText(label, s.x + 10, s.y - 10);
}

function render(now: number): void {
  const t = now / 1000;
  ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);
  drawGrid();

  const gearAngle = t * 0.8;
  const mGear = rotor(gearAngle);
  const armLocalOscillation = Math.sin(t * 3) * 0.8;
  const mLocalOffset = translator(gearRadius - 0.2, 0);
  const mLocalRotation = rotor(armLocalOscillation);
  const mLocalArm = multiply(mLocalOffset, mLocalRotation);
  const mWorldArm = multiply(mGear, mLocalArm);

  const gearDrawn = baseGear.map((p) => applyMotor(mGear, p));
  const armDrawn = baseArm.map((p) => applyMotor(mWorldArm, p));
  const centerPin = applyMotor(mGear, point(0, 0));
  const connectionPin = applyMotor(mWorldArm, point(0, 0));

  fillPolygon(gearDrawn, 'rgba(31, 149, 255, 0.18)', '#1f95ff');
  fillPolygon(armDrawn, 'rgba(255, 90, 54, 0.2)', '#ff5a36');
  drawPin(centerPin, 'Origin');
  drawPin(connectionPin, 'Pin Joint');

  requestAnimationFrame(render);
}

canvas.addEventListener('mousedown', (event) => {
  dragging = true;
  lastClientX = event.clientX;
  lastClientY = event.clientY;
  canvas.classList.add('dragging');
});

window.addEventListener('mouseup', () => {
  dragging = false;
  canvas.classList.remove('dragging');
});

window.addEventListener('mousemove', (event) => {
  if (!dragging) return;
  offsetX += event.clientX - lastClientX;
  offsetY += event.clientY - lastClientY;
  lastClientX = event.clientX;
  lastClientY = event.clientY;
});

canvas.addEventListener(
  'wheel',
  (event) => {
    event.preventDefault();
    const factor = event.deltaY > 0 ? 0.92 : 1.08;
    viewportScale = Math.min(220, Math.max(28, viewportScale * factor));
  },
  { passive: false },
);

window.addEventListener('resize', resize);

resize();
requestAnimationFrame(render);
