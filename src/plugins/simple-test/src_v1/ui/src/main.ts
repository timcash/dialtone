import * as THREE from 'three';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { setupApp } from '@ui/ui';
import { VisualizationControl } from '@ui/types';

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  private renderer: THREE.WebGLRenderer;
  private cube: THREE.Mesh;
  private material: THREE.MeshStandardMaterial;
  private frameId = 0;
  private visible = false;
  private term: Terminal;
  private fitAddon: FitAddon;

  constructor(private container: HTMLElement, canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x05070a, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const geometry = new THREE.BoxGeometry(1.5, 1.5, 1.5);
    this.material = new THREE.MeshStandardMaterial({ color: 0x475261, roughness: 0.4, metalness: 0.6 });
    this.cube = new THREE.Mesh(geometry, this.material);
    this.scene.add(this.cube);

    const gridHelper = new THREE.GridHelper(40, 40, 0x444444, 0x111111);
    gridHelper.position.y = -2;
    this.scene.add(gridHelper);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.45));
    const keyLight = new THREE.DirectionalLight(0xffffff, 0.95);
    keyLight.position.set(5, 10, 8);
    this.scene.add(keyLight);

    this.camera.position.set(0, 5, 10);
    this.camera.lookAt(0, 0, 0);

    // Terminal
    this.term = new Terminal({
      allowTransparency: true,
      convertEol: true,
      disableStdin: true,
      theme: { background: 'rgba(0,0,0,0)', foreground: '#a7adb7' },
      fontSize: 12,
    });
    this.fitAddon = new FitAddon();
    this.term.loadAddon(this.fitAddon);
    const termEl = container.querySelector('.simple-chatlog-xterm') as HTMLElement;
    if (termEl) {
      this.term.open(termEl);
      setTimeout(() => { try { this.fitAddon.fit(); } catch(e) {} }, 100);
    }
    this.term.writeln('\x1b[32m[SimpleTest] System Online\x1b[0m');

    // Interaction
    const interactBtn = container.querySelector('[aria-label="Simple Interaction Button"]');
    interactBtn?.addEventListener('click', () => {
      this.material.color.set(0x66fcf1);
      container.setAttribute('data-interacted', 'true');
      this.term.writeln('\x1b[97m[USER] Interaction Triggered\x1b[0m');
      const statusEl = document.getElementById('simple-status');
      if (statusEl) statusEl.textContent = 'Active';
    });

    window.addEventListener('resize', this.resize);
    this.animate();
  }

  private resize = () => {
    const rect = this.container.getBoundingClientRect();
    this.camera.aspect = rect.width / rect.height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(rect.width, rect.height, false);
    try { this.fitAddon.fit(); } catch(e) {}
  };

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;
    this.cube.rotation.y += 0.005;
    this.renderer.render(this.scene, this.camera);
  };

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) this.resize();
  }

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    this.renderer.dispose();
    this.term.dispose();
  }
}

try {
  const { sections } = setupApp({ title: 'simple-test', debug: true });

  sections.register('simple-three-stage', {
    containerId: 'simple-three-stage',
    load: async () => {
      const container = document.getElementById('simple-three-stage')!;
      const canvas = container.querySelector('canvas') as HTMLCanvasElement;
      return new ThreeControl(container, canvas);
    },
    header: { visible: true, title: 'Simple Stage' },
    overlays: {
      primaryKind: 'stage',
      primary: 'canvas',
      form: 'form',
      legend: '.simple-legend',
      chatlog: '.simple-chatlog',
      statusBar: '.simple-status-bar'
    }
  });

  void sections.navigateTo('simple-three-stage');

  setTimeout(() => {
    const el = document.getElementById('simple-three-stage');
    if (el) el.setAttribute('data-ready', 'true');
    const header = document.querySelector('[aria-label="App Header"]');
    if (header) header.setAttribute('data-boot', 'true');
  }, 500);

} catch (err) {
  console.error('[SimpleTest] Setup failed:', err);
}
