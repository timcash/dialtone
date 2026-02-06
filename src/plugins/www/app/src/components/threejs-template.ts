import * as THREE from "three";
import { FpsCounter } from "./fps";
import { GpuTimer } from "./gpu_timer";
import { VisibilityMixin } from "./section";
import cubeGlowVert from "../shaders/template-cube.vert.glsl?raw";
import cubeGlowFrag from "../shaders/template-cube.frag.glsl?raw";
import { startTyping } from "./typing";

/**
 * Simplest working section: one cube, camera facing it, key light + soft glow shader.
 * Use this as the starting point for new Three.js sections.
 */

class TemplateVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  private fpsCounter = new FpsCounter("threejs-template");
  private cube!: THREE.Mesh;
  private cubeMaterial!: THREE.ShaderMaterial;
  private keyLight!: THREE.DirectionalLight;
  private time = 0;
  spinSpeed = 0.35;
  private lightDir = new THREE.Vector3(1, 1, 1).normalize();
  frameCount = 0;

  constructor(container: HTMLElement) {
    this.container = container;

    this.renderer.setClearColor(0x111111, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const canvas = this.renderer.domElement;
    canvas.style.position = "absolute";
    canvas.style.top = "0";
    canvas.style.left = "0";
    canvas.style.width = "100%";
    canvas.style.height = "100%";

    const existingCanvas = container.querySelector("canvas");
    if (existingCanvas) {
      this.renderer.domElement = existingCanvas as HTMLCanvasElement;
    } else {
      this.container.appendChild(canvas);
    }

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
    this.animate();

    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);

    if (typeof ResizeObserver !== "undefined") {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    } else {
      window.addEventListener("resize", this.resize);
    }
  }

  resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    window.removeEventListener("resize", this.resize);
    this.renderer.dispose();
    if (this.container.contains(this.renderer.domElement)) {
      this.container.removeChild(this.renderer.domElement);
    }
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "threejs-template");
    if (!visible) this.fpsCounter.clear();
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;
    this.cube.rotation.x = this.time * this.spinSpeed;
    this.cube.rotation.y = this.time * this.spinSpeed * 0.7;

    this.lightDir.set(1, 1, 1).normalize();
    this.cubeMaterial.uniforms.uLightDir.value.copy(this.lightDir).transformDirection(this.camera.matrixWorldInverse);
    this.cubeMaterial.uniforms.uTime.value = this.time;

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}

export function mountThreeJsTemplate(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Template section: simplest working section">
      <h2>Start here</h2>
      <p data-typing-subtitle></p>
    </div>
    <div id="threejs-template-config-panel" class="earth-config-panel" hidden></div>
  `;

  const controls = document.querySelector(".top-right-controls");
  const toggle = document.createElement("button");
  toggle.id = "threejs-template-config-toggle";
  toggle.className = "earth-config-toggle";
  toggle.type = "button";
  toggle.setAttribute("aria-expanded", "false");
  toggle.textContent = "Config";
  controls?.prepend(toggle);

  const panel = document.getElementById(
    "threejs-template-config-panel",
  ) as HTMLDivElement | null;
  if (panel && toggle) {
    const setOpen = (open: boolean) => {
      panel.hidden = !open;
      panel.style.display = open ? "grid" : "none";
      toggle.setAttribute("aria-expanded", String(open));
    };
    setOpen(false);
    toggle.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      setOpen(panel.hidden);
    });
  }

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "One cube, one camera, one light.",
    "Copy this component for new Three.js sections.",
    "The simplest working section template.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new TemplateVisualization(container);
  if (panel) {
    const row = document.createElement("div");
    row.className = "earth-config-row";
    const label = document.createElement("label");
    label.className = "earth-config-label";
    label.htmlFor = "threejs-template-spin";
    label.textContent = "Spin";
    const slider = document.createElement("input");
    slider.id = "threejs-template-spin";
    slider.type = "range";
    slider.min = "0";
    slider.max = "1";
    slider.step = "0.01";
    slider.value = `${viz.spinSpeed}`;
    row.appendChild(label);
    row.appendChild(slider);
    const valueEl = document.createElement("span");
    valueEl.className = "earth-config-value";
    valueEl.textContent = viz.spinSpeed.toFixed(2);
    row.appendChild(valueEl);
    panel.appendChild(row);
    slider.addEventListener("input", () => {
      viz.spinSpeed = parseFloat(slider.value);
      valueEl.textContent = viz.spinSpeed.toFixed(2);
    });
  }
  return {
    dispose: () => {
      viz.dispose();
      toggle.remove();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
