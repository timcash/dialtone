import * as THREE from "three";

export interface HistogramMeta {
  xMin: number;
  xMax: number;
  yMin: number;
  yMax: number;
  mean: number;
  meanA: number;
  meanB: number;
  bimodal: boolean;
  breakEvenX: number;
  p10: number;
  p90: number;
}

function clamp01(v: number): number {
  return Math.max(0, Math.min(1, v));
}

function formatMillions(value: number): string {
  return `${(value / 1e6).toFixed(2)}M`;
}

function formatUsdMillions(value: number): string {
  const millions = value / 1e6;
  return `$${millions.toFixed(1)}M`;
}

export class PolicyHistogram {
  group = new THREE.Group();
  private bars: THREE.LineSegments[] = [];
  private barCount: number;
  private maxHeight: number;
  private chartWidth: number;
  private baseline: THREE.Line;
  private yAxis: THREE.Line;
  private meanLine: THREE.Line;
  private meanLineB: THREE.Line;
  private p10Line: THREE.Line;
  private p90Line: THREE.Line;
  private breakEvenLine: THREE.Line;
  private xMinLabel: THREE.Sprite;
  private xMaxLabel: THREE.Sprite;
  private p10Label: THREE.Sprite;
  private meanLabel: THREE.Sprite;
  private p90Label: THREE.Sprite;
  private breakEvenLabel: THREE.Sprite;
  private simCountLabel: THREE.Sprite;
  private currentMeanX = 0;
  private currentMeanBX = 0;
  private currentP10X = 0;
  private currentP90X = 0;
  private currentBreakEvenX = 0;
  private smoothedValues: number[] = [];

  constructor(barCount = 28, maxHeight = 2.8) {
    this.barCount = barCount;
    this.maxHeight = maxHeight;

    const spacing = 0.16;
    const width = 0.11;
    this.chartWidth = (this.barCount - 1) * spacing;

    for (let i = 0; i < this.barCount; i++) {
      const boxGeometry = new THREE.BoxGeometry(width, 1, width);
      boxGeometry.translate(0, 0.5, 0);
      const edges = new THREE.EdgesGeometry(boxGeometry);
      boxGeometry.dispose();
      const material = new THREE.LineBasicMaterial({
        color: 0xf5f5f5,
        transparent: true,
        opacity: 0.62,
      });

      const bar = new THREE.LineSegments(edges, material);
      const x = (i - (this.barCount - 1) / 2) * spacing;
      bar.position.set(x, 0, 0);
      bar.scale.y = 0.01;
      this.bars.push(bar);
      this.group.add(bar);
    }

    this.baseline = this.makeLine(0xa3a3a3, 0.65);
    this.yAxis = this.makeLine(0xa3a3a3, 0.65);
    this.meanLine = this.makeLine(0xff4d4d, 0.95);
    this.meanLineB = this.makeLine(0xff4d4d, 0.95);
    this.p10Line = this.makeLine(0xd4d4d4, 0.55);
    this.p90Line = this.makeLine(0xd4d4d4, 0.55);
    this.breakEvenLine = this.makeLine(0xe5e5e5, 0.65);

    this.group.add(this.baseline, this.yAxis, this.meanLine, this.meanLineB, this.p10Line, this.p90Line, this.breakEvenLine);

    this.xMinLabel = this.makeLabel();
    this.xMaxLabel = this.makeLabel();
    this.p10Label = this.makeLabel();
    this.meanLabel = this.makeLabel();
    this.p90Label = this.makeLabel();
    this.breakEvenLabel = this.makeLabel();
    this.simCountLabel = this.makeLabel();

    this.group.add(this.xMinLabel, this.xMaxLabel, this.p10Label, this.meanLabel, this.p90Label, this.breakEvenLabel, this.simCountLabel);
    this.xMinLabel.scale.set(1.45, 0.42, 1);
    this.xMaxLabel.scale.set(1.45, 0.42, 1);

    this.setLinePoints(this.baseline, -this.chartWidth / 2, 0, this.chartWidth / 2, 0);
    this.setLinePoints(this.yAxis, -this.chartWidth / 2, 0, -this.chartWidth / 2, this.maxHeight);
    this.currentMeanX = 0;
    this.currentMeanBX = 0;
    this.currentP10X = -this.chartWidth * 0.2;
    this.currentP90X = this.chartWidth * 0.2;
    this.currentBreakEvenX = 0;
    this.smoothedValues = Array.from({ length: this.barCount }, () => 0);

    this.updateAxisLabels({
      xMin: 0,
      xMax: 1,
      yMin: 0,
      yMax: 1,
      mean: 0,
      meanA: 0,
      meanB: 0,
      bimodal: false,
      breakEvenX: 0,
      p10: 0.1,
      p90: 0.9,
    });
    this.setLabel(this.simCountLabel, "Simulations 0");
    this.simCountLabel.position.set(0, -1.36, 0.05);
    this.simCountLabel.scale.set(2.25, 0.58, 1);
  }

  private makeLine(color: number, opacity: number): THREE.Line {
    const geom = new THREE.BufferGeometry();
    const mat = new THREE.LineBasicMaterial({ color, transparent: true, opacity });
    return new THREE.Line(geom, mat);
  }

  private setLinePoints(line: THREE.Line, x0: number, y0: number, x1: number, y1: number): void {
    (line.geometry as THREE.BufferGeometry).setFromPoints([
      new THREE.Vector3(x0, y0, 0),
      new THREE.Vector3(x1, y1, 0),
    ]);
  }

  private makeLabel(): THREE.Sprite {
    const canvas = document.createElement("canvas");
    canvas.width = 384;
    canvas.height = 96;
    const tex = new THREE.CanvasTexture(canvas);
    const mat = new THREE.SpriteMaterial({
      map: tex,
      transparent: true,
      depthTest: false,
      depthWrite: false,
      opacity: 0.95,
    });
    const sprite = new THREE.Sprite(mat);
    sprite.scale.set(1.8, 0.45, 1);
    return sprite;
  }

  private setLabel(sprite: THREE.Sprite, text: string): void {
    const map = (sprite.material as THREE.SpriteMaterial).map;
    if (!map) return;
    const canvas = map.image as HTMLCanvasElement;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.fillStyle = "#ffffff";
    ctx.font = "bold 32px Arial";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.shadowColor = "rgba(255, 255, 255, 0.55)";
    ctx.shadowBlur = 12;
    ctx.fillText(text, canvas.width / 2, canvas.height / 2 + 1);
    map.needsUpdate = true;
  }

  setVisible(visible: boolean): void {
    this.group.visible = visible;
  }

  private updateAxisLabels(meta: HistogramMeta): void {
    this.setLabel(this.xMinLabel, `Min ${formatUsdMillions(meta.xMin)}`);
    this.setLabel(this.xMaxLabel, `Max ${formatUsdMillions(meta.xMax)}`);
    this.setLabel(this.p10Label, `P10 ${formatMillions(meta.p10)}`);
    this.setLabel(this.meanLabel, `Mean ${formatMillions(meta.mean)}`);
    this.setLabel(this.p90Label, `P90 ${formatMillions(meta.p90)}`);
    this.setLabel(this.breakEvenLabel, "0");

    this.xMinLabel.position.set(-this.chartWidth / 2 - 0.72, 0.22, 0.05);
    this.xMaxLabel.position.set(this.chartWidth / 2 + 0.72, 0.22, 0.05);
    this.p10Label.position.set(this.currentP10X, -1.16, 0.05);
    this.meanLabel.position.set(this.currentMeanX, -0.86, 0.05);
    this.p90Label.position.set(this.currentP90X, -1.16, 0.05);
    this.breakEvenLabel.position.set(this.currentBreakEvenX, -1.44, 0.05);
  }

  update(values: number[], meta: HistogramMeta, simulationCount = 0): void {
    for (let i = 0; i < this.bars.length; i++) {
      const v = values[i] ?? 0;
      this.smoothedValues[i] = THREE.MathUtils.lerp(this.smoothedValues[i], clamp01(v), 0.16);
      const targetHeight = 0.03 + Math.pow(this.smoothedValues[i], 1.05) * this.maxHeight;
      const bar = this.bars[i];
      bar.scale.y = THREE.MathUtils.lerp(bar.scale.y, targetHeight, 0.22);
      const mat = bar.material as THREE.LineBasicMaterial;
      mat.opacity = 0.28 + this.smoothedValues[i] * 0.7;
    }

    const denom = Math.max(1e-6, meta.xMax - meta.xMin);
    const meanTarget = -this.chartWidth / 2 + clamp01((meta.meanA - meta.xMin) / denom) * this.chartWidth;
    const meanBTarget = -this.chartWidth / 2 + clamp01((meta.meanB - meta.xMin) / denom) * this.chartWidth;
    const p10Target = -this.chartWidth / 2 + clamp01((meta.p10 - meta.xMin) / denom) * this.chartWidth;
    const p90Target = -this.chartWidth / 2 + clamp01((meta.p90 - meta.xMin) / denom) * this.chartWidth;
    const breakEvenTarget = -this.chartWidth / 2 + clamp01((meta.breakEvenX - meta.xMin) / denom) * this.chartWidth;

    this.currentMeanX = THREE.MathUtils.lerp(this.currentMeanX, meanTarget, 0.14);
    this.currentMeanBX = THREE.MathUtils.lerp(this.currentMeanBX, meanBTarget, 0.14);
    this.currentP10X = THREE.MathUtils.lerp(this.currentP10X, p10Target, 0.14);
    this.currentP90X = THREE.MathUtils.lerp(this.currentP90X, p90Target, 0.14);
    this.currentBreakEvenX = THREE.MathUtils.lerp(this.currentBreakEvenX, breakEvenTarget, 0.14);

    this.setLinePoints(this.meanLine, this.currentMeanX, 0, this.currentMeanX, this.maxHeight);
    this.setLinePoints(this.meanLineB, this.currentMeanBX, 0, this.currentMeanBX, this.maxHeight);
    this.setLinePoints(this.p10Line, this.currentP10X, 0, this.currentP10X, this.maxHeight);
    this.setLinePoints(this.p90Line, this.currentP90X, 0, this.currentP90X, this.maxHeight);
    this.setLinePoints(this.breakEvenLine, this.currentBreakEvenX, 0, this.currentBreakEvenX, this.maxHeight);

    this.meanLineB.visible = meta.bimodal;
    if (!meta.bimodal) {
      this.currentMeanBX = this.currentMeanX;
    }

    this.updateAxisLabels(meta);
    this.setLabel(this.simCountLabel, `Simulations ${simulationCount.toLocaleString()}`);
    this.simCountLabel.position.set(0, -0.32, 0.05);
    this.simCountLabel.scale.set(2.9, 0.72, 1);
  }

  dispose(): void {
    for (const bar of this.bars) {
      bar.geometry.dispose();
      (bar.material as THREE.Material).dispose();
    }

    const lines = [this.baseline, this.yAxis, this.meanLine, this.meanLineB, this.p10Line, this.p90Line, this.breakEvenLine];
    for (const line of lines) {
      line.geometry.dispose();
      (line.material as THREE.Material).dispose();
    }

    const labels = [
      this.xMinLabel,
      this.xMaxLabel,
      this.p10Label,
      this.meanLabel,
      this.p90Label,
      this.breakEvenLabel,
      this.simCountLabel,
    ];
    for (const label of labels) {
      const mat = label.material as THREE.SpriteMaterial;
      mat.map?.dispose();
      mat.dispose();
    }
  }
}
