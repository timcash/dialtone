import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.dag', debug: true });

sections.register('dag-table', {
  containerId: 'dag-table',
  load: async () => {
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById('dag-table');
    if (!container) throw new Error('dag-table container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'DAG Table' },
});

sections.register('three', {
  containerId: 'three',
  load: async () => {
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById('three');
    if (!container) throw new Error('three container not found');
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'DAG Three' },
});

menu.addButton('Table', 'Navigate Table', () => {
  void sections.navigateTo('dag-table');
});
menu.addButton('Three', 'Navigate Three', () => {
  void sections.navigateTo('three');
});

const sectionSet = new Set(['dag-table', 'three']);
const sectionOrder = ['dag-table', 'three'] as const;
const defaultSection = 'dag-table';

const syncSectionFromURL = () => {
  const hashID = window.location.hash.slice(1);
  const targetID = sectionSet.has(hashID) ? hashID : defaultSection;
  const activeID = sections.getActiveSectionId();
  if (activeID === targetID) return;
  void sections.navigateTo(targetID, { updateHash: hashID !== targetID }).catch((err) => {
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
    const dagMenu = document.querySelector("button[aria-label='DAG Menu']") as HTMLButtonElement | null;
    const globalMenu = document.querySelector("button[aria-label='Toggle Global Menu']") as HTMLButtonElement | null;
    (dagMenu ?? globalMenu)?.click();
  }
});

syncSectionFromURL();
