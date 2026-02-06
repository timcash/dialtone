import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { FpsCounter } from "./fps";
import { GpuTimer } from "./gpu_timer";
import { VisibilityMixin } from "./section";
import { SearchLights } from "./search_lights";

const NX = 1;
const NY = 100;
const NZ = 100;
const TOTAL = NX * NY * NZ;
const MAX_INSTANCES = 80000;
const INITIAL_ON = 0;

/** 8-neighbor offsets in 2D (Y/Z plane) */
const NEIGHBORS = [
  [0, -1, -1], [0, -1, 0], [0, -1, 1],
  [0, 0, -1], [0, 0, 1],
  [0, 1, -1], [0, 1, 0], [0, 1, 1],
];

function makeRng(seed: number): () => number {
  let t = seed >>> 0;
  return () => {
    t += 0x6D2B79F5;
    let r = Math.imul(t ^ (t >>> 15), 1 | t);
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r);
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296;
  };
}

function index(i: number, j: number, k: number): number {
  const ii = ((i % NX) + NX) % NX;
  const jj = ((j % NY) + NY) % NY;
  const kk = ((k % NZ) + NZ) % NZ;
  return ii + jj * NX + kk * NX * NY;
}

/**
 * Vision section: 3D grid of instanced cubes. Only "on" cells visible.
 * 2D Game of Life rules (B3/S23) on the Y/Z plane.
 */

class VisionVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  private fpsCounter = new FpsCounter("about");
  private time = 0;
  frameCount = 0;

  private gridA: Uint8Array;
  private gridB: Uint8Array;
  private instancedMesh!: THREE.InstancedMesh;
  private dummy = new THREE.Object3D();
  private color = new THREE.Color();
  private composer?: EffectComposer;
  private bloomPass?: UnrealBloomPass;
  private searchLights!: SearchLights;
  private rng: () => number = Math.random;
  private birthSet = new Set<number>([3]);
  private surviveSet = new Set<number>([2, 3]);
  private lastStepTime = 0;
  private lastBurstTime = 0;
  private burstIntervalMs = 1000;
  private burstCount = 1;
  private lastSplashTime = 0;
  private splashIntervalMs = 3000;
  private lightBrushEnabled = true;

  constructor(container: HTMLElement) {
    this.container = container;
    this.gridA = new Uint8Array(TOTAL);
    this.gridB = new Uint8Array(TOTAL);

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

    this.camera.position.set(35, 35, 35);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.85));
    const keyLight = new THREE.DirectionalLight(0xffffff, 1.4);
    keyLight.position.set(40, 60, 40);
    this.scene.add(keyLight);
    this.searchLights = new SearchLights(
      this.scene,
      {
        x: NX,
        y: NY,
        z: NZ,
      },
      Math.PI / 2,
      this.rng
    );

    const geo = new THREE.BoxGeometry(0.6, 0.6, 0.6);
    const mat = new THREE.MeshStandardMaterial({
      vertexColors: true,
      roughness: 0.08,
      metalness: 0.85,
      emissive: new THREE.Color(0xffffff),
      emissiveIntensity: 0.02,
    });
    mat.onBeforeCompile = (shader) => {
      shader.fragmentShader = shader.fragmentShader.replace(
        "#include <emissivemap_fragment>",
        [
          "#include <emissivemap_fragment>",
          "totalEmissiveRadiance += vColor * 0.6;",
        ].join("\n")
      );
    };
    this.instancedMesh = new THREE.InstancedMesh(geo, mat, MAX_INSTANCES);
    this.instancedMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
    this.instancedMesh.instanceColor = new THREE.InstancedBufferAttribute(
      new Float32Array(MAX_INSTANCES * 3),
      3
    );
    this.instancedMesh.instanceColor.setUsage(THREE.DynamicDrawUsage);
    this.instancedMesh.rotation.y = Math.PI / 2;
    this.scene.add(this.instancedMesh);

    this.composer = new EffectComposer(this.renderer);
    this.composer.addPass(new RenderPass(this.scene, this.camera));
    this.bloomPass = new UnrealBloomPass(
      new THREE.Vector2(1, 1),
      0.9,
      0.6,
      0.2
    );
    this.composer.addPass(this.bloomPass);

    this.seedExactly(INITIAL_ON);
    this.updateInstances();
    this.lastStepTime = performance.now();
    this.lastBurstTime = this.lastStepTime;
    this.lastSplashTime = this.lastStepTime;

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

  private randomSeed(density: number) {
    for (let i = 0; i < TOTAL; i++) {
      this.gridA[i] = this.rng() < density ? 1 : 0;
    }
  }

  /** Turn on exactly n cells at random. */
  private seedExactly(n: number) {
    this.gridA.fill(0);
    const indices = Array.from({ length: TOTAL }, (_, i) => i);
    for (let i = 0; i < n && i < indices.length; i++) {
      const r = i + Math.floor(this.rng() * (indices.length - i));
      [indices[i], indices[r]] = [indices[r], indices[i]];
      this.gridA[indices[i]] = 1;
    }
  }

  /** Fill a central cube (radius in cells from center) with given density. */
  private centerSeed(radius: number, density: number) {
    this.gridA.fill(0);
    const cx = NX / 2;
    const cy = NY / 2;
    const cz = NZ / 2;
    const loX = Math.max(0, Math.floor(cx - radius));
    const hiX = Math.min(NX, Math.ceil(cx + radius));
    const loY = Math.max(0, Math.floor(cy - radius));
    const hiY = Math.min(NY, Math.ceil(cy + radius));
    const loZ = Math.max(0, Math.floor(cz - radius));
    const hiZ = Math.min(NZ, Math.ceil(cz + radius));
    for (let i = loX; i < hiX; i++) {
      for (let j = loY; j < hiY; j++) {
        for (let k = loZ; k < hiZ; k++) {
          if (this.rng() < density) this.gridA[index(i, j, k)] = 1;
        }
      }
    }
  }

  step() {
    const read = this.gridA;
    const write = this.gridB;
    for (let i = 0; i < NX; i++) {
      for (let j = 0; j < NY; j++) {
        for (let k = 0; k < NZ; k++) {
          let neighbors = 0;
          for (const [di, dj, dk] of NEIGHBORS) {
            neighbors += read[index(i + di, j + dj, k + dk)];
          }
          const idx = index(i, j, k);
          const alive = read[idx];
          if (alive) {
            write[idx] = this.surviveSet.has(neighbors) ? 1 : 0;
          } else {
            write[idx] = this.birthSet.has(neighbors) ? 1 : 0;
          }
        }
      }
    }
    const t = this.gridA;
    this.gridA = this.gridB;
    this.gridB = t;
  }

  private injectBurst(count: number) {
    const glider = [
      [0, 1],
      [1, 2],
      [2, 0],
      [2, 1],
      [2, 2],
    ];
    let lastSpawn: { y: number; z: number } | null = null;
    for (let n = 0; n < count; n++) {
      const j = Math.floor(this.rng() * NY);
      const k = Math.floor(this.rng() * NZ);
      for (const [dj, dk] of glider) {
        this.gridA[index(0, j + dj, k + dk)] = 1;
      }
      lastSpawn = { y: j + 1, z: k + 1 };
    }
    if (lastSpawn) {
      this.searchLights.trackSpawn(lastSpawn.y, lastSpawn.z);
    }
  }

  private injectSplash() {
    const splash = [
      [0, 1], [0, 2], [0, 3],
      [1, 0], [1, 2], [1, 4],
      [2, 1], [2, 2], [2, 3],
      [3, 0], [3, 2], [3, 4],
      [4, 1], [4, 2], [4, 3],
    ];
    const j = Math.floor(this.rng() * NY);
    const k = Math.floor(this.rng() * NZ);
    for (const [dj, dk] of splash) {
      this.gridA[index(0, j + dj, k + dk)] = 1;
    }
  }

  private injectLightTrail(cells: Array<{ y: number; z: number }>) {
    cells.forEach(({ y, z }) => {
      this.gridA[index(0, y, z)] = 1;
    });
  }

  private updateInstances() {
    const mesh = this.instancedMesh;
    const halfX = NX / 2;
    const halfY = NY / 2;
    const halfZ = NZ / 2;
    let count = 0;
    for (let i = 0; i < NX && count < MAX_INSTANCES; i++) {
      for (let j = 0; j < NY && count < MAX_INSTANCES; j++) {
        for (let k = 0; k < NZ && count < MAX_INSTANCES; k++) {
          if (this.gridA[index(i, j, k)] === 0) continue;
          this.dummy.position.set(i - halfX, j - halfY, k - halfZ);
          this.dummy.updateMatrix();
          mesh.setMatrixAt(count, this.dummy.matrix);
          const hue = ((i + j + k) / (NX + NY + NZ)) % 1;
          this.color.setHSL(hue, 0.5, 0.22);
          mesh.setColorAt(count, this.color);
          count++;
        }
      }
    }
    mesh.count = count;
    mesh.instanceColor.needsUpdate = true;
    mesh.instanceMatrix.needsUpdate = true;
  }

  /** Public: randomize grid and refresh instances (for Reset button). */
  reset(density = 0.08) {
    this.randomSeed(density);
    this.updateInstances();
  }

  /** Public: exactly n cells on at random (for Reset button). */
  resetExactly(n: number) {
    this.seedExactly(n);
    this.updateInstances();
  }

  /** Public: center-dense seed then refresh (for Reset button). */
  resetCenter(radius: number, density: number) {
    this.centerSeed(radius, density);
    this.updateInstances();
  }

  resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
    this.composer?.setSize(width, height);
    if (this.bloomPass) this.bloomPass.setSize(width, height);
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
    VisibilityMixin.setVisible(this, visible, "about");
    if (!visible) {
      this.fpsCounter.clear();
    }
  }

  setLightCount(count: number) {
    this.searchLights.setLightCount(count);
  }

  setSeed(seed: number) {
    this.rng = makeRng(seed);
    this.searchLights.resetForSeed(this.rng);
    this.gridA.fill(0);
    this.gridB.fill(0);
    this.lastStepTime = performance.now();
    this.lastBurstTime = this.lastStepTime;
    this.lastSplashTime = this.lastStepTime;
    this.updateInstances();
  }

  setDwellSeconds(seconds: number) {
    this.searchLights.setDwellSeconds(seconds);
  }

  setWanderDistance(distance: number) {
    this.searchLights.setWanderDistance(distance);
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;

    const now = performance.now();
    if (now - this.lastStepTime >= 100) {
      this.lastStepTime = now;
      this.step();
    }
    this.searchLights.update(now);
    if (this.lightBrushEnabled) {
      const cells = this.searchLights.getLightGridCells();
      this.injectLightTrail(cells);
      this.searchLights.spawnLightning(cells);
    }
    this.updateInstances();

    this.camera.lookAt(0, 0, 0);
    this.camera.updateMatrixWorld(true);

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    if (this.composer) {
      this.composer.render();
    } else {
      this.renderer.render(this.scene, this.camera);
    }
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}

export function mountAbout(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Vision: global virtual librarian">
      <h2 data-typing-title>DIALTONE</h2>
      <p data-typing-subtitle></p>
    </div>
    <div id="about-config-panel" class="earth-config-panel" hidden></div>
  `;

  const controls = document.querySelector(".top-right-controls");
  const toggle = document.createElement("button");
  toggle.id = "about-config-toggle";
  toggle.className = "earth-config-toggle";
  toggle.type = "button";
  toggle.setAttribute("aria-expanded", "false");
  toggle.textContent = "Config";
  controls?.prepend(toggle);

  const panel = document.getElementById("about-config-panel") as HTMLDivElement | null;
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

  const titleEl = container.querySelector(
    "[data-typing-title]"
  ) as HTMLHeadingElement | null;
  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  let typingTimer: number | undefined;
  let typingTimeout: number | undefined;

  const subtitles = [
    "DIALTONE is a virtual librarian.",
    "An encrypted mesh network.",
    "Can be run on any computer.",
    "Turns compute into action.",
    "Connects math, engineering, robotics, and AI.",
    "Teaching through real systems and live networks.",
    "Civics and policy made legible and testable.",
    "A new kind of library the whole world runs.",
    "Publicly owned robots, radios, and shared tools.",
    "Secure identity, keys, and signatures built in.",
    "Live maps and status for real-world operations.",
    "Learning paths for math, engineering, and robotics.",
    "From theory to practice, field to classroom.",
    "Open, network-first, community operated.",
  ];

  const startTyping = () => {
    if (!subtitleEl) return;
    let index = 0;
    let charIndex = 0;

    const step = () => {
      const full = subtitles[index];
      const next = full.slice(0, Math.min(full.length, charIndex + 1));
      subtitleEl.textContent = `| ${next || "\u00A0"}`;
      charIndex += 1;

      if (charIndex >= full.length) {
        typingTimeout = window.setTimeout(() => {
          index = (index + 1) % subtitles.length;
          charIndex = 0;
          subtitleEl.textContent = "| \u00A0";
          step();
        }, 2000);
        return;
      }
      typingTimer = window.setTimeout(step, 30);
    };
    step();
  };

  startTyping();

  const viz = new VisionVisualization(container);
  const lightConfig = {
    count: 4,
    dwell: 8,
    wander: 8,
    seed: 1337,
  };
  viz.setLightCount(lightConfig.count);
  viz.setDwellSeconds(lightConfig.dwell);
  viz.setWanderDistance(lightConfig.wander);
  viz.setSeed(lightConfig.seed);

  if (panel) {
    panel.classList.add("about-config-panel");
    const addHeader = (text: string) => {
      const header = document.createElement("h3");
      header.textContent = text;
      panel.appendChild(header);
    };
    const addSlider = (
      label: string,
      min: number,
      max: number,
      step: number,
      value: number,
      onInput: (v: number) => void,
      format: (v: number) => string = (v) => v.toFixed(0)
    ) => {
      const row = document.createElement("div");
      row.className = "earth-config-row about-config-row";
      const labelWrap = document.createElement("label");
      labelWrap.textContent = label;
      const slider = document.createElement("input");
      slider.type = "range";
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;
      labelWrap.appendChild(slider);
      row.appendChild(labelWrap);
      const valueEl = document.createElement("span");
      valueEl.className = "earth-config-value";
      valueEl.textContent = format(value);
      row.appendChild(valueEl);
      panel.appendChild(row);
      slider.addEventListener("input", () => {
        const v = parseFloat(slider.value);
        onInput(v);
        valueEl.textContent = format(v);
      });
    };

    addHeader("Light Behavior");
    addSlider("Light Count", 1, 4, 1, lightConfig.count, (v) => {
      lightConfig.count = v;
      viz.setLightCount(v);
    });
    addSlider("Seed", 1, 9999, 1, lightConfig.seed, (v) => {
      const seed = Math.round(v);
      lightConfig.seed = seed;
      viz.setSeed(seed);
    });
    addSlider("Dwell (s)", 2, 15, 1, lightConfig.dwell, (v) => {
      lightConfig.dwell = v;
      viz.setDwellSeconds(v);
    });
    addSlider("Wander", 2, 16, 1, lightConfig.wander, (v) => {
      lightConfig.wander = v;
      viz.setWanderDistance(v);
    });
  }

  return {
    dispose: () => {
      viz.dispose();
      toggle.remove();
      if (typingTimer) window.clearTimeout(typingTimer);
      if (typingTimeout) window.clearTimeout(typingTimeout);
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
