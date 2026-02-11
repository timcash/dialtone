import * as THREE from "three";
import { VisualizationControl, VisibilityMixin, startTyping } from "@ui/ui";
import cubeGlowVert from "../../shaders/wsl-cube.vert.glsl?raw";
import cubeGlowFrag from "../../shaders/wsl-cube.frag.glsl?raw";

class WslHeroViz {
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
    
    const vizContainer = this.container.querySelector('.viz-container') || this.container;
    vizContainer.appendChild(this.renderer.domElement);

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

    this.onResize();
    window.addEventListener('resize', this.onResize);
    this.animate();
  }

  onResize = () => {
    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height);
  };

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, 'wsl-hero-viz');
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
    window.removeEventListener('resize', this.onResize);
    this.renderer.dispose();
    const canvas = this.renderer.domElement;
    if (canvas.parentElement) {
        canvas.parentElement.removeChild(canvas);
    }
  }
}

export function mountHero(container: HTMLElement): VisualizationControl {
  const subtitleEl = container.querySelector('[data-typing-subtitle]') as HTMLParagraphElement;
  let stopTyping = () => {};
  if (subtitleEl) {
      stopTyping = startTyping(subtitleEl, [
          "Alpine Linux nodes.",
          "Real-time telemetry.",
          "Windows host integration.",
      ]);
  }

  const viz = new WslHeroViz(container);

  return {
      dispose: () => {
          viz.dispose();
          stopTyping();
      },
      setVisible: (v) => viz.setVisible(v),
  };
}
