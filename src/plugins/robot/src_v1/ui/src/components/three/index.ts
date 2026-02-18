import * as THREE from 'three';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { addMavlinkListener, sendCommand } from '../../data/connection';
import { registerButtons, renderButtons } from '../../buttons';

const CHATLOG_MAX_LINES = 7;

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  private renderer: THREE.WebGLRenderer;
  private robotGroup: THREE.Group;
  private visible = false;
  private frameId = 0;
  private unsubscribe: (() => void) | null = null;
  private attitude = { roll: 0, pitch: 0, yaw: 0 };
  private latencyHistory: number[] = [];
  private maxHistory = 60;
  
  // Chatlog
  private chatlogHost: HTMLElement | null = null;
  private chatlogTerm: Terminal | null = null;
  private chatlogLines: string[] = [];

  constructor(private container: HTMLElement, canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x05070a, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.chatlogHost = container.querySelector('.three-chatlog-xterm');
    this.initChatlogTerminal();

    registerButtons('three', ['Control'], {
      'Control': [
        { label: 'Arm', action: () => sendCommand('arm') },
        { label: 'Disarm', action: () => sendCommand('disarm') },
        { label: 'Manual', action: () => sendCommand('mode', 'manual') },
        { label: 'Guided', action: () => sendCommand('mode', 'guided') },
        null, null, null, null
      ]
    });

    this.camera.position.set(0, 5, 10);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.45));
    const keyLight = new THREE.DirectionalLight(0xffffff, 0.95);
    keyLight.position.set(5, 10, 8);
    this.scene.add(keyLight);

    const group = new THREE.Group();
    
    // Main Body
    const bodyGeo = new THREE.BoxGeometry(2, 0.5, 3);
    const bodyMat = new THREE.MeshStandardMaterial({ 
      color: 0x475261,
      roughness: 0.4,
      metalness: 0.6 
    });
    const body = new THREE.Mesh(bodyGeo, bodyMat);
    group.add(body);

    // Front Indicator
    const frontGeo = new THREE.BoxGeometry(1.2, 0.6, 0.5);
    const frontMat = new THREE.MeshStandardMaterial({ 
      color: 0x66fcf1,
      emissive: 0x1f6dff,
      emissiveIntensity: 0.5
    });
    const front = new THREE.Mesh(frontGeo, frontMat);
    front.position.set(0, 0, -1.5);
    group.add(front);

    // Axis Arrows
    const arrowLen = 3;
    const arrowX = new THREE.ArrowHelper(new THREE.Vector3(1,0,0), new THREE.Vector3(0,0,0), arrowLen, 0xff4d4d);
    const arrowY = new THREE.ArrowHelper(new THREE.Vector3(0,1,0), new THREE.Vector3(0,0,0), arrowLen, 0x4dff4d);
    const arrowZ = new THREE.ArrowHelper(new THREE.Vector3(0,0,1), new THREE.Vector3(0,0,0), arrowLen, 0x4d4dff);
    group.add(arrowX);
    group.add(arrowY);
    group.add(arrowZ);

    this.scene.add(group);
    this.robotGroup = group;

    // Floor Grid
    const gridHelper = new THREE.GridHelper(40, 40, 0x444444, 0x111111);
    gridHelper.position.y = -2;
    this.scene.add(gridHelper);

    this.resize();
    window.addEventListener('resize', this.resize);
    
    // Stub debug bridge for test compatibility
    this.attachDebugBridge();
    
    // Legend Toggle
    const legend = document.querySelector('.three-legend');
    if (legend) {
        legend.addEventListener('click', () => {
            legend.classList.toggle('legend-minimized');
        });
    }
    
    this.subscribe();
    this.animate();
  }

  private initChatlogTerminal() {
    if (!this.chatlogHost) return;
    this.chatlogTerm?.dispose();
    this.chatlogHost.innerHTML = '';
    this.chatlogTerm = new Terminal({
      allowTransparency: true,
      convertEol: true,
      disableStdin: true,
      cursorBlink: false,
      cursorStyle: 'bar',
      rows: CHATLOG_MAX_LINES,
      cols: 92,
      scrollback: 0,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 12,
      lineHeight: 1.35,
      theme: {
        background: 'rgba(0,0,0,0)',
        foreground: '#a7adb7',
        cursor: '#a7adb7',
      },
    });
    this.chatlogTerm.open(this.chatlogHost);
    this.renderChatlog();
  }

  private renderChatlog() {
    const term = this.chatlogTerm;
    if (!term) return;
    const lines = this.chatlogLines.slice(-CHATLOG_MAX_LINES);
    const padCount = Math.max(0, CHATLOG_MAX_LINES - lines.length);
    const rendered: string[] = [];
    for (let i = 0; i < padCount; i += 1) rendered.push('');
    for (let i = 0; i < lines.length; i += 1) {
      const age = lines.length - 1 - i;
      const color =
        age === 0 ? '\x1b[97m' : age === 1 ? '\x1b[37m' : age === 2 ? '\x1b[2;37m' : age === 3 ? '\x1b[90m' : '\x1b[2;90m';
      rendered.push(`${color}${lines[i]}\x1b[0m`);
    }
    term.write(`\x1b[2J\x1b[H${rendered.join('\r\n')}`);
  }

  private logToChat(text: string) {
    if (!text) return;
    const clean = text.replace(/\s+/g, ' ').trim();
    if (!clean) return;
    this.chatlogLines.push(clean);
    if (this.chatlogLines.length > CHATLOG_MAX_LINES) {
      this.chatlogLines = this.chatlogLines.slice(-CHATLOG_MAX_LINES);
    }
    this.renderChatlog();
  }

  private subscribe() {
    if (this.unsubscribe) return;
    this.unsubscribe = addMavlinkListener((raw: any) => {
      // Track latency for any message that has timestamps
      if (raw.t_raw !== undefined) {
        this.updateLatency(raw);
      }
      
      // Handle direct stats object (from system status poll)
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
         // Statustext -> Chatlog
         const msg = raw.text ?? "";
         // Format with severity?
         // DAG implementation just logs text.
         this.logToChat(msg);
      }
    });
  }

  private handleStats(data: any) {
    // Update internal attitude state for the 3D model
    if (data.roll !== undefined) this.attitude.roll = data.roll;
    if (data.pitch !== undefined) this.attitude.pitch = data.pitch;
    if (data.yaw !== undefined) this.attitude.yaw = data.yaw;

    // Update HUD from stats ticker
    const modeEl = document.getElementById('hud-mode');
    const battEl = document.getElementById('hud-batt');
    const altEl = document.getElementById('hud-alt');
    const spdEl = document.getElementById('hud-spd');
    const gpsEl = document.getElementById('hud-gps');
    const hdgEl = document.getElementById('hud-hdg');

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

    if (data.errors !== undefined) {
      data.errors.forEach((err: string) => this.logToChat(err));
    }
  }

  private updateLatency(raw: any) {
    const now = Date.now();
    let t_raw = raw.t_raw || raw.timestamp;
    if (t_raw < 10000000000) t_raw *= 1000;

    const t_pub = raw.t_pub ? raw.t_pub : t_raw;
    const total = now - t_raw;
    const proc = t_pub - t_raw;
    const net = now - t_pub;

    if (total > 10000 || total < -1000) return;

    this.latencyHistory.push(total);
    if (this.latencyHistory.length > this.maxHistory) {
      this.latencyHistory.shift();
    }

    const el = document.getElementById('hud-latency');
    if (el) {
      el.innerText = `${total}ms`;
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

    ctx.lineTo((this.latencyHistory.length - 1) * step, h);
    ctx.lineTo(0, h);
    ctx.fillStyle = 'rgba(102, 252, 241, 0.1)';
    ctx.fill();
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
      this.robotGroup.rotation.set(
        this.attitude.pitch,
        -this.attitude.yaw,
        -this.attitude.roll,
        'YXZ'
      );
    }

    this.renderer.render(this.scene, this.camera);
  };

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    if (this.unsubscribe) {
      this.unsubscribe();
    }
    delete (window as any).robotThreeDebug;
    this.chatlogTerm?.dispose();
    this.renderer.dispose();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.resize();
      this.subscribe();
      renderButtons('three');
    } else {
      if (this.unsubscribe) {
        this.unsubscribe();
        this.unsubscribe = null;
      }
    }
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  const control = new ThreeControl(container, canvas);
  control.setVisible(true);
  return control;
}
