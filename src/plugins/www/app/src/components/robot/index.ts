import * as THREE from "three";
import { IKSolver } from "./ik";
import { FpsCounter } from "../fps";
import { GpuTimer } from "../gpu_timer";
import { VisibilityMixin } from "../section";
import { startTyping } from "../typing";
import { setupRobotConfig } from "./config";


// Joint colors by type
const JOINT_COLORS = {
  rotation: 0x3b82f6, // Blue for Y-axis rotation joints
  bend: 0x808080, // Grey for Z-axis bend joints
};

// ============================================================================
// Joint - A rotational joint
// ============================================================================
class Joint {
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

  getMinAngle(): number {
    return (this.minAngle * 180) / Math.PI;
  }

  getMaxAngle(): number {
    return (this.maxAngle * 180) / Math.PI;
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
// Finger - A gripper finger with joint
// ============================================================================
class Finger {
  joint: Joint;
  link: THREE.Mesh;
  tip: THREE.Mesh;

  constructor(options: { side: "left" | "right" }) {
    const side = options.side === "left" ? -1 : 1;

    this.joint = new Joint({
      name: `finger-${options.side}`,
      axis: "z",
      minAngle: side === -1 ? -60 : 0,
      maxAngle: side === -1 ? 0 : 60,
      radius: 0.08,
    });

    this.joint.group.position.set(side * 0.15, 0, 0);

    const linkGeo = new THREE.BoxGeometry(0.06, 0.25, 0.06);
    const linkMat = new THREE.MeshStandardMaterial({
      color: 0x606060,
      metalness: 0.6,
      roughness: 0.3,
    });
    this.link = new THREE.Mesh(linkGeo, linkMat);
    this.link.position.y = 0.125;
    this.joint.group.add(this.link);

    const tipGeo = new THREE.SphereGeometry(0.04, 16, 16);
    const tipMat = new THREE.MeshStandardMaterial({
      color: 0xffffff,
      metalness: 0.3,
      roughness: 0.4,
      emissive: 0xffffff,
      emissiveIntensity: 0.3,
    });
    this.tip = new THREE.Mesh(tipGeo, tipMat);
    this.tip.position.y = 0.27;
    this.joint.group.add(this.tip);

    this.joint.setAngle(side * 20);
  }

  setGrip(openAmount: number): void {
    const side = this.joint.group.position.x > 0 ? 1 : -1;
    const angle = side * (5 + openAmount * 40);
    this.joint.setAngle(angle);
  }
}

// ============================================================================
// RobotArm - Assembles joints and links with forward kinematics
// ============================================================================
class RobotArm {
  base: THREE.Group;
  joints: Joint[] = [];
  links: Link[] = [];
  endEffector: THREE.Mesh;
  fingers: Finger[] = [];
  gripAmount: number = 0.5;

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

      const leftFinger = new Finger({ side: "left" });
      const rightFinger = new Finger({ side: "right" });

      gripperMount.add(leftFinger.joint.group);
      gripperMount.add(rightFinger.joint.group);

      this.fingers.push(leftFinger, rightFinger);

      lastJoint.group.add(gripperMount);
    }

    this.setGrip(this.gripAmount);
  }

  setGrip(amount: number): void {
    this.gripAmount = Math.max(0, Math.min(1, amount));
    this.fingers.forEach((finger) => finger.setGrip(this.gripAmount));
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

  setAnglesRadians(angles: number[]): void {
    for (let i = 0; i < Math.min(angles.length, this.joints.length); i++) {
      this.joints[i].setAngleRadians(angles[i]);
    }
  }

  getAngles(): number[] {
    return this.joints.map((j) => j.getAngle());
  }

  getAnglesRadians(): number[] {
    return this.joints.map((j) => j.getAngleRadians());
  }
}

// ============================================================================
// Visualization
// ============================================================================
class RobotArmVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;


  robotArm!: RobotArm;
  ikSolver!: IKSolver;

  // Target tracking
  target!: THREE.Mesh;
  targetPosition = new THREE.Vector3(2, 3, 1);
  targetLine!: THREE.Line;
  targetLineMaterial!: THREE.LineBasicMaterial;

  // Timing
  timeSinceTargetSet = 0;
  targetMoveInterval = 3.0; // Move target every 3 seconds

  time = 0;
  autoAnimate = true;

  cameraOrbitAngle = 0;
  cameraOrbitSpeed = 0.1;
  cameraRadius = 12;
  cameraHeight = 1;

  configPanel?: HTMLDivElement;
  configToggle?: HTMLButtonElement;
  sliders: { slider: HTMLInputElement; valueEl: HTMLSpanElement }[] = [];
  private setPanelOpen?: (open: boolean) => void;
  private fpsCounter = new FpsCounter("robot");

  constructor(container: HTMLElement) {
    this.container = container;

    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const canvas = this.renderer.domElement;
    canvas.style.position = "absolute";
    canvas.style.top = "0";
    canvas.style.left = "0";
    canvas.style.width = "100%";
    canvas.style.height = "100%";

    const existingCanvas = container.querySelector('canvas');
    if (existingCanvas) {
      this.renderer.domElement = existingCanvas as HTMLCanvasElement;
    } else {
      this.container.appendChild(canvas);
    }

    this.initScene();
    this.initConfigPanel();
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
    this.container.removeChild(this.renderer.domElement);
  }

  initScene() {
    this.camera.position.set(
      this.cameraRadius,
      this.cameraHeight,
      this.cameraRadius,
    );
    this.camera.lookAt(0, 3.5, 0);

    // Strong ambient light
    const ambient = new THREE.AmbientLight(0xffffff, 0.6);
    this.scene.add(ambient);

    // Multiple bright point lights
    const pointLights = [
      { pos: [-8, 12, 8], intensity: 1.0 },
      { pos: [8, 10, -8], intensity: 0.8 },
      { pos: [0, 8, 10], intensity: 0.8 },
      { pos: [-6, 4, -6], intensity: 0.6 },
      { pos: [6, 6, 6], intensity: 0.6 },
    ];
    pointLights.forEach(({ pos, intensity }) => {
      const light = new THREE.PointLight(0xffffff, intensity, 50);
      light.position.set(pos[0], pos[1], pos[2]);
      this.scene.add(light);
    });

    // Key light
    const keyLight = new THREE.DirectionalLight(0xffffff, 1.0);
    keyLight.position.set(5, 15, 10);
    this.scene.add(keyLight);

    // Fill light
    const fillLight = new THREE.DirectionalLight(0xffffff, 0.5);
    fillLight.position.set(-10, 5, -5);
    this.scene.add(fillLight);

    // Back light
    const backLight = new THREE.DirectionalLight(0xffffff, 0.4);
    backLight.position.set(0, 5, -15);
    this.scene.add(backLight);

    // Grid floor
    const gridHelper = new THREE.GridHelper(16, 32, 0x444444, 0x222222);
    (gridHelper.material as THREE.Material).opacity = 0.6;
    (gridHelper.material as THREE.Material).transparent = true;
    this.scene.add(gridHelper);

    // Build robot arm
    this.robotArm = new RobotArm();

    // Alternating joint types: Y-axis (rotation/blue) and Z-axis (bend/grey)
    this.robotArm.addJoint(
      new Joint({
        name: "base",
        axis: "y",
        minAngle: -180,
        maxAngle: 180,
      }),
    );
    this.robotArm.addLink(new Link({ length: 0.5, color: 0x505050 }));

    this.robotArm.addJoint(
      new Joint({
        name: "shoulder",
        axis: "z",
        minAngle: -100,
        maxAngle: 100,
      }),
    );
    this.robotArm.addLink(new Link({ length: 1.5, color: 0x606060 }));

    this.robotArm.addJoint(
      new Joint({
        name: "elbow",
        axis: "y",
        minAngle: -180,
        maxAngle: 180,
      }),
    );
    this.robotArm.addLink(new Link({ length: 0.5, color: 0x656565 }));

    this.robotArm.addJoint(
      new Joint({
        name: "forearm",
        axis: "z",
        minAngle: -100,
        maxAngle: 100,
      }),
    );
    this.robotArm.addLink(new Link({ length: 1.0, color: 0x707070 }));

    this.robotArm.addJoint(
      new Joint({
        name: "wrist",
        axis: "z",
        minAngle: -100,
        maxAngle: 100,
      }),
    );
    this.robotArm.addLink(new Link({ length: 0.5, color: 0x808080 }));

    this.robotArm.build();
    this.scene.add(this.robotArm.base);
    this.robotArm.setAngles([0, 30, 0, -45, -20]);

    // Create IK solver
    this.ikSolver = new IKSolver(this.robotArm);

    // Create red target sphere
    const targetGeo = new THREE.SphereGeometry(0.15, 24, 24);
    const targetMat = new THREE.MeshStandardMaterial({
      color: 0xff0000,
      emissive: 0xff0000,
      emissiveIntensity: 0.5,
    });
    this.target = new THREE.Mesh(targetGeo, targetMat);
    this.scene.add(this.target);

    // Create line from end effector to target
    this.targetLineMaterial = new THREE.LineBasicMaterial({
      color: 0xff4444,
      transparent: true,
      opacity: 0.5,
    });
    const lineGeo = new THREE.BufferGeometry().setFromPoints([
      new THREE.Vector3(),
      new THREE.Vector3(),
    ]);
    this.targetLine = new THREE.Line(lineGeo, this.targetLineMaterial);
    this.scene.add(this.targetLine);

    // Set initial target position
    this.pickNewTarget();
  }

  pickNewTarget(): void {
    // Generate random point in reachable space
    const maxReach = 4.0;
    const minReach = 1.5;

    // Random spherical coordinates
    const theta = Math.random() * Math.PI * 2;
    const phi = Math.random() * Math.PI * 0.5 + Math.PI * 0.15; // Above ground
    const r = Math.random() * (maxReach - minReach) + minReach;

    this.targetPosition.set(
      r * Math.sin(phi) * Math.cos(theta),
      r * Math.cos(phi) + 0.5, // Offset up from ground
      r * Math.sin(phi) * Math.sin(theta),
    );

    // Clamp Y to be above ground
    if (this.targetPosition.y < 0.5) {
      this.targetPosition.y = 0.5;
    }

    this.target.position.copy(this.targetPosition);
    this.timeSinceTargetSet = 0;
  }

  updateTargetLine(): void {
    const endPos = this.robotArm.getEndEffectorPosition();
    const positions = this.targetLine.geometry.attributes.position
      .array as Float32Array;

    positions[0] = endPos.x;
    positions[1] = endPos.y;
    positions[2] = endPos.z;
    positions[3] = this.targetPosition.x;
    positions[4] = this.targetPosition.y;
    positions[5] = this.targetPosition.z;

    this.targetLine.geometry.attributes.position.needsUpdate = true;
  }

  initConfigPanel() {
    setupRobotConfig(this);
    return;
    /*
    const panel = document.getElementById(
      "robot-config-panel",
    ) as HTMLDivElement | null;
    const toggle = document.getElementById(
      "robot-config-toggle",
    ) as HTMLButtonElement | null;
    if (!panel || !toggle) return;

    this.configPanel = panel;
    this.configToggle = toggle;

    const setOpen = (open: boolean) => {
      panel.hidden = !open;
      panel.style.display = open ? "grid" : "none";
      toggle.setAttribute("aria-expanded", String(open));
    };
    this.setPanelOpen = setOpen;

    setOpen(false);
    toggle.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      setOpen(panel.hidden);
    });

    const addHeader = (text: string) => {
      const header = document.createElement("h3");
      header.textContent = text;
      panel.appendChild(header);
    };

    const addSlider = (
      label: string,
      value: number,
      min: number,
      max: number,
      step: number,
      onInput: (v: number) => void,
      format: (v: number) => string = (v) => `${Math.round(v)}°`,
    ) => {
      const row = document.createElement("div");
      row.className = "earth-config-row";

      const labelWrap = document.createElement("label");
      const sliderId = `robot-slider-${label.replace(/\s+/g, "-").toLowerCase()}`;
      labelWrap.className = "earth-config-label";
      labelWrap.htmlFor = sliderId;
      labelWrap.textContent = label;

      const slider = document.createElement("input");
      slider.type = "range";
      slider.id = sliderId;
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;

      row.appendChild(labelWrap);
      row.appendChild(slider);

      const valueEl = document.createElement("span");
      valueEl.className = "earth-config-value";
      valueEl.textContent = format(value);
      row.appendChild(valueEl);
      panel.appendChild(row);

      slider.addEventListener("input", () => {
        const v = parseFloat(slider.value);
        onInput(v);
        valueEl.textContent = format(v);
      });

      return { slider, valueEl };
    };

    const addCheckbox = (
      label: string,
      checked: boolean,
      onChange: (v: boolean) => void,
    ) => {
      const row = document.createElement("div");
      row.className = "earth-config-row";

      const labelWrap = document.createElement("label");
      labelWrap.style.display = "flex";
      labelWrap.style.alignItems = "center";
      labelWrap.style.gap = "8px";

      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = checked;

      const text = document.createElement("span");
      text.textContent = label;

      labelWrap.appendChild(checkbox);
      labelWrap.appendChild(text);
      row.appendChild(labelWrap);
      panel.appendChild(row);

      checkbox.addEventListener("change", () => onChange(checkbox.checked));
    };

    const addButton = (label: string, onClick: () => void) => {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = label;
      button.addEventListener("click", onClick);
      panel.appendChild(button);
    };

    addHeader("IK Mode");
    addCheckbox("Auto Track Target", this.autoAnimate, (v) => {
      this.autoAnimate = v;
    });
    addButton("New Target", () => this.pickNewTarget());

    addHeader("Camera");
    addSlider(
      "Distance",
      this.cameraRadius,
      6,
      20,
      0.5,
      (v) => {
        this.cameraRadius = v;
      },
      (v) => v.toFixed(1),
    );
    addSlider(
      "Height",
      this.cameraHeight,
      1,
      12,
      0.5,
      (v) => {
        this.cameraHeight = v;
      },
      (v) => v.toFixed(1),
    );
    addSlider(
      "Orbit Speed",
      this.cameraOrbitSpeed,
      0,
      0.5,
      0.01,
      (v) => {
        this.cameraOrbitSpeed = v;
      },
      (v) => v.toFixed(2),
    );

    addHeader("Joint Angles");

    const jointConfigs = [
      { name: "Base (Y)", min: -180, max: 180, initial: 0 },
      { name: "Shoulder (Z)", min: -100, max: 100, initial: 30 },
      { name: "Elbow (Y)", min: -180, max: 180, initial: 0 },
      { name: "Forearm (Z)", min: -100, max: 100, initial: -45 },
      { name: "Wrist (Z)", min: -100, max: 100, initial: -20 },
    ];

    jointConfigs.forEach((config, i) => {
      const { slider, valueEl } = addSlider(
        config.name,
        config.initial,
        config.min,
        config.max,
        1,
        (v) => {
          this.robotArm.joints[i].setAngle(v);
          this.autoAnimate = false;
        },
      );
      this.sliders.push({ slider, valueEl });
    });

    addHeader("Gripper");
    addSlider(
      "Grip",
      0.5,
      0,
      1,
      0.01,
      (v) => this.robotArm.setGrip(v),
      (v) => `${Math.round(v * 100)}%`,
    );
    */
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "robot");
    if (!visible) {
      this.setPanelOpen?.(false);
      this.fpsCounter.clear();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);

    // Skip all calculations when off-screen
    if (!this.isVisible) return;

    const cpuStart = performance.now();
    this.frameCount++;
    const delta = 0.016;
    this.time += delta;

    // Camera pans around the robot
    this.cameraOrbitAngle += this.cameraOrbitSpeed * delta;
    const camX = Math.sin(this.cameraOrbitAngle) * this.cameraRadius;
    const camZ = Math.cos(this.cameraOrbitAngle) * this.cameraRadius;
    this.camera.position.set(camX, this.cameraHeight, camZ);
    this.camera.lookAt(0, 3.5, 0);

    if (this.autoAnimate) {
      this.timeSinceTargetSet += delta;

      // Move target every 3 seconds no matter what
      if (this.timeSinceTargetSet >= this.targetMoveInterval) {
        this.pickNewTarget();
      }

      // Always run IK solver - arm is always chasing
      this.ikSolver.step(this.targetPosition);

      // Update slider displays
      const angles = this.robotArm.getAngles();
      angles.forEach((angle, i) => {
        if (this.sliders[i]) {
          this.sliders[i].slider.value = `${angle}`;
          this.sliders[i].valueEl.textContent = `${Math.round(angle)}°`;
        }
      });
    }

    // Update target line
    this.updateTargetLine();

    // Pulse target sphere
    const pulse = 1 + Math.sin(this.time * 4) * 0.15;
    this.target.scale.setScalar(pulse);

    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}

export function mountRobot(container: HTMLElement) {
  // Inject HTML
  container.innerHTML = `
      <div class="marketing-overlay" aria-label="Robot visualization marketing information">
        <h2>Robotics begins with precision control</h2>
        <p data-typing-subtitle></p>
        <a class="buy-button" href="https://buy.stripe.com/test_5kQaEXcagaAoaC62N20kE00" target="_blank"
          rel="noopener noreferrer">Get the Robot Kit</a>
      </div>
      <div id="robot-config-panel" class="earth-config-panel" hidden></div>
    `;

  // Create and inject config toggle
  const controls = document.querySelector('.top-right-controls');
  const toggle = document.createElement('button');
  toggle.id = 'robot-config-toggle';
  toggle.className = 'earth-config-toggle';
  toggle.type = 'button';
  toggle.setAttribute('aria-expanded', 'false');
  toggle.textContent = 'Config';
  controls?.prepend(toggle);

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Interact with physical systems through low-latency digital twins.",
    "Build the future of automation with shared tooling.",
    "Precision control for real machines in the field.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new RobotArmVisualization(container);
  return {
    dispose: () => {
      viz.dispose();
      toggle.remove();
      stopTyping();
      container.innerHTML = '';
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
