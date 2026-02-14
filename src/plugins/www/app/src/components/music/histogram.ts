import * as THREE from "three";

export class Histogram {
    group = new THREE.Group();
    private bars: THREE.Mesh[] = [];
    private barCount = 64;
    private barWidth = 0.12;
    private barSpacing = 0.03;
    private maxHeight = 3;

    constructor() {
        const geometry = new THREE.BoxGeometry(this.barWidth, 1, this.barWidth);
        // Shift geometry so the origin is at the bottom face
        geometry.translate(0, 0.5, 0);

        for (let i = 0; i < this.barCount; i++) {
            const isWhite = i % 2 === 0;
            const material = new THREE.MeshStandardMaterial({
                color: isWhite ? 0xdddddd : 0x666666,
                emissive: isWhite ? 0x444444 : 0x111111,
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
        }
    }

    update(data: Float32Array, floor: number = -90) {
        if (data.length === 0) return;

        // We only visualize the lower half of the frequencies (usually where the most energy is)
        const sampleLimit = Math.floor(data.length / 2);
        const step = Math.floor(sampleLimit / this.barCount);
        
        for (let i = 0; i < this.barCount; i++) {
            let maxVal = -Infinity;
            const start = i * step;
            for (let j = 0; j < step; j++) {
                if (data[start + j] > maxVal) maxVal = data[start + j];
            }
            
            // Normalize dB value
            const normalized = Math.max(0, (maxVal - floor) / (0 - floor));
            const targetHeight = 0.05 + Math.pow(normalized, 1.5) * this.maxHeight;
            
            this.bars[i].scale.y = THREE.MathUtils.lerp(this.bars[i].scale.y, targetHeight, 0.2);
            const mat = this.bars[i].material as THREE.MeshStandardMaterial;
            mat.emissiveIntensity = 0.1 + normalized * 1.5;
            mat.opacity = 0.4 + normalized * 0.5;
        }
    }
}
