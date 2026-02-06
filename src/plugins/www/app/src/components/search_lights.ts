import * as THREE from "three";
import { ArcRenderer } from "./arc_renderer";

export class SearchLights {
  private orbitLight: THREE.PointLight;
  private orbitSphere: THREE.Mesh;
  private orbitLights: THREE.PointLight[] = [];
  private orbitSpheres: THREE.Mesh[] = [];
  private arcRenderer: ArcRenderer;
  private searchLights: Array<{
    light: THREE.PointLight;
    name: string;
    intensity: number;
    power: number;
    lastSparkMs: number;
    lastPowerMs: number;
    powerRegenMs: number;
    drainRatePerMs: number;
    drainUntilMs: number;
    moveTauMs: number;
    phase: "wander" | "spark" | "rest";
    phaseUntilMs: number;
    nextSparkMs: number;
    sparkPauseMs: number;
    sparkIntervalMs: number;
    sparkCooldownMs: number;
    restThreshold: number;
    wakeSpend: number;
    sparkTarget: THREE.Vector3;
    goal: THREE.Vector3;
    goalUntilMs: number;
    goalIntervalMs: number;
    velocity: THREE.Vector3;
    maxSpeed: number;
    maxAccel: number;
    brightness: number;
    bonusUntilMs: number;
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
  private sparkDurationMs = 900;
  private linkDurationMs = 1200;
  private sparkCooldownMs = 2000;
  private lowColor = new THREE.Color(0x000000);
  private midColor = new THREE.Color(0xff8a2a);
  private highColor = new THREE.Color(0xffffff);
  private linkColor = new THREE.Color(0x8fc6ff);
  private brightness = 1.35;
  private rng: () => number;
  private activeCount = 4;
  private travelBaseMs = 12000;
  private travelJitterMs = 4000;
  private dwellBaseMs = 6000;
  private dwellJitterMs = 6000;
  private wanderBase = 8;
  private wanderJitter = 4;
  private maxPower = 5;
  private powerRegenMs = 1000;
  private powerRegenJitter = 0;
  private restThreshold = 1;
  private restThresholdJitter = 0.8;
  private sparkCooldownJitterMs = 0;
  private wakeSpendBase = 1;
  private wakeSpendJitter = 0.4;
  private sparkDrainRatePerMs = 0.001;
  private sparkDrainDurationMs = 3200;
  private sparkIntervalBaseMs = 2500;
  private sparkIntervalJitterMs = 4500;
  private sparkPauseBaseMs = 700;
  private sparkPauseJitterMs = 900;
  private maxSpeedBase = 6;
  private maxSpeedJitter = 4;
  private maxAccelBase = 6;
  private maxAccelJitter = 6;
  private linkIntervalMs = 7000;
  private linkJitterMs = 5000;
  private linkBonusRatePerMs = 0.001;
  private linkBonusDurationMs = 2500;
  private nextLinkMs = 0;

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

    this.arcRenderer = new ArcRenderer(this.scene, {
      sparkColor: 0xfff2d6,
      linkColor: 0x8fc6ff,
      sparkWidth: 2.5,
      linkWidth: 2.2,
      sparkDurationMs: this.sparkDurationMs,
      linkDurationMs: this.linkDurationMs,
    });
    this.initSearchLights();
    this.lastUpdateMs = performance.now();
    this.nextLinkMs = this.lastUpdateMs + this.linkIntervalMs + this.rng() * this.linkJitterMs;
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
        power: this.maxPower * (0.3 + this.rng() * 0.7),
        lastSparkMs: 0,
        lastPowerMs: now,
        powerRegenMs: Math.max(500, this.powerRegenMs + (this.rng() - 0.5) * this.powerRegenJitter),
        drainRatePerMs: this.sparkDrainRatePerMs,
        drainUntilMs: 0,
        moveTauMs: 250 + this.rng() * 450,
        phase: "wander",
        phaseUntilMs: 0,
        nextSparkMs: now + 2000 + this.rng() * 4000,
        sparkPauseMs: this.sparkPauseBaseMs + this.rng() * this.sparkPauseJitterMs,
        sparkIntervalMs: this.sparkIntervalBaseMs + this.rng() * this.sparkIntervalJitterMs,
        sparkCooldownMs: Math.max(80, this.sparkCooldownMs + (this.rng() - 0.5) * this.sparkCooldownJitterMs),
        restThreshold: Math.max(0.2, this.restThreshold + (this.rng() - 0.5) * this.restThresholdJitter),
        wakeSpend: Math.max(0.5, this.wakeSpendBase + (this.rng() - 0.5) * this.wakeSpendJitter),
        sparkTarget: this.gridPoint(0, 0, planeSide, 0.15),
        goal: this.randomCellPosition(planeSide),
        goalUntilMs: now + 2000 + this.rng() * 3000,
        goalIntervalMs: 2000 + this.rng() * 3000,
        velocity: new THREE.Vector3(
          (this.rng() - 0.5) * 2,
          (this.rng() - 0.5) * 2,
          (this.rng() - 0.5) * 2
        ),
        maxSpeed: this.maxSpeedBase + this.rng() * this.maxSpeedJitter,
        maxAccel: this.maxAccelBase + this.rng() * this.maxAccelJitter,
        brightness: 0,
        bonusUntilMs: 0,
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


  update(now: number) {
    const prevUpdateMs = this.lastUpdateMs;
    const deltaMs = Math.max(0, now - prevUpdateMs);
    const dt = deltaMs / 1000;
    this.lastUpdateMs = now;
    this.applyLightVisibility();
    this.searchLights.forEach((state, index) => {
      if (index >= this.activeCount) return;

      if (state.drainUntilMs > prevUpdateMs && deltaMs > 0) {
        const drainMs = Math.min(deltaMs, state.drainUntilMs - prevUpdateMs);
        state.power = Math.max(0, state.power - drainMs * state.drainRatePerMs);
      } else if (state.drainRatePerMs > this.sparkDrainRatePerMs) {
        state.drainRatePerMs = this.sparkDrainRatePerMs;
      }

      if (state.power < this.maxPower) {
        const gained = (now - state.lastPowerMs) / state.powerRegenMs;
        let bonusGain = 0;
        if (state.bonusUntilMs > prevUpdateMs && deltaMs > 0) {
          const bonusMs = Math.min(deltaMs, state.bonusUntilMs - prevUpdateMs);
          bonusGain = bonusMs * this.linkBonusRatePerMs;
        }
        state.power = Math.min(this.maxPower, state.power + gained + bonusGain);
        state.lastPowerMs = now;
      } else {
        state.lastPowerMs = now;
      }

      if (state.phase !== "rest" && state.power <= state.restThreshold) {
        state.phase = "rest";
      }
      if (state.phase === "rest" && state.power >= this.maxPower) {
        state.phase = "wander";
        state.nextSparkMs = now + state.sparkIntervalMs;
      }
      if (state.phase === "wander" && now >= state.nextSparkMs) {
        this.enterSpark(state, now);
      }
      if (state.phase === "spark" && now >= state.phaseUntilMs) {
        state.phase = "wander";
        state.nextSparkMs = now + state.sparkIntervalMs;
      }

      const powerRatio = this.maxPower > 0 ? state.power / this.maxPower : 0;
      const brighten = 1 - Math.exp(-deltaMs / 250);
      state.brightness += (state.power - state.brightness) * brighten;
      state.light.intensity = state.intensity * this.brightness * state.brightness;
      const bonusRatio = state.bonusUntilMs > now
        ? Math.min(1, (state.bonusUntilMs - now) / this.linkBonusDurationMs)
        : 0;
      if (powerRatio <= 0.001) {
        state.light.color.copy(this.lowColor);
      } else if (powerRatio >= 0.5) {
        const t = (powerRatio - 0.5) / 0.5;
        state.light.color.copy(this.midColor).lerp(this.highColor, t);
      } else {
        const t = powerRatio / 0.5;
        state.light.color.copy(this.lowColor).lerp(this.midColor, t);
      }
      if (bonusRatio > 0) {
        state.light.color.lerp(this.linkColor, bonusRatio);
      }

      if (state.phase === "rest") {
        state.velocity.multiplyScalar(Math.pow(0.2, dt));
        return;
      }

      if (state.phase === "spark") {
        state.velocity.multiplyScalar(Math.pow(0.2, dt));
        this.smoothMove(state, state.sparkTarget, deltaMs);
        return;
      }

      if (now >= state.goalUntilMs) {
        state.goal.copy(this.randomCellPosition(state.planeSide));
        state.goalUntilMs = now + state.goalIntervalMs;
      }
      const toGoal = state.goal.clone().sub(state.light.position);
      const dist = toGoal.length();
      if (dist > 0.001) {
        const desiredSpeed = state.maxSpeed * Math.min(1, dist / 25);
        const desiredVel = toGoal.normalize().multiplyScalar(desiredSpeed);
        const steer = desiredVel.sub(state.velocity);
        if (steer.length() > state.maxAccel) {
          steer.setLength(state.maxAccel);
        }
        state.velocity.addScaledVector(steer, dt);
        if (state.velocity.length() > state.maxSpeed) {
          state.velocity.setLength(state.maxSpeed);
        }
      }
      state.light.position.addScaledVector(state.velocity, dt);
    });

    if (now >= this.nextLinkMs) {
      this.spawnLinkBonus(now);
      this.nextLinkMs = now + this.linkIntervalMs + this.rng() * this.linkJitterMs;
    }

    this.orbitSpheres.forEach((sphere, index) => {
      const light =
        index === 0 ? this.orbitLight : this.orbitLights[index - 1];
      const state = this.searchLights[index];
      const powerRatio = state && this.maxPower > 0
        ? Math.max(0, Math.min(1, state.brightness / this.maxPower))
        : 0;
      const scale = 0.001 + 0.999 * powerRatio;
      sphere.position.set(
        light.position.x,
        light.position.y,
        light.position.z
      );
      sphere.scale.setScalar(scale);
      const mat = sphere.material as THREE.MeshStandardMaterial;
      mat.color.copy(light.color);
      mat.emissive.copy(light.color);
    });

    this.arcRenderer.update(now);
  }

  private enterSpark(state: SearchLights["searchLights"][number], now: number) {
    state.phase = "spark";
    state.phaseUntilMs = now + state.sparkPauseMs;
    const axis = new THREE.Vector3(0, 1, 0);
    const halfY = this.gridSize.y / 2;
    const halfZ = this.gridSize.z / 2;
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
    state.sparkTarget = this.gridPoint(y, z, state.planeSide, 3.5);
    state.lastSparkMs = now - state.sparkCooldownMs;
  }

  private smoothMove(
    state: SearchLights["searchLights"][number],
    target: THREE.Vector3,
    deltaMs: number
  ) {
    const tau = Math.max(1, state.moveTauMs);
    const alpha = 1 - Math.exp(-deltaMs / tau);
    state.light.position.lerp(target, alpha);
  }

  private spawnLinkBonus(now: number) {
    const active = this.searchLights.filter((_, index) => index < this.activeCount);
    if (active.length < 2) return;
    const a = active[Math.floor(this.rng() * active.length)];
    let b = active[Math.floor(this.rng() * active.length)];
    if (a === b) {
      b = active[(active.indexOf(a) + 1) % active.length];
    }
    const offset = new THREE.Vector3(
      (this.rng() - 0.5) * 1.2,
      (this.rng() - 0.5) * 1.2,
      (this.rng() - 0.5) * 1.2
    );
    const getPoints = () => {
      const start = a.light.position.clone();
      const end = b.light.position.clone();
      const mid = start.clone().lerp(end, 0.5).add(offset);
      return [start, mid, end];
    };
    this.arcRenderer.addLinkArc(getPoints(), now, getPoints);
    a.bonusUntilMs = now + this.linkBonusDurationMs;
    b.bonusUntilMs = now + this.linkBonusDurationMs;
  }

  getPowerLevels(): number[] {
    return this.searchLights.map((state) => state.power);
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

  spawnSpark(cells: Array<{ y: number; z: number }>) {
    const sparkedCells: Array<{ y: number; z: number }> = [];
    const glider = [
      [0, 1],
      [1, 2],
      [2, 0],
      [2, 1],
      [2, 2],
    ];
    const now = performance.now();
    const axis = new THREE.Vector3(0, 1, 0);
    const halfY = this.gridSize.y / 2;
    const halfZ = this.gridSize.z / 2;
    this.searchLights.forEach((state, index) => {
      if (index >= this.activeCount) return;
      if (now - state.lastSparkMs < state.sparkCooldownMs) return;
      if (state.phase !== "spark") return;
      if (state.power <= 0) return;
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
      state.drainUntilMs = now + this.sparkDrainDurationMs;
      state.drainRatePerMs += this.sparkDrainRatePerMs;
      state.lastPowerMs = now;
      sparkedCells.push({ y, z });
      glider.forEach(([dy, dz]) => {
        const end = this.gridPlanePoint(y + dy, z + dz);
        const offset = new THREE.Vector3(
          (this.rng() - 0.5) * 0.8,
          (this.rng() - 0.5) * 0.8,
          (this.rng() - 0.5) * 0.8
        );
        const getPoints = () => {
          const start = state.light.position.clone().lerp(end, 0.05);
          const mid = start.clone().lerp(end, 0.5).add(offset);
          return [start, mid, end];
        };
        this.arcRenderer.addSparkArc(getPoints(), now, getPoints);
      });
    });
    return sparkedCells;
  }

  setLightCount(count: number) {
    this.activeCount = THREE.MathUtils.clamp(Math.round(count), 1, this.searchLights.length);
    this.applyLightVisibility();
  }

  setMaxPower(value: number) {
    this.maxPower = Math.max(0.5, value);
    this.searchLights.forEach((state) => {
      state.power = Math.min(state.power, this.maxPower);
    });
  }

  setPowerRegenRatePerSec(rate: number) {
    const safeRate = Math.max(0.05, rate);
    this.powerRegenMs = 1000 / safeRate;
    this.searchLights.forEach((state) => {
      state.powerRegenMs = this.powerRegenMs;
    });
  }

  setSparkIntervalSeconds(seconds: number) {
    this.sparkIntervalBaseMs = Math.max(300, seconds * 1000);
    this.sparkIntervalJitterMs = Math.max(0, this.sparkIntervalBaseMs * 0.8);
    const now = performance.now();
    this.searchLights.forEach((state) => {
      state.sparkIntervalMs =
        this.sparkIntervalBaseMs + this.rng() * this.sparkIntervalJitterMs;
      state.nextSparkMs = now + state.sparkIntervalMs;
    });
  }

  setSparkPauseMs(ms: number) {
    this.sparkPauseBaseMs = Math.max(100, ms);
    this.sparkPauseJitterMs = Math.max(0, this.sparkPauseBaseMs * 0.6);
    this.searchLights.forEach((state) => {
      state.sparkPauseMs =
        this.sparkPauseBaseMs + this.rng() * this.sparkPauseJitterMs;
    });
  }

  setSparkDrainRatePerMs(rate: number) {
    this.sparkDrainRatePerMs = Math.max(0.0001, rate);
    this.searchLights.forEach((state) => {
      if (state.drainUntilMs <= performance.now()) {
        state.drainRatePerMs = this.sparkDrainRatePerMs;
      }
    });
  }

  setRestThreshold(value: number) {
    this.restThreshold = Math.max(0.1, value);
    this.searchLights.forEach((state) => {
      state.restThreshold = Math.max(0.1, this.restThreshold + (this.rng() - 0.5) * this.restThresholdJitter);
    });
  }

  setGlideSpeed(value: number) {
    this.maxSpeedBase = Math.max(0.5, value);
    this.searchLights.forEach((state) => {
      state.maxSpeed = this.maxSpeedBase + this.rng() * this.maxSpeedJitter;
    });
  }

  setGlideAccel(value: number) {
    this.maxAccelBase = Math.max(0.5, value);
    this.searchLights.forEach((state) => {
      state.maxAccel = this.maxAccelBase + this.rng() * this.maxAccelJitter;
    });
  }

  getMaxLights() {
    return this.searchLights.length;
  }

  setRng(rng: () => number) {
    this.rng = rng;
  }

  resetForSeed(rng: () => number) {
    this.setRng(rng);
    this.arcRenderer.clear();
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

  setArcResolution(width: number, height: number) {
    this.arcRenderer.updateResolution(width, height);
  }
}
