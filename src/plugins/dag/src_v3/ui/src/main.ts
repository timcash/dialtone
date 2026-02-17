import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const isLocalDevHost = ['127.0.0.1', 'localhost'].includes(window.location.hostname);

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    if (isLocalDevHost) {
      void navigator.serviceWorker.getRegistrations().then((regs) => {
        regs.forEach((reg) => void reg.unregister());
      });
      if ('caches' in window) {
        void caches.keys().then((keys) => {
          keys.forEach((key) => {
            if (key.includes('dag') || key.includes('dialtone') || key.includes('workbox')) {
              void caches.delete(key);
            }
          });
        });
      }
      return;
    }
    void navigator.serviceWorker.register('/sw.js').catch((err) => {
      console.warn('[DAG PWA] service worker registration failed', err);
    });
  });
}

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
  overlays: {
    primaryKind: 'table',
    primary: "table[aria-label='DAG Table']",
    thumb: '.dag-table-thumb',
    legend: '.dag-table-legend',
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
  header: { visible: false, menuVisible: true, title: 'DAG Stage' },
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Three Canvas']",
    thumb: '.dag-controls',
    legend: '.dag-history',
    chatlog: '.dag-chatlog',
  },
});

const sectionSet = new Set(['dag-table', 'three']);
const sectionOrder = ['dag-table', 'three'] as const;
type DagSectionID = (typeof sectionOrder)[number];
let defaultSection: DagSectionID = 'dag-table';
const sectionStorageKey = 'dag.src_v3.active_section';
const apiReadyStorageKey = 'dag.src_v3.api_ready';

const readStoredSection = (): DagSectionID | null => {
  try {
    const value = window.sessionStorage.getItem(sectionStorageKey);
    if (!value) return null;
    return sectionSet.has(value) ? (value as DagSectionID) : null;
  } catch {
    return null;
  }
};

const readHashSection = (): DagSectionID | null => {
  const hashID = window.location.hash.slice(1);
  return sectionSet.has(hashID) ? (hashID as DagSectionID) : null;
};

const writeStoredSection = (sectionId: DagSectionID) => {
  try {
    window.sessionStorage.setItem(sectionStorageKey, sectionId);
  } catch {
    // ignore storage errors; hash-based routing still applies
  }
};

const isSectionActuallyVisible = (sectionId: DagSectionID): boolean => {
  const section = document.getElementById(sectionId);
  if (!section) return false;
  return !section.hidden && section.getAttribute('data-active') === 'true';
};

const navigateToSection = (sectionId: DagSectionID, updateHash = true) => {
  const active = sections.getActiveSectionId() as DagSectionID | null;
  const activeLooksWrong = active === sectionId && !isSectionActuallyVisible(sectionId);
  if (activeLooksWrong) {
    const repairTarget = sectionOrder.find((id) => id !== sectionId) ?? defaultSection;
    return sections
      .navigateTo(repairTarget, { updateHash: false })
      .then(() => sections.navigateTo(sectionId, { updateHash }))
      .then(() => {
        writeStoredSection(sectionId);
      });
  }
  return sections.navigateTo(sectionId, { updateHash }).then(() => {
    writeStoredSection(sectionId);
  });
};

const probeDagTableAPI = async (): Promise<boolean> => {
  const controller = new AbortController();
  const timeout = window.setTimeout(() => controller.abort(), 900);
  try {
    const res = await fetch('/api/dag-table', {
      method: 'GET',
      headers: { Accept: 'application/json' },
      signal: controller.signal,
    });
    return res.ok;
  } catch {
    return false;
  } finally {
    window.clearTimeout(timeout);
  }
};

const runStartupProbe = async () => {
  const apiReady = await probeDagTableAPI();
  defaultSection = apiReady ? 'dag-table' : 'three';
  try {
    window.sessionStorage.setItem(apiReadyStorageKey, apiReady ? '1' : '0');
  } catch {
    // ignore storage errors
  }
  if (!apiReady) {
    console.warn('[DAG] /api/dag-table unavailable at startup; defaulting to Stage section');
  }
};

menu.addButton('Table', 'Navigate Table', () => {
  void navigateToSection('dag-table');
});
menu.addButton('Stage', 'Navigate Stage', () => {
  void navigateToSection('three');
});

const syncSectionFromURL = () => {
  const hashID = readHashSection();
  const storedSection = readStoredSection();
  const targetID = hashID ?? storedSection ?? defaultSection;
  const activeID = sections.getActiveSectionId();
  if (activeID === targetID && isSectionActuallyVisible(targetID)) {
    writeStoredSection(targetID);
    return;
  }
  void navigateToSection(targetID, hashID !== targetID).catch((err) => {
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
      void navigateToSection(next);
    }
    return;
  }
  if (event.key === 'ArrowUp' || event.key === 'PageUp' || event.key.toLowerCase() === 'k') {
    const prev = sectionOrder[Math.max(0, idx - 1)];
    if (prev !== active) {
      event.preventDefault();
      void navigateToSection(prev);
    }
    return;
  }
  if (event.key.toLowerCase() === 'm') {
    event.preventDefault();
    const globalMenu = document.querySelector("button[aria-label='Toggle Global Menu']") as HTMLButtonElement | null;
    globalMenu?.click();
  }
});

void runStartupProbe().finally(() => {
  syncSectionFromURL();
});

const initialHashSection = readHashSection();
if (initialHashSection) {
  defaultSection = initialHashSection;
}
syncSectionFromURL();
