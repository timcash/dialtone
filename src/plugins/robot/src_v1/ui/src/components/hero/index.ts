import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import * as THREE from "three";
import { IKSolver, IKJoint, IKChain } from "./ik";

// Joint colors by type
const JOINT_COLORS = {
  rotation: 0x3b82f6, // Blue for Y-axis rotation joints
  bend: 0x808080, // Grey for Z-axis bend joints
};

// ============================================================================
// Joint - A rotational joint
// ============================================================================
class Joint implements IKJoint {
  group: THREE.Group;
  mesh: THREE.Mesh;
  material: THREE.MeshStandardMaterial;
  angle: number = 0;
  minAngle: number;
  maxAngle: number;
  axis: "x" | "y" | "z";
  name: string;

  constructor(options: {
    name: string;
    axis?: "x" | "y" | "z";
    minAngle?: number;
    maxAngle?: number;
    radius?: number;
  }) {
    this.name = options.name;
    this.axis = options.axis ?? "z";
    this.minAngle = ((options.minAngle ?? -180) * Math.PI) / 180;
    this.maxAngle = ((options.maxAngle ?? 180) * Math.PI) / 180;

    this.group = new THREE.Group();
    this.group.name = `joint-${this.name}`;

    // Color based on joint type
    const color = this.axis === "y" ? JOINT_COLORS.rotation : JOINT_COLORS.bend;

    this.material = new THREE.MeshStandardMaterial({
      color: color,
      metalness: 0.3,
      roughness: 0.4,
      emissive: color,
      emissiveIntensity: 0.2,
    });

    const geometry = new THREE.SphereGeometry(options.radius ?? 0.2, 24, 24);
    this.mesh = new THREE.Mesh(geometry, this.material);
    this.group.add(this.mesh);
  }

  setAngle(degrees: number): void {
    const radians = (degrees * Math.PI) / 180;
    this.angle = Math.max(this.minAngle, Math.min(this.maxAngle, radians));
    this.applyRotation();
  }

  setAngleRadians(radians: number): void {
    this.angle = Math.max(this.minAngle, Math.min(this.maxAngle, radians));
    this.applyRotation();
  }

  getAngle(): number {
    return (this.angle * 180) / Math.PI;
  }

  getAngleRadians(): number {
    return this.angle;
  }

  private applyRotation(): void {
    this.group.rotation.set(0, 0, 0);
    switch (this.axis) {
      case "x":
        this.group.rotation.x = this.angle;
        break;
      case "y":
        this.group.rotation.y = this.angle;
        break;
      case "z":
        this.group.rotation.z = this.angle;
        break;
    }
  }
}

// ============================================================================
// Link - A rigid connection between joints
// ============================================================================
class Link {
  mesh: THREE.Mesh;
  length: number;

  constructor(options: {
    length: number;
    width?: number;
    depth?: number;
    color?: number;
  }) {
    this.length = options.length;
    const width = options.width ?? 0.15;
    const depth = options.depth ?? 0.15;

    const geometry = new THREE.BoxGeometry(width, this.length, depth);
    const material = new THREE.MeshStandardMaterial({
      color: options.color ?? 0x505050,
      metalness: 0.6,
      roughness: 0.3,
    });
    this.mesh = new THREE.Mesh(geometry, material);
    this.mesh.position.y = this.length / 2;
  }
}

// ============================================================================
// RobotArm - Assembles joints and links with forward kinematics
// ============================================================================
class RobotArm implements IKChain {
  base: THREE.Group;
  joints: Joint[] = [];
  links: Link[] = [];
  endEffector: THREE.Mesh;

  constructor() {
    this.base = new THREE.Group();
    this.base.name = "robot-arm-base";

    const baseGeo = new THREE.CylinderGeometry(0.4, 0.5, 0.2, 32);
    const baseMat = new THREE.MeshStandardMaterial({
      color: 0x404040,
      metalness: 0.7,
      roughness: 0.2,
    });
    const baseMesh = new THREE.Mesh(baseGeo, baseMat);
    baseMesh.position.y = 0.1;
    this.base.add(baseMesh);

    const endGeo = new THREE.SphereGeometry(0.08, 24, 24);
    const endMat = new THREE.MeshStandardMaterial({
      color: 0xffffff,
      metalness: 0.3,
      roughness: 0.4,
      emissive: 0xffffff,
      emissiveIntensity: 0.3,
    });
    this.endEffector = new THREE.Mesh(endGeo, endMat);
  }

  addJoint(joint: Joint): RobotArm {
    this.joints.push(joint);
    return this;
  }

  addLink(link: Link): RobotArm {
    this.links.push(link);
    return this;
  }

  build(): void {
    let parent: THREE.Object3D = this.base;
    let yOffset = 0.2;

    for (let i = 0; i < this.joints.length; i++) {
      const joint = this.joints[i];
      const link = this.links[i];

      if (i === 0) {
        joint.group.position.y = yOffset;
      } else {
        const prevLink = this.links[i - 1];
        joint.group.position.y = prevLink.length;
      }

      parent.add(joint.group);

      if (link) {
        joint.group.add(link.mesh);
      }

      parent = joint.group;
    }

    const lastJoint = this.joints[this.joints.length - 1];
    const lastLink = this.links[this.links.length - 1];

    if (lastLink) {
        const gripperMount = new THREE.Group();
        gripperMount.position.y = lastLink.length;
        gripperMount.add(this.endEffector);
        lastJoint.group.add(gripperMount);
    }
  }

  getEndEffectorPosition(): THREE.Vector3 {
    const pos = new THREE.Vector3();
    this.endEffector.getWorldPosition(pos);
    return pos;
  }

  setAngles(angles: number[]): void {
    for (let i = 0; i < Math.min(angles.length, this.joints.length); i++) {
      this.joints[i].setAngle(angles[i]);
    }
  }

  getAngles(): number[] {
    return this.joints.map((j) => j.getAngle());
  }
}

// ============================================================================
// Visualization Control
// ============================================================================
class HeroControl implements VisualizationControl {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 1000);
  renderer: THREE.WebGLRenderer;
  frameId = 0;
  
  robotArm!: RobotArm;
  ikSolver!: IKSolver;
  
  target!: THREE.Mesh;
  targetPosition = new THREE.Vector3(2, 3, 1);
  timeSinceTargetSet = 0;
  targetMoveInterval = 3.0;
  time = 0;
  visible = false;

  cameraOrbitAngle = 0;
  cameraOrbitSpeed = 0.1;
  cameraRadius = 12;
  cameraHeight = 6.5;

  private resizeHandler: () => void;

  constructor(private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ 
        canvas, 
        antialias: true,
        alpha: true 
    });
    this.renderer.setClearColor(0x000000, 0); // Transparent background for overlay-primary
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.initScene();
    
    this.resizeHandler = () => {
        const width = this.canvas.clientWidth;
        const height = this.canvas.clientHeight;
        if (width === 0 || height === 0) return;
        
        this.camera.aspect = width / height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(width, height, false);
    };
    
    window.addEventListener('resize', this.resizeHandler);
    this.resizeHandler(); // Initial size
    
    this.animate();
  }

  initScene() {
    this.camera.position.set(this.cameraRadius, this.cameraHeight, this.cameraRadius);
    this.camera.lookAt(0, 3.5, 0);

    const ambient = new THREE.AmbientLight(0xffffff, 0.6);
    this.scene.add(ambient);

    const keyLight = new THREE.DirectionalLight(0xffffff, 1.0);
    keyLight.position.set(5, 15, 10);
    this.scene.add(keyLight);

    const fillLight = new THREE.DirectionalLight(0xffffff, 0.5);
    fillLight.position.set(-10, 5, -5);
    this.scene.add(fillLight);

    const gridHelper = new THREE.GridHelper(16, 32, 0x444444, 0x222222);
    (gridHelper.material as THREE.Material).opacity = 0.6;
    (gridHelper.material as THREE.Material).transparent = true;
    this.scene.add(gridHelper);

    // Build Robot
    this.robotArm = new RobotArm();
    this.robotArm.addJoint(new Joint({ name: "base", axis: "y", minAngle: -180, maxAngle: 180 }));
    this.robotArm.addLink(new Link({ length: 0.5, color: 0x505050 }));
    this.robotArm.addJoint(new Joint({ name: "shoulder", axis: "z", minAngle: -100, maxAngle: 100 }));
    this.robotArm.addLink(new Link({ length: 1.5, color: 0x606060 }));
    this.robotArm.addJoint(new Joint({ name: "elbow", axis: "y", minAngle: -180, maxAngle: 180 }));
    this.robotArm.addLink(new Link({ length: 0.5, color: 0x656565 }));
    this.robotArm.addJoint(new Joint({ name: "forearm", axis: "z", minAngle: -100, maxAngle: 100 }));
    this.robotArm.addLink(new Link({ length: 1.0, color: 0x707070 }));
    this.robotArm.addJoint(new Joint({ name: "wrist", axis: "z", minAngle: -100, maxAngle: 100 }));
    this.robotArm.addLink(new Link({ length: 0.5, color: 0x808080 }));
    this.robotArm.build();
    this.scene.add(this.robotArm.base);
    this.robotArm.setAngles([0, 30, 0, -45, -20]);

    this.ikSolver = new IKSolver(this.robotArm);

    // Target Sphere
    const targetGeo = new THREE.SphereGeometry(0.15, 24, 24);
    const targetMat = new THREE.MeshStandardMaterial({ color: 0xff0000, emissive: 0xff0000, emissiveIntensity: 0.5 });
    this.target = new THREE.Mesh(targetGeo, targetMat);
    this.scene.add(this.target);

    this.pickNewTarget();
  }

  pickNewTarget() {
    const maxReach = 4.0;
    const minReach = 1.5;
    const theta = Math.random() * Math.PI * 2;
    const phi = Math.random() * Math.PI * 0.5 + Math.PI * 0.15;
    const r = Math.random() * (maxReach - minReach) + minReach;

    this.targetPosition.set(
      r * Math.sin(phi) * Math.cos(theta),
      r * Math.cos(phi) + 0.5,
      r * Math.sin(phi) * Math.sin(theta),
    );
    this.target.position.copy(this.targetPosition);
    this.timeSinceTargetSet = 0;
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;

    const delta = 0.016;
    this.time += delta;

    this.cameraOrbitAngle += this.cameraOrbitSpeed * delta;
    const camX = Math.sin(this.cameraOrbitAngle) * this.cameraRadius;
    const camZ = Math.cos(this.cameraOrbitAngle) * this.cameraRadius;
    this.camera.position.set(camX, this.cameraHeight, camZ);
    this.camera.lookAt(0, 3.5, 0);

    this.timeSinceTargetSet += delta;
    if (this.timeSinceTargetSet >= this.targetMoveInterval) {
      this.pickNewTarget();
    }

    this.ikSolver.step(this.targetPosition);

    const pulse = 1 + Math.sin(this.time * 4) * 0.15;
    this.target.scale.setScalar(pulse);

    this.renderer.render(this.scene, this.camera);
  }

  dispose() {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resizeHandler);
    this.renderer.dispose();
  }

  setVisible(visible: boolean) {
    this.visible = visible;
    if (visible) {
        this.resizeHandler();
    }
  }
}

export function mountHero(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas.hero-stage") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('hero canvas not found');
  
  // Inject Buy Button if not present
  const legend = container.querySelector(".hero-legend");
  if (legend && !legend.querySelector(".buy-button")) {
      const btn = document.createElement('a');
      btn.className = 'buy-button';
      btn.href = "https://buy.stripe.com/test_5kQaEXcagaAoaC62N20kE00";
      btn.target = "_blank";
      btn.rel = "noopener noreferrer";
      btn.textContent = "Get the Robot Kit";
      legend.appendChild(btn);
  }

  return new HeroControl(canvas);
}
