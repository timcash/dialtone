import * as THREE from "three";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import cubeGlowVert from "../../shaders/template-cube.vert.glsl?raw";
import cubeGlowFrag from "../../shaders/template-cube.frag.glsl?raw";
import { startTyping } from "../util/typing";

/**
 * Radio section: aligned with threejs-template (key light, glow, FPS, backend).
 * Handheld radio built from sub-components; slow oscillation animation.
 */

class RadioVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  private fpsCounter = new FpsCounter("radio");
  private keyLight!: THREE.DirectionalLight;
  private rimLight!: THREE.DirectionalLight;
  private frontLight!: THREE.DirectionalLight;
  private lightDir = new THREE.Vector3(1, 1, 1).normalize();

  radioGroup = new THREE.Group();
  bodyGroup = new THREE.Group();
  lcdGroup = new THREE.Group();
  antennasGroup = new THREE.Group();
  knobsGroup = new THREE.Group();
  private bodyMaterial!: THREE.ShaderMaterial;
  private lcdCanvas?: HTMLCanvasElement;
  private lcdTexture?: THREE.CanvasTexture;
  private lcdTypingStop?: () => void;
  private time = 0;
  frameCount = 0;

  constructor(container: HTMLElement) {
    this.container = container;

    this.renderer.setClearColor(0x000000, 1);
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

    this.scene.add(new THREE.AmbientLight(0xccddff, 0.55));
    this.keyLight = new THREE.DirectionalLight(0xffddbb, 1.8);
    this.keyLight.position.set(4, 3, 2.5);
    this.scene.add(this.keyLight);
    this.rimLight = new THREE.DirectionalLight(0x88aaff, 1.2);
    this.rimLight.position.set(-2.5, 1, -4);
    this.scene.add(this.rimLight);
    this.frontLight = new THREE.DirectionalLight(0xffffff, 0.9);
    this.frontLight.position.set(0, 0, 5);
    this.scene.add(this.frontLight);

    this.buildRadio();
    this.scene.add(this.radioGroup);

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

  private buildRadio() {
    const bodyW = 1.2;
    const bodyH = 0.6;
    const bodyD = 0.25;

    // 1. Body (handheld case)
    const bodyGeo = new THREE.BoxGeometry(bodyW, bodyH, bodyD);
    const bodyMat = new THREE.ShaderMaterial({
      uniforms: {
        uColor: { value: new THREE.Color(0x5a6070) },
        uGlowColor: { value: new THREE.Color(0x88aacc) },
        uLightDir: { value: this.lightDir.clone() },
        uTime: { value: 0 },
      },
      vertexShader: cubeGlowVert,
      fragmentShader: cubeGlowFrag,
    });
    this.bodyMaterial = bodyMat;
    const body = new THREE.Mesh(bodyGeo, bodyMat);
    this.bodyGroup.add(body);
    this.radioGroup.add(this.bodyGroup);

    // 2. LCD screen (glow)
    const screenGeo = new THREE.PlaneGeometry(0.5, 0.22);
    const screenMat = new THREE.MeshStandardMaterial({
      color: 0x00dd88,
      emissive: 0x00aa55,
      emissiveIntensity: 1.2,
      side: THREE.DoubleSide,
    });
    this.lcdCanvas = document.createElement("canvas");
    this.lcdCanvas.width = 256;
    this.lcdCanvas.height = 96;
    this.lcdTexture = new THREE.CanvasTexture(this.lcdCanvas);
    this.lcdTexture.minFilter = THREE.LinearFilter;
    this.lcdTexture.magFilter = THREE.LinearFilter;
    this.lcdTexture.wrapS = THREE.ClampToEdgeWrapping;
    this.lcdTexture.wrapT = THREE.ClampToEdgeWrapping;
    screenMat.map = this.lcdTexture;
    screenMat.emissiveMap = this.lcdTexture;
    screenMat.needsUpdate = true;
    const screen = new THREE.Mesh(screenGeo, screenMat);
    screen.position.set(0, 0.12, bodyD / 2 + 0.01);
    this.lcdGroup.add(screen);
    this.radioGroup.add(this.lcdGroup);

    // 3. Antennas
    const antennaGeo = new THREE.CylinderGeometry(0.02, 0.02, 0.35, 12);
    const antennaMat = new THREE.MeshStandardMaterial({
      color: 0x3a3a42,
      metalness: 0.5,
      roughness: 0.4,
    });
    const antennaL = new THREE.Mesh(antennaGeo, antennaMat);
    antennaL.position.set(-bodyW * 0.32, bodyH / 2 + 0.35 / 2, 0);
    this.antennasGroup.add(antennaL);
    const antennaR = new THREE.Mesh(antennaGeo, antennaMat);
    antennaR.position.set(bodyW * 0.32, bodyH / 2 + 0.35 / 2, 0);
    this.antennasGroup.add(antennaR);
    this.radioGroup.add(this.antennasGroup);

    // 4. Knobs
    const knobGeo = new THREE.CylinderGeometry(0.04, 0.04, 0.03, 16);
    const knobMat = new THREE.MeshStandardMaterial({
      color: 0x444450,
      metalness: 0.3,
      roughness: 0.5,
    });
    const knob1 = new THREE.Mesh(knobGeo, knobMat);
    knob1.rotation.x = Math.PI / 2;
    knob1.position.set(-0.35, -0.1, bodyD / 2 + 0.02);
    this.knobsGroup.add(knob1);
    const knob2 = new THREE.Mesh(knobGeo, knobMat);
    knob2.rotation.x = Math.PI / 2;
    knob2.position.set(0.35, -0.1, bodyD / 2 + 0.02);
    this.knobsGroup.add(knob2);
    this.radioGroup.add(this.knobsGroup);
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
    VisibilityMixin.setVisible(this, visible, "radio");
    if (!visible) this.fpsCounter.clear();
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;
    if (this.lcdTexture) {
      this.lcdTexture.needsUpdate = true;
    }
    this.lightDir.copy(this.keyLight.position).normalize();
    this.bodyMaterial.uniforms.uLightDir.value.copy(this.lightDir).transformDirection(this.camera.matrixWorldInverse);
    this.bodyMaterial.uniforms.uTime.value = this.time;

    this.radioGroup.rotation.y = Math.sin(this.time * 0.4) * 0.4;
    this.radioGroup.rotation.x = Math.sin(this.time * 0.25) * 0.18;

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}

export function mountRadio(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Radio section: handheld with LCD">
      <h2>Radios for robots</h2>
      <p data-typing-subtitle></p>
    </div>
    <p data-radio-lcd class="radio-lcd-typing"></p>
  `;

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Open hardware and software.",
    "Hand-held form factor with dual antenna.",
    "Mount it on your bot and stay connected.",
    "Built for field links and shared networks.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new RadioVisualization(container);
  const lcdTextEl = container.querySelector(
    "[data-radio-lcd]"
  ) as HTMLParagraphElement | null;
  const lcdStopTyping = startTyping(lcdTextEl, subtitles);

  if (lcdTextEl && viz["lcdCanvas"]) {
    const canvas = viz["lcdCanvas"] as HTMLCanvasElement;
    const ctx = canvas.getContext("2d");
    if (ctx) {
      const render = () => {
        const text = lcdTextEl.textContent?.replace("|", "").trim() ?? "";
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = "#003a2a";
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = "#7dffcf";
        ctx.font = "14px Inter";
        ctx.textBaseline = "top";
        const paddingX = 10;
        const paddingY = 8;
        const maxWidth = canvas.width - paddingX * 2;
        const lineHeight = 18;
        const words = text.length > 0 ? text.split(/\s+/) : [];
        const lines: string[] = [];
        let current = "";
        words.forEach((word) => {
          const next = current ? `${current} ${word}` : word;
          if (ctx.measureText(next).width <= maxWidth) {
            current = next;
          } else if (current) {
            lines.push(current);
            current = word;
          } else {
            lines.push(word);
            current = "";
          }
        });
        if (current) lines.push(current);
        if (lines.length === 0) lines.push("");
        const maxLines = Math.floor((canvas.height - paddingY * 2) / lineHeight);
        lines.slice(0, maxLines).forEach((line, idx) => {
          ctx.fillText(line, paddingX, paddingY + idx * lineHeight);
        });
      };
      const observer = new MutationObserver(render);
      observer.observe(lcdTextEl, { childList: true, characterData: true, subtree: true });
      render();
      viz["lcdTypingStop"] = () => observer.disconnect();
    }
  }
  return {
    dispose: () => {
      viz.dispose();
      stopTyping();
      lcdStopTyping();
      if (viz["lcdTypingStop"]) {
        viz["lcdTypingStop"]();
      }
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
    updateUI: () => {
        // No menu yet, but consistent with other components
    }
  };
}
