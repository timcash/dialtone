import { type VisualizationControl, type SectionManager } from "../util/section";
import * as THREE from "three";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupPolicyMenu } from "./menu";
import { PolicyHistogram } from "./histogram";
import type { HistogramMeta } from "./histogram";
import { MarkovChainModel } from "./markov_chain_model";
import { MonteCarloModel } from "./monte_carlo_model";
import { ShadowCostModel } from "./shadow_cost_model";
import { runPolicyModelTests } from "./model_tests";
import type { MonteCarloParams, PolicyDomain, PolicyPreset } from "./types";

type ChildNodeDef = { parentIdx: number; names: string[] };
type PolicyScenario = {
  id: string;
  domains: PolicyDomain[];
  childDefs: ChildNodeDef[];
};

const POLICY_SCENARIOS: Record<string, PolicyScenario> = {
  "ev-charging": {
    id: "ev-charging",
    domains: [
      { id: "ev-plan", name: "EV Planning", lat: 43, lng: -95, color: 0xf59e0b, connections: [1, 7], connectionWeights: [0.75, 0.25] },
      { id: "charger-rollout", name: "Charger Rollout", lat: 18, lng: -55, color: 0x22d3ee, connections: [2, 3], connectionWeights: [0.48, 0.52] },
      { id: "grid-strain", name: "Grid Strain", lat: 49, lng: 15, color: 0xef4444, connections: [4, 7], connectionWeights: [0.7, 0.3] },
      { id: "storage-upgrade", name: "Storage Upgrade", lat: 30, lng: 78, color: 0x3b82f6, connections: [4, 5], connectionWeights: [0.58, 0.42] },
      { id: "fleet-adoption", name: "Fleet Adoption", lat: -15, lng: -35, color: 0x10b981, connections: [5, 6], connectionWeights: [0.55, 0.45] },
      { id: "rural-access", name: "Rural Access", lat: -9, lng: 42, color: 0x8b5cf6, connections: [6, 7], connectionWeights: [0.66, 0.34] },
      { id: "emissions-drop", name: "Emissions Drop", lat: -38, lng: 118, color: 0x06b6d4, connections: [6, 4], connectionWeights: [0.72, 0.28] },
      { id: "policy-stall", name: "Policy Stall", lat: -31, lng: -122, color: 0x64748b, connections: [7, 1], connectionWeights: [0.78, 0.22] },
    ],
    childDefs: [
      { parentIdx: 1, names: ["Depot Fast DC", "Street L2"] },
      { parentIdx: 3, names: ["Battery Peakers", "Smart Tariffs"] },
      { parentIdx: 4, names: ["Bus Fleets", "Delivery Vans"] },
    ],
  },
  "childhood-education": {
    id: "childhood-education",
    domains: [
      { id: "screening", name: "Early Screening", lat: 44, lng: -98, color: 0xfbbf24, connections: [1, 7], connectionWeights: [0.7, 0.3] },
      { id: "teacher", name: "Teacher Pipeline", lat: 14, lng: -62, color: 0x3b82f6, connections: [2, 3], connectionWeights: [0.6, 0.4] },
      { id: "quality", name: "Classroom Quality", lat: 52, lng: 10, color: 0x8b5cf6, connections: [3, 4], connectionWeights: [0.52, 0.48] },
      { id: "attendance", name: "Attendance Gains", lat: 28, lng: 82, color: 0x22c55e, connections: [4, 5], connectionWeights: [0.65, 0.35] },
      { id: "parent", name: "Parent Support", lat: -18, lng: -28, color: 0x10b981, connections: [5, 6], connectionWeights: [0.62, 0.38] },
      { id: "nutrition", name: "School Nutrition", lat: -8, lng: 35, color: 0x06b6d4, connections: [6, 7], connectionWeights: [0.68, 0.32] },
      { id: "graduation", name: "Graduation Lift", lat: -36, lng: 116, color: 0x2dd4bf, connections: [6, 4], connectionWeights: [0.75, 0.25] },
      { id: "dropout", name: "Dropout Risk", lat: -30, lng: -118, color: 0xef4444, connections: [7, 2], connectionWeights: [0.8, 0.2] },
    ],
    childDefs: [
      { parentIdx: 1, names: ["Residency", "Mentorship"] },
      { parentIdx: 4, names: ["Home Visits", "Care Access"] },
      { parentIdx: 6, names: ["STEM Bridge", "Apprenticeship"] },
    ],
  },
  "business-development": {
    id: "business-development",
    domains: [
      { id: "permits", name: "Permit Reform", lat: 40, lng: -100, color: 0xf59e0b, connections: [1, 7], connectionWeights: [0.73, 0.27] },
      { id: "credit", name: "SME Credit", lat: 12, lng: -58, color: 0x38bdf8, connections: [2, 3], connectionWeights: [0.55, 0.45] },
      { id: "startup", name: "Startup Hubs", lat: 50, lng: 11, color: 0xa855f7, connections: [3, 4], connectionWeights: [0.58, 0.42] },
      { id: "workforce", name: "Workforce Upskill", lat: 32, lng: 84, color: 0x10b981, connections: [4, 5], connectionWeights: [0.64, 0.36] },
      { id: "infra-bottle", name: "Infra Bottleneck", lat: -22, lng: -32, color: 0xef4444, connections: [5, 7], connectionWeights: [0.46, 0.54] },
      { id: "exports", name: "Export Growth", lat: -9, lng: 38, color: 0x22d3ee, connections: [6, 4], connectionWeights: [0.72, 0.28] },
      { id: "tax-base", name: "Tax Base Lift", lat: -42, lng: 121, color: 0x06b6d4, connections: [6, 1], connectionWeights: [0.77, 0.23] },
      { id: "stagnation", name: "Stagnation", lat: -28, lng: -116, color: 0x64748b, connections: [7, 2], connectionWeights: [0.79, 0.21] },
    ],
    childDefs: [
      { parentIdx: 1, names: ["Microloans", "Supplier Credit"] },
      { parentIdx: 2, names: ["Incubators", "R&D Grants"] },
      { parentIdx: 5, names: ["Port Digitize", "Trade Corridors"] },
    ],
  },
  "carbon-tax-loop": {
    id: "carbon-tax-loop",
    domains: [
      { id: "legislation", name: "Legislation", lat: 42, lng: -96, color: 0xa855f7, connections: [1, 7], connectionWeights: [0.62, 0.38] },
      { id: "enforcement", name: "Enforcement", lat: 16, lng: -61, color: 0xec4899, connections: [2, 3], connectionWeights: [0.7, 0.3] },
      { id: "industry", name: "Industry Shift", lat: 53, lng: 12, color: 0x10b981, connections: [3, 4], connectionWeights: [0.76, 0.24] },
      { id: "green-capex", name: "Green Capex", lat: 31, lng: 80, color: 0x0ea5e9, connections: [4, 5], connectionWeights: [0.58, 0.42] },
      { id: "growth", name: "Green Growth", lat: -19, lng: -30, color: 0x22c55e, connections: [5, 6], connectionWeights: [0.67, 0.33] },
      { id: "jobs-shift", name: "Job Transition", lat: -10, lng: 42, color: 0x84cc16, connections: [6, 7], connectionWeights: [0.56, 0.44] },
      { id: "stability", name: "Market Stability", lat: -39, lng: 122, color: 0x14b8a6, connections: [6, 4], connectionWeights: [0.74, 0.26] },
      { id: "lobby", name: "Lobby Backlash", lat: -29, lng: -120, color: 0x64748b, connections: [7, 1], connectionWeights: [0.81, 0.19] },
    ],
    childDefs: [
      { parentIdx: 2, names: ["Wind R&D", "Solar Subsidy", "Hydrogen"] },
      { parentIdx: 5, names: ["Reskilling", "Wage Bridge"] },
      { parentIdx: 7, names: ["Delay Rules", "Court Appeals"] },
    ],
  },
  "urban-transit": {
    id: "urban-transit",
    domains: [
      { id: "urban-design", name: "Urban Design", lat: 41, lng: -98, color: 0xf43f5e, connections: [1, 2], connectionWeights: [0.5, 0.5] },
      { id: "subway", name: "Subway Expansion", lat: 14, lng: -57, color: 0x0ea5e9, connections: [3, 7], connectionWeights: [0.8, 0.2] },
      { id: "pricing", name: "Congestion Pricing", lat: 52, lng: 9, color: 0xf97316, connections: [3, 7], connectionWeights: [0.55, 0.45] },
      { id: "mode-shift", name: "Mode Shift", lat: 29, lng: 81, color: 0x10b981, connections: [4, 7], connectionWeights: [0.72, 0.28] },
      { id: "ridership", name: "Ridership Gain", lat: -20, lng: -29, color: 0x22c55e, connections: [5, 7], connectionWeights: [0.82, 0.18] },
      { id: "air-quality", name: "Air Quality", lat: -11, lng: 39, color: 0x22d3ee, connections: [6, 7], connectionWeights: [0.74, 0.26] },
      { id: "economic-up", name: "Systemic Success", lat: -40, lng: 119, color: 0x06b6d4, connections: [] },
      { id: "backlash", name: "Backlash Spiral", lat: -31, lng: -122, color: 0x64748b, connections: [] },
    ],
    childDefs: [
      { parentIdx: 1, names: ["Rolling Stock", "Signal Upgrade"] },
      { parentIdx: 2, names: ["Core Tolling", "Outer Ring Fee"] },
      { parentIdx: 4, names: ["Last-Mile", "Night Service"] },
    ],
  },
};

const POLICY_PRESETS: PolicyPreset[] = [
  {
    id: "ev-charging",
    label: "EV Charging",
    scenarioId: "ev-charging",
    years: 15,
    iterations: 1800,
    discountRate: 0.038,
    volatility: 0.62,
    funding: [78, 45, 52, 44, 72, 48, 76, 80],
  },
  {
    id: "childhood-education",
    label: "Childhood Education",
    scenarioId: "childhood-education",
    years: 22,
    iterations: 2100,
    discountRate: 0.03,
    volatility: 0.5,
    funding: [52, 50, 70, 88, 56, 62, 64, 50],
  },
  {
    id: "business-development",
    label: "Business Development",
    scenarioId: "business-development",
    years: 14,
    iterations: 1900,
    discountRate: 0.04,
    volatility: 0.68,
    funding: [68, 52, 58, 56, 84, 62, 60, 72],
  },
  {
    id: "carbon-tax-loop",
    label: "Carbon Tax Loop",
    scenarioId: "carbon-tax-loop",
    years: 18,
    iterations: 2000,
    discountRate: 0.034,
    volatility: 0.72,
    funding: [58, 55, 54, 49, 66, 59, 86, 57],
  },
  {
    id: "urban-transit",
    label: "Urban Transit 30Y",
    scenarioId: "urban-transit",
    years: 30,
    iterations: 1500,
    discountRate: 0.04,
    volatility: 0.62,
    funding: [62, 55, 58, 61, 72, 68, 88, 38],
  },
];

function latLngToVec3(lat: number, lng: number, radius: number): THREE.Vector3 {
  const phi = (90 - lat) * (Math.PI / 180);
  const theta = (lng + 180) * (Math.PI / 180);
  return new THREE.Vector3(
    -radius * Math.sin(phi) * Math.cos(theta),
    radius * Math.cos(phi),
    radius * Math.sin(phi) * Math.sin(theta),
  );
}

function createTextSprite(text: string, scaleX = 1.8, scaleY = 0.45): THREE.Sprite {
  const canvas = document.createElement("canvas");
  canvas.width = 384;
  canvas.height = 96;
  const ctx = canvas.getContext("2d");
  if (ctx) {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = "#f8fbff";
    ctx.font = "700 31px Inter";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.shadowColor = "rgba(0, 0, 0, 0.65)";
    ctx.shadowBlur = 8;
    ctx.fillText(text, canvas.width / 2, canvas.height / 2 + 1);
  }
  const texture = new THREE.CanvasTexture(canvas);
  const material = new THREE.SpriteMaterial({
    map: texture,
    transparent: true,
    depthTest: false,
    depthWrite: false,
  });
  const sprite = new THREE.Sprite(material);
  sprite.scale.set(scaleX, scaleY, 1);
  return sprite;
}

const nodeGlowVert = `
varying vec3 vNormal;
varying vec3 vViewPosition;
void main() {
  vNormal = normalize(normalMatrix * normal);
  vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
  vViewPosition = -mvPosition.xyz;
  gl_Position = projectionMatrix * mvPosition;
}
`;

const nodeGlowFrag = `
uniform vec3 uColor;
uniform float uPulse;
uniform float uTime;
varying vec3 vNormal;
varying vec3 vViewPosition;
void main() {
  vec3 N = normalize(vNormal);
  vec3 V = normalize(vViewPosition);
  float fresnel = pow(1.0 - max(0.0, dot(N, V)), 2.5);
  float pulse = 0.7 + 0.3 * sin(uTime * 2.0) * uPulse;
  float glow = fresnel * 1.2;
  vec3 core = uColor * pulse;
  vec3 rim = uColor * glow * 1.5;
  gl_FragColor = vec4(core + rim, 0.85 + glow * 0.15);
}
`;

const globeVert = `
varying vec3 vNormal;
varying vec3 vViewPosition;
void main() {
  vNormal = normalize(normalMatrix * normal);
  vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
  vViewPosition = -mvPosition.xyz;
  gl_Position = projectionMatrix * mvPosition;
}
`;

const globeFrag = `
varying vec3 vNormal;
varying vec3 vViewPosition;
void main() {
  vec3 N = normalize(vNormal);
  vec3 V = normalize(vViewPosition);
  float fresnel = pow(1.0 - max(0.0, dot(N, V)), 1.5);
  float alpha = 0.08 + fresnel * 0.25;
  vec3 color = vec3(0.4, 0.6, 0.9);
  gl_FragColor = vec4(color, alpha);
}
`;

interface PolicyNode {
  mesh: THREE.Mesh;
  material: THREE.ShaderMaterial;
  label: THREE.Sprite;
  position: THREE.Vector3;
  funding: number;
  domain: PolicyDomain;
}

interface PolicyConnection {
  line: THREE.Line;
  material: THREE.LineBasicMaterial;
  fromIdx: number;
  toIdx: number;
  particles: THREE.Points;
  particleMaterial: THREE.PointsMaterial;
  particlePositions: Float32Array;
  curvePoints: THREE.Vector3[];
  phaseOffset: number;
  flowSpeed: number;
  flowDirection: 1 | -1;
}

interface ChildNode {
  mesh: THREE.Mesh;
  label: THREE.Sprite;
  position: THREE.Vector3;
  parentIdx: number;
}

interface ChildLink {
  line: THREE.Line;
  material: THREE.LineBasicMaterial;
}

class PolicySimVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 220);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;
  private fpsCounter = new FpsCounter("policy");
  private time = 0;

  private globe!: THREE.Mesh;
  private globeWireframe!: THREE.LineSegments;
  private nodes: PolicyNode[] = [];
  private childNodes: ChildNode[] = [];
  private connections: PolicyConnection[] = [];
  private childLinks: ChildLink[] = [];
  private networkGroup = new THREE.Group();
  private domains: PolicyDomain[] = [];
  private childDefs: ChildNodeDef[] = [];

  private markovModel!: MarkovChainModel;
  private shadowCostModel!: ShadowCostModel;
  private monteCarloModel!: MonteCarloModel;
  private histogram = new PolicyHistogram(28, 1.55);
  private histogramValues = Array.from({ length: 28 }, () => 0);
  private histogramMeta: HistogramMeta = {
    xMin: -1,
    xMax: 1,
    yMin: 0,
    yMax: 1,
    mean: 0,
    meanA: 0,
    meanB: 0,
    bimodal: false,
    breakEvenX: 0,
    p10: -0.5,
    p90: 0.5,
  };

  private activePreset = POLICY_PRESETS[0];
  private simParams: MonteCarloParams = {
    years: this.activePreset.years,
    iterations: this.activePreset.iterations,
    discountRate: this.activePreset.discountRate,
    volatility: this.activePreset.volatility,
  };
  private modelDirty = true;
  private lastModelSolve = 0;
  private lastBatchSolve = 0;
  private monteCarloVisible = true;
  private summaryText = "running model";
  private accumulatedNpvs: number[] = [];
  private totalSimulations = 0;
  private batchStartNode = 0;
  private activeScenarioId = "";

  orbitSpeed = 0.08;
  private globeRadius = 3;
  private nodeRadius = 3.4;

  constructor(container: HTMLElement) {
    this.container = container;

    this.renderer.setClearColor(0x0a0a12, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const canvas = this.renderer.domElement;
    canvas.style.position = "absolute";
    canvas.style.top = "0";
    canvas.style.left = "0";
    canvas.style.width = "100%";
    canvas.style.height = "100%";
    this.container.appendChild(canvas);

    // MOBILE OPTIMIZATION: Pull camera back (from 10 to 13.5)
    this.camera.position.set(0, 2.5, 13.5);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(this.camera);
    this.scene.add(this.networkGroup);
    this.scene.add(new THREE.AmbientLight(0xffffff, 0.34));
    const keyLight = new THREE.DirectionalLight(0xffffff, 0.7);
    keyLight.position.set(5, 6, 5);
    this.scene.add(keyLight);

    this.initGlobe();
    this.initHistogram();
    this.applyPreset(this.activePreset);

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

  private initGlobe() {
    const globeGeo = new THREE.IcosahedronGeometry(this.globeRadius, 2);
    const globeMat = new THREE.ShaderMaterial({
      vertexShader: globeVert,
      fragmentShader: globeFrag,
      transparent: true,
      depthWrite: false,
      side: THREE.FrontSide,
    });
    this.globe = new THREE.Mesh(globeGeo, globeMat);
    this.networkGroup.add(this.globe);

    const wireGeo = new THREE.IcosahedronGeometry(this.globeRadius * 1.002, 2);
    const wireMat = new THREE.LineBasicMaterial({
      color: 0x4488cc,
      transparent: true,
      opacity: 0.15,
    });
    this.globeWireframe = new THREE.LineSegments(new THREE.WireframeGeometry(wireGeo), wireMat);
    this.networkGroup.add(this.globeWireframe);
  }

  private initNodes() {
    for (const domain of this.domains) {
      const pos = latLngToVec3(domain.lat, domain.lng, this.nodeRadius);
      const nodeGeo = new THREE.SphereGeometry(0.18, 16, 16);
      const mat = new THREE.ShaderMaterial({
        uniforms: {
          uColor: { value: new THREE.Color(domain.color) },
          uPulse: { value: 1.0 },
          uTime: { value: 0 },
        },
        vertexShader: nodeGlowVert,
        fragmentShader: nodeGlowFrag,
        transparent: true,
      });

      const mesh = new THREE.Mesh(nodeGeo, mat);
      mesh.position.copy(pos);
      this.networkGroup.add(mesh);

      const label = createTextSprite(domain.name, 2.05, 0.5);
      label.position.set(pos.x, pos.y + 0.48, pos.z);
      this.networkGroup.add(label);

      this.nodes.push({ mesh, material: mat, label, position: pos, funding: 50, domain });
    }
  }

  private initChildNodes() {
    for (const childDef of this.childDefs) {
      const parent = this.nodes[childDef.parentIdx];
      if (!parent) continue;

      for (let i = 0; i < childDef.names.length; i++) {
        const theta = (i / childDef.names.length) * Math.PI * 2 + childDef.parentIdx * 0.23;
        const radial = 0.55 + i * 0.06;
        const pos = parent.position
          .clone()
          .add(new THREE.Vector3(Math.cos(theta) * radial, 0.28 + i * 0.03, Math.sin(theta) * radial));

        const mat = new THREE.MeshStandardMaterial({
          color: 0x95b9d8,
          emissive: 0x2e4355,
          emissiveIntensity: 0.8,
          roughness: 0.35,
          metalness: 0.25,
          transparent: true,
          opacity: 0.95,
        });
        const nodeGeo = new THREE.SphereGeometry(0.11, 14, 14);
        const mesh = new THREE.Mesh(nodeGeo, mat);
        mesh.position.copy(pos);
        this.networkGroup.add(mesh);

        const label = createTextSprite(childDef.names[i], 1.55, 0.4);
        label.position.set(pos.x, pos.y + 0.26, pos.z);
        this.networkGroup.add(label);

        const lineGeo = new THREE.BufferGeometry().setFromPoints([parent.position, pos]);
        const lineMat = new THREE.LineBasicMaterial({ color: 0x7ea7c8, transparent: true, opacity: 0.45 });
        const line = new THREE.Line(lineGeo, lineMat);
        this.networkGroup.add(line);

        this.childNodes.push({ mesh, label, position: pos, parentIdx: childDef.parentIdx });
        this.childLinks.push({ line, material: lineMat });
      }
    }
  }

  private initConnections() {
    const built = new Set<string>();

    for (let i = 0; i < this.domains.length; i++) {
      for (const j of this.domains[i].connections) {
        const key = i < j ? `${i}-${j}` : `${j}-${i}`;
        if (built.has(key)) continue;
        built.add(key);

        const from = this.nodes[i].position;
        const to = this.nodes[j].position;

        const mid = new THREE.Vector3().addVectors(from, to).multiplyScalar(0.5);
        mid.normalize().multiplyScalar(this.nodeRadius + 1.2);

        const curve = new THREE.QuadraticBezierCurve3(from, mid, to);
        const curvePoints = curve.getPoints(32);
        const lineGeo = new THREE.BufferGeometry().setFromPoints(curvePoints);
        const lineMat = new THREE.LineBasicMaterial({ color: 0x44aa66, transparent: true, opacity: 0.35 });
        const line = new THREE.Line(lineGeo, lineMat);
        this.networkGroup.add(line);

        const particleCount = 6;
        const particlePositions = new Float32Array(particleCount * 3);
        const particleGeo = new THREE.BufferGeometry();
        particleGeo.setAttribute("position", new THREE.BufferAttribute(particlePositions, 3));
        const particleMat = new THREE.PointsMaterial({
          color: 0x88ffaa,
          size: 0.06,
          transparent: true,
          opacity: 0.7,
          sizeAttenuation: true,
        });
        const particles = new THREE.Points(particleGeo, particleMat);
        this.networkGroup.add(particles);

        this.connections.push({
          line,
          material: lineMat,
          fromIdx: i,
          toIdx: j,
          particles,
          particleMaterial: particleMat,
          particlePositions,
          curvePoints,
          phaseOffset: Math.random(),
          flowSpeed: 0.3,
          flowDirection: 1,
        });
      }
    }
  }

  private initHistogram() {
    this.histogram.group.position.set(0, -2.35, -7.5);
    this.histogram.group.rotation.set(0, 0, 0);
    this.camera.add(this.histogram.group);
    
    // MOBILE OPTIMIZATION: Hide axes and labels while keeping bars
    this.histogram.setCompactMode(true);
  }

  private clearScenario(): void {
    for (const conn of this.connections) {
      this.networkGroup.remove(conn.line, conn.particles);
      conn.line.geometry.dispose();
      conn.material.dispose();
      conn.particles.geometry.dispose();
      conn.particleMaterial.dispose();
    }
    this.connections = [];

    for (const node of this.nodes) {
      this.networkGroup.remove(node.mesh, node.label);
      node.mesh.geometry.dispose();
      node.material.dispose();
      const mat = node.label.material as THREE.SpriteMaterial;
      mat.map?.dispose();
      mat.dispose();
    }
    this.nodes = [];

    for (const child of this.childNodes) {
      this.networkGroup.remove(child.mesh, child.label);
      child.mesh.geometry.dispose();
      (child.mesh.material as THREE.Material).dispose();
      const mat = child.label.material as THREE.SpriteMaterial;
      mat.map?.dispose();
      mat.dispose();
    }
    this.childNodes = [];

    for (const link of this.childLinks) {
      this.networkGroup.remove(link.line);
      link.line.geometry.dispose();
      link.material.dispose();
    }
    this.childLinks = [];
  }

  private setupScenario(scenarioId: string): void {
    const scenario = POLICY_SCENARIOS[scenarioId] ?? POLICY_SCENARIOS[POLICY_PRESETS[0].scenarioId];
    const scenarioChanged = scenario.id !== this.activeScenarioId;
    if (scenarioChanged) {
      this.clearScenario();
      this.activeScenarioId = scenario.id;
      this.domains = scenario.domains.map((d) => ({ ...d, connections: [...d.connections], connectionWeights: d.connectionWeights ? [...d.connectionWeights] : undefined }));
      this.childDefs = scenario.childDefs.map((c) => ({ parentIdx: c.parentIdx, names: [...c.names] }));
      this.initNodes();
      this.initChildNodes();
      this.initConnections();
    }
    this.markovModel = new MarkovChainModel(this.domains);
    this.shadowCostModel = new ShadowCostModel(this.domains);
    this.monteCarloModel = new MonteCarloModel(this.markovModel, this.shadowCostModel);
  }

  applyPreset(preset: PolicyPreset): void {
    this.activePreset = preset;
    this.setupScenario(preset.scenarioId);
    this.simParams = {
      years: preset.years,
      iterations: preset.iterations,
      discountRate: preset.discountRate,
      volatility: preset.volatility,
    };

    for (let i = 0; i < this.nodes.length; i++) {
      this.nodes[i].funding = preset.funding[i] ?? 50;
    }

    this.modelDirty = true;
    this.resetSimulationAccumulator();
    this.solveModel(true);
  }

  setVolatility(value: number): void {
    this.simParams.volatility = value;
    this.modelDirty = true;
    this.resetSimulationAccumulator();
    this.solveModel(true);
  }

  setMonteCarloVisible(visible: boolean): void {
    this.monteCarloVisible = visible;
    this.histogram.setVisible(visible);
  }

  getMonteCarloVisible(): boolean {
    return this.monteCarloVisible;
  }

  getVolatility(): number {
    return this.simParams.volatility;
  }

  getActivePreset(): PolicyPreset {
    return this.activePreset;
  }

  getSummaryText(): string {
    return this.summaryText;
  }

  private solveModel(force = false): void {
    if (!force && !this.modelDirty && this.time - this.lastModelSolve < 1.5) return;

    const nodeScores = this.nodes.map((node, idx) =>
      this.shadowCostModel.estimateExpectedNodeValue(idx, node.funding, this.simParams),
    );

    this.markovModel.updateWeights(nodeScores, this.simParams.volatility);

    for (const conn of this.connections) {
      const forward = this.markovModel.getWeight(conn.fromIdx, conn.toIdx);
      const reverse = this.markovModel.getWeight(conn.toIdx, conn.fromIdx);
      const dominant = Math.max(forward, reverse);
      const net = forward - reverse;

      conn.flowDirection = net >= 0 ? 1 : -1;
      conn.flowSpeed = 0.015 + dominant * 0.08;

      const hue = 0.03 + dominant * 0.33;
      const color = new THREE.Color().setHSL(hue, 0.78, 0.56);
      conn.material.color.copy(color);
      conn.material.opacity = 0.18 + dominant * 0.72;
      conn.particleMaterial.color.copy(color.clone().offsetHSL(0.05, -0.08, 0.1));
    }

    this.runSimulationBatch(true);

    this.lastModelSolve = this.time;
    this.modelDirty = false;
  }

  private resetSimulationAccumulator(): void {
    this.accumulatedNpvs = [];
    this.totalSimulations = 0;
    this.batchStartNode = 0;
    this.lastBatchSolve = 0;
    this.histogramValues = this.histogramValues.map(() => 0);
    this.histogramMeta = {
      xMin: -1,
      xMax: 1,
      yMin: 0,
      yMax: 1,
      mean: 0,
      meanA: 0,
      meanB: 0,
      bimodal: false,
      breakEvenX: 0,
      p10: -0.5,
      p90: 0.5,
    };
    this.summaryText = `${this.simParams.years}Y | E[NPV] 0.00M | P+ 0.0% | Sims 0`;
  }

  private runSimulationBatch(force = false): void {
    if (!force && this.time - this.lastBatchSolve < 0.18) return;
    if (this.nodes.length === 0) return;

    const iterations = Math.max(24, Math.min(120, Math.floor(this.simParams.iterations / 20)));
    const startNode = this.batchStartNode % this.nodes.length;
    this.batchStartNode = (this.batchStartNode + 1) % this.nodes.length;

    const batch = this.monteCarloModel.run(
      startNode,
      this.nodes.map((n) => n.funding),
      { ...this.simParams, iterations },
    );

    for (const result of batch.results) {
      this.accumulatedNpvs.push(result.totalNpv);
    }
    this.totalSimulations += batch.results.length;

    const maxStored = 25000;
    if (this.accumulatedNpvs.length > maxStored) {
      const removeCount = this.accumulatedNpvs.length - maxStored;
      this.accumulatedNpvs.splice(0, removeCount);
    }

    this.rebuildHistogramFromAccumulated();
    this.lastBatchSolve = this.time;
  }

  private rebuildHistogramFromAccumulated(): void {
    if (this.accumulatedNpvs.length === 0) return;

    const min = Math.min(...this.accumulatedNpvs);
    const max = Math.max(...this.accumulatedNpvs);
    const span = Math.max(1, max - min);
    const bins = Array.from({ length: this.histogramValues.length }, () => 0);
    for (const value of this.accumulatedNpvs) {
      const idx = Math.min(bins.length - 1, Math.floor(((value - min) / span) * bins.length));
      bins[idx]++;
    }
    const peak = Math.max(...bins, 1);
    this.histogramValues = bins.map((v) => v / peak);

    const sum = this.accumulatedNpvs.reduce((acc, v) => acc + v, 0);
    const mean = sum / this.accumulatedNpvs.length;
    const sorted = [...this.accumulatedNpvs].sort((a, b) => a - b);
    const q = (p: number) => {
      const idx = Math.min(sorted.length - 1, Math.max(0, Math.floor((sorted.length - 1) * p)));
      return sorted[idx];
    };
    const successInWindow = this.accumulatedNpvs.reduce((acc, v) => acc + (v >= 0 ? 1 : 0), 0);
    const successProbability = successInWindow / Math.max(1, this.accumulatedNpvs.length);
    const modeStats = this.detectModeMeans(bins, min, span, mean);

    this.histogramMeta = {
      xMin: min,
      xMax: max,
      yMin: 0,
      yMax: peak,
      mean,
      meanA: modeStats.meanA,
      meanB: modeStats.meanB,
      bimodal: modeStats.bimodal,
      breakEvenX: 0,
      p10: q(0.1),
      p90: q(0.9),
    };

    const npvM = mean / 1e6;
    const probPct = successProbability * 100;
    this.summaryText = `${this.simParams.years}Y | E[NPV] ${npvM.toFixed(2)}M | P+ ${probPct.toFixed(1)}% | Sims ${this.totalSimulations.toLocaleString()}`;
    
    // Update simulation count in DOM (Marketing Overlay)
    const countEl = document.getElementById("policy-sim-count");
    if (countEl) countEl.innerText = this.totalSimulations.toLocaleString();
  }

  private detectModeMeans(
    bins: number[],
    min: number,
    span: number,
    fallbackMean: number,
  ): { bimodal: boolean; meanA: number; meanB: number } {
    const localPeaks: Array<{ idx: number; count: number }> = [];
    for (let i = 0; i < bins.length; i++) {
      const prev = i > 0 ? bins[i - 1] : -1;
      const next = i < bins.length - 1 ? bins[i + 1] : -1;
      if (bins[i] >= prev && bins[i] >= next && bins[i] > 0) {
        localPeaks.push({ idx: i, count: bins[i] });
      }
    }
    if (localPeaks.length < 2) {
      return { bimodal: false, meanA: fallbackMean, meanB: fallbackMean };
    }

    localPeaks.sort((a, b) => b.count - a.count);
    const primary = localPeaks[0];
    const minGap = Math.max(3, Math.floor(bins.length * 0.2));
    const secondary = localPeaks.find(
      (p) => Math.abs(p.idx - primary.idx) >= minGap && p.count >= primary.count * 0.35,
    );
    if (!secondary) {
      return { bimodal: false, meanA: fallbackMean, meanB: fallbackMean };
    }

    const leftPeak = primary.idx < secondary.idx ? primary : secondary;
    const rightPeak = primary.idx < secondary.idx ? secondary : primary;
    let valleyIdx = leftPeak.idx;
    let valleyCount = bins[leftPeak.idx];
    for (let i = leftPeak.idx + 1; i < rightPeak.idx; i++) {
      if (bins[i] < valleyCount) {
        valleyCount = bins[i];
        valleyIdx = i;
      }
    }

    const splitValue = min + ((valleyIdx + 0.5) / bins.length) * span;
    let leftSum = 0;
    let leftN = 0;
    let rightSum = 0;
    let rightN = 0;
    for (const v of this.accumulatedNpvs) {
      if (v <= splitValue) {
        leftSum += v;
        leftN++;
      } else {
        rightSum += v;
        rightN++;
      }
    }
    if (leftN < 20 || rightN < 20) {
      return { bimodal: false, meanA: fallbackMean, meanB: fallbackMean };
    }
    return {
      bimodal: true,
      meanA: leftSum / leftN,
      meanB: rightSum / rightN,
    };
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
    this.clearScenario();
    this.networkGroup.remove(this.globe, this.globeWireframe);
    this.globe.geometry.dispose();
    (this.globe.material as THREE.Material).dispose();
    this.globeWireframe.geometry.dispose();
    (this.globeWireframe.material as THREE.Material).dispose();
    this.histogram.dispose();
    this.renderer.dispose();
    if (this.container.contains(this.renderer.domElement)) {
      this.container.removeChild(this.renderer.domElement);
    }
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "policy");
    if (!visible) this.fpsCounter.clear();
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;
    this.networkGroup.rotation.y += this.orbitSpeed * 0.016;

    this.globe.rotation.y = this.time * 0.02;
    this.globeWireframe.rotation.y = this.time * 0.02;

    for (const node of this.nodes) {
      const scale = 0.55 + (node.funding / 100) * 1.2;
      node.mesh.scale.setScalar(scale);
      node.material.uniforms.uPulse.value = node.funding / 100;
      node.material.uniforms.uTime.value = this.time;
    }

    this.solveModel(false);
    this.runSimulationBatch(false);

    for (const conn of this.connections) {
      const particleCount = conn.particlePositions.length / 3;
      const curveLen = conn.curvePoints.length;

      for (let p = 0; p < particleCount; p++) {
        const local = (this.time * conn.flowSpeed + conn.phaseOffset + p / particleCount) % 1;
        const t = conn.flowDirection === 1 ? local : 1 - local;
        const f = t * (curveLen - 1);
        const idx = Math.floor(f);
        const frac = f - idx;
        const a = conn.curvePoints[idx];
        const b = conn.curvePoints[Math.min(idx + 1, curveLen - 1)];
        conn.particlePositions[p * 3] = a.x + (b.x - a.x) * frac;
        conn.particlePositions[p * 3 + 1] = a.y + (b.y - a.y) * frac;
        conn.particlePositions[p * 3 + 2] = a.z + (b.z - a.z) * frac;
      }

      conn.particles.geometry.attributes.position.needsUpdate = true;
    }

    if (this.monteCarloVisible) {
      this.histogram.update(this.histogramValues, this.histogramMeta, this.totalSimulations);
    }

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}

export function mountPolicy(container: HTMLElement, sections: SectionManager): VisualizationControl {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Policy simulator section: interactive global policy visualization">
      <h2>Global Policy Simulator</h2>
      <p data-typing-subtitle></p>
      <div class="sim-stats" style="margin-top: 1rem; font-family: 'Inter', monospace; font-size: 0.9rem; color: #00ff88; opacity: 0.8;">
        SIMULATIONS: <span id="policy-sim-count">0</span>
      </div>
    </div>
  `;

  const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement | null;
  const subtitles = [
    "Markov transitions and shadow costs, solved in real time.",
    "Monte Carlo outcomes update with every preset and volatility shift.",
    "Watch policy flow speed adapt as edge weights rebalance.",
    "Use presets to compare short, medium, and long-range horizons.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  sections.setLoadingMessage("s-policy", "loading markov scenarios ...");
  const viz = new PolicySimVisualization(container);

  if (import.meta.env.DEV) {
    const defaultScenarioDomains = POLICY_SCENARIOS[POLICY_PRESETS[0].scenarioId].domains;
    const modelIssues = runPolicyModelTests(defaultScenarioDomains);
    if (modelIssues.length > 0) {
      console.error("[policy] model checks failed", modelIssues);
    } else {
      console.info("[policy] model checks passed");
    }
  }

  const refreshMenu = () => {
    setupPolicyMenu({
      presets: POLICY_PRESETS,
      activePresetId: viz.getActivePreset().id,
      orbitSpeed: viz.orbitSpeed,
      volatility: viz.getVolatility(),
      monteCarloVisible: viz.getMonteCarloVisible(),
      summaryText: viz.getSummaryText(),
      onPresetChange: (presetId: string) => {
        const preset = POLICY_PRESETS.find((p) => p.id === presetId);
        if (!preset) return;
        viz.applyPreset(preset);
        refreshMenu();
      },
      onOrbitSpeedChange: (value: number) => {
        viz.orbitSpeed = value;
      },
      onVolatilityChange: (value: number) => {
        viz.setVolatility(value);
        refreshMenu();
      },
      onToggleMonteCarlo: () => {
        viz.setMonteCarloVisible(!viz.getMonteCarloVisible());
        refreshMenu();
      },
    });
  };

  return {
    dispose: () => {
      viz.dispose();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      viz.setVisible(visible);
    },
    updateUI: () => {
      refreshMenu();
    },
  };
}
