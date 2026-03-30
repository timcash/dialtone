import * as THREE from 'three';
import { STLLoader } from 'three/examples/jsm/loaders/STLLoader.js';
import type { VisualizationControl } from '@ui/types';

type GearParams = {
  outer_diameter: number;
  inner_diameter: number;
  thickness: number;
  tooth_height: number;
  tooth_width: number;
  num_teeth: number;
  num_mounting_holes: number;
  mounting_hole_diameter: number;
};

type CadMode = 'gear' | 'render';
type CameraView = 'front' | 'top' | 'side' | 'isometric';

type ButtonSpec = {
  label: string;
  action: () => void;
};

const DEFAULT_PARAMS: GearParams = {
  outer_diameter: 80,
  inner_diameter: 20,
  thickness: 8,
  tooth_height: 6,
  tooth_width: 4,
  num_teeth: 20,
  num_mounting_holes: 4,
  mounting_hole_diameter: 6,
};

class CadStage implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(42, 1, 0.1, 2000);
  private renderer: THREE.WebGLRenderer;
  private root = new THREE.Group();
  private floor!: THREE.Mesh;
  private grid!: THREE.GridHelper;
  private frameId = 0;
  private visible = true;
  private mesh: THREE.Mesh | null = null;
  private wireframe: THREE.LineSegments | null = null;
  private loader = new STLLoader();
  private abortController: AbortController | null = null;
  private params: GearParams = { ...DEFAULT_PARAMS };
  private rotationVelocity = 0.005;
  private spinEnabled = true;
  private wireframeVisible = true;
  private mode: CadMode = 'gear';
  private generationSeq = 0;
  private regenerationInFlight = false;
  private pendingRegenerationStatus: string | null = null;
  private pendingRegenerationKey: string | null = null;
  private lastGeneratedKey = '';
  private stageStatus: HTMLElement;
  private stats = {
    outer: this.requireEl('cad-stat-outer'),
    inner: this.requireEl('cad-stat-inner'),
    teeth: this.requireEl('cad-stat-teeth'),
    holes: this.requireEl('cad-stat-holes'),
    status: this.requireEl('cad-stat-status'),
    mesh: this.requireEl('cad-stat-mesh'),
  };
  private form: HTMLFormElement;
  private formButtons: HTMLButtonElement[];
  private input: HTMLInputElement;
  private submitButton: HTMLButtonElement;
  private buttonClickListeners: Array<(event: MouseEvent) => void> = [];

  constructor(private container: HTMLElement, private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({
      canvas,
      antialias: true,
      alpha: true,
      powerPreference: 'high-performance',
    });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.setClearColor(0x02070d, 1);
    this.scene.fog = new THREE.Fog(0x02070d, 90, 240);

    this.camera.position.set(0, 62, 152);
    this.camera.lookAt(0, 0, 0);

    this.form = this.container.querySelector('.mode-form') as HTMLFormElement;
    this.formButtons = Array.from(this.form.querySelectorAll('button')).slice(0, 9);
    this.input = this.form.querySelector('input[aria-label="CAD Input"]') as HTMLInputElement;
    this.submitButton = this.form.querySelector('button[type="submit"]') as HTMLButtonElement;
    this.stageStatus = this.stats.status;

    this.bootstrapScene();
    this.bindControls();
    this.setControlsBusy(true);
    this.resize();
    window.addEventListener('resize', this.resize);
    void this.regenerate('Generating baseline gear...');
    this.animate();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    this.canvas.style.visibility = visible ? 'visible' : 'hidden';
  }

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    this.abortController?.abort();
    this.form.removeEventListener('submit', this.onSubmit);
    this.formButtons.forEach((button, index) => {
      button.removeEventListener('click', this.buttonClickListeners[index]);
    });
    this.disposeCurrentGeometry();
    this.renderer.dispose();
  }

  private requireEl(id: string): HTMLElement {
    const el = document.getElementById(id);
    if (!el) throw new Error(`missing element ${id}`);
    return el;
  }

  private bootstrapScene(): void {
    const ambient = new THREE.AmbientLight(0xffffff, 0.48);
    this.scene.add(ambient);

    const key = new THREE.DirectionalLight(0xd9f4ff, 1.25);
    key.position.set(80, 120, 90);
    this.scene.add(key);

    const rim = new THREE.PointLight(0x4fc3ff, 1.8, 500);
    rim.position.set(-110, 55, 90);
    this.scene.add(rim);

    this.floor = new THREE.Mesh(
      new THREE.CircleGeometry(90, 64),
      new THREE.MeshStandardMaterial({
        color: 0x06131f,
        roughness: 0.95,
        metalness: 0.08,
      }),
    );
    this.floor.rotation.x = -Math.PI / 2;
    this.floor.position.y = -20;
    this.scene.add(this.floor);

    this.grid = new THREE.GridHelper(180, 28, 0x28556c, 0x0b2531);
    this.grid.position.y = -19.8;
    this.scene.add(this.grid);

    this.root.position.y = 2;
    this.scene.add(this.root);
  }

  private buttonHandlers: Array<() => void> = [
    () => this.runCurrentModeButton(0),
    () => this.runCurrentModeButton(1),
    () => this.runCurrentModeButton(2),
    () => this.runCurrentModeButton(3),
    () => this.runCurrentModeButton(4),
    () => this.runCurrentModeButton(5),
    () => this.runCurrentModeButton(6),
    () => this.runCurrentModeButton(7),
    () => this.cycleMode(),
  ];

  private bindControls(): void {
    this.formButtons.forEach((button, index) => {
      const listener = (event: MouseEvent): void => {
        event.preventDefault();
        event.stopPropagation();
        this.buttonHandlers[index]();
      };
      this.buttonClickListeners[index] = listener;
      button.addEventListener('click', listener);
    });
    this.form.addEventListener('submit', this.onSubmit);
    this.refreshLegend();
    this.applyMode();
  }

  private onSubmit = (event: Event): void => {
    event.preventDefault();
    const raw = this.input.value.trim();
    if (!raw) return;
    const next = this.parseQuickCommand(raw);
    this.input.value = '';
    if (!next) {
      this.setStatus('Command not understood');
      this.stats.status.textContent = 'parse error';
      return;
    }
    this.params = { ...this.params, ...next };
    this.requestRegenerate(`Applying ${raw} ...`);
  };

  private parseQuickCommand(raw: string): Partial<GearParams> | null {
    const tokens = raw.split(/\s+/);
    const next: Partial<GearParams> = {};
    for (const token of tokens) {
      const [rawKey, rawValue] = token.split(':', 2);
      const key = rawKey?.trim().toLowerCase();
      const value = Number(rawValue);
      if (!key || Number.isNaN(value)) continue;
      switch (key) {
        case 'od':
        case 'outer':
          next.outer_diameter = Math.max(24, value);
          break;
        case 'id':
        case 'inner':
          next.inner_diameter = Math.max(4, value);
          break;
        case 'teeth':
          next.num_teeth = Math.max(6, Math.round(value));
          break;
        case 'holes':
          next.num_mounting_holes = Math.max(0, Math.round(value));
          break;
        case 'thickness':
          next.thickness = Math.max(2, value);
          break;
      }
    }
    return Object.keys(next).length ? next : null;
  }

  private cycleMode(): void {
    this.mode = this.mode === 'gear' ? 'render' : 'gear';
    if (this.mode === 'render') {
      this.spinEnabled = false;
      this.setCameraView('isometric');
    } else {
      this.spinEnabled = true;
    }
    this.applyMode();
    this.setStatus(`Switched to ${this.mode} mode`);
  }

  private currentModeButtons(): ButtonSpec[] {
    if (this.mode === 'render') {
      return [
        { label: this.wireframeVisible ? 'Wireframe Off' : 'Wireframe On', action: () => this.toggleWireframe() },
        { label: this.spinEnabled ? 'Spin Off' : 'Spin On', action: () => this.toggleSpin() },
        { label: 'Front', action: () => this.setCameraView('front') },
        { label: 'Top', action: () => this.setCameraView('top') },
        { label: 'Side', action: () => this.setCameraView('side') },
        { label: 'Isometric', action: () => this.setCameraView('isometric') },
        { label: 'Download', action: () => this.downloadCurrentSTL() },
        { label: 'Reset View', action: () => this.resetRenderView() },
      ];
    }
    return [
      { label: 'Scale +', action: () => this.adjustAndRegenerate({ outer_diameter: this.params.outer_diameter + 6 }, 'Scaling gear up...') },
      { label: 'Scale -', action: () => this.adjustAndRegenerate({ outer_diameter: Math.max(24, this.params.outer_diameter - 6) }, 'Scaling gear down...') },
      { label: 'Teeth +', action: () => this.adjustAndRegenerate({ num_teeth: Math.min(96, this.params.num_teeth + 2) }, 'Adding teeth...') },
      { label: 'Teeth -', action: () => this.adjustAndRegenerate({ num_teeth: Math.max(6, this.params.num_teeth - 2) }, 'Reducing teeth...') },
      { label: 'Bore +', action: () => this.adjustAndRegenerate({ inner_diameter: Math.min(this.params.outer_diameter - 8, this.params.inner_diameter + 2) }, 'Opening bore...') },
      { label: 'Bore -', action: () => this.adjustAndRegenerate({ inner_diameter: Math.max(4, this.params.inner_diameter - 2) }, 'Tightening bore...') },
      { label: 'Holes +', action: () => this.adjustAndRegenerate({ num_mounting_holes: Math.min(12, this.params.num_mounting_holes + 1) }, 'Adding mounting holes...') },
      { label: 'Download', action: () => this.downloadCurrentSTL() },
    ];
  }

  private runCurrentModeButton(index: number): void {
    const spec = this.currentModeButtons()[index];
    spec?.action();
  }

  private applyMode(): void {
    const specs = this.currentModeButtons();
    this.formButtons.slice(0, 8).forEach((button, index) => {
      button.textContent = specs[index]?.label ?? `Action ${index + 1}`;
    });
    this.formButtons[8].textContent = this.mode === 'gear' ? 'Mode: Gear' : 'Mode: Render';
    if (this.mode === 'render') {
      this.input.placeholder = 'Switch to Gear mode for param commands';
    } else {
      this.input.placeholder = 'od:92 teeth:24 holes:6';
    }
    this.setControlsBusy(this.regenerationInFlight);
  }

  private adjustAndRegenerate(update: Partial<GearParams>, status: string): void {
    this.params = { ...this.params, ...update };
    this.requestRegenerate(status);
  }

  private requestRegenerate(status: string): void {
    const requestKey = this.serializeParams();
    if (!this.regenerationInFlight && requestKey === this.lastGeneratedKey) {
      this.setStatus('Gear already current');
      return;
    }
    if (this.regenerationInFlight) {
      if (requestKey === this.pendingRegenerationKey) {
        return;
      }
      this.pendingRegenerationStatus = status;
      this.pendingRegenerationKey = requestKey;
      this.setStatus(`${status} queued...`);
      return;
    }
    this.setControlsBusy(true);
    void this.regenerate(status);
  }

  private async regenerate(status: string): Promise<void> {
    const requestKey = this.serializeParams();
    this.regenerationInFlight = true;
    this.abortController?.abort();
    this.abortController = new AbortController();
    this.setStatus(status);
    this.setModelState('rendering', 'rendering');
    this.stats.mesh.textContent = 'requesting';

    try {
      const response = await fetch('/api/cad/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(this.params),
        signal: this.abortController.signal,
      });
      if (!response.ok) {
        throw new Error(`generate failed: ${response.status}`);
      }
      const arrayBuffer = await response.arrayBuffer();
      const geometry = this.loader.parse(arrayBuffer);
      this.applyGeometry(geometry);
      this.refreshLegend();
      this.generationSeq += 1;
      this.setModelState('ready', 'ready');
      this.stats.mesh.textContent = `${Math.round(arrayBuffer.byteLength / 1024)} kb`;
      this.setStatus('Gear regenerated');
      this.lastGeneratedKey = requestKey;
      console.info(`cad-model-ready:${this.generationSeq}`);
    } catch (error) {
      if ((error as Error).name === 'AbortError') return;
      console.error('[cad/ui] regenerate failed', error);
      this.setModelState('error', 'error');
      this.stats.mesh.textContent = 'stale';
      this.setStatus('Generation failed');
    } finally {
      this.regenerationInFlight = false;
      const pending = this.pendingRegenerationStatus;
      const pendingKey = this.pendingRegenerationKey;
      this.pendingRegenerationStatus = null;
      this.pendingRegenerationKey = null;
      if (pending && pendingKey != null && pendingKey !== this.lastGeneratedKey) {
        this.requestRegenerate(pending);
        return;
      }
      this.setControlsBusy(false);
    }
  }

  private applyGeometry(geometry: THREE.BufferGeometry): void {
    geometry.center();
    geometry.computeVertexNormals();
    geometry.computeBoundingBox();
    this.disposeCurrentGeometry();

    this.mesh = new THREE.Mesh(
      geometry,
      new THREE.MeshStandardMaterial({
        color: 0x67dfff,
        emissive: 0x0c2030,
        metalness: 0.58,
        roughness: 0.3,
      }),
    );
    this.root.add(this.mesh);

    this.wireframe = new THREE.LineSegments(
      new THREE.WireframeGeometry(geometry),
      new THREE.LineBasicMaterial({
        color: 0xe1fbff,
        transparent: true,
        opacity: 0.18,
      }),
    );
    this.wireframe.visible = this.wireframeVisible;
    this.root.add(this.wireframe);

    this.refreshFloorPlane(geometry);
  }

  private disposeCurrentGeometry(): void {
    if (this.mesh) {
      this.root.remove(this.mesh);
      this.mesh.geometry.dispose();
      (this.mesh.material as THREE.Material).dispose();
      this.mesh = null;
    }
    if (this.wireframe) {
      this.root.remove(this.wireframe);
      this.wireframe.geometry.dispose();
      (this.wireframe.material as THREE.Material).dispose();
      this.wireframe = null;
    }
  }

  private refreshLegend(): void {
    this.stats.outer.textContent = `${Math.round(this.params.outer_diameter)} mm`;
    this.stats.inner.textContent = `${Math.round(this.params.inner_diameter)} mm`;
    this.stats.teeth.textContent = `${this.params.num_teeth}`;
    this.stats.holes.textContent = `${this.params.num_mounting_holes}`;
  }

  private downloadCurrentSTL(): void {
    const query = new URLSearchParams(
      Object.entries(this.params).map(([key, value]) => [key, String(value)]),
    );
    window.open(`/api/cad/download?${query.toString()}`, '_blank', 'noopener');
    this.setStatus('Downloading STL...');
  }

  private setStatus(text: string): void {
    this.stageStatus.textContent = text;
  }

  private setModelState(state: string, text: string): void {
    this.canvas.setAttribute('data-model-state', state);
    this.stageStatus.setAttribute('data-state', state);
    this.stageStatus.textContent = text;
    this.canvas.setAttribute('data-generation', String(this.generationSeq));
    this.stageStatus.setAttribute('data-generation', String(this.generationSeq));
  }

  private setControlsBusy(busy: boolean): void {
    this.form.setAttribute('data-busy', busy ? 'true' : 'false');
    this.form.setAttribute('data-generation', String(this.generationSeq));
    this.formButtons.forEach((button) => {
      button.disabled = busy;
    });
    const renderMode = this.mode === 'render';
    this.input.disabled = busy || renderMode;
    this.submitButton.disabled = busy || renderMode;
  }

  private serializeParams(): string {
    return JSON.stringify(this.params);
  }

  private refreshFloorPlane(geometry: THREE.BufferGeometry): void {
    const bounds = geometry.boundingBox;
    if (!bounds) return;
    const modelMinY = bounds.min.y + this.root.position.y;
    const floorY = modelMinY - 3;
    this.floor.position.y = floorY;
    this.grid.position.y = floorY + 0.2;
  }

  private toggleWireframe(): void {
    this.wireframeVisible = !this.wireframeVisible;
    if (this.wireframe) {
      this.wireframe.visible = this.wireframeVisible;
    }
    this.setStatus(this.wireframeVisible ? 'Wireframe enabled' : 'Wireframe hidden');
    this.applyMode();
  }

  private toggleSpin(): void {
    this.spinEnabled = !this.spinEnabled;
    if (!this.spinEnabled) {
      this.root.rotation.set(0, 0, 0);
    }
    this.setStatus(this.spinEnabled ? 'Spin resumed' : 'Spin paused');
    this.applyMode();
  }

  private resetRenderView(): void {
    this.spinEnabled = false;
    this.setCameraView('isometric');
    this.setStatus('Render view reset');
    this.applyMode();
  }

  private setCameraView(view: CameraView): void {
    switch (view) {
      case 'front':
        this.camera.position.set(0, 0, 170);
        this.camera.up.set(0, 1, 0);
        break;
      case 'top':
        this.camera.position.set(0, 170, 0.01);
        this.camera.up.set(0, 0, -1);
        break;
      case 'side':
        this.camera.position.set(170, 0, 0);
        this.camera.up.set(0, 1, 0);
        break;
      default:
        this.camera.position.set(0, 62, 152);
        this.camera.up.set(0, 1, 0);
        break;
    }
    this.camera.lookAt(0, 0, 0);
    this.camera.updateProjectionMatrix();
    this.root.rotation.set(0, 0, 0);
    this.setStatus(`Camera: ${view}`);
  }

  private animate = (): void => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;
    if (this.spinEnabled) {
      this.root.rotation.z += this.rotationVelocity;
      this.root.rotation.x = Math.cos(performance.now() * 0.0005) * 0.15;
      this.root.rotation.y = Math.sin(performance.now() * 0.00035) * 0.22;
    }
    this.renderer.render(this.scene, this.camera);
  };

  private resize = (): void => {
    const { clientWidth, clientHeight } = this.container;
    this.renderer.setSize(clientWidth, clientHeight);
    this.camera.aspect = clientWidth / clientHeight;
    this.camera.updateProjectionMatrix();
  };
}

export function mountCadStage(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector('.three-stage') as HTMLCanvasElement | null;
  if (!canvas) throw new Error('cad stage canvas not found');
  return new CadStage(container, canvas);
}
