import * as THREE from 'three';

export class CADViewer {
    scene = new THREE.Scene();
    camera = new THREE.PerspectiveCamera(75, 1, 0.1, 1000);
    renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    container: HTMLElement;
    gear?: THREE.Group;
    frameId = 0;

    constructor(container: HTMLElement) {
        this.container = container;
        this.renderer.setSize(container.clientWidth, container.clientHeight);
        this.renderer.setPixelRatio(window.devicePixelRatio);
        this.container.appendChild(this.renderer.domElement);

        this.camera.position.z = 10;
        this.initLights();
        this.fetchCADObject();
        this.animate();

        window.addEventListener('resize', this.onResize);
    }

    initLights() {
        const ambientLight = new THREE.AmbientLight(0x404040, 2);
        this.scene.add(ambientLight);
        const directionalLight = new THREE.DirectionalLight(0xffffff, 2);
        directionalLight.position.set(10, 10, 10);
        this.scene.add(directionalLight);
    }

    async fetchCADObject() {
        try {
            // In a real app, this would fetch from the backend
            // For now, we'll simulate the response or try to fetch it
            const response = await fetch('http://127.0.0.1:8081/api/cad');
            const data = await response.json();
            this.createGear(data.parameters);
        } catch (e) {
            console.error('Failed to fetch CAD object, using default', e);
            this.createGear({ teeth: 12, diameter: 5, thickness: 1 });
        }
    }

    createGear(params: any) {
        if (this.gear) this.scene.remove(this.gear);

        const group = new THREE.Group();
        const { teeth, diameter, thickness } = params;

        // Main cylinder
        const bodyGeo = new THREE.CylinderGeometry(diameter / 2, diameter / 2, thickness, 32);
        const material = new THREE.MeshStandardMaterial({ color: 0x888888, metalness: 0.8, roughness: 0.2 });
        const body = new THREE.Mesh(bodyGeo, material);
        body.rotation.x = Math.PI / 2;
        group.add(body);

        // Simple teeth
        const toothWidth = (Math.PI * diameter) / (teeth * 2);
        const toothHeight = 0.5;
        const toothGeo = new THREE.BoxGeometry(toothWidth, thickness, toothHeight);

        for (let i = 0; i < teeth; i++) {
            const angle = (i / teeth) * Math.PI * 2;
            const tooth = new THREE.Mesh(toothGeo, material);
            tooth.position.set(
                Math.cos(angle) * (diameter / 2 + toothHeight / 2),
                Math.sin(angle) * (diameter / 2 + toothHeight / 2),
                0
            );
            tooth.rotation.z = angle;
            group.add(tooth);
        }

        this.gear = group;
        this.scene.add(this.gear);
    }

    onResize = () => {
        const width = this.container.clientWidth;
        const height = this.container.clientHeight;
        this.renderer.setSize(width, height);
        this.camera.aspect = width / height;
        this.camera.updateProjectionMatrix();
    };

    isVisible = true;
    setVisible(isVisible: boolean) {
        this.isVisible = isVisible;
    }

    animate = () => {
        this.frameId = requestAnimationFrame(this.animate);
        if (!this.isVisible) return;
        if (this.gear) {
            this.gear.rotation.z += 0.01;
        }
        this.renderer.render(this.scene, this.camera);
    };

    dispose() {
        cancelAnimationFrame(this.frameId);
        window.removeEventListener('resize', this.onResize);
        this.renderer.dispose();
        this.container.removeChild(this.renderer.domElement);
    }
}

export function mountCAD(container: HTMLElement) {
    const viewer = new CADViewer(container);
    return {
        dispose: () => viewer.dispose(),
        setVisible: (v: boolean) => viewer.setVisible(v)
    };
}
