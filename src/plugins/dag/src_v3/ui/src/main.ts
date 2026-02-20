import { setupApp } from '@ui/ui';
import './style.css';

const isLocalDevHost = ['127.0.0.1', 'localhost'].includes(window.location.hostname);

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    const isTest = new URLSearchParams(window.location.search).has('test');
    if (isTest || isLocalDevHost) {
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

const sectionSet = new Set(['dag-meta-table', 'dag-3d-stage', 'dag-log-xterm']);
const sectionOrder = ['dag-meta-table', 'dag-3d-stage', 'dag-log-xterm'] as const;
type DagSectionID = (typeof sectionOrder)[number];
let defaultSection: DagSectionID = 'dag-meta-table';
const sectionStorageKey = 'dag.src_v3.active_section';
const apiReadyStorageKey = 'dag.src_v3.api_ready';

try {
  const { sections, menu } = setupApp({ title: 'dialtone.dag', debug: true });

  sections.register('dag-meta-table', {
    containerId: 'dag-meta-table',
    load: async () => {
      const { mountTable } = await import('./components/table/index').catch((err) => {
        console.error('[DAG] Failed to import table component', err);
        throw err;
      });
      const container = document.getElementById('dag-meta-table');
      if (!container) throw new Error('dag-meta-table container not found');
      return mountTable(container);
    },
    header: { visible: false, menuVisible: true, title: 'DAG Table' },
    overlays: {
      primaryKind: 'table',
      primary: '.table-wrapper',
      form: '.mode-form',
      legend: '.dag-table-legend',
    },
  });

  sections.register('dag-3d-stage', {
    containerId: 'dag-3d-stage',
    load: async () => {
      const { mountThree } = await import('./components/three/index').catch((err) => {
        console.error('[DAG] Failed to import three component', err);
        throw err;
      });
      const container = document.getElementById('dag-3d-stage');
      if (!container) throw new Error('dag-3d-stage container not found');
      return mountThree(container);
    },
    header: { visible: false, menuVisible: true, title: 'DAG Stage' },
    overlays: {
      primaryKind: 'stage',
      primary: '.three-stage',
      form: '.mode-form',
      legend: '.dag-history',
      chatlog: '.dag-chatlog',
    },
  });

  sections.register('dag-log-xterm', {
    containerId: 'dag-log-xterm',
    load: async () => {
      const { mountLog } = await import('./components/log/index').catch((err) => {
        console.error('[DAG] Failed to import log component', err);
        throw err;
      });
      const container = document.getElementById('dag-log-xterm');
      if (!container) throw new Error('dag-log-xterm container not found');
      return mountLog(container);
    },
    header: { visible: false, menuVisible: true, title: 'DAG Log' },
    overlays: {
      primaryKind: 'xterm',
      primary: '.xterm-primary',
      form: '.mode-form',
      legend: '.dag-log-legend',
    },
  });

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
    defaultSection = apiReady ? 'dag-meta-table' : 'dag-3d-stage';
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
    void navigateToSection('dag-meta-table');
  });
  menu.addButton('Stage', 'Navigate Stage', () => {
    void navigateToSection('dag-3d-stage');
  });
  menu.addButton('Log', 'Navigate Log', () => {
    void navigateToSection('dag-log-xterm');
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

  const getActiveModeForm = (): HTMLFormElement | null => {
    const activeId = sections.getActiveSectionId();
    if (!activeId) return null;
    const section = document.getElementById(activeId);
    if (!section) return null;
    const form = section.querySelector('form');
    return form instanceof HTMLFormElement ? form : null;
  };

  const getModeFormButtons = (form: HTMLFormElement): HTMLButtonElement[] =>
    Array.from(form.querySelectorAll('button')).filter((el): el is HTMLButtonElement => el instanceof HTMLButtonElement && !el.disabled);

  const getModeFormFocusables = (form: HTMLFormElement): HTMLElement[] =>
    Array.from(form.querySelectorAll('button,input,select,textarea,[tabindex]')).filter((el): el is HTMLElement => {
      if (!(el instanceof HTMLElement)) return false;
      if ((el as HTMLButtonElement).disabled) return false;
      if (el.getAttribute('tabindex') === '-1') return false;
      return true;
    });

  window.addEventListener('hashchange', syncSectionFromURL);
  window.addEventListener('pageshow', syncSectionFromURL);
  window.addEventListener('focus', syncSectionFromURL);
  document.addEventListener('visibilitychange', () => {
    if (!document.hidden) syncSectionFromURL();
  });

  window.addEventListener('keydown', (event) => {
    if (event.defaultPrevented) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;

    const modeForm = getActiveModeForm();

    if (/^[1-9]$/.test(event.key) && modeForm) {
      const idx = Number(event.key) - 1;
      const buttons = getModeFormButtons(modeForm);
      const button = buttons[idx];
      if (button) {
        event.preventDefault();
        button.focus();
        button.click();
        return;
      }
    }

    if (event.key === 'Tab' && modeForm) {
      const focusables = getModeFormFocusables(modeForm);
      if (focusables.length > 0) {
        event.preventDefault();
        const activeEl = document.activeElement as HTMLElement | null;
        const current = activeEl ? focusables.indexOf(activeEl) : -1;
        const delta = event.shiftKey ? -1 : 1;
        const next = current < 0 ? (event.shiftKey ? focusables.length - 1 : 0) : (current + delta + focusables.length) % focusables.length;
        focusables[next]?.focus();
        return;
      }
    }

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
  
  console.log('[DAG] App setup complete, starting boot signal interval');
  // Mark app as booted after a short delay to allow layout to settle
  setInterval(() => {
    const now = new Date().toISOString();
    const header = document.querySelector('[aria-label="App Header"]');
    if (header) {
      const currentBoot = header.getAttribute('data-boot');
      if (currentBoot !== 'true') {
        console.log(`[DAG | ${now}] Setting App Header data-boot=true. Previous:`, currentBoot);
        header.setAttribute('data-boot', 'true');
      }
    } else {
      console.warn(`[DAG | ${now}] App Header element NOT FOUND in interval`);
    }
  }, 2000);
} catch (err) {
  console.error('[DAG] Critical app setup failure:', err);
}
