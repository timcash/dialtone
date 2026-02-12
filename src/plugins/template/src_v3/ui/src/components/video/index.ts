import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class VideoControl implements VisualizationControl {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private raf = 0;
  private phase = 0;
  private visible = false;

  constructor(private video: HTMLVideoElement) {
    this.canvas = document.createElement('canvas');
    this.canvas.width = 320;
    this.canvas.height = 180;
    const ctx = this.canvas.getContext('2d');
    if (!ctx) throw new Error('video canvas context not available');
    this.ctx = ctx;

    const stream = this.canvas.captureStream(12);
    video.srcObject = stream;
    video.muted = true;
    video.loop = true;
    video.playsInline = true;

    video.addEventListener('play', () => video.setAttribute('data-playing', 'true'));
    video.addEventListener('pause', () => video.setAttribute('data-playing', 'false'));

    const frame = () => {
      this.raf = requestAnimationFrame(frame);
      if (!this.visible) return;
      this.phase += 0.04;
      this.ctx.fillStyle = '#101824';
      this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
      this.ctx.fillStyle = '#53d2ff';
      const x = this.canvas.width * (0.5 + 0.35 * Math.sin(this.phase));
      const y = this.canvas.height * (0.5 + 0.25 * Math.cos(this.phase * 0.7));
      this.ctx.beginPath();
      this.ctx.arc(x, y, 20, 0, Math.PI * 2);
      this.ctx.fill();
    };
    frame();
  }

  dispose(): void {
    cancelAnimationFrame(this.raf);
    this.video.pause();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.video.play().catch(() => {
        this.video.setAttribute('data-playing', 'false');
      });
      return;
    }
    this.video.pause();
  }
}

export function mountVideo(container: HTMLElement): VisualizationControl {
  const video = container.querySelector("video[aria-label='Test Video']") as HTMLVideoElement | null;
  if (!video) throw new Error('test video element not found');
  video.setAttribute('data-playing', 'false');
  return new VideoControl(video);
}
