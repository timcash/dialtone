import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { SearchLights } from "./search_lights";
import { setupAboutMenu } from "./menu";

import { VisionGrid } from "./vision_grid";

const NX = 1;
const NY = 100;
const NZ = 100;
const MAX_INSTANCES = 80000;
const INITIAL_ON = 0;

function makeRng(seed: number): () => number {
  let t = seed >>> 0;
  return () => {
    t += 0x6D2B79F5;
    let r = Math.imul(t ^ (t >>> 15), 1 | t);
    r ^= r + Math.imul(r ^ (r >>> 7), 61 | r);
    return ((r ^ (r >>> 14)) >>> 0) / 4294967296;
  };
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

  private grid: VisionGrid;
  private instancedMesh!: THREE.InstancedMesh;
  private dummy = new THREE.Object3D();
  private color = new THREE.Color();
  private composer?: EffectComposer;
  private bloomPass?: UnrealBloomPass;
  private searchLights!: SearchLights;
  private rng: () => number = Math.random;
  private lastStepTime = 0;
  private lightBrushEnabled = true;
  private lastFrameTime = performance.now();
  private stepIntervalMs = 100;
  private lastPowerLogMs = performance.now();

  constructor(container: HTMLElement) {
    this.container = container;
    this.grid = new VisionGrid(NX, NY, NZ, this.rng);

    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(window.devicePixelRatio);

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
      roughness: 0.5,
      metalness: 0.1,
      emissive: new THREE.Color(0xffffff),
      emissiveIntensity: 0.02,
    });
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

    this.grid.seedExactly(INITIAL_ON);
    this.updateInstances();
    this.lastStepTime = performance.now();

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

  private updateInstances() {
    const mesh = this.instancedMesh;
    const halfX = NX / 2;
    const halfY = NY / 2;
    const halfZ = NZ / 2;
    let count = 0;
    for (let i = 0; i < NX && count < MAX_INSTANCES; i++) {
      for (let j = 0; j < NY && count < MAX_INSTANCES; j++) {
        for (let k = 0; k < NZ && count < MAX_INSTANCES; k++) {
          if (this.grid.gridA[this.grid.index(i, j, k)] === 0) continue;
          this.dummy.position.set(i - halfX, j - halfY, k - halfZ);
          this.dummy.updateMatrix();
          mesh.setMatrixAt(count, this.dummy.matrix);
          const hue = ((i + j + k) / (NX + NY + NZ)) % 1;
          const glow = Math.min(1, this.grid.glowA[this.grid.index(i, j, k)] / this.grid.glowDurationMs);
          const lightness = 0.12 + glow * 0.22;
          this.color.setHSL(hue, 0.5, lightness);
          mesh.setColorAt(count, this.color);
          count++;
        }
      }
    }
    mesh.count = count;
    if (mesh.instanceColor) {
      mesh.instanceColor.needsUpdate = true;
    }
    mesh.instanceMatrix.needsUpdate = true;
  }

  /** Public: randomize grid and refresh instances (for Reset button). */
  reset(density = 0.08) {
    this.grid.randomSeed(density);
    this.updateInstances();
  }

  /** Public: exactly n cells on at random (for Reset button). */
  resetExactly(n: number) {
    this.grid.seedExactly(n);
    this.updateInstances();
  }

  /** Public: center-dense seed then refresh (for Reset button). */
  resetCenter(radius: number, density: number) {
    this.grid.centerSeed(radius, density);
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
    this.searchLights.setArcResolution(width * window.devicePixelRatio, height * window.devicePixelRatio);
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

  getMaxLights() {
    return this.searchLights.getMaxLights();
  }

  setSeed(seed: number) {
    this.rng = makeRng(seed);
    this.searchLights.resetForSeed(this.rng);
    this.grid.setRng(this.rng);
    this.grid.clear();
    this.lastStepTime = performance.now();
    this.updateInstances();
  }

  setDwellSeconds(seconds: number) {
    this.searchLights.setDwellSeconds(seconds);
  }

  setWanderDistance(distance: number) {
    this.searchLights.setWanderDistance(distance);
  }

  setBrightness(value: number) {
    this.searchLights.setBrightness(value);
  }

  setMaxPower(value: number) {
    this.searchLights.setMaxPower(value);
  }

  setPowerRegenRatePerSec(rate: number) {
    this.searchLights.setPowerRegenRatePerSec(rate);
  }

  setSparkIntervalSeconds(seconds: number) {
    this.searchLights.setSparkIntervalSeconds(seconds);
  }

  setSparkPauseMs(ms: number) {
    this.searchLights.setSparkPauseMs(ms);
  }

  setSparkDrainRatePerMs(rate: number) {
    this.searchLights.setSparkDrainRatePerMs(rate);
  }

  setRestThreshold(value: number) {
    this.searchLights.setRestThreshold(value);
  }

  setGlideSpeed(value: number) {
    this.searchLights.setGlideSpeed(value);
  }

  setGlideAccel(value: number) {
    this.searchLights.setGlideAccel(value);
  }

  setStepIntervalMs(intervalMs: number) {
    if (intervalMs <= 0) {
      this.stepIntervalMs = Infinity;
      return;
    }
    this.stepIntervalMs = Math.max(10, intervalMs);
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;

    const now = performance.now();
    const deltaMs = Math.min(100, now - this.lastFrameTime);
    this.lastFrameTime = now;
    this.grid.decayGlow(deltaMs);
    if (this.stepIntervalMs > 0 && now - this.lastStepTime >= this.stepIntervalMs) {
      this.lastStepTime = now;
      this.grid.step();
    }
    this.searchLights.update(now);
    if (this.lightBrushEnabled) {
      const cells = this.searchLights.getLightGridCells();
      const sparkedCells = this.searchLights.spawnSpark(cells);
      if (sparkedCells.length > 0) {
        this.grid.injectGlider(sparkedCells);
      }
    }

    if (now - this.lastPowerLogMs >= 1000) {
      this.lastPowerLogMs = now;
      const levels = this.searchLights
        .getPowerLevels()
        .map((power) => Number(power.toFixed(2)));
      console.log("[about] light power levels:", levels);
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
  `;


  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;

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

  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new VisionVisualization(container);
  const lightConfig = {
    count: 6,      // Hot
    dwell: 5,      // Frenzy
    wander: 12,    // Frenzy
    seed: 4830,
    brightness: 2.1, // Hot
  };
  const sparkConfig = {
    intervalSeconds: 1.5, // Storm
    pauseMs: 700,         // Storm
    drainRatePerMs: 0.0016, // Storm
  };
  const powerConfig = {
    maxPower: 7,       // Hot
    regenPerSec: 1.8,  // Hot
    restThreshold: 1.4, // Hot
  };
  const motionConfig = {
    glideSpeed: 12,   // Frenzy
    glideAccel: 14,   // Frenzy
  };
  const lifeConfig = {
    stepsPerSecond: 4,
  };
  viz.setLightCount(lightConfig.count);
  viz.setDwellSeconds(lightConfig.dwell);
  viz.setWanderDistance(lightConfig.wander);
  viz.setSeed(lightConfig.seed);
  viz.setBrightness(lightConfig.brightness);
  viz.setStepIntervalMs(lifeConfig.stepsPerSecond > 0 ? 1000 / lifeConfig.stepsPerSecond : 0);
  viz.setSparkIntervalSeconds(sparkConfig.intervalSeconds);
  viz.setSparkPauseMs(sparkConfig.pauseMs);
  viz.setSparkDrainRatePerMs(sparkConfig.drainRatePerMs);
  viz.setMaxPower(powerConfig.maxPower);
  viz.setPowerRegenRatePerSec(powerConfig.regenPerSec);
  viz.setRestThreshold(powerConfig.restThreshold);
  viz.setGlideSpeed(motionConfig.glideSpeed);
  viz.setGlideAccel(motionConfig.glideAccel);

  const reset = () => {
    viz.setLightCount(lightConfig.count);
    viz.setDwellSeconds(lightConfig.dwell);
    viz.setWanderDistance(lightConfig.wander);
    viz.setSeed(lightConfig.seed);
    viz.setBrightness(lightConfig.brightness);
    viz.setSparkIntervalSeconds(sparkConfig.intervalSeconds);
    viz.setSparkPauseMs(sparkConfig.pauseMs);
    viz.setSparkDrainRatePerMs(sparkConfig.drainRatePerMs);
    viz.setMaxPower(powerConfig.maxPower);
    viz.setPowerRegenRatePerSec(powerConfig.regenPerSec);
    viz.setRestThreshold(powerConfig.restThreshold);
    viz.setGlideSpeed(motionConfig.glideSpeed);
    viz.setGlideAccel(motionConfig.glideAccel);
  };

  const options = {
    viz: {
      setStepIntervalMs: (v: number) => {
        viz.setStepIntervalMs(v);
      },
      setLightCount: (v: number) => {
        lightConfig.count = v;
        reset();
      },
      setDwellSeconds: (v: number) => {
        lightConfig.dwell = v;
        reset();
      },
      setWanderDistance: (v: number) => {
        lightConfig.wander = v;
        reset();
      },
      setSeed: (v: number) => {
        lightConfig.seed = v;
        reset();
      },
      setBrightness: (v: number) => {
        lightConfig.brightness = v;
        reset();
      },
      setSparkIntervalSeconds: (v: number) => {
        sparkConfig.intervalSeconds = v;
        reset();
      },
      setSparkPauseMs: (v: number) => {
        sparkConfig.pauseMs = v;
        reset();
      },
      setSparkDrainRatePerMs: (v: number) => {
        sparkConfig.drainRatePerMs = v;
        reset();
      },
      setMaxPower: (v: number) => {
        powerConfig.maxPower = v;
        reset();
      },
      setPowerRegenRatePerSec: (v: number) => {
        powerConfig.regenPerSec = v;
        reset();
      },
      setRestThreshold: (v: number) => {
        powerConfig.restThreshold = v;
        reset();
      },
      setGlideSpeed: (v: number) => {
        motionConfig.glideSpeed = v;
        reset();
      },
      setGlideAccel: (v: number) => {
        motionConfig.glideAccel = v;
        reset();
      },
    },
    lightConfig,
    sparkConfig,
    powerConfig,
    motionConfig,
  };

  let cleanupMenu = () => { };

  return {
    dispose: () => {
      viz.dispose();
      cleanupMenu();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      viz.setVisible(visible);
      if (!visible) {
        cleanupMenu();
      }
    },
    updateUI: () => {
      cleanupMenu = setupAboutMenu(options);
    }
  };
}
