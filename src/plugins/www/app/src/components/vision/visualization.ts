import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { Line2 } from "three/examples/jsm/lines/Line2.js";
import { LineGeometry } from "three/examples/jsm/lines/LineGeometry.js";
import { LineMaterial } from "three/examples/jsm/lines/LineMaterial.js";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";

export type Landmark = { x: number; y: number; z: number; visibility?: number };

const POSE_CONNECTIONS = [
  [11, 12], [11, 13], [13, 15], [12, 14], [14, 16], // Upper body
  [11, 23], [12, 24], [23, 24], // Torso
  [23, 25], [24, 26], [25, 27], [26, 28], [27, 29], [28, 30], [29, 31], [30, 32], // Legs
  [15, 17], [15, 19], [15, 21], [17, 19], // Hands
  [16, 18], [16, 20], [16, 22], [18, 20],
  [0, 1], [1, 2], [2, 3], [0, 4], [4, 5], [5, 6], // Face
];

export class VisionVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;
  private fpsCounter = new FpsCounter("vision");

  private skeletonGroup = new THREE.Group();
  private joints: THREE.Mesh[] = [];
  private bones: Line2[] = [];
  private composer!: EffectComposer;
  private bloomPass!: UnrealBloomPass;
  private time = 0;

  // State
  private landmarks: Landmark[] = [];
  private isDemo = false;
  private demoTime = 0;

  // Configurable properties
  jointSize = 0.04;
  boneWidth = 3;
  color = 0x00ffff;
  skeletonVisible = true;
  cameraDistance = 8.0;

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 0);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const canvas = this.renderer.domElement;
    canvas.style.position = "absolute";
    canvas.style.top = "0";
    canvas.style.left = "0";
    canvas.style.width = "100%";
    canvas.style.height = "100%";
    this.container.appendChild(canvas);

    this.camera.position.set(2, 1, this.cameraDistance);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.5));
    const pointLight = new THREE.PointLight(0x00ffff, 1, 20);
    pointLight.position.set(5, 5, 5);
    this.scene.add(pointLight);

    // Add floor grid for 3D context
    const grid = new THREE.GridHelper(10, 10, 0x00ffff, 0x222222);
    grid.position.y = -2.5;
    (grid.material as THREE.Material).transparent = true;
    (grid.material as THREE.Material).opacity = 0.2;
    this.scene.add(grid);

    this.scene.add(this.skeletonGroup);
    this.initSkeleton();

    this.composer = new EffectComposer(this.renderer);
    this.composer.addPass(new RenderPass(this.scene, this.camera));
    this.bloomPass = new UnrealBloomPass(
      new THREE.Vector2(window.innerWidth, window.innerHeight),
      1.5,
      0.4,
      0.85
    );
    this.composer.addPass(this.bloomPass);

    this.resize();
    this.animate();

    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);

    if (typeof ResizeObserver !== "undefined") {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    }
  }

  private initSkeleton() {
    const jointGeo = new THREE.SphereGeometry(0.04, 16, 16);
    const jointMat = new THREE.MeshStandardMaterial({
      color: 0x00ffff,
      emissive: 0x00ffff,
      emissiveIntensity: 0.5,
    });

    for (let i = 0; i < 33; i++) {
      const mesh = new THREE.Mesh(jointGeo, jointMat);
      mesh.visible = false;
      this.skeletonGroup.add(mesh);
      this.joints.push(mesh);
    }

    const boneMat = new LineMaterial({
      color: 0x00ffff,
      linewidth: 3,
      transparent: true,
      opacity: 0.8,
      resolution: new THREE.Vector2(window.innerWidth, window.innerHeight),
    });

    POSE_CONNECTIONS.forEach(() => {
      const geo = new LineGeometry();
      const line = new Line2(geo, boneMat);
      line.visible = false;
      this.skeletonGroup.add(line);
      this.bones.push(line);
    });
  }

  setJointSize(size: number) {
    this.jointSize = size;
    this.joints.forEach(j => j.scale.setScalar(size / 0.04));
  }

  setBoneWidth(width: number) {
    this.boneWidth = width;
    this.bones.forEach(b => (b.material as LineMaterial).linewidth = width);
  }

  setColor(color: number) {
    this.color = color;
    const c = new THREE.Color(color);
    this.joints.forEach(j => {
      (j.material as THREE.MeshStandardMaterial).color.copy(c);
      (j.material as THREE.MeshStandardMaterial).emissive.copy(c);
    });
    this.bones.forEach(b => (b.material as LineMaterial).color.copy(c));
  }

  setBloomStrength(strength: number) {
    if (this.bloomPass) this.bloomPass.strength = strength;
  }

  setCameraDistance(distance: number) {
    this.cameraDistance = distance;
    this.camera.position.z = distance;
    this.camera.lookAt(0, 0, 0);
  }

  setDemo(active: boolean) {
    this.isDemo = active;
    if (!active) this.updatePose([]);
  }

  updatePose(landmarks: Landmark[]) {
    this.landmarks = landmarks;
  }

  private applyPose(landmarks: Landmark[]) {
    if (!landmarks || landmarks.length === 0) {
      this.joints.forEach((j) => (j.visible = false));
      this.bones.forEach((b) => (b.visible = false));
      return;
    }

    landmarks.forEach((lm, i) => {
      if (i >= this.joints.length) return;
      const joint = this.joints[i];
      const scale = 5.0;
      joint.position.set((0.5 - lm.x) * scale, (0.5 - lm.y) * scale, -lm.z * scale);
      joint.visible = (lm.visibility ?? 1.0) > 0.5;
    });

    POSE_CONNECTIONS.forEach(([a, b], i) => {
      if (i >= this.bones.length) return;
      const bone = this.bones[i];
      const lmA = landmarks[a];
      const lmB = landmarks[b];

      if (lmA && lmB && (lmA.visibility ?? 1.0) > 0.5 && (lmB.visibility ?? 1.0) > 0.5) {
        const scale = 5.0;
        const points = [
          (0.5 - lmA.x) * scale, (0.5 - lmA.y) * scale, -lmA.z * scale,
          (0.5 - lmB.x) * scale, (0.5 - lmB.y) * scale, -lmB.z * scale,
        ];
        bone.geometry.setPositions(points);
        bone.visible = true;
      } else {
        bone.visible = false;
      }
    });
  }

  resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
    this.composer.setSize(width, height);
    
    const pr = window.devicePixelRatio;
    this.bones.forEach(bone => {
      (bone.material as LineMaterial).resolution.set(width * pr, height * pr);
    });
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    this.renderer.dispose();
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "vision");
    if (!visible) {
      this.fpsCounter.clear();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;

    if (this.isDemo) {
      this.demoTime += 0.03;
      const demoLandmarks: Landmark[] = new Array(33).fill(null).map(() => ({ x: 0.5, y: 0.5, z: 0, visibility: 0 }));
      
      const breathe = Math.sin(this.demoTime) * 0.02;
      const wave = Math.sin(this.demoTime * 1.5);
      const walk = Math.sin(this.demoTime * 2);
      
      const hipY = 0.6 + breathe;
      const shoulderY = 0.35 + breathe;
      
      // Essential points for a human-like skeleton
      const points: Record<number, Landmark> = {
        0:  { x: 0.5, y: 0.2 + breathe, z: 0.05, visibility: 1 }, // Head
        11: { x: 0.4, y: shoulderY, z: 0, visibility: 1 },        // L Shoulder
        12: { x: 0.6, y: shoulderY, z: 0, visibility: 1 },        // R Shoulder
        13: { x: 0.3, y: shoulderY + 0.1, z: 0.1, visibility: 1 }, // L Elbow
        14: { x: 0.7, y: shoulderY + 0.1 + wave * 0.1, z: 0.1 + wave * 0.2, visibility: 1 }, // R Elbow
        15: { x: 0.2, y: shoulderY + 0.2, z: 0, visibility: 1 },   // L Wrist
        16: { x: 0.75 + wave * 0.1, y: shoulderY - 0.2 + wave * 0.1, z: 0.3, visibility: 1 }, // R Wrist (waving)
        23: { x: 0.45, y: hipY, z: 0, visibility: 1 },             // L Hip
        24: { x: 0.55, y: hipY, z: 0, visibility: 1 },             // R Hip
        25: { x: 0.43, y: hipY + 0.2 + walk * 0.05, z: walk * 0.1, visibility: 1 }, // L Knee
        26: { x: 0.57, y: hipY + 0.2 - walk * 0.05, z: -walk * 0.1, visibility: 1 }, // R Knee
        27: { x: 0.45, y: 0.9, z: walk * 0.2, visibility: 1 },     // L Ankle
        28: { x: 0.55, y: 0.9, z: -walk * 0.2, visibility: 1 },    // R Ankle
      };

      Object.entries(points).forEach(([idx, lm]) => {
        demoLandmarks[parseInt(idx)] = lm;
      });
      
      this.applyPose(demoLandmarks);
    } else {
      this.applyPose(this.landmarks);
    }

    // Subtle rotation or movement to make it feel alive
    this.skeletonGroup.rotation.y = Math.sin(this.time * 0.5) * 0.1;

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.composer.render();
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}
