import * as THREE from "three";
import { FpsCounter } from "../../util/fps";
import { GpuTimer } from "../../util/gpu_timer";
import { VisibilityMixin } from "../../util/section";
import cubeGlowVert from "../../shaders/template-cube.vert.glsl?raw";
import cubeGlowFrag from "../../shaders/template-cube.frag.glsl?raw";
import { startTyping } from "../../util/typing";

class NixVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;
  private fpsCounter = new FpsCounter("nix-viz");
  private nodes: THREE.Mesh[] = [];
  private time = 0;

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.domElement.style.width = "100%";
    this.renderer.domElement.style.height = "100%";
    this.container.appendChild(this.renderer.domElement);

    this.camera.position.set(0, 0, 5);
    this.camera.lookAt(0, 0, 0);

    const cubeGeo = new THREE.BoxGeometry(0.5, 0.5, 0.5);
    for(let i=0; i<5; i++) {
        const mat = new THREE.ShaderMaterial({
            uniforms: {
                uColor: { value: new THREE.Color(0x00ff88) },
                uGlowColor: { value: new THREE.Color(0x00aaff) },
                uLightDir: { value: new THREE.Vector3(1,1,1).normalize() },
                uTime: { value: 0 },
            },
            vertexShader: cubeGlowVert,
            fragmentShader: cubeGlowFrag,
        });
        const mesh = new THREE.Mesh(cubeGeo, mat);
        mesh.position.set((Math.random()-0.5)*4, (Math.random()-0.5)*4, (Math.random()-0.5)*2);
        this.scene.add(mesh);
        this.nodes.push(mesh);
    }

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
    VisibilityMixin.setVisible(this, visible, "nix-viz");
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.nodes.forEach((node, i) => {
        node.rotation.x += 0.01 * (i+1);
        node.rotation.y += 0.015;
        const mat = node.material as THREE.ShaderMaterial;
        mat.uniforms.uTime.value = this.time;
    });

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

export function mountNixViz(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay">
      <h2>Nix Node Explorer</h2>
      <p data-typing-subtitle></p>
    </div>
  `;
  const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
  const stopTyping = startTyping(subtitleEl, [
    "Isolated build environments.",
    "Reproducible deployments.",
    "Declarative configuration management.",
  ]);

  const viz = new NixVisualization(container);
  viz.resize();
  window.addEventListener("resize", () => viz.resize());

  return {
    dispose: () => {
      viz.dispose();
      stopTyping();
    },
    setVisible: (v: boolean) => viz.setVisible(v),
  };
}
