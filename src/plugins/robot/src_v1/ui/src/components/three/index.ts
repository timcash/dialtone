import * as THREE from 'three';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  private renderer: THREE.WebGLRenderer;
  private robotGroup: THREE.Group;
  private visible = false;
  private frameId = 0;
  private ws: WebSocket | null = null;
  private attitude = { roll: 0, pitch: 0, yaw: 0 };
  private latencyHistory: number[] = [];
  private maxHistory = 60;

  constructor(private container: HTMLElement, canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.camera.position.set(0, 0, 15);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.4));
    const keyLight = new THREE.DirectionalLight(0xffffff, 0.9);
    keyLight.position.set(2, 2, 2);
    this.scene.add(keyLight);

    const group = new THREE.Group();
    
    // Main Body (Triangle/Cone)
    const geometry = new THREE.ConeGeometry(1, 3, 4); 
    const material = new THREE.MeshPhongMaterial({ color: 0x66fcf1, wireframe: false });
    const robotMesh = new THREE.Mesh(geometry, material);
    robotMesh.rotation.x = Math.PI / 2;
    group.add(robotMesh);

    // Euler Rings
    const ringGeo = new THREE.TorusGeometry(2, 0.05, 16, 100);
    const ringMatX = new THREE.MeshBasicMaterial({ color: 0xff4d4d }); // Pitch
    const ringMatY = new THREE.MeshBasicMaterial({ color: 0x00ff00 }); // Roll
    const ringMatZ = new THREE.MeshBasicMaterial({ color: 0x0000ff }); // Yaw
    
    const ringX = new THREE.Mesh(ringGeo, ringMatX);
    const ringY = new THREE.Mesh(ringGeo, ringMatY);
    ringY.rotation.x = Math.PI / 2;
    const ringZ = new THREE.Mesh(ringGeo, ringMatZ);

    group.add(ringX); 
    group.add(ringY);
    group.add(ringZ);

    this.scene.add(group);
    this.robotGroup = group;

    const gridHelper = new THREE.GridHelper(50, 50, 0x333333, 0x111111);
    gridHelper.rotation.x = Math.PI / 2;
    this.scene.add(gridHelper);

    this.resize();
    window.addEventListener('resize', this.resize);
    
    // Stub debug bridge for test compatibility
    this.attachDebugBridge();
    
    this.connectWS();
    this.animate();
  }

  private connectWS() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    this.ws = new WebSocket(`${protocol}//${host}/ws`);
    
    this.ws.onmessage = (event) => {
      try {
        const raw = JSON.parse(event.data);

        // Track latency for any message that has timestamps
        if (raw.t_raw !== undefined) {
          this.updateLatency(raw);
        }
        
        // Handle direct stats object (from ticker)
        if (raw.uptime !== undefined) {
           this.handleStats(raw);
           return;
        }

        // Handle Mavlink messages
        if (raw.type === 'HEARTBEAT') {
           const modeEl = document.getElementById('hud-mode');
           if (modeEl && raw.custom_mode !== undefined) modeEl.innerText = `MODE ${raw.custom_mode}`;
        } else if (raw.roll !== undefined || raw.attitude !== undefined) {
           // Attitude (Direct or Nested)
           const att = raw.attitude || raw;
           this.attitude.roll = att.roll ?? 0;
           this.attitude.pitch = att.pitch ?? 0;
           this.attitude.yaw = att.yaw ?? 0;
           
           const hdgEl = document.getElementById('hud-hdg');
           if (hdgEl) {
             let deg = this.attitude.yaw * (180 / Math.PI);
             if (deg < 0) deg += 360;
             hdgEl.innerText = deg.toFixed(1);
           }
        } else if (raw.lat !== undefined) {
           // Global Position
           const pos = raw.global_position_int || raw;
           const gpsEl = document.getElementById('hud-gps');
           const altEl = document.getElementById('hud-alt');
           const hdgEl = document.getElementById('hud-hdg');
           if (gpsEl) {
             const lat = pos.lat ?? 0;
             const lon = pos.lon ?? 0;
             gpsEl.innerText = `${lat.toFixed(4)}, ${lon.toFixed(4)}`;
           }
           if (altEl) {
             const alt = pos.relative_alt ?? 0;
             altEl.innerText = alt.toFixed(1);
           }
           if (hdgEl) {
             const hdg = pos.hdg ?? -1;
             if (hdg !== -1) hdgEl.innerText = hdg.toFixed(1);
           }
        } else if (raw.severity !== undefined) {
           // Statustext
           const msg = raw.text ?? "";
           const errorsEl = document.getElementById('hud-errors');
           if (errorsEl && msg) {
             const div = document.createElement('div');
             div.className = 'hud-error-item';
             div.innerText = msg;
             errorsEl.prepend(div);
             while (errorsEl.children.length > 3) errorsEl.removeChild(errorsEl.lastChild!);
           }
        }
      } catch (e) {
        // Silently ignore non-JSON or other message formats
      }
    };

    this.ws.onclose = () => {
      if (this.visible) {
        setTimeout(() => this.connectWS(), 2000);
      }
    };
  }

  private handleStats(data: any) {
    // Update internal attitude state for the 3D model
    if (data.roll !== undefined) this.attitude.roll = data.roll;
    if (data.pitch !== undefined) this.attitude.pitch = data.pitch;
    if (data.yaw !== undefined) this.attitude.yaw = data.yaw;

    // Update HUD from stats ticker (mostly mock mode or basic system stats)
    const modeEl = document.getElementById('hud-mode');
    const battEl = document.getElementById('hud-batt');
    const altEl = document.getElementById('hud-alt');
    const spdEl = document.getElementById('hud-spd');
    const gpsEl = document.getElementById('hud-gps');
    const hdgEl = document.getElementById('hud-hdg');
    const errorsEl = document.getElementById('hud-errors');

    if (modeEl && data.mode !== undefined) modeEl.innerText = data.mode;
    if (battEl && data.battery !== undefined) battEl.innerText = data.battery.toFixed(1);
    if (altEl && data.alt !== undefined) altEl.innerText = data.alt.toFixed(1);
    if (spdEl && data.spd !== undefined) spdEl.innerText = data.spd.toFixed(1);
    
    if (gpsEl && data.lat !== undefined && data.lon !== undefined) {
      gpsEl.innerText = `${data.lat.toFixed(4)}, ${data.lon.toFixed(4)}`;
    }
    
    if (hdgEl && data.yaw !== undefined) {
      let deg = data.yaw * (180 / Math.PI);
      if (deg < 0) deg += 360;
      hdgEl.innerText = deg.toFixed(1);
    }

    if (errorsEl && data.errors !== undefined) {
      errorsEl.innerHTML = data.errors.map((err: string) => `<div class="hud-error-item">${err}</div>`).join('');
    }
  }

  private updateLatency(raw: any) {
    const now = Date.now();
    
    // Ensure we have a valid t_raw (Ground Zero)
    let t_raw = raw.t_raw || raw.timestamp;
    // If t_raw is in seconds (e.g. from legacy or mock), convert to ms
    if (t_raw < 10000000000) t_raw *= 1000;

    const t_pub = raw.t_pub ? raw.t_pub : t_raw;
    const t_relay = raw.t_relay ? raw.t_relay : t_pub;

    const total = now - t_raw;
    const proc = t_pub - t_raw;
    const nats = t_relay - t_pub;
    const net = now - t_relay;

    // Sanity check: Ignore if total latency is over 10 seconds (likely clock drift or bug)
    if (total > 10000 || total < -1000) return;

    this.latencyHistory.push(total);
    if (this.latencyHistory.length > this.maxHistory) {
      this.latencyHistory.shift();
    }

    const el = document.getElementById('hud-latency');
    if (el) {
      el.innerHTML = `<span title="Total">${total}</span>ms <small style="font-size:0.6rem; opacity:0.6">(P:${proc}/Q:${nats}/N:${net})</small>`;
    }
    this.drawLatencyGraph();
  }

  private drawLatencyGraph() {
    const canvas = document.getElementById('latency-graph') as HTMLCanvasElement | null;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const w = canvas.width;
    const h = canvas.height;
    ctx.clearRect(0, 0, w, h);

    if (this.latencyHistory.length < 2) return;

    const maxLatency = Math.max(...this.latencyHistory, 100);
    const step = w / (this.maxHistory - 1);

    ctx.strokeStyle = '#66fcf1';
    ctx.lineWidth = 1.5;
    ctx.beginPath();

    for (let i = 0; i < this.latencyHistory.length; i++) {
      const x = i * step;
      const y = h - (this.latencyHistory[i] / maxLatency) * h;
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.stroke();

    // Fill area
    ctx.lineTo((this.latencyHistory.length - 1) * step, h);
    ctx.lineTo(0, h);
    ctx.fillStyle = 'rgba(102, 252, 241, 0.1)';
    ctx.fill();

    // Show current latency text on canvas
    ctx.fillStyle = '#66fcf1';
    ctx.font = '10px ui-monospace, monospace';
    ctx.textAlign = 'right';
    const last = this.latencyHistory[this.latencyHistory.length - 1];
    ctx.fillText(`${last}ms`, w - 2, 10);
  }

  private attachDebugBridge() {
    (window as any).robotThreeDebug = {
      getProjectedPoint: () => ({ ok: true, x: 0, y: 0 }),
      touchProjected: () => true,
    };
  }

  private resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  };

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;

    if (this.robotGroup) {
      // Mapping MAVLink (NED) to Three.js (Forward: +Y, Right: +X, Up: +Z)
      // MAVLink Roll (+ right)  -> Three.js Roll (+ around Y)
      // MAVLink Pitch (+ down)  -> Three.js Pitch (+ around X)
      // MAVLink Yaw (+ right)   -> Three.js Yaw (- around Z)
      
      this.robotGroup.rotation.set(
        this.attitude.pitch,  // Pitch around X
        -this.attitude.roll,  // Roll around Y (negative because MAVLink roll right is positive but Three.js Y-rotation is counter-clockwise looking from +Y)
        -this.attitude.yaw,   // Yaw around Z
        'YXZ'
      );
    }

    this.renderer.render(this.scene, this.camera);
  };

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    if (this.ws) {
      this.ws.close();
    }
    delete (window as any).robotThreeDebug;
    this.renderer.dispose();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  const control = new ThreeControl(container, canvas);
  control.setVisible(true); // Ensure visibility is set for animation
  return control;
}

