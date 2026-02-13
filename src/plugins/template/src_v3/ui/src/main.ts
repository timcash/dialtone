import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.template', debug: true });

sections.register('hero', {
  containerId: 'hero',
  load: async () => {
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('hero');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3' },
});

sections.register('docs', {
  containerId: 'docs',
  load: async () => {
    const { mountDocs } = await import('./components/docs/index');
    const container = document.getElementById('docs');
    if (!container) throw new Error('docs container not found');
    return mountDocs(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3 Docs' },
});

sections.register('table', {
  containerId: 'table',
  load: async () => {
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById('table');
    if (!container) throw new Error('table container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template v3 Table' },
});

sections.register('three', {
  containerId: 'three',
  load: async () => {
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById('three');
    if (!container) throw new Error('three container not found');
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template v3 Three' },
});

sections.register('xterm', {
  containerId: 'xterm',
  load: async () => {
    const { mountXterm } = await import('./components/xterm/index');
    const container = document.getElementById('xterm');
    if (!container) throw new Error('xterm container not found');
    return mountXterm(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template v3 Xterm' },
});

sections.register('video', {
  containerId: 'video',
  load: async () => {
    const { mountVideo } = await import('./components/video/index');
    const container = document.getElementById('video');
    if (!container) throw new Error('video container not found');
    return mountVideo(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3 Video' },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('hero');
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo('docs');
});
menu.addButton('Table', 'Navigate Table', () => {
  void sections.navigateTo('table');
});
menu.addButton('Three', 'Navigate Three', () => {
  void sections.navigateTo('three');
});
menu.addButton('Xterm', 'Navigate Xterm', () => {
  void sections.navigateTo('xterm');
});
menu.addButton('Video', 'Navigate Video', () => {
  void sections.navigateTo('video');
});

const sectionOrder = ['hero', 'docs', 'table', 'three', 'xterm', 'video'] as const;
const wheelLockedSections = new Set(['table', 'three', 'xterm', 'video']);
let wheelGestureActive = false;
let wheelNavInFlight = false;
let wheelGestureTimer = 0;
const sectionSet = new Set(sectionOrder);

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

const hashId = window.location.hash.slice(1);
const initialId = sectionSet.has(hashId as (typeof sectionOrder)[number]) ? hashId : 'hero';
console.log(`[SectionManager] INITIAL LOAD #${initialId}`);
void sections.navigateTo(initialId, { updateHash: hashId !== initialId });
