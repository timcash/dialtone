import * as THREE from 'three';
import { setupApp } from '../../../../../plugins/ui/src_v1/ui/ui';

try {
  const { sections } = setupApp({ title: 'simple-test', debug: true });

  sections.register('simple-three-stage', {
    containerId: 'simple-three-stage',
    load: async () => {
      const container = document.getElementById('simple-three-stage');
      const canvas = container?.querySelector('canvas') as HTMLCanvasElement;
      
      const scene = new THREE.Scene();
      const camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
      const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
      renderer.setSize(window.innerWidth, window.innerHeight);
      renderer.setPixelRatio(window.devicePixelRatio);

      const geometry = new THREE.BoxGeometry(1, 1, 1);
      const material = new THREE.MeshStandardMaterial({ color: 0x00ff00 });
      const cube = new THREE.Mesh(geometry, material);
      scene.add(cube);

      const light = new THREE.DirectionalLight(0xffffff, 1);
      light.position.set(1, 1, 2);
      scene.add(light);
      scene.add(new THREE.AmbientLight(0x404040));

      camera.position.z = 3;

      let frameId: number;
      const animate = () => {
        frameId = requestAnimationFrame(animate);
        cube.rotation.x += 0.01;
        cube.rotation.y += 0.01;
        renderer.render(scene, camera);
      };

      // Handle interaction
      const interactBtn = document.querySelector('[aria-label="Simple Interaction Button"]');
      interactBtn?.addEventListener('click', () => {
        material.color.set(0xff0000); // Change to red
        container?.setAttribute('data-interacted', 'true');
        console.log('[SimpleTest] Interacted!');
      });

      return {
        setVisible: (visible: boolean) => {
          if (visible) animate();
          else cancelAnimationFrame(frameId);
        },
        dispose: () => {
          cancelAnimationFrame(frameId);
          renderer.dispose();
          geometry.dispose();
          material.dispose();
        }
      };
    },
    header: { visible: true, title: 'Simple Stage' }
  });

  void sections.navigateTo('simple-three-stage');

  // Simulation delay for readiness
  setTimeout(() => {
    const el = document.getElementById('simple-three-stage');
    if (el) el.setAttribute('data-ready', 'true');
    const header = document.querySelector('[aria-label="App Header"]');
    if (header) header.setAttribute('data-boot', 'true');
  }, 500);

} catch (err) {
  console.error('[SimpleTest] Setup failed:', err);
}
