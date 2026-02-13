import * as THREE from 'three';
import cubeGlowVert from '../../shaders/template-cube.vert.glsl?raw';
import cubeGlowFrag from '../../shaders/template-cube.frag.glsl?raw';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  private renderer: THREE.WebGLRenderer;
  private cubeMaterial: THREE.ShaderMaterial;
  private cube: THREE.Mesh;
  private keyLight: THREE.DirectionalLight;
  private lightDir = new THREE.Vector3(1, 1, 1).normalize();
  private visible = false;
  private frameId = 0;
  private time = 0;
  private spinSpeed = 0.35;
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

    const cubeGeo = new THREE.BoxGeometry(1, 1, 1);
    this.cubeMaterial = new THREE.ShaderMaterial({
      uniforms: {
        uColor: { value: new THREE.Color(0x6688aa) },
        uGlowColor: { value: new THREE.Color(0x88aacc) },
        uLightDir: { value: this.lightDir.clone() },
        uTime: { value: 0 },
      },
      vertexShader: cubeGlowVert,
      fragmentShader: cubeGlowFrag,
    });
    this.cube = new THREE.Mesh(cubeGeo, this.cubeMaterial);
    this.scene.add(this.cube);

    this.resize();
    window.addEventListener('resize', this.resize);
    this.canvas.addEventListener('wheel', this.onWheel);
    this.animate();
  }

  private onWheel = () => {
    this.wheelCount += 1;
    this.canvas.setAttribute('data-wheel-count', String(this.wheelCount));
  };

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
    this.cube.rotation.x = this.time * this.spinSpeed;
    this.cube.rotation.y = this.time * this.spinSpeed * 0.7;

    this.lightDir.set(1, 1, 1).normalize();
    this.cubeMaterial.uniforms.uLightDir.value.copy(this.lightDir).transformDirection(this.camera.matrixWorldInverse);
    this.cubeMaterial.uniforms.uTime.value = this.time;

    this.renderer.render(this.scene, this.camera);
  };

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    this.canvas.removeEventListener('wheel', this.onWheel);
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
