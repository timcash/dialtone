import * as THREE from "three";
import { polygonToCells, cellToLatLng } from "h3-js";
import { FpsCounter } from "./fps";
import { GpuTimer } from "./gpu_timer";
import { VisibilityMixin } from "./section";
import { startTyping } from "./typing";

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

  const controls = document.querySelector(".top-right-controls");
  const toggle = document.createElement("button");
  toggle.id = "geotools-config-toggle";
  toggle.className = "earth-config-toggle";
  toggle.type = "button";
  toggle.setAttribute("aria-expanded", "false");
  toggle.textContent = "Config";
  controls?.prepend(toggle);

  const panel = document.getElementById("geotools-config-panel") as HTMLDivElement | null;
  if (panel && toggle) {
    const setOpen = (open: boolean) => {
      panel.hidden = !open;
      panel.style.display = open ? "grid" : "none";
      toggle.setAttribute("aria-expanded", String(open));
    };
    setOpen(false);
    toggle.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      setOpen(panel.hidden);
    });
  }

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
  let statusValue: HTMLSpanElement | null = null;

  const updateStatus = (statusEl: HTMLSpanElement) => {
    const cells = viz.getCells();
    statusEl.textContent = currentGeojson
      ? `${cells.length.toLocaleString()} cells @ r${currentResolution}`
      : "No data loaded";
  };

  const applyGeojson = (statusEl: HTMLSpanElement) => {
    if (!currentGeojson) return;
    viz.setGeojson(currentGeojson, currentResolution);
    updateStatus(statusEl);
  };

  const makeButton = (label: string) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "earth-config-toggle";
    button.textContent = label;
    return button;
  };

  if (panel) {
    const row = document.createElement("div");
    row.className = "earth-config-row about-config-row";
    const label = document.createElement("label");
    label.className = "earth-config-label";
    label.htmlFor = "geotools-file";
    label.textContent = "GeoJSON";
    const fileInput = document.createElement("input");
    fileInput.type = "file";
    fileInput.id = "geotools-file";
    fileInput.accept = ".json,.geojson,application/geo+json";
    row.appendChild(label);
    row.appendChild(fileInput);
    panel.appendChild(row);

    const resolutionRow = document.createElement("div");
    resolutionRow.className = "earth-config-row about-config-row";
    const resolutionLabel = document.createElement("label");
    const resolutionId = "geotools-resolution";
    resolutionLabel.className = "earth-config-label";
    resolutionLabel.htmlFor = resolutionId;
    resolutionLabel.textContent = "H3 Resolution";
    const resolutionInput = document.createElement("input");
    resolutionInput.type = "range";
    resolutionInput.id = resolutionId;
    resolutionInput.min = "0";
    resolutionInput.max = "5";
    resolutionInput.step = "1";
    resolutionInput.value = `${currentResolution}`;
    resolutionRow.appendChild(resolutionLabel);
    resolutionRow.appendChild(resolutionInput);
    const resolutionValue = document.createElement("span");
    resolutionValue.className = "earth-config-value";
    resolutionValue.textContent = `${currentResolution}`;
    resolutionRow.appendChild(resolutionValue);
    panel.appendChild(resolutionRow);

    const statusRow = document.createElement("div");
    statusRow.className = "earth-config-row about-config-row";
    const statusLabel = document.createElement("label");
    statusLabel.className = "earth-config-label";
    statusLabel.textContent = "Status";
    statusRow.appendChild(statusLabel);
    statusValue = document.createElement("span");
    statusValue.className = "earth-config-value";
    statusRow.appendChild(statusValue);
    panel.appendChild(statusRow);

    const buttonsRow = document.createElement("div");
    buttonsRow.className = "earth-config-row about-config-row";
    const buttonsLabel = document.createElement("label");
    buttonsLabel.className = "earth-config-label";
    buttonsLabel.textContent = "Actions";
    const convertButton = makeButton("Convert");
    const downloadButton = makeButton("Download H3");
    buttonsRow.appendChild(buttonsLabel);
    buttonsRow.appendChild(convertButton);
    buttonsRow.appendChild(downloadButton);
    panel.appendChild(buttonsRow);

    fileInput.addEventListener("change", async () => {
      const file = fileInput.files?.[0];
      if (!file) return;
      const text = await file.text();
      currentGeojson = JSON.parse(text);
      applyGeojson(statusValue);
    });

    resolutionInput.addEventListener("input", () => {
      currentResolution = parseInt(resolutionInput.value, 10);
      resolutionValue.textContent = `${currentResolution}`;
      if (currentGeojson) applyGeojson(statusValue);
    });

    convertButton.addEventListener("click", () => {
      if (currentGeojson) applyGeojson(statusValue);
    });

    downloadButton.addEventListener("click", () => {
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
    });

    updateStatus(statusValue);
  }

  fetch("/land.geojson")
    .then((res) => res.json())
    .then((geojson) => {
      currentGeojson = geojson;
      if (statusValue) {
        applyGeojson(statusValue);
      } else {
        viz.setGeojson(currentGeojson, currentResolution);
      }
    })
    .catch(() => {
      // No default geojson available.
    });

  return {
    dispose: () => {
      viz.dispose();
      toggle.remove();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
