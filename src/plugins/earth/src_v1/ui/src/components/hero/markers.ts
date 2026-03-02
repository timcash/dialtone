import * as THREE from 'three';
import { latLngToCell, cellToBoundary } from 'h3-js';

const DEG_TO_RAD = Math.PI / 180;

export interface City {
  name: string;
  lat: number;
  lng: number;
}

export interface MarkerData {
  mesh: THREE.Mesh;
  anchor: THREE.Object3D;
  line: THREE.Line;
  cameraAnchor: THREE.Object3D;
  label: THREE.Sprite;
  hex: THREE.Line;
  camera: THREE.PerspectiveCamera;
}

export class MarkerManager {
  markers: MarkerData[] = [];
  
  constructor(
    private parent: THREE.Object3D, 
    private earthRadius: number, 
    private altitude: number,
    private aspect: number
  ) {}

  latLngToVector(lat: number, lng: number, radius: number) {
    const latRad = lat * DEG_TO_RAD;
    const lngRad = lng * DEG_TO_RAD;
    return new THREE.Vector3(
      radius * Math.cos(latRad) * Math.sin(lngRad),
      radius * Math.sin(latRad),
      radius * Math.cos(latRad) * Math.cos(lngRad),
    );
  }

  initCities(cities: City[]) {
    cities.forEach(city => {
      const groundPos = this.latLngToVector(city.lat, city.lng, this.earthRadius);
      const hoverPos = this.latLngToVector(city.lat, city.lng, this.earthRadius + this.altitude);
      const camHoverPos = this.latLngToVector(city.lat - 2, city.lng, this.earthRadius + this.altitude + 15);

      const anchor = new THREE.Object3D();
      anchor.position.copy(groundPos);
      this.parent.add(anchor);

      const cameraAnchor = new THREE.Object3D();
      cameraAnchor.position.copy(camHoverPos);
      this.parent.add(cameraAnchor);

      // Create a dedicated camera for this marker
      const camera = new THREE.PerspectiveCamera(75, this.aspect, 0.1, 10000);
      cameraAnchor.add(camera); // Parent camera to anchor so it moves with earth
      camera.lookAt(groundPos);

      const mesh = new THREE.Mesh(
        new THREE.SphereGeometry(0.8, 16, 16),
        new THREE.MeshBasicMaterial({ color: 0x7bf2d8, transparent: true, opacity: 0.5 })
      );
      mesh.position.copy(hoverPos);
      mesh.renderOrder = 10;
      this.parent.add(mesh);

      const cell = latLngToCell(city.lat, city.lng, 4);
      const boundary = cellToBoundary(cell, true);
      const hexPoints = boundary.map(([lng, lat]) => this.latLngToVector(lat, lng, this.earthRadius + 0.2));
      hexPoints.push(hexPoints[0]);
      const hex = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(hexPoints),
        new THREE.LineBasicMaterial({ color: 0x7bf2d8, transparent: true, opacity: 0.8 })
      );
      hex.renderOrder = 10;
      this.parent.add(hex);

      const line = new THREE.Line(
        new THREE.BufferGeometry().setFromPoints([groundPos, hoverPos]),
        new THREE.LineBasicMaterial({ color: 0x7bf2d8, transparent: true, opacity: 0.3 })
      );
      line.renderOrder = 10;
      this.parent.add(line);

      const label = this.createLabel(city.name);
      label.position.copy(this.latLngToVector(city.lat + 1.5, city.lng - 5, this.earthRadius + this.altitude));
      label.renderOrder = 11;
      this.parent.add(label);

      this.markers.push({ mesh, anchor, line, cameraAnchor, label, hex, camera });
    });
  }

  private createLabel(text: string): THREE.Sprite {
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('Canvas context not found');

    const fontSize = 48;
    ctx.font = `Bold ${fontSize}px Inter, system-ui, sans-serif`;
    const metrics = ctx.measureText(text);
    const width = metrics.width + 20;
    const height = fontSize + 20;

    canvas.width = width;
    canvas.height = height;
    ctx.clearRect(0, 0, width, height);
    ctx.font = `Bold ${fontSize}px Inter, system-ui, sans-serif`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.shadowColor = 'rgba(0, 0, 0, 0.8)';
    ctx.shadowBlur = 8;
    ctx.fillStyle = 'white';
    ctx.fillText(text, width / 2, height / 2);

    const texture = new THREE.CanvasTexture(canvas);
    const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ 
      map: texture, 
      transparent: true, 
      depthTest: true,
      depthWrite: false
    }));
    sprite.scale.set(width / 20, height / 20, 1);
    return sprite;
  }

  update(time: number) {
    const blink = 1.0 + Math.sin(time * 0.002) * 0.4;
    for (let i = 0; i < this.markers.length; i++) {
      this.markers[i].mesh.scale.setScalar(blink);
    }
  }

  setAspect(aspect: number) {
    this.markers.forEach(m => {
      m.camera.aspect = aspect;
      m.camera.updateProjectionMatrix();
    });
  }
}
