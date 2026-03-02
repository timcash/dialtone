import { VisualizationControl } from '@ui/types';
import * as THREE from 'three';
import { HexLayer } from '../../earth/hex_layer';
import { MarkerManager, City } from './markers';
import { LightingManager } from './lights';
import { CameraManager } from './camera';

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
  renderer: THREE.WebGLRenderer;
  frameId = 0;
  visible = false;

  earth!: THREE.Mesh;
  moon!: THREE.Mesh;
  
  orbitMarker!: THREE.Mesh;
  topDownMarker!: THREE.Mesh;
  
  hexLayers: HexLayer[] = [];
  landLayer?: HexLayer;

  markerManager!: MarkerManager;
  lightingManager!: LightingManager;
  cameraManager!: CameraManager;
  
  activeCamera!: THREE.PerspectiveCamera;

  earthRadius = 50;
  earthRotSpeed = (Math.PI * 2) / 180;
  
  cities: City[] = [
    { name: 'San Francisco', lat: 37.7749, lng: -122.4194 },
    { name: 'New York', lat: 40.7128, lng: -74.0060 },
    { name: 'London', lat: 51.5074, lng: -0.1278 },
    { name: 'Paris', lat: 48.8566, lng: 2.3522 },
    { name: 'Berlin', lat: 52.5200, lng: 13.4050 },
    { name: 'Moscow', lat: 55.7558, lng: 37.6173 },
    { name: 'Tokyo', lat: 35.6762, lng: 139.6503 },
    { name: 'Beijing', lat: 39.9042, lng: 116.4074 },
    { name: 'Mumbai', lat: 19.0760, lng: 72.8777 },
    { name: 'Sydney', lat: -33.8688, lng: 151.2093 },
    { name: 'Rio de Janeiro', lat: -22.9068, lng: -43.1729 },
    { name: 'Cape Town', lat: -33.9249, lng: 18.4241 }
  ];

  raycaster = new THREE.Raycaster();
  mouse = new THREE.Vector2();

  lastFrameTime = performance.now();

  private resizeHandler: () => void;

  constructor(private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
    this.renderer.setClearColor(0x000000, 0);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.cameraManager = new CameraManager(this.canvas.clientWidth / this.canvas.clientHeight);
    this.activeCamera = this.cameraManager.orbitCamera;
    
    this.lightingManager = new LightingManager(this.scene, this.earthRadius);
    this.initEarth();
    this.initSystemMarkers();
    
    this.markerManager = new MarkerManager(this.earth, this.earthRadius, 10, this.canvas.clientWidth / this.canvas.clientHeight);
    this.markerManager.initCities(this.cities);

    this.resizeHandler = () => {
      const width = this.canvas.clientWidth;
      const height = this.canvas.clientHeight;
      if (width <= 0 || height <= 0) return;
      const aspect = width / height;
      this.cameraManager.setAspect(aspect);
      this.markerManager.setAspect(aspect);
      this.renderer.setSize(width, height, false);
    };

    window.addEventListener('resize', this.resizeHandler);
    this.resizeHandler();
    
    this.cameraManager.orbitCamera.layers.enable(MOON_LIGHT_LAYER);
    this.cameraManager.topDownCamera.layers.enable(MOON_LIGHT_LAYER);

    this.canvas.addEventListener('click', (e) => this.onCanvasClick(e));
    this.bindFormButtons();

    void this.loadLandLayer();
    this.animate();
  }

  private initSystemMarkers() {
    // Orbit Camera Marker (Red)
    this.orbitMarker = new THREE.Mesh(
      new THREE.SphereGeometry(4.0, 16, 16),
      new THREE.MeshBasicMaterial({ color: 0xff4d4d, transparent: true, opacity: 0.8 })
    );
    this.orbitMarker.renderOrder = 20;
    this.scene.add(this.orbitMarker);

    // Top-Down Camera Marker (Yellow)
    this.topDownMarker = new THREE.Mesh(
      new THREE.SphereGeometry(15.0, 16, 16),
      new THREE.MeshBasicMaterial({ color: 0xffcc00, transparent: true, opacity: 0.8 })
    );
    this.topDownMarker.position.set(0, 380, 0); // Sit just below the actual camera at 400
    this.topDownMarker.renderOrder = 20;
    this.scene.add(this.topDownMarker);
  }

  private bindFormButtons() {
    const form = document.querySelector('form[data-mode-form="earth-hero-stage"]');
    if (!form) return;

    form.querySelectorAll('button').forEach(btn => {
      const cmd = btn.getAttribute('data-cmd');
      if (cmd === 'view-orbit') {
        btn.onclick = () => this.activeCamera = this.cameraManager.orbitCamera;
      } else if (cmd === 'view-hover') {
        btn.onclick = () => this.activeCamera = this.markerManager.markers[0].camera;
      } else if (cmd === 'view-top') {
        btn.onclick = () => this.activeCamera = this.cameraManager.topDownCamera;
      } else if (cmd === 'view-swap') {
        btn.onclick = () => this.toggleCamera();
      }
    });
  }

  private toggleCamera() {
    if (this.activeCamera === this.cameraManager.orbitCamera) {
      this.activeCamera = this.markerManager.markers[0].camera;
    } else if (this.activeCamera === this.cameraManager.topDownCamera) {
      this.activeCamera = this.cameraManager.orbitCamera;
    } else {
      // We are in hover, go to top down
      this.activeCamera = this.cameraManager.topDownCamera;
    }
  }

  private onCanvasClick(event: MouseEvent) {
    const rect = this.canvas.getBoundingClientRect();
    this.mouse.x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
    this.mouse.y = -((event.clientY - rect.top) / rect.height) * 2 + 1;

    this.raycaster.setFromCamera(this.mouse, this.activeCamera);
    
    // Check City Markers
    const cityMeshes = this.markerManager.markers.map(m => m.mesh);
    const cityIntersects = this.raycaster.intersectObjects(cityMeshes);
    if (cityIntersects.length > 0) {
      const index = this.markerManager.markers.findIndex(m => m.mesh === cityIntersects[0].object);
      this.activeCamera = this.markerManager.markers[index].camera;
      return;
    }

    // Check Orbit Marker
    if (this.raycaster.intersectObject(this.orbitMarker).length > 0) {
      this.activeCamera = this.cameraManager.orbitCamera;
      return;
    }

    // Check Top-Down Marker
    if (this.raycaster.intersectObject(this.topDownMarker).length > 0) {
      this.activeCamera = this.cameraManager.topDownCamera;
      return;
    }
  }

  private initEarth() {
    const geo = (r: number, segs: number) => new THREE.SphereGeometry(r, segs, segs);

    this.earth = new THREE.Mesh(
      geo(this.earthRadius, 64), 
      new THREE.MeshStandardMaterial({ color: 0x0b2a6f, roughness: 0.6, metalness: 0.05 })
    );
    this.scene.add(this.earth);

    this.hexLayers = [
      new HexLayer(this.earthRadius, {
        radiusOffset: 1.0, count: 240, resolution: 3, ratePerSecond: 45, durationSeconds: 3,
        palette: [new THREE.Color(0.85, 0.85, 0.86), new THREE.Color(0.65, 0.67, 0.7), new THREE.Color(0.1, 0.1, 0.12)],
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 1.5, count: 200, resolution: 3, ratePerSecond: 45, durationSeconds: 3,
        palette: [new THREE.Color(0.75, 0.75, 0.76), new THREE.Color(0.45, 0.46, 0.5), new THREE.Color(0.05, 0.05, 0.07)],
      }),
      new HexLayer(this.earthRadius, {
        radiusOffset: 2.0, count: 160, resolution: 3, ratePerSecond: 45, durationSeconds: 3,
        palette: [new THREE.Color(0.9, 0.9, 0.9), new THREE.Color(0.55, 0.56, 0.6), new THREE.Color(0.15, 0.15, 0.18)],
      }),
    ];
    this.hexLayers.forEach((l) => this.earth.add(l.mesh));

    this.moon = new THREE.Mesh(
      geo(5.5, 32),
      new THREE.MeshStandardMaterial({ color: 0xbfbfbf, map: createMoonRockTexture(128), roughness: 0.95, metalness: 0.02 }),
    );
    this.moon.layers.set(MOON_LIGHT_LAYER);
    this.scene.add(this.moon);

    // Camera Orbit Path Visualization (Elliptical)
    const near = this.earthRadius + 23.5;
    const xRadius = near + 80 + (this.cameraManager.cameraDistance - 23.5);
    const zRadius = near + (this.cameraManager.cameraDistance - 23.5) / 2;
    const orbitPoints = [];
    for (let i = 0; i <= 128; i++) {
      const a = (i / 128) * Math.PI * 2 + 0.99;
      orbitPoints.push(new THREE.Vector3(Math.cos(a) * xRadius, -10, Math.sin(a) * zRadius));
    }
    const orbitGeo = new THREE.BufferGeometry().setFromPoints(orbitPoints);
    const orbitMat = new THREE.LineBasicMaterial({ color: 0xffffff, transparent: true, opacity: 0.4 });
    const orbitLine = new THREE.LineLoop(orbitGeo, orbitMat);
    this.scene.add(orbitLine);
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
        const json = await geoResp.json();
        const cells = this.geojsonToCells(json, 3);
        if (cells.length) this.buildLandLayer(cells, 3);
      }
    } catch (err) {
      console.error('[Earth] Failed to load land layer:', err);
    }
  }

  private buildLandLayer(cells: string[], resolution: number) {
    const landLayer = new HexLayer(this.earthRadius, {
      radiusOffset: 0.1, count: cells.length, resolution, ratePerSecond: 1, durationSeconds: 9999,
      opacity: 0.95, palette: [new THREE.Color(0.2, 0.35, 0.2), new THREE.Color(0.25, 0.45, 0.25), new THREE.Color(0.4, 0.5, 0.3)],
      cells, animate: false,
    });
    landLayer.material.depthWrite = false;
    landLayer.material.polygonOffset = true;
    landLayer.material.polygonOffsetFactor = landLayer.material.polygonOffsetUnits = -1;
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
          const { latLngToCell } = require('h3-js');
          // Simplified geojson to cells for basic land display
          cells.add(latLngToCell(coords[0][1], coords[0][0], res));
        } catch {}
      });
    });
    return Array.from(cells);
  }

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;

    const now = performance.now();
    const delta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;

    this.earth.rotation.y += this.earthRotSpeed * delta;
    this.earth.updateMatrixWorld(true);

    this.markerManager.update(now);
    this.cameraManager.updateOrbit(delta, this.earthRadius);
    
    // Sync Orbit Marker to Camera position
    this.orbitMarker.position.copy(this.cameraManager.orbitCamera.position);

    // Hide marker of active camera
    this.orbitMarker.visible = this.activeCamera !== this.cameraManager.orbitCamera;
    this.topDownMarker.visible = this.activeCamera !== this.cameraManager.topDownCamera;
    this.markerManager.markers.forEach(m => {
      m.mesh.visible = this.activeCamera !== m.camera;
      m.label.visible = this.activeCamera !== m.camera;
      m.line.visible = this.activeCamera !== m.camera;
    });

    const { sunDir, sunColor } = this.lightingManager.update(now, this.activeCamera.position);
    this.hexLayers.forEach((l) => l.update(now * 0.001, sunDir, sunColor));

    const moonA = now * (this.lightingManager.sunOrbitSpeed / 10) + 0.6;
    this.moon.position.set(
      Math.cos(moonA) * 125,
      Math.sin(moonA) * Math.sin(8 * (Math.PI / 180)) * 125,
      Math.sin(moonA) * Math.cos(8 * (Math.PI / 180)) * 125,
    );

    this.renderer.render(this.scene, this.activeCamera);
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
