type GridCell = { y: number; z: number };

const NEIGHBORS = [
  [0, -1, -1], [0, -1, 0], [0, -1, 1],
  [0, 0, -1], [0, 0, 1],
  [0, 1, -1], [0, 1, 0], [0, 1, 1],
];

export class VisionGrid {
  readonly nx: number;
  readonly ny: number;
  readonly nz: number;
  readonly total: number;
  gridA: Uint8Array;
  gridB: Uint8Array;
  glowA: Float32Array;
  glowDurationMs = 1000;
  birthSet = new Set<number>([3]);
  surviveSet = new Set<number>([2, 3]);
  private rng: () => number;

  constructor(nx: number, ny: number, nz: number, rng: () => number) {
    this.nx = nx;
    this.ny = ny;
    this.nz = nz;
    this.total = nx * ny * nz;
    this.gridA = new Uint8Array(this.total);
    this.gridB = new Uint8Array(this.total);
    this.glowA = new Float32Array(this.total);
    this.rng = rng;
  }

  setRng(rng: () => number) {
    this.rng = rng;
  }

  clear() {
    this.gridA.fill(0);
    this.gridB.fill(0);
    this.glowA.fill(0);
  }

  index(i: number, j: number, k: number): number {
    const ii = ((i % this.nx) + this.nx) % this.nx;
    const jj = ((j % this.ny) + this.ny) % this.ny;
    const kk = ((k % this.nz) + this.nz) % this.nz;
    return ii + jj * this.nx + kk * this.nx * this.ny;
  }

  randomSeed(density: number) {
    for (let i = 0; i < this.total; i++) {
      this.gridA[i] = this.rng() < density ? 1 : 0;
    }
  }

  seedExactly(n: number) {
    this.gridA.fill(0);
    const indices = Array.from({ length: this.total }, (_, i) => i);
    for (let i = 0; i < n && i < indices.length; i++) {
      const r = i + Math.floor(this.rng() * (indices.length - i));
      [indices[i], indices[r]] = [indices[r], indices[i]];
      this.gridA[indices[i]] = 1;
    }
  }

  centerSeed(radius: number, density: number) {
    this.gridA.fill(0);
    const cx = this.nx / 2;
    const cy = this.ny / 2;
    const cz = this.nz / 2;
    const loX = Math.max(0, Math.floor(cx - radius));
    const hiX = Math.min(this.nx, Math.ceil(cx + radius));
    const loY = Math.max(0, Math.floor(cy - radius));
    const hiY = Math.min(this.ny, Math.ceil(cy + radius));
    const loZ = Math.max(0, Math.floor(cz - radius));
    const hiZ = Math.min(this.nz, Math.ceil(cz + radius));
    for (let i = loX; i < hiX; i++) {
      for (let j = loY; j < hiY; j++) {
        for (let k = loZ; k < hiZ; k++) {
          if (this.rng() < density) this.gridA[this.index(i, j, k)] = 1;
        }
      }
    }
  }

  step() {
    const read = this.gridA;
    const write = this.gridB;
    for (let i = 0; i < this.nx; i++) {
      for (let j = 0; j < this.ny; j++) {
        for (let k = 0; k < this.nz; k++) {
          let neighbors = 0;
          for (const [di, dj, dk] of NEIGHBORS) {
            neighbors += read[this.index(i + di, j + dj, k + dk)];
          }
          const idx = this.index(i, j, k);
          const alive = read[idx];
          if (alive) {
            write[idx] = this.surviveSet.has(neighbors) ? 1 : 0;
          } else {
            write[idx] = this.birthSet.has(neighbors) ? 1 : 0;
          }
        }
      }
    }
    const t = this.gridA;
    this.gridA = this.gridB;
    this.gridB = t;
  }

  decayGlow(deltaMs: number) {
    for (let i = 0; i < this.glowA.length; i++) {
      if (this.glowA[i] > 0) {
        this.glowA[i] = Math.max(0, this.glowA[i] - deltaMs);
      }
    }
  }

  injectLightTrail(cells: GridCell[]) {
    cells.forEach(({ y, z }) => {
      const idx = this.index(0, y, z);
      this.gridA[idx] = 1;
      this.glowA[idx] = this.glowDurationMs;
    });
  }

  injectGlider(cells: GridCell[]) {
    const glider = [
      [0, 1],
      [1, 2],
      [2, 0],
      [2, 1],
      [2, 2],
    ];
    cells.forEach(({ y, z }) => {
      for (const [dj, dk] of glider) {
        const idx = this.index(0, y + dj, z + dk);
        this.gridA[idx] = 1;
        this.glowA[idx] = this.glowDurationMs;
      }
    });
  }

  injectBurst(count: number) {
    const glider = [
      [0, 1],
      [1, 2],
      [2, 0],
      [2, 1],
      [2, 2],
    ];
    for (let n = 0; n < count; n++) {
      const j = Math.floor(this.rng() * this.ny);
      const k = Math.floor(this.rng() * this.nz);
      for (const [dj, dk] of glider) {
        this.gridA[this.index(0, j + dj, k + dk)] = 1;
      }
    }
  }

  injectSplash() {
    const splash = [
      [0, 1], [0, 2], [0, 3],
      [1, 0], [1, 2], [1, 4],
      [2, 1], [2, 2], [2, 3],
      [3, 0], [3, 2], [3, 4],
      [4, 1], [4, 2], [4, 3],
    ];
    const j = Math.floor(this.rng() * this.ny);
    const k = Math.floor(this.rng() * this.nz);
    for (const [dj, dk] of splash) {
      this.gridA[this.index(0, j + dj, k + dk)] = 1;
    }
  }
}
