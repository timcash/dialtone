import * as THREE from "three";

export class Histogram {
    group = new THREE.Group();
    private bars: THREE.Mesh[] = [];
    private labels: THREE.Sprite[] = [];
    private labelStates: { holdTime: number, decayTime: number }[] = [];
    private barCount = 64;
    private barWidth = 0.12;
    private barSpacing = 0.03;
    private maxHeight = 3;

    constructor() {
        const geometry = new THREE.BoxGeometry(this.barWidth, 1, this.barWidth);
        // Shift geometry so the origin is at the bottom face
        geometry.translate(0, 0.5, 0);

        for (let i = 0; i < this.barCount; i++) {
            const hue = i / this.barCount;
            const material = new THREE.MeshStandardMaterial({
                color: new THREE.Color().setHSL(hue, 0.7, 0.5),
                emissive: new THREE.Color().setHSL(hue, 0.7, 0.2),
                roughness: 0.4,
                metalness: 0.6,
                transparent: true,
                opacity: 0.7
            });

            const bar = new THREE.Mesh(geometry, material);
            const x = (i - (this.barCount - 1) / 2) * (this.barWidth + this.barSpacing);
            bar.position.set(x, 0, 0);
            bar.scale.y = 0.01;
            this.bars.push(bar);
            this.group.add(bar);

            // Create a pool of labels
            const labelSprite = this.createLabelSprite();
            labelSprite.position.set(x, 0, 0.2);
            labelSprite.visible = false;
            this.labels.push(labelSprite);
            this.group.add(labelSprite);
            this.labelStates.push({ holdTime: 0, decayTime: 0 });
        }
    }

    private createLabelSprite(): THREE.Sprite {
        const canvas = document.createElement("canvas");
        canvas.width = 128; // Increased for better resolution
        canvas.height = 64;
        const texture = new THREE.CanvasTexture(canvas);
        const material = new THREE.SpriteMaterial({ map: texture, transparent: true, depthTest: false, blending: THREE.AdditiveBlending });
        const sprite = new THREE.Sprite(material);
        sprite.scale.set(1.0, 0.5, 1);
        return sprite;
    }

    private updateLabelStyle(sprite: THREE.Sprite, text: string, holdTime: number) {
        const canvas = sprite.material.map!.image as HTMLCanvasElement;
        const ctx = canvas.getContext("2d");
        if (!ctx) return;
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        
        // Hold time 0-1 (0-1s)
        const intensity = Math.min(holdTime / 60, 1.0); // Assuming 60fps
        
        const r = Math.floor(150 + intensity * 105);
        const g = Math.floor(150 + intensity * 105);
        const b = Math.floor(150 + intensity * 105);
        
        ctx.shadowBlur = intensity * 15;
        ctx.shadowColor = "white";
        ctx.fillStyle = `rgb(${r},${g},${b})`;
        ctx.font = "bold 32px Arial";
        ctx.textAlign = "center";
        ctx.fillText(text, 64, 45);
        
        sprite.material.map!.needsUpdate = true;
        sprite.scale.set(1.0 + intensity * 0.4, 0.5 + intensity * 0.2, 1);
    }

    update(data: Float32Array, floor: number = -90, sampleRate: number = 44100, fftSize: number = 4096) {
        if (data.length === 0) return;

        const maxBin = Math.floor(data.length * 0.04);
        const binWidth = sampleRate / fftSize;
        const threshold = 0.35; 
        
        const heights = new Float32Array(this.barCount);
        const freqs = new Float32Array(this.barCount);

        // First pass: calculate heights and frequencies
        for (let i = 0; i < this.barCount; i++) {
            const t = i / this.barCount;
            const startBin = Math.floor(Math.pow(t, 1.5) * maxBin);
            const endBin = Math.floor(Math.pow((i + 1) / this.barCount, 1.5) * maxBin);
            const step = Math.max(1, endBin - startBin);

            let maxVal = -Infinity;
            let peakFreq = 0;
            for (let j = 0; j < step; j++) {
                const idx = startBin + j;
                if (idx < data.length && data[idx] > maxVal) {
                    maxVal = data[idx];
                    peakFreq = idx * binWidth;
                }
            }
            
            if (maxVal === -Infinity) maxVal = floor;
            heights[i] = Math.max(0, (maxVal - floor) / (0 - floor));
            freqs[i] = peakFreq;
        }

        // Second pass: Update bars and labels with peak detection
        for (let i = 0; i < this.barCount; i++) {
            const normalized = heights[i];
            const targetHeight = 0.05 + Math.pow(normalized, 1.1) * this.maxHeight;
            
            this.bars[i].scale.y = THREE.MathUtils.lerp(this.bars[i].scale.y, targetHeight, 0.25);
            const mat = this.bars[i].material as THREE.MeshStandardMaterial;
            mat.emissiveIntensity = 0.2 + normalized * 2.5;
            mat.opacity = 0.5 + normalized * 0.5;

            // Label Persistence Logic with Neighborhood Peak Detection
            const label = this.labels[i];
            const state = this.labelStates[i];

            // Check if this is a local peak (higher than neighbors)
            const prevH = i > 0 ? heights[i-1] : 0;
            const nextH = i < this.barCount - 1 ? heights[i+1] : 0;
            const isLocalPeak = normalized > threshold && normalized >= prevH && normalized >= nextH;

            if (isLocalPeak) {
                state.holdTime++;
                state.decayTime = 30; 
                label.visible = true;
                label.position.y = THREE.MathUtils.lerp(label.position.y, targetHeight + 0.5, 0.2);
                
                const peakFreq = freqs[i];
                const freqText = peakFreq > 1000 ? (peakFreq/1000).toFixed(1) + "k" : Math.round(peakFreq).toString();
                this.updateLabelStyle(label, freqText, state.holdTime);
                (label.material as THREE.SpriteMaterial).opacity = THREE.MathUtils.lerp((label.material as THREE.SpriteMaterial).opacity, 1, 0.3);
            } else {
                state.holdTime = Math.max(0, state.holdTime - 4);
                if (state.decayTime > 0) {
                    state.decayTime--;
                    // Fade faster if it's explicitly NOT a peak anymore (overtaken by neighbor)
                    const fadeSpeed = (normalized < prevH || normalized < nextH) ? 0.5 : 0.3;
                    (label.material as THREE.SpriteMaterial).opacity = THREE.MathUtils.lerp((label.material as THREE.SpriteMaterial).opacity, 0, fadeSpeed);
                } else {
                    (label.material as THREE.SpriteMaterial).opacity = THREE.MathUtils.lerp((label.material as THREE.SpriteMaterial).opacity, 0, 0.4);
                }
                
                if ((label.material as THREE.SpriteMaterial).opacity < 0.05) {
                    label.visible = false;
                }
            }
        }
    }
}
