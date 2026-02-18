import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class HeroControl implements VisualizationControl {
  private raf = 0;
  private visible = false;
  private t = 0;

  constructor(private canvas: HTMLCanvasElement) {
    const ctx = this.canvas.getContext('2d');
    if (!ctx) throw new Error('2d context not available');

    this.resize = () => {
      this.canvas.width = this.canvas.clientWidth;
      this.canvas.height = this.canvas.clientHeight;
    };
    this.resize();
    window.addEventListener('resize', this.resize);

    const frame = () => {
      this.raf = window.requestAnimationFrame(frame);
      if (!this.visible) return;
      this.t += 0.02;

      ctx.fillStyle = '#000';
      ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
      const cx = this.canvas.width * 0.5;
      const cy = this.canvas.height * 0.5;
      for (let i = 0; i < 10; i++) {
        const r = 40 + i * 16 + Math.sin(this.t + i * 0.3) * 10;
        ctx.strokeStyle = `hsla(${190 + i * 7}, 85%, 65%, 0.8)`;
        ctx.lineWidth = 2;
        ctx.beginPath();
        ctx.arc(cx, cy, r, 0, Math.PI * 2);
        ctx.stroke();
      }
    };
    frame();

    this.dispose = () => {
      window.cancelAnimationFrame(this.raf);
      window.removeEventListener('resize', this.resize);
    };
  }

  private resize: () => void;

  dispose(): void {}

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.resize();
    }
  }
}

export function mountHero(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Hero Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('hero canvas not found');
  return new HeroControl(canvas);
}
