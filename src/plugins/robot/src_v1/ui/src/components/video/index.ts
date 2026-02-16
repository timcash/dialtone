import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class VideoControl implements VisualizationControl {
  private img: HTMLImageElement;

  constructor(private container: HTMLElement) {
    this.img = document.createElement('img');
    this.img.setAttribute('aria-label', 'Robot Stream');
    this.img.style.width = '100%';
    this.img.style.height = '100%';
    this.img.style.objectFit = 'cover';
    this.img.src = '/stream';

    const video = container.querySelector("video[aria-label='Test Video']") as HTMLVideoElement | null;
    if (video) {
      video.style.display = 'none';
      container.appendChild(this.img);
    }
  }

  dispose(): void {
    this.img.src = '';
    this.img.remove();
  }

  setVisible(visible: boolean): void {
    if (visible) {
      this.img.src = '/stream';
      this.container.setAttribute('data-playing', 'true');
    } else {
      this.img.src = '';
      this.container.setAttribute('data-playing', 'false');
    }
  }
}

export function mountVideo(container: HTMLElement): VisualizationControl {
  return new VideoControl(container);
}
