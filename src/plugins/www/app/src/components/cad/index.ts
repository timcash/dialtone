import * as THREE from "three";
import { STLLoader } from "three/examples/jsm/loaders/STLLoader.js";
import glowVertexShader from "../../shaders/glow.vert.glsl?raw";
import glowFragmentShader from "../../shaders/glow.frag.glsl?raw";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupCadMenu } from "./menu";



export class CADViewer {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 2000);
  renderer = new THREE.WebGLRenderer({
    antialias: true,
    alpha: true,
    powerPreference: "high-performance",
  });
  container: HTMLElement;
  gearGroup = new THREE.Group();
  frameId = 0;
  isVisible = true;
  frameCount = 0;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();



  // Animation state
  time = 0;
  lastFrameTime = performance.now();

  // Parameters (matching gear_generator.py)
  params = {
    outer_diameter: 80,
    inner_diameter: 20,
    thickness: 8,
    tooth_height: 6,
    tooth_width: 4,
    num_teeth: 20,
    num_mounting_holes: 4,
    mounting_hole_diameter: 6,
  };
  translationX = 22;

  loader = new STLLoader();
  abortController: AbortController | null = null;
  currentMesh: THREE.Mesh | null = null;
  currentWireframe: THREE.LineSegments | null = null;
  private fpsCounter = new FpsCounter("cad");

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setSize(container.clientWidth, container.clientHeight);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.outputColorSpace = THREE.SRGBColorSpace;
    const existingCanvas = container.querySelector('canvas');
    if (existingCanvas) {
      this.renderer.domElement = existingCanvas as HTMLCanvasElement;
    } else {
      this.container.appendChild(this.renderer.domElement);
    }

    this.scene.add(this.gearGroup);
    this.camera.position.set(0, 80, 160);
    this.camera.lookAt(0, 0, 0);

    this.initLights();
    this.initLights();

    // Load default model immediately, then try to update from live backend
    this.loadDefaultModel().then(() => {
      this.updateModel();
    });

    this.animate();

    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);

    window.addEventListener("resize", this.onResize);
  }

  async loadDefaultModel() {
    try {
      console.log("[cad] Loading default gear STL...");
      const response = await fetch("/default_gear.stl");
      if (!response.ok) throw new Error("Failed to load default gear");
      const arrayBuffer = await response.arrayBuffer();
      this.applyGeometry(this.loader.parse(arrayBuffer));
    } catch (e) {
      console.error("[cad] Failed to load default gear:", e);
    }
  }

  applyGeometry(geometry: THREE.BufferGeometry) {
    geometry.center();
    geometry.computeVertexNormals();

    if (this.currentMesh) {
      this.gearGroup.remove(this.currentMesh);
      this.currentMesh.geometry.dispose();
      (this.currentMesh.material as THREE.Material).dispose();
    }
    if (this.currentWireframe) {
      this.gearGroup.remove(this.currentWireframe);
      this.currentWireframe.geometry.dispose();
      (this.currentWireframe.material as THREE.Material).dispose();
    }

    const glowMat = this.createGlowMaterial(new THREE.Color(0x06b6d4), 1.0);
    this.currentMesh = new THREE.Mesh(geometry, glowMat);
    this.gearGroup.add(this.currentMesh);

    const wireMat = new THREE.LineBasicMaterial({
      color: 0x3b82f6,
      transparent: true,
      opacity: 0.35,
    });
    this.currentWireframe = new THREE.LineSegments(
      new THREE.WireframeGeometry(geometry),
      wireMat,
    );
    this.gearGroup.add(this.currentWireframe);
    this.gearGroup.position.set(this.translationX, 0, 0);
  }

  createGlowMaterial(
    color: THREE.Color,
    intensity = 1.0,
  ): THREE.ShaderMaterial {
    return new THREE.ShaderMaterial({
      uniforms: {
        uColor: { value: color },
        uIntensity: { value: intensity },
        uTime: { value: 0 },
      },
      vertexShader: glowVertexShader,
      fragmentShader: glowFragmentShader,
      transparent: true,
      side: THREE.DoubleSide,
      blending: THREE.AdditiveBlending,
    });
  }

  initLights() {
    const ambientLight = new THREE.AmbientLight(0xffffff, 0.3);
    this.scene.add(ambientLight);

    const hemiLight = new THREE.HemisphereLight(0xffffff, 0x444444, 1.2);
    hemiLight.position.set(0, 50, 0);
    this.scene.add(hemiLight);

    const dirLight = new THREE.DirectionalLight(0xffffff, 1.5);
    dirLight.position.set(100, 150, 100);
    this.scene.add(dirLight);

    const pointLight = new THREE.PointLight(0x06b6d4, 1.8, 400);
    pointLight.position.set(-100, -50, 100);
    this.scene.add(pointLight);
  }

  offlineWarningEl: HTMLDivElement | null = null;
  panelEl: HTMLDivElement | null = null;
  toggleEl: HTMLButtonElement | null = null;

  setPanelOpen(open: boolean) {
    if (!this.panelEl || !this.toggleEl) return;
    this.panelEl.hidden = !open;
    this.panelEl.style.display = open ? "grid" : "none";
    this.toggleEl.setAttribute("aria-expanded", String(open));
  }



  fetchTimeout: any = null;
  debouncedUpdate() {
    if (this.fetchTimeout) clearTimeout(this.fetchTimeout);
    this.fetchTimeout = setTimeout(() => {
      this.updateModel();
    }, 800);
  }

  async updateModel() {
    this.abortController = new AbortController();

    try {
      const url = `/api/cad/generate`;

      console.log(`[cad] Fetching STL from: ${url}...`);
      const response = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(this.params),
        signal: this.abortController.signal,
      });

      console.log(
        `[cad] Response status: ${response.status} ${response.statusText}`,
      );
      if (!response.ok) {
        const text = await response.text();
        throw new Error(`HTTP ${response.status}: ${text}`);
      }

      console.log("[cad] STL Response OK, reading ArrayBuffer...");
      const arrayBuffer = await response.arrayBuffer();
      console.log(
        `[cad] Received ${arrayBuffer.byteLength} bytes. Parsing STL...`,
      );

      if (arrayBuffer.byteLength === 0) {
        throw new Error("Received empty STL data from backend");
      }

      const geometry = this.loader.parse(arrayBuffer);
      this.applyGeometry(geometry);

      if (this.offlineWarningEl) this.offlineWarningEl.hidden = true;
      console.log("[cad] Scene updated successfully");
    } catch (e: any) {
      if (e.name !== "AbortError") {
        console.error("[cad] Model update failed:", e);
        console.warn("[cad] Server might be offline or erroring:", e.message);
        if (this.offlineWarningEl) this.offlineWarningEl.hidden = false;
      }
    }
  }

  async downloadSTL() {
    const query = new URLSearchParams(
      Object.entries(this.params).map(([k, v]) => [k, String(v)]),
    ).toString();
    const url = `/api/cad/download?${query}`;
    window.open(url, "_blank");
  }

  onResize = () => {
    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    this.renderer.setSize(width, height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
  };

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "cad");
    if (!visible) {
      this.setPanelOpen(false);
      this.fpsCounter.clear();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    const cpuStart = performance.now();
    const now = cpuStart;
    const delta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    this.time += delta;

    // Update shaders
    this.gearGroup.children.forEach((child) => {
      if (
        child instanceof THREE.Mesh &&
        child.material instanceof THREE.ShaderMaterial
      ) {
        child.material.uniforms.uTime.value = this.time;
      }
    });

    if (this.gearGroup) {
      this.gearGroup.rotation.z += 0.005;
      this.gearGroup.rotation.y = Math.sin(this.time * 0.45) * 0.15;
      this.gearGroup.rotation.x = Math.cos(this.time * 0.25) * 0.12;
    }

    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener("resize", this.onResize);

    this.renderer.dispose();
    this.container.removeChild(this.renderer.domElement);
  }
}

export function mountCAD(container: HTMLElement) {
  // Inject HTML
  container.innerHTML = `
      <div class="marketing-overlay" aria-label="CAD marketing information">
        <h2>Parametric Logic</h2>
        <p data-typing-subtitle></p>
      </div>
    `;


  // Create and inject config toggle

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Iterate on hardware designs in real time.",
    "Change parameters and rebuild instantly.",
    "Programmatic CAD that turns code into mass.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viewer = new CADViewer(container);
  // @ts-ignore
  window.cadViewer = viewer;

  const options = {
    params: viewer.params,
    translationX: viewer.translationX,
    onParamChange: (key: any, value: any) => {
      // @ts-ignore
      viewer.params[key] = value;
      viewer.debouncedUpdate();
    },
    onTranslationChange: (value: any) => {
      viewer.translationX = value;
      if (viewer.gearGroup) {
        viewer.gearGroup.position.x = viewer.translationX;
      }
    },
    onDownloadStl: () => {
      viewer.downloadSTL();
    },
  };

  return {
    dispose: () => {
      // @ts-ignore
      delete window.cadViewer;
      viewer.dispose();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      viewer.setVisible(visible);
    },
    updateUI: () => {
      setupCadMenu(options);
    }
  };
}
