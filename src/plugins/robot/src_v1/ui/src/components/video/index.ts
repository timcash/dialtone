import { VisualizationControl } from '@ui/types';
import { addMavlinkListener } from '../../data/connection';
import { LatencyEstimator } from '../../data/latency';
import { logError, logInfo } from '../../data/logging';
import { registerButtons, renderButtons } from '../../buttons';

class VideoControl implements VisualizationControl {
  private img: HTMLImageElement | null;
  private sectionEl: HTMLElement;
  private unsubscribe: (() => void) | null = null;
  private latencyEstimator = new LatencyEstimator();
  
  // Watchdog
  private watchdogTimer: ReturnType<typeof setTimeout> | null = null;
  private isPaused = false;
  private watchdogOverlay: HTMLElement;
  private readonly WATCHDOG_TIMEOUT = 3 * 60 * 1000; // 3 minutes

  constructor(private container: HTMLElement) {
    this.sectionEl = container;
    this.img = container.querySelector('img.video-stage');
    
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

    // Register Buttons
    registerButtons('video', ['View'], {
      'View': [
        { label: 'Feed A', action: () => this.updateFeedSource('Primary') },
        { label: 'Feed B', action: () => this.updateFeedSource('Secondary') },
        { label: 'Wide', action: () => this.updateFeedSource('Wide') },
        { label: 'Zoom', action: () => this.updateFeedSource('Zoom') },
        { label: 'IR', action: () => this.updateFeedSource('IR') },
        { label: 'Map', action: () => this.updateFeedSource('Map') },
        { label: 'Log', action: () => this.updateFeedSource('Log') },
        { label: 'Bookmark', action: () => this.bookmarkFrame() },
      ]
    });
    
    this.updateFeedSource('Primary');
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
    this.sectionEl.setAttribute('data-playing', 'false');
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
    this.sectionEl.setAttribute('data-playing', 'true');
    this.subscribe();
  }

  private stopStream() {
    if (this.img) this.img.src = ''; // Stop network request
    this.sectionEl.setAttribute('data-playing', 'false');
    if (this.unsubscribe) {
        this.unsubscribe();
        this.unsubscribe = null;
    }
  }

  private subscribe() {
    if (this.unsubscribe) return;
    this.unsubscribe = addMavlinkListener((raw: any) => {
        const total = this.latencyEstimator.estimate(raw);
        if (total == null) return;
        const el = document.getElementById('vid-latency');
        if (el) el.textContent = `${Math.round(total)}ms`;
    });
  }

  private updateFeedSource(name: string) {
    logInfo('ui/video', `[Video] Switching to feed: ${name}`);
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
        logInfo('ui/video', '[Video] Bookmark saved');
        // Visual feedback? No direct button access anymore, but renderButtons re-renders.
        // I could update button label temporarily?
        // But registerButtons defines static labels/actions.
        // For dynamic feedback, I'd need to update the config and re-render.
        // Or simpler: just log. The HUD is there.
      } else {
        logError('ui/video', `[Video] Bookmark failed: ${res.status}`);
      }
    } catch (err) {
      logError('ui/video', '[Video] Bookmark error', err);
    }
  }

  dispose(): void {
    this.stopWatchdog();
    this.stopStream();
  }

  setVisible(visible: boolean): void {
    if (visible) {
      renderButtons('video');
      if (!this.isPaused) {
        this.startStream();
        this.startWatchdog();
      }
    } else {
      this.stopStream();
      this.stopWatchdog();
      this.isPaused = false;
      this.watchdogOverlay.hidden = true;
    }
  }
}

export function mountVideo(container: HTMLElement): VisualizationControl {
  return new VideoControl(container);
}
