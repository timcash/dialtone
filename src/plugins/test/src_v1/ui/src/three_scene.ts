import * as THREE from "three";

export type ThreeSceneControl = {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
};

export function mountSphereScene(canvas: HTMLCanvasElement): ThreeSceneControl {
  const renderer = new THREE.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: true,
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.setClearColor(0x000000, 1);

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  camera.position.set(0, 0.25, 3.1);

  const sphere = new THREE.Mesh(
    new THREE.SphereGeometry(0.95, 48, 32),
    new THREE.MeshStandardMaterial({
      color: 0x63b3ff,
      roughness: 0.32,
      metalness: 0.1,
    }),
  );
  scene.add(sphere);

  const ring = new THREE.Mesh(
    new THREE.TorusGeometry(1.3, 0.02, 18, 100),
    new THREE.MeshBasicMaterial({ color: 0x79f0c8 }),
  );
  ring.rotation.x = Math.PI / 2;
  ring.position.y = -0.9;
  scene.add(ring);

  scene.add(new THREE.AmbientLight(0xffffff, 0.48));
  const key = new THREE.DirectionalLight(0xffffff, 1.15);
  key.position.set(2.3, 2.6, 2.1);
  scene.add(key);

  const fill = new THREE.PointLight(0x79f0c8, 0.9, 12);
  fill.position.set(-2.1, 0.4, -1.2);
  scene.add(fill);

  let raf = 0;
  let active = true;
  const clock = new THREE.Clock();

  const resize = () => {
    const width = Math.max(1, canvas.clientWidth || canvas.parentElement?.clientWidth || 1);
    const height = Math.max(1, canvas.clientHeight || canvas.parentElement?.clientHeight || 1);
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  };

  const tick = () => {
    if (!active) return;
    raf = window.requestAnimationFrame(tick);
    const t = clock.getElapsedTime();
    sphere.rotation.y = t * 0.5;
    sphere.rotation.x = Math.sin(t * 0.35) * 0.08;
    ring.rotation.z = t * 0.18;
    renderer.render(scene, camera);
  };

  const ro = new ResizeObserver(() => resize());
  ro.observe(canvas);
  resize();
  tick();

  return {
    dispose: () => {
      active = false;
      if (raf) window.cancelAnimationFrame(raf);
      ro.disconnect();
      sphere.geometry.dispose();
      (sphere.material as THREE.Material).dispose();
      ring.geometry.dispose();
      (ring.material as THREE.Material).dispose();
      renderer.dispose();
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        if (active) return;
        active = true;
        resize();
        tick();
        return;
      }
      active = false;
      if (raf) window.cancelAnimationFrame(raf);
    },
  };
}
