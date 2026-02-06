import * as THREE from "three";

export class SearchLights {
  private orbitLight: THREE.PointLight;
  private orbitSphere: THREE.Mesh;
  private orbitLights: THREE.PointLight[] = [];
  private orbitSpheres: THREE.Mesh[] = [];
  private lightningMaterial = new THREE.LineBasicMaterial({
    color: 0xfff2d6,
    transparent: true,
    opacity: 0.9,
  });
  private lightningBolts: Array<{ line: THREE.Line; startMs: number }> = [];
  private searchLights: Array<{
    light: THREE.PointLight;
    name: string;
    intensity: number;
    energy: number;
    lastSparkMs: number;
    planeSide: number;
    start: THREE.Vector3;
    target: THREE.Vector3;
    control: THREE.Vector3;
    driftOffset: THREE.Vector3;
    driftAmp: number;
    startTime: number;
    travelMs: number;
    pauseMs: number;
  }> = [];
  private scene: THREE.Scene;
  private readonly gridSize: { x: number; y: number; z: number };
  private lastUpdateMs = 0;
  private gridRotationY: number;
  private planeOffset: number;
  private lightningDurationMs = 220;
  private sparkCooldownMs = 300;
  private energyDrain = 0.18;
  private energyRegen = 0.08;
  private lowColor = new THREE.Color(0xffd7b5);
  private highColor = new THREE.Color(0xffffff);
  private chargeThreshold = 0.12;
  private chargeBoost = 0.22;
  private minIntensityScale = 0.05;
  private brightness = 1.35;
  private rng: () => number;
  private activeCount = 4;
  private travelBaseMs = 12000;
  private travelJitterMs = 4000;
  private dwellBaseMs = 6000;
  private dwellJitterMs = 6000;
  private wanderBase = 8;
  private wanderJitter = 4;

  constructor(
    scene: THREE.Scene,
    gridSize: { x: number; y: number; z: number },
    gridRotationY = 0,
    rng: () => number = Math.random
  ) {
    this.scene = scene;
    this.gridSize = gridSize;
    this.gridRotationY = gridRotationY;
    this.planeOffset = 0.6;
    this.rng = rng;

    this.orbitLight = new THREE.PointLight(0xffffff, 4.5, 320, 1.2);
    this.orbitLight.position.set(65, 35, 0);
    this.scene.add(this.orbitLight);

    const accentLights = [
      { color: 0xf8f5ef, intensity: 2.2, distance: 220, decay: 1.1 },
      { color: 0xfff2d6, intensity: 2.0, distance: 240, decay: 1.1 },
      { color: 0xffd7b5, intensity: 1.8, distance: 200, decay: 1.2 },
      { color: 0xfaf2e6, intensity: 1.6, distance: 220, decay: 1.2 },
      { color: 0xffe8c7, intensity: 1.4, distance: 200, decay: 1.2 },
    ];
    accentLights.forEach((config) => {
      const light = new THREE.PointLight(
        config.color,
        config.intensity,
        config.distance,
        config.decay
      );
      this.scene.add(light);
      this.orbitLights.push(light);
    });

    this.orbitSphere = new THREE.Mesh(
      new THREE.SphereGeometry(1.6, 24, 24),
      new THREE.MeshStandardMaterial({
        color: 0xfdf7ee,
        emissive: 0xffffff,
        emissiveIntensity: 0.4,
        roughness: 0.25,
        metalness: 0.2,
      })
    );
    this.orbitSphere.position.set(90, 35, 0);
    this.scene.add(this.orbitSphere);
    this.orbitSpheres.push(this.orbitSphere);

    this.orbitLights.forEach((light) => {
      const sphere = new THREE.Mesh(
        new THREE.SphereGeometry(1.2, 20, 20),
        new THREE.MeshStandardMaterial({
          color: light.color,
          emissive: light.color,
          emissiveIntensity: 0.6,
          roughness: 0.2,
          metalness: 0.1,
        })
      );
      sphere.position.copy(light.position);
      this.scene.add(sphere);
      this.orbitSpheres.push(sphere);
    });

    this.initSearchLights();
    this.lastUpdateMs = performance.now();
  }

  private toWorld(local: THREE.Vector3): THREE.Vector3 {
    return local.applyAxisAngle(new THREE.Vector3(0, 1, 0), this.gridRotationY);
  }

  private gridPoint(y: number, z: number, planeSide = 1, offset = this.planeOffset): THREE.Vector3 {
    const halfX = this.gridSize.x / 2;
    const halfY = this.gridSize.y / 2;
    const halfZ = this.gridSize.z / 2;
    const clampedY = THREE.MathUtils.clamp(y, 0, this.gridSize.y - 1);
    const clampedZ = THREE.MathUtils.clamp(z, 0, this.gridSize.z - 1);
    const local = new THREE.Vector3(-halfX, clampedY - halfY, clampedZ - halfZ);
    const world = this.toWorld(local);
    const normal = new THREE.Vector3(1, 0, 0).applyAxisAngle(
      new THREE.Vector3(0, 1, 0),
      this.gridRotationY
    );
    return world.add(normal.multiplyScalar(offset * planeSide));
  }

  private gridPlanePoint(y: number, z: number): THREE.Vector3 {
    return this.gridPoint(y, z, 1, 0);
  }

  private randomCellPosition(planeSide: number): THREE.Vector3 {
    const j = Math.floor(this.rng() * this.gridSize.y);
    const k = Math.floor(this.rng() * this.gridSize.z);
    return this.gridPoint(j, k, planeSide);
  }

  private initSearchLights() {
    const now = performance.now();
    const allLights = [this.orbitLight, ...this.orbitLights];
    allLights.forEach((light, index) => {
      const planeSide = this.rng() < 0.5 ? -1 : 1;
      const start = this.randomCellPosition(planeSide);
      const target = this.randomCellPosition(planeSide);
      const control = this.computeArcControl(start, target);
      light.position.copy(start);
      this.searchLights.push({
        light,
        name: `inspector-${index + 1}`,
        intensity: light.intensity,
        energy: 1,
        lastSparkMs: 0,
        planeSide,
        start,
        target,
        control,
        driftOffset: new THREE.Vector3(
          (this.rng() - 0.5) * 4,
          (this.rng() - 0.5) * 6,
          (this.rng() - 0.5) * 6
        ),
        driftAmp: this.wanderBase + this.rng() * this.wanderJitter,
        startTime: now,
        travelMs: this.travelBaseMs + this.rng() * this.travelJitterMs,
        pauseMs: this.dwellBaseMs + this.rng() * this.dwellJitterMs,
      });
    });
  }

  trackSpawn(y: number, z: number) {
    const now = performance.now();
    const primary = this.searchLights[0];
    if (!primary) return;
    const target = this.gridPoint(y, z, primary.planeSide);
    primary.start.copy(primary.light.position);
    primary.target.copy(target);
    primary.control.copy(this.computeArcControl(primary.start, primary.target));
    primary.startTime = now;
    primary.travelMs = this.travelBaseMs + Math.random() * this.travelJitterMs;
    primary.pauseMs = this.dwellBaseMs + Math.random() * this.dwellJitterMs;
  }

  private computeArcControl(start: THREE.Vector3, target: THREE.Vector3): THREE.Vector3 {
    const mid = start.clone().lerp(target, 0.5);
    const dir = target.clone().sub(start);
    const up = new THREE.Vector3(0, 1, 0);
    const perp = dir.clone().cross(up);
    if (perp.lengthSq() < 1e-6) {
      perp.set(0, 0, 1);
    } else {
      perp.normalize();
    }
    const arc = Math.min(this.wanderBase, dir.length() * 0.5) *
      (this.rng() < 0.5 ? -1 : 1);
    return mid.add(perp.multiplyScalar(arc));
  }

  private arcPoint(start: THREE.Vector3, control: THREE.Vector3, target: THREE.Vector3, t: number): THREE.Vector3 {
    const a = start.clone().lerp(control, t);
    const b = control.clone().lerp(target, t);
    return a.lerp(b, t);
  }

  update(now: number) {
    const dt = Math.max(0, (now - this.lastUpdateMs) / 1000);
    this.lastUpdateMs = now;
    this.applyLightVisibility();
    this.searchLights.forEach((state, index) => {
      if (index >= this.activeCount) return;
      const regenRate = state.energy <= this.chargeThreshold
        ? this.energyRegen + this.chargeBoost
        : this.energyRegen;
      state.energy = Math.min(1, state.energy + dt * regenRate);
      state.light.intensity =
        state.intensity *
        this.brightness *
        (this.minIntensityScale + (1 - this.minIntensityScale) * state.energy);
      state.light.color.copy(this.lowColor).lerp(this.highColor, state.energy);
      const elapsed = now - state.startTime;
      if (elapsed <= state.travelMs) {
        const t = elapsed / state.travelMs;
        const smooth = t * t * (3 - 2 * t);
        const base = this.arcPoint(state.start, state.control, state.target, smooth);
        const driftPhase = Math.sin(smooth * Math.PI);
        const drift = state.driftOffset.clone().multiplyScalar(driftPhase * state.driftAmp);
        state.light.position.copy(base.add(drift));
        return;
      }
      if (elapsed <= state.travelMs + state.pauseMs) {
        const lingerPhase =
          (elapsed - state.travelMs) / Math.max(1, state.pauseMs);
        const breathe = Math.sin(lingerPhase * Math.PI);
        const drift = state.driftOffset.clone().multiplyScalar(breathe * (state.driftAmp * 0.4));
        state.light.position.copy(state.target.clone().add(drift));
        return;
      }
      state.start.copy(state.target);
      state.planeSide *= -1;
      state.target.copy(this.randomCellPosition(state.planeSide));
      state.control.copy(this.computeArcControl(state.start, state.target));
      state.driftOffset.set(
        (this.rng() - 0.5) * 4,
        (this.rng() - 0.5) * 6,
        (this.rng() - 0.5) * 6
      );
      state.driftAmp = this.wanderBase + this.rng() * this.wanderJitter;
      state.startTime = now;
      state.travelMs = this.travelBaseMs + this.rng() * this.travelJitterMs;
      state.pauseMs = this.dwellBaseMs + this.rng() * this.dwellJitterMs;
    });

    this.orbitSpheres.forEach((sphere, index) => {
      const light =
        index === 0 ? this.orbitLight : this.orbitLights[index - 1];
      sphere.position.set(
        light.position.x,
        light.position.y,
        light.position.z
      );
      const mat = sphere.material as THREE.MeshStandardMaterial;
      mat.color.copy(light.color);
      mat.emissive.copy(light.color);
    });

    this.updateLightning(now);
  }

  getLightGridCells(): Array<{ y: number; z: number }> {
    const halfY = this.gridSize.y / 2;
    const halfZ = this.gridSize.z / 2;
    const axis = new THREE.Vector3(0, 1, 0);
    const cells = new Map<string, { y: number; z: number }>();
    this.searchLights.forEach((state, index) => {
      if (index >= this.activeCount) return;
      const local = state.light.position
        .clone()
        .applyAxisAngle(axis, -this.gridRotationY);
      const y = THREE.MathUtils.clamp(
        Math.round(local.y + halfY),
        0,
        this.gridSize.y - 1
      );
      const z = THREE.MathUtils.clamp(
        Math.round(local.z + halfZ),
        0,
        this.gridSize.z - 1
      );
      const key = `${y}:${z}`;
      cells.set(key, { y, z });
    });
    return Array.from(cells.values());
  }

  getPrimaryLightPosition(): THREE.Vector3 | null {
    if (this.searchLights.length === 0 || this.activeCount <= 0) return null;
    const state = this.searchLights[0];
    return state ? state.light.position.clone() : null;
  }

  spawnLightning(cells: Array<{ y: number; z: number }>) {
    const now = performance.now();
    const axis = new THREE.Vector3(0, 1, 0);
    const halfY = this.gridSize.y / 2;
    const halfZ = this.gridSize.z / 2;
    this.searchLights.forEach((state, index) => {
      if (index >= this.activeCount) return;
      if (now - state.lastSparkMs < this.sparkCooldownMs) return;
      const local = state.light.position
        .clone()
        .applyAxisAngle(axis, -this.gridRotationY);
      const y = THREE.MathUtils.clamp(
        Math.round(local.y + halfY),
        0,
        this.gridSize.y - 1
      );
      const z = THREE.MathUtils.clamp(
        Math.round(local.z + halfZ),
        0,
        this.gridSize.z - 1
      );
      if (!cells.some((cell) => cell.y === y && cell.z === z)) return;
      state.lastSparkMs = now;
      state.energy = Math.max(0.15, state.energy - this.energyDrain);
      const end = this.gridPlanePoint(y, z);
      const start = state.light.position.clone().lerp(end, 0.05);
      const mid = start.clone().lerp(end, 0.5);
      mid.add(
        new THREE.Vector3(
          (this.rng() - 0.5) * 0.8,
          (this.rng() - 0.5) * 0.8,
          (this.rng() - 0.5) * 0.8
        )
      );
      const geometry = new THREE.BufferGeometry().setFromPoints([
        start,
        mid,
        end,
      ]);
      const line = new THREE.Line(geometry, this.lightningMaterial);
      this.scene.add(line);
      this.lightningBolts.push({ line, startMs: now });
    });
  }

  setLightCount(count: number) {
    this.activeCount = THREE.MathUtils.clamp(Math.round(count), 1, this.searchLights.length);
    this.applyLightVisibility();
  }

  getMaxLights() {
    return this.searchLights.length;
  }

  setRng(rng: () => number) {
    this.rng = rng;
  }

  resetForSeed(rng: () => number) {
    this.setRng(rng);
    this.lightningBolts.forEach(({ line }) => {
      line.geometry.dispose();
      this.scene.remove(line);
    });
    this.lightningBolts = [];
    this.searchLights = [];
    this.initSearchLights();
    this.applyLightVisibility();
  }

  setDwellSeconds(seconds: number) {
    this.dwellBaseMs = Math.max(500, seconds * 1000);
    this.dwellJitterMs = this.dwellBaseMs * 0.6;
  }

  setWanderDistance(distance: number) {
    this.wanderBase = Math.max(0.5, distance);
    this.wanderJitter = this.wanderBase * 0.5;
    this.searchLights.forEach((state) => {
      state.control.copy(this.computeArcControl(state.start, state.target));
      state.driftAmp = this.wanderBase + this.rng() * this.wanderJitter;
    });
  }

  setBrightness(value: number) {
    this.brightness = Math.max(0.2, value);
  }

  private applyLightVisibility() {
    this.searchLights.forEach((state, index) => {
      const active = index < this.activeCount;
      state.light.visible = active;
      state.light.intensity = active ? state.intensity : 0;
      const sphere = this.orbitSpheres[index];
      if (sphere) sphere.visible = active;
    });
  }

  private updateLightning(now: number) {
    this.lightningBolts = this.lightningBolts.filter(({ line, startMs }) => {
      const age = now - startMs;
      if (age >= this.lightningDurationMs) {
        line.geometry.dispose();
        this.scene.remove(line);
        return false;
      }
      const opacity = 1 - age / this.lightningDurationMs;
      (line.material as THREE.LineBasicMaterial).opacity = opacity;
      return true;
    });
  }
}
