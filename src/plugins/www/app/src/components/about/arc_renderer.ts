import * as THREE from "three";
import { Line2 } from "three/examples/jsm/lines/Line2.js";
import { LineGeometry } from "three/examples/jsm/lines/LineGeometry.js";
import { LineMaterial } from "three/examples/jsm/lines/LineMaterial.js";

type Arc = {
  line: Line2;
  startMs: number;
  getPoints?: () => THREE.Vector3[];
};

export class ArcRenderer {
  private scene: THREE.Scene;
  private sparkMaterial: LineMaterial;
  private linkMaterial: LineMaterial;
  private sparkArcs: Arc[] = [];
  private linkArcs: Arc[] = [];
  private sparkDurationMs: number;
  private linkDurationMs: number;

  constructor(
    scene: THREE.Scene,
    options: {
      sparkColor: number;
      linkColor: number;
      sparkWidth: number;
      linkWidth: number;
      sparkDurationMs: number;
      linkDurationMs: number;
    }
  ) {
    this.scene = scene;
    this.sparkDurationMs = options.sparkDurationMs;
    this.linkDurationMs = options.linkDurationMs;
    this.sparkMaterial = new LineMaterial({
      color: options.sparkColor,
      linewidth: options.sparkWidth,
      transparent: true,
      opacity: 1,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    this.linkMaterial = new LineMaterial({
      color: options.linkColor,
      linewidth: options.linkWidth,
      transparent: true,
      opacity: 0.8,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    this.updateResolution(window.innerWidth, window.innerHeight);
  }

  updateResolution(width: number, height: number) {
    const pr = window.devicePixelRatio;
    this.sparkMaterial.resolution.set(Math.max(1, width * pr), Math.max(1, height * pr));
    this.linkMaterial.resolution.set(Math.max(1, width * pr), Math.max(1, height * pr));
  }

  addSparkArc(points: THREE.Vector3[], now: number, getPoints?: () => THREE.Vector3[]) {
    const line = this.createArc(points, this.sparkMaterial);
    this.scene.add(line);
    this.sparkArcs.push({ line, startMs: now, getPoints });
  }

  addLinkArc(points: THREE.Vector3[], now: number, getPoints?: () => THREE.Vector3[]) {
    const line = this.createArc(points, this.linkMaterial);
    this.scene.add(line);
    this.linkArcs.push({ line, startMs: now, getPoints });
  }

  update(now: number) {
    this.sparkArcs = this.updateArcs(now, this.sparkArcs, this.sparkDurationMs);
    this.linkArcs = this.updateArcs(now, this.linkArcs, this.linkDurationMs);
  }

  clear() {
    this.sparkArcs.forEach(({ line }) => this.disposeLine(line));
    this.linkArcs.forEach(({ line }) => this.disposeLine(line));
    this.sparkArcs = [];
    this.linkArcs = [];
  }

  private updateArcs(now: number, arcs: Arc[], durationMs: number) {
    return arcs.filter((arc) => {
      const { line, startMs, getPoints } = arc;
      if (getPoints) {
        this.updateArcPoints(line, getPoints());
      }
      const age = now - startMs;
      if (age >= durationMs) {
        this.disposeLine(line);
        return false;
      }
      const opacity = 1 - age / durationMs;
      const material = line.material as LineMaterial;
      material.opacity = opacity;
      return true;
    });
  }

  private createArc(points: THREE.Vector3[], material: LineMaterial) {
    const geometry = new LineGeometry();
    const positions = points.flatMap((p) => [p.x, p.y, p.z]);
    geometry.setPositions(positions);
    const line = new Line2(geometry, material);
    line.computeLineDistances();
    return line;
  }

  private updateArcPoints(line: Line2, points: THREE.Vector3[]) {
    const geometry = line.geometry as LineGeometry;
    const positions = points.flatMap((p) => [p.x, p.y, p.z]);
    geometry.setPositions(positions);
    line.computeLineDistances();
  }

  private disposeLine(line: Line2) {
    line.geometry.dispose();
    this.scene.remove(line);
  }
}
