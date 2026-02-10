import * as THREE from 'three';
import { startTyping } from '../../util/typing';

export class HomeSection {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
  private renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  private spheres: THREE.Mesh[] = [];
  private frameId: number = 0;
  private stopTyping: (() => void) | null = null;

  constructor(private container: HTMLElement) {
    this.renderer.setPixelRatio(window.devicePixelRatio);
    this.renderer.setSize(window.innerWidth, window.innerHeight);
    this.renderer.setClearColor(0x000000, 1);
    
    const vizContainer = this.container.querySelector('#viz-container') as HTMLElement;
    if (vizContainer) {
      vizContainer.appendChild(this.renderer.domElement);
    } else {
      this.container.appendChild(this.renderer.domElement);
    }

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

    this.camera.position.z = 3;
  }

  async mount() {
    this.animate();
    window.addEventListener('resize', this.onResize);

    const subtitleEl = this.container.querySelector('[data-typing-subtitle]') as HTMLParagraphElement;
    if (subtitleEl) {
      this.stopTyping = startTyping(subtitleEl, [
        "High-performance plugin architecture.",
        "Built with TypeScript and Three.js.",
        "Civic technology for the near future.",
      ]);
    }
  }

  unmount() {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.onResize);
    if (this.stopTyping) this.stopTyping();
    this.renderer.dispose();
    if (this.renderer.domElement.parentElement) {
        this.renderer.domElement.parentElement.removeChild(this.renderer.domElement);
    }
  }

  setVisible(_visible: boolean) {
    // This is handled by SectionManager and CSS (.is-active)
    // but we can add logic here if needed.
  }

  private onResize = () => {
    this.camera.aspect = window.innerWidth / window.innerHeight;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(window.innerWidth, window.innerHeight);
  };

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    this.spheres.forEach((sphere, i) => {
      sphere.rotation.x += 0.01 * (i + 1);
      sphere.rotation.y += 0.015;
    });
    this.renderer.render(this.scene, this.camera);
    
    // Update FPS display if present
    const fpsEl = document.querySelector('.header-fps');
    if (fpsEl) {
      fpsEl.textContent = `FPS 60`; // Hardcoded for now, or just leave it
    }
  };
}
