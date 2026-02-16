import * as THREE from "three";
import { polygonToCells, cellToLatLng } from "h3-js";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupGeoToolsMenu } from "./menu";
import pointGlowVert from "../../shaders/point-glow.vert.glsl?raw";
import pointGlowFrag from "../../shaders/point-glow.frag.glsl?raw";
import { type VisualizationControl, type SectionManager } from "../util/section";

type GeojsonLike = {
  type: string;
  features?: Array<{
    geometry?: {
      type: string;
      coordinates: any;
    };
  }>;
};

class GeoToolsVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  sections: SectionManager;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  private fpsCounter = new FpsCounter("geotools");
  private geoPoints?: THREE.Points;
  private h3Points?: THREE.Points;
  private time = 0;
  private baseRadius = 1.4;
  private cellCenters: string[] = [];
  frameCount = 0;
  
  geoJsonData: GeojsonLike | null = null;
  resolution = 3;
  h3Cells: string[] = [];
  statusText = "No data loaded";

  constructor(container: HTMLElement, sections: SectionManager) {
    this.container = container;
    this.sections = sections;
    this.renderer.setClearColor(0x0b0d14, 1);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    const canvas = this.renderer.domElement;
    canvas.style.position = "absolute";
    canvas.style.top = canvas.style.left = "0";
    canvas.style.width = canvas.style.height = "100%";
    this.container.appendChild(canvas);
    this.camera.position.set(0, 0, 4);
    this.camera.lookAt(0, 0, 0);
    this.scene.add(new THREE.AmbientLight(0xffffff, 0.6));
    const keyLight = new THREE.DirectionalLight(0xffffff, 1.0);
    keyLight.position.set(2, 2, 2);
    this.scene.add(keyLight);
    const sunLight = new THREE.DirectionalLight(0xffffff, 0.8);
    sunLight.position.set(-2, 1, -2);
    this.scene.add(sunLight);
    this.resize();
    this.animate();
    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);
    if (typeof ResizeObserver !== "undefined") {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    }
    this.backgroundInit();
  }

  async backgroundInit() {
    this.sections.setLoadingMessage("s-geotools", "loading land geometry ...");
    try {
      const response = await fetch("/land.geojson");
      if (response.ok) {
        this.geoJsonData = await response.json();
        this.regenerate();
      }
    } catch (e) { console.warn("[geotools] failed to load default land.geojson", e); }
  }

  regenerate() {
    if (!this.geoJsonData) {
      this.clearPoints();
      this.h3Cells = [];
      this.statusText = "No data loaded";
      return;
    }
    this.setGeojson(this.geoJsonData, this.resolution);
    this.h3Cells = this.cellCenters;
    this.statusText = `${this.h3Cells.length.toLocaleString()} cells @ r${this.resolution}`;
  }

  resize = () => {
    const rect = this.container.getBoundingClientRect();
    this.camera.aspect = Math.max(1, rect.width) / Math.max(1, rect.height);
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(rect.width, rect.height, false);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    this.renderer.dispose();
    this.clearPoints();
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "geotools");
    if (!visible) this.fpsCounter.clear();
  }

  setGeojson(geojson: GeojsonLike | null, resolution: number) {
    if (!geojson) { this.clearPoints(); return; }
    const geoPoints = this.buildGeojsonPoints(geojson);
    const cells = this.geojsonToCells(geojson, resolution);
    this.cellCenters = cells;
    const h3Points = this.buildH3Points(cells);
    this.replacePoints(geoPoints, h3Points);
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;
    this.time += 0.016;
    this.frameCount++;
    if (this.geoPoints) {
        this.geoPoints.rotation.y += 0.0008;
        (this.geoPoints.material as THREE.ShaderMaterial).uniforms.uTime.value = this.time;
    }
    if (this.h3Points) {
        this.h3Points.rotation.y += 0.0008;
        (this.h3Points.material as THREE.ShaderMaterial).uniforms.uTime.value = this.time;
    }
    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    this.fpsCounter.tick(performance.now() - (this.lastStepTime || 0), this.gpuTimer.lastMs);
    this.lastStepTime = performance.now();
  };
  private lastStepTime = 0;

  private replacePoints(geo: THREE.Points, h3: THREE.Points) {
    this.clearPoints();
    this.geoPoints = geo; this.h3Points = h3;
    this.scene.add(geo, h3);
  }

  private clearPoints() {
    [this.geoPoints, this.h3Points].forEach(p => {
      if (!p) return;
      p.geometry.dispose(); (p.material as THREE.Material).dispose();
      this.scene.remove(p);
    });
    this.geoPoints = this.h3Points = undefined;
  }

  private buildGeojsonPoints(geojson: GeojsonLike) {
    const positions: number[] = [];
    (geojson.features ?? []).forEach(f => {
      const g = f.geometry; if (!g) return;
      const polys = g.type === "Polygon" ? [g.coordinates] : g.type === "MultiPolygon" ? g.coordinates : [];
      polys.forEach((coords: any) => coords.forEach((ring: any) => ring.forEach(([lng, lat]: any) => {
        const latRad = lat * (Math.PI / 180), lngRad = lng * (Math.PI / 180);
        positions.push(this.baseRadius * Math.cos(latRad) * Math.sin(lngRad), this.baseRadius * Math.sin(latRad), this.baseRadius * Math.cos(latRad) * Math.cos(lngRad));
      })));
    });
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
    return new THREE.Points(geometry, new THREE.ShaderMaterial({
      uniforms: { uColor: { value: new THREE.Color(0xeeeeee) }, uSize: { value: 0.6 }, uTime: { value: 0 }, uPixelRatio: { value: this.renderer.getPixelRatio() } },
      vertexShader: pointGlowVert, fragmentShader: pointGlowFrag, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
    }));
  }

  private buildH3Points(cells: string[]) {
    const positions: number[] = [];
    cells.forEach(cell => {
      const [lat, lng] = cellToLatLng(cell);
      const latRad = lat * (Math.PI / 180), lngRad = lng * (Math.PI / 180), r = this.baseRadius + 0.03;
      positions.push(r * Math.cos(latRad) * Math.sin(lngRad), r * Math.sin(latRad), r * Math.cos(latRad) * Math.cos(lngRad));
    });
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
    return new THREE.Points(geometry, new THREE.ShaderMaterial({
      uniforms: { uColor: { value: new THREE.Color(0x4cff9a) }, uSize: { value: 1.2 }, uTime: { value: 0 }, uPixelRatio: { value: this.renderer.getPixelRatio() } },
      vertexShader: pointGlowVert, fragmentShader: pointGlowFrag, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
    }));
  }

  private geojsonToCells(geojson: GeojsonLike, resolution: number) {
    const cells = new Set<string>();
    (geojson.features ?? []).forEach(f => {
      const g = f.geometry; if (!g) return;
      const polys = g.type === "Polygon" ? [g.coordinates] : g.type === "MultiPolygon" ? g.coordinates : [];
      polys.forEach((coords: any) => { try { polygonToCells(coords, resolution, true).forEach(c => cells.add(c)); } catch {} });
    });
    return Array.from(cells);
  }
}

export function mountGeoTools(container: HTMLElement, sections: SectionManager): VisualizationControl {
  container.innerHTML = `<div class="marketing-overlay"><h2>GeoTools</h2><p data-typing-subtitle></p></div>`;
  const stopTyping = startTyping(container.querySelector("[data-typing-subtitle]"), ["Upload GeoJSON and convert to H3.", "Preview outlines and cell centers.", "Download a precomputed H3 layer."]);
  const viz = new GeoToolsVisualization(container, sections);
  let updateStatusFn = () => {};
  const options = {
    currentResolution: viz.resolution,
    onResolutionChange: (v: number) => { viz.resolution = v; viz.regenerate(); updateStatusFn(); },
    onFile: async (file: File) => {
      sections.setLoadingMessage("s-geotools", `parsing ${file.name} ...`);
      const json = JSON.parse(await file.text());
      if (json.type === "FeatureCollection") { viz.geoJsonData = json; viz.regenerate(); updateStatusFn(); }
    },
    onConvert: () => {
      if (!viz.geoJsonData) return;
      viz.statusText = "Converting to H3..."; updateStatusFn();
      setTimeout(() => { viz.regenerate(); updateStatusFn(); }, 50);
    },
    onDownload: () => {
      if (viz.h3Cells.length === 0) return;
      const url = URL.createObjectURL(new Blob([JSON.stringify(viz.h3Cells)], { type: "application/json" }));
      const a = document.createElement("a"); a.href = url; a.download = `h3_res${viz.resolution}_${viz.h3Cells.length}.json`; a.click(); URL.revokeObjectURL(url);
    },
    getStatusText: () => viz.statusText,
  };
  return {
    dispose: () => { viz.dispose(); stopTyping(); container.innerHTML = ""; },
    setVisible: (v) => viz.setVisible(v),
    updateUI: () => { const { updateStatus } = setupGeoToolsMenu(options); updateStatusFn = updateStatus; }
  };
}
