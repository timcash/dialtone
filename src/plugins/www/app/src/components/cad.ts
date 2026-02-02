import * as THREE from "three";
import { STLLoader } from "three/examples/jsm/loaders/STLLoader.js";
import glowVertexShader from "../shaders/glow.vert.glsl?raw";
import glowFragmentShader from "../shaders/glow.frag.glsl?raw";

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

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setSize(container.clientWidth, container.clientHeight);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.outputColorSpace = THREE.SRGBColorSpace;
    this.container.appendChild(this.renderer.domElement);

    this.scene.add(this.gearGroup);
    this.camera.position.set(0, 80, 160);
    this.camera.lookAt(0, 0, 0);

    this.initLights();
    this.initConfigPanel();

    // Load default model immediately, then try to update from live backend
    this.loadDefaultModel().then(() => {
      this.updateModel();
    });

    this.animate();

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

  private setPanelOpen(open: boolean) {
    if (!this.panelEl || !this.toggleEl) return;
    this.panelEl.hidden = !open;
    this.panelEl.style.display = open ? "grid" : "none";
    this.toggleEl.setAttribute("aria-expanded", String(open));
  }

  initConfigPanel() {
    this.panelEl = document.getElementById(
      "cad-config-panel",
    ) as HTMLDivElement | null;
    this.toggleEl = document.getElementById(
      "cad-config-toggle",
    ) as HTMLButtonElement | null;
    if (!this.panelEl || !this.toggleEl) return;

    this.setPanelOpen(false);
    this.toggleEl.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      this.setPanelOpen(this.panelEl!.hidden);
    });

    const addHeader = (text: string) => {
      const header = document.createElement("h3");
      header.textContent = text;
      this.panelEl?.appendChild(header);
    };

    const addSlider = (
      id: string,
      label: string,
      min: number,
      max: number,
      step: number,
    ) => {
      const row = document.createElement("div");
      row.className = "earth-config-row cad-config-row";
      const labelWrap = document.createElement("label");
      labelWrap.textContent = label;
      const slider = document.createElement("input");
      slider.type = "range";
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      // @ts-ignore
      slider.value = String(this.params[id]);
      labelWrap.appendChild(slider);
      row.appendChild(labelWrap);
      const valueEl = document.createElement("span");
      valueEl.className = "earth-config-value";
      valueEl.textContent = slider.value;
      row.appendChild(valueEl);
      this.panelEl?.appendChild(row);

      slider.addEventListener("input", () => {
        const v = parseFloat(slider.value);
        // @ts-ignore
        this.params[id] = v;
        valueEl.textContent = slider.value;
        this.debouncedUpdate();
      });
    };

    addHeader("Gear Parameters");

    this.offlineWarningEl = document.createElement("div");
    this.offlineWarningEl.className = "offline-warning";
    this.offlineWarningEl.innerHTML =
      "⚠️ CAD Server Offline. Start with <code>./dialtone.sh www cad demo</code> to enable parametric changes.";
    this.offlineWarningEl.hidden = true;
    this.panelEl?.appendChild(this.offlineWarningEl);

    addSlider("outer_diameter", "Outer Dia", 20, 200, 1);
    addSlider("inner_diameter", "Inner Dia", 5, 100, 1);
    addSlider("thickness", "Thickness", 2, 50, 1);
    addSlider("num_teeth", "Num Teeth", 5, 100, 1);
    addSlider("num_mounting_holes", "Mount Holes", 0, 12, 1);
    addSlider("mounting_hole_diameter", "Hole Dia", 2, 20, 1);

    const dlBtn = document.createElement("button");
    dlBtn.className = "premium-button";
    dlBtn.textContent = "Download STL";
    dlBtn.style.marginTop = "1rem";
    dlBtn.addEventListener("click", (e) => {
      e.preventDefault();
      this.downloadSTL();
    });
    this.panelEl?.appendChild(dlBtn);

    addHeader("Visualization");
    const addTranslationSlider = () => {
      const row = document.createElement("div");
      row.className = "earth-config-row cad-config-row";
      const labelWrap = document.createElement("label");
      labelWrap.textContent = "Translation X";
      const slider = document.createElement("input");
      slider.type = "range";
      slider.min = "-200";
      slider.max = "200";
      slider.step = "1";
      slider.value = String(this.translationX);
      labelWrap.appendChild(slider);
      row.appendChild(labelWrap);
      const valueEl = document.createElement("span");
      valueEl.className = "earth-config-value";
      valueEl.textContent = slider.value;
      row.appendChild(valueEl);
      this.panelEl?.appendChild(row);

      slider.addEventListener("input", () => {
        this.translationX = parseFloat(slider.value);
        valueEl.textContent = slider.value;
        if (this.gearGroup) {
          this.gearGroup.position.x = this.translationX;
        }
      });
    };
    addTranslationSlider();

    const divider = document.createElement("div");
    divider.className = "code-divider";
    this.panelEl?.appendChild(divider);

    const ghBtn = document.createElement("button");
    ghBtn.className = "premium-button github-button";
    ghBtn.innerHTML = "<span>View Source on GitHub</span>";
    ghBtn.style.background = "rgba(255, 255, 255, 0.1)";
    ghBtn.style.border = "1px solid rgba(255, 255, 255, 0.2)";
    ghBtn.addEventListener("click", (e) => {
      e.preventDefault();
      window.open(
        "https://github.com/timcash/dialtone/blob/main/src/plugins/cad/backend/main.py",
        "_blank",
      );
    });
    this.panelEl?.appendChild(ghBtn);
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

  isVisible = true;
  setVisible(isVisible: boolean) {
    this.isVisible = isVisible;
    if (!isVisible) {
      this.setPanelOpen(false);
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    const now = performance.now();
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

    this.renderer.render(this.scene, this.camera);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener("resize", this.onResize);
    this.renderer.dispose();
    this.container.removeChild(this.renderer.domElement);
  }
}

export function mountCAD(container: HTMLElement) {
  const viewer = new CADViewer(container);
  // @ts-ignore
  window.cadViewer = viewer;
  return {
    dispose: () => {
      // @ts-ignore
      delete window.cadViewer;
      viewer.dispose();
    },
    setVisible: (v: boolean) => viewer.setVisible(v),
  };
}
