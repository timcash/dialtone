import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { addMavlinkListener } from '../../data/connection';

class VideoControl implements VisualizationControl {
  private img: HTMLImageElement | null;
  private form: HTMLFormElement | null = null;
  private thumbButtons: HTMLButtonElement[] = [];
  private modeButton: HTMLButtonElement | null = null;
  private visible = false;
  private unsubscribe: (() => void) | null = null;
  
  // Watchdog
  private watchdogTimer: ReturnType<typeof setTimeout> | null = null;
  private isPaused = false;
  private watchdogOverlay: HTMLElement;
  private readonly WATCHDOG_TIMEOUT = 3 * 60 * 1000; // 3 minutes

  constructor(private container: HTMLElement) {
    this.img = container.querySelector('img.video-stage');
    this.form = container.querySelector("form[data-mode-form='video']");
    
    // Create Watchdog Overlay
    this.watchdogOverlay = document.createElement('div');
    this.watchdogOverlay.className = 'video-watchdog';
    this.watchdogOverlay.hidden = true;
    this.watchdogOverlay.innerHTML = `
      <div class="watchdog-content">
        <h3>Are you still watching?</h3>
        <p>Video paused to save bandwidth.</p>
        <button class="watchdog-btn">Continue Watching</button>
      </div>
    `;
    this.container.appendChild(this.watchdogOverlay);
    
    const btn = this.watchdogOverlay.querySelector('.watchdog-btn');
    if (btn) btn.addEventListener('click', () => this.resumeStream());

    if (this.form) {
      this.thumbButtons = Array.from(this.form.querySelectorAll("button[aria-label^='Video Thumb']"));
      this.modeButton = this.form.querySelector("button[aria-label='Video Mode']");
      this.bindButtons();
    }
    
    this.updateFeedSource('Primary');
    // Don't subscribe immediately, wait for setVisible
  }

  private startWatchdog() {
    this.stopWatchdog();
    this.watchdogTimer = setTimeout(() => this.pauseStream(), this.WATCHDOG_TIMEOUT);
  }

  private stopWatchdog() {
    if (this.watchdogTimer) {
      clearTimeout(this.watchdogTimer);
      this.watchdogTimer = null;
    }
  }

  private pauseStream() {
    this.isPaused = true;
    this.stopStream();
    this.watchdogOverlay.hidden = false;
  }

  private resumeStream() {
    this.isPaused = false;
    this.watchdogOverlay.hidden = true;
    this.startStream();
    this.startWatchdog();
  }

  private startStream() {
    if (this.img && !this.img.src.includes('/stream')) {
        this.img.src = '/stream?t=' + Date.now();
    }
    this.subscribe();
  }

  private stopStream() {
    if (this.img) this.img.src = ''; // Stop network request
    if (this.unsubscribe) {
        this.unsubscribe();
        this.unsubscribe = null;
    }
  }

  private subscribe() {
    if (this.unsubscribe) return;
    this.unsubscribe = addMavlinkListener((raw: any) => {
        // Latency calculation
        if (raw.t_raw !== undefined) {
            const now = Date.now();
            let t_raw = raw.t_raw || raw.timestamp;
            if (t_raw < 10000000000) t_raw *= 1000;
            
            const total = now - t_raw;
            if (total > 10000 || total < -1000) return;

            const el = document.getElementById('vid-latency');
            if (el) el.textContent = `${total}ms`;
        }
    });
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
    
    // Reset watchdog on interaction
    if (this.visible && !this.isPaused) {
        this.startWatchdog();
    }
    
    const feeds = ['Primary', 'Secondary', 'Wide', 'Zoom', 'IR', 'Map', 'Log'];
    if (idx < feeds.length) {
      this.updateFeedSource(feeds[idx]);
    }
  }

  private updateFeedSource(name: string) {
    console.log(`[Video] Switching to feed: ${name}`);
    const sourceEl = this.container.querySelector('#vid-source');
    if (sourceEl) sourceEl.textContent = name.toUpperCase();
  }

  private async bookmarkFrame() {
    if (!this.img) return;
    
    try {
      const canvas = document.createElement('canvas');
      canvas.width = this.img.naturalWidth || 1280;
      canvas.height = this.img.naturalHeight || 720;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      
      ctx.drawImage(this.img, 0, 0, canvas.width, canvas.height);
      
      const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.8));
      if (!blob) throw new Error('Frame capture failed');
      
      const formData = new FormData();
      formData.append('image', blob, `bookmark_${Date.now()}.jpg`);
      
      const res = await fetch('/api/bookmark', {
        method: 'POST',
        body: formData
      });
      
      if (res.ok) {
        console.log('[Video] Bookmark saved');
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
    this.stopWatchdog();
    this.stopStream();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      if (!this.isPaused) {
        this.startStream();
        this.startWatchdog();
      }
    } else {
      this.stopStream();
      this.stopWatchdog();
      // We don't reset isPaused here; if user returns, they might still see paused state if we wanted persistence,
      // but usually navigating away resets the interaction.
      // The user requirement: "pops up after 3 minutes".
      // If I navigate away and back, should it reset?
      // "when not the current section ... stops streaming" -> Verified by stopStream() in else block.
      // Resetting isPaused to false on navigate away is safer for UX.
      this.isPaused = false;
      this.watchdogOverlay.hidden = true;
    }
  }
}

export function mountVideo(container: HTMLElement): VisualizationControl {
  return new VideoControl(container);
}
