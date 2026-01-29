import * as THREE from 'three';
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
  camera = new THREE.PerspectiveCamera(90, 1, 0.01, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;

  earth!: THREE.Mesh;
  cloud1!: THREE.Mesh;
  cloud2!: THREE.Mesh;
  cloud3!: THREE.Mesh;
  cloud4!: THREE.Mesh;
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

  orbitAngle = 0;
  earthRadius = 5;
  orbitSpeed = 0.000214;
  shaderTimeScale = 0.14;
  timeScale = TIME_SCALE;
  earthRotSpeed = 0.000042;
  cloud1RotSpeed = 0.000082;
  cloud2RotSpeed = 0.000085;
  cloud3RotSpeed = 0.000031;
  cloud4RotSpeed = 0.000073;
  orbitHeightBase = 0.79;
  orbitHeightOsc = 0.13;
  orbitHeightSpeed = 0.00015;
  timeOscSpeed = 0.00028;
  cameraEuler = new THREE.Euler(160 * DEG_TO_RAD, -180 * DEG_TO_RAD, 62 * DEG_TO_RAD, 'XYZ');
  cameraExtraQuat = new THREE.Quaternion();
  cameraOffset = new THREE.Vector3(0.83, 0.25, 0.82);
  cameraOffsetWorld = new THREE.Vector3();
  cameraLookTarget = new THREE.Vector3();
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
    this.camera.aspect = width / height;
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
    this.sunGlow.position.set(0, 0, -this.sunDistance);
    this.scene.add(this.sunGlow);

    this.sunLight = new THREE.PointLight(0xffb347, 1.85, 200);
    this.sunLight.position.copy(this.sunGlow.position);
    this.scene.add(this.sunLight);
  }

  initConfigPanel() {
    const panel = document.getElementById('earth-config-panel') as HTMLDivElement | null;
    const toggle = document.getElementById('earth-config-toggle') as HTMLButtonElement | null;
    if (!panel || !toggle) {
      return;
    }

    this.configPanel = panel;
    this.configToggle = toggle;

    const setOpen = (open: boolean) => {
      panel.hidden = !open;
      panel.style.display = open ? 'grid' : 'none';
      toggle.setAttribute('aria-expanded', String(open));
    };

    setOpen(false);
    toggle.addEventListener('click', (event) => {
      event.preventDefault();
      event.stopPropagation();
      setOpen(panel.hidden);
    });

    const addSection = (title: string) => {
      const header = document.createElement('h3');
      header.textContent = title;
      panel.appendChild(header);
    };

    const addSlider = (
      key: string,
      label: string,
      value: number,
      min: number,
      max: number,
      step: number,
      onInput: (next: number) => void,
      format: (val: number) => string = (val) => val.toFixed(3)
    ) => {
      const row = document.createElement('div');
      row.className = 'earth-config-row';

      const labelWrap = document.createElement('label');
      labelWrap.textContent = label;

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;

      labelWrap.appendChild(slider);
      row.appendChild(labelWrap);

      const valueEl = document.createElement('span');
      valueEl.className = 'earth-config-value';
      valueEl.textContent = format(value);
      row.appendChild(valueEl);
      panel.appendChild(row);
      this.configValueMap.set(key, valueEl);

      slider.addEventListener('input', () => {
        const nextValue = parseFloat(slider.value);
        onInput(nextValue);
        valueEl.textContent = format(nextValue);
      });
    };

    const addCopyButton = () => {
      const button = document.createElement('button');
      button.type = 'button';
      button.textContent = 'Copy Config';
      button.addEventListener('click', () => {
        const payload = JSON.stringify(this.buildConfigSnapshot(), null, 2);
        if (navigator.clipboard?.writeText) {
          navigator.clipboard.writeText(payload).catch(() => {
            console.log(payload);
          });
        } else {
          console.log(payload);
        }
      });
      panel.appendChild(button);
    };

    addSection('Orbit');
    addSlider('orbitSpeed', 'Orbit Speed', this.orbitSpeed, 0, 0.005, 0.000001, (v) => {
      this.orbitSpeed = v;
    }, (v) => v.toFixed(6));
    addSlider('orbitHeightBase', 'Orbit Base', this.orbitHeightBase, 0.05, 1.5, 0.01, (v) => {
      this.orbitHeightBase = v;
    }, (v) => v.toFixed(2));
    addSlider('orbitHeightOsc', 'Orbit Osc', this.orbitHeightOsc, 0, 0.5, 0.01, (v) => {
      this.orbitHeightOsc = v;
    }, (v) => v.toFixed(2));
    addSlider('orbitHeightSpeed', 'Orbit Osc Spd', this.orbitHeightSpeed, 0, 0.005, 0.00001, (v) => {
      this.orbitHeightSpeed = v;
    }, (v) => v.toFixed(5));
    addSlider('timeOscSpeed', 'Time Osc Spd', this.timeOscSpeed, 0, 0.002, 0.00001, (v) => {
      this.timeOscSpeed = v;
    }, (v) => v.toFixed(5));

    addSection('Rotation');
    addSlider('earthRotSpeed', 'Earth Rot', this.earthRotSpeed, 0, 0.0002, 0.000001, (v) => {
      this.earthRotSpeed = v;
    }, (v) => v.toFixed(6));
    addSlider('cloud1RotSpeed', 'Cloud1 Rot', this.cloud1RotSpeed, 0, 0.0001, 0.000001, (v) => {
      this.cloud1RotSpeed = v;
    }, (v) => v.toFixed(6));
    addSlider('cloud2RotSpeed', 'Cloud2 Rot', this.cloud2RotSpeed, 0, 0.0001, 0.000001, (v) => {
      this.cloud2RotSpeed = v;
    }, (v) => v.toFixed(6));
    addSlider('cloud3RotSpeed', 'Cloud3 Rot', this.cloud3RotSpeed, 0, 0.0001, 0.000001, (v) => {
      this.cloud3RotSpeed = v;
    }, (v) => v.toFixed(6));
    addSlider('cloud4RotSpeed', 'Cloud4 Rot', this.cloud4RotSpeed, 0, 0.0001, 0.000001, (v) => {
      this.cloud4RotSpeed = v;
    }, (v) => v.toFixed(6));
    addSlider('shaderTimeScale', 'Shader Time', this.shaderTimeScale, 0.1, 2, 0.01, (v) => {
      this.shaderTimeScale = v;
    }, (v) => v.toFixed(2));

    addSection('Camera');
    addSlider('cameraPitch', 'Pitch', this.cameraEuler.x / DEG_TO_RAD, -180, 180, 1, (v) => {
      this.cameraEuler.x = v * DEG_TO_RAD;
    }, (v) => `${Math.round(v)}°`);
    addSlider('cameraYaw', 'Yaw', this.cameraEuler.y / DEG_TO_RAD, -180, 180, 1, (v) => {
      this.cameraEuler.y = v * DEG_TO_RAD;
    }, (v) => `${Math.round(v)}°`);
    addSlider('cameraRoll', 'Roll', this.cameraEuler.z / DEG_TO_RAD, -180, 180, 1, (v) => {
      this.cameraEuler.z = v * DEG_TO_RAD;
    }, (v) => `${Math.round(v)}°`);
    addSlider('cameraOffsetX', 'Offset X', this.cameraOffset.x, -2, 2, 0.01, (v) => {
      this.cameraOffset.x = v;
    }, (v) => v.toFixed(2));
    addSlider('cameraOffsetY', 'Offset Y', this.cameraOffset.y, -2, 2, 0.01, (v) => {
      this.cameraOffset.y = v;
    }, (v) => v.toFixed(2));
    addSlider('cameraOffsetZ', 'Offset Z', this.cameraOffset.z, -2, 2, 0.01, (v) => {
      this.cameraOffset.z = v;
    }, (v) => v.toFixed(2));

    addSection('Sun Orbit');
    addSlider('sunDistance', 'Sun Distance', this.sunDistance, 10, 100, 1, (v) => {
      this.sunDistance = v;
    }, (v) => v.toFixed(0));
    addSlider('sunOrbitHeight', 'Orbit Height', this.sunOrbitHeight, 0, 100, 1, (v) => {
      this.sunOrbitHeight = v;
    }, (v) => v.toFixed(0));
    addSlider('sunOrbitAngleDeg', 'Orbit Angle', this.sunOrbitAngleDeg, 0, 360, 1, (v) => {
      this.sunOrbitAngleDeg = v;
    }, (v) => `${Math.round(v)}°`);
    addSlider('sunOrbitSpeed', 'Orbit Speed', this.sunOrbitSpeed, 0, 0.01, 0.0001, (v) => {
      this.sunOrbitSpeed = v;
    }, (v) => v.toFixed(4));

    addSection('Key Light');
    addSlider('keyLightDistance', 'Key Distance', this.keyLightDistance, 5, 150, 1, (v) => {
      this.keyLightDistance = v;
    }, (v) => v.toFixed(0));
    addSlider('keyLightHeight', 'Key Height', this.keyLightHeight, -50, 50, 1, (v) => {
      this.keyLightHeight = v;
    }, (v) => v.toFixed(0));
    addSlider('keyLightAngleDeg', 'Key Angle', this.keyLightAngleDeg, 0, 360, 1, (v) => {
      this.keyLightAngleDeg = v;
    }, (v) => `${Math.round(v)}°`);

    addSection('Light');
    addSlider('sunKeyLight', 'Key Light', this.sunKeyLight.intensity, 0, 2, 0.05, (v) => {
      this.sunKeyLight.intensity = v;
    }, (v) => v.toFixed(2));
    addSlider('sunLight', 'Sun Light', this.sunLight.intensity, 0, 2, 0.05, (v) => {
      this.sunLight.intensity = v;
    }, (v) => v.toFixed(2));
    addSlider('ambientLight', 'Ambient', this.ambientLight.intensity, 0, 1, 0.02, (v) => {
      this.ambientLight.intensity = v;
    }, (v) => v.toFixed(2));

    addSection('Material');
    addSlider('materialColorScale', 'Color Scale', this.materialColorScale, 0.5, 1.5, 0.01, (v) => {
      this.materialColorScale = v;
    }, (v) => v.toFixed(2));

    addCopyButton();
  }

  getOscillatingTimeScale(now: number) {
    if (this.timeOscSpeed <= 0) {
      return this.timeScale;
    }
    const osc = (Math.sin(now * this.timeOscSpeed) + 1) / 2;
    let bias = Math.pow(osc, 4);
    if (osc > 0.985) {
      bias = 1;
    }
    const scaled = bias * 100;
    return Math.max(0.05, Math.min(100, scaled));
  }

  updateTelemetry(orbitRadius: number, timeScaleValue: number) {
    const kmPerUnit = 6371 / this.earthRadius;
    const altitudeKm = (orbitRadius - this.earthRadius) * kmPerUnit;
    const speedKmPerSec = this.orbitSpeed * timeScaleValue * orbitRadius * kmPerUnit;
    if (this.altitudeEl) {
      this.altitudeEl.textContent = `${altitudeKm.toFixed(0)} KM`;
    }
    if (this.speedEl) {
      this.speedEl.textContent = `${speedKmPerSec.toFixed(2)} KM/S`;
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    const now = performance.now();
    const rawDelta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    const timeScaleValue = this.getOscillatingTimeScale(now);
    this.timeScale = timeScaleValue;
    const deltaSeconds = rawDelta * this.timeScale;
    const cloudTime = now * 0.001 * this.shaderTimeScale;

    this.earth.rotation.y += this.earthRotSpeed * deltaSeconds;
    this.cloud1.rotateOnAxis(this.cloud1Axis, this.cloud1RotSpeed * rawDelta);
    this.cloud2.rotateOnAxis(this.cloud2Axis, this.cloud2RotSpeed * rawDelta);
    this.cloud3.rotateOnAxis(this.cloud3Axis, this.cloud3RotSpeed * rawDelta);
    this.cloud4.rotateOnAxis(this.cloud4Axis, this.cloud4RotSpeed * rawDelta);

    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;
    (this.cloud3.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;
    (this.cloud4.material as THREE.ShaderMaterial).uniforms.uTime.value = cloudTime;

    this.orbitAngle += this.orbitSpeed * deltaSeconds;
    const orbitHeight = this.orbitHeightBase + Math.sin(now * this.orbitHeightSpeed) * this.orbitHeightOsc;
    const orbitRadius = this.earthRadius + orbitHeight;
    this.issGroup.position.x = Math.cos(this.orbitAngle) * orbitRadius;
    this.issGroup.position.z = Math.sin(this.orbitAngle) * orbitRadius;
    this.issGroup.position.y = Math.sin(this.orbitAngle * 0.5) * 0.5;

    this.issGroup.lookAt(
      Math.cos(this.orbitAngle + 0.01) * orbitRadius,
      Math.sin((this.orbitAngle + 0.01) * 0.5) * 0.5,
      Math.sin(this.orbitAngle + 0.01) * orbitRadius
    );

    this.camera.position.copy(this.issGroup.position);
    this.cameraOffsetWorld.copy(this.cameraOffset).applyQuaternion(this.issGroup.quaternion);
    this.camera.position.add(this.cameraOffsetWorld);
    this.cameraLookTarget.copy(this.issGroup.position).multiplyScalar(0.9);
    this.camera.lookAt(this.cameraLookTarget);
    this.cameraExtraQuat.setFromEuler(this.cameraEuler);
    this.camera.quaternion.multiply(this.cameraExtraQuat);

    const sunRadius = this.earthRadius + this.sunOrbitHeight;
    const orbitAngle = now * this.sunOrbitSpeed + this.sunOrbitAngleDeg * DEG_TO_RAD;
    const sunX = Math.cos(orbitAngle) * sunRadius;
    const sunZ = Math.sin(orbitAngle) * sunRadius;
    const sunY = Math.sin(orbitAngle * 0.5) * 0.5;
    this.sunLight.position.set(sunX, sunY, sunZ);
    this.sunGlow.position.copy(this.sunLight.position);

    const keyAngle = this.keyLightAngleDeg * DEG_TO_RAD;
    const keyRadius = this.earthRadius + this.keyLightDistance;
    const keyX = Math.cos(keyAngle) * keyRadius;
    const keyZ = Math.sin(keyAngle) * keyRadius;
    this.sunKeyLight.position.set(keyX, this.keyLightHeight, keyZ);

    const sunDir = this.sunLight.position.clone().normalize();
    const keyDir = this.sunKeyLight.position.clone().normalize();
    const keyIntensity = this.sunKeyLight.intensity;
    const sunIntensity = this.sunLight.intensity;
    const ambientIntensity = this.ambientLight.intensity;
    this.earthMaterial.uniforms.uSunDir.value.copy(sunDir);
    this.earthMaterial.uniforms.uKeyDir.value.copy(keyDir);
    this.earthMaterial.uniforms.uKeyIntensity.value = keyIntensity;
    this.earthMaterial.uniforms.uSunIntensity.value = sunIntensity;
    this.earthMaterial.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.earthMaterial.uniforms.uColorScale.value = this.materialColorScale;
    this.cloud1Material.uniforms.uSunDir.value.copy(sunDir);
    this.cloud1Material.uniforms.uKeyDir.value.copy(keyDir);
    this.cloud1Material.uniforms.uKeyIntensity.value = keyIntensity;
    this.cloud1Material.uniforms.uSunIntensity.value = sunIntensity;
    this.cloud1Material.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.cloud1Material.uniforms.uColorScale.value = this.materialColorScale;
    this.cloud2Material.uniforms.uSunDir.value.copy(sunDir);
    this.cloud2Material.uniforms.uKeyDir.value.copy(keyDir);
    this.cloud2Material.uniforms.uKeyIntensity.value = keyIntensity;
    this.cloud2Material.uniforms.uSunIntensity.value = sunIntensity;
    this.cloud2Material.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.cloud2Material.uniforms.uColorScale.value = this.materialColorScale;
    this.cloud3Material.uniforms.uSunDir.value.copy(sunDir);
    this.cloud3Material.uniforms.uKeyDir.value.copy(keyDir);
    this.cloud3Material.uniforms.uKeyIntensity.value = keyIntensity;
    this.cloud3Material.uniforms.uSunIntensity.value = sunIntensity;
    this.cloud3Material.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.cloud3Material.uniforms.uColorScale.value = this.materialColorScale;
    this.cloud4Material.uniforms.uSunDir.value.copy(sunDir);
    this.cloud4Material.uniforms.uKeyDir.value.copy(keyDir);
    this.cloud4Material.uniforms.uKeyIntensity.value = keyIntensity;
    this.cloud4Material.uniforms.uSunIntensity.value = sunIntensity;
    this.cloud4Material.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.cloud4Material.uniforms.uColorScale.value = this.materialColorScale;
    this.atmosphereMaterial.uniforms.uSunDir.value.copy(sunDir);
    this.atmosphereMaterial.uniforms.uKeyDir.value.copy(keyDir);
    this.atmosphereMaterial.uniforms.uKeyIntensity.value = keyIntensity;
    this.atmosphereMaterial.uniforms.uSunIntensity.value = sunIntensity;
    this.atmosphereMaterial.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.atmosphereMaterial.uniforms.uColorScale.value = this.materialColorScale;
    this.sunAtmosphereMaterial.uniforms.uSunDir.value.copy(sunDir);
    this.sunAtmosphereMaterial.uniforms.uSunIntensity.value = sunIntensity;
    this.sunAtmosphereMaterial.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(this.camera.position);
    this.sunAtmosphereMaterial.uniforms.uColorScale.value = this.materialColorScale;

    this.renderer.render(this.scene, this.camera);
    this.updateTelemetry(orbitRadius, timeScaleValue);
  };

  buildConfigSnapshot() {
    const toDeg = (rad: number) => Math.round(rad / DEG_TO_RAD);
    return {
      orbit: {
        speed: Number(this.orbitSpeed.toFixed(6)),
        heightBase: Number(this.orbitHeightBase.toFixed(3)),
        heightOsc: Number(this.orbitHeightOsc.toFixed(3)),
        heightSpeed: Number(this.orbitHeightSpeed.toFixed(6)),
        timeOscSpeed: Number(this.timeOscSpeed.toFixed(6))
      },
      rotation: {
        earth: Number(this.earthRotSpeed.toFixed(9)),
        cloud1: Number(this.cloud1RotSpeed.toFixed(9)),
        cloud2: Number(this.cloud2RotSpeed.toFixed(9)),
        cloud3: Number(this.cloud3RotSpeed.toFixed(9)),
        cloud4: Number(this.cloud4RotSpeed.toFixed(9)),
        shaderTimeScale: Number(this.shaderTimeScale.toFixed(3))
      },
      camera: {
        pitch: toDeg(this.cameraEuler.x),
        yaw: toDeg(this.cameraEuler.y),
        roll: toDeg(this.cameraEuler.z),
        offsetX: Number(this.cameraOffset.x.toFixed(3)),
        offsetY: Number(this.cameraOffset.y.toFixed(3)),
        offsetZ: Number(this.cameraOffset.z.toFixed(3))
      },
      sun: {
        distance: Number(this.sunDistance.toFixed(2)),
        orbitHeight: Number(this.sunOrbitHeight.toFixed(2)),
        orbitAngleDeg: Number(this.sunOrbitAngleDeg.toFixed(1)),
        orbitSpeed: Number(this.sunOrbitSpeed.toFixed(5))
      },
      keyLight: {
        distance: Number(this.keyLightDistance.toFixed(2)),
        height: Number(this.keyLightHeight.toFixed(2)),
        angleDeg: Number(this.keyLightAngleDeg.toFixed(1))
      },
      light: {
        key: Number(this.sunKeyLight.intensity.toFixed(3)),
        sun: Number(this.sunLight.intensity.toFixed(3)),
        ambient: Number(this.ambientLight.intensity.toFixed(3))
      },
      material: {
        colorScale: Number(this.materialColorScale.toFixed(3))
      }
    };
  }
}

export function mountEarth(container: HTMLElement) {
  const orbit = new ProceduralOrbit(container);
  return () => orbit.dispose();
}
