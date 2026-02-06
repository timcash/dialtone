import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { FpsCounter } from "./fps";
import { GpuTimer } from "./gpu_timer";
import { VisibilityMixin } from "./section";
import { SearchLights } from "./search_lights";
import { startTyping } from "./typing";
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
    count: 4,
    dwell: 8,
    wander: 8,
    seed: 1337,
    brightness: 1.35,
  };
  const sparkConfig = {
    intervalSeconds: 4,
    pauseMs: 1200,
    drainRatePerMs: 0.001,
  };
  const powerConfig = {
    maxPower: 5,
    regenPerSec: 1,
    restThreshold: 1,
  };
  const motionConfig = {
    glideSpeed: 7,
    glideAccel: 8,
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

  let presetSwapTimer: number | null = null;
  if (panel) {
    panel.classList.add("about-config-panel");
    const addHeader = (text: string) => {
      const header = document.createElement("h3");
      header.textContent = text;
      panel.appendChild(header);
    };
    const sliderRegistry: Record<string, { slider: HTMLInputElement; valueEl: HTMLSpanElement }> = {};
    const addSlider = (
      label: string,
      min: number,
      max: number,
      step: number,
      value: number,
      onInput: (v: number) => void,
      format: (v: number) => string = (v) => v.toFixed(0),
      key?: string
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
      if (key) {
        sliderRegistry[key] = { slider, valueEl };
      }
    };
    const setSliderValue = (key: string, value: number, format?: (v: number) => string) => {
      const entry = sliderRegistry[key];
      if (!entry) return;
      entry.slider.value = `${value}`;
      entry.valueEl.textContent = format ? format(value) : value.toFixed(0);
    };

    addHeader("Presets");
    const lightPresets = [
      {
        label: "Dim",
        count: 3,
        brightness: 0.9,
        maxPower: 4,
        regenPerSec: 0.7,
        restThreshold: 0.8,
      },
      {
        label: "Balanced",
        count: 4,
        brightness: 1.35,
        maxPower: 5,
        regenPerSec: 1,
        restThreshold: 1,
      },
      {
        label: "Bright",
        count: 5,
        brightness: 1.7,
        maxPower: 6,
        regenPerSec: 1.3,
        restThreshold: 1.2,
      },
      {
        label: "Hot",
        count: 6,
        brightness: 2.1,
        maxPower: 7,
        regenPerSec: 1.8,
        restThreshold: 1.4,
      },
    ];
    const motionPresets = [
      { label: "Floaty", glideSpeed: 4.5, glideAccel: 5.5, wander: 6, dwell: 10 },
      { label: "Steady", glideSpeed: 7, glideAccel: 8, wander: 8, dwell: 8 },
      { label: "Agile", glideSpeed: 10, glideAccel: 12, wander: 10, dwell: 6 },
      { label: "Frenzy", glideSpeed: 12, glideAccel: 14, wander: 12, dwell: 5 },
    ];
    const sparkPresets = [
      { label: "Rare", intervalSeconds: 7, pauseMs: 1500, drainRatePerMs: 0.0006 },
      { label: "Normal", intervalSeconds: 4, pauseMs: 1200, drainRatePerMs: 0.001 },
      { label: "Active", intervalSeconds: 2.5, pauseMs: 900, drainRatePerMs: 0.0013 },
      { label: "Storm", intervalSeconds: 1.5, pauseMs: 700, drainRatePerMs: 0.0016 },
    ];
    let lightPresetIndex = 1;
    let motionPresetIndex = 1;
    let sparkPresetIndex = 1;
    let presetSwapEnabled = 0;
    const presetSwapMs = 5000;

    const applyLightPreset = (index: number, syncUi = false) => {
      const preset = lightPresets[index];
      lightConfig.count = preset.count;
      lightConfig.brightness = preset.brightness;
      powerConfig.maxPower = preset.maxPower;
      powerConfig.regenPerSec = preset.regenPerSec;
      powerConfig.restThreshold = preset.restThreshold;
      viz.setLightCount(preset.count);
      viz.setBrightness(preset.brightness);
      viz.setMaxPower(preset.maxPower);
      viz.setPowerRegenRatePerSec(preset.regenPerSec);
      viz.setRestThreshold(preset.restThreshold);
      if (syncUi) {
        setSliderValue("lightPreset", index, (v) => lightPresets[Math.round(v)]?.label ?? `${v}`);
      }
    };
    const applyMotionPreset = (index: number, syncUi = false) => {
      const preset = motionPresets[index];
      lightConfig.wander = preset.wander;
      lightConfig.dwell = preset.dwell;
      motionConfig.glideSpeed = preset.glideSpeed;
      motionConfig.glideAccel = preset.glideAccel;
      viz.setWanderDistance(preset.wander);
      viz.setDwellSeconds(preset.dwell);
      viz.setGlideSpeed(preset.glideSpeed);
      viz.setGlideAccel(preset.glideAccel);
      if (syncUi) {
        setSliderValue("motionPreset", index, (v) => motionPresets[Math.round(v)]?.label ?? `${v}`);
      }
    };
    const applySparkPreset = (index: number, syncUi = false) => {
      const preset = sparkPresets[index];
      sparkConfig.intervalSeconds = preset.intervalSeconds;
      sparkConfig.pauseMs = preset.pauseMs;
      sparkConfig.drainRatePerMs = preset.drainRatePerMs;
      viz.setSparkIntervalSeconds(preset.intervalSeconds);
      viz.setSparkPauseMs(preset.pauseMs);
      viz.setSparkDrainRatePerMs(preset.drainRatePerMs);
      if (syncUi) {
        setSliderValue("sparkPreset", index, (v) => sparkPresets[Math.round(v)]?.label ?? `${v}`);
      }
    };

    applyLightPreset(lightPresetIndex);
    applyMotionPreset(motionPresetIndex);
    applySparkPreset(sparkPresetIndex);

    addSlider(
      "Light Preset",
      0,
      lightPresets.length - 1,
      1,
      lightPresetIndex,
      (v) => {
        lightPresetIndex = Math.round(v);
        applyLightPreset(lightPresetIndex);
      },
      (v) => lightPresets[Math.round(v)]?.label ?? `${v}`,
      "lightPreset"
    );
    addSlider(
      "Motion Preset",
      0,
      motionPresets.length - 1,
      1,
      motionPresetIndex,
      (v) => {
        motionPresetIndex = Math.round(v);
        applyMotionPreset(motionPresetIndex);
      },
      (v) => motionPresets[Math.round(v)]?.label ?? `${v}`,
      "motionPreset"
    );
    addSlider(
      "Spark Preset",
      0,
      sparkPresets.length - 1,
      1,
      sparkPresetIndex,
      (v) => {
        sparkPresetIndex = Math.round(v);
        applySparkPreset(sparkPresetIndex);
      },
      (v) => sparkPresets[Math.round(v)]?.label ?? `${v}`,
      "sparkPreset"
    );
    addSlider(
      "Preset Swap",
      0,
      1,
      1,
      presetSwapEnabled,
      (v) => {
        presetSwapEnabled = Math.round(v);
        if (presetSwapEnabled && presetSwapTimer === null) {
          presetSwapTimer = window.setInterval(() => {
            lightPresetIndex = (lightPresetIndex + 1) % lightPresets.length;
            motionPresetIndex = (motionPresetIndex + 1) % motionPresets.length;
            sparkPresetIndex = (sparkPresetIndex + 1) % sparkPresets.length;
            applyLightPreset(lightPresetIndex, true);
            applyMotionPreset(motionPresetIndex, true);
            applySparkPreset(sparkPresetIndex, true);
          }, presetSwapMs);
        } else if (!presetSwapEnabled && presetSwapTimer !== null) {
          window.clearInterval(presetSwapTimer);
          presetSwapTimer = null;
        }
      },
      (v) => (Math.round(v) === 1 ? "On" : "Off")
    );
    addHeader("Seed");
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
      if (presetSwapTimer !== null) {
        window.clearInterval(presetSwapTimer);
        presetSwapTimer = null;
      }
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
