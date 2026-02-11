const FPS_SAMPLE_MS = 500;

const formatFpsLabel = (
  label: string,
  fps: number | null,
  cpuMs: number | null,
  gpuMs: number | null,
): string => {
  if (fps === null) {
    return "FPS --";
  }
  const cpuText = cpuMs === null ? "CPU --" : `CPU ${cpuMs.toFixed(2)} ms`;
  const gpuText = gpuMs === null ? "GPU --" : `GPU ${gpuMs.toFixed(2)} ms`;
  return `FPS (${label}): ${Math.round(fps)} · ${cpuText} · ${gpuText}`;
};

const getFpsElement = (): HTMLElement | null =>
  document.querySelector(".header-fps");

export const FpsDisplay = {
  set(label: string, fps: number | null, cpuMs: number | null, gpuMs: number | null) {
    const el = getFpsElement();
    if (!el) return;
    const next = formatFpsLabel(label, fps, cpuMs, gpuMs);
    if (el.textContent !== next) {
      el.textContent = next;
    }
  },
  clear() {
    const el = getFpsElement();
    if (!el) return;
    if (el.textContent !== "FPS --") {
      el.textContent = "FPS --";
    }
  },
};

export class FpsCounter {
  private label: string;
  private lastSample = performance.now();
  private frames = 0;
  private cpuTotalMs = 0;
  private gpuTotalMs = 0;
  private cpuSamples = 0;
  private gpuSamples = 0;

  constructor(label: string) {
    this.label = label;
  }

  tick(cpuMs?: number, gpuMs?: number | null) {
    this.frames += 1;
    if (cpuMs !== undefined) {
      this.cpuTotalMs += cpuMs;
      this.cpuSamples += 1;
    }
    if (gpuMs !== undefined && gpuMs !== null) {
      this.gpuTotalMs += gpuMs;
      this.gpuSamples += 1;
    }
    const now = performance.now();
    const elapsed = now - this.lastSample;
    if (elapsed < FPS_SAMPLE_MS) return;

    const fps = (this.frames * 1000) / elapsed;
    const avgCpuMs =
      this.cpuSamples > 0 ? this.cpuTotalMs / this.cpuSamples : null;
    const avgGpuMs =
      this.gpuSamples > 0 ? this.gpuTotalMs / this.gpuSamples : null;
    this.frames = 0;
    this.lastSample = now;
    this.cpuTotalMs = 0;
    this.gpuTotalMs = 0;
    this.cpuSamples = 0;
    this.gpuSamples = 0;
    FpsDisplay.set(this.label, fps, avgCpuMs, avgGpuMs);
  }

  clear() {
    this.frames = 0;
    this.lastSample = performance.now();
    this.cpuTotalMs = 0;
    this.gpuTotalMs = 0;
    this.cpuSamples = 0;
    this.gpuSamples = 0;
    FpsDisplay.clear();
  }
}
