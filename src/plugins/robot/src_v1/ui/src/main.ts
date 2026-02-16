import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { connect, JSONCodec, type NatsConnection } from 'nats.ws';


export let NATS_CONNECTION: NatsConnection | null = null;
export const NATS_JSON_CODEC = JSONCodec();

const { sections, menu } = setupApp({ title: 'dialtone.robot', debug: true });

const HOSTNAME = window.location.hostname;
const PROTOCOL = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

const term = new Terminal({
  theme: {
    background: '#0a0a0c',
    foreground: '#c5c6c7',
    cursor: '#66fcf1',
    selectionBackground: 'rgba(102, 252, 241, 0.3)'
  },
  fontFamily: '"Orbitron", monospace',
  fontSize: 12,
  cursorBlink: true,
  convertEol: true
});
const fitAddon = new FitAddon();
term.loadAddon(fitAddon);

const termContainer = document.getElementById('terminal-container');
if (termContainer) {
  term.open(termContainer);
  fitAddon.fit();

  // Resize Observer for Terminal
  new ResizeObserver(() => {
    fitAddon.fit();
  }).observe(termContainer);
}

sections.register('hero', {
  containerId: 'hero',
  load: async () => {
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('hero');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot' },
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
});

sections.register('controls', {
  containerId: 'controls',
  load: async () => {
    const { mountControls } = await import('./components/controls/index');
    const container = document.getElementById('controls');
    if (!container) throw new Error('controls container not found');
    return mountControls(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Controls' },
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
menu.addButton('Controls', 'Navigate Controls', () => {
  void sections.navigateTo('controls');
});

const sectionOrder = ['hero', 'docs', 'table', 'three', 'xterm', 'video', 'controls'] as const;
const wheelLockedSections = new Set(['table', 'three', 'xterm', 'video']);
let wheelGestureActive = false;
let wheelNavInFlight = false;
let wheelGestureTimer = 0;
const sectionSet = new Set(sectionOrder);
const defaultSection = sectionOrder[0];

const navigateByDelta = (delta: 1 | -1) => {
  const current = sections.getActiveSectionId() ?? sectionOrder[0];
  const currentIndex = sectionOrder.indexOf(current as (typeof sectionOrder)[number]);
  if (currentIndex < 0) return;
  const nextIndex = Math.max(0, Math.min(sectionOrder.length - 1, currentIndex + delta));
  const nextId = sectionOrder[nextIndex];
  if (nextId === current) return;
  void sections.navigateTo(nextId).catch((err) => {
    console.error('[SectionManager] keyboard navigation failed', err);
  });
};

window.addEventListener(
  'wheel',
  (event) => {
    if (Math.abs(event.deltaY) < 4) return;
    const current = sections.getActiveSectionId() ?? sectionOrder[0];
    if (wheelLockedSections.has(current)) {
      return;
    }
    event.preventDefault();
    if (wheelGestureTimer) {
      window.clearTimeout(wheelGestureTimer);
    }
    wheelGestureTimer = window.setTimeout(() => {
      wheelGestureActive = false;
    }, 650);
    if (wheelGestureActive || wheelNavInFlight) return;

    wheelGestureActive = true;
    wheelNavInFlight = true;
    void sections
      .navigateTo(
        sectionOrder[
          Math.max(
            0,
            Math.min(
              sectionOrder.length - 1,
              sectionOrder.indexOf(current as (typeof sectionOrder)[number]) + (event.deltaY > 0 ? 1 : -1)
            )
          )
        ]
      )
      .catch((err) => {
        console.error('[SectionManager] wheel navigation failed', err);
      })
      .finally(() => {
        wheelNavInFlight = false;
      });
  },
  { passive: false, capture: true }
);

window.addEventListener('keydown', (event) => {
  const target = event.target as HTMLElement | null;
  if (target) {
    const tag = target.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable) {
      return;
    }
  }

  if (event.key === 'ArrowDown') {
    event.preventDefault();
    navigateByDelta(1);
  } else if (event.key === 'ArrowUp') {
    event.preventDefault();
    navigateByDelta(-1);
  }
});

if (import.meta.hot) {
  window.addEventListener('hashchange', () => {
    const id = window.location.hash.slice(1);
    if (id !== 'xterm') return;
    const xtermEl = document.querySelector('#xterm [aria-label="Xterm Terminal"]') as HTMLElement | null;
    if (xtermEl) {
      xtermEl.removeAttribute('data-ready');
    }
  });
}

const syncSectionFromURL = (reason = 'event') => {
  const currentURL = window.location.href;
  const hashId = window.location.hash.slice(1);
  const targetId = sectionSet.has(hashId as (typeof sectionOrder)[number]) ? hashId : defaultSection;
  const activeId = sections.getActiveSectionId();
  console.log(`[SectionManager] URL PAGE reason=${reason} ${currentURL} hash=${hashId || '(none)'} active=${activeId || '(none)'} target=${targetId}`);
  if (activeId === targetId) return;
  console.log(`[SectionManager] URL SYNC #${targetId}`);
  void sections
    .navigateTo(targetId, { updateHash: hashId !== targetId })
    .then(() => {
      const nextActive = sections.getActiveSectionId();
      console.log(`[SectionManager] URL SYNC DONE target=${targetId} active=${nextActive || '(none)'}`);
      if (nextActive !== targetId) {
        window.setTimeout(() => syncSectionFromURL('retry'), 120);
      }
    })
    .catch((err) => {
      console.error(`[SectionManager] URL SYNC FAILED #${targetId}`, err);
      window.setTimeout(() => syncSectionFromURL('retry-error'), 250);
    });
};

window.addEventListener('hashchange', () => syncSectionFromURL('hashchange'));
window.addEventListener('pageshow', () => syncSectionFromURL('pageshow'));
window.addEventListener('focus', () => syncSectionFromURL('focus'));
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) {
    syncSectionFromURL('visibility');
  }
});

syncSectionFromURL('startup');
