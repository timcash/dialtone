import * as THREE from "three";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupPolicyMenu } from "./menu";

// Policy domains with positions spread across a sphere
const POLICY_DOMAINS = [
  { name: "Energy",         lat: 40,  lng: -100, color: 0xf59e0b, connections: [1, 4, 6] },
  { name: "Water",          lat: 10,  lng: -60,  color: 0x3b82f6, connections: [0, 5, 7] },
  { name: "Health",         lat: 50,  lng: 10,   color: 0xef4444, connections: [3, 5, 7] },
  { name: "Education",      lat: 30,  lng: 80,   color: 0x8b5cf6, connections: [2, 6, 7] },
  { name: "Infrastructure", lat: -20, lng: -30,  color: 0x10b981, connections: [0, 1, 6] },
  { name: "Agriculture",    lat: -10, lng: 40,   color: 0x22d3ee, connections: [1, 2, 7] },
  { name: "Climate",        lat: -40, lng: 120,  color: 0x06b6d4, connections: [0, 3, 4] },
  { name: "Transport",      lat: -30, lng: -120, color: 0xfbbf24, connections: [2, 3, 5] },
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
  position: THREE.Vector3;
  funding: number;
  domain: typeof POLICY_DOMAINS[number];
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
}

class PolicySimVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 200);
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
  private connections: PolicyConnection[] = [];

  orbitSpeed = 0.08;
  private orbitAngle = 0;
  private cameraRadius = 10;
  private cameraHeight = 3;
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

    const existingCanvas = container.querySelector("canvas");
    if (existingCanvas) {
      this.renderer.domElement = existingCanvas as HTMLCanvasElement;
    } else {
      this.container.appendChild(canvas);
    }

    this.camera.position.set(0, this.cameraHeight, this.cameraRadius);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.3));
    const keyLight = new THREE.DirectionalLight(0xffffff, 0.6);
    keyLight.position.set(5, 5, 5);
    this.scene.add(keyLight);

    this.initGlobe();
    this.initNodes();
    this.initConnections();

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
    this.scene.add(this.globe);

    const wireGeo = new THREE.IcosahedronGeometry(this.globeRadius * 1.002, 2);
    const wireMat = new THREE.LineBasicMaterial({
      color: 0x4488cc,
      transparent: true,
      opacity: 0.15,
    });
    this.globeWireframe = new THREE.LineSegments(
      new THREE.WireframeGeometry(wireGeo),
      wireMat,
    );
    this.scene.add(this.globeWireframe);
  }

  private initNodes() {
    const nodeGeo = new THREE.SphereGeometry(0.18, 16, 16);

    for (const domain of POLICY_DOMAINS) {
      const pos = latLngToVec3(domain.lat, domain.lng, this.nodeRadius);
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
      this.scene.add(mesh);

      this.nodes.push({ mesh, material: mat, position: pos, funding: 50, domain });
    }
  }

  private initConnections() {
    const built = new Set<string>();

    for (let i = 0; i < POLICY_DOMAINS.length; i++) {
      for (const j of POLICY_DOMAINS[i].connections) {
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
        const lineMat = new THREE.LineBasicMaterial({
          color: 0x44aa66,
          transparent: true,
          opacity: 0.4,
        });
        const line = new THREE.Line(lineGeo, lineMat);
        this.scene.add(line);

        const numParticles = 6;
        const particlePositions = new Float32Array(numParticles * 3);
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
        this.scene.add(particles);

        this.connections.push({
          line, material: lineMat, fromIdx: i, toIdx: j,
          particles, particleMaterial: particleMat, particlePositions, curvePoints,
        });
      }
    }
  }

  setFunding(index: number, value: number) {
    if (index >= 0 && index < this.nodes.length) {
      this.nodes[index].funding = value;
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

    this.orbitAngle += this.orbitSpeed * 0.016;
    const camX = Math.cos(this.orbitAngle) * this.cameraRadius;
    const camZ = Math.sin(this.orbitAngle) * this.cameraRadius;
    this.camera.position.set(camX, this.cameraHeight, camZ);
    this.camera.lookAt(0, 0, 0);

    this.globe.rotation.y = this.time * 0.02;
    this.globeWireframe.rotation.y = this.time * 0.02;

    for (const node of this.nodes) {
      const scale = 0.5 + (node.funding / 100) * 1.5;
      node.mesh.scale.setScalar(scale);
      node.material.uniforms.uPulse.value = node.funding / 100;
      node.material.uniforms.uTime.value = this.time;
    }

    for (const conn of this.connections) {
      const fromFunding = this.nodes[conn.fromIdx].funding;
      const toFunding = this.nodes[conn.toIdx].funding;
      const balance = (fromFunding + toFunding) / 200;

      conn.material.color.setRGB(1.0 - balance, balance, balance * 0.3);
      conn.material.opacity = 0.2 + balance * 0.5;
      conn.particleMaterial.color.setRGB(0.3 + balance * 0.5, 0.5 + balance * 0.5, 0.3 + balance * 0.15);

      const numP = conn.particlePositions.length / 3;
      const curveLen = conn.curvePoints.length;
      for (let p = 0; p < numP; p++) {
        const t = ((this.time * 0.3 + p / numP) % 1.0);
        const idx = Math.floor(t * (curveLen - 1));
        const frac = t * (curveLen - 1) - idx;
        const a = conn.curvePoints[idx];
        const bPt = conn.curvePoints[Math.min(idx + 1, curveLen - 1)];
        conn.particlePositions[p * 3] = a.x + (bPt.x - a.x) * frac;
        conn.particlePositions[p * 3 + 1] = a.y + (bPt.y - a.y) * frac;
        conn.particlePositions[p * 3 + 2] = a.z + (bPt.z - a.z) * frac;
      }
      conn.particles.geometry.attributes.position.needsUpdate = true;
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

export function mountPolicy(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Policy simulator section: interactive global policy visualization">
      <h2>Global Policy Simulator</h2>
      <p data-typing-subtitle></p>
    </div>
  `;

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]",
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Adjust funding. Watch cascading effects.",
    "Eight domains. One interconnected system.",
    "Balance resources across a simulated world.",
    "Every policy decision has consequences.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new PolicySimVisualization(container);
  const menuOptions = {
    domains: POLICY_DOMAINS.map((d) => d.name),
    orbitSpeed: viz.orbitSpeed,
    onFundingChange: (index: number, value: number) => viz.setFunding(index, value),
    onOrbitSpeedChange: (value: number) => { viz.orbitSpeed = value; },
  };

  return {
    dispose: () => {
      viz.dispose();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      viz.setVisible(visible);
      if (visible) {
        setupPolicyMenu(menuOptions);
      }
    },
  };
}
