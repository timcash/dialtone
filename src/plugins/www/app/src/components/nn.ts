import * as THREE from "three";
import glowVertexShader from "../shaders/glow.vert.glsl?raw";
import glowFragmentShader from "../shaders/glow.frag.glsl?raw";

const COLORS = {
  input: new THREE.Color(0x06b6d4), // cyan
  hidden: new THREE.Color(0x8b5cf6), // purple
  output: new THREE.Color(0xec4899), // pink
  connection: new THREE.Color(0x3b82f6), // blue
  active: new THREE.Color(0x22d3ee), // bright cyan
};

interface Neuron {
  mesh: THREE.Mesh;
  layer: number;
  index: number;
  position: THREE.Vector3;
  activation: number;
}

interface Connection {
  line: THREE.Line;
  from: Neuron;
  to: Neuron;
  weight: number;
  pulseOffset: number;
}

class NeuralNetworkVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(45, 1, 0.1, 1000);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;

  // Neural network structure
  neurons: Neuron[] = [];
  connections: Connection[] = [];
  layers: number[] = [4, 8, 12, 8, 4];

  // Materials
  materials: THREE.ShaderMaterial[] = [];

  // Animation
  time = 0;
  lastFrameTime = performance.now();

  // Camera - configurable
  cameraOrbitAngle = 0;
  cameraOrbitSpeed = 0.06;
  cameraRadius = 14;
  cameraHeight = 2;
  cameraHeightOsc = 1.5;
  cameraHeightSpeed = 0.15;
  cameraLookAtY = 0;

  // Signal propagation
  signalSpeed = 0.8;
  signalTime = 0;

  // Config panel
  configPanel?: HTMLDivElement;
  configToggle?: HTMLButtonElement;
  private setPanelOpen?: (open: boolean) => void;

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
    this.initConfigPanel();
    this.resize();
    this.animate();

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

  initScene() {
    this.camera.position.set(0, this.cameraHeight, this.cameraRadius);
    this.camera.lookAt(0, this.cameraLookAtY, 0);

    // Ambient light
    const ambient = new THREE.AmbientLight(0x404040, 0.4);
    this.scene.add(ambient);

    // Colored point lights
    const lights = [
      { color: 0x06b6d4, pos: [-10, 8, 10], intensity: 0.8 },
      { color: 0x8b5cf6, pos: [10, 6, -8], intensity: 0.7 },
      { color: 0xec4899, pos: [0, -6, 12], intensity: 0.6 },
    ];
    lights.forEach(({ color, pos, intensity }) => {
      const light = new THREE.PointLight(color, intensity, 60);
      light.position.set(pos[0], pos[1], pos[2]);
      this.scene.add(light);
    });

    this.initNeurons();
    this.initConnections();
    this.initGrid();
  }

  initNeurons() {
    const layerSpacing = 4;
    const totalWidth = (this.layers.length - 1) * layerSpacing;
    const startX = -totalWidth / 2;

    const nodeGeo = new THREE.SphereGeometry(0.25, 24, 24);

    this.layers.forEach((neuronCount, layerIndex) => {
      const x = startX + layerIndex * layerSpacing;
      const totalHeight = (neuronCount - 1) * 1.2;
      const startY = totalHeight / 2;

      let color: THREE.Color;
      if (layerIndex === 0) {
        color = COLORS.input;
      } else if (layerIndex === this.layers.length - 1) {
        color = COLORS.output;
      } else {
        color = COLORS.hidden;
      }

      for (let i = 0; i < neuronCount; i++) {
        const y = startY - i * 1.2;
        const z = Math.sin(layerIndex * 0.5 + i * 0.3) * 0.5;

        const mat = this.createGlowMaterial(color, 1.2);
        const mesh = new THREE.Mesh(nodeGeo, mat);
        const position = new THREE.Vector3(x, y, z);
        mesh.position.copy(position);

        this.scene.add(mesh);

        this.neurons.push({
          mesh,
          layer: layerIndex,
          index: i,
          position,
          activation: Math.random(),
        });
      }
    });
  }

  initConnections() {
    for (let l = 0; l < this.layers.length - 1; l++) {
      const currentLayerNeurons = this.neurons.filter((n) => n.layer === l);
      const nextLayerNeurons = this.neurons.filter((n) => n.layer === l + 1);

      currentLayerNeurons.forEach((fromNeuron) => {
        nextLayerNeurons.forEach((toNeuron) => {
          const connectionProb = l < 2 ? 0.6 : 0.4;
          if (Math.random() > connectionProb) return;

          const weight = Math.random() * 2 - 1;
          const pulseOffset = Math.random() * Math.PI * 2;

          const curve = this.createConnectionCurve(
            fromNeuron.position,
            toNeuron.position,
          );
          const points = curve.getPoints(30);
          const geometry = new THREE.BufferGeometry().setFromPoints(points);

          const material = new THREE.LineBasicMaterial({
            color: COLORS.connection,
            transparent: true,
            opacity: 0.15 + Math.abs(weight) * 0.2,
          });

          const line = new THREE.Line(geometry, material);
          this.scene.add(line);

          this.connections.push({
            line,
            from: fromNeuron,
            to: toNeuron,
            weight,
            pulseOffset,
          });
        });
      });
    }
  }

  createConnectionCurve(
    from: THREE.Vector3,
    to: THREE.Vector3,
  ): THREE.QuadraticBezierCurve3 {
    const mid = new THREE.Vector3(
      (from.x + to.x) / 2,
      (from.y + to.y) / 2,
      (from.z + to.z) / 2 + 0.3,
    );
    return new THREE.QuadraticBezierCurve3(from, mid, to);
  }

  initGrid() {
    const gridGeo = new THREE.PlaneGeometry(30, 30, 30, 30);
    const gridMat = new THREE.MeshBasicMaterial({
      color: 0x8b5cf6,
      wireframe: true,
      transparent: true,
      opacity: 0.08,
    });
    const grid = new THREE.Mesh(gridGeo, gridMat);
    grid.rotation.x = -Math.PI / 2;
    grid.position.y = -6;
    this.scene.add(grid);
  }

  initConfigPanel() {
    const panel = document.getElementById(
      "nn-config-panel",
    ) as HTMLDivElement | null;
    const toggle = document.getElementById(
      "nn-config-toggle",
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

    const addSection = (title: string) => {
      const header = document.createElement("h3");
      header.textContent = title;
      panel.appendChild(header);
    };

    const addSlider = (
      label: string,
      value: number,
      min: number,
      max: number,
      step: number,
      onInput: (v: number) => void,
      format: (v: number) => string = (v) => v.toFixed(2),
    ) => {
      const row = document.createElement("div");
      row.className = "earth-config-row";

      const labelWrap = document.createElement("label");
      labelWrap.textContent = label;

      const slider = document.createElement("input");
      slider.type = "range";
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;

      labelWrap.appendChild(slider);
      row.appendChild(labelWrap);

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
    };

    const addCopyButton = () => {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = "Copy Config";
      button.addEventListener("click", () => {
        const payload = JSON.stringify(this.buildConfigSnapshot(), null, 2);
        if (navigator.clipboard?.writeText) {
          navigator.clipboard
            .writeText(payload)
            .catch(() => console.log(payload));
        } else {
          console.log(payload);
        }
      });
      panel.appendChild(button);
    };

    addSection("Camera");
    addSlider("Radius", this.cameraRadius, 5, 30, 0.5, (v) => {
      this.cameraRadius = v;
    });
    addSlider("Height", this.cameraHeight, -5, 10, 0.5, (v) => {
      this.cameraHeight = v;
    });
    addSlider("Height Osc", this.cameraHeightOsc, 0, 5, 0.1, (v) => {
      this.cameraHeightOsc = v;
    });
    addSlider("Height Speed", this.cameraHeightSpeed, 0, 0.5, 0.01, (v) => {
      this.cameraHeightSpeed = v;
    });
    addSlider(
      "Orbit Speed",
      this.cameraOrbitSpeed,
      0,
      0.2,
      0.005,
      (v) => {
        this.cameraOrbitSpeed = v;
      },
      (v) => v.toFixed(3),
    );
    addSlider("Look At Y", this.cameraLookAtY, -5, 5, 0.5, (v) => {
      this.cameraLookAtY = v;
    });

    addSection("Signal");
    addSlider("Signal Speed", this.signalSpeed, 0.1, 2, 0.1, (v) => {
      this.signalSpeed = v;
    });

    addCopyButton();
  }

  buildConfigSnapshot() {
    return {
      camera: {
        radius: this.cameraRadius,
        height: this.cameraHeight,
        heightOsc: this.cameraHeightOsc,
        heightSpeed: this.cameraHeightSpeed,
        orbitSpeed: this.cameraOrbitSpeed,
        lookAtY: this.cameraLookAtY,
      },
      signal: {
        speed: this.signalSpeed,
      },
    };
  }

  isVisible = true;
  frameCount = 0;

  setVisible(visible: boolean) {
    if (this.isVisible !== visible) {
      console.log(
        `%c[neural] ${visible ? "▶️ Resuming" : "⏸️ Pausing"} at frame ${this.frameCount}`,
        visible ? "color: #22c55e" : "color: #f59e0b",
      );
    }
    this.isVisible = visible;
    if (!visible) {
      this.setPanelOpen?.(false);
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);

    // Skip all calculations when off-screen
    if (!this.isVisible) return;

    this.frameCount++;
    const now = performance.now();
    const delta = (now - this.lastFrameTime) / 1000;
    this.lastFrameTime = now;
    this.time += delta;
    this.signalTime += delta * this.signalSpeed;

    // Update shader uniforms
    this.materials.forEach((mat) => {
      mat.uniforms.uTime.value = this.time;
    });

    // Animate neurons
    this.neurons.forEach((neuron) => {
      const mat = neuron.mesh.material as THREE.ShaderMaterial;
      const layerPhase = neuron.layer * 0.8;
      const wave = Math.sin(
        this.signalTime * 3 - layerPhase + neuron.index * 0.2,
      );
      const activation = 0.8 + wave * 0.5;
      mat.uniforms.uIntensity.value = activation;
      const scale = 1 + wave * 0.15;
      neuron.mesh.scale.setScalar(scale);
    });

    // Animate connections
    this.connections.forEach((conn) => {
      const mat = conn.line.material as THREE.LineBasicMaterial;
      const phase =
        this.signalTime * 2 - conn.from.layer * 0.6 + conn.pulseOffset;
      const pulse = Math.sin(phase);
      const baseOpacity = 0.1 + Math.abs(conn.weight) * 0.15;
      mat.opacity = baseOpacity + Math.max(0, pulse) * 0.4;
      if (pulse > 0.5) {
        mat.color.lerp(conn.weight > 0 ? COLORS.active : COLORS.output, 0.1);
      } else {
        mat.color.lerp(COLORS.connection, 0.05);
      }
    });

    // Camera orbits around center
    this.cameraOrbitAngle += this.cameraOrbitSpeed * delta;

    const camX = Math.sin(this.cameraOrbitAngle) * this.cameraRadius;
    const camZ = Math.cos(this.cameraOrbitAngle) * this.cameraRadius;
    const camY =
      this.cameraHeight +
      Math.sin(this.time * this.cameraHeightSpeed) * this.cameraHeightOsc;

    this.camera.position.set(camX, camY, camZ);
    this.camera.lookAt(0, this.cameraLookAtY, 0);

    this.renderer.render(this.scene, this.camera);
  };
}

export function mountNeuralNetwork(container: HTMLElement) {
  // Inject HTML
  container.innerHTML = `
      <div class="marketing-overlay" aria-label="Neural network marketing information">
        <h2>Mathematics powers autonomy</h2>
        <p>From gradients to control loops. Experience the logic that drives intelligent behavior.</p>
        <button class="buy-button">Coming Soon</button>
      </div>
      <div id="nn-config-panel" class="earth-config-panel" hidden></div>
    `;

  // Create and inject config toggle
  const controls = document.querySelector('.top-right-controls');
  const toggle = document.createElement('button');
  toggle.id = 'nn-config-toggle';
  toggle.className = 'earth-config-toggle';
  toggle.type = 'button';
  toggle.setAttribute('aria-expanded', 'false');
  toggle.textContent = 'Config';
  controls?.prepend(toggle);

  const viz = new NeuralNetworkVisualization(container);
  return {
    dispose: () => {
      viz.dispose();
      toggle.remove();
      container.innerHTML = '';
    },
    setVisible: (visible: boolean) => viz.setVisible(visible),
  };
}
