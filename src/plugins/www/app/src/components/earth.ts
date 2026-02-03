import * as THREE from "three";
import { HexLayer } from "./hex_layer";
import earthVertexShader from "../shaders/earth.vert.glsl?raw";
import earthFragmentShader from "../shaders/earth.frag.glsl?raw";
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

const DEG_TO_RAD = Math.PI / 180;
const TIME_SCALE = 1;
const SUN_ORBIT_PERIOD_MS = 5000;

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
  earthMaterial!: THREE.ShaderMaterial;
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
  cloudAmount = 0.75;

  // Rotations
  earthRotSpeed = 0.000042;
  cloud1RotSpeed = (Math.PI * 2) / 100;
  cloud2RotSpeed = (Math.PI * 2) / 120;
  cloud1Opacity = 0.35;
  cloud2Opacity = 0.2;
  cloudBrightness = 1.0;
  cameraDistance = 4.5;
  cameraOffsetX = 5.0;
  cameraYaw = 1;

  // Lights
  sunGlow!: THREE.Mesh;
  sunLight!: THREE.PointLight;
  sunKeyLight!: THREE.DirectionalLight;
  ambientLight!: THREE.AmbientLight;

  sunDistance = 78;
  sunOrbitHeight = 87;
  sunOrbitAngleDeg = 0;
  sunOrbitSpeed = (Math.PI * 2) / SUN_ORBIT_PERIOD_MS / 2;
  sunOrbitIncline = 20 * DEG_TO_RAD;

  keyLightDistance = 147;
  keyLightHeight = 40;
  keyLightAngleDeg = 63;
  materialColorScale = 1.25;

  lastFrameTime = performance.now();
  configPanel?: HTMLDivElement;
  configToggle?: HTMLButtonElement;
  configValueMap = new Map<string, HTMLSpanElement>();
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
    this.initLights();
    this.initConfigPanel();
    this.resize();
    this.camera.position.set(this.cameraOffsetX, 0, this.cameraDistance);
    this.camera.lookAt(0, 0, 0);
    this.camera.rotation.y += this.cameraYaw;
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

    const earthMat = new THREE.ShaderMaterial({
      uniforms: {
        uTime: { value: 0 },
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyIntensity: { value: 0.8 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale },
      },
      vertexShader: earthVertexShader,
      fragmentShader: earthFragmentShader,
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
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyIntensity: { value: 0.8 },
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
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyIntensity: { value: 0.8 },
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
    this.sunKeyLight = new THREE.DirectionalLight(0xffd19a, 0.3);
    this.sunKeyLight.position.set(10, 5, 10);
    this.scene.add(this.sunKeyLight);
    this.sunKeyLight.target.position.set(0, 0, 0);
    this.scene.add(this.sunKeyLight.target);
    this.ambientLight = new THREE.AmbientLight(0x090a10, 0.26);
    this.scene.add(this.ambientLight);

    this.sunGlow = new THREE.Mesh(
      new THREE.SphereGeometry(6, 32, 32),
      new THREE.MeshBasicMaterial({ color: 0xffa63d }),
    );
    this.scene.add(this.sunGlow);

    const hemiLight = new THREE.HemisphereLight(0xffffff, 0x111111, 1.0);
    this.scene.add(hemiLight);
    this.sunLight = new THREE.PointLight(0xffb347, 1.85, 200);
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

    // Rotations
    this.earth.rotation.y += this.earthRotSpeed * deltaSeconds;
    this.cloud1.rotateOnAxis(this.cloud1Axis, this.cloud1RotSpeed * rawDelta);
    this.cloud2.rotateOnAxis(this.cloud2Axis, this.cloud2RotSpeed * rawDelta);

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uTime.value =
      cloudTime;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uTime.value =
      cloudTime;

    this.camera.position.set(this.cameraOffsetX, 0, this.cameraDistance);
    this.camera.lookAt(0, 0, 0);
    this.camera.rotation.y += this.cameraYaw;

    // Sun Orbit
    const sunRad = this.earthRadius + this.sunOrbitHeight;
    const sunA = now * this.sunOrbitSpeed + this.sunOrbitAngleDeg * DEG_TO_RAD;
    const sinA = Math.sin(sunA);
    const cosA = Math.cos(sunA);
    const y = sinA * Math.sin(this.sunOrbitIncline) * sunRad;
    const z = sinA * Math.cos(this.sunOrbitIncline) * sunRad;
    this.sunLight.position.set(cosA * sunRad, y, z);
    this.sunGlow.position.copy(this.sunLight.position);

    const sDir = this.sunLight.position.clone().normalize();
    this.earthMaterial.uniforms.uSunDir.value.copy(sDir);
    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(
      sDir,
    );
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(
      sDir,
    );

    (
      this.cloud1.material as THREE.ShaderMaterial
    ).uniforms.uSunIntensity.value = 0.5 * this.cloudBrightness;
    (
      this.cloud2.material as THREE.ShaderMaterial
    ).uniforms.uSunIntensity.value = 0.5 * this.cloudBrightness;

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uOpacity.value =
      this.cloud1Opacity;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uOpacity.value =
      this.cloud2Opacity;

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uCloudAmount.value =
      this.cloudAmount;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uCloudAmount.value =
      this.cloudAmount;

    this.hexLayers.forEach((l) => l.update(now * 0.001));
    this.atmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(
      this.camera.position,
    );

    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };

  buildConfigSnapshot() {
    return {
      camera: { distance: this.cameraDistance },
      sun: { angle: this.sunOrbitAngleDeg, speed: this.sunOrbitSpeed },
    };
  }
}

export function mountEarth(container: HTMLElement) {
  // Inject HTML
  container.innerHTML = `
      <div class="marketing-overlay" aria-label="Unified Networks marketing information">
        <h2>Now is the time to learn and build</h2>
        <p>Robotics is moving from labs to the planet. Learn the math, build the software, and connect the machines.</p>
        <a class="buy-button" href="https://buy.stripe.com/test_5kQaEXcagaAoaC62N20kE00" target="_blank"
          rel="noopener noreferrer">Get the Robot Kit</a>
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

  const orbit = new ProceduralOrbit(container);
  return {
    dispose: () => {
      orbit.dispose();
      toggle.remove();
      container.innerHTML = '';
    },
    setVisible: (visible: boolean) => orbit.setVisible(visible),
  };
}
