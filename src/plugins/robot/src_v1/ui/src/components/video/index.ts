import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class VideoControl implements VisualizationControl {
  private img: HTMLImageElement | null;
  private form: HTMLFormElement | null = null;
  private thumbButtons: HTMLButtonElement[] = [];
  private modeButton: HTMLButtonElement | null = null;
  private visible = false;

  constructor(private container: HTMLElement) {
    this.img = container.querySelector('img.video-stage');
    this.form = container.querySelector("form[data-mode-form='video']");
    
    if (this.form) {
      this.thumbButtons = Array.from(this.form.querySelectorAll("button[aria-label^='Video Thumb']"));
      this.modeButton = this.form.querySelector("button[aria-label='Video Mode']");
      this.bindButtons();
    }
    
    this.updateFeedSource('Primary');
  }

  private bindButtons() {
    this.thumbButtons.forEach((btn, idx) => {
      btn.addEventListener('click', () => this.handleThumbClick(idx));
    });
  }

  private handleThumbClick(idx: number) {
    // 0-based index: 0=Feed A, 1=Feed B, ..., 7=Bookmark
    if (idx === 7) {
      this.bookmarkFrame();
      return;
    }
    
    const feeds = ['Primary', 'Secondary', 'Wide', 'Zoom', 'IR', 'Map', 'Log'];
    if (idx < feeds.length) {
      this.updateFeedSource(feeds[idx]);
    }
  }

  private updateFeedSource(name: string) {
    // For now, we mock feed switching by logging or updating UI text
    // In real implementation, this would switch mjpeg url or webrtc track
    console.log(`[Video] Switching to feed: ${name}`);
    const sourceEl = this.container.querySelector('#vid-source');
    if (sourceEl) sourceEl.textContent = name.toUpperCase();
    
    // Example: Toggle source URL if we had multiple
    // if (this.img) this.img.src = `/stream?feed=${name.toLowerCase()}`;
  }

  private async bookmarkFrame() {
    if (!this.img) return;
    
    try {
      // Create a canvas to grab the frame
      const canvas = document.createElement('canvas');
      canvas.width = this.img.naturalWidth || 1280;
      canvas.height = this.img.naturalHeight || 720;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      
      // Draw current image state to canvas
      // Note: this works for MJPEG <img> tags in most browsers if CORS is satisfied
      ctx.drawImage(this.img, 0, 0, canvas.width, canvas.height);
      
      const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.8));
      if (!blob) throw new Error('Frame capture failed');
      
      // Upload to backend
      const formData = new FormData();
      formData.append('image', blob, `bookmark_${Date.now()}.jpg`);
      
      const res = await fetch('/api/bookmark', {
        method: 'POST',
        body: formData
      });
      
      if (res.ok) {
        console.log('[Video] Bookmark saved');
        // Visual feedback?
        const btn = this.thumbButtons[7];
        const originalText = btn.textContent;
        btn.textContent = 'Saved!';
        setTimeout(() => btn.textContent = originalText, 1000);
      } else {
        console.error('[Video] Bookmark failed', res.status);
      }
    } catch (err) {
      console.error('[Video] Bookmark error', err);
    }
  }

  dispose(): void {
    if (this.img) this.img.src = '';
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (this.img) {
      if (visible) {
        // Re-attach stream source when visible to save bandwidth when hidden?
        // Or keep it running. For now, keep simple.
        if (!this.img.src.includes('/stream')) {
            this.img.src = '/stream';
        }
      } else {
        // Optional: stop stream when hidden
        // this.img.src = '';
      }
    }
  }
}

export function mountVideo(container: HTMLElement): VisualizationControl {
  return new VideoControl(container);
}
