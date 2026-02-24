import { setupApp } from '@ui/ui';
import './style.css';

declare const APP_VERSION: string;

// 1. Initialize App
const { sections, menu } = setupApp({ 
    title: 'dialtone.wsl',
    debug: true
});

// Display version
const versionEl = document.getElementById('app-version');
if (versionEl) {
  versionEl.textContent = `v${APP_VERSION || 'dev'}`;
}

// 2. Register Sections
sections.register('home', { 
    containerId: 'home',
    load: async () => {
        sections.setLoadingMessage('home', 'loading hero ...');
        const { mountHero } = await import('./components/home/index');
        const container = document.getElementById('home');
        if (!container) throw new Error('home container not found');
        return mountHero(container);
    },
    header: { visible: false, menuVisible: true, title: 'WSL' },
    overlays: {
        primaryKind: 'stage',
        primary: '.viz-container',
        thumb: '[data-mode-form="home"]',
        legend: '.marketing-overlay',
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
    header: { visible: false, menuVisible: true, title: 'WSL Docs' },
    overlays: {
        primaryKind: 'docs',
        primary: '.settings-container',
        thumb: '[data-mode-form="docs"]',
        legend: '.docs-legend',
    },
});

sections.register('table', { 
    containerId: 'table',
    load: async () => {
        sections.setLoadingMessage('table', 'loading spreadsheet ...');
        const { mountTable } = await import('./components/table/index');
        const container = document.getElementById('table');
        if (!container) throw new Error('table container not found');
        return mountTable(container);
    },
    header: { visible: false, menuVisible: true, title: 'WSL Table' },
    overlays: {
        primaryKind: 'table',
        primary: '.explorer-container',
        thumb: '[data-mode-form="table"]',
        legend: '.table-legend',
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
        primaryKind: 'docs',
        primary: '.settings-container',
        thumb: '[data-mode-form="settings"]',
        legend: '.settings-legend',
    },
});

// 3. Setup Global Menu
menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('home');
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo('docs');
});
menu.addButton('Table', 'Navigate Table', () => {
  void sections.navigateTo('table');
});
menu.addButton('Settings', 'Navigate Settings', () => {
  void sections.navigateTo('settings');
});

// 4. Navigation Sync
const sectionOrder = ['home', 'docs', 'table', 'settings'] as const;
const sectionSet = new Set(sectionOrder);
const defaultSection = sectionOrder[0];

const syncSectionFromURL = () => {
  const hashId = window.location.hash.slice(1);
  const targetId = sectionSet.has(hashId as (typeof sectionOrder)[number]) ? hashId : defaultSection;
  const activeId = sections.getActiveSectionId();
  if (activeId === targetId) return;
  void sections.navigateTo(targetId, { updateHash: hashId !== targetId }).catch(() => {});
};

window.addEventListener('hashchange', syncSectionFromURL);
window.addEventListener('pageshow', syncSectionFromURL);
window.addEventListener('focus', syncSectionFromURL);

// 5. Global Keyboard Shortcuts
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
    menu.toggle();
  }
});

syncSectionFromURL();
