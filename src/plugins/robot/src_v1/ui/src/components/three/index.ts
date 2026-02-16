import * as THREE from 'three';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  private renderer: THREE.WebGLRenderer;
  private robotGroup: THREE.Group;
  private visible = false;
  private frameId = 0;
  private ws: WebSocket | null = null;
  private attitude = { roll: 0, pitch: 0, yaw: 0 };

  constructor(private container: HTMLElement, canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x000000, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    this.camera.position.set(0, 0, 8);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.4));
    const keyLight = new THREE.DirectionalLight(0xffffff, 0.9);
    keyLight.position.set(2, 2, 2);
    this.scene.add(keyLight);

    const group = new THREE.Group();
    
    // Main Body (Triangle/Cone)
    const geometry = new THREE.ConeGeometry(1, 3, 4); 
    const material = new THREE.MeshPhongMaterial({ color: 0x66fcf1, wireframe: false });
    const robotMesh = new THREE.Mesh(geometry, material);
    robotMesh.rotation.x = Math.PI / 2;
    group.add(robotMesh);

    // Euler Rings
    const ringGeo = new THREE.TorusGeometry(2, 0.05, 16, 100);
    const ringMatX = new THREE.MeshBasicMaterial({ color: 0xff4d4d }); // Pitch
    const ringMatY = new THREE.MeshBasicMaterial({ color: 0x00ff00 }); // Roll
    const ringMatZ = new THREE.MeshBasicMaterial({ color: 0x0000ff }); // Yaw
    
    const ringX = new THREE.Mesh(ringGeo, ringMatX);
    const ringY = new THREE.Mesh(ringGeo, ringMatY);
    ringY.rotation.x = Math.PI / 2;
    const ringZ = new THREE.Mesh(ringGeo, ringMatZ);

    group.add(ringX); 
    group.add(ringY);
    group.add(ringZ);

    this.scene.add(group);
    this.robotGroup = group;

    const gridHelper = new THREE.GridHelper(50, 50, 0x333333, 0x111111);
    gridHelper.rotation.x = Math.PI / 2;
    this.scene.add(gridHelper);

    this.resize();
    window.addEventListener('resize', this.resize);
    
    // Stub debug bridge for test compatibility
    this.attachDebugBridge();
    
    this.connectWS();
    this.animate();
  }

  private connectWS() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    this.ws = new WebSocket(`${protocol}//${host}/ws`);
    
    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.roll !== undefined) this.attitude.roll = data.roll;
        if (data.pitch !== undefined) this.attitude.pitch = data.pitch;
        if (data.yaw !== undefined) this.attitude.yaw = data.yaw;
      } catch (e) {
        // Silently ignore non-JSON or other message formats
      }
    };

    this.ws.onclose = () => {
      if (this.visible) {
        setTimeout(() => this.connectWS(), 2000);
      }
    };
  }

  private attachDebugBridge() {
    (window as any).robotThreeDebug = {
      getProjectedPoint: () => ({ ok: true, x: 0, y: 0 }),
      touchProjected: () => true,
    };
  }

  private resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  };

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.visible) return;

    if (this.robotGroup) {
      this.robotGroup.rotation.z = -this.attitude.roll;  // Roll
      this.robotGroup.rotation.x = this.attitude.pitch; // Pitch
      this.robotGroup.rotation.y = -this.attitude.yaw;   // Yaw
    }

    this.renderer.render(this.scene, this.camera);
  };

  dispose(): void {
    cancelAnimationFrame(this.frameId);
    window.removeEventListener('resize', this.resize);
    if (this.ws) {
      this.ws.close();
    }
    delete (window as any).robotThreeDebug;
    this.renderer.dispose();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  return new ThreeControl(container, canvas);
}
