import * as THREE from "three";
import { FpsCounter } from "../../util/fps";
import { GpuTimer } from "../../util/gpu_timer";
import { VisibilityMixin } from "../../util/section";
import cubeGlowVert from "../../shaders/template-cube.vert.glsl?raw";
import cubeGlowFrag from "../../shaders/template-cube.frag.glsl?raw";
import { startTyping } from "../../util/typing";
import { setupDemoMenu } from "./menu";

class DemoVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;
  private fpsCounter = new FpsCounter("demo");
  private cube!: THREE.Mesh;
  private cubeMaterial!: THREE.ShaderMaterial;
  private time = 0;
  spinSpeed = 0.5;
  private lightDir = new THREE.Vector3(1, 1, 1).normalize();

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.domElement.style.width = "100%";
    this.renderer.domElement.style.height = "100%";
    this.container.appendChild(this.renderer.domElement);

    this.camera.position.set(0, 0, 3);
    this.camera.lookAt(0, 0, 0);

    const cubeGeo = new THREE.BoxGeometry(1, 1, 1);
    this.cubeMaterial = new THREE.ShaderMaterial({
      uniforms: {
        uColor: { value: new THREE.Color(0x00ff88) },
        uGlowColor: { value: new THREE.Color(0x00aaff) },
        uLightDir: { value: this.lightDir.clone() },
        uTime: { value: 0 },
      },
      vertexShader: cubeGlowVert,
      fragmentShader: cubeGlowFrag,
    });
    this.cube = new THREE.Mesh(cubeGeo, this.cubeMaterial);
    this.scene.add(this.cube);

    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);
    this.animate();
  }

  resize() {
    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "demo");
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;
    this.cube.rotation.x = this.time * this.spinSpeed;
    this.cube.rotation.y = this.time * this.spinSpeed * 0.8;

    this.cubeMaterial.uniforms.uTime.value = this.time;
    this.cubeMaterial.uniforms.uLightDir.value.copy(this.lightDir).transformDirection(this.camera.matrixWorldInverse);

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    this.fpsCounter.tick(performance.now() - cpuStart, this.gpuTimer.lastMs);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.renderer.dispose();
    this.container.removeChild(this.renderer.domElement);
  }
}

export function mountDemo(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay">
      <h2>Swarm Node V2</h2>
      <p data-typing-subtitle></p>
    </div>
  `;
  const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
  const stopTyping = startTyping(subtitleEl, [
    "Decentralized data mesh.",
    "Decentralized data mesh.",
    "Multi-writer consistency.",
    "Holepunch powered connectivity.",
  ]);

  const viz = new DemoVisualization(container);
  viz.resize();
  window.addEventListener("resize", () => viz.resize());

  return {
    dispose: () => {
      viz.dispose();
      stopTyping();
    },
    setVisible: (v: boolean) => {
      viz.setVisible(v);
      if (v) setupDemoMenu(viz);
    },
  };
}
