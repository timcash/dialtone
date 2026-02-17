import * as THREE from 'three';

export type DagCameraView = 'iso' | 'top' | 'side' | 'front';

export class DagStageCamera {
  private readonly lookYOffset = -3;

  constructor(private readonly camera: THREE.PerspectiveCamera) {}

  private applyViewPosition(center: THREE.Vector3, dist: number, view: DagCameraView) {
    if (view === 'top') {
      this.camera.position.set(center.x, center.y + dist * 1.35, center.z + 0.01);
    } else if (view === 'side') {
      this.camera.position.set(center.x + dist * 1.2, center.y + dist * 0.42, center.z);
    } else if (view === 'front') {
      this.camera.position.set(center.x, center.y + dist * 0.42, center.z + dist * 1.2);
    } else {
      this.camera.position.set(center.x + dist * 0.75, center.y + dist * 0.95, center.z + dist * 0.75);
    }
  }

  framePoint(center: THREE.Vector3, maxDim: number, view: DagCameraView) {
    const fov = THREE.MathUtils.degToRad(this.camera.fov);
    const aspectScale = this.camera.aspect < 1 ? 1 / this.camera.aspect : 1;
    const dist = ((maxDim * aspectScale) / (2 * Math.tan(fov / 2))) * 1.2 + 14;
    const target = center.clone();
    target.y += this.lookYOffset;
    this.applyViewPosition(center, dist, view);
    this.camera.lookAt(target);
    this.camera.updateProjectionMatrix();
  }

  framePointFixed(center: THREE.Vector3, fixedDistance: number, view: DagCameraView) {
    const target = center.clone();
    target.y += this.lookYOffset;
    this.applyViewPosition(center, fixedDistance, view);
    this.camera.lookAt(target);
    this.camera.updateProjectionMatrix();
  }
}
