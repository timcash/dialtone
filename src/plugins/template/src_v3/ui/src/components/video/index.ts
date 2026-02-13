import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class VideoControl implements VisualizationControl {
  constructor(private video: HTMLVideoElement) {
    video.src = '/video1.mp4';
    video.muted = true;
    video.loop = true;
    video.playsInline = true;
    video.autoplay = true;
    video.preload = 'auto';

    video.addEventListener('play', () => video.setAttribute('data-playing', 'true'));
    video.addEventListener('playing', () => video.setAttribute('data-playing', 'true'));
    video.addEventListener('pause', () => video.setAttribute('data-playing', 'false'));
  }

  dispose(): void {
    this.video.pause();
    this.video.removeAttribute('src');
    this.video.load();
  }

  setVisible(visible: boolean): void {
    if (visible) {
      this.video
        .play()
        .then(() => {
          this.video.setAttribute('data-playing', 'true');
        })
        .catch(() => {
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
