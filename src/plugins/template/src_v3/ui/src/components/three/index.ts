import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class ThreeControl implements VisualizationControl {
  private visible = false;
  private raf = 0;
  private phase = 0;
  private wheelCount = 0;

  constructor(canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('three canvas context not available');

    const resize = () => {
      canvas.width = canvas.clientWidth;
      canvas.height = canvas.clientHeight;
    };
    resize();
    window.addEventListener('resize', resize);

    canvas.addEventListener('wheel', () => {
      this.wheelCount += 1;
      canvas.setAttribute('data-wheel-count', String(this.wheelCount));
    });

    const frame = () => {
      this.raf = requestAnimationFrame(frame);
      if (!this.visible) return;
      this.phase += 0.02;
      const w = canvas.width;
      const h = canvas.height;
      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = '#111a28';
      ctx.fillRect(0, 0, w, h);
      ctx.strokeStyle = '#53d2ff';
      ctx.lineWidth = 2;
      for (let i = 0; i < 12; i++) {
        const x = (i / 12) * w;
        const y = h * 0.5 + Math.sin(this.phase + i * 0.4) * h * 0.2;
        ctx.beginPath();
        ctx.arc(x, y, 6, 0, Math.PI * 2);
        ctx.stroke();
      }
    };
    frame();

    this.dispose = () => {
      cancelAnimationFrame(this.raf);
      window.removeEventListener('resize', resize);
    };
  }

  dispose(): void {}

  setVisible(visible: boolean): void {
    this.visible = visible;
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  canvas.setAttribute('data-wheel-count', '0');
  return new ThreeControl(canvas);
}
