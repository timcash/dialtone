import * as THREE from 'three';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

type HoveredCubeID = 'cube_left' | 'cube_right' | '';

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  private renderer: THREE.WebGLRenderer;
  private raycaster = new THREE.Raycaster();
  private pointer = new THREE.Vector2(2, 2);
  private cubes: Array<{ id: Exclude<HoveredCubeID, ''>; mesh: THREE.Mesh; material: THREE.MeshStandardMaterial }> = [];
  private hoveredCubeId: HoveredCubeID = '';
  private keyLight: THREE.DirectionalLight;
  private visible = false;
  private frameId = 0;
  private time = 0;
  private spinSpeed = 0.4;
  private wheelCount = 0;

  constructor(private container: HTMLElement, private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.camera.position.set(0, 0, 3);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.4));
    this.keyLight = new THREE.DirectionalLight(0xffffff, 0.9);
    this.keyLight.position.set(2, 2, 2);
    this.scene.add(this.keyLight);

    const cubeGeometry = new THREE.BoxGeometry(0.9, 0.9, 0.9);
    this.addCube('cube_left', cubeGeometry, -0.8);
    this.addCube('cube_right', cubeGeometry, 0.8);

    this.resize();
    window.addEventListener('resize', this.resize);
    this.canvas.addEventListener('wheel', this.onWheel);
    this.canvas.addEventListener('mousemove', this.onMouseMove);
    this.canvas.addEventListener('mouseleave', this.onMouseLeave);
    this.canvas.setAttribute('data-hovered-cube', '');
    this.attachDebugBridge();
    this.animate();
  }

  private addCube(id: Exclude<HoveredCubeID, ''>, geometry: THREE.BoxGeometry, x: number) {
    const material = new THREE.MeshStandardMaterial({
      color: 0x444444,
      emissive: 0x000000,
      emissiveIntensity: 1.0,
      roughness: 0.45,
      metalness: 0.2,
    });
    const mesh = new THREE.Mesh(geometry, material);
    mesh.position.set(x, 0, 0);
    mesh.userData = { id };
    this.scene.add(mesh);
    this.cubes.push({ id, mesh, material });
  }

  private setHoveredCube(id: HoveredCubeID) {
    if (this.hoveredCubeId === id) {
      return;
    }
    this.hoveredCubeId = id;
    this.canvas.setAttribute('data-hovered-cube', id);
    for (const cube of this.cubes) {
      if (cube.id === id) {
        cube.material.emissive.setHex(0x1f6dff);
      } else {
        cube.material.emissive.setHex(0x000000);
      }
    }
    console.log(`[Three #three] hover cube: ${id || 'none'}`);
  }

  private onWheel = () => {
    this.wheelCount += 1;
    this.canvas.setAttribute('data-wheel-count', String(this.wheelCount));
  };

  private onMouseMove = (event: MouseEvent) => {
    const rect = this.canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    if (x < 0 || y < 0 || x > rect.width || y > rect.height) {
      this.setHoveredCube('');
      return;
    }
    this.pointer.x = (x / rect.width) * 2 - 1;
    this.pointer.y = -(y / rect.height) * 2 + 1;
    this.raycaster.setFromCamera(this.pointer, this.camera);
    const intersects = this.raycaster.intersectObjects(
      this.cubes.map((c) => c.mesh),
      false
    );
    const id = (intersects[0]?.object.userData?.id ?? '') as HoveredCubeID;
    this.setHoveredCube(id);
  };

  private onMouseLeave = () => {
    this.setHoveredCube('');
  };

  private getProjectedPoint = (id: Exclude<HoveredCubeID, ''>): { ok: boolean; x: number; y: number } => {
    const cube = this.cubes.find((c) => c.id === id);
    if (!cube) return { ok: false, x: 0, y: 0 };
    const rect = this.canvas.getBoundingClientRect();
    this.scene.updateMatrixWorld(true);
    this.camera.updateMatrixWorld(true);
    const projected = cube.mesh.position.clone().project(this.camera);
    const x = Math.round((projected.x * 0.5 + 0.5) * rect.width + rect.left);
    const y = Math.round((-projected.y * 0.5 + 0.5) * rect.height + rect.top);
    return { ok: true, x, y };
  };

  private attachDebugBridge() {
    (window as Window & {
      templateThreeDebug?: { getProjectedPoint: (id: Exclude<HoveredCubeID, ''>) => { ok: boolean; x: number; y: number } };
    }).templateThreeDebug = {
      getProjectedPoint: this.getProjectedPoint,
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

    this.time += 0.016;
    for (let i = 0; i < this.cubes.length; i += 1) {
      const mesh = this.cubes[i].mesh;
      const dir = i === 0 ? 1 : -1;
      mesh.rotation.x = this.time * this.spinSpeed * 0.8;
      mesh.rotation.y = this.time * this.spinSpeed * dir;
    }

    this.renderer.render(this.scene, this.camera);
  };

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    this.canvas.removeEventListener('wheel', this.onWheel);
    this.canvas.removeEventListener('mousemove', this.onMouseMove);
    this.canvas.removeEventListener('mouseleave', this.onMouseLeave);
    const win = window as Window & { templateThreeDebug?: unknown };
    if (win.templateThreeDebug) {
      delete win.templateThreeDebug;
    }
    this.renderer.dispose();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  canvas.setAttribute('data-wheel-count', '0');
  return new ThreeControl(container, canvas);
}
