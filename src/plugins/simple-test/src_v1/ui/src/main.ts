import * as THREE from 'three';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { setupApp } from '../../../../../plugins/ui/src_v1/ui/ui';
import { VisualizationControl } from '../../../../../plugins/ui/src_v1/ui/types';

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
  private renderer: THREE.WebGLRenderer;
  private cube: THREE.Mesh;
  private material: THREE.MeshStandardMaterial;
  private frameId = 0;
  private visible = false;
  private term: Terminal;
  private fitAddon: FitAddon;

  constructor(private container: HTMLElement, canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.setSize(window.innerWidth, window.innerHeight);

    const geometry = new THREE.BoxGeometry(1, 1, 1);
    this.material = new THREE.MeshStandardMaterial({ color: 0x00ff00 });
    this.cube = new THREE.Mesh(geometry, this.material);
    this.scene.add(this.cube);

    const light = new THREE.DirectionalLight(0xffffff, 1);
    light.position.set(1, 1, 2);
    this.scene.add(light);
    this.scene.add(new THREE.AmbientLight(0x404040));

    this.camera.position.z = 3;

    // Terminal Setup
    this.term = new Terminal({
      allowTransparency: true,
      theme: { background: 'transparent' },
      fontSize: 12,
    });
    this.fitAddon = new FitAddon();
    this.term.loadAddon(this.fitAddon);
    const termEl = container.querySelector('.simple-chatlog-xterm') as HTMLElement;
    if (termEl) {
      this.term.open(termEl);
      try {
        this.fitAddon.fit();
      } catch (e) {
        console.warn('[SimpleTest] fit() failed during init, will retry on resize/visible');
      }
    }
    this.term.writeln('\x1b[32m[SimpleTest] Terminal Initialized\x1b[0m');

    // Interaction
    const interactBtn = container.querySelector('[aria-label="Simple Interaction Button"]');
    interactBtn?.addEventListener('click', () => {
      this.material.color.set(0xff0000);
      container.setAttribute('data-interacted', 'true');
      this.term.writeln('\x1b[31m[SimpleTest] Button Clicked!\x1b[0m');
      
      const statusEl = document.getElementById('simple-status');
      if (statusEl) statusEl.textContent = 'Interacted';
    });

    window.addEventListener('resize', this.resize);
    this.animate();
  }

  private resize = () => {
    const width = window.innerWidth;
    const height = window.innerHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height);
    try {
      this.fitAddon.fit();
    } catch (e) {}
  };

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;
    this.cube.rotation.x += 0.01;
    this.cube.rotation.y += 0.01;
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
      try {
        console.log('[SimpleTest] load() starting...');
        const container = document.getElementById('simple-three-stage');
        if (!container) {
          throw new Error('Container #simple-three-stage NOT FOUND in DOM');
        }
        const canvas = container.querySelector('canvas');
        if (!canvas) {
          throw new Error('Canvas NOT FOUND in #simple-three-stage');
        }
        const ctl = new ThreeControl(container, canvas as HTMLCanvasElement);
        console.log('[SimpleTest] load() success');
        return ctl;
      } catch (err: any) {
        console.error('[SimpleTest] load() error:', err.message || String(err));
        throw err;
      }
    },
    header: { visible: true, title: 'Simple Stage' },
    overlays: {
      primaryKind: 'stage',
      primary: '.overlay-primary',
      form: '.overlay-form',
      legend: '.overlay-legend',
      chatlog: '.overlay-chatlog',
      statusBar: '.overlay-status-bar'
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
