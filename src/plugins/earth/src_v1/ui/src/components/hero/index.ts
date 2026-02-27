import { VisualizationControl } from '@ui/types';
import * as THREE from 'three';
import { polygonToCells } from 'h3-js';
import { HexLayer } from '../../earth/hex_layer';
import cloudVertexShader from '../../shaders/cloud.vert.glsl?raw';
import cloudFragmentShader from '../../shaders/cloud.frag.glsl?raw';
import atmosphereVertexShader from '../../shaders/atmosphere.vert.glsl?raw';
import atmosphereFragmentShader from '../../shaders/atmosphere.frag.glsl?raw';
import sunAtmosphereVertexShader from '../../shaders/sun_atmosphere.vert.glsl?raw';
import sunAtmosphereFragmentShader from '../../shaders/sun_atmosphere.frag.glsl?raw';

const DEG_TO_RAD = Math.PI / 180;
const SUN_COLOR = new THREE.Color(1.0, 1.0, 1.0);
const KEY1_COLOR = new THREE.Color(0.9, 0.95, 1.0);
const KEY2_COLOR = new THREE.Color(0.85, 0.9, 1.0);
const MOON_LIGHT_LAYER = 1;

function createMoonRockTexture(size = 128): THREE.CanvasTexture {
  const canvas = document.createElement('canvas');
  canvas.width = size;
  canvas.height = size;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Failed to create texture context');

  ctx.fillStyle = '#7a7a7a';
  ctx.fillRect(0, 0, size, size);
  const img = ctx.getImageData(0, 0, size, size);
  const d = img.data;
  for (let i = 0; i < d.length; i += 4) {
    const n = (Math.random() * 2 - 1) * 24;
    d[i + 0] = Math.max(0, Math.min(255, d[i + 0] + n));
    d[i + 1] = Math.max(0, Math.min(255, d[i + 1] + n));
    d[i + 2] = Math.max(0, Math.min(255, d[i + 2] + n));
    d[i + 3] = 255;
  }
  ctx.putImageData(img, 0, 0);

  for (let i = 0; i < 22; i += 1) {
    const x = Math.random() * size;
    const y = Math.random() * size;
    const r = 4 + Math.random() * 14;
    const g = ctx.createRadialGradient(x, y, r * 0.2, x, y, r);
    g.addColorStop(0, 'rgba(40,40,40,0.35)');
    g.addColorStop(0.6, 'rgba(90,90,90,0.10)');
    g.addColorStop(1, 'rgba(120,120,120,0.00)');
    ctx.fillStyle = g;
    ctx.beginPath();
    ctx.arc(x, y, r, 0, Math.PI * 2);
    ctx.fill();
  }

  const tex = new THREE.CanvasTexture(canvas);
  tex.wrapS = tex.wrapT = THREE.RepeatWrapping;
  tex.repeat.set(2, 2);
  tex.colorSpace = THREE.SRGBColorSpace;
  tex.needsUpdate = true;
  return tex;
}

class HeroControl implements VisualizationControl {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(75, 1, 0.1, 10000);
  renderer: THREE.WebGLRenderer;
  frameId = 0;
  visible = false;

  earth!: THREE.Mesh;
  cloud1!: THREE.Mesh;
  cloud2!: THREE.Mesh;
  atmosphere!: THREE.Mesh;
  sunAtmosphere!: THREE.Mesh;
  moon!: THREE.Mesh;

  earthMaterial!: THREE.MeshStandardMaterial;
  cloud1Material!: THREE.ShaderMaterial;
  cloud2Material!: THREE.ShaderMaterial;
  atmosphereMaterial!: THREE.ShaderMaterial;
  sunAtmosphereMaterial!: THREE.ShaderMaterial;

  hexLayers: HexLayer[] = [];
  landLayer?: HexLayer;

  sunLight!: THREE.PointLight;
  sunKeyLight!: THREE.DirectionalLight;
  sunKeyLight2!: THREE.DirectionalLight;
  ambientLight!: THREE.AmbientLight;
  sunGlow!: THREE.Mesh;

  cloud1Axis = new THREE.Vector3(0, 1, 0);
  cloud2Axis = new THREE.Vector3(0.2, 1, -0.1).normalize();

  earthRadius = 50;
  shaderTimeScale = 0.28;
  cloudAmount = 1.0;
  earthRotSpeed = (Math.PI * 2) / 180;
  cloud1RotSpeed = (Math.PI * 2) / 240;
  cloud2RotSpeed = (Math.PI * 2) / 280;
  cloud1Opacity = 0.95;
  cloud2Opacity = 0.9;
  cloudBrightness = 5.0;

  cameraDistance = 23.5;
  cameraOrbit = 5.74;
  cameraOrbitSpeed = 0.1;
  cameraFarOffset = 40;
  cameraOrbitYOffset = -10;
  cameraShellOffset = 0.4;
  cameraTangentSpeed = 0.6;
  cameraYaw = 0.99;

  sunOrbitHeight = 870;
  sunOrbitAngleDeg = 0;
  sunOrbitSpeed = 0.0006283185307179586;
  sunOrbitIncline = 20 * DEG_TO_RAD;

  moonRadius = 5.5;
  moonOrbitRadius = 125;
  moonOrbitIncline = 8 * DEG_TO_RAD;
  moonOrbitPhaseRad = 0.6;

  materialColorScale = 1.25;
  lastFrameTime = performance.now();

  private resizeHandler: () => void;

  constructor(private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
    this.renderer.setClearColor(0x000000, 0);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.initLayers();
    this.initLights();

    this.resizeHandler = () => {
      const width = this.canvas.clientWidth;
      const height = this.canvas.clientHeight;
      if (width <= 0 || height <= 0) return;
      this.camera.aspect = width / height;
      this.camera.updateProjectionMatrix();
      this.renderer.setSize(width, height, false);
    };

    window.addEventListener('resize', this.resizeHandler);
    this.resizeHandler();
    this.updateCamera();
    this.camera.layers.enable(MOON_LIGHT_LAYER);

    void this.loadLandLayer();
    this.animate();
  }

  private initLayers() {
    const geo = (r: number, segs: number) => new THREE.SphereGeometry(r, segs, segs);

    this.earthMaterial = new THREE.MeshStandardMaterial({ color: 0x0b2a6f, roughness: 0.6, metalness: 0.05 });
    this.earth = new THREE.Mesh(geo(this.earthRadius, 64), this.earthMaterial);
    this.scene.add(this.earth);

    this.cloud1Material = this.createCloudMaterial(0.04, this.cloud1Opacity);
    this.cloud1 = new THREE.Mesh(geo(this.earthRadius + 1.2, 48), this.cloud1Material);
    this.cloud1.renderOrder = 2;
    this.scene.add(this.cloud1);

    this.cloud2Material = this.createCloudMaterial(0.1, this.cloud2Opacity);
    this.cloud2 = new THREE.Mesh(geo(this.earthRadius + 1.5, 48), this.cloud2Material);
    this.cloud2.renderOrder = 2;
    this.scene.add(this.cloud2);

    this.hexLayers = [
      new HexLayer(this.earthRadius, {
        radiusOffset: 1.0,
        count: 240,
        resolution: 3,
        ratePerSecond: 45,
        durationSeconds: 3,
        palette: [new THREE.Color(0.85, 0.85, 0.86), new THREE.Color(0.65, 0.67, 0.7), new THREE.Color(0.1, 0.1, 0.12)],
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 1.5,
        count: 200,
        resolution: 3,
        ratePerSecond: 45,
        durationSeconds: 3,
        palette: [new THREE.Color(0.75, 0.75, 0.76), new THREE.Color(0.45, 0.46, 0.5), new THREE.Color(0.05, 0.05, 0.07)],
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 2.0,
        count: 160,
        resolution: 3,
        ratePerSecond: 45,
        durationSeconds: 3,
        palette: [new THREE.Color(0.9, 0.9, 0.9), new THREE.Color(0.55, 0.56, 0.6), new THREE.Color(0.15, 0.15, 0.18)],
      }),
    ];
    this.hexLayers.forEach((l) => this.earth.add(l.mesh));

    this.atmosphereMaterial = new THREE.ShaderMaterial({
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
    this.atmosphere = new THREE.Mesh(geo(this.earthRadius + 2.0, 32), this.atmosphereMaterial);
    this.scene.add(this.atmosphere);

    this.sunAtmosphereMaterial = new THREE.ShaderMaterial({
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
    this.sunAtmosphere = new THREE.Mesh(geo(this.earthRadius + 3.2, 32), this.sunAtmosphereMaterial);
    this.scene.add(this.sunAtmosphere);

    this.moon = new THREE.Mesh(
      geo(this.moonRadius, 32),
      new THREE.MeshStandardMaterial({ color: 0xbfbfbf, map: createMoonRockTexture(128), roughness: 0.95, metalness: 0.02 }),
    );
    this.moon.layers.set(MOON_LIGHT_LAYER);
    this.scene.add(this.moon);
  }

  private createCloudMaterial(scale: number, opacity: number) {
    return new THREE.ShaderMaterial({
      transparent: true,
      depthWrite: false,
      vertexShader: cloudVertexShader,
      fragmentShader: cloudFragmentShader.replace(/CLOUD_SCALE/g, scale.toFixed(2)),
      uniforms: {
        uTime: { value: 0 },
        uTint: { value: new THREE.Color(1, 1, 1) },
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
      },
    });
  }

  private initLights() {
    this.sunKeyLight = new THREE.DirectionalLight(0xffffff, 0.35);
    this.sunKeyLight.position.set(100, 50, 100);
    this.scene.add(this.sunKeyLight, this.sunKeyLight.target);

    this.sunKeyLight2 = new THREE.DirectionalLight(0xffffff, 0.22);
    this.sunKeyLight2.position.set(-100, -50, -100);
    this.scene.add(this.sunKeyLight2, this.sunKeyLight2.target);

    this.ambientLight = new THREE.AmbientLight(0x090a10, 0.26);
    this.scene.add(this.ambientLight);

    this.sunGlow = new THREE.Mesh(new THREE.SphereGeometry(60, 32, 32), new THREE.MeshBasicMaterial({ color: 0xffe08a }));
    this.scene.add(this.sunGlow);

    this.scene.add(new THREE.HemisphereLight(0xffffff, 0x111111, 1.0));

    this.sunLight = new THREE.PointLight(0xffffff, 2.1, 220);
    this.sunLight.layers.enable(MOON_LIGHT_LAYER);
    this.scene.add(this.sunLight);
  }

  private async loadLandLayer() {
    try {
      const resp = await fetch('/land.h3.json');
      if (resp.ok) {
        const payload = await resp.json();
        const cells = Array.isArray(payload) ? payload : payload?.cells;
        const res = payload?.resolution ?? 3;
        if (cells?.length) {
          this.buildLandLayer(cells, res);
          return;
        }
      }

      const geoResp = await fetch('/land.geojson');
      if (geoResp.ok) {
        const cells = this.geojsonToCells(await geoResp.json(), 3);
        if (cells.length) this.buildLandLayer(cells, 3);
      }
    } catch {
      // Optional land layer.
    }
  }

  private buildLandLayer(cells: string[], resolution: number) {
    const landLayer = new HexLayer(this.earthRadius, {
      radiusOffset: 0.6,
      count: cells.length,
      resolution,
      ratePerSecond: 1,
      durationSeconds: 9999,
      opacity: 0.95,
      palette: [new THREE.Color(0.2, 0.35, 0.2), new THREE.Color(0.25, 0.45, 0.25), new THREE.Color(0.4, 0.5, 0.3)],
      cells,
      animate: false,
    });
    landLayer.material.depthWrite = false;
    landLayer.material.polygonOffset = true;
    landLayer.material.polygonOffsetFactor = -1;
    landLayer.material.polygonOffsetUnits = -1;
    landLayer.mesh.renderOrder = 1;
    this.hexLayers.push(landLayer);
    this.earth.add(landLayer.mesh);
    this.landLayer = landLayer;
  }

  private geojsonToCells(geojson: any, res: number) {
    const cells = new Set<string>();
    geojson?.features?.forEach((f: any) => {
      const g = f?.geometry;
      if (!g) return;
      const polys = g.type === 'Polygon' ? [g.coordinates] : g.type === 'MultiPolygon' ? g.coordinates : [];
      polys.forEach((coords: any) => {
        try {
          polygonToCells(coords, res, true).forEach((c) => cells.add(c));
        } catch {
          // Ignore malformed geometry.
        }
      });
    });
    return Array.from(cells);
  }

  private updateCamera() {
    const near = this.earthRadius + Math.max(6, this.cameraDistance);
    const orbit = this.cameraOrbit + this.cameraYaw;
    this.camera.position.set(
      Math.cos(orbit) * (near + this.cameraFarOffset),
      this.cameraOrbitYOffset,
      Math.sin(orbit) * near,
    );
    this.camera.lookAt(
      new THREE.Vector3(
        Math.cos(orbit * this.cameraTangentSpeed) * (this.earthRadius + this.cameraShellOffset),
        0,
        Math.sin(orbit * this.cameraTangentSpeed) * (this.earthRadius + this.cameraShellOffset),
      ),
    );
  }

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;

    const now = performance.now();
    const delta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;

    const ds = delta;
    const ct = now * 0.001 * this.shaderTimeScale;

    this.earth.rotation.y += this.earthRotSpeed * ds;
    this.cloud1.rotateOnAxis(this.cloud1Axis, this.cloud1RotSpeed * delta);
    this.cloud2.rotateOnAxis(this.cloud2Axis, this.cloud2RotSpeed * delta);

    this.cloud1Material.uniforms.uTime.value = ct;
    this.cloud2Material.uniforms.uTime.value = ct;

    this.cameraOrbit += this.cameraOrbitSpeed * ds;
    this.updateCamera();

    const sunRad = this.earthRadius + this.sunOrbitHeight;
    const sunA = now * this.sunOrbitSpeed + this.sunOrbitAngleDeg * DEG_TO_RAD;
    const ky = Math.sin(sunA) * Math.sin(this.sunOrbitIncline) * sunRad;
    const kz = Math.sin(sunA) * Math.cos(this.sunOrbitIncline) * sunRad;
    this.sunLight.position.set(Math.cos(sunA) * sunRad, ky, kz);
    this.sunGlow.position.copy(this.sunLight.position);

    const sDir = this.sunLight.position.clone().normalize();
    this.cloud1Material.uniforms.uSunDir.value.copy(sDir);
    this.cloud2Material.uniforms.uSunDir.value.copy(sDir);
    this.cloud1Material.uniforms.uSunIntensity.value = 0.5 * this.cloudBrightness;
    this.cloud2Material.uniforms.uSunIntensity.value = 0.5 * this.cloudBrightness;
    this.cloud1Material.uniforms.uOpacity.value = this.cloud1Opacity;
    this.cloud2Material.uniforms.uOpacity.value = this.cloud2Opacity;
    this.cloud1Material.uniforms.uCloudAmount.value = this.cloudAmount;
    this.cloud2Material.uniforms.uCloudAmount.value = this.cloudAmount;

    this.hexLayers.forEach((l) => l.update(now * 0.001, sDir, SUN_COLOR));

    this.atmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(this.camera.position);

    const moonA = now * (this.sunOrbitSpeed / 10) + this.moonOrbitPhaseRad;
    this.moon.position.set(
      Math.cos(moonA) * this.moonOrbitRadius,
      Math.sin(moonA) * Math.sin(this.moonOrbitIncline) * this.moonOrbitRadius,
      Math.sin(moonA) * Math.cos(this.moonOrbitIncline) * this.moonOrbitRadius,
    );

    this.renderer.render(this.scene, this.camera);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resizeHandler);
    this.renderer.dispose();
  }

  setVisible(visible: boolean) {
    this.visible = visible;
    if (visible) {
      this.lastFrameTime = performance.now();
      this.resizeHandler();
    }
  }
}

export function mountHero(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector('canvas.hero-stage') as HTMLCanvasElement | null;
  if (!canvas) throw new Error('hero canvas not found');

  return new HeroControl(canvas);
}
