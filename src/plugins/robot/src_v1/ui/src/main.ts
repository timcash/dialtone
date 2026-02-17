import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';
import { JSONCodec, connect, type NatsConnection } from 'nats.ws';

export let NATS_CONNECTION: NatsConnection | null = null;
export const NATS_JSON_CODEC = JSONCodec();

declare const APP_VERSION: string;

const { sections, menu } = setupApp({ title: 'dialtone.robot', debug: true });

// Display version
const versionEl = document.getElementById('app-version');
if (versionEl) {
  versionEl.textContent = `v${APP_VERSION}`;
}

// --- NATS Connection ---
async function initNATS() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const hostname = window.location.hostname;

  try {
    const res = await fetch('/api/init');
    const data = await res.json();
    const wsPort = data.ws_port || 4223;
    const wsPath = data.ws_path || '';

    let server = '';
    if (wsPath) {
      server = `${protocol}//${window.location.host}${wsPath}`;
    } else {
      server = `${protocol}//${hostname}:${wsPort}`;
    }

    console.log(`[NATS] Connecting to ${server}...`);
    NATS_CONNECTION = await connect({ servers: [server] });
    console.log(`[NATS] Connected.`);

    NATS_CONNECTION.closed().then(() => {
      console.warn('[NATS] Connection closed, retrying...');
      setTimeout(initNATS, 2000);
    });
  } catch (err) {
    console.error('[NATS] Connection failed:', err);
    setTimeout(initNATS, 5000);
  }
}

initNATS();

function sendCommand(cmd: string, mode?: string) {
  if (!NATS_CONNECTION) {
    console.warn('[NATS] Not connected, cannot send command:', cmd);
    return;
  }
  const payload: any = { cmd };
  if (mode) payload.mode = mode;
  NATS_CONNECTION.publish('rover.command', NATS_JSON_CODEC.encode(payload));
}

// --- Button Listeners for Three Section ---
document.getElementById('three-arm')?.addEventListener('click', () => sendCommand('arm'));
document.getElementById('three-disarm')?.addEventListener('click', () => sendCommand('disarm'));
document.getElementById('three-manual')?.addEventListener('click', () => sendCommand('mode', 'manual'));
document.getElementById('three-guided')?.addEventListener('click', () => sendCommand('mode', 'guided'));

sections.register('hero', {
  containerId: 'hero',
  load: async () => {
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('hero');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot' },
  overlays: {
    primaryKind: 'stage',
    primary: '.hero-stage',
    thumb: '', // No thumb for hero
    legend: '', // No legend for hero
  },
});

sections.register('docs', {
  containerId: 'docs',
  load: async () => {
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

sections.register('table', {
  containerId: 'table',
  load: async () => {
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById('table');
    if (!container) throw new Error('table container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Telemetry' },
  overlays: {
    primaryKind: 'table',
    primary: '.telemetry-table',
    thumb: '.telemetry-thumb',
    legend: '.telemetry-legend',
  },
});

sections.register('three', {
  containerId: 'three',
  load: async () => {
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById('three');
    if (!container) throw new Error('three container not found');
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot 3D' },
  overlays: {
    primaryKind: 'stage',
    primary: '.three-stage',
    thumb: '.three-thumb',
    legend: '.three-legend',
  },
});

sections.register('xterm', {
  containerId: 'xterm',
  load: async () => {
    const { mountXterm } = await import('./components/xterm/index');
    const container = document.getElementById('xterm');
    if (!container) throw new Error('xterm container not found');
    return mountXterm(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Terminal' },
  overlays: {
    primaryKind: 'xterm',
    primary: '.xterm-primary',
    thumb: '.xterm-thumb',
    legend: '.xterm-legend',
  },
});

sections.register('video', {
  containerId: 'video',
  load: async () => {
    const { mountVideo } = await import('./components/video/index');
    const container = document.getElementById('video');
    if (!container) throw new Error('video container not found');
    return mountVideo(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Camera' },
  overlays: {
    primaryKind: 'stage', // using stage for video canvas or primaryKind: 'video' could be added
    primary: '.video-primary',
    thumb: '.video-thumb',
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

const sectionOrder = ['hero', 'docs', 'table', 'three', 'xterm', 'video'] as const;
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
