import { setupApp } from '@ui/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.cloudflare', debug: true });
const SECTION_IDS = {
  hero: 'cloudflare-hero-stage',
  status: 'cloudflare-status-table',
  docs: 'cloudflare-docs-docs',
  three: 'cloudflare-three-stage',
  xterm: 'cloudflare-log-xterm',
} as const;

sections.register(SECTION_IDS.hero, {
  containerId: SECTION_IDS.hero,
  load: async () => {
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById(SECTION_IDS.hero);
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Cloudflare v1' },
});

sections.register(SECTION_IDS.status, {
  containerId: SECTION_IDS.status,
  load: async () => {
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById(SECTION_IDS.status);
    if (!container) throw new Error('status container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'Cloudflare Status' },
});

sections.register(SECTION_IDS.xterm, {
  containerId: SECTION_IDS.xterm,
  load: async () => {
    const { mountXterm } = await import('./components/xterm/index');
    const container = document.getElementById(SECTION_IDS.xterm);
    if (!container) throw new Error('xterm container not found');
    return mountXterm(container);
  },
  header: { visible: false, menuVisible: true, title: 'Cloudflare Terminal' },
});

sections.register(SECTION_IDS.docs, {
  containerId: SECTION_IDS.docs,
  load: async () => {
    const { mountDocs } = await import('./components/docs/index');
    const container = document.getElementById(SECTION_IDS.docs);
    if (!container) throw new Error('docs container not found');
    return mountDocs(container);
  },
  header: { visible: false, menuVisible: true, title: 'Cloudflare v1 Docs' },
});

sections.register(SECTION_IDS.three, {
  containerId: SECTION_IDS.three,
  load: async () => {
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById(SECTION_IDS.three);
    if (!container) throw new Error('three container not found');
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'Cloudflare v1 Three' },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo(SECTION_IDS.hero);
});
menu.addButton('Status', 'Navigate Status', () => {
  void sections.navigateTo(SECTION_IDS.status);
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo(SECTION_IDS.docs);
});
menu.addButton('Three', 'Navigate Three', () => {
  void sections.navigateTo(SECTION_IDS.three);
});
menu.addButton('Xterm', 'Navigate Xterm', () => {
  void sections.navigateTo(SECTION_IDS.xterm);
});

const sectionOrder = [SECTION_IDS.hero, SECTION_IDS.status, SECTION_IDS.docs, SECTION_IDS.three, SECTION_IDS.xterm] as const;
const wheelLockedSections: ReadonlySet<string> = new Set([SECTION_IDS.status, SECTION_IDS.three, SECTION_IDS.xterm]);
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
  void sections.navigateTo(nextId).catch((err: unknown) => {
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
      .catch((err: unknown) => {
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
    if (id !== SECTION_IDS.xterm) return;
    const xtermEl = document.querySelector(`#${SECTION_IDS.xterm} [aria-label="Xterm Terminal"]`) as HTMLElement | null;
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
    .catch((err: unknown) => {
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
