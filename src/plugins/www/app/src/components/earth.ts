import * as THREE from 'three';
import { HexLayer } from './hex_layer';
import earthVertexShader from '../shaders/earth.vert.glsl?raw';
import earthFragmentShader from '../shaders/earth.frag.glsl?raw';
import cloudVertexShader from '../shaders/cloud.vert.glsl?raw';
import cloudFragmentShader from '../shaders/cloud.frag.glsl?raw';
import cloudIceFragmentShader from '../shaders/cloud_ice.frag.glsl?raw';
import atmosphereVertexShader from '../shaders/atmosphere.vert.glsl?raw';
import atmosphereFragmentShader from '../shaders/atmosphere.frag.glsl?raw';
import sunAtmosphereVertexShader from '../shaders/sun_atmosphere.vert.glsl?raw';
import sunAtmosphereFragmentShader from '../shaders/sun_atmosphere.frag.glsl?raw';

const DEG_TO_RAD = Math.PI / 180;
const TIME_SCALE = 1;

class ProceduralOrbit {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(75, 1, 0.01, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;

  earth!: THREE.Mesh;
  cloud1!: THREE.Mesh;
  cloud2!: THREE.Mesh;
  cloud3!: THREE.Mesh;
  cloud4!: THREE.Mesh;
  hexLayers: HexLayer[] = [];
  atmosphere!: THREE.Mesh;
  sunAtmosphere!: THREE.Mesh;
  issGroup!: THREE.Group;
  earthMaterial!: THREE.ShaderMaterial;
  cloud1Material!: THREE.ShaderMaterial;
  cloud2Material!: THREE.ShaderMaterial;
  cloud3Material!: THREE.ShaderMaterial;
  cloud4Material!: THREE.ShaderMaterial;
  atmosphereMaterial!: THREE.ShaderMaterial;
  sunAtmosphereMaterial!: THREE.ShaderMaterial;
  cloud1Axis = new THREE.Vector3(0, 1, 0);
  cloud2Axis = new THREE.Vector3(0.2, 1, -0.1).normalize();
  cloud3Axis = new THREE.Vector3(-0.1, 1, 0.2).normalize();
  cloud4Axis = new THREE.Vector3(0.3, 1, 0.05).normalize();

  // Settings
  earthRadius = 5;
  shaderTimeScale = 0.14;
  timeScale = TIME_SCALE;

  // Rotations
  orbitAngle = 0;
  orbitSpeed = 0.000214;
  earthRotSpeed = 0.000042;
  cloud1RotSpeed = 0.000082;
  cloud2RotSpeed = 0.000085;
  cloud3RotSpeed = 0.000031;
  cloud4RotSpeed = 0.000073;
  orbitHeightBase = 0.79;

  // Camera POI sequence
  poiSequence = [
    { offset: new THREE.Vector3(0, 0, 1.25), look: new THREE.Vector3(0, 0, 0), euler: new THREE.Euler(0, 0, 0) },
    { offset: new THREE.Vector3(0.5, 0.7, 1.5), look: new THREE.Vector3(0, 0.1, 0), euler: new THREE.Euler(-15 * DEG_TO_RAD, 10 * DEG_TO_RAD, 0) },
    { offset: new THREE.Vector3(-0.8, -0.4, 1.1), look: new THREE.Vector3(0.1, -0.1, 0), euler: new THREE.Euler(10 * DEG_TO_RAD, -25 * DEG_TO_RAD, 5 * DEG_TO_RAD) },
    { offset: new THREE.Vector3(1.2, 0.2, 0.8), look: new THREE.Vector3(-0.2, 0, 0), euler: new THREE.Euler(5 * DEG_TO_RAD, 40 * DEG_TO_RAD, -5 * DEG_TO_RAD) },
  ];
  currentPoiIndex = 0;
  nextPoiIndex = 1;
  phaseStartTime = performance.now();
  dwellDuration = 7000;
  transitionDuration = 4000;

  // Intermediate state for lerps
  cameraOffset = new THREE.Vector3().copy(this.poiSequence[0].offset);
  cameraLookTarget = new THREE.Vector3().copy(this.poiSequence[0].look);
  cameraEuler = new THREE.Euler().copy(this.poiSequence[0].euler);
  cameraOffsetWorld = new THREE.Vector3();
  cameraExtraQuat = new THREE.Quaternion();

  // Lights
  sunGlow!: THREE.Mesh;
  sunLight!: THREE.PointLight;
  sunKeyLight!: THREE.DirectionalLight;
  ambientLight!: THREE.AmbientLight;

  sunDistance = 78;
  sunOrbitHeight = 87;
  sunOrbitAngleDeg = 103;
  sunOrbitSpeed = 0.0005;

  keyLightDistance = 147;
  keyLightHeight = 40;
  keyLightAngleDeg = 63;
  materialColorScale = 1.25;

  lastFrameTime = performance.now();
  altitudeEl?: HTMLElement;
  speedEl?: HTMLElement;
  configPanel?: HTMLDivElement;
  configToggle?: HTMLButtonElement;
  configValueMap = new Map<string, HTMLSpanElement>();

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.container.appendChild(this.renderer.domElement);

    this.altitudeEl = document.querySelector('[data-telemetry="altitude"]') || undefined;
    this.speedEl = document.querySelector('[data-telemetry="speed"]') || undefined;

    this.initLayers();
    this.initISS();
    this.initLights();
    this.initConfigPanel();
    this.resize();
    this.animate();

    // @ts-ignore: Expose for testing
    window.earthDebug = this;

    if (typeof ResizeObserver !== 'undefined') {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    } else {
      window.addEventListener('resize', this.resize);
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
    window.removeEventListener('resize', this.resize);
    this.renderer.dispose();
    this.container.removeChild(this.renderer.domElement);
  }

  initLayers() {
    const geo = (r: number) => new THREE.SphereGeometry(r, 128, 128);

    const earthMat = new THREE.ShaderMaterial({
      uniforms: {
        uTime: { value: 0 },
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyIntensity: { value: 0.8 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: earthVertexShader,
      fragmentShader: earthFragmentShader
    });
    this.earthMaterial = earthMat;
    this.earth = new THREE.Mesh(geo(this.earthRadius), earthMat);
    this.scene.add(this.earth);

    const cloud1Mat = this.createCloudMaterial(0.2, 0.35);
    this.cloud1Material = cloud1Mat;
    this.cloud1 = new THREE.Mesh(geo(this.earthRadius + 0.05), cloud1Mat);
    this.scene.add(this.cloud1);

    const cloud2Mat = this.createCloudMaterial(0.5, 0.2);
    this.cloud2Material = cloud2Mat;
    this.cloud2 = new THREE.Mesh(geo(this.earthRadius + 0.08), cloud2Mat);
    this.scene.add(this.cloud2);

    const cloud3Mat = this.createCloudMaterial(0.9, 0.12);
    this.cloud3Material = cloud3Mat;
    this.cloud3 = new THREE.Mesh(geo(this.earthRadius + 0.12), cloud3Mat);
    this.scene.add(this.cloud3);

    const cloud4Mat = this.createCloudMaterial(
      1.4,
      0.2,
      new THREE.Color(0.65, 0.85, 1.0),
      cloudIceFragmentShader,
      { uGlow: { value: 0.35 } }
    );
    this.cloud4Material = cloud4Mat;
    this.cloud4 = new THREE.Mesh(geo(this.earthRadius + 0.18), cloud4Mat);
    this.scene.add(this.cloud4);

    this.hexLayers = [
      new HexLayer(this.earthRadius, {
        radiusOffset: 0.06,
        count: 420,
        resolution: 3,
        ratePerSecond: 100,
        durationSeconds: 3,
        palette: [
          new THREE.Color(0.85, 0.85, 0.86),
          new THREE.Color(0.65, 0.67, 0.7),
          new THREE.Color(0.1, 0.1, 0.12)
        ]
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 0.08,
        count: 380,
        resolution: 3,
        ratePerSecond: 100,
        durationSeconds: 3,
        palette: [
          new THREE.Color(0.75, 0.75, 0.76),
          new THREE.Color(0.45, 0.46, 0.5),
          new THREE.Color(0.05, 0.05, 0.07)
        ]
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 0.12,
        count: 340,
        resolution: 3,
        ratePerSecond: 100,
        durationSeconds: 3,
        palette: [
          new THREE.Color(0.9, 0.9, 0.9),
          new THREE.Color(0.55, 0.56, 0.6),
          new THREE.Color(0.15, 0.15, 0.18)
        ]
      })
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
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: atmosphereVertexShader,
      fragmentShader: atmosphereFragmentShader
    });
    this.atmosphereMaterial = atmoMat;
    this.atmosphere = new THREE.Mesh(geo(this.earthRadius + 0.2), atmoMat);
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
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: sunAtmosphereVertexShader,
      fragmentShader: sunAtmosphereFragmentShader
    });
    this.sunAtmosphereMaterial = sunAtmoMat;
    this.sunAtmosphere = new THREE.Mesh(geo(this.earthRadius + 0.32), sunAtmoMat);
    this.scene.add(this.sunAtmosphere);
  }

  createCloudMaterial(
    scale: number,
    opacity: number,
    tint: THREE.Color = new THREE.Color(1, 1, 1),
    fragmentShaderBase: string = cloudFragmentShader,
    extraUniforms: Record<string, THREE.IUniform> = {}
  ) {
    const fragmentShader = fragmentShaderBase
      .replace(/CLOUD_SCALE/g, scale.toFixed(2))
      .replace(/CLOUD_OPACITY/g, opacity.toFixed(2));
    return new THREE.ShaderMaterial({
      transparent: true,
      uniforms: {
        uTime: { value: 0 },
        uTint: { value: tint },
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyIntensity: { value: 0.8 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale },
        ...extraUniforms
      },
      vertexShader: cloudVertexShader,
      fragmentShader
    });
  }

  initISS() {
    this.issGroup = new THREE.Group();
    const body = new THREE.Mesh(
      new THREE.CylinderGeometry(0.02, 0.02, 0.15),
      new THREE.MeshStandardMaterial({ color: 0xcccccc })
    );
    body.rotation.z = Math.PI / 2;
    const panelGeo = new THREE.BoxGeometry(0.005, 0.08, 0.4);
    const panelMat = new THREE.MeshStandardMaterial({ color: 0x113366, metalness: 0.8, roughness: 0.2 });
    const leftP = new THREE.Mesh(panelGeo, panelMat);
    const rightP = leftP.clone();
    leftP.position.x = -0.1;
    rightP.position.x = 0.1;
    this.issGroup.add(body, leftP, rightP);
    this.scene.add(this.issGroup);
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
      new THREE.MeshBasicMaterial({ color: 0xffa63d })
    );
    this.scene.add(this.sunGlow);

    this.sunLight = new THREE.PointLight(0xffb347, 1.85, 200);
    this.scene.add(this.sunLight);
  }

  initConfigPanel() {
    const panel = document.getElementById('earth-config-panel') as HTMLDivElement | null;
    const toggle = document.getElementById('earth-config-toggle') as HTMLButtonElement | null;
    if (!panel || !toggle) return;

    this.configPanel = panel;
    this.configToggle = toggle;

    const setOpen = (open: boolean) => {
      panel.hidden = !open;
      panel.style.display = open ? 'grid' : 'none';
      toggle.setAttribute('aria-expanded', String(open));
    };

    setOpen(false);
    toggle.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      setOpen(panel.hidden);
    });

    const addSection = (title: string) => {
      const header = document.createElement('h3');
      header.textContent = title;
      panel.appendChild(header);
    };

    const addSlider = (key: string, label: string, value: number, min: number, max: number, step: number, onInput: (v: number) => void, format: (v: number) => string = (v) => v.toFixed(3)) => {
      const row = document.createElement('div');
      row.className = 'earth-config-row';
      const labelWrap = document.createElement('label');
      labelWrap.textContent = label;
      const slider = document.createElement('input');
      slider.type = 'range'; slider.min = `${min}`; slider.max = `${max}`; slider.step = `${step}`; slider.value = `${value}`;
      labelWrap.appendChild(slider);
      row.appendChild(labelWrap);
      const valueEl = document.createElement('span');
      valueEl.className = 'earth-config-value';
      valueEl.textContent = format(value);
      row.appendChild(valueEl);
      panel.appendChild(row);
      this.configValueMap.set(key, valueEl);
      slider.addEventListener('input', () => {
        const next = parseFloat(slider.value);
        onInput(next);
        valueEl.textContent = format(next);
      });
    };

    const addCopyButton = () => {
      const btn = document.createElement('button');
      btn.textContent = 'Copy Config';
      btn.addEventListener('click', () => {
        const payload = JSON.stringify(this.buildConfigSnapshot(), null, 2);
        navigator.clipboard?.writeText(payload);
      });
      panel.appendChild(btn);
    };

    addSection('Orbit');
    addSlider('orbitSpeed', 'Orbit Speed', this.orbitSpeed, 0, 0.005, 0.000001, (v) => this.orbitSpeed = v, (v) => v.toFixed(6));
    addSlider('orbitHeight', 'Orbit Height', this.orbitHeightBase, 0.05, 1.5, 0.01, (v) => this.orbitHeightBase = v);

    addSection('Rotation');
    addSlider('earthRot', 'Earth Rot', this.earthRotSpeed, 0, 0.0002, 0.000001, (v) => this.earthRotSpeed = v, (v) => v.toFixed(6));
    addSlider('sunOrbitSpeed', 'Sun Orbit', this.sunOrbitSpeed, 0, 0.005, 0.0001, (v) => this.sunOrbitSpeed = v, (v) => v.toFixed(4));

    addSection('Camera');
    addSlider('dwell', 'Dwell (ms)', this.dwellDuration, 1000, 15000, 100, (v) => this.dwellDuration = v, (v) => v.toFixed(0));
    addSlider('transition', 'Transition (ms)', this.transitionDuration, 1000, 10000, 100, (v) => this.transitionDuration = v, (v) => v.toFixed(0));

    addCopyButton();
  }

  updateTelemetry(orbitRadius: number) {
    const kmPerUnit = 6371 / this.earthRadius;
    const altitudeKm = (orbitRadius - this.earthRadius) * kmPerUnit;
    if (this.altitudeEl) this.altitudeEl.textContent = `${altitudeKm.toFixed(0)} KM`;
  }

  isVisible = true;
  setVisible(v: boolean) { this.isVisible = v; }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    const now = performance.now();
    const rawDelta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    const deltaSeconds = rawDelta * this.timeScale;
    const cloudTime = now * 0.001 * this.shaderTimeScale;

    // Rotations
    this.earth.rotation.y += this.earthRotSpeed * deltaSeconds;
    this.cloud1.rotateOnAxis(this.cloud1Axis, this.cloud1RotSpeed * rawDelta);
    this.cloud2.rotateOnAxis(this.cloud2Axis, this.cloud2RotSpeed * rawDelta);
    this.cloud3.rotateOnAxis(this.cloud3Axis, this.cloud3RotSpeed * rawDelta);
    this.cloud4.rotateOnAxis(this.cloud4Axis, this.cloud4RotSpeed * rawDelta);

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;
    (this.cloud3.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;
    (this.cloud4.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;

    // Orbit Position
    this.orbitAngle += this.orbitSpeed * deltaSeconds;
    const orbitRadius = this.earthRadius + this.orbitHeightBase;
    this.issGroup.position.set(Math.cos(this.orbitAngle) * orbitRadius, Math.sin(this.orbitAngle * 0.5) * 0.5, Math.sin(this.orbitAngle) * orbitRadius);
    this.issGroup.lookAt(Math.cos(this.orbitAngle + 0.01) * orbitRadius, Math.sin((this.orbitAngle + 0.01) * 0.5) * 0.5, Math.sin(this.orbitAngle + 0.01) * orbitRadius);

    // Camera Panning
    const elapsedPhase = now - this.phaseStartTime;
    const cycle = this.dwellDuration + this.transitionDuration;
    if (elapsedPhase > cycle) {
      this.currentPoiIndex = this.nextPoiIndex;
      this.nextPoiIndex = (this.currentPoiIndex + 1) % this.poiSequence.length;
      this.phaseStartTime = now;
    }

    const currentPOI = this.poiSequence[this.currentPoiIndex];
    const nextPOI = this.poiSequence[this.nextPoiIndex];

    if (elapsedPhase > this.dwellDuration) {
      const t = (elapsedPhase - this.dwellDuration) / this.transitionDuration;
      const ease = t * t * (3 - 2 * t);
      this.cameraOffset.lerpVectors(currentPOI.offset, nextPOI.offset, ease);
      this.cameraLookTarget.lerpVectors(currentPOI.look, nextPOI.look, ease);
      const qS = new THREE.Quaternion().setFromEuler(currentPOI.euler);
      const qE = new THREE.Quaternion().setFromEuler(nextPOI.euler);
      this.cameraExtraQuat.slerpQuaternions(qS, qE, ease);
      this.cameraEuler.setFromQuaternion(this.cameraExtraQuat);
    } else {
      this.cameraOffset.copy(currentPOI.offset);
      this.cameraLookTarget.copy(currentPOI.look);
      this.cameraEuler.copy(currentPOI.euler);
    }

    this.camera.position.copy(this.issGroup.position);
    this.cameraOffsetWorld.copy(this.cameraOffset).applyQuaternion(this.issGroup.quaternion);
    this.camera.position.add(this.cameraOffsetWorld);

    const target = this.cameraLookTarget.clone().applyQuaternion(this.issGroup.quaternion).add(this.issGroup.position);
    this.camera.lookAt(target);
    this.cameraExtraQuat.setFromEuler(this.cameraEuler);
    this.camera.quaternion.multiply(this.cameraExtraQuat);

    // Sun Orbit
    const sunRad = this.earthRadius + this.sunOrbitHeight;
    const sunA = now * this.sunOrbitSpeed + this.sunOrbitAngleDeg * DEG_TO_RAD;
    this.sunLight.position.set(Math.cos(sunA) * sunRad, Math.sin(sunA * 0.5) * 5, Math.sin(sunA) * sunRad);
    this.sunGlow.position.copy(this.sunLight.position);

    // Shaders
    const sDir = this.sunLight.position.clone().normalize();
    this.earthMaterial.uniforms.uSunDir.value.copy(sDir);
    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(sDir);
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(sDir);
    (this.cloud3.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(sDir);
    (this.cloud4.material as THREE.ShaderMaterial).uniforms.uSunDir.value.copy(sDir);

    this.hexLayers.forEach(l => l.update(now * 0.001));
    this.atmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(this.camera.position);

    this.renderer.render(this.scene, this.camera);
    this.updateTelemetry(orbitRadius);
  };

  buildConfigSnapshot() {
    return {
      camera: { index: this.currentPoiIndex, offset: this.cameraOffset },
      sun: { angle: this.sunOrbitAngleDeg, speed: this.sunOrbitSpeed }
    };
  }
}

export function mountEarth(container: HTMLElement) {
  const orbit = new ProceduralOrbit(container);
  return {
    dispose: () => orbit.dispose(),
    setVisible: (v: boolean) => orbit.setVisible(v),
  };
}
