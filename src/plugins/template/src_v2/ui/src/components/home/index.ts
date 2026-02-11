import * as THREE from 'three';
import { VisualizationControl, VisibilityMixin, startTyping } from '../../dialtone-ui';

class HeroSection {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(75, 1, 0.1, 1000);
  private renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  private spheres: THREE.Mesh[] = [];
  private frameId: number = 0;
  isVisible = true;
  frameCount = 0;

  constructor(private container: HTMLElement) {
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.setSize(container.clientWidth, container.clientHeight);
    this.renderer.setClearColor(0x000000, 1);
    
    // In many Dialtone templates, we look for a viz-container or append to root
    const vizContainer = this.container.querySelector('.viz-container') || this.container;
    vizContainer.appendChild(this.renderer.domElement);

    const geometry = new THREE.SphereGeometry(0.5, 32, 32);
    for (let i = 0; i < 5; i++) {
      const material = new THREE.MeshPhongMaterial({ 
        color: 0x00ff88,
        emissive: 0x004422,
        specular: 0x555555,
        shininess: 30
      });
      const sphere = new THREE.Mesh(geometry, material);
      sphere.position.set(
        (Math.random() - 0.5) * 4,
        (Math.random() - 0.5) * 4,
        (Math.random() - 0.5) * 2
      );
      this.scene.add(sphere);
      this.spheres.push(sphere);
    }

    const light = new THREE.DirectionalLight(0xffffff, 1);
    light.position.set(1, 1, 2);
    this.scene.add(light);
    
    const ambientLight = new THREE.AmbientLight(0x404040, 2);
    this.scene.add(ambientLight);

    this.camera.position.set(0, 0, 5);
    
    window.addEventListener('resize', this.onResize);
    this.animate();
  }

  dispose() {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.onResize);
    this.renderer.dispose();
    const canvas = this.renderer.domElement;
    if (canvas.parentElement) {
        canvas.parentElement.removeChild(canvas);
    }
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, 'hero-viz');
  }

  private onResize = () => {
    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height);
  };

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;
    this.frameCount++;

    this.spheres.forEach((sphere, i) => {
      sphere.rotation.x += 0.01 * (i + 1);
      sphere.rotation.y += 0.015;
    });
    this.renderer.render(this.scene, this.camera);
  };
}

export function mountHero(container: HTMLElement): VisualizationControl {
    // If container is empty, inject default hero layout
    if (!container.innerHTML.trim() || !container.querySelector('.marketing-overlay')) {
        container.innerHTML = `
            <div id="viz-container" class="viz-container"></div>
            <div class="marketing-overlay" aria-label="Hero Title">
                <h2>dialtone.template</h2>
                <p data-typing-subtitle></p>
            </div>
        `;
    }

    const subtitleEl = container.querySelector('[data-typing-subtitle]') as HTMLParagraphElement;
    let stopTyping = () => {};
    if (subtitleEl) {
        stopTyping = startTyping(subtitleEl, [
            "High-performance plugin architecture.",
            "Built with TypeScript and Three.js.",
            "Civic technology for the near future.",
        ]);
    }

    const viz = new HeroSection(container);

    return {
        dispose: () => {
            viz.dispose();
            stopTyping();
            container.innerHTML = '';
        },
        setVisible: (v) => viz.setVisible(v),
    };
}
