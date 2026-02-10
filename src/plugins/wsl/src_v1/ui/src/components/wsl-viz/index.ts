import * as THREE from "three";
import { VisibilityMixin } from "../../util/section";
import cubeGlowVert from "../../shaders/template-cube.vert.glsl?raw";
import cubeGlowFrag from "../../shaders/template-cube.frag.glsl?raw";

class WslVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  container: HTMLElement;
  frameId = 0;
  isVisible = true;
  private nodes: THREE.Mesh[] = [];
  private time = 0;

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 0);
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.domElement.style.width = "100%";
    this.renderer.domElement.style.height = "100%";
    this.container.appendChild(this.renderer.domElement);

    this.camera.position.set(0, 0, 5);
    this.camera.lookAt(0, 0, 0);

    const cubeGeo = new THREE.BoxGeometry(0.8, 0.8, 0.8);
    for(let i=0; i<8; i++) {
        const mat = new THREE.ShaderMaterial({
            uniforms: {
                uColor: { value: new THREE.Color(0x00ff88) },
                uGlowColor: { value: new THREE.Color(0x00aaff) },
                uLightDir: { value: new THREE.Vector3(1,1,1).normalize() },
                uTime: { value: 0 },
            },
            vertexShader: cubeGlowVert,
            fragmentShader: cubeGlowFrag,
            transparent: true,
        });
        const mesh = new THREE.Mesh(cubeGeo, mat);
        mesh.position.set((Math.random()-0.5)*6, (Math.random()-0.5)*6, (Math.random()-0.5)*4);
        this.scene.add(mesh);
        this.nodes.push(mesh);
    }

    this.animate();
  }

  resize() {
    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  }

  setVisible(visible: boolean) {
    this.isVisible = visible;
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.nodes.forEach((node, i) => {
        node.rotation.x += 0.01 * (i+1);
        node.rotation.y += 0.015;
        const mat = node.material as THREE.ShaderMaterial;
        mat.uniforms.uTime.value = this.time;
    });

    this.renderer.render(this.scene, this.camera);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.renderer.dispose();
    if (this.container.contains(this.renderer.domElement)) {
        this.container.removeChild(this.renderer.domElement);
    }
  }
}

export function mountWslViz(container: HTMLElement) {
  const viz = new WslVisualization(container);
  viz.resize();
  window.addEventListener("resize", () => viz.resize());

  return {
    dispose: () => viz.dispose(),
    setVisible: (v: boolean) => viz.setVisible(v),
  };
}
