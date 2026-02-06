import * as THREE from "three";
import { HexLayer } from "./hex_layer";
import { polygonToCells } from "h3-js";
import cloudVertexShader from "../shaders/cloud.vert.glsl?raw";
import cloudFragmentShader from "../shaders/cloud.frag.glsl?raw";
import atmosphereVertexShader from "../shaders/atmosphere.vert.glsl?raw";
import atmosphereFragmentShader from "../shaders/atmosphere.frag.glsl?raw";
import sunAtmosphereVertexShader from "../shaders/sun_atmosphere.vert.glsl?raw";
import sunAtmosphereFragmentShader from "../shaders/sun_atmosphere.frag.glsl?raw";
import { setupConfigPanel } from "./earth/config_ui";
import { FpsCounter } from "./fps";
import { GpuTimer } from "./gpu_timer";
import { VisibilityMixin } from "./section";
import { startTyping } from "./typing";

const DEG_TO_RAD = Math.PI / 180;
const TIME_SCALE = 1;
const SUN_ORBIT_PERIOD_MS = 5000;
// Human-scale axial rotation:
// Base axial rotation period at timeScale=1:
// 1 full rotation / 30s while the section is visible.
// (Animation pauses when you scroll off the section via VisibilityMixin.)
// Note: rotation is applied as earthRotSpeed * deltaSeconds, where deltaSeconds already includes `timeScale`.
const EARTH_ROT_PERIOD_SECONDS = 30;

// Light colors (shader-driven). Neutral white/cool.
const SUN_COLOR = new THREE.Color(1.0, 1.0, 1.0);
const KEY1_COLOR = new THREE.Color(0.9, 0.95, 1.0);
const KEY2_COLOR = new THREE.Color(0.85, 0.9, 1.0);
const KEY2_PHASE_OFFSET_RAD = Math.PI / 2; // 2π/4 behind the sun

const MOON_LIGHT_LAYER = 1;

function createMoonRockTexture(size = 128) {
  const canvas = document.createElement("canvas");
  canvas.width = size;
  canvas.height = size;

  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to create 2D context for moon texture");

  // Base mid-grey.
  ctx.fillStyle = "#7a7a7a";
  ctx.fillRect(0, 0, size, size);

  // Fine grain noise.
  const img = ctx.getImageData(0, 0, size, size);
  const d = img.data;
  for (let i = 0; i < d.length; i += 4) {
    const n = (Math.random() * 2 - 1) * 24; // +/- 24
    d[i + 0] = Math.max(0, Math.min(255, d[i + 0] + n));
    d[i + 1] = Math.max(0, Math.min(255, d[i + 1] + n));
    d[i + 2] = Math.max(0, Math.min(255, d[i + 2] + n));
    d[i + 3] = 255;
  }
  ctx.putImageData(img, 0, 0);

  // Simple crater-ish blotches.
  for (let i = 0; i < 22; i++) {
    const x = Math.random() * size;
    const y = Math.random() * size;
    const r = 4 + Math.random() * 14;
    const g = ctx.createRadialGradient(x, y, r * 0.2, x, y, r);
    g.addColorStop(0, "rgba(40,40,40,0.35)");
    g.addColorStop(0.6, "rgba(90,90,90,0.10)");
    g.addColorStop(1, "rgba(120,120,120,0.00)");
    ctx.fillStyle = g;
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fill();
  }

  const tex = new THREE.CanvasTexture(canvas);
  tex.wrapS = THREE.RepeatWrapping;
  tex.wrapT = THREE.RepeatWrapping;
  tex.repeat.set(2, 2);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.needsUpdate = true;
  return tex;
}

export class ProceduralOrbit {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(75, 1, 0.01, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();

  isVisible = true;
  frameCount = 0;

  earth!: THREE.Mesh;
  cloud1!: THREE.Mesh;
  cloud2!: THREE.Mesh;
  hexLayers: HexLayer[] = [];
  atmosphere!: THREE.Mesh;
  sunAtmosphere!: THREE.Mesh;
  moon!: THREE.Mesh;
  earthMaterial!: THREE.MeshStandardMaterial;
  cloud1Material!: THREE.ShaderMaterial;
  cloud2Material!: THREE.ShaderMaterial;
  atmosphereMaterial!: THREE.ShaderMaterial;
  sunAtmosphereMaterial!: THREE.ShaderMaterial;
  cloud1Axis = new THREE.Vector3(0, 1, 0);
  cloud2Axis = new THREE.Vector3(0.2, 1, -0.1).normalize();

  // Settings
  earthRadius = 5;
  shaderTimeScale = 0.28;
  timeScale = TIME_SCALE;
  // Clouds: make them more visible by default.
  cloudAmount = 0.92;
  cloudOpacityOscAmp = 0.12;
  cloudAmountOscAmp = 0.18;
  cloudOscSpeed = 0.45;

  // Rotations
  earthRotSpeed = (Math.PI * 2) / EARTH_ROT_PERIOD_SECONDS;
  cloud1RotSpeed = (Math.PI * 2) / 100;
  cloud2RotSpeed = (Math.PI * 2) / 120;
  cloud1Opacity = 0.82;
  cloud2Opacity = 0.78;
  cloudBrightness = 1.25;
  cameraDistance = 6.0;
  cameraOrbit = 0.4;
  cameraYaw = 0.2;
  cameraAnchor = new THREE.Vector3(0, 0, 0);

  // Lights
  sunGlow!: THREE.Mesh;
  sunLight!: THREE.PointLight;
  sunKeyLight!: THREE.DirectionalLight;
  sunKeyLight2!: THREE.DirectionalLight;
  ambientLight!: THREE.AmbientLight;

  sunDistance = 78;
  sunOrbitHeight = 87;
  sunOrbitAngleDeg = 0;
  sunOrbitSpeed = (Math.PI * 2) / SUN_ORBIT_PERIOD_MS / 2;
  sunOrbitIncline = 20 * DEG_TO_RAD;
  sunOrbitAngleRad = 0;
  sunTimeMs = performance.now();

  // Moon orbit (visual / demo-scale)
  moonRadius = 0.55;
  moonOrbitRadius = 12.5;
  moonOrbitIncline = 8 * DEG_TO_RAD;
  moonOrbitPhaseRad = 0.6;

  keyLightDistance = 147;
  keyLightHeight = 40;
  keyLightAngleDeg = 63;
  materialColorScale = 1.25;

  lastFrameTime = performance.now();
  configPanel?: HTMLDivElement;
  configToggle?: HTMLButtonElement;
  configValueMap = new Map<string, HTMLSpanElement>();
  configSliderMap = new Map<
    string,
    {
      slider: HTMLInputElement;
      valueEl: HTMLSpanElement;
      format: (v: number) => string;
      getValue: () => number;
    }
  >();
  private setConfigPanelOpen?: (open: boolean) => void;
  private fpsCounter = new FpsCounter("earth");

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 1.5));
    this.renderer.domElement.style.position = "absolute";
    this.renderer.domElement.style.top = "0";
    this.renderer.domElement.style.left = "0";
    this.renderer.domElement.style.width = "100%";
    this.renderer.domElement.style.height = "100%";
    this.renderer.domElement.style.display = "block";
    this.container.appendChild(this.renderer.domElement);
    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);


    this.initLayers();
    this.loadLandLayer();
    this.initLights();
    this.initConfigPanel();
    this.resize();
    this.initCameraAnchor();
    this.updateCamera(this.cameraAnchor);
    // Ensure we render both the default layer and the moon light-only layer.
    this.camera.layers.enable(MOON_LIGHT_LAYER);
    this.animate();

    // @ts-ignore: Expose for testing
    window.earthDebug = this;
    (window as any).THREE = THREE;

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
    const ratio = width / height;

    this.camera.aspect = ratio;

    // Centered but mobile FOV adjusted
    if (ratio < 1) {
      this.camera.fov = 95;
    } else {
      this.camera.fov = 75;
    }

    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    window.removeEventListener("resize", this.resize);
    this.renderer.dispose();
    this.container.removeChild(this.renderer.domElement);
  }

  initLayers() {
    const geo = (r: number, segments: number) =>
      new THREE.SphereGeometry(r, segments, segments);
    const earthSegments = 64;
    const cloudSegments = 48;
    const atmoSegments = 32;

    const earthMat = new THREE.MeshStandardMaterial({
      color: new THREE.Color(0x0b2a6f),
      roughness: 0.6,
      metalness: 0.05,
    });
    this.earthMaterial = earthMat;
    this.earth = new THREE.Mesh(geo(this.earthRadius, earthSegments), earthMat);
    this.scene.add(this.earth);

    const cloud1Mat = this.createCloudMaterial(0.2, this.cloud1Opacity);
    this.cloud1Material = cloud1Mat;
    this.cloud1 = new THREE.Mesh(
      geo(this.earthRadius + 0.05, cloudSegments),
      cloud1Mat,
    );
    this.scene.add(this.cloud1);

    const cloud2Mat = this.createCloudMaterial(0.5, this.cloud2Opacity);
    this.cloud2Material = cloud2Mat;
    this.cloud2 = new THREE.Mesh(
      geo(this.earthRadius + 0.08, cloudSegments),
      cloud2Mat,
    );
    this.scene.add(this.cloud2);

    // Reduced cloud layers for performance (2 layers instead of 4)

    this.hexLayers = [
      new HexLayer(this.earthRadius, {
        radiusOffset: 0.06,
        count: 240,
        resolution: 3,
        ratePerSecond: 45,
        durationSeconds: 3,
        palette: [
          new THREE.Color(0.85, 0.85, 0.86),
          new THREE.Color(0.65, 0.67, 0.7),
          new THREE.Color(0.1, 0.1, 0.12),
        ],
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 0.08,
        count: 200,
        resolution: 3,
        ratePerSecond: 45,
        durationSeconds: 3,
        palette: [
          new THREE.Color(0.75, 0.75, 0.76),
          new THREE.Color(0.45, 0.46, 0.5),
          new THREE.Color(0.05, 0.05, 0.07),
        ],
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 0.12,
        count: 160,
        resolution: 3,
        ratePerSecond: 45,
        durationSeconds: 3,
        palette: [
          new THREE.Color(0.9, 0.9, 0.9),
          new THREE.Color(0.55, 0.56, 0.6),
          new THREE.Color(0.15, 0.15, 0.18),
        ],
      }),
    ];
    this.hexLayers.forEach((layer) => this.earth.add(layer.mesh));

    const atmoMat = new THREE.ShaderMaterial({
      side: THREE.BackSide,
      transparent: true,
      blending: THREE.AdditiveBlending,
      uniforms: {
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uSunColor: { value: SUN_COLOR.clone() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyColor: { value: KEY1_COLOR.clone() },
        uKeyDir2: { value: new THREE.Vector3(0, 1, 0).normalize() },
        uKey2Color: { value: KEY2_COLOR.clone() },
        uKeyIntensity: { value: 0.8 },
        uKeyIntensity2: { value: 0.55 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale },
      },
      vertexShader: atmosphereVertexShader,
      fragmentShader: atmosphereFragmentShader,
    });
    this.atmosphereMaterial = atmoMat;
    this.atmosphere = new THREE.Mesh(
      geo(this.earthRadius + 0.2, atmoSegments),
      atmoMat,
    );
    this.scene.add(this.atmosphere);

    const sunAtmoMat = new THREE.ShaderMaterial({
      side: THREE.BackSide,
      transparent: true,
      blending: THREE.AdditiveBlending,
      uniforms: {
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uCameraPos: { value: new THREE.Vector3() },
        uColorScale: { value: this.materialColorScale },
      },
      vertexShader: sunAtmosphereVertexShader,
      fragmentShader: sunAtmosphereFragmentShader,
    });
    this.sunAtmosphereMaterial = sunAtmoMat;
    this.sunAtmosphere = new THREE.Mesh(
      geo(this.earthRadius + 0.32, atmoSegments),
      sunAtmoMat,
    );
    this.scene.add(this.sunAtmosphere);

    // Moon: lit by the scene lights (reflects sunLight + key lights)
    const moonMat = new THREE.MeshStandardMaterial({
      color: new THREE.Color(0.75, 0.75, 0.75),
      map: createMoonRockTexture(128),
      roughness: 0.95,
      metalness: 0.02,
    });
    this.moon = new THREE.Mesh(geo(this.moonRadius, 32), moonMat);
    // Moon should only reflect sunlight (no ambient/hemi/key lights).
    this.moon.layers.set(MOON_LIGHT_LAYER);
    this.scene.add(this.moon);
  }

  private async loadLandLayer() {
    try {
      const precomputed = await fetch("/land.h3.json");
      if (precomputed.ok) {
        const payload = await precomputed.json();
        const cells = Array.isArray(payload) ? payload : payload?.cells;
        const resolution = payload?.resolution ?? 3;
        if (Array.isArray(cells) && cells.length > 0) {
          this.buildLandLayer(cells, resolution);
          return;
        }
      }
      const response = await fetch("/land.geojson");
      if (!response.ok) return;
      const geojson = await response.json();
      const cells = this.geojsonToCells(geojson, 3);
      if (cells.length === 0) return;
      this.buildLandLayer(cells, 3);
    } catch {
      // Land layer is optional; ignore load errors.
    }
  }

  private buildLandLayer(cells: string[], resolution: number) {
    const landRadiusOffset = 0.03;
    const landLayer = new HexLayer(this.earthRadius, {
      radiusOffset: landRadiusOffset,
      count: cells.length,
      resolution,
      ratePerSecond: 1,
      durationSeconds: 9999,
      opacity: 0.95,
      palette: [
        new THREE.Color(0.2, 0.35, 0.2),
        new THREE.Color(0.25, 0.45, 0.25),
        new THREE.Color(0.4, 0.5, 0.3),
      ],
      cells,
      animate: false,
    });
    landLayer.material.depthWrite = false;
    landLayer.material.depthTest = true;
    landLayer.material.polygonOffset = true;
    landLayer.material.polygonOffsetFactor = -1;
    landLayer.material.polygonOffsetUnits = -1;
    landLayer.mesh.renderOrder = 1;
    landLayer.mesh.frustumCulled = false;
    this.hexLayers.push(landLayer);
    this.earth.add(landLayer.mesh);
    console.log("[earth] land layer ready", {
      cells: cells.length,
      resolution,
      earthRadius: this.earthRadius,
      landRadius: this.earthRadius + landRadiusOffset,
      cloud1Radius: this.earthRadius + 0.05,
      cloud2Radius: this.earthRadius + 0.08,
    });
  }

  private geojsonToCells(geojson: any, resolution: number) {
    const cells = new Set<string>();
    if (!geojson?.features) return [];
    geojson.features.forEach((feature: any) => {
      const geometry = feature?.geometry;
      if (!geometry) return;
      const polygons =
        geometry.type === "Polygon"
          ? [geometry.coordinates]
          : geometry.type === "MultiPolygon"
            ? geometry.coordinates
            : [];
      polygons.forEach((coords: number[][][]) => {
        try {
          polygonToCells(coords, resolution, true).forEach((cell) => cells.add(cell));
        } catch {
          // Skip invalid polygons.
        }
      });
    });
    return Array.from(cells);
  }

  createCloudMaterial(
    scale: number,
    opacity: number,
    tint: THREE.Color = new THREE.Color(1, 1, 1),
    fragmentShaderBase: string = cloudFragmentShader,
    extraUniforms: Record<string, THREE.IUniform> = {},
  ) {
    const fragmentShader = fragmentShaderBase.replace(
      /CLOUD_SCALE/g,
      scale.toFixed(2),
    );
    return new THREE.ShaderMaterial({
      transparent: true,
      uniforms: {
        uTime: { value: 0 },
        uTint: { value: tint },
        uOpacity: { value: opacity },
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uSunColor: { value: SUN_COLOR.clone() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyColor: { value: KEY1_COLOR.clone() },
        uKeyDir2: { value: new THREE.Vector3(0, 1, 0).normalize() },
        uKey2Color: { value: KEY2_COLOR.clone() },
        uKeyIntensity: { value: 0.8 },
        uKeyIntensity2: { value: 0.55 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale },
        uCloudAmount: { value: this.cloudAmount },
        ...extraUniforms,
      },
      vertexShader: cloudVertexShader,
      fragmentShader,
    });
  }

  initLights() {
    // Note: core Earth lighting is shader-driven; these lights are primarily for
    // non-shader meshes / debugging.
    this.sunKeyLight = new THREE.DirectionalLight(0xffffff, 0.35);
    this.sunKeyLight.position.set(10, 5, 10);
    this.scene.add(this.sunKeyLight);
    this.sunKeyLight.target.position.set(0, 0, 0);
    this.scene.add(this.sunKeyLight.target);

    this.sunKeyLight2 = new THREE.DirectionalLight(0xffffff, 0.22);
    this.sunKeyLight2.position.set(-10, -5, -10);
    this.scene.add(this.sunKeyLight2);
    this.sunKeyLight2.target.position.set(0, 0, 0);
    this.scene.add(this.sunKeyLight2.target);
    this.ambientLight = new THREE.AmbientLight(0x090a10, 0.26);
    this.scene.add(this.ambientLight);

    this.sunGlow = new THREE.Mesh(
      new THREE.SphereGeometry(6, 32, 32),
      new THREE.MeshBasicMaterial({ color: 0xffe08a }),
    );
    this.scene.add(this.sunGlow);

    const hemiLight = new THREE.HemisphereLight(0xffffff, 0x111111, 1.0);
    this.scene.add(hemiLight);
    this.sunLight = new THREE.PointLight(0xffffff, 2.1, 220);
    // The moon is rendered on a dedicated layer so it only sees the sun light.
    this.sunLight.layers.enable(MOON_LIGHT_LAYER);
    this.scene.add(this.sunLight);
  }

  initConfigPanel() {
    const { setOpen } = setupConfigPanel(this);
    this.setConfigPanelOpen = setOpen;
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "earth");
    if (!visible) {
      this.setConfigPanelOpen?.(false);
      this.fpsCounter.clear();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.frameCount++;
    const cpuStart = performance.now();
    const now = cpuStart;
    const rawDelta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    const deltaSeconds = rawDelta * this.timeScale;
    const cloudTime = now * 0.001 * this.shaderTimeScale;
    const osc = Math.sin(now * 0.001 * this.cloudOscSpeed);
    const oscOffset = Math.cos(now * 0.001 * this.cloudOscSpeed * 0.9);
    const cloudAmount = Math.max(
      0,
      Math.min(1, this.cloudAmount + osc * this.cloudAmountOscAmp),
    );
    const cloud1Opacity = Math.max(
      0.5,
      this.cloud1Opacity + oscOffset * this.cloudOpacityOscAmp,
    );
    const cloud2Opacity = Math.max(
      0.5,
      this.cloud2Opacity - oscOffset * this.cloudOpacityOscAmp,
    );

    // Rotations
    this.earth.rotation.y += this.earthRotSpeed * deltaSeconds;
    this.cloud1.rotateOnAxis(this.cloud1Axis, this.cloud1RotSpeed * rawDelta);
    this.cloud2.rotateOnAxis(this.cloud2Axis, this.cloud2RotSpeed * rawDelta);

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uTime.value =
      cloudTime;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uTime.value =
      cloudTime;

    this.updateCamera(this.cameraAnchor);

    // Sun Orbit
    const sunRad = this.earthRadius + this.sunOrbitHeight;
    const sunA = now * this.sunOrbitSpeed + this.sunOrbitAngleDeg * DEG_TO_RAD;
    const twoPi = Math.PI * 2;
    this.sunOrbitAngleRad = ((sunA % twoPi) + twoPi) % twoPi;
    this.sunTimeMs = now;
    const sinA = Math.sin(sunA);
    const cosA = Math.cos(sunA);
    const y = sinA * Math.sin(this.sunOrbitIncline) * sunRad;
    const z = sinA * Math.cos(this.sunOrbitIncline) * sunRad;
    this.sunLight.position.set(cosA * sunRad, y, z);
    this.sunGlow.position.copy(this.sunLight.position);

    const sDir = this.sunLight.position.clone().normalize();
    // Second key light: same orbit, trailing by 2π/4 radians.
    const keyA = sunA - KEY2_PHASE_OFFSET_RAD;
    const sinK = Math.sin(keyA);
    const cosK = Math.cos(keyA);
    const ky = sinK * Math.sin(this.sunOrbitIncline) * sunRad;
    const kz = sinK * Math.cos(this.sunOrbitIncline) * sunRad;
    const keyPos = new THREE.Vector3(cosK * sunRad, ky, kz);
    const kDir2 = keyPos.clone().normalize();

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(
      sDir,
    );
    ((this.cloud1.material as THREE.ShaderMaterial).uniforms as any).uKeyDir2.value.copy(
      kDir2,
    );
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(
      sDir,
    );
    ((this.cloud2.material as THREE.ShaderMaterial).uniforms as any).uKeyDir2.value.copy(
      kDir2,
    );

    (
      this.cloud1.material as THREE.ShaderMaterial
    ).uniforms.uSunIntensity.value = 0.5 * this.cloudBrightness;
    (
      this.cloud2.material as THREE.ShaderMaterial
    ).uniforms.uSunIntensity.value = 0.5 * this.cloudBrightness;

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uOpacity.value =
      cloud1Opacity;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uOpacity.value =
      cloud2Opacity;

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uCloudAmount.value =
      cloudAmount;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uCloudAmount.value =
      cloudAmount;

    this.hexLayers.forEach((l) => l.update(now * 0.001));
    this.atmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    (this.atmosphereMaterial.uniforms as any).uKeyDir2.value.copy(kDir2);
    this.sunAtmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(
      this.camera.position,
    );
    this.syncConfigSliders();

    // Keep debug lights moving with the same orbits.
    this.sunKeyLight.position.copy(this.sunLight.position);
    this.sunKeyLight2.position.copy(keyPos);

    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);

    this.updateMoonPosition(now);
  };

  private syncConfigSliders() {
    if (!this.configSliderMap.size) return;
    this.configSliderMap.forEach((entry) => {
      if (document.activeElement === entry.slider) return;
      const value = entry.getValue();
      entry.slider.value = `${value}`;
      entry.valueEl.textContent = entry.format(value);
    });
  }

  private updateCamera(anchor: THREE.Vector3) {
    const orbitX = Math.cos(this.cameraOrbit) * this.cameraDistance;
    const orbitZ = Math.sin(this.cameraOrbit) * this.cameraDistance;
    this.camera.position.set(anchor.x + orbitX, anchor.y, anchor.z + orbitZ);
    this.camera.lookAt(anchor);
    this.camera.rotation.y = this.cameraYaw;
  }

  private initCameraAnchor() {
    const now = performance.now();
    this.updateMoonPosition(now);
    this.cameraAnchor.copy(this.moon.position).multiplyScalar(0.5);
  }

  private updateMoonPosition(now: number) {
    // Moon orbit: 1/10th the speed of the sun (same timebase as sunA).
    const moonA = now * (this.sunOrbitSpeed / 10) + this.moonOrbitPhaseRad;
    const moonSinA = Math.sin(moonA);
    const moonCosA = Math.cos(moonA);
    const moonY = moonSinA * Math.sin(this.moonOrbitIncline) * this.moonOrbitRadius;
    const moonZ = moonSinA * Math.cos(this.moonOrbitIncline) * this.moonOrbitRadius;
    this.moon.position.set(moonCosA * this.moonOrbitRadius, moonY, moonZ);
  }

  setSunOrbitAngleRad(angleRad: number) {
    const offsetRad = angleRad - this.sunTimeMs * this.sunOrbitSpeed;
    this.sunOrbitAngleDeg = offsetRad / DEG_TO_RAD;
  }
  buildConfigSnapshot() {
    return {
      camera: {
        distance: this.cameraDistance,
        yaw: this.cameraYaw,
        orbit: this.cameraOrbit,
      },
      sun: { angle: this.sunOrbitAngleDeg, speed: this.sunOrbitSpeed },
    };
  }
}

export function mountEarth(container: HTMLElement) {
  // Inject HTML
  container.innerHTML = `
      <div class="marketing-overlay" aria-label="Unified Networks marketing information">
        <h2>Now is the time to learn and build</h2>
        <p data-typing-subtitle></p>
      </div>
      <div id="earth-config-panel" class="earth-config-panel" hidden></div>
    `;

  // Create and inject config toggle
  const controls = document.querySelector('.top-right-controls');
  const toggle = document.createElement('button');
  toggle.id = 'earth-config-toggle';
  toggle.className = 'earth-config-toggle';
  toggle.type = 'button';
  toggle.setAttribute('aria-expanded', 'false');
  toggle.textContent = 'Config';
  controls?.prepend(toggle);

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Connect math to real machines.",
    "Build robots, radios, and AI systems.",
    "Learn fast, deploy safely, iterate together.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const orbit = new ProceduralOrbit(container);
  return {
    dispose: () => {
      orbit.dispose();
      toggle.remove();
      stopTyping();
      container.innerHTML = '';
    },
    setVisible: (visible: boolean) => orbit.setVisible(visible),
  };
}
