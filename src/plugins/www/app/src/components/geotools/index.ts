import * as THREE from "three";
import { polygonToCells, cellToLatLng } from "h3-js";
import { FpsCounter } from "../fps";
import { GpuTimer } from "../gpu_timer";
import { VisibilityMixin } from "../section";
import { startTyping } from "../typing";
import { setupGeoToolsConfig } from "./config";

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
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  private fpsCounter = new FpsCounter("geotools");
  private geoPoints?: THREE.Points;
  private h3Points?: THREE.Points;
  private time = 0;
  private lightDir = new THREE.Vector3(1, 1, 1).normalize();
  private keyLight!: THREE.DirectionalLight;
  private baseRadius = 1.4;
  private cellCenters: string[] = [];
  frameCount = 0;

  constructor(container: HTMLElement) {
    this.container = container;

    this.renderer.setClearColor(0x0b0d14, 1);
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

    this.camera.position.set(0, 0, 4);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.4));
    this.keyLight = new THREE.DirectionalLight(0xffffff, 0.9);
    this.keyLight.position.set(2, 2, 2);
    this.scene.add(this.keyLight);

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
    window.removeEventListener("resize", this.resize);
    this.renderer.dispose();
    if (this.container.contains(this.renderer.domElement)) {
      this.container.removeChild(this.renderer.domElement);
    }
    this.clearPoints();
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "geotools");
    if (!visible) this.fpsCounter.clear();
  }

  setGeojson(geojson: GeojsonLike, resolution: number) {
    const geoPoints = this.buildGeojsonPoints(geojson);
    const cells = this.geojsonToCells(geojson, resolution);
    this.cellCenters = cells;
    const h3Points = this.buildH3Points(cells);
    this.replacePoints(geoPoints, h3Points);
  }

  getCells() {
    return this.cellCenters;
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;
    if (this.geoPoints) this.geoPoints.rotation.y += 0.0008;
    if (this.h3Points) this.h3Points.rotation.y += 0.0008;

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };

  private replacePoints(geo: THREE.Points, h3: THREE.Points) {
    this.clearPoints();
    this.geoPoints = geo;
    this.h3Points = h3;
    this.scene.add(geo);
    this.scene.add(h3);
  }

  private clearPoints() {
    [this.geoPoints, this.h3Points].forEach((points) => {
      if (!points) return;
      points.geometry.dispose();
      (points.material as THREE.Material).dispose();
      this.scene.remove(points);
    });
    this.geoPoints = undefined;
    this.h3Points = undefined;
  }

  private buildGeojsonPoints(geojson: GeojsonLike) {
    const positions: number[] = [];
    const maxPoints = 25000;
    const total = this.countGeojsonPoints(geojson);
    const step = Math.max(1, Math.floor(total / maxPoints));
    let idx = 0;
    const pushPoint = (lng: number, lat: number) => {
      if (idx % step === 0) {
        const v = this.latLngToVector(lat, lng, this.baseRadius);
        positions.push(v.x, v.y, v.z);
      }
      idx += 1;
    };
    (geojson.features ?? []).forEach((feature) => {
      const geometry = feature.geometry;
      if (!geometry) return;
      const polygons =
        geometry.type === "Polygon"
          ? [geometry.coordinates]
          : geometry.type === "MultiPolygon"
            ? geometry.coordinates
            : [];
      polygons.forEach((coords: number[][][]) => {
        coords.forEach((ring) => {
          ring.forEach(([lng, lat]) => pushPoint(lng, lat));
        });
      });
    });
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute(
      "position",
      new THREE.Float32BufferAttribute(positions, 3),
    );
    const material = new THREE.PointsMaterial({
      color: 0xffffff,
      size: 0.02,
      sizeAttenuation: true,
      transparent: true,
      opacity: 0.6,
    });
    return new THREE.Points(geometry, material);
  }

  private buildH3Points(cells: string[]) {
    const positions: number[] = [];
    const maxPoints = 35000;
    const step = Math.max(1, Math.floor(cells.length / maxPoints));
    cells.forEach((cell, i) => {
      if (i % step !== 0) return;
      const [lat, lng] = cellToLatLng(cell);
      const v = this.latLngToVector(lat, lng, this.baseRadius + 0.03);
      positions.push(v.x, v.y, v.z);
    });
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute(
      "position",
      new THREE.Float32BufferAttribute(positions, 3),
    );
    const material = new THREE.PointsMaterial({
      color: 0x4cff9a,
      size: 0.03,
      sizeAttenuation: true,
      transparent: true,
      opacity: 0.85,
    });
    return new THREE.Points(geometry, material);
  }

  private countGeojsonPoints(geojson: GeojsonLike) {
    let count = 0;
    (geojson.features ?? []).forEach((feature) => {
      const geometry = feature.geometry;
      if (!geometry) return;
      const polygons =
        geometry.type === "Polygon"
          ? [geometry.coordinates]
          : geometry.type === "MultiPolygon"
            ? geometry.coordinates
            : [];
      polygons.forEach((coords: number[][][]) => {
        coords.forEach((ring) => {
          count += ring.length;
        });
      });
    });
    return count;
  }

  private geojsonToCells(geojson: GeojsonLike, resolution: number) {
    const cells = new Set<string>();
    (geojson.features ?? []).forEach((feature) => {
      const geometry = feature.geometry;
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

  private latLngToVector(lat: number, lng: number, radius: number) {
    const phi = (90 - lat) * (Math.PI / 180);
    const theta = (lng + 180) * (Math.PI / 180);
    return new THREE.Vector3(
      radius * Math.sin(phi) * Math.cos(theta),
      radius * Math.cos(phi),
      radius * Math.sin(phi) * Math.sin(theta),
    );
  }
}

export function mountGeoTools(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="GeoTools section: GeoJSON to H3">
      <h2>GeoTools</h2>
      <p data-typing-subtitle></p>
    </div>
    <div id="geotools-config-panel" class="earth-config-panel" hidden></div>
  `;

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Upload GeoJSON and convert to H3.",
    "Preview outlines and cell centers.",
    "Download a precomputed H3 layer.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new GeoToolsVisualization(container);
  let currentGeojson: GeojsonLike | null = null;
  let currentResolution = 3;

  const applyGeojson = () => {
    if (!currentGeojson) return;
    viz.setGeojson(currentGeojson, currentResolution);
  };
  const config = setupGeoToolsConfig({
    currentResolution,
    onResolutionChange: (value) => {
      currentResolution = value;
      if (currentGeojson) applyGeojson();
    },
    onFile: async (file) => {
      const text = await file.text();
      currentGeojson = JSON.parse(text);
      applyGeojson();
    },
    onConvert: () => {
      if (currentGeojson) applyGeojson();
    },
    onDownload: () => {
      const cells = viz.getCells();
      if (!cells.length) return;
      const payload = {
        resolution: currentResolution,
        cells,
        createdAt: new Date().toISOString(),
      };
      const blob = new Blob([JSON.stringify(payload)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "land.h3.json";
      link.click();
      URL.revokeObjectURL(url);
    },
    getStatusText: () => {
      const cells = viz.getCells();
      return currentGeojson
        ? `${cells.length.toLocaleString()} cells @ r${currentResolution}`
        : "No data loaded";
    },
  });

  fetch("/land.geojson")
    .then((res) => res.json())
    .then((geojson) => {
      currentGeojson = geojson;
      applyGeojson();
      config.updateStatus();
    })
    .catch(() => {
      // No default geojson available.
    });

  return {
    dispose: () => {
      viz.dispose();
      config.dispose();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
