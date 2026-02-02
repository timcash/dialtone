import * as THREE from "three";
import { cellToBoundary, latLngToCell } from "h3-js";

const DEG_TO_RAD = Math.PI / 180;

export type HexLayerSettings = {
  radiusOffset: number;
  count: number;
  resolution: number;
  ratePerSecond: number;
  durationSeconds: number;
  palette: THREE.Color[];
  opacity?: number;
};

export class HexLayer {
  mesh: THREE.Mesh;
  material: THREE.ShaderMaterial;
  private radius: number;
  private durationSeconds: number;
  private resolution: number;
  private hexes: Array<{
    startTime: number;
    vertexStart: number;
    vertexCount: number;
    sides: number;
  }> = [];
  private positionAttr!: THREE.BufferAttribute;
  private startAttr!: THREE.BufferAttribute;

  constructor(baseRadius: number, settings: HexLayerSettings) {
    this.radius = baseRadius + settings.radiusOffset;
    this.durationSeconds = settings.durationSeconds;
    this.resolution = settings.resolution;
    const { geometry, material } = this.buildGeometry(this.radius, settings);
    this.mesh = new THREE.Mesh(geometry, material);
    this.material = material;
    this.positionAttr = geometry.getAttribute(
      "position",
    ) as THREE.BufferAttribute;
    this.startAttr = geometry.getAttribute("aStart") as THREE.BufferAttribute;
  }

  update(timeSeconds: number) {
    this.material.uniforms.uTime.value = timeSeconds;
    let updated = false;
    const positions = this.positionAttr.array as Float32Array;
    const starts = this.startAttr.array as Float32Array;
    this.hexes.forEach((hex) => {
      if (timeSeconds - hex.startTime >= this.durationSeconds) {
        const boundary = this.randomBoundary(hex.sides);
        if (!boundary) {
          return;
        }
        this.writeHexVertices(boundary, hex.vertexStart, positions);
        hex.startTime = timeSeconds;
        this.writeHexStarts(
          hex.vertexStart,
          hex.vertexCount,
          timeSeconds,
          starts,
        );
        updated = true;
      }
    });
    if (updated) {
      this.positionAttr.needsUpdate = true;
      this.startAttr.needsUpdate = true;
    }
  }

  private buildGeometry(radius: number, settings: HexLayerSettings) {
    const positions: number[] = [];
    const colors: number[] = [];
    const starts: number[] = [];
    const palette = settings.palette.length
      ? settings.palette
      : [
          new THREE.Color(0.7, 0.7, 0.72),
          new THREE.Color(0.4, 0.4, 0.45),
          new THREE.Color(0.1, 0.1, 0.12),
        ];
    const cells = this.sampleHexCells(settings.count, settings.resolution);
    this.shuffleCells(cells);
    cells.forEach((cell, index) => {
      const boundary = cellToBoundary(cell, true);
      const tint = palette[index % palette.length];
      const start = index / settings.ratePerSecond;
      const vertexStart = positions.length / 3;
      const center = boundary
        .reduce(
          (acc, [lng, lat]) => acc.add(this.latLngToVector(lat, lng, radius)),
          new THREE.Vector3(),
        )
        .divideScalar(boundary.length);

      for (let i = 0; i < boundary.length; i += 1) {
        const [lngA, latA] = boundary[i];
        const [lngB, latB] = boundary[(i + 1) % boundary.length];
        const a = this.latLngToVector(latA, lngA, radius);
        const b = this.latLngToVector(latB, lngB, radius);
        positions.push(
          center.x,
          center.y,
          center.z,
          a.x,
          a.y,
          a.z,
          b.x,
          b.y,
          b.z,
        );
        starts.push(start, start, start);
        colors.push(
          tint.r,
          tint.g,
          tint.b,
          tint.r,
          tint.g,
          tint.b,
          tint.r,
          tint.g,
          tint.b,
        );
      }

      const vertexCount = boundary.length * 3;
      this.hexes.push({
        startTime: start,
        vertexStart,
        vertexCount,
        sides: boundary.length,
      });
    });

    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute(
      "position",
      new THREE.Float32BufferAttribute(positions, 3),
    );
    geometry.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));
    geometry.setAttribute(
      "aStart",
      new THREE.Float32BufferAttribute(starts, 1),
    );
    const material = new THREE.ShaderMaterial({
      transparent: true,
      vertexColors: true,
      side: THREE.DoubleSide,
      uniforms: {
        uTime: { value: 0 },
        uDuration: { value: settings.durationSeconds },
        uOpacity: { value: settings.opacity ?? 0.7 },
      },
      vertexShader: `
        attribute float aStart;
        varying vec3 vColor;
        varying float vStart;
        void main() {
          vColor = color;
          vStart = aStart;
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragmentShader: `
        precision mediump float;
        uniform float uTime;
        uniform float uDuration;
        uniform float uOpacity;
        varying vec3 vColor;
        varying float vStart;
        void main() {
          float age = uTime - vStart;
          float isActive = step(0.0, age) * step(age, uDuration);
          float fadeIn = smoothstep(0.0, 0.8, age);
          float fadeOut = 1.0 - smoothstep(uDuration - 0.8, uDuration, age);
          float alpha = isActive * fadeIn * fadeOut * uOpacity;
          gl_FragColor = vec4(vColor, alpha);
        }
      `,
    });

    return { geometry, material };
  }

  private sampleHexCells(count: number, resolution: number) {
    const goldenAngle = Math.PI * (3 - Math.sqrt(5));
    const cells = new Set<string>();
    let i = 0;
    while (cells.size < count && i < count * 6) {
      const t = (i + 0.5) / count;
      const y = 1 - 2 * t;
      const r = Math.sqrt(1 - y * y);
      const theta = goldenAngle * i;
      const x = Math.cos(theta) * r;
      const z = Math.sin(theta) * r;
      const lat = Math.asin(y) / DEG_TO_RAD;
      const lng = Math.atan2(z, x) / DEG_TO_RAD;
      cells.add(latLngToCell(lat, lng, resolution));
      i += 1;
    }
    return Array.from(cells).slice(0, count);
  }

  private shuffleCells(cells: string[]) {
    for (let i = cells.length - 1; i > 0; i -= 1) {
      const j = Math.floor(Math.random() * (i + 1));
      [cells[i], cells[j]] = [cells[j], cells[i]];
    }
  }

  private randomBoundary(sides: number) {
    for (let attempt = 0; attempt < 40; attempt += 1) {
      const lat = Math.random() * 180 - 90;
      const lng = Math.random() * 360 - 180;
      const cell = latLngToCell(lat, lng, this.resolution);
      const boundary = cellToBoundary(cell, true);
      if (boundary.length === sides) {
        return boundary;
      }
    }
    return null;
  }

  private writeHexVertices(
    boundary: number[][],
    vertexStart: number,
    positions: Float32Array,
  ) {
    const center = boundary
      .reduce(
        (acc, [lng, lat]) =>
          acc.add(this.latLngToVector(lat, lng, this.radius)),
        new THREE.Vector3(),
      )
      .divideScalar(boundary.length);
    let writeIndex = vertexStart * 3;
    for (let i = 0; i < boundary.length; i += 1) {
      const [lngA, latA] = boundary[i];
      const [lngB, latB] = boundary[(i + 1) % boundary.length];
      const a = this.latLngToVector(latA, lngA, this.radius);
      const b = this.latLngToVector(latB, lngB, this.radius);
      positions[writeIndex++] = center.x;
      positions[writeIndex++] = center.y;
      positions[writeIndex++] = center.z;
      positions[writeIndex++] = a.x;
      positions[writeIndex++] = a.y;
      positions[writeIndex++] = a.z;
      positions[writeIndex++] = b.x;
      positions[writeIndex++] = b.y;
      positions[writeIndex++] = b.z;
    }
  }

  private writeHexStarts(
    vertexStart: number,
    vertexCount: number,
    startTime: number,
    starts: Float32Array,
  ) {
    for (let i = 0; i < vertexCount; i += 1) {
      starts[vertexStart + i] = startTime;
    }
  }

  private latLngToVector(lat: number, lng: number, radius: number) {
    const phi = (90 - lat) * DEG_TO_RAD;
    const theta = (lng + 180) * DEG_TO_RAD;
    return new THREE.Vector3(
      radius * Math.sin(phi) * Math.cos(theta),
      radius * Math.cos(phi),
      radius * Math.sin(phi) * Math.sin(theta),
    );
  }
}
