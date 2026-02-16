import { type VisualizationControl, type SectionManager } from "../util/section";
import * as THREE from "three";
import glowVertexShader from "../../shaders/glow.vert.glsl?raw";
import glowFragmentShader from "../../shaders/glow.frag.glsl?raw";
import gridVertexShader from "../../shaders/grid.vert.glsl?raw";
import gridFragmentShader from "../../shaders/grid.frag.glsl?raw";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupMathMenu } from "./menu";



const COLORS = {
  cyan: new THREE.Color(0x06b6d4),
  purple: new THREE.Color(0x8b5cf6),
  blue: new THREE.Color(0x3b82f6),
  pink: new THREE.Color(0xec4899),
  green: new THREE.Color(0x10b981),
  orange: new THREE.Color(0xf97316),
  white: new THREE.Color(0xffffff),
};

class MathVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;


  // Orbital groups
  innerOrbit!: THREE.Group;
  middleOrbit!: THREE.Group;
  outerOrbit!: THREE.Group;

  // Individual objects
  torus!: THREE.Mesh;
  torusWireframe!: THREE.LineSegments;
  lemniscate!: THREE.Mesh;
  graphGroup!: THREE.Group;
  nodes: THREE.Mesh[] = [];
  gridBottom!: THREE.Mesh;
  cube!: THREE.LineSegments;
  tetrahedron!: THREE.Mesh;
  octahedron!: THREE.Mesh;
  icosahedron!: THREE.LineSegments;
  kleinBottle!: THREE.Mesh;
  mobiusStrip!: THREE.Mesh;
  sphere!: THREE.Mesh;
  cone!: THREE.Mesh;
  cylinder!: THREE.LineSegments;
  dodecahedron!: THREE.LineSegments;

  // Materials
  materials: THREE.ShaderMaterial[] = [];
  gridMaterial!: THREE.ShaderMaterial;

  // Animation state
  time = 0;
  lastFrameTime = performance.now();

  // Camera orbit parameters - configurable
  cameraOrbitRadius = 16;
  cameraOrbitSpeed = 0.06;
  cameraOrbitAngle = 0;
  cameraHeight = 4;
  cameraHeightOsc = 2;
  cameraHeightSpeed = 0.12;
  cameraLookAtY = -1.5;

  // Orbit rotation speeds
  innerOrbitSpeed = 0.003;
  middleOrbitSpeed = 0.0018;
  outerOrbitSpeed = 0.001;

  // Camera Roll
  cameraRoll = 0;
  cameraRollSpeed = 0;

  // Curve parameters
  curveA = 1;
  curveB = 1;
  curveC = 1;
  curveD = 1;
  curveE = 1;
  curveF = 1;

  // Grid parameters
  gridOpacity = 0.5;
  gridOpacityOsc = 0.2;
  gridOscSpeed = 0.5;


  private fpsCounter = new FpsCounter("math");

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 0);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    const existingCanvas = container.querySelector('canvas');
    if (existingCanvas) {
      this.renderer.domElement = existingCanvas as HTMLCanvasElement;
    } else {
      this.container.appendChild(this.renderer.domElement);
    }

    this.initScene();
    // this.initConfigPanel(); // Menu setup on visibility
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

  createGlowMaterial(
    color: THREE.Color,
    intensity = 1.0,
  ): THREE.ShaderMaterial {
    const mat = new THREE.ShaderMaterial({
      uniforms: {
        uColor: { value: color },
        uIntensity: { value: intensity },
        uTime: { value: 0 },
      },
      vertexShader: glowVertexShader,
      fragmentShader: glowFragmentShader,
      transparent: true,
      side: THREE.DoubleSide,
      blending: THREE.AdditiveBlending,
    });
    this.materials.push(mat);
    return mat;
  }

  createGridMaterial(color: THREE.Color): THREE.ShaderMaterial {
    const mat = new THREE.ShaderMaterial({
      uniforms: {
        uColor: { value: color },
        uTime: { value: 0 },
        uGridSize: { value: 2.0 },
      },
      vertexShader: gridVertexShader,
      fragmentShader: gridFragmentShader,
      transparent: true,
      side: THREE.DoubleSide,
      blending: THREE.AdditiveBlending,
    });
    this.materials.push(mat);
    return mat;
  }

  initScene() {
    this.camera.position.set(0, this.cameraHeight, this.cameraOrbitRadius);
    this.camera.lookAt(0, this.cameraLookAtY, 0);

    // Ambient light
    const ambient = new THREE.AmbientLight(0x404040, 0.6);
    this.scene.add(ambient);

    // Point lights
    const lights = [
      { color: 0x06b6d4, pos: [-12, 10, 12], intensity: 1.0 },
      { color: 0x8b5cf6, pos: [12, 8, -12], intensity: 0.9 },
      { color: 0xec4899, pos: [0, -8, 15], intensity: 0.7 },
      { color: 0x3b82f6, pos: [-15, 5, 0], intensity: 0.6 },
    ];
    lights.forEach(({ color, pos, intensity }) => {
      const light = new THREE.PointLight(color, intensity, 80);
      light.position.set(pos[0], pos[1], pos[2]);
      this.scene.add(light);
    });

    // Create orbital groups
    this.innerOrbit = new THREE.Group();
    this.middleOrbit = new THREE.Group();
    this.outerOrbit = new THREE.Group();
    this.scene.add(this.innerOrbit);
    this.scene.add(this.middleOrbit);
    this.scene.add(this.outerOrbit);

    this.initGrid();
    this.initInnerOrbit();
    this.initMiddleOrbit();
    this.initOuterOrbit();
    this.initGraph();
    this.initMappingLines();
  }

  initGrid() {
    const gridGeo = new THREE.PlaneGeometry(30, 30, 60, 60);
    this.gridMaterial = this.createGridMaterial(COLORS.purple);
    this.gridBottom = new THREE.Mesh(gridGeo, this.gridMaterial);
    this.gridBottom.rotation.x = -Math.PI / 2;
    this.gridBottom.position.y = -4;
    this.scene.add(this.gridBottom);
  }

  initInnerOrbit() {
    const radius = 4;

    // Torus
    const torusGeo = new THREE.TorusGeometry(0.8, 0.3, 32, 64);
    const torusMat = this.createGlowMaterial(COLORS.cyan, 1.2);
    this.torus = new THREE.Mesh(torusGeo, torusMat);
    this.torus.position.set(radius, 0, 0);
    this.innerOrbit.add(this.torus);

    const wireGeo = new THREE.TorusGeometry(0.82, 0.31, 16, 32);
    const wireMat = new THREE.LineBasicMaterial({
      color: 0x22d3ee,
      transparent: true,
      opacity: 0.3,
    });
    this.torusWireframe = new THREE.LineSegments(
      new THREE.WireframeGeometry(wireGeo),
      wireMat,
    );
    this.torus.add(this.torusWireframe);

    // Tetrahedron
    const tetraGeo = new THREE.TetrahedronGeometry(0.8);
    const tetraMat = this.createGlowMaterial(COLORS.pink, 1.0);
    this.tetrahedron = new THREE.Mesh(tetraGeo, tetraMat);
    this.tetrahedron.position.set(-radius, 0, 0);
    this.innerOrbit.add(this.tetrahedron);

    // Sphere
    const sphereGeo = new THREE.SphereGeometry(0.6, 32, 32);
    const sphereMat = this.createGlowMaterial(COLORS.blue, 1.0);
    this.sphere = new THREE.Mesh(sphereGeo, sphereMat);
    this.sphere.position.set(0, 0, radius);
    this.innerOrbit.add(this.sphere);

    // Cube wireframe
    const cubeGeo = new THREE.BoxGeometry(1, 1, 1);
    const cubeMat = new THREE.LineBasicMaterial({
      color: 0x8b5cf6,
      transparent: true,
      opacity: 0.8,
    });
    this.cube = new THREE.LineSegments(
      new THREE.WireframeGeometry(cubeGeo),
      cubeMat,
    );
    this.cube.position.set(0, 0, -radius);
    this.innerOrbit.add(this.cube);
  }

  initMiddleOrbit() {
    const radius = 8;

    // Lemniscate
    const points: THREE.Vector3[] = [];
    const segments = 150;
    const a = 1.5;
    for (let i = 0; i <= segments; i++) {
      const t = (i / segments) * Math.PI * 2;
      const denom = 1 + Math.sin(t) * Math.sin(t);
      const x = (a * Math.cos(t)) / denom;
      const y = (a * Math.sin(t) * Math.cos(t)) / denom;
      points.push(new THREE.Vector3(x, y, 0));
    }
    const curve = new THREE.CatmullRomCurve3(points, true);
    const tubeGeo = new THREE.TubeGeometry(curve, 128, 0.12, 16, true);
    const lemniscateMat = this.createGlowMaterial(COLORS.cyan, 1.0);
    this.lemniscate = new THREE.Mesh(tubeGeo, lemniscateMat);
    this.lemniscate.position.set(radius, 0, 0);
    this.middleOrbit.add(this.lemniscate);

    // Octahedron
    const octaGeo = new THREE.OctahedronGeometry(0.9);
    const octaMat = this.createGlowMaterial(COLORS.green, 1.1);
    this.octahedron = new THREE.Mesh(octaGeo, octaMat);
    this.octahedron.position.set(-radius, 0, 0);
    this.middleOrbit.add(this.octahedron);

    // Icosahedron wireframe
    const icoGeo = new THREE.IcosahedronGeometry(0.8);
    const icoMat = new THREE.LineBasicMaterial({
      color: 0x10b981,
      transparent: true,
      opacity: 0.7,
    });
    this.icosahedron = new THREE.LineSegments(
      new THREE.WireframeGeometry(icoGeo),
      icoMat,
    );
    this.icosahedron.position.set(0, 0, radius);
    this.middleOrbit.add(this.icosahedron);

    // Cone
    const coneGeo = new THREE.ConeGeometry(0.6, 1.2, 32);
    const coneMat = this.createGlowMaterial(COLORS.orange, 1.0);
    this.cone = new THREE.Mesh(coneGeo, coneMat);
    this.cone.position.set(0, 0, -radius);
    this.middleOrbit.add(this.cone);
  }

  initOuterOrbit() {
    const radius = 12;

    // Mobius strip
    const mobiusPoints: THREE.Vector3[] = [];
    const mobiusSegments = 200;
    for (let i = 0; i <= mobiusSegments; i++) {
      const t = (i / mobiusSegments) * Math.PI * 2;
      const r = 1.2;
      const w = 0.4;
      for (let j = -1; j <= 1; j += 0.5) {
        const x = (r + j * w * Math.cos(t / 2)) * Math.cos(t);
        const y = (r + j * w * Math.cos(t / 2)) * Math.sin(t);
        const z = j * w * Math.sin(t / 2);
        mobiusPoints.push(new THREE.Vector3(x, z, y));
      }
    }
    const mobiusCurve = new THREE.CatmullRomCurve3(
      mobiusPoints.filter((_, i) => i % 5 === 0),
      true,
    );
    const mobiusGeo = new THREE.TubeGeometry(mobiusCurve, 100, 0.08, 8, true);
    const mobiusMat = this.createGlowMaterial(COLORS.pink, 0.9);
    this.mobiusStrip = new THREE.Mesh(mobiusGeo, mobiusMat);
    this.mobiusStrip.position.set(radius, 0, 0);
    this.mobiusStrip.scale.setScalar(0.8);
    this.outerOrbit.add(this.mobiusStrip);

    // Dodecahedron wireframe
    const dodecaGeo = new THREE.DodecahedronGeometry(0.9);
    const dodecaMat = new THREE.LineBasicMaterial({
      color: 0xf97316,
      transparent: true,
      opacity: 0.7,
    });
    this.dodecahedron = new THREE.LineSegments(
      new THREE.WireframeGeometry(dodecaGeo),
      dodecaMat,
    );
    this.dodecahedron.position.set(-radius, 0, 0);
    this.outerOrbit.add(this.dodecahedron);

    // Cylinder wireframe
    const cylGeo = new THREE.CylinderGeometry(0.5, 0.5, 1.2, 32);
    const cylMat = new THREE.LineBasicMaterial({
      color: 0x3b82f6,
      transparent: true,
      opacity: 0.7,
    });
    this.cylinder = new THREE.LineSegments(
      new THREE.WireframeGeometry(cylGeo),
      cylMat,
    );
    this.cylinder.position.set(0, 0, radius);
    this.outerOrbit.add(this.cylinder);

    // Klein bottle approximation
    const kleinPoints: THREE.Vector3[] = [];
    for (let i = 0; i <= 100; i++) {
      const u = (i / 100) * Math.PI * 2;
      for (let j = 0; j <= 20; j++) {
        const v = (j / 20) * Math.PI * 2;
        const rr = 0.8;
        const x = (rr + 0.3 * Math.cos(v)) * Math.cos(u);
        const y = (rr + 0.3 * Math.cos(v)) * Math.sin(u);
        const z = 0.3 * Math.sin(v) * Math.sin(u / 2);
        kleinPoints.push(new THREE.Vector3(x, z, y));
      }
    }
    const kleinCurve = new THREE.CatmullRomCurve3(
      kleinPoints.filter((_, i) => i % 7 === 0),
      true,
    );
    const kleinGeo = new THREE.TubeGeometry(kleinCurve, 80, 0.06, 8, true);
    const kleinMat = this.createGlowMaterial(COLORS.purple, 0.8);
    this.kleinBottle = new THREE.Mesh(kleinGeo, kleinMat);
    this.kleinBottle.position.set(0, 0, -radius);
    this.outerOrbit.add(this.kleinBottle);
  }

  initGraph() {
    this.graphGroup = new THREE.Group();
    this.graphGroup.position.set(0, 3, 0);

    const nodePositions = [
      new THREE.Vector3(-1.5, 1.5, 0),
      new THREE.Vector3(-0.5, 1.8, 0.3),
      new THREE.Vector3(0.5, 1.8, -0.2),
      new THREE.Vector3(1.5, 1.5, 0.1),
      new THREE.Vector3(-1.8, 0.5, 0.2),
      new THREE.Vector3(-0.8, 0.7, -0.3),
      new THREE.Vector3(0, 0.5, 0.4),
      new THREE.Vector3(0.8, 0.7, -0.1),
      new THREE.Vector3(1.8, 0.5, 0.2),
      new THREE.Vector3(-1.2, -0.5, 0.1),
      new THREE.Vector3(0, -0.3, -0.2),
      new THREE.Vector3(1.2, -0.5, 0.1),
    ];

    const nodeGeo = new THREE.SphereGeometry(0.15, 24, 24);

    nodePositions.forEach((pos, i) => {
      const nodeMat = this.createGlowMaterial(COLORS.blue, 1.5);
      const node = new THREE.Mesh(nodeGeo, nodeMat);
      node.position.copy(pos);
      node.userData.index = i;
      this.graphGroup.add(node);
      this.nodes.push(node);
    });

    const edges = [
      [0, 1],
      [1, 2],
      [2, 3],
      [0, 4],
      [0, 5],
      [1, 5],
      [1, 6],
      [2, 6],
      [2, 7],
      [3, 7],
      [3, 8],
      [4, 5],
      [5, 6],
      [6, 7],
      [7, 8],
      [4, 9],
      [5, 9],
      [5, 10],
      [6, 10],
      [7, 10],
      [7, 11],
      [8, 11],
      [9, 10],
      [10, 11],
    ];

    const lineMat = new THREE.LineBasicMaterial({
      color: 0x8b5cf6,
      transparent: true,
      opacity: 0.5,
    });

    edges.forEach(([i, j]) => {
      const points = [nodePositions[i], nodePositions[j]];
      const geometry = new THREE.BufferGeometry().setFromPoints(points);
      const line = new THREE.Line(geometry, lineMat.clone());
      this.graphGroup.add(line);
    });

    this.scene.add(this.graphGroup);
  }

  initMappingLines() {
    const createCurve = (
      start: THREE.Vector3,
      end: THREE.Vector3,
      color: number,
    ) => {
      const mid = new THREE.Vector3(
        (start.x + end.x) / 2,
        (start.y + end.y) / 2 + 2,
        (start.z + end.z) / 2,
      );
      const curve = new THREE.QuadraticBezierCurve3(start, mid, end);
      const points = curve.getPoints(40);
      const geometry = new THREE.BufferGeometry().setFromPoints(points);
      const material = new THREE.LineBasicMaterial({
        color,
        transparent: true,
        opacity: 0.25,
      });
      return new THREE.Line(geometry, material);
    };

    const graphPos = new THREE.Vector3(0, 3, 0);
    this.scene.add(createCurve(graphPos, new THREE.Vector3(4, 0, 0), 0x06b6d4));
    this.scene.add(
      createCurve(graphPos, new THREE.Vector3(-4, 0, 0), 0xec4899),
    );
    this.scene.add(createCurve(graphPos, new THREE.Vector3(0, 0, 4), 0x3b82f6));
    this.scene.add(
      createCurve(graphPos, new THREE.Vector3(0, 0, -4), 0x8b5cf6),
    );
  }

  initConfigPanel() {
    // Menu is initialized in mountMath
    return;



  }

  buildConfigSnapshot() {
    return {
      camera: {
        radius: this.cameraOrbitRadius,
        height: this.cameraHeight,
        heightOsc: this.cameraHeightOsc,
        heightSpeed: this.cameraHeightSpeed,
        orbitSpeed: this.cameraOrbitSpeed,
        lookAtY: this.cameraLookAtY,
      },
      orbit: {
        innerSpeed: this.innerOrbitSpeed,
        middleSpeed: this.middleOrbitSpeed,
        outerSpeed: this.outerOrbitSpeed,
      },
    };
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "math");
    if (!visible) {
      this.fpsCounter.clear();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);

    // Skip all calculations when off-screen
    if (!this.isVisible) return;

    const cpuStart = performance.now();
    this.frameCount++;
    const now = performance.now();
    const delta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    this.time += delta;

    // Update shader uniforms
    this.materials.forEach((mat) => {
      mat.uniforms.uTime.value = this.time;
    });

    // Rotate orbital groups
    this.innerOrbit.rotation.y += this.innerOrbitSpeed;
    this.middleOrbit.rotation.y -= this.middleOrbitSpeed;
    this.outerOrbit.rotation.y += this.outerOrbitSpeed;

    // Rotate individual objects
    this.torus.rotation.x += 0.008;
    this.torus.rotation.z += 0.005;
    this.tetrahedron.rotation.x += 0.01;
    this.tetrahedron.rotation.y += 0.008;
    this.sphere.rotation.y += 0.005;
    this.cube.rotation.x += 0.007;
    this.cube.rotation.y += 0.009;
    this.lemniscate.rotation.z += 0.004;
    this.octahedron.rotation.x += 0.006;
    this.octahedron.rotation.z += 0.008;
    this.icosahedron.rotation.x += 0.005;
    this.icosahedron.rotation.y += 0.007;
    this.cone.rotation.y += 0.008;
    this.mobiusStrip.rotation.y += 0.006;
    this.mobiusStrip.rotation.z += 0.003;
    this.dodecahedron.rotation.x += 0.004;
    this.dodecahedron.rotation.y += 0.006;
    this.cylinder.rotation.x += 0.005;
    this.kleinBottle.rotation.y += 0.007;
    this.kleinBottle.rotation.x += 0.003;

    // Animate graph breathing
    const breathe = 1 + Math.sin(this.time * 1.5) * 0.02;
    this.graphGroup.scale.setScalar(breathe);

    // Animate node intensities
    this.nodes.forEach((node, i) => {
      const mat = node.material as THREE.ShaderMaterial;
      mat.uniforms.uIntensity.value =
        1.2 + Math.sin(this.time * 2 + i * 0.7) * 0.5;
    });

    // Camera orbits around center
    this.cameraOrbitAngle += this.cameraOrbitSpeed * delta;

    const camX = Math.sin(this.cameraOrbitAngle) * this.cameraOrbitRadius;
    const camZ = Math.cos(this.cameraOrbitAngle) * this.cameraOrbitRadius;
    const camY =
      this.cameraHeight +
      Math.sin(this.time * this.cameraHeightSpeed) * this.cameraHeightOsc;

    this.camera.position.set(camX, camY, camZ);
    this.camera.lookAt(0, this.cameraLookAtY, 0);

    this.gpuTimer.begin(this.gl);
    this.renderer.render(this.scene, this.camera);
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}

export function mountMath(container: HTMLElement, sections: SectionManager): VisualizationControl {
  // Inject HTML
  container.innerHTML = `
      <div class="marketing-overlay" aria-label="Mathematics marketing information">
        <h2>Mathematics powers autonomy</h2>
        <p data-typing-subtitle></p>
      </div>
    `;



  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "From first principles to autonomous systems.",
    "Experience the logic that drives intelligent behavior.",
    "Learn the math behind motion and control.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  sections.setLoadingMessage("s-math", "loading manifold projections ...");
  const viz = new MathVisualization(container);
  // const menu = setupMathMenu(viz); // Menu setup on visibility

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
      setupMathMenu(viz);
    }
  };
}
