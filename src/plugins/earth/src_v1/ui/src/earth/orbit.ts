import * as THREE from 'three';
import type { VisualizationControl } from '../../../../../ui/src_v1/ui/types';
import { polygonToCells } from 'h3-js';
import { HexLayer } from './hex_layer';

import cloudVertexShader from '../shaders/cloud.vert.glsl?raw';
import cloudFragmentShader from '../shaders/cloud.frag.glsl?raw';
import atmosphereVertexShader from '../shaders/atmosphere.vert.glsl?raw';
import atmosphereFragmentShader from '../shaders/atmosphere.frag.glsl?raw';
import sunAtmosphereVertexShader from '../shaders/sun_atmosphere.vert.glsl?raw';
import sunAtmosphereFragmentShader from '../shaders/sun_atmosphere.frag.glsl?raw';

const DEG_TO_RAD = Math.PI / 180;
const SUN_COLOR = new THREE.Color(1.0, 1.0, 1.0);
const KEY1_COLOR = new THREE.Color(0.9, 0.95, 1.0);
const KEY2_COLOR = new THREE.Color(0.85, 0.9, 1.0);

class OrbitScene {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(75, 1, 0.1, 10000);
  private renderer = new THREE.WebGLRenderer({ antialias: true });

  private frameId = 0;
  private isVisible = true;
  private resizeObserver?: ResizeObserver;
  private lastFrameTime = performance.now();

  private earth!: THREE.Mesh;
  private cloud1!: THREE.Mesh;
  private cloud2!: THREE.Mesh;
  private atmosphere!: THREE.Mesh;
  private sunAtmosphere!: THREE.Mesh;
  private moon!: THREE.Mesh;

  private cloud1Material!: THREE.ShaderMaterial;
  private cloud2Material!: THREE.ShaderMaterial;
  private atmosphereMaterial!: THREE.ShaderMaterial;
  private sunAtmosphereMaterial!: THREE.ShaderMaterial;

  private earthRadius = 50;
  private cloudAmount = 1.0;
  private earthRotSpeed = (Math.PI * 2) / 180;
  private cloud1RotSpeed = (Math.PI * 2) / 240;
  private cloud2RotSpeed = (Math.PI * 2) / 280;
  private cloud1Opacity = 0.95;
  private cloud2Opacity = 0.9;
  private cloudBrightness = 5.0;
  private cameraDistance = 23.5;
  private cameraOrbit = 5.74;
  private cameraOrbitSpeed = 0.1;
  private cameraFarOffset = 40;
  private cameraOrbitYOffset = -10;
  private cameraShellOffset = 0.4;
  private cameraTangentSpeed = 0.6;
  private cameraYaw = 0.99;
  private shaderTimeScale = 0.28;

  private sunLight!: THREE.PointLight;
  private sunGlow!: THREE.Mesh;
  private sunOrbitHeight = 870;
  private sunOrbitSpeed = 0.0006283185307179586;
  private sunOrbitAngleDeg = 0;
  private sunOrbitIncline = 20 * DEG_TO_RAD;

  private moonOrbitRadius = 125;
  private moonOrbitIncline = 8 * DEG_TO_RAD;
  private moonOrbitPhaseRad = 0.6;

  private hexLayers: HexLayer[] = [];
  private landLayer?: HexLayer;

  constructor(private container: HTMLElement) {
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    const canvas = this.renderer.domElement;
    canvas.style.position = 'absolute';
    canvas.style.top = '0';
    canvas.style.left = '0';
    canvas.style.width = '100%';
    canvas.style.height = '100%';
    canvas.style.display = 'block';
    canvas.setAttribute('aria-label', 'Earth Canvas');
    container.appendChild(canvas);

    this.initLayers();
    this.initLights();
    this.resize();
    this.updateCamera();
    this.animate();
    this.loadLandLayer().catch(() => undefined);

    if (typeof ResizeObserver !== 'undefined') {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    } else {
      window.addEventListener('resize', this.resize);
    }
  }

  private initLayers() {
    const geo = (r: number, segs: number) => new THREE.SphereGeometry(r, segs, segs);

    const earthMaterial = new THREE.MeshStandardMaterial({ color: 0x0b2a6f, roughness: 0.6, metalness: 0.05 });
    this.earth = new THREE.Mesh(geo(this.earthRadius, 64), earthMaterial);
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
      new HexLayer(this.earthRadius, { radiusOffset: 1.0, count: 220, resolution: 3, ratePerSecond: 45, durationSeconds: 3, palette: [new THREE.Color(0.85, 0.85, 0.86), new THREE.Color(0.65, 0.67, 0.7), new THREE.Color(0.1, 0.1, 0.12)] }),
      new HexLayer(this.earthRadius, { radiusOffset: 1.5, count: 180, resolution: 3, ratePerSecond: 45, durationSeconds: 3, palette: [new THREE.Color(0.75, 0.75, 0.76), new THREE.Color(0.45, 0.46, 0.5), new THREE.Color(0.05, 0.05, 0.07)] }),
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
        uColorScale: { value: 1.25 },
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
        uColorScale: { value: 1.25 },
      },
      vertexShader: sunAtmosphereVertexShader,
      fragmentShader: sunAtmosphereFragmentShader,
    });
    this.sunAtmosphere = new THREE.Mesh(geo(this.earthRadius + 3.2, 32), this.sunAtmosphereMaterial);
    this.scene.add(this.sunAtmosphere);

    this.moon = new THREE.Mesh(geo(5.5, 32), new THREE.MeshStandardMaterial({ color: 0xbfbfbf, roughness: 0.95, metalness: 0.02 }));
    this.scene.add(this.moon);
  }

  private async loadLandLayer() {
    try {
      const geoResp = await fetch('/land.geojson');
      if (!geoResp.ok) return;
      const cells = this.geojsonToCells(await geoResp.json(), 3);
      if (!cells.length) return;
      const landLayer = new HexLayer(this.earthRadius, {
        radiusOffset: 0.6,
        count: cells.length,
        resolution: 3,
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
      this.landLayer = landLayer;
      this.hexLayers.push(landLayer);
      this.earth.add(landLayer.mesh);
    } catch {
      // Land overlay is optional.
    }
  }

  private geojsonToCells(geojson: any, res: number): string[] {
    const cells = new Set<string>();
    geojson?.features?.forEach((f: any) => {
      const g = f?.geometry;
      if (!g) return;
      const polys = g.type === 'Polygon' ? [g.coordinates] : g.type === 'MultiPolygon' ? g.coordinates : [];
      polys.forEach((coords: any) => {
        try {
          polygonToCells(coords, res, true).forEach((c) => cells.add(c));
        } catch {
          // skip malformed geometry
        }
      });
    });
    return Array.from(cells);
  }

  private createCloudMaterial(scale: number, opacity: number): THREE.ShaderMaterial {
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
        uColorScale: { value: 1.25 },
        uCloudAmount: { value: this.cloudAmount },
      },
    });
  }

  private initLights() {
    const sunKeyLight = new THREE.DirectionalLight(0xffffff, 0.35);
    sunKeyLight.position.set(100, 50, 100);
    this.scene.add(sunKeyLight, sunKeyLight.target);

    const sunKeyLight2 = new THREE.DirectionalLight(0xffffff, 0.22);
    sunKeyLight2.position.set(-100, -50, -100);
    this.scene.add(sunKeyLight2, sunKeyLight2.target);

    this.scene.add(new THREE.AmbientLight(0x090a10, 0.26));
    this.scene.add(new THREE.HemisphereLight(0xffffff, 0x111111, 1.0));

    this.sunGlow = new THREE.Mesh(
      new THREE.SphereGeometry(60, 32, 32),
      new THREE.MeshBasicMaterial({ color: 0xffe08a }),
    );
    this.scene.add(this.sunGlow);

    this.sunLight = new THREE.PointLight(0xffffff, 2.1, 220);
    this.scene.add(this.sunLight);
  }

  private resize = () => {
    const rect = this.container.getBoundingClientRect();
    this.camera.aspect = Math.max(1, rect.width) / Math.max(1, rect.height);
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(rect.width, rect.height, false);
  };

  setVisible(visible: boolean) {
    this.isVisible = visible;
  }

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    window.removeEventListener('resize', this.resize);
    this.renderer.dispose();
    this.container.removeChild(this.renderer.domElement);
  }

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    const now = performance.now();
    const delta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;

    const ds = delta;
    const ct = now * 0.001 * this.shaderTimeScale;

    this.earth.rotation.y += this.earthRotSpeed * ds;
    this.cloud1.rotateOnAxis(new THREE.Vector3(0, 1, 0), this.cloud1RotSpeed * delta);
    this.cloud2.rotateOnAxis(new THREE.Vector3(0.2, 1, -0.1).normalize(), this.cloud2RotSpeed * delta);

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
}

export function mountEarth(container: HTMLElement): VisualizationControl {
  container.innerHTML = `
    <div class="earth-stage" aria-label="Three Stage"></div>
    <aside class="earth-marketing" aria-label="Earth Hero Copy">
      <h2>Global Virtual Library</h2>
      <p>Explore orbital geometry, atmospheric shading, and dynamic globe overlays.</p>
    </aside>
    <div class="earth-legend" aria-label="Earth Legend">Three.js + H3 hex layer</div>
  `;

  const stage = container.querySelector('.earth-stage') as HTMLElement | null;
  if (!stage) throw new Error('earth stage missing');
  const orbit = new OrbitScene(stage);

  return {
    dispose: () => {
      orbit.dispose();
      container.innerHTML = '';
    },
    setVisible: (visible: boolean) => orbit.setVisible(visible),
  };
}
