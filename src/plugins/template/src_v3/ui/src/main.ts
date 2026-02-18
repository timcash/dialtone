import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.template', debug: true });

sections.register('template-hero-stage', {
  containerId: 'template-hero-stage',
  load: async () => {
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('template-hero-stage');
    if (!container) throw new Error('template-hero-stage container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template Hero Stage' },
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Hero Canvas']",
    statusBar: '.template-status-bar',
  },
});

sections.register('template-docs-docs', {
  containerId: 'template-docs-docs',
  load: async () => {
    const { mountDocs } = await import('./components/docs/index');
    const container = document.getElementById('template-docs-docs');
    if (!container) throw new Error('template-docs-docs container not found');
    return mountDocs(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template Docs' },
  overlays: {
    primaryKind: 'docs',
    primary: "[aria-label='Docs Title']",
    statusBar: '.template-status-bar',
  },
});

sections.register('template-meta-table', {
  containerId: 'template-meta-table',
  load: async () => {
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById('template-meta-table');
    if (!container) throw new Error('template-meta-table container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template Meta Table' },
  overlays: {
    primaryKind: 'table',
    primary: "table[aria-label='Template Table']",
    modeForm: "form[data-mode-form='table']",
    statusBar: '.template-status-bar',
  },
});

sections.register('template-three-stage', {
  containerId: 'template-three-stage',
  load: async () => {
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById('template-three-stage');
    if (!container) throw new Error('template-three-stage container not found');
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template Three Stage' },
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Three Canvas']",
    modeForm: "form[data-mode-form='three']",
    legend: '.three-history',
    chatlog: '.three-chatlog',
    statusBar: '.template-status-bar',
  },
});

sections.register('template-log-xterm', {
  containerId: 'template-log-xterm',
  load: async () => {
    const { mountLog } = await import('./components/log/index');
    const container = document.getElementById('template-log-xterm');
    if (!container) throw new Error('template-log-xterm container not found');
    return mountLog(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template Log Xterm' },
  overlays: {
    primaryKind: 'xterm',
    primary: "[aria-label='Log Terminal']",
    statusBar: '.template-status-bar',
  },
});

sections.register('template-demo-video', {
  containerId: 'template-demo-video',
  load: async () => {
    const { mountVideo } = await import('./components/video/index');
    const container = document.getElementById('template-demo-video');
    if (!container) throw new Error('template-demo-video container not found');
    return mountVideo(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template Demo Video' },
  overlays: {
    primaryKind: 'video',
    primary: "video[aria-label='Test Video']",
    statusBar: '.template-status-bar',
  },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('template-hero-stage');
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo('template-docs-docs');
});
menu.addButton('Table', 'Navigate Table', () => {
  void sections.navigateTo('template-meta-table');
});
menu.addButton('Stage', 'Navigate Stage', () => {
  void sections.navigateTo('template-three-stage');
});
menu.addButton('Log', 'Navigate Log', () => {
  void sections.navigateTo('template-log-xterm');
});
menu.addButton('Video', 'Navigate Video', () => {
  void sections.navigateTo('template-demo-video');
});

const sectionOrder = [
  'template-hero-stage',
  'template-docs-docs',
  'template-meta-table',
  'template-three-stage',
  'template-log-xterm',
  'template-demo-video',
] as const;
const wheelLockedSections = new Set(['template-meta-table', 'template-three-stage', 'template-log-xterm', 'template-demo-video']);
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
