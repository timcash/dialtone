import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';
import { initConnection, sendCommand } from './data/connection';
import { handleButtonKey } from './buttons';

declare const APP_VERSION: string;

const { sections, menu } = setupApp({ title: 'dialtone.robot', debug: true });

const isLocalDevHost = ['127.0.0.1', 'localhost'].includes(window.location.hostname);

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    // Aggressively unregister any service workers to prevent stale app shell
    void navigator.serviceWorker.getRegistrations().then((regs) => {
        regs.forEach((reg) => {
            console.log('[SW] Unregistering stale worker:', reg.scope);
            void reg.unregister();
        });
    });
    
    // Clear caches that might be holding stale assets
    if ('caches' in window) {
        void caches.keys().then((keys) => {
            keys.forEach((key) => {
            if (key.includes('robot') || key.includes('dialtone') || key.includes('workbox')) {
                console.log('[Cache] Deleting stale cache:', key);
                void caches.delete(key);
            }
            });
        });
    }
  });
}

// Display version
const versionEl = document.getElementById('app-version');
if (versionEl) {
  const stamp = isLocalDevHost ? ` (dev-${new Date().toLocaleTimeString()})` : '';
  versionEl.textContent = `v${APP_VERSION}${stamp}`;
}

// Initialize Connection (NATS + Polling)
initConnection();

const checkForUpdate = async () => {
  try {
    const res = await fetch('/api/init');
    if (!res.ok) return;
    const data = await res.json();
    if (data.version && data.version !== APP_VERSION) {
      showUpdateToast(data.version);
    }
  } catch (err) {
    // Ignore offline errors
  }
};

const showUpdateToast = (newVersion: string) => {
  if (document.getElementById('update-toast')) return;
  const toast = document.createElement('button');
  toast.id = 'update-toast';
  toast.style.cssText = `
    position: fixed; top: 80px; right: 20px; z-index: 2000;
    background: var(--theme-primary, #7bf2d8); color: #000;
    padding: 12px 20px; border-radius: 8px; border: none;
    font-weight: 700; cursor: pointer; box-shadow: 0 4px 12px rgba(0,0,0,0.5);
    animation: slideIn 0.3s ease-out;
  `;
  toast.textContent = `Update Available: v${newVersion} (Click to Reload)`;
  toast.onclick = () => {
    // Force reload bypassing cache
    window.location.reload();
  };
  document.body.appendChild(toast);
};

// Check for updates on load and periodically
checkForUpdate();
setInterval(checkForUpdate, 60000);

sections.register('hero', {
  containerId: 'hero',
  load: async () => {
    sections.setLoadingMessage('hero', 'loading hero ...');
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('hero');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot' },
  overlays: {
    primaryKind: 'stage',
    primary: '.hero-stage',
    thumb: '.mode-form',
    legend: '.hero-legend',
  },
});

sections.register('docs', {
  containerId: 'docs',
  load: async () => {
    sections.setLoadingMessage('docs', 'loading documentation ...');
    const { mountDocs } = await import('./components/docs/index');
    const container = document.getElementById('docs');
    if (!container) throw new Error('docs container not found');
    return mountDocs(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Docs' },
  overlays: {
    primaryKind: 'docs',
    primary: '.docs-primary',
    thumb: '.docs-thumb',
    legend: '.docs-legend',
  },
});

sections.register('settings', {
  containerId: 'settings',
  load: async () => {
    sections.setLoadingMessage('settings', 'loading settings ...');
    const { mountSettings } = await import('./components/settings/index');
    const container = document.getElementById('settings');
    if (!container) throw new Error('settings container not found');
    return mountSettings(container);
  },
  header: { visible: false, menuVisible: true, title: 'Settings' },
  overlays: {
    primaryKind: 'docs', // Reuse docs layout logic
    primary: '.settings-primary',
    thumb: '.settings-thumb',
    legend: '.settings-legend',
  },
});

sections.register('table', {
  containerId: 'table',
  load: async () => {
    sections.setLoadingMessage('table', 'loading telemetry ...');
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById('table');
    if (!container) throw new Error('table container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Telemetry' },
  overlays: {
    primaryKind: 'table',
    primary: '.table-wrapper', // Updated to wrapper
    thumb: '.mode-form',
    legend: '.telemetry-legend',
  },
});

sections.register('three', {
  containerId: 'three',
  load: async () => {
    sections.setLoadingMessage('three', 'loading 3d environment ...');
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById('three');
    if (!container) throw new Error('three container not found');
    
    // Apply chatlog setting on load
    const chatlog = container.querySelector('.three-chatlog') as HTMLElement;
    if (chatlog) {
        chatlog.hidden = localStorage.getItem('robot.chatlog.enabled') !== 'true';
    }
    
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot 3D' },
  overlays: {
    primaryKind: 'stage',
    primary: '.three-stage',
    thumb: '.mode-form',
    legend: '.three-legend',
    chatlog: '.three-chatlog', // Added chatlog overlay
  },
});

sections.register('xterm', {
  containerId: 'xterm',
  load: async () => {
    sections.setLoadingMessage('xterm', 'loading terminal ...');
    const { mountXterm } = await import('./components/xterm/index');
    const container = document.getElementById('xterm');
    if (!container) throw new Error('xterm container not found');
    return mountXterm(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Terminal' },
  overlays: {
    primaryKind: 'xterm',
    primary: '.xterm-primary',
    thumb: '.mode-form',
    legend: '.xterm-legend',
  },
});

sections.register('video', {
  containerId: 'video',
  load: async () => {
    sections.setLoadingMessage('video', 'loading camera stream ...');
    const { mountVideo } = await import('./components/video/index');
    const container = document.getElementById('video');
    if (!container) throw new Error('video container not found');
    return mountVideo(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Camera' },
  overlays: {
    primaryKind: 'stage',
    primary: '.video-stage',
    thumb: '.mode-form',
    legend: '.video-legend',
  },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('hero');
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo('docs');
});
menu.addButton('Telemetry', 'Navigate Telemetry', () => {
  void sections.navigateTo('table');
});
menu.addButton('Three', 'Navigate Three', () => {
  void sections.navigateTo('three');
});
menu.addButton('Terminal', 'Navigate Terminal', () => {
  void sections.navigateTo('xterm');
});
menu.addButton('Camera', 'Navigate Camera', () => {
  void sections.navigateTo('video');
});
menu.addButton('Settings', 'Navigate Settings', () => {
  void sections.navigateTo('settings');
});

const sectionOrder = ['hero', 'docs', 'table', 'three', 'xterm', 'video', 'settings'] as const;
const sectionSet = new Set(sectionOrder);
const defaultSection = sectionOrder[0];

const syncSectionFromURL = () => {
  const hashId = window.location.hash.slice(1);
  const targetId = sectionSet.has(hashId as (typeof sectionOrder)[number]) ? hashId : defaultSection;
  const activeId = sections.getActiveSectionId();
  if (activeId === targetId) return;
  void sections.navigateTo(targetId, { updateHash: hashId !== targetId }).catch((err) => {
    console.error('[SectionManager] URL sync failed', err);
  });
};

window.addEventListener('hashchange', syncSectionFromURL);
window.addEventListener('pageshow', syncSectionFromURL);
window.addEventListener('focus', syncSectionFromURL);
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) syncSectionFromURL();
});

window.addEventListener('keydown', (event) => {
  if (event.defaultPrevented) return;
  const target = event.target as HTMLElement | null;
  if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;

  const active = sections.getActiveSectionId() ?? defaultSection;
  
  // Handle Number Keys 1-9
  if (event.key >= '1' && event.key <= '9') {
    event.preventDefault();
    handleButtonKey(active, parseInt(event.key) - 1);
    return;
  }

  const idx = sectionOrder.indexOf(active as (typeof sectionOrder)[number]);
  if (idx < 0) return;

  if (event.key === 'ArrowDown' || event.key === 'PageDown' || event.key.toLowerCase() === 'j') {
    const next = sectionOrder[Math.min(sectionOrder.length - 1, idx + 1)];
    if (next !== active) {
      event.preventDefault();
      void sections.navigateTo(next);
    }
    return;
  }
  if (event.key === 'ArrowUp' || event.key === 'PageUp' || event.key.toLowerCase() === 'k') {
    const prev = sectionOrder[Math.max(0, idx - 1)];
    if (prev !== active) {
      event.preventDefault();
      void sections.navigateTo(prev);
    }
    return;
  }
  if (event.key.toLowerCase() === 'm') {
    event.preventDefault();
    const globalMenu = document.querySelector("button[aria-label='Toggle Global Menu']") as HTMLButtonElement | null;
    globalMenu?.click();
  }
});

let lastWheelTime = 0;
const wheelThrottle = 800; // ms

window.addEventListener('wheel', (event) => {
  const now = Date.now();
  if (now - lastWheelTime < wheelThrottle) return;

  const active = sections.getActiveSectionId() ?? defaultSection;
  const idx = sectionOrder.indexOf(active as (typeof sectionOrder)[number]);
  if (idx < 0) return;

  if (Math.abs(event.deltaY) > 20) {
    if (event.deltaY > 0 && idx < sectionOrder.length - 1) {
      lastWheelTime = now;
      void sections.navigateTo(sectionOrder[idx + 1]);
    } else if (event.deltaY < 0 && idx > 0) {
      lastWheelTime = now;
      void sections.navigateTo(sectionOrder[idx - 1]);
    }
  }
}, { passive: true });

syncSectionFromURL();
