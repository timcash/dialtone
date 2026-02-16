import * as THREE from "three";
import { STLLoader } from "three/examples/jsm/loaders/STLLoader.js";
import glowVertexShader from "../../shaders/glow.vert.glsl?raw";
import glowFragmentShader from "../../shaders/glow.frag.glsl?raw";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupCadMenu } from "./menu";
import type { SectionManager, VisualizationControl } from "../util/section";

export class CADViewer {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 2000);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: "high-performance" });
  container: HTMLElement;
  sections: SectionManager;
  gearGroup = new THREE.Group();
  frameId = 0;
  isVisible = true;
  frameCount = 0;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  time = 0;
  lastFrameTime = performance.now();
  params = { outer_diameter: 80, inner_diameter: 20, thickness: 8, tooth_height: 6, tooth_width: 4, num_teeth: 20, num_mounting_holes: 4, mounting_hole_diameter: 6 };
  translationX = 22;
  loader = new STLLoader();
  abortController: AbortController | null = null;
  currentMesh: THREE.Mesh | null = null;
  currentWireframe: THREE.LineSegments | null = null;
  private fpsCounter = new FpsCounter("cad");

  constructor(container: HTMLElement, sections: SectionManager) {
    this.container = container;
    this.sections = sections;
    this.renderer.setSize(container.clientWidth, container.clientHeight);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.container.appendChild(this.renderer.domElement);
    this.scene.add(this.gearGroup);
    this.camera.position.set(0, 80, 160);
    this.camera.lookAt(0, 0, 0);
    this.initLights();
    this.animate();
    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);
    window.addEventListener("resize", this.onResize);
    this.backgroundInit();
  }

  async backgroundInit() {
    await this.loadDefaultModel();
    await this.updateModel();
  }

  async loadDefaultModel() {
    try {
      this.sections.setLoadingMessage("s-cad", `loading gear(${this.params.num_mounting_holes} mounting holes) ...`);
      const response = await fetch("/default_gear.stl");
      if (response.ok) this.applyGeometry(this.loader.parse(await response.arrayBuffer()));
    } catch (e) {}
  }

  applyGeometry(geometry: THREE.BufferGeometry) {
    geometry.center(); geometry.computeVertexNormals();
    if (this.currentMesh) { this.gearGroup.remove(this.currentMesh); this.currentMesh.geometry.dispose(); (this.currentMesh.material as THREE.Material).dispose(); }
    if (this.currentWireframe) { this.gearGroup.remove(this.currentWireframe); this.currentWireframe.geometry.dispose(); (this.currentWireframe.material as THREE.Material).dispose(); }
    this.currentMesh = new THREE.Mesh(geometry, new THREE.ShaderMaterial({ uniforms: { uColor: { value: new THREE.Color(0x06b6d4) }, uIntensity: { value: 1.0 }, uTime: { value: 0 } }, vertexShader: glowVertexShader, fragmentShader: glowFragmentShader, transparent: true, side: THREE.DoubleSide, blending: THREE.AdditiveBlending }));
    this.gearGroup.add(this.currentMesh);
    this.currentWireframe = new THREE.LineSegments(new THREE.WireframeGeometry(geometry), new THREE.LineBasicMaterial({ color: 0x3b82f6, transparent: true, opacity: 0.35 }));
    this.gearGroup.add(this.currentWireframe);
    this.gearGroup.position.set(this.translationX, 0, 0);
  }

  initLights() {
    this.scene.add(new THREE.AmbientLight(0xffffff, 0.3));
    const hemi = new THREE.HemisphereLight(0xffffff, 0x444444, 1.2); hemi.position.set(0, 50, 0); this.scene.add(hemi);
    const dir = new THREE.DirectionalLight(0xffffff, 1.5); dir.position.set(100, 150, 100); this.scene.add(dir);
    const point = new THREE.PointLight(0x06b6d4, 1.8, 400); point.position.set(-100, -50, 100); this.scene.add(point);
  }

  debouncedUpdate() { if (this.fetchTimeout) clearTimeout(this.fetchTimeout); this.fetchTimeout = setTimeout(() => this.updateModel(), 800); }
  private fetchTimeout: any = null;

  async updateModel() {
    this.abortController = new AbortController();
    this.sections.setLoadingMessage("s-cad", `generating parametric gear ...`);
    try {
      const response = await fetch(`/api/cad/generate`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(this.params), signal: this.abortController.signal });
      if (response.ok) this.applyGeometry(this.loader.parse(await response.arrayBuffer()));
    } catch (e) {}
  }

  onResize = () => { this.renderer.setSize(this.container.clientWidth, this.container.clientHeight); this.camera.aspect = this.container.clientWidth / this.container.clientHeight; this.camera.updateProjectionMatrix(); };
  setVisible(v: boolean) { VisibilityMixin.setVisible(this, v, "cad"); if (!v) this.fpsCounter.clear(); }
  animate = () => {
    this.frameId = requestAnimationFrame(this.animate); if (!this.isVisible) return;
    const now = performance.now(), delta = (now - this.lastFrameTime) / 1000; this.lastFrameTime = now; this.time += delta;
    this.gearGroup.children.forEach(c => { if (c instanceof THREE.Mesh && c.material instanceof THREE.ShaderMaterial) c.material.uniforms.uTime.value = this.time; });
    this.gearGroup.rotation.z += 0.005; this.gearGroup.rotation.y = Math.sin(this.time * 0.45) * 0.15; this.gearGroup.rotation.x = Math.cos(this.time * 0.25) * 0.12;
    this.gpuTimer.begin(this.gl); this.renderer.render(this.scene, this.camera); this.gpuTimer.end(this.gl); this.gpuTimer.poll(this.gl);
    this.fpsCounter.tick(performance.now() - now, this.gpuTimer.lastMs);
  };
  dispose() { cancelAnimationFrame(this.frameId); window.removeEventListener("resize", this.onResize); this.renderer.dispose(); this.container.removeChild(this.renderer.domElement); }
}

export function mountCAD(container: HTMLElement, sections: SectionManager): VisualizationControl {
  container.innerHTML = `<div class="marketing-overlay"><h2>Parametric Logic</h2><p data-typing-subtitle></p></div>`;
  const stopTyping = startTyping(container.querySelector("[data-typing-subtitle]"), ["Iterate on hardware designs in real time.", "Change parameters and rebuild instantly.", "Programmatic CAD that turns code into mass."]);
  const viewer = new CADViewer(container, sections);
  const options = { params: viewer.params, translationX: viewer.translationX, onParamChange: (k: any, v: any) => { (viewer.params as any)[k] = v; viewer.debouncedUpdate(); }, onTranslationChange: (v: any) => { viewer.translationX = v; viewer.gearGroup.position.x = v; }, onDownloadStl: () => { const q = new URLSearchParams(Object.entries(viewer.params).map(([k, v]) => [k, String(v)])).toString(); window.open(`/api/cad/download?${q}`, "_blank"); } };
  return { dispose: () => { viewer.dispose(); stopTyping(); container.innerHTML = ""; }, setVisible: (v) => viewer.setVisible(v), updateUI: () => setupCadMenu(options) };
}
