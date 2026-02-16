import * as THREE from "three";
import { EffectComposer } from "three/examples/jsm/postprocessing/EffectComposer.js";
import { RenderPass } from "three/examples/jsm/postprocessing/RenderPass.js";
import { UnrealBloomPass } from "three/examples/jsm/postprocessing/UnrealBloomPass.js";
import { Line2 } from "three/examples/jsm/lines/Line2.js";
import { LineGeometry } from "three/examples/jsm/lines/LineGeometry.js";
import { LineMaterial } from "three/examples/jsm/lines/LineMaterial.js";
import { FpsCounter } from "../util/fps";
import { GpuTimer } from "../util/gpu_timer";
import { VisibilityMixin } from "../util/section";
import { CIRCLE_OF_FIFTHS, NOTE_TO_INDEX } from "./constants";
import { AudioAnalyzer } from "./audio";
import { Histogram } from "./histogram";

export class MusicVisualization {
  scene = new THREE.Scene();
  camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  container: HTMLElement;
  frameId = 0;
  resizeObserver?: ResizeObserver;
  gl!: WebGLRenderingContext | WebGL2RenderingContext;
  gpuTimer = new GpuTimer();
  isVisible = true;
  frameCount = 0;
  private fpsCounter = new FpsCounter("music");
  
  public analyzer = new AudioAnalyzer();
  private noteMeshes: THREE.Mesh[] = [];
  private semitoneToMesh = new Map<number, THREE.Mesh>();
  private labelSprites: THREE.Sprite[] = [];
  private circleGroup = new THREE.Group();
  private centerLabelSprite!: THREE.Sprite;
  private histogram = new Histogram();
  
  private majorLine!: Line2;
  private minorLine!: Line2;
  private fifthLine!: Line2;
  
  private composer!: EffectComposer;
  private bloomPass!: UnrealBloomPass;
  
  private time = 0;
  
  sensitivity = 2.0;
  floor = -60;
  rotation = 0;
  autoRotationSpeed = 0.015;

  public isDemoMode = false; 
  public demoMode: 'fifths' | 'major3rd' | 'minor3rd' | 'chromatic-fifths' = 'fifths';
  private demoInterval: any = null;
  public demoNoteIndex = 0;

  constructor(container: HTMLElement) {
    this.container = container;
    this.renderer.setClearColor(0x000000, 0);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));

    const canvas = this.renderer.domElement;
    canvas.style.position = "absolute";
    canvas.style.top = "0";
    canvas.style.left = "0";
    canvas.style.width = "100%";
    canvas.style.height = "100%";
    this.container.appendChild(canvas);

    this.camera.position.set(0, 0, 18);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.5));
    const pointLight = new THREE.PointLight(0x00ffff, 1, 20);
    pointLight.position.set(5, 5, 5);
    this.scene.add(pointLight);

    this.scene.add(this.circleGroup);
    this.histogram.group.position.set(0, -6.5, -2);
    this.scene.add(this.histogram.group);
    
    this.createCircle();
    this.createHarmonicLines();
    this.createCenterLabel();
    
    // Post-processing for intense glow
    this.composer = new EffectComposer(this.renderer);
    this.composer.addPass(new RenderPass(this.scene, this.camera));
    this.bloomPass = new UnrealBloomPass(
      new THREE.Vector2(window.innerWidth, window.innerHeight),
      1.2, // Reduced Strength
      0.4, // Radius
      0.9  // Increased Threshold (less things glow)
    );
    this.composer.addPass(this.bloomPass);

    this.resize();
    this.animate();

    this.gl = this.renderer.getContext();
    this.gpuTimer.init(this.gl);

    if (typeof ResizeObserver !== "undefined") {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    }
    
    const resumeOnGesture = async () => {
        await this.analyzer.resume();
        window.removeEventListener('mousedown', resumeOnGesture);
        window.removeEventListener('keydown', resumeOnGesture);
    };
    window.addEventListener('mousedown', resumeOnGesture);
    window.addEventListener('keydown', resumeOnGesture);

    this.enableMic();
  }

  async toggleDemo() {
    if (!this.isDemoMode) {
      this.isDemoMode = true;
      this.demoMode = 'fifths';
      await this.analyzer.resume();
      this.startDemoLoop();
    } else {
      // Cycle through modes, then turn off
      const modes: ('fifths' | 'major3rd' | 'minor3rd' | 'chromatic-fifths')[] = ['fifths', 'major3rd', 'minor3rd', 'chromatic-fifths'];
      const currentIndex = modes.indexOf(this.demoMode);
      
      if (currentIndex === modes.length - 1) {
        this.isDemoMode = false;
        this.stopDemoLoop();
      } else {
        this.demoMode = modes[currentIndex + 1];
        this.stopDemoLoop();
        await this.analyzer.resume();
        this.startDemoLoop();
      }
    }
  }

  async toggleSound() {
      this.analyzer.isSoundOn = !this.analyzer.isSoundOn;
      if (this.isDemoMode) {
          this.stopDemoLoop();
          await this.analyzer.resume();
          this.startDemoLoop();
      }
  }

  public startDemoLoop() {
    if (this.analyzer.isActive) return;

    this.demoNoteIndex = 0;
    
    const getFreq = (noteName: string) => {
      const semitone = NOTE_TO_INDEX[noteName];
      return 440 * Math.pow(2, (semitone - 9) / 12);
    };

    const playNext = async () => {
      if (!this.isDemoMode || this.analyzer.isActive) return;
      
      let duration = 2000;
      
      switch(this.demoMode) {
        case 'major3rd': {
          const baseNote = CIRCLE_OF_FIFTHS[this.demoNoteIndex];
          const freq = getFreq(baseNote);
          await this.analyzer.startDemo(freq);
          this.demoNoteIndex = (this.demoNoteIndex + 4) % 12; // 4 steps in circle is Major 3rd relationship? No, 4 steps clockwise is actually Major 3rd in music theory *on circle of fifths*.
          break;
        }
        case 'minor3rd': {
          const baseNote = CIRCLE_OF_FIFTHS[this.demoNoteIndex];
          const freq = getFreq(baseNote);
          await this.analyzer.startDemo(freq);
          this.demoNoteIndex = (this.demoNoteIndex + 3) % 12; // 3 steps clockwise is Minor 3rd relationship
          break;
        }
        case 'chromatic-fifths': {
          const startNoteName = CIRCLE_OF_FIFTHS[this.demoNoteIndex];
          const endNoteIndex = (this.demoNoteIndex + 1) % 12;
          const endNoteName = CIRCLE_OF_FIFTHS[endNoteIndex];
          
          const startSemi = NOTE_TO_INDEX[startNoteName];
          let endSemi = NOTE_TO_INDEX[endNoteName];
          // We want to move towards the next note in the circle
          // This mode plays the chromatic scale between the two fifths
          const diff = (endSemi - startSemi + 12) % 12;
          
          for (let i = 0; i <= diff; i++) {
            const currentSemi = (startSemi + i) % 12;
            const isBound = i === 0 || i === diff;
            const freq = 440 * Math.pow(2, (currentSemi - 9) / 12);
            await this.analyzer.startDemo(freq);
            await new Promise(r => setTimeout(r, isBound ? 1000 : 300));
          }
          
          this.demoNoteIndex = endNoteIndex;
          duration = 500;
          break;
        }
        default: { // fifths
          const noteName = CIRCLE_OF_FIFTHS[this.demoNoteIndex];
          const freq = getFreq(noteName);
          await this.analyzer.startDemo(freq);
          this.demoNoteIndex = (this.demoNoteIndex + 1) % CIRCLE_OF_FIFTHS.length;
          break;
        }
      }
      
      this.demoInterval = setTimeout(playNext, duration);
    };
    playNext();
  }

  public stopDemoLoop() {
    if (this.demoInterval) {
      clearTimeout(this.demoInterval);
      this.demoInterval = null;
    }
    this.analyzer.stopDemo();
  }

  async enableMic() {
    this.stopDemoLoop();
    this.isDemoMode = false;
    await this.analyzer.enable();
  }

  private createCenterLabel() {
    const canvas = document.createElement("canvas");
    canvas.width = 256;
    canvas.height = 128;
    const labelTexture = new THREE.CanvasTexture(canvas);
    const spriteMaterial = new THREE.SpriteMaterial({ map: labelTexture, transparent: true });
    this.centerLabelSprite = new THREE.Sprite(spriteMaterial);
    this.centerLabelSprite.scale.set(3, 1.5, 1);
    this.scene.add(this.centerLabelSprite);
  }

  private updateCenterLabel(freq: number, note: string, energy: number) {
    const canvas = this.centerLabelSprite.material.map!.image as HTMLCanvasElement;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    if (energy > 0.1) {
      ctx.fillStyle = "rgba(255, 255, 255, 0.9)";
      ctx.font = "bold 60px Inter";
      ctx.textAlign = "center";
      ctx.fillText(note, 128, 65);
      
      ctx.fillStyle = "rgba(200, 200, 200, 0.7)";
      ctx.font = "24px Inter";
      ctx.fillText(`${freq.toFixed(1)} Hz`, 128, 105);
    }
    this.centerLabelSprite.material.map!.needsUpdate = true;
  }

  private createCircle() {
    const radius = 3;
    const geometry = new THREE.SphereGeometry(0.2, 32, 32);
    
    CIRCLE_OF_FIFTHS.forEach((note, i) => {
      const angle = (i / 12) * Math.PI * 2 - Math.PI / 2;
      const x = Math.cos(angle) * radius;
      const y = -Math.sin(angle) * radius; 

      const noteIndex = NOTE_TO_INDEX[note];
      const hue = noteIndex / 12;
      const material = new THREE.MeshStandardMaterial({
        color: new THREE.Color().setHSL(hue, 0.8, 0.5),
        emissive: new THREE.Color().setHSL(hue, 0.8, 0.2),
        metalness: 0.8,
        roughness: 0.2
      });

      const mesh = new THREE.Mesh(geometry, material);
      mesh.position.set(x, y, 0);
      
      const freq = 440 * Math.pow(2, (noteIndex - 9) / 12);
      
      mesh.userData = { note, baseScale: 1, angle, noteIndex, freq };
      this.circleGroup.add(mesh);
      this.noteMeshes.push(mesh);
      this.semitoneToMesh.set(noteIndex, mesh);

      const labelCanvas = document.createElement("canvas");
      labelCanvas.width = 128;
      labelCanvas.height = 64;
      const ctx = labelCanvas.getContext("2d");
      if (ctx) {
        ctx.fillStyle = "#ffffff";
        ctx.font = "bold 28px Inter";
        ctx.textAlign = "center";
        ctx.fillText(note, 64, 40);
      }
      const labelTexture = new THREE.CanvasTexture(labelCanvas);
      const spriteMaterial = new THREE.SpriteMaterial({ map: labelTexture, transparent: true, opacity: 0.8 });
      const sprite = new THREE.Sprite(spriteMaterial);
      sprite.position.set(x * 1.25, y * 1.25, 0);
      sprite.scale.set(1.0, 0.5, 1);
      this.circleGroup.add(sprite);
      this.labelSprites.push(sprite);
    });

    const lineMaterial = new THREE.LineBasicMaterial({ color: 0x444444, transparent: true, opacity: 0.3 });
    for (let i = 0; i < 12; i++) {
      const next = (i + 1) % 12;
      const points = [this.noteMeshes[i].position, this.noteMeshes[next].position];
      const lineGeo = new THREE.BufferGeometry().setFromPoints(points);
      const line = new THREE.Line(lineGeo, lineMaterial);
      this.circleGroup.add(line);
    }
  }

  private createHarmonicLines() {
    const mat = (color: number) => new LineMaterial({
        color,
        linewidth: 4,
        transparent: true,
        opacity: 0,
        blending: THREE.AdditiveBlending,
        depthTest: false,
        resolution: new THREE.Vector2(window.innerWidth, window.innerHeight)
    });
    
    this.majorLine = new Line2(new LineGeometry(), mat(0xff00ff));
    this.minorLine = new Line2(new LineGeometry(), mat(0x00ffff));
    this.fifthLine = new Line2(new LineGeometry(), mat(0xffff00));
    
    this.circleGroup.add(this.majorLine, this.minorLine, this.fifthLine);
  }

  resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
    this.composer.setSize(width, height);
    this.bloomPass.setSize(width, height);
    
    const pr = window.devicePixelRatio;
    [this.majorLine, this.minorLine, this.fifthLine].forEach(line => {
        (line.material as LineMaterial).resolution.set(width * pr, height * pr);
    });
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    this.renderer.dispose();
    this.stopDemoLoop();
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "music");
    if (!visible) {
        this.fpsCounter.clear();
    } else {
        this.analyzer.resume();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    if (!this.isVisible) return;

    this.time += 0.016;
    this.frameCount++;
    
    this.circleGroup.rotation.z = THREE.MathUtils.lerp(this.circleGroup.rotation.z, this.rotation + this.time * this.autoRotationSpeed, 0.1);

    let chroma: Float32Array;
    if (this.isDemoMode && (this.analyzer.isSuspended || !this.analyzer.isActive)) {
        chroma = new Float32Array(12).fill(0);
        const noteName = CIRCLE_OF_FIFTHS[this.demoNoteIndex === 0 ? CIRCLE_OF_FIFTHS.length - 1 : this.demoNoteIndex - 1];
        const semitone = NOTE_TO_INDEX[noteName];
        chroma[semitone] = 1.0; 
    } else {
        chroma = this.analyzer.getChromagram(this.sensitivity, this.floor);
    }
    
    this.histogram.update(
      this.analyzer.getFrequencyData(), 
      this.floor, 
      this.analyzer.sampleRate,
      this.analyzer.fftSize
    );

    let strongestIndex = -1;
    let maxEnergy = 0;

    this.noteMeshes.forEach((mesh, i) => {
      const { noteIndex } = mesh.userData;
      const energy = chroma[noteIndex];
      
      if (energy > maxEnergy) {
        maxEnergy = energy;
        strongestIndex = i;
      }

      // Scale pulse
      const targetScale = 1.0 + energy * 2.0;
      mesh.scale.lerp(new THREE.Vector3(targetScale, targetScale, targetScale), 0.2);

      const mat = mesh.material as THREE.MeshStandardMaterial;
      mat.emissiveIntensity = 0.1 + energy * 2.0;
      
      mesh.position.z = Math.sin(this.time + mesh.userData.angle * 2) * 0.1;
    });

    if (strongestIndex >= 0 && maxEnergy > 0.1) {
      const strongestMesh = this.noteMeshes[strongestIndex];
      const strongest = strongestMesh.userData;
      const strongestSemitone = strongest.noteIndex;
      
      const realFreq = this.isDemoMode ? strongest.freq : this.analyzer.getPeakFrequency();
      this.updateCenterLabel(realFreq, strongest.note, maxEnergy);

      const updateHarmonicLine = (line: Line2, interval: number) => {
        const targetSemitone = (strongestSemitone + interval) % 12;
        const targetMesh = this.semitoneToMesh.get(targetSemitone);
        if (targetMesh) {
          const points = [
              strongestMesh.position.x, strongestMesh.position.y, strongestMesh.position.z,
              targetMesh.position.x, targetMesh.position.y, targetMesh.position.z
          ];
          line.geometry.setPositions(points);
          const mat = line.material as LineMaterial;
          mat.opacity = THREE.MathUtils.lerp(mat.opacity, maxEnergy * 0.8, 0.2);
        }
      };

      updateHarmonicLine(this.majorLine, 4);
      updateHarmonicLine(this.minorLine, 3);
      updateHarmonicLine(this.fifthLine, 7);
    } else {
      this.updateCenterLabel(0, "", 0);
      [this.majorLine, this.minorLine, this.fifthLine].forEach(line => {
        const mat = line.material as LineMaterial;
        mat.opacity = THREE.MathUtils.lerp(mat.opacity, 0, 0.1);
      });
    }

    const cpuStart = performance.now();
    this.gpuTimer.begin(this.gl);
    this.composer.render();
    this.gpuTimer.end(this.gl);
    this.gpuTimer.poll(this.gl);
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, this.gpuTimer.lastMs);
  };
}
