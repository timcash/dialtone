import * as THREE from 'three';
import { STLLoader } from 'three/examples/jsm/loaders/STLLoader.js';
import glowVertexShader from '../shaders/glow.vert.glsl?raw';
import glowFragmentShader from '../shaders/glow.frag.glsl?raw';

export class CADViewer {
    scene = new THREE.Scene();
    camera = new THREE.PerspectiveCamera(45, 1, 0.1, 2000);
    renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: "high-performance" });
    container: HTMLElement;
    gearGroup = new THREE.Group();
    frameId = 0;

    // Animation state
    time = 0;
    lastFrameTime = performance.now();

    // Parameters (matching gear_generator.py)
    params = {
        outer_diameter: 80,
        inner_diameter: 20,
        thickness: 8,
        tooth_height: 6,
        tooth_width: 4,
        num_teeth: 20,
        num_mounting_holes: 4,
        mounting_hole_diameter: 6
    };

    loader = new STLLoader();
    abortController: AbortController | null = null;
    currentMesh: THREE.Mesh | null = null;
    currentWireframe: THREE.LineSegments | null = null;

    constructor(container: HTMLElement) {
        this.container = container;
        this.renderer.setSize(container.clientWidth, container.clientHeight);
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
        this.renderer.outputColorSpace = THREE.SRGBColorSpace;
        this.container.appendChild(this.renderer.domElement);

        this.scene.add(this.gearGroup);
        // Position camera to see the default 80mm gear nicely
        this.camera.position.set(120, 100, 200);
        this.camera.lookAt(0, 0, 0);

        this.initLights();
        this.initUI();
        this.updateModel();
        this.animate();

        window.addEventListener('resize', this.onResize);
    }

    createGlowMaterial(color: THREE.Color, intensity = 1.0): THREE.ShaderMaterial {
        return new THREE.ShaderMaterial({
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
    }

    initLights() {
        const ambientLight = new THREE.AmbientLight(0xffffff, 0.3);
        this.scene.add(ambientLight);

        const hemiLight = new THREE.HemisphereLight(0xffffff, 0x444444, 1.2);
        hemiLight.position.set(0, 50, 0);
        this.scene.add(hemiLight);

        const dirLight = new THREE.DirectionalLight(0xffffff, 1.5);
        dirLight.position.set(100, 150, 100);
        this.scene.add(dirLight);

        const pointLight = new THREE.PointLight(0x06b6d4, 1.8, 400);
        pointLight.position.set(-100, -50, 100);
        this.scene.add(pointLight);
    }

    initUI() {
        const inputs = [
            'outer_diameter', 'inner_diameter', 'thickness',
            'tooth_height', 'tooth_width', 'num_teeth',
            'num_mounting_holes', 'mounting_hole_diameter'
        ];

        inputs.forEach(id => {
            const inp = document.getElementById(`inp-${id}`) as HTMLInputElement;
            const val = document.getElementById(`val-${id}`) as HTMLSpanElement;
            if (inp && val) {
                // Initialize values
                // @ts-ignore
                inp.value = String(this.params[id]);
                val.textContent = inp.value;

                inp.addEventListener('input', () => {
                    const v = parseFloat(inp.value);
                    // @ts-ignore
                    this.params[id] = v;
                    val.textContent = inp.value;
                    this.debouncedUpdate();
                });
            }
        });

        const dlBtn = document.getElementById('btn-download-stl');
        if (dlBtn) {
            dlBtn.addEventListener('click', (e) => {
                e.preventDefault();
                this.downloadSTL();
            });
        }
    }

    fetchTimeout: any = null;
    debouncedUpdate() {
        if (this.fetchTimeout) clearTimeout(this.fetchTimeout);
        this.fetchTimeout = setTimeout(() => {
            this.updateModel();
            this.fetchSourceCode();
        }, 800);
    }

    async updateModel() {
        if (this.abortController) {
            this.abortController.abort();
        }
        this.abortController = new AbortController();

        try {
            // @ts-ignore
            const isLive = window.CAD_LIVE === true || (typeof process !== 'undefined' && process.env.CAD_LIVE === 'true');
            const baseUrl = isLive ? 'http://127.0.0.1:8081' : '';
            const url = `${baseUrl}/api/cad/generate`;

            const response = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(this.params),
                signal: this.abortController.signal
            });

            if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);

            const arrayBuffer = await response.arrayBuffer();
            const geometry = this.loader.parse(arrayBuffer);
            geometry.center();
            geometry.computeVertexNormals();

            if (this.currentMesh) {
                this.gearGroup.remove(this.currentMesh);
                this.currentMesh.geometry.dispose();
                (this.currentMesh.material as THREE.Material).dispose();
            }
            if (this.currentWireframe) {
                this.gearGroup.remove(this.currentWireframe);
                this.currentWireframe.geometry.dispose();
                (this.currentWireframe.material as THREE.Material).dispose();
            }

            const glowMat = this.createGlowMaterial(new THREE.Color(0x06b6d4), 1.0);
            this.currentMesh = new THREE.Mesh(geometry, glowMat);
            this.gearGroup.add(this.currentMesh);

            const wireMat = new THREE.LineBasicMaterial({ color: 0x3b82f6, transparent: true, opacity: 0.35 });
            this.currentWireframe = new THREE.LineSegments(new THREE.WireframeGeometry(geometry), wireMat);
            this.gearGroup.add(this.currentWireframe);

        } catch (e: any) {
            if (e.name !== 'AbortError') {
                console.warn('CAD model sync failed:', e);
            }
        }
    }

    async fetchSourceCode() {
        const codeElement = document.getElementById('cad-code-content');
        try {
            // @ts-ignore
            const isLive = window.CAD_LIVE === true || (typeof process !== 'undefined' && process.env.CAD_LIVE === 'true');
            const baseUrl = isLive ? 'http://127.0.0.1:8081' : '';
            // We use GET for metadata/source
            const response = await fetch(`${baseUrl}/api/cad`);
            if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
            const data = await response.json();
            if (codeElement && data.source_code) {
                codeElement.textContent = data.source_code;
            }
        } catch (e) {
            console.warn('Failed to fetch source code:', e);
        }
    }

    async downloadSTL() {
        const query = new URLSearchParams(Object.entries(this.params).map(([k, v]) => [k, String(v)])).toString();
        // @ts-ignore
        const isLive = window.CAD_LIVE === true || (typeof process !== 'undefined' && process.env.CAD_LIVE === 'true');
        const baseUrl = isLive ? 'http://127.0.0.1:8081' : '';
        const url = `${baseUrl}/api/cad/download?${query}`;
        window.open(url, '_blank');
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

        const now = performance.now();
        const delta = (now - this.lastFrameTime) / 1000;
        this.lastFrameTime = now;
        this.time += delta;

        // Update shaders
        this.gearGroup.children.forEach(child => {
            if (child instanceof THREE.Mesh && child.material instanceof THREE.ShaderMaterial) {
                child.material.uniforms.uTime.value = this.time;
            }
        });

        if (this.gearGroup) {
            this.gearGroup.rotation.z += 0.005;
            this.gearGroup.rotation.y = Math.sin(this.time * 0.45) * 0.15;
            this.gearGroup.rotation.x = Math.cos(this.time * 0.25) * 0.12;
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
