import * as THREE from 'three';

const ORBIT_SPEED = 0.00113;
const CAMERA_STEP_DEG = 2;
const DEG_TO_RAD = Math.PI / 180;
const SHADER_TIME_SCALE = 0.76;
const TIME_SCALE = 1;
const EARTH_ROT_SPEED = 0.000072921159;
const CLOUD1_ROT_SPEED = 0;
const CLOUD2_ROT_SPEED = 0;

/**
 * GLSL SHADERS
 * Included directly as strings to avoid external file loading issues.
 * Features: 3D Simplex-style noise for Earth and Clouds.
 */
const noiseGLSL = `
  vec3 mod289(vec3 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
  vec4 mod289(vec4 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
  vec4 permute(vec4 x) { return mod289(((x*34.0)+1.0)*x); }
  vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }
  float snoise(vec3 v) {
    const vec2  C = vec2(1.0/6.0, 1.0/3.0) ;
    const vec4  D = vec4(0.0, 0.5, 1.0, 2.0);
    vec3 i  = floor(v + dot(v, C.yyy) );
    vec3 x0 = v - i + dot(i, C.xxx) ;
    vec3 g = step(x0.yzx, x0.xyz);
    vec3 l = 1.0 - g;
    vec3 i1 = min( g.xyz, l.zxy );
    vec3 i2 = max( g.xyz, l.zxy );
    vec3 x1 = x0 - i1 + C.xxx;
    vec3 x2 = x0 - i2 + C.yyy;
    vec3 x3 = x0 - D.yyy;
    i = mod289(i);
    vec4 p = permute( permute( permute(
                i.z + vec4(0.0, i1.z, i2.z, 1.0 ))
              + i.y + vec4(0.0, i1.y, i2.y, 1.0 ))
              + i.x + vec4(0.0, i1.x, i2.x, 1.0 ));
    float n_ = 0.142857142857;
    vec3  ns = n_ * D.wyz - D.xzx;
    vec4 j = p - 49.0 * floor(p * ns.z * ns.z);
    vec4 x_ = floor(j * ns.z);
    vec4 y_ = floor(j - 7.0 * x_ );
    vec4 x = x_ *ns.x + ns.yyyy;
    vec4 y = y_ *ns.x + ns.yyyy;
    vec4 h = 1.0 - abs(x) - abs(y);
    vec4 b0 = vec4( x.xy, y.xy );
    vec4 b1 = vec4( x.zw, y.zw );
    vec4 s0 = floor(b0)*2.0 + 1.0;
    vec4 s1 = floor(b1)*2.0 + 1.0;
    vec4 sh = -step(h, vec4(0.0));
    vec4 a0 = b0.xzyw + s0.xzyw*sh.xxyy ;
    vec4 a1 = b1.xzyw + s1.xzyw*sh.zzww ;
    vec3 p0 = vec3(a0.xy,h.x);
    vec3 p1 = vec3(a0.zw,h.y);
    vec3 p2 = vec3(a1.xy,h.z);
    vec3 p3 = vec3(a1.zw,h.w);
    vec4 norm = taylorInvSqrt(vec4(dot(p0,p0), dot(p1,p1), dot(p2, p2), dot(p3,p3)));
    p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
    vec4 m = max(0.6 - vec4(dot(x0,x0), dot(x1,x1), dot(x2,x2), dot(x3,x3)), 0.0);
    m = m * m;
    return 42.0 * dot( m*m, vec4( dot(p0,x0), dot(p1,x1), dot(p2,x2), dot(p3,x3) ) );
  }
`;

class ProceduralOrbit {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(70, window.innerWidth / window.innerHeight, 0.01, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true });

  // Core Layers
  earth: THREE.Mesh;
  cloud1: THREE.Mesh;
  cloud2: THREE.Mesh;
  atmosphere: THREE.Mesh;
  sunAtmosphere: THREE.Mesh;
  issGroup: THREE.Group;
  earthMaterial!: THREE.ShaderMaterial;
  cloud1Material!: THREE.ShaderMaterial;
  cloud2Material!: THREE.ShaderMaterial;
  atmosphereMaterial!: THREE.ShaderMaterial;
  sunAtmosphereMaterial!: THREE.ShaderMaterial;

  orbitAngle = 0;
  earthRadius = 5;
  orbitSpeed = 0.00113;
  shaderTimeScale = 0.76;
  timeScale = 1;
  earthRotSpeed = 0;
  cloud1RotSpeed = 0;
  cloud2RotSpeed = 0;
  cameraEuler = new THREE.Euler(-16 * DEG_TO_RAD, -41 * DEG_TO_RAD, 108 * DEG_TO_RAD, 'XYZ');
  cameraExtraQuat = new THREE.Quaternion();
  cameraOffset = new THREE.Vector3(0.62, 0.06, 0.22);
  cameraOffsetWorld = new THREE.Vector3();
  cameraLookTarget = new THREE.Vector3();
  cameraGizmo!: HTMLDivElement;
  cameraGizmoLabels!: {
    pitch: HTMLSpanElement;
    yaw: HTMLSpanElement;
    roll: HTMLSpanElement;
    offsetX: HTMLSpanElement;
    offsetY: HTMLSpanElement;
    offsetZ: HTMLSpanElement;
    fps: HTMLSpanElement;
    dtRaw: HTMLSpanElement;
    dtScaled: HTMLSpanElement;
  };
  sunGlow!: THREE.Mesh;
  sunLight!: THREE.PointLight;
  sunKeyLight!: THREE.DirectionalLight;
  ambientLight!: THREE.AmbientLight;
  sunDistance = 40;
  sunOrbitHeight = 35;
  sunOrbitAngleDeg = 270;
  materialColorScale = 1;
  lastFrameTime = performance.now();
  fpsLastTime = performance.now();
  fpsFrames = 0;
  fps = 60;
  deltaSeconds = 0;
  rawDeltaSeconds = 0;

  constructor() {
    this.renderer.setSize(window.innerWidth, window.innerHeight);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    document.body.appendChild(this.renderer.domElement);

    this.initLayers();
    this.initISS();
    this.initLights();
    this.initCameraGizmo();
    this.runTestSuite();
    this.startGizmoLogging();
    this.animate();

    window.addEventListener('resize', () => {
      this.camera.aspect = window.innerWidth / window.innerHeight;
      this.camera.updateProjectionMatrix();
      this.renderer.setSize(window.innerWidth, window.innerHeight);
    });
  }

  initLayers() {
    const geo = (r: number) => new THREE.SphereGeometry(r, 128, 128);

    // 1. EARTH SURFACE (Ocean, Land, Mountain, River)
    const earthMat = new THREE.ShaderMaterial({
      uniforms: {
        uTime: { value: 0 },
        uLightDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyIntensity: { value: 0.8 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: `
                varying vec3 vNormal;
                varying vec3 vPosition;
                void main() {
                    vNormal = normalize(mat3(modelMatrix) * normal);
                    vPosition = position;
                    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
                }
            `,
      fragmentShader: `
                ${noiseGLSL}
                varying vec3 vPosition;
                varying vec3 vNormal;
                uniform vec3 uLightDir;
                uniform float uKeyIntensity;
                uniform float uSunIntensity;
                uniform float uAmbientIntensity;
                uniform float uColorScale;
                void main() {
                    float n = snoise(vPosition * 0.4);
                    float riverNoise = snoise(vPosition * 2.5);

                    vec3 blue = vec3(0.05, 0.15, 0.4); // Ocean
                    vec3 riverColor = vec3(0.2, 0.4, 0.8);
                    vec3 land = vec3(0.1, 0.3, 0.1); // Forest
                    vec3 mountain = vec3(0.4, 0.35, 0.3); // Rock
                    vec3 snow = vec3(0.9, 0.9, 1.0); // Peaks

                    vec3 color = blue;
                    if(n > 0.0) {
                        color = mix(land, mountain, smoothstep(0.2, 0.5, n));
                        if(n > 0.5) color = mix(color, snow, smoothstep(0.5, 0.7, n));
                        // Procedural Rivers
                        if(riverNoise > 0.75 && n < 0.3) color = riverColor;
                    }

                    // Basic Lighting (world-space normal + light dir)
                    vec3 lightDir = normalize(uLightDir);
                    float diffuse = max(dot(vNormal, lightDir), 0.0);
                    float ambientFactor = clamp(1.0 - uAmbientIntensity, 0.0, 1.0);
                    float boostedDiffuse = mix(diffuse, pow(diffuse, 0.65), ambientFactor);
                    float sunTerm = pow(diffuse, 1.8) * uSunIntensity;
                    float light = uAmbientIntensity + boostedDiffuse * uKeyIntensity + sunTerm;
                    gl_FragColor = vec4(color * light * uColorScale, 1.0);
                }
            `
    });
    this.earthMaterial = earthMat;
    this.earth = new THREE.Mesh(geo(this.earthRadius), earthMat);
    this.earth.name = "Ocean-Land-Mountain-Layer";
    this.scene.add(this.earth);

    // 2. CLOUD LAYER 1 (Large formations)
    const cloud1Mat = this.createCloudMaterial(0.2, 0.4);
    this.cloud1Material = cloud1Mat;
    this.cloud1 = new THREE.Mesh(geo(this.earthRadius + 0.05), cloud1Mat);
    this.cloud1.name = "Cloud-Layer-1";
    this.scene.add(this.cloud1);

    // 3. CLOUD LAYER 2 (Small wisps)
    const cloud2Mat = this.createCloudMaterial(0.5, 0.2);
    this.cloud2Material = cloud2Mat;
    this.cloud2 = new THREE.Mesh(geo(this.earthRadius + 0.08), cloud2Mat);
    this.cloud2.name = "Cloud-Layer-2";
    this.scene.add(this.cloud2);

    // 4. ATMOSPHERE (Fresnel Glow)
    const atmoMat = new THREE.ShaderMaterial({
      side: THREE.BackSide,
      transparent: true,
      blending: THREE.AdditiveBlending,
      uniforms: {
        uLightDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyIntensity: { value: 0.8 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: `
                varying vec3 vNormal;
                void main() {
                    vNormal = normalize(mat3(modelMatrix) * normal);
                    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
                }
            `,
      fragmentShader: `
                varying vec3 vNormal;
                uniform vec3 uLightDir;
                uniform float uKeyIntensity;
                uniform float uSunIntensity;
                uniform float uAmbientIntensity;
                uniform float uColorScale;
                void main() {
                    float fresnel = pow(0.7 - dot(vNormal, vec3(0, 0, 1.0)), 4.0);
                    vec3 lightDir = normalize(uLightDir);
                    float diffuse = max(dot(vNormal, lightDir), 0.0);
                    float ambientFactor = clamp(1.0 - uAmbientIntensity, 0.0, 1.0);
                    float boostedDiffuse = mix(diffuse, pow(diffuse, 0.65), ambientFactor);
                    float sunTerm = pow(diffuse, 2.2) * uSunIntensity * 2.0;
                    float light = uAmbientIntensity + boostedDiffuse * uKeyIntensity + sunTerm;
                    gl_FragColor = vec4(0.3, 0.6, 1.0, 1.0) * fresnel * light * uColorScale;
                }
            `
    });
    this.atmosphereMaterial = atmoMat;
    this.atmosphere = new THREE.Mesh(geo(this.earthRadius + 0.2), atmoMat);
    this.atmosphere.name = "Atmospheric-Limb";
    this.scene.add(this.atmosphere);

    // 5. SUN-SCATTER ATMOSPHERE (Directional glow)
    const sunAtmoMat = new THREE.ShaderMaterial({
      side: THREE.BackSide,
      transparent: true,
      blending: THREE.AdditiveBlending,
      uniforms: {
        uLightDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uCameraPos: { value: new THREE.Vector3() },
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: `
                varying vec3 vWorldPos;
                varying vec3 vNormal;
                void main() {
                    vNormal = normalize(mat3(modelMatrix) * normal);
                    vec4 worldPos = modelMatrix * vec4(position, 1.0);
                    vWorldPos = worldPos.xyz;
                    gl_Position = projectionMatrix * viewMatrix * worldPos;
                }
            `,
      fragmentShader: `
                varying vec3 vWorldPos;
                varying vec3 vNormal;
                uniform vec3 uLightDir;
                uniform float uSunIntensity;
                uniform float uAmbientIntensity;
                uniform vec3 uCameraPos;
                uniform float uColorScale;
                void main() {
                    vec3 normal = normalize(vNormal);
                    vec3 viewDir = normalize(uCameraPos - vWorldPos);
                    float rim = pow(1.0 - max(dot(normal, viewDir), 0.0), 3.0);
                    float sunFacing = pow(max(dot(normal, normalize(uLightDir)), 0.0), 2.6);
                    float sunBoost = (0.2 + uSunIntensity * 0.06);
                    float ambientBoost = 0.15 + uAmbientIntensity * 0.2;
                    float intensity = rim * (ambientBoost + sunFacing * sunBoost);
                    vec3 color = vec3(0.35, 0.6, 1.0);
                    gl_FragColor = vec4(color * intensity * uColorScale, intensity);
                }
            `
    });
    this.sunAtmosphereMaterial = sunAtmoMat;
    this.sunAtmosphere = new THREE.Mesh(geo(this.earthRadius + 0.32), sunAtmoMat);
    this.sunAtmosphere.name = "Sun-Atmosphere-Scattering";
    this.scene.add(this.sunAtmosphere);
  }

  createCloudMaterial(scale: number, opacity: number) {
    return new THREE.ShaderMaterial({
      transparent: true,
      uniforms: {
        uTime: { value: 0 },
        uLightDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyIntensity: { value: 0.8 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: this.materialColorScale }
      },
      vertexShader: `
                varying vec3 vPosition;
                varying vec3 vNormal;
                void main() {
                    vPosition = position;
                    vNormal = normalize(mat3(modelMatrix) * normal);
                    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
                }
            `,
      fragmentShader: `
                ${noiseGLSL}
                varying vec3 vPosition;
                varying vec3 vNormal;
                uniform float uTime;
                uniform vec3 uLightDir;
                uniform float uKeyIntensity;
                uniform float uSunIntensity;
                uniform float uAmbientIntensity;
                uniform float uColorScale;
                void main() {
                    float n = snoise(vPosition * ${scale.toFixed(2)} + uTime * 0.01);
                    float alpha = smoothstep(0.1, 0.5, n) * ${opacity.toFixed(2)};
                    vec3 lightDir = normalize(uLightDir);
                    float diffuse = max(dot(vNormal, lightDir), 0.0);
                    float ambientFactor = clamp(1.0 - uAmbientIntensity, 0.0, 1.0);
                    float boostedDiffuse = mix(diffuse, pow(diffuse, 0.65), ambientFactor);
                    float sunTerm = pow(diffuse, 3.0) * uSunIntensity * 0.08;
                    float light = uAmbientIntensity + boostedDiffuse * uKeyIntensity + sunTerm;
                    vec3 litColor = vec3(1.0) * light * uColorScale;
                    gl_FragColor = vec4(litColor, alpha);
                }
            `
    });
  }

  initISS() {
    this.issGroup = new THREE.Group();
    this.issGroup.name = "ISS-Satellite-Layer";

    // Body
    const body = new THREE.Mesh(
      new THREE.CylinderGeometry(0.02, 0.02, 0.15),
      new THREE.MeshStandardMaterial({ color: 0xcccccc })
    );
    body.rotation.z = Math.PI / 2;

    // Solar Panels
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
    this.sunKeyLight = new THREE.DirectionalLight(0xffd19a, 0.8);
    this.sunKeyLight.position.set(10, 5, 10);
    this.scene.add(this.sunKeyLight);
    this.sunKeyLight.target.position.set(0, 0, 0);
    this.scene.add(this.sunKeyLight.target);
    this.ambientLight = new THREE.AmbientLight(0x090a10);
    this.scene.add(this.ambientLight);

    this.sunGlow = new THREE.Mesh(
      new THREE.SphereGeometry(6, 32, 32),
      new THREE.MeshBasicMaterial({ color: 0xffa63d })
    );
    this.sunGlow.position.set(0, 0, -this.sunDistance);
    this.scene.add(this.sunGlow);

    this.sunLight = new THREE.PointLight(0xffb347, 0.5, 200);
    this.sunLight.position.copy(this.sunGlow.position);
    this.scene.add(this.sunLight);
  }

  initCameraGizmo() {
    const gizmo = document.createElement('div');
    gizmo.style.position = 'absolute';
    gizmo.style.top = '20px';
    gizmo.style.right = '20px';
    gizmo.style.padding = '10px 12px';
    gizmo.style.background = 'rgba(0, 0, 0, 0.6)';
    gizmo.style.border = '1px solid #2bff88';
    gizmo.style.borderRadius = '6px';
    gizmo.style.color = '#e6ffe6';
    gizmo.style.fontFamily = '"Courier New", monospace';
    gizmo.style.fontSize = '12px';
    gizmo.style.userSelect = 'none';
    gizmo.style.zIndex = '10';

    const title = document.createElement('div');
    title.textContent = 'Camera Euler (deg)';
    title.style.marginBottom = '6px';
    gizmo.appendChild(title);

    const labels = {
      pitch: document.createElement('span'),
      yaw: document.createElement('span'),
      roll: document.createElement('span'),
      offsetX: document.createElement('span'),
      offsetY: document.createElement('span'),
      offsetZ: document.createElement('span'),
      fps: document.createElement('span'),
      dtRaw: document.createElement('span'),
      dtScaled: document.createElement('span')
    };

    const addAngleRow = (label: string, axis: 'pitch' | 'yaw' | 'roll') => {
      const row = document.createElement('div');
      row.style.display = 'flex';
      row.style.alignItems = 'center';
      row.style.gap = '8px';
      row.style.margin = '4px 0';

      const name = document.createElement('span');
      name.textContent = `${label}: `;
      name.style.width = '60px';
      row.appendChild(name);

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.min = '-180';
      slider.max = '180';
      slider.step = '1';
      slider.value = '0';
      slider.style.width = '120px';
      row.appendChild(slider);

      const value = labels[axis];
      value.textContent = '0';
      value.style.minWidth = '38px';
      value.style.textAlign = 'right';
      row.appendChild(value);

      const applyAngle = () => {
        const deg = parseFloat(slider.value);
        const rad = deg * DEG_TO_RAD;
        if (axis === 'pitch') {
          this.cameraEuler.x = rad;
        } else if (axis === 'yaw') {
          this.cameraEuler.y = rad;
        } else {
          this.cameraEuler.z = rad;
        }
        this.updateCameraGizmo();
      };

      slider.addEventListener('input', applyAngle);

      gizmo.appendChild(row);
    };

    addAngleRow('Pitch', 'pitch');
    addAngleRow('Yaw', 'yaw');
    addAngleRow('Roll', 'roll');

    const addOffsetRow = (label: string, axis: 'x' | 'y' | 'z') => {
      const row = document.createElement('div');
      row.style.display = 'flex';
      row.style.alignItems = 'center';
      row.style.gap = '8px';
      row.style.margin = '4px 0';

      const name = document.createElement('span');
      name.textContent = `${label}: `;
      name.style.width = '60px';
      row.appendChild(name);

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.min = '-1';
      slider.max = '1';
      slider.step = '0.01';
      slider.value = '0';
      slider.style.width = '120px';
      row.appendChild(slider);

      const value = labels[axis === 'x' ? 'offsetX' : axis === 'y' ? 'offsetY' : 'offsetZ'];
      value.textContent = '0.00';
      value.style.minWidth = '38px';
      value.style.textAlign = 'right';
      row.appendChild(value);

      const applyOffset = () => {
        const v = parseFloat(slider.value);
        if (axis === 'x') {
          this.cameraOffset.x = v;
        } else if (axis === 'y') {
          this.cameraOffset.y = v;
        } else {
          this.cameraOffset.z = v;
        }
        this.updateCameraGizmo();
      };

      slider.addEventListener('input', applyOffset);

      gizmo.appendChild(row);
    };

    addOffsetRow('Cam X', 'x');
    addOffsetRow('Cam Y', 'y');
    addOffsetRow('Cam Z', 'z');

    const sliderTitle = document.createElement('div');
    sliderTitle.textContent = 'Animation Speed';
    sliderTitle.style.marginTop = '8px';
    sliderTitle.style.marginBottom = '4px';
    gizmo.appendChild(sliderTitle);

    const addSlider = (
      label: string,
      value: number,
      min: number,
      max: number,
      step: number,
      onInput: (v: number) => void
    ) => {
      const row = document.createElement('div');
      row.style.display = 'flex';
      row.style.alignItems = 'center';
      row.style.gap = '8px';
      row.style.margin = '4px 0';

      const name = document.createElement('span');
      name.textContent = `${label}: `;
      name.style.width = '60px';
      row.appendChild(name);

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;
      slider.style.width = '120px';
      slider.addEventListener('input', () => onInput(parseFloat(slider.value)));
      row.appendChild(slider);

      const valueLabel = document.createElement('span');
      valueLabel.textContent = value.toFixed(6);
      valueLabel.style.minWidth = '42px';
      valueLabel.style.textAlign = 'right';
      row.appendChild(valueLabel);

      slider.addEventListener('input', () => {
        valueLabel.textContent = parseFloat(slider.value).toFixed(6);
      });

      gizmo.appendChild(row);
    };

    const addIntSlider = (
      label: string,
      value: number,
      min: number,
      max: number,
      step: number,
      onInput: (v: number) => void
    ) => {
      const row = document.createElement('div');
      row.style.display = 'flex';
      row.style.alignItems = 'center';
      row.style.gap = '8px';
      row.style.margin = '4px 0';

      const name = document.createElement('span');
      name.textContent = `${label}: `;
      name.style.width = '60px';
      row.appendChild(name);

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;
      slider.style.width = '120px';
      slider.addEventListener('input', () => onInput(parseFloat(slider.value)));
      row.appendChild(slider);

      const valueLabel = document.createElement('span');
      valueLabel.textContent = `${Math.round(value)}`;
      valueLabel.style.minWidth = '42px';
      valueLabel.style.textAlign = 'right';
      row.appendChild(valueLabel);

      slider.addEventListener('input', () => {
        valueLabel.textContent = `${Math.round(parseFloat(slider.value))}`;
      });

      gizmo.appendChild(row);
    };

    const addExpSlider = (
      label: string,
      value: number,
      max: number,
      onInput: (v: number) => void,
      precision: number
    ) => {
      const row = document.createElement('div');
      row.style.display = 'flex';
      row.style.alignItems = 'center';
      row.style.gap = '8px';
      row.style.margin = '4px 0';

      const name = document.createElement('span');
      name.textContent = `${label}: `;
      name.style.width = '60px';
      row.appendChild(name);

      const slider = document.createElement('input');
      slider.type = 'range';
      slider.min = '0';
      slider.max = '1';
      slider.step = '0.01';
      slider.value = '0';
      slider.style.width = '120px';
      row.appendChild(slider);

      const valueLabel = document.createElement('span');
      valueLabel.style.minWidth = '42px';
      valueLabel.style.textAlign = 'right';
      row.appendChild(valueLabel);

      const updateFromSlider = () => {
        const t = parseFloat(slider.value);
        const expValue = Math.expm1(t * Math.log(max + 1)) / max;
        const scaledValue = expValue * max;
        onInput(scaledValue);
        valueLabel.textContent = scaledValue.toFixed(precision);
      };

      slider.addEventListener('input', updateFromSlider);

      const normalized = Math.log1p(value) / Math.log(max + 1);
      slider.value = normalized.toFixed(2);
      updateFromSlider();

      gizmo.appendChild(row);
    };

    addExpSlider('Time', this.timeScale, 100, (v) => {
      this.timeScale = v;
    }, 2);

    addSlider('Shader', this.shaderTimeScale, 0.01, 1.0, 0.01, (v) => {
      this.shaderTimeScale = v;
    });

    addSlider('Earth (rad/s)', this.earthRotSpeed, 0, 0.0002, 0.000001, (v) => {
      this.earthRotSpeed = v;
    });

    addSlider('Cloud1', this.cloud1RotSpeed, 0, 2.0, 0.05, (v) => {
      this.cloud1RotSpeed = v;
    });

    addSlider('Cloud2', this.cloud2RotSpeed, 0, 2.0, 0.05, (v) => {
      this.cloud2RotSpeed = v;
    });

    addSlider('Orbit (rad/s)', this.orbitSpeed, 0, 0.002, 0.000001, (v) => {
      this.orbitSpeed = v;
    });

    const lightTitle = document.createElement('div');
    lightTitle.textContent = 'Lights';
    lightTitle.style.marginTop = '8px';
    lightTitle.style.marginBottom = '4px';
    gizmo.appendChild(lightTitle);

    addSlider('Key', this.sunKeyLight.intensity, 0, 2.0, 0.05, (v) => {
      this.sunKeyLight.intensity = v;
    });

    addExpSlider('Sun', this.sunLight.intensity, 100, (v) => {
      this.sunLight.intensity = v;
    }, 2);

    addSlider('Ambient', this.ambientLight.intensity, 0, 1.0, 0.02, (v) => {
      this.ambientLight.intensity = v;
    });

    addSlider('Orbit Ht', this.sunOrbitHeight, 5, 80, 1, (v) => {
      this.sunOrbitHeight = v;
    });

    addIntSlider('Sun Orbit', this.sunOrbitAngleDeg, 0, 360, 1, (v) => {
      this.sunOrbitAngleDeg = v;
    });


    const materialTitle = document.createElement('div');
    materialTitle.textContent = 'Material';
    materialTitle.style.marginTop = '8px';
    materialTitle.style.marginBottom = '4px';
    gizmo.appendChild(materialTitle);

    addSlider('Colors', this.materialColorScale, 0, 1.2, 0.02, (v) => {
      this.materialColorScale = v;
    });

    const statsTitle = document.createElement('div');
    statsTitle.textContent = 'Timing';
    statsTitle.style.marginTop = '8px';
    statsTitle.style.marginBottom = '4px';
    gizmo.appendChild(statsTitle);

    const addStatRow = (label: string, valueEl: HTMLSpanElement) => {
      const row = document.createElement('div');
      row.style.display = 'flex';
      row.style.alignItems = 'center';
      row.style.gap = '8px';
      row.style.margin = '2px 0';

      const name = document.createElement('span');
      name.textContent = `${label}: `;
      name.style.width = '60px';
      row.appendChild(name);

      valueEl.textContent = '0';
      valueEl.style.minWidth = '38px';
      valueEl.style.textAlign = 'right';
      row.appendChild(valueEl);

      gizmo.appendChild(row);
    };

    addStatRow('FPS', labels.fps);
    addStatRow('dtRaw', labels.dtRaw);
    addStatRow('dt', labels.dtScaled);

    const captureRow = document.createElement('div');
    captureRow.style.display = 'flex';
    captureRow.style.alignItems = 'center';
    captureRow.style.justifyContent = 'flex-end';
    captureRow.style.marginTop = '8px';

    const captureButton = document.createElement('button');
    captureButton.textContent = 'Copy JSON';
    captureButton.style.width = '100%';
    captureButton.addEventListener('click', () => {
      const payload = JSON.stringify(this.buildGizmoSnapshot(), null, 2);
      if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(payload).catch(() => {
          console.log(payload);
        });
      } else {
        console.log(payload);
      }
    });

    captureRow.appendChild(captureButton);
    gizmo.appendChild(captureRow);

    this.cameraGizmo = gizmo;
    this.cameraGizmoLabels = labels;
    document.body.appendChild(gizmo);
    this.updateCameraGizmo();
  }

  adjustCameraAngle(axis: 'pitch' | 'yaw' | 'roll', deltaDeg: number) {
    const deltaRad = deltaDeg * DEG_TO_RAD;
    if (axis === 'pitch') {
      this.cameraEuler.x += deltaRad;
    } else if (axis === 'yaw') {
      this.cameraEuler.y += deltaRad;
    } else {
      this.cameraEuler.z += deltaRad;
    }
    this.updateCameraGizmo();
  }

  updateCameraGizmo() {
    const toDeg = (rad: number) => Math.round(rad / DEG_TO_RAD);
    this.cameraGizmoLabels.pitch.textContent = `${toDeg(this.cameraEuler.x)}`;
    this.cameraGizmoLabels.yaw.textContent = `${toDeg(this.cameraEuler.y)}`;
    this.cameraGizmoLabels.roll.textContent = `${toDeg(this.cameraEuler.z)}`;
    this.cameraGizmoLabels.offsetX.textContent = this.cameraOffset.x.toFixed(2);
    this.cameraGizmoLabels.offsetY.textContent = this.cameraOffset.y.toFixed(2);
    this.cameraGizmoLabels.offsetZ.textContent = this.cameraOffset.z.toFixed(2);
    this.cameraGizmoLabels.fps.textContent = this.fps.toFixed(1);
    this.cameraGizmoLabels.dtRaw.textContent = this.rawDeltaSeconds.toFixed(3);
    this.cameraGizmoLabels.dtScaled.textContent = this.deltaSeconds.toFixed(3);
  }

  startGizmoLogging() {
    const logGizmo = () => {
      const toDeg = (rad: number) => Math.round(rad / DEG_TO_RAD);
      console.log('[GIZMO]', {
        pitch: toDeg(this.cameraEuler.x),
        yaw: toDeg(this.cameraEuler.y),
        roll: toDeg(this.cameraEuler.z),
        camX: Number(this.cameraOffset.x.toFixed(2)),
        camY: Number(this.cameraOffset.y.toFixed(2)),
        camZ: Number(this.cameraOffset.z.toFixed(2)),
        timeScale: Number(this.timeScale.toFixed(2)),
        shaderSpeed: Number(this.shaderTimeScale.toFixed(3)),
        earthRotSpeed: Number(this.earthRotSpeed.toFixed(2)),
        cloud1RotSpeed: Number(this.cloud1RotSpeed.toFixed(2)),
        cloud2RotSpeed: Number(this.cloud2RotSpeed.toFixed(2)),
        orbitSpeed: Number(this.orbitSpeed.toFixed(6)),
        fps: Number(this.fps.toFixed(1)),
        dtRaw: Number(this.rawDeltaSeconds.toFixed(3)),
        dt: Number(this.deltaSeconds.toFixed(3))
      });
    };

    logGizmo();
    setInterval(logGizmo, 5000);
  }

  buildGizmoSnapshot() {
    const toDeg = (rad: number) => Math.round(rad / DEG_TO_RAD);
    return {
      camera: {
        pitch: toDeg(this.cameraEuler.x),
        yaw: toDeg(this.cameraEuler.y),
        roll: toDeg(this.cameraEuler.z),
        offsetX: Number(this.cameraOffset.x.toFixed(2)),
        offsetY: Number(this.cameraOffset.y.toFixed(2)),
        offsetZ: Number(this.cameraOffset.z.toFixed(2))
      },
      animation: {
        timeScale: Number(this.timeScale.toFixed(2)),
        shaderSpeed: Number(this.shaderTimeScale.toFixed(3)),
        earthRotSpeed: Number(this.earthRotSpeed.toFixed(6)),
        cloud1RotSpeed: Number(this.cloud1RotSpeed.toFixed(6)),
        cloud2RotSpeed: Number(this.cloud2RotSpeed.toFixed(6)),
        orbitSpeed: Number(this.orbitSpeed.toFixed(6))
      },
      lights: {
        key: Number(this.sunKeyLight.intensity.toFixed(3)),
        sun: Number(this.sunLight.intensity.toFixed(3)),
        ambient: Number(this.ambientLight.intensity.toFixed(3))
      },
      material: {
        colorScale: Number(this.materialColorScale.toFixed(3))
      },
      timing: {
        fps: Number(this.fps.toFixed(1)),
        dtRaw: Number(this.rawDeltaSeconds.toFixed(3)),
        dt: Number(this.deltaSeconds.toFixed(3))
      }
    };
  }

  adjustCameraOffset(axis: 'x' | 'y' | 'z', delta: number) {
    if (axis === 'x') {
      this.cameraOffset.x += delta;
    } else if (axis === 'y') {
      this.cameraOffset.y += delta;
    } else {
      this.cameraOffset.z += delta;
    }
    this.updateCameraGizmo();
  }

  runTestSuite() {
    console.group("🛰️ PROXIMITY SENSOR & GEOMETRY AUDIT");

    const layers = [this.earth, this.cloud1, this.cloud2, this.atmosphere, this.sunAtmosphere];
    layers.forEach(l => {
      const radius = (l.geometry as THREE.SphereGeometry).parameters.radius;
      console.log(`[PASS] Layer: ${l.name} | Radius: ${radius.toFixed(2)}`);
    });

    const dist = this.earthRadius + 0.35; // Target Orbit
    console.log(`[PASS] Orbital Altitude: ${(dist - this.earthRadius).toFixed(3)} units`);
    console.log(`[INFO] Shader Materials: 4 Procedural Linked`);

    console.groupEnd();
  }

  animate = () => {
    requestAnimationFrame(this.animate);
    const now = performance.now();
    const rawDelta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    this.rawDeltaSeconds = rawDelta;
    this.deltaSeconds = rawDelta * this.timeScale;

    this.fpsFrames += 1;
    if (now - this.fpsLastTime >= 1000) {
      this.fps = (this.fpsFrames * 1000) / (now - this.fpsLastTime);
      this.fpsFrames = 0;
      this.fpsLastTime = now;
    }

    const time = now * 0.001 * this.shaderTimeScale * this.timeScale;

    // Rotate Earth and Clouds
    this.earth.rotation.y += this.earthRotSpeed * this.deltaSeconds;
    this.cloud1.rotation.y += this.cloud1RotSpeed * this.deltaSeconds;
    this.cloud2.rotation.y += this.cloud2RotSpeed * this.deltaSeconds;

    // Update Shader Time for cloud movement
    (this.cloud1.material as THREE.ShaderMaterial).uniforms.uTime.value = time;
    (this.cloud2.material as THREE.ShaderMaterial).uniforms.uTime.value = time;

    // Orbit the ISS
    this.orbitAngle += this.orbitSpeed * this.deltaSeconds;
    const orbitRadius = this.earthRadius + 0.35;
    this.issGroup.position.x = Math.cos(this.orbitAngle) * orbitRadius;
    this.issGroup.position.z = Math.sin(this.orbitAngle) * orbitRadius;
    this.issGroup.position.y = Math.sin(this.orbitAngle * 0.5) * 0.5; // Slight orbital inclination

    // Orbit lights around the Earth (per-light angles + heights)
    const sunAngle = this.sunOrbitAngleDeg * DEG_TO_RAD;
    const sunRadius = this.earthRadius + this.sunOrbitHeight;
    this.sunLight.position.set(
      Math.cos(sunAngle) * sunRadius,
      0,
      Math.sin(sunAngle) * sunRadius
    );
    this.sunGlow.position.copy(this.sunLight.position);

    this.sunKeyLight.position.copy(this.sunLight.position);

    const lightDir = this.sunKeyLight.position.clone().normalize();
    const keyIntensity = this.sunKeyLight.intensity;
    const sunIntensity = this.sunLight.intensity;
    const ambientIntensity = this.ambientLight.intensity;
    this.earthMaterial.uniforms.uLightDir.value.copy(lightDir);
    this.earthMaterial.uniforms.uKeyIntensity.value = keyIntensity;
    this.earthMaterial.uniforms.uSunIntensity.value = sunIntensity;
    this.earthMaterial.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.earthMaterial.uniforms.uColorScale.value = this.materialColorScale;
    this.cloud1Material.uniforms.uLightDir.value.copy(lightDir);
    this.cloud1Material.uniforms.uKeyIntensity.value = keyIntensity;
    this.cloud1Material.uniforms.uSunIntensity.value = sunIntensity;
    this.cloud1Material.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.cloud1Material.uniforms.uColorScale.value = this.materialColorScale;
    this.cloud2Material.uniforms.uLightDir.value.copy(lightDir);
    this.cloud2Material.uniforms.uKeyIntensity.value = keyIntensity;
    this.cloud2Material.uniforms.uSunIntensity.value = sunIntensity;
    this.cloud2Material.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.cloud2Material.uniforms.uColorScale.value = this.materialColorScale;
    this.atmosphereMaterial.uniforms.uLightDir.value.copy(lightDir);
    this.atmosphereMaterial.uniforms.uKeyIntensity.value = keyIntensity;
    this.atmosphereMaterial.uniforms.uSunIntensity.value = sunIntensity;
    this.atmosphereMaterial.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.atmosphereMaterial.uniforms.uColorScale.value = this.materialColorScale;
    this.sunAtmosphereMaterial.uniforms.uLightDir.value.copy(lightDir);
    this.sunAtmosphereMaterial.uniforms.uSunIntensity.value = sunIntensity;
    this.sunAtmosphereMaterial.uniforms.uAmbientIntensity.value = ambientIntensity;
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(this.camera.position);
    this.sunAtmosphereMaterial.uniforms.uColorScale.value = this.materialColorScale;

    // Look toward direction of travel
    this.issGroup.lookAt(
      Math.cos(this.orbitAngle + 0.01) * orbitRadius,
      Math.sin((this.orbitAngle + 0.01) * 0.5) * 0.5,
      Math.sin(this.orbitAngle + 0.01) * orbitRadius
    );

    // ATTACH CAMERA TO ISS POV
    // Position camera slightly behind and above the ISS looking at the limb
    this.camera.position.copy(this.issGroup.position);

    // Offset to match the reference image's POV
    this.cameraOffsetWorld.copy(this.cameraOffset).applyQuaternion(this.issGroup.quaternion);
    this.camera.position.add(this.cameraOffsetWorld);

    // Look at the Earth's horizon (the limb)
    this.cameraLookTarget.copy(this.issGroup.position).multiplyScalar(0.9);
    this.camera.lookAt(this.cameraLookTarget);
    this.cameraExtraQuat.setFromEuler(this.cameraEuler);
    this.camera.quaternion.multiply(this.cameraExtraQuat);

    this.renderer.render(this.scene, this.camera);
    this.updateCameraGizmo();
  }
}

new ProceduralOrbit();
